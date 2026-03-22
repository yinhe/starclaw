package provider

import (
	"log"

	"github.com/yinhe/starclaw/internal/model"
	"github.com/yinhe/starclaw/internal/security"
)

// CreateFromConfig creates a ModelProvider from a ModelConfig, checking the registry first.
// This is a shared factory used by both the chat handler and the system tool (for agent delegation).
func CreateFromConfig(registry *Registry, cfg model.ModelConfig) ModelProvider {
	// Decrypt API key if stored encrypted (transparent: handles both enc: and plaintext)
	cfg.APIKey = security.DecryptAPIKey(cfg.APIKey)

	// Check registry first, but only if no user-specific API key override.
	// This allows user-configured API keys (e.g. star-ai sk-star-xxx) to
	// take priority over pre-registered providers (e.g. Ed25519 identity auth).
	if registry != nil && (cfg.APIKey == "" || cfg.APIKey == "claw-identity") {
		if p, ok := registry.Get(cfg.Provider); ok {
			log.Printf("[Provider] Using registry provider for %s (api_key=%q)", cfg.Provider, cfg.APIKey)
			return p
		}
		log.Printf("[Provider] Registry miss for %s (registered: %v)", cfg.Provider, registry.List())
	} else {
		log.Printf("[Provider] Skipping registry: registry=%v, provider=%s, api_key_len=%d", registry != nil, cfg.Provider, len(cfg.APIKey))
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
	case "grok", "xai":
		return NewGrokProvider(GrokConfig{
			APIKey:  cfg.APIKey,
			BaseURL: cfg.BaseURL,
		})
	case "minimax":
		return NewMiniMaxProvider(MiniMaxConfig{
			APIKey:  cfg.APIKey,
			BaseURL: cfg.BaseURL,
		})
	case "star-ai", "starai":
		starCfg := StarAIConfig{
			APIKey:  cfg.APIKey,
			BaseURL: cfg.BaseURL,
		}
		// Always try to inject Identity for Ed25519 signature auth.
		// SignedTransport sends both signature headers AND keeps the API key,
		// so the remote DualAuth will prefer the Claw signature path and check
		// star energy instead of local balance.
		if registry != nil {
			if rp, ok := registry.Get("star-ai"); ok {
				if sp, ok := rp.(*StarAIProvider); ok && sp.inner.client.Transport != nil {
					if st, ok := sp.inner.client.Transport.(*SignedTransport); ok {
						starCfg.Identity = st.Identity
						log.Printf("[Provider] Injected Identity for star-ai (api_key_len=%d)", len(cfg.APIKey))
					}
				}
			}
		}
		return NewStarAIProvider(starCfg)
	default:
		// OpenAI-compatible fallback
		return NewOpenAIProvider(OpenAIConfig{
			APIKey:  cfg.APIKey,
			BaseURL: cfg.BaseURL,
		})
	}
}
