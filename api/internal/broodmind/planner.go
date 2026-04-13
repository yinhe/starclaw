package broodmind

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// ════════════════════════════════════════════════════════════
// BroodMind v3 — Planner Engine (深度认知规划)
//
// 让虫群真正"思考":
//   1. Goal Decomposition — 目标自主分解为子任务树
//   2. Resource Selection — 自动选择模型/工具/Agent/节点
//   3. Execution Planning — 生成DAG执行计划 + 依赖解析
//   4. Reflection Loop — 执行后反思 + 经验固化
//   5. Adaptive Re-plan — 失败时自适应重规划
//
// ════════════════════════════════════════════════════════════

// ── Types ──

type GoalStatus string

const (
	GoalPending    GoalStatus = "pending"
	GoalPlanning   GoalStatus = "planning"
	GoalExecuting  GoalStatus = "executing"
	GoalReflecting GoalStatus = "reflecting"
	GoalCompleted  GoalStatus = "completed"
	GoalFailed     GoalStatus = "failed"
	GoalCancelled  GoalStatus = "cancelled"
)

type StepStatus string

const (
	StepPending   StepStatus = "pending"
	StepReady     StepStatus = "ready"     // all deps satisfied
	StepRunning   StepStatus = "running"
	StepCompleted StepStatus = "completed"
	StepFailed    StepStatus = "failed"
	StepSkipped   StepStatus = "skipped"
)

// ResourceType classifies what kind of resource a step needs.
type ResourceType string

const (
	ResModel  ResourceType = "model"   // LLM model
	ResTool   ResourceType = "tool"    // MCP tool / function
	ResAgent  ResourceType = "agent"   // sub-agent
	ResWorker ResourceType = "worker"  // Nerve Bus worker bee
	ResNode   ResourceType = "node"    // remote node
	ResHuman  ResourceType = "human"   // human-in-the-loop
)

// ResourceSelection represents a chosen resource for a step.
type ResourceSelection struct {
	Type       ResourceType `json:"type"`
	ID         string       `json:"id"`         // model name, tool name, agent id, etc.
	Reason     string       `json:"reason"`     // why this resource was selected
	Confidence float64      `json:"confidence"` // 0-1 selection confidence
	Fallback   string       `json:"fallback"`   // fallback resource ID
}

// PlanStep represents one step in an execution plan.
type PlanStep struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	Description string              `json:"description"`
	DependsOn   []string            `json:"depends_on,omitempty"` // step IDs
	Resources   []ResourceSelection `json:"resources"`
	Status      StepStatus          `json:"status"`
	Priority    int                 `json:"priority"` // 0=highest
	Retries     int                 `json:"retries"`
	MaxRetries  int                 `json:"max_retries"`
	Input       map[string]interface{} `json:"input,omitempty"`
	Output      map[string]interface{} `json:"output,omitempty"`
	Error       string              `json:"error,omitempty"`
	StartedAt   *time.Time          `json:"started_at,omitempty"`
	CompletedAt *time.Time          `json:"completed_at,omitempty"`
	DurationMs  int64               `json:"duration_ms,omitempty"`
}

