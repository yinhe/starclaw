package network

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw/internal/node"
)

// P2PHandler exposes DHT, Creep, Hivemind, and Evolution endpoints.
type P2PHandler struct {
	dht       *node.DHT
	creep     *node.CreepEngine
	hivemind  *node.HivemindEngine
	evolution *node.EvolutionEngine
}

// NewP2PHandler creates the P7 handler with all four engines.
func NewP2PHandler(dht *node.DHT, creep *node.CreepEngine, hivemind *node.HivemindEngine, evolution *node.EvolutionEngine) *P2PHandler {
	return &P2PHandler{
		dht:       dht,
		creep:     creep,
		hivemind:  hivemind,
		evolution: evolution,
	}
}

// ════════════════════════════════════════════════════════════════
//  DHT endpoints (inter-node RPC)
// ════════════════════════════════════════════════════════════════

// HandleDHTPing responds to DHT ping (liveness check).
func (h *P2PHandler) HandleDHTPing(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// HandleDHTFindNode handles FIND_NODE RPC.
func (h *P2PHandler) HandleDHTFindNode(c *gin.Context) {
	var req struct {
		Target  string `json:"target"`
		FromID  string `json:"from_id"`
		Address string `json:"address"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	nodes := h.dht.HandleFindNode(req.Target, req.FromID, req.Address)
	c.JSON(http.StatusOK, gin.H{"nodes": nodes})
}

// HandleDHTStore handles STORE RPC.
func (h *P2PHandler) HandleDHTStore(c *gin.Context) {
	var req struct {
		Key       string `json:"key"`
		Value     string `json:"value"`
		Publisher string `json:"publisher"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.dht.HandleStore(req.Key, req.Value, req.Publisher); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "stored"})
}

