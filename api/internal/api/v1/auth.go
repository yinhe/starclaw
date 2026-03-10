package v1

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/yinhe/starclaw/internal/config"
	"github.com/yinhe/starclaw/internal/model"
	"github.com/yinhe/starclaw/internal/node"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// ── Token login rate limiter: 5 failures per IP → lock 15 min ──

const (
	tokenMaxAttempts  = 5
	tokenLockDuration = 15 * time.Minute
)

type loginAttempt struct {
	count    int
	lockedAt time.Time
}

var (
	tokenAttempts sync.Map // map[ip]loginAttempt
)

func checkTokenRateLimit(ip string) error {
	val, ok := tokenAttempts.Load(ip)
	if !ok {
		return nil
	}
	a := val.(loginAttempt)
	if a.count >= tokenMaxAttempts {
		remaining := tokenLockDuration - time.Since(a.lockedAt)
		if remaining > 0 {
			return fmt.Errorf("登录失败次数过多，请 %d 分钟后重试", int(remaining.Minutes())+1)
		}
		tokenAttempts.Delete(ip)
	}
	return nil
}

func recordTokenFailure(ip string) {
	val, _ := tokenAttempts.Load(ip)
	a, _ := val.(loginAttempt)
	a.count++
	if a.count >= tokenMaxAttempts {
		a.lockedAt = time.Now()
	}
	tokenAttempts.Store(ip, a)
}

func clearTokenFailure(ip string) {
	tokenAttempts.Delete(ip)
}

type AuthHandler struct {
	db       *gorm.DB
	cfg      *config.Config
	identity *node.Identity
}

func NewAuthHandler(db *gorm.DB, cfg *config.Config, identity *node.Identity) *AuthHandler {
	return &AuthHandler{db: db, cfg: cfg, identity: identity}
}

type RegisterRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Username string `json:"username"`
	Password string `json:"password" binding:"required,min=6"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type PhoneRegisterRequest struct {
	Phone    string `json:"phone" binding:"required,min=11,max=15"`
	Username string `json:"username"`
	Password string `json:"password" binding:"required,min=6"`
}

type PhoneLoginRequest struct {
	Phone    string `json:"phone" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type AuthResponse struct {
	Token string     `json:"token"`
	User  model.User `json:"user"`
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check email uniqueness
	var count int64
	h.db.Model(&model.User{}).Where("email = ?", req.Email).Count(&count)
	if count > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "该邮箱已被注册"})
		return
	}

	// Auto-generate username if not provided
	username := req.Username
	if username == "" {
		b := make([]byte, 2)
		rand.Read(b)
		username = "Claw#" + hex.EncodeToString(b)
	}

	// Check username uniqueness
	h.db.Model(&model.User{}).Where("username = ?", username).Count(&count)
	if count > 0 {
		if req.Username == "" {
			b := make([]byte, 3)
			rand.Read(b)
			username = "Claw#" + hex.EncodeToString(b)
		} else {
			c.JSON(http.StatusConflict, gin.H{"error": "该用户名已被使用"})
			return
		}
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
		return
	}

	user := model.User{
		Email:    &req.Email,
		Username: username,
		Password: string(hashedPassword),
	}

	if err := h.db.Create(&user).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "注册失败，请重试"})
		return
	}

	token, err := h.generateToken(&user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	c.JSON(http.StatusCreated, AuthResponse{Token: token, User: user})
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user model.User
	if err := h.db.Where("email = ?", req.Email).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	token, err := h.generateToken(&user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, AuthResponse{Token: token, User: user})
}

func (h *AuthHandler) PhoneRegister(c *gin.Context) {
	var req PhoneRegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check phone uniqueness
	var count int64
	h.db.Model(&model.User{}).Where("phone = ?", req.Phone).Count(&count)
	if count > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "该手机号已被注册"})
		return
	}

	username := req.Username
	if username == "" {
		username = "Claw#" + req.Phone[len(req.Phone)-4:]
	}

	// Check username uniqueness
	h.db.Model(&model.User{}).Where("username = ?", username).Count(&count)
	if count > 0 {
		if req.Username == "" {
			// Auto-generated username collided, append random suffix
			b := make([]byte, 2)
			rand.Read(b)
			username = "Claw#" + hex.EncodeToString(b)
		} else {
			c.JSON(http.StatusConflict, gin.H{"error": "该用户名已被使用"})
			return
		}
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
		return
	}

	user := model.User{
		Phone:    &req.Phone,
		Username: username,
		Password: string(hashedPassword),
	}

	if err := h.db.Create(&user).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "注册失败，请重试"})
		return
	}

	token, err := h.generateToken(&user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	c.JSON(http.StatusCreated, AuthResponse{Token: token, User: user})
}

func (h *AuthHandler) PhoneLogin(c *gin.Context) {
	var req PhoneLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user model.User
	if err := h.db.Where("phone = ?", req.Phone).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	token, err := h.generateToken(&user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, AuthResponse{Token: token, User: user})
}

