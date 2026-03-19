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
		// ── Qwen 通用对话 (domestic) ──
		"qwen-max", "qwen-plus", "qwen-turbo", "qwen-flash", "qwen-long",
		// ── Qwen3 / 3.5 ──
		"qwen3.5-plus", "qwen3.5-flash", "qwen3-max",
		// ── QwQ 推理 ──
		"qwq-plus", "qwq-max", "qwq-32b",
		// ── Qwen 视觉 (VL) ──
		"qwen3-vl-plus", "qwen3-vl-flash", "qwen-vl-max", "qwen-vl-plus",
		// ── Qwen 代码 (Coder) ──
		"qwen3-coder-plus", "qwen3-coder-flash", "qwen-coder-plus", "qwen-coder-turbo",
		// ── Qwen 数学 / OCR / 多模态 / 深度研究 ──
		"qwen-math-plus", "qwen-math-turbo",
		"qwen-ocr",
		"qwen3-omni-flash", "qwen-omni-turbo",
		"qwen-deep-research",
		// ── DeepSeek (domestic) ──
		"deepseek-chat", "deepseek-reasoner",
		// ── MiniMax (domestic) ──
		"MiniMax-M2.5", "MiniMax-M2.5-highspeed",
		"MiniMax-M2.1", "MiniMax-M2",
		"MiniMax-Text-01", "MiniMax-VL-01",
		// ── OpenAI GPT-4.1 ──
		"gpt-4.1", "gpt-4.1-mini", "gpt-4.1-nano",
		// ── OpenAI GPT-4o ──
		"gpt-4o", "gpt-4o-mini", "gpt-4o-search-preview", "chatgpt-4o-latest",
		// ── OpenAI GPT-4/3.5 (legacy) ──
		"gpt-4-turbo", "gpt-4", "gpt-3.5-turbo",
		// ── OpenAI Reasoning (o 系列) ──
		"o3", "o3-mini", "o3-pro", "o4-mini",
		"o1", "o1-mini", "o1-pro",
		// ── OpenAI Codex ──
		"codex-mini-latest",
		// ── Anthropic Claude 4 ──
		"claude-opus-4", "claude-sonnet-4",
		// ── Anthropic Claude 3.7 ──
		"claude-3.7-sonnet",
		// ── Anthropic Claude 3.5 ──
		"claude-3.5-sonnet", "claude-3.5-haiku",
		// ── Anthropic Claude 3 ──
		"claude-3-opus", "claude-3-haiku",
		// ── Google Gemini 2.5 ──
		"gemini-2.5-pro", "gemini-2.5-flash", "gemini-2.5-flash-lite",
		// ── Google Gemini 2.0 ──
		"gemini-2.0-flash", "gemini-2.0-flash-lite",
		// ── Google Gemini 1.5 (legacy) ──
		"gemini-1.5-pro", "gemini-1.5-flash",
		// ── Grok 3 ──
		"grok-3", "grok-3-mini", "grok-3-fast",
		// ── Grok 2 ──
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
