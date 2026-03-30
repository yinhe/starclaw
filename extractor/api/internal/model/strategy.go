package model

import (
	"time"

	"gorm.io/datatypes"
)

type Strategy struct {
	ID          string         `json:"id" gorm:"primaryKey;size:36"`
	Name        string         `json:"name" gorm:"size:100;not null"`
	Type        string         `json:"type" gorm:"size:30;index"` // grid, momentum, mean_revert, trend, pair, ai_signal
	Description string         `json:"description" gorm:"size:500"`
	ScriptPath  string         `json:"script_path" gorm:"size:255"` // Python strategy file path
	Params      datatypes.JSON `json:"params"`                      // strategy-specific params
	Accounts    datatypes.JSON `json:"accounts"`                    // bound QMT accounts []string
	Status      string         `json:"status" gorm:"size:20;default:stopped;index"` // stopped, running, paused, error
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

type StrategyRun struct {
	ID             string     `json:"id" gorm:"primaryKey;size:36"`
	StrategyID     string     `json:"strategy_id" gorm:"size:36;index"`
	Account        string     `json:"account" gorm:"size:50;index"`
	StartedAt      time.Time  `json:"started_at"`
	StoppedAt      *time.Time `json:"stopped_at"`
	PnLRealized    float64    `json:"pnl_realized"`
	PnLUnrealized  float64    `json:"pnl_unrealized"`
	TradesCount    int        `json:"trades_count"`
	WinRate        float64    `json:"win_rate"`
	MaxDrawdown    float64    `json:"max_drawdown"`
	CreatedAt      time.Time  `json:"created_at"`
}
