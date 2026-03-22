package handler

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw-router/internal/model"
	"github.com/yinhe/starclaw-router/internal/provider"
	"gorm.io/gorm"
)

type AdminHandler struct {
	db       *gorm.DB
	registry *provider.Registry
}

func NewAdminHandler(db *gorm.DB, reg *provider.Registry) *AdminHandler {
	return &AdminHandler{db: db, registry: reg}
}

// Overview returns platform-wide statistics
func (h *AdminHandler) Overview(c *gin.Context) {
	var userCount int64
	h.db.Model(&model.User{}).Count(&userCount)

	var keyCount int64
	h.db.Model(&model.APIKey{}).Count(&keyCount)

	var orderCount int64
	h.db.Model(&model.PaymentOrder{}).Where("status = ?", "paid").Count(&orderCount)

	var totalRevenue int64
	h.db.Model(&model.PaymentOrder{}).Where("status = ?", "paid").Select("COALESCE(SUM(amount_cents),0)").Scan(&totalRevenue)

	var totalRequests int64
	h.db.Model(&model.UsageRecord{}).Count(&totalRequests)

	// Today's stats
	today := time.Now().Truncate(24 * time.Hour)
	var todayRequests int64
	h.db.Model(&model.UsageRecord{}).Where("created_at >= ?", today).Count(&todayRequests)

	var todayUsers int64
	h.db.Model(&model.User{}).Where("created_at >= ?", today).Count(&todayUsers)

	c.JSON(http.StatusOK, gin.H{
		"users":          userCount,
		"api_keys":       keyCount,
		"paid_orders":    orderCount,
		"total_revenue":  totalRevenue,
		"total_requests": totalRequests,
		"today_requests": todayRequests,
		"today_users":    todayUsers,
	})
}

// ListUsers returns paginated user list
func (h *AdminHandler) ListUsers(c *gin.Context) {
	page := 1
	pageSize := 50
	fmt.Sscanf(c.DefaultQuery("page", "1"), "%d", &page)
	fmt.Sscanf(c.DefaultQuery("page_size", "50"), "%d", &pageSize)
	if page < 1 {
		page = 1
	}
	if pageSize > 200 {
		pageSize = 200
	}

	query := h.db.Model(&model.User{})

	if q := c.Query("q"); q != "" {
		query = query.Where("email LIKE ? OR name LIKE ?", "%"+q+"%", "%"+q+"%")
	}
	if s := c.Query("status"); s != "" {
		query = query.Where("status = ?", s)
	}

	var total int64
	query.Count(&total)

	var users []model.User
	query.Order("created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&users)

	c.JSON(http.StatusOK, gin.H{
		"users":     users,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"pages":     (total + int64(pageSize) - 1) / int64(pageSize),
	})
}

// UpdateUser allows admin to update user status, balance, admin flag, etc.
func (h *AdminHandler) UpdateUser(c *gin.Context) {
	userID := c.Param("id")

	var req struct {
		Status    *string `json:"status"`
		Balance   *int64  `json:"balance"`
		FreeQuota *int64  `json:"free_quota"`
		IsAdmin   *bool   `json:"is_admin"`
		Name      *string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	updates := map[string]interface{}{}
	if req.Status != nil {
		updates["status"] = *req.Status
	}
	if req.Balance != nil {
		updates["balance"] = *req.Balance
	}
	if req.FreeQuota != nil {
		updates["free_quota"] = *req.FreeQuota
	}
	if req.IsAdmin != nil {
		updates["is_admin"] = *req.IsAdmin
	}
	if req.Name != nil {
		updates["name"] = *req.Name
	}

	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "nothing to update"})
		return
	}

	result := h.db.Model(&model.User{}).Where("id = ?", userID).Updates(updates)
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "user updated"})
}

