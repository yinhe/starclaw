package provider

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// PriceSyncer periodically fetches latest model pricing from provider APIs
// and updates the in-memory registry. Also writes a cached pricing snapshot
// to disk for transparency and debugging.
type PriceSyncer struct {
	registry    *Registry
	httpC       *http.Client
	interval    time.Duration
	stopCh      chan struct{}
	snapshotDir string // directory to write pricing snapshots
	mu          sync.Mutex
	lastSync    time.Time
	lastErrors  map[string]string // provider → last error
}

func NewPriceSyncer(registry *Registry, snapshotDir string, interval time.Duration) *PriceSyncer {
	if interval < time.Minute {
		interval = 6 * time.Hour // default: sync every 6 hours
	}
	return &PriceSyncer{
		registry:    registry,
		httpC:       &http.Client{Timeout: 30 * time.Second},
		interval:    interval,
		stopCh:      make(chan struct{}),
		snapshotDir: snapshotDir,
		lastErrors:  make(map[string]string),
	}
}

// Start begins the periodic price sync loop
func (s *PriceSyncer) Start() {
	log.Printf("[price-sync] starting periodic sync every %s", s.interval)
	go func() {
		// Initial sync on startup (delayed 30s to let everything initialize)
		time.Sleep(30 * time.Second)
		s.SyncAll()

		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.SyncAll()
			case <-s.stopCh:
				return
			}
		}
	}()
}

func (s *PriceSyncer) Stop() {
	select {
	case <-s.stopCh:
	default:
		close(s.stopCh)
	}
}

// SyncAll fetches pricing from all supported providers
func (s *PriceSyncer) SyncAll() {
	s.mu.Lock()
	defer s.mu.Unlock()

	log.Println("[price-sync] starting price sync for all providers...")
	start := time.Now()

	updated := 0
	errors := 0

	// Qwen / DashScope (China domestic)
	if n, err := s.syncQwen(); err != nil {
		s.lastErrors["qwen"] = err.Error()
		log.Printf("[price-sync] qwen failed: %v", err)
		errors++
	} else {
		delete(s.lastErrors, "qwen")
		updated += n
	}

	// DeepSeek
	if n, err := s.syncDeepSeek(); err != nil {
		s.lastErrors["deepseek"] = err.Error()
		log.Printf("[price-sync] deepseek failed: %v", err)
		errors++
	} else {
		delete(s.lastErrors, "deepseek")
		updated += n
	}

	// OpenAI
	if n, err := s.syncOpenAI(); err != nil {
		s.lastErrors["openai"] = err.Error()
		log.Printf("[price-sync] openai failed: %v", err)
		errors++
	} else {
		delete(s.lastErrors, "openai")
		updated += n
	}

	s.lastSync = time.Now()
	s.writeSnapshot()

	log.Printf("[price-sync] done in %s: %d models updated, %d provider errors",
		time.Since(start).Round(time.Millisecond), updated, errors)
}

// LastSync returns the time of the last successful sync
func (s *PriceSyncer) LastSync() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastSync
}

// LastErrors returns the last sync errors per provider
func (s *PriceSyncer) LastErrors() map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]string, len(s.lastErrors))
	for k, v := range s.lastErrors {
		out[k] = v
	}
	return out
}

