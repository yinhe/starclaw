package infra

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw/internal/model"
	"gorm.io/gorm"
)

// IntegrationHandler manages messaging platform integrations (Feishu, DingTalk, Slack, etc.)
type IntegrationHandler struct {
	db *gorm.DB
}

func NewIntegrationHandler(db *gorm.DB) *IntegrationHandler {
	return &IntegrationHandler{db: db}
}

// List returns all integrations for the current user
func (h *IntegrationHandler) List(c *gin.Context) {
	userID := c.GetString("user_id")
	typeFilter := c.Query("type")

	query := h.db.Where("user_id = ?", userID)
	if typeFilter != "" {
		query = query.Where("type = ?", typeFilter)
	}

	var integrations []model.Integration
	if err := query.Order("created_at DESC").Find(&integrations).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	// Mask secrets in response
	for i := range integrations {
		integrations[i].Config = maskConfig(integrations[i].Type, integrations[i].Config)
	}

	c.JSON(http.StatusOK, gin.H{"integrations": integrations})
}

// Create adds a new integration
func (h *IntegrationHandler) Create(c *gin.Context) {
	userID := c.GetString("user_id")

	var req struct {
		Type   model.IntegrationType `json:"type" binding:"required"`
		Name   string                `json:"name" binding:"required"`
		Config json.RawMessage       `json:"config" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	// Validate integration type
	if !isValidIntegrationType(req.Type) {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("不支持的集成类型: %s", req.Type)})
		return
	}

	// Validate config structure
	if err := validateConfig(req.Type, req.Config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	integration := model.Integration{
		UserID:  userID,
		Type:    req.Type,
		Name:    req.Name,
		Config:  string(req.Config),
		Enabled: true,
	}

	if err := h.db.Create(&integration).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建失败"})
		return
	}

	integration.Config = maskConfig(integration.Type, integration.Config)
	c.JSON(http.StatusCreated, gin.H{"integration": integration})
}

// Update modifies an existing integration
func (h *IntegrationHandler) Update(c *gin.Context) {
	userID := c.GetString("user_id")
	id := c.Param("id")

	var integration model.Integration
	if err := h.db.Where("id = ? AND user_id = ?", id, userID).First(&integration).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "集成不存在"})
		return
	}

	var req struct {
		Name    *string          `json:"name"`
		Config  *json.RawMessage `json:"config"`
		Enabled *bool            `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	updates := map[string]interface{}{}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}
	if req.Config != nil {
		if err := validateConfig(integration.Type, *req.Config); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		updates["config"] = string(*req.Config)
	}

	if err := h.db.Model(&integration).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败"})
		return
	}

	h.db.First(&integration, "id = ?", id)
	integration.Config = maskConfig(integration.Type, integration.Config)
	c.JSON(http.StatusOK, gin.H{"integration": integration})
}

// Delete removes an integration
func (h *IntegrationHandler) Delete(c *gin.Context) {
	userID := c.GetString("user_id")
	id := c.Param("id")

	result := h.db.Where("id = ? AND user_id = ?", id, userID).Delete(&model.Integration{})
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "集成不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

// Test verifies the integration credentials are valid
func (h *IntegrationHandler) Test(c *gin.Context) {
	userID := c.GetString("user_id")
	id := c.Param("id")

	var integration model.Integration
	if err := h.db.Where("id = ? AND user_id = ?", id, userID).First(&integration).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "集成不存在"})
		return
	}

	switch integration.Type {
	case model.IntegrationFeishu:
		result, err := testFeishu(integration.Config)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"success": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "message": result})

	case model.IntegrationDingtalk:
		result, err := testDingtalk(integration.Config)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"success": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "message": result})

	default:
		c.JSON(http.StatusOK, gin.H{"success": false, "error": fmt.Sprintf("暂不支持测试 %s 类型的集成", integration.Type)})
	}
}

// --- Helpers ---

func isValidIntegrationType(t model.IntegrationType) bool {
	switch t {
	case model.IntegrationFeishu, model.IntegrationDingtalk, model.IntegrationWeCom,
		model.IntegrationSlack, model.IntegrationDiscord, model.IntegrationTelegram:
		return true
	}
	return false
}

func validateConfig(t model.IntegrationType, raw json.RawMessage) error {
	switch t {
	case model.IntegrationFeishu:
		var cfg model.FeishuConfig
		if err := json.Unmarshal(raw, &cfg); err != nil {
			return fmt.Errorf("飞书配置格式错误: %w", err)
		}
		if cfg.AppID == "" && cfg.WebhookURL == "" {
			return fmt.Errorf("请至少配置 App ID + App Secret 或 Webhook URL")
		}
		if cfg.AppID != "" && cfg.AppSecret == "" {
			return fmt.Errorf("配置了 App ID 时，App Secret 不能为空")
		}
	case model.IntegrationDingtalk:
		var cfg model.DingtalkConfig
		if err := json.Unmarshal(raw, &cfg); err != nil {
			return fmt.Errorf("钉钉配置格式错误: %w", err)
		}
		if cfg.AppKey == "" && cfg.WebhookURL == "" {
			return fmt.Errorf("请至少配置 App Key + App Secret 或 Webhook URL")
		}
	case model.IntegrationSlack:
		var cfg model.SlackConfig
		if err := json.Unmarshal(raw, &cfg); err != nil {
			return fmt.Errorf("Slack 配置格式错误: %w", err)
		}
		if cfg.BotToken == "" && cfg.WebhookURL == "" {
			return fmt.Errorf("请至少配置 Bot Token 或 Webhook URL")
		}
	}
	return nil
}

