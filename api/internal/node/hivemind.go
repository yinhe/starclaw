package node

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"sort"
	"sync"
	"time"
)

// ── Agent Capabilities ──

// AgentCapability describes an agent available on a node for squad collaboration.
type AgentCapability struct {
	AgentID     string   `json:"agent_id"`
	Name        string   `json:"name"`
	Specialty   string   `json:"specialty"`   // coding / design / video / writing / testing / sales / general
	Skills      []string `json:"skills"`      // ["ui_design","golang","react"]
	Description string   `json:"description"` // short summary (≤50 chars)
	Available   bool     `json:"available"`   // currently idle and accepting tasks
}

// ── Node Capabilities ──

// NodeCapability describes what a node can do.
type NodeCapability struct {
	NodeID       string            `json:"node_id"`
	Address      string            `json:"address"`
	Models       []string          `json:"models"` // available model IDs
	HasGPU       bool              `json:"has_gpu"`
	GPUMemoryMB  int               `json:"gpu_memory_mb"` // 0 if no GPU
	CPUCores     int               `json:"cpu_cores"`
	MemoryMB     int               `json:"memory_mb"`
	Region       string            `json:"region"`
	LatencyMs    int               `json:"latency_ms"`     // average response latency
	ActiveTasks  int               `json:"active_tasks"`   // current task count
	MaxTasks     int               `json:"max_tasks"`      // concurrency limit
	CostPerToken float64           `json:"cost_per_token"` // relative cost factor
	Online       bool              `json:"online"`
	LastReport   int64             `json:"last_report"`          // Unix timestamp
	Agents       []AgentCapability `json:"agents,omitempty"`     // agents available for squad collaboration
	TeamRoles    []string          `json:"team_roles,omitempty"` // team agent roles this node can fulfill (e.g. "architect", "coder", "reviewer")
}

// Load returns the utilization ratio (0.0 = idle, 1.0 = fully loaded).
func (nc *NodeCapability) Load() float64 {
	if nc.MaxTasks <= 0 {
		return 1.0
	}
	return float64(nc.ActiveTasks) / float64(nc.MaxTasks)
}

// HasModel returns true if the node supports a given model.
func (nc *NodeCapability) HasModel(model string) bool {
	for _, m := range nc.Models {
		if m == model {
			return true
		}
	}
	return false
}

// HasAgent returns true if the node has an agent with the given name (case-insensitive substring match).
func (nc *NodeCapability) HasAgent(name string) bool {
	for _, a := range nc.Agents {
		if a.Name == name {
			return true
		}
	}
	return false
}

// FindAgentBySpecialty returns the first available agent matching a specialty.
func (nc *NodeCapability) FindAgentBySpecialty(specialty string) *AgentCapability {
	for i := range nc.Agents {
		if nc.Agents[i].Specialty == specialty && nc.Agents[i].Available {
			return &nc.Agents[i]
		}
	}
	// Fallback: return any available agent
	for i := range nc.Agents {
		if nc.Agents[i].Available {
			return &nc.Agents[i]
		}
	}
	return nil
}

// ── Task Definition ──

// TaskType describes the kind of work to route.
type TaskType string

const (
	TaskChat      TaskType = "chat"
	TaskEmbed     TaskType = "embed"
	TaskRAG       TaskType = "rag"
	TaskWorkflow  TaskType = "workflow"
	TaskAgentExec TaskType = "agent_exec"
)

// TaskRequest is a unit of work to be routed to a node.
type TaskRequest struct {
	ID          string          `json:"id"`
	Type        TaskType        `json:"type"`
	Model       string          `json:"model"`    // required model
	Priority    int             `json:"priority"` // 0=normal, 1=high, 2=critical
	RequireGPU  bool            `json:"require_gpu"`
	PreferLocal bool            `json:"prefer_local"` // prefer local node if capable
	MaxLatency  int             `json:"max_latency"`  // max acceptable latency in ms (0=any)
	Payload     json.RawMessage `json:"payload"`
	CreatedAt   int64           `json:"created_at"`
}

// TaskAssignment is the routing decision for a task.
type TaskAssignment struct {
	TaskID  string  `json:"task_id"`
	NodeID  string  `json:"node_id"`
	Address string  `json:"address"`
	Score   float64 `json:"score"`  // routing score (higher = better fit)
	Reason  string  `json:"reason"` // why this node was chosen
}

// RoutingStrategy defines how tasks are assigned to nodes.
type RoutingStrategy string

