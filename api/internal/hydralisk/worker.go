package hydralisk

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ════════════════════════════════════════════════════════════
// Phase 3C — Hydralisk v0 Heavy Worker
//
// Batch processing engine for long-running, resource-intensive tasks:
//   - Queued batch job execution with priority scheduling
//   - Data pipeline stages (extract → transform → load)
//   - GPU-aware resource reporting
//   - Concurrency control and backpressure
//   - Job lifecycle: queued → running → completed/failed/cancelled
//
// Hydralisk runs alongside the Claw agent runtime and handles
// tasks that are too heavy for interactive agent loops.
// ════════════════════════════════════════════════════════════

// JobStatus represents the lifecycle state of a batch job
type JobStatus string

const (
	JobQueued    JobStatus = "queued"
	JobRunning   JobStatus = "running"
	JobCompleted JobStatus = "completed"
	JobFailed    JobStatus = "failed"
	JobCancelled JobStatus = "cancelled"
)

// JobType categorizes the kind of batch work
type JobType string

const (
	JobTypeBatch    JobType = "batch"    // generic batch processing
	JobTypePipeline JobType = "pipeline" // multi-stage data pipeline
	JobTypeExport   JobType = "export"   // data export / report generation
	JobTypeImport   JobType = "import"   // bulk data ingestion
	JobTypeTrain    JobType = "train"    // model training / fine-tuning
	JobTypeIndex    JobType = "index"    // knowledge base indexing
)

// Job represents a batch processing job
type Job struct {
	ID          string    `json:"id"`
	Type        JobType   `json:"type"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Priority    int       `json:"priority"` // 0=low, 1=normal, 2=high, 3=critical
	Status      JobStatus `json:"status"`

	// Execution
	AgentID   string `json:"agent_id,omitempty"`
	NodeID    string `json:"node_id,omitempty"`
	UserID    string `json:"user_id,omitempty"`
	Payload   string `json:"payload"`   // JSON input data
	Result    string `json:"result,omitempty"`
	Error     string `json:"error,omitempty"`
	Progress  int    `json:"progress"`  // 0-100
	StageInfo string `json:"stage_info,omitempty"` // current pipeline stage

	// Resource requirements
	RequireGPU bool    `json:"require_gpu"`
	MinCPU     float64 `json:"min_cpu"`
	MinMemMB   int64   `json:"min_mem_mb"`

	// Timing
	CreatedAt   time.Time  `json:"created_at"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	TimeoutSec  int        `json:"timeout_sec"` // 0 = default (600s)

	// Pipeline stages (for pipeline-type jobs)
	Stages []*PipelineStage `json:"stages,omitempty"`

	// Internal
	cancelFunc context.CancelFunc `json:"-"`
}

// PipelineStage represents one stage of a data pipeline
type PipelineStage struct {
	Name      string    `json:"name"`
	Status    JobStatus `json:"status"`
	Input     string    `json:"input,omitempty"`
	Output    string    `json:"output,omitempty"`
	Error     string    `json:"error,omitempty"`
	StartedAt *time.Time `json:"started_at,omitempty"`
	DoneAt    *time.Time `json:"done_at,omitempty"`
	Duration  float64   `json:"duration_sec,omitempty"`
}

// GPUInfo reports GPU availability on this node
type GPUInfo struct {
	Available bool    `json:"available"`
	Count     int     `json:"count"`
	Model     string  `json:"model,omitempty"`
	MemoryMB  int64   `json:"memory_mb,omitempty"`
	UsedPct   float64 `json:"used_pct,omitempty"`
}

// WorkerConfig configures the Hydralisk worker
type WorkerConfig struct {
	MaxConcurrent int           // max simultaneous jobs
	DefaultTimeout time.Duration // default job timeout
	QueueSize     int           // max pending queue size
}

// DefaultWorkerConfig returns sensible defaults
func DefaultWorkerConfig() *WorkerConfig {
	return &WorkerConfig{
		MaxConcurrent:  3,
		DefaultTimeout: 10 * time.Minute,
		QueueSize:      100,
	}
}

