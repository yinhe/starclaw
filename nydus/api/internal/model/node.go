package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// NydusNode represents a registered Claw node (≈ GitHub User).
// Each Claw node has an Ed25519 identity (node_id = "claw:xxxx").
type NydusNode struct {
	ID           string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	NodeID       string    `json:"node_id" gorm:"type:varchar(80);uniqueIndex;not null"` // claw:6aff1154...
	Name         string    `json:"name" gorm:"type:varchar(100)"`                        // human-readable name
	PublicKey    string    `json:"public_key" gorm:"type:text;not null"`                 // Ed25519 public key (base64)
	SSHPubKey    string    `json:"ssh_pub_key" gorm:"type:text"`                         // SSH public key for git push
	Role         string    `json:"role" gorm:"type:varchar(20);default:member"`           // owner / admin / member / readonly
	TeamID       string    `json:"team_id" gorm:"type:varchar(36);index"`                // Squad/TeamAgent instance ID
	Token        string    `json:"-" gorm:"type:varchar(64);uniqueIndex"`                // Bearer token for simplified auth
	LastSeenAt   *time.Time `json:"last_seen_at"`
	RegisteredAt time.Time  `json:"registered_at"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

func (n *NydusNode) BeforeCreate(tx *gorm.DB) error {
	if n.ID == "" {
		n.ID = uuid.New().String()
	}
	if n.RegisteredAt.IsZero() {
		n.RegisteredAt = time.Now()
	}
	return nil
}
