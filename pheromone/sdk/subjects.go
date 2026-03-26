package sdk

// Standard event subjects used across StarClaw services.
// All events are published under pheromone.events.{subject}.
const (
	// --- Nydus deploy events ---
	SubjectDeployStarted   = "nydus.deploy.started"
	SubjectDeployCompleted = "nydus.deploy.completed"
	SubjectDeployFailed    = "nydus.deploy.failed"

	// --- Hive instance events ---
	SubjectInstanceCreated  = "hive.instance.created"
	SubjectInstanceDeleted  = "hive.instance.deleted"
	SubjectInstanceStarted  = "hive.instance.started"
	SubjectInstanceStopped  = "hive.instance.stopped"
	SubjectInstanceError    = "hive.instance.error"

	// --- Queen user/billing events ---
	SubjectUserCreated  = "queen.user.created"
	SubjectUserUpgraded = "queen.user.upgraded"
	SubjectPayment      = "queen.billing.payment"

	// --- Forge CI events ---
	SubjectBuildStarted   = "forge.build.started"
	SubjectBuildCompleted = "forge.build.completed"
	SubjectBuildFailed    = "forge.build.failed"

	// --- Synapse usage events ---
	SubjectUsageAlert = "synapse.usage.alert"
)

// Standard RPC method names.
const (
	RPCCheckCredit   = "check-credit"
	RPCDeductCredit  = "deduct-credit"
	RPCGetUser       = "get-user"
	RPCListInstances = "list-instances"
	RPCGetUsage      = "get-usage"
	RPCDeployStatus  = "deploy-status"
)

// DeployEvent is published when a deployment starts, completes, or fails.
type DeployEvent struct {
	Service  string `json:"service"`
	Branch   string `json:"branch,omitempty"`
	Commit   string `json:"commit,omitempty"`
	Status   string `json:"status"` // started | completed | failed
	Duration string `json:"duration,omitempty"`
	Error    string `json:"error,omitempty"`
	Actor    string `json:"actor,omitempty"`
}

// InstanceEvent is published when a Hive instance changes state.
type InstanceEvent struct {
	InstanceID string `json:"instance_id"`
	UserID     string `json:"user_id,omitempty"`
	Domain     string `json:"domain,omitempty"`
	Status     string `json:"status"`
	Plan       string `json:"plan,omitempty"`
	Error      string `json:"error,omitempty"`
}

// UserEvent is published when a user-related action occurs in Queen.
type UserEvent struct {
	UserID   string `json:"user_id"`
	Username string `json:"username,omitempty"`
	Action   string `json:"action"` // created | upgraded | deleted
	Plan     string `json:"plan,omitempty"`
}

// BuildEvent is published when a Forge CI build changes state.
type BuildEvent struct {
	ProjectID string `json:"project_id"`
	BuildID   string `json:"build_id"`
	Status    string `json:"status"` // started | completed | failed
	Branch    string `json:"branch,omitempty"`
	Duration  string `json:"duration,omitempty"`
	Error     string `json:"error,omitempty"`
}

// CreditRequest is the payload for check-credit / deduct-credit RPC.
type CreditRequest struct {
	UserID string  `json:"user_id"`
	Amount float64 `json:"amount,omitempty"`
	Reason string  `json:"reason,omitempty"`
}

// CreditResponse is returned by check-credit / deduct-credit RPC.
type CreditResponse struct {
	UserID  string  `json:"user_id"`
	Balance float64 `json:"balance"`
	OK      bool    `json:"ok"`
}
