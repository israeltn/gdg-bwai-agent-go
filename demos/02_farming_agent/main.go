// Demo 01: AI Farming Agent — Multi-Agent, Tool Calling + Memory
//
// A complete Nigerian farming assistant that shows three core AI agent concepts
// in one live demo:
//
//   MULTI-AGENT    ScoutAgent gathers live data with tools.
//                  PlannerAgent synthesises an actionable recommendation.
//
//   TOOL CALLING   get_crop_price, check_weather, convert_currency, get_farming_tip
//
//   MEMORY         save_memory and recall_memory backed by SQLite.
//                  Facts persist between questions and across restarts.
//
// ─────────────────────────────────────────────────────────────────────────────
// ⚠  INTENTIONAL VULNERABILITY (sets up Demo 02)
//
// save_memory accepts ANY key — including PII such as phone numbers, bank
// account details, or national ID numbers.  These are stored unencrypted in a
// plain SQLite file on disk.  Demo 02 shows how to fix this with a key allowlist.
// ─────────────────────────────────────────────────────────────────────────────
//
// Example session:
//   You:   Should I plant maize in Lokoja this week?
//   Scout: [calls check_weather + get_crop_price + get_farming_tip]
//   Plan:  Weather is good, price is ₦350/kg — plant now. Steps: …
//
//   You:   Remember my farm is in Lokoja
//   Scout: [calls save_memory(farm_location = Lokoja)]
//
//   You:   What crops should I grow?
//   Scout: [calls recall_memory(farm_location)] → Lokoja
//          [calls check_weather(city=Lokoja) + get_farming_tip …]
//
//   You:   Remember my contact number is 08012345678          ← PII demo
//   Scout: [saves phone_number = 08012345678 — no validation!]
//
// Commands: 'memory' shows all stored facts  |  'quit' exits
//
// Run: go run ./demos/01_farming_agent/
package main

import (
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/gdg-lokoja/bwai-agent-go/internal/farming"

	_ "modernc.org/sqlite"
)

// ── SQLite memory store ───────────────────────────────────────────────────────

func openMemoryDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS memory (
		key   TEXT PRIMARY KEY,
		value TEXT NOT NULL
	)`)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("create table: %w", err)
	}
	return db, nil
}

func listMemory(db *sql.DB) {
	rows, err := db.Query(`SELECT key, value FROM memory ORDER BY key`)
	if err != nil {
		fmt.Printf("  [error: %v]\n", err)
		return
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var k, v string
		rows.Scan(&k, &v)
		fmt.Printf("  %-22s = %s\n", k, v)
		count++
	}
	if count == 0 {
		fmt.Println("  (empty)")
	}
}

// loadMemoryContext reads all stored facts from SQLite and formats them as a
// plain-text block to inject into the Scout's query. This ensures the LLM
// always has access to persisted facts even if it forgets to call recall_memory.
func loadMemoryContext(db *sql.DB) string {
	rows, err := db.Query(`SELECT key, value FROM memory ORDER BY key`)
	if err != nil {
		return ""
	}
	defer rows.Close()
	var sb strings.Builder
	for rows.Next() {
		var k, v string
		rows.Scan(&k, &v)
		sb.WriteString(fmt.Sprintf("  %s = %s\n", k, v))
	}
	return sb.String()
}

// ── Memory tool input types ───────────────────────────────────────────────────

type SaveInput struct {
	Key   string `json:"key"   jsonschema:"required" jsonschema_description:"Short label for this fact, e.g. farm_location, grown_crops, phone_number"`
	Value string `json:"value" jsonschema:"required" jsonschema_description:"Value to remember"`
}

type RecallInput struct {
	Key string `json:"key" jsonschema:"required" jsonschema_description:"Label used when the fact was saved"`
}

// ── Memory tool constructors ──────────────────────────────────────────────────

// newSaveMemoryTool stores any key/value in SQLite.
// ⚠  No key validation — intentionally accepts PII. Fixed in Demo 02.
func newSaveMemoryTool(db *sql.DB) (tool.InvokableTool, error) {
	fn := func(_ context.Context, in SaveInput) (string, error) {
		_, err := db.Exec(
			`INSERT INTO memory (key, value) VALUES (?, ?)
			 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
			in.Key, in.Value,
		)
		if err != nil {
			return "", fmt.Errorf("save: %w", err)
		}
		return fmt.Sprintf("Remembered: %q = %q", in.Key, in.Value), nil
	}
	return utils.InferTool(
		"save_memory",
		"Save a fact to long-term memory — farm location, crops, preferences, or anything the user tells you.",
		fn,
	)
}

