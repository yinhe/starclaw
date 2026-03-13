package provider

import (
	"context"
)

// StarAIProvider wraps OpenAI-compatible API with star-ai.net gateway
type StarAIProvider struct {
	inner *OpenAIProvider
}

type StarAIConfig struct {
	APIKey  string
	BaseURL string // default: https://star-ai.net/v1
}

func NewStarAIProvider(cfg StarAIConfig) *StarAIProvider {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://star-ai.net/v1"
	}
	inner := NewOpenAIProvider(OpenAIConfig{
		APIKey:  cfg.APIKey,
		BaseURL: baseURL,
	})
	inner.models = []string{
		// OpenAI
		"gpt-4o",
		"gpt-4o-mini",
		"gpt-4.1",
		"gpt-4.1-mini",
		"gpt-4.1-nano",
		"o3",
		"o3-mini",
		"o4-mini",
		// Anthropic
		"claude-sonnet-4-20250514",
		"claude-3-5-haiku-20241022",
		// DeepSeek
		"deepseek-chat",
		"deepseek-reasoner",
		// Qwen
		"qwen-plus",
		"qwen-turbo",
		"qwen-max",
		// Gemini
		"gemini-2.0-flash",
		"gemini-2.5-pro-preview-06-05",
	}
	return &StarAIProvider{inner: inner}
}

func (p *StarAIProvider) Name() string {
	return "star-ai"
}

func (p *StarAIProvider) Models() []string {
	return p.inner.Models()
}

func (p *StarAIProvider) Chat(ctx context.Context, req *ChatRequest) (<-chan *ChatChunk, error) {
	return p.inner.Chat(ctx, req)
}

func (p *StarAIProvider) ChatSync(ctx context.Context, req *ChatRequest) (*ChatChunk, error) {
	return p.inner.ChatSync(ctx, req)
}
