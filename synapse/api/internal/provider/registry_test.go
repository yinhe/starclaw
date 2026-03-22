package provider

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRegistryLoadDir(t *testing.T) {
	// Find providers dir relative to test
	dir := filepath.Join("..", "..", "providers")
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Skip("providers directory not found, skipping")
	}

	reg := NewRegistry()
	if err := reg.LoadDir(dir); err != nil {
		t.Fatalf("LoadDir failed: %v", err)
	}

	// Should have loaded at least qwen, openai, deepseek, anthropic, google, fal
	expectedProviders := []string{"qwen", "openai", "deepseek", "anthropic", "google", "fal"}
	for _, slug := range expectedProviders {
		if _, ok := reg.GetProvider(slug); !ok {
			t.Errorf("expected provider %q to be loaded", slug)
		}
	}

	// Check specific model
	entry, ok := reg.GetModel("openai/gpt-4o")
	if !ok {
		t.Fatal("expected model openai/gpt-4o to be loaded")
	}
	if entry.Slug != "openai" {
		t.Errorf("expected slug 'openai', got %q", entry.Slug)
	}
	if entry.Model.Type != "chat" {
		t.Errorf("expected type 'chat', got %q", entry.Model.Type)
	}
	if entry.Model.InputPrice <= 0 {
		t.Error("expected positive input price for gpt-4o")
	}

	// Check domestic model
	qwenEntry, ok := reg.GetModel("qwen/qwen-max")
	if !ok {
		t.Fatal("expected model qwen/qwen-max to be loaded")
	}
	if qwenEntry.Model.InputPriceCNY <= 0 {
		t.Error("expected positive CNY input price for qwen-max")
	}

	// ListModels should return all models
	allModels := reg.ListModels()
	if len(allModels) < 10 {
		t.Errorf("expected at least 10 models, got %d", len(allModels))
	}
}

func TestIsDomestic(t *testing.T) {
	tests := []struct {
		slug     string
		expected bool
	}{
		{"qwen", true},
		{"deepseek", true},
		{"openai", false},
		{"anthropic", false},
		{"google", false},
		{"grok", false},
		{"fal", false},
	}

	for _, tt := range tests {
		if got := IsDomestic(tt.slug); got != tt.expected {
			t.Errorf("IsDomestic(%q) = %v, want %v", tt.slug, got, tt.expected)
		}
	}
}

func TestGetAPIKey(t *testing.T) {
	dir := filepath.Join("..", "..", "providers")
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Skip("providers directory not found, skipping")
	}

	reg := NewRegistry()
	reg.LoadDir(dir)

	// Set a test env var
	os.Setenv("DASHSCOPE_API_KEY", "test-key-123")
	defer os.Unsetenv("DASHSCOPE_API_KEY")

	key := reg.GetAPIKey("qwen")
	if key != "test-key-123" {
		t.Errorf("expected 'test-key-123', got %q", key)
	}

	// Non-existent provider
	key = reg.GetAPIKey("nonexistent")
	if key != "" {
		t.Errorf("expected empty string for nonexistent provider, got %q", key)
	}
}
