package swarm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

// BillingClient talks to Queen API for centralized billing in hosted mode.
// It caches balance locally and syncs periodically to reduce latency.
type BillingClient struct {
	queenURL    string
	nodeToken   string // X-Node-Token for internal API auth
	httpC       *http.Client
	cache       map[string]*balanceCache
	mu          sync.RWMutex
	enabled     bool
	stopCh      chan struct{}
}

type balanceCache struct {
	Balance   int64
	UpdatedAt time.Time
}

const balanceCacheTTL = 30 * time.Second

// NewBillingClient creates a billing client for Queen API
func NewBillingClient(queenURL, nodeToken string) *BillingClient {
	enabled := queenURL != "" && nodeToken != ""
	bc := &BillingClient{
		queenURL:  queenURL,
		nodeToken: nodeToken,
		httpC:     &http.Client{Timeout: 5 * time.Second},
		cache:     make(map[string]*balanceCache),
		enabled:   enabled,
		stopCh:    make(chan struct{}),
	}
	if enabled {
		log.Printf("[billing-client] Initialized, queen=%s", queenURL)
	}
	return bc
}

// IsEnabled returns true when centralized billing is active
func (bc *BillingClient) IsEnabled() bool {
	return bc.enabled
}

// CheckBalance returns true if user has positive balance on Queen.
// Uses local cache to avoid latency on every request.
func (bc *BillingClient) CheckBalance(userID string) (bool, int64, error) {
	if !bc.enabled {
		return true, 0, nil
	}

	// Check cache first
	bc.mu.RLock()
	if cached, ok := bc.cache[userID]; ok && time.Since(cached.UpdatedAt) < balanceCacheTTL {
		bc.mu.RUnlock()
		return cached.Balance > 0, cached.Balance, nil
	}
	bc.mu.RUnlock()

	// Call Queen API
	body := map[string]string{
		"user_id": userID,
	}
	data, _ := json.Marshal(body)

	req, _ := http.NewRequest("POST", bc.queenURL+"/internal/billing/check", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Node-Token", bc.nodeToken)

	resp, err := bc.httpC.Do(req)
	if err != nil {
		// On network error, allow request (fail open)
		log.Printf("[billing-client] check balance failed for %s: %v (fail open)", userID, err)
		return true, 0, nil
	}
	defer resp.Body.Close()

	var result struct {
		Balance  int64 `json:"balance"`
		HasQuota bool  `json:"has_quota"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return true, 0, nil
	}

	// Update cache
	bc.mu.Lock()
	bc.cache[userID] = &balanceCache{Balance: result.Balance, UpdatedAt: time.Now()}
	bc.mu.Unlock()

	return result.HasQuota, result.Balance, nil
}

// Consume reports resource usage to Queen and deducts balance.
// Returns the new balance or an error if insufficient funds.
func (bc *BillingClient) Consume(userID, resourceType string, quantity int64, remark string) (int64, error) {
	if !bc.enabled {
		return 0, nil
	}

	body := map[string]interface{}{
		"user_id":       userID,
		"resource_type": resourceType,
		"quantity":      quantity,
		"remark":        remark,
	}
	data, _ := json.Marshal(body)

	req, _ := http.NewRequest("POST", bc.queenURL+"/internal/billing/consume", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Node-Token", bc.nodeToken)

	resp, err := bc.httpC.Do(req)
	if err != nil {
		return 0, fmt.Errorf("billing consume failed: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Deducted int64  `json:"deducted"`
		Balance  int64  `json:"balance"`
		Error    string `json:"error"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	if resp.StatusCode == http.StatusPaymentRequired {
		// Insufficient balance — update cache
		bc.mu.Lock()
		bc.cache[userID] = &balanceCache{Balance: 0, UpdatedAt: time.Now()}
		bc.mu.Unlock()
		return 0, fmt.Errorf("余额不足，请充值后继续使用")
	}

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("billing error: %s", result.Error)
	}

	// Update cache with new balance
	bc.mu.Lock()
	bc.cache[userID] = &balanceCache{Balance: result.Balance, UpdatedAt: time.Now()}
	bc.mu.Unlock()

	return result.Balance, nil
}

// GetBalance fetches user balance from Queen (bypasses cache)
func (bc *BillingClient) GetBalance(userID string) (int64, error) {
	if !bc.enabled {
		return 0, nil
	}

	req, _ := http.NewRequest("GET", fmt.Sprintf("%s/internal/billing/balance/%s", bc.queenURL, userID), nil)
	req.Header.Set("X-Node-Token", bc.nodeToken)

	resp, err := bc.httpC.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var result struct {
		Balance int64 `json:"balance"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	bc.mu.Lock()
	bc.cache[userID] = &balanceCache{Balance: result.Balance, UpdatedAt: time.Now()}
	bc.mu.Unlock()

	return result.Balance, nil
}

// InvalidateCache clears cached balance for a user (call after recharge)
func (bc *BillingClient) InvalidateCache(userID string) {
	bc.mu.Lock()
	delete(bc.cache, userID)
	bc.mu.Unlock()
}
