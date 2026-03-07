package provider

import (
	"context"
)

// GeminiProvider uses Google's OpenAI-compatible API endpoint
type GeminiProvider struct {
	inner *OpenAIProvider
}

type GeminiConfig struct {
	APIKey  string
	BaseURL string
}

func NewGeminiProvider(cfg GeminiConfig) *GeminiProvider {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://generativelanguage.googleapis.com/v1beta/openai"
	}
	inner := NewOpenAIProvider(OpenAIConfig{
		APIKey:  cfg.APIKey,
		BaseURL: baseURL,
	})
	inner.models = []string{
		// ── Gemini 2.5 ──
		"gemini-2.5-pro",
		"gemini-2.5-flash",

		// ── Gemini 2.0 ──
		"gemini-2.0-flash",
		"gemini-2.0-flash-lite",

		// ── Gemini 1.5 ──
		"gemini-1.5-pro",
		"gemini-1.5-flash",

		// ── Embedding ──
		"text-embedding-004",
	}
	return &GeminiProvider{inner: inner}
}

func (p *GeminiProvider) Name() string {
	return "google"
}

func (p *GeminiProvider) Models() []string {
	return p.inner.Models()
}

func (p *GeminiProvider) Chat(ctx context.Context, req *ChatRequest) (<-chan *ChatChunk, error) {
	return p.inner.Chat(ctx, req)
}

func (p *GeminiProvider) ChatSync(ctx context.Context, req *ChatRequest) (*ChatChunk, error) {
	return p.inner.ChatSync(ctx, req)
}
