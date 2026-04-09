// Demo 01: Basic Chat
//
// The simplest possible use of eino: direct ChatModel usage with multi-turn
// conversation history. No agents, no tools — just a model and a message loop.
//
// Run: go run ./demos/01_basic_chat/
// Requires: Ollama running locally with qwen2.5:3b (or change DefaultOllamaConfig)
package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/cloudwego/eino/schema"
	"github.com/gdg-lokoja/bwai-agent-go/internal/farming"
)

func main() {
	ctx := context.Background()

	// ── Create the model ──────────────────────────────────────────────────────
	cfg := farming.DefaultOllamaConfig()
	model, err := farming.NewChatModel(ctx, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating model: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("╔══════════════════════════════════════════════════════════╗")
	fmt.Println("║  Demo 01: Basic Chat  (eino + Ollama)                    ║")
	fmt.Println("║  Nigerian Farming Assistant — multi-turn conversation     ║")
	fmt.Println("╚══════════════════════════════════════════════════════════╝")
	fmt.Printf("Model: %s @ %s\n\n", cfg.Model, cfg.BaseURL)
	fmt.Println("Type your message and press Enter. Type 'quit' to exit.")
	fmt.Println(strings.Repeat("─", 58))

	// ── Conversation history ──────────────────────────────────────────────────
	// eino represents conversation history as a slice of *schema.Message.
	// We prepend a system message and keep appending user + assistant turns.
	history := []*schema.Message{
		schema.SystemMessage(
			"You are a helpful Nigerian farming assistant. " +
				"You help smallholder farmers in Nigeria with crop advice, market prices, " +
				"weather impacts, and farming best practices. " +
				"Keep answers concise and practical. Use naira (₦) for prices.",
		),
	}

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("\nYou: ")
		if !scanner.Scan() {
			break
		}
		userInput := strings.TrimSpace(scanner.Text())
		if userInput == "" {
			continue
		}
		if strings.ToLower(userInput) == "quit" {
			fmt.Println("Goodbye!")
			break
		}

		// Add user message to history
		history = append(history, schema.UserMessage(userInput))

		// ── Call the model ─────────────────────────────────────────────────
		// model.Stream returns a StreamReader that yields *schema.Message chunks.
		// We print each chunk as it arrives for a responsive feel.
		stream, err := model.Stream(ctx, history)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			// Remove the failed user message so history stays clean
			history = history[:len(history)-1]
			continue
		}

		fmt.Print("\nAssistant: ")
		fullContent := collectStream(stream)
		fmt.Println()

		// Add the complete assistant response to history for the next turn
		history = append(history, schema.AssistantMessage(fullContent, nil))

		fmt.Printf("\n[History: %d messages]\n", len(history))
	}
}

// collectStream reads all chunks from a streaming response, prints each chunk
// as it arrives, and returns the concatenated full content string.
func collectStream(stream *schema.StreamReader[*schema.Message]) string {
	defer stream.Close()

	var sb strings.Builder
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Printf("\n[stream error: %v]\n", err)
			break
		}
		fmt.Print(chunk.Content)
		sb.WriteString(chunk.Content)
	}
	return sb.String()
}
