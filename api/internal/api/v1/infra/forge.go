package infra

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw/internal/model"
	"gorm.io/gorm"
)

type ForgeHandler struct {
	db *gorm.DB
}

func NewForgeHandler(db *gorm.DB) *ForgeHandler {
	return &ForgeHandler{db: db}
}

// ════════════════════════════════════════════════════════════
// Projects
// ════════════════════════════════════════════════════════════

func (h *ForgeHandler) CreateProject(c *gin.Context) {
	userID := c.GetString("user_id")
	var req struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
		SquadID     string `json:"squad_id"`
		NydusRepo   string `json:"nydus_repo"`
		Visibility  string `json:"visibility"`
		Tags        string `json:"tags"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Visibility == "" {
		req.Visibility = "team"
	}

	project := model.ForgeProject{
		Name:        req.Name,
		Description: req.Description,
		SquadID:     req.SquadID,
		UserID:      userID,
		NydusRepo:   req.NydusRepo,
		Visibility:  req.Visibility,
		Tags:        req.Tags,
	}
	if err := h.db.Create(&project).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create project"})
		return
	}

	// Create default board
	defaultColumns, _ := json.Marshal([]gin.H{
		{"name": "Backlog", "status": "open"},
		{"name": "In Progress", "status": "in_progress"},
		{"name": "Review", "status": "review"},
		{"name": "Done", "status": "done"},
	})
	h.db.Create(&model.ForgeBoard{
		ProjectID: project.ID,
		Name:      "Default Board",
		Columns:   string(defaultColumns),
		IsDefault: true,
	})

	c.JSON(http.StatusCreated, gin.H{"project": project})
}

func (h *ForgeHandler) ListProjects(c *gin.Context) {
	userID := c.GetString("user_id")
	query := h.db.Where("user_id = ? AND status = ?", userID, "active")
	if squadID := c.Query("squad_id"); squadID != "" {
		query = query.Where("squad_id = ?", squadID)
	}
	var projects []model.ForgeProject
	query.Order("updated_at DESC").Find(&projects)

	// Attach issue counts
	type countRow struct {
		ProjectID string
		Total     int64
		Open      int64
		Done      int64
	}
	var counts []countRow
	h.db.Model(&model.ForgeIssue{}).
		Select("project_id, COUNT(*) as total, SUM(CASE WHEN status='open' OR status='in_progress' THEN 1 ELSE 0 END) as open, SUM(CASE WHEN status='done' OR status='closed' THEN 1 ELSE 0 END) as done").
		Group("project_id").Find(&counts)
	countMap := map[string]countRow{}
	for _, cr := range counts {
		countMap[cr.ProjectID] = cr
	}

	result := make([]gin.H, 0, len(projects))
	for _, p := range projects {
		cr := countMap[p.ID]
		result = append(result, gin.H{
			"project":      p,
			"total_issues": cr.Total,
			"open_issues":  cr.Open,
			"done_issues":  cr.Done,
		})
	}
	c.JSON(http.StatusOK, gin.H{"projects": result, "total": len(result)})
}

func (h *ForgeHandler) GetProject(c *gin.Context) {
	id := c.Param("id")
	var project model.ForgeProject
	if err := h.db.Where("id = ?", id).First(&project).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
		return
	}

	var milestones []model.ForgeMilestone
	h.db.Where("project_id = ?", id).Order("created_at DESC").Find(&milestones)

	var boards []model.ForgeBoard
	h.db.Where("project_id = ?", id).Find(&boards)

	c.JSON(http.StatusOK, gin.H{"project": project, "milestones": milestones, "boards": boards})
}

func (h *ForgeHandler) UpdateProject(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
		NydusRepo   *string `json:"nydus_repo"`
		Visibility  *string `json:"visibility"`
		Tags        *string `json:"tags"`
		Status      *string `json:"status"`
	}
	c.ShouldBindJSON(&req)
	updates := map[string]interface{}{}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.NydusRepo != nil {
		updates["nydus_repo"] = *req.NydusRepo
	}
	if req.Visibility != nil {
		updates["visibility"] = *req.Visibility
	}
	if req.Tags != nil {
		updates["tags"] = *req.Tags
	}
	if req.Status != nil {
		updates["status"] = *req.Status
	}

	if len(updates) > 0 {
		h.db.Model(&model.ForgeProject{}).Where("id = ?", id).Updates(updates)
	}
	var project model.ForgeProject
	h.db.Where("id = ?", id).First(&project)
	c.JSON(http.StatusOK, gin.H{"project": project})
}

// ════════════════════════════════════════════════════════════
// Issues
// ════════════════════════════════════════════════════════════

func (h *ForgeHandler) CreateIssue(c *gin.Context) {
	projectID := c.Param("id")
	var req struct {
		Title        string `json:"title" binding:"required"`
		Body         string `json:"body"`
		Type         string `json:"type"`
		Priority     string `json:"priority"`
		AssigneeNode string `json:"assignee_node"`
		MilestoneID  string `json:"milestone_id"`
		Labels       string `json:"labels"`
		StoryPoints  int    `json:"story_points"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Auto-increment issue number
	var maxNum int
	h.db.Model(&model.ForgeIssue{}).Where("project_id = ?", projectID).
		Select("COALESCE(MAX(number), 0)").Scan(&maxNum)

	nodeID := c.GetString("node_id")
	if nodeID == "" {
		nodeID = "user:" + c.GetString("user_id")
	}

	issue := model.ForgeIssue{
		ProjectID:    projectID,
		Number:       maxNum + 1,
		Title:        req.Title,
		Body:         req.Body,
		Type:         orDefault(req.Type, "task"),
		Priority:     orDefault(req.Priority, "medium"),
		Status:       "open",
		AssigneeNode: req.AssigneeNode,
		ReporterNode: nodeID,
		MilestoneID:  req.MilestoneID,
		Labels:       req.Labels,
		StoryPoints:  req.StoryPoints,
	}
	if err := h.db.Create(&issue).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create issue"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"issue": issue})
}

