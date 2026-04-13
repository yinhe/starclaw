package federation

import (
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/yinhe/starclaw/internal/nerve"
	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════
// Swarm Federation — 虫群联邦 (Phase 5C)
//
// 多 Overmind 虫群互联互通:
//   1. Federation Protocol — 发现/握手/注册/心跳
//   2. Cross-Swarm Router — 跨虫群任务撮合与路由
//   3. Trust Chain — 跨虫群信誉 · 违约惩罚 · 渐进信任
//
// 每个 Overmind 对外是一个 "Swarm" 节点。
// 联邦注册中心 (可选) 或 P2P 发现。
// ════════════════════════════════════════════════════════════

// ── Types ──

type SwarmStatus string
type TrustLevel string
type TaskRouteStatus string

const (
	SwarmOnline    SwarmStatus = "online"
	SwarmDegraded  SwarmStatus = "degraded"
	SwarmOffline   SwarmStatus = "offline"
	SwarmSuspended SwarmStatus = "suspended"

	TrustNone     TrustLevel = "none"     // 未知虫群
	TrustBasic    TrustLevel = "basic"    // 握手通过
	TrustVerified TrustLevel = "verified" // 多次成功交互
	TrustAllied   TrustLevel = "allied"   // 联盟级信任

	RouteProposed  TaskRouteStatus = "proposed"
	RouteAccepted  TaskRouteStatus = "accepted"
	RouteRejected  TaskRouteStatus = "rejected"
	RouteRunning   TaskRouteStatus = "running"
	RouteCompleted TaskRouteStatus = "completed"
	RouteFailed    TaskRouteStatus = "failed"
)

// ── Data Structures ──

type SwarmNode struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Endpoint     string            `json:"endpoint"` // e.g. "https://swarm-b.starclaw.net/v1"
	Region       string            `json:"region"`   // e.g. "cn-east", "us-west"
	Status       SwarmStatus       `json:"status"`
	Trust        TrustLevel        `json:"trust"`
	Capabilities []string          `json:"capabilities"` // what this swarm can do
	NodeCount    int               `json:"node_count"`   // Claw nodes in this swarm
	AgentCount   int               `json:"agent_count"`
	Reputation   float64           `json:"reputation"` // 0-100
	LastSeen     time.Time         `json:"last_seen"`
	JoinedAt     time.Time         `json:"joined_at"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

type Handshake struct {
	ID          string     `json:"id"`
	FromSwarm   string     `json:"from_swarm"`
	ToSwarm     string     `json:"to_swarm"`
	Status      string     `json:"status"` // pending, accepted, rejected
	Challenge   string     `json:"challenge,omitempty"`
	Response    string     `json:"response,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

type TaskRoute struct {
	ID          string                 `json:"id"`
	SourceSwarm string                 `json:"source_swarm"`
	TargetSwarm string                 `json:"target_swarm"`
	TaskType    string                 `json:"task_type"`
	Description string                 `json:"description"`
	Params      map[string]interface{} `json:"params,omitempty"`
	Priority    string                 `json:"priority"`
	Status      TaskRouteStatus        `json:"status"`
	Bid         float64                `json:"bid"` // StarEnergy offered
	Result      map[string]interface{} `json:"result,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
}

type TrustEvent struct {
	ID        string    `json:"id"`
	SwarmID   string    `json:"swarm_id"`
	Type      string    `json:"type"`  // handshake_ok, task_success, task_fail, violation, alliance
	Delta     float64   `json:"delta"` // reputation change
	Details   string    `json:"details"`
	Timestamp time.Time `json:"timestamp"`
}

// ── Engine ──

type EngineConfig struct {
	SwarmID           string        `json:"swarm_id"`
	SwarmName         string        `json:"swarm_name"`
	Endpoint          string        `json:"endpoint"`
	Region            string        `json:"region"`
	HeartbeatInterval time.Duration `json:"heartbeat_interval"`
	TrustDecayRate    float64       `json:"trust_decay_rate"`    // per day
	MinTrustForRoute  float64       `json:"min_trust_for_route"` // min reputation to accept routed tasks
}

func DefaultConfig(swarmID string) *EngineConfig {
	return &EngineConfig{
		SwarmID:           swarmID,
		SwarmName:         "Swarm-" + swarmID[:8],
		Endpoint:          "http://localhost:8080/v1",
		Region:            "local",
		HeartbeatInterval: 30 * time.Second,
		TrustDecayRate:    0.5,
		MinTrustForRoute:  20,
	}
}

type Engine struct {
	mu          sync.RWMutex
	db          *gorm.DB
	config      *EngineConfig
	swarms      map[string]*SwarmNode
	handshakes  map[string]*Handshake
	routes      []TaskRoute
	trustEvents []TrustEvent
	stats       EngineStats
	startAt     time.Time
	nextID      int
}

type EngineStats struct {
	SwarmsKnown       int     `json:"swarms_known"`
	SwarmsOnline      int     `json:"swarms_online"`
	SwarmsAllied      int     `json:"swarms_allied"`
	HandshakesTotal   int     `json:"handshakes_total"`
	HandshakesOK      int     `json:"handshakes_ok"`
	RoutesProposed    int     `json:"routes_proposed"`
	RoutesCompleted   int     `json:"routes_completed"`
	RoutesFailed      int     `json:"routes_failed"`
	TotalVolumeRouted float64 `json:"total_volume_routed"` // StarEnergy
	TrustEventsLogged int     `json:"trust_events_logged"`
	Uptime            string  `json:"uptime"`
}

var (
	globalEngine *Engine
	engineOnce   sync.Once
)

func InitEngine(swarmID string, cfg *EngineConfig, db *gorm.DB) *Engine {
	if cfg == nil {
		cfg = DefaultConfig(swarmID)
	}
	engineOnce.Do(func() {
		globalEngine = &Engine{
			db:          db,
			config:      cfg,
			swarms:      make(map[string]*SwarmNode),
			handshakes:  make(map[string]*Handshake),
			routes:      make([]TaskRoute, 0),
			trustEvents: make([]TrustEvent, 0),
			startAt:     time.Now(),
		}
		globalEngine.loadFromDB()
		log.Printf("[federation] engine initialized — swarm=%s region=%s db=%v", cfg.SwarmName, cfg.Region, db != nil)
	})
	return globalEngine
}

func GetEngine() *Engine {
	return globalEngine
}

func (e *Engine) genID(prefix string) string {
	e.nextID++
	return fmt.Sprintf("%s-%d-%d", prefix, time.Now().UnixMilli(), e.nextID)
}

// ══════════════ Discovery & Registration ══════════════

func (e *Engine) RegisterSwarm(id, name, endpoint, region string, capabilities []string, nodeCount, agentCount int, meta map[string]string) *SwarmNode {
	e.mu.Lock()
	defer e.mu.Unlock()

	node := &SwarmNode{
		ID:           id,
		Name:         name,
		Endpoint:     endpoint,
		Region:       region,
		Status:       SwarmOnline,
		Trust:        TrustNone,
		Capabilities: capabilities,
		NodeCount:    nodeCount,
		AgentCount:   agentCount,
		Reputation:   10, // initial reputation
		LastSeen:     time.Now(),
		JoinedAt:     time.Now(),
		Metadata:     meta,
	}
	e.swarms[id] = node
	e.stats.SwarmsKnown++
	go e.persistSwarm(node)

	if bus := nerve.GetBus(); bus != nil {
		bus.Publish("federation.swarm.registered", "federation", map[string]interface{}{
			"swarm_id": id,
			"name":     name,
			"region":   region,
		})
	}
	log.Printf("[federation] swarm registered: %s (%s, %s)", name, id, region)
	return node
}

func (e *Engine) Heartbeat(swarmID string, nodeCount, agentCount int) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	node, ok := e.swarms[swarmID]
	if !ok {
		return fmt.Errorf("swarm %s not found", swarmID)
	}
	node.LastSeen = time.Now()
	node.NodeCount = nodeCount
	node.AgentCount = agentCount
	if node.Status == SwarmOffline {
		node.Status = SwarmOnline
	}
	go e.persistSwarm(node)
	return nil
}

func (e *Engine) ListSwarms(statusFilter string) []*SwarmNode {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var result []*SwarmNode
	for _, s := range e.swarms {
		if statusFilter != "" && string(s.Status) != statusFilter {
			continue
		}
		result = append(result, s)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Reputation > result[j].Reputation })
	return result
}

func (e *Engine) GetSwarm(id string) *SwarmNode {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.swarms[id]
}

// ══════════════ Handshake ══════════════

func (e *Engine) InitHandshake(targetSwarmID string) (*Handshake, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	target, ok := e.swarms[targetSwarmID]
	if !ok {
		return nil, fmt.Errorf("swarm %s not found", targetSwarmID)
	}
	if target.Trust != TrustNone {
		return nil, fmt.Errorf("swarm %s already has trust level %s", targetSwarmID, target.Trust)
	}

	hs := &Handshake{
		ID:        e.genID("hs"),
		FromSwarm: e.config.SwarmID,
		ToSwarm:   targetSwarmID,
		Status:    "pending",
		Challenge: fmt.Sprintf("challenge-%d", time.Now().UnixNano()),
		CreatedAt: time.Now(),
	}
	e.handshakes[hs.ID] = hs
	e.stats.HandshakesTotal++
	go e.persistHandshake(hs)
	return hs, nil
}

func (e *Engine) CompleteHandshake(handshakeID, response string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	hs, ok := e.handshakes[handshakeID]
	if !ok {
		return fmt.Errorf("handshake %s not found", handshakeID)
	}
	if hs.Status != "pending" {
		return fmt.Errorf("handshake %s is not pending", handshakeID)
	}

	now := time.Now()
	hs.Response = response
	hs.Status = "accepted"
	hs.CompletedAt = &now
	e.stats.HandshakesOK++
	go e.persistHandshake(hs)

	// Upgrade trust
	if target, ok := e.swarms[hs.ToSwarm]; ok {
		target.Trust = TrustBasic
		target.Reputation += 10
		go e.persistSwarm(target)
		e.recordTrustEvent(hs.ToSwarm, "handshake_ok", 10, "Handshake completed successfully")
	}

	if bus := nerve.GetBus(); bus != nil {
		bus.Publish("federation.handshake.completed", "federation", map[string]interface{}{
			"handshake_id": hs.ID,
			"target_swarm": hs.ToSwarm,
		})
	}
	return nil
}

// ══════════════ Cross-Swarm Task Routing ══════════════

func (e *Engine) ProposeRoute(targetSwarm, taskType, description, priority string, params map[string]interface{}, bid float64) (*TaskRoute, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	target, ok := e.swarms[targetSwarm]
	if !ok {
		return nil, fmt.Errorf("swarm %s not found", targetSwarm)
	}
	if target.Trust == TrustNone {
		return nil, fmt.Errorf("swarm %s has no trust — complete handshake first", targetSwarm)
	}
	if target.Reputation < e.config.MinTrustForRoute {
		return nil, fmt.Errorf("swarm %s reputation %.1f below minimum %.1f", targetSwarm, target.Reputation, e.config.MinTrustForRoute)
	}

	route := TaskRoute{
		ID:          e.genID("route"),
		SourceSwarm: e.config.SwarmID,
		TargetSwarm: targetSwarm,
		TaskType:    taskType,
		Description: description,
		Params:      params,
		Priority:    priority,
		Status:      RouteProposed,
		Bid:         bid,
		CreatedAt:   time.Now(),
	}
	e.routes = append(e.routes, route)
	e.stats.RoutesProposed++
	go e.persistTaskRoute(&route)

	if bus := nerve.GetBus(); bus != nil {
		bus.Publish("federation.route.proposed", "federation", map[string]interface{}{
			"route_id":     route.ID,
			"target_swarm": targetSwarm,
			"task_type":    taskType,
			"bid":          bid,
		})
	}
	return &route, nil
}

func (e *Engine) AcceptRoute(routeID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	for i := range e.routes {
		if e.routes[i].ID == routeID {
			if e.routes[i].Status != RouteProposed {
				return fmt.Errorf("route %s is not proposed (status=%s)", routeID, e.routes[i].Status)
			}
			e.routes[i].Status = RouteAccepted
			r := e.routes[i]
			go e.persistTaskRoute(&r)
			return nil
		}
	}
	return fmt.Errorf("route %s not found", routeID)
}

func (e *Engine) CompleteRoute(routeID string, result map[string]interface{}, success bool) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	for i := range e.routes {
		if e.routes[i].ID == routeID {
			now := time.Now()
			e.routes[i].CompletedAt = &now
			e.routes[i].Result = result

			swarmID := e.routes[i].TargetSwarm
			if success {
				e.routes[i].Status = RouteCompleted
				e.stats.RoutesCompleted++
				e.stats.TotalVolumeRouted += e.routes[i].Bid

				// Boost trust
				if target, ok := e.swarms[swarmID]; ok {
					target.Reputation += 5
					if target.Reputation >= 50 && target.Trust == TrustBasic {
						target.Trust = TrustVerified
						log.Printf("[federation] swarm %s upgraded to verified trust", swarmID)
					}
					if target.Reputation >= 80 && target.Trust == TrustVerified {
						target.Trust = TrustAllied
						log.Printf("[federation] swarm %s upgraded to allied trust", swarmID)
						e.stats.SwarmsAllied++
					}
				}
				e.recordTrustEvent(swarmID, "task_success", 5, "Cross-swarm task completed successfully")
			} else {
				e.routes[i].Status = RouteFailed
				e.stats.RoutesFailed++

				// Penalize trust
				if target, ok := e.swarms[swarmID]; ok {
					target.Reputation -= 10
					if target.Reputation < 0 {
						target.Reputation = 0
					}
					if target.Reputation < 10 && target.Trust != TrustNone {
						old := target.Trust
						target.Trust = TrustNone
						log.Printf("[federation] swarm %s trust downgraded %s → none (reputation=%.1f)", swarmID, old, target.Reputation)
					}
					go e.persistSwarm(target)
				}
				e.recordTrustEvent(swarmID, "task_fail", -10, "Cross-swarm task failed")
			}
			r := e.routes[i]
			go e.persistTaskRoute(&r)
			return nil
		}
	}
	return fmt.Errorf("route %s not found", routeID)
}

func (e *Engine) ListRoutes(status string, limit int) []TaskRoute {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var result []TaskRoute
	for _, r := range e.routes {
		if status != "" && string(r.Status) != status {
			continue
		}
		result = append(result, r)
	}
	if limit > 0 && len(result) > limit {
		result = result[len(result)-limit:]
	}
	return result
}

// FindBestSwarm finds the best swarm for a given capability requirement.
func (e *Engine) FindBestSwarm(capability string) *SwarmNode {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var best *SwarmNode
	for _, s := range e.swarms {
		if s.Status != SwarmOnline || s.Trust == TrustNone {
			continue
		}
		for _, cap := range s.Capabilities {
			if cap == capability {
				if best == nil || s.Reputation > best.Reputation {
					best = s
				}
				break
			}
		}
	}
	return best
}

// ══════════════ Trust ══════════════

func (e *Engine) recordTrustEvent(swarmID, eventType string, delta float64, details string) {
	evt := TrustEvent{
		ID:        e.genID("trust"),
		SwarmID:   swarmID,
		Type:      eventType,
		Delta:     delta,
		Details:   details,
		Timestamp: time.Now(),
	}
	e.trustEvents = append(e.trustEvents, evt)
	if len(e.trustEvents) > 500 {
		e.trustEvents = e.trustEvents[1:]
	}
	e.stats.TrustEventsLogged++
	go e.persistTrustEvent(&evt)
}

func (e *Engine) ListTrustEvents(swarmID string, limit int) []TrustEvent {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var result []TrustEvent
	for _, evt := range e.trustEvents {
		if swarmID != "" && evt.SwarmID != swarmID {
			continue
		}
		result = append(result, evt)
	}
	if limit > 0 && len(result) > limit {
		result = result[len(result)-limit:]
	}
	return result
}

func (e *Engine) SuspendSwarm(swarmID, reason string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	s, ok := e.swarms[swarmID]
	if !ok {
		return fmt.Errorf("swarm %s not found", swarmID)
	}
	s.Status = SwarmSuspended
	s.Trust = TrustNone
	s.Reputation = 0
	go e.persistSwarm(s)
	e.recordTrustEvent(swarmID, "violation", -100, "Suspended: "+reason)
	log.Printf("[federation] swarm %s SUSPENDED: %s", swarmID, reason)
	return nil
}

// ── Stats ──

func (e *Engine) Stats() *EngineStats {
	e.mu.RLock()
	defer e.mu.RUnlock()
	s := e.stats
	s.Uptime = time.Since(e.startAt).Round(time.Second).String()
	online := 0
	allied := 0
	for _, sw := range e.swarms {
		if sw.Status == SwarmOnline {
			online++
		}
		if sw.Trust == TrustAllied {
			allied++
		}
	}
	s.SwarmsKnown = len(e.swarms)
	s.SwarmsOnline = online
	s.SwarmsAllied = allied
	return &s
}