func maskConfig(t model.IntegrationType, configJSON string) string {
	switch t {
	case model.IntegrationFeishu:
		var cfg model.FeishuConfig
		if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
			return configJSON
		}
		cfg.AppSecret = maskString(cfg.AppSecret)
		b, _ := json.Marshal(cfg)
		return string(b)

	case model.IntegrationDingtalk:
		var cfg model.DingtalkConfig
		if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
			return configJSON
		}
		cfg.AppSecret = maskString(cfg.AppSecret)
		cfg.SignSecret = maskString(cfg.SignSecret)
		b, _ := json.Marshal(cfg)
		return string(b)

	case model.IntegrationSlack:
		var cfg model.SlackConfig
		if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
			return configJSON
		}
		cfg.BotToken = maskString(cfg.BotToken)
		b, _ := json.Marshal(cfg)
		return string(b)

	case model.IntegrationDiscord:
		var cfg model.DiscordConfig
		if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
			return configJSON
		}
		cfg.BotToken = maskString(cfg.BotToken)
		b, _ := json.Marshal(cfg)
		return string(b)

	case model.IntegrationTelegram:
		var cfg model.TelegramConfig
		if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
			return configJSON
		}
		cfg.BotToken = maskString(cfg.BotToken)
		b, _ := json.Marshal(cfg)
		return string(b)

	case model.IntegrationWeCom:
		var cfg model.WeComConfig
		if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
			return configJSON
		}
		cfg.Secret = maskString(cfg.Secret)
		b, _ := json.Marshal(cfg)
		return string(b)
	}
	return configJSON
}

func maskString(s string) string {
	if s == "" {
		return ""
	}
	if len(s) <= 8 {
		return strings.Repeat("*", len(s))
	}
	return s[:4] + strings.Repeat("*", len(s)-8) + s[len(s)-4:]
}

// testFeishu tests Feishu credentials by fetching tenant_access_token
func testFeishu(configJSON string) (string, error) {
	var cfg model.FeishuConfig
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return "", fmt.Errorf("配置解析失败")
	}

	if cfg.AppID != "" && cfg.AppSecret != "" {
		body := fmt.Sprintf(`{"app_id":"%s","app_secret":"%s"}`, cfg.AppID, cfg.AppSecret)
		resp, err := (&http.Client{Timeout: 10 * time.Second}).Post(
			"https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal",
			"application/json; charset=utf-8",
			strings.NewReader(body),
		)
		if err != nil {
			return "", fmt.Errorf("连接飞书失败: %w", err)
		}
		defer resp.Body.Close()
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 5*1024))

		var result struct {
			Code int    `json:"code"`
			Msg  string `json:"msg"`
		}
		json.Unmarshal(respBody, &result)
		if result.Code != 0 {
			return "", fmt.Errorf("认证失败 (code=%d): %s", result.Code, result.Msg)
		}
		return "飞书应用凭证验证成功 ✓", nil
	}

	if cfg.WebhookURL != "" {
		return "Webhook URL 已配置（发送时验证）", nil
	}

	return "", fmt.Errorf("未配置有效凭证")
}

// testDingtalk tests DingTalk credentials
func testDingtalk(configJSON string) (string, error) {
	var cfg model.DingtalkConfig
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return "", fmt.Errorf("配置解析失败")
	}

	if cfg.AppKey != "" && cfg.AppSecret != "" {
		body := fmt.Sprintf(`{"appKey":"%s","appSecret":"%s"}`, cfg.AppKey, cfg.AppSecret)
		resp, err := (&http.Client{Timeout: 10 * time.Second}).Post(
			"https://api.dingtalk.com/v1.0/oauth2/accessToken",
			"application/json",
			strings.NewReader(body),
		)
		if err != nil {
			return "", fmt.Errorf("连接钉钉失败: %w", err)
		}
		defer resp.Body.Close()
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 5*1024))

		var result struct {
			AccessToken string `json:"accessToken"`
			ErrCode     int    `json:"errcode"`
			ErrMsg      string `json:"errmsg"`
		}
		json.Unmarshal(respBody, &result)
		if result.AccessToken == "" {
			return "", fmt.Errorf("认证失败: %s", string(respBody))
		}
		return "钉钉应用凭证验证成功 ✓", nil
	}

	if cfg.WebhookURL != "" {
		return "Webhook URL 已配置（发送时验证）", nil
	}

	return "", fmt.Errorf("未配置有效凭证")
}
