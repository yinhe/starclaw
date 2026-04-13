package abathur

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// ════════════════════════════════════════════════════════════
// Abathur v1 — 进化编排引擎
//
// 职责:
//   1. Evolution Plan: 分析系统状态 → 生成优先级排序的进化计划
//   2. Sprint: 将计划拆解为工蜂任务 → 调度执行 → 追踪完成
//   3. Worker Dispatch: 分配任务给 5 类工蜂 (sense/scout/dev/test/ops)
//   4. Hotfix Triage: 紧急问题评估 → 快速修复调度
//   5. Capability Assessment: 能力差距分析 → 推荐新技能/工具
//   6. Experience Distillation: 从完成的冲刺中提炼经验
// ════════════════════════════════════════════════════════════

// ── Types ──

type PlanStatus string

const (
	PlanDraft    PlanStatus = "draft"
	PlanPending  PlanStatus = "pending_approval"
	PlanApproved PlanStatus = "approved"
	PlanActive   PlanStatus = "active"
	PlanDone     PlanStatus = "completed"
	PlanRejected PlanStatus = "rejected"
)

type SprintStatus string

const (
	SprintPlanning SprintStatus = "planning"
	SprintActive   SprintStatus = "active"
	SprintReview   SprintStatus = "review"
	SprintDone     SprintStatus = "completed"
	SprintAborted  SprintStatus = "aborted"
)

type TaskStatus string

const (
	TaskPending    TaskStatus = "pending"
	TaskAssigned   TaskStatus = "assigned"
	TaskInProgress TaskStatus = "in_progress"
	TaskBlocked    TaskStatus = "blocked"
	TaskDone       TaskStatus = "completed"
	TaskFailed     TaskStatus = "failed"
	TaskSkipped    TaskStatus = "skipped"
)

type TaskPriority string

const (
	PriorityP0 TaskPriority = "P0" // Critical: immediate
	PriorityP1 TaskPriority = "P1" // High: same sprint
	PriorityP2 TaskPriority = "P2" // Medium: next sprint
	PriorityP3 TaskPriority = "P3" // Low: backlog
)

type WorkerType string

const (
	WorkerSense WorkerType = "sense_claw"
	WorkerScout WorkerType = "scout_claw"
	WorkerDev   WorkerType = "dev_team"
	WorkerTest  WorkerType = "test_claw"
	WorkerOps   WorkerType = "ops_claw"
)

type HotfixSeverity string

const (
	SevP0 HotfixSeverity = "P0" // Service down
	SevP1 HotfixSeverity = "P1" // Major feature broken
	SevP2 HotfixSeverity = "P2" // Minor feature broken
	SevP3 HotfixSeverity = "P3" // Cosmetic / nice-to-have
)

// ── Data Structures ──

type EvolutionPlan struct {
	ID              string     `json:"id"`
	Title           string     `json:"title"`
	Description     string     `json:"description"`
	Status          PlanStatus `json:"status"`
	Goals           []string   `json:"goals"`
	Tasks           []PlanTask `json:"tasks"`
	SuccessCriteria []string   `json:"success_criteria"`
	RollbackPlan    string     `json:"rollback_plan,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	ApprovedAt      *time.Time `json:"approved_at,omitempty"`
	ApprovedBy      string     `json:"approved_by,omitempty"`
}

type PlanTask struct {
	Title    string       `json:"title"`
	Assignee WorkerType   `json:"assignee"`
	Priority TaskPriority `json:"priority"`
	Estimate string       `json:"estimate"` // e.g. "2h", "1d"
}

type Sprint struct {
	ID        string       `json:"id"`
	PlanID    string       `json:"plan_id"`
	Title     string       `json:"title"`
	Status    SprintStatus `json:"status"`
	Tasks     []Task       `json:"tasks"`
	StartedAt *time.Time   `json:"started_at,omitempty"`
	EndedAt   *time.Time   `json:"ended_at,omitempty"`
	Report    string       `json:"report,omitempty"`
}

type Task struct {
	ID          string       `json:"id"`
	SprintID    string       `json:"sprint_id"`
	Title       string       `json:"title"`
	Description string       `json:"description,omitempty"`
	Assignee    WorkerType   `json:"assignee"`
	Priority    TaskPriority `json:"priority"`
	Status      TaskStatus   `json:"status"`
	Result      string       `json:"result,omitempty"`
	Error       string       `json:"error,omitempty"`
	CreatedAt   time.Time    `json:"created_at"`
	StartedAt   *time.Time   `json:"started_at,omitempty"`
	CompletedAt *time.Time   `json:"completed_at,omitempty"`
}

type HotfixRequest struct {
	ID          string         `json:"id"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Severity    HotfixSeverity `json:"severity"`
	Source      string         `json:"source"` // who reported
	Status      TaskStatus     `json:"status"`
	AssignedTo  []WorkerType   `json:"assigned_to"`
	CreatedAt   time.Time      `json:"created_at"`
	ResolvedAt  *time.Time     `json:"resolved_at,omitempty"`
	Resolution  string         `json:"resolution,omitempty"`
}

