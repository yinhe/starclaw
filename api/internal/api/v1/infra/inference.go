package infra

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw/internal/inference"
	"github.com/yinhe/starclaw/internal/provider"
)

// InferenceHandler exposes the inference router API.
type InferenceHandler struct {
	router      *inference.InferenceRouter
	providers   *provider.Registry
	settlement  *inference.SettlementClient
	spotChecker *inference.SpotChecker
}

// InferenceHandlerOption configures optional dependencies for InferenceHandler.
type InferenceHandlerOption func(*InferenceHandler)

// WithSettlement sets the settlement client for star energy reporting.
func WithSettlement(s *inference.SettlementClient) InferenceHandlerOption {
	return func(h *InferenceHandler) { h.settlement = s }
}

// WithSpotChecker sets the spot-checker for contributor trust verification.
func WithSpotChecker(sc *inference.SpotChecker) InferenceHandlerOption {
	return func(h *InferenceHandler) { h.spotChecker = sc }
}

// NewInferenceHandler creates a new handler wrapping the inference router.
func NewInferenceHandler(router *inference.InferenceRouter, providers *provider.Registry, opts ...interface{}) *InferenceHandler {
	h := &InferenceHandler{router: router, providers: providers}
	for _, opt := range opts {
		switch v := opt.(type) {
		case *inference.SettlementClient:
			h.settlement = v
		case *inference.SpotChecker:
			h.spotChecker = v
		case InferenceHandlerOption:
			v(h)
		}
	}
	return h
}

