package model

import (
	"time"

	"gorm.io/gorm"
)

type AccountBinding struct {
	ID            string    `json:"id" gorm:"primaryKey;size:36"`
	QMTAccount    string    `json:"qmt_account" gorm:"uniqueIndex;size:50"`
	Label         string    `json:"label" gorm:"size:100"`
	Group         string    `json:"group" gorm:"size:20;index"` // stable, trend, ai, arb, lab
	Status        string    `json:"status" gorm:"size:20;default:active"`
	TotalAssets   float64   `json:"total_assets"`
	Available     float64   `json:"available"`
	Frozen        float64   `json:"frozen"`
	MaxDrawdownPct float64  `json:"max_drawdown_pct"` // per-group risk limit
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func SeedAccountBindings(db *gorm.DB) {
	accounts := []AccountBinding{
		{ID: "acc-1006", QMTAccount: "test1006", Label: "稳健A-1", Group: "stable", MaxDrawdownPct: 2.0},
		{ID: "acc-1007", QMTAccount: "test1007", Label: "稳健A-2", Group: "stable", MaxDrawdownPct: 2.0},
		{ID: "acc-1008", QMTAccount: "test1008", Label: "稳健A-3", Group: "stable", MaxDrawdownPct: 2.0},
		{ID: "acc-1009", QMTAccount: "test1009", Label: "趋势B-1", Group: "trend", MaxDrawdownPct: 3.0},
		{ID: "acc-1010", QMTAccount: "test1010", Label: "趋势B-2", Group: "trend", MaxDrawdownPct: 3.0},
		{ID: "acc-1011", QMTAccount: "test1011", Label: "趋势B-3", Group: "trend", MaxDrawdownPct: 3.0},
		{ID: "acc-1012", QMTAccount: "test1012", Label: "AI C-1", Group: "ai", MaxDrawdownPct: 5.0},
		{ID: "acc-1013", QMTAccount: "test1013", Label: "AI C-2", Group: "ai", MaxDrawdownPct: 5.0},
		{ID: "acc-1014", QMTAccount: "test1014", Label: "套利D", Group: "arb", MaxDrawdownPct: 2.0},
		{ID: "acc-1015", QMTAccount: "test1015", Label: "实验E", Group: "lab", MaxDrawdownPct: 3.0},
	}
	for _, a := range accounts {
		a.Status = "active"
		db.Where("qmt_account = ?", a.QMTAccount).FirstOrCreate(&a)
	}
}
