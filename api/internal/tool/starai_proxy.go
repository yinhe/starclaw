package tool

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/yinhe/starclaw/internal/model"
	"github.com/yinhe/starclaw/internal/node"
	"github.com/yinhe/starclaw/internal/provider"
	"gorm.io/gorm"
)

// StarAIBalance holds cached star-ai.net credit balance (from Queen via Synapse)
type StarAIBalance struct {
	Balance    float64   `json:"balance"`     // star energy display value (⚡)
	BalanceRaw int64     `json:"balance_raw"` // internal units (1⚡ = 10000)
	StarStatus string    `json:"star_status"` // active / hibernated
	UserID     string    `json:"user_id"`
	ClawID     string    `json:"claw_id"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type StarAIUsageItem struct {
	Month        string  `json:"month"`
	ResourceType string  `json:"resource_type"`
	Total        int64   `json:"total"`
	TotalCost    float64 `json:"total_cost"`
}

type StarAIUsageSummary struct {
	Period        string                      `json:"period"`
	Usage         map[string]int64            `json:"usage"`
	Cost          map[string]float64          `json:"cost"`
	UsageBySource map[string]map[string]int64 `json:"usage_by_source"`
	History       []StarAIUsageItem           `json:"history"`
}

// StarAI proxy singleton — set once during router init, used by all tools
var (
	staraiMu      sync.RWMutex
	staraiClient  *http.Client   // HTTP client with SignedTransport for StarAI Router
	staraiBaseURL string         // e.g. "https://api.star-ai.net/v1"
	staraiAPIKey  string         // provisioned API key (sk-star-xxx)
	staraiDB      *gorm.DB       // DB for syncing API key to model_configs
	staraiBalance *StarAIBalance // cached balance from star-ai.net
)

// InitStarAIProxy initializes the shared StarAI proxy client.
// Called once during Claw router initialization when Identity is available.
func InitStarAIProxy(identity *node.Identity, baseURL string, db *gorm.DB) {
	staraiMu.Lock()
	defer staraiMu.Unlock()
	staraiBaseURL = baseURL
	staraiDB = db
	staraiClient = &http.Client{
		Transport: &provider.SignedTransport{Identity: identity},
	}

	// Auto-provision API key in background
	go autoProvisionAPIKey()
}

// IsStarAIProvider returns true if the current context has provider=star-ai
func IsStarAIProvider(ctx context.Context) bool {
	if p, ok := ctx.Value(CtxKeyProvider).(string); ok {
		return p == "star-ai" || p == "starai"
	}
	return false
}

// GetStarAIClient returns the shared StarAI HTTP client and base URL.
// Returns nil, "" if StarAI proxy is not initialized.
func GetStarAIClient() (*http.Client, string) {
	staraiMu.RLock()
	defer staraiMu.RUnlock()
	return staraiClient, staraiBaseURL
}

// GetStarAIAPIKey returns the provisioned API key, or "" if not yet provisioned.
func GetStarAIAPIKey() string {
	staraiMu.RLock()
	defer staraiMu.RUnlock()
	return staraiAPIKey
}

// StarAIProxyURL builds a full proxy URL for a sub-provider.
// e.g. StarAIProxyURL("fal", "/fal-ai/veo3") → "https://api.star-ai.net/v1/proxy/fal/fal-ai/veo3"
func StarAIProxyURL(subProvider, path string) string {
	staraiMu.RLock()
	base := staraiBaseURL
	staraiMu.RUnlock()
	return base + "/proxy/" + subProvider + path
}

// RefreshStarAIKey forces re-provisioning of the API key (called on 401).
func RefreshStarAIKey() {
	go autoProvisionAPIKey()
}

// GetStarAIBalance returns the cached star-ai.net balance.
// If cache is stale (>2min) or force=true, refreshes from star-ai.net synchronously.
// Returns nil if star-ai proxy is not initialized or unreachable.
func GetStarAIBalance(force bool) *StarAIBalance {
	staraiMu.RLock()
	cached := staraiBalance
	client := staraiClient
	baseURL := staraiBaseURL
	staraiMu.RUnlock()

	if client == nil || baseURL == "" {
		return nil
	}

	// Return cache if fresh enough
	if !force && cached != nil && time.Since(cached.UpdatedAt) < 2*time.Minute {
		return cached
	}

	// Refresh via /v1/claw/sync which returns real Queen star energy
	syncURL := strings.TrimSuffix(baseURL, "/v1") + "/v1/claw/sync"
	req, err := http.NewRequest("GET", syncURL, nil)
	if err != nil {
		return cached // return stale cache on error
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[StarAI] balance refresh failed: %v", err)
		return cached
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	var result struct {
		StarEnergyDisplay *float64 `json:"star_energy_display"` // real Queen balance in ⚡
		StarEnergy        *int64   `json:"star_energy"`         // internal units
		StarStatus        string   `json:"star_status"`
		Balance           float64  `json:"balance"` // local Synapse balance (fallback)
		UserID            string   `json:"user_id"`
		ClawID            string   `json:"claw_id"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		log.Printf("[StarAI] sync response parse failed: %v", err)
		return cached
	}

	bal := &StarAIBalance{
		UserID:    result.UserID,
		ClawID:    result.ClawID,
		UpdatedAt: time.Now(),
	}
	// Prefer real Queen star energy; fall back to local Synapse balance
	if result.StarEnergyDisplay != nil {
		bal.Balance = *result.StarEnergyDisplay
		bal.StarStatus = result.StarStatus
		if result.StarEnergy != nil {
			bal.BalanceRaw = *result.StarEnergy
		}
		log.Printf("[StarAI] sync balance: %.1f⚡ (status=%s, user=%s)", bal.Balance, bal.StarStatus, bal.UserID)
	} else {
		bal.Balance = result.Balance / 100.0 // cents → yuan as fallback
		log.Printf("[StarAI] sync balance (local fallback): %.2f (user=%s)", bal.Balance, bal.UserID)
	}
	staraiMu.Lock()
	staraiBalance = bal
	staraiMu.Unlock()
	return bal
}

