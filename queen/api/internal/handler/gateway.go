package handler

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"starclaw.net/queen/api/internal/config"
	"starclaw.net/queen/api/internal/database"
	"starclaw.net/queen/api/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// GatewayHandler handles star-ai.net OpenAI-compatible API gateway
type GatewayHandler struct {
	httpClient *http.Client
}

func NewGatewayHandler() *GatewayHandler {
	return &GatewayHandler{
		httpClient: &http.Client{Timeout: 5 * time.Minute},
	}
}

// ==================== API Key Management ====================

// POST /v1/api-keys — create a new API key
func (h *GatewayHandler) CreateKey(c *gin.Context) {
	userID := c.GetString("user_id")
	var req struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}

	// Limit: max 5 keys per user
	var count int64
	database.DB.Model(&model.APIKey{}).Where("user_id = ?", userID).Count(&count)
	if count >= 5 {
		c.JSON(http.StatusConflict, gin.H{"error": "最多创建 5 个 API Key"})
		return
	}

	key := generateAPIKey()
	apiKey := model.APIKey{
		ID:        uuid.New().String(),
		UserID:    userID,
		Name:      req.Name,
		Key:       key,
		KeyPrefix: key[:8] + "...",
		Enabled:   true,
		RateLimit: 60,
	}
	if err := database.DB.Create(&apiKey).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建失败"})
		return
	}

	// Return the full key only on creation (never shown again)
	c.JSON(http.StatusOK, gin.H{
		"id":         apiKey.ID,
		"name":       apiKey.Name,
		"key":        key,
		"key_prefix": apiKey.KeyPrefix,
		"created_at": apiKey.CreatedAt,
	})
}

// GET /v1/api-keys — list user's API keys
func (h *GatewayHandler) ListKeys(c *gin.Context) {
	userID := c.GetString("user_id")
	var keys []model.APIKey
	database.DB.Where("user_id = ?", userID).Order("created_at DESC").Find(&keys)
	c.JSON(http.StatusOK, gin.H{"keys": keys})
}

// DELETE /v1/api-keys/:id — delete an API key
func (h *GatewayHandler) DeleteKey(c *gin.Context) {
	userID := c.GetString("user_id")
	id := c.Param("id")
	result := database.DB.Where("id = ? AND user_id = ?", id, userID).Delete(&model.APIKey{})
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "key not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

// GET /v1/api-keys/usage — usage stats for the user
func (h *GatewayHandler) Usage(c *gin.Context) {
	userID := c.GetString("user_id")

	// Total stats
	var totalReqs int64
	var totalTokens int64
	database.DB.Model(&model.APIKey{}).Where("user_id = ?", userID).
		Select("COALESCE(SUM(total_requests), 0)").Scan(&totalReqs)
	database.DB.Model(&model.APIKey{}).Where("user_id = ?", userID).
		Select("COALESCE(SUM(total_tokens), 0)").Scan(&totalTokens)

	// Recent logs (last 50)
	var logs []model.GatewayUsageLog
	database.DB.Where("user_id = ?", userID).Order("created_at DESC").Limit(50).Find(&logs)

	c.JSON(http.StatusOK, gin.H{
		"total_requests": totalReqs,
		"total_tokens":   totalTokens,
		"recent_logs":    logs,
	})
}

// ==================== OpenAI-Compatible Gateway ====================

// POST /v1/chat/completions — OpenAI-compatible proxy endpoint
func (h *GatewayHandler) ChatCompletions(c *gin.Context) {
	// 1. Authenticate via API key
	apiKey, userID, err := h.authenticateKey(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{
			"message": err.Error(),
			"type":    "authentication_error",
		}})
		return
	}

	// 2. Check balance
	bal := ensureBalance(userID)
	if bal.Balance <= 0 {
		c.JSON(http.StatusPaymentRequired, gin.H{"error": gin.H{
			"message": "余额不足，请充值后使用",
			"type":    "insufficient_balance",
		}})
		return
	}

	// 3. Parse request
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid request body"}})
		return
	}

	var req gatewayRequest
	if err := json.Unmarshal(bodyBytes, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid JSON"}})
		return
	}

	if req.Model == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "model is required"}})
		return
	}

	// 4. Resolve upstream provider
	providerName, upstreamURL, upstreamKey := h.resolveProvider(req.Model)
	if upstreamKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{
			"message": fmt.Sprintf("model %q is not supported", req.Model),
			"type":    "invalid_request_error",
		}})
		return
	}

	start := time.Now()

	// 5. Proxy to upstream
	if req.Stream {
		h.proxyStream(c, apiKey, userID, providerName, upstreamURL, upstreamKey, bodyBytes, req.Model, start)
	} else {
		h.proxySync(c, apiKey, userID, providerName, upstreamURL, upstreamKey, bodyBytes, req.Model, start)
	}
}