// GetUser returns a single user with details
func (h *AdminHandler) GetUser(c *gin.Context) {
	userID := c.Param("id")

	var user model.User
	if err := h.db.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	var keyCount int64
	h.db.Model(&model.APIKey{}).Where("user_id = ?", userID).Count(&keyCount)

	var requestCount int64
	h.db.Model(&model.UsageRecord{}).Where("user_id = ?", userID).Count(&requestCount)

	var orderCount int64
	h.db.Model(&model.PaymentOrder{}).Where("user_id = ? AND status = ?", userID, "paid").Count(&orderCount)

	c.JSON(http.StatusOK, gin.H{
		"user":          user,
		"api_key_count": keyCount,
		"request_count": requestCount,
		"order_count":   orderCount,
	})
}

// AllLogs returns usage logs for all users with pagination/filtering
func (h *AdminHandler) AllLogs(c *gin.Context) {
	page := 1
	pageSize := 50
	fmt.Sscanf(c.DefaultQuery("page", "1"), "%d", &page)
	fmt.Sscanf(c.DefaultQuery("page_size", "50"), "%d", &pageSize)
	if page < 1 {
		page = 1
	}
	if pageSize > 200 {
		pageSize = 200
	}

	query := h.db.Model(&model.UsageRecord{})

	if uid := c.Query("user_id"); uid != "" {
		query = query.Where("user_id = ?", uid)
	}
	if m := c.Query("model"); m != "" {
		query = query.Where("model LIKE ?", "%"+m+"%")
	}
	if s := c.Query("status"); s != "" {
		query = query.Where("status = ?", s)
	}
	if p := c.Query("provider"); p != "" {
		query = query.Where("provider = ?", p)
	}
	if from := c.Query("from"); from != "" {
		if t, err := time.Parse("2006-01-02", from); err == nil {
			query = query.Where("created_at >= ?", t)
		}
	}
	if to := c.Query("to"); to != "" {
		if t, err := time.Parse("2006-01-02", to); err == nil {
			query = query.Where("created_at < ?", t.AddDate(0, 0, 1))
		}
	}

	var total int64
	query.Count(&total)

	var records []model.UsageRecord
	query.Order("created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&records)

	c.JSON(http.StatusOK, gin.H{
		"logs":      records,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"pages":     (total + int64(pageSize) - 1) / int64(pageSize),
	})
}

// AllOrders returns all payment orders with pagination
func (h *AdminHandler) AllOrders(c *gin.Context) {
	page := 1
	pageSize := 50
	fmt.Sscanf(c.DefaultQuery("page", "1"), "%d", &page)
	fmt.Sscanf(c.DefaultQuery("page_size", "50"), "%d", &pageSize)
	if page < 1 {
		page = 1
	}
	if pageSize > 200 {
		pageSize = 200
	}

	query := h.db.Model(&model.PaymentOrder{})

	if uid := c.Query("user_id"); uid != "" {
		query = query.Where("user_id = ?", uid)
	}
	if s := c.Query("status"); s != "" {
		query = query.Where("status = ?", s)
	}
	if ch := c.Query("channel"); ch != "" {
		query = query.Where("channel = ?", ch)
	}

	var total int64
	query.Count(&total)

	var orders []model.PaymentOrder
	query.Order("created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&orders)

	c.JSON(http.StatusOK, gin.H{
		"orders":    orders,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"pages":     (total + int64(pageSize) - 1) / int64(pageSize),
	})
}

// AdminMe returns the current admin user's roles and permissions (no extra perm needed)
func (h *AdminHandler) AdminMe(c *gin.Context) {
	userID := c.GetString("user_id")

	var user model.User
	if err := h.db.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	roles, _ := c.Get("roles")
	permissions, _ := c.Get("permissions")

	c.JSON(http.StatusOK, gin.H{
		"user": gin.H{
			"id":       user.ID,
			"email":    user.Email,
			"name":     user.Name,
			"is_admin": user.IsAdmin,
		},
		"roles":       roles,
		"permissions": permissions,
	})
}

// ListRoles returns all roles with their permissions
func (h *AdminHandler) ListRoles(c *gin.Context) {
	var roles []model.Role
	h.db.Preload("Permissions").Order("name").Find(&roles)

	c.JSON(http.StatusOK, gin.H{"roles": roles})
}

// GetRole returns a single role with permissions and assigned users
func (h *AdminHandler) GetRole(c *gin.Context) {
	roleID := c.Param("id")

	var role model.Role
	if err := h.db.Preload("Permissions").Where("id = ?", roleID).First(&role).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "role not found"})
		return
	}

	// Users assigned to this role
	var userRoles []model.UserRole
	h.db.Where("role_id = ?", roleID).Find(&userRoles)
	userIDs := make([]string, len(userRoles))
	for i, ur := range userRoles {
		userIDs[i] = ur.UserID
	}

	var users []model.User
	if len(userIDs) > 0 {
		h.db.Where("id IN ?", userIDs).Find(&users)
	}

	c.JSON(http.StatusOK, gin.H{
		"role":  role,
		"users": users,
	})
}

