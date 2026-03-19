package model

import "time"

// CityPartner represents a city-level distribution partner
type CityPartner struct {
	ID           string     `json:"id" gorm:"primaryKey;type:varchar(36)"`
	UserID       string     `json:"user_id" gorm:"type:varchar(36);index"`       // linked Queen user
	ClawID       string     `json:"claw_id" gorm:"type:varchar(60);uniqueIndex"` // claw:xxxx node address
	Name         string     `json:"name" gorm:"type:varchar(100)"`
	Company      string     `json:"company" gorm:"type:varchar(200)"`
	City         string     `json:"city" gorm:"type:varchar(100);index"`
	Phone        string     `json:"phone" gorm:"type:varchar(20)"`
	Email        string     `json:"email" gorm:"type:varchar(200)"`
	Experience   string     `json:"experience" gorm:"type:text"`
	RefCode      string     `json:"ref_code" gorm:"type:varchar(32);uniqueIndex"`   // UTM referral code
	CommRate     float64    `json:"comm_rate" gorm:"default:0.20"`                  // commission rate (0.20 = 20%)
	Status       string     `json:"status" gorm:"type:varchar(20);default:pending"` // pending / approved / rejected / suspended
	ApprovedAt   *time.Time `json:"approved_at"`
	TotalEarned  int64      `json:"total_earned" gorm:"default:0"`  // lifetime commission earned (分)
	TotalClients int        `json:"total_clients" gorm:"default:0"` // total referred clients
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// CityClient tracks clients referred by a city partner
type CityClient struct {
	ID          string     `json:"id" gorm:"primaryKey;type:varchar(36)"`
	PartnerID   string     `json:"partner_id" gorm:"type:varchar(36);index;not null"`
	UserID      string     `json:"user_id" gorm:"type:varchar(36);index"` // linked Queen user ID
	ClientName  string     `json:"client_name" gorm:"type:varchar(200)"`
	ContactInfo string     `json:"contact_info" gorm:"type:varchar(200)"`
	Plan        string     `json:"plan" gorm:"type:varchar(50)"`                // community / starter / pro / enterprise / unlimited
	Status      string     `json:"status" gorm:"type:varchar(20);default:lead"` // lead / trial / active / churned
	MRR         int64      `json:"mrr" gorm:"default:0"`                        // monthly recurring revenue (分)
	RefSource   string     `json:"ref_source" gorm:"type:varchar(50)"`          // utm_source value
	SignedAt    *time.Time `json:"signed_at"`
	RenewAt     *time.Time `json:"renew_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// Commission records individual commission events
type Commission struct {
	ID         string    `json:"id" gorm:"primaryKey;type:varchar(36)"`
	PartnerID  string    `json:"partner_id" gorm:"type:varchar(36);index;not null"`
	ClientID   string    `json:"client_id" gorm:"type:varchar(36);index"`
	OrderNo    string    `json:"order_no" gorm:"type:varchar(64);index"`         // linked payment order
	Type       string    `json:"type" gorm:"type:varchar(20)"`                   // signup / renewal / upgrade
	Amount     int64     `json:"amount"`                                         // commission amount (分)
	Rate       float64   `json:"rate"`                                           // rate applied
	BaseAmount int64     `json:"base_amount"`                                    // original order amount (分)
	Status     string    `json:"status" gorm:"type:varchar(20);default:pending"` // pending / approved / paid / rejected
	Month      string    `json:"month" gorm:"type:varchar(7);index"`             // YYYY-MM for grouping
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// Payout tracks commission payouts to partners
type Payout struct {
	ID         string     `json:"id" gorm:"primaryKey;type:varchar(36)"`
	PartnerID  string     `json:"partner_id" gorm:"type:varchar(36);index;not null"`
	Amount     int64      `json:"amount"`                                         // payout amount (分)
	Method     string     `json:"method" gorm:"type:varchar(20)"`                 // alipay / bank
	Account    string     `json:"account" gorm:"type:varchar(200)"`               // target account
	Status     string     `json:"status" gorm:"type:varchar(20);default:pending"` // pending / processing / completed / failed
	Month      string     `json:"month" gorm:"type:varchar(7)"`                   // YYYY-MM
	InvoiceURL string     `json:"invoice_url" gorm:"type:varchar(500)"`
	PaidAt     *time.Time `json:"paid_at"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// MarketingMaterial represents downloadable sales materials
type MarketingMaterial struct {
	ID          string    `json:"id" gorm:"primaryKey;type:varchar(36)"`
	Title       string    `json:"title" gorm:"type:varchar(200)"`
	Category    string    `json:"category" gorm:"type:varchar(50)"` // brochure / deck / case_study / video / quote_template
	Description string    `json:"description" gorm:"type:text"`
	FileURL     string    `json:"file_url" gorm:"type:varchar(500)"`
	FileSize    int64     `json:"file_size"`
	SortOrder   int       `json:"sort_order" gorm:"default:0"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
