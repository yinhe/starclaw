package provider

import (
	"context"
)

// OpenRouterProvider wraps OpenAI-compatible API with OpenRouter's endpoint
type OpenRouterProvider struct {
	inner *OpenAIProvider
}

type OpenRouterConfig struct {
	APIKey string
}

func NewOpenRouterProvider(cfg OpenRouterConfig) *OpenRouterProvider {
	inner := NewOpenAIProvider(OpenAIConfig{
		APIKey:  cfg.APIKey,
		BaseURL: "https://openrouter.ai/api/v1",
	})
	inner.models = []string{
		"openai/gpt-4o",
		"anthropic/claude-sonnet-4-20250514",
		"google/gemini-2.0-flash",
		"deepseek/deepseek-chat",
		"meta-llama/llama-3.1-405b-instruct",
	}
	return &OpenRouterProvider{inner: inner}
}

func (p *OpenRouterProvider) Name() string {
	return "openrouter"
}

func (p *OpenRouterProvider) Models() []string {
	return p.inner.Models()
}

func (p *OpenRouterProvider) Chat(ctx context.Context, req *ChatRequest) (<-chan *ChatChunk, error) {
	return p.inner.Chat(ctx, req)
}

func (p *OpenRouterProvider) ChatSync(ctx context.Context, req *ChatRequest) (*ChatChunk, error) {
	return p.inner.ChatSync(ctx, req)
}
