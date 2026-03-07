package v1

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw/internal/model"
	"gorm.io/gorm"
)

type MemoryHandler struct {
	db *gorm.DB
}

func NewMemoryHandler(db *gorm.DB) *MemoryHandler {
	return &MemoryHandler{db: db}
}

// List returns memories for a specific agent
func (h *MemoryHandler) List(c *gin.Context) {
	userID := c.GetString("user_id")
	agentID := c.Query("agent_id")

	q := h.db.Where("user_id = ?", userID)
	if agentID != "" {
		q = q.Where("agent_id = ?", agentID)
	}

	var memories []model.Memory
	q.Order("importance DESC, updated_at DESC").Limit(50).Find(&memories)
	c.JSON(http.StatusOK, gin.H{"memories": memories})
}

// Create adds a new memory entry
func (h *MemoryHandler) Create(c *gin.Context) {
	userID := c.GetString("user_id")

	var req struct {
		AgentID    string  `json:"agent_id" binding:"required"`
		Key        string  `json:"key" binding:"required"`
		Content    string  `json:"content" binding:"required"`
		Category   string  `json:"category"`
		Importance float64 `json:"importance"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Importance <= 0 {
		req.Importance = 0.5
	}
	if req.Category == "" {
		req.Category = "fact"
	}

	mem := model.Memory{
		UserID:     userID,
		AgentID:    req.AgentID,
		Key:        req.Key,
		Content:    req.Content,
		Category:   req.Category,
		Importance: req.Importance,
	}
	if err := h.db.Create(&mem).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create memory"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"memory": mem})
}

// Update modifies an existing memory
func (h *MemoryHandler) Update(c *gin.Context) {
	userID := c.GetString("user_id")
	id := c.Param("id")

	var mem model.Memory
	if err := h.db.Where("id = ? AND user_id = ?", id, userID).First(&mem).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "memory not found"})
		return
	}

	var req struct {
		Content    string  `json:"content"`
		Importance float64 `json:"importance"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := map[string]interface{}{}
	if req.Content != "" {
		updates["content"] = req.Content
	}
	if req.Importance > 0 {
		updates["importance"] = req.Importance
	}

	h.db.Model(&mem).Updates(updates)
	c.JSON(http.StatusOK, gin.H{"memory": mem})
}

// Delete removes a memory
func (h *MemoryHandler) Delete(c *gin.Context) {
	userID := c.GetString("user_id")
	id := c.Param("id")

	result := h.db.Where("id = ? AND user_id = ?", id, userID).Delete(&model.Memory{})
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "memory not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

// Recall retrieves relevant memories for an agent (used by agent runtime)
func (h *MemoryHandler) Recall(c *gin.Context) {
	userID := c.GetString("user_id")
	agentID := c.Param("agent_id")

	var memories []model.Memory
	h.db.Where("user_id = ? AND agent_id = ?", userID, agentID).
		Order("importance DESC, access_count DESC").
		Limit(10).
		Find(&memories)

	// Update access counts
	ids := make([]string, len(memories))
	for i, m := range memories {
		ids[i] = m.ID
	}
	if len(ids) > 0 {
		h.db.Model(&model.Memory{}).Where("id IN ?", ids).
			Updates(map[string]interface{}{
				"access_count":  gorm.Expr("access_count + 1"),
				"last_access_at": time.Now(),
			})
	}

	c.JSON(http.StatusOK, gin.H{"memories": memories})
}
