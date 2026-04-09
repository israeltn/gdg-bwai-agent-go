// Demo 05: Tool Calling with SQLite Memory
//
// A ReAct agent that can use farming tools AND persist facts to a local
// SQLite database. Memory survives between questions (and even between runs).
//
// Tools available to the agent:
//   get_crop_price    — live crop market prices
//   check_weather     — weather + farming forecast
//   convert_currency  — NGN ↔ USD
//   get_farming_tip   — expert crop advice
//   save_memory       — store a key/value fact to SQLite
//   recall_memory     — retrieve a stored fact by key
//
// Example session:
//   You: Remember that my farm is in Lokoja
//   Agent: [calls save_memory(key=farm_location, value=Lokoja)] → Saved!
//   You: What's the weather at my farm?
//   Agent: [calls recall_memory(key=farm_location)] → Lokoja
//          [calls check_weather(city=Lokoja)] → 34°C, partly cloudy…
//
// Run: go run ./demos/05_memory_tools/
// Requires: Ollama running locally + go get modernc.org/sqlite
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

	// Pure-Go SQLite driver — registers the "sqlite" driver with database/sql
	_ "modernc.org/sqlite"
)

// ─── SQLite memory store ──────────────────────────────────────────────────────

// openMemoryDB opens (or creates) a SQLite database file and ensures the
// memory table exists. Returns the db handle for use in tool closures.
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

// ─── Memory tool input structs ────────────────────────────────────────────────

type SaveMemoryInput struct {
	Key   string `json:"key"   jsonschema:"required" jsonschema_description:"A short label for this fact, e.g. farm_location, preferred_crop"`
	Value string `json:"value" jsonschema:"required" jsonschema_description:"The value to remember, e.g. Lokoja, yam"`
}

type RecallMemoryInput struct {
	Key string `json:"key" jsonschema:"required" jsonschema_description:"The label used when saving the fact"`
}

// ─── Memory tool constructors ─────────────────────────────────────────────────

