package ops

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw/internal/model"
	"gorm.io/gorm"
)

type AdminHandler struct {
	db *gorm.DB
}

func NewAdminHandler(db *gorm.DB) *AdminHandler {
	return &AdminHandler{db: db}
}

// ListUsers returns all users (admin only)
func (h *AdminHandler) ListUsers(c *gin.Context) {
	var users []model.User
	h.db.Order("created_at DESC").Find(&users)
	c.JSON(http.StatusOK, gin.H{"users": users})
}

// UpdateUserRole changes a user's role (admin only)
func (h *AdminHandler) UpdateUserRole(c *gin.Context) {
	userID := c.Param("id")
	var req struct {
		Role string `json:"role" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Role != "admin" && req.Role != "user" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "role must be 'admin' or 'user'"})
		return
	}

	// Prevent self-demotion
	currentUserID := c.GetString("user_id")
	if userID == currentUserID && req.Role != "admin" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot demote yourself"})
		return
	}

	result := h.db.Model(&model.User{}).Where("id = ?", userID).Update("role", req.Role)
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "role updated"})
}

// DeleteUser removes a user account (admin only)
func (h *AdminHandler) DeleteUser(c *gin.Context) {
	userID := c.Param("id")
	currentUserID := c.GetString("user_id")

	if userID == currentUserID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot delete yourself"})
		return
	}

	result := h.db.Where("id = ?", userID).Delete(&model.User{})
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "user deleted"})
}

// SystemStats returns system-wide statistics (admin only)
func (h *AdminHandler) SystemStats(c *gin.Context) {
	var userCount, agentCount, convCount, wfCount int64
	h.db.Model(&model.User{}).Count(&userCount)
	h.db.Model(&model.Agent{}).Count(&agentCount)
	h.db.Model(&model.Conversation{}).Count(&convCount)
	h.db.Model(&model.Workflow{}).Count(&wfCount)

	c.JSON(http.StatusOK, gin.H{
		"users":         userCount,
		"agents":        agentCount,
		"conversations": convCount,
		"workflows":     wfCount,
	})
}