const (
	StrategyLatency  RoutingStrategy = "latency"     // minimize latency
	StrategyCost     RoutingStrategy = "cost"        // minimize cost
	StrategyBalanced RoutingStrategy = "balanced"    // balanced (default)
	StrategyLocal    RoutingStrategy = "local_first" // prefer local node
)

// AgentProvider is a callback that returns the current local agent capabilities.
// It is set by the router after DB and agent system are initialized.
type AgentProvider func() []AgentCapability

// HivemindEngine routes tasks across the cluster based on node capabilities.
type HivemindEngine struct {
	selfNodeID string
	selfCap    *NodeCapability // this node's capabilities
	peers      map[string]*NodeCapability
	mu         sync.RWMutex
	strategy   RoutingStrategy
	gossip     *GossipEngine
	httpC      *http.Client
	stopCh     chan struct{}

	// Agent capability provider (set by router)
	agentProvider AgentProvider

	// Metrics
	taskCount    int64
	routedRemote int64
	routedLocal  int64
}

// NewHivemindEngine creates a new cluster task routing engine.
func NewHivemindEngine(selfNodeID string, gossip *GossipEngine) *HivemindEngine {
	return &HivemindEngine{
		selfNodeID: selfNodeID,
		peers:      make(map[string]*NodeCapability),
		strategy:   StrategyBalanced,
		gossip:     gossip,
		httpC:      &http.Client{Timeout: 5 * time.Second},
		stopCh:     make(chan struct{}),
	}
}

// SetAgentProvider sets the callback that provides local agent capabilities.
// Called by the router after DB is initialized.
func (hm *HivemindEngine) SetAgentProvider(provider AgentProvider) {
	hm.mu.Lock()
	hm.agentProvider = provider
	hm.mu.Unlock()
}

// SetSelfCapability sets this node's reported capabilities.
func (hm *HivemindEngine) SetSelfCapability(cap *NodeCapability) {
	hm.mu.Lock()
	hm.selfCap = cap
	hm.mu.Unlock()
}

// SetStrategy sets the routing strategy.
func (hm *HivemindEngine) SetStrategy(s RoutingStrategy) {
	hm.mu.Lock()
	hm.strategy = s
	hm.mu.Unlock()
}

// Start begins capability advertisement and collection.
func (hm *HivemindEngine) Start(interval time.Duration) {
	if interval < 10*time.Second {
		interval = 30 * time.Second
	}
	go hm.capabilityLoop(interval)
	log.Printf("[hivemind] started with strategy=%s, interval=%s", hm.strategy, interval)
}

// Stop halts the engine.
func (hm *HivemindEngine) Stop() {
	select {
	case <-hm.stopCh:
	default:
		close(hm.stopCh)
	}
}

// ── Task Routing ──

// RouteTask selects the best node for a task.
func (hm *HivemindEngine) RouteTask(task *TaskRequest) (*TaskAssignment, error) {
	hm.mu.RLock()
	strategy := hm.strategy
	candidates := hm.buildCandidates(task)
	hm.mu.RUnlock()

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no available node for model=%s gpu=%v", task.Model, task.RequireGPU)
	}

	// Score each candidate
	type scored struct {
		cap   *NodeCapability
		score float64
	}
	var scoredList []scored
	for _, c := range candidates {
		s := hm.scoreNode(c, task, strategy)
		scoredList = append(scoredList, scored{cap: c, score: s})
	}

	// Sort by score descending
	sort.Slice(scoredList, func(i, j int) bool {
		return scoredList[i].score > scoredList[j].score
	})

	best := scoredList[0]
	reason := fmt.Sprintf("strategy=%s score=%.2f load=%.0f%% latency=%dms",
		strategy, best.score, best.cap.Load()*100, best.cap.LatencyMs)

	hm.mu.Lock()
	hm.taskCount++
	if best.cap.NodeID == hm.selfNodeID {
		hm.routedLocal++
	} else {
		hm.routedRemote++
	}
	hm.mu.Unlock()

	return &TaskAssignment{
		TaskID:  task.ID,
		NodeID:  best.cap.NodeID,
		Address: best.cap.Address,
		Score:   best.score,
		Reason:  reason,
	}, nil
}

// buildCandidates filters nodes that can handle the task.
func (hm *HivemindEngine) buildCandidates(task *TaskRequest) []*NodeCapability {
	var candidates []*NodeCapability

	check := func(cap *NodeCapability) {
		if !cap.Online {
			return
		}
		if cap.Load() >= 1.0 {
			return // fully loaded
		}
		if task.Model != "" && !cap.HasModel(task.Model) {
			return
		}
		if task.RequireGPU && !cap.HasGPU {
			return
		}
		if task.MaxLatency > 0 && cap.LatencyMs > task.MaxLatency {
			return
		}
		candidates = append(candidates, cap)
	}

	// Check self
	if hm.selfCap != nil {
		check(hm.selfCap)
	}

	// Check peers
	for _, cap := range hm.peers {
		check(cap)
	}

	return candidates
}

