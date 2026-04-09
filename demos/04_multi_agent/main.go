// Demo 04: Multi-Agent Pipeline
//
// Two specialized agents collaborate in sequence on each query:
//   1. ScoutAgent  — calls tools to gather real data (prices, weather)
//   2. PlannerAgent — receives the gathered data and crafts a farming plan
//
// This is different from the Supervisor pattern (demo 03) where one agent
// routes to another. Here both agents always run, each doing a distinct job:
// Scout = data collection, Planner = synthesis and recommendation.
//
// Flow:
//   User query
//     → ScoutAgent (uses crop price + weather tools) → raw data text
//     → PlannerAgent (no tools, works with the data text) → farming plan
//
// Run: go run ./demos/04_multi_agent/
package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/gdg-lokoja/bwai-agent-go/internal/farming"
)

func main() {
	ctx := context.Background()

	cfg := farming.DefaultOllamaConfig()
	chatModel, err := farming.NewChatModel(ctx, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating model: %v\n", err)
		os.Exit(1)
	}

	// ── Build tools ───────────────────────────────────────────────────────────
	allTools, err := farming.AllTools()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error building tools: %v\n", err)
		os.Exit(1)
	}

	// InvokableTool embeds BaseTool — convert for ToolsNodeConfig
	baseTools := make([]tool.BaseTool, len(allTools))
	for i, t := range allTools {
		baseTools[i] = t
	}

	// ── Agent 1: ScoutAgent ───────────────────────────────────────────────────
	// Responsible for gathering raw data using tools.
	// It calls crop price, weather, and currency tools as needed.
	scoutAgent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "ScoutAgent",
		Description: "Gathers market and weather data for Nigerian farming queries",
		Instruction: "You are a data scout for Nigerian farmers. " +
			"When given a farming query, use your tools to gather ALL relevant data: " +
			"crop prices, weather conditions, and currency info if needed. " +
			"Report the raw data clearly and concisely. Do not give advice — just facts.",
		Model: chatModel,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{Tools: baseTools},
		},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating ScoutAgent: %v\n", err)
		os.Exit(1)
	}

	// ── Agent 2: PlannerAgent ─────────────────────────────────────────────────
	// Receives the raw data from ScoutAgent and creates a practical farming plan.
	// No tools — it works purely from the data provided in its input.
	plannerAgent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "PlannerAgent",
		Description: "Creates actionable farming plans from gathered data",
		Instruction: "You are a Nigerian agricultural planner. " +
			"You will receive a user question and data gathered by a scout. " +
			"Use this data to create a clear, actionable farming recommendation. " +
			"Structure your response: 1) Situation summary, 2) Recommendation, 3) Action steps.",
		Model: chatModel,
		// No tools — PlannerAgent works from the data given to it
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating PlannerAgent: %v\n", err)
		os.Exit(1)
	}

	// Create a runner for each agent (streaming enabled for live output)
	scoutRunner := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent:           scoutAgent,
		EnableStreaming: true,
	})
	plannerRunner := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent:           plannerAgent,
		EnableStreaming: true,
	})

	fmt.Println("╔══════════════════════════════════════════════════════════╗")
	fmt.Println("║  Demo 04: Multi-Agent Pipeline  (eino adk)               ║")
	fmt.Println("║  ScoutAgent gathers data → PlannerAgent creates a plan   ║")
	fmt.Println("╚══════════════════════════════════════════════════════════╝")
	fmt.Printf("Model: %s @ %s\n\n", cfg.Model, cfg.BaseURL)
	fmt.Println("Try:")
	fmt.Println("  Should I grow maize this season in Kano?")
	fmt.Println("  Is now a good time to sell my tomatoes in Lagos?")
	fmt.Println("  What crops should I grow in Lokoja given current prices?")
	fmt.Println("\nType 'quit' to exit.")
	fmt.Println(strings.Repeat("─", 58))

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("\nYou: ")
		if !scanner.Scan() {
			break
		}
		query := strings.TrimSpace(scanner.Text())
		if query == "" {
			continue
		}
		if strings.ToLower(query) == "quit" {
			fmt.Println("Goodbye!")
			break
		}

		fmt.Println()
		runPipeline(ctx, scoutRunner, plannerRunner, query)
	}
}

// runPipeline executes the two-agent pipeline for a single query.
// Step 1: ScoutAgent gathers data (printed live).
// Step 2: PlannerAgent builds a plan from that data (printed live).
func runPipeline(ctx context.Context, scout, planner *adk.Runner, query string) {
	// ── Step 1: Scout collects data ───────────────────────────────────────────
	fmt.Println("── ScoutAgent: gathering data ─────────────────────────────")
	scoutResult := runAgent(ctx, scout, query)
	fmt.Println()

	if scoutResult == "" {
		fmt.Println("[ScoutAgent returned no data]")
		return
	}

	// ── Step 2: Planner creates a farming plan ────────────────────────────────
	// We pass the original question + the scout's findings as the planner's input.
	fmt.Println("── PlannerAgent: creating farming plan ────────────────────")
	plannerInput := fmt.Sprintf(
		"User question: %s\n\nData gathered by scout:\n%s",
		query, scoutResult,
	)
	runAgent(ctx, planner, plannerInput)
	fmt.Println()
}

// runAgent runs a single agent, prints all message output as it arrives,
// and returns the full concatenated text response.
func runAgent(ctx context.Context, runner *adk.Runner, query string) string {
	iter := runner.Query(ctx, query)

	var sb strings.Builder
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
		var text string

		if mv.IsStreaming {
			text = drainStream(mv.MessageStream)
		} else if mv.Message != nil {
			text = mv.Message.Content
		}

		if text != "" {
			fmt.Print(text)
			sb.WriteString(text)
		}
	}

	return sb.String()
}

// drainStream reads all chunks from a message stream, prints them, and
// returns the concatenated content.
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
			fmt.Printf("[stream error: %v]", err)
			break
		}
		fmt.Print(chunk.Content)
		sb.WriteString(chunk.Content)
	}
	return sb.String()
}
