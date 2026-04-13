package broodnet

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ════════════════════════════════════════════════════════════
// BroodOS Network — SwarmOrchestrator (端到端多节点协作)
//
// Ties together Formation, Hydralisk, TaskMarket, BroodMind,
// and Hive Discovery into coherent multi-node workflows:
//
//   - Plan:    decompose a high-level goal into sub-tasks
//   - Route:   assign sub-tasks to best nodes (local, market, formation)
//   - Execute: dispatch, monitor, retry on failure
//   - Settle:  collect results, pay executors, record metrics
//
// Workflow lifecycle: planned → routing → executing → aggregating → done/failed
//
// Routing strategies:
//   - local:     run on this node
//   - formation: dispatch to a formation member via Formation engine
//   - market:    post to TaskMarket for competitive bidding
//   - hive:      use Hive Discovery to find best node
// ════════════════════════════════════════════════════════════

// ── Types ──

// RouteStrategy determines how a sub-task gets assigned
type RouteStrategy string

const (
	RouteLocal     RouteStrategy = "local"
	RouteFormation RouteStrategy = "formation"
	RouteMarket    RouteStrategy = "market"
	RouteHive      RouteStrategy = "hive"
	RouteAuto      RouteStrategy = "auto" // orchestrator picks best
)

// WorkflowStatus tracks the overall workflow state
type WorkflowStatus string

const (
	WFPlanned     WorkflowStatus = "planned"
	WFRouting     WorkflowStatus = "routing"
	WFExecuting   WorkflowStatus = "executing"
	WFAggregating WorkflowStatus = "aggregating"
	WFDone        WorkflowStatus = "done"
	WFFailed      WorkflowStatus = "failed"
	WFCancelled   WorkflowStatus = "cancelled"
)

// SubTaskStatus tracks individual sub-task state
type SubTaskStatus string

const (
	STPending   SubTaskStatus = "pending"
	STRouted    SubTaskStatus = "routed"
	STRunning   SubTaskStatus = "running"
	STDone      SubTaskStatus = "done"
	STFailed    SubTaskStatus = "failed"
	STRetrying  SubTaskStatus = "retrying"
	STSkipped   SubTaskStatus = "skipped"
)

// SubTask is one unit of work in a workflow
type SubTask struct {
	ID           string          `json:"id"`
	WorkflowID   string          `json:"workflow_id"`
	Name         string          `json:"name"`
	Description  string          `json:"description,omitempty"`
	Category     TaskCategory    `json:"category"`
	Route        RouteStrategy   `json:"route"`
	AssignedNode string          `json:"assigned_node,omitempty"`
	MarketTaskID string          `json:"market_task_id,omitempty"` // if routed via market
	Payload      json.RawMessage `json:"payload,omitempty"`
	DependsOn    []string        `json:"depends_on,omitempty"` // IDs of prerequisite sub-tasks
	Budget       int64           `json:"budget"`
	MaxRetries   int             `json:"max_retries"`
	Retries      int             `json:"retries"`
	Status       SubTaskStatus   `json:"status"`
	Result       string          `json:"result,omitempty"`
	Error        string          `json:"error,omitempty"`
	StartedAt    *time.Time      `json:"started_at,omitempty"`
	CompletedAt  *time.Time      `json:"completed_at,omitempty"`
	DurationMs   int64           `json:"duration_ms,omitempty"`
}

// Workflow is a multi-step, multi-node orchestrated plan
type Workflow struct {
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	Description  string         `json:"description,omitempty"`
	InitiatorID  string         `json:"initiator_id"` // node that created it
	FormationID  string         `json:"formation_id,omitempty"`
	SubTasks     []*SubTask     `json:"sub_tasks"`
	Status       WorkflowStatus `json:"status"`
	TotalBudget  int64          `json:"total_budget"`
	SpentBudget  int64          `json:"spent_budget"`
	SpentStars   float64        `json:"spent_stars"`
	CreatedAt    time.Time      `json:"created_at"`
	StartedAt    *time.Time     `json:"started_at,omitempty"`
	CompletedAt  *time.Time     `json:"completed_at,omitempty"`
	DurationMs   int64          `json:"duration_ms,omitempty"`
}

