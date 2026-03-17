package v1

import (
	"bytes"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw/internal/developer"
	"github.com/yinhe/starclaw/internal/model"
	"gorm.io/gorm"
)

// DeveloperHandler manages the developer portal: OpenAPI, plugins, playground.
type DeveloperHandler struct {
	db *gorm.DB
}

// NewDeveloperHandler creates the developer handler.
func NewDeveloperHandler(db *gorm.DB) *DeveloperHandler {
	return &DeveloperHandler{db: db}
}

// ════════════════════════════════════════════════════════════════
//  OpenAPI Spec
// ════════════════════════════════════════════════════════════════

// GetOpenAPISpec returns the generated OpenAPI 3.0 specification.
func (h *DeveloperHandler) GetOpenAPISpec(c *gin.Context) {
	scheme := "http"
	if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	serverURL := scheme + "://" + c.Request.Host

	spec := developer.GenerateSpec(serverURL)
	data, err := spec.ToJSON()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate spec"})
		return
	}
	c.Data(http.StatusOK, "application/json", data)
}

// SwaggerUI serves the Swagger UI HTML page.
func (h *DeveloperHandler) SwaggerUI(c *gin.Context) {
	scheme := "http"
	if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	specURL := scheme + "://" + c.Request.Host + "/api/v1/developer/openapi.json"

	html := `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>StarClaw API — Developer Portal</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
  <style>
    body { margin: 0; background: #1a1a2e; }
    .swagger-ui .topbar { display: none; }
    .swagger-ui { max-width: 1200px; margin: 0 auto; }
  </style>
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    SwaggerUIBundle({
      url: "` + specURL + `",
      dom_id: '#swagger-ui',
      deepLinking: true,
      presets: [SwaggerUIBundle.presets.apis, SwaggerUIBundle.SwaggerUIStandalonePreset],
      layout: "BaseLayout",
      defaultModelsExpandDepth: 1,
      tryItOutEnabled: true,
    });
  </script>
</body>
</html>`
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}

// ════════════════════════════════════════════════════════════════
//  Plugin Marketplace
// ════════════════════════════════════════════════════════════════

// ListPlugins returns published plugins.
func (h *DeveloperHandler) ListPlugins(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	category := c.Query("category")
	search := c.Query("q")
	sort := c.DefaultQuery("sort", "popular")

	if page < 1 {
		page = 1
	}
	if pageSize > 50 {
		pageSize = 50
	}

	q := h.db.Model(&model.PluginListing{}).Where("status = ?", "published").Preload("Creator")
	if category != "" {
		q = q.Where("category = ?", category)
	}
	if search != "" {
		q = q.Where("display_name LIKE ? OR description LIKE ?", "%"+search+"%", "%"+search+"%")
	}

	var total int64
	q.Count(&total)

	switch sort {
	case "newest":
		q = q.Order("created_at DESC")
	case "rating":
		q = q.Order("rating DESC")
	default:
		q = q.Order("install_count DESC")
	}

	var plugins []model.PluginListing
	q.Offset((page - 1) * pageSize).Limit(pageSize).Find(&plugins)

	c.JSON(http.StatusOK, gin.H{
		"items":     plugins,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// GetPlugin returns a single plugin detail.
func (h *DeveloperHandler) GetPlugin(c *gin.Context) {
	id := c.Param("id")
	var plugin model.PluginListing
	if err := h.db.Preload("Creator").Where("id = ?", id).First(&plugin).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "plugin not found"})
		return
	}
	c.JSON(http.StatusOK, plugin)
}