// GET /v1/models — list available models
func (h *GatewayHandler) ListModels(c *gin.Context) {
	models := []gin.H{}
	for _, m := range supportedModels {
		models = append(models, gin.H{
			"id":       m.ID,
			"object":   "model",
			"provider": m.Provider,
			"owned_by": m.Provider,
		})
	}
	c.JSON(http.StatusOK, gin.H{"object": "list", "data": models})
}

// ==================== Proxy Logic ====================

func (h *GatewayHandler) proxySync(c *gin.Context, apiKey *model.APIKey, userID, providerName, upstreamURL, upstreamKey string, body []byte, modelName string, start time.Time) {
	resp, err := h.doUpstreamRequest(providerName, upstreamURL, upstreamKey, body, false)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": gin.H{"message": "upstream error: " + err.Error()}})
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	// Extract usage from response
	var parsed struct {
		Usage *struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}
	json.Unmarshal(respBody, &parsed)

	promptTokens, completionTokens, totalTokens := 0, 0, 0
	if parsed.Usage != nil {
		promptTokens = parsed.Usage.PromptTokens
		completionTokens = parsed.Usage.CompletionTokens
		totalTokens = parsed.Usage.TotalTokens
	}

	// Bill and log
	cost := h.calculateAndBill(userID, modelName, promptTokens, completionTokens, totalTokens)
	h.logUsage(apiKey, userID, modelName, providerName, promptTokens, completionTokens, totalTokens, cost, time.Since(start).Milliseconds(), resp.StatusCode)

	// Forward response as-is
	c.Data(resp.StatusCode, "application/json", respBody)
}

func (h *GatewayHandler) proxyStream(c *gin.Context, apiKey *model.APIKey, userID, providerName, upstreamURL, upstreamKey string, body []byte, modelName string, start time.Time) {
	resp, err := h.doUpstreamRequest(providerName, upstreamURL, upstreamKey, body, true)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": gin.H{"message": "upstream error: " + err.Error()}})
		return
	}
	defer resp.Body.Close()

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Transfer-Encoding", "chunked")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "streaming not supported"})
		return
	}

	promptTokens, completionTokens, totalTokens := 0, 0, 0
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 256*1024), 256*1024)

	for scanner.Scan() {
		line := scanner.Text()

		// Parse SSE for usage extraction
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			if data != "[DONE]" {
				var chunk struct {
					Usage *struct {
						PromptTokens     int `json:"prompt_tokens"`
						CompletionTokens int `json:"completion_tokens"`
						TotalTokens      int `json:"total_tokens"`
					} `json:"usage"`
				}
				if json.Unmarshal([]byte(data), &chunk) == nil && chunk.Usage != nil {
					promptTokens = chunk.Usage.PromptTokens
					completionTokens = chunk.Usage.CompletionTokens
					totalTokens = chunk.Usage.TotalTokens
				}
			}
		}

		// Forward line to client
		fmt.Fprintf(c.Writer, "%s\n", line)
		flusher.Flush()
	}

	// Bill and log after stream ends
	cost := h.calculateAndBill(userID, modelName, promptTokens, completionTokens, totalTokens)
	h.logUsage(apiKey, userID, modelName, providerName, promptTokens, completionTokens, totalTokens, cost, time.Since(start).Milliseconds(), resp.StatusCode)
}

