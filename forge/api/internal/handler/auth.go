package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"starclaw.net/forge/internal/config"
)

// AuthMiddleware checks JWT token in Authorization header.
// Skips: /health, /api/auth/login, /api/webhooks/*
func AuthMiddleware(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path

		// Skip public endpoints
		if path == "/health" ||
			path == "/api/auth/login" ||
			strings.HasPrefix(path, "/api/webhooks/") {
			c.Next()
			return
		}

		// Skip if no whitelist configured (dev mode)
		if len(cfg.Whitelist) == 0 {
			c.Next()
			return
		}

		auth := c.GetHeader("Authorization")
		if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "未登录，请先认证"})
			return
		}

		token := strings.TrimPrefix(auth, "Bearer ")
		claims, err := verifyToken(token, cfg.Secret)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "认证已过期，请重新登录"})
			return
		}

		// Check if node is still in whitelist
		nodeID, _ := claims["node_id"].(string)
		if _, ok := cfg.Whitelist[nodeID]; !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "节点不在白名单中"})
			return
		}

		c.Set("node_id", nodeID)
		c.Next()
	}
}

// LoginHandler handles POST /api/auth/login
func LoginHandler(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			NodeID   string `json:"node_id" binding:"required"`
			Password string `json:"password" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "请提供 node_id 和 password"})
			return
		}

		// Check whitelist
		if len(cfg.Whitelist) == 0 {
			c.JSON(http.StatusForbidden, gin.H{"error": "白名单未配置"})
			return
		}

		expectedPass, ok := cfg.Whitelist[req.NodeID]
		if !ok {
			c.JSON(http.StatusForbidden, gin.H{"error": "节点不在白名单中"})
			return
		}
		if expectedPass != req.Password {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "密码错误"})
			return
		}

		// Generate token (7 days)
		token, err := generateToken(req.NodeID, cfg.Secret, 7*24*time.Hour)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "生成 token 失败"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"token":   token,
			"node_id": req.NodeID,
			"expires": time.Now().Add(7 * 24 * time.Hour).Unix(),
		})
	}
}

// MeHandler returns current authenticated node info
func MeHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		nodeID, _ := c.Get("node_id")
		c.JSON(http.StatusOK, gin.H{"node_id": nodeID})
	}
}

// ── Simple HMAC-based token (no external JWT dependency) ──

func generateToken(nodeID, secret string, ttl time.Duration) (string, error) {
	payload := map[string]interface{}{
		"node_id": nodeID,
		"exp":     time.Now().Add(ttl).Unix(),
		"iat":     time.Now().Unix(),
	}
	payloadJSON, _ := json.Marshal(payload)
	payloadHex := hex.EncodeToString(payloadJSON)
	sig := sign(payloadHex, secret)
	return fmt.Sprintf("%s.%s", payloadHex, sig), nil
}

func verifyToken(token, secret string) (map[string]interface{}, error) {
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid token format")
	}
	payloadHex, sig := parts[0], parts[1]

	// Verify signature
	expected := sign(payloadHex, secret)
	if !hmac.Equal([]byte(sig), []byte(expected)) {
		return nil, fmt.Errorf("invalid signature")
	}

	// Decode payload
	payloadJSON, err := hex.DecodeString(payloadHex)
	if err != nil {
		return nil, fmt.Errorf("invalid payload encoding")
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(payloadJSON, &claims); err != nil {
		return nil, fmt.Errorf("invalid payload JSON")
	}

	// Check expiry
	if exp, ok := claims["exp"].(float64); ok {
		if time.Now().Unix() > int64(exp) {
			return nil, fmt.Errorf("token expired")
		}
	}

	return claims, nil
}

func sign(data, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(data))
	return hex.EncodeToString(mac.Sum(nil))
}
