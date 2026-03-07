package provider

import (
	"context"
)

// QwenProvider wraps OpenAI-compatible DashScope API for Qwen (通义千问)
type QwenProvider struct {
	inner *OpenAIProvider
}

type QwenConfig struct {
	APIKey  string
	BaseURL string
}

func NewQwenProvider(cfg QwenConfig) *QwenProvider {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://dashscope.aliyuncs.com/compatible-mode/v1"
	}
	inner := NewOpenAIProvider(OpenAIConfig{
		APIKey:  cfg.APIKey,
		BaseURL: baseURL,
	})
	inner.models = []string{
		// ── Qwen3.5 系列 ──
		"qwen3.5-plus",
		"qwen3.5-plus-2026-02-15",
		"qwen3.5-flash",
		"qwen3.5-27b",
		// ── 千问核心系列 (Qwen Core) ──
		"qwen3-max",
		"qwen3-max-2026-01-23",
		"qwen-max",
		"qwen-max-latest",
		"qwen-max-2025-01-25",
		"qwen-plus",
		"qwen-plus-latest",
		"qwen-plus-2025-12-01",
		"qwen-flash",
		"qwen-turbo",
		"qwen-turbo-latest",
		"qwen-long",
		"qwen-long-latest",
		// ── 推理模型 (Reasoning / QwQ) ──
		"qwq-plus",
		"qwq-plus-latest",
		"qwq-max",
		"qwq-max-latest",
		"qwq-32b",
		"qwq-32b-preview",
		// ── 视觉理解 (Vision / VL) ──
		"qwen3-vl-plus",
		"qwen3-vl-flash",
		"qwen3-vl-flash-2026-01-22",
		"qwen-vl-max",
		"qwen-vl-max-latest",
		"qwen-vl-max-2025-08-13",
		"qwen-vl-plus",
		"qwen-vl-plus-latest",
		"qvq-max",
		"qvq-max-latest",
		"qvq-plus",
		"qvq-plus-latest",
		"qvq-72b-preview",
		"qwen-vl-ocr",
		"qwen-vl-ocr-2025-11-20",
		// ── 编程模型 (Coder) ──
		"qwen3-coder-plus",
		"qwen3-coder-plus-2025-09-23",
		"qwen3-coder-flash",
		"qwen3-coder-next",
		"qwen-coder-plus",
		"qwen-coder-plus-latest",
		"qwen-coder-turbo",
		"qwen-coder-turbo-latest",
		// ── 数学模型 (Math) ──
		"qwen-math-plus",
		"qwen-math-plus-latest",
		"qwen-math-plus-0919",
		"qwen-math-turbo",
		"qwen-math-turbo-latest",
		// ── 全模怀(Omni) ──
		"qwen3-omni-flash",
		"qwen3-omni-flash-2025-12-01",
		"qwen3-omni-flash-realtime",
		"qwen3-omni-flash-realtime-2025-12-01",
		"qwen-omni-turbo",
		"qwen-omni-turbo-latest",
		"qwen-omni-turbo-2025-03-26",
		"qwen-omni-turbo-realtime",
		"qwen-omni-turbo-realtime-2025-05-08",
		// ── 翻译模型 (Translation) ──
		"qwen-mt-plus",
		"qwen-mt-flash",
		"qwen-mt-turbo",
		"qwen-mt-lite",
		"qwen3-livetranslate-flash-realtime",
		// ── 深入研究 ──
		"qwen-deep-research",
		// ── 文档理解 ──
		"qwen-doc-turbo",
		// ── 角色扮演 ──
		"qwen-flash-character",
		"qwen-flash-character-2026-02-26",
		"qwen-plus-character",
		// ── 图像生成 (Image Generation) ──
		"qwen-image-max",
		"qwen-image-max-2025-12-30",
		"qwen-image-plus",
		"qwen-image-plus-2026-01-09",
		"qwen-image-edit-max",
		"qwen-image-edit-max-2026-01-16",
		"qwen-image-edit-plus",
		"qwen-image-edit-plus-2025-12-15",
		"qwen-mt-image",
		"z-image-turbo",
		"wan2.6-t2i",
		"wan2.6-image",
		"flux-schnell",
		"flux-dev",
		"flux-merged",
		"stable-diffusion-xl",
		"stable-diffusion-3.5-large",
		"stable-diffusion-3.5-large-turbo",
		// ── 视频生成 (Video Generation) ──
		"wan2.6-t2v-plus",
		"wan2.6-t2v-turbo",
		"wan2.6-i2v-plus",
		"wan2.6-i2v-turbo",
		// ── 语音合成 (TTS) ──
		"qwen3-tts-flash",
		"qwen3-tts-flash-2025-11-27",
		"qwen3-tts-flash-realtime",
		"qwen3-tts-flash-realtime-2025-11-27",
		"qwen3-tts-instruct-flash",
		"qwen3-tts-instruct-flash-2026-01-26",
		"qwen3-tts-instruct-flash-realtime",
		"qwen3-tts-instruct-flash-realtime-2026-01-22",
		"qwen3-tts-vc",
		"qwen3-tts-vc-2026-01-22",
		"qwen3-tts-vc-realtime",
		"qwen3-tts-vc-realtime-2026-01-15",
		"qwen3-tts-vd",
		"qwen3-tts-vd-2026-01-26",
		"qwen3-tts-vd-realtime",
		"qwen3-tts-vd-realtime-2026-01-15",
		"qwen-tts",
		"qwen-tts-2025-05-22",
		"qwen-tts-realtime",
		"qwen-tts-realtime-2025-07-15",
		"qwen-voice-enrollment",
		"qwen-voice-design",
		"cosyvoice-v3.5-plus",
		"voice-enrollment",
		"sambert-zhiyuan-v1",
		// ── 语音识别 (ASR) ──
		"qwen3-asr-flash",
		"qwen3-asr-flash-2025-09-08",
		"qwen3-asr-flash-realtime",
		"qwen3-asr-flash-realtime-2026-02-10",
		"qwen3-asr-flash-filetrans",
		"qwen3-asr-flash-filetrans-2025-11-17",
		"fun-asr",
		"fun-asr-flash-8k-realtime",
		"qwen-audio-turbo",
		"qwen-audio-turbo-latest",
		"qwen-audio-asr",
		"gummy-chat-v1",
		"gummy-realtime-v1",
		"speech-biasing",
		"paraformer-v2",
		"paraformer-v1",
		"paraformer-mtl-v1",
		"paraformer-8k-v2",
		"paraformer-8k-v1",
		"paraformer-realtime-v2",
		"paraformer-realtime-v1",
		"paraformer-realtime-8k-v2",
		"paraformer-realtime-8k-v1",
		// ── 向量模型 (Embedding) ──
		"text-embedding-v4",
		"qwen3-vl-embedding",
		"qwen3-vl-rerank",
		"tongyi-embedding-vision-flash",
		// ── GUI 交互 ──
		"gui-plus",
		// ── 对话分析 ──
		"tongyi-xiaomi-analysis-pro",
		"tongyi-xiaomi-analysis-flash",
		// ── 意图理解 ──
		"tongyi-intent-detect-v3",
		// ── NLU ──
		"opennlu-v1",
		// ── 法律 ──
		"farui-plus",
		// ── 开源版 Qwen3 ──
		"qwen3-235b-a22b",
		"qwen3-32b",
		"qwen3-14b",
		"qwen3-8b",
		"qwen3-4b",
		"qwen3-1.7b",
		"qwen3-0.6b",
		"qwen3-vl-30b-a3b-thinking",
		"qwen3-omni-30b-a3b-captioner",
		"qwen3-coder-480b-a35b-instruct",
		"qwen3-coder-30b-a3b-instruct",
		// ── 开源版 Qwen2.5 ──
		"qwen2.5-72b-instruct",
		"qwen2.5-32b-instruct",
		"qwen2.5-14b-instruct",
		"qwen2.5-7b-instruct",
		"qwen2.5-3b-instruct",
		"qwen2.5-coder-32b-instruct",
		"qwen2.5-coder-14b-instruct",
		"qwen2.5-coder-7b-instruct",
		"qwen2.5-vl-72b-instruct",
		"qwen2.5-vl-32b-instruct",
		"qwen2.5-vl-7b-instruct",
		"qwen2.5-vl-3b-instruct",
		"qwen2.5-omni-7b",
		// ── 开源版 Qwen2 ──
		"qwen2-vl-72b-instruct",
		// ── 开源版 Qwen1.5 ──
		"qwen1.5-110b-chat",
		// ── 第三方模垀(Third-party via DashScope) ──
		"deepseek-v3",
		"deepseek-v3.2",
		"deepseek-r1",
		"deepseek-r1-distill-qwen-32b",
		"deepseek-r1-distill-qwen-14b",
		"deepseek-r1-distill-qwen-7b",
		"siliconflow/deepseek-r1-0528",
		"kimi-k2.5",
		"kimi/kimi-k2.5",
		"glm-5",
		"MiniMax-M2.5",
		"MiniMax/MiniMax-M2.5",
		"abab6.5g-chat",
		"abab6.5t-chat",
		"abab6.5s-chat",
	}
	return &QwenProvider{inner: inner}
}

func (p *QwenProvider) Name() string {
	return "qwen"
}

func (p *QwenProvider) Models() []string {
	return p.inner.Models()
}

func (p *QwenProvider) Chat(ctx context.Context, req *ChatRequest) (<-chan *ChatChunk, error) {
	return p.inner.Chat(ctx, req)
}

func (p *QwenProvider) ChatSync(ctx context.Context, req *ChatRequest) (*ChatChunk, error) {
	return p.inner.ChatSync(ctx, req)
}
