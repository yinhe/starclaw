package provider

import (
	"context"
)

// FalProvider wraps OpenAI-compatible API with fal.ai's endpoint and Key auth.
// fal.ai is a fast inference platform supporting LLM, image, video, audio models.
// LLM models use the any-llm OpenAI-compatible endpoint.
// Image/Video/Audio models use fal's native REST API (https://fal.run/fal-ai/{model}).
type FalProvider struct {
	inner *OpenAIProvider
}

type FalConfig struct {
	APIKey  string
	BaseURL string
}

func NewFalProvider(cfg FalConfig) *FalProvider {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://fal.run/fal-ai/any-llm/v1"
	}
	inner := NewOpenAIProvider(OpenAIConfig{
		APIKey:     cfg.APIKey,
		BaseURL:    baseURL,
		AuthPrefix: "Key",
	})
	inner.models = []string{
		// ── LLM (via any-llm, OpenAI-compatible) ──
		"meta-llama/llama-3.3-70b-instruct",
		"meta-llama/llama-3.1-405b-instruct",
		"meta-llama/llama-3.1-70b-instruct",
		"meta-llama/llama-3.1-8b-instruct",
		"google/gemma-2-27b-it",
		"google/gemma-2-9b-it",
		"mistralai/mistral-large-latest",
		"mistralai/mistral-small-latest",
		"mistralai/mixtral-8x22b-instruct-v0.1",
		"deepseek-ai/DeepSeek-V3",
		"deepseek-ai/DeepSeek-R1",
		"Qwen/Qwen2.5-72B-Instruct",
		"Qwen/Qwen2.5-Coder-32B-Instruct",
		"WizardLMTeam/WizardLM-2-8x22B",
		"nvidia/Llama-3.1-Nemotron-70B-Instruct",
		"01-ai/Yi-1.5-34B-Chat",

		// ── Image Generation ──
		"fal-ai/flux-pro",
		"fal-ai/flux-pro/v1.1",
		"fal-ai/flux-pro/v1.1-ultra",
		"fal-ai/flux-dev",
		"fal-ai/flux-schnell",
		"fal-ai/flux-realism",
		"fal-ai/flux-2-flex",
		"fal-ai/flux-pro/kontext",
		"fal-ai/flux-pro/kontext/max",
		"fal-ai/nano-banana",
		"fal-ai/nano-banana-2",
		"fal-ai/nano-banana-pro",
		"fal-ai/stable-diffusion-v35-large",
		"fal-ai/stable-diffusion-v35-large-turbo",
		"fal-ai/stable-diffusion-v35-medium",
		"fal-ai/stable-cascade",
		"fal-ai/recraft-v3",
		"fal-ai/recraft-v3/svg",
		"fal-ai/ideogram/v3",
		"fal-ai/ideogram/v2/turbo",
		"fal-ai/bytedance/seedream/v4/text-to-image",
		"fal-ai/qwen-image",
		"fal-ai/omnigen-v1",
		"fal-ai/aura-flow",
		"fal-ai/kolors",
		"fal-ai/playground-v25",

		// ── Image Editing & Enhancement ──
		"fal-ai/flux-pro/kontext/edit",
		"fal-ai/nano-banana-pro/edit",
		"fal-ai/flux-pro/fill",
		"fal-ai/flux-dev/inpainting",
		"fal-ai/flux-dev/canny",
		"fal-ai/flux-dev/depth",
		"fal-ai/flux-lora",
		"fal-ai/flux-dev/image-to-image",
		"fal-ai/aura-sr",
		"fal-ai/creative-upscaler",
		"fal-ai/clarity-upscaler",
		"fal-ai/ic-light",
		"fal-ai/bria/eraser",
		"fal-ai/birefnet",
		"fal-ai/imageutils/rembg",
		"fal-ai/face-swap",

		// ── Video Generation ──
		"fal-ai/wan-25-preview/text-to-video",
		"fal-ai/wan-25-preview/image-to-video",
		"fal-ai/wan/v2.1/text-to-video",
		"fal-ai/wan/v2.1/image-to-video",
		"fal-ai/kling-video/o3/standard/text-to-video",
		"fal-ai/kling-video/o3/standard/image-to-video",
		"fal-ai/kling-video/o3/pro/text-to-video",
		"fal-ai/kling-video/o3/pro/image-to-video",
		"fal-ai/kling-video/v2.5-turbo/pro/text-to-video",
		"fal-ai/kling-video/v2.5-turbo/pro/image-to-video",
		"fal-ai/kling-video/v2/master/text-to-video",
		"fal-ai/kling-video/v2/master/image-to-video",
		"fal-ai/veo3/text-to-video",
		"fal-ai/veo3/image-to-video",
		"fal-ai/veo3.1/first-last-frame-to-video",
		"fal-ai/sora/v2/text-to-video",
		"fal-ai/sora/v2/image-to-video",
		"fal-ai/ovi/text-to-video",
		"fal-ai/ovi/image-to-video",
		"fal-ai/minimax-video/image-to-video",
		"fal-ai/minimax/video-01-live/image-to-video",
		"fal-ai/luma-dream-machine",
		"fal-ai/luma-dream-machine/image-to-video",
		"fal-ai/runway-gen3/turbo/image-to-video",
		"fal-ai/ltx-video",
		"fal-ai/ltx-video/v2",
		"fal-ai/ltx-video/v2-fp8",
		"fal-ai/hunyuan-video",
		"fal-ai/pixverse/v4/text-to-video",
		"fal-ai/pixverse/v4/image-to-video",
		"fal-ai/cogvideox-5b",
		"fal-ai/animatediff-v2v",

		// ── Music Generation ──
		"fal-ai/ace-step",
		"fal-ai/ace-step/prompt-to-audio",
		"fal-ai/minimax-music/v2",
		"fal-ai/minimax-music/v1.5",
		"fal-ai/minimax-music",
		"fal-ai/diffrhythm",
		"fal-ai/yue",
		"fal-ai/sonauto/v2/text-to-music",
		"fal-ai/sonauto/v2/inpaint",
		"fal-ai/stable-audio",
		"fal-ai/stable-audio-25/text-to-audio",
		"fal-ai/lyria2",
		"fal-ai/elevenlabs/music",
		"fal-ai/cassetteai/music-generator",
		"fal-ai/beatoven/music-generation",

		// ── TTS / Speech ──
		"fal-ai/elevenlabs/tts/eleven-v3",
		"fal-ai/elevenlabs/tts/multilingual-v2",
		"fal-ai/elevenlabs/text-to-dialogue/eleven-v3",
		"fal-ai/playai/tts/v3",
		"fal-ai/kokoro/american-english",
		"fal-ai/kokoro/mandarin-chinese",
		"fal-ai/kokoro/japanese",
		"fal-ai/kokoro/british-english",
		"fal-ai/kokoro/french",
		"fal-ai/kokoro/spanish",
		"fal-ai/kokoro/italian",
		"fal-ai/kokoro/hindi",
		"fal-ai/kokoro/brazilian-portuguese",
		"fal-ai/f5-tts",
		"fal-ai/maskgct",
		"fal-ai/csm-1b",
		"fal-ai/zonos",

		// ── ASR / Audio Utility ──
		"fal-ai/whisper",
		"fal-ai/wizper",
		"fal-ai/mmaudio-v2/text-to-audio",
		"fal-ai/elevenlabs/sound-effects/v2",
		"fal-ai/cassetteai/sound-effects-generator",
		"fal-ai/beatoven/sound-effect-generation",

		// ── Avatar & Lip Sync ──
		"fal-ai/sadtalker",
		"fal-ai/live-portrait",
		"fal-ai/latent-sync/lip-sync",
		"fal-ai/sync-lipsync",

		// ── LoRA Training ──
		"fal-ai/flux-lora-fast-training",
		"fal-ai/flux-lora-portrait-trainer",

		// ── Vision & Utility ──
		"fal-ai/florence-2-large",
		"fal-ai/sam2",
		"fal-ai/moondream/batched",
		"fal-ai/imageutils/depth",
		"fal-ai/imageutils/nsfw",
	}
	return &FalProvider{inner: inner}
}

func (p *FalProvider) Name() string {
	return "fal"
}

func (p *FalProvider) Models() []string {
	return p.inner.Models()
}

func (p *FalProvider) Chat(ctx context.Context, req *ChatRequest) (<-chan *ChatChunk, error) {
	return p.inner.Chat(ctx, req)
}

func (p *FalProvider) ChatSync(ctx context.Context, req *ChatRequest) (*ChatChunk, error) {
	return p.inner.ChatSync(ctx, req)
}
