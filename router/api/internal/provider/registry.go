package provider

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// ProviderConfig represents a provider YAML config file
type ProviderConfig struct {
	Name          string        `yaml:"name"`
	Type          string        `yaml:"type"` // llm, compute, hybrid
	Enabled       bool          `yaml:"enabled"`
	Endpoint      string        `yaml:"endpoint"`
	Description   string        `yaml:"description,omitempty"`
	Auth          AuthConfig    `yaml:"auth"`
	Models        []ModelConfig `yaml:"models"`
	Regions       []string      `yaml:"regions,omitempty"`
	ExecutionMode string        `yaml:"execution_mode,omitempty"` // sync, async
}

type AuthConfig struct {
	Type   string `yaml:"type"`    // bearer, x-api-key, custom_header
	KeyEnv string `yaml:"key_env"` // env var name for the API key
}

// ModelConfig represents a single model within a provider
type ModelConfig struct {
	Name            string   `yaml:"name"`
	Type            string   `yaml:"type"` // chat, reasoning, image, video, tts, stt, embedding, image_edit
	ContextLength   int      `yaml:"context_length,omitempty"`
	InputPrice      float64  `yaml:"input_price,omitempty"`      // per 1M tokens (USD)
	OutputPrice     float64  `yaml:"output_price,omitempty"`     // per 1M tokens (USD)
	InputPriceCNY   float64  `yaml:"input_price_cny,omitempty"`  // per 千 tokens (CNY)
	OutputPriceCNY  float64  `yaml:"output_price_cny,omitempty"` // per 千 tokens (CNY)
	PricePerCall    float64  `yaml:"price_per_call,omitempty"`   // per call (USD)
	PricePerCallCNY float64  `yaml:"price_per_call_cny,omitempty"`
	PricePerChar    float64  `yaml:"price_per_char,omitempty"`
	PricePerCharCNY float64  `yaml:"price_per_char_cny,omitempty"`
	PricePerMin     float64  `yaml:"price_per_min,omitempty"`
	Description     string   `yaml:"description,omitempty"`
	Regions         []string `yaml:"regions,omitempty"`
}

// Registry holds all loaded provider configurations
type Registry struct {
	mu        sync.RWMutex
	providers map[string]*ProviderConfig // keyed by provider slug (e.g. "openai", "qwen")
	models    map[string]*ModelEntry     // keyed by full model name (e.g. "openai/gpt-4o")
}

// ModelEntry is a resolved model with its parent provider info
type ModelEntry struct {
	Model    ModelConfig
	Provider *ProviderConfig
	Slug     string // provider slug
}

func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[string]*ProviderConfig),
		models:    make(map[string]*ModelEntry),
	}
}

// LoadDir loads all YAML provider configs from a directory (including subdirs)
func (r *Registry) LoadDir(dir string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	files, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil {
		return fmt.Errorf("glob providers: %w", err)
	}

	// Also load custom/*.yaml
	customFiles, _ := filepath.Glob(filepath.Join(dir, "custom", "*.yaml"))
	files = append(files, customFiles...)

	for _, f := range files {
		if err := r.loadFile(f); err != nil {
			log.Printf("[star-ai] warning: failed to load provider %s: %v", f, err)
			continue
		}
	}

	log.Printf("[star-ai] loaded %d providers, %d models from %s", len(r.providers), len(r.models), dir)
	return nil
}

func (r *Registry) loadFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var cfg ProviderConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}

	if !cfg.Enabled {
		return nil // skip disabled providers
	}

	// Derive slug from filename: "openai.yaml" → "openai"
	slug := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	r.providers[slug] = &cfg

	for i := range cfg.Models {
		m := &cfg.Models[i]
		r.models[m.Name] = &ModelEntry{
			Model:    *m,
			Provider: &cfg,
			Slug:     slug,
		}
	}

	return nil
}

// GetProvider returns a provider config by slug
func (r *Registry) GetProvider(slug string) (*ProviderConfig, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.providers[slug]
	return p, ok
}

// GetModel returns a model entry by full name (e.g. "openai/gpt-4o")
func (r *Registry) GetModel(name string) (*ModelEntry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	m, ok := r.models[name]
	return m, ok
}

// ListProviders returns all loaded provider configs
func (r *Registry) ListProviders() map[string]*ProviderConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string]*ProviderConfig, len(r.providers))
	for k, v := range r.providers {
		out[k] = v
	}
	return out
}

// ListModels returns all loaded model entries
func (r *Registry) ListModels() []*ModelEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*ModelEntry, 0, len(r.models))
	for _, v := range r.models {
		out = append(out, v)
	}
	return out
}

// GetAPIKey returns the API key for a provider from environment variables
func (r *Registry) GetAPIKey(slug string) string {
	r.mu.RLock()
	p, ok := r.providers[slug]
	r.mu.RUnlock()
	if !ok {
		return ""
	}
	return os.Getenv(p.Auth.KeyEnv)
}

// IsDomestic returns true if the provider can be reached directly from China
func IsDomestic(slug string) bool {
	switch slug {
	case "qwen", "deepseek", "minimax":
		return true
	default:
		return false
	}
}