// PublishPlugin submits a new plugin for review.
func (h *DeveloperHandler) PublishPlugin(c *gin.Context) {
	userID := c.GetString("user_id")

	var req struct {
		Name        string `json:"name" binding:"required"`
		DisplayName string `json:"display_name" binding:"required"`
		Description string `json:"description"`
		Category    string `json:"category"`
		Icon        string `json:"icon"`
		Readme      string `json:"readme"`
		SpecJSON    string `json:"spec_json" binding:"required"`
		Pricing     string `json:"pricing"`
		PriceCents  int    `json:"price_cents"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Verify creator profile
	var profile model.CreatorProfile
	if err := h.db.Where("user_id = ?", userID).First(&profile).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "please register as a creator first"})
		return
	}

	plugin := model.PluginListing{
		CreatorID:   userID,
		Name:        req.Name,
		DisplayName: req.DisplayName,
		Description: req.Description,
		Category:    req.Category,
		Icon:        req.Icon,
		Readme:      req.Readme,
		SpecJSON:    req.SpecJSON,
		Pricing:     req.Pricing,
		PriceCents:  req.PriceCents,
		Status:      "pending_review",
	}
	if plugin.Pricing == "" {
		plugin.Pricing = "free"
	}

	if err := h.db.Create(&plugin).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create plugin"})
		return
	}
	c.JSON(http.StatusCreated, plugin)
}

// MyPlugins returns plugins created by the current user.
func (h *DeveloperHandler) MyPlugins(c *gin.Context) {
	userID := c.GetString("user_id")
	var plugins []model.PluginListing
	h.db.Where("creator_id = ?", userID).Order("created_at DESC").Find(&plugins)
	c.JSON(http.StatusOK, plugins)
}

// InstallPlugin installs a plugin for the current user.
func (h *DeveloperHandler) InstallPlugin(c *gin.Context) {
	userID := c.GetString("user_id")
	pluginID := c.Param("id")

	var plugin model.PluginListing
	if err := h.db.Where("id = ? AND status = ?", pluginID, "published").First(&plugin).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "plugin not found"})
		return
	}

	// Check if already installed
	var existing model.PluginInstall
	if err := h.db.Where("plugin_id = ? AND user_id = ?", pluginID, userID).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "already installed"})
		return
	}

	install := model.PluginInstall{
		PluginID: pluginID,
		UserID:   userID,
		Version:  plugin.Version,
	}
	h.db.Create(&install)
	h.db.Model(&plugin).Update("install_count", gorm.Expr("install_count + 1"))

	c.JSON(http.StatusCreated, gin.H{"installed": true})
}

// UninstallPlugin uninstalls a plugin.
func (h *DeveloperHandler) UninstallPlugin(c *gin.Context) {
	userID := c.GetString("user_id")
	pluginID := c.Param("id")

	result := h.db.Where("plugin_id = ? AND user_id = ?", pluginID, userID).Delete(&model.PluginInstall{})
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "not installed"})
		return
	}
	h.db.Model(&model.PluginListing{}).Where("id = ?", pluginID).
		Update("install_count", gorm.Expr("GREATEST(install_count - 1, 0)"))

	c.JSON(http.StatusOK, gin.H{"uninstalled": true})
}

// MyInstalled returns plugins installed by the current user.
func (h *DeveloperHandler) MyInstalled(c *gin.Context) {
	userID := c.GetString("user_id")
	var installs []model.PluginInstall
	h.db.Preload("Plugin").Where("user_id = ?", userID).Order("created_at DESC").Find(&installs)
	c.JSON(http.StatusOK, installs)
}

// RatePlugin rates a plugin.
func (h *DeveloperHandler) RatePlugin(c *gin.Context) {
	userID := c.GetString("user_id")
	pluginID := c.Param("id")

	var req struct {
		Score   int    `json:"score" binding:"required,min=1,max=5"`
		Comment string `json:"comment"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Update or create rating
	var existing model.PluginRating
	if err := h.db.Where("plugin_id = ? AND user_id = ?", pluginID, userID).First(&existing).Error; err == nil {
		existing.Score = req.Score
		existing.Comment = req.Comment
		h.db.Save(&existing)
	} else {
		rating := model.PluginRating{
			PluginID: pluginID,
			UserID:   userID,
			Score:    req.Score,
			Comment:  req.Comment,
		}
		h.db.Create(&rating)
	}

	// Recalculate average
	var avgScore float64
	var count int64
	h.db.Model(&model.PluginRating{}).Where("plugin_id = ?", pluginID).Count(&count)
	if count > 0 {
		h.db.Model(&model.PluginRating{}).Where("plugin_id = ?", pluginID).Select("AVG(score)").Scan(&avgScore)
	}
	h.db.Model(&model.PluginListing{}).Where("id = ?", pluginID).Updates(map[string]interface{}{
		"rating":       avgScore,
		"rating_count": count,
	})

	c.JSON(http.StatusOK, gin.H{"avg_rating": avgScore, "rating_count": count})
}

// AdminListPendingPlugins returns plugins pending review.
func (h *DeveloperHandler) AdminListPendingPlugins(c *gin.Context) {
	var plugins []model.PluginListing
	h.db.Preload("Creator").Where("status = ?", "pending_review").Order("created_at ASC").Find(&plugins)
	c.JSON(http.StatusOK, plugins)
}

// AdminReviewPlugin approves or rejects a plugin.
func (h *DeveloperHandler) AdminReviewPlugin(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Action string `json:"action" binding:"required"` // approve, reject
		Note   string `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var plugin model.PluginListing
	if err := h.db.Where("id = ?", id).First(&plugin).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "plugin not found"})
		return
	}

	switch req.Action {
	case "approve":
		plugin.Status = "published"
	case "reject":
		plugin.Status = "draft"
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "action must be approve or reject"})
		return
	}
	plugin.ReviewNote = req.Note
	h.db.Save(&plugin)

	c.JSON(http.StatusOK, gin.H{"status": plugin.Status})
}

// PluginCategories returns available plugin categories.
func (h *DeveloperHandler) PluginCategories(c *gin.Context) {
	categories := []map[string]string{
		{"id": "api", "name": "API Integration", "description": "Connect to third-party APIs and services"},
		{"id": "data", "name": "Data Processing", "description": "Data extraction, transformation, and analysis"},
		{"id": "productivity", "name": "Productivity", "description": "Calendar, email, task management integrations"},
		{"id": "dev", "name": "Developer Tools", "description": "Code generation, testing, deployment helpers"},
		{"id": "media", "name": "Media", "description": "Image, audio, video generation and processing"},
		{"id": "finance", "name": "Finance", "description": "Payment, accounting, crypto integrations"},
		{"id": "social", "name": "Social", "description": "Social media, messaging, community tools"},
	}
	c.JSON(http.StatusOK, categories)
}

// ════════════════════════════════════════════════════════════════
//  API Playground
// ════════════════════════════════════════════════════════════════

// PlaygroundExecute proxies an API call and records it.
func (h *DeveloperHandler) PlaygroundExecute(c *gin.Context) {
	userID := c.GetString("user_id")

	var req struct {
		Method  string            `json:"method" binding:"required"`
		Path    string            `json:"path" binding:"required"`
		Body    string            `json:"body"`
		Headers map[string]string `json:"headers"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Build internal request
	scheme := "http"
	if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	targetURL := scheme + "://" + c.Request.Host + "/api/v1" + req.Path

	var bodyReader io.Reader
	if req.Body != "" {
		bodyReader = bytes.NewBufferString(req.Body)
	}

	httpReq, err := http.NewRequest(req.Method, targetURL, bodyReader)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	// Forward auth token
	httpReq.Header.Set("Authorization", c.GetHeader("Authorization"))
	httpReq.Header.Set("Content-Type", "application/json")
	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}

	start := time.Now()
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	durationMs := time.Since(start).Milliseconds()

	if err != nil {
		// Record failed request
		record := model.PlaygroundRequest{
			UserID:      userID,
			Method:      req.Method,
			Path:        req.Path,
			RequestBody: req.Body,
			DurationMs:  durationMs,
		}
		h.db.Create(&record)
		c.JSON(http.StatusBadGateway, gin.H{"error": "request failed: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	// Read response (max 64KB for playground)
	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 65536))

	// Record the request
	record := model.PlaygroundRequest{
		UserID:       userID,
		Method:       req.Method,
		Path:         req.Path,
		RequestBody:  req.Body,
		ResponseCode: resp.StatusCode,
		ResponseBody: string(bodyBytes),
		DurationMs:   durationMs,
	}
	h.db.Create(&record)

	c.JSON(http.StatusOK, gin.H{
		"status_code":   resp.StatusCode,
		"response_body": string(bodyBytes),
		"duration_ms":   durationMs,
		"headers":       resp.Header,
	})
}

// PlaygroundHistory returns recent playground requests.
func (h *DeveloperHandler) PlaygroundHistory(c *gin.Context) {
	userID := c.GetString("user_id")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if limit > 200 {
		limit = 200
	}

	var records []model.PlaygroundRequest
	h.db.Where("user_id = ?", userID).Order("created_at DESC").Limit(limit).Find(&records)
	c.JSON(http.StatusOK, records)
}

// DeveloperStats returns developer portal overview.
func (h *DeveloperHandler) DeveloperStats(c *gin.Context) {
	var totalPlugins, publishedPlugins int64
	h.db.Model(&model.PluginListing{}).Count(&totalPlugins)
	h.db.Model(&model.PluginListing{}).Where("status = ?", "published").Count(&publishedPlugins)

	var totalInstalls int64
	h.db.Model(&model.PluginInstall{}).Count(&totalInstalls)

	var totalCreators int64
	h.db.Model(&model.CreatorProfile{}).Count(&totalCreators)

	c.JSON(http.StatusOK, gin.H{
		"total_plugins":     totalPlugins,
		"published_plugins": publishedPlugins,
		"total_installs":    totalInstalls,
		"total_creators":    totalCreators,
	})
}
