package provider

import (
	"context"
)

// GrokProvider wraps OpenAI-compatible API with xAI's Grok endpoint
type GrokProvider struct {
	inner *OpenAIProvider
}

type GrokConfig struct {
	APIKey  string
	BaseURL string
}

func NewGrokProvider(cfg GrokConfig) *GrokProvider {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://api.x.ai/v1"
	}
	inner := NewOpenAIProvider(OpenAIConfig{
		APIKey:  cfg.APIKey,
		BaseURL: baseURL,
	})
	inner.models = []string{
		"grok-3",
		"grok-3-mini",
		"grok-3-fast",
		"grok-2",
		"grok-2-mini",
		"grok-2-vision",
		"grok-beta",
	}
	return &GrokProvider{inner: inner}
}

func (p *GrokProvider) Name() string {
	return "grok"
}

func (p *GrokProvider) Models() []string {
	return p.inner.Models()
}

func (p *GrokProvider) Chat(ctx context.Context, req *ChatRequest) (<-chan *ChatChunk, error) {
	return p.inner.Chat(ctx, req)
}

func (p *GrokProvider) ChatSync(ctx context.Context, req *ChatRequest) (*ChatChunk, error) {
	return p.inner.ChatSync(ctx, req)
}
