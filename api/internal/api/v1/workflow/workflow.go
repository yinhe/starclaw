package workflow

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw/internal/model"
	"github.com/yinhe/starclaw/internal/provider"
	"github.com/yinhe/starclaw/internal/tool"
	"github.com/yinhe/starclaw/internal/workflow"
	"gorm.io/gorm"
)

type WorkflowHandler struct {
	db               *gorm.DB
	providerRegistry *provider.Registry
	toolRegistry     *tool.Registry
}

func NewWorkflowHandler(db *gorm.DB, pr *provider.Registry, tr *tool.Registry) *WorkflowHandler {
	return &WorkflowHandler{db: db, providerRegistry: pr, toolRegistry: tr}
}

func (h *WorkflowHandler) List(c *gin.Context) {
	userID := c.GetString("user_id")

	var workflows []model.Workflow
	q := h.db.Where("user_id = ?", userID)
	if cat := c.Query("category"); cat != "" {
		q = q.Where("category = ?", cat)
	}
	q.Order("updated_at DESC").Find(&workflows)

	// Collect distinct categories for tab rendering
	var categories []string
	h.db.Model(&model.Workflow{}).Where("user_id = ? AND category != ''", userID).Distinct("category").Pluck("category", &categories)

	c.JSON(http.StatusOK, gin.H{"workflows": workflows, "categories": categories})
}

func (h *WorkflowHandler) Create(c *gin.Context) {
	userID := c.GetString("user_id")

	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Definition  string `json:"definition"`
		Category    string `json:"category"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	wf := model.Workflow{
		UserID:      userID,
		Name:        req.Name,
		Description: req.Description,
		Definition:  req.Definition,
		Category:    req.Category,
	}
	if err := h.db.Create(&wf).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create workflow"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"workflow": wf})
}

func (h *WorkflowHandler) Get(c *gin.Context) {
	userID := c.GetString("user_id")
	id := c.Param("id")

	var wf model.Workflow
	if err := h.db.Where("id = ? AND user_id = ?", id, userID).First(&wf).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "workflow not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"workflow": wf})
}

func (h *WorkflowHandler) Update(c *gin.Context) {
	userID := c.GetString("user_id")
	id := c.Param("id")

	var wf model.Workflow
	if err := h.db.Where("id = ? AND user_id = ?", id, userID).First(&wf).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "workflow not found"})
		return
	}

	var req struct {
		Name        string  `json:"name"`
		Description string  `json:"description"`
		Definition  string  `json:"definition"`
		Category    *string `json:"category"` // pointer to distinguish empty vs absent
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := map[string]interface{}{}
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.Definition != "" {
		updates["definition"] = req.Definition
	}
	if req.Category != nil {
		updates["category"] = *req.Category
	}

	h.db.Model(&wf).Updates(updates)
	h.db.First(&wf, "id = ?", id)

	c.JSON(http.StatusOK, gin.H{"workflow": wf})
}

func (h *WorkflowHandler) Delete(c *gin.Context) {
	userID := c.GetString("user_id")
	id := c.Param("id")

	result := h.db.Where("id = ? AND user_id = ?", id, userID).Delete(&model.Workflow{})
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "workflow not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

func (h *WorkflowHandler) Run(c *gin.Context) {
	userID := c.GetString("user_id")
	id := c.Param("id")

	var wf model.Workflow
	if err := h.db.Where("id = ? AND user_id = ?", id, userID).First(&wf).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "workflow not found"})
		return
	}

	var req struct {
		Input string `json:"input"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Build provider resolver from DB
	resolver := func(modelName string) provider.ModelProvider {
		// Try to find model config by model_name
		var cfg model.ModelConfig
		if err := h.db.Where("model_name = ? AND user_id = ?", modelName, userID).First(&cfg).Error; err != nil {
			// Fallback: try any matching model
			h.db.Where("model_name = ?", modelName).First(&cfg)
		}
		if cfg.ID == "" {
			return nil
		}
		return resolveProvider(cfg, h.providerRegistry)
	}

	engine := workflow.NewEngine(resolver, h.toolRegistry)

	// Create run record
	run := model.WorkflowRun{
		WorkflowID: wf.ID,
		UserID:     userID,
		Input:      req.Input,
		Status:     "running",
	}
	h.db.Create(&run)

	startTime := time.Now()
	result, err := engine.Execute(c.Request.Context(), wf.Definition, req.Input)
	durationMs := time.Since(startTime).Milliseconds()

	if err != nil {
		h.db.Model(&run).Updates(map[string]interface{}{
			"status":      "error",
			"error":       err.Error(),
			"duration_ms": durationMs,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "run_id": run.ID})
		return
	}

	h.db.Model(&run).Updates(map[string]interface{}{
		"status":      "success",
		"output":      result,
		"duration_ms": durationMs,
	})

	c.JSON(http.StatusOK, gin.H{"output": result, "run_id": run.ID, "duration_ms": durationMs})
}