// RegisterContributor handles contributor registration (POST /v1/inference/register).
// Called by compute contributor nodes (protected by NodeSignatureAuth middleware).
func (h *InferenceHandler) RegisterContributor(c *gin.Context) {
	var req struct {
		Address     string   `json:"address" binding:"required"`
		Models      []string `json:"models" binding:"required"`
		MaxTokens   int      `json:"max_tokens"`
		GPUMemoryMB int      `json:"gpu_memory_mb"`
		MaxJobs     int      `json:"max_jobs"`
		Region      string   `json:"region"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Node identity is set by NodeSignatureAuth middleware
	nodeID := c.GetString("node_id")
	pubKey := c.GetString("node_pubkey")
	if nodeID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing node identity"})
		return
	}

	h.router.Registry.Register(&inference.ContributorInfo{
		NodeID:      nodeID,
		PublicKey:   pubKey,
		Address:     req.Address,
		Models:      req.Models,
		MaxTokens:   req.MaxTokens,
		GPUMemoryMB: req.GPUMemoryMB,
		MaxJobs:     req.MaxJobs,
		Region:      req.Region,
	})

	c.JSON(http.StatusOK, gin.H{
		"message":  "contributor registered",
		"node_id":  nodeID,
		"models":   req.Models,
		"max_jobs": req.MaxJobs,
	})
}

// Heartbeat handles contributor heartbeat (POST /v1/inference/heartbeat).
// Called periodically by contributor nodes (protected by NodeSignatureAuth middleware).
func (h *InferenceHandler) Heartbeat(c *gin.Context) {
	var req struct {
		ActiveJobs int   `json:"active_jobs"`
		LatencyMs  int64 `json:"latency_ms"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	nodeID := c.GetString("node_id")
	if nodeID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing node identity"})
		return
	}

	if !h.router.Registry.Heartbeat(nodeID, req.ActiveJobs, req.LatencyMs) {
		c.JSON(http.StatusNotFound, gin.H{"error": "contributor not registered"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

// UnregisterContributor handles contributor deregistration (POST /v1/inference/unregister).
func (h *InferenceHandler) UnregisterContributor(c *gin.Context) {
	nodeID := c.GetString("node_id")
	if nodeID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing node identity"})
		return
	}
	h.router.Registry.Unregister(nodeID)
	c.JSON(http.StatusOK, gin.H{"message": "contributor unregistered"})
}

// ListContributors returns all registered contributors (GET /v1/inference/contributors).
func (h *InferenceHandler) ListContributors(c *gin.Context) {
	contributors := h.router.Registry.ListContributors()
	stats := h.router.Registry.Stats()
	c.JSON(http.StatusOK, gin.H{
		"contributors": contributors,
		"stats":        stats,
	})
}

// Infer routes an inference request to the best contributor (POST /v1/inference/completions).
// Supports both streaming (SSE) and non-streaming responses.
func (h *InferenceHandler) Infer(c *gin.Context) {
	var req inference.InferenceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, contributor, err := h.router.Route(c.Request.Context(), &req)
	if err != nil {
		status := http.StatusServiceUnavailable
		c.JSON(status, gin.H{
			"error":   err.Error(),
			"details": "no contributor available or contributor unreachable",
		})
		return
	}
	defer resp.Body.Close()

	// Add routing metadata headers
	contributorID := ""
	if contributor != nil {
		contributorID = contributor.NodeID
	}
	c.Header("X-Routed-To", contributorID)
	c.Header("X-Router-ID", h.router.Identity.NodeID)

	if req.Stream {
		// SSE streaming proxy: forward contributor's SSE stream to client
		h.proxySSEStream(c, resp, contributorID)
	} else {
		// Non-streaming: read full response and forward
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "failed to read contributor response"})
			return
		}
		c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), body)
	}

	// Update contributor: decrement active jobs + record trust event
	if contributor != nil {
		h.router.Registry.Heartbeat(contributor.NodeID, max(0, contributor.ActiveJobs-1), 0)
		// Record success in trust score (failure is recorded if Route returned error above)
		if contributor.Trust != nil {
			contributor.Trust.RecordSuccess(0) // latency already tracked in Heartbeat
		}
	}

	// Report usage for star energy settlement (async, non-blocking)
	if h.settlement != nil && contributor != nil {
		requesterClaw := c.GetString("claw_id") // set by auth middleware if available
		if requesterClaw == "" {
			requesterClaw = h.router.Identity.NodeID // fallback: this router pays
		}
		h.settlement.ReportAsync(inference.SettlementReport{
			RequesterClaw:   requesterClaw,
			ContributorClaw: contributor.NodeID,
			Model:           req.Model,
		})
	}

	// Spot-check: randomly verify contributor response quality (async, non-blocking)
	if h.spotChecker != nil && contributor != nil && !req.Stream && h.spotChecker.ShouldCheck() {
		// For non-streaming responses, we can capture the body for verification
		// (streaming spot-checks are more complex, deferred to future)
		if bodyBytes := c.GetString("_response_body"); bodyBytes != "" {
			h.spotChecker.VerifyAsync(&req, contributor.NodeID, bodyBytes)
		}
	}
}

// proxySSEStream forwards an SSE stream from the contributor to the client.
func (h *InferenceHandler) proxySSEStream(c *gin.Context, resp *http.Response, contributorID string) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	flusher, _ := c.Writer.(http.Flusher)
	buf := make([]byte, 4096)

	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			c.Writer.Write(buf[:n])
			if flusher != nil {
				flusher.Flush()
			}
		}
		if err != nil {
			if err != io.EOF {
				log.Printf("[inference/router] SSE proxy error from contributor %s: %v", contributorID, err)
			}
			return
		}
	}
}

// Execute handles inference execution on this node (POST /v1/inference/execute).
// This is the contributor-side endpoint called by the router.
// Protected by NodeSignatureAuth — only trusted routers can invoke this.
func (h *InferenceHandler) Execute(c *gin.Context) {
	routerNodeID := c.GetString("node_id")

	var req inference.InferenceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Find a local provider that supports this model
	p := h.findProviderForModel(req.Model)
	if p == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "no local provider for model " + req.Model,
			"node_id": h.router.Identity.NodeID,
		})
		return
	}

	// Convert inference request to provider ChatRequest
	chatReq := &provider.ChatRequest{
		Model:       req.Model,
		Messages:    toProviderMessages(req.Messages),
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		Stream:      req.Stream,
	}

	start := time.Now()

	if req.Stream {
		h.executeStream(c, p, chatReq, routerNodeID, start)
	} else {
		h.executeSync(c, p, chatReq, routerNodeID, start)
	}
}