// JobExecutor is the function signature for executing a job
// Implementations should check ctx for cancellation and update progress via the callback
type JobExecutor func(ctx context.Context, job *Job, progress func(pct int, stage string)) (result string, err error)

// Worker is the Hydralisk batch processing engine
type Worker struct {
	mu       sync.RWMutex
	config   *WorkerConfig
	jobs     map[string]*Job
	queue    []*Job // priority-sorted queue
	running  int
	executor JobExecutor
	gpu      GPUInfo
	nodeID   string

	stopCh chan struct{}
	wakeCh chan struct{} // signals new job available
}

// NewWorker creates a new Hydralisk worker
func NewWorker(cfg *WorkerConfig, nodeID string, executor JobExecutor) *Worker {
	if cfg == nil {
		cfg = DefaultWorkerConfig()
	}
	w := &Worker{
		config:   cfg,
		jobs:     make(map[string]*Job),
		executor: executor,
		nodeID:   nodeID,
		stopCh:   make(chan struct{}),
		wakeCh:   make(chan struct{}, 1),
	}
	go w.dispatchLoop()
	log.Printf("[hydralisk] worker started (maxConcurrent=%d, timeout=%s, queue=%d)",
		cfg.MaxConcurrent, cfg.DefaultTimeout, cfg.QueueSize)
	return w
}

// Stop shuts down the worker gracefully
func (w *Worker) Stop() {
	close(w.stopCh)
	// Cancel all running jobs
	w.mu.Lock()
	for _, j := range w.jobs {
		if j.Status == JobRunning && j.cancelFunc != nil {
			j.cancelFunc()
		}
	}
	w.mu.Unlock()
}

// SetGPU updates GPU info for this node
func (w *Worker) SetGPU(info GPUInfo) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.gpu = info
}

// ── Job Submission ──

// Submit queues a new job and returns its ID
func (w *Worker) Submit(job *Job) (*Job, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if len(w.queue) >= w.config.QueueSize {
		return nil, fmt.Errorf("job queue full (%d/%d)", len(w.queue), w.config.QueueSize)
	}

	if job.RequireGPU && !w.gpu.Available {
		return nil, fmt.Errorf("job requires GPU but none available on this node")
	}

	job.ID = uuid.New().String()
	job.Status = JobQueued
	job.CreatedAt = time.Now()
	job.NodeID = w.nodeID
	if job.TimeoutSec == 0 {
		job.TimeoutSec = int(w.config.DefaultTimeout.Seconds())
	}

	w.jobs[job.ID] = job
	w.insertQueue(job)

	log.Printf("[hydralisk] job %s queued (type=%s, priority=%d, name=%s)",
		job.ID[:8], job.Type, job.Priority, job.Name)

	// Wake dispatch loop
	select {
	case w.wakeCh <- struct{}{}:
	default:
	}

	return job, nil
}

// Cancel attempts to cancel a job
func (w *Worker) Cancel(jobID string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	job, ok := w.jobs[jobID]
	if !ok {
		return fmt.Errorf("job %s not found", jobID)
	}

	switch job.Status {
	case JobQueued:
		job.Status = JobCancelled
		w.removeFromQueue(jobID)
	case JobRunning:
		if job.cancelFunc != nil {
			job.cancelFunc()
		}
		job.Status = JobCancelled
		now := time.Now()
		job.CompletedAt = &now
	default:
		return fmt.Errorf("job %s is already %s", jobID, job.Status)
	}

	return nil
}

// ── Queries ──

// GetJob returns a job by ID
func (w *Worker) GetJob(jobID string) (*Job, bool) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	j, ok := w.jobs[jobID]
	return j, ok
}

// ListJobs returns jobs filtered by status
func (w *Worker) ListJobs(status JobStatus, jobType JobType, limit int) []*Job {
	w.mu.RLock()
	defer w.mu.RUnlock()

	if limit <= 0 {
		limit = 50
	}

	var result []*Job
	for _, j := range w.jobs {
		if status != "" && j.Status != status {
			continue
		}
		if jobType != "" && j.Type != jobType {
			continue
		}
		result = append(result, j)
		if len(result) >= limit {
			break
		}
	}
	return result
}

