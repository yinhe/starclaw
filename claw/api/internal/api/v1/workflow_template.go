package v1

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw/internal/model"
	"gorm.io/gorm"
)

type WorkflowTemplateHandler struct {
	db *gorm.DB
}

func NewWorkflowTemplateHandler(db *gorm.DB) *WorkflowTemplateHandler {
	return &WorkflowTemplateHandler{db: db}
}

// List returns all workflow templates, optionally filtered by category
func (h *WorkflowTemplateHandler) List(c *gin.Context) {
	category := c.Query("category")
	q := h.db.Order("clone_count DESC, created_at DESC")
	if category != "" {
		q = q.Where("category = ?", category)
	}
	var templates []model.WorkflowTemplate
	q.Find(&templates)
	c.JSON(http.StatusOK, gin.H{"templates": templates})
}

// Publish creates a template from an existing workflow
func (h *WorkflowTemplateHandler) Publish(c *gin.Context) {
	userID := c.GetString("user_id")

	var req struct {
		WorkflowID  string `json:"workflow_id" binding:"required"`
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
		Category    string `json:"category"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var wf model.Workflow
	if err := h.db.Where("id = ? AND user_id = ?", req.WorkflowID, userID).First(&wf).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "workflow not found"})
		return
	}

	tmpl := model.WorkflowTemplate{
		UserID:      userID,
		Name:        req.Name,
		Description: req.Description,
		Category:    req.Category,
		Definition:  wf.Definition,
	}
	if err := h.db.Create(&tmpl).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to publish template"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"template": tmpl})
}

// Clone creates a new workflow from a template
func (h *WorkflowTemplateHandler) Clone(c *gin.Context) {
	userID := c.GetString("user_id")
	tmplID := c.Param("id")

	var tmpl model.WorkflowTemplate
	if err := h.db.Where("id = ?", tmplID).First(&tmpl).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "template not found"})
		return
	}

	wf := model.Workflow{
		UserID:      userID,
		Name:        tmpl.Name + " (模板)",
		Description: tmpl.Description,
		Definition:  tmpl.Definition,
	}
	if err := h.db.Create(&wf).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to clone template"})
		return
	}

	h.db.Model(&tmpl).Update("clone_count", gorm.Expr("clone_count + 1"))
	c.JSON(http.StatusCreated, gin.H{"workflow": wf})
}

// Delete removes a template (owner only)
func (h *WorkflowTemplateHandler) Delete(c *gin.Context) {
	userID := c.GetString("user_id")
	tmplID := c.Param("id")

	result := h.db.Where("id = ? AND user_id = ?", tmplID, userID).Delete(&model.WorkflowTemplate{})
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "template not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}
