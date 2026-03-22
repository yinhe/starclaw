package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// BillingService communicates with Queen's internal credit API
// to check balances, freeze/unfreeze credits, and consume.
type BillingService struct {
	queenURL string // e.g. https://queen.starclaw.net
	token    string // internal service token
	client   *http.Client
}

func NewBillingService(queenURL, token string) *BillingService {
	return &BillingService{
		queenURL: queenURL,
		token:    token,
		client:   &http.Client{Timeout: 10 * time.Second},
	}
}

// BalanceResponse from Queen GET /internal/credits/balance/:claw_id
type BalanceResponse struct {
	ClawID  string `json:"claw_id"`
	Balance int64  `json:"balance"`
	Status  string `json:"status"`
}

// FreezeResponse from Queen POST /internal/credits/freeze
type FreezeResponse struct {
	FreezeID string `json:"freeze_id"`
	Frozen   int64  `json:"frozen"`
}

// ConsumeResponse from Queen POST /internal/credits/consume
type ConsumeResponse struct {
	ClawID   string `json:"claw_id"`
	Deducted int64  `json:"deducted"`
	Balance  int64  `json:"balance"`
}

// GetBalance returns the credit balance for a claw.
func (s *BillingService) GetBalance(clawID string) (*BalanceResponse, error) {
	resp, err := s.doGet(fmt.Sprintf("/internal/credits/balance/%s", clawID))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, s.readError(resp)
	}
	var result BalanceResponse
	json.NewDecoder(resp.Body).Decode(&result)
	return &result, nil
}

// Freeze freezes credits for a pending order. Returns freeze_id for later settlement.
func (s *BillingService) Freeze(clawID string, amount int64, remark string) (*FreezeResponse, error) {
	body := map[string]interface{}{
		"claw_id": clawID,
		"amount":  amount,
		"remark":  remark,
	}
	resp, err := s.doPost("/internal/credits/freeze", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, s.readError(resp)
	}
	var result FreezeResponse
	json.NewDecoder(resp.Body).Decode(&result)
	return &result, nil
}

// Consume directly deducts credits (for confirmed orders).
func (s *BillingService) Consume(clawID string, amount int64, resourceType, remark string) (*ConsumeResponse, error) {
	body := map[string]interface{}{
		"claw_id":       clawID,
		"amount":        amount,
		"resource_type": resourceType,
		"remark":        remark,
	}
	resp, err := s.doPost("/internal/credits/consume", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusPaymentRequired {
		return nil, fmt.Errorf("余额不足")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, s.readError(resp)
	}
	var result ConsumeResponse
	json.NewDecoder(resp.Body).Decode(&result)
	return &result, nil
}

// Unfreeze releases a previously frozen amount (order cancelled/failed).
func (s *BillingService) Unfreeze(clawID string, amount int64, freezeID string) error {
	body := map[string]interface{}{
		"claw_id":   clawID,
		"amount":    amount,
		"freeze_id": freezeID,
	}
	resp, err := s.doPost("/internal/credits/unfreeze", body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return s.readError(resp)
	}
	return nil
}

func (s *BillingService) doGet(path string) (*http.Response, error) {
	req, err := http.NewRequest("GET", s.queenURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+s.token)
	return s.client.Do(req)
}

func (s *BillingService) doPost(path string, body interface{}) (*http.Response, error) {
	jsonData, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", s.queenURL+path, bytes.NewReader(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.token)
	return s.client.Do(req)
}

func (s *BillingService) readError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)
	var errResp struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(body, &errResp) == nil && errResp.Error != "" {
		return fmt.Errorf("queen: %s", errResp.Error)
	}
	return fmt.Errorf("queen: HTTP %d", resp.StatusCode)
}
