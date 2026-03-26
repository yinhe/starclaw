package billing

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"gorm.io/gorm"
	pheromone "starclaw.net/pheromone/sdk"
	"starclaw.net/synapse/api/internal/model"
)

// phClient holds the Pheromone client singleton for use across billing.
var (
	phClient *pheromone.Client
	phMu     sync.RWMutex
	phDB     *gorm.DB
)

// SetPheromone stores the Pheromone client for billing/event use.
func SetPheromone(client *pheromone.Client) {
	phMu.Lock()
	phClient = client
	phMu.Unlock()
}

// SetPheromoneDB stores the DB reference for RPC handlers.
func SetPheromoneDB(db *gorm.DB) {
	phMu.Lock()
	phDB = db
	phMu.Unlock()
}

// PublishUsageAlert publishes a usage alert event to the Pheromone ESB.
func PublishUsageAlert(userID, model string, cost, threshold float64, message string) {
	phMu.RLock()
	c := phClient
	phMu.RUnlock()
	if c == nil {
		return
	}
	if err := c.Publish(pheromone.SubjectUsageAlert, pheromone.UsageAlertEvent{
		UserID:    userID,
		Model:     model,
		Cost:      cost,
		Threshold: threshold,
		Message:   message,
	}); err != nil {
		log.Printf("[synapse] pheromone publish usage alert failed: %v", err)
	}
}

// RegisterSynapseRPC registers Synapse's RPC handlers on the Pheromone ESB.
func RegisterSynapseRPC(client *pheromone.Client) {
	if err := client.HandleRPC(pheromone.RPCGetUsage, handleGetUsage); err != nil {
		log.Printf("[synapse] pheromone RPC register %s failed: %v", pheromone.RPCGetUsage, err)
	}
	log.Printf("[synapse] pheromone RPC handlers registered: %s", pheromone.RPCGetUsage)
}

// SubscribeQueenEvents subscribes to Queen billing events for real-time updates.
func SubscribeQueenEvents(client *pheromone.Client) {
	if err := client.Subscribe("queen.billing.>", func(subject string, data []byte) {
		log.Printf("[synapse] queen billing event: %s", subject)
	}); err != nil {
		log.Printf("[synapse] subscribe queen events failed: %v", err)
	}
	if err := client.Subscribe("queen.user.>", func(subject string, data []byte) {
		log.Printf("[synapse] queen user event: %s", subject)
	}); err != nil {
		log.Printf("[synapse] subscribe queen user events failed: %v", err)
	}
	log.Printf("[synapse] subscribed to queen.billing.> and queen.user.> events")
}

type getUsageRequest struct {
	UserID string `json:"user_id"`
	Days   int    `json:"days"`
}

type usageSummary struct {
	UserID      string  `json:"user_id"`
	TotalCost   float64 `json:"total_cost"`
	TotalTokens int64   `json:"total_tokens"`
	CallCount   int64   `json:"call_count"`
	Days        int     `json:"days"`
}

func handleGetUsage(data []byte) (interface{}, error) {
	phMu.RLock()
	db := phDB
	phMu.RUnlock()
	if db == nil {
		return nil, fmt.Errorf("database not available")
	}

	var req getUsageRequest
	_ = json.Unmarshal(data, &req)
	if req.UserID == "" {
		return nil, fmt.Errorf("user_id is required")
	}
	if req.Days <= 0 {
		req.Days = 7
	}

	since := time.Now().AddDate(0, 0, -req.Days)
	var result usageSummary
	result.UserID = req.UserID
	result.Days = req.Days

	db.Model(&model.UsageRecord{}).Where("user_id = ? AND created_at >= ?", req.UserID, since).
		Select("COALESCE(SUM(cost_cents), 0) as total_cost, COALESCE(SUM(total_tokens), 0) as total_tokens, COUNT(*) as call_count").
		Scan(&result)

	return result, nil
}

// PheromoneCredit provides RPC-based credit operations via Pheromone ESB.
// Falls back to HTTP QueenCreditClient when RPC is unavailable.
type PheromoneCredit struct {
	mu   sync.RWMutex
	ph   *pheromone.Client
	http *QueenCreditClient // HTTP fallback
}

// NewPheromoneCredit creates a hybrid credit client (RPC-first, HTTP-fallback).
func NewPheromoneCredit(ph *pheromone.Client, httpClient *QueenCreditClient) *PheromoneCredit {
	return &PheromoneCredit{ph: ph, http: httpClient}
}

// CheckBalance checks credit balance via RPC, falls back to HTTP.
func (p *PheromoneCredit) CheckBalance(userID string) (balance float64, ok bool, err error) {
	if p.ph != nil {
		bal, rpcOk, rpcErr := p.checkBalanceRPC(userID)
		if rpcErr == nil {
			return bal, rpcOk, nil
		}
		log.Printf("[star-ai] pheromone RPC check-credit failed, falling back to HTTP: %v", rpcErr)
	}

	// HTTP fallback
	if p.http != nil && p.http.Enabled() {
		httpBal, httpErr := p.http.GetBalance(userID)
		if httpErr != nil {
			return 0, false, httpErr
		}
		return float64(httpBal.Balance), httpBal.Balance > 0, nil
	}

	return 0, false, nil
}

func (p *PheromoneCredit) checkBalanceRPC(userID string) (float64, bool, error) {
	req := pheromone.CreditRequest{UserID: userID}
	data, err := p.ph.Call("queen", pheromone.RPCCheckCredit, req, 3*time.Second)
	if err != nil {
		return 0, false, err
	}

	var resp pheromone.CreditResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return 0, false, err
	}
	return resp.Balance, resp.OK, nil
}
