package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Team represents an enterprise team/tenant within the Overlord
type Team struct {
	ID          string         `json:"id" gorm:"type:varchar(36);primaryKey"`
	Name        string         `json:"name" gorm:"type:varchar(100);uniqueIndex;not null"`
	DisplayName string         `json:"display_name" gorm:"type:varchar(200)"`
	MaxNodes    int            `json:"max_nodes" gorm:"default:0"`      // 0 = unlimited
	MaxTokens   int64          `json:"max_tokens" gorm:"default:0"`     // daily aggregate limit
	Status      string         `json:"status" gorm:"type:varchar(20);default:active"` // active, suspended
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`
}

func (t *Team) BeforeCreate(tx *gorm.DB) error {
	if t.ID == "" {
		t.ID = uuid.New().String()
	}
	return nil
}

// AdminUser represents an Overlord console administrator
type AdminUser struct {
	ID           string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	Username     string    `json:"username" gorm:"type:varchar(100);uniqueIndex;not null"`
	PasswordHash string    `json:"-" gorm:"type:varchar(255);not null"`
	Role         string    `json:"role" gorm:"type:varchar(20);default:viewer"` // superadmin, admin, operator, viewer
	TeamID       string    `json:"team_id" gorm:"type:varchar(36);index"`       // empty = global access
	Email        string    `json:"email" gorm:"type:varchar(255)"`
	LastLoginAt  *time.Time `json:"last_login_at"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

func (u *AdminUser) BeforeCreate(tx *gorm.DB) error {
	if u.ID == "" {
		u.ID = uuid.New().String()
	}
	return nil
}

// RolePermissions defines what each role can do
var RolePermissions = map[string][]string{
	"superadmin": {"*"},
	"admin":      {"claws.read", "claws.write", "claws.delete", "teams.read", "teams.write", "nydus.read", "nydus.write", "molt.read", "molt.write", "audit.read", "webhook.read", "webhook.write", "stats.read"},
	"operator":   {"claws.read", "claws.write", "nydus.read", "nydus.write", "molt.read", "molt.approve", "audit.read", "stats.read"},
	"viewer":     {"claws.read", "nydus.read", "molt.read", "audit.read", "stats.read"},
}

// HasPermission checks if a role has a specific permission
func HasPermission(role, perm string) bool {
	perms, ok := RolePermissions[role]
	if !ok {
		return false
	}
	for _, p := range perms {
		if p == "*" || p == perm {
			return true
		}
	}
	return false
}