func (h *GatewayHandler) doUpstreamRequest(providerName, upstreamURL, upstreamKey string, body []byte, stream bool) (*http.Response, error) {
	if providerName == "anthropic" {
		return h.doAnthropicRequest(upstreamURL, upstreamKey, body, stream)
	}

	// OpenAI-compatible providers (openai, deepseek, qwen, gemini)
	req, err := http.NewRequest("POST", upstreamURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+upstreamKey)

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("upstream unreachable: %w", err)
	}
	return resp, nil
}

// doAnthropicRequest converts OpenAI format → Anthropic Messages API → OpenAI format response
func (h *GatewayHandler) doAnthropicRequest(upstreamURL, upstreamKey string, body []byte, stream bool) (*http.Response, error) {
	// Parse the OpenAI-format request
	var oaiReq struct {
		Model       string       `json:"model"`
		Messages    []oaiMessage `json:"messages"`
		MaxTokens   int          `json:"max_tokens"`
		Temperature float64      `json:"temperature"`
		Stream      bool         `json:"stream"`
	}
	if err := json.Unmarshal(body, &oaiReq); err != nil {
		return nil, fmt.Errorf("failed to parse request: %w", err)
	}

	// Convert messages: extract system, convert roles
	var systemPrompt string
	var anthropicMsgs []anthropicMsg
	for _, m := range oaiReq.Messages {
		if m.Role == "system" {
			systemPrompt = fmt.Sprintf("%v", m.Content)
			continue
		}
		role := m.Role
		if role == "tool" {
			role = "user"
		}
		content := fmt.Sprintf("%v", m.Content)
		anthropicMsgs = append(anthropicMsgs, anthropicMsg{Role: role, Content: content})
	}

	maxTokens := oaiReq.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	anthropicReq := map[string]interface{}{
		"model":      oaiReq.Model,
		"messages":   anthropicMsgs,
		"max_tokens": maxTokens,
		"stream":     stream,
	}
	if systemPrompt != "" {
		anthropicReq["system"] = systemPrompt
	}
	if oaiReq.Temperature > 0 {
		anthropicReq["temperature"] = oaiReq.Temperature
	}

	reqBody, _ := json.Marshal(anthropicReq)
	req, err := http.NewRequest("POST", upstreamURL+"/v1/messages", bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", upstreamKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("anthropic unreachable: %w", err)
	}

	if !stream {
		// Convert Anthropic response → OpenAI format
		return h.convertAnthropicSyncResponse(resp, oaiReq.Model)
	}

	// For streaming, wrap in a converter pipe
	return h.convertAnthropicStreamResponse(resp, oaiReq.Model)
}

type oaiMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

type anthropicMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func (h *GatewayHandler) convertAnthropicSyncResponse(resp *http.Response, model string) (*http.Response, error) {
	defer resp.Body.Close()
	rawBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		// Return error as-is wrapped in OpenAI error format
		oaiErr, _ := json.Marshal(map[string]interface{}{
			"error": map[string]interface{}{
				"message": string(rawBody),
				"type":    "upstream_error",
			},
		})
		return &http.Response{
			StatusCode: resp.StatusCode,
			Body:       io.NopCloser(bytes.NewReader(oaiErr)),
			Header:     http.Header{"Content-Type": {"application/json"}},
		}, nil
	}

	var anthResp struct {
		ID      string `json:"id"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Usage *struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	json.Unmarshal(rawBody, &anthResp)

	content := ""
	for _, c := range anthResp.Content {
		if c.Type == "text" {
			content += c.Text
		}
	}

	oaiResp := map[string]interface{}{
		"id":     anthResp.ID,
		"object": "chat.completion",
		"model":  model,
		"choices": []map[string]interface{}{{
			"index":         0,
			"message":       map[string]string{"role": "assistant", "content": content},
			"finish_reason": "stop",
		}},
	}
	if anthResp.Usage != nil {
		oaiResp["usage"] = map[string]int{
			"prompt_tokens":     anthResp.Usage.InputTokens,
			"completion_tokens": anthResp.Usage.OutputTokens,
			"total_tokens":      anthResp.Usage.InputTokens + anthResp.Usage.OutputTokens,
		}
	}

	oaiBody, _ := json.Marshal(oaiResp)
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewReader(oaiBody)),
		Header:     http.Header{"Content-Type": {"application/json"}},
	}, nil
}

func (h *GatewayHandler) convertAnthropicStreamResponse(resp *http.Response, model string) (*http.Response, error) {
	if resp.StatusCode != http.StatusOK {
		return resp, nil
	}

	pr, pw := io.Pipe()

	go func() {
		defer pw.Close()
		defer resp.Body.Close()
		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 256*1024), 256*1024)

		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")

			var event struct {
				Type  string `json:"type"`
				Delta *struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"delta"`
				Usage *struct {
					InputTokens  int `json:"input_tokens"`
					OutputTokens int `json:"output_tokens"`
				} `json:"usage"`
			}
			if json.Unmarshal([]byte(data), &event) != nil {
				continue
			}

			switch event.Type {
			case "content_block_delta":
				if event.Delta != nil && event.Delta.Type == "text_delta" {
					chunk := map[string]interface{}{
						"id":     "chatcmpl-" + model,
						"object": "chat.completion.chunk",
						"model":  model,
						"choices": []map[string]interface{}{{
							"index": 0,
							"delta": map[string]string{"content": event.Delta.Text},
						}},
					}
					b, _ := json.Marshal(chunk)
					fmt.Fprintf(pw, "data: %s\n\n", b)
				}
			case "message_stop":
				chunk := map[string]interface{}{
					"id":     "chatcmpl-" + model,
					"object": "chat.completion.chunk",
					"model":  model,
					"choices": []map[string]interface{}{{
						"index":         0,
						"delta":         map[string]string{},
						"finish_reason": "stop",
					}},
				}
				if event.Usage != nil {
					chunk["usage"] = map[string]int{
						"prompt_tokens":     event.Usage.InputTokens,
						"completion_tokens": event.Usage.OutputTokens,
						"total_tokens":      event.Usage.InputTokens + event.Usage.OutputTokens,
					}
				}
				b, _ := json.Marshal(chunk)
				fmt.Fprintf(pw, "data: %s\n\n", b)
				fmt.Fprintf(pw, "data: [DONE]\n\n")
			case "message_delta":
				// message_delta often carries final usage
				if event.Usage != nil {
					chunk := map[string]interface{}{
						"id":      "chatcmpl-" + model,
						"object":  "chat.completion.chunk",
						"model":   model,
						"choices": []map[string]interface{}{},
						"usage": map[string]int{
							"prompt_tokens":     event.Usage.InputTokens,
							"completion_tokens": event.Usage.OutputTokens,
							"total_tokens":      event.Usage.InputTokens + event.Usage.OutputTokens,
						},
					}
					b, _ := json.Marshal(chunk)
					fmt.Fprintf(pw, "data: %s\n\n", b)
				}
			}
		}
	}()

	return &http.Response{
		StatusCode: 200,
		Body:       pr,
		Header:     http.Header{"Content-Type": {"text/event-stream"}},
	}, nil
}

// ==================== Auth ====================

func (h *GatewayHandler) authenticateKey(c *gin.Context) (*model.APIKey, string, error) {
	auth := c.GetHeader("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return nil, "", fmt.Errorf("missing Authorization header")
	}
	key := strings.TrimPrefix(auth, "Bearer ")
	if key == "" || len(key) < 10 {
		return nil, "", fmt.Errorf("invalid API key")
	}

	var apiKey model.APIKey
	if err := database.DB.Where("`key` = ? AND enabled = ?", key, true).First(&apiKey).Error; err != nil {
		return nil, "", fmt.Errorf("invalid or disabled API key")
	}

	// Update last used
	now := time.Now()
	database.DB.Model(&apiKey).Updates(map[string]interface{}{
		"last_used_at":   now,
		"total_requests": gorm.Expr("total_requests + 1"),
	})

	return &apiKey, apiKey.UserID, nil
}

// ==================== Billing ====================

