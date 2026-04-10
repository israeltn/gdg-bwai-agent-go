// ============================================================
//  Gemini API Intro — Google Developer Group Lokoja
//  Run:  go run demos/01_gemini_intro/main.go
//
//  Requires: GEMINI_API_KEY in .env at project root
// ============================================================

package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"google.golang.org/genai"
)

func askGemini(question string) string {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Could not load .env file — make sure it exists with GEMINI_API_KEY=...")
	}

	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		log.Fatal("GEMINI_API_KEY is missing from .env")
	}

	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		Backend: genai.BackendGeminiAPI,
		APIKey:  apiKey,
	})
	if err != nil {
		log.Fatal("Failed to create Gemini client:", err)
	}

	result, err := client.Models.GenerateContent(
		ctx,
		"gemini-3-flash-preview",
		genai.Text(question),
		nil,
	)
	if err != nil {
		log.Fatal("Gemini request failed:", err)
	}

	return result.Text()
}

func main() {
	fmt.Println("=== Gemini AI Demo — GDG Lokoja ===")
	fmt.Println()

	question := "In one sentence, why should developers learn Go?"
	fmt.Println("Question:", question)
	fmt.Println()

	answer := askGemini(question)
	fmt.Println("Gemini says:", answer)
}
