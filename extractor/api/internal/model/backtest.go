package model

import (
	"time"

	"gorm.io/datatypes"
)

type BacktestJob struct {
	ID             string         `json:"id" gorm:"primaryKey;size:36"`
	StrategyID     string         `json:"strategy_id" gorm:"size:36;index"`
	StrategyName   string         `json:"strategy_name" gorm:"size:100"`
	StartDate      string         `json:"start_date" gorm:"size:10"`
	EndDate        string         `json:"end_date" gorm:"size:10"`
	InitialCapital float64        `json:"initial_capital"`
	Params         datatypes.JSON `json:"params"`
	Status         string         `json:"status" gorm:"size:20;default:pending"` // pending, running, completed, failed
	Result         datatypes.JSON `json:"result"`                                // BacktestResult JSON
	ErrorMsg       string         `json:"error_msg" gorm:"size:500"`
	CreatedAt      time.Time      `json:"created_at"`
	FinishedAt     *time.Time     `json:"finished_at"`
}
