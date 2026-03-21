package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// TeamAgentTemplate defines a reusable team agent blueprint (e.g. DevClaw, MarketClaw).
type TeamAgentTemplate struct {
	ID          string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	Name        string    `json:"name" gorm:"type:varchar(200);not null"`          // "DevClaw"
	Category    string    `json:"category" gorm:"type:varchar(50);index"`          // development | marketing | support | data | ops | legal | ...
	Description string    `json:"description" gorm:"type:text"`
	Icon        string    `json:"icon" gorm:"type:varchar(50)"`                    // emoji or icon name
	Roles       string    `json:"roles" gorm:"type:json"`                          // []TeamRole JSON
	Topology    string    `json:"topology" gorm:"type:json"`                       // TopologyConfig JSON
	QualityGate string    `json:"quality_gate" gorm:"type:json"`                   // QualityGateConfig JSON
	Escalation  string    `json:"escalation" gorm:"type:json"`                     // EscalationConfig JSON
	IsOfficial  bool      `json:"is_official" gorm:"default:false"`                // official StarClaw template
	Version     string    `json:"version" gorm:"type:varchar(20);default:v1"`
	UserID      string    `json:"user_id" gorm:"type:varchar(36);index"`           // creator (empty for official)
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (t *TeamAgentTemplate) BeforeCreate(tx *gorm.DB) error {
	if t.ID == "" {
		t.ID = uuid.New().String()
	}
	return nil
}

// TeamInstance is a running team agent instance on a Claw node.
type TeamInstance struct {
	ID           string     `json:"id" gorm:"type:varchar(36);primaryKey"`
	TemplateID   string     `json:"template_id" gorm:"type:varchar(36);index;not null"`
	TemplateName string     `json:"template_name" gorm:"type:varchar(200)"`
	TeamID       string     `json:"team_id" gorm:"type:varchar(36);index"`              // enterprise team (tenant)
	ClawNodeID   string     `json:"claw_node_id" gorm:"type:varchar(36);index"`         // which Claw runs this
	UserID       string     `json:"user_id" gorm:"type:varchar(36);index"`
	Name         string     `json:"name" gorm:"type:varchar(200);not null"`              // "DevClaw-宠物电商"
	Goal         string     `json:"goal" gorm:"type:text"`                               // user's requirement
	Status       string     `json:"status" gorm:"type:varchar(20);default:forming;index"` // forming → ready → running → paused → maintenance → completed → disbanded
	RoleMap      string     `json:"role_map" gorm:"type:json"`                            // {role_code → agent_id}
	Config       string     `json:"config" gorm:"type:json"`                              // runtime config overrides

	// Budget
	EnergyBudget int `json:"energy_budget" gorm:"default:0"` // star energy budget
	EnergyUsed   int `json:"energy_used" gorm:"default:0"`   // consumed so far

	// Metrics
	MissionCount int     `json:"mission_count" gorm:"default:0"`
	AvgScore     float64 `json:"avg_score" gorm:"default:0"`

	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	DisbandedAt *time.Time `json:"disbanded_at"`
}

func (t *TeamInstance) BeforeCreate(tx *gorm.DB) error {
	if t.ID == "" {
		t.ID = uuid.New().String()
	}
	return nil
}

// TeamMission tracks a task dispatched to a team instance (mirrors Claw's Mission).
type TeamMission struct {
	ID             string     `json:"id" gorm:"type:varchar(36);primaryKey"`
	InstanceID     string     `json:"instance_id" gorm:"type:varchar(36);index;not null"`
	ClawMissionID  string     `json:"claw_mission_id" gorm:"type:varchar(36)"`              // ID on Claw side
	Title          string     `json:"title" gorm:"type:varchar(300)"`
	Goal           string     `json:"goal" gorm:"type:text"`
	Status         string     `json:"status" gorm:"type:varchar(20);default:planning;index"` // planning → confirming → executing → reviewing → completed → failed → cancelled
	SprintCount    int        `json:"sprint_count" gorm:"default:0"`
	TotalSteps     int        `json:"total_steps" gorm:"default:0"`
	DoneSteps      int        `json:"done_steps" gorm:"default:0"`
	ReviewScore    float64    `json:"review_score" gorm:"default:0"`
	EnergyUsed     int        `json:"energy_used" gorm:"default:0"`
	PreviewURL     string     `json:"preview_url" gorm:"type:varchar(500)"`
	Deliverables   string     `json:"deliverables" gorm:"type:json"` // delivery manifest
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	CompletedAt    *time.Time `json:"completed_at"`
}

func (m *TeamMission) BeforeCreate(tx *gorm.DB) error {
	if m.ID == "" {
		m.ID = uuid.New().String()
	}
	return nil
}
