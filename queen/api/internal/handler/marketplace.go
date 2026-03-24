package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"starclaw.net/queen/api/internal/database"
	"starclaw.net/queen/api/internal/model"
)

type MarketplaceHandler struct{}

// GET /marketplace/items?type=agent&q=keyword&page=1&size=20
func (h *MarketplaceHandler) List(c *gin.Context) {
	typ := c.Query("type")
	q := c.Query("q")
	page := 1
	size := 20

	query := database.DB.Model(&model.MarketplaceItem{}).Where("status IN ?", []string{model.ItemStatusPublished, model.ItemStatusApproved})
	if typ != "" {
		query = query.Where("type = ?", typ)
	}
	if q != "" {
		query = query.Where("name LIKE ? OR description LIKE ? OR tags LIKE ?", "%"+q+"%", "%"+q+"%", "%"+q+"%")
	}

	var total int64
	query.Count(&total)

	var items []model.MarketplaceItem
	query.Preload("Author").Order("downloads DESC").Offset((page - 1) * size).Limit(size).Find(&items)

	c.JSON(http.StatusOK, gin.H{
		"items": items,
		"total": total,
		"page":  page,
		"size":  size,
	})
}

// GET /marketplace/items/:id
func (h *MarketplaceHandler) Get(c *gin.Context) {
	id := c.Param("id")
	var item model.MarketplaceItem
	if err := database.DB.Preload("Author").First(&item, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "未找到"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"item": item})
}

