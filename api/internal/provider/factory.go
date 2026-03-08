package provider

import (
	"github.com/yinhe/starclaw/internal/model"
)

// CreateFromConfig creates a ModelProvider from a ModelConfig, checking the registry first.
// This is a shared factory used by both the chat handler and the system tool (for agent delegation).
func CreateFromConfig(registry *Registry, cfg model.ModelConfig) ModelProvider {
	// Check registry first
	if registry != nil {
		if p, ok := registry.Get(cfg.Provider); ok {
			return p
		}
	}

	// Dynamically create provider based on DB config
	switch cfg.Provider {
	case "anthropic":
		return NewAnthropicProvider(AnthropicConfig{
			APIKey:  cfg.APIKey,
			BaseURL: cfg.BaseURL,
		})
	case "deepseek":
		return NewDeepSeekProvider(DeepSeekConfig{
			APIKey:  cfg.APIKey,
			BaseURL: cfg.BaseURL,
		})
	case "ollama":
		return NewOllamaProvider(OllamaConfig{
			BaseURL: cfg.BaseURL,
		})
	case "openrouter":
		return NewOpenRouterProvider(OpenRouterConfig{
			APIKey: cfg.APIKey,
		})
	case "qwen":
		return NewQwenProvider(QwenConfig{
			APIKey:  cfg.APIKey,
			BaseURL: cfg.BaseURL,
		})
	case "google", "gemini":
		return NewGeminiProvider(GeminiConfig{
			APIKey:  cfg.APIKey,
			BaseURL: cfg.BaseURL,
		})
	case "fal":
		return NewFalProvider(FalConfig{
			APIKey:  cfg.APIKey,
			BaseURL: cfg.BaseURL,
		})
	case "grok", "xai":
		return NewGrokProvider(GrokConfig{
			APIKey:  cfg.APIKey,
			BaseURL: cfg.BaseURL,
		})
	default:
		// OpenAI-compatible fallback
		return NewOpenAIProvider(OpenAIConfig{
			APIKey:  cfg.APIKey,
			BaseURL: cfg.BaseURL,
		})
	}
}
