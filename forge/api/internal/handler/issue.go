package handler

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"starclaw.net/forge/internal/model"
)

type IssueHandler struct{ DB *gorm.DB }

func (h *IssueHandler) List(c *gin.Context) {
	var issues []model.ForgeIssue
	q := h.DB.Where("project_id = ?", c.Param("id")).Order("position ASC, created_at DESC")
	if s := c.Query("status"); s != "" && s != "all" {
		q = q.Where("status = ?", s)
	}
	if s := c.Query("sprint_id"); s != "" {
		q = q.Where("sprint_id = ?", s)
	}
	if s := c.Query("assignee"); s != "" {
		q = q.Where("assignee = ?", s)
	}
	if s := c.Query("service"); s != "" {
		q = q.Where("service = ?", s)
	}
	if s := c.Query("type"); s != "" {
		q = q.Where("type = ?", s)
	}
	q.Find(&issues)
	c.JSON(http.StatusOK, gin.H{"issues": issues})
}

func (h *IssueHandler) Get(c *gin.Context) {
	var issue model.ForgeIssue
	if err := h.DB.First(&issue, "id = ?", c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "issue not found"})
		return
	}
	// Load comments
	var comments []model.ForgeIssueComment
	h.DB.Where("issue_id = ?", issue.ID).Order("created_at ASC").Find(&comments)
	c.JSON(http.StatusOK, gin.H{"issue": issue, "comments": comments})
}

func (h *IssueHandler) GetByKey(c *gin.Context) {
	var issue model.ForgeIssue
	if err := h.DB.First(&issue, "key = ?", c.Param("key")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "issue not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"issue": issue})
}

func (h *IssueHandler) Create(c *gin.Context) {
	projectID := c.Param("id")
	var project model.ForgeProject
	if err := h.DB.First(&project, "id = ?", projectID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
		return
	}

	var req struct {
		Title       string `json:"title" binding:"required"`
		Body        string `json:"body"`
		Type        string `json:"type"`
		Priority    string `json:"priority"`
		Assignee    string `json:"assignee"`
		Service     string `json:"service"`
		TaskType    string `json:"task_type"`
		SprintID    string `json:"sprint_id"`
		EpicID      string `json:"epic_id"`
		Labels      string `json:"labels"`
		StoryPoints int    `json:"story_points"`
		DependsOn   string `json:"depends_on"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Auto-increment issue number
	h.DB.Model(&project).Update("issue_seq", gorm.Expr("issue_seq + 1"))
	h.DB.First(&project, "id = ?", projectID) // reload
	num := project.IssueSeq

	issue := model.ForgeIssue{
		ProjectID:   projectID,
		Number:      num,
		Key:         fmt.Sprintf("%s-%d", project.Key, num),
		Title:       req.Title,
		Body:        req.Body,
		Type:        orDefault(req.Type, "task"),
		Priority:    orDefault(req.Priority, "medium"),
		Status:      "backlog",
		Assignee:    req.Assignee,
		Reporter:    "forge",
		Service:     req.Service,
		TaskType:    orDefault(req.TaskType, "code"),
		SprintID:    req.SprintID,
		EpicID:      req.EpicID,
		Labels:      req.Labels,
		StoryPoints: req.StoryPoints,
		DependsOn:   req.DependsOn,
	}

	h.DB.Create(&issue)

	// Activity log
	h.DB.Create(&model.ForgeActivity{
		ProjectID: projectID,
		IssueID:   issue.ID,
		Type:      "issue",
		Actor:     "forge",
		Summary:   fmt.Sprintf("Created %s: %s", issue.Key, issue.Title),
		Source:    "forge",
	})

	c.JSON(http.StatusCreated, gin.H{"issue": issue})
}

func (h *IssueHandler) Update(c *gin.Context) {
	var issue model.ForgeIssue
	if err := h.DB.First(&issue, "id = ?", c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "issue not found"})
		return
	}
	var req map[string]interface{}
	c.ShouldBindJSON(&req)
	h.DB.Model(&issue).Updates(req)
	h.DB.First(&issue, "id = ?", issue.ID) // reload
	c.JSON(http.StatusOK, gin.H{"issue": issue})
}

func (h *IssueHandler) Transition(c *gin.Context) {
	var issue model.ForgeIssue
	if err := h.DB.First(&issue, "id = ?", c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "issue not found"})
		return
	}
	var req struct {
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	oldStatus := issue.Status
	updates := map[string]interface{}{"status": req.Status}
	if req.Status == "done" || req.Status == "closed" {
		now := time.Now()
		updates["closed_at"] = &now
	}
	h.DB.Model(&issue).Updates(updates)

	h.DB.Create(&model.ForgeActivity{
		ProjectID: issue.ProjectID,
		IssueID:   issue.ID,
		Type:      "issue",
		Actor:     "forge",
		Summary:   fmt.Sprintf("%s: %s → %s", issue.Key, oldStatus, req.Status),
		Source:    "forge",
	})

	h.DB.First(&issue, "id = ?", issue.ID)
	c.JSON(http.StatusOK, gin.H{"issue": issue})
}

func (h *IssueHandler) AddComment(c *gin.Context) {
	var req struct {
		Author string `json:"author"`
		Body   string `json:"body" binding:"required"`
		IsAI   bool   `json:"is_ai"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	comment := model.ForgeIssueComment{
		IssueID: c.Param("id"),
		Author:  orDefault(req.Author, "anonymous"),
		Body:    req.Body,
		IsAI:    req.IsAI,
	}
	h.DB.Create(&comment)
	c.JSON(http.StatusCreated, gin.H{"comment": comment})
}

func (h *IssueHandler) Board(c *gin.Context) {
	projectID := c.Param("id")
	columns := []string{"backlog", "todo", "in_progress", "review", "done"}
	board := make(map[string][]model.ForgeIssue)
	for _, col := range columns {
		var issues []model.ForgeIssue
		q := h.DB.Where("project_id = ? AND status = ?", projectID, col).Order("position ASC")
		if s := c.Query("sprint_id"); s != "" {
			q = q.Where("sprint_id = ?", s)
		}
		q.Find(&issues)
		board[col] = issues
	}
	c.JSON(http.StatusOK, gin.H{"board": board, "columns": columns})
}

func orDefault(val, def string) string {
	if val == "" {
		return def
	}
	return val
}
