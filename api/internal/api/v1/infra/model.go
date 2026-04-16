package infra

import (
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw/internal/model"
	"github.com/yinhe/starclaw/internal/provider"
	"github.com/yinhe/starclaw/internal/security"
	"gorm.io/gorm"
)

// SeedPlatformModels creates platform-level model configs from environment variables.
// These are shared configs available to all users in hosted mode.
// Env vars: PLATFORM_QWEN_API_KEY, PLATFORM_OPENAI_API_KEY, PLATFORM_FAL_API_KEY, etc.
func SeedPlatformModels(db *gorm.DB) {
	type platformDef struct {
		ID          string
		Provider    string
		ModelName   string
		DisplayName string
		EnvKey      string
		BaseURL     string
	}

	defs := []platformDef{
		{ID: "platform-qwen", Provider: "qwen", ModelName: "qwen3-max", DisplayName: "通义千问 (平台)", EnvKey: "PLATFORM_QWEN_API_KEY"},
		{ID: "platform-openai", Provider: "openai", ModelName: "gpt-4o", DisplayName: "OpenAI (平台)", EnvKey: "PLATFORM_OPENAI_API_KEY"},
		{ID: "platform-deepseek", Provider: "deepseek", ModelName: "deepseek-chat", DisplayName: "DeepSeek (平台)", EnvKey: "PLATFORM_DEEPSEEK_API_KEY"},
	}

	for _, d := range defs {
		apiKey := os.Getenv(d.EnvKey)
		if apiKey == "" {
			continue
		}
		var existing model.ModelConfig
		encKey := security.EncryptAPIKey(apiKey)
		if err := db.Where("id = ?", d.ID).First(&existing).Error; err == nil {
			// Update key if changed (compare plaintext against decrypted stored)
			if security.DecryptAPIKey(existing.APIKey) != apiKey {
				db.Model(&existing).Update("api_key", encKey)
			}
			continue
		}
		cfg := model.ModelConfig{
			ID:          d.ID,
			UserID:      "platform",
			Provider:    d.Provider,
			ModelName:   d.ModelName,
			DisplayName: d.DisplayName,
			APIKey:      encKey,
			BaseURL:     d.BaseURL,
			MaxTokens:   16384,
			Temperature: 0.7,
			IsPlatform:  true,
			IsEnabled:   true,
		}
		db.Create(&cfg)
		log.Printf("[Platform] Seeded model config: %s (%s)", d.DisplayName, d.Provider)
	}
}

type ModelHandler struct {
	db         *gorm.DB
	registry   *provider.Registry
	deployMode string
}

func NewModelHandler(db *gorm.DB, registry *provider.Registry, deployMode string) *ModelHandler {
	return &ModelHandler{db: db, registry: registry, deployMode: deployMode}
}

type CreateModelRequest struct {
	Provider    string  `json:"provider" binding:"required"`
	ModelName   string  `json:"model_name"`
	DisplayName string  `json:"display_name"`
	APIKey      string  `json:"api_key"`
	BaseURL     string  `json:"base_url"`
	MaxTokens   int     `json:"max_tokens"`
	Temperature float64 `json:"temperature"`
}

// defaultModelForProvider returns a sensible default model name for a provider
func defaultModelForProvider(p string) string {
	switch p {
	case "qwen":
		return "qwen3-max"
	case "star-ai":
		return "qwen3-max"
	case "openai":
		return "gpt-4o"
	case "anthropic":
		return "claude-sonnet-4-20250514"
	case "deepseek":
		return "deepseek-chat"
	case "google":
		return "gemini-2.0-flash"
	case "zhipu":
		return "glm-4-plus"
	case "moonshot":
		return "moonshot-v1-128k"
	case "volcengine":
		return "doubao-seed-2-0-pro-260215"
	default:
		return ""
	}
}

func (h *ModelHandler) List(c *gin.Context) {
	userID := c.GetString("user_id")

	var models []model.ModelConfig
	if h.deployMode == "hosted" {
		// In hosted mode, show user's own configs + platform configs (for providers user hasn't configured)
		if err := h.db.Where("user_id = ? OR is_platform = ?", userID, true).Find(&models).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch models"})
			return
		}
	} else {
		if err := h.db.Where("user_id = ?", userID).Find(&models).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch models"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"models":    models,
		"providers": h.registry.List(),
	})
}