// scoreNode computes a routing score (higher = better fit).
func (hm *HivemindEngine) scoreNode(cap *NodeCapability, task *TaskRequest, strategy RoutingStrategy) float64 {
	score := 100.0

	// Load penalty (0-40 points)
	loadPenalty := cap.Load() * 40.0
	score -= loadPenalty

	// Latency penalty (0-30 points)
	if cap.LatencyMs > 0 {
		latencyPenalty := math.Min(float64(cap.LatencyMs)/10.0, 30.0)
		score -= latencyPenalty
	}

	// Cost penalty (0-20 points)
	if cap.CostPerToken > 0 {
		costPenalty := math.Min(cap.CostPerToken*10.0, 20.0)
		score -= costPenalty
	}

	// GPU bonus
	if task.RequireGPU && cap.HasGPU {
		score += 10.0
		// Larger GPU memory = better
		if cap.GPUMemoryMB > 0 {
			score += math.Min(float64(cap.GPUMemoryMB)/1000.0, 10.0)
		}
	}

	// Strategy-specific adjustments
	switch strategy {
	case StrategyLatency:
		// Extra weight on latency
		if cap.LatencyMs > 0 {
			score -= float64(cap.LatencyMs) / 5.0
		}
	case StrategyCost:
		// Extra weight on cost
		if cap.CostPerToken > 0 {
			score -= cap.CostPerToken * 20.0
		}
	case StrategyLocal:
		// Strong preference for local
		if cap.NodeID == hm.selfNodeID {
			score += 30.0
		}
	case StrategyBalanced:
		// Default scoring, no extra adjustments
	}

	// Priority boost for local node when prefer_local
	if task.PreferLocal && cap.NodeID == hm.selfNodeID {
		score += 15.0
	}

	return math.Max(score, 0)
}

// ── Capability exchange ──

// UpdatePeerCapability updates a remote peer's capabilities.
func (hm *HivemindEngine) UpdatePeerCapability(cap *NodeCapability) {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	cap.LastReport = time.Now().Unix()
	hm.peers[cap.NodeID] = cap
}

// capabilityLoop periodically broadcasts and collects capabilities.
func (hm *HivemindEngine) capabilityLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	cleanTicker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()
	defer cleanTicker.Stop()

	for {
		select {
		case <-ticker.C:
			hm.broadcastCapability()
		case <-cleanTicker.C:
			hm.cleanStalePeers()
		case <-hm.stopCh:
			return
		}
	}
}

func (hm *HivemindEngine) broadcastCapability() {
	hm.mu.RLock()
	cap := hm.selfCap
	provider := hm.agentProvider
	hm.mu.RUnlock()

	if cap == nil || hm.gossip == nil {
		return
	}

	// Inject fresh agent capabilities before broadcast
	if provider != nil {
		cap.Agents = provider()
	}

	// Derive TeamRoles from agent specialties
	cap.TeamRoles = deriveTeamRoles(cap.Agents)

	capJSON, _ := json.Marshal(cap)

	peers := hm.gossip.GetPeers()
	for _, p := range peers {
		if p.Address == "" {
			continue
		}
		go func(addr string) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			req, _ := http.NewRequestWithContext(ctx, "POST", addr+"/v1/peer/hivemind/capability",
				bytes.NewReader(capJSON))
			req.Header.Set("Content-Type", "application/json")
			resp, err := hm.httpC.Do(req)
			if err != nil {
				return
			}
			resp.Body.Close()
		}(p.Address)
	}
}

// deriveTeamRoles maps agent specialties to team agent roles.
// Every node with an LLM can be an architect/reviewer; coding agents → coder; etc.
func deriveTeamRoles(agents []AgentCapability) []string {
	roleSet := map[string]bool{
		"architect": true, // every node can architect (LLM planning)
		"reviewer":  true, // every node can review
	}
	for _, a := range agents {
		switch a.Specialty {
		case "coding":
			roleSet["coder"] = true
			roleSet["tester"] = true
		case "design":
			roleSet["designer"] = true
		case "video":
			roleSet["video_producer"] = true
		case "writing":
			roleSet["content_writer"] = true
			roleSet["copywriter"] = true
		case "testing":
			roleSet["tester"] = true
		case "sales":
			roleSet["sdr"] = true
			roleSet["campaign_manager"] = true
		case "general":
			roleSet["ops"] = true
		}
	}
	roles := make([]string, 0, len(roleSet))
	for r := range roleSet {
		roles = append(roles, r)
	}
	sort.Strings(roles)
	return roles
}

