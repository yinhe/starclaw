package v1

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/yinhe/starclaw/internal/config"
	"github.com/yinhe/starclaw/internal/model"
	"github.com/yinhe/starclaw/internal/node"
	"github.com/yinhe/starclaw/internal/provider"
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
}

// NewOverlordInternalHandler creates the handler.
// SetProviderRegistry injects the provider registry for chat proxy.
func (h *OverlordInternalHandler) SetProviderRegistry(pr *provider.Registry) {
	h.providerRegistry = pr
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

	squad := model.Squad{
		Name:        req.Name,
		Description: req.Description,
		CaptainNode: h.identity.NodeID,
		UserID:      "overlord:" + req.OverlordRef, // mark as overlord-managed
		Status:      "active",
		MaxMembers:  req.MaxMembers,
		Tags:        tagsJSON,
	}

	if err := h.db.Create(&squad).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create squad"})
		return
	}

	// Auto-add self as captain
	member := model.SquadMember{
		SquadID:  squad.ID,
		NodeID:   h.identity.NodeID,
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