func (h *GatewayHandler) calculateAndBill(userID, modelName string, prompt, completion, total int) int64 {
	if total == 0 {
		return 0
	}

	// Pricing per 1M tokens (in 分)
	pricing := getModelPricing(modelName)
	cost := int64(float64(prompt)*pricing.InputPer1M/1_000_000 + float64(completion)*pricing.OutputPer1M/1_000_000)
	if cost < 1 && total > 0 {
		cost = 1 // minimum 1 分
	}

	// Deduct balance
	db := database.DB
	db.Transaction(func(tx *gorm.DB) error {
		var bal model.UserBalance
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ?", userID).First(&bal).Error; err != nil {
			return nil
		}
		if bal.Balance < cost {
			// Allow overdraft for in-flight requests, but log it
			log.Printf("[gateway] overdraft: user=%s, balance=%d, cost=%d", userID, bal.Balance, cost)
		}
		before := bal.Balance
		bal.Balance -= cost
		bal.TotalOut += cost
		tx.Save(&bal)

		tx.Create(&model.BalanceTransaction{
			ID:     uuid.New().String(),
			UserID: userID,
			Type:   "gateway_consume",
			Amount: -cost,
			Before: before,
			After:  bal.Balance,
			Remark: fmt.Sprintf("star-ai gateway: %s (%d tokens)", modelName, prompt+completion),
		})
		return nil
	})

	return cost
}

func (h *GatewayHandler) logUsage(apiKey *model.APIKey, userID, modelName, providerName string, prompt, completion, total int, cost, durationMs int64, statusCode int) {
	// Update key stats
	database.DB.Model(apiKey).Update("total_tokens", gorm.Expr("total_tokens + ?", total))

	// Insert usage log
	database.DB.Create(&model.GatewayUsageLog{
		ID:               uuid.New().String(),
		APIKeyID:         apiKey.ID,
		UserID:           userID,
		Model:            modelName,
		Provider:         providerName,
		PromptTokens:     prompt,
		CompletionTokens: completion,
		TotalTokens:      total,
		CostFen:          cost,
		DurationMs:       durationMs,
		StatusCode:       statusCode,
	})
}

// ==================== Provider Routing ====================

type modelEntry struct {
	ID       string
	Provider string
}

type modelPricing struct {
	InputPer1M  float64 // 分 per 1M input tokens
	OutputPer1M float64 // 分 per 1M output tokens
}

