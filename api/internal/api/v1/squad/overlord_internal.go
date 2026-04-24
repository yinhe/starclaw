package squad

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/yinhe/starclaw/internal/config"
	"github.com/yinhe/starclaw/internal/model"
	"github.com/yinhe/starclaw/internal/node"
	"github.com/yinhe/starclaw/internal/provider"
	"github.com/yinhe/starclaw/internal/tool"
	"gorm.io/gorm"
)

// OverlordInternalHandler serves endpoints called by Overlord for Team Agent orchestration.
// Authenticated via X-Overlord-Token header matching OVERLORD_CLAW_TOKEN env.
type OverlordInternalHandler struct {
	db               *gorm.DB
	identity         *node.Identity
	token            string
	cfg              *config.Config
	providerRegistry *provider.Registry
	toolRegistry     *tool.Registry
}

// NewOverlordInternalHandler creates the handler.
// SetProviderRegistry injects the provider registry for chat proxy.
func (h *OverlordInternalHandler) SetProviderRegistry(pr *provider.Registry) {
	h.providerRegistry = pr
}

// SetToolRegistry injects the tool registry for skill listing.
func (h *OverlordInternalHandler) SetToolRegistry(tr *tool.Registry) {
	h.toolRegistry = tr
}

func NewOverlordInternalHandler(db *gorm.DB, identity *node.Identity, cfg ...*config.Config) *OverlordInternalHandler {
	token := os.Getenv("OVERLORD_CLAW_TOKEN")
	if token == "" {
		token = "overlord-internal-default"
	}
	h := &OverlordInternalHandler{db: db, identity: identity, token: token}
	if len(cfg) > 0 {
		h.cfg = cfg[0]
	}
	return h
}

// AuthMiddleware validates the X-Overlord-Token header.
func (h *OverlordInternalHandler) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		provided := c.GetHeader("X-Overlord-Token")
		if provided == "" || provided != h.token {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid overlord token"})
			return
		}
		c.Next()
	}
}

// ── Squad ──

// POST /v1/internal/squad/create
func (h *OverlordInternalHandler) CreateSquad(c *gin.Context) {
	var req struct {
		Name        string   `json:"name" binding:"required"`
		Description string   `json:"description"`
		MaxMembers  int      `json:"max_members"`
		Tags        []string `json:"tags"`
		OverlordRef string   `json:"overlord_ref"` // TeamInstance ID
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.MaxMembers <= 0 {
		req.MaxMembers = 10
	}

	tagsJSON := "[]"
	if len(req.Tags) > 0 {
		tagsJSON = toJSON(req.Tags)
	}

	captainNode := ""
	if h.identity != nil {
		captainNode = h.identity.NodeID
	}

	sysUserID := h.ensureSystemUser("overlord-team-agent")

	squad := model.Squad{
		Name:        req.Name,
		Description: req.Description,
		CaptainNode: captainNode,
		UserID:      sysUserID,
		Status:      "active",
		MaxMembers:  req.MaxMembers,
		Tags:        tagsJSON,
	}

	if err := h.db.Create(&squad).Error; err != nil {
		log.Printf("[overlord-internal] failed to create squad: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create squad"})
		return
	}

	// Auto-add self as captain
	member := model.SquadMember{
		SquadID:  squad.ID,
		NodeID:   captainNode,
		Role:     "captain",
		Status:   "online",
		JoinedAt: time.Now(),
	}
	h.db.Create(&member)

	log.Printf("[overlord-internal] created squad %s (%s) for overlord ref %s", squad.ID, req.Name, req.OverlordRef)

	c.JSON(http.StatusOK, gin.H{"squad": squad})
}

