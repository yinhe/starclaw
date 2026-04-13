package security

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// ════════════════════════════════════════════════════════════
// Agent Security v1 — Sandbox Executor
//
// Wraps tool execution with resource constraints:
//   - Execution timeout (per tool call)
//   - Max output size (truncate oversized results)
//   - Concurrent execution limit (prevent fork bombs)
//   - Execution audit logging
//
// The sandbox does NOT do OS-level isolation (that's Chitin's job).
// It provides application-level guardrails for the agent runtime.
// ════════════════════════════════════════════════════════════

// SandboxConfig holds sandbox resource limits
type SandboxConfig struct {
	DefaultTimeout   time.Duration   // default per-tool timeout
	LongRunTimeout   time.Duration   // timeout for long-running tools (delegate, etc.)
	LongRunningTools map[string]bool // tool names that use LongRunTimeout
	MaxOutputSize    int             // max bytes in tool result
	MaxConcurrent    int             // max concurrent tool executions per agent
	AuditEnabled     bool            // log every tool execution
}

// DefaultSandboxConfig returns sensible defaults
func DefaultSandboxConfig() *SandboxConfig {
	return &SandboxConfig{
		DefaultTimeout: 30 * time.Second,
		LongRunTimeout: 5 * time.Minute,
		LongRunningTools: map[string]bool{
			"system":           true, // delegate to another agent
			"video_generation": true, // video gen can take minutes
			"music_generation": true, // music gen can take minutes
			"image_generation": true, // image gen
			"dubbing":          true, // dubbing pipeline
			"mv_production":    true, // MV merge pipeline
			"comic_production": true, // comic pipeline
			"code_execute":     true, // code execution
		},
		MaxOutputSize: 64 * 1024, // 64 KB
		MaxConcurrent: 5,
		AuditEnabled:  true,
	}
}

// SandboxViolation represents a security boundary violation
type SandboxViolation struct {
	AgentID   string    `json:"agent_id"`
	ToolName  string    `json:"tool_name"`
	Reason    string    `json:"reason"`
	Timestamp time.Time `json:"timestamp"`
}

// Sandbox enforces execution constraints on tool calls
type Sandbox struct {
	config     *SandboxConfig
	mu         sync.RWMutex
	violations []SandboxViolation
	execCount  map[string]*atomic.Int32 // agent_id → concurrent count
	stats      SandboxStats
}

// SandboxStats tracks aggregate sandbox metrics
type SandboxStats struct {
	TotalExecutions   int64 `json:"total_executions"`
	TimeoutKills      int64 `json:"timeout_kills"`
	OutputTruncated   int64 `json:"output_truncated"`
	ConcurrencyDenied int64 `json:"concurrency_denied"`
	TrustDenied       int64 `json:"trust_denied"`
	TotalViolations   int64 `json:"total_violations"`
}

// NewSandbox creates a new sandbox with the given config
func NewSandbox(cfg *SandboxConfig) *Sandbox {
	if cfg == nil {
		cfg = DefaultSandboxConfig()
	}
	return &Sandbox{
		config:     cfg,
		violations: make([]SandboxViolation, 0),
		execCount:  make(map[string]*atomic.Int32),
	}
}

// ToolExecutor is a function that actually runs the tool
type ToolExecutor func(ctx context.Context) (string, error)

