package swarm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ════════════════════════════════════════════════════════════
// Phase 3C — Swarm Formation Engine
//
// Enables cross-node Agent collaboration:
//   - Dispatch agent tasks to remote Claw nodes via Hive discovery
//   - Track formation membership and roles
//   - Publish swarm events to Pheromone ESB
//   - Handle incoming delegated tasks from other nodes
//
// Integration:
//   Claw (local) → Hive /discovery/select → remote Claw /v1/swarm/task
//   Events → Pheromone ESB → all subscribed nodes
// ════════════════════════════════════════════════════════════

// FormationRole defines a node's role in a swarm formation
type FormationRole string

const (
	RoleCaptain  FormationRole = "captain"   // orchestrator / coordinator
	RoleWorker   FormationRole = "worker"    // executes delegated tasks
	RoleObserver FormationRole = "observer"  // monitors, doesn't execute
)

// FormationMember represents a node participating in a formation
type FormationMember struct {
	NodeID   string        `json:"node_id"`
	ClawID   string        `json:"claw_id"`
	Address  string        `json:"address"`
	Role     FormationRole `json:"role"`
	Agents   []string      `json:"agents"`
	JoinedAt time.Time     `json:"joined_at"`
	LastSeen time.Time     `json:"last_seen"`
	Status   string        `json:"status"` // active, stale, left
}

// Formation represents a group of nodes working together
type Formation struct {
	ID        string             `json:"id"`
	Name      string             `json:"name"`
	CaptainID string             `json:"captain_id"`
	Members   []*FormationMember `json:"members"`
	CreatedAt time.Time          `json:"created_at"`
	Status    string             `json:"status"` // active, disbanded
}

// SwarmTask represents a task delegated to a remote node
type SwarmTask struct {
	ID          string    `json:"id"`
	FormationID string    `json:"formation_id,omitempty"`
	SourceNode  string    `json:"source_node"`
	TargetNode  string    `json:"target_node"`
	AgentID     string    `json:"agent_id"`
	TaskType    string    `json:"task_type"` // chat, workflow, tool_call
	Payload     string    `json:"payload"`
	Priority    int       `json:"priority"`
	Status      string    `json:"status"` // pending, dispatched, running, completed, failed
	Result      string    `json:"result,omitempty"`
	Error       string    `json:"error,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	DispatchedAt *time.Time `json:"dispatched_at,omitempty"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
}

// SwarmEvent is published to Pheromone for cross-node visibility
type SwarmEvent struct {
	Type      string      `json:"type"`
	NodeID    string      `json:"node_id"`
	ClawID    string      `json:"claw_id"`
	Timestamp time.Time   `json:"timestamp"`
	Data      interface{} `json:"data"`
}

// FormationEngine manages swarm formations and cross-node task dispatch
type FormationEngine struct {
	mu         sync.RWMutex
	formations map[string]*Formation
	tasks      map[string]*SwarmTask
	localNode  string
	localClaw  string
	localAddr  string
	hiveURL    string // Hive discovery URL (e.g. http://hive:9090)
	httpC      *http.Client
}

// NewFormationEngine creates a new formation engine
func NewFormationEngine(nodeID, clawID, localAddr string) *FormationEngine {
	hiveURL := os.Getenv("HIVE_URL")
	if hiveURL == "" {
		hiveURL = os.Getenv("HIVE_DISCOVERY_URL")
	}

	fe := &FormationEngine{
		formations: make(map[string]*Formation),
		tasks:      make(map[string]*SwarmTask),
		localNode:  nodeID,
		localClaw:  clawID,
		localAddr:  localAddr,
		hiveURL:    hiveURL,
		httpC:      &http.Client{Timeout: 30 * time.Second},
	}
	log.Printf("[formation] engine ready (node=%s, hive=%s)", nodeID, hiveURL)
	return fe
}

// ── Formation Management ──

