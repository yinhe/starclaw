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
	Gene            string         `json:"gene" gorm:"type:json"`                     // Hexad: clean gene definition (name+prompt+model+temp)
	IsPublic        bool           `json:"is_public" gorm:"default:false"`
	IsBuiltin       bool           `json:"is_builtin" gorm:"default:false"`
	SourceID        string         `json:"source_id" gorm:"type:varchar(36);index"`        // marketplace item ID if installed from Queen
	SourceVersion   string         `json:"source_version" gorm:"type:varchar(20)"`         // installed bundle version
	ManifestID      string         `json:"manifest_id" gorm:"type:varchar(100);index"`     // manifest ID from discovery (e.g. "devclaw", "douyin_viral/director")
	BridgePort      int            `json:"bridge_port" gorm:"default:0"`                   // MCP bridge port if agent has a bridge
	RoleCode        string         `json:"role_code" gorm:"type:varchar(50);index"`        // team role code (e.g. architect, drone, tester)
	TeamInstanceID  string         `json:"team_instance_id" gorm:"type:varchar(36);index"` // Overlord team instance ID
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `json:"-" gorm:"index"`

	User   User         `json:"user,omitempty" gorm:"foreignKey:UserID"`
	Model  ModelConfig  `json:"model,omitempty" gorm:"-"`
	Skills []AgentSkill `json:"skills,omitempty" gorm:"foreignKey:AgentID"`
}

// AgentSkill tracks a skill or instinct installed on a specific agent.
// Hexad: ability_type='skill' (user asks→do) or 'instinct' (auto→do)
type AgentSkill struct {
	ID               string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	AgentID          string    `json:"agent_id" gorm:"type:varchar(36);index;not null"`
	SkillID          string    `json:"skill_id" gorm:"type:varchar(36)"` // Queen marketplace item ID
	SkillName        string    `json:"skill_name" gorm:"type:varchar(100);not null"`
	SkillSpec        string    `json:"skill_spec" gorm:"type:longtext"` // JSON spec
	Version          string    `json:"version" gorm:"type:varchar(20)"`
	AbilityType      string    `json:"ability_type" gorm:"type:varchar(20);default:skill;index"` // skill | instinct
	InstinctCategory string    `json:"instinct_category" gorm:"type:varchar(20)"`                // care/time/monitor/event
	Enabled          bool      `json:"enabled" gorm:"default:true"`
	InstalledAt      time.Time `json:"installed_at"`
}

// AgentGland stores runtime configuration for an agent (Heptad: 腺体).
// Examples: API keys, account credentials, thresholds, feature toggles.
// Credential values are encrypted at rest.
type AgentGland struct {
	ID        string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	AgentID   string    `json:"agent_id" gorm:"type:varchar(36);index;not null"`
	UserID    string    `json:"user_id" gorm:"type:varchar(36);index;not null"`
	Key       string    `json:"key" gorm:"type:varchar(100);not null"`            // e.g. "qmt_account", "sip_server"
	Value     string    `json:"value" gorm:"type:text"`                           // plaintext or encrypted
	Category  string    `json:"category" gorm:"type:varchar(20);default:general"` // credential | threshold | toggle | endpoint | general
	Encrypted bool      `json:"encrypted" gorm:"default:false"`
	Required  bool      `json:"required" gorm:"default:false"`
	Label     string    `json:"label" gorm:"type:varchar(100)"`     // display name, e.g. "QMT 账号"
	HelpText  string    `json:"help_text" gorm:"type:varchar(500)"` // tooltip / placeholder
	SortOrder int       `json:"sort_order" gorm:"default:0"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (g *AgentGland) BeforeCreate(tx *gorm.DB) error {
	if g.ID == "" {
		g.ID = uuid.New().String()
	}
	return nil
}

// AgentMCPBinding links an Agent to an MCP server (Hexad: 外接)
type AgentMCPBinding struct {
	ID        string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	AgentID   string    `json:"agent_id" gorm:"type:varchar(36);index;not null"`
	MCPID     string    `json:"mcp_id" gorm:"type:varchar(36);index;not null"`
	CreatedAt time.Time `json:"created_at"`
}

func (b *AgentMCPBinding) BeforeCreate(tx *gorm.DB) error {
	if b.ID == "" {
		b.ID = uuid.New().String()
	}
	return nil
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
	if a.Gene == "" {
		a.Gene = "{}"
	}
	return nil
}
