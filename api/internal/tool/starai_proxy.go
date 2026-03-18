package tool

import (
	"context"
	"net/http"
	"sync"

	"github.com/yinhe/starclaw/internal/node"
	"github.com/yinhe/starclaw/internal/provider"
)

// StarAI proxy singleton — set once during router init, used by all tools
var (
	staraiMu       sync.RWMutex
	staraiClient   *http.Client // HTTP client with SignedTransport for StarAI Router
	staraiBaseURL  string       // e.g. "https://api.star-ai.net/v1"
	staraiIdentity *node.Identity
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

// StarAIProxyURL builds a full proxy URL for a sub-provider.
// e.g. StarAIProxyURL("fal", "/fal-ai/veo3") → "https://api.star-ai.net/v1/proxy/fal/fal-ai/veo3"
func StarAIProxyURL(subProvider, path string) string {
	staraiMu.RLock()
	base := staraiBaseURL
	staraiMu.RUnlock()
	return base + "/proxy/" + subProvider + path
}
