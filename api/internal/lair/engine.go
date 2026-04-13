package lair

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// ════════════════════════════════════════════════════════════
// Lair v1 — 受管多节点集群引擎
//
// 职责:
//   1. 节点注册: Agent节点加入Lair集群
//   2. 部署管理: 向节点部署/更新/移除Agent实例
//   3. 资源池: 聚合集群资源，感知节点负载
//   4. 健康监控: 心跳检测，自动标记离线节点
//   5. 调度: 基于负载/标签选择最优节点部署
//   6. 集群级操作: 批量部署、滚动更新、回滚
// ════════════════════════════════════════════════════════════

// ── Types ──

type NodeStatus string

const (
	NodeOnline      NodeStatus = "online"
	NodeOffline     NodeStatus = "offline"
	NodeDraining    NodeStatus = "draining"
	NodeMaintenance NodeStatus = "maintenance"
)

type DeployStatus string

const (
	DeployPending   DeployStatus = "pending"
	DeployRunning   DeployStatus = "running"
	DeploySuccess   DeployStatus = "success"
	DeployFailed    DeployStatus = "failed"
	DeployRolledBack DeployStatus = "rolled_back"
)

// ── Data Structures ──

type LairNode struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Address    string            `json:"address"`    // host:port
	BrooddPort int               `json:"broodd_port"` // broodd HTTP port
	Status     NodeStatus        `json:"status"`
	Labels     map[string]string `json:"labels,omitempty"`
	Resources  NodeResources     `json:"resources"`
	Instances  int               `json:"instances"`     // running instance count
	MaxInstances int             `json:"max_instances"`
	LastHeartbeat time.Time      `json:"last_heartbeat"`
	JoinedAt   time.Time         `json:"joined_at"`
}

type NodeResources struct {
	CPUTotal    float64 `json:"cpu_total"`
	CPUUsed     float64 `json:"cpu_used"`
	MemoryTotalMB int   `json:"memory_total_mb"`
	MemoryUsedMB  int   `json:"memory_used_mb"`
	DiskTotalMB   int   `json:"disk_total_mb"`
	DiskUsedMB    int   `json:"disk_used_mb"`
}

type Deployment struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	AgentID     string       `json:"agent_id"`
	Version     string       `json:"version"`
	NodeID      string       `json:"node_id"`
	Status      DeployStatus `json:"status"`
	Replicas    int          `json:"replicas"`
	Image       string       `json:"image,omitempty"`
	Command     string       `json:"command,omitempty"`
	Error       string       `json:"error,omitempty"`
	CreatedAt   time.Time    `json:"created_at"`
	CompletedAt *time.Time   `json:"completed_at,omitempty"`
}

type RolloutPlan struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	AgentID     string       `json:"agent_id"`
	Version     string       `json:"version"`
	Strategy    string       `json:"strategy"` // rolling, blue_green, canary
	NodeIDs     []string     `json:"node_ids"`
	BatchSize   int          `json:"batch_size"`
	Status      DeployStatus `json:"status"`
	Progress    int          `json:"progress"` // 0-100%
	Deployments []string     `json:"deployment_ids"`
	CreatedAt   time.Time    `json:"created_at"`
	CompletedAt *time.Time   `json:"completed_at,omitempty"`
}

// ── Engine ──

type EngineConfig struct {
	HeartbeatTimeout int    `json:"heartbeat_timeout_sec"`
	MaxNodes         int    `json:"max_nodes"`
	DefaultReplicas  int    `json:"default_replicas"`
	RolloutStrategy  string `json:"rollout_strategy"`
	RolloutBatchSize int    `json:"rollout_batch_size"`
}

func DefaultEngineConfig() *EngineConfig {
	return &EngineConfig{
		HeartbeatTimeout: 90,
		MaxNodes:         50,
		DefaultReplicas:  1,
		RolloutStrategy:  "rolling",
		RolloutBatchSize: 2,
	}
}

