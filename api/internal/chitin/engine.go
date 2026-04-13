package chitin

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// ════════════════════════════════════════════════════════════
// Chitin v1 — Agent 运行时引擎
//
// 职责:
//   1. 实例管理: 启动/停止/重启 Agent 实例
//   2. 双模式运行: Process (进程) + Container (Docker/Podman)
//   3. 资源限制: CPU/内存/磁盘配额
//   4. 健康检查: 定期探测实例存活，自动重启
//   5. 日志收集: stdout/stderr 聚合
//   6. 生命周期: 创建 → 运行 → 停止 → 销毁
// ════════════════════════════════════════════════════════════

// ── Types ──

type RuntimeMode string

const (
	ModeProcess   RuntimeMode = "process"
	ModeContainer RuntimeMode = "container"
)

type InstanceStatus string

const (
	InstCreating  InstanceStatus = "creating"
	InstRunning   InstanceStatus = "running"
	InstStopped   InstanceStatus = "stopped"
	InstFailed    InstanceStatus = "failed"
	InstRestarting InstanceStatus = "restarting"
	InstDestroyed InstanceStatus = "destroyed"
)

type RestartPolicy string

const (
	RestartAlways    RestartPolicy = "always"
	RestartOnFailure RestartPolicy = "on_failure"
	RestartNever     RestartPolicy = "never"
)

// ── Data Structures ──

type ResourceLimits struct {
	CPUCores    float64 `json:"cpu_cores"`     // e.g. 0.5, 1.0, 2.0
	MemoryMB    int     `json:"memory_mb"`
	DiskMB      int     `json:"disk_mb"`
	NetworkMbps int     `json:"network_mbps,omitempty"`
}

type Instance struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	AgentID       string         `json:"agent_id"`
	Mode          RuntimeMode    `json:"mode"`
	Status        InstanceStatus `json:"status"`
	Image         string         `json:"image,omitempty"`    // container image
	Command       string         `json:"command,omitempty"`  // process command
	Env           map[string]string `json:"env,omitempty"`
	Limits        ResourceLimits `json:"limits"`
	RestartPolicy RestartPolicy  `json:"restart_policy"`
	RestartCount  int            `json:"restart_count"`
	MaxRestarts   int            `json:"max_restarts"`
	HealthCheck   string         `json:"health_check,omitempty"` // URL or command
	Healthy       bool           `json:"healthy"`
	PID           int            `json:"pid,omitempty"`
	ContainerID   string         `json:"container_id,omitempty"`
	Port          int            `json:"port,omitempty"`
	LogTail       []string       `json:"log_tail,omitempty"` // last N log lines
	CreatedAt     time.Time      `json:"created_at"`
	StartedAt     *time.Time     `json:"started_at,omitempty"`
	StoppedAt     *time.Time     `json:"stopped_at,omitempty"`
	LastHealthAt  *time.Time     `json:"last_health_at,omitempty"`
}

type InstanceEvent struct {
	InstanceID string    `json:"instance_id"`
	Type       string    `json:"type"` // created, started, stopped, failed, restarted, health_ok, health_fail, destroyed
	Message    string    `json:"message"`
	Timestamp  time.Time `json:"timestamp"`
}

// ── Engine ──

type EngineConfig struct {
	DefaultMode       RuntimeMode    `json:"default_mode"`
	DefaultLimits     ResourceLimits `json:"default_limits"`
	DefaultRestart    RestartPolicy  `json:"default_restart_policy"`
	MaxRestarts       int            `json:"max_restarts"`
	HealthInterval    int            `json:"health_interval_sec"`
	MaxInstances      int            `json:"max_instances"`
	LogTailLines      int            `json:"log_tail_lines"`
	ContainerRuntime  string         `json:"container_runtime"` // docker, podman
}

func DefaultEngineConfig() *EngineConfig {
	return &EngineConfig{
		DefaultMode: ModeProcess,
		DefaultLimits: ResourceLimits{
			CPUCores: 0.5,
			MemoryMB: 512,
			DiskMB:   2048,
		},
		DefaultRestart:   RestartOnFailure,
		MaxRestarts:      5,
		HealthInterval:   30,
		MaxInstances:     20,
		LogTailLines:     100,
		ContainerRuntime: "docker",
	}
}

