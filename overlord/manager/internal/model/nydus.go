package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// NydusTunnel represents a managed tunnel between Overlord and a Claw node
type NydusTunnel struct {
	ID         string `json:"id" gorm:"type:varchar(36);primaryKey"`
	ClawNodeID string `json:"claw_node_id" gorm:"type:varchar(36);index;not null"`
	ClawName   string `json:"claw_name" gorm:"type:varchar(200)"`
	Team       string `json:"team" gorm:"type:varchar(100);index"`

	// Tunnel config
	LocalPort  int    `json:"local_port" gorm:"not null"`                             // port on Overlord side
	RemotePort int    `json:"remote_port" gorm:"not null"`                            // port on Claw side
	Protocol   string `json:"protocol" gorm:"type:varchar(10);default:tcp"`           // tcp, udp
	Mode       string `json:"mode" gorm:"type:varchar(20);default:forward"`           // forward, reverse
	Status     string `json:"status" gorm:"type:varchar(20);default:pending;index"`   // pending, active, error, closed

	// Metrics
	BytesIn     int64  `json:"bytes_in" gorm:"default:0"`
	BytesOut    int64  `json:"bytes_out" gorm:"default:0"`
	Connections int    `json:"connections" gorm:"default:0"`
	LastError   string `json:"last_error" gorm:"type:text"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

func (t *NydusTunnel) BeforeCreate(tx *gorm.DB) error {
	if t.ID == "" {
		t.ID = uuid.New().String()
	}
	return nil
}