// ── Orchestrator ──

// Orchestrator coordinates multi-node workflows
type Orchestrator struct {
	mu        sync.RWMutex
	nodeID    string
	workflows []*Workflow
	byID      map[string]*Workflow
	stats     OrchestratorStats
}

// OrchestratorStats tracks orchestrator metrics
type OrchestratorStats struct {
	TotalWorkflows   int                      `json:"total_workflows"`
	ActiveWorkflows  int                      `json:"active_workflows"`
	CompletedWF      int                      `json:"completed_workflows"`
	FailedWF         int                      `json:"failed_workflows"`
	TotalSubTasks    int                      `json:"total_sub_tasks"`
	CompletedST      int                      `json:"completed_sub_tasks"`
	FailedST         int                      `json:"failed_sub_tasks"`
	TotalSpent       int64                    `json:"total_spent"`
	SpentStars       float64                  `json:"spent_stars"`
	ByRoute          map[RouteStrategy]int    `json:"by_route"`
	ByStatus         map[WorkflowStatus]int   `json:"by_status"`
	AvgDurationMs    int64                    `json:"avg_duration_ms"`
	totalDurationMs  int64
}

var (
	globalOrch *Orchestrator
	orchOnce   sync.Once
)

// InitOrchestrator creates the global orchestrator
func InitOrchestrator(nodeID string) *Orchestrator {
	orchOnce.Do(func() {
		globalOrch = &Orchestrator{
			nodeID:    nodeID,
			workflows: make([]*Workflow, 0),
			byID:      make(map[string]*Workflow),
			stats: OrchestratorStats{
				ByRoute:  make(map[RouteStrategy]int),
				ByStatus: make(map[WorkflowStatus]int),
			},
		}
		log.Printf("[broodnet/orch] orchestrator ready (node=%s)", nodeID)
	})
	return globalOrch
}

// GetOrchestrator returns the global instance
func GetOrchestrator() *Orchestrator {
	return globalOrch
}

// ── Workflow CRUD ──

// PlanWorkflow creates a new workflow with sub-tasks
func (o *Orchestrator) PlanWorkflow(name, description, formationID string, subtasks []SubTaskInput) (*Workflow, error) {
	if name == "" {
		return nil, fmt.Errorf("workflow name required")
	}
	if len(subtasks) == 0 {
		return nil, fmt.Errorf("at least one sub-task required")
	}

	o.mu.Lock()
	defer o.mu.Unlock()

	wf := &Workflow{
		ID:          "wf:" + uuid.New().String()[:8],
		Name:        name,
		Description: description,
		InitiatorID: o.nodeID,
		FormationID: formationID,
		Status:      WFPlanned,
		CreatedAt:   time.Now(),
	}

	totalBudget := int64(0)
	for _, st := range subtasks {
		sub := &SubTask{
			ID:         "st:" + uuid.New().String()[:8],
			WorkflowID: wf.ID,
			Name:       st.Name,
			Description: st.Description,
			Category:   st.Category,
			Route:      st.Route,
			Payload:    st.Payload,
			DependsOn:  st.DependsOn,
			Budget:     st.Budget,
			MaxRetries: st.MaxRetries,
			Status:     STPending,
		}
		if sub.Category == "" {
			sub.Category = CatCompute
		}
		if sub.Route == "" {
			sub.Route = RouteAuto
		}
		if sub.MaxRetries <= 0 {
			sub.MaxRetries = 2
		}
		wf.SubTasks = append(wf.SubTasks, sub)
		totalBudget += sub.Budget
	}
	wf.TotalBudget = totalBudget

	o.workflows = append(o.workflows, wf)
	o.byID[wf.ID] = wf
	o.stats.TotalWorkflows++
	o.stats.TotalSubTasks += len(wf.SubTasks)
	o.stats.ActiveWorkflows++
	o.stats.ByStatus[WFPlanned]++

	log.Printf("[broodnet/orch] workflow %s planned: %q (%d sub-tasks, budget=%.2f⚡)",
		wf.ID, name, len(wf.SubTasks), float64(totalBudget)/10000.0)

	return wf, nil
}

