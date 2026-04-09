// Demo 03: Supervisor Agent
//
// Uses eino's adk (Agent Development Kit) to build a supervisor pattern:
//   - A Supervisor agent receives the user query
//   - It decides which Sub-Agent to transfer the task to
//   - Sub-agents are specialized: MarketAgent, WeatherAgent, AdviceAgent
//   - The Runner orchestrates execution and streams events back
//
// Key eino types used:
//   adk.NewChatModelAgent  — creates an agent backed by a ChatModel
//   adk.ChatModelAgentConfig — configures name, description, instruction, model, tools
//   adk.NewRunner / runner.Query — runs the agent and returns an event stream
//   adk.AgentEvent — each event has Output (message) or Action (exit/transfer)
//
// Run: go run ./demos/03_supervisor_agent/
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

	// ── Build farming tools ───────────────────────────────────────────────────
	allTools, err := farming.AllTools()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error building tools: %v\n", err)
		os.Exit(1)
	}

	// Convert []tool.InvokableTool to []tool.BaseTool (InvokableTool embeds BaseTool)
	baseTools := make([]tool.BaseTool, len(allTools))
	for i, t := range allTools {
		baseTools[i] = t
	}

	// ── Create specialized sub-agents ─────────────────────────────────────────
	// Each sub-agent has a focused instruction and only the tools it needs.

	// Sub-agent 1: Market prices
	marketAgent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "MarketAgent",
		Description: "Handles questions about Nigerian crop market prices and currency conversion",
		Instruction: "You are a Nigerian crop market expert. Answer questions about crop prices " +
			"in Nigerian markets and help with NGN/USD currency conversions. " +
			"Use the get_crop_price and convert_currency tools when needed.",
		Model: chatModel,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{Tools: baseTools},
		},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating MarketAgent: %v\n", err)
		os.Exit(1)
	}

	// Sub-agent 2: Weather and field conditions
	weatherAgent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "WeatherAgent",
		Description: "Handles questions about weather conditions and their impact on farming",
		Instruction: "You are a Nigerian agricultural meteorologist. Answer questions about " +
			"weather conditions in Nigerian cities and give farming advice based on weather. " +
			"Use the check_weather tool to get current conditions.",
		Model: chatModel,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{Tools: baseTools},
		},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating WeatherAgent: %v\n", err)
		os.Exit(1)
	}

	// Sub-agent 3: General farming advice
	adviceAgent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "AdviceAgent",
		Description: "Handles questions about farming techniques, crop tips, and best practices",
		Instruction: "You are a Nigerian agricultural extension officer. Give practical farming " +
			"tips and advice to smallholder farmers. Use the get_farming_tip tool to provide " +
			"detailed crop-specific advice.",
		Model: chatModel,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{Tools: baseTools},
		},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating AdviceAgent: %v\n", err)
		os.Exit(1)
	}

	// ── Create the Supervisor agent ───────────────────────────────────────────
	// The supervisor sees the sub-agents and can transfer queries to them.
	// eino's adk.SetSubAgents wires the supervisor ↔ sub-agent relationship.
	supervisorAgent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "Supervisor",
		Description: "Main supervisor that routes queries to specialized farming agents",
		Instruction: "You are a supervisor for a Nigerian farming assistant system. " +
			"You have three specialist sub-agents:\n" +
			"- MarketAgent: handles crop prices and currency questions\n" +
			"- WeatherAgent: handles weather and field condition questions\n" +
			"- AdviceAgent: handles farming tips and agricultural best practices\n\n" +
			"For each user query, decide which specialist is best suited and transfer the task to them. " +
			"If a query covers multiple topics, handle them one at a time starting with the most important.",
		Model: chatModel,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating Supervisor: %v\n", err)
		os.Exit(1)
	}

	// Wire sub-agents to the supervisor using SetSubAgents
	supervisorWithSubs, err := adk.SetSubAgents(ctx, supervisorAgent,
		[]adk.Agent{marketAgent, weatherAgent, adviceAgent})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error setting sub-agents: %v\n", err)
		os.Exit(1)
	}

	// ── Create the Runner ─────────────────────────────────────────────────────
	// Runner.Query is the simplest way to run an agent with a string query.
	// EnableStreaming=true means agents stream their responses token by token.
	runner := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent:           supervisorWithSubs,
		EnableStreaming: true,
	})

	fmt.Println("╔══════════════════════════════════════════════════════════╗")
	fmt.Println("║  Demo 03: Supervisor Agent  (eino adk)                   ║")
	fmt.Println("║  Supervisor routes to: MarketAgent | WeatherAgent |       ║")
	fmt.Println("║  AdviceAgent                                              ║")
	fmt.Println("╚══════════════════════════════════════════════════════════╝")
	fmt.Printf("Model: %s @ %s\n\n", cfg.Model, cfg.BaseURL)
	fmt.Println("Try:")
	fmt.Println("  What is the price of tomato in Abuja?")
	fmt.Println("  Is it good weather for harvesting in Kano?")
	fmt.Println("  Give me tips for growing rice")
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
		runQuery(ctx, runner, query)
	}
}

// runQuery runs a single query through the supervisor and prints all events.
// AgentEvents can carry: Output (a message), Action (exit/transfer), or Err.
func runQuery(ctx context.Context, runner *adk.Runner, query string) {
	iter := runner.Query(ctx, query)

	for {
		event, ok := iter.Next()
		if !ok {
			break // iterator exhausted
		}

		if event.Err != nil {
			fmt.Printf("[Error from %s]: %v\n", event.AgentName, event.Err)
			continue
		}

		// Print which agent is speaking
		if event.AgentName != "" {
			fmt.Printf("[%s]: ", event.AgentName)
		}

		// Handle message output
		if event.Output != nil && event.Output.MessageOutput != nil {
			mv := event.Output.MessageOutput
			if mv.IsStreaming {
				// Stream the message chunks as they arrive
				printMessageStream(mv.MessageStream)
			} else if mv.Message != nil {
				fmt.Print(mv.Message.Content)
			}
			fmt.Println()
		}

		// Handle agent actions
		if event.Action != nil {
			if event.Action.Exit {
				fmt.Println("[Agent finished]")
			}
			if event.Action.TransferToAgent != nil {
				fmt.Printf("[Supervisor transferring to: %s]\n", event.Action.TransferToAgent.DestAgentName)
			}
		}
	}
}

// printMessageStream reads and prints a streaming message.
func printMessageStream(stream *schema.StreamReader[*schema.Message]) {
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
