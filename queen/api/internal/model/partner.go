package model

import "time"

// MaxCerebrates is the maximum number of Cerebrate (核心) seats in the team.
const MaxCerebrates = 5

// TeamPartner represents a team partner (团队合伙人).
// Two levels: Overlord (领主, regular) and Cerebrate (脑虫, core — max 5, elected by team vote).
type TeamPartner struct {
	ID     string `json:"id" gorm:"primaryKey;type:varchar(36)"`
	UserID string `json:"user_id" gorm:"type:varchar(36);index"`
	ClawID string `json:"claw_id" gorm:"type:varchar(60);uniqueIndex"` // claw:xxxx node address
	Name   string `json:"name" gorm:"type:varchar(100)"`
	Phone  string `json:"phone" gorm:"type:varchar(20)"`
	Email  string `json:"email" gorm:"type:varchar(200)"`
	Region string `json:"region" gorm:"type:varchar(100)"`                // responsible region
	Level  string `json:"level" gorm:"type:varchar(20);default:overlord"` // overlord (领主) / cerebrate (脑虫, max 5)
	Status string `json:"status" gorm:"type:varchar(20);default:active"`  // active / suspended / terminated

	// Dual-track compensation
	BaseSalary     int64   `json:"base_salary" gorm:"default:0"`         // monthly base (分)
	DirectCommRate float64 `json:"direct_comm_rate" gorm:"default:0.30"` // direct-sign commission rate
	ManageFeeRate  float64 `json:"manage_fee_rate" gorm:"default:0.05"`  // management fee from city partners

	// Stats
	TotalRevenue    int64 `json:"total_revenue" gorm:"default:0"`    // lifetime revenue generated
	TotalCommission int64 `json:"total_commission" gorm:"default:0"` // lifetime commission earned
	ActiveClients   int   `json:"active_clients" gorm:"default:0"`
	ManagedCities   int   `json:"managed_cities" gorm:"default:0"`

	// Option pool transition fields
	LegacyCommRate float64    `json:"legacy_comm_rate" gorm:"default:0"` // pre-transition commission rate
	TransitionEnd  *time.Time `json:"transition_end"`                    // when grace period expires
	TransitionDebt int64      `json:"transition_debt" gorm:"default:0"`  // excess commission during transition (分)

	JoinedAt  time.Time `json:"joined_at"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName keeps the DB table as core_partners for backward compatibility.
func (TeamPartner) TableName() string { return "core_partners" }

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

// PartnerCommission tracks dual-track commission for team partners
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

// EquityGrant represents stock option / equity grants to team partners
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

// TeamVote records votes for Cerebrate (脑虫) elections.
// Team partners (Overlords) vote to elect up to 5 Cerebrates.
type TeamVote struct {
	ID          string    `json:"id" gorm:"primaryKey;type:varchar(36)"`
	ElectionID  string    `json:"election_id" gorm:"type:varchar(36);index;not null"`  // which election round
	VoterID     string    `json:"voter_id" gorm:"type:varchar(36);index;not null"`     // TeamPartner.ID who voted
	CandidateID string    `json:"candidate_id" gorm:"type:varchar(36);index;not null"` // TeamPartner.ID being voted for
	CreatedAt   time.Time `json:"created_at"`
}

// PartnerInvite represents an invitation code for joining the partner network.
// Admin creates team_partner invites; TeamPartners create city_partner invites;
// CityPartners create referral invites (upgraded ref_code with controls).
type PartnerInvite struct {
	ID          string     `json:"id" gorm:"primaryKey;type:varchar(36)"`
	Code        string     `json:"code" gorm:"type:varchar(20);uniqueIndex"`      // e.g. "SC-A3F8-K9M2"
	Alias       string     `json:"alias" gorm:"type:varchar(50);uniqueIndex"`     // human-readable alias, e.g. "SC-BEIJING-001"
	Type        string     `json:"type" gorm:"type:varchar(20);index"`            // team_partner / city_partner / referral
	CreatorID   string     `json:"creator_id" gorm:"type:varchar(36);index"`      // partner ID or "admin"
	CreatorType string     `json:"creator_type" gorm:"type:varchar(20)"`          // admin / team_partner / city_partner
	CreatorName string     `json:"creator_name" gorm:"type:varchar(100)"`         // display name of creator
	Label       string     `json:"label" gorm:"type:varchar(200)"`                // internal label / note
	MaxUses     int        `json:"max_uses" gorm:"default:1"`                     // 0 = unlimited
	UsedCount   int        `json:"used_count" gorm:"default:0"`                   // current usage
	Region      string     `json:"region" gorm:"type:varchar(100)"`               // for city partners: target city
	CommRate    float64    `json:"comm_rate" gorm:"default:0"`                    // default commission rate (0 = use system default)
	Level       string     `json:"level" gorm:"type:varchar(20)"`                 // for team partners: default level
	BaseSalary  int64      `json:"base_salary" gorm:"default:0"`                  // for team partners: monthly base (分)
	PresetName  string     `json:"preset_name" gorm:"type:varchar(100)"`          // pre-fill candidate name
	PresetPhone string     `json:"preset_phone" gorm:"type:varchar(20)"`          // pre-fill candidate phone
	PresetEmail string     `json:"preset_email" gorm:"type:varchar(200)"`         // pre-fill candidate email
	ExpiresAt   *time.Time `json:"expires_at"`                                    // nil = never expires
	Status      string     `json:"status" gorm:"type:varchar(20);default:active"` // active / expired / revoked
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// JoinURL returns the landing page URL for this invite code.
func (i *PartnerInvite) JoinURL(baseURL string) string {
	code := i.Alias
	if code == "" {
		code = i.Code
	}
	return baseURL + "/join?code=" + code
}

// DisplayCode returns alias if set, otherwise code.
func (i *PartnerInvite) DisplayCode() string {
	if i.Alias != "" {
		return i.Alias
	}
	return i.Code
}

// PartnerInviteUse records each use of an invitation code.
type PartnerInviteUse struct {
	ID        string    `json:"id" gorm:"primaryKey;type:varchar(36)"`
	InviteID  string    `json:"invite_id" gorm:"type:varchar(36);index;not null"` // PartnerInvite.ID
	Code      string    `json:"code" gorm:"type:varchar(20);index"`               // denormalized for quick lookup
	ClawID    string    `json:"claw_id" gorm:"type:varchar(60);index"`            // the claw_id that used the code
	UserID    string    `json:"user_id" gorm:"type:varchar(36);index"`            // Queen user who used it
	PartnerID string    `json:"partner_id" gorm:"type:varchar(36)"`               // created TeamPartner.ID or CityPartner.ID
	Type      string    `json:"type" gorm:"type:varchar(20)"`                     // team_partner / city_partner
	CreatedAt time.Time `json:"created_at"`
}

// TeamElection represents a Cerebrate election round.
type TeamElection struct {
	ID          string     `json:"id" gorm:"primaryKey;type:varchar(36)"`
	Title       string     `json:"title" gorm:"type:varchar(200)"`
	Description string     `json:"description" gorm:"type:text"`
	Seats       int        `json:"seats" gorm:"default:5"`                      // number of Cerebrate seats
	Status      string     `json:"status" gorm:"type:varchar(20);default:open"` // open / closed / cancelled
	StartAt     time.Time  `json:"start_at"`
	EndAt       time.Time  `json:"end_at"`
	ClosedAt    *time.Time `json:"closed_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}
