package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"starclaw.net/synapse/api/internal/billing"
	"starclaw.net/synapse/api/internal/model"
	"starclaw.net/synapse/api/internal/provider"
)

// GenerationHandler manages async media generation tasks (video, image, audio)
// and provides tracking/monitoring APIs.
type GenerationHandler struct {
	db       *gorm.DB
	registry *provider.Registry
	meter    *billing.Meter
}

func NewGenerationHandler(db *gorm.DB, reg *provider.Registry, meter *billing.Meter) *GenerationHandler {
	return &GenerationHandler{db: db, registry: reg, meter: meter}
}

// ListGenerations returns paginated generation history for the authenticated user.
// GET /v1/generations?type=video&status=running&limit=50&offset=0
func (h *GenerationHandler) ListGenerations(c *gin.Context) {
	userID := c.GetString("user_id")
	clawID := c.GetString("claw_id")
	authType := c.GetString("auth_type")

	query := h.db.Model(&model.Generation{})
	if authType == "claw" && clawID != "" {
		query = query.Where("claw_id = ?", clawID)
	} else if userID != "" {
		query = query.Where("user_id = ?", userID)
	} else {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	// Filters
	if t := c.Query("type"); t != "" {
		query = query.Where("type = ?", t)
	}
	if s := c.Query("status"); s != "" {
		query = query.Where("status = ?", s)
	}
	if m := c.Query("model"); m != "" {
		query = query.Where("model LIKE ?", "%"+m+"%")
	}

	// Count
	var total int64
	query.Count(&total)

	// Pagination
	limit := 50
	offset := 0
	if l := c.Query("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
	}
	if o := c.Query("offset"); o != "" {
		fmt.Sscanf(o, "%d", &offset)
	}
	if limit > 100 {
		limit = 100
	}

	var gens []model.Generation
	query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&gens)

	c.JSON(http.StatusOK, gin.H{
		"generations": gens,
		"total":       total,
		"limit":       limit,
		"offset":      offset,
	})
}

// GetGeneration returns a single generation by ID.
// GET /v1/generations/:id
func (h *GenerationHandler) GetGeneration(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetString("user_id")
	clawID := c.GetString("claw_id")

	var gen model.Generation
	query := h.db.Where("id = ? OR task_id = ?", id, id)
	if clawID != "" {
		query = query.Where("claw_id = ?", clawID)
	} else {
		query = query.Where("user_id = ?", userID)
	}

	if err := query.First(&gen).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "generation not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"generation": gen})
}

// GenerationStats returns summary stats for the user's generations.
// GET /v1/generations/stats
func (h *GenerationHandler) GenerationStats(c *gin.Context) {
	userID := c.GetString("user_id")
	clawID := c.GetString("claw_id")
	authType := c.GetString("auth_type")

	var where string
	var arg string
	if authType == "claw" && clawID != "" {
		where = "claw_id = ?"
		arg = clawID
	} else {
		where = "user_id = ?"
		arg = userID
	}

	var total, running, succeeded, failed int64
	h.db.Model(&model.Generation{}).Where(where, arg).Count(&total)
	h.db.Model(&model.Generation{}).Where(where+" AND status = ?", arg, "running").Count(&running)
	h.db.Model(&model.Generation{}).Where(where+" AND status = ?", arg, "succeeded").Count(&succeeded)
	h.db.Model(&model.Generation{}).Where(where+" AND status = ?", arg, "failed").Count(&failed)

	var totalCost float64
	h.db.Model(&model.Generation{}).Where(where, arg).Select("COALESCE(SUM(cost_cents), 0)").Scan(&totalCost)

	// Model breakdown
	type modelCount struct {
		Model string `json:"model"`
		Count int64  `json:"count"`
	}
	var byModel []modelCount
	h.db.Model(&model.Generation{}).Where(where, arg).
		Select("model, COUNT(*) as count").Group("model").Order("count DESC").Scan(&byModel)

	c.JSON(http.StatusOK, gin.H{
		"total":      total,
		"running":    running,
		"succeeded":  succeeded,
		"failed":     failed,
		"total_cost": totalCost,
		"by_model":   byModel,
	})
}

// ── Proxy with tracking ──