// CreateFormation creates a new formation with this node as captain
func (fe *FormationEngine) CreateFormation(name string) *Formation {
	fe.mu.Lock()
	defer fe.mu.Unlock()

	f := &Formation{
		ID:        uuid.New().String(),
		Name:      name,
		CaptainID: fe.localNode,
		Members: []*FormationMember{{
			NodeID:   fe.localNode,
			ClawID:   fe.localClaw,
			Address:  fe.localAddr,
			Role:     RoleCaptain,
			JoinedAt: time.Now(),
			LastSeen: time.Now(),
			Status:   "active",
		}},
		CreatedAt: time.Now(),
		Status:    "active",
	}
	fe.formations[f.ID] = f

	fe.publishEvent("formation.created", map[string]interface{}{
		"formation_id": f.ID,
		"name":         f.Name,
		"captain":      fe.localNode,
	})

	log.Printf("[formation] created %s (%s)", f.ID[:8], name)
	return f
}

// JoinFormation adds a remote node to a formation
func (fe *FormationEngine) JoinFormation(formationID, nodeID, clawID, address string, agents []string, role FormationRole) error {
	fe.mu.Lock()
	defer fe.mu.Unlock()

	f, ok := fe.formations[formationID]
	if !ok {
		return fmt.Errorf("formation %s not found", formationID)
	}

	// Check if already a member
	for _, m := range f.Members {
		if m.NodeID == nodeID {
			m.LastSeen = time.Now()
			m.Status = "active"
			m.Agents = agents
			return nil
		}
	}

	f.Members = append(f.Members, &FormationMember{
		NodeID:   nodeID,
		ClawID:   clawID,
		Address:  address,
		Role:     role,
		Agents:   agents,
		JoinedAt: time.Now(),
		LastSeen: time.Now(),
		Status:   "active",
	})

	fe.publishEvent("formation.joined", map[string]interface{}{
		"formation_id": formationID,
		"node_id":      nodeID,
		"role":         role,
	})

	return nil
}

// DisbandFormation marks a formation as disbanded
func (fe *FormationEngine) DisbandFormation(formationID string) error {
	fe.mu.Lock()
	defer fe.mu.Unlock()

	f, ok := fe.formations[formationID]
	if !ok {
		return fmt.Errorf("formation %s not found", formationID)
	}
	f.Status = "disbanded"

	fe.publishEvent("formation.disbanded", map[string]interface{}{
		"formation_id": formationID,
	})
	return nil
}

// GetFormation returns a formation by ID
func (fe *FormationEngine) GetFormation(id string) (*Formation, bool) {
	fe.mu.RLock()
	defer fe.mu.RUnlock()
	f, ok := fe.formations[id]
	return f, ok
}

// ListFormations returns all active formations
func (fe *FormationEngine) ListFormations() []*Formation {
	fe.mu.RLock()
	defer fe.mu.RUnlock()
	var result []*Formation
	for _, f := range fe.formations {
		if f.Status == "active" {
			result = append(result, f)
		}
	}
	return result
}

// ── Cross-Node Task Dispatch ──

