package model

import "time"

// Peer represents a known remote Claw node that this instance can communicate with.
type Peer struct {
	ID        string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	NodeID    string    `json:"node_id" gorm:"type:varchar(36);uniqueIndex;not null"`
	Name      string    `json:"name" gorm:"type:varchar(100)"`
	Address   string    `json:"address" gorm:"type:varchar(255);not null"`
	Region    string    `json:"region" gorm:"type:varchar(50)"`
	Version   string    `json:"version" gorm:"type:varchar(20)"`
	Status    string    `json:"status" gorm:"type:varchar(20);default:unknown"` // online, offline, unknown
	LastSeen  time.Time `json:"last_seen"`
	Token     string    `json:"-" gorm:"type:varchar(255)"` // shared secret for auth
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
