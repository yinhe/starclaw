package broodmind

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

// ════════════════════════════════════════════════════════════
// BroodMind v1 — Trajectory Tracker
//
// Records agent execution traces: every tool call, LLM round,
// timing, and final outcome. These trajectories are the raw
// material for reflection and distillation.
// ════════════════════════════════════════════════════════════

// TraceStatus represents the outcome of a trajectory
type TraceStatus string

const (
	TraceRunning   TraceStatus = "running"
	TraceCompleted TraceStatus = "completed"
	TraceFailed    TraceStatus = "failed"
	TraceCancelled TraceStatus = "cancelled"
)

// ToolStep records a single tool invocation within a trajectory
type ToolStep struct {
	Index     int       `json:"index"`
	ToolName  string    `json:"tool_name"`
	Arguments string    `json:"arguments,omitempty"`
	Result    string    `json:"result,omitempty"`
	Error     string    `json:"error,omitempty"`
	StartedAt time.Time `json:"started_at"`
	Duration  int64     `json:"duration_ms"`
}

// Trajectory captures a complete agent execution trace
type Trajectory struct {
	ID             string      `json:"id"`
	AgentID        string      `json:"agent_id"`
	UserID         string      `json:"user_id,omitempty"`
	NodeID         string      `json:"node_id,omitempty"`
	ConversationID string      `json:"conversation_id,omitempty"`
	Model          string      `json:"model"`
	Task           string      `json:"task"`
	Status         TraceStatus `json:"status"`
	Steps          []ToolStep  `json:"steps"`
	LLMRounds      int         `json:"llm_rounds"`
	TotalTokens    int         `json:"total_tokens"`
	FinalOutput    string      `json:"final_output,omitempty"`
	ErrorMsg       string      `json:"error_msg,omitempty"`
	StartedAt      time.Time   `json:"started_at"`
	CompletedAt    *time.Time  `json:"completed_at,omitempty"`
	DurationMs     int64       `json:"duration_ms"`
	Tags           []string    `json:"tags,omitempty"`
}

// TrajectoryStore manages trajectory storage and retrieval
type TrajectoryStore struct {
	mu      sync.RWMutex
	traces  []*Trajectory
	byID    map[string]*Trajectory
	maxSize int
}

// NewTrajectoryStore creates a new trajectory store
func NewTrajectoryStore(maxSize int) *TrajectoryStore {
	if maxSize <= 0 {
		maxSize = 500
	}
	return &TrajectoryStore{
		traces:  make([]*Trajectory, 0, maxSize),
		byID:    make(map[string]*Trajectory),
		maxSize: maxSize,
	}
}

// Begin starts recording a new trajectory, returns its ID
func (ts *TrajectoryStore) Begin(agentID, userID, nodeID, model, task string) string {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	t := &Trajectory{
		ID:        "trace:" + uuid.New().String()[:8],
		AgentID:   agentID,
		UserID:    userID,
		NodeID:    nodeID,
		Model:     model,
		Task:      task,
		Status:    TraceRunning,
		Steps:     make([]ToolStep, 0),
		StartedAt: time.Now(),
	}

	ts.traces = append(ts.traces, t)
	ts.byID[t.ID] = t

	// Evict oldest if over capacity
	if len(ts.traces) > ts.maxSize {
		old := ts.traces[0]
		ts.traces = ts.traces[1:]
		delete(ts.byID, old.ID)
	}

	return t.ID
}

// RecordStep adds a tool step to an active trajectory
func (ts *TrajectoryStore) RecordStep(traceID string, toolName, args, result, errMsg string, duration time.Duration) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	t, ok := ts.byID[traceID]
	if !ok {
		return
	}

	step := ToolStep{
		Index:     len(t.Steps),
		ToolName:  toolName,
		Arguments: truncate(args, 500),
		Result:    truncate(result, 1000),
		Error:     errMsg,
		StartedAt: time.Now().Add(-duration),
		Duration:  duration.Milliseconds(),
	}
	t.Steps = append(t.Steps, step)
	t.LLMRounds++
}

// Complete marks a trajectory as finished
func (ts *TrajectoryStore) Complete(traceID, finalOutput string, totalTokens int, status TraceStatus, errMsg string) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	t, ok := ts.byID[traceID]
	if !ok {
		return
	}

	now := time.Now()
	t.Status = status
	t.FinalOutput = truncate(finalOutput, 2000)
	t.TotalTokens = totalTokens
	t.ErrorMsg = errMsg
	t.CompletedAt = &now
	t.DurationMs = now.Sub(t.StartedAt).Milliseconds()
}

// Get retrieves a trajectory by ID
func (ts *TrajectoryStore) Get(traceID string) *Trajectory {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	return ts.byID[traceID]
}

// Recent returns the N most recent trajectories
func (ts *TrajectoryStore) Recent(limit int) []*Trajectory {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	if limit <= 0 || limit > len(ts.traces) {
		limit = len(ts.traces)
	}
	// Return newest first
	result := make([]*Trajectory, limit)
	for i := 0; i < limit; i++ {
		result[i] = ts.traces[len(ts.traces)-1-i]
	}
	return result
}

// ByAgent returns trajectories for a specific agent
func (ts *TrajectoryStore) ByAgent(agentID string, limit int) []*Trajectory {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	var result []*Trajectory
	for i := len(ts.traces) - 1; i >= 0 && len(result) < limit; i-- {
		if ts.traces[i].AgentID == agentID {
			result = append(result, ts.traces[i])
		}
	}
	return result
}

// Completed returns only finished trajectories (for reflection/distillation)
func (ts *TrajectoryStore) Completed(limit int) []*Trajectory {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	var result []*Trajectory
	for i := len(ts.traces) - 1; i >= 0 && len(result) < limit; i-- {
		if ts.traces[i].Status == TraceCompleted || ts.traces[i].Status == TraceFailed {
			result = append(result, ts.traces[i])
		}
	}
	return result
}

// Stats returns trajectory statistics
func (ts *TrajectoryStore) Stats() map[string]interface{} {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	byStatus := map[TraceStatus]int{}
	byAgent := map[string]int{}
	totalSteps := 0
	totalTokens := 0
	var totalDuration int64

	for _, t := range ts.traces {
		byStatus[t.Status]++
		byAgent[t.AgentID]++
		totalSteps += len(t.Steps)
		totalTokens += t.TotalTokens
		totalDuration += t.DurationMs
	}

	avgDuration := int64(0)
	if len(ts.traces) > 0 {
		avgDuration = totalDuration / int64(len(ts.traces))
	}

	return map[string]interface{}{
		"total":        len(ts.traces),
		"by_status":    byStatus,
		"by_agent":     byAgent,
		"total_steps":  totalSteps,
		"total_tokens": totalTokens,
		"avg_duration": avgDuration,
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
