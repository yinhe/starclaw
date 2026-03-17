package v1

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	agentpkg "github.com/yinhe/starclaw/internal/agent"
	"gorm.io/gorm"
)

// AgentAdvancedHandler manages P10 features: multimodal, proactive goals, collaboration.
type AgentAdvancedHandler struct {
	db            *gorm.DB
	mmRouter      *agentpkg.MultimodalRouter
	proactive     *agentpkg.ProactiveEngine
	collaboration *agentpkg.CollaborationEngine
}

// NewAgentAdvancedHandler creates the handler.
func NewAgentAdvancedHandler(db *gorm.DB, mm *agentpkg.MultimodalRouter, pe *agentpkg.ProactiveEngine, ce *agentpkg.CollaborationEngine) *AgentAdvancedHandler {
	return &AgentAdvancedHandler{db: db, mmRouter: mm, proactive: pe, collaboration: ce}
}

// ════════════════════════════════════════════════════════════════
//  Multimodal Endpoints
// ════════════════════════════════════════════════════════════════

// MultimodalChat handles a multimodal chat request.
func (h *AgentAdvancedHandler) MultimodalChat(c *gin.Context) {
	var req agentpkg.MultimodalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Process all inputs through the modality router
	combinedText, analyses, err := h.mmRouter.ProcessInputs(c.Request.Context(), req.Inputs)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Generate outputs for requested modalities
	outputs, err := h.mmRouter.GenerateOutputs(c.Request.Context(), combinedText, req.OutputModality)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate outputs"})
		return
	}

	c.JSON(http.StatusOK, agentpkg.MultimodalResponse{
		Outputs:       outputs,
		InputAnalysis: analyses,
	})
}

// SupportedModalities returns available modalities and their MIME types.
func (h *AgentAdvancedHandler) SupportedModalities(c *gin.Context) {
	c.JSON(http.StatusOK, h.mmRouter.SupportedModalities())
}

// ════════════════════════════════════════════════════════════════
//  Proactive Goals
// ════════════════════════════════════════════════════════════════

// CreateGoal creates a new autonomous goal.
func (h *AgentAdvancedHandler) CreateGoal(c *gin.Context) {
	userID := c.GetString("user_id")

	var req struct {
		AgentID       string `json:"agent_id" binding:"required"`
		Title         string `json:"title" binding:"required"`
		Description   string `json:"description"`
		Priority      int    `json:"priority"`
		Deadline      string `json:"deadline"`
		TriggerType   string `json:"trigger_type"`
		TriggerConfig string `json:"trigger_config"`
		MaxSteps      int    `json:"max_steps"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	goal := agentpkg.Goal{
		AgentID:       req.AgentID,
		UserID:        userID,
		Title:         req.Title,
		Description:   req.Description,
		Priority:      req.Priority,
		TriggerType:   req.TriggerType,
		TriggerConfig: req.TriggerConfig,
		MaxSteps:      req.MaxSteps,
		Status:        agentpkg.GoalPending,
	}
	if goal.Priority == 0 {
		goal.Priority = 5
	}
	if goal.MaxSteps == 0 {
		goal.MaxSteps = 20
	}
	if goal.TriggerType == "" {
		goal.TriggerType = "manual"
	}

	if err := h.proactive.CreateGoal(&goal); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create goal"})
		return
	}
	c.JSON(http.StatusCreated, goal)
}

// ListGoals returns the user's goals.
func (h *AgentAdvancedHandler) ListGoals(c *gin.Context) {
	userID := c.GetString("user_id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	status := agentpkg.GoalStatus(c.Query("status"))

	if page < 1 {
		page = 1
	}

	goals, total := h.proactive.ListGoals(userID, status, page, pageSize)
	c.JSON(http.StatusOK, gin.H{
		"items":     goals,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// GetGoal returns a single goal with its steps.
func (h *AgentAdvancedHandler) GetGoal(c *gin.Context) {
	id := c.Param("id")
	goal, err := h.proactive.GetGoal(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "goal not found"})
		return
	}

	steps, _ := h.proactive.GetSteps(id)
	c.JSON(http.StatusOK, gin.H{"goal": goal, "steps": steps})
}

// ActivateGoal manually activates a pending goal.
func (h *AgentAdvancedHandler) ActivateGoal(c *gin.Context) {
	userID := c.GetString("user_id")
	id := c.Param("id")

	goal, err := h.proactive.GetGoal(id)
	if err != nil || goal.UserID != userID {
		c.JSON(http.StatusNotFound, gin.H{"error": "goal not found"})
		return
	}

	h.db.Model(goal).Update("status", agentpkg.GoalActive)
	c.JSON(http.StatusOK, gin.H{"status": "active"})
}

// CancelGoal cancels a goal.
func (h *AgentAdvancedHandler) CancelGoal(c *gin.Context) {
	userID := c.GetString("user_id")
	id := c.Param("id")

	if err := h.proactive.CancelGoal(id, userID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "goal not found or cannot cancel"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "cancelled"})
}

// GoalStats returns proactive engine statistics.
func (h *AgentAdvancedHandler) GoalStats(c *gin.Context) {
	userID := c.GetString("user_id")
	stats := h.proactive.Stats(userID)
	c.JSON(http.StatusOK, stats)
}

// DecompositionPrompt returns the goal decomposition prompt template.
func (h *AgentAdvancedHandler) DecompositionPrompt(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"prompt":  agentpkg.GetGoalDecompositionPrompt(),
		"actions": []string{"think", "tool_call", "sub_goal", "decide", "report"},
	})
}

// ════════════════════════════════════════════════════════════════
//  Multi-Agent Collaboration
// ════════════════════════════════════════════════════════════════

// CreateCollaboration starts a new collaboration session.
func (h *AgentAdvancedHandler) CreateCollaboration(c *gin.Context) {
	userID := c.GetString("user_id")

	var req struct {
		Title     string `json:"title" binding:"required"`
		GoalID    string `json:"goal_id"`
		Protocol  string `json:"protocol"`
		MaxAgents int    `json:"max_agents"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	collab := agentpkg.Collaboration{
		Title:     req.Title,
		GoalID:    req.GoalID,
		Protocol:  req.Protocol,
		CreatorID: userID,
		MaxAgents: req.MaxAgents,
	}
	if collab.Protocol == "" {
		collab.Protocol = "consensus"
	}
	if collab.MaxAgents == 0 {
		collab.MaxAgents = 5
	}

	if err := h.collaboration.CreateCollaboration(c.Request.Context(), &collab); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create collaboration"})
		return
	}
	c.JSON(http.StatusCreated, collab)
}