// Goal represents a high-level objective that the planner decomposes.
type Goal struct {
	ID          string                 `json:"id"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	Context     string                 `json:"context,omitempty"` // conversation/task context
	Constraints []string               `json:"constraints,omitempty"`
	Status      GoalStatus             `json:"status"`
	Steps       []*PlanStep            `json:"steps"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	AgentID     string                 `json:"agent_id,omitempty"`
	UserID      string                 `json:"user_id,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	Reflection  *GoalReflection        `json:"reflection,omitempty"`
	ReplanCount int                    `json:"replan_count"`
}

// GoalReflection captures post-execution analysis.
type GoalReflection struct {
	Success      bool     `json:"success"`
	Summary      string   `json:"summary"`
	Lessons      []string `json:"lessons"`
	Improvements []string `json:"improvements"`
	Quality      float64  `json:"quality"` // 0-1
	ReflectedAt  time.Time `json:"reflected_at"`
}

// ── Planner Engine ──

type PlannerConfig struct {
	MaxStepsPerGoal int     `json:"max_steps_per_goal"`
	MaxReplanCount  int     `json:"max_replan_count"`
	DefaultRetries  int     `json:"default_retries"`
	QualityFloor    float64 `json:"quality_floor"` // min reflection quality to store lesson
}

func DefaultPlannerConfig() *PlannerConfig {
	return &PlannerConfig{
		MaxStepsPerGoal: 20,
		MaxReplanCount:  3,
		DefaultRetries:  2,
		QualityFloor:    0.6,
	}
}

type Planner struct {
	mu     sync.RWMutex
	goals  map[string]*Goal
	config *PlannerConfig
	stats  PlannerStats
	nextID int
}

type PlannerStats struct {
	GoalsCreated   int     `json:"goals_created"`
	GoalsCompleted int     `json:"goals_completed"`
	GoalsFailed    int     `json:"goals_failed"`
	StepsExecuted  int     `json:"steps_executed"`
	StepsFailed    int     `json:"steps_failed"`
	Replans        int     `json:"replans"`
	AvgStepsPerGoal float64 `json:"avg_steps_per_goal"`
	AvgQuality     float64 `json:"avg_quality"`
	Uptime         string  `json:"uptime"`
}

func NewPlanner(cfg *PlannerConfig) *Planner {
	if cfg == nil {
		cfg = DefaultPlannerConfig()
	}
	return &Planner{
		goals:  make(map[string]*Goal),
		config: cfg,
	}
}

func (p *Planner) genID(prefix string) string {
	p.nextID++
	return fmt.Sprintf("%s-%d-%d", prefix, time.Now().UnixMilli(), p.nextID)
}

// ── Goal Lifecycle ──

// CreateGoal creates a new goal and decomposes it into steps.
func (p *Planner) CreateGoal(title, description, context, agentID, userID string, constraints []string) *Goal {
	p.mu.Lock()
	defer p.mu.Unlock()

	g := &Goal{
		ID:          p.genID("goal"),
		Title:       title,
		Description: description,
		Context:     context,
		Constraints: constraints,
		Status:      GoalPending,
		Steps:       make([]*PlanStep, 0),
		AgentID:     agentID,
		UserID:      userID,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Metadata:    make(map[string]interface{}),
	}
	p.goals[g.ID] = g
	p.stats.GoalsCreated++

	log.Printf("[broodmind/planner] goal created: %s — %s", g.ID, title)
	return g
}

// Decompose breaks a goal into ordered plan steps.
// In v3 this is a structured decomposition; future versions will use LLM.
func (p *Planner) Decompose(goalID string, steps []PlanStep) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	g, ok := p.goals[goalID]
	if !ok {
		return fmt.Errorf("goal %s not found", goalID)
	}
	if g.Status != GoalPending && g.Status != GoalFailed {
		return fmt.Errorf("goal %s is not pending/failed (status=%s)", goalID, g.Status)
	}

	if len(steps) > p.config.MaxStepsPerGoal {
		return fmt.Errorf("too many steps (%d > max %d)", len(steps), p.config.MaxStepsPerGoal)
	}

	g.Steps = make([]*PlanStep, len(steps))
	for i := range steps {
		s := steps[i]
		if s.ID == "" {
			s.ID = p.genID("step")
		}
		if s.Status == "" {
			s.Status = StepPending
		}
		if s.MaxRetries == 0 {
			s.MaxRetries = p.config.DefaultRetries
		}
		g.Steps[i] = &s
	}

	g.Status = GoalPlanning
	g.UpdatedAt = time.Now()

	// Resolve initial ready steps (those with no deps)
	p.resolveReadySteps(g)

	log.Printf("[broodmind/planner] goal %s decomposed into %d steps", goalID, len(steps))
	return nil
}

// SelectResources assigns resources to a specific step.
func (p *Planner) SelectResources(goalID, stepID string, resources []ResourceSelection) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	g, ok := p.goals[goalID]
	if !ok {
		return fmt.Errorf("goal %s not found", goalID)
	}

	for _, s := range g.Steps {
		if s.ID == stepID {
			s.Resources = resources
			return nil
		}
	}
	return fmt.Errorf("step %s not found in goal %s", stepID, goalID)
}

// StartExecution moves goal to executing status and returns ready steps.
func (p *Planner) StartExecution(goalID string) ([]*PlanStep, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	g, ok := p.goals[goalID]
	if !ok {
		return nil, fmt.Errorf("goal %s not found", goalID)
	}
	if g.Status != GoalPlanning {
		return nil, fmt.Errorf("goal %s is not in planning state (status=%s)", goalID, g.Status)
	}

	g.Status = GoalExecuting
	g.UpdatedAt = time.Now()

	return p.readySteps(g), nil
}

// CompleteStep marks a step as completed with its output.
func (p *Planner) CompleteStep(goalID, stepID string, output map[string]interface{}) ([]*PlanStep, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	g, ok := p.goals[goalID]
	if !ok {
		return nil, fmt.Errorf("goal %s not found", goalID)
	}

	for _, s := range g.Steps {
		if s.ID == stepID {
			now := time.Now()
			s.Status = StepCompleted
			s.Output = output
			s.CompletedAt = &now
			if s.StartedAt != nil {
				s.DurationMs = now.Sub(*s.StartedAt).Milliseconds()
			}
			p.stats.StepsExecuted++
			break
		}
	}

	g.UpdatedAt = time.Now()

	// Resolve newly ready steps
	p.resolveReadySteps(g)
	ready := p.readySteps(g)

	// Check if all steps are done
	if p.allStepsDone(g) {
		g.Status = GoalReflecting
		log.Printf("[broodmind/planner] goal %s all steps done → reflecting", goalID)
	}

	return ready, nil
}

// FailStep marks a step as failed and optionally triggers re-plan.
func (p *Planner) FailStep(goalID, stepID, errMsg string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	g, ok := p.goals[goalID]
	if !ok {
		return fmt.Errorf("goal %s not found", goalID)
	}

	for _, s := range g.Steps {
		if s.ID == stepID {
			s.Retries++
			p.stats.StepsFailed++
			if s.Retries <= s.MaxRetries {
				s.Status = StepReady // retry
				s.Error = fmt.Sprintf("retry %d/%d: %s", s.Retries, s.MaxRetries, errMsg)
				log.Printf("[broodmind/planner] step %s retrying (%d/%d)", stepID, s.Retries, s.MaxRetries)
				return nil
			}
			s.Status = StepFailed
			s.Error = errMsg
			now := time.Now()
			s.CompletedAt = &now

			// Can we re-plan?
			if g.ReplanCount < p.config.MaxReplanCount {
				g.ReplanCount++
				g.Status = GoalFailed // allows Decompose again
				p.stats.Replans++
				log.Printf("[broodmind/planner] step %s exhausted retries → goal %s eligible for re-plan (%d/%d)",
					stepID, goalID, g.ReplanCount, p.config.MaxReplanCount)
			} else {
				g.Status = GoalFailed
				now := time.Now()
				g.CompletedAt = &now
				p.stats.GoalsFailed++
				log.Printf("[broodmind/planner] goal %s FAILED (no more re-plans)", goalID)
			}
			break
		}
	}
	return nil
}

// Reflect performs post-execution reflection and stores lessons.
func (p *Planner) Reflect(goalID string, success bool, summary string, lessons, improvements []string, quality float64) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	g, ok := p.goals[goalID]
	if !ok {
		return fmt.Errorf("goal %s not found", goalID)
	}

	now := time.Now()
	g.Reflection = &GoalReflection{
		Success:      success,
		Summary:      summary,
		Lessons:      lessons,
		Improvements: improvements,
		Quality:      quality,
		ReflectedAt:  now,
	}

	if success {
		g.Status = GoalCompleted
		p.stats.GoalsCompleted++
	} else {
		g.Status = GoalFailed
		p.stats.GoalsFailed++
	}
	g.CompletedAt = &now
	g.UpdatedAt = now

	log.Printf("[broodmind/planner] goal %s reflected: success=%v quality=%.2f lessons=%d",
		goalID, success, quality, len(lessons))
	return nil
}

// CancelGoal cancels a goal.
func (p *Planner) CancelGoal(goalID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	g, ok := p.goals[goalID]
	if !ok {
		return fmt.Errorf("goal %s not found", goalID)
	}
	g.Status = GoalCancelled
	now := time.Now()
	g.CompletedAt = &now
	g.UpdatedAt = now
	return nil
}

// ── Queries ──

func (p *Planner) GetGoal(id string) *Goal {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.goals[id]
}

func (p *Planner) ListGoals(status string, limit int) []*Goal {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var result []*Goal
	for _, g := range p.goals {
		if status != "" && string(g.Status) != status {
			continue
		}
		result = append(result, g)
	}
	if limit > 0 && len(result) > limit {
		result = result[len(result)-limit:]
	}
	return result
}

func (p *Planner) Stats() PlannerStats {
	p.mu.RLock()
	defer p.mu.RUnlock()
	s := p.stats
	if s.GoalsCompleted > 0 {
		// Compute avg steps and quality
		totalSteps := 0
		totalQ := 0.0
		counted := 0
		for _, g := range p.goals {
			if g.Status == GoalCompleted {
				totalSteps += len(g.Steps)
				if g.Reflection != nil {
					totalQ += g.Reflection.Quality
					counted++
				}
			}
		}
		if s.GoalsCompleted > 0 {
			s.AvgStepsPerGoal = float64(totalSteps) / float64(s.GoalsCompleted)
		}
		if counted > 0 {
			s.AvgQuality = totalQ / float64(counted)
		}
	}
	return s
}

// ── Internal helpers ──

func (p *Planner) resolveReadySteps(g *Goal) {
	completedIDs := make(map[string]bool)
	for _, s := range g.Steps {
		if s.Status == StepCompleted {
			completedIDs[s.ID] = true
		}
	}

	for _, s := range g.Steps {
		if s.Status != StepPending {
			continue
		}
		allDepsMet := true
		for _, dep := range s.DependsOn {
			if !completedIDs[dep] {
				allDepsMet = false
				break
			}
		}
		if allDepsMet {
			s.Status = StepReady
		}
	}
}

func (p *Planner) readySteps(g *Goal) []*PlanStep {
	var ready []*PlanStep
	for _, s := range g.Steps {
		if s.Status == StepReady {
			ready = append(ready, s)
		}
	}
	return ready
}

func (p *Planner) allStepsDone(g *Goal) bool {
	for _, s := range g.Steps {
		if s.Status != StepCompleted && s.Status != StepFailed && s.Status != StepSkipped {
			return false
		}
	}
	return len(g.Steps) > 0
}
