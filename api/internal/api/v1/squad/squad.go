package squad

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw/internal/forge"
	"github.com/yinhe/starclaw/internal/model"
	"github.com/yinhe/starclaw/internal/node"
	"gorm.io/gorm"
)

// SquadHandler handles Squad CRUD and Mission management.
type SquadHandler struct {
	db       *gorm.DB
	identity *node.Identity
}

// NewSquadHandler creates a new SquadHandler.
func NewSquadHandler(db *gorm.DB, identity *node.Identity) *SquadHandler {
	return &SquadHandler{db: db, identity: identity}
}

// ── Squad CRUD ──

// CreateSquad creates a new squad with the current node as captain.
func (h *SquadHandler) CreateSquad(c *gin.Context) {
	userID := c.GetString("user_id")

	var req struct {
		Name        string   `json:"name" binding:"required"`
		Description string   `json:"description"`
		MaxMembers  int      `json:"max_members"`
		IsPublic    bool     `json:"is_public"`
		Tags        []string `json:"tags"`
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
		UserID:      userID,
		Status:      "forming",
		MaxMembers:  req.MaxMembers,
		IsPublic:    req.IsPublic,
		Tags:        tagsJSON,
	}

	if err := h.db.Create(&squad).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create squad"})
		return
	}

	// Auto-add self as captain member
	member := model.SquadMember{
		SquadID:  squad.ID,
		NodeID:   h.identity.NodeID,
		Role:     "captain",
		Status:   "online",
		JoinedAt: time.Now(),
	}
	h.db.Create(&member)

	c.JSON(http.StatusOK, gin.H{"squad": squad})
}

// ListSquads returns squads the current user owns or participates in.
func (h *SquadHandler) ListSquads(c *gin.Context) {
	userID := c.GetString("user_id")

	var squads []model.Squad
	h.db.Where("user_id = ?", userID).Order("created_at DESC").Find(&squads)

	// Also find squads where this node is a member
	var memberSquadIDs []string
	h.db.Model(&model.SquadMember{}).Where("node_id = ?", h.identity.NodeID).Pluck("squad_id", &memberSquadIDs)

	if len(memberSquadIDs) > 0 {
		var peerSquads []model.Squad
		h.db.Where("id IN ? AND user_id != ?", memberSquadIDs, userID).Find(&peerSquads)
		squads = append(squads, peerSquads...)
	}

	c.JSON(http.StatusOK, gin.H{"squads": squads})
}

// GetSquad returns a squad with its members.
func (h *SquadHandler) GetSquad(c *gin.Context) {
	squadID := c.Param("id")

	var squad model.Squad
	if err := h.db.Where("id = ?", squadID).First(&squad).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "squad not found"})
		return
	}

	var members []model.SquadMember
	h.db.Where("squad_id = ?", squadID).Find(&members)

	c.JSON(http.StatusOK, gin.H{"squad": squad, "members": members})
}

