package workflow

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw/internal/model"
	"gorm.io/gorm"
)

type ScheduleHandler struct {
	db *gorm.DB
}

func NewScheduleHandler(db *gorm.DB) *ScheduleHandler {
	return &ScheduleHandler{db: db}
}

func (h *ScheduleHandler) List(c *gin.Context) {
	userID := c.GetString("user_id")
	var schedules []model.Schedule
	h.db.Where("user_id = ?", userID).Order("created_at DESC").Find(&schedules)

	// Join conversation titles
	type convTitle struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	}
	var convIDs []string
	for _, s := range schedules {
		if s.ConversationID != "" {
			convIDs = append(convIDs, s.ConversationID)
		}
	}
	convMap := make(map[string]string)
	if len(convIDs) > 0 {
		var titles []convTitle
		h.db.Raw("SELECT id, COALESCE(title,'') as title FROM conversations WHERE id IN ?", convIDs).Scan(&titles)
		for _, t := range titles {
			convMap[t.ID] = t.Title
		}
	}

	type enrichedSchedule struct {
		model.Schedule
		ConversationTitle string `json:"conversation_title"`
	}
	result := make([]enrichedSchedule, len(schedules))
	for i, s := range schedules {
		result[i] = enrichedSchedule{Schedule: s, ConversationTitle: convMap[s.ConversationID]}
	}

	c.JSON(http.StatusOK, gin.H{"schedules": result})
}

func (h *ScheduleHandler) Create(c *gin.Context) {
	userID := c.GetString("user_id")

	var req struct {
		WorkflowID string `json:"workflow_id" binding:"required"`
		CronExpr   string `json:"cron_expr" binding:"required"`
		Input      string `json:"input"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Verify workflow ownership
	var wf model.Workflow
	if err := h.db.Where("id = ? AND user_id = ?", req.WorkflowID, userID).First(&wf).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "workflow not found"})
		return
	}

	schedule := model.Schedule{
		UserID:     userID,
		WorkflowID: req.WorkflowID,
		CronExpr:   req.CronExpr,
		Input:      req.Input,
		Enabled:    true,
	}
	if err := h.db.Create(&schedule).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create schedule"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"schedule": schedule})
}

func (h *ScheduleHandler) Toggle(c *gin.Context) {
	userID := c.GetString("user_id")
	id := c.Param("id")

	var schedule model.Schedule
	if err := h.db.Where("id = ? AND user_id = ?", id, userID).First(&schedule).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "schedule not found"})
		return
	}

	h.db.Model(&schedule).Update("enabled", !schedule.Enabled)
	c.JSON(http.StatusOK, gin.H{"enabled": !schedule.Enabled})
}

func (h *ScheduleHandler) BatchDelete(c *gin.Context) {
	userID := c.GetString("user_id")
	var req struct {
		IDs []string `json:"ids"`
	}
	c.ShouldBindJSON(&req) // optional body

	q := h.db.Where("user_id = ?", userID)
	if len(req.IDs) > 0 {
		q = q.Where("id IN ?", req.IDs)
	}
	result := q.Delete(&model.Schedule{})
	c.JSON(http.StatusOK, gin.H{"deleted": result.RowsAffected})
}

func (h *ScheduleHandler) Delete(c *gin.Context) {
	userID := c.GetString("user_id")
	id := c.Param("id")

	result := h.db.Where("id = ? AND user_id = ?", id, userID).Delete(&model.Schedule{})
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "schedule not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}
