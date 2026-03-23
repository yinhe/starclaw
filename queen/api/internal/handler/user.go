package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"starclaw.net/queen/api/internal/database"
	"starclaw.net/queen/api/internal/model"
	"golang.org/x/crypto/bcrypt"
)

type UserHandler struct{}

// GET /user/profile
func (h *UserHandler) GetProfile(c *gin.Context) {
	userID := c.GetString("user_id")
	var user model.User
	if err := database.DB.First(&user, "id = ?", userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": user})
}

// PUT /user/profile
func (h *UserHandler) UpdateProfile(c *gin.Context) {
	userID := c.GetString("user_id")
	var req struct {
		Nickname string `json:"nickname"`
		Avatar   string `json:"avatar"`
		Bio      string `json:"bio"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	updates := map[string]interface{}{}
	if req.Nickname != "" {
		updates["nickname"] = req.Nickname
	}
	if req.Avatar != "" {
		updates["avatar"] = req.Avatar
	}
	if req.Bio != "" {
		updates["bio"] = req.Bio
	}

	database.DB.Model(&model.User{}).Where("id = ?", userID).Updates(updates)
	c.JSON(http.StatusOK, gin.H{"message": "更新成功"})
}

// PUT /user/password
func (h *UserHandler) ChangePassword(c *gin.Context) {
	userID := c.GetString("user_id")
	var req struct {
		OldPassword string `json:"old_password" binding:"required"`
		NewPassword string `json:"new_password" binding:"required,min=6"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	var user model.User
	database.DB.First(&user, "id = ?", userID)
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.OldPassword)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "原密码错误"})
		return
	}

	hashed, _ := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	database.DB.Model(&user).Update("password", string(hashed))
	c.JSON(http.StatusOK, gin.H{"message": "密码已更新"})
}
