package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════════
//  Team — 本地多 Agent 协作团队
//  区别于 Squad（跨 Claw 节点 P2P 组队），Team 是同一 Claw 内的 Agent 组队
// ════════════════════════════════════════════════════════════════

type Team struct {
	ID            string         `json:"id" gorm:"type:varchar(36);primaryKey"`
	Name          string         `json:"name" gorm:"type:varchar(200);not null"`
	Description   string         `json:"description" gorm:"type:text"`
	Icon          string         `json:"icon" gorm:"type:varchar(50);default:Swords"`
	UserID        string         `json:"user_id" gorm:"type:varchar(36);index;not null"`
	CoordinatorID string         `json:"coordinator_id" gorm:"type:varchar(36)"` // 团长 Agent ID
	Topology      string         `json:"topology" gorm:"type:varchar(20);default:sequential"` // sequential | parallel | round_robin | free
	Status        string         `json:"status" gorm:"type:varchar(20);default:active;index"` // active | paused
	TemplateID    string         `json:"template_id" gorm:"type:varchar(50)"`                 // 来源模板 ID
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `json:"-" gorm:"index"`

	Members     []TeamMember `json:"members,omitempty" gorm:"foreignKey:TeamID"`
	Coordinator *Agent       `json:"coordinator,omitempty" gorm:"foreignKey:CoordinatorID;references:ID"`
}

func (t *Team) BeforeCreate(tx *gorm.DB) error {
	if t.ID == "" {
		t.ID = uuid.New().String()
	}
	return nil
}

// ════════════════════════════════════════════════════════════════
//  TeamMember — 团队成员（本地 Agent）
// ════════════════════════════════════════════════════════════════

type TeamMember struct {
	ID        string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	TeamID    string    `json:"team_id" gorm:"type:varchar(36);index;not null"`
	AgentID   string    `json:"agent_id" gorm:"type:varchar(36);index;not null"`
	Role      string    `json:"role" gorm:"type:varchar(20);default:member"` // coordinator | member
	Specialty string    `json:"specialty" gorm:"type:varchar(100)"`          // 角色特长描述
	Order     int       `json:"order" gorm:"default:0"`                     // 执行顺序（sequential 模式）
	CreatedAt time.Time `json:"created_at"`

	Agent *Agent `json:"agent,omitempty" gorm:"foreignKey:AgentID;references:ID"`
}

func (m *TeamMember) BeforeCreate(tx *gorm.DB) error {
	if m.ID == "" {
		m.ID = uuid.New().String()
	}
	return nil
}
