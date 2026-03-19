package handler

import (
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw-router/internal/provider"
)

// ProviderProxyHandler forwards requests to sub-providers with injected API keys.
// This is the core of StarAI's "super router" — Claw tools send requests here
// and the Router handles auth + key injection + forwarding.
//
// Usage: ANY /v1/proxy/:provider/*path
//
//	e.g. POST /v1/proxy/fal/fal-ai/veo3         → https://queue.fal.run/fal-ai/veo3
//	     POST /v1/proxy/qwen/services/aigc/...   → https://dashscope.aliyuncs.com/compatible-mode/v1/services/aigc/...
//	     POST /v1/proxy/minimax/music_generation  → https://api.minimaxi.com/v1/music_generation
type ProviderProxyHandler struct {
	registry *provider.Registry
	genH     *GenerationHandler
}

func NewProviderProxyHandler(reg *provider.Registry) *ProviderProxyHandler {
	return &ProviderProxyHandler{registry: reg}
}

// SetGenerationHandler enables generation tracking for video/image proxy requests.
func (h *ProviderProxyHandler) SetGenerationHandler(gh *GenerationHandler) {
	h.genH = gh
}

func (h *ProviderProxyHandler) Forward(c *gin.Context) {
	provSlug := c.Param("provider")
	subPath := c.Param("path") // e.g. "/fal-ai/veo3" or "/music_generation"

	// Intercept video generation requests for tracking
	if h.genH != nil && c.Request.Method == "POST" {
		if provSlug == "dashscope" && strings.Contains(subPath, "video-generation") {
			body, _ := io.ReadAll(c.Request.Body)
			h.genH.ProxyDashScopeVideo(c, provSlug, subPath, body)
			return
		}
		if provSlug == "fal" && isVideoPath(subPath) {
			body, _ := io.ReadAll(c.Request.Body)
			h.genH.ProxyFalVideo(c, provSlug, subPath, body)
			return
		}
	}

	// Look up provider config
	prov, ok := h.registry.GetProvider(provSlug)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"message": "unknown provider: " + provSlug,
				"type":    "invalid_request_error",
			},
		})
		return
	}

	// Build upstream URL
	// Provider endpoint already has base path, subPath is the rest
	// e.g. endpoint="https://queue.fal.run" + path="/fal-ai/veo3"
	// e.g. endpoint="https://api.minimaxi.com/v1" + path="/music_generation"
	upstreamURL := strings.TrimRight(prov.Endpoint, "/") + subPath

	// Read request body
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{"message": "failed to read body", "type": "invalid_request_error"},
		})
		return
	}

	// Create upstream request
	upstreamReq, err := http.NewRequestWithContext(c.Request.Context(), c.Request.Method, upstreamURL, strings.NewReader(string(body)))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{"message": "failed to create upstream request", "type": "server_error"},
		})
		return
	}

	// Copy relevant headers from original request
	for _, h := range []string{"Content-Type", "Accept", "X-Request-Id", "X-DashScope-Async", "X-DashScope-OssResourceResolve"} {
		if v := c.GetHeader(h); v != "" {
			upstreamReq.Header.Set(h, v)
		}
	}
	if upstreamReq.Header.Get("Content-Type") == "" {
		upstreamReq.Header.Set("Content-Type", "application/json")
	}

	// Inject provider API key based on auth type
	apiKey := h.registry.GetAPIKey(provSlug)
	switch prov.Auth.Type {
	case "bearer":
		upstreamReq.Header.Set("Authorization", "Bearer "+apiKey)
	case "key":
		// fal.ai style: "Authorization: Key xxx"
		upstreamReq.Header.Set("Authorization", "Key "+apiKey)
	case "x-api-key":
		upstreamReq.Header.Set("X-Api-Key", apiKey)
	default:
		upstreamReq.Header.Set("Authorization", "Bearer "+apiKey)
	}

	log.Printf("[star-ai] proxy → %s %s (provider=%s)", c.Request.Method, upstreamURL, provSlug)

	// Forward request
	client := &http.Client{Timeout: 5 * time.Minute} // longer timeout for video/music gen
	resp, err := client.Do(upstreamReq)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"error": gin.H{
				"message": "upstream provider unreachable: " + err.Error(),
				"type":    "server_error",
			},
		})
		return
	}
	defer resp.Body.Close()

	// Stream response back to client
	for k, vals := range resp.Header {
		for _, v := range vals {
			c.Writer.Header().Add(k, v)
		}
	}
	c.Writer.WriteHeader(resp.StatusCode)

	// Stream body
	if f, ok := c.Writer.(http.Flusher); ok {
		buf := make([]byte, 8192)
		for {
			n, err := resp.Body.Read(buf)
			if n > 0 {
				c.Writer.Write(buf[:n])
				f.Flush()
			}
			if err != nil {
				break
			}
		}
	} else {
		io.Copy(c.Writer, resp.Body)
	}
}

// ForwardGET handles GET requests (e.g. fal.ai async status polling)
func (h *ProviderProxyHandler) ForwardGET(c *gin.Context) {
	h.Forward(c)
}

// isVideoPath checks if a fal.ai proxy path is a video generation endpoint
func isVideoPath(path string) bool {
	videoKeywords := []string{"veo3", "sora-2", "kling-video", "minimax-video", "luma-dream-machine"}
	for _, kw := range videoKeywords {
		if strings.Contains(path, kw) {
			return true
		}
	}
	return false
}
