package v1

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/yinhe/starclaw/internal/billing"
	"github.com/yinhe/starclaw/internal/model"
	"gorm.io/gorm"
)

const queenMarketplaceURL = "https://starclaw.net/api/marketplace"

type TemplateHandler struct {
	db      *gorm.DB
	billing *billing.QueenClient // optional, nil when billing is disabled
}

func NewTemplateHandler(db *gorm.DB, billingClient *billing.QueenClient) *TemplateHandler {
	return &TemplateHandler{db: db, billing: billingClient}
}

// pricingInfo describes pricing embedded in a template's config JSON.
type pricingInfo struct {
	Type     string `json:"type"`     // free, one_time, subscription
	Price    int    `json:"price"`    // cents (分)
	Period   string `json:"period"`   // month, quarter, year
	Currency string `json:"currency"` // CNY
	Display  string `json:"display"`  // e.g. "¥2,999/季度"
}

// parsePricing extracts pricing info from a template's config JSON.
func parsePricing(configJSON string) *pricingInfo {
	var cfg struct {
		Pricing *pricingInfo `json:"pricing"`
	}
	if json.Unmarshal([]byte(configJSON), &cfg) != nil || cfg.Pricing == nil {
		return nil
	}
	if cfg.Pricing.Type == "" || cfg.Pricing.Type == "free" {
		return nil
	}
	if cfg.Pricing.Currency == "" {
		cfg.Pricing.Currency = "CNY"
	}
	return cfg.Pricing
}

// expiresFromPeriod calculates expiry time from a subscription period.
func expiresFromPeriod(period string) *time.Time {
	var d time.Duration
	switch period {
	case "month":
		d = 30 * 24 * time.Hour
	case "quarter":
		d = 90 * 24 * time.Hour
	case "year":
		d = 365 * 24 * time.Hour
	default:
		return nil // one_time = permanent
	}
	t := time.Now().Add(d)
	return &t
}

