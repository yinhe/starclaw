package broodnet

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
)

// ════════════════════════════════════════════════════════════
// TaskMarket HTTP Handlers — exposed via Claw /v1/broodnet/*
// ════════════════════════════════════════════════════════════

var (
	globalMarket *TaskMarket
	marketOnce   sync.Once
)

// InitMarket creates the global TaskMarket instance
func InitMarket(nodeID string, cfg *MarketConfig) *TaskMarket {
	marketOnce.Do(func() {
		globalMarket = NewTaskMarket(nodeID, cfg)
	})
	return globalMarket
}

// GetMarket returns the global market
func GetMarket() *TaskMarket {
	return globalMarket
}

// HandleMarketStats handles GET /broodnet/stats
func HandleMarketStats(w http.ResponseWriter, r *http.Request) {
	m := GetMarket()
	if m == nil {
		http.Error(w, `{"error":"market not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"stats":  m.Stats(),
		"config": m.MarketConfig(),
	})
}

// HandleMarketPost handles POST /broodnet/tasks
func HandleMarketPost(w http.ResponseWriter, r *http.Request) {
	m := GetMarket()
	if m == nil {
		http.Error(w, `{"error":"market not initialized"}`, http.StatusServiceUnavailable)
		return
	}

	var req struct {
		PostedBy     string           `json:"posted_by"`
		Category     TaskCategory     `json:"category"`
		Title        string           `json:"title"`
		Description  string           `json:"description"`
		Payload      json.RawMessage  `json:"payload"`
		Requirements TaskRequirements `json:"requirements"`
		Budget       int64            `json:"budget"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if req.PostedBy == "" {
		req.PostedBy = m.nodeID
	}
	if req.Category == "" {
		req.Category = CatCompute
	}

	task, err := m.PostTask(req.PostedBy, req.Category, req.Title, req.Description,
		req.Payload, req.Requirements, req.Budget)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(task)
}

// HandleMarketList handles GET /broodnet/tasks?status=...&category=...
func HandleMarketList(w http.ResponseWriter, r *http.Request) {
	m := GetMarket()
	if m == nil {
		http.Error(w, `{"error":"market not initialized"}`, http.StatusServiceUnavailable)
		return
	}

	status := MarketTaskStatus(r.URL.Query().Get("status"))
	category := TaskCategory(r.URL.Query().Get("category"))
	tasks := m.ListTasks(status, category, 50)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"tasks": tasks, "count": len(tasks)})
}

// HandleMarketOpen handles GET /broodnet/tasks/open?category=...
func HandleMarketOpen(w http.ResponseWriter, r *http.Request) {
	m := GetMarket()
	if m == nil {
		http.Error(w, `{"error":"market not initialized"}`, http.StatusServiceUnavailable)
		return
	}

	category := TaskCategory(r.URL.Query().Get("category"))
	tasks := m.OpenTasks(category, 50)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"tasks": tasks, "count": len(tasks)})
}

// HandleMarketGet handles GET /broodnet/tasks/:id
func HandleMarketGet(w http.ResponseWriter, r *http.Request) {
	m := GetMarket()
	if m == nil {
		http.Error(w, `{"error":"market not initialized"}`, http.StatusServiceUnavailable)
		return
	}

	parts := strings.Split(r.URL.Path, "/")
	id := parts[len(parts)-1]
	task := m.GetTask(id)
	if task == nil {
		http.Error(w, `{"error":"task not found"}`, http.StatusNotFound)
		return
	}

	bids := m.GetBids(id)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"task": task,
		"bids": bids,
	})
}

// HandleMarketBid handles POST /broodnet/tasks/bid
func HandleMarketBid(w http.ResponseWriter, r *http.Request) {
	m := GetMarket()
	if m == nil {
		http.Error(w, `{"error":"market not initialized"}`, http.StatusServiceUnavailable)
		return
	}

	var req struct {
		TaskID   string `json:"task_id"`
		BidderID string `json:"bidder_id"`
		Price    int64  `json:"price"`
		ETA      int64  `json:"eta_seconds"`
		Reason   string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if req.BidderID == "" {
		req.BidderID = m.nodeID
	}

	bid, err := m.PlaceBid(req.TaskID, req.BidderID, req.Price, req.ETA, req.Reason)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(bid)
}

// HandleMarketMatch handles POST /broodnet/tasks/match
func HandleMarketMatch(w http.ResponseWriter, r *http.Request) {
	m := GetMarket()
	if m == nil {
		http.Error(w, `{"error":"market not initialized"}`, http.StatusServiceUnavailable)
		return
	}

	var req struct {
		TaskID string `json:"task_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}

	winner, err := m.MatchBest(req.TaskID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	task := m.GetTask(req.TaskID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"winner": winner,
		"task":   task,
	})
}

// HandleMarketComplete handles POST /broodnet/tasks/complete
func HandleMarketComplete(w http.ResponseWriter, r *http.Request) {
	m := GetMarket()
	if m == nil {
		http.Error(w, `{"error":"market not initialized"}`, http.StatusServiceUnavailable)
		return
	}

	var req struct {
		TaskID     string `json:"task_id"`
		ExecutorID string `json:"executor_id"`
		Result     string `json:"result"`
		Error      string `json:"error"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}

	var opErr error
	if req.Error != "" {
		opErr = m.FailTask(req.TaskID, req.ExecutorID, req.Error)
	} else {
		opErr = m.CompleteTask(req.TaskID, req.ExecutorID, req.Result)
	}
	if opErr != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": opErr.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
}

// HandleMarketSettle handles POST /broodnet/tasks/settle
func HandleMarketSettle(w http.ResponseWriter, r *http.Request) {
	m := GetMarket()
	if m == nil {
		http.Error(w, `{"error":"market not initialized"}`, http.StatusServiceUnavailable)
		return
	}

	var req struct {
		TaskID string `json:"task_id"`
		TxnID  string `json:"txn_id"`
		Amount int64  `json:"amount"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}

	if err := m.SettleTask(req.TaskID, req.TxnID, req.Amount); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
}

// HandleMarketCancel handles POST /broodnet/tasks/cancel
func HandleMarketCancel(w http.ResponseWriter, r *http.Request) {
	m := GetMarket()
	if m == nil {
		http.Error(w, `{"error":"market not initialized"}`, http.StatusServiceUnavailable)
		return
	}

	var req struct {
		TaskID      string `json:"task_id"`
		RequesterID string `json:"requester_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}

	if err := m.CancelTask(req.TaskID, req.RequesterID); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
}
