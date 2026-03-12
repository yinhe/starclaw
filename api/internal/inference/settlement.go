package inference

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

// SettlementClient reports inference usage to the swarm ledger for star credit settlement.
type SettlementClient struct {
	ledgerURL string // swarm ledger endpoint, e.g. https://api.starclaw.net
	nodeToken string // internal API auth token
	httpC     *http.Client
}

// NewSettlementClient creates a settlement reporter.
// If ledgerURL is empty, settlement is disabled (standalone mode).
func NewSettlementClient(ledgerURL, nodeToken string) *SettlementClient {
	return &SettlementClient{
		ledgerURL: ledgerURL,
		nodeToken: nodeToken,
		httpC:     &http.Client{Timeout: 10 * time.Second},
	}
}

// Enabled returns true if settlement reporting is configured.
func (s *SettlementClient) Enabled() bool {
	return s.ledgerURL != "" && s.nodeToken != ""
}

// SettlementReport contains the data needed for inference settlement.
type SettlementReport struct {
	RequesterClaw    string `json:"requester_claw"`
	ContributorClaw  string `json:"contributor_claw"`
	Model            string `json:"model"`
	PromptTokens     int64  `json:"prompt_tokens"`
	CompletionTokens int64  `json:"completion_tokens"`
}

// ReportAsync sends a settlement report to the swarm ledger in a background goroutine.
// Failures are logged but do not block the caller.
func (s *SettlementClient) ReportAsync(report SettlementReport) {
	if !s.Enabled() {
		return
	}
	go func() {
		data, _ := json.Marshal(report)
		req, err := http.NewRequest("POST", s.ledgerURL+"/internal/inference/settle", bytes.NewReader(data))
		if err != nil {
			log.Printf("[inference/settle] failed to create request: %v", err)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+s.nodeToken)

		resp, err := s.httpC.Do(req)
		if err != nil {
			log.Printf("[inference/settle] report failed: %v", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			log.Printf("[inference/settle] ledger returned %d for %s\u2192%s (%s)",
				resp.StatusCode, report.RequesterClaw, report.ContributorClaw, report.Model)
			return
		}

		log.Printf("[inference/settle] settled %s→%s model=%s tokens=%d+%d",
			report.RequesterClaw[:16], report.ContributorClaw[:16],
			report.Model, report.PromptTokens, report.CompletionTokens)
	}()
}
