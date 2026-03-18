package provider

import (
	"context"
	"net/http"

	"github.com/yinhe/starclaw/internal/node"
)

// StarAIProvider wraps OpenAI-compatible API with star-ai.net gateway
type StarAIProvider struct {
	inner *OpenAIProvider
}

type StarAIConfig struct {
	APIKey   string
	BaseURL  string         // default: https://api.star-ai.net/v1
	Identity *node.Identity // if set, use Ed25519 signature auth instead of API key
}

func NewStarAIProvider(cfg StarAIConfig) *StarAIProvider {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://api.star-ai.net/v1"
	}
	// Auto-correct legacy URL missing api. prefix
	if baseURL == "https://star-ai.net/v1" {
		baseURL = "https://api.star-ai.net/v1"
	}
	inner := NewOpenAIProvider(OpenAIConfig{
		APIKey:  cfg.APIKey,
		BaseURL: baseURL,
	})

	// If identity is provided, use Ed25519 signature auth (Claw signature)
	if cfg.Identity != nil {
		inner.client = &http.Client{
			Transport: &SignedTransport{Identity: cfg.Identity},
		}
	}
	inner.models = []string{
		// ── Qwen (domestic, default) ──
		"qwen-plus", "qwen-max", "qwen-turbo", "qwen-flash", "qwen-long",
		"qwen3.5-plus", "qwen3-max",
		"qwq-plus", "qwq-max", "qwq-32b",
		"qwen3-vl-plus", "qwen3-vl-flash", "qwen-vl-max", "qwen-vl-plus",
		"qwen3-coder-plus", "qwen3-coder-flash", "qwen-coder-plus", "qwen-coder-turbo",
		"qwen-math-plus", "qwen-math-turbo",
		"qwen3-omni-flash", "qwen-omni-turbo",
		"qwen-deep-research",
		// ── DeepSeek (domestic) ──
		"deepseek-chat", "deepseek-reasoner",
		// ── MiniMax (domestic) ──
		"MiniMax-M2.5", "MiniMax-M2.5-highspeed",
		"MiniMax-M2.1", "MiniMax-M2",
		"MiniMax-Text-01", "MiniMax-VL-01",
		// ── OpenAI (overseas, via proxy) ──
		"o3", "o3-mini", "o3-pro", "o4-mini",
		"o1", "o1-mini", "o1-pro",
		"gpt-4.1", "gpt-4.1-mini", "gpt-4.1-nano",
		"gpt-4o", "gpt-4o-mini", "gpt-4o-search-preview",
		"chatgpt-4o-latest", "gpt-4-turbo", "gpt-4", "gpt-3.5-turbo",
		"codex-mini-latest",
		// ── Anthropic (overseas, via proxy) ──
		"claude-sonnet-4-20250514", "claude-3-7-sonnet-20250219",
		"claude-3-5-sonnet-20241022", "claude-3-5-haiku-20241022",
		"claude-3-opus-20240229",
		// ── Gemini (overseas, via proxy) ──
		"gemini-2.5-pro", "gemini-2.5-flash",
		"gemini-2.0-flash", "gemini-2.0-flash-lite",
		"gemini-1.5-pro", "gemini-1.5-flash",
		// ── Grok (overseas, via proxy) ──
		"grok-3", "grok-3-mini", "grok-3-fast",
		"grok-2", "grok-2-mini", "grok-2-vision",
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
