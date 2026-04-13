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
	c.JSON(http.StatusOK, gin.H{"schedules": schedules})
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
