package handler

import (
	"strings"
	"testing"
)

func TestGenerateAPIKey(t *testing.T) {
	key := generateAPIKey()
	if !strings.HasPrefix(key, "sk-") {
		t.Errorf("key should start with sk-, got %s", key)
	}
	if len(key) != 51 { // "sk-" + 48 hex chars
		t.Errorf("key length should be 51, got %d", len(key))
	}

	// Keys should be unique
	key2 := generateAPIKey()
	if key == key2 {
		t.Error("generated keys should be unique")
	}
}

func TestGetModelPricing(t *testing.T) {
	tests := []struct {
		model    string
		wantIn   float64
		wantOut  float64
	}{
		{"gpt-4o", 150, 600},
		{"gpt-4o-mini", 15, 60},
		{"gpt-4.1", 200, 800},
		{"gpt-4.1-mini", 15, 60},
		{"gpt-4.1-nano", 15, 60},
		{"o3", 200, 800},
		{"o3-mini", 15, 60},
		{"o4-mini", 15, 60},
		{"claude-sonnet-4-20250514", 200, 800},
		{"claude-3-5-haiku-20241022", 50, 200},
		{"deepseek-chat", 10, 20},
		{"deepseek-reasoner", 10, 20},
		{"qwen-plus", 10, 20},
		{"gemini-2.0-flash", 5, 15},
		{"gemini-2.5-pro-preview-06-05", 100, 400},
		{"unknown-model", 100, 400}, // default
	}

	for _, tt := range tests {
		p := getModelPricing(tt.model)
		if p.InputPer1M != tt.wantIn {
			t.Errorf("model %s: input pricing = %.0f, want %.0f", tt.model, p.InputPer1M, tt.wantIn)
		}
		if p.OutputPer1M != tt.wantOut {
			t.Errorf("model %s: output pricing = %.0f, want %.0f", tt.model, p.OutputPer1M, tt.wantOut)
		}
	}
}

func TestResolveProvider(t *testing.T) {
	h := &GatewayHandler{}

	tests := []struct {
		model        string
		wantProvider string
	}{
		{"gpt-4o", "openai"},
		{"gpt-4.1-mini", "openai"},
		{"o3", "openai"},
		{"o4-mini", "openai"},
		{"claude-sonnet-4-20250514", "anthropic"},
		{"claude-3-5-haiku-20241022", "anthropic"},
		{"deepseek-chat", "deepseek"},
		{"deepseek-reasoner", "deepseek"},
		{"qwen-plus", "qwen"},
		{"gemini-2.0-flash", "gemini"},
		{"gemini-2.5-pro-preview-06-05", "gemini"},
	}

	for _, tt := range tests {
		provider, _, _ := h.resolveProvider(tt.model)
		if provider != tt.wantProvider {
			t.Errorf("model %s: provider = %s, want %s", tt.model, provider, tt.wantProvider)
		}
	}
}

func TestResolveProviderUnknown(t *testing.T) {
	h := &GatewayHandler{}
	provider, _, key := h.resolveProvider("totally-unknown-model")
	if provider != "" {
		t.Errorf("unknown model should return empty provider, got %s", provider)
	}
	if key != "" {
		t.Errorf("unknown model should return empty key, got %s", key)
	}
}

func TestResolveProviderFallbackPrefix(t *testing.T) {
	h := &GatewayHandler{}

	// These models aren't in the explicit list but should be resolved by prefix
	tests := []struct {
		model        string
		wantProvider string
	}{
		{"gpt-4-turbo", "openai"},
		{"claude-3-opus-20240229", "anthropic"},
		{"deepseek-coder", "deepseek"},
		{"qwen-turbo-latest", "qwen"},
		{"gemini-1.5-pro", "gemini"},
	}

	for _, tt := range tests {
		provider, _, _ := h.resolveProvider(tt.model)
		if provider != tt.wantProvider {
			t.Errorf("model %s: provider = %s, want %s", tt.model, provider, tt.wantProvider)
		}
	}
}

func TestSupportedModelsNotEmpty(t *testing.T) {
	if len(supportedModels) == 0 {
		t.Error("supportedModels should not be empty")
	}

	// Check all entries have both fields
	for _, m := range supportedModels {
		if m.ID == "" {
			t.Error("model ID should not be empty")
		}
		if m.Provider == "" {
			t.Errorf("model %s: provider should not be empty", m.ID)
		}
	}
}

func TestDefaultUpstreams(t *testing.T) {
	expected := []string{"openai", "anthropic", "deepseek", "qwen", "gemini"}
	for _, provider := range expected {
		url, ok := defaultUpstreams[provider]
		if !ok {
			t.Errorf("missing default upstream for %s", provider)
		}
		if !strings.HasPrefix(url, "https://") {
			t.Errorf("upstream URL for %s should start with https://, got %s", provider, url)
		}
	}
}
