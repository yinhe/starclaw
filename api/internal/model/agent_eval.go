package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AgentTestCase represents a test case for evaluating an agent
type AgentTestCase struct {
	ID             string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	UserID         string    `json:"user_id" gorm:"type:varchar(36);index;not null"`
	AgentID        string    `json:"agent_id" gorm:"type:varchar(36);index;not null"`
	Name           string    `json:"name" gorm:"type:varchar(200);not null"`
	Input          string    `json:"input" gorm:"type:text;not null"`
	ExpectedOutput string    `json:"expected_output" gorm:"type:text"`
	Tags           string    `json:"tags" gorm:"type:varchar(500)"`
	CreatedAt      time.Time `json:"created_at"`
}

func (t *AgentTestCase) BeforeCreate(tx *gorm.DB) error {
	if t.ID == "" {
		t.ID = uuid.New().String()
	}
	return nil
}

// AgentTestRun represents a single execution of a test case
type AgentTestRun struct {
	ID           string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	TestCaseID   string    `json:"test_case_id" gorm:"type:varchar(36);index;not null"`
	AgentID      string    `json:"agent_id" gorm:"type:varchar(36);index;not null"`
	UserID       string    `json:"user_id" gorm:"type:varchar(36);index;not null"`
	ActualOutput string    `json:"actual_output" gorm:"type:text"`
	Score        float64   `json:"score" gorm:"default:0"`       // 0.0 - 1.0
	DurationMs   int64     `json:"duration_ms" gorm:"default:0"`
	TokensUsed   int       `json:"tokens_used" gorm:"default:0"`
	Status       string    `json:"status" gorm:"type:varchar(20);default:pending"` // pending, passed, failed, error
	ErrorMsg     string    `json:"error_msg" gorm:"type:text"`
	CreatedAt    time.Time `json:"created_at"`
}

func (r *AgentTestRun) BeforeCreate(tx *gorm.DB) error {
	if r.ID == "" {
		r.ID = uuid.New().String()
	}
	return nil
}
