package model

import "time"

// InvestorPool represents the global Star Diamond (星钻) profit-sharing pool.
// 10% of all transaction profit flows into this pool.
// Total supply: 1亿 Star Diamonds (fixed, never inflated).
//
// Pricing: price = max(NAV, round floor price)
//
//	NAV = (TotalRaised + TotalDeposited - TotalDistributed) / TotalShares
//
// Funding rounds (每期 10% 份额, 5× 递增):
//
//	孢子期:  10% @ ¥0.50/份  → 募资 ¥500万
//	幼虫期:  10% @ ¥2.50/份  → 募资 ¥2500万
//	虫兵期:  10% @ ¥12.50/份 → 募资 ¥1.25亿
//	领主期:  10% @ ¥62.50/份 → 募资 ¥6.25亿
//	虫后期:  10% @ ¥312.50/份→ 募资 ¥31.25亿
type InvestorPool struct {
	ID             string    `json:"id" gorm:"primaryKey;type:varchar(36)"`
	TotalShares    int64     `json:"total_shares" gorm:"default:0"`                 // total shares ever issued
	MaxShares      int64     `json:"max_shares" gorm:"default:100000000"`           // pool cap = 1亿 shares
	TotalDeposited int64     `json:"total_deposited" gorm:"default:0"`              // total profit deposited (分)
	TotalDistrib   int64     `json:"total_distributed" gorm:"default:0"`            // total dividends distributed (分)
	PoolBalance    int64     `json:"pool_balance" gorm:"default:0"`                 // undistributed balance (分)
	TotalRaised    int64     `json:"total_raised" gorm:"default:0"`                 // total investment raised (分)
	SeedTotal      int64     `json:"seed_total" gorm:"default:10000000"`            // seed round budget = 10% of 1亿 = 1000万 star diamonds
	SeedIssued     int64     `json:"seed_issued" gorm:"default:0"`                  // seed round shares already issued
	CurrentRound   string    `json:"current_round" gorm:"type:varchar(10)"`         // seed / angel / a / b / c / closed
	SharePrice     int64     `json:"share_price" gorm:"default:20"`                 // current price per share (分), driven by max(NAV, round floor)
	Status         string    `json:"status" gorm:"type:varchar(20);default:active"` // active / paused / closed
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// FundingRound tracks each investment round of the investor pool.
// Each period offers 10% of total pool shares at an escalating price (5× per period).
type FundingRound struct {
	ID            string     `json:"id" gorm:"primaryKey;type:varchar(36)"`
	Round         string     `json:"round" gorm:"type:varchar(10);uniqueIndex"`       // angel / a / b / c
	Label         string     `json:"label" gorm:"type:varchar(30)"`                   // 天使轮 / A轮 / B轮 / C轮
	SharePrice    int64      `json:"share_price"`                                     // price per share (分)
	Multiplier    int        `json:"multiplier"`                                      // 1 / 5 / 25 / 125
	SharesQuota   int64      `json:"shares_quota"`                                    // shares available this round (10% of MaxShares)
	SharesSold    int64      `json:"shares_sold" gorm:"default:0"`                    // shares sold so far
	AmountRaised  int64      `json:"amount_raised" gorm:"default:0"`                  // total CNY raised this round (分)
	InvestorCount int64      `json:"investor_count" gorm:"default:0"`                 // investors who bought in this round
	Status        string     `json:"status" gorm:"type:varchar(20);default:upcoming"` // upcoming / open / closed / sold_out
	OpenedAt      *time.Time `json:"opened_at"`
	ClosedAt      *time.Time `json:"closed_at"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// StarDiamondTotal is the fixed total supply of Star Diamonds.
const StarDiamondTotal int64 = 100_000_000 // 1亿

// RoundConfig defines the 5 funding periods (虫族命名).
//
//	spore    孢子期 — 生命起源，最早期支持者
//	larva    幼虫期 — 初具雏形，天使合伙人
//	zergling 虫兵期 — 成军出征，战略合伙人
//	overlord 领主期 — 领主加持，规模扩张
//	queen    虫后期 — 虫后降临，终极融资
var RoundConfig = []struct {
	Round      string
	Label      string
	Multiplier int
	Price      int64 // floor price per star diamond (分)
	MinInvest  int64 // 每笔最低投资 (分)
	MaxInvest  int64 // 每笔最高投资 (分)
}{
	{"spore", "孢子期", 1, 50, 1_000_000, 5_000_000},            // ¥0.50/份, ¥1万-¥5万
	{"larva", "幼虫期", 5, 250, 5_000_000, 25_000_000},          // ¥2.50/份, ¥5万-¥25万
	{"zergling", "虫兵期", 25, 1250, 10_000_000, 100_000_000},   // ¥12.50/份, ¥10万-¥100万
	{"overlord", "领主期", 125, 6250, 50_000_000, 500_000_000},  // ¥62.50/份, ¥50万-¥500万
	{"queen", "虫后期", 625, 31250, 100_000_000, 2_000_000_000}, // ¥312.50/份, ¥100万-¥2000万
}

// NextRound returns the next round name after the given round, or "" if no more rounds.
func NextRound(current string) string {
	order := []string{"spore", "larva", "zergling", "overlord", "queen"}
	for i, r := range order {
		if r == current && i+1 < len(order) {
			return order[i+1]
		}
	}
	return ""
}

// RoundFloorPrice returns the floor price for a given round code.
func RoundFloorPrice(round string) int64 {
	for _, rc := range RoundConfig {
		if rc.Round == round {
			return rc.Price
		}
	}
	return 50 // default spore price
}

// RoundLimits returns the min/max investment limits for a given round code (in 分).
func RoundLimits(round string) (minFen, maxFen int64) {
	for _, rc := range RoundConfig {
		if rc.Round == round {
			return rc.MinInvest, rc.MaxInvest
		}
	}
	return 1_000_000, 5_000_000 // default spore limits
}

// RoundLabel returns the display label for a given round code.
func RoundLabel(round string) string {
	for _, rc := range RoundConfig {
		if rc.Round == round {
			return rc.Label
		}
	}
	return round
}

// TierMaxInvest returns the per-transaction max investment (分) for the given investor tier.
//
//	team (团队合伙人): ¥10万 = 10_000_000 分
//	city (城市合伙人): ¥5万  =  5_000_000 分
func TierMaxInvest(tier string) int64 {
	switch tier {
	case "team":
		return 10_000_000 // ¥100,000
	default: // "city" or empty
		return 5_000_000 // ¥50,000
	}
}

// TierLabel returns the display label for an investor tier.
func TierLabel(tier string) string {
	switch tier {
	case "team":
		return "团队合伙人"
	default:
		return "城市合伙人"
	}
}

// DiamondOrder represents a direct payment order for purchasing Star Diamonds.
// Flow: create order → pay via Alipay/WeChat → callback issues shares.
type DiamondOrder struct {
	ID            string     `json:"id" gorm:"primaryKey;type:varchar(36)"`
	OrderNo       string     `json:"order_no" gorm:"type:varchar(64);uniqueIndex"`
	UserID        string     `json:"user_id" gorm:"type:varchar(36);index"`
	InvestorID    string     `json:"investor_id" gorm:"type:varchar(36);index"`
	Amount        int64      `json:"amount"`                                         // 支付金额 (分)
	Shares        int64      `json:"shares"`                                         // 预计获得星钻数
	PricePerShare int64      `json:"price_per_share"`                                // 下单时价格 (分)
	Round         string     `json:"round" gorm:"type:varchar(20)"`                  // 下单时轮次
	PayMethod     string     `json:"pay_method" gorm:"type:varchar(20)"`             // alipay / wechatpay
	PayForm       string     `json:"pay_form" gorm:"type:varchar(20);default:h5"`    // pc / h5 / native
	Status        string     `json:"status" gorm:"type:varchar(20);default:pending"` // pending / paid / failed / expired / refunded
	TradeNo       string     `json:"trade_no" gorm:"type:varchar(128)"`              // 第三方交易号
	Subject       string     `json:"subject" gorm:"type:varchar(200)"`
	PayURL        string     `json:"pay_url" gorm:"type:text"` // 支付链接
	CallbackRaw   string     `json:"-" gorm:"type:text"`
	PaidAt        *time.Time `json:"paid_at"`
	ExpireAt      *time.Time `json:"expire_at"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// CalcNAV computes the Net Asset Value per Star Diamond (分).
// NAV = (TotalRaised + TotalDeposited - TotalDistributed) / TotalShares
// Returns 0 if no shares have been issued yet.
func (p *InvestorPool) CalcNAV() int64 {
	if p.TotalShares <= 0 {
		return 0
	}
	assets := p.TotalRaised + p.TotalDeposited - p.TotalDistrib
	if assets <= 0 {
		return 0
	}
	return assets / p.TotalShares
}

// CalcPrice returns the dynamic Star Diamond price = max(NAV, roundFloorPrice).
// roundFloorPrice is the floor price of the current funding round (分).
func (p *InvestorPool) CalcPrice(roundFloorPrice int64) int64 {
	nav := p.CalcNAV()
	if nav > roundFloorPrice {
		return nav
	}
	return roundFloorPrice
}

// Investor represents an individual investor holding shares in the pool.
//
// Flow: Register → Sign Agreement (1/3/5yr) → Recharge (¥1万 min, multiple times)
//
//	→ Cumulative ≥ ¥10万 → Activated (profit sharing starts)
type Investor struct {
	ID             string     `json:"id" gorm:"primaryKey;type:varchar(36)"`
	UserID         string     `json:"user_id" gorm:"type:varchar(36);uniqueIndex"` // Queen user ID
	ClawID         string     `json:"claw_id" gorm:"type:varchar(60);index"`       // optional claw address
	Name           string     `json:"name" gorm:"type:varchar(100)"`
	Email          string     `json:"email" gorm:"type:varchar(200)"`
	Phone          string     `json:"phone" gorm:"type:varchar(20)"`
	Tier           string     `json:"tier" gorm:"type:varchar(20);default:city"` // team / city — determines per-txn purchase cap
	Shares         int64      `json:"shares" gorm:"default:0"`                   // current share holdings
	TotalDividends int64      `json:"total_dividends" gorm:"default:0"`          // lifetime dividends received (分)
	TotalInvested  int64      `json:"total_invested" gorm:"default:0"`           // lifetime recharge amount (分)
	Activated      bool       `json:"activated" gorm:"default:false"`            // true when TotalInvested >= ActivationThreshold
	ActivatedAt    *time.Time `json:"activated_at"`                              // when profit sharing started

	// Agreement
	AgreementTerm      int        `json:"agreement_term" gorm:"default:0"` // investment term: 1 / 3 / 5 (years), 0=not signed
	AgreementSignedAt  *time.Time `json:"agreement_signed_at"`             // when agreement was signed
	AgreementExpiresAt *time.Time `json:"agreement_expires_at"`            // when agreement expires

	Source    string     `json:"source" gorm:"type:varchar(30)"`                // self_register / seed_grant / admin_grant
	Status    string     `json:"status" gorm:"type:varchar(20);default:active"` // active / frozen / exited
	JoinedAt  time.Time  `json:"joined_at"`
	ExitedAt  *time.Time `json:"exited_at"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// InvestorTransaction records share acquisitions and exits.
type InvestorTransaction struct {
	ID            string    `json:"id" gorm:"primaryKey;type:varchar(36)"`
	InvestorID    string    `json:"investor_id" gorm:"type:varchar(36);index;not null"`
	Type          string    `json:"type" gorm:"type:varchar(20);index"` // seed_grant / purchase / transfer_in / transfer_out / exit
	Shares        int64     `json:"shares"`                             // number of shares (positive=acquire, negative=release)
	Amount        int64     `json:"amount" gorm:"default:0"`            // CNY amount paid/received (分)
	PricePerShare int64     `json:"price_per_share" gorm:"default:0"`   // price at time of transaction (分)
	Remark        string    `json:"remark" gorm:"type:varchar(500)"`
	CreatedAt     time.Time `json:"created_at"`
}

// InvestorDividend records periodic dividend distributions to investors.
type InvestorDividend struct {
	ID          string     `json:"id" gorm:"primaryKey;type:varchar(36)"`
	InvestorID  string     `json:"investor_id" gorm:"type:varchar(36);index;not null"`
	Period      string     `json:"period" gorm:"type:varchar(7);index"`            // YYYY-MM
	PoolDeposit int64      `json:"pool_deposit"`                                   // total pool deposit this period (分)
	ShareRatio  float64    `json:"share_ratio"`                                    // investor's share / total shares at snapshot
	Amount      int64      `json:"amount"`                                         // dividend amount (分)
	Shares      int64      `json:"shares"`                                         // investor's shares at snapshot
	TotalShares int64      `json:"total_shares"`                                   // pool total shares at snapshot
	Status      string     `json:"status" gorm:"type:varchar(20);default:pending"` // pending / paid / failed
	PaidAt      *time.Time `json:"paid_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

// PoolDeposit records each profit injection into the investor pool (10% of margin).
type PoolDeposit struct {
	ID          string    `json:"id" gorm:"primaryKey;type:varchar(36)"`
	SourceType  string    `json:"source_type" gorm:"type:varchar(30);index"` // tool_usage / token_usage / subscription
	SourceID    string    `json:"source_id" gorm:"type:varchar(36)"`         // ToolUsageRecord ID or other reference
	Amount      int64     `json:"amount"`                                    // deposit amount (分)
	MarginTotal int64     `json:"margin_total"`                              // total margin this came from (分)
	Rate        float64   `json:"rate" gorm:"default:0.10"`                  // investor share rate (10%)
	ClawID      string    `json:"claw_id" gorm:"type:varchar(60)"`           // which claw node generated this
	CreatedAt   time.Time `json:"created_at"`
}