// ProxyDashScopeVideo intercepts DashScope video generation requests,
// records a Generation entry, forwards the request, and extracts the task_id.
// POST /v1/proxy/dashscope/api/v1/services/aigc/video-generation/*
func (h *GenerationHandler) ProxyDashScopeVideo(c *gin.Context, provSlug, subPath string, body []byte) {
	userID := c.GetString("user_id")
	clawID := c.GetString("claw_id")

	// Parse request to extract model/prompt
	var reqBody struct {
		Model string `json:"model"`
		Input struct {
			Prompt string `json:"prompt"`
			ImgURL string `json:"img_url"`
		} `json:"input"`
		Parameters struct {
			Size     string `json:"size"`
			Duration int    `json:"duration"`
		} `json:"parameters"`
	}
	json.Unmarshal(body, &reqBody)

	// Create generation record (pending)
	now := time.Now()
	gen := model.Generation{
		UserID:    userID,
		ClawID:    clawID,
		Provider:  "dashscope",
		Model:     reqBody.Model,
		Type:      "video",
		Prompt:    reqBody.Input.Prompt,
		Status:    "pending",
		Duration:  reqBody.Parameters.Duration,
		StartedAt: &now,
	}
	// Parse size
	if reqBody.Parameters.Size != "" {
		parts := strings.Split(reqBody.Parameters.Size, "*")
		if len(parts) == 2 {
			fmt.Sscanf(parts[0], "%d", &gen.Width)
			fmt.Sscanf(parts[1], "%d", &gen.Height)
		}
	}

	// Forward to DashScope
	prov, ok := h.registry.GetProvider(provSlug)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "unknown provider: " + provSlug, "type": "invalid_request_error"}})
		return
	}

	upstreamURL := strings.TrimRight(prov.Endpoint, "/") + subPath
	upstreamReq, err := http.NewRequestWithContext(c.Request.Context(), "POST", upstreamURL, strings.NewReader(string(body)))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": "failed to create upstream request", "type": "server_error"}})
		return
	}

	apiKey := h.registry.GetAPIKey(provSlug)
	upstreamReq.Header.Set("Authorization", "Bearer "+apiKey)
	upstreamReq.Header.Set("Content-Type", "application/json")
	upstreamReq.Header.Set("X-DashScope-Async", "enable")

	log.Printf("[star-ai] proxy+track → %s %s (model=%s)", "POST", upstreamURL, reqBody.Model)

	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Do(upstreamReq)
	if err != nil {
		gen.Status = "failed"
		gen.ErrorMsg = err.Error()
		h.db.Create(&gen)
		c.JSON(http.StatusBadGateway, gin.H{"error": gin.H{"message": "upstream provider unreachable: " + err.Error(), "type": "server_error"}})
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	// Extract task_id from DashScope response
	var result struct {
		Output struct {
			TaskID     string `json:"task_id"`
			TaskStatus string `json:"task_status"`
		} `json:"output"`
	}
	json.Unmarshal(respBody, &result)

	if result.Output.TaskID != "" {
		gen.TaskID = result.Output.TaskID
		gen.Status = "running"
	} else if resp.StatusCode != 200 {
		gen.Status = "failed"
		gen.ErrorMsg = string(respBody)
	}

	// Calculate cost
	costCents, upstreamCents := h.meter.CalculateCost("dashscope/"+reqBody.Model, 0, 0)
	if costCents <= 0 {
		if entry, ok := h.registry.GetModel("dashscope/" + reqBody.Model); ok && entry.Model.PricePerCallCNY > 0 {
			upstreamCents = entry.Model.PricePerCallCNY * 100
			costCents = upstreamCents * 1.3
		}
	}
	gen.CostCents = costCents

	// Actually deduct from user balance
	if costCents > 0 && gen.Status != "failed" {
		authType := c.GetString("auth_type")
		if authType == "claw" && clawID != "" {
			if err := h.meter.DeductClaw(clawID, costCents, "dashscope/"+reqBody.Model, "/v1/proxy/dashscope", nil); err != nil {
				log.Printf("[star-ai] dashscope video deduct failed: claw=%s cost=%.2f err=%v", clawID, costCents, err)
			} else if costCents > upstreamCents {
				if qc := h.meter.QueenCredit(); qc != nil && qc.Enabled() {
					go qc.ProfitSplit(&billing.ProfitSplitRequest{
						ClawID: clawID, CostCents: costCents, UpstreamCents: upstreamCents,
						Model: "dashscope/" + reqBody.Model, Endpoint: "/v1/proxy/dashscope",
					})
				}
			}
		} else if userID != "" {
			record := &model.UsageRecord{
				UserID: userID, Provider: "dashscope", Model: reqBody.Model,
				Endpoint: "/v1/proxy/dashscope" + subPath, Via: "proxy", Status: "ok",
			}
			if err := h.meter.Deduct(userID, costCents, upstreamCents, record); err != nil {
				log.Printf("[star-ai] dashscope video deduct failed: user=%s cost=%.2f err=%v", userID, costCents, err)
			}
		}
	}

	h.db.Create(&gen)

	log.Printf("[star-ai] generation tracked+billed: id=%s task=%s model=%s status=%s cost=%.2f分",
		gen.ID, gen.TaskID, gen.Model, gen.Status, gen.CostCents)

	// Forward response to client
	for k, vals := range resp.Header {
		for _, v := range vals {
			c.Writer.Header().Add(k, v)
		}
	}
	c.Writer.WriteHeader(resp.StatusCode)
	c.Writer.Write(respBody)
}

