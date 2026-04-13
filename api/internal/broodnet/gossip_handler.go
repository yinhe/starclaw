package broodnet

import (
	"encoding/json"
	"net/http"
	"strings"
)

// ════════════════════════════════════════════════════════════
// GossipNet HTTP Handlers — /v1/broodnet/gossip/*
// ════════════════════════════════════════════════════════════

// HandleGossipStats handles GET /broodnet/gossip/stats
func HandleGossipStats(w http.ResponseWriter, r *http.Request) {
	gn := GetGossip()
	if gn == nil {
		http.Error(w, `{"error":"gossip net not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"stats":  gn.GossipStats(),
		"config": gn.GossipConfig(),
		"self":   json.RawMessage(gn.SelfInfo()),
	})
}

// HandleGossipAnnounce handles POST /broodnet/gossip/announce
func HandleGossipAnnounce(w http.ResponseWriter, r *http.Request) {
	gn := GetGossip()
	if gn == nil {
		http.Error(w, `{"error":"gossip net not initialized"}`, http.StatusServiceUnavailable)
		return
	}

	var peer PeerInfo
	if err := json.NewDecoder(r.Body).Decode(&peer); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}

	if err := gn.Announce(&peer); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
}

// HandleGossipExchange handles POST /broodnet/gossip/exchange
// Pull-based gossip: receive peer's view, merge, return our view
func HandleGossipExchange(w http.ResponseWriter, r *http.Request) {
	gn := GetGossip()
	if gn == nil {
		http.Error(w, `{"error":"gossip net not initialized"}`, http.StatusServiceUnavailable)
		return
	}

	var incoming GossipMessage
	if err := json.NewDecoder(r.Body).Decode(&incoming); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}

	merged := gn.MergeGossip(&incoming)

	// Return our own view
	outgoing := gn.PrepareGossip()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"merged":  merged,
		"message": outgoing,
	})
}

// HandleGossipPeers handles GET /broodnet/gossip/peers?state=alive
func HandleGossipPeers(w http.ResponseWriter, r *http.Request) {
	gn := GetGossip()
	if gn == nil {
		http.Error(w, `{"error":"gossip net not initialized"}`, http.StatusServiceUnavailable)
		return
	}

	state := NodeState(r.URL.Query().Get("state"))
	peers := gn.ListPeers(state)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"peers": peers,
		"count": len(peers),
	})
}

// HandleGossipPeer handles GET /broodnet/gossip/peers/:id
func HandleGossipPeer(w http.ResponseWriter, r *http.Request) {
	gn := GetGossip()
	if gn == nil {
		http.Error(w, `{"error":"gossip net not initialized"}`, http.StatusServiceUnavailable)
		return
	}

	parts := strings.Split(r.URL.Path, "/")
	nodeID := parts[len(parts)-1]
	peer := gn.GetPeer(nodeID)
	if peer == nil {
		http.Error(w, `{"error":"peer not found"}`, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(peer)
}

// HandleGossipTopology handles GET /broodnet/gossip/topology
func HandleGossipTopology(w http.ResponseWriter, r *http.Request) {
	gn := GetGossip()
	if gn == nil {
		http.Error(w, `{"error":"gossip net not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(gn.Topology())
}

// HandleGossipDiscover handles GET /broodnet/gossip/discover?capability=...&category=...&tier=...
func HandleGossipDiscover(w http.ResponseWriter, r *http.Request) {
	gn := GetGossip()
	if gn == nil {
		http.Error(w, `{"error":"gossip net not initialized"}`, http.StatusServiceUnavailable)
		return
	}

	var results []*PeerInfo

	if cap := r.URL.Query().Get("capability"); cap != "" {
		results = gn.FindByCapability(cap)
	} else if cat := r.URL.Query().Get("category"); cat != "" {
		results = gn.FindByCategory(TaskCategory(cat))
	} else if tier := r.URL.Query().Get("tier"); tier != "" {
		results = gn.FindByTier(TrustTier(tier))
	} else {
		results = gn.AlivePeers()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"nodes": results,
		"count": len(results),
	})
}

// HandleGossipSweep handles POST /broodnet/gossip/sweep (manual health check)
func HandleGossipSweep(w http.ResponseWriter, r *http.Request) {
	gn := GetGossip()
	if gn == nil {
		http.Error(w, `{"error":"gossip net not initialized"}`, http.StatusServiceUnavailable)
		return
	}

	suspects, dead := gn.HealthSweep()
	pruned := gn.Prune()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"suspects": suspects,
		"dead":     dead,
		"pruned":   pruned,
	})
}

// HandleGossipMeta handles POST /broodnet/gossip/meta (update self metadata)
func HandleGossipMeta(w http.ResponseWriter, r *http.Request) {
	gn := GetGossip()
	if gn == nil {
		http.Error(w, `{"error":"gossip net not initialized"}`, http.StatusServiceUnavailable)
		return
	}

	var meta map[string]string
	if err := json.NewDecoder(r.Body).Decode(&meta); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}

	gn.UpdateSelfMeta(meta)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "self": json.RawMessage(gn.SelfInfo())})
}
