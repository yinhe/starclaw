package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw-overlord/api/internal/model"
	"gorm.io/gorm"
)

// AdminAuth validates admin credentials via Basic Auth or X-Admin-Token header
// and injects the admin user into the context.
func AdminAuth(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Try X-Admin-Token header first (for API tokens / console)
		token := c.GetHeader("X-Admin-Token")
		if token != "" {
			hash := hashToken(token)
			var user model.AdminUser
			if err := db.Where("password_hash = ?", hash).First(&user).Error; err != nil {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
				return
			}
			c.Set("admin_user", &user)
			c.Set("admin_role", user.Role)
			c.Set("admin_team", user.TeamID)
			c.Next()
			return
		}

		// Try Basic Auth
		username, password, ok := c.Request.BasicAuth()
		if !ok {
			// Allow unauthenticated access for Claw-facing endpoints (register/heartbeat)
			c.Set("admin_role", "")
			c.Next()
			return
		}

		hash := hashToken(password)
		var user model.AdminUser
		if err := db.Where("username = ? AND password_hash = ?", username, hash).First(&user).Error; err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
			return
		}

		// Update last login
		now := time.Now()
		db.Model(&user).Update("last_login_at", &now)

		c.Set("admin_user", &user)
		c.Set("admin_role", user.Role)
		c.Set("admin_team", user.TeamID)
		c.Next()
	}
}

// RequirePermission checks that the authenticated admin has the specified permission
func RequirePermission(perm string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, _ := c.Get("admin_role")
		roleStr, _ := role.(string)
		if roleStr == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}
		if !model.HasPermission(roleStr, perm) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "insufficient permissions", "required": perm})
			return
		}
		c.Next()
	}
}

// TeamScope restricts queries to the admin's team (unless superadmin/global admin)
func TeamScope(c *gin.Context, db *gorm.DB) *gorm.DB {
	teamID, exists := c.Get("admin_team")
	if !exists {
		return db
	}
	tid, _ := teamID.(string)
	if tid == "" {
		return db // global access
	}
	return db.Where("team = ?", tid)
}

// GetAdminActor returns the admin username or client IP for audit logging
func GetAdminActor(c *gin.Context) string {
	if user, ok := c.Get("admin_user"); ok {
		if u, ok := user.(*model.AdminUser); ok {
			return u.Username
		}
	}
	if h := c.GetHeader("X-Admin-User"); h != "" {
		return h
	}
	return c.ClientIP()
}

func hashToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

// HashTokenExported is the exported version of hashToken for use by handlers
func HashTokenExported(raw string) string {
	return hashToken(raw)
}

// SplitEvents parses a comma-separated events string into a slice
func SplitEvents(events string) []string {
	if events == "" || events == "*" {
		return nil
	}
	parts := strings.Split(events, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}
