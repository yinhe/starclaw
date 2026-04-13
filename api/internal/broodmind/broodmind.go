package broodmind

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

// ════════════════════════════════════════════════════════════
// BroodMind v0 — Cognitive Engine
//
// The "brain" of StarClaw. Provides:
//   - Memory management (3-layer: sensory → working → long-term)
//   - Context building for agent conversations
//   - Memory search & retrieval API
//   - Distillation (promote working → long-term)
//
// Future versions will add:
//   - Vector embeddings for semantic search
//   - Reasoning chains (plan → execute → reflect)
//   - Cross-node memory sync via Pheromone
//   - Privacy controls per memory type
// ════════════════════════════════════════════════════════════

// SkillDistillHook is called during the distill cycle with reflection artifacts
// and their source trajectory. Cerebrate registers this to create skill memories.
type SkillDistillHook func(artifacts []*ReflectionArtifact, trajectory *Trajectory)

// BroodMind is the main cognitive engine instance
type BroodMind struct {
	Memory     *MemoryStore
	Trajectory *TrajectoryStore
	Reflection *ReflectionEngine
	Distiller  *Distiller
	Arbiter    *Arbiter
	MemSync    *MemSync
	Policy     *PolicyEngine
	Planner    *Planner
	nodeID     string
	skillHook  SkillDistillHook // Hermes-inspired: bridge reflection → Cerebrate skill memories
}

var instance *BroodMind

// Init creates the global BroodMind instance
func Init(nodeID string) *BroodMind {
	instance = &BroodMind{
		Memory:     NewMemoryStore(10000),
		Trajectory: NewTrajectoryStore(500),
		Reflection: NewReflectionEngine(1000),
		Distiller:  NewDistiller(),
		Arbiter:    NewArbiter(nil),
		MemSync:    NewMemSync(nodeID, nil),
		Policy:     NewPolicyEngine(),
		Planner:    NewPlanner(nil),
		nodeID:     nodeID,
	}
	go instance.distillLoop()
	instance.MemSync.StartSyncLoop()
	log.Printf("[broodmind] v3 initialized for node %s (trajectory+reflection+distill+arbiter+memsync+planner)", nodeID)
	return instance
}

// Get returns the global BroodMind instance
func Get() *BroodMind {
	return instance
}

// NodeID returns the node identifier
func (bm *BroodMind) NodeID() string {
	return bm.nodeID
}

// StoreMemory is a convenience function for agents to store memories
func (bm *BroodMind) StoreMemory(layer MemoryLayer, memType MemoryType, content string, tags []string, source string) string {
	return bm.Memory.Store(&MemoryEntry{
		Layer:   layer,
		Type:    memType,
		Content: content,
		Tags:    tags,
		Source:  source,
		NodeID:  bm.nodeID,
	})
}

// BuildContext retrieves relevant memories for an agent conversation.
// Returns formatted context string to inject into the system prompt.
func (bm *BroodMind) BuildContext(query string, agentID string, maxTokens int) string {
	if maxTokens <= 0 {
		maxTokens = 2000
	}

	// Search across all layers for relevant memories
	results := bm.Memory.Search(query, "", "", 10)

	if len(results) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("<memory_context>\n")

	charBudget := maxTokens * 4 // rough chars-to-tokens
	used := 0
	for _, m := range results {
		entry := formatMemoryEntry(m)
		if used+len(entry) > charBudget {
			break
		}
		sb.WriteString(entry)
		sb.WriteString("\n")
		used += len(entry)
	}

	sb.WriteString("</memory_context>")
	return sb.String()
}

func formatMemoryEntry(m *MemoryEntry) string {
	prefix := ""
	switch m.Layer {
	case LayerLongTerm:
		prefix = "[LT]"
	case LayerWorking:
		prefix = "[WK]"
	case LayerSensory:
		prefix = "[SN]"
	}
	return prefix + " " + m.Content
}

// ── HTTP Handlers ──

