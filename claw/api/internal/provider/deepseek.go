package provider

import (
	"context"
)

// DeepSeekProvider uses OpenAI-compatible API format
type DeepSeekProvider struct {
	inner *OpenAIProvider
}

type DeepSeekConfig struct {
	APIKey  string
	BaseURL string
}

func NewDeepSeekProvider(cfg DeepSeekConfig) *DeepSeekProvider {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://api.deepseek.com/v1"
	}
	inner := NewOpenAIProvider(OpenAIConfig{
		APIKey:  cfg.APIKey,
		BaseURL: baseURL,
	})
	inner.models = []string{"deepseek-chat", "deepseek-reasoner"}
	return &DeepSeekProvider{inner: inner}
}

func (p *DeepSeekProvider) Name() string {
	return "deepseek"
}

func (p *DeepSeekProvider) Models() []string {
	return p.inner.Models()
}

func (p *DeepSeekProvider) Chat(ctx context.Context, req *ChatRequest) (<-chan *ChatChunk, error) {
	return p.inner.Chat(ctx, req)
}

func (p *DeepSeekProvider) ChatSync(ctx context.Context, req *ChatRequest) (*ChatChunk, error) {
	return p.inner.ChatSync(ctx, req)
}
