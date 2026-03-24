package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"starclaw.net/forge/internal/model"
)

type ProjectHandler struct{ DB *gorm.DB }

func (h *ProjectHandler) List(c *gin.Context) {
	var projects []model.ForgeProject
	q := h.DB.Order("created_at DESC")
	if s := c.Query("status"); s != "" {
		q = q.Where("status = ?", s)
	}
	q.Find(&projects)
	c.JSON(http.StatusOK, gin.H{"projects": projects})
}

func (h *ProjectHandler) Get(c *gin.Context) {
	var project model.ForgeProject
	if err := h.DB.First(&project, "id = ?", c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
		return
	}

	// Stats
	var issueCount, doneCount int64
	h.DB.Model(&model.ForgeIssue{}).Where("project_id = ?", project.ID).Count(&issueCount)
	h.DB.Model(&model.ForgeIssue{}).Where("project_id = ? AND status IN ('done','closed')", project.ID).Count(&doneCount)

	var sprintCount int64
	h.DB.Model(&model.ForgeSprint{}).Where("project_id = ?", project.ID).Count(&sprintCount)

	c.JSON(http.StatusOK, gin.H{
		"project":      project,
		"issue_count":  issueCount,
		"done_count":   doneCount,
		"sprint_count": sprintCount,
	})
}

func (h *ProjectHandler) Create(c *gin.Context) {
	var req struct {
		Name        string `json:"name" binding:"required"`
		Key         string `json:"key" binding:"required"`
		Description string `json:"description"`
		OwnerType   string `json:"owner_type"`
		NydusRepo   string `json:"nydus_repo"`
		Tags        string `json:"tags"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	project := model.ForgeProject{
		Name:        req.Name,
		Key:         req.Key,
		Description: req.Description,
		OwnerType:   req.OwnerType,
		NydusRepo:   req.NydusRepo,
		Tags:        req.Tags,
		Status:      "active",
	}
	if project.OwnerType == "" {
		project.OwnerType = "monorepo"
	}

	if err := h.DB.Create(&project).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "project key already exists"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"project": project})
}

func (h *ProjectHandler) Update(c *gin.Context) {
	var project model.ForgeProject
	if err := h.DB.First(&project, "id = ?", c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
		return
	}
	var req map[string]interface{}
	c.ShouldBindJSON(&req)
	h.DB.Model(&project).Updates(req)
	c.JSON(http.StatusOK, gin.H{"project": project})
}

func (h *ProjectHandler) Delete(c *gin.Context) {
	h.DB.Model(&model.ForgeProject{}).Where("id = ?", c.Param("id")).Update("status", "archived")
	c.JSON(http.StatusOK, gin.H{"status": "archived"})
}