// UpdateSquad updates a squad's info (captain only).
func (h *SquadHandler) UpdateSquad(c *gin.Context) {
	squadID := c.Param("id")
	userID := c.GetString("user_id")

	var squad model.Squad
	if err := h.db.Where("id = ? AND user_id = ?", squadID, userID).First(&squad).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "squad not found"})
		return
	}

	var req struct {
		Name        *string  `json:"name"`
		Description *string  `json:"description"`
		MaxMembers  *int     `json:"max_members"`
		IsPublic    *bool    `json:"is_public"`
		Tags        []string `json:"tags"`
		Status      *string  `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := map[string]interface{}{}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.MaxMembers != nil {
		updates["max_members"] = *req.MaxMembers
	}
	if req.IsPublic != nil {
		updates["is_public"] = *req.IsPublic
	}
	if req.Tags != nil {
		updates["tags"] = toJSON(req.Tags)
	}
	if req.Status != nil && (*req.Status == "active" || *req.Status == "disbanded") {
		updates["status"] = *req.Status
	}

	h.db.Model(&squad).Updates(updates)
	h.db.First(&squad, "id = ?", squadID)

	c.JSON(http.StatusOK, gin.H{"squad": squad})
}

// DeleteSquad disbands a squad (captain only).
func (h *SquadHandler) DeleteSquad(c *gin.Context) {
	squadID := c.Param("id")
	userID := c.GetString("user_id")

	result := h.db.Where("id = ? AND user_id = ?", squadID, userID).Delete(&model.Squad{})
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "squad not found"})
		return
	}

	// Clean up members
	h.db.Where("squad_id = ?", squadID).Delete(&model.SquadMember{})

	c.JSON(http.StatusOK, gin.H{"message": "squad disbanded"})
}

// ── Members ──

// InviteMember invites a peer to join the squad.
func (h *SquadHandler) InviteMember(c *gin.Context) {
	squadID := c.Param("id")
	userID := c.GetString("user_id")

	var squad model.Squad
	if err := h.db.Where("id = ? AND user_id = ?", squadID, userID).First(&squad).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "squad not found"})
		return
	}

	var req struct {
		NodeID    string `json:"node_id" binding:"required"` // claw:xxx
		Specialty string `json:"specialty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check member count
	var count int64
	h.db.Model(&model.SquadMember{}).Where("squad_id = ?", squadID).Count(&count)
	if int(count) >= squad.MaxMembers {
		c.JSON(http.StatusConflict, gin.H{"error": "squad is full"})
		return
	}

	// Check if already a member
	var existing model.SquadMember
	if err := h.db.Where("squad_id = ? AND node_id = ?", squadID, req.NodeID).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "node already a member"})
		return
	}

	// Look up peer info
	var peer model.Peer
	h.db.Where("node_id = ?", req.NodeID).First(&peer)

	member := model.SquadMember{
		SquadID:   squadID,
		NodeID:    req.NodeID,
		PeerID:    peer.ID,
		Role:      "member",
		Specialty: req.Specialty,
		Status:    "offline",
		JoinedAt:  time.Now(),
	}

	if err := h.db.Create(&member).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to add member"})
		return
	}

	// TODO: Send invite via Nydus relay to the remote node (P2)

	c.JSON(http.StatusOK, gin.H{"member": member})
}

// ListMembers returns squad members.
func (h *SquadHandler) ListMembers(c *gin.Context) {
	squadID := c.Param("id")

	var members []model.SquadMember
	h.db.Where("squad_id = ?", squadID).Find(&members)

	c.JSON(http.StatusOK, gin.H{"members": members})
}

// RemoveMember removes a member from the squad.
func (h *SquadHandler) RemoveMember(c *gin.Context) {
	squadID := c.Param("id")
	nodeID := c.Param("nodeId")
	userID := c.GetString("user_id")

	// Verify caller is captain
	var squad model.Squad
	if err := h.db.Where("id = ? AND user_id = ?", squadID, userID).First(&squad).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "squad not found"})
		return
	}

	// Cannot remove captain
	if nodeID == squad.CaptainNode {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot remove captain"})
		return
	}

	result := h.db.Where("squad_id = ? AND node_id = ?", squadID, nodeID).Delete(&model.SquadMember{})
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "member not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "member removed"})
}

// ── Mission CRUD ──

