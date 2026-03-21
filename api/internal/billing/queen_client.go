package billing

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"
)

// QueenClient extends the billing communication with Queen API.
// It adds resolve-partners and investor-deposit on top of the existing
// balance check/consume capabilities.
type QueenClient struct {
	queenURL  string
	nodeToken string
	clawID    string // this node's claw_id, sent in billing requests
	httpC     *http.Client
	enabled   bool

	// Partner resolution cache (claw_id → partners)
	partnerCache   map[string]*partnerCacheEntry
	partnerCacheMu sync.RWMutex
}

type partnerCacheEntry struct {
	CityPartnerID string
	CorePartnerID string
	UpdatedAt     time.Time
}

const partnerCacheTTL = 5 * time.Minute

// NewQueenClient creates a new Queen API client for billing gateway.
func NewQueenClient(queenURL, nodeToken, clawID string) *QueenClient {
	enabled := queenURL != "" && nodeToken != ""
	qc := &QueenClient{
		queenURL:     queenURL,
		nodeToken:    nodeToken,
		clawID:       clawID,
		httpC:        &http.Client{Timeout: 5 * time.Second},
		enabled:      enabled,
		partnerCache: make(map[string]*partnerCacheEntry),
	}
	if enabled {
		log.Printf("[queen-client] Billing gateway client initialized, queen=%s", queenURL)
	}
	return qc
}

// IsEnabled returns true when Queen API is reachable
func (qc *QueenClient) IsEnabled() bool {
	return qc.enabled
}

// CheckBalance checks if user has positive balance on Queen.
func (qc *QueenClient) CheckBalance(userID string) (bool, int64, error) {
	if !qc.enabled {
		return true, 0, nil
	}

	body, _ := json.Marshal(map[string]string{"user_id": userID, "claw_id": qc.clawID})
	req, _ := http.NewRequest("POST", qc.queenURL+"/internal/billing/check", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Node-Token", qc.nodeToken)

	resp, err := qc.httpC.Do(req)
	if err != nil {
		log.Printf("[queen-client] check balance failed: %v (fail open)", err)
		return true, 0, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		log.Printf("[queen-client] check balance HTTP %d: %s (fail open)", resp.StatusCode, string(respBody))
		return true, 0, nil // fail open on non-200
	}

	var result struct {
		Balance  int64 `json:"balance"`
		HasQuota bool  `json:"has_quota"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("[queen-client] check balance decode error: %v (fail open)", err)
		return true, 0, nil
	}
	log.Printf("[queen-client] check balance: user=%s claw=%s has_quota=%v balance=%d", userID, qc.clawID, result.HasQuota, result.Balance)
	return result.HasQuota, result.Balance, nil
}

// Consume deducts balance for resource usage on Queen.
// amountFen is the actual cost in 分; if > 0, Queen uses it directly instead of auto-calculating.
func (qc *QueenClient) Consume(userID, resourceType string, quantity int64, amountFen int64, remark string) (int64, error) {
	if !qc.enabled {
		return 0, nil
	}

	payload := map[string]interface{}{
		"user_id":       userID,
		"claw_id":       qc.clawID,
		"resource_type": resourceType,
		"quantity":      quantity,
		"remark":        remark,
	}
	if amountFen > 0 {
		payload["amount"] = amountFen
	}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", qc.queenURL+"/internal/billing/consume", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Node-Token", qc.nodeToken)

	resp, err := qc.httpC.Do(req)
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
		return 0, fmt.Errorf("余额不足，请充值后继续使用")
	}
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("billing error: %s", result.Error)
	}

	return result.Balance, nil
}

// ResolvePartners queries Queen for the partner chain of a claw_id.
// Returns (cityPartnerID, corePartnerID). Uses cache.
func (qc *QueenClient) ResolvePartners(clawID string) (string, string) {
	if !qc.enabled || clawID == "" {
		return "", ""
	}

	// Check cache
	qc.partnerCacheMu.RLock()
	if cached, ok := qc.partnerCache[clawID]; ok && time.Since(cached.UpdatedAt) < partnerCacheTTL {
		qc.partnerCacheMu.RUnlock()
		return cached.CityPartnerID, cached.CorePartnerID
	}
	qc.partnerCacheMu.RUnlock()

	// Call Queen API
	url := fmt.Sprintf("%s/internal/billing/resolve-partners?claw_id=%s", qc.queenURL, clawID)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("X-Node-Token", qc.nodeToken)

	resp, err := qc.httpC.Do(req)
	if err != nil {
		log.Printf("[queen-client] resolve-partners failed: %v", err)
		return "", ""
	}
	defer resp.Body.Close()

	var result struct {
		CityPartnerID string `json:"city_partner_id"`
		CorePartnerID string `json:"core_partner_id"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	// Update cache
	qc.partnerCacheMu.Lock()
	qc.partnerCache[clawID] = &partnerCacheEntry{
		CityPartnerID: result.CityPartnerID,
		CorePartnerID: result.CorePartnerID,
		UpdatedAt:     time.Now(),
	}
	qc.partnerCacheMu.Unlock()

	return result.CityPartnerID, result.CorePartnerID
}

// DepositInvestorPool deposits profit into the investor pool on Queen.
func (qc *QueenClient) DepositInvestorPool(sourceType, sourceID string, amount, marginTotal int64, clawID string) {
	if !qc.enabled || amount <= 0 {
		return
	}

	body, _ := json.Marshal(map[string]interface{}{
		"source_type":  sourceType,
		"source_id":    sourceID,
		"amount":       amount,
		"margin_total": marginTotal,
		"rate":         InvestorShareRate,
		"claw_id":      clawID,
	})

	req, _ := http.NewRequest("POST", qc.queenURL+"/internal/investor/deposit", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Node-Token", qc.nodeToken)

	resp, err := qc.httpC.Do(req)
	if err != nil {
		log.Printf("[queen-client] investor deposit failed: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[queen-client] investor deposit returned %d", resp.StatusCode)
	}
}