// Stats returns worker statistics
func (w *Worker) Stats() map[string]interface{} {
	w.mu.RLock()
	defer w.mu.RUnlock()

	byStatus := map[string]int{}
	byType := map[string]int{}
	for _, j := range w.jobs {
		byStatus[string(j.Status)]++
		byType[string(j.Type)]++
	}

	return map[string]interface{}{
		"total_jobs":     len(w.jobs),
		"queued":         len(w.queue),
		"running":        w.running,
		"max_concurrent": w.config.MaxConcurrent,
		"by_status":      byStatus,
		"by_type":        byType,
		"gpu":            w.gpu,
		"node_id":        w.nodeID,
	}
}

// ── Internal Dispatch ──

func (w *Worker) dispatchLoop() {
	for {
		select {
		case <-w.stopCh:
			return
		case <-w.wakeCh:
			w.tryDispatch()
		case <-time.After(5 * time.Second):
			w.tryDispatch()
			w.cleanOldJobs()
		}
	}
}

func (w *Worker) tryDispatch() {
	for {
		w.mu.Lock()
		if w.running >= w.config.MaxConcurrent || len(w.queue) == 0 {
			w.mu.Unlock()
			return
		}

		job := w.queue[0]
		w.queue = w.queue[1:]
		job.Status = JobRunning
		now := time.Now()
		job.StartedAt = &now
		w.running++
		w.mu.Unlock()

		go w.executeJob(job)
	}
}

func (w *Worker) executeJob(job *Job) {
	timeout := time.Duration(job.TimeoutSec) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	job.cancelFunc = cancel
	defer cancel()

	log.Printf("[hydralisk] job %s started (type=%s, name=%s, timeout=%s)",
		job.ID[:8], job.Type, job.Name, timeout)

	progressFn := func(pct int, stage string) {
		w.mu.Lock()
		job.Progress = pct
		if stage != "" {
			job.StageInfo = stage
		}
		w.mu.Unlock()
	}

	result, err := w.executor(ctx, job, progressFn)

	w.mu.Lock()
	defer w.mu.Unlock()

	now := time.Now()
	job.CompletedAt = &now
	w.running--

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			job.Status = JobFailed
			job.Error = "timeout exceeded"
		} else if ctx.Err() == context.Canceled {
			job.Status = JobCancelled
		} else {
			job.Status = JobFailed
			job.Error = err.Error()
		}
		log.Printf("[hydralisk] job %s failed: %v", job.ID[:8], err)
	} else {
		job.Status = JobCompleted
		job.Result = result
		job.Progress = 100
		log.Printf("[hydralisk] job %s completed (%.1fs)", job.ID[:8], now.Sub(*job.StartedAt).Seconds())
	}

	// Wake dispatch loop for next job
	select {
	case w.wakeCh <- struct{}{}:
	default:
	}
}

func (w *Worker) insertQueue(job *Job) {
	// Insert in priority order (higher priority first)
	idx := len(w.queue)
	for i, q := range w.queue {
		if job.Priority > q.Priority {
			idx = i
			break
		}
	}
	w.queue = append(w.queue, nil)
	copy(w.queue[idx+1:], w.queue[idx:])
	w.queue[idx] = job
}

func (w *Worker) removeFromQueue(jobID string) {
	for i, j := range w.queue {
		if j.ID == jobID {
			w.queue = append(w.queue[:i], w.queue[i+1:]...)
			return
		}
	}
}

func (w *Worker) cleanOldJobs() {
	w.mu.Lock()
	defer w.mu.Unlock()

	cutoff := time.Now().Add(-24 * time.Hour)
	for id, j := range w.jobs {
		if j.CompletedAt != nil && j.CompletedAt.Before(cutoff) {
			delete(w.jobs, id)
		}
	}
}