// POST /v1/internal/squad/disband
func (h *OverlordInternalHandler) DisbandSquad(c *gin.Context) {
	var req struct {
		SquadID string `json:"squad_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result := h.db.Model(&model.Squad{}).Where("id = ?", req.SquadID).Update("status", "disbanded")
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "squad not found"})
		return
	}

	h.db.Where("squad_id = ?", req.SquadID).Delete(&model.SquadMember{})
	log.Printf("[overlord-internal] disbanded squad %s", req.SquadID)

	c.JSON(http.StatusOK, gin.H{"message": "squad disbanded"})
}

// GET /v1/internal/squad/:id
func (h *OverlordInternalHandler) GetSquad(c *gin.Context) {
	squadID := c.Param("id")

	var squad model.Squad
	if err := h.db.First(&squad, "id = ?", squadID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "squad not found"})
		return
	}

	var members []model.SquadMember
	h.db.Where("squad_id = ?", squadID).Find(&members)

	c.JSON(http.StatusOK, gin.H{"squad": squad, "members": members})
}

// ── Mission ──

// POST /v1/internal/mission/create
func (h *OverlordInternalHandler) CreateMission(c *gin.Context) {
	var req struct {
		SquadID string `json:"squad_id" binding:"required"`
		Title   string `json:"title" binding:"required"`
		Goal    string `json:"goal" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var squad model.Squad
	if err := h.db.First(&squad, "id = ?", req.SquadID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "squad not found"})
		return
	}

	mission := model.Mission{
		SquadID:     req.SquadID,
		Title:       req.Title,
		Goal:        req.Goal,
		Status:      "planning",
		CaptainNode: squad.CaptainNode,
		UserID:      squad.UserID,
	}

	if err := h.db.Create(&mission).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create mission"})
		return
	}

	log.Printf("[overlord-internal] created mission %s for squad %s: %s", mission.ID, req.SquadID, req.Title)

	c.JSON(http.StatusOK, gin.H{"mission": mission})
}

// POST /v1/internal/mission/start
func (h *OverlordInternalHandler) StartMission(c *gin.Context) {
	var req struct {
		MissionID string `json:"mission_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var mission model.Mission
	if err := h.db.First(&mission, "id = ?", req.MissionID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "mission not found"})
		return
	}

	if mission.Status != "planning" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "mission already started or completed"})
		return
	}

	// Set to executing — the Squad Engine poll loop will pick it up
	h.db.Model(&mission).Update("status", "executing")
	log.Printf("[overlord-internal] started mission %s", req.MissionID)

	c.JSON(http.StatusOK, gin.H{"mission": mission, "message": "mission started"})
}

// GET /v1/internal/mission/:id
func (h *OverlordInternalHandler) GetMission(c *gin.Context) {
	missionID := c.Param("id")

	var mission model.Mission
	if err := h.db.First(&mission, "id = ?", missionID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "mission not found"})
		return
	}

	var steps []model.MissionStep
	h.db.Where("mission_id = ?", missionID).Order("sequence ASC").Find(&steps)

	c.JSON(http.StatusOK, gin.H{"mission": mission, "steps": steps})
}

// ── Auth Exchange (Overlord ↔ Claw token bridge) ──

// POST /v1/internal/auth/exchange
// Overlord sends its admin user info, Claw creates/finds a local user and returns a JWT.
func (h *OverlordInternalHandler) AuthExchange(c *gin.Context) {
	var req struct {
		OverlordUserID string `json:"overlord_user_id" binding:"required"`
		Username       string `json:"username" binding:"required"`
		Role           string `json:"role"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Find or create local user mapped to this Overlord user
	overlordEmail := "overlord+" + req.OverlordUserID + "@claw.local"
	var user model.User
	if err := h.db.Where("email = ?", overlordEmail).First(&user).Error; err != nil {
		// Create new user for this Overlord identity
		randBytes := make([]byte, 16)
		rand.Read(randBytes)
		user = model.User{
			Email:    &overlordEmail,
			Username: "OL:" + req.Username,
			Password: hex.EncodeToString(randBytes), // random password, not usable for direct login
			Role:     "user",
		}
		if err := h.db.Create(&user).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user"})
			return
		}
		log.Printf("[overlord-internal] created local user %s for overlord user %s (%s)", user.ID, req.OverlordUserID, req.Username)
	}

	// Generate JWT for this user
	jwtSecret := "starclaw-secret"
	expireHours := 24
	if h.cfg != nil {
		if h.cfg.JWT.Secret != "" {
			jwtSecret = h.cfg.JWT.Secret
		}
		if h.cfg.JWT.ExpireHour > 0 {
			expireHours = h.cfg.JWT.ExpireHour
		}
	}

	claims := jwt.MapClaims{
		"sub":      user.ID,
		"username": user.Username,
		"role":     user.Role,
		"exp":      time.Now().Add(time.Duration(expireHours) * time.Hour).Unix(),
		"iat":      time.Now().Unix(),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := tok.SignedString([]byte(jwtSecret))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to sign token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token":      tokenStr,
		"user_id":    user.ID,
		"username":   user.Username,
		"expires_at": time.Now().Add(time.Duration(expireHours) * time.Hour).Format(time.RFC3339),
	})
}

