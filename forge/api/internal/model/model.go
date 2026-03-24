package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════════
//  Forge Data Models — AI-Native Project Management
// ════════════════════════════════════════════════════════════════

// ForgeProject represents a project (monorepo-level or team-level).
type ForgeProject struct {
	ID          string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	Name        string    `json:"name" gorm:"type:varchar(200);not null"`
	Key         string    `json:"key" gorm:"type:varchar(10);uniqueIndex;not null"` // Issue prefix: SC-1, SC-2
	Description string    `json:"description" gorm:"type:text"`
	OwnerType   string    `json:"owner_type" gorm:"type:varchar(20);default:monorepo"` // monorepo / team / personal
	OwnerID     string    `json:"owner_id" gorm:"type:varchar(36);index"`
	NydusRepo   string    `json:"nydus_repo" gorm:"type:varchar(100)"`
	Status      string    `json:"status" gorm:"type:varchar(20);default:active"` // active / archived
	Tags        string    `json:"tags" gorm:"type:varchar(500)"`
	IssueSeq    int       `json:"issue_seq" gorm:"default:0"` // auto-increment for issue numbers
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (p *ForgeProject) BeforeCreate(tx *gorm.DB) error {
	if p.ID == "" {
		p.ID = uuid.New().String()
	}
	return nil
}

// ForgeIssue represents a work item (task/bug/feature/epic).
type ForgeIssue struct {
	ID             string     `json:"id" gorm:"type:varchar(36);primaryKey"`
	ProjectID      string     `json:"project_id" gorm:"type:varchar(36);index;not null"`
	Number         int        `json:"number" gorm:"index"`
	Key            string     `json:"key" gorm:"type:varchar(20);index"` // "SC-42"
	Title          string     `json:"title" gorm:"type:varchar(500);not null"`
	Body           string     `json:"body" gorm:"type:text"`
	Type           string     `json:"type" gorm:"type:varchar(20);default:task"`       // epic / story / task / bug / improvement
	Priority       string     `json:"priority" gorm:"type:varchar(20);default:medium"` // critical / high / medium / low
	Status         string     `json:"status" gorm:"type:varchar(20);default:backlog;index"`
	Assignee       string     `json:"assignee" gorm:"type:varchar(100);index"`
	Reporter       string     `json:"reporter" gorm:"type:varchar(100)"`
	Service        string     `json:"service" gorm:"type:varchar(50);index"` // claw/api, queen/api, etc.
	TaskType       string     `json:"task_type" gorm:"type:varchar(20)"`     // code / agent / config / doc / design / review
	SprintID       string     `json:"sprint_id" gorm:"type:varchar(36);index"`
	MilestoneID    string     `json:"milestone_id" gorm:"type:varchar(36);index"`
	EpicID         string     `json:"epic_id" gorm:"type:varchar(36);index"` // parent epic
	PRDID          string     `json:"prd_id" gorm:"type:varchar(36);index"`
	Labels         string     `json:"labels" gorm:"type:varchar(500)"`
	StoryPoints    int        `json:"story_points" gorm:"default:0"`
	Branch         string     `json:"branch" gorm:"type:varchar(200)"`
	PRNumber       int        `json:"pr_number" gorm:"default:0"`
	DevBridgeTask  string     `json:"devbridge_task" gorm:"type:varchar(20)"`
	DependsOn      string     `json:"depends_on" gorm:"type:varchar(500)"` // JSON: ["issue-id-1","issue-id-2"]
	Position       int        `json:"position" gorm:"default:0"`           // board ordering
	DueDate        *time.Time `json:"due_date"`
	ClosedAt       *time.Time `json:"closed_at"`
	DispatchedAt   *time.Time `json:"dispatched_at"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

func (i *ForgeIssue) BeforeCreate(tx *gorm.DB) error {
	if i.ID == "" {
		i.ID = uuid.New().String()
	}
	return nil
}

// ForgeSprint represents a development iteration.
type ForgeSprint struct {
	ID        string     `json:"id" gorm:"type:varchar(36);primaryKey"`
	ProjectID string     `json:"project_id" gorm:"type:varchar(36);index;not null"`
	PRDID     string     `json:"prd_id" gorm:"type:varchar(36);index"`
	Name      string     `json:"name" gorm:"type:varchar(200);not null"`
	Goal      string     `json:"goal" gorm:"type:text"`
	Status    string     `json:"status" gorm:"type:varchar(20);default:planned"` // planned / active / completed
	SeqNum    int        `json:"seq_num" gorm:"default:1"`                       // Sprint 1, 2, 3...
	StartDate *time.Time `json:"start_date"`
	EndDate   *time.Time `json:"end_date"`
	Velocity  int        `json:"velocity" gorm:"default:0"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

func (s *ForgeSprint) BeforeCreate(tx *gorm.DB) error {
	if s.ID == "" {
		s.ID = uuid.New().String()
	}
	return nil
}

// ForgeMilestone represents a release milestone.
type ForgeMilestone struct {
	ID          string     `json:"id" gorm:"type:varchar(36);primaryKey"`
	ProjectID   string     `json:"project_id" gorm:"type:varchar(36);index;not null"`
	Title       string     `json:"title" gorm:"type:varchar(200);not null"`
	Description string     `json:"description" gorm:"type:text"`
	DueDate     *time.Time `json:"due_date"`
	Status      string     `json:"status" gorm:"type:varchar(20);default:open"` // open / closed
	Progress    int        `json:"progress" gorm:"default:0"`
	CreatedAt   time.Time  `json:"created_at"`
}

func (m *ForgeMilestone) BeforeCreate(tx *gorm.DB) error {
	if m.ID == "" {
		m.ID = uuid.New().String()
	}
	return nil
}

// ForgeActivity is a global activity log entry.
type ForgeActivity struct {
	ID        string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	ProjectID string    `json:"project_id" gorm:"type:varchar(36);index"`
	IssueID   string    `json:"issue_id" gorm:"type:varchar(36);index"`
	Type      string    `json:"type" gorm:"type:varchar(20);index"` // commit / pr / issue / deploy / ci / devbridge / devclaw
	Actor     string    `json:"actor" gorm:"type:varchar(100)"`
	Summary   string    `json:"summary" gorm:"type:varchar(500)"`
	Detail    string    `json:"detail" gorm:"type:text"`
	Service   string    `json:"service" gorm:"type:varchar(50)"`
	Source    string    `json:"source" gorm:"type:varchar(20)"` // nydus / github / devbridge / overlord / forge
	CreatedAt time.Time `json:"created_at"`
}

func (a *ForgeActivity) BeforeCreate(tx *gorm.DB) error {
	if a.ID == "" {
		a.ID = uuid.New().String()
	}
	return nil
}

// ForgePRD represents a Product Requirements Document generated by AI.
type ForgePRD struct {
	ID                 string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	ProjectID          string    `json:"project_id" gorm:"type:varchar(36);index;not null"`
	Prompt             string    `json:"prompt" gorm:"type:text;not null"`
	Title              string    `json:"title" gorm:"type:varchar(200)"`
	Objective          string    `json:"objective" gorm:"type:text"`
	Features           string    `json:"features" gorm:"type:text"`           // JSON array
	NonFunctional      string    `json:"non_functional" gorm:"type:text"`     // JSON array
	AcceptanceCriteria string    `json:"acceptance_criteria" gorm:"type:text"` // JSON array
	Services           string    `json:"services" gorm:"type:varchar(500)"`   // JSON array
	EstimatedSprints   int       `json:"estimated_sprints" gorm:"default:1"`
	Status             string    `json:"status" gorm:"type:varchar(20);default:draft"` // draft / confirmed / planned / executing / done
	RawLLMResponse     string    `json:"raw_llm_response" gorm:"type:text"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

func (p *ForgePRD) BeforeCreate(tx *gorm.DB) error {
	if p.ID == "" {
		p.ID = uuid.New().String()
	}
	return nil
}

// ForgeIssueComment is a comment on an issue.
type ForgeIssueComment struct {
	ID        string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	IssueID   string    `json:"issue_id" gorm:"type:varchar(36);index;not null"`
	Author    string    `json:"author" gorm:"type:varchar(100)"`
	Body      string    `json:"body" gorm:"type:text;not null"`
	IsAI      bool      `json:"is_ai" gorm:"default:false"`
	CreatedAt time.Time `json:"created_at"`
}

func (c *ForgeIssueComment) BeforeCreate(tx *gorm.DB) error {
	if c.ID == "" {
		c.ID = uuid.New().String()
	}
	return nil
}

// ForgeAgent tracks registered editor sessions (Windsurf/Cursor/VS Code).
type ForgeAgent struct {
	ID           string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	Name         string    `json:"name" gorm:"type:varchar(50);uniqueIndex;not null"` // windsurf-1, cursor-1
	Type         string    `json:"type" gorm:"type:varchar(20)"`                      // windsurf / cursor / vscode / devclaw
	Capabilities string    `json:"capabilities" gorm:"type:varchar(500)"`             // JSON: ["go","react"]
	Services     string    `json:"services" gorm:"type:varchar(500)"`                 // JSON: ["claw/api","claw/web"]
	Status       string    `json:"status" gorm:"type:varchar(20);default:idle"`       // idle / busy / offline
	CurrentIssue string    `json:"current_issue" gorm:"type:varchar(36)"`
	LastSeenAt   time.Time `json:"last_seen_at"`
	RegisteredAt time.Time `json:"registered_at"`
}

func (a *ForgeAgent) BeforeCreate(tx *gorm.DB) error {
	if a.ID == "" {
		a.ID = uuid.New().String()
	}
	return nil
}

// AllModels returns all models for AutoMigrate.
func AllModels() []interface{} {
	return []interface{}{
		&ForgeProject{},
		&ForgeIssue{},
		&ForgeSprint{},
		&ForgeMilestone{},
		&ForgeActivity{},
		&ForgePRD{},
		&ForgeIssueComment{},
		&ForgeAgent{},
	}
}
