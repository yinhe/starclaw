package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"log"
	"net/http"
	"regexp"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw-router/internal/billing"
	"github.com/yinhe/starclaw-router/internal/middleware"
	"github.com/yinhe/starclaw-router/internal/model"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var phoneRegex = regexp.MustCompile(`^1[3-9]\d{9}$`)

type AuthHandler struct {
	db          *gorm.DB
	jwtSecret   string
	expireHours int
	queenCredit *billing.QueenCreditClient
}

func NewAuthHandler(db *gorm.DB, jwtSecret string, expireHours int) *AuthHandler {
	return &AuthHandler{db: db, jwtSecret: jwtSecret, expireHours: expireHours}
}

// SetQueenCredit enables star energy display in profile for Claw-authenticated users.
func (h *AuthHandler) SetQueenCredit(qc *billing.QueenCreditClient) {
	h.queenCredit = qc
}

type registerRequest struct {
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Password string `json:"password" binding:"required,min=6"`
	Name     string `json:"name"`
}

// Register creates a new user and auto-generates the first API key.
// Accepts email or phone (at least one) + password.
func (h *AuthHandler) Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	if req.Email == "" && req.Phone == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email or phone required"})
		return
	}
	if req.Phone != "" && !phoneRegex.MatchString(req.Phone) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid phone number"})
		return
	}

	// Check duplicate
	var existing model.User
	if req.Email != "" {
		if err := h.db.Where("email = ?", req.Email).First(&existing).Error; err == nil {
			c.JSON(http.StatusConflict, gin.H{"error": "email already registered"})
			return
		}
	}
	if req.Phone != "" {
		if err := h.db.Where("phone = ?", req.Phone).First(&existing).Error; err == nil {
			c.JSON(http.StatusConflict, gin.H{"error": "phone already registered"})
			return
		}
	}

	// Hash password
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	name := req.Name
	if name == "" {
		if req.Phone != "" {
			name = req.Phone[:3] + "****" + req.Phone[7:]
		} else {
			name = req.Email
		}
	}

	identifier := req.Email
	if identifier == "" {
		identifier = req.Phone
	}

	user := model.User{
		Email:        req.Email,
		Phone:        req.Phone,
		PasswordHash: string(hash),
		Name:         name,
		FreeQuota:    0,
		Status:       "active",
	}

	if err := h.db.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user"})
		return
	}

	// Auto-create first API key
	rawKey := model.GenerateAPIKey()
	keyHash := sha256.Sum256([]byte(rawKey))

	apiKey := model.APIKey{
		UserID:    user.ID,
		Name:      "Default",
		KeyHash:   hex.EncodeToString(keyHash[:]),
		KeyPrefix: rawKey[:16] + "...",
		IsEnabled: true,
	}
	h.db.Create(&apiKey)

	// Generate JWT
	token, err := middleware.GenerateJWT(h.jwtSecret, user.ID, identifier, h.expireHours)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"token": token,
		"user": gin.H{
			"id":    user.ID,
			"email": user.Email,
			"phone": user.Phone,
			"name":  user.Name,
		},
		"api_key": gin.H{
			"key":        rawKey,
			"key_prefix": apiKey.KeyPrefix,
			"message":    "Save this API key — it will not be shown again",
		},
	})
}

type loginRequest struct {
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Password string `json:"password" binding:"required"`
}

// Login authenticates via email or phone + password and returns a JWT
func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	if req.Email == "" && req.Phone == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email or phone required"})
		return
	}

	var user model.User
	if req.Phone != "" {
		if err := h.db.Where("phone = ?", req.Phone).First(&user).Error; err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid phone or password"})
			return
		}
	} else {
		if err := h.db.Where("email = ?", req.Email).First(&user).Error; err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid email or password"})
			return
		}
	}

	if user.Status != "active" {
		c.JSON(http.StatusForbidden, gin.H{"error": "account disabled"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	identifier := user.Email
	if identifier == "" {
		identifier = user.Phone
	}

	token, err := middleware.GenerateJWT(h.jwtSecret, user.ID, identifier, h.expireHours)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token": token,
		"user": gin.H{
			"id":    user.ID,
			"email": user.Email,
			"phone": user.Phone,
			"name":  user.Name,
		},
	})
}

// UpdateProfile updates name/email
func (h *AuthHandler) UpdateProfile(c *gin.Context) {
	userID := c.GetString("user_id")

	var req struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	updates := map[string]interface{}{}
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Email != "" {
		// Check duplicate
		var existing model.User
		if err := h.db.Where("email = ? AND id != ?", req.Email, userID).First(&existing).Error; err == nil {
			c.JSON(http.StatusConflict, gin.H{"error": "email already in use"})
			return
		}
		updates["email"] = req.Email
	}

	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "nothing to update"})
		return
	}

	if err := h.db.Model(&model.User{}).Where("id = ?", userID).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "update failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "profile updated"})
}

// ChangePassword verifies old password and sets new one
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	userID := c.GetString("user_id")

	var req struct {
		OldPassword string `json:"old_password" binding:"required"`
		NewPassword string `json:"new_password" binding:"required,min=6"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "old_password and new_password (min 6) required"})
		return
	}

	var user model.User
	if err := h.db.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.OldPassword)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "incorrect current password"})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	h.db.Model(&user).Update("password_hash", string(hash))
	c.JSON(http.StatusOK, gin.H{"message": "password changed"})
}

// Profile returns the current user's info (JWT required)
func (h *AuthHandler) Profile(c *gin.Context) {
	userID := c.GetString("user_id")

	var user model.User
	if err := h.db.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	// Count API keys
	var keyCount int64
	h.db.Model(&model.APIKey{}).Where("user_id = ?", userID).Count(&keyCount)

	userInfo := gin.H{
		"id":         user.ID,
		"email":      user.Email,
		"phone":      user.Phone,
		"name":       user.Name,
		"claw_id":    user.ClawID,
		"balance":    user.Balance,
		"free_quota": user.FreeQuota,
		"status":     user.Status,
		"created_at": user.CreatedAt,
	}

	// Fetch star energy from Queen if user has a Claw address
	if user.ClawID != "" && h.queenCredit != nil && h.queenCredit.Enabled() {
		if bal, err := h.queenCredit.GetBalance(user.ClawID); err == nil {
			userInfo["star_energy"] = bal.Balance                            // internal units (1⚡ = 10000)
			userInfo["star_energy_display"] = float64(bal.Balance) / 10000.0 // display ⚡
			userInfo["star_status"] = bal.Status
		} else {
			log.Printf("[star-ai] queen balance check failed for %s: %v", user.ClawID, err)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"user":          userInfo,
		"api_key_count": keyCount,
	})
}
