package engine

import (
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"starclaw.net/extractor/api/internal/model"
)

const (
	ReinvestRatio  = 0.60
	StarEnergyRatio = 0.20
	InvestorRatio  = 0.10
	ReserveRatio   = 0.10
	// ¥1 = 100⚡ = 1,000,000 internal units
	YuanToStarEnergyUnits = 1_000_000
)

// RunDailySettlement calculates the day's P&L and distributes profits.
func RunDailySettlement(db *gorm.DB, date string) (*model.Settlement, error) {
	var existing model.Settlement
	if err := db.Where("date = ?", date).First(&existing).Error; err == nil {
		return &existing, fmt.Errorf("settlement for %s already exists", date)
	}

	// Sum net PnL for the day
	var totalNetPnL float64
	db.Model(&model.DailyPnL{}).Where("date = ?", date).Select("COALESCE(SUM(net_pnl), 0)").Scan(&totalNetPnL)

	s := model.Settlement{
		ID:          uuid.New().String(),
		Date:        date,
		TotalNetPnL: totalNetPnL,
		Status:      "pending",
		CreatedAt:   time.Now(),
	}

	if totalNetPnL > 0 {
		s.ReinvestAmount = totalNetPnL * ReinvestRatio
		s.StarEnergyAmount = totalNetPnL * StarEnergyRatio
		s.InvestorDividend = totalNetPnL * InvestorRatio
		s.ReserveAmount = totalNetPnL * ReserveRatio
		s.StarEnergyUnits = int64(s.StarEnergyAmount * YuanToStarEnergyUnits)
	}
	// Loss days: no distribution

	if err := db.Create(&s).Error; err != nil {
		return nil, err
	}

	// TODO: POST to Queen /internal/credits/inject for star energy
	// TODO: POST to Queen investor pool for dividends
	// TODO: Publish Pheromone event: extractor.settlement.done

	s.Status = "settled"
	db.Model(&s).Update("status", "settled")

	log.Printf("[settlement] %s: net=%.2f reinvest=%.2f energy=%.2f(%.0d units) investor=%.2f reserve=%.2f",
		date, totalNetPnL, s.ReinvestAmount, s.StarEnergyAmount, s.StarEnergyUnits, s.InvestorDividend, s.ReserveAmount)

	return &s, nil
}
