package billing

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// Client calls queen-api internal billing endpoints for fund escrow
type Client struct {
	baseURL    string // e.g. "http://queen-api:8085"
	nodeToken  string // X-Node-Token for internal auth
	httpClient *http.Client
}

func NewClient(queenAPIURL, nodeToken string) *Client {
	return &Client{
		baseURL:   queenAPIURL,
		nodeToken: nodeToken,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Freeze freezes amount from user balance for bounty escrow
func (c *Client) Freeze(userID string, amountCents int64, bountyID string) error {
	body := map[string]interface{}{
		"user_id":   userID,
		"amount":    amountCents,
		"bounty_id": bountyID,
	}
	resp, err := c.post("/internal/billing/freeze", body)
	if err != nil {
		return fmt.Errorf("billing freeze request failed: %w", err)
	}
	if resp.Error != "" {
		return fmt.Errorf("billing freeze: %s", resp.Error)
	}
	log.Printf("[bounty-billing] Frozen %d cents for user=%s bounty=%s", amountCents, userID, bountyID)
	return nil
}

// Unfreeze returns frozen amount back to user (cancel/expire)
func (c *Client) Unfreeze(userID string, amountCents int64, bountyID string) error {
	body := map[string]interface{}{
		"user_id":   userID,
		"amount":    amountCents,
		"bounty_id": bountyID,
	}
	resp, err := c.post("/internal/billing/unfreeze", body)
	if err != nil {
		return fmt.Errorf("billing unfreeze request failed: %w", err)
	}
	if resp.Error != "" {
		return fmt.Errorf("billing unfreeze: %s", resp.Error)
	}
	log.Printf("[bounty-billing] Unfrozen %d cents for user=%s bounty=%s", amountCents, userID, bountyID)
	return nil
}

// Settle transfers frozen amount from creator to completer (with platform fee)
func (c *Client) Settle(fromUserID, toUserID string, amountCents int64, feeRate float64, bountyID string) error {
	body := map[string]interface{}{
		"from_user_id": fromUserID,
		"to_user_id":   toUserID,
		"amount":       amountCents,
		"fee_rate":     feeRate,
		"bounty_id":    bountyID,
	}
	resp, err := c.post("/internal/billing/settle", body)
	if err != nil {
		return fmt.Errorf("billing settle request failed: %w", err)
	}
	if resp.Error != "" {
		return fmt.Errorf("billing settle: %s", resp.Error)
	}
	log.Printf("[bounty-billing] Settled bounty=%s from=%s to=%s amount=%d", bountyID, fromUserID, toUserID, amountCents)
	return nil
}

type apiResponse struct {
	Error string `json:"error"`
}

func (c *Client) post(path string, body interface{}) (*apiResponse, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", c.baseURL+path, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Node-Token", c.nodeToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	var result apiResponse
	json.Unmarshal(data, &result)

	if resp.StatusCode >= 400 {
		if result.Error != "" {
			return &result, nil
		}
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(data))
	}

	return &result, nil
}
