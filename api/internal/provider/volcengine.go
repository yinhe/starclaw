package provider

import (
	"context"
)

// VolcEngineProvider wraps OpenAI-compatible Ark API for ByteDance Doubao (豆包)
// Docs: https://www.volcengine.com/docs/82379/1330310
type VolcEngineProvider struct {
	inner *OpenAIProvider
}

type VolcEngineConfig struct {
	APIKey  string
	BaseURL string
}

func NewVolcEngineProvider(cfg VolcEngineConfig) *VolcEngineProvider {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://ark.cn-beijing.volces.com/api/v3"
	}
	inner := NewOpenAIProvider(OpenAIConfig{
		APIKey:  cfg.APIKey,
		BaseURL: baseURL,
	})
	inner.models = []string{
		// ── Doubao-seed 旗舰系列 ──
		"doubao-seed-2-0-pro-260215",
		"doubao-seed-2-0-lite-260215",
		// ── Doubao 1.5 系列 ──
		"doubao-1.5-pro-256k-250115",
		"doubao-1.5-pro-32k-250115",
		"doubao-1.5-lite-32k-250115",
		// ── Doubao Pro 系列 ──
		"doubao-pro-256k-241115",
		"doubao-pro-32k-241115",
		"doubao-pro-4k-241115",
		// ── Doubao Lite 系列 ──
		"doubao-lite-128k-241115",
		"doubao-lite-32k-241115",
		"doubao-lite-4k-241115",
		// ── 深度思考 (Thinking) ──
		"doubao-1.5-thinking-pro-250415",
		"doubao-1.5-thinking-pro-m-250415",
		// ── 视觉理解 (Vision) ──
		"doubao-1.5-vision-pro-250328",
		"doubao-vision-pro-32k-241028",
		"doubao-vision-lite-32k-241028",
		// ── 角色扮演 (Character) ──
		"doubao-character-pro-32k",
		"doubao-character-lite-32k",
		// ── 编程 (Coder) ──
		"doubao-1.5-coder-pro",
		// ── 视频生成 (Video) ──
		"doubao-seedance-1-0-lite-t2v-250428",
		"doubao-seedance-1-0-lite-i2v-250428",
		// ── 图像生成 (Image) ──
		"doubao-seedream-3-0-t2i-250401",
		// ── 向量 (Embedding) ──
		"doubao-embedding-large-text-240915",
		"doubao-embedding-text-240515",
		// ── 语音合成 (TTS) ──
		"doubao-tts",
		// ── 语音识别 (ASR) ──
		"doubao-speech-pro-250301",
		// ── 第三方模型 (via Volcengine) ──
		"deepseek-r1-250120",
		"deepseek-v3-250324",
		"deepseek-r1-distill-qwen-32b-250120",
		"deepseek-r1-distill-qwen-7b-250120",
	}
	return &VolcEngineProvider{inner: inner}
}

func (p *VolcEngineProvider) Name() string {
	return "volcengine"
}

func (p *VolcEngineProvider) Models() []string {
	return p.inner.Models()
}

func (p *VolcEngineProvider) Chat(ctx context.Context, req *ChatRequest) (<-chan *ChatChunk, error) {
	return p.inner.Chat(ctx, req)
}

func (p *VolcEngineProvider) ChatSync(ctx context.Context, req *ChatRequest) (*ChatChunk, error) {
	return p.inner.ChatSync(ctx, req)
}