// POST /marketplace/items (auth required)
func (h *MarketplaceHandler) Create(c *gin.Context) {
	userID := c.GetString("user_id")
	var req struct {
		Type        string `json:"type" binding:"required"`
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
		Icon        string `json:"icon"`
		Tags        string `json:"tags"`
		Config      string `json:"config"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	now := time.Now()
	item := model.MarketplaceItem{
		ID:           uuid.New().String(),
		UserID:       userID,
		Type:         req.Type,
		Name:         req.Name,
		Description:  req.Description,
		Icon:         req.Icon,
		Tags:         req.Tags,
		Config:       req.Config,
		Status:       model.ItemStatusPendingReview,
		ReviewStatus: "pending",
		SubmittedAt:  &now,
	}
	if err := database.DB.Create(&item).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"item": item})
}

// PUT /marketplace/items/:id (auth required, owner only)
func (h *MarketplaceHandler) Update(c *gin.Context) {
	userID := c.GetString("user_id")
	id := c.Param("id")

	var item model.MarketplaceItem
	if err := database.DB.First(&item, "id = ? AND user_id = ?", id, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "未找到或无权限"})
		return
	}

	var req map[string]interface{}
	c.ShouldBindJSON(&req)
	delete(req, "id")
	delete(req, "user_id")
	delete(req, "downloads")
	delete(req, "rating")

	database.DB.Model(&item).Updates(req)
	c.JSON(http.StatusOK, gin.H{"message": "更新成功"})
}

// DELETE /marketplace/items/:id (auth required, owner only)
func (h *MarketplaceHandler) Delete(c *gin.Context) {
	userID := c.GetString("user_id")
	id := c.Param("id")
	result := database.DB.Where("id = ? AND user_id = ?", id, userID).Delete(&model.MarketplaceItem{})
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "未找到或无权限"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

// GET /marketplace/my?type=agent (auth required)
func (h *MarketplaceHandler) My(c *gin.Context) {
	userID := c.GetString("user_id")
	typ := c.Query("type")

	query := database.DB.Where("user_id = ?", userID)
	if typ != "" {
		query = query.Where("type = ?", typ)
	}

	var items []model.MarketplaceItem
	query.Order("created_at DESC").Find(&items)

	c.JSON(http.StatusOK, gin.H{"items": items})
}

// GET /marketplace/stats (public)
func (h *MarketplaceHandler) Stats(c *gin.Context) {
	visible := []string{model.ItemStatusPublished, model.ItemStatusApproved}
	var agentCount, skillCount, workflowCount, mcpCount int64
	database.DB.Model(&model.MarketplaceItem{}).Where("type = ? AND status IN ?", "agent", visible).Count(&agentCount)
	database.DB.Model(&model.MarketplaceItem{}).Where("type = ? AND status IN ?", "skill", visible).Count(&skillCount)
	database.DB.Model(&model.MarketplaceItem{}).Where("type = ? AND status IN ?", "workflow", visible).Count(&workflowCount)
	database.DB.Model(&model.MarketplaceItem{}).Where("type = ? AND status IN ?", "mcp", visible).Count(&mcpCount)

	c.JSON(http.StatusOK, gin.H{
		"agents":    agentCount,
		"skills":    skillCount,
		"workflows": workflowCount,
		"mcp":       mcpCount,
		"total":     agentCount + skillCount + workflowCount + mcpCount,
	})
}

// ---- Developer Center: Submit for Review ----

// POST /marketplace/items/:id/submit — submit a draft or rejected item for review
func (h *MarketplaceHandler) Submit(c *gin.Context) {
	userID := c.GetString("user_id")
	id := c.Param("id")

	var item model.MarketplaceItem
	if err := database.DB.First(&item, "id = ? AND user_id = ?", id, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "未找到或无权限"})
		return
	}

	// Only draft or rejected items can be (re-)submitted
	if item.Status != model.ItemStatusDraft && item.Status != model.ItemStatusRejected {
		c.JSON(http.StatusConflict, gin.H{"error": "当前状态不允许提交审核"})
		return
	}

	now := time.Now()
	database.DB.Model(&item).Updates(map[string]interface{}{
		"status":        model.ItemStatusPendingReview,
		"review_status": "pending",
		"review_note":   "",
		"submitted_at":  now,
	})
	c.JSON(http.StatusOK, gin.H{"message": "已提交审核"})
}

// ---- Admin: Review Management ----

// GET /admin/marketplace/pending?type=agent&page=1&size=20
func (h *MarketplaceHandler) AdminListPending(c *gin.Context) {
	status := c.DefaultQuery("status", "pending_review")
	typ := c.Query("type")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	if page < 1 {
		page = 1
	}

	query := database.DB.Model(&model.MarketplaceItem{}).Where("status = ?", status)
	if typ != "" {
		query = query.Where("type = ?", typ)
	}

	var total int64
	query.Count(&total)

	var items []model.MarketplaceItem
	query.Preload("Author").Order("submitted_at DESC").Offset((page - 1) * size).Limit(size).Find(&items)

	c.JSON(http.StatusOK, gin.H{
		"items": items,
		"total": total,
		"page":  page,
		"size":  size,
	})
}

// GET /admin/marketplace/stats — review queue statistics
func (h *MarketplaceHandler) AdminReviewStats(c *gin.Context) {
	var pending, approved, rejected, total int64
	database.DB.Model(&model.MarketplaceItem{}).Where("status = ?", model.ItemStatusPendingReview).Count(&pending)
	database.DB.Model(&model.MarketplaceItem{}).Where("status = ?", model.ItemStatusApproved).Count(&approved)
	database.DB.Model(&model.MarketplaceItem{}).Where("status = ?", model.ItemStatusRejected).Count(&rejected)
	database.DB.Model(&model.MarketplaceItem{}).Count(&total)

	c.JSON(http.StatusOK, gin.H{
		"pending":  pending,
		"approved": approved,
		"rejected": rejected,
		"total":    total,
	})
}

// PUT /admin/marketplace/items/:id/approve
func (h *MarketplaceHandler) AdminApprove(c *gin.Context) {
	reviewerID := c.GetString("user_id")
	id := c.Param("id")

	var item model.MarketplaceItem
	if err := database.DB.First(&item, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "未找到"})
		return
	}

	if item.Status != model.ItemStatusPendingReview {
		c.JSON(http.StatusConflict, gin.H{"error": "当前状态不允许审批"})
		return
	}

	var req struct {
		Note string `json:"note"`
	}
	c.ShouldBindJSON(&req)

	now := time.Now()
	database.DB.Model(&item).Updates(map[string]interface{}{
		"status":        model.ItemStatusApproved,
		"review_status": "approved",
		"reviewer_id":   reviewerID,
		"review_note":   req.Note,
		"reviewed_at":   now,
	})
	c.JSON(http.StatusOK, gin.H{"message": "已通过审核"})
}

// PUT /admin/marketplace/items/:id/reject
func (h *MarketplaceHandler) AdminReject(c *gin.Context) {
	reviewerID := c.GetString("user_id")
	id := c.Param("id")

	var req struct {
		Note string `json:"note" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请填写拒绝原因"})
		return
	}

	var item model.MarketplaceItem
	if err := database.DB.First(&item, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "未找到"})
		return
	}

	if item.Status != model.ItemStatusPendingReview {
		c.JSON(http.StatusConflict, gin.H{"error": "当前状态不允许拒绝"})
		return
	}

	now := time.Now()
	database.DB.Model(&item).Updates(map[string]interface{}{
		"status":        model.ItemStatusRejected,
		"review_status": "rejected",
		"reviewer_id":   reviewerID,
		"review_note":   req.Note,
		"reviewed_at":   now,
	})
	c.JSON(http.StatusOK, gin.H{"message": "已拒绝"})
}