type Engine struct {
	mu        sync.RWMutex
	nodeID    string
	config    *EngineConfig
	instances map[string]*Instance
	events    []InstanceEvent
	stats     EngineStats
	startAt   time.Time
	nextID    int
}

type EngineStats struct {
	InstancesTotal   int       `json:"instances_total"`
	InstancesRunning int       `json:"instances_running"`
	InstancesStopped int       `json:"instances_stopped"`
	InstancesFailed  int       `json:"instances_failed"`
	TotalRestarts    int       `json:"total_restarts"`
	HealthChecks     int       `json:"health_checks"`
	HealthFailures   int       `json:"health_failures"`
	Uptime           string    `json:"uptime"`
	LastEvent        time.Time `json:"last_event,omitempty"`
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
			nodeID:    nodeID,
			config:    cfg,
			instances: make(map[string]*Instance),
			events:    make([]InstanceEvent, 0),
			startAt:   time.Now(),
		}
		log.Printf("[chitin] runtime engine ready (mode=%s, max=%d, runtime=%s)",
			cfg.DefaultMode, cfg.MaxInstances, cfg.ContainerRuntime)
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

func (e *Engine) addEvent(instID, eventType, message string) {
	ev := InstanceEvent{
		InstanceID: instID,
		Type:       eventType,
		Message:    message,
		Timestamp:  time.Now(),
	}
	e.events = append(e.events, ev)
	if len(e.events) > 500 {
		e.events = e.events[1:]
	}
	e.stats.LastEvent = ev.Timestamp
}

// ── Instance Management ──

func (e *Engine) CreateInstance(name, agentID string, mode RuntimeMode, image, command string, env map[string]string, limits *ResourceLimits, restart RestartPolicy, healthCheck string, port int) (*Instance, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if len(e.instances) >= e.config.MaxInstances {
		return nil, fmt.Errorf("max instances (%d) reached", e.config.MaxInstances)
	}

	if mode == "" {
		mode = e.config.DefaultMode
	}
	if limits == nil {
		l := e.config.DefaultLimits
		limits = &l
	}
	if restart == "" {
		restart = e.config.DefaultRestart
	}

	inst := &Instance{
		ID:            e.genID("inst"),
		Name:          name,
		AgentID:       agentID,
		Mode:          mode,
		Status:        InstCreating,
		Image:         image,
		Command:       command,
		Env:           env,
		Limits:        *limits,
		RestartPolicy: restart,
		MaxRestarts:   e.config.MaxRestarts,
		HealthCheck:   healthCheck,
		Port:          port,
		LogTail:       make([]string, 0),
		CreatedAt:     time.Now(),
	}

	e.instances[inst.ID] = inst
	e.stats.InstancesTotal++
	e.addEvent(inst.ID, "created", fmt.Sprintf("Instance %s created (mode=%s, agent=%s)", name, mode, agentID))

	log.Printf("[chitin] instance created: %s — %s (mode=%s)", inst.ID, name, mode)
	return inst, nil
}

func (e *Engine) StartInstance(instID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	inst, ok := e.instances[instID]
	if !ok {
		return fmt.Errorf("instance %s not found", instID)
	}
	if inst.Status == InstRunning {
		return fmt.Errorf("instance %s already running", instID)
	}

	now := time.Now()
	inst.Status = InstRunning
	inst.StartedAt = &now
	inst.Healthy = true
	inst.LastHealthAt = &now
	e.stats.InstancesRunning++

	e.addEvent(instID, "started", fmt.Sprintf("Instance %s started", inst.Name))
	return nil
}

func (e *Engine) StopInstance(instID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	inst, ok := e.instances[instID]
	if !ok {
		return fmt.Errorf("instance %s not found", instID)
	}
	if inst.Status == InstStopped || inst.Status == InstDestroyed {
		return fmt.Errorf("instance %s already stopped", instID)
	}

	now := time.Now()
	inst.Status = InstStopped
	inst.StoppedAt = &now
	inst.Healthy = false
	if e.stats.InstancesRunning > 0 {
		e.stats.InstancesRunning--
	}
	e.stats.InstancesStopped++

	e.addEvent(instID, "stopped", fmt.Sprintf("Instance %s stopped", inst.Name))
	return nil
}