// ── Qwen / DashScope ──
// DashScope doesn't have a public pricing API, but we can fetch model list
// and use their published pricing page data.
// For now, we query the /v1/models endpoint to detect new models,
// then apply known pricing from their documentation.
func (s *PriceSyncer) syncQwen() (int, error) {
	apiKey := os.Getenv("DASHSCOPE_API_KEY")
	if apiKey == "" {
		return 0, fmt.Errorf("DASHSCOPE_API_KEY not set")
	}

	req, _ := http.NewRequest("GET", "https://dashscope.aliyuncs.com/compatible-mode/v1/models", nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := s.httpC.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var result struct {
		Data []struct {
			ID      string `json:"id"`
			OwnedBy string `json:"owned_by"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	// Known Qwen pricing (from https://help.aliyun.com/zh/model-studio/pricing)
	// Prices are per 千 tokens (CNY)
	qwenPricing := map[string][2]float64{
		"qwen-max":         {0.0025, 0.01},
		"qwen-plus":        {0.0008, 0.002},
		"qwen-turbo":       {0.0003, 0.0006},
		"qwen-flash":       {0, 0},
		"qwen-long":        {0.0005, 0.002},
		"qwen3-max":        {0.0025, 0.01},
		"qwen3.5-plus":     {0.0008, 0.0048},
		"qwen3.5-flash":    {0, 0},
		"qwq-plus":         {0.0008, 0.002},
		"qwq-max":          {0.0025, 0.01},
		"qwen3-coder-plus": {0.0008, 0.002},
		"qwen3-coder-flash": {0, 0},
		"qwen3-vl-plus":    {0.0008, 0.002},
		"qwen3-vl-flash":   {0, 0},
		"qwen-vl-max":      {0.003, 0.009},
		"qwen-vl-plus":     {0.0008, 0.002},
		"qwen3-omni-flash": {0, 0},
	}

	updated := 0
	s.registry.mu.Lock()
	defer s.registry.mu.Unlock()

	for _, m := range result.Data {
		modelID := m.ID
		fullName := "qwen/" + modelID

		if pricing, ok := qwenPricing[modelID]; ok {
			if entry, exists := s.registry.models[fullName]; exists {
				if entry.Model.InputPriceCNY != pricing[0] || entry.Model.OutputPriceCNY != pricing[1] {
					entry.Model.InputPriceCNY = pricing[0]
					entry.Model.OutputPriceCNY = pricing[1]
					updated++
					log.Printf("[price-sync] updated %s: input=%.4f output=%.4f CNY/千tokens",
						fullName, pricing[0], pricing[1])
				}
			}
		}
	}

	return updated, nil
}

// ── DeepSeek ──
func (s *PriceSyncer) syncDeepSeek() (int, error) {
	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	if apiKey == "" {
		return 0, fmt.Errorf("DEEPSEEK_API_KEY not set")
	}

	req, _ := http.NewRequest("GET", "https://api.deepseek.com/models", nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := s.httpC.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	// DeepSeek published pricing (from https://api-docs.deepseek.com/zh-cn/quick_start/pricing)
	deepseekPricing := map[string][2]float64{
		"deepseek-chat":     {0.001, 0.002},   // 1元/百万输入, 2元/百万输出
		"deepseek-reasoner": {0.004, 0.016},    // 4元/百万输入, 16元/百万输出
	}

	updated := 0
	s.registry.mu.Lock()
	defer s.registry.mu.Unlock()

	for modelID, pricing := range deepseekPricing {
		fullName := "deepseek/" + modelID
		if entry, exists := s.registry.models[fullName]; exists {
			if entry.Model.InputPriceCNY != pricing[0] || entry.Model.OutputPriceCNY != pricing[1] {
				entry.Model.InputPriceCNY = pricing[0]
				entry.Model.OutputPriceCNY = pricing[1]
				updated++
				log.Printf("[price-sync] updated %s: input=%.4f output=%.4f CNY/千tokens",
					fullName, pricing[0], pricing[1])
			}
		}
	}

	return updated, nil
}

// ── OpenAI ──
func (s *PriceSyncer) syncOpenAI() (int, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return 0, fmt.Errorf("OPENAI_API_KEY not set")
	}

	req, _ := http.NewRequest("GET", "https://api.openai.com/v1/models", nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := s.httpC.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	// OpenAI published pricing (USD per 1M tokens, from https://openai.com/pricing)
	openaiPricing := map[string][2]float64{
		"gpt-4.1":              {2.00, 8.00},
		"gpt-4.1-mini":         {0.40, 1.60},
		"gpt-4.1-nano":         {0.10, 0.40},
		"gpt-4o":               {2.50, 10.00},
		"gpt-4o-mini":          {0.15, 0.60},
		"o3":                   {2.00, 8.00},
		"o3-mini":              {1.10, 4.40},
		"o4-mini":              {1.10, 4.40},
		"gpt-4-turbo":          {10.00, 30.00},
	}

	updated := 0
	s.registry.mu.Lock()
	defer s.registry.mu.Unlock()

	for modelID, pricing := range openaiPricing {
		fullName := "openai/" + modelID
		if entry, exists := s.registry.models[fullName]; exists {
			if entry.Model.InputPrice != pricing[0] || entry.Model.OutputPrice != pricing[1] {
				entry.Model.InputPrice = pricing[0]
				entry.Model.OutputPrice = pricing[1]
				updated++
				log.Printf("[price-sync] updated %s: input=$%.2f output=$%.2f /1M tokens",
					fullName, pricing[0], pricing[1])
			}
		}
	}

	return updated, nil
}

// writeSnapshot writes current pricing to a JSON file for transparency
func (s *PriceSyncer) writeSnapshot() {
	if s.snapshotDir == "" {
		return
	}
	os.MkdirAll(s.snapshotDir, 0755)

	type PriceEntry struct {
		Model          string  `json:"model"`
		Provider       string  `json:"provider"`
		Type           string  `json:"type"`
		InputPriceCNY  float64 `json:"input_price_cny,omitempty"`
		OutputPriceCNY float64 `json:"output_price_cny,omitempty"`
		InputPriceUSD  float64 `json:"input_price_usd,omitempty"`
		OutputPriceUSD float64 `json:"output_price_usd,omitempty"`
		PricePerCall   float64 `json:"price_per_call_cny,omitempty"`
	}

	var entries []PriceEntry
	for _, e := range s.registry.ListModels() {
		pe := PriceEntry{
			Model:          e.Model.Name,
			Provider:       e.Slug,
			Type:           e.Model.Type,
			InputPriceCNY:  e.Model.InputPriceCNY,
			OutputPriceCNY: e.Model.OutputPriceCNY,
			InputPriceUSD:  e.Model.InputPrice,
			OutputPriceUSD: e.Model.OutputPrice,
			PricePerCall:   e.Model.PricePerCallCNY,
		}
		entries = append(entries, pe)
	}

	snapshot := struct {
		SyncedAt string       `json:"synced_at"`
		Count    int          `json:"count"`
		Errors   map[string]string `json:"errors,omitempty"`
		Models   []PriceEntry `json:"models"`
	}{
		SyncedAt: time.Now().Format(time.RFC3339),
		Count:    len(entries),
		Errors:   s.lastErrors,
		Models:   entries,
	}

	data, _ := json.MarshalIndent(snapshot, "", "  ")
	path := s.snapshotDir + "/pricing-snapshot.json"
	if err := os.WriteFile(path, data, 0644); err != nil {
		log.Printf("[price-sync] failed to write snapshot: %v", err)
	}

	// Also write a dated version for history
	dated := fmt.Sprintf("%s/pricing-%s.json", s.snapshotDir, time.Now().Format("2006-01-02"))
	os.WriteFile(dated, data, 0644)

	_ = strings.TrimSpace("") // suppress unused import
}
