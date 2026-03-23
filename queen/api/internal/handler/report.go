package handler

import (
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"starclaw.net/queen/api/internal/database"
	"starclaw.net/queen/api/internal/middleware"
	"starclaw.net/queen/api/internal/model"
)

type ReportHandler struct{}

// ============================================================
// User API (authenticated)
// ============================================================

// POST /reports — submit a content report
func (h *ReportHandler) Create(c *gin.Context) {
	userID := c.GetString("user_id")
	var req struct {
		TargetType  string `json:"target_type" binding:"required"`
		TargetID    string `json:"target_id" binding:"required"`
		TargetTitle string `json:"target_title"`
		AuthorID    string `json:"author_id"`
		Reason      string `json:"reason" binding:"required"`
		Detail      string `json:"detail"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, http.StatusBadRequest, middleware.CodeBadRequest, "请填写举报类型和原因")
		return
	}

	if !slices.Contains(model.ReportTargetTypes, req.TargetType) {
		middleware.Fail(c, http.StatusBadRequest, middleware.CodeBadRequest, "不支持的举报目标类型")
		return
	}
	if !slices.Contains(model.ReportReasons, req.Reason) {
		middleware.Fail(c, http.StatusBadRequest, middleware.CodeBadRequest, "不支持的举报原因")
		return
	}

	// Prevent duplicate reports from same user on same target
	var existing model.ContentReport
	if err := database.DB.Where("reporter_id = ? AND target_type = ? AND target_id = ? AND status = ?",
		userID, req.TargetType, req.TargetID, "pending").First(&existing).Error; err == nil {
		middleware.Fail(c, http.StatusConflict, middleware.CodeConflict, "你已举报过此内容，请等待审核")
		return
	}

	report := model.ContentReport{
		ID:          uuid.New().String(),
		ReporterID:  userID,
		TargetType:  req.TargetType,
		TargetID:    req.TargetID,
		TargetTitle: req.TargetTitle,
		AuthorID:    req.AuthorID,
		Reason:      req.Reason,
		Detail:      req.Detail,
		Status:      "pending",
	}
	if err := database.DB.Create(&report).Error; err != nil {
		middleware.Fail(c, http.StatusInternalServerError, middleware.CodeInternal, "提交举报失败")
		return
	}

	middleware.OK(c, gin.H{"report_id": report.ID, "message": "举报已提交，我们会尽快处理"})
}

// GET /reports/mine — list my submitted reports
func (h *ReportHandler) MyReports(c *gin.Context) {
	userID := c.GetString("user_id")
	var reports []model.ContentReport
	database.DB.Where("reporter_id = ?", userID).Order("created_at DESC").Limit(50).Find(&reports)
	middleware.OK(c, gin.H{"reports": reports, "total": len(reports)})
}

// GET /reports/reasons — list valid report reasons
func (h *ReportHandler) Reasons(c *gin.Context) {
	reasons := []gin.H{
		{"id": "spam", "label": "垃圾信息"},
		{"id": "abuse", "label": "辱骂/人身攻击"},
		{"id": "nsfw", "label": "不当内容"},
		{"id": "illegal", "label": "违法违规"},
		{"id": "other", "label": "其他"},
	}
	middleware.OK(c, gin.H{"reasons": reasons})
}

// ============================================================
// Admin API
// ============================================================

// GET /admin/reports — list all reports (paginated, filterable)
func (h *ReportHandler) AdminList(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	status := c.Query("status")
	targetType := c.Query("target_type")
	if page < 1 {
		page = 1
	}

	query := database.DB.Model(&model.ContentReport{})
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if targetType != "" {
		query = query.Where("target_type = ?", targetType)
	}

	var total int64
	query.Count(&total)

	var reports []model.ContentReport
	query.Order("created_at DESC").Offset((page - 1) * size).Limit(size).Find(&reports)

	middleware.OK(c, gin.H{
		"reports": reports,
		"total":   total,
		"page":    page,
		"size":    size,
	})
}

// GET /admin/reports/stats — report statistics
func (h *ReportHandler) AdminStats(c *gin.Context) {
	db := database.DB
	var total, pending, reviewed, resolved, dismissed int64
	db.Model(&model.ContentReport{}).Count(&total)
	db.Model(&model.ContentReport{}).Where("status = ?", "pending").Count(&pending)
	db.Model(&model.ContentReport{}).Where("status = ?", "reviewed").Count(&reviewed)
	db.Model(&model.ContentReport{}).Where("status = ?", "resolved").Count(&resolved)
	db.Model(&model.ContentReport{}).Where("status = ?", "dismissed").Count(&dismissed)

	middleware.OK(c, gin.H{
		"total":     total,
		"pending":   pending,
		"reviewed":  reviewed,
		"resolved":  resolved,
		"dismissed": dismissed,
	})
}

// PUT /admin/reports/:id — review/resolve a report
func (h *ReportHandler) AdminReview(c *gin.Context) {
	id := c.Param("id")
	reviewerID := c.GetString("user_id")

	var req struct {
		Status     string `json:"status" binding:"required"`   // reviewed / resolved / dismissed
		Resolution string `json:"resolution"`                   // warn / hide / delete / ban / none
		ReviewNote string `json:"review_note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, http.StatusBadRequest, middleware.CodeBadRequest)
		return
	}

	validStatuses := []string{"reviewed", "resolved", "dismissed"}
	if !slices.Contains(validStatuses, req.Status) {
		middleware.Fail(c, http.StatusBadRequest, middleware.CodeBadRequest, "无效的审核状态")
		return
	}

	var report model.ContentReport
	if err := database.DB.First(&report, "id = ?", id).Error; err != nil {
		middleware.Fail(c, http.StatusNotFound, middleware.CodeNotFound, "举报记录不存在")
		return
	}

	now := time.Now()
	updates := map[string]interface{}{
		"status":      req.Status,
		"reviewer_id": reviewerID,
		"reviewed_at": &now,
	}
	if req.Resolution != "" {
		updates["resolution"] = req.Resolution
	}
	if req.ReviewNote != "" {
		updates["review_note"] = req.ReviewNote
	}

	database.DB.Model(&report).Updates(updates)
	middleware.OK(c, gin.H{"message": "审核完成", "report_id": id})
}

