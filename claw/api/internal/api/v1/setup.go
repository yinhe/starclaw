package v1

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/yinhe/starclaw/internal/config"
	"github.com/yinhe/starclaw/internal/model"
	"github.com/yinhe/starclaw/internal/node"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// SetupHandler handles first-time initialization for single-user (Owner) mode.
// In opensource deploy_mode, each Claw has exactly one owner.
// Setup generates a permanent Owner Token (claw_ + 32 hex) for authentication.
type SetupHandler struct {
	db       *gorm.DB
	cfg      *config.Config
	identity *node.Identity
}

func NewSetupHandler(db *gorm.DB, cfg *config.Config, identity *node.Identity) *SetupHandler {
	return &SetupHandler{db: db, cfg: cfg, identity: identity}
}

// Status returns whether the initial setup has been completed.
// In opensource mode, setup is complete when an owner user exists.
// In hosted mode, setup is always considered complete (multi-user registration).
func (h *SetupHandler) Status(c *gin.Context) {
	if h.cfg.Server.DeployMode == "hosted" {
		c.JSON(http.StatusOK, gin.H{
			"setup_completed": true,
			"deploy_mode":     "hosted",
		})
		return
	}

	var count int64
	h.db.Model(&model.User{}).Where("owner_token IS NOT NULL").Count(&count)
	c.JSON(http.StatusOK, gin.H{
		"setup_completed": count > 0,
		"deploy_mode":     h.cfg.Server.DeployMode,
	})
}

// Setup performs first-time initialization: creates or promotes the owner user.
// - Fresh install: creates a new owner user with generated token.
// - Upgrade: promotes the first existing user to owner.
// Only works once; rejects if an owner already exists.
func (h *SetupHandler) Setup(c *gin.Context) {
	if h.cfg.Server.DeployMode == "hosted" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "hosted 模式不支持 Setup，请使用注册登录"})
		return
	}

	// Check if owner already exists
	var ownerCount int64
	h.db.Model(&model.User{}).Where("owner_token IS NOT NULL").Count(&ownerCount)
	if ownerCount > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "已完成初始化，无法重复设置"})
		return
	}

	var req struct {
		Password string `json:"password"` // optional, recommended for public-facing instances
		Username string `json:"username"` // optional, auto-generated if empty
	}
	c.ShouldBindJSON(&req)

	// Generate owner token: 32 hex chars (16 bytes = 128-bit entropy)
	tokenBytes := make([]byte, 16)
	if _, err := rand.Read(tokenBytes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}
	ownerToken := hex.EncodeToString(tokenBytes)

	// Hash password if provided
	var hashedPw string
	if req.Password != "" {
		if len(req.Password) < 6 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "密码长度至少 6 位"})
			return
		}
		hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
			return
		}
		hashedPw = string(hashed)
	}

	// Check if there's an existing user to promote (upgrade from old version)
	var existingUser model.User
	if err := h.db.Order("created_at ASC").First(&existingUser).Error; err == nil {
		// Promote existing user to owner
		updates := map[string]interface{}{
			"owner_token": ownerToken,
			"role":        "owner",
		}
		if hashedPw != "" {
			updates["password"] = hashedPw
		}
		if req.Username != "" {
			updates["username"] = req.Username
		}
		h.db.Model(&existingUser).Updates(updates)
		existingUser.OwnerToken = &ownerToken
		existingUser.Role = "owner"
		if req.Username != "" {
			existingUser.Username = req.Username
		}

		jwtToken, _ := h.generateJWT(&existingUser)
		c.JSON(http.StatusCreated, gin.H{
			"owner_token": ownerToken,
			"token":       jwtToken,
			"user":        existingUser,
		})
		return
	}

	// Fresh install: create new owner user
	username := req.Username
	if username == "" {
		b := make([]byte, 2)
		rand.Read(b)
		username = "Claw#" + hex.EncodeToString(b)
	}

	user := model.User{
		Username:   username,
		Password:   hashedPw,
		OwnerToken: &ownerToken,
		Role:       "owner",
	}

	if err := h.db.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "初始化失败: " + err.Error()})
		return
	}

	jwtToken, _ := h.generateJWT(&user)
	c.JSON(http.StatusCreated, gin.H{
		"owner_token": ownerToken,
		"token":       jwtToken,
		"user":        user,
	})
}

// PasswordLogin authenticates the owner via password (for token recovery).
// Returns the owner_token so it can be stored again in localStorage.
// Only works in opensource mode when the owner has set a password.
func (h *SetupHandler) PasswordLogin(c *gin.Context) {
	var req struct {
		Password   string `json:"password" binding:"required"`
		DeviceID   string `json:"device_id"`
		DeviceName string `json:"device_name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请输入密码"})
		return
	}

	// Find the owner user
	var user model.User
	if err := h.db.Where("owner_token IS NOT NULL").First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未找到 Owner 用户"})
		return
	}

	if user.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Owner 未设置密码，请通过 CLI 重置: starclaw reset-token"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "密码错误"})
		return
	}

	// Auto-approve device on password login (password proves identity)
	if req.DeviceID != "" {
		now := time.Now()
		var device model.AuthorizedDevice
		if err := h.db.Where("user_id = ? AND device_id = ?", user.ID, req.DeviceID).First(&device).Error; err != nil {
			device = model.AuthorizedDevice{
				UserID:     user.ID,
				DeviceID:   req.DeviceID,
				DeviceName: req.DeviceName,
				Approved:   true,
				LastUsedAt: &now,
			}
			h.db.Create(&device)
		} else if !device.Approved || device.Revoked {
			h.db.Model(&device).Updates(map[string]interface{}{
				"approved":     true,
				"revoked":      false,
				"last_used_at": now,
			})
		}
	}

	jwtToken, err := h.generateJWT(&user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate JWT"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"owner_token": *user.OwnerToken,
		"token":       jwtToken,
		"user":        user,
	})
}

func (h *SetupHandler) generateJWT(user *model.User) (string, error) {
	role := user.Role
	if role == "" {
		role = "owner"
	}
	claims := jwt.MapClaims{
		"sub":      user.ID,
		"username": user.Username,
		"role":     role,
		"exp":      time.Now().Add(time.Duration(h.cfg.JWT.ExpireHour) * time.Hour).Unix(),
		"iat":      time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(h.cfg.JWT.Secret))
}
