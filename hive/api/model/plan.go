package model

import "time"

// Plan defines a pricing tier for Claw instances.
type Plan struct {
	ID          string `gorm:"primaryKey;size:30" json:"id"` // free, basic, pro, enterprise
	DisplayName string `gorm:"size:50" json:"display_name"`
	DeployMode  string `gorm:"size:20" json:"deploy_mode"` // hive, ecs
	PriceDaily  int64  `json:"price_daily"`                 // star energy per day (0 = free)
	PriceMonthly int64 `json:"price_monthly"`               // star energy per month (discount)

	// Resource limits
	CPU         float64 `json:"cpu"`          // cores
	MemoryMB    int     `json:"memory_mb"`    // MB
	StorageGB   int     `json:"storage_gb"`   // GB
	BandwidthMB int     `json:"bandwidth_mb"` // Mbps (ECS only)

	// Features
	CustomDomain bool `json:"custom_domain"`
	SSLIncluded  bool `json:"ssl_included"`
	BackupDaily  bool `json:"backup_daily"`
	SLAPercent   int  `json:"sla_percent"` // e.g. 99, 999 (99.9%)

	ExpireDays int  `json:"expire_days"` // 0 = no expiry (paid plans)
	IsActive   bool `gorm:"default:true" json:"is_active"`

	CreatedAt time.Time `json:"created_at"`
}

func (Plan) TableName() string { return "plans" }

// DefaultPlans returns the initial plan definitions.
// Star energy: 1⚡ = 10000 internal units. Prices here are in internal energy units.
// ¥1/day = 100分/day = 100⚡/day = 1,000,000 energy/day
func DefaultPlans() []Plan {
	return []Plan{
		{
			ID: "free", DisplayName: "免费体验",
			DeployMode: "hive", PriceDaily: 0, PriceMonthly: 0,
			CPU: 0.5, MemoryMB: 512, StorageGB: 2, BandwidthMB: 0,
			CustomDomain: false, SSLIncluded: true, BackupDaily: false, SLAPercent: 0,
			ExpireDays: 7, IsActive: true,
		},
		{
			ID: "basic", DisplayName: "基础版",
			DeployMode: "hive", PriceDaily: 500_000, PriceMonthly: 10_000_000,
			CPU: 1, MemoryMB: 1024, StorageGB: 10, BandwidthMB: 0,
			CustomDomain: false, SSLIncluded: true, BackupDaily: true, SLAPercent: 99,
			ExpireDays: 0, IsActive: true,
		},
		{
			ID: "pro", DisplayName: "专业版",
			DeployMode: "ecs", PriceDaily: 2_000_000, PriceMonthly: 40_000_000,
			CPU: 2, MemoryMB: 4096, StorageGB: 40, BandwidthMB: 5,
			CustomDomain: true, SSLIncluded: true, BackupDaily: true, SLAPercent: 999,
			ExpireDays: 0, IsActive: true,
		},
		{
			ID: "enterprise", DisplayName: "企业版",
			DeployMode: "ecs", PriceDaily: 8_000_000, PriceMonthly: 160_000_000,
			CPU: 4, MemoryMB: 8192, StorageGB: 100, BandwidthMB: 10,
			CustomDomain: true, SSLIncluded: true, BackupDaily: true, SLAPercent: 999,
			ExpireDays: 0, IsActive: true,
		},
	}
}

// Order represents a purchase or subscription for a Claw instance.
type Order struct {
	ID         string `gorm:"primaryKey;size:36" json:"id"`
	InstanceID string `gorm:"size:36;index" json:"instance_id"`
	ClawID     string `gorm:"size:100;index" json:"claw_id"` // payer's claw address
	PlanID     string `gorm:"size:30" json:"plan_id"`
	Type       string `gorm:"size:20" json:"type"` // create, renew, upgrade

	// Billing
	Amount   int64  `json:"amount"`   // energy deducted
	FreezeID string `gorm:"size:36" json:"freeze_id"` // Queen freeze ID (for pending orders)
	Status   string `gorm:"size:20;default:pending" json:"status"` // pending, paid, failed, refunded

	// Period
	PeriodStart time.Time  `json:"period_start"`
	PeriodEnd   time.Time  `json:"period_end"`

	CreatedAt time.Time `json:"created_at"`
	PaidAt    *time.Time `json:"paid_at"`
}

func (Order) TableName() string { return "orders" }
