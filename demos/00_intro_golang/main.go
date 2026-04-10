// ============================================================
//  Intro to Go — Google Developer Group Lokoja
//  Run:  go run demos/00_intro_golang/main.go
// ============================================================

package main

import "fmt"

// A function takes inputs and returns an output.
func add(a int, b int) int {
	return a + b
}

func greet(name string) string {
	return "Hello, " + name + "!"
}

func main() {

	// ── 1. Print ─────────────────────────────────────────────
	fmt.Println("=== GDG Lokoja — Intro to Go ===")
	fmt.Println()

	// ── 2. Variables ─────────────────────────────────────────
	var city string = "Lokoja"     // string  — text
	var year int = 2026            // int     — whole number
	var temperature float64 = 38.5 // float64 — decimal number

	fmt.Println("City       :", city)
	fmt.Println("Year       :", year)
	fmt.Printf("Temperature: %.1f°C\n", temperature)
	fmt.Println()

	// Short declaration — Go figures out the type for you
	language := "Go"
	fmt.Println("Language   :", language)
	fmt.Println()

	// ── 3. Functions ─────────────────────────────────────────
	sum := add(10, 32)
	fmt.Println("10 + 32 =", sum)

	message := greet("GDG Developer")
	fmt.Println(message)
	fmt.Println()

	// ── 4. if Statement ──────────────────────────────────────
	score := 75

	if score >= 50 {
		fmt.Println("Score", score, "→ You passed!")
	} else {
		fmt.Println("Score", score, "→ Try again.")
	}
}