type Lesson struct {
	ID        string    `json:"id"`
	SprintID  string    `json:"sprint_id"`
	Type      string    `json:"type"` // success_pattern, failure_lesson, optimization, tool_gap
	Content   string    `json:"content"`
	Impact    string    `json:"impact,omitempty"`    // high, medium, low
	CreatedAt time.Time `json:"created_at"`
}

type CapabilityGap struct {
	Area        string `json:"area"`
	Description string `json:"description"`
	Severity    string `json:"severity"` // critical, moderate, minor
	Suggestion  string `json:"suggestion"`
}

// ── Engine ──

type EngineConfig struct {
	EvolutionFreq      string `json:"evolution_frequency"` // daily, weekly, biweekly
	AutoDeploy         bool   `json:"auto_deploy"`
	RiskTolerance      string `json:"risk_tolerance"` // conservative, moderate, aggressive
	MaxConcurrentTasks int    `json:"max_concurrent_tasks"`
	HumanApproval      bool   `json:"human_approval_required"`
}

func DefaultEngineConfig() *EngineConfig {
	return &EngineConfig{
		EvolutionFreq:      "weekly",
		AutoDeploy:         false,
		RiskTolerance:      "moderate",
		MaxConcurrentTasks: 5,
		HumanApproval:      true,
	}
}

type Engine struct {
	mu       sync.RWMutex
	nodeID   string
	config   *EngineConfig
	plans    map[string]*EvolutionPlan
	sprints  map[string]*Sprint
	tasks    map[string]*Task
	hotfixes map[string]*HotfixRequest
	lessons  []Lesson
	stats    EngineStats
	startAt  time.Time
	nextID   int
}

type EngineStats struct {
	PlansCreated     int       `json:"plans_created"`
	PlansApproved    int       `json:"plans_approved"`
	PlansRejected    int       `json:"plans_rejected"`
	SprintsCompleted int       `json:"sprints_completed"`
	SprintsAborted   int       `json:"sprints_aborted"`
	TasksDispatched  int       `json:"tasks_dispatched"`
	TasksCompleted   int       `json:"tasks_completed"`
	TasksFailed      int       `json:"tasks_failed"`
	HotfixesTriaged  int       `json:"hotfixes_triaged"`
	HotfixesResolved int       `json:"hotfixes_resolved"`
	LessonsDistilled int       `json:"lessons_distilled"`
	Uptime           string    `json:"uptime"`
	LastActivity     time.Time `json:"last_activity,omitempty"`
}

var (
	globalEngine *Engine
	engineOnce   sync.Once
)

func InitEngine(nodeID string, cfg *EngineConfig) *Engine {
	if cfg == nil {
		cfg = DefaultEngineConfig()
	}
	engineOnce.Do(func() {
		globalEngine = &Engine{
			nodeID:   nodeID,
			config:   cfg,
			plans:    make(map[string]*EvolutionPlan),
			sprints:  make(map[string]*Sprint),
			tasks:    make(map[string]*Task),
			hotfixes: make(map[string]*HotfixRequest),
			lessons:  make([]Lesson, 0),
			startAt:  time.Now(),
		}
		log.Printf("[abathur] evolution engine ready (freq=%s, risk=%s, approval=%v)",
			cfg.EvolutionFreq, cfg.RiskTolerance, cfg.HumanApproval)
	})
	return globalEngine
}

func GetEngine() *Engine {
	return globalEngine
}

func (e *Engine) genID(prefix string) string {
	e.nextID++
	return fmt.Sprintf("%s-%d-%d", prefix, time.Now().Unix(), e.nextID)
}

// ── Plan Management ──

