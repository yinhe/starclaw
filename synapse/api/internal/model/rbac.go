package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Role represents an RBAC role (e.g. super_admin, admin, operator, viewer)
type Role struct {
	ID          string       `json:"id" gorm:"type:varchar(36);primaryKey"`
	Name        string       `json:"name" gorm:"type:varchar(50);uniqueIndex"`
	Description string       `json:"description" gorm:"type:varchar(200)"`
	Permissions []Permission `json:"permissions,omitempty" gorm:"many2many:role_permissions;"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

func (r *Role) BeforeCreate(tx *gorm.DB) error {
	if r.ID == "" {
		r.ID = uuid.New().String()
	}
	return nil
}

// Permission represents a granular permission (e.g. manage_users, view_logs)
type Permission struct {
	ID          string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	Name        string    `json:"name" gorm:"type:varchar(50);uniqueIndex"`
	Description string    `json:"description" gorm:"type:varchar(200)"`
	CreatedAt   time.Time `json:"created_at"`
}

func (p *Permission) BeforeCreate(tx *gorm.DB) error {
	if p.ID == "" {
		p.ID = uuid.New().String()
	}
	return nil
}

// UserRole links a user to a role (many-to-many)
type UserRole struct {
	UserID    string    `json:"user_id" gorm:"type:varchar(36);primaryKey"`
	RoleID    string    `json:"role_id" gorm:"type:varchar(36);primaryKey"`
	CreatedAt time.Time `json:"created_at"`
}

// ── Default data ──

// DefaultPermissions returns the built-in permission set
func DefaultPermissions() []Permission {
	return []Permission{
		{Name: "view_overview", Description: "查看总览统计"},
		{Name: "view_users", Description: "查看用户列表"},
		{Name: "manage_users", Description: "编辑/禁用用户"},
		{Name: "view_logs", Description: "查看请求日志"},
		{Name: "view_orders", Description: "查看支付订单"},
		{Name: "manage_orders", Description: "修改订单状态"},
		{Name: "view_providers", Description: "查看模型/Provider"},
		{Name: "manage_providers", Description: "管理模型/Provider"},
		{Name: "manage_roles", Description: "管理角色与权限分配"},
		{Name: "manage_system", Description: "系统设置"},
	}
}

// DefaultRoles returns the built-in role set with their permission names
func DefaultRoles() map[string]struct {
	Description string
	Permissions []string
} {
	return map[string]struct {
		Description string
		Permissions []string
	}{
		"super_admin": {
			Description: "超级管理员 — 拥有全部权限",
			Permissions: []string{
				"view_overview", "view_users", "manage_users",
				"view_logs", "view_orders", "manage_orders",
				"view_providers", "manage_providers",
				"manage_roles", "manage_system",
			},
		},
		"admin": {
			Description: "管理员 — 日常管理（不可分配角色和系统设置）",
			Permissions: []string{
				"view_overview", "view_users", "manage_users",
				"view_logs", "view_orders", "manage_orders",
				"view_providers", "manage_providers",
			},
		},
		"operator": {
			Description: "运营 — 只读 + 订单管理",
			Permissions: []string{
				"view_overview", "view_users",
				"view_logs", "view_orders", "manage_orders",
				"view_providers",
			},
		},
		"viewer": {
			Description: "观察者 — 只读",
			Permissions: []string{
				"view_overview", "view_users",
				"view_logs", "view_orders",
				"view_providers",
			},
		},
	}
}