func newRecallMemoryTool(db *sql.DB) (tool.InvokableTool, error) {
	fn := func(_ context.Context, in RecallInput) (string, error) {
		var v string
		err := db.QueryRow(`SELECT value FROM memory WHERE key = ?`, in.Key).Scan(&v)
		if err == sql.ErrNoRows {
			return fmt.Sprintf("No memory found for %q.", in.Key), nil
		}
		if err != nil {
			return "", fmt.Errorf("recall: %w", err)
		}
		return fmt.Sprintf("Recalled: %q = %q", in.Key, v), nil
	}
	return utils.InferTool(
		"recall_memory",
		"Retrieve a previously saved fact by its key.",
		fn,
	)
}

// ── Agent construction ────────────────────────────────────────────────────────

// buildScoutAgent creates the Scout — responsible for ALL data gathering:
// live farming data via tools and memory I/O (save + recall).
func buildScoutAgent(ctx context.Context, db *sql.DB) (*adk.Runner, error) {
	cfg := farming.DefaultOllamaConfig()
	model, err := farming.NewChatModel(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("scout model: %w", err)
	}

	farmTools, err := farming.AllTools()
	if err != nil {
		return nil, fmt.Errorf("farming tools: %w", err)
	}
	saveTool, err := newSaveMemoryTool(db)
	if err != nil {
		return nil, err
	}
	recallTool, err := newRecallMemoryTool(db)
	if err != nil {
		return nil, err
	}

	invokable := append(farmTools, saveTool, recallTool)
	baseTools := make([]tool.BaseTool, len(invokable))
	for i, t := range invokable {
		baseTools[i] = t
	}

	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "ScoutAgent",
		Description: "Gathers live farming data and manages persistent memory",
		Instruction: "You are a data scout for Nigerian farmers. For every query:\n" +
			"1. Check memory first — call recall_memory for any personal facts that might help " +
			"(farm_location, grown_crops, farm_size, preferred_currency).\n" +
			"2. Gather live data — call the relevant farming tools.\n" +
			"3. If the user tells you a fact about themselves, ALWAYS call save_memory immediately.\n" +
			"Report all gathered data clearly and concisely. Do not give advice — just facts.",
		Model: model,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{Tools: baseTools},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("scout agent: %w", err)
	}

	return adk.NewRunner(ctx, adk.RunnerConfig{
		Agent:           agent,
		EnableStreaming:  true,
	}), nil
}

// buildPlannerAgent creates the Planner — receives Scout's raw data and
// produces an actionable farming recommendation. No tools needed.
func buildPlannerAgent(ctx context.Context) (*adk.Runner, error) {
	cfg := farming.DefaultOllamaConfig()
	model, err := farming.NewChatModel(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("planner model: %w", err)
	}

	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "PlannerAgent",
		Description: "Creates actionable farming plans from gathered data",
		Instruction: "You are a Nigerian agricultural planner. You receive a user question and " +
			"data gathered by a scout agent. Use that data to produce a clear, actionable " +
			"recommendation for a smallholder farmer.\n\n" +
			"Structure your response:\n" +
			"1. Situation — summarise the key facts from the scout data\n" +
			"2. Recommendation — your main advice in one sentence\n" +
			"3. Action steps — 2–3 concrete things the farmer should do\n\n" +
			"Be practical, specific to Nigerian farming conditions, and brief.",
		Model: model,
	})
	if err != nil {
		return nil, fmt.Errorf("planner agent: %w", err)
	}

	return adk.NewRunner(ctx, adk.RunnerConfig{
		Agent:           agent,
		EnableStreaming:  true,
	}), nil
}

// ── Agent execution ───────────────────────────────────────────────────────────

