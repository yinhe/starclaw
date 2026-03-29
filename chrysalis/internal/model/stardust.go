package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// StardustAccount tracks a node's stardust balance (co-generated from star energy spending).
type StardustAccount struct {
	ID        string `json:"id" gorm:"type:varchar(36);primaryKey"`
	ClawID    string `json:"claw_id" gorm:"type:varchar(60);uniqueIndex;not null"`
	Balance   int64  `json:"balance" gorm:"default:0"`
	TotalIn   int64  `json:"total_in" gorm:"default:0"`
	TotalOut  int64  `json:"total_out" gorm:"default:0"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (s *StardustAccount) BeforeCreate(tx *gorm.DB) error {
	if s.ID == "" {
		s.ID = uuid.New().String()
	}
	return nil
}

// StardustTransaction records stardust income/spending.
type StardustTransaction struct {
	ID     string `json:"id" gorm:"type:varchar(36);primaryKey"`
	ClawID string `json:"claw_id" gorm:"type:varchar(60);index;not null"`
	Amount int64  `json:"amount"` // positive = earned, negative = spent
	Type   string `json:"type" gorm:"type:varchar(30)"`   // cogen / battle_reward / craft_spend / shop_spend
	Remark string `json:"remark" gorm:"type:varchar(200)"`
	CreatedAt time.Time `json:"created_at"`
}

func (t *StardustTransaction) BeforeCreate(tx *gorm.DB) error {
	if t.ID == "" {
		t.ID = uuid.New().String()
	}
	return nil
}

// CoGenRatio: spend N star energy → earn N/10 stardust
const CoGenRatio = 10