// AssignRole assigns a role to a user
func (h *AdminHandler) AssignRole(c *gin.Context) {
	userID := c.Param("id")

	var req struct {
		RoleID string `json:"role_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "role_id required"})
		return
	}

	// Verify user exists
	var user model.User
	if err := h.db.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	// Verify role exists
	var role model.Role
	if err := h.db.Where("id = ?", req.RoleID).First(&role).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "role not found"})
		return
	}

	// Check duplicate
	var existing model.UserRole
	if err := h.db.Where("user_id = ? AND role_id = ?", userID, req.RoleID).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "role already assigned"})
		return
	}

	ur := model.UserRole{UserID: userID, RoleID: req.RoleID}
	if err := h.db.Create(&ur).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to assign role"})
		return
	}

	// Also set is_admin=true for backward compat
	h.db.Model(&model.User{}).Where("id = ?", userID).Update("is_admin", true)

	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("role '%s' assigned to user", role.Name)})
}

// RevokeRole removes a role from a user
func (h *AdminHandler) RevokeRole(c *gin.Context) {
	userID := c.Param("id")
	roleID := c.Param("role_id")

	result := h.db.Where("user_id = ? AND role_id = ?", userID, roleID).Delete(&model.UserRole{})
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "user-role assignment not found"})
		return
	}

	// If user has no more roles, clear is_admin
	var count int64
	h.db.Model(&model.UserRole{}).Where("user_id = ?", userID).Count(&count)
	if count == 0 {
		h.db.Model(&model.User{}).Where("id = ?", userID).Update("is_admin", false)
	}

	c.JSON(http.StatusOK, gin.H{"message": "role revoked"})
}

// ListPermissions returns all permissions
func (h *AdminHandler) ListPermissions(c *gin.Context) {
	var perms []model.Permission
	h.db.Order("name").Find(&perms)

	c.JSON(http.StatusOK, gin.H{"permissions": perms})
}

// ListProviders returns all registered providers and their models
func (h *AdminHandler) ListProviders(c *gin.Context) {
	entries := h.registry.ListModels()

	type providerInfo struct {
		Slug       string `json:"slug"`
		ModelCount int    `json:"model_count"`
	}

	provMap := map[string]int{}
	for _, e := range entries {
		provMap[e.Slug]++
	}

	providers := make([]providerInfo, 0, len(provMap))
	for slug, count := range provMap {
		providers = append(providers, providerInfo{Slug: slug, ModelCount: count})
	}

	models := make([]gin.H, 0, len(entries))
	for _, e := range entries {
		models = append(models, gin.H{
			"name":           e.Model.Name,
			"provider":       e.Slug,
			"type":           e.Model.Type,
			"context_length": e.Model.ContextLength,
			"input_price":    e.Model.InputPrice,
			"output_price":   e.Model.OutputPrice,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"providers": providers,
		"models":    models,
	})
}