// HandleMemoryStore handles POST /broodmind/memory
func HandleMemoryStore(w http.ResponseWriter, r *http.Request) {
	if instance == nil {
		http.Error(w, `{"error":"broodmind not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		Layer   string   `json:"layer"`
		Type    string   `json:"type"`
		Content string   `json:"content"`
		Tags    []string `json:"tags"`
		Source  string   `json:"source"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if req.Content == "" {
		http.Error(w, `{"error":"content required"}`, http.StatusBadRequest)
		return
	}

	layer := MemoryLayer(req.Layer)
	if layer == "" {
		layer = LayerWorking
	}
	memType := MemoryType(req.Type)
	if memType == "" {
		memType = MemSemantic
	}

	id := instance.StoreMemory(layer, memType, req.Content, req.Tags, req.Source)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"id": id})
}

// HandleMemorySearch handles GET /broodmind/memory/search?q=...&type=...&layer=...&limit=...
func HandleMemorySearch(w http.ResponseWriter, r *http.Request) {
	if instance == nil {
		http.Error(w, `{"error":"broodmind not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	query := r.URL.Query().Get("q")
	if query == "" {
		http.Error(w, `{"error":"q parameter required"}`, http.StatusBadRequest)
		return
	}
	memType := MemoryType(r.URL.Query().Get("type"))
	layer := MemoryLayer(r.URL.Query().Get("layer"))
	limit := 20

	results := instance.Memory.Search(query, memType, layer, limit)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"results": results,
		"count":   len(results),
	})
}

// HandleMemoryRetrieve handles GET /broodmind/memory/:id
func HandleMemoryRetrieve(w http.ResponseWriter, r *http.Request) {
	if instance == nil {
		http.Error(w, `{"error":"broodmind not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	// Extract ID from path — expects /broodmind/memory/{id}
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 4 {
		http.Error(w, `{"error":"id required"}`, http.StatusBadRequest)
		return
	}
	id := parts[len(parts)-1]

	entry := instance.Memory.Retrieve(id)
	if entry == nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entry)
}

// HandleMemoryDelete handles DELETE /broodmind/memory/:id
func HandleMemoryDelete(w http.ResponseWriter, r *http.Request) {
	if instance == nil {
		http.Error(w, `{"error":"broodmind not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 4 {
		http.Error(w, `{"error":"id required"}`, http.StatusBadRequest)
		return
	}
	id := parts[len(parts)-1]
	ok := instance.Memory.Delete(id)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"deleted": ok})
}

// HandleMemoryDistill handles POST /broodmind/memory/distill
func HandleMemoryDistill(w http.ResponseWriter, r *http.Request) {
	if instance == nil {
		http.Error(w, `{"error":"broodmind not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	promoted := instance.Memory.Distill()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{"promoted": promoted})
}

// HandleMemoryStats handles GET /broodmind/stats
func HandleMemoryStats(w http.ResponseWriter, r *http.Request) {
	if instance == nil {
		http.Error(w, `{"error":"broodmind not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"memory":     instance.Memory.Stats(),
		"trajectory": instance.Trajectory.Stats(),
		"reflection": instance.Reflection.Stats(),
		"distiller":  instance.Distiller.Stats(),
		"arbiter":    instance.Arbiter.Stats(),
		"memsync":    instance.MemSync.Stats(),
		"policy":     instance.Policy.Stats(),
		"planner":    instance.Planner.Stats(),
	})
}

// ── v1: Trajectory Handlers ──

// HandleTrajectoryList handles GET /broodmind/trajectories?limit=20&agent=...
func HandleTrajectoryList(w http.ResponseWriter, r *http.Request) {
	if instance == nil {
		http.Error(w, `{"error":"broodmind not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	agent := r.URL.Query().Get("agent")
	limit := 20
	if v := r.URL.Query().Get("limit"); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n > 0 {
			limit = n
		}
	}
	var traces []*Trajectory
	if agent != "" {
		traces = instance.Trajectory.ByAgent(agent, limit)
	} else {
		traces = instance.Trajectory.Recent(limit)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"trajectories": traces, "count": len(traces)})
}

// HandleTrajectoryGet handles GET /broodmind/trajectories/:id
func HandleTrajectoryGet(w http.ResponseWriter, r *http.Request) {
	if instance == nil {
		http.Error(w, `{"error":"broodmind not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	parts := strings.Split(r.URL.Path, "/")
	id := parts[len(parts)-1]
	t := instance.Trajectory.Get(id)
	if t == nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(t)
}

// ── v1: Reflection Handlers ──

// HandleReflectionList handles GET /broodmind/reflections?limit=20
func HandleReflectionList(w http.ResponseWriter, r *http.Request) {
	if instance == nil {
		http.Error(w, `{"error":"broodmind not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	limit := 20
	if v := r.URL.Query().Get("limit"); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n > 0 {
			limit = n
		}
	}
	arts := instance.Reflection.Recent(limit)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"reflections": arts, "count": len(arts)})
}

// HandleReflectionCandidates handles GET /broodmind/reflections/candidates
func HandleReflectionCandidates(w http.ResponseWriter, r *http.Request) {
	if instance == nil {
		http.Error(w, `{"error":"broodmind not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	candidates := instance.Reflection.Candidates(0.7)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"candidates": candidates, "count": len(candidates)})
}

// ── v1: Distilled Artifact Handlers ──

// HandleDistilledList handles GET /broodmind/distilled?type=...&limit=20
func HandleDistilledList(w http.ResponseWriter, r *http.Request) {
	if instance == nil {
		http.Error(w, `{"error":"broodmind not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n > 0 {
			limit = n
		}
	}
	dtype := r.URL.Query().Get("type")
	var arts []*DistilledArtifact
	if dtype != "" {
		arts = instance.Distiller.ByType(DistillType(dtype), limit)
	} else {
		arts = instance.Distiller.All(limit)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"artifacts": arts, "count": len(arts)})
}

// HandleDistilledApprove handles POST /broodmind/distilled/:id/approve
func HandleDistilledApprove(w http.ResponseWriter, r *http.Request) {
	if instance == nil {
		http.Error(w, `{"error":"broodmind not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	parts := strings.Split(r.URL.Path, "/")
	// expect .../distilled/{id}/approve
	if len(parts) < 3 {
		http.Error(w, `{"error":"id required"}`, http.StatusBadRequest)
		return
	}
	id := parts[len(parts)-2]
	ok := instance.Distiller.Approve(id, "admin")
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"approved": ok})
}

// HandleDistillNow handles POST /broodmind/distill — triggers an immediate distillation cycle
func HandleDistillNow(w http.ResponseWriter, r *http.Request) {
	if instance == nil {
		http.Error(w, `{"error":"broodmind not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	produced := instance.runDistillCycle()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"distilled": produced,
		"stats":     instance.Distiller.Stats(),
	})
}

// ── v1: Distillation Loop ──

// distillLoop runs periodic distillation: reflect completed trajectories, then distill candidates
func (bm *BroodMind) distillLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		bm.runDistillCycle()
	}
}

// SetSkillDistillHook registers a callback for Cerebrate skill distillation.
// Called during each distill cycle with high-quality reflection artifacts.
func (bm *BroodMind) SetSkillDistillHook(hook SkillDistillHook) {
	bm.skillHook = hook
}

func (bm *BroodMind) runDistillCycle() int {
	// 1. Reflect on recent completed trajectories
	traces := bm.Trajectory.Completed(50)
	for _, t := range traces {
		artifacts := bm.Reflection.Reflect(t)
		// Hermes-inspired: push artifacts to Cerebrate for skill memory creation
		if bm.skillHook != nil && len(artifacts) > 0 {
			bm.skillHook(artifacts, t)
		}
	}

	// 2. Distill high-quality reflection candidates
	candidates := bm.Reflection.Candidates(bm.Distiller.QualityThreshold)
	produced := bm.Distiller.Distill(candidates)

	// 3. Mark reflected candidates as distilled
	var distilledRefIDs []string
	for _, da := range produced {
		distilledRefIDs = append(distilledRefIDs, da.SourceRefID)
	}
	if len(distilledRefIDs) > 0 {
		bm.Reflection.MarkDistilled(distilledRefIDs)
	}

	// 4. Also promote high-value memories (existing v0 distill)
	bm.Memory.Distill()

	if len(produced) > 0 {
		log.Printf("[broodmind] distill cycle: %d traces reflected, %d artifacts distilled",
			len(traces), len(produced))
	}
	return len(produced)
}