// ProxyFalVideo intercepts fal.ai video generation requests with tracking.
func (h *GenerationHandler) ProxyFalVideo(c *gin.Context, provSlug, subPath string, body []byte) {
	userID := c.GetString("user_id")
	clawID := c.GetString("claw_id")

	var reqBody struct {
		Prompt string `json:"prompt"`
	}
	json.Unmarshal(body, &reqBody)

	// Determine model from fal endpoint path
	falModel := determineFalModel(subPath)

	now := time.Now()
	gen := model.Generation{
		UserID:    userID,
		ClawID:    clawID,
		Provider:  "fal",
		Model:     falModel,
		Type:      "video",
		Prompt:    reqBody.Prompt,
		Status:    "pending",
		StartedAt: &now,
	}

	// Forward to fal.ai
	prov, ok := h.registry.GetProvider(provSlug)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "unknown provider", "type": "invalid_request_error"}})
		return
	}

	upstreamURL := strings.TrimRight(prov.Endpoint, "/") + subPath
	upstreamReq, err := http.NewRequestWithContext(c.Request.Context(), "POST", upstreamURL, strings.NewReader(string(body)))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": "failed to create request", "type": "server_error"}})
		return
	}

	apiKey := h.registry.GetAPIKey(provSlug)
	upstreamReq.Header.Set("Authorization", "Key "+apiKey)
	upstreamReq.Header.Set("Content-Type", "application/json")

	log.Printf("[star-ai] proxy+track → %s %s (model=%s)", "POST", upstreamURL, falModel)

	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Do(upstreamReq)
	if err != nil {
		gen.Status = "failed"
		gen.ErrorMsg = err.Error()
		h.db.Create(&gen)
		c.JSON(http.StatusBadGateway, gin.H{"error": gin.H{"message": "upstream unreachable: " + err.Error(), "type": "server_error"}})
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	// fal.ai returns request_id for async
	var result struct {
		RequestID string `json:"request_id"`
	}
	json.Unmarshal(respBody, &result)

	if result.RequestID != "" {
		gen.TaskID = result.RequestID
		gen.Status = "running"
	} else if resp.StatusCode != 200 {
		gen.Status = "failed"
		gen.ErrorMsg = string(respBody)
	}

	// Calculate cost for fal video
	falFullName := "fal/" + falModel
	costCents, upstreamCents := h.meter.CalculateCost(falFullName, 0, 0)
	if costCents <= 0 {
		if entry, ok := h.registry.GetModel(falFullName); ok && entry.Model.PricePerCall > 0 {
			upstreamCents = entry.Model.PricePerCall * 7.2 * 100 // USD→CNY→分
			costCents = upstreamCents * 1.3
		}
	}
	gen.CostCents = costCents

	// Actually deduct from user balance
	if costCents > 0 && gen.Status != "failed" {
		authType := c.GetString("auth_type")
		if authType == "claw" && clawID != "" {
			if err := h.meter.DeductClaw(clawID, costCents, falFullName, "/v1/proxy/fal", nil); err != nil {
				log.Printf("[star-ai] fal video deduct failed: claw=%s cost=%.2f err=%v", clawID, costCents, err)
			} else {
				if qc := h.meter.QueenCredit(); qc != nil && qc.Enabled() {
					go qc.ProfitSplit(&billing.ProfitSplitRequest{
						ClawID: clawID, CostCents: costCents, UpstreamCents: upstreamCents,
						Model: falFullName, Endpoint: "/v1/proxy/fal",
					})
				}
			}
		} else if userID != "" {
			record := &model.UsageRecord{
				UserID: userID, Provider: "fal", Model: falModel,
				Endpoint: "/v1/proxy/fal" + subPath, Via: "proxy", Status: "ok",
			}
			if err := h.meter.Deduct(userID, costCents, upstreamCents, record); err != nil {
				log.Printf("[star-ai] fal video deduct failed: user=%s cost=%.2f err=%v", userID, costCents, err)
			}
		}
	}

	h.db.Create(&gen)

	log.Printf("[star-ai] generation tracked+billed: id=%s task=%s model=%s status=%s cost=%.2f分",
		gen.ID, gen.TaskID, gen.Model, gen.Status, gen.CostCents)

	for k, vals := range resp.Header {
		for _, v := range vals {
			c.Writer.Header().Add(k, v)
		}
	}
	c.Writer.WriteHeader(resp.StatusCode)
	c.Writer.Write(respBody)
}