var supportedModels = []modelEntry{
	// ── OpenAI: Reasoning ──
	{ID: "o3", Provider: "openai"},
	{ID: "o3-mini", Provider: "openai"},
	{ID: "o3-pro", Provider: "openai"},
	{ID: "o4-mini", Provider: "openai"},
	{ID: "o1", Provider: "openai"},
	{ID: "o1-mini", Provider: "openai"},
	{ID: "o1-pro", Provider: "openai"},
	// ── OpenAI: GPT-4.1 ──
	{ID: "gpt-4.1", Provider: "openai"},
	{ID: "gpt-4.1-mini", Provider: "openai"},
	{ID: "gpt-4.1-nano", Provider: "openai"},
	// ── OpenAI: GPT-4o ──
	{ID: "gpt-4o", Provider: "openai"},
	{ID: "gpt-4o-mini", Provider: "openai"},
	{ID: "gpt-4o-search-preview", Provider: "openai"},
	{ID: "gpt-4o-mini-search-preview", Provider: "openai"},
	{ID: "chatgpt-4o-latest", Provider: "openai"},
	// ── OpenAI: Legacy ──
	{ID: "gpt-4-turbo", Provider: "openai"},
	{ID: "gpt-4", Provider: "openai"},
	{ID: "gpt-3.5-turbo", Provider: "openai"},
	// ── OpenAI: Codex ──
	{ID: "codex-mini-latest", Provider: "openai"},
	// ── Anthropic ──
	{ID: "claude-sonnet-4-20250514", Provider: "anthropic"},
	{ID: "claude-3-7-sonnet-20250219", Provider: "anthropic"},
	{ID: "claude-3-5-sonnet-20241022", Provider: "anthropic"},
	{ID: "claude-3-5-haiku-20241022", Provider: "anthropic"},
	{ID: "claude-3-opus-20240229", Provider: "anthropic"},
	// ── DeepSeek ──
	{ID: "deepseek-chat", Provider: "deepseek"},
	{ID: "deepseek-reasoner", Provider: "deepseek"},
	// ── Qwen: 核心系列 ──
	{ID: "qwen3.5-plus", Provider: "qwen"},
	{ID: "qwen3-max", Provider: "qwen"},
	{ID: "qwen-max", Provider: "qwen"},
	{ID: "qwen-plus", Provider: "qwen"},
	{ID: "qwen-turbo", Provider: "qwen"},
	{ID: "qwen-flash", Provider: "qwen"},
	{ID: "qwen-long", Provider: "qwen"},
	// ── Qwen: 推理 (QwQ) ──
	{ID: "qwq-plus", Provider: "qwen"},
	{ID: "qwq-max", Provider: "qwen"},
	{ID: "qwq-32b", Provider: "qwen"},
	// ── Qwen: 视觉 ──
	{ID: "qwen3-vl-plus", Provider: "qwen"},
	{ID: "qwen3-vl-flash", Provider: "qwen"},
	{ID: "qwen-vl-max", Provider: "qwen"},
	{ID: "qwen-vl-plus", Provider: "qwen"},
	// ── Qwen: 编程 ──
	{ID: "qwen3-coder-plus", Provider: "qwen"},
	{ID: "qwen3-coder-flash", Provider: "qwen"},
	{ID: "qwen-coder-plus", Provider: "qwen"},
	{ID: "qwen-coder-turbo", Provider: "qwen"},
	// ── Qwen: 数学 ──
	{ID: "qwen-math-plus", Provider: "qwen"},
	{ID: "qwen-math-turbo", Provider: "qwen"},
	// ── Qwen: 全模态 ──
	{ID: "qwen3-omni-flash", Provider: "qwen"},
	{ID: "qwen-omni-turbo", Provider: "qwen"},
	// ── Qwen: 其他 ──
	{ID: "qwen-deep-research", Provider: "qwen"},
	// ── Gemini ──
	{ID: "gemini-2.5-pro", Provider: "gemini"},
	{ID: "gemini-2.5-flash", Provider: "gemini"},
	{ID: "gemini-2.0-flash", Provider: "gemini"},
	{ID: "gemini-2.0-flash-lite", Provider: "gemini"},
	{ID: "gemini-1.5-pro", Provider: "gemini"},
	{ID: "gemini-1.5-flash", Provider: "gemini"},
	// ── Grok ──
	{ID: "grok-3", Provider: "grok"},
	{ID: "grok-3-mini", Provider: "grok"},
	{ID: "grok-3-fast", Provider: "grok"},
	{ID: "grok-2", Provider: "grok"},
	{ID: "grok-2-mini", Provider: "grok"},
	{ID: "grok-2-vision", Provider: "grok"},
	// ── MiniMax ──
	{ID: "MiniMax-M2.5", Provider: "minimax"},
	{ID: "MiniMax-M2.5-highspeed", Provider: "minimax"},
	{ID: "MiniMax-M2.1", Provider: "minimax"},
	{ID: "MiniMax-M2", Provider: "minimax"},
	{ID: "MiniMax-Text-01", Provider: "minimax"},
	{ID: "MiniMax-VL-01", Provider: "minimax"},
}

// Default upstream URLs (OpenAI-compatible)
var defaultUpstreams = map[string]string{
	"openai":    "https://api.openai.com/v1",
	"anthropic": "https://api.anthropic.com/v1",
	"deepseek":  "https://api.deepseek.com/v1",
	"qwen":      "https://dashscope.aliyuncs.com/compatible-mode/v1",
	"gemini":    "https://generativelanguage.googleapis.com/v1beta/openai",
	"grok":      "https://api.x.ai/v1",
	"minimax":   "https://api.minimax.io/v1",
}

