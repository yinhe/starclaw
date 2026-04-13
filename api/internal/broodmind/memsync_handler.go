package broodmind

import (
	"encoding/json"
	"net/http"
	"strings"
)

// ════════════════════════════════════════════════════════════
// MemSync HTTP Handlers — exposed via Claw /v1/broodmind/sync/*
// ════════════════════════════════════════════════════════════

// HandleSyncStats handles GET /broodmind/sync/stats
func HandleSyncStats(w http.ResponseWriter, r *http.Request) {
	if instance == nil || instance.MemSync == nil {
		http.Error(w, `{"error":"memsync not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"stats":  instance.MemSync.Stats(),
		"config": instance.MemSync.SyncConfig(),
		"peers":  instance.MemSync.ListPeers(),
	})
}

// HandleSyncReceive handles POST /broodmind/sync/receive — incoming sync from peers
func HandleSyncReceive(w http.ResponseWriter, r *http.Request) {
	if instance == nil || instance.MemSync == nil {
		http.Error(w, `{"error":"memsync not initialized"}`, http.StatusServiceUnavailable)
		return
	}

	var req struct {
		Origin  string       `json:"origin"`
		Entries []*SyncEntry `json:"entries"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if req.Origin == "" || len(req.Entries) == 0 {
		http.Error(w, `{"error":"origin and entries required"}`, http.StatusBadRequest)
		return
	}

	applied, conflicts := instance.MemSync.ReceiveFromPeer(req.Origin, req.Entries)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"applied":   applied,
		"conflicts": conflicts,
	})
}

// HandleSyncFlush handles POST /broodmind/sync/flush — trigger immediate sync
func HandleSyncFlush(w http.ResponseWriter, r *http.Request) {
	if instance == nil || instance.MemSync == nil {
		http.Error(w, `{"error":"memsync not initialized"}`, http.StatusServiceUnavailable)
		return
	}

	pushed := instance.MemSync.FlushToPeers()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"peers_synced": pushed,
		"stats":        instance.MemSync.Stats(),
	})
}

// HandleSyncPeers handles GET /broodmind/sync/peers
func HandleSyncPeers(w http.ResponseWriter, r *http.Request) {
	if instance == nil || instance.MemSync == nil {
		http.Error(w, `{"error":"memsync not initialized"}`, http.StatusServiceUnavailable)
		return
	}

	peers := instance.MemSync.ListPeers()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"peers": peers,
		"count": len(peers),
	})
}

// HandleSyncAddPeer handles POST /broodmind/sync/peers
func HandleSyncAddPeer(w http.ResponseWriter, r *http.Request) {
	if instance == nil || instance.MemSync == nil {
		http.Error(w, `{"error":"memsync not initialized"}`, http.StatusServiceUnavailable)
		return
	}

	var req struct {
		NodeID  string `json:"node_id"`
		Address string `json:"address"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if req.NodeID == "" || req.Address == "" {
		http.Error(w, `{"error":"node_id and address required"}`, http.StatusBadRequest)
		return
	}

	instance.MemSync.AddPeer(req.NodeID, req.Address)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
}

// HandleSyncRemovePeer handles DELETE /broodmind/sync/peers/:id
func HandleSyncRemovePeer(w http.ResponseWriter, r *http.Request) {
	if instance == nil || instance.MemSync == nil {
		http.Error(w, `{"error":"memsync not initialized"}`, http.StatusServiceUnavailable)
		return
	}

	parts := strings.Split(r.URL.Path, "/")
	nodeID := parts[len(parts)-1]
	if nodeID == "" {
		http.Error(w, `{"error":"peer node_id required"}`, http.StatusBadRequest)
		return
	}

	instance.MemSync.RemovePeer(nodeID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
}

// HandleSyncConflicts handles GET /broodmind/sync/conflicts?resolved=false
func HandleSyncConflicts(w http.ResponseWriter, r *http.Request) {
	if instance == nil || instance.MemSync == nil {
		http.Error(w, `{"error":"memsync not initialized"}`, http.StatusServiceUnavailable)
		return
	}

	resolved := r.URL.Query().Get("resolved") == "true"
	conflicts := instance.MemSync.ListConflicts(resolved, 50)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"conflicts": conflicts,
		"count":     len(conflicts),
	})
}

// HandleSyncResolveConflict handles POST /broodmind/sync/conflicts/resolve
func HandleSyncResolveConflict(w http.ResponseWriter, r *http.Request) {
	if instance == nil || instance.MemSync == nil {
		http.Error(w, `{"error":"memsync not initialized"}`, http.StatusServiceUnavailable)
		return
	}

	var req struct {
		ConflictID string `json:"conflict_id"`
		Winner     string `json:"winner"` // "local", "remote", "merge"
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if req.ConflictID == "" || req.Winner == "" {
		http.Error(w, `{"error":"conflict_id and winner required"}`, http.StatusBadRequest)
		return
	}

	if err := instance.MemSync.ResolveConflict(req.ConflictID, req.Winner); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
}