func (e *Engine) RestartInstance(instID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	inst, ok := e.instances[instID]
	if !ok {
		return fmt.Errorf("instance %s not found", instID)
	}

	if inst.RestartCount >= inst.MaxRestarts {
		inst.Status = InstFailed
		e.stats.InstancesFailed++
		e.addEvent(instID, "failed", fmt.Sprintf("Instance %s exceeded max restarts (%d)", inst.Name, inst.MaxRestarts))
		return fmt.Errorf("instance %s exceeded max restarts", instID)
	}

	now := time.Now()
	inst.Status = InstRestarting
	inst.RestartCount++
	e.stats.TotalRestarts++

	// Simulate restart completing
	inst.Status = InstRunning
	inst.StartedAt = &now
	inst.Healthy = true
	inst.LastHealthAt = &now

	e.addEvent(instID, "restarted", fmt.Sprintf("Instance %s restarted (count=%d)", inst.Name, inst.RestartCount))
	return nil
}

func (e *Engine) DestroyInstance(instID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	inst, ok := e.instances[instID]
	if !ok {
		return fmt.Errorf("instance %s not found", instID)
	}

	if inst.Status == InstRunning {
		if e.stats.InstancesRunning > 0 {
			e.stats.InstancesRunning--
		}
	}

	inst.Status = InstDestroyed
	now := time.Now()
	inst.StoppedAt = &now
	inst.Healthy = false

	e.addEvent(instID, "destroyed", fmt.Sprintf("Instance %s destroyed", inst.Name))
	delete(e.instances, instID)
	return nil
}

// ── Health ──

func (e *Engine) RecordHealthCheck(instID string, healthy bool, message string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	inst, ok := e.instances[instID]
	if !ok {
		return fmt.Errorf("instance %s not found", instID)
	}

	now := time.Now()
	inst.Healthy = healthy
	inst.LastHealthAt = &now
	e.stats.HealthChecks++

	if !healthy {
		e.stats.HealthFailures++
		e.addEvent(instID, "health_fail", fmt.Sprintf("Health check failed: %s", message))

		// Auto-restart if policy allows
		if inst.RestartPolicy == RestartAlways || inst.RestartPolicy == RestartOnFailure {
			if inst.RestartCount < inst.MaxRestarts {
				inst.RestartCount++
				inst.Status = InstRunning
				inst.StartedAt = &now
				e.stats.TotalRestarts++
				e.addEvent(instID, "restarted", fmt.Sprintf("Auto-restart after health failure (count=%d)", inst.RestartCount))
			} else {
				inst.Status = InstFailed
				e.stats.InstancesFailed++
				if e.stats.InstancesRunning > 0 {
					e.stats.InstancesRunning--
				}
			}
		}
	} else {
		e.addEvent(instID, "health_ok", "Health check passed")
	}
	return nil
}

// ── Logs ──

func (e *Engine) AppendLog(instID, line string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	inst, ok := e.instances[instID]
	if !ok {
		return fmt.Errorf("instance %s not found", instID)
	}

	inst.LogTail = append(inst.LogTail, line)
	if len(inst.LogTail) > e.config.LogTailLines {
		inst.LogTail = inst.LogTail[1:]
	}
	return nil
}

// ── Queries ──

func (e *Engine) GetInstance(instID string) (*Instance, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	inst, ok := e.instances[instID]
	if !ok {
		return nil, fmt.Errorf("instance %s not found", instID)
	}
	return inst, nil
}

func (e *Engine) ListInstances(status string) []*Instance {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*Instance, 0, len(e.instances))
	for _, inst := range e.instances {
		if status == "" || string(inst.Status) == status {
			result = append(result, inst)
		}
	}
	return result
}

func (e *Engine) ListEvents(limit int) []InstanceEvent {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if limit <= 0 || limit > len(e.events) {
		limit = len(e.events)
	}
	if limit == 0 {
		return nil
	}
	start := len(e.events) - limit
	result := make([]InstanceEvent, limit)
	copy(result, e.events[start:])
	return result
}

func (e *Engine) Stats() *EngineStats {
	e.mu.RLock()
	defer e.mu.RUnlock()
	s := e.stats
	s.Uptime = time.Since(e.startAt).Round(time.Second).String()
	// Recount running
	running := 0
	for _, inst := range e.instances {
		if inst.Status == InstRunning {
			running++
		}
	}
	s.InstancesRunning = running
	return &s
}

func (e *Engine) Config() *EngineConfig {
	return e.config
}
