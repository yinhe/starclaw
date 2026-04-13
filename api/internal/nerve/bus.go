package nerve

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// ════════════════════════════════════════════════════════════
// Nerve Bus — 虫群神经系统
//
// 连接所有引擎的中央事件/调度总线。引擎通过 Nerve Bus 通讯：
//   - Publish: 发布事件（异步广播给所有订阅者）
//   - Dispatch: 派发任务给指定工蜂（同步回调）
//   - Subscribe: 订阅某类事件
//   - RegisterWorker: 注册工蜂处理器
//
// 设计原则:
//   - 进程内通讯，无网络开销
//   - 异步事件 + 同步调度
//   - 所有交互有审计日志
//   - 为后续 Pheromone NATS 集成预留接口
// ════════════════════════════════════════════════════════════

// ── Event ──

type Event struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`      // e.g. "sense.alert.fired", "abathur.task.dispatched"
	Source    string                 `json:"source"`    // engine name
	Target    string                 `json:"target"`    // optional: specific target engine
	Payload   map[string]interface{} `json:"payload"`
	Timestamp time.Time              `json:"timestamp"`
}

// ── Task Dispatch ──

type TaskRequest struct {
	ID          string                 `json:"id"`
	WorkerType  string                 `json:"worker_type"` // sense_claw, scout_claw, dev_team, test_claw, ops_claw
	Action      string                 `json:"action"`      // e.g. "run_health_check", "run_test_suite", "deploy"
	Params      map[string]interface{} `json:"params"`
	Priority    string                 `json:"priority"` // P0, P1, P2, P3
	RequestedBy string                 `json:"requested_by"`
	Timeout     time.Duration          `json:"timeout"`
	CreatedAt   time.Time              `json:"created_at"`
}

type TaskResult struct {
	RequestID string                 `json:"request_id"`
	WorkerType string                `json:"worker_type"`
	Status    string                 `json:"status"` // completed, failed, timeout
	Output    map[string]interface{} `json:"output"`
	Error     string                 `json:"error,omitempty"`
	Duration  time.Duration          `json:"duration"`
}

// ── Handler Interfaces ──

type EventHandler func(event Event)
type WorkerHandler func(req TaskRequest) TaskResult

// ── Bus ──

type Bus struct {
	mu          sync.RWMutex
	subscribers map[string][]EventHandler   // eventType → handlers
	workers     map[string]WorkerHandler    // workerType → handler
	eventLog    []Event                     // ring buffer
	taskLog     []taskLogEntry              // ring buffer
	stats       BusStats
	maxLogSize  int
	nextID      int
}

type taskLogEntry struct {
	Request  TaskRequest `json:"request"`
	Result   TaskResult  `json:"result"`
	Duration float64     `json:"duration_ms"`
}

type BusStats struct {
	EventsPublished  int       `json:"events_published"`
	EventsDelivered  int       `json:"events_delivered"`
	TasksDispatched  int       `json:"tasks_dispatched"`
	TasksCompleted   int       `json:"tasks_completed"`
	TasksFailed      int       `json:"tasks_failed"`
	TasksTimedOut    int       `json:"tasks_timed_out"`
	WorkersRegistered int      `json:"workers_registered"`
	SubscriberCount  int       `json:"subscriber_count"`
	LastEvent        time.Time `json:"last_event,omitempty"`
	LastDispatch     time.Time `json:"last_dispatch,omitempty"`
}

var (
	globalBus *Bus
	busOnce   sync.Once
)

func InitBus() *Bus {
	busOnce.Do(func() {
		globalBus = &Bus{
			subscribers: make(map[string][]EventHandler),
			workers:     make(map[string]WorkerHandler),
			eventLog:    make([]Event, 0, 500),
			taskLog:     make([]taskLogEntry, 0, 200),
			maxLogSize:  500,
		}
		log.Printf("[nerve] bus initialized — neural wiring active")
	})
	return globalBus
}

func GetBus() *Bus {
	return globalBus
}

func (b *Bus) genID(prefix string) string {
	b.nextID++
	return fmt.Sprintf("%s-%d-%d", prefix, time.Now().UnixMilli(), b.nextID)
}

// ── Subscribe ──

// Subscribe registers a handler for a specific event type.
// Use "*" to subscribe to all events.
func (b *Bus) Subscribe(eventType string, handler EventHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subscribers[eventType] = append(b.subscribers[eventType], handler)
	b.stats.SubscriberCount++
	log.Printf("[nerve] subscriber registered for %q (total=%d)", eventType, b.stats.SubscriberCount)
}

// ── Publish ──

// Publish broadcasts an event to all matching subscribers (async, non-blocking).
func (b *Bus) Publish(eventType, source string, payload map[string]interface{}) string {
	b.mu.Lock()
	event := Event{
		ID:        b.genID("evt"),
		Type:      eventType,
		Source:    source,
		Payload:   payload,
		Timestamp: time.Now(),
	}
	b.eventLog = append(b.eventLog, event)
	if len(b.eventLog) > b.maxLogSize {
		b.eventLog = b.eventLog[1:]
	}
	b.stats.EventsPublished++
	b.stats.LastEvent = event.Timestamp

	// Collect handlers to invoke outside the lock
	var handlers []EventHandler
	if hs, ok := b.subscribers[eventType]; ok {
		handlers = append(handlers, hs...)
	}
	if hs, ok := b.subscribers["*"]; ok {
		handlers = append(handlers, hs...)
	}
	b.mu.Unlock()

	// Deliver asynchronously
	for _, h := range handlers {
		go func(handler EventHandler) {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[nerve] event handler panic for %q: %v", eventType, r)
				}
			}()
			handler(event)
			b.mu.Lock()
			b.stats.EventsDelivered++
			b.mu.Unlock()
		}(h)
	}

	return event.ID
}

// ── Worker Registration ──

// RegisterWorker registers a handler for a worker type.
// When Abathur dispatches a task to this worker type, this handler is called.
func (b *Bus) RegisterWorker(workerType string, handler WorkerHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.workers[workerType] = handler
	b.stats.WorkersRegistered++
	log.Printf("[nerve] worker registered: %s (total=%d)", workerType, b.stats.WorkersRegistered)
}

// ── Dispatch ──

// Dispatch sends a task to a specific worker type and waits for the result.
// Returns an error if no worker is registered or the task times out.
func (b *Bus) Dispatch(req TaskRequest) (*TaskResult, error) {
	if req.ID == "" {
		b.mu.Lock()
		req.ID = b.genID("task")
		b.mu.Unlock()
	}
	if req.CreatedAt.IsZero() {
		req.CreatedAt = time.Now()
	}
	if req.Timeout == 0 {
		req.Timeout = 30 * time.Second
	}

	b.mu.RLock()
	handler, ok := b.workers[req.WorkerType]
	b.mu.RUnlock()

	if !ok {
		b.mu.Lock()
		b.stats.TasksFailed++
		b.mu.Unlock()
		return nil, fmt.Errorf("no worker registered for type %q", req.WorkerType)
	}

	b.mu.Lock()
	b.stats.TasksDispatched++
	b.stats.LastDispatch = time.Now()
	b.mu.Unlock()

	// Publish dispatch event
	b.Publish("nerve.task.dispatched", "nerve", map[string]interface{}{
		"task_id":     req.ID,
		"worker_type": req.WorkerType,
		"action":      req.Action,
		"priority":    req.Priority,
	})

	// Execute with timeout
	start := time.Now()
	resultCh := make(chan TaskResult, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				resultCh <- TaskResult{
					RequestID:  req.ID,
					WorkerType: req.WorkerType,
					Status:     "failed",
					Error:      fmt.Sprintf("worker panic: %v", r),
					Duration:   time.Since(start),
				}
			}
		}()
		result := handler(req)
		result.RequestID = req.ID
		result.WorkerType = req.WorkerType
		result.Duration = time.Since(start)
		resultCh <- result
	}()

	var result TaskResult
	select {
	case result = <-resultCh:
		// got result
	case <-time.After(req.Timeout):
		result = TaskResult{
			RequestID:  req.ID,
			WorkerType: req.WorkerType,
			Status:     "timeout",
			Error:      fmt.Sprintf("task timed out after %v", req.Timeout),
			Duration:   req.Timeout,
		}
		b.mu.Lock()
		b.stats.TasksTimedOut++
		b.mu.Unlock()
	}

	// Record result
	b.mu.Lock()
	entry := taskLogEntry{
		Request:  req,
		Result:   result,
		Duration: float64(result.Duration.Milliseconds()),
	}
	b.taskLog = append(b.taskLog, entry)
	if len(b.taskLog) > 200 {
		b.taskLog = b.taskLog[1:]
	}
	if result.Status == "completed" {
		b.stats.TasksCompleted++
	} else {
		b.stats.TasksFailed++
	}
	b.mu.Unlock()

	// Publish result event
	b.Publish("nerve.task.completed", "nerve", map[string]interface{}{
		"task_id":     req.ID,
		"worker_type": req.WorkerType,
		"status":      result.Status,
		"duration_ms": result.Duration.Milliseconds(),
	})

	return &result, nil
}

// ── Query ──

func (b *Bus) Stats() *BusStats {
	b.mu.RLock()
	defer b.mu.RUnlock()
	s := b.stats
	return &s
}

func (b *Bus) RecentEvents(limit int) []Event {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if limit <= 0 || limit > len(b.eventLog) {
		limit = len(b.eventLog)
	}
	if limit == 0 {
		return nil
	}
	start := len(b.eventLog) - limit
	result := make([]Event, limit)
	copy(result, b.eventLog[start:])
	return result
}

func (b *Bus) RecentTasks(limit int) []taskLogEntry {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if limit <= 0 || limit > len(b.taskLog) {
		limit = len(b.taskLog)
	}
	if limit == 0 {
		return nil
	}
	start := len(b.taskLog) - limit
	result := make([]taskLogEntry, limit)
	copy(result, b.taskLog[start:])
	return result
}

func (b *Bus) RegisteredWorkers() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	names := make([]string, 0, len(b.workers))
	for k := range b.workers {
		names = append(names, k)
	}
	return names
}