// POST /admin/reports/:id/action — execute moderation action on the content
func (h *ReportHandler) AdminAction(c *gin.Context) {
	id := c.Param("id")

	var report model.ContentReport
	if err := database.DB.First(&report, "id = ?", id).Error; err != nil {
		middleware.Fail(c, http.StatusNotFound, middleware.CodeNotFound)
		return
	}

	var req struct {
		Action string `json:"action" binding:"required"` // hide / delete / ban_author
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, http.StatusBadRequest, middleware.CodeBadRequest)
		return
	}

	// Execute action based on target type
	switch req.Action {
	case "hide", "delete":
		// Soft-delete content in the respective service table
		// For now, we mark the report as resolved with the action
		now := time.Now()
		database.DB.Model(&report).Updates(map[string]interface{}{
			"status":      "resolved",
			"resolution":  req.Action,
			"reviewer_id": c.GetString("user_id"),
			"reviewed_at": &now,
		})

	case "ban_author":
		// Ban the content author
		if report.AuthorID != "" {
			database.DB.Model(&model.User{}).Where("id = ?", report.AuthorID).
				Update("status", "banned")
		}
		now := time.Now()
		database.DB.Model(&report).Updates(map[string]interface{}{
			"status":      "resolved",
			"resolution":  "ban",
			"reviewer_id": c.GetString("user_id"),
			"reviewed_at": &now,
		})

	default:
		middleware.Fail(c, http.StatusBadRequest, middleware.CodeBadRequest, "不支持的操作")
		return
	}

	middleware.OK(c, gin.H{"message": "操作已执行", "action": req.Action})
}
