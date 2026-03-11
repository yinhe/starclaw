package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw-router/internal/model"
	"gorm.io/gorm"
)

// RBACAuth loads the user's roles and permissions into the gin context.
// It requires at least one role assigned (i.e. the user is an admin-panel user).
// Falls back to is_admin=true for backward compatibility.
func RBACAuth(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("user_id")
		if userID == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
			return
		}

		// Load user roles
		var userRoles []model.UserRole
		db.Where("user_id = ?", userID).Find(&userRoles)

		// Backward compat: if no RBAC roles but is_admin=true, treat as super_admin
		if len(userRoles) == 0 {
			var user model.User
			if err := db.Where("id = ?", userID).First(&user).Error; err != nil {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
				return
			}
			if !user.IsAdmin {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "admin access required"})
				return
			}
			// is_admin=true → grant all permissions
			c.Set("permissions", []string{"*"})
			c.Set("roles", []string{"super_admin"})
			c.Next()
			return
		}

		// Collect role IDs
		roleIDs := make([]string, len(userRoles))
		for i, ur := range userRoles {
			roleIDs[i] = ur.RoleID
		}

		// Load role names
		var roles []model.Role
		db.Where("id IN ?", roleIDs).Find(&roles)
		roleNames := make([]string, len(roles))
		for i, r := range roles {
			roleNames[i] = r.Name
		}

		// Load permissions via role_permissions join
		var perms []model.Permission
		db.Joins("JOIN role_permissions ON role_permissions.permission_id = permissions.id").
			Where("role_permissions.role_id IN ?", roleIDs).
			Distinct().
			Find(&perms)

		permNames := make([]string, len(perms))
		for i, p := range perms {
			permNames[i] = p.Name
		}

		c.Set("roles", roleNames)
		c.Set("permissions", permNames)
		c.Next()
	}
}

// RequirePermission returns middleware that checks the user has a specific permission.
// Must be used after RBACAuth.
func RequirePermission(perm string) gin.HandlerFunc {
	return func(c *gin.Context) {
		perms, exists := c.Get("permissions")
		if !exists {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "no permissions loaded"})
			return
		}

		permList, ok := perms.([]string)
		if !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "invalid permissions"})
			return
		}

		for _, p := range permList {
			if p == "*" || p == perm {
				c.Next()
				return
			}
		}

		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"error":    "permission denied",
			"required": perm,
		})
	}
}
