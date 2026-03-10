package provider

import (
	"context"
)

// MiniMaxProvider wraps OpenAI-compatible API with MiniMax's endpoint
type MiniMaxProvider struct {
	inner *OpenAIProvider
}

type MiniMaxConfig struct {
	APIKey  string
	BaseURL string
}

func NewMiniMaxProvider(cfg MiniMaxConfig) *MiniMaxProvider {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://api.minimax.chat/v1"
	}
	inner := NewOpenAIProvider(OpenAIConfig{
		APIKey:  cfg.APIKey,
		BaseURL: baseURL,
	})
	inner.models = []string{
		"MiniMax-Text-01",
		"MiniMax-M1",
		"abab7",
		"abab6.5s",
		"abab6.5t",
		"abab6.5g",
	}
	return &MiniMaxProvider{inner: inner}
}

func (p *MiniMaxProvider) Name() string {
	return "minimax"
}

func (p *MiniMaxProvider) Models() []string {
	return p.inner.Models()
}

func (p *MiniMaxProvider) Chat(ctx context.Context, req *ChatRequest) (<-chan *ChatChunk, error) {
	return p.inner.Chat(ctx, req)
}

func (p *MiniMaxProvider) ChatSync(ctx context.Context, req *ChatRequest) (*ChatChunk, error) {
	return p.inner.ChatSync(ctx, req)
}