func GetStarAIUsageSummary() (*StarAIUsageSummary, error) {
	staraiMu.RLock()
	client := staraiClient
	baseURL := staraiBaseURL
	staraiMu.RUnlock()

	if client == nil || baseURL == "" {
		return nil, http.ErrServerClosed
	}

	summaryURL := strings.TrimSuffix(baseURL, "/v1") + "/v1/claw/usage-summary"
	req, err := http.NewRequest("GET", summaryURL, nil)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, http.ErrNoLocation
	}

	var summary StarAIUsageSummary
	if err := json.Unmarshal(body, &summary); err != nil {
		return nil, err
	}
	return &summary, nil
}

// autoProvisionAPIKey calls Router /v1/claw/provision to get or verify API key.
func autoProvisionAPIKey() {
	staraiMu.RLock()
	client := staraiClient
	baseURL := staraiBaseURL
	staraiMu.RUnlock()

	if client == nil || baseURL == "" {
		return
	}

	// Retry with backoff
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt*5) * time.Second)
		}

		provURL := strings.TrimSuffix(baseURL, "/v1") + "/v1/claw/provision"
		req, err := http.NewRequest("POST", provURL, strings.NewReader("{}"))
		if err != nil {
			log.Printf("[StarAI] provision request build failed: %v", err)
			continue
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			log.Printf("[StarAI] provision call failed (attempt %d): %v", attempt+1, err)
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		var result struct {
			Status    string  `json:"status"`
			APIKey    string  `json:"api_key"`
			KeyPrefix string  `json:"key_prefix"`
			UserID    string  `json:"user_id"`
			ClawID    string  `json:"claw_id"`
			Balance   float64 `json:"balance"`
			Message   string  `json:"message"`
		}
		if err := json.Unmarshal(body, &result); err != nil {
			log.Printf("[StarAI] provision response parse failed: %s", string(body))
			continue
		}

		switch result.Status {
		case "created":
			// New key — store it in memory and sync to DB
			staraiMu.Lock()
			staraiAPIKey = result.APIKey
			staraiMu.Unlock()
			syncAPIKeyToConfig(result.APIKey, result.KeyPrefix)
			log.Printf("[StarAI] API key provisioned: %s (user=%s, balance=%.2f分)", result.KeyPrefix, result.UserID, result.Balance)
			return

		case "existing":
			// Key already exists on Router side — ensure local DB is synced
			syncAPIKeyToConfig("", result.KeyPrefix)
			log.Printf("[StarAI] API key already active: %s (user=%s, balance=%.2f分)", result.KeyPrefix, result.UserID, result.Balance)
			return

		default:
			log.Printf("[StarAI] provision unexpected: status=%s msg=%s", result.Status, result.Message)
		}
	}
	log.Printf("[StarAI] provision failed after 3 attempts — will use Ed25519 signature auth")
}

// syncAPIKeyToConfig updates all star-ai model_configs with the provisioned API key.
// If fullKey is empty (existing key), only updates configs still using the placeholder "claw-identity".
func syncAPIKeyToConfig(fullKey, keyPrefix string) {
	staraiMu.RLock()
	db := staraiDB
	staraiMu.RUnlock()
	if db == nil {
		return
	}

	if fullKey != "" {
		// New key — update all star-ai configs to use it
		result := db.Model(&model.ModelConfig{}).
			Where("provider = ?", "star-ai").
			Update("api_key", fullKey)
		if result.RowsAffected > 0 {
			log.Printf("[StarAI] synced API key %s to %d provider config(s)", keyPrefix, result.RowsAffected)
		}
	} else {
		// Existing key — only update placeholder configs
		result := db.Model(&model.ModelConfig{}).
			Where("provider = ? AND api_key = ?", "star-ai", "claw-identity").
			Update("api_key", keyPrefix+"...")
		if result.RowsAffected > 0 {
			log.Printf("[StarAI] synced key prefix %s to %d placeholder config(s)", keyPrefix, result.RowsAffected)
		}
	}
}