func (e *Engine) CreatePlan(title, description string, goals []string, tasks []PlanTask, criteria []string, rollback string) *EvolutionPlan {
	e.mu.Lock()
	defer e.mu.Unlock()

	status := PlanDraft
	if !e.config.HumanApproval {
		status = PlanApproved
	} else {
		status = PlanPending
	}

	plan := &EvolutionPlan{
		ID:              e.genID("plan"),
		Title:           title,
		Description:     description,
		Status:          status,
		Goals:           goals,
		Tasks:           tasks,
		SuccessCriteria: criteria,
		RollbackPlan:    rollback,
		CreatedAt:       time.Now(),
	}

	e.plans[plan.ID] = plan
	e.stats.PlansCreated++
	e.stats.LastActivity = time.Now()
	log.Printf("[abathur] plan created: %s — %s (status=%s)", plan.ID, title, status)
	return plan
}

func (e *Engine) ApprovePlan(planID, approver string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	plan, ok := e.plans[planID]
	if !ok {
		return fmt.Errorf("plan %s not found", planID)
	}
	if plan.Status != PlanPending && plan.Status != PlanDraft {
		return fmt.Errorf("plan %s cannot be approved (status=%s)", planID, plan.Status)
	}

	now := time.Now()
	plan.Status = PlanApproved
	plan.ApprovedAt = &now
	plan.ApprovedBy = approver
	e.stats.PlansApproved++
	e.stats.LastActivity = now
	log.Printf("[abathur] plan approved: %s by %s", planID, approver)
	return nil
}

func (e *Engine) RejectPlan(planID, reason string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	plan, ok := e.plans[planID]
	if !ok {
		return fmt.Errorf("plan %s not found", planID)
	}

	plan.Status = PlanRejected
	plan.RollbackPlan = reason
	e.stats.PlansRejected++
	e.stats.LastActivity = time.Now()
	return nil
}

func (e *Engine) ListPlans(status string) []*EvolutionPlan {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*EvolutionPlan, 0)
	for _, p := range e.plans {
		if status == "" || string(p.Status) == status {
			result = append(result, p)
		}
	}
	return result
}

func (e *Engine) GetPlan(planID string) (*EvolutionPlan, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	plan, ok := e.plans[planID]
	if !ok {
		return nil, fmt.Errorf("plan %s not found", planID)
	}
	return plan, nil
}

// ── Sprint Management ──

func (e *Engine) CreateSprint(planID, title string) (*Sprint, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	plan, ok := e.plans[planID]
	if !ok {
		return nil, fmt.Errorf("plan %s not found", planID)
	}
	if plan.Status != PlanApproved {
		return nil, fmt.Errorf("plan %s not approved (status=%s)", planID, plan.Status)
	}

	now := time.Now()
	sprint := &Sprint{
		ID:        e.genID("sprint"),
		PlanID:    planID,
		Title:     title,
		Status:    SprintActive,
		Tasks:     make([]Task, 0),
		StartedAt: &now,
	}

	// Create tasks from plan
	for _, pt := range plan.Tasks {
		task := Task{
			ID:        e.genID("task"),
			SprintID:  sprint.ID,
			Title:     pt.Title,
			Assignee:  pt.Assignee,
			Priority:  pt.Priority,
			Status:    TaskPending,
			CreatedAt: now,
		}
		sprint.Tasks = append(sprint.Tasks, task)
		e.tasks[task.ID] = &sprint.Tasks[len(sprint.Tasks)-1]
		e.stats.TasksDispatched++
	}

	plan.Status = PlanActive
	e.sprints[sprint.ID] = sprint
	e.stats.LastActivity = now
	log.Printf("[abathur] sprint created: %s from plan %s (%d tasks)", sprint.ID, planID, len(sprint.Tasks))
	return sprint, nil
}

func (e *Engine) ListSprints(status string) []*Sprint {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*Sprint, 0)
	for _, s := range e.sprints {
		if status == "" || string(s.Status) == status {
			result = append(result, s)
		}
	}
	return result
}

func (e *Engine) GetSprint(sprintID string) (*Sprint, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	sprint, ok := e.sprints[sprintID]
	if !ok {
		return nil, fmt.Errorf("sprint %s not found", sprintID)
	}
	return sprint, nil
}

func (e *Engine) CompleteSprint(sprintID, report string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	sprint, ok := e.sprints[sprintID]
	if !ok {
		return fmt.Errorf("sprint %s not found", sprintID)
	}

	now := time.Now()
	sprint.Status = SprintDone
	sprint.EndedAt = &now
	sprint.Report = report
	e.stats.SprintsCompleted++
	e.stats.LastActivity = now

	// Mark the parent plan as completed
	if plan, ok := e.plans[sprint.PlanID]; ok {
		plan.Status = PlanDone
	}

	log.Printf("[abathur] sprint completed: %s", sprintID)
	return nil
}