// DispatchTask sends a task to a remote node
func (fe *FormationEngine) DispatchTask(ctx context.Context, targetAddr, agentID, taskType, payload string) (*SwarmTask, error) {
	task := &SwarmTask{
		ID:         uuid.New().String(),
		SourceNode: fe.localNode,
		AgentID:    agentID,
		TaskType:   taskType,
		Payload:    payload,
		Status:     "dispatched",
		CreatedAt:  time.Now(),
	}

	now := time.Now()
	task.DispatchedAt = &now

	// Send to remote node
	body, _ := json.Marshal(task)
	reqURL := targetAddr + "/v1/swarm/task"
	req, err := http.NewRequestWithContext(ctx, "POST", reqURL, bytes.NewReader(body))
	if err != nil {
		task.Status = "failed"
		task.Error = err.Error()
		return task, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := fe.httpC.Do(req)
	if err != nil {
		task.Status = "failed"
		task.Error = err.Error()
		fe.publishEvent("swarm.task.failed", map[string]interface{}{
			"task_id": task.ID,
			"error":   err.Error(),
		})
		return task, fmt.Errorf("dispatch to %s failed: %w", targetAddr, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		task.Status = "failed"
		task.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
		return task, fmt.Errorf("dispatch rejected: HTTP %d", resp.StatusCode)
	}

	task.Status = "running"
	task.TargetNode = targetAddr

	fe.mu.Lock()
	fe.tasks[task.ID] = task
	fe.mu.Unlock()

	fe.publishEvent("swarm.task.dispatched", map[string]interface{}{
		"task_id":  task.ID,
		"target":   targetAddr,
		"agent_id": agentID,
		"type":     taskType,
	})

	log.Printf("[formation] task %s dispatched to %s (agent=%s)", task.ID[:8], targetAddr, agentID)
	return task, nil
}

// DispatchViaHive selects the best node via Hive discovery and dispatches
func (fe *FormationEngine) DispatchViaHive(ctx context.Context, agentID, taskType, payload string) (*SwarmTask, error) {
	if fe.hiveURL == "" {
		return nil, fmt.Errorf("HIVE_URL not configured")
	}

	// Query Hive for best node
	selectURL := fmt.Sprintf("%s/hive/discovery/select?agent=%s", fe.hiveURL, agentID)
	req, err := http.NewRequestWithContext(ctx, "GET", selectURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := fe.httpC.Do(req)
	if err != nil {
		return nil, fmt.Errorf("hive discovery failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("no qualifying node found (HTTP %d)", resp.StatusCode)
	}

	var node struct {
		ID      string `json:"id"`
		Address string `json:"address"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&node); err != nil {
		return nil, fmt.Errorf("decode node: %w", err)
	}

	return fe.DispatchTask(ctx, node.Address, agentID, taskType, payload)
}

// CompleteTask marks a dispatched task as completed
func (fe *FormationEngine) CompleteTask(taskID, result, errMsg string) bool {
	fe.mu.Lock()
	defer fe.mu.Unlock()

	task, ok := fe.tasks[taskID]
	if !ok {
		return false
	}

	now := time.Now()
	task.CompletedAt = &now
	task.Result = result
	task.Error = errMsg
	if errMsg != "" {
		task.Status = "failed"
	} else {
		task.Status = "completed"
	}

	fe.publishEvent("swarm.task.completed", map[string]interface{}{
		"task_id": taskID,
		"status":  task.Status,
	})
	return true
}

// GetTask returns a task by ID
func (fe *FormationEngine) GetTask(id string) (*SwarmTask, bool) {
	fe.mu.RLock()
	defer fe.mu.RUnlock()
	t, ok := fe.tasks[id]
	return t, ok
}

// ListTasks returns tasks filtered by status
func (fe *FormationEngine) ListTasks(status string) []*SwarmTask {
	fe.mu.RLock()
	defer fe.mu.RUnlock()
	var result []*SwarmTask
	for _, t := range fe.tasks {
		if status == "" || t.Status == status {
			result = append(result, t)
		}
	}
	return result
}

// Stats returns formation engine statistics
func (fe *FormationEngine) Stats() map[string]interface{} {
	fe.mu.RLock()
	defer fe.mu.RUnlock()

	activeFormations := 0
	totalMembers := 0
	for _, f := range fe.formations {
		if f.Status == "active" {
			activeFormations++
			totalMembers += len(f.Members)
		}
	}

	byStatus := map[string]int{}
	for _, t := range fe.tasks {
		byStatus[t.Status]++
	}

	return map[string]interface{}{
		"active_formations": activeFormations,
		"total_members":     totalMembers,
		"total_tasks":       len(fe.tasks),
		"tasks_by_status":   byStatus,
		"hive_url":          fe.hiveURL,
		"local_node":        fe.localNode,
	}
}

// ── Pheromone Events ──

func (fe *FormationEngine) publishEvent(eventType string, data interface{}) {
	apiURL := os.Getenv("PHEROMONE_API_URL")
	if apiURL == "" {
		return
	}

	evt := SwarmEvent{
		Type:      eventType,
		NodeID:    fe.localNode,
		ClawID:    fe.localClaw,
		Timestamp: time.Now(),
		Data:      data,
	}

	go func() {
		payload, err := json.Marshal(evt)
		if err != nil {
			return
		}
		body, _ := json.Marshal(map[string]interface{}{
			"subject": "pheromone.events.swarm." + eventType,
			"payload": json.RawMessage(payload),
		})
		req, err := http.NewRequest("POST", apiURL+"/api/events", bytes.NewReader(body))
		if err != nil {
			return
		}
		req.Header.Set("Content-Type", "application/json")
		if token := os.Getenv("PHEROMONE_TOKEN"); token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err := fe.httpC.Do(req)
		if err != nil {
			log.Printf("[formation] pheromone publish %s failed: %v", eventType, err)
			return
		}
		resp.Body.Close()
	}()
}
