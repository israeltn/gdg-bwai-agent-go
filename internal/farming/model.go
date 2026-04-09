package farming

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino-ext/components/model/ollama"
)

// OllamaConfig holds connection settings for the Ollama backend.
// Change these to point at a different Ollama host or model.
type OllamaConfig struct {
	BaseURL string // e.g. "http://localhost:11434"
	Model   string // e.g. "qwen2.5:3b", "llama3.2", "gemma2:2b"
}

// DefaultOllamaConfig returns the standard local Ollama config used by all demos.
func DefaultOllamaConfig() OllamaConfig {
	return OllamaConfig{
		BaseURL: "http://localhost:11434",
		Model:   "gemma3",
	}
}

// NewChatModel creates an eino ChatModel backed by Ollama.
// The returned model implements model.BaseChatModel (Generate + Stream),
// and model.ToolCallingChatModel (WithTools) for agent use.
func NewChatModel(ctx context.Context, cfg OllamaConfig) (model.ToolCallingChatModel, error) {
	m, err := ollama.NewChatModel(ctx, &ollama.ChatModelConfig{
		BaseURL: cfg.BaseURL,
		Model:   cfg.Model,
	})
	if err != nil {
		return nil, fmt.Errorf("create ollama model: %w", err)
	}
	return m, nil
}