type Engine struct {
	mu          sync.RWMutex
	nodeID      string
	config      *EngineConfig
	nodes       map[string]*LairNode
	deployments map[string]*Deployment
	rollouts    map[string]*RolloutPlan
	stats       EngineStats
	startAt     time.Time
	nextID      int
}

type EngineStats struct {
	NodesTotal       int       `json:"nodes_total"`
	NodesOnline      int       `json:"nodes_online"`
	DeploymentsTotal int       `json:"deployments_total"`
	DeploymentsActive int      `json:"deployments_active"`
	RolloutsTotal    int       `json:"rollouts_total"`
	InstancesTotal   int       `json:"instances_total"`
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
			nodeID:      nodeID,
			config:      cfg,
			nodes:       make(map[string]*LairNode),
			deployments: make(map[string]*Deployment),
			rollouts:    make(map[string]*RolloutPlan),
			startAt:     time.Now(),
		}
		log.Printf("[lair] cluster engine ready (max_nodes=%d, heartbeat=%ds)", cfg.MaxNodes, cfg.HeartbeatTimeout)
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

// ── Node Management ──

func (e *Engine) RegisterNode(name, address string, brooddPort, maxInstances int, labels map[string]string, resources NodeResources) (*LairNode, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if len(e.nodes) >= e.config.MaxNodes {
		return nil, fmt.Errorf("max nodes (%d) reached", e.config.MaxNodes)
	}

	node := &LairNode{
		ID:            e.genID("node"),
		Name:          name,
		Address:       address,
		BrooddPort:    brooddPort,
		Status:        NodeOnline,
		Labels:        labels,
		Resources:     resources,
		MaxInstances:  maxInstances,
		LastHeartbeat: time.Now(),
		JoinedAt:      time.Now(),
	}

	e.nodes[node.ID] = node
	e.stats.NodesTotal++
	e.stats.NodesOnline++
	e.stats.LastActivity = time.Now()
	log.Printf("[lair] node registered: %s — %s (%s)", node.ID, name, address)
	return node, nil
}

func (e *Engine) Heartbeat(nodeID string, resources *NodeResources, instances int) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	node, ok := e.nodes[nodeID]
	if !ok {
		return fmt.Errorf("node %s not found", nodeID)
	}

	node.LastHeartbeat = time.Now()
	node.Instances = instances
	if resources != nil {
		node.Resources = *resources
	}
	if node.Status == NodeOffline {
		node.Status = NodeOnline
		e.stats.NodesOnline++
	}
	return nil
}

func (e *Engine) SetNodeStatus(nodeID string, status NodeStatus) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	node, ok := e.nodes[nodeID]
	if !ok {
		return fmt.Errorf("node %s not found", nodeID)
	}

	old := node.Status
	node.Status = status
	if old == NodeOnline && status != NodeOnline {
		e.stats.NodesOnline--
	} else if old != NodeOnline && status == NodeOnline {
		e.stats.NodesOnline++
	}
	e.stats.LastActivity = time.Now()
	return nil
}

func (e *Engine) RemoveNode(nodeID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	node, ok := e.nodes[nodeID]
	if !ok {
		return fmt.Errorf("node %s not found", nodeID)
	}
	if node.Status == NodeOnline {
		e.stats.NodesOnline--
	}
	delete(e.nodes, nodeID)
	return nil
}

func (e *Engine) ListNodes(status string) []*LairNode {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*LairNode, 0, len(e.nodes))
	for _, n := range e.nodes {
		if status == "" || string(n.Status) == status {
			result = append(result, n)
		}
	}
	return result
}

// ── Deployment ──