// ── Task Management ──

func (e *Engine) DispatchTask(sprintID, title, description string, assignee WorkerType, priority TaskPriority) (*Task, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	sprint, ok := e.sprints[sprintID]
	if !ok {
		return nil, fmt.Errorf("sprint %s not found", sprintID)
	}
	if sprint.Status != SprintActive {
		return nil, fmt.Errorf("sprint %s not active", sprintID)
	}

	// Concurrency limit
	activeCount := 0
	for _, t := range sprint.Tasks {
		if t.Status == TaskInProgress || t.Status == TaskAssigned {
			activeCount++
		}
	}
	if activeCount >= e.config.MaxConcurrentTasks {
		return nil, fmt.Errorf("max concurrent tasks (%d) reached", e.config.MaxConcurrentTasks)
	}

	now := time.Now()
	task := Task{
		ID:          e.genID("task"),
		SprintID:    sprintID,
		Title:       title,
		Description: description,
		Assignee:    assignee,
		Priority:    priority,
		Status:      TaskAssigned,
		CreatedAt:   now,
		StartedAt:   &now,
	}

	sprint.Tasks = append(sprint.Tasks, task)
	e.tasks[task.ID] = &sprint.Tasks[len(sprint.Tasks)-1]
	e.stats.TasksDispatched++
	e.stats.LastActivity = now
	log.Printf("[abathur] task dispatched: %s → %s (priority=%s)", task.ID, assignee, priority)
	return &task, nil
}

func (e *Engine) UpdateTask(taskID string, status TaskStatus, result, errMsg string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	task, ok := e.tasks[taskID]
	if !ok {
		return fmt.Errorf("task %s not found", taskID)
	}

	now := time.Now()
	task.Status = status
	if result != "" {
		task.Result = result
	}
	if errMsg != "" {
		task.Error = errMsg
	}

	switch status {
	case TaskInProgress:
		task.StartedAt = &now
	case TaskDone:
		task.CompletedAt = &now
		e.stats.TasksCompleted++
	case TaskFailed:
		task.CompletedAt = &now
		e.stats.TasksFailed++
	}

	e.stats.LastActivity = now
	log.Printf("[abathur] task updated: %s → %s", taskID, status)
	return nil
}

func (e *Engine) ListTasks(sprintID string) []Task {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if sprintID != "" {
		sprint, ok := e.sprints[sprintID]
		if !ok {
			return nil
		}
		return sprint.Tasks
	}

	result := make([]Task, 0)
	for _, t := range e.tasks {
		result = append(result, *t)
	}
	return result
}

// ── Hotfix Triage ──

func (e *Engine) TriageHotfix(title, description string, severity HotfixSeverity, source string) *HotfixRequest {
	e.mu.Lock()
	defer e.mu.Unlock()

	var workers []WorkerType
	switch severity {
	case SevP0:
		workers = []WorkerType{WorkerDev, WorkerOps, WorkerSense}
	case SevP1:
		workers = []WorkerType{WorkerDev, WorkerOps}
	case SevP2:
		workers = []WorkerType{WorkerDev}
	case SevP3:
		workers = []WorkerType{WorkerDev}
	}

	hf := &HotfixRequest{
		ID:          e.genID("hotfix"),
		Title:       title,
		Description: description,
		Severity:    severity,
		Source:      source,
		Status:      TaskAssigned,
		AssignedTo:  workers,
		CreatedAt:   time.Now(),
	}

	e.hotfixes[hf.ID] = hf
	e.stats.HotfixesTriaged++
	e.stats.LastActivity = time.Now()
	log.Printf("[abathur] hotfix triaged: %s [%s] — %s → %v", hf.ID, severity, title, workers)
	return hf
}

func (e *Engine) ResolveHotfix(hotfixID, resolution string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	hf, ok := e.hotfixes[hotfixID]
	if !ok {
		return fmt.Errorf("hotfix %s not found", hotfixID)
	}

	now := time.Now()
	hf.Status = TaskDone
	hf.ResolvedAt = &now
	hf.Resolution = resolution
	e.stats.HotfixesResolved++
	e.stats.LastActivity = now
	return nil
}