// runAgent runs one query through a runner, prints all events live, and
// returns the full concatenated text response.
func runAgent(ctx context.Context, runner *adk.Runner, query string) string {
	iter := runner.Query(ctx, query)
	var sb strings.Builder

	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		if event.Err != nil {
			fmt.Printf("  [error: %v]\n", event.Err)
			continue
		}
		if event.Output == nil || event.Output.MessageOutput == nil {
			continue
		}

		mv := event.Output.MessageOutput
		switch mv.Role {
		case schema.Tool:
			msg, _ := mv.GetMessage()
			if msg != nil && msg.Content != "" {
				fmt.Printf("  [tool %s] %s\n", mv.ToolName, truncate(msg.Content, 100))
			}
		case schema.Assistant:
			var text string
			if mv.IsStreaming {
				text = drainStream(mv.MessageStream)
			} else if mv.Message != nil {
				text = mv.Message.Content
				fmt.Print(text)
			}
			sb.WriteString(text)
		}
	}
	return sb.String()
}

func drainStream(stream *schema.StreamReader[*schema.Message]) string {
	if stream == nil {
		return ""
	}
	defer stream.Close()
	var sb strings.Builder
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}
		fmt.Print(chunk.Content)
		sb.WriteString(chunk.Content)
	}
	return sb.String()
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// ── Main ──────────────────────────────────────────────────────────────────────

func main() {
	ctx := context.Background()

	db, err := openMemoryDB("farming_agent.db")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening memory db: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	scout, err := buildScoutAgent(ctx, db)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error building ScoutAgent: %v\n", err)
		os.Exit(1)
	}

	planner, err := buildPlannerAgent(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error building PlannerAgent: %v\n", err)
		os.Exit(1)
	}

	cfg := farming.DefaultOllamaConfig()

	fmt.Println("╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║  Demo 01: AI Farming Agent                                   ║")
	fmt.Println("║  Multi-Agent · Tool Calling · Persistent Memory              ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════╝")
	fmt.Printf("  Model   : %s @ %s\n", cfg.Model, cfg.BaseURL)
	fmt.Printf("  Memory  : farming_agent.db (SQLite — unencrypted on disk)\n")
	fmt.Println()
	fmt.Println("  Agents:")
	fmt.Println("    ScoutAgent  — gathers data using tools + reads/writes memory")
	fmt.Println("    PlannerAgent — synthesises an actionable recommendation")
	fmt.Println()
	fmt.Println("  Tools available to Scout:")
	fmt.Println("    get_crop_price  check_weather  convert_currency  get_farming_tip")
	fmt.Println("    save_memory     recall_memory")
	fmt.Println()
	fmt.Println("  Try these queries:")
	fmt.Println("    Should I plant maize in Lokoja this week?")
	fmt.Println("    What is the price of yam compared to rice?")
	fmt.Println("    Remember my farm is in Lokoja")
	fmt.Println("    What crops should I grow at my farm?")
	fmt.Println("    Remember my contact number is 08012345678   ← PII demo")
	fmt.Println()
	fmt.Println("  Commands: 'memory' = show stored facts | 'quit' = exit")
	fmt.Println(strings.Repeat("─", 64))

	fmt.Println("\n  Current memory store:")
	listMemory(db)

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("\nYou: ")
		if !scanner.Scan() {
			break
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}
		switch strings.ToLower(input) {
		case "quit":
			fmt.Println("Goodbye!")
			return
		case "memory":
			fmt.Println("\n  Stored memory (farming_agent.db):")
			listMemory(db)
			continue
		}

		// ── Step 1: Scout gathers data ──────────────────────────────────────
		fmt.Println("\n── ScoutAgent: gathering data ─────────────────────────────────")
		scoutQuery := input
		if mem := loadMemoryContext(db); mem != "" {
			scoutQuery = "Facts already stored in memory (use these directly — no need to call recall_memory for them):\n" +
				mem + "\nUser query: " + input
		}
		scoutData := runAgent(ctx, scout, scoutQuery)
		fmt.Println()

		if scoutData == "" {
			fmt.Println("  [Scout returned no data]")
			continue
		}

		// ── Step 2: Planner creates recommendation ──────────────────────────
		fmt.Println("── PlannerAgent: building recommendation ──────────────────────")
		plannerInput := fmt.Sprintf(
			"User question: %s\n\nData gathered by scout:\n%s",
			input, scoutData,
		)
		fmt.Print("Plan: ")
		runAgent(ctx, planner, plannerInput)
		fmt.Println()
	}
}
