# 🌾 Building AI Agents in Go + Gemma

**Build with AI · GDG Lokoja 2026**

A hands-on workshop showing how to build AI agents in Go using the
[eino](https://github.com/cloudwego/eino) framework and Gemma running locally via Ollama.

---

## Prerequisites

**1. Install Go** (1.21+)
https://go.dev/dl/

**2. Install Ollama**
https://ollama.ai

**3. Pull the Gemma model**
```bash
ollama pull gemma3
```

**4. Clone and install dependencies**
```bash
git clone https://github.com/israeltn/gdg-bwai-agent-go
cd bwai-agent-go
go mod download
```

---

## Workshop Demos

Each demo builds on the previous one. Run them in order.

### Demo 01 — Basic Chat

The simplest eino setup: a ChatModel with conversation history.
No agents, no tools — just a model and a multi-turn message loop.

```bash
go run ./demos/01_basic_chat/
```

### Demo 02 — Intent Routing

Detect the user's intent with a fast model call, then route to a
specialized handler. Each focused handler gives more accurate answers
than one model trying to do everything.

```bash
go run ./demos/02_intent_routing/
```

### Demo 03 — Supervisor Agent

A Supervisor agent receives each query and transfers it to the right
Sub-Agent (MarketAgent, WeatherAgent, AdviceAgent). Uses eino's `adk`
package for agent orchestration.

```bash
go run ./demos/03_supervisor_agent/
```

### Demo 04 — Multi-Agent Pipeline

Two agents collaborate in sequence: **Scout** gathers real data using
tools, then **Planner** synthesises a farming recommendation from that
data. Shows how agents can hand off work to each other.

```bash
go run ./demos/04_multi_agent/
```

### Demo 05 — Tool Calling + SQLite Memory

A ReAct agent with four farming tools plus two memory tools (`save_memory`,
`recall_memory`) backed by a local SQLite database. Facts persist between
questions and across restarts.

```bash
go run ./demos/05_memory_tools/
```

---

## Project Structure

```
bwai-agent-go/
├── internal/farming/
│   ├── model.go          eino ChatModel backed by Ollama
│   └── tools.go          4 farming tools (crop price, weather, currency, tips)
└── demos/
    ├── 01_basic_chat/    Multi-turn chat with conversation history
    ├── 02_intent_routing/ Intent classifier → specialized handler
    ├── 03_supervisor_agent/ Supervisor routes to specialist sub-agents
    ├── 04_multi_agent/   Scout + Planner pipeline
    └── 05_memory_tools/  ReAct agent with SQLite memory
```

---

## The Farming Tools

All demos share the same four tools defined in `internal/farming/tools.go`:

| Tool | What it does |
|---|---|
| `get_crop_price` | Market price for yam, maize, rice, tomato, cassava, pepper |
| `check_weather` | Weather + farming forecast for Nigerian cities |
| `convert_currency` | NGN ↔ USD conversion |
| `get_farming_tip` | Expert growing advice per crop |

---

## Adding Your Own Tool

Tools are plain Go functions wrapped with eino's `InferTool` helper.
The JSON schema for the LLM is derived automatically from your input struct.

1. Define an input struct and a function in `internal/farming/tools.go`:

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

2. Add it to `AllTools()` in the same file:

```go
NewSoilTool,
```

The agent automatically includes it in its reasoning — no other changes needed.

---

## Why Go + Gemma for Nigeria?

| | Go + Gemma 2B | Python + GPT-4 |
|---|---|---|
| RAM required | 1.5 GB | 4–8 GB |
| Works offline | ✅ Yes | ❌ No |
| API cost | ₦0 | ₦500–5,000/day |
| Binary size | ~8 MB | 200 MB+ env |
| Startup time | < 1 second | 3–10 seconds |
| Runs on ₦30k laptop | ✅ Yes | ⚠️ Maybe |

---

## License

MIT — built for learning. Share freely, build something great! 🚀

**GDG Lokoja · Build with AI 2026**