// UpdateGenerationStatus is called internally when a status poll reveals completion.
// POST /v1/internal/generations/:task_id/status
func (h *GenerationHandler) UpdateGenerationStatus(c *gin.Context) {
	taskID := c.Param("task_id")
	var req struct {
		Status    string `json:"status"` // succeeded, failed
		ResultURL string `json:"result_url"`
		ErrorMsg  string `json:"error_msg"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}

	updates := map[string]interface{}{"status": req.Status}
	if req.ResultURL != "" {
		updates["result_url"] = req.ResultURL
	}
	if req.ErrorMsg != "" {
		updates["error_msg"] = req.ErrorMsg
	}
	if req.Status == "succeeded" || req.Status == "failed" {
		now := time.Now()
		updates["completed_at"] = &now
	}

	result := h.db.Model(&model.Generation{}).Where("task_id = ?", taskID).Updates(updates)
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "generation not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"updated": result.RowsAffected})
}

// ProxyFalImage intercepts fal.ai image generation requests with tracking.
func (h *GenerationHandler) ProxyFalImage(c *gin.Context, provSlug, subPath string, body []byte) {
	userID := c.GetString("user_id")
	clawID := c.GetString("claw_id")

	var reqBody struct {
		Prompt string `json:"prompt"`
	}
	json.Unmarshal(body, &reqBody)

	falModel := determineFalImageModel(subPath)

	now := time.Now()
	gen := model.Generation{
		UserID:    userID,
		ClawID:    clawID,
		Provider:  "fal",
		Model:     falModel,
		Type:      "image",
		Prompt:    reqBody.Prompt,
		Status:    "pending",
		StartedAt: &now,
	}

	// Forward to fal.ai
	prov, ok := h.registry.GetProvider(provSlug)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "unknown provider", "type": "invalid_request_error"}})
		return
	}

	upstreamURL := strings.TrimRight(prov.Endpoint, "/") + subPath
	upstreamReq, err := http.NewRequestWithContext(c.Request.Context(), "POST", upstreamURL, strings.NewReader(string(body)))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": "failed to create request", "type": "server_error"}})
		return
	}

	apiKey := h.registry.GetAPIKey(provSlug)
	upstreamReq.Header.Set("Authorization", "Key "+apiKey)
	upstreamReq.Header.Set("Content-Type", "application/json")

	log.Printf("[star-ai] proxy+track → %s %s (model=%s type=image)", "POST", upstreamURL, falModel)

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(upstreamReq)
	if err != nil {
		gen.Status = "failed"
		gen.ErrorMsg = err.Error()
		h.db.Create(&gen)
		c.JSON(http.StatusBadGateway, gin.H{"error": gin.H{"message": "upstream unreachable: " + err.Error(), "type": "server_error"}})
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	// fal.ai queue returns request_id for async; sync returns images directly
	var result struct {
		RequestID string `json:"request_id"`
	}
	json.Unmarshal(respBody, &result)

	if result.RequestID != "" {
		gen.TaskID = result.RequestID
		gen.Status = "running"
	} else if resp.StatusCode == 200 {
		gen.Status = "succeeded"
		completed := time.Now()
		gen.CompletedAt = &completed
	} else {
		gen.Status = "failed"
		gen.ErrorMsg = string(respBody)
	}

	// Calculate cost for fal image
	falFullName := "fal/" + falModel
	costCents, upstreamCents := h.meter.CalculateCost(falFullName, 0, 0)
	if costCents <= 0 {
		if entry, ok := h.registry.GetModel(falFullName); ok {
			if entry.Model.PricePerCallCNY > 0 {
				upstreamCents = entry.Model.PricePerCallCNY * 100
				costCents = upstreamCents * 1.3
			} else if entry.Model.PricePerCall > 0 {
				upstreamCents = entry.Model.PricePerCall * 7.2 * 100
				costCents = upstreamCents * 1.3
			}
		}
	}
	gen.CostCents = costCents

	// Actually deduct from user balance
	if costCents > 0 && gen.Status != "failed" {
		authType := c.GetString("auth_type")
		if authType == "claw" && clawID != "" {
			if err := h.meter.DeductClaw(clawID, costCents, falFullName, "/v1/proxy/fal", nil); err != nil {
				log.Printf("[star-ai] fal image deduct failed: claw=%s cost=%.2f err=%v", clawID, costCents, err)
			} else if costCents > upstreamCents {
				if qc := h.meter.QueenCredit(); qc != nil && qc.Enabled() {
					go qc.ProfitSplit(&billing.ProfitSplitRequest{
						ClawID: clawID, CostCents: costCents, UpstreamCents: upstreamCents,
						Model: falFullName, Endpoint: "/v1/proxy/fal",
					})
				}
			}
		} else if userID != "" {
			record := &model.UsageRecord{
				UserID: userID, Provider: "fal", Model: falModel,
				Endpoint: "/v1/proxy/fal" + subPath, Via: "proxy", Status: "ok",
			}
			if err := h.meter.Deduct(userID, costCents, upstreamCents, record); err != nil {
				log.Printf("[star-ai] fal image deduct failed: user=%s cost=%.2f err=%v", userID, costCents, err)
			}
		}
	}

	h.db.Create(&gen)

	log.Printf("[star-ai] generation tracked+billed: id=%s task=%s model=%s type=image status=%s cost=%.2f分",
		gen.ID, gen.TaskID, gen.Model, gen.Status, gen.CostCents)

	for k, vals := range resp.Header {
		for _, v := range vals {
			c.Writer.Header().Add(k, v)
		}
	}
	c.Writer.WriteHeader(resp.StatusCode)
	c.Writer.Write(respBody)
}

func determineFalImageModel(path string) string {
	switch {
	// ── Nano Banana (Google) ──
	case strings.Contains(path, "nano-banana-2"):
		return "nano-banana-2"
	case strings.Contains(path, "nano-banana-pro"):
		return "nano-banana-pro"
	case strings.Contains(path, "nano-banana"):
		return "nano-banana"
	// ── Seedream (ByteDance) ──
	case strings.Contains(path, "seedream"):
		if strings.Contains(path, "v5") {
			return "bytedance/seedream/v5/lite/edit"
		}
		if strings.Contains(path, "v4.5") {
			return "bytedance/seedream/v4.5/edit"
		}
		return "bytedance/seedream/v4/text-to-image"
	// ── Flux Kontext ──
	case strings.Contains(path, "flux-pro/kontext") || strings.Contains(path, "flux-kontext"):
		return "flux-pro/kontext"
	// ── FLUX.2 ──
	case strings.Contains(path, "flux-2-pro"):
		return "flux-2-pro"
	case strings.Contains(path, "flux-2-max"):
		return "flux-2-max"
	case strings.Contains(path, "flux-2-flex"):
		return "flux-2-flex"
	case strings.Contains(path, "flux-2/turbo"):
		return "flux-2/turbo"
	case strings.Contains(path, "flux-2/flash"):
		return "flux-2/flash"
	case strings.Contains(path, "flux-2/klein/4b"):
		return "flux-2/klein/4b"
	case strings.Contains(path, "flux-2/klein/9b"):
		return "flux-2/klein/9b"
	case strings.Contains(path, "flux-2/lora"):
		return "flux-2/lora"
	case strings.Contains(path, "flux-2"):
		return "flux-2"
	// ── FLUX.1 ──
	case strings.Contains(path, "flux-pro"):
		return "flux-pro/v1.1"
	case strings.Contains(path, "flux-lora"):
		return "flux-lora"
	case strings.Contains(path, "flux/schnell"):
		return "flux/schnell"
	case strings.Contains(path, "flux/dev"):
		return "flux/dev"
	case strings.Contains(path, "flux"):
		return "flux/schnell"
	// ── Qwen Image ──
	case strings.Contains(path, "qwen-image"):
		return "qwen-image"
	// ── GPT Image ──
	case strings.Contains(path, "gpt-image"):
		return "gpt-image-1.5/edit"
	// ── Reve ──
	case strings.Contains(path, "reve"):
		return "reve/edit"
	// ── Upscale/BG ──
	case strings.Contains(path, "seedvr/upscale"):
		return "seedvr/upscale/image"
	case strings.Contains(path, "remove-bg") || strings.Contains(path, "background/remove"):
		return "remove-bg"
	default:
		return "fal-image-unknown"
	}
}

func determineFalModel(path string) string {
	switch {
	case strings.Contains(path, "veo3.1/fast"):
		return "veo3.1/fast"
	case strings.Contains(path, "veo3.1"):
		return "veo3.1"
	case strings.Contains(path, "veo3"):
		return "veo3.1" // veo3 deprecated, map to 3.1
	case strings.Contains(path, "sora-2"):
		return "sora-2"
	case strings.Contains(path, "kling-video/o3"):
		if strings.Contains(path, "pro") {
			return "kling-video/o3/pro/image-to-video"
		}
		return "kling-video/o3/standard/image-to-video"
	case strings.Contains(path, "kling-video/v3/pro"):
		if strings.Contains(path, "image") {
			return "kling-video/v3/pro/image-to-video"
		}
		return "kling-video/v3/pro/text-to-video"
	case strings.Contains(path, "kling-video/v3/standard"):
		if strings.Contains(path, "image") {
			return "kling-video/v3/standard/image-to-video"
		}
		return "kling-video/v3/standard/text-to-video"
	case strings.Contains(path, "kling-video"):
		return "kling-video/v3/standard/text-to-video"
	case strings.Contains(path, "wan-25") || strings.Contains(path, "wan/v2"):
		return "wan-25-preview/text-to-video"
	case strings.Contains(path, "minimax-video"):
		return "minimax-video"
	case strings.Contains(path, "luma-dream-machine"):
		return "luma-dream-machine"
	case strings.Contains(path, "ltx-2.3"):
		return "ltx-2.3/text-to-video"
	case strings.Contains(path, "ltx-2"):
		return "ltx-2-19b/image-to-video"
	case strings.Contains(path, "ovi"):
		return "ovi/image-to-video"
	case strings.Contains(path, "grok-imagine-video"):
		return "xai/grok-imagine-video/reference-to-video"
	default:
		return "fal-unknown"
	}
}

func isVideoPath(path string) bool {
	videoKeywords := []string{"veo3", "sora-2", "kling-video", "minimax-video", "luma-dream-machine",
		"wan-25", "wan/v2", "ltx-2", "ovi", "grok-imagine-video", "goal-force"}
	for _, kw := range videoKeywords {
		if strings.Contains(path, kw) {
			return true
		}
	}
	return false
}

func isImagePath(path string) bool {
	imageKeywords := []string{"flux", "nano-banana", "seedream", "qwen-image", "gpt-image",
		"reve", "seedvr", "remove-bg", "background/remove", "onereward",
		"stable-diffusion", "recraft", "ideogram", "z-image"}
	for _, kw := range imageKeywords {
		if strings.Contains(path, kw) {
			return true
		}
	}
	return false
}
