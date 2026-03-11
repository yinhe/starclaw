package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw-router/internal/billing"
	"github.com/yinhe/starclaw-router/internal/model"
	"github.com/yinhe/starclaw-router/internal/provider"
	"github.com/yinhe/starclaw-router/internal/proxy"
	"gorm.io/gorm"
)

type ChatHandler struct {
	db       *gorm.DB
	proxy    *proxy.Client
	registry *provider.Registry
	meter    *billing.Meter
}

func NewChatHandler(db *gorm.DB, proxyClient *proxy.Client, reg *provider.Registry, meter *billing.Meter) *ChatHandler {
	return &ChatHandler{db: db, proxy: proxyClient, registry: reg, meter: meter}
}

// ChatCompletions handles POST /v1/chat/completions
// Routes domestic models directly, overseas models via proxy
func (h *ChatHandler) ChatCompletions(c *gin.Context) {
	start := time.Now()
	userID := c.GetString("user_id")
	apiKeyID := c.GetString("api_key_id")

	// Balance check
	if err := h.meter.CheckBalance(userID); err != nil {
		c.JSON(http.StatusPaymentRequired, openAIError("insufficient balance", "billing_error"))
		return
	}

	// Read body
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, openAIError("invalid request body", "invalid_request_error"))
		return
	}

	// Parse model field to determine routing
	var req struct {
		Model  string `json:"model"`
		Stream bool   `json:"stream"`
	}
	if err := json.Unmarshal(bodyBytes, &req); err != nil {
		c.JSON(http.StatusBadRequest, openAIError("invalid JSON", "invalid_request_error"))
		return
	}

	if req.Model == "" {
		c.JSON(http.StatusBadRequest, openAIError("model is required", "invalid_request_error"))
		return
	}

	// Parse provider from model: "qwen/qwen-max" → provider="qwen", model="qwen-max"
	provSlug, modelName := parseModelName(req.Model)

	// Validate model exists in registry
	if _, ok := h.registry.GetModel(req.Model); !ok {
		// Allow unknown models through (might be new), just log
		log.Printf("[star-ai] unknown model: %s (provider=%s)", req.Model, provSlug)
	}

	via := "direct"

	if provider.IsDomestic(provSlug) {
		h.forwardDomestic(c, provSlug, modelName, bodyBytes, req.Stream)
	} else if _, ok := h.registry.GetProvider(provSlug); ok {
		via = "proxy"
		// Proxy expects raw provider model name (e.g. gpt-4o-mini), not "openai/gpt-4o-mini"
		var bodyMap map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &bodyMap); err == nil {
			bodyMap["model"] = modelName
			if rewritten, err := json.Marshal(bodyMap); err == nil {
				bodyBytes = rewritten
			}
		}
		h.forwardToProxy(c, "/chat", bodyBytes)
	} else {
		c.JSON(http.StatusBadRequest, openAIError(
			fmt.Sprintf("unknown provider: %s", provSlug),
			"invalid_request_error",
		))
		return
	}

	// Record usage + billing (async)
	go h.recordAndBill(userID, apiKeyID, provSlug, req.Model, "/v1/chat/completions", via, time.Since(start))
}

// Embeddings handles POST /v1/embeddings
func (h *ChatHandler) Embeddings(c *gin.Context) {
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, openAIError("invalid request body", "invalid_request_error"))
		return
	}

	var req struct {
		Model string `json:"model"`
	}
	json.Unmarshal(bodyBytes, &req)

	provSlug, modelName := parseModelName(req.Model)

	if provider.IsDomestic(provSlug) {
		h.forwardDomestic(c, provSlug, modelName, bodyBytes, false)
	} else {
		h.forwardToProxy(c, "/v1/embeddings", bodyBytes)
	}
}