func (h *ForgeHandler) ListIssues(c *gin.Context) {
	projectID := c.Param("id")
	query := h.db.Where("project_id = ?", projectID)

	if status := c.Query("status"); status != "" && status != "all" {
		query = query.Where("status = ?", status)
	}
	if assignee := c.Query("assignee"); assignee != "" {
		query = query.Where("assignee_node = ?", assignee)
	}
	if issueType := c.Query("type"); issueType != "" {
		query = query.Where("type = ?", issueType)
	}
	if priority := c.Query("priority"); priority != "" {
		query = query.Where("priority = ?", priority)
	}
	if milestoneID := c.Query("milestone_id"); milestoneID != "" {
		query = query.Where("milestone_id = ?", milestoneID)
	}

	var issues []model.ForgeIssue
	query.Order("number DESC").Find(&issues)
	c.JSON(http.StatusOK, gin.H{"issues": issues, "total": len(issues)})
}

func (h *ForgeHandler) GetIssue(c *gin.Context) {
	projectID := c.Param("id")
	number, _ := strconv.Atoi(c.Param("number"))

	var issue model.ForgeIssue
	if err := h.db.Where("project_id = ? AND number = ?", projectID, number).First(&issue).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "issue not found"})
		return
	}

	var comments []model.ForgeIssueComment
	h.db.Where("issue_id = ?", issue.ID).Order("created_at ASC").Find(&comments)

	c.JSON(http.StatusOK, gin.H{"issue": issue, "comments": comments})
}

func (h *ForgeHandler) UpdateIssue(c *gin.Context) {
	projectID := c.Param("id")
	number, _ := strconv.Atoi(c.Param("number"))

	var issue model.ForgeIssue
	if err := h.db.Where("project_id = ? AND number = ?", projectID, number).First(&issue).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "issue not found"})
		return
	}

	var req struct {
		Title        *string `json:"title"`
		Body         *string `json:"body"`
		Type         *string `json:"type"`
		Priority     *string `json:"priority"`
		Status       *string `json:"status"`
		AssigneeNode *string `json:"assignee_node"`
		MilestoneID  *string `json:"milestone_id"`
		SprintID     *string `json:"sprint_id"`
		MissionID    *string `json:"mission_id"`
		PRNumber     *int    `json:"pr_number"`
		Branch       *string `json:"branch"`
		Labels       *string `json:"labels"`
		StoryPoints  *int    `json:"story_points"`
	}
	c.ShouldBindJSON(&req)

	updates := map[string]interface{}{}
	if req.Title != nil {
		updates["title"] = *req.Title
	}
	if req.Body != nil {
		updates["body"] = *req.Body
	}
	if req.Type != nil {
		updates["type"] = *req.Type
	}
	if req.Priority != nil {
		updates["priority"] = *req.Priority
	}
	if req.Status != nil {
		updates["status"] = *req.Status
		if *req.Status == "done" || *req.Status == "closed" {
			now := time.Now()
			updates["closed_at"] = &now
		}
	}
	if req.AssigneeNode != nil {
		updates["assignee_node"] = *req.AssigneeNode
	}
	if req.MilestoneID != nil {
		updates["milestone_id"] = *req.MilestoneID
	}
	if req.SprintID != nil {
		updates["sprint_id"] = *req.SprintID
	}
	if req.MissionID != nil {
		updates["mission_id"] = *req.MissionID
	}
	if req.PRNumber != nil {
		updates["pr_number"] = *req.PRNumber
	}
	if req.Branch != nil {
		updates["branch"] = *req.Branch
	}
	if req.Labels != nil {
		updates["labels"] = *req.Labels
	}
	if req.StoryPoints != nil {
		updates["story_points"] = *req.StoryPoints
	}

	if len(updates) > 0 {
		h.db.Model(&issue).Updates(updates)
	}
	h.db.Where("project_id = ? AND number = ?", projectID, number).First(&issue)
	c.JSON(http.StatusOK, gin.H{"issue": issue})
}