func (e *Engine) ListHotfixes() []*HotfixRequest {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*HotfixRequest, 0, len(e.hotfixes))
	for _, hf := range e.hotfixes {
		result = append(result, hf)
	}
	return result
}

// ── Capability Assessment ──

func (e *Engine) AssessCapability() []CapabilityGap {
	e.mu.RLock()
	defer e.mu.RUnlock()

	gaps := make([]CapabilityGap, 0)

	// Analyze task failure patterns
	failsByWorker := make(map[WorkerType]int)
	totalByWorker := make(map[WorkerType]int)
	for _, t := range e.tasks {
		totalByWorker[t.Assignee]++
		if t.Status == TaskFailed {
			failsByWorker[t.Assignee]++
		}
	}

	for w, total := range totalByWorker {
		fails := failsByWorker[w]
		if total > 0 && float64(fails)/float64(total) > 0.3 {
			gaps = append(gaps, CapabilityGap{
				Area:        string(w),
				Description: fmt.Sprintf("%s has %.0f%% failure rate (%d/%d)", w, float64(fails)/float64(total)*100, fails, total),
				Severity:    "moderate",
				Suggestion:  fmt.Sprintf("Review %s's tools and prompts, consider adding new capabilities", w),
			})
		}
	}

	// Check for workers with no tasks
	allWorkers := []WorkerType{WorkerSense, WorkerScout, WorkerDev, WorkerTest, WorkerOps}
	for _, w := range allWorkers {
		if totalByWorker[w] == 0 && len(e.tasks) > 5 {
			gaps = append(gaps, CapabilityGap{
				Area:        string(w),
				Description: fmt.Sprintf("%s has never been assigned a task", w),
				Severity:    "minor",
				Suggestion:  fmt.Sprintf("Consider integrating %s into the evolution loop", w),
			})
		}
	}

	// Check hotfix patterns
	unresolvedP0 := 0
	for _, hf := range e.hotfixes {
		if hf.Severity == SevP0 && hf.Status != TaskDone {
			unresolvedP0++
		}
	}
	if unresolvedP0 > 0 {
		gaps = append(gaps, CapabilityGap{
			Area:        "incident_response",
			Description: fmt.Sprintf("%d unresolved P0 hotfixes", unresolvedP0),
			Severity:    "critical",
			Suggestion:  "Prioritize P0 resolution, consider adding automated recovery tools",
		})
	}

	return gaps
}

// ── Experience Distillation ──

func (e *Engine) DistillLesson(sprintID, lessonType, content, impact string) *Lesson {
	e.mu.Lock()
	defer e.mu.Unlock()

	lesson := Lesson{
		ID:        e.genID("lesson"),
		SprintID:  sprintID,
		Type:      lessonType,
		Content:   content,
		Impact:    impact,
		CreatedAt: time.Now(),
	}

	e.lessons = append(e.lessons, lesson)
	if len(e.lessons) > 500 {
		e.lessons = e.lessons[1:]
	}
	e.stats.LessonsDistilled++
	e.stats.LastActivity = time.Now()
	return &lesson
}

func (e *Engine) ListLessons(limit int) []Lesson {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if limit <= 0 || limit > len(e.lessons) {
		limit = len(e.lessons)
	}
	if limit == 0 {
		return nil
	}
	start := len(e.lessons) - limit
	result := make([]Lesson, limit)
	copy(result, e.lessons[start:])
	return result
}

// ── Stats ──

func (e *Engine) Stats() *EngineStats {
	e.mu.RLock()
	defer e.mu.RUnlock()
	s := e.stats
	s.Uptime = time.Since(e.startAt).Round(time.Second).String()
	return &s
}

func (e *Engine) Config() *EngineConfig {
	return e.config
}

// WorkerStats returns per-worker task statistics
func (e *Engine) WorkerStats() map[string]map[string]int {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make(map[string]map[string]int)
	for _, w := range []WorkerType{WorkerSense, WorkerScout, WorkerDev, WorkerTest, WorkerOps} {
		result[string(w)] = map[string]int{"total": 0, "completed": 0, "failed": 0, "active": 0}
	}

	for _, t := range e.tasks {
		ws := string(t.Assignee)
		if _, ok := result[ws]; !ok {
			result[ws] = map[string]int{}
		}
		result[ws]["total"]++
		switch t.Status {
		case TaskDone:
			result[ws]["completed"]++
		case TaskFailed:
			result[ws]["failed"]++
		case TaskInProgress, TaskAssigned:
			result[ws]["active"]++
		}
	}
	return result
}