// TokenLogin authenticates with a compact server-bound API token.
// Tracks device on first use; rejects revoked devices.
// Rate limited: 5 failures per IP → locked 15 minutes.
func (h *AuthHandler) TokenLogin(c *gin.Context) {
	ip := c.ClientIP()

	if err := checkTokenRateLimit(ip); err != nil {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": err.Error()})
		return
	}

	var req struct {
		Token      string `json:"token" binding:"required"`
		DeviceID   string `json:"device_id" binding:"required"`
		DeviceName string `json:"device_name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Cryptographic verification (HMAC-SHA256)
	payload := h.identity.VerifyAPIToken(req.Token)
	if payload == nil {
		recordTokenFailure(ip)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的 Token"})
		return
	}

	// Look up user
	var user model.User
	if err := h.db.Where("id = ?", payload.UserID).First(&user).Error; err != nil {
		recordTokenFailure(ip)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的 Token"})
		return
	}

	// Check token revocation (regenerate invalidates old tokens)
	if user.TokenIssuedAt != nil && payload.IssuedAt < user.TokenIssuedAt.Unix() {
		recordTokenFailure(ip)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Token 已失效，请重新生成"})
		return
	}

	// Check device authorization
	var device model.AuthorizedDevice
	err := h.db.Where("user_id = ? AND device_id = ?", user.ID, req.DeviceID).First(&device).Error
	if err == nil && device.Revoked {
		recordTokenFailure(ip)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "此设备已被撤销授权"})
		return
	}

	// Auto-register new device
	now := time.Now()
	if err != nil {
		device = model.AuthorizedDevice{
			UserID:     user.ID,
			DeviceID:   req.DeviceID,
			DeviceName: req.DeviceName,
			LastUsedAt: &now,
		}
		h.db.Create(&device)
	} else {
		h.db.Model(&device).Updates(map[string]interface{}{
			"last_used_at": now,
			"device_name":  req.DeviceName,
		})
	}

	clearTokenFailure(ip)

	token, err2 := h.generateToken(&user)
	if err2 != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, AuthResponse{Token: token, User: user})
}

// GetAPIToken returns the current user's API token (one per user).
// In opensource mode (owner_token set), returns the owner token directly.
func (h *AuthHandler) GetAPIToken(c *gin.Context) {
	userID, _ := c.Get("user_id")
	// Check if user has an owner_token (opensource / single-user mode)
	var user model.User
	if err := h.db.Where("id = ?", userID).First(&user).Error; err == nil && user.OwnerToken != nil && *user.OwnerToken != "" {
		c.JSON(http.StatusOK, gin.H{
			"api_token": *user.OwnerToken,
			"node_id":   h.identity.NodeID,
		})
		return
	}
	newToken := h.identity.GenerateAPIToken(userID.(string))
	c.JSON(http.StatusOK, gin.H{
		"api_token": newToken,
		"node_id":   h.identity.NodeID,
	})
}

// RegenerateToken creates a new token, invalidates old one, clears all devices.
func (h *AuthHandler) RegenerateToken(c *gin.Context) {
	userID, _ := c.Get("user_id")
	now := time.Now()
	h.db.Model(&model.User{}).Where("id = ?", userID).Update("token_issued_at", now)
	h.db.Where("user_id = ?", userID).Delete(&model.AuthorizedDevice{})
	// Check if user has owner_token — regenerate it
	var user model.User
	if err := h.db.Where("id = ?", userID).First(&user).Error; err == nil && user.OwnerToken != nil && *user.OwnerToken != "" {
		tokenBytes := make([]byte, 16)
		rand.Read(tokenBytes)
		newOwnerToken := hex.EncodeToString(tokenBytes)
		h.db.Model(&model.User{}).Where("id = ?", userID).Update("owner_token", newOwnerToken)
		c.JSON(http.StatusOK, gin.H{
			"api_token": newOwnerToken,
			"node_id":   h.identity.NodeID,
		})
		return
	}
	newToken := h.identity.GenerateAPIToken(userID.(string))
	c.JSON(http.StatusOK, gin.H{
		"api_token": newToken,
		"node_id":   h.identity.NodeID,
	})
}

// ListDevices returns all devices that have used the token.
func (h *AuthHandler) ListDevices(c *gin.Context) {
	userID, _ := c.Get("user_id")
	var devices []model.AuthorizedDevice
	h.db.Where("user_id = ?", userID).Order("created_at desc").Find(&devices)
	c.JSON(http.StatusOK, gin.H{"devices": devices})
}

// RevokeDevice blocks a specific device from using the token.
func (h *AuthHandler) RevokeDevice(c *gin.Context) {
	userID, _ := c.Get("user_id")
	deviceID := c.Param("deviceID")
	result := h.db.Model(&model.AuthorizedDevice{}).
		Where("user_id = ? AND device_id = ?", userID, deviceID).
		Update("revoked", true)
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "设备不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "设备已撤销"})
}

func (h *AuthHandler) generateToken(user *model.User) (string, error) {
	role := user.Role
	if role == "" {
		role = "user"
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
