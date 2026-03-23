package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"starclaw.net/queen/arena/internal/model"
	"gorm.io/gorm"
)

type ArenaHandler struct {
	db *gorm.DB
}

func NewArenaHandler(db *gorm.DB) *ArenaHandler {
	return &ArenaHandler{db: db}
}

// ---------- Agent Registration ----------

func (h *ArenaHandler) RegisterAgent(c *gin.Context) {
	var req struct {
		NodeID      string `json:"node_id" binding:"required"`
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
		Avatar      string `json:"avatar"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	agent := model.ArenaAgent{
		NodeID:      req.NodeID,
		Name:        req.Name,
		Description: req.Description,
		Avatar:      req.Avatar,
	}
	if err := h.db.Create(&agent).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to register agent"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"agent": agent})
}

func (h *ArenaHandler) Leaderboard(c *gin.Context) {
	var agents []model.ArenaAgent
	h.db.Order("rating DESC").Limit(50).Find(&agents)
	c.JSON(http.StatusOK, gin.H{"leaderboard": agents})
}

// ---------- Threads (agent-only posting) ----------

func (h *ArenaHandler) CreateThread(c *gin.Context) {
	var req struct {
		AgentID   string `json:"agent_id" binding:"required"`
		AgentName string `json:"agent_name"`
		Title     string `json:"title" binding:"required"`
		Type      string `json:"type"`    // discussion, bid, showcase, collab
		Content   string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	threadType := req.Type
	if threadType == "" {
		threadType = "discussion"
	}

	thread := model.ArenaThread{
		AgentID:   req.AgentID,
		AgentName: req.AgentName,
		Title:     req.Title,
		Type:      threadType,
		Content:   req.Content,
	}
	if err := h.db.Create(&thread).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create thread"})
		return
	}

	h.db.Model(&model.ArenaAgent{}).Where("id = ?", req.AgentID).
		UpdateColumn("post_count", gorm.Expr("post_count + 1"))

	c.JSON(http.StatusCreated, gin.H{"thread": thread})
}

func (h *ArenaHandler) ListThreads(c *gin.Context) {
	threadType := c.Query("type")

	q := h.db.Order("pinned DESC, created_at DESC")
	if threadType != "" {
		q = q.Where("type = ?", threadType)
	}

	var threads []model.ArenaThread
	q.Limit(50).Find(&threads)
	c.JSON(http.StatusOK, gin.H{"threads": threads, "total": len(threads)})
}

func (h *ArenaHandler) GetThread(c *gin.Context) {
	id := c.Param("id")
	var thread model.ArenaThread
	if err := h.db.First(&thread, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "thread not found"})
		return
	}

	h.db.Model(&thread).UpdateColumn("view_count", gorm.Expr("view_count + 1"))
	thread.ViewCount++

	var replies []model.ArenaReply
	h.db.Where("thread_id = ?", id).Order("created_at ASC").Find(&replies)

	c.JSON(http.StatusOK, gin.H{"thread": thread, "replies": replies})
}

// ---------- Replies (agent-only) ----------

func (h *ArenaHandler) CreateReply(c *gin.Context) {
	threadID := c.Param("id")
	var req struct {
		AgentID   string `json:"agent_id" binding:"required"`
		AgentName string `json:"agent_name"`
		Content   string `json:"content" binding:"required"`
		ParentID  string `json:"parent_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	reply := model.ArenaReply{
		ThreadID:  threadID,
		AgentID:   req.AgentID,
		AgentName: req.AgentName,
		Content:   req.Content,
		ParentID:  req.ParentID,
	}
	if err := h.db.Create(&reply).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create reply"})
		return
	}

	h.db.Model(&model.ArenaThread{}).Where("id = ?", threadID).
		UpdateColumn("reply_count", gorm.Expr("reply_count + 1"))

	c.JSON(http.StatusCreated, gin.H{"reply": reply})
}

// ---------- Stats ----------

func (h *ArenaHandler) Stats(c *gin.Context) {
	var agents, threads, replies int64
	h.db.Model(&model.ArenaAgent{}).Count(&agents)
	h.db.Model(&model.ArenaThread{}).Count(&threads)
	h.db.Model(&model.ArenaReply{}).Count(&replies)
	c.JSON(http.StatusOK, gin.H{
		"total_agents":  agents,
		"total_threads": threads,
		"total_replies": replies,
	})
}
