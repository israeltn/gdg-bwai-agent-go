// Demo 02: Intent Routing
//
// Shows how to detect the user's intent with a fast model call, then route
// the query to a specialized handler. This pattern avoids giving a single
// model all system prompts at once — each focused handler is more accurate.
//
// Flow:
//   User query
//     → Classifier model call → intent label (price | weather | currency | tip | general)
//     → Route to the matching specialized system prompt
//     → Stream the final response
//
// Run: go run ./demos/02_intent_routing/
package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/gdg-lokoja/bwai-agent-go/internal/farming"
)

// handlerPrompts maps intent labels to focused system prompts.
var handlerPrompts = map[string]string{
	"price": "You are a Nigerian crop market price expert. " +
		"Answer questions about current market prices for crops in Nigerian cities. " +
		"Reference prices (NGN/kg, Lokoja): yam ₦850, maize ₦350, tomato ₦600, " +
		"rice ₦1200, cassava ₦180, pepper ₦1500. " +
		"Note prices vary by city and season.",

	"weather": "You are a Nigerian agricultural meteorologist. " +
		"Answer questions about weather and its impact on farming in Nigeria. " +
		"Typical conditions: Lokoja 34°C partly cloudy, Abuja 31°C clear, " +
		"Lagos 28°C rainy, Kano 40°C dry. Give practical farming advice.",

	"currency": "You are a currency assistant for Nigerian farmers. " +
		"Help convert between NGN and USD. Current rate: ₦1,580 per $1 (April 2026). " +
		"Show the calculation clearly.",

	"tip": "You are an expert Nigerian agricultural extension officer. " +
		"Give practical farming tips for Nigerian smallholder farmers. " +
		"Focus on locally available inputs and Nigerian crop varieties. " +
		"Crops: yam, maize, tomato, cassava, rice, pepper.",

	"general": "You are a helpful Nigerian farming assistant. " +
		"Help smallholder farmers with any agriculture-related questions. " +
		"Be concise and practical.",
}

// classifierPrompt instructs the model to return exactly one intent label.
const classifierPrompt = `You are an intent classifier for a Nigerian farming assistant.
Classify the user's query into exactly ONE of these categories:
- price    (asking about crop market prices)
- weather  (asking about weather or its farming impact)
- currency (asking about currency conversion NGN/USD)
- tip      (asking for farming advice or tips)
- general  (anything else about farming)

Respond with ONLY the single word category label. No explanation, no punctuation.`

func main() {
	ctx := context.Background()

	cfg := farming.DefaultOllamaConfig()
	chatModel, err := farming.NewChatModel(ctx, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating model: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("╔══════════════════════════════════════════════════════════╗")
	fmt.Println("║  Demo 02: Intent Routing  (eino + Ollama)                ║")
	fmt.Println("║  Queries routed to specialized handlers by intent         ║")
	fmt.Println("╚══════════════════════════════════════════════════════════╝")
	fmt.Printf("Model: %s @ %s\n\n", cfg.Model, cfg.BaseURL)
	fmt.Println("Try:")
	fmt.Println("  What is the price of yam in Lokoja?")
	fmt.Println("  What is the weather in Kano for farming?")
	fmt.Println("  Convert 50000 NGN to USD")
	fmt.Println("  Give me tips for growing maize")
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

		// ── Step 1: Classify intent ────────────────────────────────────────
		intent := detectIntent(ctx, chatModel, query)
		fmt.Printf("\n[Routed to: %s handler]\n", intent)

		// ── Step 2: Get the specialized system prompt for this intent ──────
		systemPrompt, ok := handlerPrompts[intent]
		if !ok {
			systemPrompt = handlerPrompts["general"]
		}

		// ── Step 3: Stream the specialized response ────────────────────────
		messages := []*schema.Message{
			schema.SystemMessage(systemPrompt),
			schema.UserMessage(query),
		}

		stream, err := chatModel.Stream(ctx, messages)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}

		fmt.Print("Assistant: ")
		printStream(stream)
		fmt.Println()
	}
}

// detectIntent calls the model with a classification prompt and returns
// a clean intent label. Uses Generate (not Stream) because we want the
// full answer before routing — classification is a fast, single-token call.
func detectIntent(ctx context.Context, chatModel model.BaseChatModel, query string) string {
	messages := []*schema.Message{
		schema.SystemMessage(classifierPrompt),
		schema.UserMessage(query),
	}

	resp, err := chatModel.Generate(ctx, messages)
	if err != nil {
		// On error, fall back to general
		return "general"
	}

	// Clean up the response: lowercase, trim whitespace and punctuation
	label := strings.ToLower(strings.TrimSpace(resp.Content))
	label = strings.Trim(label, ".,!?\"'")

	// Validate the label is one we know
	if _, ok := handlerPrompts[label]; ok {
		return label
	}

	// If the model returned something unexpected, try to find a keyword match
	for _, known := range []string{"price", "weather", "currency", "tip"} {
		if strings.Contains(label, known) {
			return known
		}
	}

	return "general"
}

// printStream reads a streaming response and prints each chunk as it arrives.
func printStream(stream *schema.StreamReader[*schema.Message]) {
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
