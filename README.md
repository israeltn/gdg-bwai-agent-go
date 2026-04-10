# Building Secure AI Agents in Go

**Build with AI · GDG Lokoja 2026**
Startechone, Lokoja — April 11 · Prince Audu Abubakar University, Anyigba — April 18

A 20-minute hands-on workshop. Build a complete AI agent in Go, then break it and fix it — live.

Stack: [eino](https://github.com/cloudwego/eino) + Gemma via Ollama — no API key, no internet required, runs on a ₦30k laptop.

---

## Prerequisites

**1. Install Go** (1.21+)
https://go.dev/dl/

```bash
go version
```

**2. Install Ollama**
https://ollama.ai

**3. Pull the Gemma model**
```bash
ollama pull gemma4:e2b
```

**4. Clone and install dependencies**
```bash
git clone https://github.com/israeltn/gdg-bwai-agent-go
cd bwai-agent-go
go mod download
```

**5. Verify your setup**
```bash
ollama run gemma4:e2b "Hello"
go run ./demos/00_intro_golang/
```

---

## Workshop — 4 Demos

| Demo | Folder | Title | What you see |
|------|--------|-------|-------------|
| 00 | `00_intro_golang` | Intro to Go | Variables, functions, if statements — your first Go programme |
| 01 | `01_gemini_intro` | Gemini API | Call Google Gemini from Go with a single API key |
| 02 | `02_farming_agent` | AI Farming Agent | Multi-agent · Tool calling · Memory · PII vulnerability |
| 03 | `03_secure_patterns` | Secure AI Agents | OWASP LLM Top 10 — fix the vulnerabilities from Demo 02 |

---

## Demo 00 — Intro to Go

The simplest possible Go programme. No frameworks, no API keys — just Go.

```bash
go run ./demos/00_intro_golang/
```

Covers: `fmt.Println`, variables (`string`, `int`, `float64`), functions, `if/else`.

---

## Demo 01 — Gemini API

Call Google's Gemini model from Go. Requires a free API key from [aistudio.google.com](https://aistudio.google.com).

Add your key to `.env`:
```
GEMINI_API_KEY=your_key_here
```

```bash
go run ./demos/01_gemini_intro/
```

---

## Demo 02 — AI Farming Agent

A complete Nigerian farming assistant built on two collaborating agents:

**ScoutAgent** — gathers all data using tools and reads/writes memory.
**PlannerAgent** — receives Scout's data and produces an actionable recommendation.

```bash
go run ./demos/02_farming_agent/
```

**What to try:**
```
Should I plant maize in Lokoja this week?
What is the price of yam compared to rice in Abuja?
Remember my farm is in Lokoja
What crops should I grow at my farm?
Remember my contact number is 08012345678
memory
```

The last two prompts are the **PII demo** — the agent stores a phone number in the
SQLite database with no validation. The `memory` command shows it sitting there
unencrypted. Demo 03 fixes this.

**Tools available to ScoutAgent:**

| Tool | What it does |
|------|--------------|
| `get_crop_price` | Market price for yam, maize, rice, tomato, cassava, pepper |
| `check_weather` | Weather + farming forecast for Nigerian cities |
| `convert_currency` | NGN ↔ USD conversion |
| `get_farming_tip` | Expert growing advice per crop |
| `save_memory` | Store a fact to SQLite (⚠ accepts PII — fixed in Demo 03) |
| `recall_memory` | Retrieve a stored fact by key |

---

## Demo 03 — Building Secure AI Agents

Picks up every vulnerability from Demo 02 and fixes it — live, in Go code.

```bash
go run ./demos/03_secure_patterns/
```

Run the scenarios in order:

| # | OWASP Risk | Vulnerability in Demo 02 | The Fix | Time |
|---|-----------|--------------------------|---------|------|
| 1 | LLM06 Excessive Agency | ScoutAgent holds all 6 tools | `toolsFor()` — scope tools per agent | ~2 min |
| 2 | LLM01 Prompt Injection | Raw classifier output used as routing key | First-word extraction + allowlist in Go | ~4 min |
| 3 | LLM02 Sensitive Data | `save_memory` accepts any key including PII | Key allowlist + value length cap | ~2 min |

Each scenario shows the insecure output (red) and the secure output (green) side by side, then prints one key takeaway.

---

## Project Structure

```
bwai-agent-go/
├── internal/farming/
│   ├── model.go              eino ChatModel backed by Ollama
│   └── tools.go              farming tools (crop price, weather, currency, tips)
└── demos/
    ├── 00_intro_golang/      Variables, functions, if statements — your first Go programme
    ├── 01_gemini_intro/      Call Gemini from Go with a single API key
    ├── 02_farming_agent/     Multi-agent · tool calling · memory · PII demo
    └── 03_secure_patterns/   OWASP LLM Top 10 — 3 scenarios
```

---

## Adding Your Own Tool

Tools are plain Go functions wrapped with eino's `InferTool` helper.
The JSON schema for the LLM is derived automatically from your input struct.

**Step 1** — define an input struct and function in `internal/farming/tools.go`:

```go
type SoilInput struct {
    State string `json:"state" jsonschema:"required" jsonschema_description:"Nigerian state name"`
}

func soilFn(_ context.Context, input SoilInput) (string, error) {
    return fmt.Sprintf("Soil in %s: loamy, pH 6.2, good for yam and cassava.", input.State), nil
}

func NewSoilTool() (tool.InvokableTool, error) {
    return utils.InferTool("check_soil", "Get soil quality for a Nigerian state.", soilFn)
}
```

**Step 2** — add it to `AllTools()` in the same file:

```go
NewSoilTool,
```

The agent automatically picks it up — no other changes needed.

---

## Why Go + Gemma for Nigeria?

| | Go + Gemma | Python + GPT-4 |
|--|-----------|----------------|
| RAM required | 1.5 GB | 4–8 GB |
| Works offline | Yes | No |
| API cost | ₦0 | ₦500–5,000/day |
| Binary size | ~8 MB | 200 MB+ env |
| Startup time | < 1 second | 3–10 seconds |
| Runs on ₦30k laptop | Yes | Maybe |

---

## License

MIT — built for learning. Share freely, build something great.

**GDG Lokoja · Build with AI 2026**
