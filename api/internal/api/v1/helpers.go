package v1

import (
	"github.com/yinhe/starclaw/internal/model"
	"github.com/yinhe/starclaw/internal/provider"
	"github.com/yinhe/starclaw/internal/sandbox"
)

func getUploadDir() string {
	return sandbox.UploadsDir()
}

func resolveProvider(cfg model.ModelConfig, registry *provider.Registry) provider.ModelProvider {
	if p, ok := registry.Get(cfg.Provider); ok {
		return p
	}
	switch cfg.Provider {
	case "anthropic":
		return provider.NewAnthropicProvider(provider.AnthropicConfig{APIKey: cfg.APIKey, BaseURL: cfg.BaseURL})
	case "deepseek":
		return provider.NewDeepSeekProvider(provider.DeepSeekConfig{APIKey: cfg.APIKey, BaseURL: cfg.BaseURL})
	case "ollama":
		return provider.NewOllamaProvider(provider.OllamaConfig{BaseURL: cfg.BaseURL})
	case "openrouter":
		return provider.NewOpenRouterProvider(provider.OpenRouterConfig{APIKey: cfg.APIKey})
	default:
		return provider.NewOpenAIProvider(provider.OpenAIConfig{APIKey: cfg.APIKey, BaseURL: cfg.BaseURL})
	}
}
