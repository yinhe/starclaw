package partner

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// ════════════════════════════════════════════════════════════
// Partner v1 — 伙伴关系管理引擎
//
// 职责:
//   1. 伙伴管理: 核心伙伴(Team Partner) CRUD
//   2. 城市合伙人: City Partner 注册、审批、佣金阶梯
//   3. 投资人管理: Investor 登记、股权份额、分红
//   4. 佣金结算: 推荐码追踪、佣金计算、结算周期
//   5. CRM: 客户归属、推荐链、业绩统计
//   6. 审计: 操作日志、权限变更追踪
// ════════════════════════════════════════════════════════════

// ── Types ──

type PartnerLevel string

const (
	LevelTeam     PartnerLevel = "team"     // 核心伙伴
	LevelCity     PartnerLevel = "city"     // 城市合伙人
	LevelInvestor PartnerLevel = "investor" // 投资人
)

type PartnerStatus string

const (
	StatusPending  PartnerStatus = "pending"
	StatusActive   PartnerStatus = "active"
	StatusSuspended PartnerStatus = "suspended"
	StatusTerminated PartnerStatus = "terminated"
)

type CommissionTier struct {
	MinRevenue float64 `json:"min_revenue"` // 达到此营收
	Rate       float64 `json:"rate"`        // 佣金比例 (0.0~1.0)
	Label      string  `json:"label"`       // 阶梯名称
}

// ── Data Structures ──

type Partner struct {
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	Level        PartnerLevel   `json:"level"`
	Status       PartnerStatus  `json:"status"`
	Email        string         `json:"email,omitempty"`
	Phone        string         `json:"phone,omitempty"`
	ReferralCode string         `json:"referral_code"`
	ParentID     string         `json:"parent_id,omitempty"` // 上级伙伴
	CityCode     string         `json:"city_code,omitempty"` // 负责城市
	Equity       float64        `json:"equity,omitempty"`    // 股权份额 (%)
	CommTiers    []CommissionTier `json:"commission_tiers,omitempty"`
	Customers    int            `json:"customers"`
	TotalRevenue float64        `json:"total_revenue"`
	TotalComm    float64        `json:"total_commission"`
	JoinedAt     time.Time      `json:"joined_at"`
	ApprovedAt   *time.Time     `json:"approved_at,omitempty"`
	ApprovedBy   string         `json:"approved_by,omitempty"`
}

type CommissionRecord struct {
	ID         string    `json:"id"`
	PartnerID  string    `json:"partner_id"`
	CustomerID string    `json:"customer_id"`
	OrderID    string    `json:"order_id"`
	Revenue    float64   `json:"revenue"`
	Rate       float64   `json:"rate"`
	Amount     float64   `json:"amount"`
	Status     string    `json:"status"` // pending, settled, cancelled
	Period     string    `json:"period"` // e.g. "2026-04"
	CreatedAt  time.Time `json:"created_at"`
	SettledAt  *time.Time `json:"settled_at,omitempty"`
}

type Settlement struct {
	ID          string    `json:"id"`
	PartnerID   string    `json:"partner_id"`
	Period      string    `json:"period"`
	TotalOrders int       `json:"total_orders"`
	TotalRev    float64   `json:"total_revenue"`
	TotalComm   float64   `json:"total_commission"`
	Status      string    `json:"status"` // draft, confirmed, paid
	CreatedAt   time.Time `json:"created_at"`
	PaidAt      *time.Time `json:"paid_at,omitempty"`
}

