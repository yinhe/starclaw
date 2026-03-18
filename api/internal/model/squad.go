package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════════
//  Squad — 战队：多个 Claw 节点组成的协作团队
// ════════════════════════════════════════════════════════════════

type Squad struct {
	ID          string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	Name        string    `json:"name" gorm:"type:varchar(200);not null"`
	Description string    `json:"description" gorm:"type:text"`
	CaptainNode string    `json:"captain_node" gorm:"type:varchar(50);index;not null"` // claw:xxx
	UserID      string    `json:"user_id" gorm:"type:varchar(36);index;not null"`
	Status      string    `json:"status" gorm:"type:varchar(20);default:forming;index"` // forming / active / disbanded
	MaxMembers  int       `json:"max_members" gorm:"default:10"`
	IsPublic    bool      `json:"is_public" gorm:"default:false"`
	Tags        string    `json:"tags" gorm:"type:json"` // ["dev","design","video"]
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (s *Squad) BeforeCreate(tx *gorm.DB) error {
	if s.ID == "" {
		s.ID = uuid.New().String()
	}
	return nil
}

// ════════════════════════════════════════════════════════════════
//  SquadMember — 战队成员
// ════════════════════════════════════════════════════════════════

type SquadMember struct {
	ID          string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	SquadID     string    `json:"squad_id" gorm:"type:varchar(36);index;not null"`
	NodeID      string    `json:"node_id" gorm:"type:varchar(50);index;not null"` // claw:xxx
	PeerID      string    `json:"peer_id" gorm:"type:varchar(36)"`                // local Peer table ID
	Role        string    `json:"role" gorm:"type:varchar(20);default:member"`    // captain / member
	Specialty   string    `json:"specialty" gorm:"type:varchar(50)"`              // coding / design / video / sales
	AgentExport string    `json:"agent_export" gorm:"type:json"`                  // exported agent summaries
	Status      string    `json:"status" gorm:"type:varchar(20);default:offline"` // online / offline / busy
	JoinedAt    time.Time `json:"joined_at"`
}

func (m *SquadMember) BeforeCreate(tx *gorm.DB) error {
	if m.ID == "" {
		m.ID = uuid.New().String()
	}
	return nil
}

// ════════════════════════════════════════════════════════════════
//  Mission — 战队任务
// ════════════════════════════════════════════════════════════════

type Mission struct {
	ID            string     `json:"id" gorm:"type:varchar(36);primaryKey"`
	SquadID       string     `json:"squad_id" gorm:"type:varchar(36);index;not null"`
	Title         string     `json:"title" gorm:"type:varchar(500);not null"`
	Goal          string     `json:"goal" gorm:"type:text;not null"`
	Status        string     `json:"status" gorm:"type:varchar(20);default:planning;index"` // planning / executing / reviewing / completed / failed
	CaptainNode   string     `json:"captain_node" gorm:"type:varchar(50)"`
	Plan          string     `json:"plan" gorm:"type:json"`
	FinalResult   string     `json:"final_result" gorm:"type:longtext"`
	TotalSteps    int        `json:"total_steps" gorm:"default:0"`
	DoneSteps     int        `json:"done_steps" gorm:"default:0"`
	UserID        string     `json:"user_id" gorm:"type:varchar(36);index"`
	RepoPath      string     `json:"repo_path" gorm:"type:varchar(500)"`      // Git bare repo 路径
	WorkspacePath string     `json:"workspace_path" gorm:"type:varchar(500)"` // 工作目录路径
	CurrentSprint int        `json:"current_sprint" gorm:"default:0"`         // 当前 Sprint 序号
	MaxSprints    int        `json:"max_sprints" gorm:"default:4"`            // 最大 Sprint 数
	PreviewURL    string     `json:"preview_url" gorm:"type:varchar(500)"`    // 最新预览 URL
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	CompletedAt   *time.Time `json:"completed_at"`
}

func (m *Mission) BeforeCreate(tx *gorm.DB) error {
	if m.ID == "" {
		m.ID = uuid.New().String()
	}
	return nil
}

// ════════════════════════════════════════════════════════════════
//  Sprint — 敏捷迭代：Mission 的一轮开发周期
// ════════════════════════════════════════════════════════════════

type Sprint struct {
	ID           string     `json:"id" gorm:"type:varchar(36);primaryKey"`
	MissionID    string     `json:"mission_id" gorm:"type:varchar(36);index;not null"`
	Number       int        `json:"number" gorm:"default:0"`                              // Sprint 序号: 0, 1, 2...
	Goal         string     `json:"goal" gorm:"type:text;not null"`                       // Sprint 目标
	Status       string     `json:"status" gorm:"type:varchar(20);default:pending;index"` // pending / planning / executing / reviewing / done / failed
	TotalSteps   int        `json:"total_steps" gorm:"default:0"`
	DoneSteps    int        `json:"done_steps" gorm:"default:0"`
	PreviewURL   string     `json:"preview_url" gorm:"type:varchar(500)"`
	ReviewNotes  string     `json:"review_notes" gorm:"type:longtext"`  // Captain LLM 审查笔记
	UserFeedback string     `json:"user_feedback" gorm:"type:longtext"` // 用户反馈
	StartedAt    *time.Time `json:"started_at"`
	CompletedAt  *time.Time `json:"completed_at"`
	CreatedAt    time.Time  `json:"created_at"`
}

func (s *Sprint) BeforeCreate(tx *gorm.DB) error {
	if s.ID == "" {
		s.ID = uuid.New().String()
	}
	return nil
}

// ════════════════════════════════════════════════════════════════
//  MissionStep — 任务步骤（委派给具体节点执行）
// ════════════════════════════════════════════════════════════════

type MissionStep struct {
	ID           string     `json:"id" gorm:"type:varchar(36);primaryKey"`
	MissionID    string     `json:"mission_id" gorm:"type:varchar(36);index;not null"`
	SprintID     string     `json:"sprint_id" gorm:"type:varchar(36);index"` // 所属 Sprint
	TargetNode   string     `json:"target_node" gorm:"type:varchar(50)"`     // claw:xxx
	TargetAgent  string     `json:"target_agent" gorm:"type:varchar(200)"`   // agent name
	Task         string     `json:"task" gorm:"type:text;not null"`
	Input        string     `json:"input" gorm:"type:longtext"`                     // upstream output as context
	Output       string     `json:"output" gorm:"type:longtext"`                    // execution result
	Status       string     `json:"status" gorm:"type:varchar(20);default:pending"` // pending / dispatched / running / done / failed
	ErrorMsg     string     `json:"error_msg" gorm:"type:text"`
	DependsOn    string     `json:"depends_on" gorm:"type:json"` // ["step-id-1"]
	Sequence     int        `json:"sequence" gorm:"default:0"`
	Branch       string     `json:"branch" gorm:"type:varchar(200)"`     // Git 分支名
	CommitHash   string     `json:"commit_hash" gorm:"type:varchar(64)"` // 最终提交 hash
	DispatchedAt *time.Time `json:"dispatched_at"`
	CompletedAt  *time.Time `json:"completed_at"`
	CreatedAt    time.Time  `json:"created_at"`
}

func (s *MissionStep) BeforeCreate(tx *gorm.DB) error {
	if s.ID == "" {
		s.ID = uuid.New().String()
	}
	return nil
}

// ════════════════════════════════════════════════════════════════
//  StepReview — Code Review 记录
// ════════════════════════════════════════════════════════════════

type StepReview struct {
	ID           string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	StepID       string    `json:"step_id" gorm:"type:varchar(36);index;not null"`
	ReviewerNode string    `json:"reviewer_node" gorm:"type:varchar(50)"`          // 审查者节点 ID
	Status       string    `json:"status" gorm:"type:varchar(20);default:pending"` // pending / approved / changes_requested
	Comments     string    `json:"comments" gorm:"type:longtext"`
	DiffSummary  string    `json:"diff_summary" gorm:"type:longtext"`
	CreatedAt    time.Time `json:"created_at"`
}

func (r *StepReview) BeforeCreate(tx *gorm.DB) error {
	if r.ID == "" {
		r.ID = uuid.New().String()
	}
	return nil
}
