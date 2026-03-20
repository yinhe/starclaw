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

	"github.com/yinhe/starclaw/internal/node"
	"github.com/yinhe/starclaw/internal/provider"
)

// StarAI proxy singleton — set once during router init, used by all tools
var (
	staraiMu       sync.RWMutex
	staraiClient   *http.Client // HTTP client with SignedTransport for StarAI Router
	staraiBaseURL  string       // e.g. "https://api.star-ai.net/v1"
	staraiIdentity *node.Identity
	staraiAPIKey   string // provisioned API key (sk-star-xxx)
)

// InitStarAIProxy initializes the shared StarAI proxy client.
// Called once during Claw router initialization when Identity is available.
func InitStarAIProxy(identity *node.Identity, baseURL string) {
	staraiMu.Lock()
	defer staraiMu.Unlock()
	staraiIdentity = identity
	staraiBaseURL = baseURL
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
			// New key — store it
			staraiMu.Lock()
			staraiAPIKey = result.APIKey
			staraiMu.Unlock()
			log.Printf("[StarAI] API key provisioned: %s (user=%s, balance=%.2f分)", result.KeyPrefix, result.UserID, result.Balance)
			return

		case "existing":
			// Key already exists on Router side
			log.Printf("[StarAI] API key already active: %s (user=%s, balance=%.2f分)", result.KeyPrefix, result.UserID, result.Balance)
			return

		default:
			log.Printf("[StarAI] provision unexpected: status=%s msg=%s", result.Status, result.Message)
		}
	}
	log.Printf("[StarAI] provision failed after 3 attempts — will use Ed25519 signature auth")
}
