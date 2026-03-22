package handler

import "testing"

func TestParseModelName(t *testing.T) {
	tests := []struct {
		input        string
		wantProvider string
		wantModel    string
	}{
		{"qwen/qwen-max", "qwen", "qwen-max"},
		{"openai/gpt-4o", "openai", "gpt-4o"},
		{"deepseek/deepseek-chat", "deepseek", "deepseek-chat"},
		{"anthropic/claude-sonnet-4", "anthropic", "claude-sonnet-4"},
		{"google/gemini-2.5-pro", "google", "gemini-2.5-pro"},
		{"grok/grok-3", "grok", "grok-3"},
		{"fal/flux-pro", "fal", "flux-pro"},
		// Heuristic fallbacks (no prefix)
		{"gpt-4o", "openai", "gpt-4o"},
		{"claude-3-opus", "anthropic", "claude-3-opus"},
		{"qwen-max", "qwen", "qwen-max"},
		{"deepseek-chat", "deepseek", "deepseek-chat"},
		{"gemini-2.5-flash", "google", "gemini-2.5-flash"},
		{"grok-3", "grok", "grok-3"},
		{"o3-mini", "openai", "o3-mini"},
		{"o4-mini", "openai", "o4-mini"},
		// Unknown → defaults to openai
		{"some-unknown-model", "openai", "some-unknown-model"},
	}

	for _, tt := range tests {
		provider, model := parseModelName(tt.input)
		if provider != tt.wantProvider || model != tt.wantModel {
			t.Errorf("parseModelName(%q) = (%q, %q), want (%q, %q)",
				tt.input, provider, model, tt.wantProvider, tt.wantModel)
		}
	}
}
