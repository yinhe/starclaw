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
	Username string `json:"username" binding:"required,min=3,max=50"`
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

	// Check username uniqueness
	h.db.Model(&model.User{}).Where("username = ?", req.Username).Count(&count)
	if count > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "该用户名已被使用"})
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
		return
	}

	user := model.User{
		Email:    &req.Email,
		Username: req.Username,
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
		username = "user_" + req.Phone[len(req.Phone)-4:]
	}

	// Check username uniqueness
	h.db.Model(&model.User{}).Where("username = ?", username).Count(&count)
	if count > 0 {
		if req.Username == "" {
			// Auto-generated username collided, append random suffix
			b := make([]byte, 2)
			rand.Read(b)
			username = username + "_" + hex.EncodeToString(b)
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
// Rate limited: 5 failures per IP → locked 15 minutes.
func (h *AuthHandler) TokenLogin(c *gin.Context) {
	ip := c.ClientIP()

	// Rate limit check
	if err := checkTokenRateLimit(ip); err != nil {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": err.Error()})
		return
	}

	var req struct {
		Token string `json:"token" binding:"required"`
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

	// Check revocation: token must be issued after TokenIssuedAt
	if user.TokenIssuedAt != nil && payload.IssuedAt < user.TokenIssuedAt.Unix() {
		recordTokenFailure(ip)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Token 已失效，请重新生成"})
		return
	}

	// Success — clear rate limit
	clearTokenFailure(ip)

	token, err := h.generateToken(&user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, AuthResponse{Token: token, User: user})
}

// RegenerateToken creates a new server-bound API token for the current user.
// Old tokens are invalidated by updating TokenIssuedAt.
func (h *AuthHandler) RegenerateToken(c *gin.Context) {
	userID, _ := c.Get("userID")
	now := time.Now()
	if err := h.db.Model(&model.User{}).Where("id = ?", userID).Update("token_issued_at", now).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to regenerate token"})
		return
	}
	newToken := h.identity.GenerateAPIToken(userID.(string))
	c.JSON(http.StatusOK, gin.H{
		"api_token": newToken,
		"node_id":   h.identity.NodeID,
	})
}

// GetAPIToken returns a freshly signed API token for the current user.
func (h *AuthHandler) GetAPIToken(c *gin.Context) {
	userID, _ := c.Get("userID")
	newToken := h.identity.GenerateAPIToken(userID.(string))
	c.JSON(http.StatusOK, gin.H{
		"api_token": newToken,
		"node_id":   h.identity.NodeID,
	})
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
