package model

import "time"

// CorePartner represents an internal core partner (核心合伙人)
type CorePartner struct {
	ID     string `json:"id" gorm:"primaryKey;type:varchar(36)"`
	UserID string `json:"user_id" gorm:"type:varchar(36);index"`
	ClawID string `json:"claw_id" gorm:"type:varchar(60);uniqueIndex"` // claw:xxxx node address
	Name   string `json:"name" gorm:"type:varchar(100)"`
	Phone  string `json:"phone" gorm:"type:varchar(20)"`
	Email  string `json:"email" gorm:"type:varchar(200)"`
	Region string `json:"region" gorm:"type:varchar(100)"`               // responsible region
	Level  string `json:"level" gorm:"type:varchar(20);default:partner"` // partner / senior / director
	Status string `json:"status" gorm:"type:varchar(20);default:active"` // active / suspended / terminated

	// Dual-track compensation
	BaseSalary     int64   `json:"base_salary" gorm:"default:0"`         // monthly base (分)
	DirectCommRate float64 `json:"direct_comm_rate" gorm:"default:0.30"` // direct-sign commission rate
	ManageFeeRate  float64 `json:"manage_fee_rate" gorm:"default:0.05"`  // management fee from city partners

	// Stats
	TotalRevenue    int64 `json:"total_revenue" gorm:"default:0"`    // lifetime revenue generated
	TotalCommission int64 `json:"total_commission" gorm:"default:0"` // lifetime commission earned
	ActiveClients   int   `json:"active_clients" gorm:"default:0"`
	ManagedCities   int   `json:"managed_cities" gorm:"default:0"`

	JoinedAt  time.Time `json:"joined_at"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CRMDeal represents a client deal in the sales pipeline
type CRMDeal struct {
	ID          string     `json:"id" gorm:"primaryKey;type:varchar(36)"`
	PartnerID   string     `json:"partner_id" gorm:"type:varchar(36);index;not null"`
	CompanyName string     `json:"company_name" gorm:"type:varchar(200)"`
	ContactName string     `json:"contact_name" gorm:"type:varchar(100)"`
	ContactInfo string     `json:"contact_info" gorm:"type:varchar(200)"`
	Industry    string     `json:"industry" gorm:"type:varchar(100)"`
	Stage       string     `json:"stage" gorm:"type:varchar(20);default:lead;index"` // lead / opportunity / negotiation / signed / delivery / active / renewal / churned
	DealValue   int64      `json:"deal_value" gorm:"default:0"`                      // expected annual value (分)
	Plan        string     `json:"plan" gorm:"type:varchar(50)"`                     // starter / pro / enterprise / unlimited / whitelabel
	Source      string     `json:"source" gorm:"type:varchar(50)"`                   // inbound / outbound / referral / event
	Priority    string     `json:"priority" gorm:"type:varchar(10);default:medium"`  // low / medium / high / urgent
	Notes       string     `json:"notes" gorm:"type:text"`
	NextAction  string     `json:"next_action" gorm:"type:varchar(200)"`
	NextDate    *time.Time `json:"next_date"`
	SignedAt    *time.Time `json:"signed_at"`
	DeliveredAt *time.Time `json:"delivered_at"`
	RenewAt     *time.Time `json:"renew_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// PartnerCommission tracks dual-track commission for core partners
type PartnerCommission struct {
	ID         string    `json:"id" gorm:"primaryKey;type:varchar(36)"`
	PartnerID  string    `json:"partner_id" gorm:"type:varchar(36);index;not null"`
	DealID     string    `json:"deal_id" gorm:"type:varchar(36);index"`
	CityID     string    `json:"city_id" gorm:"type:varchar(36);index"` // if from city partner
	Type       string    `json:"type" gorm:"type:varchar(20)"`          // salary / direct / manage_fee
	Amount     int64     `json:"amount"`
	Rate       float64   `json:"rate"`
	BaseAmount int64     `json:"base_amount"`
	Month      string    `json:"month" gorm:"type:varchar(7);index"`
	Status     string    `json:"status" gorm:"type:varchar(20);default:pending"` // pending / approved / paid
	Remark     string    `json:"remark" gorm:"type:varchar(500)"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// EquityGrant represents stock option / equity grants to core partners
type EquityGrant struct {
	ID            string    `json:"id" gorm:"primaryKey;type:varchar(36)"`
	PartnerID     string    `json:"partner_id" gorm:"type:varchar(36);index;not null"`
	TotalShares   int64     `json:"total_shares"`                     // total shares granted
	VestedShares  int64     `json:"vested_shares" gorm:"default:0"`   // currently vested
	CliffMonths   int       `json:"cliff_months" gorm:"default:12"`   // cliff period
	VestingMonths int       `json:"vesting_months" gorm:"default:48"` // total vesting period
	GrantDate     time.Time `json:"grant_date"`
	CliffDate     time.Time `json:"cliff_date"`
	FullVestDate  time.Time `json:"full_vest_date"`
	StrikePrice   float64   `json:"strike_price" gorm:"default:0"`                 // exercise price per share (元)
	CurrentValue  float64   `json:"current_value" gorm:"default:0"`                // estimated value per share (元)
	Status        string    `json:"status" gorm:"type:varchar(20);default:active"` // active / cancelled / exercised
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// Deployment tracks one-click deployment instances for partner's clients
type Deployment struct {
	ID         string     `json:"id" gorm:"primaryKey;type:varchar(36)"`
	PartnerID  string     `json:"partner_id" gorm:"type:varchar(36);index"`
	DealID     string     `json:"deal_id" gorm:"type:varchar(36);index"`
	ClientName string     `json:"client_name" gorm:"type:varchar(200)"`
	Type       string     `json:"type" gorm:"type:varchar(20)"` // docker / k8s / cloud
	Region     string     `json:"region" gorm:"type:varchar(50)"`
	Domain     string     `json:"domain" gorm:"type:varchar(200)"`
	AdminEmail string     `json:"admin_email" gorm:"type:varchar(200)"`
	Version    string     `json:"version" gorm:"type:varchar(20)"`
	Status     string     `json:"status" gorm:"type:varchar(20);default:pending"` // pending / provisioning / running / stopped / failed
	Config     string     `json:"config" gorm:"type:text"`                        // JSON config blob
	HealthURL  string     `json:"health_url" gorm:"type:varchar(300)"`
	StartedAt  *time.Time `json:"started_at"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}
