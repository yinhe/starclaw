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
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `json:"-" gorm:"index"`

	User  User        `json:"user,omitempty" gorm:"foreignKey:UserID"`
	Model ModelConfig `json:"model,omitempty" gorm:"-"`
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