func (h *WorkflowHandler) ListRuns(c *gin.Context) {
	userID := c.GetString("user_id")
	wfID := c.Param("id")

	var runs []model.WorkflowRun
	h.db.Where("workflow_id = ? AND user_id = ?", wfID, userID).Order("created_at DESC").Limit(50).Find(&runs)
	c.JSON(http.StatusOK, gin.H{"runs": runs})
}

func generateToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// EnableWebhook generates a webhook token for external triggering
func (h *WorkflowHandler) EnableWebhook(c *gin.Context) {
	userID := c.GetString("user_id")
	wfID := c.Param("id")

	var wf model.Workflow
	if err := h.db.Where("id = ? AND user_id = ?", wfID, userID).First(&wf).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "workflow not found"})
		return
	}

	token := generateToken()
	h.db.Model(&wf).Update("webhook_token", token)

	c.JSON(http.StatusOK, gin.H{
		"webhook_token": token,
		"webhook_url":   "/v1/webhooks/workflow/" + token,
	})
}

// DisableWebhook removes the webhook token
func (h *WorkflowHandler) DisableWebhook(c *gin.Context) {
	userID := c.GetString("user_id")
	wfID := c.Param("id")

	result := h.db.Model(&model.Workflow{}).Where("id = ? AND user_id = ?", wfID, userID).Update("webhook_token", nil)
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "workflow not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "webhook disabled"})
}

// Webhook handles external workflow trigger (no auth required)
func (h *WorkflowHandler) Webhook(c *gin.Context) {
	token := c.Param("token")

	var wf model.Workflow
	if err := h.db.Where("webhook_token = ? AND webhook_token != ''", token).First(&wf).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "invalid webhook"})
		return
	}

	var req struct {
		Input string `json:"input"`
	}
	c.ShouldBindJSON(&req)

	resolver := func(modelName string) provider.ModelProvider {
		var cfg model.ModelConfig
		if err := h.db.Where("model_name = ? AND user_id = ?", modelName, wf.UserID).First(&cfg).Error; err != nil {
			h.db.Where("model_name = ?", modelName).First(&cfg)
		}
		if cfg.ID == "" {
			return nil
		}
		return resolveProvider(cfg, h.providerRegistry)
	}

	engine := workflow.NewEngine(resolver, h.toolRegistry)

	run := model.WorkflowRun{
		WorkflowID: wf.ID,
		UserID:     wf.UserID,
		Input:      req.Input,
		Status:     "running",
	}
	h.db.Create(&run)

	startTime := time.Now()
	result, err := engine.Execute(c.Request.Context(), wf.Definition, req.Input)
	durationMs := time.Since(startTime).Milliseconds()

	if err != nil {
		h.db.Model(&run).Updates(map[string]interface{}{
			"status": "error", "error": err.Error(), "duration_ms": durationMs,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "run_id": run.ID})
		return
	}

	h.db.Model(&run).Updates(map[string]interface{}{
		"status": "success", "output": result, "duration_ms": durationMs,
	})
	c.JSON(http.StatusOK, gin.H{"output": result, "run_id": run.ID, "duration_ms": durationMs})
}

func resolveProvider(cfg model.ModelConfig, registry *provider.Registry) provider.ModelProvider {
	if p, ok := registry.Get(cfg.Provider); ok {
		return p
	}
	switch cfg.Provider {
	case "anthropic":
		return provider.NewAnthropicProvider(provider.AnthropicConfig{APIKey: cfg.APIKey, BaseURL: cfg.BaseURL})
	case "deepseek":
		return provider.NewDeepSeekProvider(provider.DeepSeekConfig{APIKey: cfg.APIKey, BaseURL: cfg.BaseURL})
	case "ollama":
		return provider.NewOllamaProvider(provider.OllamaConfig{BaseURL: cfg.BaseURL})
	case "openrouter":
		return provider.NewOpenRouterProvider(provider.OpenRouterConfig{APIKey: cfg.APIKey})
	default:
		return provider.NewOpenAIProvider(provider.OpenAIConfig{APIKey: cfg.APIKey, BaseURL: cfg.BaseURL})
	}
}