func (h *ForgeHandler) AddIssueComment(c *gin.Context) {
	projectID := c.Param("id")
	number, _ := strconv.Atoi(c.Param("number"))

	var issue model.ForgeIssue
	if err := h.db.Where("project_id = ? AND number = ?", projectID, number).First(&issue).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "issue not found"})
		return
	}

	var req struct {
		Body string `json:"body" binding:"required"`
		IsAI bool   `json:"is_ai"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	nodeID := c.GetString("node_id")
	if nodeID == "" {
		nodeID = "user:" + c.GetString("user_id")
	}

	comment := model.ForgeIssueComment{
		IssueID:    issue.ID,
		AuthorNode: nodeID,
		Body:       req.Body,
		IsAI:       req.IsAI,
	}
	h.db.Create(&comment)
	c.JSON(http.StatusCreated, gin.H{"comment": comment})
}

// ════════════════════════════════════════════════════════════
// Milestones
// ════════════════════════════════════════════════════════════

func (h *ForgeHandler) CreateMilestone(c *gin.Context) {
	projectID := c.Param("id")
	var req struct {
		Title       string `json:"title" binding:"required"`
		Description string `json:"description"`
		DueDate     string `json:"due_date"` // YYYY-MM-DD
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ms := model.ForgeMilestone{
		ProjectID:   projectID,
		Title:       req.Title,
		Description: req.Description,
	}
	if req.DueDate != "" {
		if t, err := time.Parse("2006-01-02", req.DueDate); err == nil {
			ms.DueDate = &t
		}
	}
	h.db.Create(&ms)
	c.JSON(http.StatusCreated, gin.H{"milestone": ms})
}

func (h *ForgeHandler) ListMilestones(c *gin.Context) {
	projectID := c.Param("id")
	var milestones []model.ForgeMilestone
	h.db.Where("project_id = ?", projectID).Order("created_at DESC").Find(&milestones)

	// Attach issue progress per milestone
	result := make([]gin.H, 0, len(milestones))
	for _, ms := range milestones {
		var total, closed int64
		h.db.Model(&model.ForgeIssue{}).Where("milestone_id = ?", ms.ID).Count(&total)
		h.db.Model(&model.ForgeIssue{}).Where("milestone_id = ? AND (status = 'done' OR status = 'closed')", ms.ID).Count(&closed)
		result = append(result, gin.H{
			"milestone":     ms,
			"total_issues":  total,
			"closed_issues": closed,
			"progress":      safePercent(closed, total),
		})
	}
	c.JSON(http.StatusOK, gin.H{"milestones": result})
}

func (h *ForgeHandler) CloseMilestone(c *gin.Context) {
	msID := c.Param("ms_id")
	now := time.Now()
	h.db.Model(&model.ForgeMilestone{}).Where("id = ?", msID).Updates(map[string]interface{}{
		"status": "closed", "closed_at": &now,
	})
	c.JSON(http.StatusOK, gin.H{"message": "milestone closed"})
}

// ════════════════════════════════════════════════════════════
// Boards (Kanban)
// ════════════════════════════════════════════════════════════

func (h *ForgeHandler) GetBoard(c *gin.Context) {
	projectID := c.Param("id")

	// Get default board or specific board
	boardID := c.Query("board_id")
	var board model.ForgeBoard
	if boardID != "" {
		h.db.Where("id = ?", boardID).First(&board)
	} else {
		h.db.Where("project_id = ? AND is_default = ?", projectID, true).First(&board)
	}
	if board.ID == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "board not found"})
		return
	}

	// Parse columns
	var columns []gin.H
	json.Unmarshal([]byte(board.Columns), &columns)

	// Fetch all issues and group by status
	var issues []model.ForgeIssue
	h.db.Where("project_id = ?", projectID).Order("priority DESC, number ASC").Find(&issues)

	issuesByStatus := map[string][]model.ForgeIssue{}
	for _, issue := range issues {
		issuesByStatus[issue.Status] = append(issuesByStatus[issue.Status], issue)
	}

	// Build board view
	boardView := make([]gin.H, 0, len(columns))
	for _, col := range columns {
		status, _ := col["status"].(string)
		boardView = append(boardView, gin.H{
			"column": col,
			"issues": issuesByStatus[status],
			"count":  len(issuesByStatus[status]),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"board":   board,
		"columns": boardView,
		"total":   len(issues),
	})
}

// ════════════════════════════════════════════════════════════
// Helpers
// ════════════════════════════════════════════════════════════

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func safePercent(a, b int64) string {
	if b == 0 {
		return "0%"
	}
	return fmt.Sprintf("%.0f%%", float64(a)/float64(b)*100)
}