// CreateMission creates a mission for a squad.
func (h *SquadHandler) CreateMission(c *gin.Context) {
	squadID := c.Param("id")
	userID := c.GetString("user_id")

	var squad model.Squad
	if err := h.db.Where("id = ? AND user_id = ?", squadID, userID).First(&squad).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "squad not found"})
		return
	}

	var req struct {
		Title string `json:"title" binding:"required"`
		Goal  string `json:"goal" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	mission := model.Mission{
		SquadID:     squadID,
		Title:       req.Title,
		Goal:        req.Goal,
		Status:      "planning",
		CaptainNode: squad.CaptainNode,
		UserID:      userID,
	}

	if err := h.db.Create(&mission).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create mission"})
		return
	}

	// Forge bridge: auto-create epic issue
	go forge.OnMissionCreated(h.db, mission)

	c.JSON(http.StatusOK, gin.H{"mission": mission})
}

// ListMissions returns missions for a squad.
func (h *SquadHandler) ListMissions(c *gin.Context) {
	squadID := c.Param("id")

	var missions []model.Mission
	h.db.Where("squad_id = ?", squadID).Order("created_at DESC").Find(&missions)

	c.JSON(http.StatusOK, gin.H{"missions": missions})
}

// GetMission returns a mission with its steps.
func (h *SquadHandler) GetMission(c *gin.Context) {
	missionID := c.Param("id")

	var mission model.Mission
	if err := h.db.Where("id = ?", missionID).First(&mission).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "mission not found"})
		return
	}

	var steps []model.MissionStep
	h.db.Where("mission_id = ?", missionID).Order("sequence ASC").Find(&steps)

	c.JSON(http.StatusOK, gin.H{"mission": mission, "steps": steps})
}

// StartMission triggers the Squad engine to plan and execute a mission.
func (h *SquadHandler) StartMission(c *gin.Context) {
	missionID := c.Param("id")
	userID := c.GetString("user_id")

	var mission model.Mission
	if err := h.db.Where("id = ? AND user_id = ?", missionID, userID).First(&mission).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "mission not found"})
		return
	}

	if mission.Status != "planning" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "mission already started"})
		return
	}

	// Update status to executing — actual orchestration handled by SquadEngine (P3)
	h.db.Model(&mission).Update("status", "executing")

	c.JSON(http.StatusOK, gin.H{"mission": mission, "message": "mission started"})
}

// CancelMission cancels a running mission.
func (h *SquadHandler) CancelMission(c *gin.Context) {
	missionID := c.Param("id")
	userID := c.GetString("user_id")

	var mission model.Mission
	if err := h.db.Where("id = ? AND user_id = ?", missionID, userID).First(&mission).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "mission not found"})
		return
	}

	h.db.Model(&mission).Update("status", "failed")
	h.db.Model(&model.MissionStep{}).Where("mission_id = ? AND status IN ?", missionID, []string{"pending", "dispatched", "running"}).Update("status", "failed")

	c.JSON(http.StatusOK, gin.H{"message": "mission cancelled"})
}

// ── HiveMind Dashboard endpoints ──

// ListAllMissions returns all missions across all squads for the current user.
func (h *SquadHandler) ListAllMissions(c *gin.Context) {
	userID := c.GetString("user_id")

	var missions []model.Mission
	h.db.Where("user_id = ?", userID).Order("created_at DESC").Limit(20).Find(&missions)

	c.JSON(http.StatusOK, gin.H{"missions": missions})
}

// ListMissionSteps returns all steps for a mission.
func (h *SquadHandler) ListMissionSteps(c *gin.Context) {
	missionID := c.Param("id")

	var steps []model.MissionStep
	h.db.Where("mission_id = ?", missionID).Order("sequence ASC").Find(&steps)

	c.JSON(http.StatusOK, gin.H{"steps": steps})
}

// ListSprints returns all sprints for a mission.
func (h *SquadHandler) ListSprints(c *gin.Context) {
	missionID := c.Param("id")

	var sprints []model.Sprint
	h.db.Where("mission_id = ?", missionID).Order("number ASC").Find(&sprints)

	c.JSON(http.StatusOK, gin.H{"sprints": sprints})
}

// ── User Feedback (20.7.9) ──

// SubmitFeedback stores user feedback on a mission's current sprint.
func (h *SquadHandler) SubmitFeedback(c *gin.Context) {
	missionID := c.Param("id")

	var req struct {
		Feedback string `json:"feedback" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "feedback is required"})
		return
	}

	var sprint model.Sprint
	if err := h.db.Where("mission_id = ?", missionID).
		Order("number DESC").First(&sprint).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no sprint found for mission"})
		return
	}

	existing := sprint.UserFeedback
	if existing != "" {
		req.Feedback = existing + "\n---\n" + req.Feedback
	}

	h.db.Model(&sprint).Update("user_feedback", req.Feedback)

	c.JSON(http.StatusOK, gin.H{"message": "feedback submitted", "sprint_number": sprint.Number})
}

// ListStepReviews returns all code reviews for a mission's steps.
func (h *SquadHandler) ListStepReviews(c *gin.Context) {
	missionID := c.Param("id")

	var steps []model.MissionStep
	h.db.Where("mission_id = ?", missionID).Find(&steps)

	stepIDs := make([]string, len(steps))
	for i, s := range steps {
		stepIDs[i] = s.ID
	}

	var reviews []model.StepReview
	if len(stepIDs) > 0 {
		h.db.Where("step_id IN ?", stepIDs).Order("created_at ASC").Find(&reviews)
	}

	c.JSON(http.StatusOK, gin.H{"reviews": reviews})
}

// ── Helpers ──

func toJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}
