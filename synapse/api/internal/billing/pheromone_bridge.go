package billing

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	pheromone "starclaw.net/pheromone/sdk"
)

// PheromoneCredit provides RPC-based credit operations via Pheromone ESB.
// Falls back to HTTP QueenCreditClient when RPC is unavailable.
type PheromoneCredit struct {
	mu     sync.RWMutex
	ph     *pheromone.Client
	http   *QueenCreditClient // HTTP fallback
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