// SubTaskInput is the user-facing input for defining sub-tasks
type SubTaskInput struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Category    TaskCategory    `json:"category,omitempty"`
	Route       RouteStrategy   `json:"route,omitempty"`
	Payload     json.RawMessage `json:"payload,omitempty"`
	DependsOn   []string        `json:"depends_on,omitempty"`
	Budget      int64           `json:"budget,omitempty"`
	MaxRetries  int             `json:"max_retries,omitempty"`
}

// StartWorkflow begins execution of a planned workflow
func (o *Orchestrator) StartWorkflow(wfID string) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	wf, ok := o.byID[wfID]
	if !ok {
		return fmt.Errorf("workflow %s not found", wfID)
	}
	if wf.Status != WFPlanned {
		return fmt.Errorf("workflow %s not in planned state (current=%s)", wfID, wf.Status)
	}

	now := time.Now()
	wf.Status = WFRouting
	wf.StartedAt = &now

	// Auto-route sub-tasks that have no dependencies
	routed := 0
	for _, st := range wf.SubTasks {
		if len(st.DependsOn) == 0 && st.Status == STPending {
			st.Status = STRouted
			o.stats.ByRoute[st.Route]++
			routed++
		}
	}

	wf.Status = WFExecuting
	o.stats.ByStatus[WFExecuting]++

	log.Printf("[broodnet/orch] workflow %s started: %d sub-tasks routed", wfID, routed)
	return nil
}

// AdvanceSubTask moves a sub-task through its lifecycle
func (o *Orchestrator) AdvanceSubTask(wfID, stID string, status SubTaskStatus, result, errMsg string) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	wf, ok := o.byID[wfID]
	if !ok {
		return fmt.Errorf("workflow %s not found", wfID)
	}

	var target *SubTask
	for _, st := range wf.SubTasks {
		if st.ID == stID {
			target = st
			break
		}
	}
	if target == nil {
		return fmt.Errorf("sub-task %s not found in workflow %s", stID, wfID)
	}

	now := time.Now()
	target.Status = status
	switch status {
	case STRunning:
		target.StartedAt = &now
	case STDone:
		target.Result = result
		target.CompletedAt = &now
		if target.StartedAt != nil {
			target.DurationMs = now.Sub(*target.StartedAt).Milliseconds()
		}
		o.stats.CompletedST++
		wf.SpentBudget += target.Budget
		wf.SpentStars = float64(wf.SpentBudget) / 10000.0

		// Unblock dependent sub-tasks
		o.unblockDependents(wf, stID)

	case STFailed:
		target.Error = errMsg
		target.CompletedAt = &now
		if target.StartedAt != nil {
			target.DurationMs = now.Sub(*target.StartedAt).Milliseconds()
		}

		// Retry if possible
		if target.Retries < target.MaxRetries {
			target.Retries++
			target.Status = STRetrying
			log.Printf("[broodnet/orch] sub-task %s retrying (%d/%d)", stID, target.Retries, target.MaxRetries)
		} else {
			o.stats.FailedST++
		}
	}

	// Check if workflow is complete
	o.checkWorkflowCompletion(wf)

	return nil
}

// unblockDependents routes sub-tasks whose dependencies are all done
func (o *Orchestrator) unblockDependents(wf *Workflow, completedID string) {
	for _, st := range wf.SubTasks {
		if st.Status != STPending {
			continue
		}
		allDone := true
		for _, dep := range st.DependsOn {
			for _, other := range wf.SubTasks {
				if other.ID == dep && other.Status != STDone {
					allDone = false
					break
				}
			}
			if !allDone {
				break
			}
		}
		if allDone && len(st.DependsOn) > 0 {
			st.Status = STRouted
			o.stats.ByRoute[st.Route]++
			log.Printf("[broodnet/orch] sub-task %s unblocked", st.ID)
		}
	}
}

