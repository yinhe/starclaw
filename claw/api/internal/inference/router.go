package inference

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/yinhe/starclaw/internal/node"
)

// InferenceRequest is the payload clients send to the router.
type InferenceRequest struct {
	Model       string                   `json:"model" binding:"required"`
	Messages    []map[string]interface{} `json:"messages" binding:"required"`
	Temperature float64                  `json:"temperature,omitempty"`
	MaxTokens   int                      `json:"max_tokens,omitempty"`
	Stream      bool                     `json:"stream"`
}

// InferenceRouter routes inference requests to miner nodes with signature auth.
type InferenceRouter struct {
	Registry *MinerRegistry
	Identity *node.Identity
	httpC    *http.Client
}

// NewInferenceRouter creates a new router.
func NewInferenceRouter(identity *node.Identity) *InferenceRouter {
	return &InferenceRouter{
		Registry: NewMinerRegistry(),
		Identity: identity,
		httpC: &http.Client{
			Timeout: 5 * time.Minute, // long timeout for inference
		},
	}
}

// Route selects a miner and forwards the request, returning a streaming reader.
// The caller is responsible for closing the returned response body.
func (r *InferenceRouter) Route(ctx context.Context, req *InferenceRequest) (*http.Response, *MinerInfo, error) {
	miner := r.Registry.SelectMiner(req.Model)
	if miner == nil {
		return nil, nil, fmt.Errorf("no available miner for model %q", req.Model)
	}

	// Build the request to forward to the miner
	body, err := json.Marshal(req)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	minerURL := miner.Address + "/v1/inference/execute"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", minerURL, bytes.NewReader(body))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// Sign the request with this router's identity
	signRequest(httpReq, r.Identity, body)

	start := time.Now()
	resp, err := r.httpC.Do(httpReq)
	latency := time.Since(start).Milliseconds()

	if err != nil {
		// Mark miner as potentially offline
		r.Registry.Heartbeat(miner.NodeID, miner.ActiveJobs, 0)
		return nil, miner, fmt.Errorf("miner %s unreachable: %w", miner.NodeID[:16], err)
	}

	// Update miner latency
	r.Registry.Heartbeat(miner.NodeID, miner.ActiveJobs+1, latency)

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, miner, fmt.Errorf("miner %s returned HTTP %d: %s", miner.NodeID[:16], resp.StatusCode, string(respBody))
	}

	log.Printf("[inference/router] routed %s to miner %s (%s) latency=%dms",
		req.Model, miner.NodeID[:16], miner.Address, latency)

	return resp, miner, nil
}

// signRequest signs an outgoing HTTP request with the node's Ed25519 key.
// Protocol: sign("METHOD\nPATH\nTIMESTAMP\nBODY_SHA256")
func signRequest(req *http.Request, identity *node.Identity, body []byte) {
	ts := fmt.Sprintf("%d", time.Now().Unix())
	bodyHash := sha256.Sum256(body)
	message := fmt.Sprintf("%s\n%s\n%s\n%s",
		req.Method,
		req.URL.Path,
		ts,
		hex.EncodeToString(bodyHash[:]),
	)

	sig := identity.Sign([]byte(message))

	req.Header.Set("X-Node-ID", identity.NodeID)
	req.Header.Set("X-Node-PubKey", identity.PublicKeyHex())
	req.Header.Set("X-Node-Signature", hex.EncodeToString(sig))
	req.Header.Set("X-Node-Timestamp", ts)
}
