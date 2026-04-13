package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Memory categories (L1 user profile + L2 skill experience)
const (
	MemCatPreference = "preference" // 用户偏好（语言、风格、工具习惯）
	MemCatFact       = "fact"       // 用户事实（姓名、职业、项目信息）
	MemCatContext    = "context"    // 上下文记忆（正在做的事情、近期目标）
	MemCatSkill      = "skill"      // 技能经验（成功的操作方法、工具使用经验）
	MemCatInstruct   = "instruct"   // 用户指令（"以后都用中文回答"、"不要加 emoji"）
	MemCatSummary    = "summary"    // 会话摘要（自动生成的对话总结）
)

// Memory scopes
const (
	MemScopeAgent  = "agent"  // 绑定特定 Agent
	MemScopeGlobal = "global" // 全局记忆，所有 Agent 可见
)

// Memory Palace rooms
const (
	MemRoomUser    = "user"
	MemRoomProject = "project"
	MemRoomTask    = "task"
	MemRoomSkill   = "skill"
	MemRoomOrg     = "org"
	MemRoomPolicy  = "policy"
)

// Memory represents a cross-session memory entry (Cerebrate L1/L2)
type Memory struct {
	ID             string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	UserID         string    `json:"user_id" gorm:"type:varchar(36);index;not null"`
	AgentID        string    `json:"agent_id" gorm:"type:varchar(36);index;not null"`
	Key            string    `json:"key" gorm:"type:varchar(200);not null"`
	Content        string    `json:"content" gorm:"type:text;not null"`
	Category       string    `json:"category" gorm:"type:varchar(50);index"`                  // preference, fact, context, skill, instruct, summary
	Source         string    `json:"source" gorm:"type:varchar(50)"`                          // auto_extract, user_explicit, system
	Scope          string    `json:"scope" gorm:"type:varchar(20);default:agent;index"`       // agent, global
	Room           string    `json:"room,omitempty" gorm:"type:varchar(50);index"`            // user, project, task, skill, org, policy
	Anchor         string    `json:"anchor,omitempty" gorm:"type:varchar(255);index"`         // stable entity anchor, e.g. project/starclaw
	Path           string    `json:"path,omitempty" gorm:"type:varchar(500)"`                 // palace path, e.g. user/default > project/starclaw
	ConversationID string    `json:"conversation_id,omitempty" gorm:"type:varchar(36);index"` // 来源会话
	Tags           string    `json:"tags,omitempty" gorm:"type:json"`                         // JSON array
	Importance     float64   `json:"importance" gorm:"default:0.5"`                           // 0.0 - 1.0
	Embedding      []byte    `json:"-" gorm:"type:longblob"`                                  // P4: vector embedding for semantic recall
	IsSeed         bool      `json:"is_seed" gorm:"default:false"`                            // Hexad: seed memory from marketplace install (never decays)
	AccessCount    int       `json:"access_count" gorm:"default:0"`
	LastAccessAt   time.Time `json:"last_access_at"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (m *Memory) BeforeCreate(tx *gorm.DB) error {
	if m.ID == "" {
		m.ID = uuid.New().String()
	}
	return nil
}
