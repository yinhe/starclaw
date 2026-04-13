package knowledge

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

// List returns memories for a specific agent, with optional category and search filter
func (h *MemoryHandler) List(c *gin.Context) {
	userID := c.GetString("user_id")
	agentID := c.Query("agent_id")
	category := c.Query("category")
	search := c.Query("search")

	q := h.db.Where("user_id = ?", userID)
	if agentID != "" {
		q = q.Where("agent_id = ?", agentID)
	}
	if category != "" {
		q = q.Where("category = ?", category)
	}
	if search != "" {
		q = q.Where("(key LIKE ? OR content LIKE ?)", "%"+search+"%", "%"+search+"%")
	}

	var total int64
	q.Model(&model.Memory{}).Count(&total)

	var memories []model.Memory
	q.Order("importance DESC, updated_at DESC").Limit(100).Find(&memories)
	c.JSON(http.StatusOK, gin.H{"memories": memories, "total": total})
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
		Source:     "user_explicit",
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

// Clear removes all memories for a user+agent
func (h *MemoryHandler) Clear(c *gin.Context) {
	userID := c.GetString("user_id")
	agentID := c.Query("agent_id")

	q := h.db.Where("user_id = ?", userID)
	if agentID != "" {
		q = q.Where("agent_id = ?", agentID)
	}
	result := q.Delete(&model.Memory{})
	c.JSON(http.StatusOK, gin.H{"deleted": result.RowsAffected})
}

// Stats returns memory statistics for the user
func (h *MemoryHandler) Stats(c *gin.Context) {
	userID := c.GetString("user_id")

	var total int64
	h.db.Model(&model.Memory{}).Where("user_id = ?", userID).Count(&total)

	// Count by category
	type catCount struct {
		Category string
		Count    int64
	}
	var counts []catCount
	h.db.Model(&model.Memory{}).Where("user_id = ?", userID).
		Select("category, count(*) as count").Group("category").Find(&counts)

	catMap := map[string]int64{}
	for _, cc := range counts {
		catMap[cc.Category] = cc.Count
	}

	c.JSON(http.StatusOK, gin.H{
		"total":      total,
		"categories": catMap,
	})
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
				"access_count":   gorm.Expr("access_count + 1"),
				"last_access_at": time.Now(),
			})
	}

	c.JSON(http.StatusOK, gin.H{"memories": memories})
}