func (e *Engine) Deploy(name, agentID, version, nodeID, image, command string, replicas int) (*Deployment, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if nodeID != "" {
		node, ok := e.nodes[nodeID]
		if !ok {
			return nil, fmt.Errorf("node %s not found", nodeID)
		}
		if node.Status != NodeOnline {
			return nil, fmt.Errorf("node %s not online (status=%s)", nodeID, node.Status)
		}
	} else {
		// Auto-select node with lowest load
		nodeID = e.selectNode()
		if nodeID == "" {
			return nil, fmt.Errorf("no available nodes")
		}
	}

	if replicas <= 0 {
		replicas = e.config.DefaultReplicas
	}

	dep := &Deployment{
		ID:        e.genID("deploy"),
		Name:      name,
		AgentID:   agentID,
		Version:   version,
		NodeID:    nodeID,
		Status:    DeployRunning,
		Replicas:  replicas,
		Image:     image,
		Command:   command,
		CreatedAt: time.Now(),
	}

	e.deployments[dep.ID] = dep
	e.stats.DeploymentsTotal++
	e.stats.DeploymentsActive++
	e.stats.InstancesTotal += replicas
	e.stats.LastActivity = time.Now()

	log.Printf("[lair] deployment: %s → node %s (%s@%s x%d)", dep.ID, nodeID, agentID, version, replicas)
	return dep, nil
}

func (e *Engine) selectNode() string {
	var bestID string
	var bestLoad float64 = 999
	for _, n := range e.nodes {
		if n.Status != NodeOnline || n.Instances >= n.MaxInstances {
			continue
		}
		load := float64(n.Instances) / float64(n.MaxInstances+1)
		if n.Resources.CPUTotal > 0 {
			load = n.Resources.CPUUsed / n.Resources.CPUTotal
		}
		if load < bestLoad {
			bestLoad = load
			bestID = n.ID
		}
	}
	return bestID
}

func (e *Engine) FinishDeployment(deployID string, success bool, errMsg string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	dep, ok := e.deployments[deployID]
	if !ok {
		return fmt.Errorf("deployment %s not found", deployID)
	}

	now := time.Now()
	dep.CompletedAt = &now
	if success {
		dep.Status = DeploySuccess
	} else {
		dep.Status = DeployFailed
		dep.Error = errMsg
		e.stats.InstancesTotal -= dep.Replicas
	}
	e.stats.DeploymentsActive--
	e.stats.LastActivity = now
	return nil
}

func (e *Engine) ListDeployments(status string) []*Deployment {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*Deployment, 0, len(e.deployments))
	for _, d := range e.deployments {
		if status == "" || string(d.Status) == status {
			result = append(result, d)
		}
	}
	return result
}

// ── Rollout ──

func (e *Engine) CreateRollout(name, agentID, version, strategy string, nodeIDs []string, batchSize int) (*RolloutPlan, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if strategy == "" {
		strategy = e.config.RolloutStrategy
	}
	if batchSize <= 0 {
		batchSize = e.config.RolloutBatchSize
	}
	if len(nodeIDs) == 0 {
		for id, n := range e.nodes {
			if n.Status == NodeOnline {
				nodeIDs = append(nodeIDs, id)
			}
		}
	}
	if len(nodeIDs) == 0 {
		return nil, fmt.Errorf("no target nodes")
	}

	rollout := &RolloutPlan{
		ID:        e.genID("rollout"),
		Name:      name,
		AgentID:   agentID,
		Version:   version,
		Strategy:  strategy,
		NodeIDs:   nodeIDs,
		BatchSize: batchSize,
		Status:    DeployRunning,
		Progress:  0,
		CreatedAt: time.Now(),
	}

	e.rollouts[rollout.ID] = rollout
	e.stats.RolloutsTotal++
	e.stats.LastActivity = time.Now()
	log.Printf("[lair] rollout: %s — %s@%s → %d nodes (%s)", rollout.ID, agentID, version, len(nodeIDs), strategy)
	return rollout, nil
}

func (e *Engine) UpdateRollout(rolloutID string, progress int, status DeployStatus) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	r, ok := e.rollouts[rolloutID]
	if !ok {
		return fmt.Errorf("rollout %s not found", rolloutID)
	}
	r.Progress = progress
	r.Status = status
	if status == DeploySuccess || status == DeployFailed || status == DeployRolledBack {
		now := time.Now()
		r.CompletedAt = &now
	}
	e.stats.LastActivity = time.Now()
	return nil
}

func (e *Engine) ListRollouts() []*RolloutPlan {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*RolloutPlan, 0, len(e.rollouts))
	for _, r := range e.rollouts {
		result = append(result, r)
	}
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