// PUT /admin/marketplace/items/:id/remove — admin take-down
func (h *MarketplaceHandler) AdminRemove(c *gin.Context) {
	reviewerID := c.GetString("user_id")
	id := c.Param("id")

	var req struct {
		Note string `json:"note" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请填写下架原因"})
		return
	}

	var item model.MarketplaceItem
	if err := database.DB.First(&item, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "未找到"})
		return
	}

	now := time.Now()
	database.DB.Model(&item).Updates(map[string]interface{}{
		"status":        model.ItemStatusRemoved,
		"review_status": "removed",
		"reviewer_id":   reviewerID,
		"review_note":   req.Note,
		"reviewed_at":   now,
	})
	c.JSON(http.StatusOK, gin.H{"message": "已下架"})
}

// GET /marketplace/items/:id/install-spec
// Returns the install specification for a marketplace item (skill/agent).
// Used by Overlord to install skills on Claw agents.
func (h *MarketplaceHandler) InstallSpec(c *gin.Context) {
	id := c.Param("id")
	var item model.MarketplaceItem
	if err := database.DB.First(&item, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "item not found"})
		return
	}

	if item.Status != model.ItemStatusPublished && item.Status != model.ItemStatusApproved {
		c.JSON(http.StatusForbidden, gin.H{"error": "item not available for install"})
		return
	}

	// Increment download count
	database.DB.Model(&item).Update("downloads", item.Downloads+1)

	c.JSON(http.StatusOK, gin.H{
		"id":      item.ID,
		"name":    item.Name,
		"type":    item.Type,
		"version": item.Version,
		"config":  item.Config, // JSON spec (function definition for skills, agent config for agents)
		"tags":    item.Tags,
		"icon":    item.Icon,
	})
}

// GET /marketplace/skills/search?q=code&name=web_search
// Searches skills by name or keyword. Used by Overlord to find skills for agent provisioning.
func (h *MarketplaceHandler) SearchSkills(c *gin.Context) {
	q := c.Query("q")
	name := c.Query("name")

	query := database.DB.Model(&model.MarketplaceItem{}).
		Where("type = ? AND status IN ?", "skill", []string{model.ItemStatusPublished, model.ItemStatusApproved})

	if name != "" {
		query = query.Where("name = ?", name)
	} else if q != "" {
		query = query.Where("name LIKE ? OR description LIKE ? OR tags LIKE ?", "%"+q+"%", "%"+q+"%", "%"+q+"%")
	}

	var items []model.MarketplaceItem
	query.Order("downloads DESC").Limit(50).Find(&items)

	type SkillInfo struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		Version string `json:"version"`
		Config  string `json:"config"`
		Icon    string `json:"icon"`
	}
	skills := make([]SkillInfo, 0, len(items))
	for _, item := range items {
		skills = append(skills, SkillInfo{
			ID:      item.ID,
			Name:    item.Name,
			Version: item.Version,
			Config:  item.Config,
			Icon:    item.Icon,
		})
	}

	c.JSON(http.StatusOK, gin.H{"skills": skills, "total": len(skills)})
}
