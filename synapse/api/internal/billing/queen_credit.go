package billing

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/yinhe/starclaw-router/internal/config"
)

// QueenCreditClient calls Queen's internal credit API for star energy billing.
type QueenCreditClient struct {
	baseURL string
	token   string
	client  *http.Client
}

// NewQueenCreditClient creates a new client for Queen's internal credit API.
func NewQueenCreditClient(cfg config.QueenConfig) *QueenCreditClient {
	return &QueenCreditClient{
		baseURL: cfg.URL,
		token:   cfg.Token,
		client:  &http.Client{Timeout: 5 * time.Second},
	}
}

// Enabled returns true if Queen credit billing is configured.
func (q *QueenCreditClient) Enabled() bool {
	return q.baseURL != "" && q.token != ""
}

// CreditBalance holds the response from Queen's balance check.
type CreditBalance struct {
	ClawID  string `json:"claw_id"`
	Balance int64  `json:"balance"`
	Status  string `json:"status"`
}

// GetBalance checks the star energy balance for a claw address.
func (q *QueenCreditClient) GetBalance(clawID string) (*CreditBalance, error) {
	url := fmt.Sprintf("%s/internal/credits/balance/%s", q.baseURL, clawID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Node-Token", q.token)

	resp, err := q.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("queen unreachable: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("queen balance check failed (%d): %s", resp.StatusCode, string(body))
	}

	var result CreditBalance
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("queen response parse error: %w", err)
	}
	return &result, nil
}

// CheckBalance returns nil if the claw has sufficient star energy balance.
func (q *QueenCreditClient) CheckBalance(clawID string) error {
	bal, err := q.GetBalance(clawID)
	if err != nil {
		return err
	}
	if bal.Status == "hibernated" {
		return fmt.Errorf("claw hibernated (0 star energy)")
	}
	if bal.Balance <= 0 {
		return fmt.Errorf("insufficient star energy")
	}
	return nil
}

// ConsumeRequest is the request body for consuming star energy.
type ConsumeRequest struct {
	ClawID       string `json:"claw_id"`
	Amount       int64  `json:"amount"`
	ResourceType string `json:"resource_type"` // tokens / image / video
	Quantity     int64  `json:"quantity"`      // e.g. token count
	Remark       string `json:"remark"`
}

// ConsumeResponse is the response from Queen after consuming star energy.
type ConsumeResponse struct {
	Deducted int64 `json:"deducted"`
	Balance  int64 `json:"balance"`
}

// Consume deducts star energy from a claw account via Queen's internal API.
func (q *QueenCreditClient) Consume(req *ConsumeRequest) (*ConsumeResponse, error) {
	body, _ := json.Marshal(req)
	url := fmt.Sprintf("%s/internal/credits/consume", q.baseURL)
	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("X-Node-Token", q.token)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := q.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("queen unreachable: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusPaymentRequired {
		return nil, fmt.Errorf("insufficient star energy")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("queen consume failed (%d): %s", resp.StatusCode, string(respBody))
	}

	var result ConsumeResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("queen response parse error: %w", err)
	}

	log.Printf("[star-ai] consumed %d star units from %s (remaining: %d)", result.Deducted, req.ClawID, result.Balance)
	return &result, nil
}

// ConsumptionRecord represents a single tool consumption record from Queen.
type ConsumptionRecord struct {
	ID        string    `json:"id"`
	Remark    string    `json:"remark"`
	Amount    int64     `json:"amount"` // energy units (1⚡ = 10000)
	CreatedAt time.Time `json:"created_at"`
}

// GetConsumption fetches recent tool consumption records for a claw from Queen.
func (q *QueenCreditClient) GetConsumption(clawID string, days int) ([]ConsumptionRecord, error) {
	url := fmt.Sprintf("%s/internal/billing/consumption?claw_id=%s&days=%d", q.baseURL, clawID, days)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Node-Token", q.token)

	resp, err := q.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("queen unreachable: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("queen consumption query failed (%d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Records []ConsumptionRecord `json:"records"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("queen response parse error: %w", err)
	}
	return result.Records, nil
}