// List returns marketplace templates with optional category/search filter
func (h *TemplateHandler) List(c *gin.Context) {
	category := c.Query("category")
	search := c.Query("q")
	featured := c.Query("featured")

	q := h.db.Preload("Author", func(db *gorm.DB) *gorm.DB {
		return db.Select("id, username")
	}).Order("featured DESC, install_count DESC, created_at DESC")

	if category != "" {
		q = q.Where("category = ?", category)
	}
	if search != "" {
		like := "%" + search + "%"
		q = q.Where("name LIKE ? OR description LIKE ?", like, like)
	}
	if featured == "true" {
		q = q.Where("featured = ?", true)
	}

	var templates []model.AgentTemplate
	if err := q.Limit(100).Find(&templates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list templates"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"templates": templates})
}

// Get returns a single template by ID
func (h *TemplateHandler) Get(c *gin.Context) {
	id := c.Param("id")
	var tpl model.AgentTemplate
	if err := h.db.Preload("Author", func(db *gorm.DB) *gorm.DB {
		return db.Select("id, username")
	}).First(&tpl, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "template not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"template": tpl})
}

// Publish creates a template from the user's agent
func (h *TemplateHandler) Publish(c *gin.Context) {
	userID := c.GetString("user_id")

	var req struct {
		AgentID     string `json:"agent_id" binding:"required"`
		Category    string `json:"category"`
		Tags        string `json:"tags"`
		Icon        string `json:"icon"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var agent model.Agent
	if err := h.db.Where("id = ? AND user_id = ?", req.AgentID, userID).First(&agent).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
		return
	}

	desc := req.Description
	if desc == "" {
		desc = agent.Description
	}

	tpl := model.AgentTemplate{
		AuthorID:     userID,
		Name:         agent.Name,
		Description:  desc,
		Category:     req.Category,
		Tags:         req.Tags,
		SystemPrompt: agent.SystemPrompt,
		Tools:        agent.Tools,
		Config:       agent.Config,
		Icon:         req.Icon,
	}
	if tpl.Tags == "" {
		tpl.Tags = "[]"
	}
	if tpl.Category == "" {
		tpl.Category = "assistant"
	}

	if err := h.db.Create(&tpl).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to publish template"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"template": tpl})
}

// Install creates an agent from a template for the current user
func (h *TemplateHandler) Install(c *gin.Context) {
	userID := c.GetString("user_id")
	tplID := c.Param("id")

	var tpl model.AgentTemplate
	if err := h.db.First(&tpl, "id = ?", tplID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "template not found"})
		return
	}

	agent := model.Agent{
		UserID:       userID,
		Name:         tpl.Name,
		Description:  tpl.Description,
		SystemPrompt: tpl.SystemPrompt,
		Tools:        tpl.Tools,
		Config:       tpl.Config,
	}
	if err := h.db.Create(&agent).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to install template"})
		return
	}

	// Increment install count
	h.db.Model(&tpl).UpdateColumn("install_count", gorm.Expr("install_count + 1"))

	// A2: Creator notification — notify template author about installation
	if tpl.AuthorID != "" && tpl.AuthorID != userID {
		h.db.Create(&model.Notification{
			UserID: tpl.AuthorID,
			Type:   model.NotifySuccess,
			Title:  fmt.Sprintf("你的「%s」又被 1 人安装了！(累计 %d 次)", tpl.Name, tpl.InstallCount+1),
		})
	}

	c.JSON(http.StatusCreated, gin.H{"agent": agent})
}

// installBundleSkill mirrors Queen's AgentSkillSpec for JSON parsing.
type installBundleSkill struct {
	Name    string `json:"name"`
	Spec    string `json:"spec"`
	Version string `json:"version"`
}

// installBundleMCP mirrors Queen's AgentMCPSpec for JSON parsing.
type installBundleMCP struct {
	Name        string `json:"name"`
	BaseURL     string `json:"base_url"`
	Description string `json:"description"`
	Tools       string `json:"tools"`
}

// installBundleWorkflow mirrors Queen's AgentWorkflowSpec for JSON parsing.
type installBundleWorkflow struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Definition  string `json:"definition"`
}

// installBundlePlugin mirrors Queen's AgentPluginSpec for JSON parsing.
type installBundlePlugin struct {
	Name string `json:"name"` // e.g. "trading_scan"
	Spec string `json:"spec"` // full JSON plugin definition
}

// InstallRemote creates an agent from a remote marketplace template (e.g. from Queen/starclaw.net).
// If the config contains skills/mcp_servers/workflows, they are installed as a bundle.
func (h *TemplateHandler) InstallRemote(c *gin.Context) {
	userID := c.GetString("user_id")

	var req struct {
		Name         string `json:"name" binding:"required"`
		Description  string `json:"description"`
		SystemPrompt string `json:"system_prompt"`
		Tools        string `json:"tools"`
		Config       string `json:"config"`
		Icon         string `json:"icon"`
		Source       string `json:"source"` // e.g. "starclaw.net"
		SourceID     string `json:"source_id"`
		// Full installation bundle (optional)
		Skills     []installBundleSkill    `json:"skills"`
		MCPServers []installBundleMCP      `json:"mcp_servers"`
		Workflows  []installBundleWorkflow `json:"workflows"`
		Plugins    []installBundlePlugin   `json:"plugins"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tx := h.db.Begin()

	// 1. Create agent
	agent := model.Agent{
		UserID:       userID,
		Name:         req.Name,
		Description:  req.Description,
		SystemPrompt: req.SystemPrompt,
		Tools:        req.Tools,
		Config:       req.Config,
		SourceID:     req.SourceID,
	}
	if err := tx.Create(&agent).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to install agent"})
		return
	}

	// 2. Install skills (passive + active)
	skillsInstalled := 0
	for _, s := range req.Skills {
		skill := model.AgentSkill{
			AgentID:   agent.ID,
			SkillName: s.Name,
			SkillSpec: s.Spec,
			Version:   s.Version,
		}
		if err := tx.Create(&skill).Error; err == nil {
			skillsInstalled++
		}
	}

	// 3. Register MCP servers
	mcpInstalled := 0
	for _, m := range req.MCPServers {
		mcp := model.MCPServer{
			UserID:  userID,
			Name:    m.Name,
			BaseURL: m.BaseURL,
			Status:  "active",
		}
		if err := tx.Create(&mcp).Error; err == nil {
			mcpInstalled++
		}
	}

	// 4. Create workflows
	wfInstalled := 0
	for _, w := range req.Workflows {
		wf := model.Workflow{
			UserID:      userID,
			Name:        w.Name,
			Description: w.Description + " [agent:" + agent.Name + "]",
			Definition:  w.Definition,
		}
		if err := tx.Create(&wf).Error; err == nil {
			wfInstalled++
		}
	}

	tx.Commit()

	// 5. Install JSON tool plugins (write to plugins/ directory, outside tx)
	pluginsInstalled := 0
	if len(req.Plugins) > 0 {
		pluginDir := "plugins"
		_ = os.MkdirAll(pluginDir, 0755)
		for _, p := range req.Plugins {
			if p.Name == "" || p.Spec == "" {
				continue
			}
			path := pluginDir + "/" + p.Name + ".json"
			if err := os.WriteFile(path, []byte(p.Spec), 0644); err == nil {
				pluginsInstalled++
			}
		}
	}

	c.JSON(http.StatusCreated, gin.H{
		"agent":               agent,
		"skills_installed":    skillsInstalled,
		"mcp_installed":       mcpInstalled,
		"workflows_installed": wfInstalled,
		"plugins_installed":   pluginsInstalled,
	})
}

// CommunityList returns marketplace items: local templates first (authoritative
// pricing/bundle data), then merges Queen community items excluding duplicates.
func (h *TemplateHandler) CommunityList(c *gin.Context) {
	q := c.Query("q")

	// 1. Always load local templates first (builtin have pricing, bundle, etc.)
	var items []gin.H
	localNames := make(map[string]bool)

	var templates []model.AgentTemplate
	tq := h.db.Preload("Author").Order("featured DESC, install_count DESC, created_at DESC").Limit(200)
	if q != "" {
		tq = tq.Where("name LIKE ? OR description LIKE ?", "%"+q+"%", "%"+q+"%")
	}
	tq.Find(&templates)
	for _, t := range templates {
		authorName := "StarClaw 官方"
		if t.Author.Username != "" {
			authorName = t.Author.Username
		}
		if strings.Contains(t.Name, "Q8bot") || strings.Contains(t.Name, "麒博") {
			authorName = "Q8bot官方"
		}
		if strings.Contains(t.Name, "Cicada") || strings.Contains(t.Name, "蝉") {
			authorName = "StarClaw 官方"
		}
		localNames[t.Name] = true
		items = append(items, gin.H{
			"id": t.ID, "name": t.Name, "description": t.Description,
			"icon": t.Icon, "tags": t.Tags, "config": t.Config,
			"downloads": t.InstallCount, "rating": t.Rating,
			"author": gin.H{"nickname": authorName},
		})
	}

	// 2. Include builtin agents not already covered by templates
	var agents []model.Agent
	h.db.Where("is_builtin = ? AND is_public = ?", true, true).Find(&agents)
	for _, a := range agents {
		if localNames[a.Name] || a.Name == "全能助手" {
			continue
		}
		localNames[a.Name] = true
		items = append(items, gin.H{
			"id": a.ID, "name": a.Name, "description": a.Description,
			"icon": "", "tags": "[]", "config": fmt.Sprintf(`{"system_prompt":%q,"tools":%s}`, a.SystemPrompt, a.Tools),
			"downloads": 0, "rating": 0,
			"author": gin.H{"nickname": "StarClaw 官方"},
		})
	}

	// 3. Merge Queen community items (skip names that exist locally)
	queenURL := fmt.Sprintf("%s/items?type=agent&size=500", queenMarketplaceURL)
	if q != "" {
		queenURL += "&q=" + q
	}
	client := &http.Client{Timeout: 8 * time.Second}
	if resp, err := client.Get(queenURL); err == nil {
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			if body, err2 := io.ReadAll(resp.Body); err2 == nil {
				var result struct {
					Items []json.RawMessage `json:"items"`
				}
				if json.Unmarshal(body, &result) == nil {
					for _, raw := range result.Items {
						var peek struct {
							Name string `json:"name"`
						}
						if json.Unmarshal(raw, &peek) == nil && !localNames[peek.Name] {
							var item map[string]interface{}
							if json.Unmarshal(raw, &item) == nil {
								items = append(items, gin.H(item))
								localNames[peek.Name] = true
							}
						}
					}
				}
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"items": items, "total": len(items)})
}

// CommunityGet returns a single marketplace item.
// Local templates are checked FIRST (authoritative pricing/bundle data),
// then Queen marketplace, then local agents as last fallback.
func (h *TemplateHandler) CommunityGet(c *gin.Context) {
	id := c.Param("id")

	// 1. Try local template first (builtin templates have pricing, bundle, etc.)
	var tpl model.AgentTemplate
	if err := h.db.Preload("Author").Where("id = ?", id).First(&tpl).Error; err == nil {
		authorName := "StarClaw 官方"
		if tpl.Author.Username != "" {
			authorName = tpl.Author.Username
		}
		if strings.Contains(tpl.Name, "Q8bot") || strings.Contains(tpl.Name, "麒博") {
			authorName = "Q8bot官方"
		}
		c.JSON(http.StatusOK, gin.H{
			"item": gin.H{
				"id": tpl.ID, "name": tpl.Name, "description": tpl.Description,
				"icon": tpl.Icon, "tags": tpl.Tags, "config": tpl.Config,
				"downloads": tpl.InstallCount, "rating": tpl.Rating,
				"author": gin.H{"nickname": authorName},
			},
		})
		return
	}

	// 2. Try local agent
	var agent model.Agent
	if err := h.db.Where("id = ?", id).First(&agent).Error; err == nil {
		c.JSON(http.StatusOK, gin.H{
			"item": gin.H{
				"id": agent.ID, "name": agent.Name, "description": agent.Description,
				"icon": "", "tags": "[]", "config": fmt.Sprintf(`{"system_prompt":%q,"tools":%s}`, agent.SystemPrompt, agent.Tools),
				"downloads": 0, "rating": 0,
				"author": gin.H{"nickname": "StarClaw 官方"},
			},
		})
		return
	}

	// 3. Try Queen marketplace (for community items with Queen IDs)
	queenURL := fmt.Sprintf("%s/items/%s", queenMarketplaceURL, id)
	client := &http.Client{Timeout: 8 * time.Second}
	if resp, err := client.Get(queenURL); err == nil {
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			if body, err2 := io.ReadAll(resp.Body); err2 == nil {
				var result map[string]interface{}
				if json.Unmarshal(body, &result) == nil {
					c.JSON(http.StatusOK, result)
					return
				}
			}
		}
	}

	c.JSON(http.StatusNotFound, gin.H{"error": "item not found"})
}

// Rate adds a rating to a template
func (h *TemplateHandler) Rate(c *gin.Context) {
	tplID := c.Param("id")
	var req struct {
		Rating float64 `json:"rating" binding:"required,min=1,max=5"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var tpl model.AgentTemplate
	if err := h.db.First(&tpl, "id = ?", tplID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "template not found"})
		return
	}

	// Simple running average
	newCount := tpl.RatingCount + 1
	newRating := (tpl.Rating*float64(tpl.RatingCount) + req.Rating) / float64(newCount)

	h.db.Model(&tpl).Updates(map[string]interface{}{
		"rating":       newRating,
		"rating_count": newCount,
	})

	// A2: Creator notification — notify on good ratings (≥4)
	userID := c.GetString("user_id")
	if tpl.AuthorID != "" && tpl.AuthorID != userID && req.Rating >= 4 {
		h.db.Create(&model.Notification{
			UserID: tpl.AuthorID,
			Type:   model.NotifySuccess,
			Title:  fmt.Sprintf("你的「%s」收到了 %.0f 星好评！(均分 %.1f)", tpl.Name, req.Rating, newRating),
		})
	}

	c.JSON(http.StatusOK, gin.H{"rating": newRating, "rating_count": newCount})
}

// Categories returns available categories
func (h *TemplateHandler) Categories(c *gin.Context) {
	categories := []gin.H{
		{"id": "assistant", "name": "通用助手", "name_en": "Assistant", "icon": "Bot"},
		{"id": "coding", "name": "编程开发", "name_en": "Coding", "icon": "Code2"},
		{"id": "writing", "name": "写作创作", "name_en": "Writing", "icon": "PenTool"},
		{"id": "data", "name": "数据分析", "name_en": "Data Analysis", "icon": "BarChart3"},
		{"id": "creative", "name": "创意设计", "name_en": "Creative", "icon": "Palette"},
		{"id": "devops", "name": "运维部署", "name_en": "DevOps", "icon": "Server"},
		{"id": "research", "name": "学术研究", "name_en": "Research", "icon": "BookOpen"},
		{"id": "business", "name": "商业办公", "name_en": "Business", "icon": "Briefcase"},
	}
	c.JSON(http.StatusOK, gin.H{"categories": categories})
}

// ── Paid Marketplace Purchases (Direct Alipay/WeChat) ──

// Purchase creates a direct Alipay/WeChat payment order via Queen → Synapse.
// Flow: parse pricing → create payment order → return pay_url → frontend opens payment page.
// After payment, frontend polls PollPurchaseStatus → when paid, agent is auto-installed.
func (h *TemplateHandler) Purchase(c *gin.Context) {
	userID := c.GetString("user_id")
	templateID := c.Param("id")

	var req struct {
		PayMethod string `json:"pay_method"` // alipay / wechatpay (default: alipay)
	}
	c.ShouldBindJSON(&req)
	if req.PayMethod == "" {
		req.PayMethod = "alipay"
	}

	// 1. Load template
	configJSON, templateName, _ := h.loadTemplateInfo(templateID)
	if configJSON == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "template not found"})
		return
	}

	// 2. Parse pricing
	pricing := parsePricing(configJSON)
	if pricing == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "this template is free, use install instead"})
		return
	}

	// 3. Check if user already has an active purchase
	var existing model.MarketplacePurchase
	if err := h.db.Where("user_id = ? AND template_id = ? AND status = ?", userID, templateID, "active").First(&existing).Error; err == nil {
		if existing.IsActive() {
			c.JSON(http.StatusConflict, gin.H{"error": "already_purchased", "purchase": existing})
			return
		}
		h.db.Model(&existing).Update("status", "expired")
	}

	// 4. Check billing is enabled
	if h.billing == nil || !h.billing.IsEnabled() {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "billing_unavailable",
			"message": "计费系统未连接，请先加入 StarClaw 网络（设置 → 加入星爪网络）",
		})
		return
	}

	// 5. Create payment order via Queen → Synapse (Alipay/WeChat)
	priceFen := int64(pricing.Price)
	result, err := h.billing.CreateMarketplacePayment(userID, templateID, templateName, req.PayMethod, priceFen)
	if err != nil {
		log.Printf("[marketplace] create payment failed: user=%s template=%s err=%v", userID, templateID, err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "payment_failed", "message": "创建支付订单失败: " + err.Error()})
		return
	}

	// 6. Create local pending purchase record (will be activated after payment)
	purchase := model.MarketplacePurchase{
		UserID:       userID,
		TemplateID:   templateID,
		TemplateName: templateName,
		PricingType:  pricing.Type,
		PriceCents:   pricing.Price,
		Period:       pricing.Period,
		Currency:     pricing.Currency,
		Status:       "pending_payment",
		PurchasedAt:  time.Now(),
	}
	h.db.Create(&purchase)

	log.Printf("[marketplace] payment order created: order=%s user=%s template=%s(%s) method=%s",
		result.OrderNo, userID, templateID, templateName, req.PayMethod)

	c.JSON(http.StatusOK, gin.H{
		"order_no":      result.OrderNo,
		"pay_url":       result.PayURL,
		"code_url":      result.CodeURL,
		"pay_method":    result.PayMethod,
		"amount_yuan":   result.AmountYuan,
		"price_display": pricing.Display,
		"purchase_id":   purchase.ID,
	})
}

// PollPurchaseStatus checks whether a marketplace payment has been completed.
// Frontend polls this every 3s after opening the payment URL.
// When status=paid, the agent is auto-installed and returned.
func (h *TemplateHandler) PollPurchaseStatus(c *gin.Context) {
	userID := c.GetString("user_id")
	orderNo := c.Param("order_no")

	if h.billing == nil || !h.billing.IsEnabled() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "error", "message": "billing not connected"})
		return
	}

	// Query Queen for order status
	status, err := h.billing.QueryMarketplaceOrder(orderNo)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "order not found"})
		return
	}

	if status.Status != "paid" {
		c.JSON(http.StatusOK, gin.H{"status": status.Status})
		return
	}

	// Payment confirmed! Auto-install the agent
	templateID := status.TemplateID
	configJSON, templateName, templateDesc := h.loadTemplateInfo(templateID)

	// Update local purchase record
	var purchase model.MarketplacePurchase
	if err := h.db.Where("user_id = ? AND template_id = ? AND status = ?", userID, templateID, "pending_payment").
		First(&purchase).Error; err == nil {
		pricing := parsePricing(configJSON)
		var expiresAt *time.Time
		if pricing != nil && pricing.Type == "subscription" {
			expiresAt = expiresFromPeriod(pricing.Period)
		}
		h.db.Model(&purchase).Updates(map[string]interface{}{
			"status":     "active",
			"expires_at": expiresAt,
		})
	}

	// Install the agent
	var cfg struct {
		SystemPrompt string `json:"system_prompt"`
		Tools        string `json:"tools"`
	}
	json.Unmarshal([]byte(configJSON), &cfg)

	agent := model.Agent{
		UserID:       userID,
		Name:         templateName,
		Description:  templateDesc,
		SystemPrompt: cfg.SystemPrompt,
		Tools:        cfg.Tools,
		Config:       configJSON,
		SourceID:     templateID,
	}

	var agentID string
	if err := h.db.Create(&agent).Error; err == nil {
		agentID = agent.ID
		h.db.Model(&purchase).Update("agent_id", agentID)
		h.db.Model(&model.AgentTemplate{}).Where("id = ?", templateID).
			UpdateColumn("install_count", gorm.Expr("install_count + 1"))
	}

	log.Printf("[marketplace] payment complete, agent installed: order=%s template=%s agent=%s", orderNo, templateName, agentID)

	c.JSON(http.StatusOK, gin.H{
		"status":   "paid",
		"agent_id": agentID,
		"message":  fmt.Sprintf("支付成功，已安装「%s」", templateName),
	})
}

// loadTemplateInfo loads template config, name, description from local DB.
func (h *TemplateHandler) loadTemplateInfo(templateID string) (configJSON, name, desc string) {
	var tpl model.AgentTemplate
	if err := h.db.Where("id = ?", templateID).First(&tpl).Error; err == nil {
		return tpl.Config, tpl.Name, tpl.Description
	}
	var agent model.Agent
	if err := h.db.Where("id = ?", templateID).First(&agent).Error; err == nil {
		cfg := fmt.Sprintf(`{"system_prompt":%q,"tools":%s}`, agent.SystemPrompt, agent.Tools)
		return cfg, agent.Name, agent.Description
	}
	return "", "", ""
}

// ListPurchases returns the user's marketplace purchases.
func (h *TemplateHandler) ListPurchases(c *gin.Context) {
	userID := c.GetString("user_id")
	status := c.DefaultQuery("status", "active")

	var purchases []model.MarketplacePurchase
	q := h.db.Where("user_id = ?", userID).Order("purchased_at DESC")
	if status != "all" {
		q = q.Where("status = ?", status)
	}
	q.Limit(100).Find(&purchases)

	// Mark expired subscriptions
	now := time.Now()
	for i, p := range purchases {
		if p.Status == "active" && p.ExpiresAt != nil && p.ExpiresAt.Before(now) {
			h.db.Model(&p).Update("status", "expired")
			purchases[i].Status = "expired"
		}
	}

	c.JSON(http.StatusOK, gin.H{"purchases": purchases})
}

// CheckAccess returns whether the user has an active purchase for a template.
func (h *TemplateHandler) CheckAccess(c *gin.Context) {
	userID := c.GetString("user_id")
	templateID := c.Param("id")

	var purchase model.MarketplacePurchase
	err := h.db.Where("user_id = ? AND template_id = ? AND status = ?", userID, templateID, "active").
		Order("purchased_at DESC").First(&purchase).Error

	if err != nil {
		c.JSON(http.StatusOK, gin.H{"has_access": false})
		return
	}

	active := purchase.IsActive()
	if !active {
		// Auto-expire
		h.db.Model(&purchase).Update("status", "expired")
	}

	c.JSON(http.StatusOK, gin.H{
		"has_access": active,
		"purchase":   purchase,
	})
}

// SeedBuiltinTemplates seeds default templates on first run and adds new ones incrementally.
func SeedBuiltinTemplates(db *gorm.DB) {
	templates := []model.AgentTemplate{
		{
			ID:           uuid.New().String(),
			AuthorID:     model.SystemUserID,
			Name:         "全栈开发助手",
			Description:  "精通前后端开发的全栈工程师，擅长 React/Vue/Go/Python/Node.js，能够帮你设计架构、编写代码、调试问题。",
			Category:     "coding",
			Tags:         `["fullstack","react","go","python","nodejs"]`,
			SystemPrompt: "你是一位经验丰富的全栈开发工程师，精通以下技术栈：\n- 前端：React, Vue.js, TypeScript, Tailwind CSS\n- 后端：Go (Gin), Python (FastAPI), Node.js (Express)\n- 数据库：MySQL, PostgreSQL, Redis, MongoDB\n- DevOps：Docker, Kubernetes, CI/CD\n\n你的工作方式：\n1. 先理解需求，确认技术选型\n2. 给出清晰的架构设计\n3. 编写高质量、可维护的代码\n4. 考虑错误处理和边界情况\n5. 提供测试建议",
			Tools:        `["web_search","code_sandbox","browser"]`,
			Config:       `{"temperature":0.3,"max_tokens":4096}`,
			Icon:         "Code2",
			Featured:     true,
			IsBuiltin:    true,
		},
		{
			ID:           uuid.New().String(),
			AuthorID:     model.SystemUserID,
			Name:         "学术论文助手",
			Description:  "帮助撰写、润色和翻译学术论文，支持 APA/MLA/Chicago 引用格式，提供文献综述和研究方法指导。",
			Category:     "research",
			Tags:         `["academic","paper","research","translation"]`,
			SystemPrompt: "你是一位资深学术写作助手，具有以下能力：\n- 帮助撰写学术论文各部分（摘要、引言、方法、结果、讨论）\n- 润色英文学术写作，提升语言质量\n- 支持 APA, MLA, Chicago 引用格式\n- 提供文献综述框架和研究方法建议\n- 中英文学术翻译\n\n请始终保持学术严谨性，注明引用来源，避免抄袭。",
			Tools:        `["web_search"]`,
			Config:       `{"temperature":0.2,"max_tokens":4096}`,
			Icon:         "BookOpen",
			Featured:     true,
			IsBuiltin:    true,
		},
		{
			ID:           uuid.New().String(),
			AuthorID:     model.SystemUserID,
			Name:         "数据分析师",
			Description:  "专业数据分析师，能够帮你进行数据清洗、可视化、统计分析和机器学习建模。支持 Python/SQL。",
			Category:     "data",
			Tags:         `["data","python","sql","visualization","ml"]`,
			SystemPrompt: "你是一位专业的数据分析师，精通：\n- Python 数据分析（Pandas, NumPy, Scikit-learn）\n- 数据可视化（Matplotlib, Seaborn, Plotly）\n- SQL 查询优化\n- 统计分析和假设检验\n- 机器学习建模\n\n工作流程：\n1. 理解数据和业务问题\n2. 数据探索和清洗\n3. 特征工程\n4. 分析/建模\n5. 可视化呈现结果\n6. 提供可执行的业务建议",
			Tools:        `["code_sandbox","web_search"]`,
			Config:       `{"temperature":0.2,"max_tokens":4096}`,
			Icon:         "BarChart3",
			Featured:     true,
			IsBuiltin:    true,
		},
		{
			ID:           uuid.New().String(),
			AuthorID:     model.SystemUserID,
			Name:         "创意写作家",
			Description:  "帮你创作小说、诗歌、剧本、广告文案等各类创意内容，支持多种风格和语调。",
			Category:     "writing",
			Tags:         `["creative","writing","copywriting","story"]`,
			SystemPrompt: "你是一位才华横溢的创意写作家，擅长：\n- 小说和短篇故事创作\n- 诗歌和散文\n- 广告文案和品牌故事\n- 剧本和对话写作\n- 社交媒体内容\n\n你能根据用户需求调整风格（幽默、正式、感性、简洁等），并且善于运用比喻、排比等修辞手法。每次创作前，先了解目标受众和使用场景。",
			Tools:        `["web_search"]`,
			Config:       `{"temperature":0.8,"max_tokens":4096}`,
			Icon:         "PenTool",
			Featured:     true,
			IsBuiltin:    true,
		},
		{
			ID:           uuid.New().String(),
			AuthorID:     model.SystemUserID,
			Name:         "DevOps 运维专家",
			Description:  "精通 Docker/K8s/CI/CD 的运维专家，帮你设计部署架构、编写配置文件、排查线上问题。",
			Category:     "devops",
			Tags:         `["docker","kubernetes","cicd","linux","monitoring"]`,
			SystemPrompt: "你是一位资深 DevOps 工程师，精通：\n- 容器化：Docker, Docker Compose, Podman\n- 编排：Kubernetes, Helm, ArgoCD\n- CI/CD：GitHub Actions, GitLab CI, Jenkins\n- 监控：Prometheus, Grafana, ELK\n- 云平台：AWS, GCP, 阿里云\n- Linux 系统管理和网络\n\n你注重：安全性、高可用、自动化、可观测性。提供生产级别的配置和最佳实践。",
			Tools:        `["code_sandbox","web_search"]`,
			Config:       `{"temperature":0.2,"max_tokens":4096}`,
			Icon:         "Server",
			Featured:     false,
			IsBuiltin:    true,
		},
		{
			ID:           uuid.New().String(),
			AuthorID:     model.SystemUserID,
			Name:         "产品经理助手",
			Description:  "帮你撰写 PRD、用户故事、竞品分析，进行需求优先级排序和产品规划。",
			Category:     "business",
			Tags:         `["product","prd","user_story","analysis"]`,
			SystemPrompt: "你是一位经验丰富的产品经理，擅长：\n- 撰写产品需求文档（PRD）\n- 用户故事编写和验收标准\n- 竞品分析和市场调研\n- 需求优先级排序（RICE/MoSCoW）\n- 产品路线图规划\n- 数据驱动决策\n\n你善于站在用户角度思考，用数据说话，并且能够清晰地与开发团队沟通技术可行性。",
			Tools:        `["web_search"]`,
			Config:       `{"temperature":0.4,"max_tokens":4096}`,
			Icon:         "Briefcase",
			Featured:     false,
			IsBuiltin:    true,
		},
		{
			ID:           uuid.New().String(),
			AuthorID:     model.SystemUserID,
			Name:         "UI/UX 设计顾问",
			Description:  "提供界面设计建议、配色方案、组件规范，帮你打造出色的用户体验。",
			Category:     "creative",
			Tags:         `["design","ui","ux","color","component"]`,
			SystemPrompt: "你是一位资深 UI/UX 设计顾问，精通：\n- 界面设计原则和设计系统\n- 配色理论和色彩搭配\n- 响应式设计和移动端适配\n- 用户体验研究和可用性测试\n- Figma/Sketch 组件规范\n- Tailwind CSS / shadcn/ui 实现\n\n你注重：\n1. 一致性和可访问性（WCAG）\n2. 视觉层次和信息架构\n3. 微交互和动画\n4. 设计到代码的高效转换",
			Tools:        `["web_search","browser"]`,
			Config:       `{"temperature":0.5,"max_tokens":4096}`,
			Icon:         "Palette",
			Featured:     false,
			IsBuiltin:    true,
		},
		{
			ID:           uuid.New().String(),
			AuthorID:     model.SystemUserID,
			Name:         "英语口语教练",
			Description:  "模拟真实对话场景练习英语口语，纠正语法错误，教授地道表达和俚语。",
			Category:     "assistant",
			Tags:         `["english","speaking","language","education"]`,
			SystemPrompt: "你是一位专业的英语口语教练。你的教学方法：\n1. 根据学生水平调整难度\n2. 模拟真实场景对话（面试、旅行、商务等）\n3. 指出语法和表达错误，给出正确示范\n4. 教授地道的英语表达和常用俚语\n5. 定期总结学习要点\n\n请用友好、鼓励的方式交流。每次对话后，简要总结学到的新表达。如果学生用中文提问，用中英双语回答。",
			Tools:        `[]`,
			Config:       `{"temperature":0.6,"max_tokens":2048}`,
			Icon:         "Bot",
			Featured:     false,
			IsBuiltin:    true,
		},
		{
			ID:          uuid.New().String(),
			AuthorID:    model.SystemUserID,
			Name:        "短剧导演",
			Description: "好莱坞风格 AI 短剧导演，从剧本构思到成片交付的一站式制作。擅长场景编排、镜头语言、配音字幕、音乐配乐的全流程把控。",
			Category:    "creative",
			Tags:        `["drama","video","director","hollywood","short_film","dubbing","subtitle","music"]`,
			SystemPrompt: `你是一位经验丰富的好莱坞级短剧导演（Director Agent），具备从创意构思到成片交付的全流程制作能力。

## 你的身份与风格
- 你以好莱坞一线导演的视角思考每一个镜头：构图、光影、色调、运镜、节奏
- 你追求电影级质感，每个画面都要有视觉冲击力和叙事张力
- 你善于用「展示」而非「告诉」来推动故事——画面会说话

## 制作工作流（严格按步骤执行）

### 第一步：剧本创作（Screenplay）
1. 与用户确认短剧主题、风格、时长（默认 30-60 秒）
2. 编写分场剧本，每场包含：
   - 场景编号 + 场景描述
   - 镜头说明（景别、运镜、光线）
   - 角色动作和表情
   - 旁白/对白文字
   - 配乐建议

### 第二步：视觉风格确定（Visual Style）
1. 确定全片统一的 style_prefix（例如：cinematic film style, dramatic lighting, shallow depth of field, warm color grading）
2. 确定视频尺寸（横屏 1280*720 / 竖屏 720*1280）
3. 确定每个场景的详细画面提示词（英文，电影级描述）

### 第三步：逐场景生成视频（Scene Production）
1. 为第一个场景调用 video_generation（action: generate_video），使用 style_prefix 保持风格一致
2. 为后续场景使用 ref_video_id 引用上一场景，自动提取尾帧实现画面衔接
3. 每个场景生成后用 check_status 确认完成
4. 所有场景完成后系统会自动合成最终视频

### 第四步：配音（Dubbing）
1. 根据剧本旁白，编写 narrations JSON（text + start/end 时间戳）
2. 选择合适音色：
   - 女声旁白推荐 longyuan（温柔知性）或 longwan（端庄大气）
   - 男声旁白推荐 longjing（播音腔）或 longfei（浑厚低沉）
   - 活泼内容推荐 longxiaochun（女）或 longshuo（男）
3. 调用 dubbing 工具的 add_voiceover 为合成视频添加配音

### 第五步：字幕（Subtitles）
如果配音时已自动添加字幕，可跳过。如需单独调整字幕，使用 subtitle 工具。

### 第六步：配乐（Music Score）
1. 根据短剧氛围生成配乐描述词
2. 调用 music 工具生成背景音乐
3. 使用 mv 工具将音乐与视频混合

## 镜头语言指南
- **建立镜头**（Establishing Shot）：远景交代环境，宽广大气
- **中景/近景**（Medium/Close-up）：展示角色情感和互动
- **特写**（Close-up/Detail）：强调关键道具或表情
- **运镜**：dolly in（推进增加紧张感）、crane shot（俯瞰全景）、tracking shot（跟拍动态）、static shot（静止冥想）
- **转场**：淡入淡出、叠化、硬切——根据节奏选择

## 提示词写作规范（给 video_generation 的 prompt）
- 用英文写，具体且画面感强
- 格式示例：A young woman in a flowing white dress walks slowly through a misty forest at dawn, volumetric god rays filtering through tall pine trees, cinematic shallow depth of field, warm golden tones, slow dolly forward
- 包含：主体 + 动作 + 环境 + 光线 + 色调 + 运镜 + 风格

## 注意事项
- 每个场景视频最长 10 秒，短剧通过多场景合成实现
- style_prefix 是保持全片视觉一致性的关键，不要遗漏
- 配音时间戳必须与视频时长严格对齐
- 先完成所有视频场景，再统一配音配乐`,
			Tools:     `["video_generation","dubbing","subtitle","music","image_generation","mv","web_search"]`,
			Config:    `{"temperature":0.7,"max_tokens":8192}`,
			Icon:      "Video",
			Featured:  true,
			IsBuiltin: true,
		},
	}

	for i := range templates {
		var exists int64
		db.Model(&model.AgentTemplate{}).Where("name = ? AND is_builtin = ?", templates[i].Name, true).Count(&exists)
		if exists == 0 {
			db.Create(&templates[i])
		}
	}
}
