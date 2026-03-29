package engine

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

// QueenClient calls Queen internal APIs for billing.
type QueenClient struct {
	baseURL   string
	nodeToken string
	httpC     *http.Client
}

func NewQueenClient() *QueenClient {
	return &QueenClient{
		baseURL:   envOr("QUEEN_API_URL", "http://queen-api:8091"),
		nodeToken: envOr("QUEEN_NODE_TOKEN", ""),
		httpC:     &http.Client{Timeout: 10 * time.Second},
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// ConsumeStarEnergy deducts star energy from a claw node's account.
// Returns remaining balance or error.
func (q *QueenClient) ConsumeStarEnergy(clawID string, amount int64, remark string) (int64, error) {
	if q.nodeToken == "" {
		return 0, fmt.Errorf("billing not configured (no QUEEN_NODE_TOKEN)")
	}

	body, _ := json.Marshal(map[string]interface{}{
		"claw_id":       clawID,
		"amount":        amount,
		"resource_type": "arena_shop",
		"remark":        remark,
	})

	req, err := http.NewRequest("POST", q.baseURL+"/internal/credits/consume", bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Node-Token", q.nodeToken)

	resp, err := q.httpC.Do(req)
	if err != nil {
		return 0, fmt.Errorf("queen api unreachable: %w", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if resp.StatusCode != 200 {
		errMsg := "billing failed"
		if e, ok := result["error"].(string); ok {
			errMsg = e
		}
		return 0, fmt.Errorf(errMsg)
	}

	balance := int64(0)
	if b, ok := result["balance"].(float64); ok {
		balance = int64(b)
	}
	return balance, nil
}

// GetBalance returns the star energy balance for a claw node.
func (q *QueenClient) GetBalance(clawID string) (int64, error) {
	if q.nodeToken == "" {
		return 0, fmt.Errorf("billing not configured")
	}

	req, err := http.NewRequest("GET", q.baseURL+"/internal/credits/balance/"+clawID, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("X-Node-Token", q.nodeToken)

	resp, err := q.httpC.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if b, ok := result["balance"].(float64); ok {
		return int64(b), nil
	}
	return 0, nil
}
