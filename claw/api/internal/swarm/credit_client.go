package swarm

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/yinhe/starclaw/internal/node"
)

// HPStatus represents claw health (star energy = HP)
type HPStatus string

const (
	HPFull       HPStatus = "full"       // > 1000 ⚡
	HPHealthy    HPStatus = "healthy"    // 100–1000 ⚡
	HPLow        HPStatus = "low"        // 10–100 ⚡
	HPCritical   HPStatus = "critical"   // 1–10 ⚡
	HPHibernated HPStatus = "hibernated" // 0 ⚡
	HPUnknown    HPStatus = "unknown"    // not yet queried
)

// CreditClient handles star energy operations with Queen's ledger API.
// It provides Ed25519-signed transfers, balance queries, and HP monitoring.
type CreditClient struct {
	queenURL string
	identity *node.Identity
	httpC    *http.Client

	mu        sync.RWMutex
	cached    *CreditBalance
	hp        HPStatus
	callbacks []func(HPStatus)
}

// NewCreditClient creates a credit client for interacting with Queen's ledger.
func NewCreditClient(queenURL string, identity *node.Identity) *CreditClient {
	return &CreditClient{
		queenURL: strings.TrimRight(queenURL, "/"),
		identity: identity,
		httpC:    &http.Client{Timeout: 10 * time.Second},
		hp:       HPUnknown,
	}
}

// OnHPChange registers a callback that fires when HP status changes.
func (cc *CreditClient) OnHPChange(fn func(HPStatus)) {
	cc.mu.Lock()
	cc.callbacks = append(cc.callbacks, fn)
	cc.mu.Unlock()
}

// HP returns the current HP status.
func (cc *CreditClient) HP() HPStatus {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	return cc.hp
}

// CachedBalance returns the last known balance (updated via heartbeat or direct query).
func (cc *CreditClient) CachedBalance() *CreditBalance {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	return cc.cached
}

// UpdateFromHeartbeat updates cached balance from swarm heartbeat response data.
func (cc *CreditClient) UpdateFromHeartbeat(cb *CreditBalance) {
	if cb == nil {
		return
	}
	cc.mu.Lock()
	cc.cached = cb
	oldHP := cc.hp
	cc.hp = HPStatus(cb.HPStatus)
	callbacks := cc.callbacks
	cc.mu.Unlock()

	if oldHP != HPStatus(cb.HPStatus) && oldHP != HPUnknown {
		newHP := HPStatus(cb.HPStatus)
		logHPChange(oldHP, newHP, cb.BalanceEnergy)
		for _, fn := range callbacks {
			fn(newHP)
		}
	}
}

