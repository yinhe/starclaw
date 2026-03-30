package engine

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

// ClawClient communicates with Claw AI Agent for secondary confirmation.
// Flow: Extractor quantitative scoring → Claw LLM analysis → confirmed trade list
type ClawClient struct {
	BaseURL    string // e.g. "http://localhost:8080" or "https://xxx.starclaw.me"
	APIKey     string // Claw API key or Ed25519 auth
	HTTPClient *http.Client
}

func NewClawClient(baseURL, apiKey string) *ClawClient {
	return &ClawClient{
		BaseURL: baseURL,
		APIKey:  apiKey,
		HTTPClient: &http.Client{
			Timeout: 60 * time.Second, // LLM may take a while
		},
	}
}

// ClawConfirmRequest is sent to Claw's native chat API.
type ClawConfirmRequest struct {
	AgentID string `json:"agent_id"`
	Message string `json:"message"`
	Stream  bool   `json:"stream"`
}

// ClawConfirmResponse from Claw /v1/chat/completions.
type ClawConfirmResponse struct {
	// Claw native format
	Content string `json:"content"`
	Role    string `json:"role"`
	// OpenAI compat fallback
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// CandidateStock represents a quantitatively scored stock candidate.
type CandidateStock struct {
	Code        string  `json:"code"`
	Score       float64 `json:"score"`
	TrendOK     bool    `json:"trend_ok"`
	TodayChange float64 `json:"today_change"`
	VolumeRatio float64 `json:"volume_ratio"`
	Reason      string  `json:"reason"`
	// After Claw confirmation
	ClawAction     string   `json:"claw_action,omitempty"` // confirm, reject, reduce
	ClawConfidence float64  `json:"claw_confidence,omitempty"`
	RiskFlags      []string `json:"risk_flags,omitempty"`
	Suggestion     string   `json:"suggestion,omitempty"`
	ReducePosition bool     `json:"reduce_position,omitempty"`
}

// RequestConfirmation sends candidates to Claw AI for secondary analysis.
// Returns the raw LLM response text.
func (c *ClawClient) RequestConfirmation(prompt string, model string) (string, error) {
	// Use first available agent, or env-configured agent ID
	agentID := os.Getenv("EXTRACTOR_CLAW_AGENT_ID")
	if agentID == "" {
		agentID = "default"
	}

	req := ClawConfirmRequest{
		AgentID: agentID,
		Message: prompt,
		Stream:  false,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	url := c.BaseURL + "/v1/chat/completions"
	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
	}

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("claw request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("claw returned %d: %s", resp.StatusCode, string(respBody))
	}

	var clawResp ClawConfirmResponse
	if err := json.Unmarshal(respBody, &clawResp); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}

	// Try Claw native format first (content field), then OpenAI compat (choices)
	content := clawResp.Content
	if content == "" && len(clawResp.Choices) > 0 {
		content = clawResp.Choices[0].Message.Content
	}
	if content == "" {
		return "", fmt.Errorf("claw returned empty response")
	}

	log.Printf("[claw] AI response length: %d chars", len(content))
	return content, nil
}

// Ping checks if Claw is reachable.
func (c *ClawClient) Ping() error {
	resp, err := c.HTTPClient.Get(c.BaseURL + "/health")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("claw health: status %d", resp.StatusCode)
	}
	return nil
}
