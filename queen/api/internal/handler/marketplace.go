package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/yinhe/starclaw-queen/api/internal/database"
	"github.com/yinhe/starclaw-queen/api/internal/model"
)

type MarketplaceHandler struct{}

// GET /marketplace/items?type=agent&q=keyword&page=1&size=20
func (h *MarketplaceHandler) List(c *gin.Context) {
	typ := c.Query("type")
	q := c.Query("q")
	page := 1
	size := 20

	query := database.DB.Model(&model.MarketplaceItem{}).Where("status = ?", "published")
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

	item := model.MarketplaceItem{
		ID:          uuid.New().String(),
		UserID:      userID,
		Type:        req.Type,
		Name:        req.Name,
		Description: req.Description,
		Icon:        req.Icon,
		Tags:        req.Tags,
		Config:      req.Config,
		Status:      "published",
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
	var agentCount, skillCount, workflowCount, mcpCount int64
	database.DB.Model(&model.MarketplaceItem{}).Where("type = ? AND status = ?", "agent", "published").Count(&agentCount)
	database.DB.Model(&model.MarketplaceItem{}).Where("type = ? AND status = ?", "skill", "published").Count(&skillCount)
	database.DB.Model(&model.MarketplaceItem{}).Where("type = ? AND status = ?", "workflow", "published").Count(&workflowCount)
	database.DB.Model(&model.MarketplaceItem{}).Where("type = ? AND status = ?", "mcp", "published").Count(&mcpCount)

	c.JSON(http.StatusOK, gin.H{
		"agents":    agentCount,
		"skills":    skillCount,
		"workflows": workflowCount,
		"mcp":       mcpCount,
		"total":     agentCount + skillCount + workflowCount + mcpCount,
	})
}
