package v1

import (
	"io"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw/internal/inference"
)

// InferenceHandler exposes the inference router API.
type InferenceHandler struct {
	router *inference.InferenceRouter
}

// NewInferenceHandler creates a new handler wrapping the inference router.
func NewInferenceHandler(router *inference.InferenceRouter) *InferenceHandler {
	return &InferenceHandler{router: router}
}

// RegisterMiner handles miner registration (POST /v1/inference/register).
// Called by miner nodes (protected by NodeSignatureAuth middleware).
func (h *InferenceHandler) RegisterMiner(c *gin.Context) {
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

	h.router.Registry.Register(&inference.MinerInfo{
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
		"message":  "miner registered",
		"node_id":  nodeID,
		"models":   req.Models,
		"max_jobs": req.MaxJobs,
	})
}

// Heartbeat handles miner heartbeat (POST /v1/inference/heartbeat).
// Called periodically by miner nodes (protected by NodeSignatureAuth middleware).
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
		c.JSON(http.StatusNotFound, gin.H{"error": "miner not registered"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

// UnregisterMiner handles miner deregistration (POST /v1/inference/unregister).
func (h *InferenceHandler) UnregisterMiner(c *gin.Context) {
	nodeID := c.GetString("node_id")
	if nodeID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing node identity"})
		return
	}
	h.router.Registry.Unregister(nodeID)
	c.JSON(http.StatusOK, gin.H{"message": "miner unregistered"})
}

// ListMiners returns all registered miners (GET /v1/inference/miners).
func (h *InferenceHandler) ListMiners(c *gin.Context) {
	miners := h.router.Registry.ListMiners()
	stats := h.router.Registry.Stats()
	c.JSON(http.StatusOK, gin.H{
		"miners": miners,
		"stats":  stats,
	})
}

// Infer routes an inference request to the best miner (POST /v1/inference/completions).
// Supports both streaming (SSE) and non-streaming responses.
func (h *InferenceHandler) Infer(c *gin.Context) {
	var req inference.InferenceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, miner, err := h.router.Route(c.Request.Context(), &req)
	if err != nil {
		status := http.StatusServiceUnavailable
		c.JSON(status, gin.H{
			"error":   err.Error(),
			"details": "no miner available or miner unreachable",
		})
		return
	}
	defer resp.Body.Close()

	// Add routing metadata headers
	minerID := ""
	if miner != nil {
		minerID = miner.NodeID
	}
	c.Header("X-Routed-To", minerID)
	c.Header("X-Router-ID", h.router.Identity.NodeID)

	if req.Stream {
		// SSE streaming proxy: forward miner's SSE stream to client
		h.proxySSEStream(c, resp, minerID)
	} else {
		// Non-streaming: read full response and forward
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "failed to read miner response"})
			return
		}
		c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), body)
	}

	// Update miner: decrement active jobs
	if miner != nil {
		h.router.Registry.Heartbeat(miner.NodeID, max(0, miner.ActiveJobs-1), 0)
	}
}

// proxySSEStream forwards an SSE stream from the miner to the client.
func (h *InferenceHandler) proxySSEStream(c *gin.Context, resp *http.Response, minerID string) {
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
				log.Printf("[inference/router] SSE proxy error from miner %s: %v", minerID, err)
			}
			return
		}
	}
}

// Execute handles inference execution on this node (POST /v1/inference/execute).
// This is the miner-side endpoint called by the router.
// Protected by NodeSignatureAuth — only trusted routers can invoke this.
func (h *InferenceHandler) Execute(c *gin.Context) {
	// This endpoint will be implemented by the miner (Phase 6: 推理挖矿).
	// For now, return a placeholder acknowledging the signed request.
	routerNodeID := c.GetString("node_id")
	c.JSON(http.StatusOK, gin.H{
		"message":        "inference execution endpoint (miner-side, Phase 6)",
		"router_node_id": routerNodeID,
		"status":         "not_implemented",
	})
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
