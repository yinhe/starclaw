package v1

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw/internal/model"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type SettingsHandler struct {
	db *gorm.DB
}

func NewSettingsHandler(db *gorm.DB) *SettingsHandler {
	return &SettingsHandler{db: db}
}

func (h *SettingsHandler) GetProfile(c *gin.Context) {
	userID := c.GetString("user_id")

	var user model.User
	if err := h.db.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user": gin.H{
			"id":         user.ID,
			"username":   user.Username,
			"email":      user.Email,
			"phone":      user.Phone,
			"created_at": user.CreatedAt,
		},
	})
}

func (h *SettingsHandler) UpdateProfile(c *gin.Context) {
	userID := c.GetString("user_id")

	var req struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Phone    string `json:"phone"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := map[string]interface{}{}
	if req.Username != "" {
		updates["username"] = req.Username
	}
	if req.Email != "" {
		updates["email"] = req.Email
	}
	if req.Phone != "" {
		updates["phone"] = req.Phone
	}

	if len(updates) > 0 {
		h.db.Model(&model.User{}).Where("id = ?", userID).Updates(updates)
	}

	c.JSON(http.StatusOK, gin.H{"message": "updated"})
}

func (h *SettingsHandler) ChangePassword(c *gin.Context) {
	userID := c.GetString("user_id")

	var req struct {
		OldPassword string `json:"old_password" binding:"required"`
		NewPassword string `json:"new_password" binding:"required,min=6"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user model.User
	if err := h.db.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.OldPassword)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "旧密码不正确"})
		return
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
		return
	}

	h.db.Model(&user).Update("password", string(hashed))
	c.JSON(http.StatusOK, gin.H{"message": "password changed"})
}

// API Keys - stored as model configs with user association
func (h *SettingsHandler) GetAPIKeys(c *gin.Context) {
	userID := c.GetString("user_id")

	var keys []model.ModelConfig
	h.db.Where("user_id = ?", userID).Group("provider").Find(&keys)

	// Build response with masked api_key (struct has json:"-" so we must do it manually)
	type keyItem struct {
		ID          string `json:"id"`
		Provider    string `json:"provider"`
		DisplayName string `json:"display_name"`
		APIKey      string `json:"api_key"`
	}
	items := make([]keyItem, 0, len(keys))
	for _, k := range keys {
		masked := ""
		if k.APIKey != "" {
			if len(k.APIKey) > 8 {
				masked = k.APIKey[:4] + "****" + k.APIKey[len(k.APIKey)-4:]
			} else {
				masked = "****"
			}
		}
		items = append(items, keyItem{
			ID:          k.ID,
			Provider:    k.Provider,
			DisplayName: k.DisplayName,
			APIKey:      masked,
		})
	}

	c.JSON(http.StatusOK, gin.H{"api_keys": items})
}
