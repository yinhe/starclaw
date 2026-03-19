package billing

import (
	"fmt"
	"log"
	"math"
	"strings"

	"github.com/yinhe/starclaw-router/internal/model"
	"github.com/yinhe/starclaw-router/internal/provider"
	"gorm.io/gorm"
)

// USD to CNY exchange rate (approximate, can be overridden via config)
const DefaultUSDToCNY = 7.2

// Meter calculates costs for API calls and manages balance deductions
type Meter struct {
	db          *gorm.DB
	registry    *provider.Registry
	queenCredit *QueenCreditClient
	rate        float64 // USD to CNY
	markup      float64 // markup multiplier (e.g. 1.3 = 30% margin)
}

func NewMeter(db *gorm.DB, registry *provider.Registry) *Meter {
	return &Meter{
		db:       db,
		registry: registry,
		rate:     DefaultUSDToCNY,
		markup:   1.3, // 30% margin over provider cost
	}
}

// SetQueenCredit sets the Queen credit client for star energy billing.
func (m *Meter) SetQueenCredit(qc *QueenCreditClient) {
	m.queenCredit = qc
}

// QueenCredit returns the Queen credit client (may be nil if not configured).
func (m *Meter) QueenCredit() *QueenCreditClient {
	return m.queenCredit
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

// CalculateCost returns the cost in cents (分) for a given model usage.
// modelName can be "provider/model" (preferred) or bare "model" (fallback scans all entries).
// Returns float64 to support sub-cent precision for cheap models.
func (m *Meter) CalculateCost(modelName string, promptTokens, completionTokens int) (costCents float64, upstreamCents float64) {
	entry, ok := m.registry.GetModel(modelName)
	if !ok {
		// Fallback: try matching bare model name against all registered models
		for _, e := range m.registry.ListModels() {
			// e.g. "qwen/qwen-plus" ends with "/qwen-plus", match "qwen-plus"
			if strings.HasSuffix(e.Model.Name, "/"+modelName) || e.Model.Name == modelName {
				entry = e
				ok = true
				break
			}
		}
		if !ok {
			return 0, 0
		}
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
func (m *Meter) calculateTokenCost(mc provider.ModelConfig, promptTokens, completionTokens int) (costCents, upstreamCents float64) {
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

	// Convert to cents (分): 1 CNY = 100 分 (float64 preserves sub-cent precision)
	upstreamCents = upstreamCNY * 100
	costCents = upstreamCNY * 100 * m.markup

	return costCents, upstreamCents
}

// calculateCallCost for per-call models (image, video, tts, etc.)
func (m *Meter) calculateCallCost(mc provider.ModelConfig) (costCents, upstreamCents float64) {
	var upstreamCNY float64

	if mc.PricePerCallCNY > 0 {
		upstreamCNY = mc.PricePerCallCNY
	} else if mc.PricePerCall > 0 {
		upstreamCNY = mc.PricePerCall * m.rate
	} else {
		return 0, 0
	}

	upstreamCents = upstreamCNY * 100
	costCents = upstreamCNY * 100 * m.markup

	return costCents, upstreamCents
}

// ── Star Energy billing (Claw signature auth) ──

// CnyFenToStarUnits is the conversion rate: 1分(¥0.01) = 1 Star = 10000 internal units
const CnyFenToStarUnits = 10000

// CheckClawBalance returns nil if the claw has sufficient star energy via Queen.
func (m *Meter) CheckClawBalance(clawID string) error {
	if m.queenCredit == nil || !m.queenCredit.Enabled() {
		return fmt.Errorf("star energy billing not configured")
	}
	return m.queenCredit.CheckBalance(clawID)
}

// CalculateStarCost converts a CNY cost (cents/分) to star energy units.
// Rate: 1分 = 1 Star = 10000 units
func (m *Meter) CalculateStarCost(costCents float64) int64 {
	return int64(math.Round(costCents * CnyFenToStarUnits))
}

// DeductClaw deducts star energy from a claw via Queen's internal API.
func (m *Meter) DeductClaw(clawID string, costCents float64, modelName, endpoint string, record *model.UsageRecord) error {
	if m.queenCredit == nil || !m.queenCredit.Enabled() {
		return fmt.Errorf("star energy billing not configured")
	}

	starUnits := m.CalculateStarCost(costCents)
	if starUnits <= 0 {
		// Record usage even with zero cost
		if record != nil {
			m.db.Create(record)
		}
		return nil
	}

	_, err := m.queenCredit.Consume(&ConsumeRequest{
		ClawID:       clawID,
		Amount:       starUnits,
		ResourceType: "tokens",
		Remark:       fmt.Sprintf("router %s %s", modelName, endpoint),
	})

	// Always record usage
	if record != nil {
		record.CostCents = costCents
		m.db.Create(record)
	}

	return err
}

// Deduct subtracts cost from user balance (tries free quota first, then balance)
// costCents/upstreamCents are in 分 (float64 for sub-cent precision).
// Balance deduction rounds up to nearest 分 (minimum charge ¥0.01 per billable request).
func (m *Meter) Deduct(userID string, costCents, upstreamCents float64, record *model.UsageRecord) error {
	// Round up to nearest 分 for actual balance deduction (balance is stored as int64 分)
	deductFen := int64(math.Ceil(costCents))

	return m.db.Transaction(func(tx *gorm.DB) error {
		var user model.User
		if err := tx.Where("id = ?", userID).First(&user).Error; err != nil {
			return err
		}

		// Try free quota first
		if user.FreeQuota >= deductFen {
			if err := tx.Model(&user).Update("free_quota", gorm.Expr("free_quota - ?", deductFen)).Error; err != nil {
				return err
			}
		} else if user.Balance >= deductFen {
			if err := tx.Model(&user).Update("balance", gorm.Expr("balance - ?", deductFen)).Error; err != nil {
				return err
			}
		} else {
			// Partial: use remaining free quota + balance
			remaining := deductFen
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

		// Update usage record with cost (full float precision for display)
		record.CostCents = costCents
		record.UpstreamCost = upstreamCents
		if err := tx.Create(record).Error; err != nil {
			log.Printf("[star-ai] failed to create usage record: %v", err)
		}

		return nil
	})
}
