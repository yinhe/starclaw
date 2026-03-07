package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AgentTemplate represents a shared agent template in the Creep marketplace
type AgentTemplate struct {
	ID           string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	AuthorID     string    `json:"author_id" gorm:"type:varchar(36);index;not null"`
	Name         string    `json:"name" gorm:"type:varchar(200);not null"`
	Description  string    `json:"description" gorm:"type:text"`
	Category     string    `json:"category" gorm:"type:varchar(50);index"` // assistant, coding, writing, data, creative, devops, research
	Tags         string    `json:"tags" gorm:"type:json"`                  // JSON array of tag strings
	SystemPrompt string    `json:"system_prompt" gorm:"type:longtext"`
	Tools        string    `json:"tools" gorm:"type:json"`
	Config       string    `json:"config" gorm:"type:json"`
	Icon         string    `json:"icon" gorm:"type:varchar(50)"`  // emoji or icon name
	Featured     bool      `json:"featured" gorm:"default:false"` // staff pick
	InstallCount int       `json:"install_count" gorm:"default:0"`
	Rating       float64   `json:"rating" gorm:"default:0"`
	RatingCount  int       `json:"rating_count" gorm:"default:0"`
	IsBuiltin    bool      `json:"is_builtin" gorm:"default:false"` // shipped with StarClaw
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`

	Author User `json:"author,omitempty" gorm:"foreignKey:AuthorID"`
}

func (t *AgentTemplate) BeforeCreate(tx *gorm.DB) error {
	if t.ID == "" {
		t.ID = uuid.New().String()
	}
	if t.Tools == "" {
		t.Tools = "[]"
	}
	if t.Config == "" {
		t.Config = "{}"
	}
	if t.Tags == "" {
		t.Tags = "[]"
	}
	return nil
}
