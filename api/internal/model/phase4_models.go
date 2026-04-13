package model

import (
	"time"

	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════
// Phase 4D — GORM Persistence Models
//
// Covers 12 engines that were previously in-memory only:
//   BroodNet: TaskMarket, Orchestrator, Reputation, Pricing, Gossip
//   Phase4C: Abathur, SenseClaw, TestClaw, Cocoon, Chitin, Lair, Partner
// ════════════════════════════════════════════════════════════

// ── Abathur (Evolution Orchestration) ──

type EvolutionPlanRecord struct {
	gorm.Model
	PlanID      string `gorm:"uniqueIndex;size:64"`
	NodeID      string `gorm:"index;size:64"`
	Name        string `gorm:"size:200"`
	Goal        string `gorm:"size:2000"`
	Status      string `gorm:"size:32;index"` // draft, active, completed, cancelled
	Priority    int
	SprintCount int
	TaskCount   int
	ApprovedBy  string     `gorm:"size:128"`
	ApprovedAt  *time.Time
}

type SprintRecord struct {
	gorm.Model
	SprintID string `gorm:"uniqueIndex;size:64"`
	PlanID   string `gorm:"index;size:64"`
	NodeID   string `gorm:"index;size:64"`
	Name     string `gorm:"size:200"`
	Status   string `gorm:"size:32;index"` // planning, active, completed
	Tasks    int
	Done     int
	StartAt  *time.Time
	EndAt    *time.Time
}

type TaskRecord struct {
	gorm.Model
	TaskID     string `gorm:"uniqueIndex;size:64"`
	SprintID   string `gorm:"index;size:64"`
	NodeID     string `gorm:"index;size:64"`
	Title      string `gorm:"size:500"`
	WorkerType string `gorm:"size:32;index"` // sense_claw, scout_claw, dev_team, test_claw, ops_claw
	Status     string `gorm:"size:32;index"` // pending, assigned, running, done, failed
	Priority   int
	Result     string `gorm:"type:text"`
}

type HotfixRecord struct {
	gorm.Model
	HotfixID    string `gorm:"uniqueIndex;size:64"`
	NodeID      string `gorm:"index;size:64"`
	Title       string `gorm:"size:500"`
	Severity    string `gorm:"size:16;index"` // P0, P1, P2, P3
	Status      string `gorm:"size:32;index"`
	AssignedTo  string `gorm:"size:32"`
	Description string `gorm:"type:text"`
}

// ── SenseClaw (Observability) ──

type FeedbackRecord struct {
	gorm.Model
	FeedbackID string `gorm:"uniqueIndex;size:64"`
	NodeID     string `gorm:"index;size:64"`
	Type       string `gorm:"size:32;index"` // bug, feature, crash, ux, praise
	Source     string `gorm:"size:128"`
	Title      string `gorm:"size:500"`
	Body       string `gorm:"type:text"`
	Severity   string `gorm:"size:16"`
	Status     string `gorm:"size:32;index"` // open, acknowledged, resolved
	ResolvedBy string `gorm:"size:128"`
	ResolvedAt *time.Time
}

type AlertRecord struct {
	gorm.Model
	AlertID    string `gorm:"uniqueIndex;size:64"`
	NodeID     string `gorm:"index;size:64"`
	Severity   string `gorm:"size:16;index"` // critical, warning, info
	Source     string `gorm:"size:128"`
	Title      string `gorm:"size:500"`
	Message    string `gorm:"type:text"`
	Status     string `gorm:"size:32;index"` // firing, resolved, silenced
	ResolvedAt *time.Time
	SilencedAt *time.Time
}

type AnomalyRecord struct {
	gorm.Model
	AnomalyID  string  `gorm:"uniqueIndex;size:64"`
	NodeID     string  `gorm:"index;size:64"`
	Service    string  `gorm:"size:128;index"`
	Metric     string  `gorm:"size:128"`
	Expected   float64
	Actual     float64
	Deviation  float64
	Severity   string  `gorm:"size:16"`
	Status     string  `gorm:"size:32"`
}

// ── TestClaw (Testing & Validation) ──

type TestSuiteRecord struct {
	gorm.Model
	SuiteID     string `gorm:"uniqueIndex;size:64"`
	NodeID      string `gorm:"index;size:64"`
	Name        string `gorm:"size:200"`
	Type        string `gorm:"size:32;index"` // smoke, regression, deploy, benchmark, molt
	Status      string `gorm:"size:32;index"` // created, running, completed
	Description string `gorm:"size:1000"`
	TotalCases  int
	Passed      int
	Failed      int
	Skipped     int
	Duration    float64
	CompletedAt *time.Time
}

type TestCaseRecord struct {
	gorm.Model
	CaseID     string  `gorm:"uniqueIndex;size:64"`
	SuiteID    string  `gorm:"index;size:64"`
	Name       string  `gorm:"size:200"`
	Type       string  `gorm:"size:32"`
	Target     string  `gorm:"size:200"`
	Method     string  `gorm:"size:16"`
	Endpoint   string  `gorm:"size:500"`
	ExpectCode int
	Result     string  `gorm:"size:32;index"` // pending, pass, fail, skip, error
	StatusCode int
	LatencyMs  float64
	Error      string  `gorm:"type:text"`
}

type BenchmarkRecord struct {
	gorm.Model
	BenchmarkID string  `gorm:"uniqueIndex;size:64"`
	NodeID      string  `gorm:"index;size:64"`
	Target      string  `gorm:"size:200"`
	Endpoint    string  `gorm:"size:500"`
	Requests    int
	Concurrency int
	AvgLatency  float64
	P50Latency  float64
	P95Latency  float64
	P99Latency  float64
	MaxLatency  float64
	RPS         float64
	ErrorRate   float64
}

// ── Cocoon (Packaging & Build) ──

type CocoonSpecRecord struct {
	gorm.Model
	SpecID      string `gorm:"uniqueIndex;size:64"`
	NodeID      string `gorm:"index;size:64"`
	Name        string `gorm:"size:200;index"`
	Version     string `gorm:"size:32"`
	Type        string `gorm:"size:32"` // agent, skill, workflow, plugin, bundle
	Description string `gorm:"size:1000"`
	Author      string `gorm:"size:128"`
	EntryPoint  string `gorm:"size:500"`
	Valid       bool
	Errors      string `gorm:"type:text"` // JSON array
}

type BuildRecord struct {
	gorm.Model
	BuildID    string `gorm:"uniqueIndex;size:64"`
	SpecID     string `gorm:"index;size:64"`
	NodeID     string `gorm:"index;size:64"`
	Name       string `gorm:"size:200"`
	Version    string `gorm:"size:32"`
	Target     string `gorm:"size:32"` // linux/amd64, darwin/arm64, etc
	Status     string `gorm:"size:32;index"` // pending, running, success, failed
	OutputPath string `gorm:"size:500"`
	OutputSize int64
	Checksum   string  `gorm:"size:64"`
	Duration   float64
	Error      string  `gorm:"type:text"`
	StartedAt  *time.Time
	FinishedAt *time.Time
}

type PublishRecord struct {
	gorm.Model
	PublishID string `gorm:"uniqueIndex;size:64"`
	BuildID   string `gorm:"index;size:64"`
	Name      string `gorm:"size:200"`
	Version   string `gorm:"size:32"`
	Target    string `gorm:"size:32"` // nydus, forge, local
	URL       string `gorm:"size:500"`
	Status    string `gorm:"size:32"`
	Error     string `gorm:"type:text"`
}

// ── Chitin (Runtime) ──

type RuntimeInstance struct {
	gorm.Model
	InstanceID    string `gorm:"uniqueIndex;size:64"`
	NodeID        string `gorm:"index;size:64"`
	Name          string `gorm:"size:200"`
	AgentID       string `gorm:"index;size:64"`
	Mode          string `gorm:"size:16"` // process, container
	Status        string `gorm:"size:32;index"` // creating, running, stopped, failed, destroyed
	Image         string `gorm:"size:500"`
	Command       string `gorm:"size:1000"`
	CPUCores      float64
	MemoryMB      int
	DiskMB        int
	RestartPolicy string `gorm:"size:16"`
	RestartCount  int
	MaxRestarts   int
	HealthCheck   string `gorm:"size:500"`
	Healthy       bool
	PID           int
	ContainerID   string `gorm:"size:128"`
	Port          int
	StartedAt     *time.Time
	StoppedAt     *time.Time
	LastHealthAt  *time.Time
}

type RuntimeEvent struct {
	gorm.Model
	InstanceID string `gorm:"index;size:64"`
	NodeID     string `gorm:"index;size:64"`
	Type       string `gorm:"size:32;index"` // created, started, stopped, failed, restarted, health_ok, health_fail
	Message    string `gorm:"size:1000"`
}

// ── Lair (Multi-Node Cluster) ──

type LairNodeRecord struct {
	gorm.Model
	LairNodeID   string `gorm:"uniqueIndex;size:64"`
	NodeID       string `gorm:"index;size:64"` // self node
	Name         string `gorm:"size:200"`
	Address      string `gorm:"size:200"`
	BrooddPort   int
	Status       string `gorm:"size:32;index"` // online, offline, draining, maintenance
	Labels       string `gorm:"type:text"` // JSON
	CPUTotal     float64
	CPUUsed      float64
	MemTotalMB   int
	MemUsedMB    int
	DiskTotalMB  int
	DiskUsedMB   int
	Instances    int
	MaxInstances int
	LastHeartbeat time.Time
}

type DeploymentRecord struct {
	gorm.Model
	DeployID  string `gorm:"uniqueIndex;size:64"`
	NodeID    string `gorm:"index;size:64"`
	Name      string `gorm:"size:200"`
	AgentID   string `gorm:"index;size:64"`
	Version   string `gorm:"size:32"`
	TargetNode string `gorm:"size:64"`
	Status    string `gorm:"size:32;index"` // pending, running, success, failed, rolled_back
	Replicas  int
	Image     string `gorm:"size:500"`
	Command   string `gorm:"size:1000"`
	Error     string `gorm:"type:text"`
	CompletedAt *time.Time
}

type RolloutRecord struct {
	gorm.Model
	RolloutID string `gorm:"uniqueIndex;size:64"`
	NodeID    string `gorm:"index;size:64"`
	Name      string `gorm:"size:200"`
	AgentID   string `gorm:"index;size:64"`
	Version   string `gorm:"size:32"`
	Strategy  string `gorm:"size:16"` // rolling, blue_green, canary
	NodeIDs   string `gorm:"type:text"` // JSON array
	BatchSize int
	Status    string `gorm:"size:32;index"`
	Progress  int
	CompletedAt *time.Time
}

// ── Partner (CRM) ──

type PartnerRecord struct {
	gorm.Model
	PartnerID    string  `gorm:"uniqueIndex;size:64"`
	NodeID       string  `gorm:"index;size:64"`
	Name         string  `gorm:"size:200"`
	Level        string  `gorm:"size:16;index"` // team, city, investor
	Status       string  `gorm:"size:16;index"` // pending, active, suspended, terminated
	Email        string  `gorm:"size:200"`
	Phone        string  `gorm:"size:32"`
	ReferralCode string  `gorm:"uniqueIndex;size:32"`
	ParentID     string  `gorm:"index;size:64"`
	CityCode     string  `gorm:"size:16"`
	Equity       float64
	Customers    int
	TotalRevenue float64
	TotalComm    float64
	ApprovedBy   string  `gorm:"size:128"`
	ApprovedAt   *time.Time
}

type CommissionDBRecord struct {
	gorm.Model
	CommID     string  `gorm:"uniqueIndex;size:64"`
	PartnerID  string  `gorm:"index;size:64"`
	CustomerID string  `gorm:"size:64"`
	OrderID    string  `gorm:"size:64"`
	Revenue    float64
	Rate       float64
	Amount     float64
	Status     string  `gorm:"size:16;index"` // pending, settled, cancelled
	Period     string  `gorm:"size:16;index"` // e.g. "2026-04"
	SettledAt  *time.Time
}

type SettlementRecord struct {
	gorm.Model
	SettleID    string  `gorm:"uniqueIndex;size:64"`
	PartnerID   string  `gorm:"index;size:64"`
	Period      string  `gorm:"size:16"`
	TotalOrders int
	TotalRev    float64
	TotalComm   float64
	Status      string  `gorm:"size:16;index"` // draft, confirmed, paid
	PaidAt      *time.Time
}

// ── BroodNet: TaskMarket ──

type MarketTask struct {
	gorm.Model
	MarketTaskID string  `gorm:"uniqueIndex;size:64"`
	NodeID       string  `gorm:"index;size:64"`
	Title        string  `gorm:"size:500"`
	Type         string  `gorm:"size:32;index"` // compute, inference, data, deploy, test, custom
	Description  string  `gorm:"type:text"`
	RequesterID  string  `gorm:"index;size:64"`
	AssigneeID   string  `gorm:"index;size:64"`
	Status       string  `gorm:"size:32;index"` // open, bidding, matched, running, completed, failed, expired
	Budget       float64
	FinalPrice   float64
	Deadline     *time.Time
	CompletedAt  *time.Time
}

type MarketBid struct {
	gorm.Model
	BidID     string  `gorm:"uniqueIndex;size:64"`
	TaskID    string  `gorm:"index;size:64"`
	BidderID  string  `gorm:"index;size:64"`
	Price     float64
	EstTime   int // seconds
	Status    string  `gorm:"size:16"` // pending, accepted, rejected
}

// ── BroodNet: Reputation ──

type ReputationEntry struct {
	gorm.Model
	NodeTargetID string  `gorm:"index;size:64"` // the node being rated
	EventType    string  `gorm:"size:32;index"`
	Delta        float64
	Score        float64 // score after this event
	Source       string  `gorm:"size:64"`
	Details      string  `gorm:"size:500"`
}

// ── BroodNet: Gossip ──

type GossipPeer struct {
	gorm.Model
	PeerID    string `gorm:"uniqueIndex;size:64"`
	NodeID    string `gorm:"index;size:64"` // owning node
	Address   string `gorm:"size:200"`
	Status    string `gorm:"size:16"` // active, suspect, dead
	Metadata  string `gorm:"type:text"` // JSON
	LastSeen  time.Time
}
