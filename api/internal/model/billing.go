package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Tenant represents an organization / team
type Tenant struct {
	ID        string         `json:"id" gorm:"type:varchar(36);primaryKey"`
	Name      string         `json:"name" gorm:"type:varchar(200);not null"`
	OwnerID   string         `json:"owner_id" gorm:"type:varchar(36);index;not null"`
	Balance   float64        `json:"balance" gorm:"type:decimal(12,2);default:0"` // 余额（元）
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	Members []TenantMember `json:"members,omitempty" gorm:"foreignKey:TenantID"`
}

func (t *Tenant) BeforeCreate(tx *gorm.DB) error {
	if t.ID == "" {
		t.ID = uuid.New().String()
	}
	return nil
}

// TenantMember links a user to a tenant with a role
type TenantMember struct {
	ID       string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	TenantID string    `json:"tenant_id" gorm:"type:varchar(36);uniqueIndex:idx_tenant_user;not null"`
	UserID   string    `json:"user_id" gorm:"type:varchar(36);uniqueIndex:idx_tenant_user;not null"`
	Role     string    `json:"role" gorm:"type:varchar(20);default:'member'"` // owner, admin, member
	JoinedAt time.Time `json:"joined_at"`
}

func (m *TenantMember) BeforeCreate(tx *gorm.DB) error {
	if m.ID == "" {
		m.ID = uuid.New().String()
	}
	if m.JoinedAt.IsZero() {
		m.JoinedAt = time.Now()
	}
	return nil
}

// Plan defines a recharge package (充值套餐)
type Plan struct {
	ID          string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	Name        string    `json:"name" gorm:"type:varchar(50);uniqueIndex;not null"`
	DisplayName string    `json:"display_name" gorm:"type:varchar(100);not null"`
	Price       float64   `json:"price" gorm:"default:0"`      // 充值金额（元）
	Credits     float64   `json:"credits" gorm:"default:0"`    // 实际到账金额（含赠送）
	BonusPct    int       `json:"bonus_pct" gorm:"default:0"`  // 赠送百分比
	Tag         string    `json:"tag" gorm:"type:varchar(30)"` // 标签：热门、超值等
	SortOrder   int       `json:"sort_order" gorm:"default:0"`
	CreatedAt   time.Time `json:"created_at"`
}

func (p *Plan) BeforeCreate(tx *gorm.DB) error {
	if p.ID == "" {
		p.ID = uuid.New().String()
	}
	return nil
}

// UsageRecord tracks daily resource consumption per tenant
type UsageRecord struct {
	ID           string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	TenantID     string    `json:"tenant_id" gorm:"type:varchar(36);index;not null"`
	UserID       string    `json:"user_id" gorm:"type:varchar(36);index"`
	ResourceType string    `json:"resource_type" gorm:"type:varchar(30);index;not null"` // tokens, video, image, music, storage
	Source       string    `json:"source" gorm:"type:varchar(20);default:self"`          // starai = StarAI platform, self = user's own API key
	Quantity     int64     `json:"quantity" gorm:"default:0"`
	Cost         float64   `json:"cost" gorm:"type:decimal(10,4);default:0"` // 本次消费金额
	Date         string    `json:"date" gorm:"type:varchar(10);index;not null"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (u *UsageRecord) BeforeCreate(tx *gorm.DB) error {
	if u.ID == "" {
		u.ID = uuid.New().String()
	}
	return nil
}

// Transaction records all balance changes (recharge / consumption)
type Transaction struct {
	ID        string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	TenantID  string    `json:"tenant_id" gorm:"type:varchar(36);index;not null"`
	UserID    string    `json:"user_id" gorm:"type:varchar(36);index"`
	Type      string    `json:"type" gorm:"type:varchar(20);not null"` // recharge, consume
	Amount    float64   `json:"amount" gorm:"type:decimal(12,2);not null"`
	Balance   float64   `json:"balance" gorm:"type:decimal(12,2)"` // 交易后余额
	Remark    string    `json:"remark" gorm:"type:varchar(200)"`
	CreatedAt time.Time `json:"created_at"`
}

func (t *Transaction) BeforeCreate(tx *gorm.DB) error {
	if t.ID == "" {
		t.ID = uuid.New().String()
	}
	return nil
}

// Invoice is kept for compatibility but not actively used in recharge model
type Invoice struct {
	ID        string     `json:"id" gorm:"type:varchar(36);primaryKey"`
	TenantID  string     `json:"tenant_id" gorm:"type:varchar(36);index;not null"`
	Period    string     `json:"period" gorm:"type:varchar(7);not null"`
	PlanName  string     `json:"plan_name" gorm:"type:varchar(50)"`
	Amount    float64    `json:"amount" gorm:"default:0"`
	Status    string     `json:"status" gorm:"type:varchar(20);default:'pending'"`
	Items     string     `json:"items" gorm:"type:text"`
	CreatedAt time.Time  `json:"created_at"`
	PaidAt    *time.Time `json:"paid_at"`
}

func (i *Invoice) BeforeCreate(tx *gorm.DB) error {
	if i.ID == "" {
		i.ID = uuid.New().String()
	}
	return nil
}