// Execute runs a tool within sandbox constraints
func (s *Sandbox) Execute(ctx context.Context, agentID, toolName string, exec ToolExecutor) (string, error) {
	atomic.AddInt64(&s.stats.TotalExecutions, 1)

	// 1. Check concurrency limit
	counter := s.getCounter(agentID)
	current := counter.Add(1)
	defer counter.Add(-1)

	if int(current) > s.config.MaxConcurrent {
		atomic.AddInt64(&s.stats.ConcurrencyDenied, 1)
		s.recordViolation(agentID, toolName, fmt.Sprintf("concurrency limit exceeded (%d/%d)", current, s.config.MaxConcurrent))
		return "", fmt.Errorf("sandbox: agent %s exceeded concurrent execution limit (%d)", agentID, s.config.MaxConcurrent)
	}

	// 2. Apply timeout (long-running tools get extended timeout)
	timeout := s.config.DefaultTimeout
	if s.config.LongRunningTools[toolName] && s.config.LongRunTimeout > 0 {
		timeout = s.config.LongRunTimeout
	}
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// 3. Audit log
	if s.config.AuditEnabled {
		log.Printf("[sandbox] agent=%s tool=%s START (concurrent=%d)", agentID, toolName, current)
	}

	start := time.Now()

	// 4. Execute with timeout
	type execResult struct {
		output string
		err    error
	}
	ch := make(chan execResult, 1)
	go func() {
		out, err := exec(execCtx)
		ch <- execResult{out, err}
	}()

	var result string
	var execErr error

	select {
	case r := <-ch:
		result = r.output
		execErr = r.err
	case <-execCtx.Done():
		atomic.AddInt64(&s.stats.TimeoutKills, 1)
		s.recordViolation(agentID, toolName, fmt.Sprintf("execution timeout after %s", timeout))
		return "", fmt.Errorf("sandbox: tool %s timed out after %s", toolName, timeout)
	}

	duration := time.Since(start)

	// 5. Truncate oversized output
	if len(result) > s.config.MaxOutputSize {
		atomic.AddInt64(&s.stats.OutputTruncated, 1)
		result = result[:s.config.MaxOutputSize] + fmt.Sprintf("\n... [truncated: %d bytes > %d max]", len(result), s.config.MaxOutputSize)
	}

	// 6. Audit log completion
	if s.config.AuditEnabled {
		status := "OK"
		if execErr != nil {
			status = "ERR"
		}
		log.Printf("[sandbox] agent=%s tool=%s %s (%s, %d bytes)", agentID, toolName, status, duration.Round(time.Millisecond), len(result))
	}

	return result, execErr
}

// RecordTrustDenial records a trust-level access denial
func (s *Sandbox) RecordTrustDenial(agentID, toolName, reason string) {
	atomic.AddInt64(&s.stats.TrustDenied, 1)
	s.recordViolation(agentID, toolName, "trust denied: "+reason)
}

// GetViolations returns recent violations
func (s *Sandbox) GetViolations(limit int) []SandboxViolation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 || limit > len(s.violations) {
		limit = len(s.violations)
	}
	// Return newest first
	result := make([]SandboxViolation, limit)
	for i := 0; i < limit; i++ {
		result[i] = s.violations[len(s.violations)-1-i]
	}
	return result
}

// GetStats returns sandbox statistics
func (s *Sandbox) GetStats() SandboxStats {
	return SandboxStats{
		TotalExecutions:   atomic.LoadInt64(&s.stats.TotalExecutions),
		TimeoutKills:      atomic.LoadInt64(&s.stats.TimeoutKills),
		OutputTruncated:   atomic.LoadInt64(&s.stats.OutputTruncated),
		ConcurrencyDenied: atomic.LoadInt64(&s.stats.ConcurrencyDenied),
		TrustDenied:       atomic.LoadInt64(&s.stats.TrustDenied),
		TotalViolations:   atomic.LoadInt64(&s.stats.TotalViolations),
	}
}

// ── Internal ──

func (s *Sandbox) getCounter(agentID string) *atomic.Int32 {
	s.mu.Lock()
	defer s.mu.Unlock()

	if c, ok := s.execCount[agentID]; ok {
		return c
	}
	c := &atomic.Int32{}
	s.execCount[agentID] = c
	return c
}

func (s *Sandbox) recordViolation(agentID, toolName, reason string) {
	atomic.AddInt64(&s.stats.TotalViolations, 1)

	s.mu.Lock()
	defer s.mu.Unlock()

	v := SandboxViolation{
		AgentID:   agentID,
		ToolName:  toolName,
		Reason:    reason,
		Timestamp: time.Now(),
	}
	s.violations = append(s.violations, v)

	// Cap violations at 1000
	if len(s.violations) > 1000 {
		s.violations = s.violations[len(s.violations)-500:]
	}

	log.Printf("[sandbox] VIOLATION agent=%s tool=%s reason=%s", agentID, toolName, reason)
}
