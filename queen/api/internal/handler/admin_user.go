package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"starclaw.net/queen/api/internal/database"
	"starclaw.net/queen/api/internal/middleware"
	"starclaw.net/queen/api/internal/model"
)

type AdminUserHandler struct{}

// GET /admin/users — list users with pagination and filters
func (h *AdminUserHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	search := c.Query("search")
	role := c.Query("role")
	status := c.Query("status")
	if page < 1 {
		page = 1
	}

	query := database.DB.Model(&model.User{})
	if search != "" {
		query = query.Where("email LIKE ? OR nickname LIKE ? OR phone LIKE ?",
			"%"+search+"%", "%"+search+"%", "%"+search+"%")
	}
	if role != "" {
		query = query.Where("role = ?", role)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}

	var total int64
	query.Count(&total)

	var users []model.User
	query.Order("created_at DESC").Offset((page - 1) * size).Limit(size).Find(&users)

	middleware.OK(c, gin.H{
		"users": users,
		"total": total,
		"page":  page,
		"size":  size,
	})
}

// GET /admin/users/:id — get user detail
func (h *AdminUserHandler) Get(c *gin.Context) {
	id := c.Param("id")
	var user model.User
	if err := database.DB.First(&user, "id = ?", id).Error; err != nil {
		middleware.Fail(c, http.StatusNotFound, middleware.CodeNotFound, "用户不存在")
		return
	}

	// Get user balance
	var balance model.UserBalance
	database.DB.FirstOrCreate(&balance, model.UserBalance{UserID: id})

	middleware.OK(c, gin.H{"user": user, "balance": balance})
}

// PUT /admin/users/:id/role — update user role
func (h *AdminUserHandler) UpdateRole(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Role string `json:"role" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, http.StatusBadRequest, middleware.CodeBadRequest, "请选择角色")
		return
	}
	if req.Role != "user" && req.Role != "developer" && req.Role != "admin" {
		middleware.Fail(c, http.StatusBadRequest, middleware.CodeBadRequest, "无效角色")
		return
	}

	result := database.DB.Model(&model.User{}).Where("id = ?", id).Update("role", req.Role)
	if result.RowsAffected == 0 {
		middleware.Fail(c, http.StatusNotFound, middleware.CodeNotFound, "用户不存在")
		return
	}
	middleware.OK(c, gin.H{"message": "角色已更新"})
}

// PUT /admin/users/:id/status — ban or activate user
func (h *AdminUserHandler) UpdateStatus(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, http.StatusBadRequest, middleware.CodeBadRequest)
		return
	}
	if req.Status != "active" && req.Status != "banned" {
		middleware.Fail(c, http.StatusBadRequest, middleware.CodeBadRequest, "无效状态")
		return
	}

	result := database.DB.Model(&model.User{}).Where("id = ?", id).Update("status", req.Status)
	if result.RowsAffected == 0 {
		middleware.Fail(c, http.StatusNotFound, middleware.CodeNotFound, "用户不存在")
		return
	}
	middleware.OK(c, gin.H{"message": "用户状态已更新"})
}

// GET /admin/users/stats — user statistics
func (h *AdminUserHandler) Stats(c *gin.Context) {
	db := database.DB
	var total, active, banned, admins, developers int64
	db.Model(&model.User{}).Count(&total)
	db.Model(&model.User{}).Where("status = ?", "active").Count(&active)
	db.Model(&model.User{}).Where("status = ?", "banned").Count(&banned)
	db.Model(&model.User{}).Where("role = ?", "admin").Count(&admins)
	db.Model(&model.User{}).Where("role = ?", "developer").Count(&developers)

	middleware.OK(c, gin.H{
		"total":      total,
		"active":     active,
		"banned":     banned,
		"admins":     admins,
		"developers": developers,
	})
}
