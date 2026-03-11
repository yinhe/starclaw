package billing

import (
	"fmt"
	"log"

	"github.com/yinhe/starclaw-router/internal/model"
	"github.com/yinhe/starclaw-router/internal/provider"
	"gorm.io/gorm"
)

// USD to CNY exchange rate (approximate, can be overridden via config)
const DefaultUSDToCNY = 7.2

// Meter calculates costs for API calls and manages balance deductions
type Meter struct {
	db       *gorm.DB
	registry *provider.Registry
	rate     float64 // USD to CNY
	markup   float64 // markup multiplier (e.g. 1.3 = 30% margin)
}

func NewMeter(db *gorm.DB, registry *provider.Registry) *Meter {
	return &Meter{
		db:       db,
		registry: registry,
		rate:     DefaultUSDToCNY,
		markup:   1.3, // 30% margin over provider cost
	}
}

// CheckBalance returns nil if the user has sufficient balance to proceed
func (m *Meter) CheckBalance(userID string) error {
	var user model.User
	if err := m.db.Where("id = ?", userID).First(&user).Error; err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	// Allow if user has balance OR free quota remaining
	if user.Balance > 0 || user.FreeQuota > 0 {
		return nil
	}

	return fmt.Errorf("insufficient balance")
}

// CalculateCost returns the cost in cents (分) for a given model usage
func (m *Meter) CalculateCost(modelName string, promptTokens, completionTokens int) (costCents int64, upstreamCents int64) {
	entry, ok := m.registry.GetModel(modelName)
	if !ok {
		return 0, 0
	}

	mc := entry.Model

	switch mc.Type {
	case "chat", "reasoning", "embedding":
		return m.calculateTokenCost(mc, promptTokens, completionTokens)
	case "image", "video", "tts", "stt", "image_edit":
		return m.calculateCallCost(mc)
	default:
		return 0, 0
	}
}

// calculateTokenCost for token-based models (chat, reasoning, embedding)
func (m *Meter) calculateTokenCost(mc provider.ModelConfig, promptTokens, completionTokens int) (costCents, upstreamCents int64) {
	var upstreamCNY float64

	if mc.InputPriceCNY > 0 || mc.OutputPriceCNY > 0 {
		// CNY pricing (per 千 tokens = per 1K tokens)
		upstreamCNY = (float64(promptTokens)/1000.0)*mc.InputPriceCNY +
			(float64(completionTokens)/1000.0)*mc.OutputPriceCNY
	} else {
		// USD pricing (per 1M tokens) → convert to CNY
		upstreamUSD := (float64(promptTokens)/1_000_000.0)*mc.InputPrice +
			(float64(completionTokens)/1_000_000.0)*mc.OutputPrice
		upstreamCNY = upstreamUSD * m.rate
	}

	// Convert to cents (分): 1 CNY = 100 分
	upstreamCents = int64(upstreamCNY * 100)
	costCents = int64(upstreamCNY * 100 * m.markup)

	return costCents, upstreamCents
}

// calculateCallCost for per-call models (image, video, tts, etc.)
func (m *Meter) calculateCallCost(mc provider.ModelConfig) (costCents, upstreamCents int64) {
	var upstreamCNY float64

	if mc.PricePerCallCNY > 0 {
		upstreamCNY = mc.PricePerCallCNY
	} else if mc.PricePerCall > 0 {
		upstreamCNY = mc.PricePerCall * m.rate
	} else {
		return 0, 0
	}

	upstreamCents = int64(upstreamCNY * 100)
	costCents = int64(upstreamCNY * 100 * m.markup)

	return costCents, upstreamCents
}

// Deduct subtracts cost from user balance (tries free quota first, then balance)
func (m *Meter) Deduct(userID string, costCents, upstreamCents int64, record *model.UsageRecord) error {
	return m.db.Transaction(func(tx *gorm.DB) error {
		var user model.User
		if err := tx.Where("id = ?", userID).First(&user).Error; err != nil {
			return err
		}

		// Try free quota first
		if user.FreeQuota >= costCents {
			if err := tx.Model(&user).Update("free_quota", gorm.Expr("free_quota - ?", costCents)).Error; err != nil {
				return err
			}
		} else if user.Balance >= costCents {
			if err := tx.Model(&user).Update("balance", gorm.Expr("balance - ?", costCents)).Error; err != nil {
				return err
			}
		} else {
			// Partial: use remaining free quota + balance
			remaining := costCents
			if user.FreeQuota > 0 {
				remaining -= user.FreeQuota
				tx.Model(&user).Update("free_quota", 0)
			}
			if user.Balance >= remaining {
				tx.Model(&user).Update("balance", gorm.Expr("balance - ?", remaining))
			} else {
				return fmt.Errorf("insufficient balance")
			}
		}

		// Update usage record with cost
		record.CostCents = costCents
		record.UpstreamCost = upstreamCents
		if err := tx.Create(record).Error; err != nil {
			log.Printf("[star-ai] failed to create usage record: %v", err)
		}

		return nil
	})
}
