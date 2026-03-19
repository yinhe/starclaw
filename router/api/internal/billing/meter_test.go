package billing

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/yinhe/starclaw-router/internal/provider"
)

func setupRegistry(t *testing.T) *provider.Registry {
	dir := filepath.Join("..", "..", "providers")
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Skip("providers directory not found, skipping")
	}

	reg := provider.NewRegistry()
	if err := reg.LoadDir(dir); err != nil {
		t.Fatalf("LoadDir failed: %v", err)
	}
	return reg
}

func TestCalculateCost_ChatModel(t *testing.T) {
	reg := setupRegistry(t)
	meter := NewMeter(nil, reg) // db=nil, we only test calculation

	// OpenAI gpt-4o: input $2.50/1M, output $10.00/1M
	costCents, upstreamCents := meter.CalculateCost("openai/gpt-4o", 1000, 500)

	if upstreamCents <= 0 {
		t.Error("expected positive upstream cost")
	}
	if costCents <= upstreamCents {
		t.Error("expected marked-up cost > upstream cost")
	}

	t.Logf("openai/gpt-4o: 1000+500 tokens → cost=%.4f分 upstream=%.4f分", costCents, upstreamCents)
}

func TestCalculateCost_DomesticModel(t *testing.T) {
	reg := setupRegistry(t)
	meter := NewMeter(nil, reg)

	// Qwen qwen-max: CNY pricing
	costCents, upstreamCents := meter.CalculateCost("qwen/qwen-max", 1000, 500)

	if upstreamCents <= 0 {
		t.Error("expected positive upstream cost for qwen-max")
	}
	if costCents <= upstreamCents {
		t.Error("expected marked-up cost > upstream cost")
	}

	t.Logf("qwen/qwen-max: 1000+500 tokens → cost=%.4f分 upstream=%.4f分", costCents, upstreamCents)
}

func TestCalculateCost_ImageModel(t *testing.T) {
	reg := setupRegistry(t)
	meter := NewMeter(nil, reg)

	// fal/flux-pro: $0.05 per call
	costCents, upstreamCents := meter.CalculateCost("fal/flux-pro", 0, 0)

	if upstreamCents <= 0 {
		t.Error("expected positive upstream cost for fal/flux-pro")
	}
	if costCents <= upstreamCents {
		t.Error("expected marked-up cost > upstream cost")
	}

	t.Logf("fal/flux-pro: per call → cost=%.4f分 upstream=%.4f分", costCents, upstreamCents)
}

func TestCalculateCost_UnknownModel(t *testing.T) {
	reg := setupRegistry(t)
	meter := NewMeter(nil, reg)

	costCents, upstreamCents := meter.CalculateCost("unknown/model", 1000, 500)

	if costCents != 0 || upstreamCents != 0 {
		t.Errorf("expected 0 cost for unknown model, got cost=%.4f upstream=%.4f", costCents, upstreamCents)
	}
}
