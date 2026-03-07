package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// RequireRole returns a middleware that checks if the user has the required role
func RequireRole(roles ...string) gin.HandlerFunc {
	roleSet := make(map[string]bool, len(roles))
	for _, r := range roles {
		roleSet[r] = true
	}

	return func(c *gin.Context) {
		userRole, exists := c.Get("role")
		if !exists {
			c.JSON(http.StatusForbidden, gin.H{"error": "no role assigned"})
			c.Abort()
			return
		}

		role, ok := userRole.(string)
		if !ok || !roleSet[role] {
			c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireAdmin is a shorthand for RequireRole("admin")
func RequireAdmin() gin.HandlerFunc {
	return RequireRole("admin")
}