// checkWorkflowCompletion checks if a workflow has finished
func (o *Orchestrator) checkWorkflowCompletion(wf *Workflow) {
	allDone := true
	anyFailed := false
	for _, st := range wf.SubTasks {
		switch st.Status {
		case STDone, STSkipped:
			continue
		case STFailed:
			if st.Retries >= st.MaxRetries {
				anyFailed = true
			} else {
				allDone = false // still retrying
			}
		default:
			allDone = false
		}
	}

	if !allDone {
		return
	}

	now := time.Now()
	wf.CompletedAt = &now
	if wf.StartedAt != nil {
		wf.DurationMs = now.Sub(*wf.StartedAt).Milliseconds()
	}
	o.stats.ActiveWorkflows--

	if anyFailed {
		wf.Status = WFFailed
		o.stats.FailedWF++
		o.stats.ByStatus[WFFailed]++
		log.Printf("[broodnet/orch] workflow %s FAILED (%dms)", wf.ID, wf.DurationMs)
	} else {
		wf.Status = WFDone
		o.stats.CompletedWF++
		o.stats.ByStatus[WFDone]++
		o.stats.TotalSpent += wf.SpentBudget
		o.stats.SpentStars = float64(o.stats.TotalSpent) / 10000.0
		o.stats.totalDurationMs += wf.DurationMs
		if o.stats.CompletedWF > 0 {
			o.stats.AvgDurationMs = o.stats.totalDurationMs / int64(o.stats.CompletedWF)
		}
		log.Printf("[broodnet/orch] workflow %s DONE (%dms, spent=%.2f⚡)",
			wf.ID, wf.DurationMs, wf.SpentStars)
	}
}

// CancelWorkflow cancels an active workflow
func (o *Orchestrator) CancelWorkflow(wfID string) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	wf, ok := o.byID[wfID]
	if !ok {
		return fmt.Errorf("workflow %s not found", wfID)
	}
	if wf.Status == WFDone || wf.Status == WFFailed || wf.Status == WFCancelled {
		return fmt.Errorf("workflow %s already finished", wfID)
	}

	wf.Status = WFCancelled
	for _, st := range wf.SubTasks {
		if st.Status == STPending || st.Status == STRouted {
			st.Status = STSkipped
		}
	}
	o.stats.ActiveWorkflows--
	o.stats.ByStatus[WFCancelled]++
	return nil
}

// ── Query ──

// GetWorkflow returns a workflow by ID
func (o *Orchestrator) GetWorkflow(id string) *Workflow {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.byID[id]
}

// ListWorkflows returns workflows filtered by status
func (o *Orchestrator) ListWorkflows(status WorkflowStatus, limit int) []*Workflow {
	o.mu.RLock()
	defer o.mu.RUnlock()

	if limit <= 0 {
		limit = 50
	}
	var result []*Workflow
	for i := len(o.workflows) - 1; i >= 0 && len(result) < limit; i-- {
		wf := o.workflows[i]
		if status != "" && wf.Status != status {
			continue
		}
		result = append(result, wf)
	}
	return result
}

// ReadySubTasks returns sub-tasks that are routed and ready for execution
func (o *Orchestrator) ReadySubTasks(wfID string) []*SubTask {
	o.mu.RLock()
	defer o.mu.RUnlock()

	wf, ok := o.byID[wfID]
	if !ok {
		return nil
	}
	var ready []*SubTask
	for _, st := range wf.SubTasks {
		if st.Status == STRouted || st.Status == STRetrying {
			ready = append(ready, st)
		}
	}
	return ready
}

// Stats returns orchestrator metrics
func (o *Orchestrator) Stats() *OrchestratorStats {
	o.mu.RLock()
	defer o.mu.RUnlock()
	s := o.stats
	return &s
}

// ── High-level convenience: RunWorkflow ──

// RunWorkflowSync plans, starts, and waits for a simple workflow (for testing/demo)
func (o *Orchestrator) RunWorkflowSync(ctx context.Context, name string, subtasks []SubTaskInput) (*Workflow, error) {
	wf, err := o.PlanWorkflow(name, "", "", subtasks)
	if err != nil {
		return nil, err
	}
	if err := o.StartWorkflow(wf.ID); err != nil {
		return nil, err
	}

	// Simulate: auto-advance all routed sub-tasks to done
	for _, st := range wf.SubTasks {
		if st.Status == STRouted {
			_ = o.AdvanceSubTask(wf.ID, st.ID, STRunning, "", "")
			_ = o.AdvanceSubTask(wf.ID, st.ID, STDone, "auto-completed", "")
		}
	}

	return o.GetWorkflow(wf.ID), nil
}
