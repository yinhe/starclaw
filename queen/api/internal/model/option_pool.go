package model

import "time"

// PartnerOptionInvestment records each partner's option purchase per round.
// CalcPartnerCommRate aggregates by round to determine the dynamic commission rate.
type PartnerOptionInvestment struct {
	ID          string    `json:"id" gorm:"primaryKey;type:varchar(36)"`
	PartnerID   string    `json:"partner_id" gorm:"type:varchar(36);index;not null"`
	PartnerType string    `json:"partner_type" gorm:"type:varchar(10);index"`       // city / team
	Round       string    `json:"round" gorm:"type:varchar(10);index"`              // spore / larva / zergling / overlord / queen
	Amount      int64     `json:"amount"`                                           // investment amount (分)
	Shares      int64     `json:"shares"`                                           // star diamond shares acquired
	Price       int64     `json:"price"`                                            // price per share at purchase (分)
	CommRate    float64   `json:"comm_rate"`                                        // commission rate computed for this investment
	Status      string    `json:"status" gorm:"type:varchar(20);default:completed"` // completed / refunded
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// PartnerRoundConfig defines per-round investment limits for city and team partners.
// Team limits = City limits × 2. Each round has 10,000,000 shares quota.
var PartnerRoundConfig = []struct {
	Round         string
	Label         string
	Price         int64 // floor price per star diamond (分)
	Quota         int64 // shares available this round
	CityMinInvest int64 // city partner min per transaction (分)
	CityMaxInvest int64 // city partner max per round (分) = round cap
	TeamMinInvest int64 // team partner min per transaction (分)
	TeamMaxInvest int64 // team partner max per round (分) = round cap
}{
	{"spore", "孢子期", 100, 10_000_000, 1_000_000, 5_000_000, 2_000_000, 10_000_000},
	{"larva", "幼虫期", 500, 10_000_000, 5_000_000, 25_000_000, 10_000_000, 50_000_000},
	{"zergling", "虫兵期", 2500, 10_000_000, 10_000_000, 100_000_000, 20_000_000, 200_000_000},
	{"overlord", "领主期", 12500, 10_000_000, 50_000_000, 500_000_000, 100_000_000, 1_000_000_000},
	{"queen", "虫后期", 62500, 10_000_000, 100_000_000, 2_000_000_000, 200_000_000, 4_000_000_000},
}

// PartnerOptionPoolQuota is the total star diamond shares reserved for partner options.
// 5 rounds × 10,000,000 = 50,000,000 (50% of total 1亿 supply).
const PartnerOptionPoolQuota int64 = 50_000_000

// GetPartnerRoundConfig returns the config for a given round, or nil if not found.
func GetPartnerRoundConfig(round string) *struct {
	Round         string
	Label         string
	Price         int64
	Quota         int64
	CityMinInvest int64
	CityMaxInvest int64
	TeamMinInvest int64
	TeamMaxInvest int64
} {
	for i := range PartnerRoundConfig {
		if PartnerRoundConfig[i].Round == round {
			return &PartnerRoundConfig[i]
		}
	}
	return nil
}

// PartnerRoundLimits returns (minInvest, maxInvest) for a given round and partner type.
func PartnerRoundLimits(round, partnerType string) (minFen, maxFen int64) {
	rc := GetPartnerRoundConfig(round)
	if rc == nil {
		return 1_000_000, 5_000_000 // fallback to spore city
	}
	if partnerType == "team" {
		return rc.TeamMinInvest, rc.TeamMaxInvest
	}
	return rc.CityMinInvest, rc.CityMaxInvest
}

// CalcPartnerCommRate computes the dynamic commission rate for a partner
// based on their option investments across all rounds.
//
// Formula per round: rate = baseRate + (roundTotal / roundMax) × rateRange
// Final rate = max across all rounds, clamped to [baseRate, baseRate+rateRange].
//
// City: 10% → 30% (rateRange = 0.20)
// Team: 10% → 20% (rateRange = 0.10)
func CalcPartnerCommRate(investments []PartnerOptionInvestment, partnerType string, baseRate, cityMaxRate, teamMaxRate float64) float64 {
	var rateRange float64
	if partnerType == "city" {
		rateRange = cityMaxRate - baseRate
	} else {
		rateRange = teamMaxRate - baseRate
	}
	if rateRange <= 0 {
		return baseRate
	}

	// Aggregate investment totals per round
	roundTotals := make(map[string]int64)
	for _, inv := range investments {
		if inv.Status != "refunded" {
			roundTotals[inv.Round] += inv.Amount
		}
	}

	bestRate := baseRate
	for roundName, totalAmount := range roundTotals {
		_, maxInvest := PartnerRoundLimits(roundName, partnerType)
		if maxInvest <= 0 {
			continue
		}
		ratio := float64(totalAmount) / float64(maxInvest)
		if ratio > 1.0 {
			ratio = 1.0
		}
		roundRate := baseRate + ratio*rateRange
		if roundRate > bestRate {
			bestRate = roundRate
		}
	}

	// Clamp
	maxRate := baseRate + rateRange
	if bestRate > maxRate {
		bestRate = maxRate
	}
	return bestRate
}