// AvailableModels returns all available models grouped by configured providers
func (h *ModelHandler) AvailableModels(c *gin.Context) {
	userID := c.GetString("user_id")

	var configs []model.ModelConfig
	if h.deployMode == "hosted" {
		h.db.Where("(user_id = ? OR is_platform = ?) AND is_enabled = ?", userID, true, true).Find(&configs)
	} else {
		h.db.Where("user_id = ? AND is_enabled = ?", userID, true).Find(&configs)
	}

	type ProviderModels struct {
		ConfigID string   `json:"config_id"`
		Provider string   `json:"provider"`
		BaseURL  string   `json:"base_url"`
		Models   []string `json:"models"`
	}

	var result []ProviderModels
	for _, cfg := range configs {
		p := provider.CreateFromConfig(h.registry, cfg)
		if p == nil {
			continue
		}
		result = append(result, ProviderModels{
			ConfigID: cfg.ID,
			Provider: cfg.Provider,
			BaseURL:  cfg.BaseURL,
			Models:   p.Models(),
		})
	}

	c.JSON(http.StatusOK, gin.H{"providers": result})
}

func (h *ModelHandler) Create(c *gin.Context) {
	userID := c.GetString("user_id")

	var req CreateModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Prevent duplicate provider configs for the same user
	var count int64
	h.db.Model(&model.ModelConfig{}).Where("user_id = ? AND provider = ?", userID, req.Provider).Count(&count)
	if count > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "该提供商已配置，无需重复添加"})
		return
	}

	// Auto-set default model name if not provided
	if req.ModelName == "" {
		req.ModelName = defaultModelForProvider(req.Provider)
	}
	if req.ModelName == "" {
		req.ModelName = "default"
	}

	m := model.ModelConfig{
		UserID:      userID,
		Provider:    req.Provider,
		ModelName:   req.ModelName,
		DisplayName: req.DisplayName,
		APIKey:      security.EncryptAPIKey(req.APIKey),
		BaseURL:     req.BaseURL,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		IsEnabled:   true,
	}

	if m.MaxTokens == 0 {
		m.MaxTokens = 4096
	}
	if m.Temperature == 0 {
		m.Temperature = 0.7
	}

	if err := h.db.Create(&m).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create model config"})
		return
	}

	c.JSON(http.StatusCreated, m)
}

func (h *ModelHandler) Update(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetString("user_id")

	var m model.ModelConfig
	if err := h.db.Where("id = ? AND user_id = ?", id, userID).First(&m).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "model config not found"})
		return
	}

	var req struct {
		Provider    string  `json:"provider"`
		ModelName   string  `json:"model_name"`
		DisplayName string  `json:"display_name"`
		APIKey      string  `json:"api_key"`
		BaseURL     string  `json:"base_url"`
		MaxTokens   int     `json:"max_tokens"`
		Temperature float64 `json:"temperature"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Use map for selective updates (GORM ignores zero values with struct)
	updates := map[string]interface{}{}
	if req.BaseURL != "" {
		updates["base_url"] = req.BaseURL
	}
	if req.APIKey != "" {
		updates["api_key"] = security.EncryptAPIKey(req.APIKey)
	}
	if req.Provider != "" {
		updates["provider"] = req.Provider
	}
	if req.ModelName != "" {
		updates["model_name"] = req.ModelName
	}
	if req.DisplayName != "" {
		updates["display_name"] = req.DisplayName
	}
	if req.MaxTokens > 0 {
		updates["max_tokens"] = req.MaxTokens
	}
	if req.Temperature > 0 {
		updates["temperature"] = req.Temperature
	}

	if len(updates) > 0 {
		h.db.Model(&m).Updates(updates)
	}

	// Reload to return fresh data
	h.db.Where("id = ?", id).First(&m)
	c.JSON(http.StatusOK, m)
}

func (h *ModelHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetString("user_id")

	result := h.db.Where("id = ? AND user_id = ?", id, userID).Delete(&model.ModelConfig{})
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "model config not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "model config deleted"})
}