func (h *GatewayHandler) resolveProvider(modelName string) (providerName, upstreamURL, apiKey string) {
	// Find provider for this model
	provider := ""
	for _, m := range supportedModels {
		if m.ID == modelName {
			provider = m.Provider
			break
		}
	}

	// Fallback: infer from model name prefix
	if provider == "" {
		switch {
		case strings.HasPrefix(modelName, "gpt-") || strings.HasPrefix(modelName, "o1") || strings.HasPrefix(modelName, "o3") || strings.HasPrefix(modelName, "o4") || strings.HasPrefix(modelName, "chatgpt-") || strings.HasPrefix(modelName, "codex-"):
			provider = "openai"
		case strings.HasPrefix(modelName, "claude-"):
			provider = "anthropic"
		case strings.HasPrefix(modelName, "deepseek-"):
			provider = "deepseek"
		case strings.HasPrefix(modelName, "qwen") || strings.HasPrefix(modelName, "qwq") || strings.HasPrefix(modelName, "qvq"):
			provider = "qwen"
		case strings.HasPrefix(modelName, "gemini-"):
			provider = "gemini"
		case strings.HasPrefix(modelName, "grok-"):
			provider = "grok"
		case strings.HasPrefix(modelName, "MiniMax-"):
			provider = "minimax"
		default:
			return "", "", ""
		}
	}

	// Get upstream URL and API key from config
	cfg := config.C.Gateway.Providers[provider]
	upstreamURL = cfg.BaseURL
	if upstreamURL == "" {
		upstreamURL = defaultUpstreams[provider]
	}
	apiKey = cfg.APIKey

	return provider, upstreamURL, apiKey
}

func getModelPricing(modelName string) modelPricing {
	// Pricing in 分 per 1M tokens (approximate, generous margins)
	// NOTE: check -mini/-nano BEFORE prefix matches so o3-mini is cheap
	switch {
	case strings.HasSuffix(modelName, "-mini") || strings.HasSuffix(modelName, "-nano") || strings.HasSuffix(modelName, "-lite"):
		return modelPricing{InputPer1M: 15, OutputPer1M: 60}
	// OpenAI premium
	case modelName == "o3-pro":
		return modelPricing{InputPer1M: 1000, OutputPer1M: 4000}
	case strings.HasPrefix(modelName, "o3") || strings.HasPrefix(modelName, "o4") ||
		modelName == "gpt-4.1":
		return modelPricing{InputPer1M: 200, OutputPer1M: 800}
	case modelName == "gpt-4o":
		return modelPricing{InputPer1M: 150, OutputPer1M: 600}
	// Anthropic
	case strings.Contains(modelName, "opus"):
		return modelPricing{InputPer1M: 1000, OutputPer1M: 4000}
	case strings.Contains(modelName, "sonnet"):
		return modelPricing{InputPer1M: 200, OutputPer1M: 800}
	case strings.Contains(modelName, "haiku"):
		return modelPricing{InputPer1M: 50, OutputPer1M: 200}
	// DeepSeek (cheap)
	case strings.HasPrefix(modelName, "deepseek-"):
		return modelPricing{InputPer1M: 10, OutputPer1M: 20}
	// Qwen
	case strings.HasPrefix(modelName, "qwen3") || strings.HasPrefix(modelName, "qwq") || strings.HasPrefix(modelName, "qvq"):
		return modelPricing{InputPer1M: 20, OutputPer1M: 40}
	case strings.HasPrefix(modelName, "qwen-"):
		return modelPricing{InputPer1M: 10, OutputPer1M: 20}
	// Gemini
	case strings.HasPrefix(modelName, "gemini-2.5-pro"):
		return modelPricing{InputPer1M: 100, OutputPer1M: 400}
	case strings.HasPrefix(modelName, "gemini-"):
		return modelPricing{InputPer1M: 5, OutputPer1M: 15}
	// Grok
	case modelName == "grok-3":
		return modelPricing{InputPer1M: 200, OutputPer1M: 800}
	case strings.HasPrefix(modelName, "grok-"):
		return modelPricing{InputPer1M: 50, OutputPer1M: 200}
	// MiniMax
	case strings.HasPrefix(modelName, "MiniMax-"):
		return modelPricing{InputPer1M: 50, OutputPer1M: 200}
	default:
		return modelPricing{InputPer1M: 100, OutputPer1M: 400}
	}
}

// ==================== Helpers ====================

func generateAPIKey() string {
	b := make([]byte, 24)
	rand.Read(b)
	return "sk-" + hex.EncodeToString(b)
}

// gatewayRequest is the minimal parsed request for routing decisions
type gatewayRequest struct {
	Model  string `json:"model"`
	Stream bool   `json:"stream"`
}