// POST /v1/internal/auth/verify
// Verifies a Claw JWT and returns user info. Used by Overlord for node-login flow.
func (h *OverlordInternalHandler) AuthVerify(c *gin.Context) {
	var req struct {
		Token string `json:"token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	jwtSecret := "starclaw-secret"
	if h.cfg != nil && h.cfg.JWT.Secret != "" {
		jwtSecret = h.cfg.JWT.Secret
	}

	tok, err := jwt.Parse(req.Token, func(t *jwt.Token) (interface{}, error) {
		return []byte(jwtSecret), nil
	})
	if err != nil || !tok.Valid {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}

	claims, ok := tok.Claims.(jwt.MapClaims)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid claims"})
		return
	}

	userID, _ := claims["sub"].(string)
	username, _ := claims["username"].(string)
	role, _ := claims["role"].(string)

	// Verify user still exists
	var user model.User
	if err := h.db.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"valid":    true,
		"user_id":  userID,
		"username": username,
		"role":     role,
	})
}

// ── Chat Proxy (OpenAI-compatible, for Overlord) ──

// POST /v1/internal/chat/completions
// Accepts OpenAI-compatible chat format, resolves model via DB ModelConfig or provider registry,
// and returns a non-streaming or SSE streaming response.
func (h *OverlordInternalHandler) ChatCompletions(c *gin.Context) {
	var req struct {
		Model    string `json:"model" binding:"required"`
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages" binding:"required"`
		MaxTokens   int     `json:"max_tokens"`
		Temperature float64 `json:"temperature"`
		Stream      bool    `json:"stream"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Resolve provider for this model
	// 1. Try DB ModelConfig (platform models)
	var modelCfg model.ModelConfig
	found := h.db.Where("model_name = ? AND is_enabled = ?", req.Model, true).
		Order("is_platform DESC").First(&modelCfg).Error == nil

	var p provider.ModelProvider
	if found {
		p = provider.CreateFromConfig(h.providerRegistry, modelCfg)
	} else if h.providerRegistry != nil {
		// 2. Fallback: try star-ai provider (default for all models)
		if sp, ok := h.providerRegistry.Get("star-ai"); ok {
			p = sp
		}
	}

	if p == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("model %q not available", req.Model)})
		return
	}

	// Build provider request
	msgs := make([]provider.ChatMessage, 0, len(req.Messages))
	for _, m := range req.Messages {
		msgs = append(msgs, provider.ChatMessage{Role: m.Role, Content: m.Content})
	}
	chatReq := &provider.ChatRequest{
		Model:       req.Model,
		Messages:    msgs,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		Stream:      req.Stream,
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Minute)
	defer cancel()

	if req.Stream {
		h.chatStream(c, ctx, p, chatReq)
	} else {
		h.chatSync(c, ctx, p, chatReq)
	}
}

func (h *OverlordInternalHandler) chatSync(c *gin.Context, ctx context.Context, p provider.ModelProvider, req *provider.ChatRequest) {
	chunk, err := p.ChatSync(ctx, req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "chat failed: " + err.Error()})
		return
	}
	// Return OpenAI-compatible response
	resp := gin.H{
		"id":    chunk.ID,
		"model": req.Model,
		"choices": []gin.H{{
			"index":   0,
			"message": gin.H{"role": "assistant", "content": chunk.Content},
		}},
	}
	if chunk.Usage != nil {
		resp["usage"] = gin.H{
			"prompt_tokens":     chunk.Usage.PromptTokens,
			"completion_tokens": chunk.Usage.CompletionTokens,
			"total_tokens":      chunk.Usage.TotalTokens,
		}
	}
	c.JSON(http.StatusOK, resp)
}

func (h *OverlordInternalHandler) chatStream(c *gin.Context, ctx context.Context, p provider.ModelProvider, req *provider.ChatRequest) {
	ch, err := p.Chat(ctx, req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "stream failed: " + err.Error()})
		return
	}

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.WriteHeader(http.StatusOK)

	for chunk := range ch {
		if chunk.Error != "" {
			data, _ := json.Marshal(gin.H{"error": chunk.Error})
			fmt.Fprintf(c.Writer, "data: %s\n\n", data)
			c.Writer.Flush()
			break
		}
		// OpenAI-compatible SSE chunk
		sseChunk := gin.H{
			"id":    chunk.ID,
			"model": req.Model,
			"choices": []gin.H{{
				"index": 0,
				"delta": gin.H{"content": chunk.Content},
			}},
		}
		data, _ := json.Marshal(sseChunk)
		fmt.Fprintf(c.Writer, "data: %s\n\n", data)
		c.Writer.Flush()

		if chunk.Done {
			break
		}
	}
	fmt.Fprintf(c.Writer, "data: [DONE]\n\n")
	c.Writer.Flush()
}

// GET /v1/internal/models
// Returns available models from DB (platform + user) for Overlord model management.
func (h *OverlordInternalHandler) ListModels(c *gin.Context) {
	var models []model.ModelConfig
	h.db.Where("is_enabled = ?", true).Order("is_platform DESC, provider ASC, model_name ASC").Find(&models)
	// Sanitize: never expose API keys
	type safeModel struct {
		ID          string  `json:"id"`
		Provider    string  `json:"provider"`
		ModelName   string  `json:"model_name"`
		DisplayName string  `json:"display_name"`
		MaxTokens   int     `json:"max_tokens"`
		Temperature float64 `json:"temperature"`
		IsPlatform  bool    `json:"is_platform"`
	}
	safe := make([]safeModel, 0, len(models))
	for _, m := range models {
		safe = append(safe, safeModel{
			ID: m.ID, Provider: m.Provider, ModelName: m.ModelName,
			DisplayName: m.DisplayName, MaxTokens: m.MaxTokens,
			Temperature: m.Temperature, IsPlatform: m.IsPlatform,
		})
	}
	c.JSON(http.StatusOK, gin.H{"models": safe, "total": len(safe)})
}

// GET /v1/internal/skills
// Returns all available skills/tools from the ToolRegistry and installed plugins.
func (h *OverlordInternalHandler) ListSkills(c *gin.Context) {
	type SkillInfo struct {
		Name             string   `json:"name"`
		Description      string   `json:"description"`
		Type             string   `json:"type"`   // builtin, plugin, mcp
		Status           string   `json:"status"` // active
		Category         string   `json:"category"`
		CategoryLabel    string   `json:"category_label"`
		Subcategory      string   `json:"subcategory,omitempty"`
		SubcategoryLabel string   `json:"subcategory_label,omitempty"`
		Industry         string   `json:"industry"`
		Tags             []string `json:"tags,omitempty"`
	}

	builtinNames := map[string]bool{
		"system": true, "code": true, "web_search": true, "http_request": true,
		"browser": true, "video_generation": true, "image_generation": true,
		"music_generation": true, "audio_analysis": true, "mv_production": true,
		"dubbing": true, "comic_production": true, "document": true,
		"desktop": true, "deploy_web": true, "bind_domain": true,
		"verify_online": true,
	}

	var skills []SkillInfo

	if h.toolRegistry != nil {
		for _, name := range h.toolRegistry.List() {
			t, ok := h.toolRegistry.Get(name)
			if !ok {
				continue
			}
			typ := "builtin"
			if !builtinNames[name] {
				if catalog := tool.DescribeCapability(name, "plugin", tool.MetadataFor(t)); catalog.Subcategory == "mcp" {
					typ = "mcp"
				} else {
					typ = "plugin"
				}
			}
			catalog := tool.DescribeCapability(name, typ, tool.MetadataFor(t))
			skills = append(skills, SkillInfo{
				Name:             name,
				Description:      t.Description(),
				Type:             typ,
				Status:           "active",
				Category:         catalog.Category,
				CategoryLabel:    catalog.CategoryLabel,
				Subcategory:      catalog.Subcategory,
				SubcategoryLabel: catalog.SubcategoryLabel,
				Industry:         catalog.Industry,
				Tags:             catalog.Tags,
			})
		}
	}

	// Also include installed plugins from DB
	var plugins []model.PluginListing
	h.db.Where("status = ?", "published").Order("install_count DESC").Find(&plugins)
	type PluginInfo struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		DisplayName string `json:"display_name"`
		Description string `json:"description"`
		Category    string `json:"category"`
		Icon        string `json:"icon"`
		Version     string `json:"version"`
		Pricing     string `json:"pricing"`
	}
	pluginList := make([]PluginInfo, 0, len(plugins))
	for _, p := range plugins {
		pluginList = append(pluginList, PluginInfo{
			ID: p.ID, Name: p.Name, DisplayName: p.DisplayName,
			Description: p.Description, Category: p.Category,
			Icon: p.Icon, Version: p.Version, Pricing: p.Pricing,
		})
	}

	// MCP servers (node-level)
	var mcpServers []model.MCPServer
	h.db.Where("status = ?", "active").Find(&mcpServers)
	type MCPInfo struct {
		Name    string `json:"name"`
		BaseURL string `json:"base_url"`
	}
	mcpList := make([]MCPInfo, 0, len(mcpServers))
	for _, s := range mcpServers {
		mcpList = append(mcpList, MCPInfo{Name: s.Name, BaseURL: s.BaseURL})
	}

	c.JSON(http.StatusOK, gin.H{
		"skills":      skills,
		"plugins":     pluginList,
		"mcp_servers": mcpList,
		"total":       len(skills),
	})
}

// GET /v1/internal/agents
// Returns published agent templates from the marketplace for Overlord to use as role presets.
func (h *OverlordInternalHandler) ListAgents(c *gin.Context) {
	category := c.Query("category")

	type AgentInfo struct {
		ID           string  `json:"id"`
		Name         string  `json:"name"`
		Description  string  `json:"description"`
		Category     string  `json:"category"`
		SystemPrompt string  `json:"system_prompt"`
		Model        string  `json:"model"`
		Tools        string  `json:"tools"` // JSON array
		Icon         string  `json:"icon"`
		Rating       float64 `json:"rating"`
		InstallCount int     `json:"install_count"`
		IsOfficial   bool    `json:"is_official"`
	}

	q := h.db.Table("agent_templates").
		Select("agent_templates.id, agent_templates.name, agent_templates.description, agent_templates.category, agent_templates.system_prompt, agent_templates.model, agent_templates.tools, agent_templates.icon, agent_templates.rating, agent_templates.install_count, agent_templates.is_official").
		Where("agent_templates.is_public = ? OR agent_templates.is_official = ?", true, true)

	if category != "" {
		q = q.Where("agent_templates.category = ?", category)
	}

	q = q.Order("agent_templates.install_count DESC, agent_templates.rating DESC").Limit(100)

	var agents []AgentInfo
	q.Find(&agents)

	// Also get categories for filtering
	var categories []struct {
		Category string `json:"category"`
		Count    int    `json:"count"`
	}
	h.db.Table("agent_templates").
		Select("category, COUNT(*) as count").
		Where("is_public = ? OR is_official = ?", true, true).
		Group("category").Having("count > 0").Find(&categories)

	c.JSON(http.StatusOK, gin.H{
		"agents":     agents,
		"categories": categories,
		"total":      len(agents),
	})
}

// ── Agent Development (Sandbox + Publish) ──

// POST /v1/internal/agent-sandbox
// Creates a temporary agent, runs test conversations, returns results.
// Used by DevClaw to test agents before publishing.
func (h *OverlordInternalHandler) AgentSandbox(c *gin.Context) {
	var req struct {
		Name         string `json:"name" binding:"required"`
		SystemPrompt string `json:"system_prompt" binding:"required"`
		Model        string `json:"model"`
		Tools        string `json:"tools"`  // JSON array of tool names
		Config       string `json:"config"` // JSON config
		TestMessages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"test_messages"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	modelName := req.Model
	if modelName == "" {
		modelName = "deepseek-chat"
	}

	// Resolve provider
	var modelCfg model.ModelConfig
	found := h.db.Where("model_name = ? AND is_enabled = ?", modelName, true).
		Order("is_platform DESC").First(&modelCfg).Error == nil
	var p provider.ModelProvider
	if found {
		p = provider.CreateFromConfig(h.providerRegistry, modelCfg)
	} else if h.providerRegistry != nil {
		if sp, ok := h.providerRegistry.Get("star-ai"); ok {
			p = sp
		}
	}
	if p == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("model %q not available", modelName)})
		return
	}

	// Run test conversations
	type TestResult struct {
		Input   string          `json:"input"`
		Output  string          `json:"output"`
		Verdict string          `json:"verdict"` // pass, fail, warning
		Checks  map[string]bool `json:"checks"`
		Error   string          `json:"error,omitempty"`
	}

	var results []TestResult
	totalScore := 0.0

	for _, tm := range req.TestMessages {
		msgs := []provider.ChatMessage{
			{Role: "system", Content: req.SystemPrompt},
			{Role: tm.Role, Content: tm.Content},
		}
		chatReq := &provider.ChatRequest{
			Model:       modelName,
			Messages:    msgs,
			MaxTokens:   2048,
			Temperature: 0.3,
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
		chunk, err := p.ChatSync(ctx, chatReq)
		cancel()

		if err != nil {
			results = append(results, TestResult{
				Input: tm.Content, Output: "", Verdict: "fail",
				Error: err.Error(), Checks: map[string]bool{},
			})
			continue
		}

		output := chunk.Content

		// Basic quality checks
		checks := map[string]bool{
			"non_empty":          len(output) > 10,
			"no_prompt_leak":     !strings.Contains(strings.ToLower(output), "system prompt") && !strings.Contains(strings.ToLower(output), "你的提示词"),
			"reasonable_length":  len([]rune(output)) < 5000,
			"stays_in_character": true, // placeholder — could use LLM judge
		}

		passed := 0
		for _, v := range checks {
			if v {
				passed++
			}
		}
		score := float64(passed) / float64(len(checks)) * 10
		totalScore += score

		verdict := "pass"
		if score < 5 {
			verdict = "fail"
		} else if score < 7 {
			verdict = "warning"
		}

		results = append(results, TestResult{
			Input: tm.Content, Output: output, Verdict: verdict, Checks: checks,
		})
	}

	overallScore := 0.0
	if len(results) > 0 {
		overallScore = totalScore / float64(len(results))
	}

	passCount := 0
	for _, r := range results {
		if r.Verdict == "pass" {
			passCount++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"results":          results,
		"overall_score":    overallScore,
		"pass_count":       passCount,
		"total_tests":      len(results),
		"ready_to_publish": overallScore >= 7.0 && passCount == len(results),
	})
}

// POST /v1/internal/agent-publish
// Creates an AgentTemplate + optionally an AgentListing from a config.
// Used by DevClaw to publish agents after sandbox testing.
func (h *OverlordInternalHandler) AgentPublish(c *gin.Context) {
	var req struct {
		Name         string `json:"name" binding:"required"`
		Description  string `json:"description"`
		SystemPrompt string `json:"system_prompt" binding:"required"`
		Model        string `json:"model"`
		Tools        string `json:"tools"`  // JSON array
		Config       string `json:"config"` // JSON config
		Category     string `json:"category"`
		Tags         string `json:"tags"` // JSON array
		Icon         string `json:"icon"`
		Pricing      string `json:"pricing"` // free, one_time, subscription
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Tools == "" {
		req.Tools = "[]"
	}
	if req.Config == "" {
		req.Config = `{"temperature":0.3,"max_tokens":4096}`
	}
	if req.Category == "" {
		req.Category = "assistant"
	}
	if req.Tags == "" {
		req.Tags = "[]"
	}

	// Create AgentTemplate
	tpl := model.AgentTemplate{
		AuthorID:     "overlord:devclaw", // mark as DevClaw-created
		Name:         req.Name,
		Description:  req.Description,
		Category:     req.Category,
		Tags:         req.Tags,
		SystemPrompt: req.SystemPrompt,
		Tools:        req.Tools,
		Config:       req.Config,
		Icon:         req.Icon,
	}
	if err := h.db.Create(&tpl).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create template: " + err.Error()})
		return
	}

	log.Printf("[overlord-internal] DevClaw published agent template %s: %s", tpl.ID, req.Name)

	resp := gin.H{
		"template_id": tpl.ID,
		"name":        tpl.Name,
		"category":    tpl.Category,
		"status":      "published",
	}

	c.JSON(http.StatusCreated, resp)
}

// ensureSystemUser creates or finds a system user for internal operations.
func (h *OverlordInternalHandler) ensureSystemUser(username string) string {
	var user model.User
	if err := h.db.Where("username = ?", username).First(&user).Error; err == nil {
		return user.ID
	}
	user = model.User{
		Username: username,
		Password: "!system-no-login",
		Role:     "system",
	}
	h.db.Create(&user)
	return user.ID
}

// ── Team Agent Management (Phase 1: Agent 化) ──

// POST /v1/internal/agents/register
// Registers a team agent on this Claw node. Called by Overlord during instance provisioning.
func (h *OverlordInternalHandler) RegisterAgent(c *gin.Context) {
	var req struct {
		Name           string   `json:"name" binding:"required"`
		RoleCode       string   `json:"role_code" binding:"required"`
		TeamInstanceID string   `json:"team_instance_id" binding:"required"`
		SystemPrompt   string   `json:"system_prompt"`
		ModelName      string   `json:"model_name"`
		Tools          []string `json:"tools"` // tool names to enable
		Config         string   `json:"config"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	toolsJSON, _ := json.Marshal(req.Tools)
	if req.Config == "" {
		req.Config = "{}"
	}

	// Ensure a system user exists for team agents
	sysUserID := h.ensureSystemUser("overlord-team-agent")

	agent := model.Agent{
		UserID:         sysUserID,
		Name:           req.Name,
		RoleCode:       req.RoleCode,
		TeamInstanceID: req.TeamInstanceID,
		SystemPrompt:   req.SystemPrompt,
		ModelName:      req.ModelName,
		Tools:          string(toolsJSON),
		Config:         req.Config,
		IsBuiltin:      true,
	}
	if err := h.db.Create(&agent).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to register agent: " + err.Error()})
		return
	}

	log.Printf("[overlord-internal] registered team agent %s (%s) for instance %s", agent.Name, agent.RoleCode, req.TeamInstanceID)
	c.JSON(http.StatusCreated, gin.H{"agent": agent})
}

