package agent

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════════
//  Proactive Agent — Goal-Driven Autonomous Execution
// ════════════════════════════════════════════════════════════════

// GoalStatus tracks the lifecycle of a goal.
type GoalStatus string

const (
	GoalPending    GoalStatus = "pending"
	GoalActive     GoalStatus = "active"
	GoalCompleted  GoalStatus = "completed"
	GoalFailed     GoalStatus = "failed"
	GoalCancelled  GoalStatus = "cancelled"
)

// Goal represents a high-level objective that an agent pursues autonomously.
type Goal struct {
	ID            string     `json:"id" gorm:"type:varchar(36);primaryKey"`
	AgentID       string     `json:"agent_id" gorm:"type:varchar(36);index;not null"`
	UserID        string     `json:"user_id" gorm:"type:varchar(36);index;not null"`
	Title         string     `json:"title" gorm:"type:varchar(500);not null"`
	Description   string     `json:"description" gorm:"type:text"`
	Status        GoalStatus `json:"status" gorm:"type:varchar(20);default:pending;index"`
	Priority      int        `json:"priority" gorm:"default:5"` // 1-10, higher = more important
	Deadline      *time.Time `json:"deadline"`
	TriggerType   string     `json:"trigger_type" gorm:"type:varchar(50)"` // schedule, event, condition, manual
	TriggerConfig string     `json:"trigger_config" gorm:"type:json"`      // cron expression, event type, condition JSON
	Progress      float64    `json:"progress" gorm:"default:0"`            // 0.0 - 1.0
	Result        string     `json:"result" gorm:"type:text"`
	ErrorMsg      string     `json:"error_msg" gorm:"type:text"`
	MaxSteps      int        `json:"max_steps" gorm:"default:20"`
	StepsUsed     int        `json:"steps_used" gorm:"default:0"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	CompletedAt   *time.Time `json:"completed_at"`
}

func (g *Goal) BeforeCreate(tx *gorm.DB) error {
	if g.ID == "" {
		g.ID = uuid.New().String()
	}
	return nil
}

// GoalStep represents a single step in pursuing a goal.
type GoalStep struct {
	ID          string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	GoalID      string    `json:"goal_id" gorm:"type:varchar(36);index;not null"`
	StepNumber  int       `json:"step_number" gorm:"not null"`
	Action      string    `json:"action" gorm:"type:varchar(200);not null"` // think, tool_call, sub_goal, decide, report
	Description string    `json:"description" gorm:"type:text"`
	Input       string    `json:"input" gorm:"type:json"`
	Output      string    `json:"output" gorm:"type:text"`
	Status      string    `json:"status" gorm:"type:varchar(20);default:pending"` // pending, running, done, error, skipped
	DurationMs  int64     `json:"duration_ms"`
	CreatedAt   time.Time `json:"created_at"`
}

func (s *GoalStep) BeforeCreate(tx *gorm.DB) error {
	if s.ID == "" {
		s.ID = uuid.New().String()
	}
	return nil
}

// ════════════════════════════════════════════════════════════════
//  Goal Decomposition — LLM-powered plan generation
// ════════════════════════════════════════════════════════════════

// DecomposedPlan is the output of goal decomposition.
type DecomposedPlan struct {
	Steps     []PlannedStep `json:"steps"`
	Reasoning string        `json:"reasoning"`
}

// PlannedStep is a single planned action.
type PlannedStep struct {
	Action      string `json:"action"`
	Description string `json:"description"`
	Tool        string `json:"tool,omitempty"`
	DependsOn   []int  `json:"depends_on,omitempty"` // indices of prerequisite steps
}

// GetGoalDecompositionPrompt returns the system prompt for goal decomposition.
func GetGoalDecompositionPrompt() string {
	return `You are a goal decomposition engine. Given a high-level goal, break it down into concrete, actionable steps.

Return a JSON object:
{
  "reasoning": "your analysis of the goal and approach",
  "steps": [
    {"action": "think|tool_call|sub_goal|decide|report", "description": "what to do", "tool": "tool_name_if_applicable", "depends_on": []}
  ]
}

Actions:
- think: Analyze information or reason about the next step
- tool_call: Use a specific tool (web_search, code, http_request, etc.)
- sub_goal: Create a nested sub-goal for complex sub-tasks
- decide: Make a decision based on gathered information
- report: Summarize findings and report to the user

Keep plans concise (5-15 steps). Ensure dependencies are correct.`
}

// ParseDecomposedPlan parses LLM output into a plan.
func ParseDecomposedPlan(llmOutput string) (DecomposedPlan, error) {
	// Extract JSON from output
	output := llmOutput
	if idx := findJSONStart(output); idx >= 0 {
		output = output[idx:]
	}
	if idx := findJSONEnd(output); idx >= 0 {
		output = output[:idx+1]
	}

	var plan DecomposedPlan
	err := json.Unmarshal([]byte(output), &plan)
	return plan, err
}

func findJSONStart(s string) int {
	for i, c := range s {
		if c == '{' {
			return i
		}
	}
	return -1
}

func findJSONEnd(s string) int {
	depth := 0
	for i, c := range s {
		if c == '{' {
			depth++
		} else if c == '}' {
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// ════════════════════════════════════════════════════════════════
//  Proactive Engine — manages goal lifecycle
// ════════════════════════════════════════════════════════════════

// TriggerEvaluator checks if a goal's trigger condition is met.
type TriggerEvaluator func(goal Goal) bool

// ProactiveEngine manages autonomous goal pursuit.
type ProactiveEngine struct {
	db     *gorm.DB
	stopCh chan struct{}
	wg     sync.WaitGroup

	mu         sync.RWMutex
	evaluators map[string]TriggerEvaluator // trigger_type -> evaluator
}

// NewProactiveEngine creates the engine.
func NewProactiveEngine(db *gorm.DB) *ProactiveEngine {
	e := &ProactiveEngine{
		db:         db,
		stopCh:     make(chan struct{}),
		evaluators: make(map[string]TriggerEvaluator),
	}

	// Register default trigger evaluators
	e.evaluators["schedule"] = scheduleEvaluator
	e.evaluators["manual"] = func(_ Goal) bool { return false } // manual triggers don't auto-fire

	return e
}

// Start begins the background trigger evaluation loop.
func (e *ProactiveEngine) Start() {
	log.Println("[Proactive] Engine starting...")
	e.wg.Add(1)
	go e.triggerLoop()
}

// Stop gracefully shuts down.
func (e *ProactiveEngine) Stop() {
	close(e.stopCh)
	e.wg.Wait()
	log.Println("[Proactive] Engine stopped")
}

// RegisterTrigger adds a custom trigger evaluator.
func (e *ProactiveEngine) RegisterTrigger(triggerType string, eval TriggerEvaluator) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.evaluators[triggerType] = eval
}

// triggerLoop checks pending goals periodically.
func (e *ProactiveEngine) triggerLoop() {
	defer e.wg.Done()

	select {
	case <-e.stopCh:
		return
	case <-time.After(30 * time.Second):
	}

	log.Println("[Proactive] Trigger evaluation loop started (every 60s)")
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-e.stopCh:
			return
		case <-ticker.C:
			e.evaluateTriggers()
		}
	}
}

// evaluateTriggers checks all pending goals.
func (e *ProactiveEngine) evaluateTriggers() {
	var goals []Goal
	e.db.Where("status = ?", GoalPending).Find(&goals)

	e.mu.RLock()
	defer e.mu.RUnlock()

	for _, goal := range goals {
		eval, ok := e.evaluators[goal.TriggerType]
		if !ok {
			continue
		}
		if eval(goal) {
			// Activate the goal
			e.db.Model(&goal).Update("status", GoalActive)
			log.Printf("[Proactive] Goal activated: %s (%s)", goal.Title, goal.ID)
		}
	}

	// Check for deadline-passed goals
	e.db.Model(&Goal{}).Where("status IN ? AND deadline < ?",
		[]GoalStatus{GoalPending, GoalActive}, time.Now()).
		Updates(map[string]interface{}{"status": GoalFailed, "error_msg": "deadline exceeded"})
}

// scheduleEvaluator checks if a cron-scheduled goal should fire.
func scheduleEvaluator(goal Goal) bool {
	if goal.TriggerConfig == "" {
		return false
	}
	// Simple time-based check: if trigger_config contains an ISO timestamp, check if it's past
	var config struct {
		At string `json:"at"` // ISO timestamp
	}
	if err := json.Unmarshal([]byte(goal.TriggerConfig), &config); err != nil {
		return false
	}
	t, err := time.Parse(time.RFC3339, config.At)
	if err != nil {
		return false
	}
	return time.Now().After(t)
}

// ── Goal Management ──

// CreateGoal adds a new goal.
func (e *ProactiveEngine) CreateGoal(goal *Goal) error {
	if goal.TriggerConfig == "" {
		goal.TriggerConfig = "{}"
	}
	return e.db.Create(goal).Error
}

// GetGoal retrieves a goal by ID.
func (e *ProactiveEngine) GetGoal(id string) (*Goal, error) {
	var goal Goal
	err := e.db.Where("id = ?", id).First(&goal).Error
	return &goal, err
}

// ListGoals returns goals for a user.
func (e *ProactiveEngine) ListGoals(userID string, status GoalStatus, page, pageSize int) ([]Goal, int64) {
	q := e.db.Model(&Goal{}).Where("user_id = ?", userID)
	if status != "" {
		q = q.Where("status = ?", status)
	}

	var total int64
	q.Count(&total)

	var goals []Goal
	q.Order("priority DESC, created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&goals)
	return goals, total
}

// CancelGoal cancels a goal.
func (e *ProactiveEngine) CancelGoal(id, userID string) error {
	return e.db.Model(&Goal{}).Where("id = ? AND user_id = ? AND status IN ?",
		id, userID, []GoalStatus{GoalPending, GoalActive}).
		Update("status", GoalCancelled).Error
}

// RecordStep records a step execution for a goal.
func (e *ProactiveEngine) RecordStep(goalID string, step *GoalStep) error {
	step.GoalID = goalID
	if err := e.db.Create(step).Error; err != nil {
		return err
	}

	// Update goal progress
	var goal Goal
	if err := e.db.Where("id = ?", goalID).First(&goal).Error; err != nil {
		return err
	}

	goal.StepsUsed++
	if goal.MaxSteps > 0 {
		goal.Progress = float64(goal.StepsUsed) / float64(goal.MaxSteps)
		if goal.Progress > 1.0 {
			goal.Progress = 1.0
		}
	}
	return e.db.Save(&goal).Error
}

// CompleteGoal marks a goal as completed.
func (e *ProactiveEngine) CompleteGoal(id, result string) error {
	now := time.Now()
	return e.db.Model(&Goal{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":       GoalCompleted,
		"progress":     1.0,
		"result":       result,
		"completed_at": now,
	}).Error
}

// GetSteps returns all steps for a goal.
func (e *ProactiveEngine) GetSteps(goalID string) ([]GoalStep, error) {
	var steps []GoalStep
	err := e.db.Where("goal_id = ?", goalID).Order("step_number ASC").Find(&steps).Error
	return steps, err
}

// Stats returns proactive engine statistics.
func (e *ProactiveEngine) Stats(userID string) map[string]interface{} {
	q := e.db.Model(&Goal{})
	if userID != "" {
		q = q.Where("user_id = ?", userID)
	}

	var total, pending, active, completed, failed int64
	q.Count(&total)
	e.db.Model(&Goal{}).Where("user_id = ? AND status = ?", userID, GoalPending).Count(&pending)
	e.db.Model(&Goal{}).Where("user_id = ? AND status = ?", userID, GoalActive).Count(&active)
	e.db.Model(&Goal{}).Where("user_id = ? AND status = ?", userID, GoalCompleted).Count(&completed)
	e.db.Model(&Goal{}).Where("user_id = ? AND status = ?", userID, GoalFailed).Count(&failed)

	return map[string]interface{}{
		"total_goals":     total,
		"pending":         pending,
		"active":          active,
		"completed":       completed,
		"failed":          failed,
		"completion_rate": safeDiv64(float64(completed), float64(total)),
	}
}

func safeDiv64(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return a / b
}

// ════════════════════════════════════════════════════════════════
//  Agent Collaboration Protocol (A2A v2)
// ════════════════════════════════════════════════════════════════

// CollaborationRole defines an agent's role in a multi-agent collaboration.
type CollaborationRole string

const (
	RoleLeader    CollaborationRole = "leader"
	RoleWorker    CollaborationRole = "worker"
	RoleReviewer  CollaborationRole = "reviewer"
	RoleObserver  CollaborationRole = "observer"
)

// Collaboration represents a multi-agent collaboration session.
type Collaboration struct {
	ID          string            `json:"id" gorm:"type:varchar(36);primaryKey"`
	GoalID      string            `json:"goal_id" gorm:"type:varchar(36);index"`
	Title       string            `json:"title" gorm:"type:varchar(500);not null"`
	Status      string            `json:"status" gorm:"type:varchar(20);default:forming"` // forming, active, consensus, completed, failed
	Protocol    string            `json:"protocol" gorm:"type:varchar(50);default:consensus"` // consensus, delegation, auction, voting
	CreatorID   string            `json:"creator_id" gorm:"type:varchar(36);index;not null"`
	MaxAgents   int               `json:"max_agents" gorm:"default:5"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

func (c *Collaboration) BeforeCreate(tx *gorm.DB) error {
	if c.ID == "" {
		c.ID = uuid.New().String()
	}
	return nil
}

// CollaborationMember represents an agent participating in a collaboration.
type CollaborationMember struct {
	ID              string            `json:"id" gorm:"type:varchar(36);primaryKey"`
	CollaborationID string            `json:"collaboration_id" gorm:"type:varchar(36);index;not null"`
	AgentID         string            `json:"agent_id" gorm:"type:varchar(36);index;not null"`
	Role            CollaborationRole `json:"role" gorm:"type:varchar(20);not null"`
	Capabilities    string            `json:"capabilities" gorm:"type:json"` // what this agent can do
	Vote            string            `json:"vote" gorm:"type:varchar(50)"`  // for consensus protocol
	Output          string            `json:"output" gorm:"type:text"`       // agent's contribution
	JoinedAt        time.Time         `json:"joined_at"`
}

func (m *CollaborationMember) BeforeCreate(tx *gorm.DB) error {
	if m.ID == "" {
		m.ID = uuid.New().String()
	}
	return nil
}

// CollaborationMessage is a message exchanged between agents in a collaboration.
type CollaborationMessage struct {
	ID              string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	CollaborationID string    `json:"collaboration_id" gorm:"type:varchar(36);index;not null"`
	SenderAgentID   string    `json:"sender_agent_id" gorm:"type:varchar(36);index"`
	MessageType     string    `json:"message_type" gorm:"type:varchar(50)"` // proposal, vote, result, question, answer, delegate
	Content         string    `json:"content" gorm:"type:text;not null"`
	CreatedAt       time.Time `json:"created_at" gorm:"index"`
}

func (m *CollaborationMessage) BeforeCreate(tx *gorm.DB) error {
	if m.ID == "" {
		m.ID = uuid.New().String()
	}
	return nil
}

// CollaborationEngine manages multi-agent collaborations.
type CollaborationEngine struct {
	db *gorm.DB
}

// NewCollaborationEngine creates the engine.
func NewCollaborationEngine(db *gorm.DB) *CollaborationEngine {
	return &CollaborationEngine{db: db}
}

// CreateCollaboration starts a new multi-agent collaboration.
func (e *CollaborationEngine) CreateCollaboration(ctx context.Context, collab *Collaboration) error {
	return e.db.Create(collab).Error
}

// JoinCollaboration adds an agent to a collaboration.
func (e *CollaborationEngine) JoinCollaboration(collabID, agentID string, role CollaborationRole, capabilities string) error {
	member := CollaborationMember{
		CollaborationID: collabID,
		AgentID:         agentID,
		Role:            role,
		Capabilities:    capabilities,
		JoinedAt:        time.Now(),
	}
	return e.db.Create(&member).Error
}

// SendMessage sends a message in a collaboration.
func (e *CollaborationEngine) SendMessage(collabID, senderAgentID, msgType, content string) error {
	msg := CollaborationMessage{
		CollaborationID: collabID,
		SenderAgentID:   senderAgentID,
		MessageType:     msgType,
		Content:         content,
	}
	return e.db.Create(&msg).Error
}

// GetMessages retrieves collaboration messages.
func (e *CollaborationEngine) GetMessages(collabID string) ([]CollaborationMessage, error) {
	var msgs []CollaborationMessage
	err := e.db.Where("collaboration_id = ?", collabID).Order("created_at ASC").Find(&msgs).Error
	return msgs, err
}

// GetMembers retrieves collaboration members.
func (e *CollaborationEngine) GetMembers(collabID string) ([]CollaborationMember, error) {
	var members []CollaborationMember
	err := e.db.Where("collaboration_id = ?", collabID).Find(&members).Error
	return members, err
}

// SubmitVote records an agent's vote in a consensus protocol.
func (e *CollaborationEngine) SubmitVote(collabID, agentID, vote string) error {
	return e.db.Model(&CollaborationMember{}).
		Where("collaboration_id = ? AND agent_id = ?", collabID, agentID).
		Update("vote", vote).Error
}

// CheckConsensus checks if consensus has been reached.
func (e *CollaborationEngine) CheckConsensus(collabID string) (bool, string) {
	var members []CollaborationMember
	e.db.Where("collaboration_id = ?", collabID).Find(&members)

	if len(members) == 0 {
		return false, ""
	}

	votes := make(map[string]int)
	totalVoted := 0
	for _, m := range members {
		if m.Vote != "" {
			votes[m.Vote]++
			totalVoted++
		}
	}

	// Require majority (>50%) to reach consensus
	threshold := len(members) / 2
	for vote, count := range votes {
		if count > threshold {
			return true, vote
		}
	}

	return false, ""
}
