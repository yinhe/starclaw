package model

import (
	"time"

	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════
// Phase 5 — GORM Persistence Models
//
// Covers 4 Swarm Civilization engines:
//   Autonomy:   Decision, DecisionRule, EvolutionInsight, PerformanceSnapshot
//   Exchange:   Order, Trade, AgentService, ServiceRequest, ServiceBid, ServiceRating
//   Federation: SwarmNode, Handshake, TaskRoute, TrustEvent
//   SwarmCtl:   PhysicalUnit, Formation, Mission, MissionLog
// ════════════════════════════════════════════════════════════

// ── Autonomy (Phase 5A) ──

type AutonomyDecision struct {
	gorm.Model
	DecisionID  string     `gorm:"uniqueIndex;size:64"`
	NodeID      string     `gorm:"index;size:64"`
	Type        string     `gorm:"size:32;index"` // health_action, scale_action, hotfix_action, evolution_action
	Trigger     string     `gorm:"size:500"`
	Description string     `gorm:"type:text"`
	Level       int        `gorm:"index"` // 0-3
	Status      string     `gorm:"size:16;index"` // pending, approved, rejected, executed, failed, vetoed
	Risk        string     `gorm:"size:16"`
	Reasoning   string     `gorm:"type:text"`
	WorkerType  string     `gorm:"size:32"`
	Action      string     `gorm:"size:64"`
	ActionParams string    `gorm:"type:text"` // JSON
	Priority    string     `gorm:"size:16"`
	Result      string     `gorm:"type:text"` // JSON
	ApprovedBy  string     `gorm:"size:128"`
	ExecutedAt  *time.Time
}

type AutonomyRule struct {
	gorm.Model
	RuleID      string `gorm:"uniqueIndex;size:64"`
	NodeID      string `gorm:"index;size:64"`
	Name        string `gorm:"size:200"`
	Description string `gorm:"size:1000"`
	Enabled     bool
	MinLevel    int        // minimum autonomy level to fire
	Risk        string     `gorm:"size:16"`
	Conditions  string     `gorm:"type:text"` // JSON array of RuleCondition
	WorkerType  string     `gorm:"size:32"`
	Action      string     `gorm:"size:64"`
	ActionParams string    `gorm:"type:text"` // JSON
	Priority    string     `gorm:"size:16"`
	CooldownSec int
	LastFired   *time.Time
}

type AutonomyInsight struct {
	gorm.Model
	InsightID   string `gorm:"uniqueIndex;size:64"`
	NodeID      string `gorm:"index;size:64"`
	Category    string `gorm:"size:32;index"` // performance, reliability, efficiency, capability
	Title       string `gorm:"size:500"`
	Description string `gorm:"type:text"`
	Severity    string `gorm:"size:16"` // info, warning, critical
	Suggestion  string `gorm:"type:text"`
	AutoFix     bool
	Status      string `gorm:"size:16;index"` // detected, proposed, accepted, applied, dismissed
}

type AutonomySnapshot struct {
	gorm.Model
	NodeID             string  `gorm:"index;size:64"`
	DecisionCount      int
	SuccessRate        float64
	AvgResponseMs      float64
	ActiveWorkers      int
	AutonomyLevel      int
	ConsecutiveSuccess int
	ConsecutiveFail    int
	NerveStats         string `gorm:"type:text"` // JSON
}

// ── Exchange (Phase 5B) ──

type ExchangeOrder struct {
	gorm.Model
	OrderID   string  `gorm:"uniqueIndex;size:64"`
	NodeID    string  `gorm:"index;size:64"`
	Side      string  `gorm:"size:8;index"` // buy, sell
	Type      string  `gorm:"size:8"`       // limit, market
	Price     float64
	Quantity  float64
	Filled    float64
	Status    string     `gorm:"size:16;index"` // open, filled, partial, cancelled
	FilledAt  *time.Time
}

type ExchangeTrade struct {
	gorm.Model
	TradeID   string  `gorm:"uniqueIndex;size:64"`
	BuyOrder  string  `gorm:"index;size:64"`
	SellOrder string  `gorm:"index;size:64"`
	BuyerID   string  `gorm:"index;size:64"`
	SellerID  string  `gorm:"index;size:64"`
	Price     float64
	Quantity  float64
	Total     float64
}

type ExchangeService struct {
	gorm.Model
	ServiceID   string  `gorm:"uniqueIndex;size:64"`
	AgentID     string  `gorm:"index;size:64"`
	NodeID      string  `gorm:"index;size:64"`
	Name        string  `gorm:"size:200"`
	Description string  `gorm:"size:1000"`
	Category    string  `gorm:"size:32;index"` // compute, inference, data, creative, analysis
	BasePrice   float64
	Rating      float64
	TotalCalls  int
	Status      string `gorm:"size:16;index"` // active, paused, retired
	Tags        string `gorm:"size:500"`      // JSON array
}

type ExchangeRequest struct {
	gorm.Model
	RequestID   string  `gorm:"uniqueIndex;size:64"`
	RequesterID string  `gorm:"index;size:64"`
	Title       string  `gorm:"size:500"`
	Description string  `gorm:"type:text"`
	Category    string  `gorm:"size:32;index"`
	Budget      float64
	Params      string  `gorm:"type:text"` // JSON
	Status      string  `gorm:"size:16;index"` // open, assigned, completed, cancelled
	AssignedTo  string  `gorm:"size:64"`
	CompletedAt *time.Time
}

type ExchangeBid struct {
	gorm.Model
	BidID     string  `gorm:"uniqueIndex;size:64"`
	AgentID   string  `gorm:"index;size:64"`
	ServiceID string  `gorm:"index;size:64"`
	RequestID string  `gorm:"index;size:64"`
	Price     float64
	ETA       string `gorm:"size:64"`
	Message   string `gorm:"size:500"`
	Status    string `gorm:"size:16;index"` // pending, accepted, rejected, complete
}

type ExchangeRating struct {
	gorm.Model
	RatingID  string  `gorm:"uniqueIndex;size:64"`
	ServiceID string  `gorm:"index;size:64"`
	RequestID string  `gorm:"index;size:64"`
	RaterID   string  `gorm:"index;size:64"`
	Score     float64
	Comment   string `gorm:"size:1000"`
}

// ── Federation (Phase 5C) ──

type FederationSwarm struct {
	gorm.Model
	SwarmID      string  `gorm:"uniqueIndex;size:64"`
	Name         string  `gorm:"size:200"`
	Endpoint     string  `gorm:"size:500"`
	Region       string  `gorm:"size:32"`
	Status       string  `gorm:"size:16;index"` // online, degraded, offline, suspended
	Trust        string  `gorm:"size:16;index"` // none, basic, verified, allied
	Capabilities string  `gorm:"type:text"`     // JSON array
	NodeCount    int
	AgentCount   int
	Reputation   float64
	LastSeen     time.Time
	JoinedAt     time.Time
	Metadata     string `gorm:"type:text"` // JSON
}

type FederationHandshake struct {
	gorm.Model
	HandshakeID string     `gorm:"uniqueIndex;size:64"`
	FromSwarm   string     `gorm:"index;size:64"`
	ToSwarm     string     `gorm:"index;size:64"`
	Status      string     `gorm:"size:16;index"` // pending, accepted, rejected
	Challenge   string     `gorm:"size:128"`
	Response    string     `gorm:"size:128"`
	CompletedAt *time.Time
}

type FederationTaskRoute struct {
	gorm.Model
	RouteID     string  `gorm:"uniqueIndex;size:64"`
	SourceSwarm string  `gorm:"index;size:64"`
	TargetSwarm string  `gorm:"index;size:64"`
	TaskType    string  `gorm:"size:32;index"`
	Description string  `gorm:"type:text"`
	Params      string  `gorm:"type:text"` // JSON
	Priority    string  `gorm:"size:16"`
	Status      string  `gorm:"size:16;index"` // proposed, accepted, rejected, running, completed, failed
	Bid         float64
	Result      string     `gorm:"type:text"` // JSON
	CompletedAt *time.Time
}

type FederationTrustEvent struct {
	gorm.Model
	EventID string  `gorm:"uniqueIndex;size:64"`
	SwarmID string  `gorm:"index;size:64"`
	Type    string  `gorm:"size:32;index"` // handshake_ok, task_success, task_fail, violation, alliance
	Delta   float64
	Details string `gorm:"size:500"`
}

// ── SwarmCtl (Phase 5D) ──

type SwarmCtlUnit struct {
	gorm.Model
	UnitID       string  `gorm:"uniqueIndex;size:64"`
	NodeID       string  `gorm:"index;size:64"`
	Name         string  `gorm:"size:200"`
	UnitType     string  `gorm:"size:16;index"` // zergling, roach, hydralisk, mutalisk, claw
	Domain       string  `gorm:"size:16;index"` // ground, air, digital
	Status       string  `gorm:"size:16;index"` // ready, busy, offline, damaged
	Lat          float64
	Lon          float64
	Alt          float64
	Battery      float64
	Health       float64
	Capabilities string  `gorm:"size:500"`  // JSON array
	PayloadKg    float64
	SpeedMps     float64
	AssignedTo   string  `gorm:"size:64"`   // mission ID
	LastSeen     time.Time
	Metadata     string `gorm:"type:text"` // JSON
}

type SwarmCtlFormation struct {
	gorm.Model
	FormationID string  `gorm:"uniqueIndex;size:64"`
	NodeID      string  `gorm:"index;size:64"`
	Name        string  `gorm:"size:200"`
	Shape       string  `gorm:"size:16"` // line, wedge, circle, scatter, column, overwatch
	UnitIDs     string  `gorm:"type:text"` // JSON array
	LeaderID    string  `gorm:"size:64"`
	CenterLat   float64
	CenterLon   float64
	CenterAlt   float64
	Spacing     float64
	Heading     float64
	Status      string `gorm:"size:16;index"` // forming, formed, moving, dissolved
}

type SwarmCtlMission struct {
	gorm.Model
	MissionID   string  `gorm:"uniqueIndex;size:64"`
	NodeID      string  `gorm:"index;size:64"`
	Name        string  `gorm:"size:200"`
	Type        string  `gorm:"size:16;index"` // patrol, escort, survey, delivery, search_rescue
	Status      string  `gorm:"size:16;index"` // planned, active, complete, aborted, failed
	Priority    string  `gorm:"size:8"`
	Domains     string  `gorm:"size:64"`       // JSON array
	UnitIDs     string  `gorm:"type:text"`     // JSON array
	FormationID string  `gorm:"size:64"`
	Waypoints   string  `gorm:"type:text"`     // JSON array of Position
	Objectives  string  `gorm:"type:text"`     // JSON array
	Constraints string  `gorm:"type:text"`     // JSON
	Params      string  `gorm:"type:text"`     // JSON
	Progress    float64
	StartedAt   *time.Time
	CompletedAt *time.Time
}

type SwarmCtlMissionLog struct {
	gorm.Model
	MissionID string    `gorm:"index;size:64"`
	UnitID    string    `gorm:"size:64"`
	Event     string    `gorm:"size:200"`
	Details   string    `gorm:"size:1000"`
	EventTime time.Time `gorm:"index"`
}