// GET /v1/internal/agents/team/:instanceId
// Lists all agents registered for a team instance.
func (h *OverlordInternalHandler) ListTeamAgents(c *gin.Context) {
	instanceID := c.Param("instanceId")
	var agents []model.Agent
	h.db.Where("team_instance_id = ?", instanceID).Preload("Skills").Find(&agents)
	c.JSON(http.StatusOK, gin.H{"agents": agents, "total": len(agents)})
}

// DELETE /v1/internal/agents/:id
func (h *OverlordInternalHandler) DeleteAgent(c *gin.Context) {
	agentID := c.Param("id")
	// Delete skills first
	h.db.Where("agent_id = ?", agentID).Delete(&model.AgentSkill{})
	// Delete agent
	result := h.db.Delete(&model.Agent{}, "id = ?", agentID)
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "agent deleted"})
}

// POST /v1/internal/agents/:id/skills
// Installs a skill on an agent. Called by Overlord after fetching skill spec from Queen.
func (h *OverlordInternalHandler) InstallSkill(c *gin.Context) {
	agentID := c.Param("id")
	var req struct {
		SkillID   string `json:"skill_id"`
		SkillName string `json:"skill_name" binding:"required"`
		SkillSpec string `json:"skill_spec"` // JSON function spec
		Version   string `json:"version"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Verify agent exists
	var agent model.Agent
	if err := h.db.First(&agent, "id = ?", agentID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
		return
	}

	// Check if skill already installed
	var existing model.AgentSkill
	if h.db.Where("agent_id = ? AND skill_name = ?", agentID, req.SkillName).First(&existing).Error == nil {
		// Update existing
		h.db.Model(&existing).Updates(map[string]interface{}{
			"skill_spec": req.SkillSpec,
			"version":    req.Version,
		})
		c.JSON(http.StatusOK, gin.H{"skill": existing, "updated": true})
		return
	}

	skill := model.AgentSkill{
		AgentID:   agentID,
		SkillID:   req.SkillID,
		SkillName: req.SkillName,
		SkillSpec: req.SkillSpec,
		Version:   req.Version,
	}
	if err := h.db.Create(&skill).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to install skill: " + err.Error()})
		return
	}

	// Update agent's tools array to include this skill
	var tools []string
	json.Unmarshal([]byte(agent.Tools), &tools)
	found := false
	for _, t := range tools {
		if t == req.SkillName {
			found = true
			break
		}
	}
	if !found {
		tools = append(tools, req.SkillName)
		toolsJSON, _ := json.Marshal(tools)
		h.db.Model(&agent).Update("tools", string(toolsJSON))
	}

	log.Printf("[overlord-internal] installed skill %s on agent %s (%s)", req.SkillName, agent.Name, agentID)
	c.JSON(http.StatusCreated, gin.H{"skill": skill})
}

// DELETE /v1/internal/agents/:id/skills/:skillName
func (h *OverlordInternalHandler) UninstallSkill(c *gin.Context) {
	agentID := c.Param("id")
	skillName := c.Param("skillName")

	result := h.db.Where("agent_id = ? AND skill_name = ?", agentID, skillName).Delete(&model.AgentSkill{})
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "skill not found"})
		return
	}

	// Remove from agent's tools array
	var agent model.Agent
	if h.db.First(&agent, "id = ?", agentID).Error == nil {
		var tools []string
		json.Unmarshal([]byte(agent.Tools), &tools)
		filtered := make([]string, 0, len(tools))
		for _, t := range tools {
			if t != skillName {
				filtered = append(filtered, t)
			}
		}
		toolsJSON, _ := json.Marshal(filtered)
		h.db.Model(&agent).Update("tools", string(toolsJSON))
	}

	c.JSON(http.StatusOK, gin.H{"message": "skill uninstalled"})
}
