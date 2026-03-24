package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Agent struct {
	ID              string         `json:"id" gorm:"type:varchar(36);primaryKey"`
	UserID          string         `json:"user_id" gorm:"type:varchar(36);index;not null"`
	Name            string         `json:"name" gorm:"type:varchar(200);not null"`
	Description     string         `json:"description" gorm:"type:text"`
	SystemPrompt    string         `json:"system_prompt" gorm:"type:longtext"`
	ModelID         string         `json:"model_id" gorm:"type:varchar(36);default:null"`
	ModelName       string         `json:"model_name" gorm:"type:varchar(100)"`
	Tools           string         `json:"tools" gorm:"type:json"`                    // JSON array of tool names
	KnowledgeBaseID string         `json:"knowledge_base_id" gorm:"type:varchar(36)"` // optional RAG KB
	Config          string         `json:"config" gorm:"type:json"`                   // JSON config (temperature, max_tokens, etc.)
	IsPublic        bool           `json:"is_public" gorm:"default:false"`
	IsBuiltin       bool           `json:"is_builtin" gorm:"default:false"`
	SourceID        string         `json:"source_id" gorm:"type:varchar(36);index"`        // marketplace item ID if installed from Queen
	RoleCode        string         `json:"role_code" gorm:"type:varchar(50);index"`        // team role code (e.g. architect, drone, tester)
	TeamInstanceID  string         `json:"team_instance_id" gorm:"type:varchar(36);index"` // Overlord team instance ID
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `json:"-" gorm:"index"`

	User   User         `json:"user,omitempty" gorm:"foreignKey:UserID"`
	Model  ModelConfig  `json:"model,omitempty" gorm:"-"`
	Skills []AgentSkill `json:"skills,omitempty" gorm:"foreignKey:AgentID"`
}

// AgentSkill tracks a skill installed on a specific agent from Queen marketplace
type AgentSkill struct {
	ID          string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	AgentID     string    `json:"agent_id" gorm:"type:varchar(36);index;not null"`
	SkillID     string    `json:"skill_id" gorm:"type:varchar(36)"` // Queen marketplace item ID
	SkillName   string    `json:"skill_name" gorm:"type:varchar(100);not null"`
	SkillSpec   string    `json:"skill_spec" gorm:"type:longtext"` // JSON: function spec from marketplace
	Version     string    `json:"version" gorm:"type:varchar(20)"`
	InstalledAt time.Time `json:"installed_at"`
}

func (s *AgentSkill) BeforeCreate(tx *gorm.DB) error {
	if s.ID == "" {
		s.ID = uuid.New().String()
	}
	if s.InstalledAt.IsZero() {
		s.InstalledAt = time.Now()
	}
	return nil
}

func (a *Agent) BeforeCreate(tx *gorm.DB) error {
	if a.ID == "" {
		a.ID = uuid.New().String()
	}
	if a.Tools == "" {
		a.Tools = "[]"
	}
	if a.Config == "" {
		a.Config = "{}"
	}
	return nil
}