func (h *InferenceHandler) executeSync(c *gin.Context, p provider.ModelProvider, req *provider.ChatRequest, routerNodeID string, start time.Time) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Minute)
	defer cancel()

	chunk, err := p.ChatSync(ctx, req)
	if err != nil {
		log.Printf("[inference/execute] sync error for %s from router %s: %v", req.Model, routerNodeID[:16], err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	latency := time.Since(start).Milliseconds()
	log.Printf("[inference/execute] served %s (sync) for router %s latency=%dms tokens=%d",
		req.Model, routerNodeID[:16], latency, usageTotal(chunk.Usage))

	c.JSON(http.StatusOK, gin.H{
		"content":    chunk.Content,
		"role":       chunk.Role,
		"usage":      chunk.Usage,
		"node_id":    h.router.Identity.NodeID,
		"latency_ms": latency,
	})
}

func (h *InferenceHandler) executeStream(c *gin.Context, p provider.ModelProvider, req *provider.ChatRequest, routerNodeID string, start time.Time) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Minute)
	defer cancel()

	ch, err := p.Chat(ctx, req)
	if err != nil {
		log.Printf("[inference/execute] stream error for %s from router %s: %v", req.Model, routerNodeID[:16], err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	flusher, _ := c.Writer.(http.Flusher)

	var totalTokens int
	for chunk := range ch {
		data, _ := json.Marshal(gin.H{
			"content": chunk.Content,
			"done":    chunk.Done,
			"usage":   chunk.Usage,
		})
		c.Writer.Write([]byte("data: " + string(data) + "\n\n"))
		if flusher != nil {
			flusher.Flush()
		}
		if chunk.Usage != nil {
			totalTokens = chunk.Usage.TotalTokens
		}
	}

	latency := time.Since(start).Milliseconds()
	log.Printf("[inference/execute] served %s (stream) for router %s latency=%dms tokens=%d",
		req.Model, routerNodeID[:16], latency, totalTokens)
}

// findProviderForModel finds a local provider that supports the given model.
func (h *InferenceHandler) findProviderForModel(model string) provider.ModelProvider {
	if h.providers == nil {
		return nil
	}
	// Try ollama first (most common local provider)
	if p, ok := h.providers.Get("ollama"); ok {
		for _, m := range p.Models() {
			if m == model {
				return p
			}
		}
	}
	// Fallback: check all providers
	for _, name := range h.providers.List() {
		p, _ := h.providers.Get(name)
		for _, m := range p.Models() {
			if m == model {
				return p
			}
		}
	}
	return nil
}

// toProviderMessages converts inference request messages to provider ChatMessages.
func toProviderMessages(msgs []map[string]interface{}) []provider.ChatMessage {
	out := make([]provider.ChatMessage, 0, len(msgs))
	for _, m := range msgs {
		role, _ := m["role"].(string)
		content, _ := m["content"].(string)
		out = append(out, provider.ChatMessage{Role: role, Content: content})
	}
	return out
}

func usageTotal(u *provider.TokenUsage) int {
	if u == nil {
		return 0
	}
	return u.TotalTokens
}

// RouterStatus returns the router's status and identity (GET /v1/inference/status).
func (h *InferenceHandler) RouterStatus(c *gin.Context) {
	stats := h.router.Registry.Stats()
	c.JSON(http.StatusOK, gin.H{
		"router_node_id": h.router.Identity.NodeID,
		"router_pubkey":  h.router.Identity.PublicKeyHex(),
		"registry":       stats,
		"timestamp":      time.Now().Unix(),
	})
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