func (hm *HivemindEngine) cleanStalePeers() {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	cutoff := time.Now().Add(-5 * time.Minute).Unix()
	for nodeID, cap := range hm.peers {
		if cap.LastReport < cutoff {
			delete(hm.peers, nodeID)
		}
	}
}

// ── HTTP handler ──

// HandleCapability processes incoming capability advertisement from a peer.
func (hm *HivemindEngine) HandleCapability(cap *NodeCapability) {
	if cap.NodeID == hm.selfNodeID {
		return
	}
	hm.UpdatePeerCapability(cap)
}

// HandleRouteRequest processes an incoming task routing request from a peer.
func (hm *HivemindEngine) HandleRouteRequest(task *TaskRequest) (*TaskAssignment, error) {
	return hm.RouteTask(task)
}

// ForwardTask sends a task to a remote node for execution.
func (hm *HivemindEngine) ForwardTask(ctx context.Context, assignment *TaskAssignment, task *TaskRequest) (json.RawMessage, error) {
	reqBody, _ := json.Marshal(task)
	req, err := http.NewRequestWithContext(ctx, "POST", assignment.Address+"/v1/peer/hivemind/execute",
		bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := hm.httpC.Do(req)
	if err != nil {
		return nil, fmt.Errorf("forward to %s: %w", assignment.NodeID[:min(16, len(assignment.NodeID))], err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("remote node %s returned %d", assignment.NodeID[:min(16, len(assignment.NodeID))], resp.StatusCode)
	}

	var result json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}

// ── Squad Integration ──

// GetAllAgentCapabilities returns agent capabilities from all known nodes (self + peers).
func (hm *HivemindEngine) GetAllAgentCapabilities() map[string][]AgentCapability {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	result := make(map[string][]AgentCapability)
	if hm.selfCap != nil && len(hm.selfCap.Agents) > 0 {
		result[hm.selfNodeID] = hm.selfCap.Agents
	}
	for nodeID, cap := range hm.peers {
		if len(cap.Agents) > 0 {
			result[nodeID] = cap.Agents
		}
	}
	return result
}

// FindBestNodeForSpecialty returns the best node ID and agent for a given specialty.
// It considers agent availability, node load, and latency.
func (hm *HivemindEngine) FindBestNodeForSpecialty(specialty string, excludeNodes map[string]bool) (nodeID string, agent *AgentCapability) {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	type candidate struct {
		nodeID string
		agent  *AgentCapability
		score  float64
	}
	var candidates []candidate

	check := func(nid string, cap *NodeCapability) {
		if excludeNodes != nil && excludeNodes[nid] {
			return
		}
		if !cap.Online || cap.Load() >= 1.0 {
			return
		}
		ag := cap.FindAgentBySpecialty(specialty)
		if ag == nil {
			return
		}
		score := 100.0 - cap.Load()*40.0 - math.Min(float64(cap.LatencyMs)/10.0, 30.0)
		// Exact specialty match bonus
		if ag.Specialty == specialty {
			score += 20.0
		}
		candidates = append(candidates, candidate{nodeID: nid, agent: ag, score: score})
	}

	if hm.selfCap != nil {
		check(hm.selfNodeID, hm.selfCap)
	}
	for nid, cap := range hm.peers {
		check(nid, cap)
	}

	if len(candidates) == 0 {
		return "", nil
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})

	best := candidates[0]
	return best.nodeID, best.agent
}

// Stats returns Hivemind routing statistics.
func (hm *HivemindEngine) Stats() map[string]interface{} {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	peerStats := make([]map[string]interface{}, 0, len(hm.peers))
	for _, cap := range hm.peers {
		peerStats = append(peerStats, map[string]interface{}{
			"node_id":      cap.NodeID,
			"models":       len(cap.Models),
			"has_gpu":      cap.HasGPU,
			"load":         fmt.Sprintf("%.0f%%", cap.Load()*100),
			"latency_ms":   cap.LatencyMs,
			"active_tasks": cap.ActiveTasks,
			"online":       cap.Online,
		})
	}

	return map[string]interface{}{
		"strategy":      hm.strategy,
		"known_peers":   len(hm.peers),
		"total_tasks":   hm.taskCount,
		"routed_local":  hm.routedLocal,
		"routed_remote": hm.routedRemote,
		"peers":         peerStats,
	}
}