// JoinCollaboration adds an agent to a collaboration.
func (h *AgentAdvancedHandler) JoinCollaboration(c *gin.Context) {
	collabID := c.Param("id")

	var req struct {
		AgentID      string `json:"agent_id" binding:"required"`
		Role         string `json:"role"`
		Capabilities string `json:"capabilities"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	role := agentpkg.CollaborationRole(req.Role)
	if role == "" {
		role = agentpkg.RoleWorker
	}

	if err := h.collaboration.JoinCollaboration(collabID, req.AgentID, role, req.Capabilities); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to join"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"joined": true})
}

// CollaborationMessages retrieves messages in a collaboration.
func (h *AgentAdvancedHandler) CollaborationMessages(c *gin.Context) {
	collabID := c.Param("id")
	msgs, err := h.collaboration.GetMessages(collabID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get messages"})
		return
	}
	c.JSON(http.StatusOK, msgs)
}

// CollaborationMembers retrieves members of a collaboration.
func (h *AgentAdvancedHandler) CollaborationMembers(c *gin.Context) {
	collabID := c.Param("id")
	members, err := h.collaboration.GetMembers(collabID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get members"})
		return
	}
	c.JSON(http.StatusOK, members)
}

// SendCollaborationMessage sends a message in a collaboration.
func (h *AgentAdvancedHandler) SendCollaborationMessage(c *gin.Context) {
	collabID := c.Param("id")

	var req struct {
		AgentID     string `json:"agent_id" binding:"required"`
		MessageType string `json:"message_type" binding:"required"`
		Content     string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.collaboration.SendMessage(collabID, req.AgentID, req.MessageType, req.Content); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to send message"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"sent": true})
}

// SubmitVote records an agent's vote in a consensus collaboration.
func (h *AgentAdvancedHandler) SubmitVote(c *gin.Context) {
	collabID := c.Param("id")

	var req struct {
		AgentID string `json:"agent_id" binding:"required"`
		Vote    string `json:"vote" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.collaboration.SubmitVote(collabID, req.AgentID, req.Vote); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to submit vote"})
		return
	}

	reached, decision := h.collaboration.CheckConsensus(collabID)
	c.JSON(http.StatusOK, gin.H{
		"voted":             true,
		"consensus_reached": reached,
		"decision":          decision,
	})
}

// ListCollaborations returns active collaborations.
func (h *AgentAdvancedHandler) ListCollaborations(c *gin.Context) {
	userID := c.GetString("user_id")
	var collabs []agentpkg.Collaboration
	h.db.Where("creator_id = ?", userID).Order("created_at DESC").Find(&collabs)
	c.JSON(http.StatusOK, collabs)
}