// QueryBalance fetches balance directly from Queen's public credit API.
func (cc *CreditClient) QueryBalance() (*CreditBalance, error) {
	if cc.queenURL == "" {
		return nil, fmt.Errorf("queen_url not configured")
	}

	url := fmt.Sprintf("%s/v1/credits/balance?claw_id=%s", cc.queenURL, cc.identity.NodeID)
	resp, err := cc.httpC.Get(url)
	if err != nil {
		return nil, fmt.Errorf("query balance: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			ClawID        string  `json:"claw_id"`
			Balance       int64   `json:"balance"`
			BalanceEnergy float64 `json:"balance_energy"`
			Frozen        int64   `json:"frozen"`
			FrozenEnergy  float64 `json:"frozen_energy"`
			TotalIn       int64   `json:"total_in"`
			TotalOut      int64   `json:"total_out"`
			Nonce         int64   `json:"nonce"`
			Status        string  `json:"status"`
			HPStatus      string  `json:"hp_status"`
			TrustLevel    string  `json:"trust_level"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode balance: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("balance query failed: %s", result.Msg)
	}

	cb := &CreditBalance{
		Balance:       result.Data.Balance,
		BalanceEnergy: result.Data.BalanceEnergy,
		Frozen:        result.Data.Frozen,
		FrozenEnergy:  result.Data.FrozenEnergy,
		TotalIn:       result.Data.TotalIn,
		TotalOut:      result.Data.TotalOut,
		Nonce:         result.Data.Nonce,
		Status:        result.Data.Status,
		HPStatus:      result.Data.HPStatus,
		TrustLevel:    result.Data.TrustLevel,
		UpdatedAt:     time.Now(),
	}

	cc.UpdateFromHeartbeat(cb)
	return cb, nil
}

// TransferRequest holds parameters for a claw-to-claw transfer.
type TransferRequest struct {
	ToClaw string `json:"to_claw"`
	Amount int64  `json:"amount"` // in internal units (1 Star = 10000)
	Remark string `json:"remark"`
}

// TransferResult holds the result of a successful transfer.
type TransferResult struct {
	TxnID        string  `json:"txn_id"`
	From         string  `json:"from"`
	To           string  `json:"to"`
	Amount       int64   `json:"amount"`
	AmountEnergy float64 `json:"amount_energy"`
	NewBalance   int64   `json:"new_balance"`
}

// Transfer sends star energy to another claw address, signed with Ed25519.
func (cc *CreditClient) Transfer(req TransferRequest) (*TransferResult, error) {
	if cc.queenURL == "" {
		return nil, fmt.Errorf("queen_url not configured")
	}
	if req.Amount <= 0 {
		return nil, fmt.Errorf("amount must be positive")
	}
	if !strings.HasPrefix(req.ToClaw, "claw:") {
		return nil, fmt.Errorf("invalid target address: must start with claw:")
	}
	if req.ToClaw == cc.identity.NodeID {
		return nil, fmt.Errorf("cannot transfer to self")
	}

	// Get current nonce
	cb := cc.CachedBalance()
	var nonce int64
	if cb != nil {
		nonce = cb.Nonce + 1
	} else {
		// Query fresh nonce
		fresh, err := cc.QueryBalance()
		if err != nil {
			return nil, fmt.Errorf("cannot get nonce: %w", err)
		}
		nonce = fresh.Nonce + 1
	}

	// Sign: message = "transfer|{from}|{to}|{amount}|{nonce}"
	message := fmt.Sprintf("transfer|%s|%s|%d|%d", cc.identity.NodeID, req.ToClaw, req.Amount, nonce)
	signature := ed25519.Sign(cc.identity.PrivateKey, []byte(message))
	pubKeyHex := hex.EncodeToString(cc.identity.PublicKey)
	sigHex := hex.EncodeToString(signature)

	body := map[string]interface{}{
		"from_claw":  cc.identity.NodeID,
		"to_claw":    req.ToClaw,
		"amount":     req.Amount,
		"nonce":      nonce,
		"public_key": pubKeyHex,
		"signature":  sigHex,
		"remark":     req.Remark,
	}
	data, _ := json.Marshal(body)

	url := cc.queenURL + "/v1/credits/transfer"
	httpReq, _ := http.NewRequest("POST", url, strings.NewReader(string(data)))
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := cc.httpC.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("transfer request: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			TxnID        string  `json:"txn_id"`
			From         string  `json:"from"`
			To           string  `json:"to"`
			Amount       int64   `json:"amount"`
			AmountEnergy float64 `json:"amount_energy"`
			NewBalance   int64   `json:"new_balance"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode transfer response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("transfer failed: %s", result.Msg)
	}

	// Update cached nonce
	cc.mu.Lock()
	if cc.cached != nil {
		cc.cached.Nonce = nonce
		cc.cached.Balance = result.Data.NewBalance
	}
	cc.mu.Unlock()

	return &TransferResult{
		TxnID:        result.Data.TxnID,
		From:         result.Data.From,
		To:           result.Data.To,
		Amount:       result.Data.Amount,
		AmountEnergy: result.Data.AmountEnergy,
		NewBalance:   result.Data.NewBalance,
	}, nil
}

// TransactionRecord represents a single credit transaction.
type TransactionRecord struct {
	ID        string    `json:"id"`
	FromClaw  string    `json:"from_claw"`
	ToClaw    string    `json:"to_claw"`
	Amount    int64     `json:"amount"`
	Fee       int64     `json:"fee"`
	Type      string    `json:"type"`
	Remark    string    `json:"remark"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// TransactionList holds paginated transaction results.
type TransactionList struct {
	Transactions []TransactionRecord `json:"transactions"`
	Total        int64               `json:"total"`
	Page         int                 `json:"page"`
	PageSize     int                 `json:"page_size"`
}

// ListTransactions fetches transaction history from Queen.
func (cc *CreditClient) ListTransactions(page, pageSize int, txnType string) (*TransactionList, error) {
	if cc.queenURL == "" {
		return nil, fmt.Errorf("queen_url not configured")
	}

	url := fmt.Sprintf("%s/v1/credits/transactions?claw_id=%s&page=%d&page_size=%d",
		cc.queenURL, cc.identity.NodeID, page, pageSize)
	if txnType != "" {
		url += "&type=" + txnType
	}

	resp, err := cc.httpC.Get(url)
	if err != nil {
		return nil, fmt.Errorf("list transactions: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			Transactions []TransactionRecord `json:"transactions"`
			Total        int64               `json:"total"`
			Page         int                 `json:"page"`
			PageSize     int                 `json:"page_size"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode transactions: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("list transactions failed: %s", result.Msg)
	}

	return &TransactionList{
		Transactions: result.Data.Transactions,
		Total:        result.Data.Total,
		Page:         result.Data.Page,
		PageSize:     result.Data.PageSize,
	}, nil
}

// Stats returns a summary map for API exposure.
func (cc *CreditClient) Stats() map[string]interface{} {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	stats := map[string]interface{}{
		"hp":        string(cc.hp),
		"claw_id":   cc.identity.NodeID,
		"queen_url": cc.queenURL,
	}
	if cc.cached != nil {
		stats["balance_energy"] = cc.cached.BalanceEnergy
		stats["frozen_energy"] = cc.cached.FrozenEnergy
		stats["total_in"] = cc.cached.TotalIn
		stats["total_out"] = cc.cached.TotalOut
		stats["nonce"] = cc.cached.Nonce
		stats["status"] = cc.cached.Status
		stats["trust_level"] = cc.cached.TrustLevel
		stats["updated_at"] = cc.cached.UpdatedAt
	}
	return stats
}

func logHPChange(old, new HPStatus, stars float64) {
	switch new {
	case HPHibernated:
		log.Printf("[credits] ⚠️  HP: %s → %s (%.1f ⚡) — HIBERNATED: star energy depleted, recharge to restore full functionality", old, new, stars)
	case HPCritical:
		log.Printf("[credits] ⚠️  HP: %s → %s (%.1f ⚡) — CRITICAL: only basic text chat available", old, new, stars)
	case HPLow:
		log.Printf("[credits] ⚠️  HP: %s → %s (%.1f ⚡) — LOW: high-cost operations (video/image) restricted", old, new, stars)
	case HPHealthy:
		log.Printf("[credits] HP: %s → %s (%.1f ⚡) — healthy", old, new, stars)
	case HPFull:
		log.Printf("[credits] HP: %s → %s (%.1f ⚡) — full health", old, new, stars)
	default:
		log.Printf("[credits] HP: %s → %s (%.1f ⚡)", old, new, stars)
	}
}