// HandleDHTFindValue handles FIND_VALUE RPC.
func (h *P2PHandler) HandleDHTFindValue(c *gin.Context) {
	var req struct {
		Key     string `json:"key"`
		FromID  string `json:"from_id"`
		Address string `json:"address"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	value, nodes := h.dht.HandleFindValue(req.Key, req.FromID, req.Address)
	if value != nil {
		c.JSON(http.StatusOK, gin.H{"found": true, "value": string(value)})
	} else {
		c.JSON(http.StatusOK, gin.H{"found": false, "nodes": nodes})
	}
}

// HandleDHTStats returns DHT statistics (authenticated).
func (h *P2PHandler) HandleDHTStats(c *gin.Context) {
	c.JSON(http.StatusOK, h.dht.Stats())
}

// ════════════════════════════════════════════════════════════════
//  Creep endpoints (CRDT sync)
// ════════════════════════════════════════════════════════════════

// HandleCreepSync handles anti-entropy sync request from a peer.
func (h *P2PHandler) HandleCreepSync(c *gin.Context) {
	var req struct {
		Digest map[string]string `json:"digest"`
		NodeID string            `json:"node_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	entries, need := h.creep.HandleSync(req.Digest)
	c.JSON(http.StatusOK, gin.H{"entries": entries, "need": need})
}

// HandleCreepPush handles incoming entries pushed by a peer.
func (h *P2PHandler) HandleCreepPush(c *gin.Context) {
	var req struct {
		Entries []node.CreepEntry `json:"entries"`
		NodeID  string            `json:"node_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	merged := h.creep.HandlePush(req.Entries)
	c.JSON(http.StatusOK, gin.H{"merged": merged})
}

// HandleCreepGet retrieves a value from the local Creep store (authenticated).
func (h *P2PHandler) HandleCreepGet(c *gin.Context) {
	ns := c.Query("namespace")
	key := c.Query("key")
	if ns == "" || key == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "namespace and key required"})
		return
	}
	entry := h.creep.Get(ns, key)
	if entry == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, entry)
}

// HandleCreepSet sets a LWW register value (authenticated).
func (h *P2PHandler) HandleCreepSet(c *gin.Context) {
	var req struct {
		Namespace string          `json:"namespace"`
		Key       string          `json:"key"`
		Value     json.RawMessage `json:"value"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	h.creep.SetRegister(req.Namespace, req.Key, req.Value)
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// HandleCreepStats returns Creep statistics (authenticated).
func (h *P2PHandler) HandleCreepStats(c *gin.Context) {
	c.JSON(http.StatusOK, h.creep.Stats())
}

// ════════════════════════════════════════════════════════════════
//  Hivemind endpoints (task routing)
// ════════════════════════════════════════════════════════════════

// HandleHivemindCapability receives capability advertisement from a peer.
func (h *P2PHandler) HandleHivemindCapability(c *gin.Context) {
	var cap node.NodeCapability
	if err := c.ShouldBindJSON(&cap); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	h.hivemind.HandleCapability(&cap)
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// HandleHivemindRoute routes a task to the best node (authenticated).
func (h *P2PHandler) HandleHivemindRoute(c *gin.Context) {
	var task node.TaskRequest
	if err := c.ShouldBindJSON(&task); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	assignment, err := h.hivemind.RouteTask(&task)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, assignment)
}

// HandleHivemindExecute executes a task forwarded from another node.
func (h *P2PHandler) HandleHivemindExecute(c *gin.Context) {
	// This endpoint is called by peers to execute tasks on this node.
	// The actual execution is delegated to the inference/agent system.
	// For now, acknowledge receipt.
	var task node.TaskRequest
	if err := c.ShouldBindJSON(&task); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"status":  "accepted",
		"task_id": task.ID,
		"message": "task queued for local execution",
	})
}

// HandleHivemindStats returns Hivemind statistics (authenticated).
func (h *P2PHandler) HandleHivemindStats(c *gin.Context) {
	c.JSON(http.StatusOK, h.hivemind.Stats())
}

// ════════════════════════════════════════════════════════════════
//  Evolution endpoints (agent self-improvement)
// ════════════════════════════════════════════════════════════════

// HandleEvolutionSeed seeds an initial variant for an agent (authenticated).
func (h *P2PHandler) HandleEvolutionSeed(c *gin.Context) {
	var variant node.AgentVariant
	if err := c.ShouldBindJSON(&variant); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	h.evolution.SeedVariant(&variant)
	c.JSON(http.StatusOK, gin.H{"status": "seeded", "variant_id": variant.ID})
}

// HandleEvolutionEval records an evaluation for a variant (authenticated).
func (h *P2PHandler) HandleEvolutionEval(c *gin.Context) {
	var req struct {
		VariantID    string  `json:"variant_id"`
		Completion   float64 `json:"completion"`
		Satisfaction float64 `json:"satisfaction"`
		LatencyMs    float64 `json:"latency_ms"`
		CostEff      float64 `json:"cost_efficiency"`
		ErrorRate    float64 `json:"error_rate"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	h.evolution.RecordEvaluation(req.VariantID, req.Completion, req.Satisfaction, req.LatencyMs, req.CostEff, req.ErrorRate)
	c.JSON(http.StatusOK, gin.H{"status": "recorded"})
}

// HandleEvolutionBest returns the best variant for an agent (authenticated).
func (h *P2PHandler) HandleEvolutionBest(c *gin.Context) {
	agentID := c.Query("agent_id")
	if agentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "agent_id required"})
		return
	}
	best := h.evolution.GetBestVariant(agentID)
	if best == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no evaluated variants"})
		return
	}
	c.JSON(http.StatusOK, best)
}

// HandleEvolutionEvolve triggers one generation of evolution (authenticated).
func (h *P2PHandler) HandleEvolutionEvolve(c *gin.Context) {
	var req struct {
		AgentID string `json:"agent_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	h.evolution.Evolve(req.AgentID)
	c.JSON(http.StatusOK, gin.H{"status": "evolved"})
}

// HandleEvolutionStats returns evolution statistics (authenticated).
func (h *P2PHandler) HandleEvolutionStats(c *gin.Context) {
	c.JSON(http.StatusOK, h.evolution.Stats())
}

// HandleP2POverview returns combined stats from all P7 subsystems.
func (h *P2PHandler) HandleP2POverview(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"dht":       h.dht.Stats(),
		"creep":     h.creep.Stats(),
		"hivemind":  h.hivemind.Stats(),
		"evolution": h.evolution.Stats(),
	})
}
