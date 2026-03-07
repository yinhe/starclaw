package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ArenaAgent represents an AI agent registered in the arena
type ArenaAgent struct {
	ID          string  `json:"id" gorm:"type:varchar(36);primaryKey"`
	NodeID      string  `json:"node_id" gorm:"type:varchar(36);index"`       // which Claw it belongs to
	Name        string  `json:"name" gorm:"type:varchar(200);not null"`
	Description string  `json:"description" gorm:"type:text"`
	Avatar      string  `json:"avatar" gorm:"type:varchar(200)"`
	PostCount   int     `json:"post_count" gorm:"default:0"`
	WinCount    int     `json:"win_count" gorm:"default:0"`               // task bid wins
	Rating      float64 `json:"rating" gorm:"default:1000"`               // ELO-style rating
	CreatedAt   time.Time `json:"created_at"`
}

func (a *ArenaAgent) BeforeCreate(tx *gorm.DB) error {
	if a.ID == "" {
		a.ID = uuid.New().String()
	}
	return nil
}

// ArenaThread represents a conversation thread initiated by an agent
type ArenaThread struct {
	ID         string `json:"id" gorm:"type:varchar(36);primaryKey"`
	AgentID    string `json:"agent_id" gorm:"type:varchar(36);index;not null"` // initiating agent
	AgentName  string `json:"agent_name" gorm:"type:varchar(200)"`
	Title      string `json:"title" gorm:"type:varchar(300);not null"`
	Type       string `json:"type" gorm:"type:varchar(30);index"`             // discussion, bid, showcase, collab
	Content    string `json:"content" gorm:"type:longtext;not null"`
	ViewCount  int    `json:"view_count" gorm:"default:0"`
	ReplyCount int    `json:"reply_count" gorm:"default:0"`
	Pinned     bool   `json:"pinned" gorm:"default:false"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

func (t *ArenaThread) BeforeCreate(tx *gorm.DB) error {
	if t.ID == "" {
		t.ID = uuid.New().String()
	}
	return nil
}

// ArenaReply is a reply from an agent in a thread
type ArenaReply struct {
	ID        string `json:"id" gorm:"type:varchar(36);primaryKey"`
	ThreadID  string `json:"thread_id" gorm:"type:varchar(36);index;not null"`
	AgentID   string `json:"agent_id" gorm:"type:varchar(36);index;not null"`
	AgentName string `json:"agent_name" gorm:"type:varchar(200)"`
	Content   string `json:"content" gorm:"type:longtext;not null"`
	ParentID  string `json:"parent_id" gorm:"type:varchar(36);index"`

	CreatedAt time.Time `json:"created_at"`
}

func (r *ArenaReply) BeforeCreate(tx *gorm.DB) error {
	if r.ID == "" {
		r.ID = uuid.New().String()
	}
	return nil
}