// newSaveMemoryTool returns an eino InvokableTool that stores facts in SQLite.
func newSaveMemoryTool(db *sql.DB) (tool.InvokableTool, error) {
	saveFn := func(_ context.Context, input SaveMemoryInput) (string, error) {
		_, err := db.Exec(
			`INSERT INTO memory (key, value) VALUES (?, ?)
			 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
			input.Key, input.Value,
		)
		if err != nil {
			return "", fmt.Errorf("save memory: %w", err)
		}
		return fmt.Sprintf("Remembered: %q = %q", input.Key, input.Value), nil
	}
	return utils.InferTool(
		"save_memory",
		"Save a fact to long-term memory. Use this to remember things the user tells you, like their farm location or preferred crops.",
		saveFn,
	)
}

// newRecallMemoryTool returns an eino InvokableTool that retrieves facts from SQLite.
func newRecallMemoryTool(db *sql.DB) (tool.InvokableTool, error) {
	recallFn := func(_ context.Context, input RecallMemoryInput) (string, error) {
		var value string
		err := db.QueryRow(`SELECT value FROM memory WHERE key = ?`, input.Key).Scan(&value)
		if err == sql.ErrNoRows {
			return fmt.Sprintf("No memory found for key %q.", input.Key), nil
		}
		if err != nil {
			return "", fmt.Errorf("recall memory: %w", err)
		}
		return fmt.Sprintf("Recalled: %q = %q", input.Key, value), nil
	}
	return utils.InferTool(
		"recall_memory",
		"Retrieve a previously saved fact from long-term memory by its key.",
		recallFn,
	)
}

// listAllMemory is a helper to show the user what's stored (not an agent tool).
func listAllMemory(db *sql.DB) {
	rows, err := db.Query(`SELECT key, value FROM memory ORDER BY key`)
	if err != nil {
		fmt.Printf("  [error reading memory: %v]\n", err)
		return
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var k, v string
		rows.Scan(&k, &v)
		fmt.Printf("  %s = %s\n", k, v)
		count++
	}
	if count == 0 {
		fmt.Println("  (empty)")
	}
}

// ─── Main ─────────────────────────────────────────────────────────────────────

func main() {
	ctx := context.Background()

	// ── Open SQLite database ──────────────────────────────────────────────────
	db, err := openMemoryDB("farming_memory.db")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening memory db: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// ── Create the Ollama model ───────────────────────────────────────────────
	cfg := farming.DefaultOllamaConfig()
	chatModel, err := farming.NewChatModel(ctx, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating model: %v\n", err)
		os.Exit(1)
	}

	// ── Build all tools: farming + memory ─────────────────────────────────────
	farmingTools, err := farming.AllTools()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error building farming tools: %v\n", err)
		os.Exit(1)
	}

	saveTool, err := newSaveMemoryTool(db)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error building save_memory tool: %v\n", err)
		os.Exit(1)
	}
	recallTool, err := newRecallMemoryTool(db)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error building recall_memory tool: %v\n", err)
		os.Exit(1)
	}

	// Combine farming tools and memory tools into one slice
	allInvokable := append(farmingTools, saveTool, recallTool)
	baseTools := make([]tool.BaseTool, len(allInvokable))
	for i, t := range allInvokable {
		baseTools[i] = t
	}

	// ── Create the agent ──────────────────────────────────────────────────────
	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "FarmingMemoryAgent",
		Description: "Nigerian farming assistant with persistent memory",
		Instruction: "You are a Nigerian farming assistant with long-term memory. " +
			"You help smallholder farmers with crop prices, weather, currency, and farming tips. " +
			"Use save_memory to remember important facts the user shares (like their location or crops). " +
			"Use recall_memory to look up things you've stored before answering questions that need them. " +
			"Always check memory first when the user asks about 'my farm', 'my crops', etc.",
		Model: chatModel,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{Tools: baseTools},
		},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating agent: %v\n", err)
		os.Exit(1)
	}

	runner := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent:           agent,
		EnableStreaming: true,
	})

	fmt.Println("╔══════════════════════════════════════════════════════════╗")
	fmt.Println("║  Demo 05: Tool Calling + SQLite Memory  (eino adk)       ║")
	fmt.Println("║  Agent remembers facts across questions using SQLite      ║")
	fmt.Println("╚══════════════════════════════════════════════════════════╝")
	fmt.Printf("Model: %s @ %s\n", cfg.Model, cfg.BaseURL)
	fmt.Println("Memory file: farming_memory.db")
	fmt.Println()
	fmt.Println("Try:")
	fmt.Println("  Remember that my farm is in Lokoja")
	fmt.Println("  What is the price of yam where I farm?")
	fmt.Println("  Save that I grow maize and pepper")
	fmt.Println("  What crops do I grow?")
	fmt.Println()
	fmt.Println("Commands: 'memory' = show all stored facts, 'quit' = exit")
	fmt.Println(strings.Repeat("─", 58))

	// Show what's already in memory from previous runs
	fmt.Println("\nCurrent memory store:")
	listAllMemory(db)

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
			fmt.Println("Stored memory:")
			listAllMemory(db)
			continue
		}

		fmt.Println()
		runAgentQuery(ctx, runner, input)
		fmt.Println()
	}
}

// runAgentQuery runs one query through the agent and prints all events.
func runAgentQuery(ctx context.Context, runner *adk.Runner, query string) {
	iter := runner.Query(ctx, query)

	for {
		event, ok := iter.Next()
		if !ok {
			break
		}

		if event.Err != nil {
			fmt.Printf("[Error: %v]\n", event.Err)
			continue
		}

		if event.Output == nil || event.Output.MessageOutput == nil {
			continue
		}

		mv := event.Output.MessageOutput
		switch mv.Role {
		case schema.Tool:
			// Show tool calls so the user can see memory being used
			msg, _ := mv.GetMessage()
			if msg != nil && msg.Content != "" {
				fmt.Printf("[tool %s]: %s\n", mv.ToolName, truncate(msg.Content, 120))
			}
		case schema.Assistant:
			// Stream or print the assistant's final response
			if mv.IsStreaming {
				fmt.Print("Assistant: ")
				printStream(mv.MessageStream)
				fmt.Println()
			} else if mv.Message != nil && mv.Message.Content != "" {
				fmt.Printf("Assistant: %s\n", mv.Message.Content)
			}
		}
	}
}

// printStream drains a message stream to stdout.
func printStream(stream *schema.StreamReader[*schema.Message]) {
	if stream == nil {
		return
	}
	defer stream.Close()
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Printf("[stream error: %v]", err)
			break
		}
		fmt.Print(chunk.Content)
	}
}

// truncate shortens a string to maxLen characters, adding "…" if cut.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "…"
}
