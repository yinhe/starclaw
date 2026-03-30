package model

import "time"

type Settlement struct {
	ID                string    `json:"id" gorm:"primaryKey;size:36"`
	Date              string    `json:"date" gorm:"size:10;uniqueIndex"` // YYYY-MM-DD
	TotalNetPnL       float64   `json:"total_net_pnl"`
	ReinvestAmount    float64   `json:"reinvest_amount"`     // 60%
	StarEnergyAmount  float64   `json:"star_energy_amount"`  // 20%
	InvestorDividend  float64   `json:"investor_dividend"`   // 10%
	ReserveAmount     float64   `json:"reserve_amount"`      // 10%
	StarEnergyUnits   int64     `json:"star_energy_units"`   // converted to internal units
	QueenTxID         string    `json:"queen_tx_id" gorm:"size:100"`
	Status            string    `json:"status" gorm:"size:20;default:pending"` // pending, settled, failed
	CreatedAt         time.Time `json:"created_at"`
}