// forwardDomestic sends request directly to domestic LLM provider
func (h *ChatHandler) forwardDomestic(c *gin.Context, provSlug, modelName string, body []byte, stream bool) {
	prov, ok := h.registry.GetProvider(provSlug)
	if !ok {
		c.JSON(http.StatusBadRequest, openAIError("unsupported domestic provider", "invalid_request_error"))
		return
	}
	baseURL := prov.Endpoint

	// Rewrite model name in body (strip provider prefix)
	var bodyMap map[string]interface{}
	json.Unmarshal(body, &bodyMap)
	bodyMap["model"] = modelName
	rewritten, _ := json.Marshal(bodyMap)

	url := baseURL + "/chat/completions"
	if strings.Contains(c.Request.URL.Path, "embeddings") {
		url = baseURL + "/embeddings"
	}

	upstreamReq, err := http.NewRequestWithContext(c.Request.Context(), "POST", url, bytes.NewReader(rewritten))
	if err != nil {
		c.JSON(http.StatusInternalServerError, openAIError("failed to create upstream request", "server_error"))
		return
	}

	apiKey := h.registry.GetAPIKey(provSlug)
	upstreamReq.Header.Set("Authorization", "Bearer "+apiKey)
	upstreamReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 3 * time.Minute}
	resp, err := client.Do(upstreamReq)
	if err != nil {
		c.JSON(http.StatusBadGateway, openAIError("upstream provider unreachable: "+err.Error(), "server_error"))
		return
	}
	defer resp.Body.Close()

	// Stream response headers back to client
	for k, vals := range resp.Header {
		for _, v := range vals {
			c.Writer.Header().Add(k, v)
		}
	}
	c.Writer.WriteHeader(resp.StatusCode)

	if stream {
		// Stream SSE chunks
		if f, ok := c.Writer.(http.Flusher); ok {
			buf := make([]byte, 4096)
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
		}
	} else {
		io.Copy(c.Writer, resp.Body)
	}
}

// forwardToProxy sends request to the Node.js overseas relay proxy
func (h *ChatHandler) forwardToProxy(c *gin.Context, path string, body []byte) {
	headers := http.Header{}
	headers.Set("Content-Type", "application/json")

	err := h.proxy.ForwardStream(c.Writer, "POST", path, bytes.NewReader(body), headers)
	if err != nil {
		log.Printf("[star-ai] proxy forward error: %v", err)
		c.JSON(http.StatusBadGateway, openAIError("proxy unreachable: "+err.Error(), "server_error"))
	}
}

func (h *ChatHandler) recordAndBill(userID, apiKeyID, provSlug, modelName, endpoint, via string, duration time.Duration) {
	// Estimate tokens (TODO: parse from upstream response for accuracy)
	estPrompt := 500
	estCompletion := 200

	costCents, upstreamCents := h.meter.CalculateCost(modelName, estPrompt, estCompletion)

	record := &model.UsageRecord{
		UserID:           userID,
		APIKeyID:         apiKeyID,
		Provider:         provSlug,
		Model:            modelName,
		Endpoint:         endpoint,
		PromptTokens:     estPrompt,
		CompletionTokens: estCompletion,
		TotalTokens:      estPrompt + estCompletion,
		Duration:         int(duration.Milliseconds()),
		Via:              via,
		Status:           "ok",
	}

	if costCents > 0 {
		if err := h.meter.Deduct(userID, costCents, upstreamCents, record); err != nil {
			log.Printf("[star-ai] billing deduct failed: %v", err)
			// Still record usage even if billing fails
			record.CostCents = costCents
			record.UpstreamCost = upstreamCents
			h.db.Create(record)
		}
	} else {
		h.db.Create(record)
	}
}

// parseModelName splits "provider/model" into (provider, model)
// e.g. "qwen/qwen-max" → ("qwen", "qwen-max")
// e.g. "gpt-4o" → ("openai", "gpt-4o") (fallback)
func parseModelName(model string) (string, string) {
	if idx := strings.Index(model, "/"); idx > 0 {
		return model[:idx], model[idx+1:]
	}
	// Heuristic fallback for raw model names
	lower := strings.ToLower(model)
	switch {
	case strings.HasPrefix(lower, "qwen"):
		return "qwen", model
	case strings.HasPrefix(lower, "deepseek"):
		return "deepseek", model
	case strings.HasPrefix(lower, "gpt") || strings.HasPrefix(lower, "o1") || strings.HasPrefix(lower, "o3") || strings.HasPrefix(lower, "o4"):
		return "openai", model
	case strings.HasPrefix(lower, "claude"):
		return "anthropic", model
	case strings.HasPrefix(lower, "gemini"):
		return "google", model
	case strings.HasPrefix(lower, "grok"):
		return "grok", model
	default:
		return "openai", model // default fallback
	}
}

func openAIError(msg, errType string) gin.H {
	return gin.H{
		"error": gin.H{
			"message": msg,
			"type":    errType,
		},
	}
}
