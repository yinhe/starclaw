package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ComplianceLog records compliance-relevant events (data access, export, sensitive content, etc.)
type ComplianceLog struct {
	ID        string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	TeamID    string    `json:"team_id" gorm:"type:varchar(36);index"`
	Actor     string    `json:"actor" gorm:"type:varchar(200)"`       // who triggered the event
	EventType string    `json:"event_type" gorm:"type:varchar(50);index;not null"` // data_access, data_export, sensitive_word, policy_violation, audit_export
	Severity  string    `json:"severity" gorm:"type:varchar(20);default:info"`     // info, warning, critical
	Resource  string    `json:"resource" gorm:"type:varchar(255)"`    // what was accessed/affected
	Detail    string    `json:"detail" gorm:"type:text"`              // JSON or free-form detail
	IPAddress string    `json:"ip_address" gorm:"type:varchar(50)"`
	Resolved  bool      `json:"resolved" gorm:"default:false"`
	CreatedAt time.Time `json:"created_at" gorm:"index"`
}

func (c *ComplianceLog) BeforeCreate(tx *gorm.DB) error {
	if c.ID == "" {
		c.ID = uuid.New().String()
	}
	return nil
}

// SensitiveWordRule defines words/patterns that trigger compliance alerts when detected in AI input/output
type SensitiveWordRule struct {
	ID        string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	Word      string    `json:"word" gorm:"type:varchar(200);not null;index"`
	Category  string    `json:"category" gorm:"type:varchar(50)"` // pii, financial, medical, political, custom
	Action    string    `json:"action" gorm:"type:varchar(20);default:log"` // log, block, mask
	Enabled   bool      `json:"enabled" gorm:"default:true"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (s *SensitiveWordRule) BeforeCreate(tx *gorm.DB) error {
	if s.ID == "" {
		s.ID = uuid.New().String()
	}
	return nil
}

// DataFlowRecord documents where data flows (for compliance data-flow diagrams)
type DataFlowRecord struct {
	ID          string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	Source      string    `json:"source" gorm:"type:varchar(200);not null"`      // e.g. "user_input", "knowledge_base"
	Destination string    `json:"destination" gorm:"type:varchar(200);not null"` // e.g. "openai_api", "local_model", "database"
	DataType    string    `json:"data_type" gorm:"type:varchar(100)"`            // e.g. "prompt", "document", "pii"
	Encryption  string    `json:"encryption" gorm:"type:varchar(50)"`            // e.g. "tls_1.3", "aes_256", "none"
	Region      string    `json:"region" gorm:"type:varchar(100)"`               // e.g. "cn-east", "us-west", "local"
	CrossBorder bool      `json:"cross_border" gorm:"default:false"`             // data crosses national border
	Description string    `json:"description" gorm:"type:varchar(500)"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (d *DataFlowRecord) BeforeCreate(tx *gorm.DB) error {
	if d.ID == "" {
		d.ID = uuid.New().String()
	}
	return nil
}