type AuditEntry struct {
	ID        string    `json:"id"`
	ActorID   string    `json:"actor_id"`
	Action    string    `json:"action"`
	TargetID  string    `json:"target_id"`
	Details   string    `json:"details,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// ── Engine ──

type EngineConfig struct {
	DefaultCommTiers []CommissionTier `json:"default_commission_tiers"`
	SettlementCycle  string           `json:"settlement_cycle"` // monthly, biweekly
	AutoApprove      bool             `json:"auto_approve"`
	MaxCityPartners  int              `json:"max_city_partners"`
}

func DefaultEngineConfig() *EngineConfig {
	return &EngineConfig{
		DefaultCommTiers: []CommissionTier{
			{MinRevenue: 0, Rate: 0.05, Label: "Bronze"},
			{MinRevenue: 10000, Rate: 0.08, Label: "Silver"},
			{MinRevenue: 50000, Rate: 0.12, Label: "Gold"},
			{MinRevenue: 200000, Rate: 0.15, Label: "Platinum"},
		},
		SettlementCycle: "monthly",
		AutoApprove:     false,
		MaxCityPartners: 200,
	}
}

type Engine struct {
	mu          sync.RWMutex
	nodeID      string
	config      *EngineConfig
	partners    map[string]*Partner
	commissions []CommissionRecord
	settlements []Settlement
	audit       []AuditEntry
	stats       EngineStats
	startAt     time.Time
	nextID      int
}

type EngineStats struct {
	PartnersTotal    int       `json:"partners_total"`
	PartnersActive   int       `json:"partners_active"`
	TeamPartners     int       `json:"team_partners"`
	CityPartners     int       `json:"city_partners"`
	Investors        int       `json:"investors"`
	TotalRevenue     float64   `json:"total_revenue"`
	TotalCommissions float64   `json:"total_commissions"`
	PendingApprovals int       `json:"pending_approvals"`
	Uptime           string    `json:"uptime"`
	LastActivity     time.Time `json:"last_activity,omitempty"`
}

var (
	globalEngine *Engine
	engineOnce   sync.Once
)

func InitEngine(nodeID string, cfg *EngineConfig) *Engine {
	if cfg == nil {
		cfg = DefaultEngineConfig()
	}
	engineOnce.Do(func() {
		globalEngine = &Engine{
			nodeID:      nodeID,
			config:      cfg,
			partners:    make(map[string]*Partner),
			commissions: make([]CommissionRecord, 0),
			settlements: make([]Settlement, 0),
			audit:       make([]AuditEntry, 0),
			startAt:     time.Now(),
		}
		log.Printf("[partner] engine ready (cycle=%s, auto_approve=%v, tiers=%d)",
			cfg.SettlementCycle, cfg.AutoApprove, len(cfg.DefaultCommTiers))
	})
	return globalEngine
}

func GetEngine() *Engine {
	return globalEngine
}

func (e *Engine) genID(prefix string) string {
	e.nextID++
	return fmt.Sprintf("%s-%d-%d", prefix, time.Now().Unix(), e.nextID)
}

func (e *Engine) addAudit(actorID, action, targetID, details string) {
	entry := AuditEntry{
		ID:        e.genID("audit"),
		ActorID:   actorID,
		Action:    action,
		TargetID:  targetID,
		Details:   details,
		Timestamp: time.Now(),
	}
	e.audit = append(e.audit, entry)
	if len(e.audit) > 1000 {
		e.audit = e.audit[1:]
	}
}

// ── Partner CRUD ──

func (e *Engine) RegisterPartner(name string, level PartnerLevel, email, phone, parentID, cityCode string, equity float64) *Partner {
	e.mu.Lock()
	defer e.mu.Unlock()

	status := StatusPending
	if e.config.AutoApprove {
		status = StatusActive
	}

	code := fmt.Sprintf("SC%s%04d", string(level[0]), e.nextID+1)

	p := &Partner{
		ID:           e.genID("partner"),
		Name:         name,
		Level:        level,
		Status:       status,
		Email:        email,
		Phone:        phone,
		ReferralCode: code,
		ParentID:     parentID,
		CityCode:     cityCode,
		Equity:       equity,
		CommTiers:    e.config.DefaultCommTiers,
		JoinedAt:     time.Now(),
	}

	e.partners[p.ID] = p
	e.stats.PartnersTotal++
	if status == StatusActive {
		e.stats.PartnersActive++
	} else {
		e.stats.PendingApprovals++
	}
	switch level {
	case LevelTeam:
		e.stats.TeamPartners++
	case LevelCity:
		e.stats.CityPartners++
	case LevelInvestor:
		e.stats.Investors++
	}
	e.stats.LastActivity = time.Now()
	e.addAudit("system", "register", p.ID, fmt.Sprintf("Partner %s registered as %s", name, level))
	log.Printf("[partner] registered: %s — %s (%s, status=%s)", p.ID, name, level, status)
	return p
}

func (e *Engine) ApprovePartner(partnerID, approver string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	p, ok := e.partners[partnerID]
	if !ok {
		return fmt.Errorf("partner %s not found", partnerID)
	}
	if p.Status != StatusPending {
		return fmt.Errorf("partner %s not pending (status=%s)", partnerID, p.Status)
	}

	now := time.Now()
	p.Status = StatusActive
	p.ApprovedAt = &now
	p.ApprovedBy = approver
	e.stats.PartnersActive++
	if e.stats.PendingApprovals > 0 {
		e.stats.PendingApprovals--
	}
	e.stats.LastActivity = now
	e.addAudit(approver, "approve", partnerID, "Partner approved")
	return nil
}

func (e *Engine) SuspendPartner(partnerID, reason string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	p, ok := e.partners[partnerID]
	if !ok {
		return fmt.Errorf("partner %s not found", partnerID)
	}

	p.Status = StatusSuspended
	if e.stats.PartnersActive > 0 {
		e.stats.PartnersActive--
	}
	e.stats.LastActivity = time.Now()
	e.addAudit("admin", "suspend", partnerID, reason)
	return nil
}

func (e *Engine) GetPartner(partnerID string) (*Partner, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	p, ok := e.partners[partnerID]
	if !ok {
		return nil, fmt.Errorf("partner %s not found", partnerID)
	}
	return p, nil
}

func (e *Engine) ListPartners(level string, status string) []*Partner {
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make([]*Partner, 0)
	for _, p := range e.partners {
		if level != "" && string(p.Level) != level {
			continue
		}
		if status != "" && string(p.Status) != status {
			continue
		}
		result = append(result, p)
	}
	return result
}

// ── Commission ──

func (e *Engine) RecordCommission(partnerID, customerID, orderID string, revenue float64) (*CommissionRecord, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	p, ok := e.partners[partnerID]
	if !ok {
		return nil, fmt.Errorf("partner %s not found", partnerID)
	}
	if p.Status != StatusActive {
		return nil, fmt.Errorf("partner %s not active", partnerID)
	}

	// Determine rate from tiers
	rate := 0.0
	for _, tier := range p.CommTiers {
		if p.TotalRevenue >= tier.MinRevenue {
			rate = tier.Rate
		}
	}

	amount := revenue * rate
	now := time.Now()
	rec := CommissionRecord{
		ID:         e.genID("comm"),
		PartnerID:  partnerID,
		CustomerID: customerID,
		OrderID:    orderID,
		Revenue:    revenue,
		Rate:       rate,
		Amount:     amount,
		Status:     "pending",
		Period:     now.Format("2006-01"),
		CreatedAt:  now,
	}

	p.TotalRevenue += revenue
	p.TotalComm += amount
	p.Customers++
	e.commissions = append(e.commissions, rec)
	if len(e.commissions) > 2000 {
		e.commissions = e.commissions[1:]
	}

	e.stats.TotalRevenue += revenue
	e.stats.TotalCommissions += amount
	e.stats.LastActivity = now
	return &rec, nil
}

func (e *Engine) ListCommissions(partnerID string, limit int) []CommissionRecord {
	e.mu.RLock()
	defer e.mu.RUnlock()
	var result []CommissionRecord
	for _, c := range e.commissions {
		if partnerID == "" || c.PartnerID == partnerID {
			result = append(result, c)
		}
	}
	if limit > 0 && len(result) > limit {
		result = result[len(result)-limit:]
	}
	return result
}

// ── Settlement ──

func (e *Engine) CreateSettlement(partnerID, period string) (*Settlement, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	p, ok := e.partners[partnerID]
	if !ok {
		return nil, fmt.Errorf("partner %s not found", partnerID)
	}

	var totalRev, totalComm float64
	var orders int
	for _, c := range e.commissions {
		if c.PartnerID == partnerID && c.Period == period && c.Status == "pending" {
			totalRev += c.Revenue
			totalComm += c.Amount
			orders++
		}
	}

	s := Settlement{
		ID:          e.genID("settle"),
		PartnerID:   partnerID,
		Period:      period,
		TotalOrders: orders,
		TotalRev:    totalRev,
		TotalComm:   totalComm,
		Status:      "draft",
		CreatedAt:   time.Now(),
	}

	e.settlements = append(e.settlements, s)
	e.stats.LastActivity = time.Now()
	e.addAudit("system", "settlement_created", p.ID, fmt.Sprintf("Period %s: %d orders, %.2f commission", period, orders, totalComm))
	return &s, nil
}

func (e *Engine) ListSettlements(partnerID string) []Settlement {
	e.mu.RLock()
	defer e.mu.RUnlock()
	var result []Settlement
	for _, s := range e.settlements {
		if partnerID == "" || s.PartnerID == partnerID {
			result = append(result, s)
		}
	}
	return result
}

// ── Audit ──

func (e *Engine) ListAudit(limit int) []AuditEntry {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if limit <= 0 || limit > len(e.audit) {
		limit = len(e.audit)
	}
	if limit == 0 {
		return nil
	}
	start := len(e.audit) - limit
	result := make([]AuditEntry, limit)
	copy(result, e.audit[start:])
	return result
}

// ── Stats ──

func (e *Engine) Stats() *EngineStats {
	e.mu.RLock()
	defer e.mu.RUnlock()
	s := e.stats
	s.Uptime = time.Since(e.startAt).Round(time.Second).String()
	return &s
}

func (e *Engine) Config() *EngineConfig {
	return e.config
}
