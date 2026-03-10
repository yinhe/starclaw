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
		baseURL = "https://api.minimax.io/v1"
	}
	inner := NewOpenAIProvider(OpenAIConfig{
		APIKey:  cfg.APIKey,
		BaseURL: baseURL,
	})
	inner.models = []string{
		// Text — M2.5 flagship (2026-02)
		"MiniMax-M2.5",
		"MiniMax-M2.5-highspeed",
		// Text — M2.1 multilingual coding (2025-12)
		"MiniMax-M2.1",
		"MiniMax-M2.1-highspeed",
		// Text — M2 series
		"MiniMax-M2",
		"MiniMax-M2-her",
		// Text — earlier
		"MiniMax-Text-01",
		"MiniMax-VL-01",
		// Video — Hailuo 2.3 (2025-10)
		"MiniMax-Hailuo-2.3",
		"MiniMax-Hailuo-2.3-Fast",
		// Speech / TTS
		"MiniMax-Speech-2.8-hd",
		"MiniMax-Speech-2.6",
		// Music
		"MiniMax-Music-2.5+",
		"MiniMax-Music-2.5",
		// Image
		"image-01",
		"image-01-live",
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
