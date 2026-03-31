package middleware

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/yinhe/starclaw/internal/config"
	"github.com/yinhe/starclaw/internal/model"
	"gorm.io/gorm"
)

// TokenClaims holds parsed JWT or Owner Token claims
type TokenClaims struct {
	UserID   string
	Username string
	Role     string
}

// ParseToken validates a JWT string and returns the claims
func ParseToken(tokenStr string, secret string) (*TokenClaims, error) {
	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	if err != nil || !token.Valid {
		return nil, fmt.Errorf("invalid or expired token")
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid token claims")
	}
	tc := &TokenClaims{Role: "user"}
	if sub, ok := claims["sub"].(string); ok {
		tc.UserID = sub
	}
	if u, ok := claims["username"].(string); ok {
		tc.Username = u
	}
	if r, ok := claims["role"].(string); ok {
		tc.Role = r
	}
	return tc, nil
}

// ResolveToken validates a JWT, Owner Token, or Service Token and returns claims.
// Strategy: try JWT first; then Service Token (svc-*); then DB lookup as Owner Token.
// Used by both AuthRequired middleware and WebSocket handler.
func ResolveToken(tokenStr string, cfg *config.Config, db *gorm.DB) (*TokenClaims, error) {
	// Try JWT first
	if claims, err := ParseToken(tokenStr, cfg.JWT.Secret); err == nil {
		return claims, nil
	}

	// Try Service Token (svc-*)
	if strings.HasPrefix(tokenStr, "svc-") {
		var svcToken model.ServiceToken
		if err := db.Where("token = ? AND revoked = ?", tokenStr, false).First(&svcToken).Error; err == nil {
			if svcToken.IsValid() {
				now := time.Now()
				db.Model(&svcToken).Update("last_used_at", &now)
				return &TokenClaims{
					UserID:   svcToken.UserID,
					Username: "service:" + svcToken.Name,
					Role:     "service",
				}, nil
			}
		}
		return nil, fmt.Errorf("invalid or revoked service token")
	}

	// Fallback: Owner Token (plain hex string stored in DB)
	var user model.User
	if err := db.Where("owner_token = ?", tokenStr).First(&user).Error; err != nil {
		return nil, fmt.Errorf("invalid token")
	}
	role := user.Role
	if role == "" {
		role = "owner"
	}
	return &TokenClaims{
		UserID:   user.ID,
		Username: user.Username,
		Role:     role,
	}, nil
}

func AuthRequired(cfg *config.Config, db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header"})
			c.Abort()
			return
		}

		tokenStr := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
		if tokenStr == authHeader {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization format"})
			c.Abort()
			return
		}

		claims, err := ResolveToken(tokenStr, cfg, db)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			c.Abort()
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)
		c.Next()
	}
}
