package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"starclaw.net/forge/internal/model"
)

type SprintHandler struct{ DB *gorm.DB }

func (h *SprintHandler) List(c *gin.Context) {
	var sprints []model.ForgeSprint
	h.DB.Where("project_id = ?", c.Param("id")).Order("seq_num ASC").Find(&sprints)

	// Attach issue stats per sprint
	type sprintInfo struct {
		model.ForgeSprint
		TotalIssues int `json:"total_issues"`
		DoneIssues  int `json:"done_issues"`
		TotalPoints int `json:"total_points"`
		DonePoints  int `json:"done_points"`
	}
	var result []sprintInfo
	for _, s := range sprints {
		var total, done int64
		h.DB.Model(&model.ForgeIssue{}).Where("sprint_id = ?", s.ID).Count(&total)
		h.DB.Model(&model.ForgeIssue{}).Where("sprint_id = ? AND status IN ('done','closed')", s.ID).Count(&done)

		var totalPts, donePts struct{ Sum int }
		h.DB.Model(&model.ForgeIssue{}).Where("sprint_id = ?", s.ID).Select("COALESCE(SUM(story_points),0) as sum").Scan(&totalPts)
		h.DB.Model(&model.ForgeIssue{}).Where("sprint_id = ? AND status IN ('done','closed')", s.ID).Select("COALESCE(SUM(story_points),0) as sum").Scan(&donePts)

		result = append(result, sprintInfo{
			ForgeSprint: s,
			TotalIssues: int(total),
			DoneIssues:  int(done),
			TotalPoints: totalPts.Sum,
			DonePoints:  donePts.Sum,
		})
	}
	c.JSON(http.StatusOK, gin.H{"sprints": result})
}

func (h *SprintHandler) Create(c *gin.Context) {
	projectID := c.Param("id")
	var req struct {
		Name  string `json:"name" binding:"required"`
		Goal  string `json:"goal"`
		PRDID string `json:"prd_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Auto seq_num
	var maxSeq struct{ Max int }
	h.DB.Model(&model.ForgeSprint{}).Where("project_id = ?", projectID).Select("COALESCE(MAX(seq_num),0) as max").Scan(&maxSeq)

	sprint := model.ForgeSprint{
		ProjectID: projectID,
		PRDID:     req.PRDID,
		Name:      req.Name,
		Goal:      req.Goal,
		Status:    "planned",
		SeqNum:    maxSeq.Max + 1,
	}
	h.DB.Create(&sprint)
	c.JSON(http.StatusCreated, gin.H{"sprint": sprint})
}

func (h *SprintHandler) Update(c *gin.Context) {
	var sprint model.ForgeSprint
	if err := h.DB.First(&sprint, "id = ?", c.Param("sid")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "sprint not found"})
		return
	}
	var req map[string]interface{}
	c.ShouldBindJSON(&req)
	h.DB.Model(&sprint).Updates(req)
	h.DB.First(&sprint, "id = ?", sprint.ID)
	c.JSON(http.StatusOK, gin.H{"sprint": sprint})
}

func (h *SprintHandler) Burndown(c *gin.Context) {
	sprintID := c.Param("sid")
	var sprint model.ForgeSprint
	if err := h.DB.First(&sprint, "id = ?", sprintID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "sprint not found"})
		return
	}

	// Get all issues in this sprint
	var issues []model.ForgeIssue
	h.DB.Where("sprint_id = ?", sprintID).Find(&issues)

	totalPoints := 0
	donePoints := 0
	for _, i := range issues {
		totalPoints += i.StoryPoints
		if i.Status == "done" || i.Status == "closed" {
			donePoints += i.StoryPoints
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"sprint":       sprint,
		"total_points": totalPoints,
		"done_points":  donePoints,
		"remaining":    totalPoints - donePoints,
		"total_issues": len(issues),
	})
}
