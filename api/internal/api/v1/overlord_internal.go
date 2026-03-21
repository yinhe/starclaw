package v1

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw/internal/model"
	"github.com/yinhe/starclaw/internal/node"
	"gorm.io/gorm"
)

// OverlordInternalHandler serves endpoints called by Overlord for Team Agent orchestration.
// Authenticated via X-Overlord-Token header matching OVERLORD_CLAW_TOKEN env.
type OverlordInternalHandler struct {
	db       *gorm.DB
	identity *node.Identity
	token    string
}

// NewOverlordInternalHandler creates the handler.
func NewOverlordInternalHandler(db *gorm.DB, identity *node.Identity) *OverlordInternalHandler {
	token := os.Getenv("OVERLORD_CLAW_TOKEN")
	if token == "" {
		token = "overlord-internal-default"
	}
	return &OverlordInternalHandler{db: db, identity: identity, token: token}
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
