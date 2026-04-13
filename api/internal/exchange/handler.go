package exchange

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// ════════════════════════════════════════════════════════════
// Exchange HTTP Handlers — /v1/exchange/*
// ════════════════════════════════════════════════════════════

// HandleStats handles GET /exchange/stats
func HandleStats(w http.ResponseWriter, r *http.Request) {
	eng := GetEngine()
	if eng == nil {
		http.Error(w, `{"error":"exchange engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(eng.Stats())
}

// HandlePlaceOrder handles POST /exchange/order
func HandlePlaceOrder(w http.ResponseWriter, r *http.Request) {
	eng := GetEngine()
	if eng == nil {
		http.Error(w, `{"error":"exchange engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		NodeID   string  `json:"node_id"`
		Side     string  `json:"side"`
		Type     string  `json:"type"`
		Price    float64 `json:"price"`
		Quantity float64 `json:"quantity"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if req.NodeID == "" {
		req.NodeID = "local"
	}
	if req.Type == "" {
		req.Type = "limit"
	}
	order, err := eng.PlaceOrder(req.NodeID, OrderSide(req.Side), OrderType(req.Type), req.Price, req.Quantity)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(order)
}

// HandleCancelOrder handles POST /exchange/order/cancel
func HandleCancelOrder(w http.ResponseWriter, r *http.Request) {
	eng := GetEngine()
	if eng == nil {
		http.Error(w, `{"error":"exchange engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		OrderID string `json:"order_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if err := eng.CancelOrder(req.OrderID); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "cancelled"})
}

// HandleOrderBook handles GET /exchange/orderbook?depth=10
func HandleOrderBook(w http.ResponseWriter, r *http.Request) {
	eng := GetEngine()
	if eng == nil {
		http.Error(w, `{"error":"exchange engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	depth := 10
	if d := r.URL.Query().Get("depth"); d != "" {
		if n, err := strconv.Atoi(d); err == nil && n > 0 {
			depth = n
		}
	}
	book := eng.GetOrderBook(depth)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(book)
}

// HandleTrades handles GET /exchange/trades?limit=20
func HandleTrades(w http.ResponseWriter, r *http.Request) {
	eng := GetEngine()
	if eng == nil {
		http.Error(w, `{"error":"exchange engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	trades := eng.RecentTrades(limit)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"trades": trades,
		"count":  len(trades),
	})
}

// ── Marketplace Handlers ──

// HandleListService handles POST /exchange/service
func HandleListService(w http.ResponseWriter, r *http.Request) {
	eng := GetEngine()
	if eng == nil {
		http.Error(w, `{"error":"exchange engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		AgentID     string   `json:"agent_id"`
		NodeID      string   `json:"node_id"`
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Category    string   `json:"category"`
		BasePrice   float64  `json:"base_price"`
		Tags        []string `json:"tags"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if req.Name == "" || req.AgentID == "" {
		http.Error(w, `{"error":"name and agent_id required"}`, http.StatusBadRequest)
		return
	}
	svc := eng.ListService(req.AgentID, req.NodeID, req.Name, req.Description, req.Category, req.BasePrice, req.Tags)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(svc)
}

// HandleGetServices handles GET /exchange/services?category=&limit=20
func HandleGetServices(w http.ResponseWriter, r *http.Request) {
	eng := GetEngine()
	if eng == nil {
		http.Error(w, `{"error":"exchange engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	category := r.URL.Query().Get("category")
	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	services := eng.GetServices(category, limit)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"services": services,
		"count":    len(services),
	})
}

// HandleCreateRequest handles POST /exchange/request
func HandleCreateRequest(w http.ResponseWriter, r *http.Request) {
	eng := GetEngine()
	if eng == nil {
		http.Error(w, `{"error":"exchange engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		RequesterID string                 `json:"requester_id"`
		Title       string                 `json:"title"`
		Description string                 `json:"description"`
		Category    string                 `json:"category"`
		Budget      float64                `json:"budget"`
		Params      map[string]interface{} `json:"params"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if req.Title == "" {
		http.Error(w, `{"error":"title required"}`, http.StatusBadRequest)
		return
	}
	svcReq := eng.CreateRequest(req.RequesterID, req.Title, req.Description, req.Category, req.Budget, req.Params)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(svcReq)
}

// HandleListRequests handles GET /exchange/requests?status=&limit=20
func HandleListRequests(w http.ResponseWriter, r *http.Request) {
	eng := GetEngine()
	if eng == nil {
		http.Error(w, `{"error":"exchange engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	status := r.URL.Query().Get("status")
	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	requests := eng.ListRequests(status, limit)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"requests": requests,
		"count":    len(requests),
	})
}

// HandlePlaceBid handles POST /exchange/bid
func HandlePlaceBid(w http.ResponseWriter, r *http.Request) {
	eng := GetEngine()
	if eng == nil {
		http.Error(w, `{"error":"exchange engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		RequestID string  `json:"request_id"`
		AgentID   string  `json:"agent_id"`
		ServiceID string  `json:"service_id"`
		Price     float64 `json:"price"`
		ETA       string  `json:"eta"`
		Message   string  `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	bid, err := eng.PlaceBid(req.RequestID, req.AgentID, req.ServiceID, req.Price, req.ETA, req.Message)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(bid)
}

// HandleAcceptBid handles POST /exchange/bid/accept
func HandleAcceptBid(w http.ResponseWriter, r *http.Request) {
	eng := GetEngine()
	if eng == nil {
		http.Error(w, `{"error":"exchange engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		RequestID string `json:"request_id"`
		BidID     string `json:"bid_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if err := eng.AcceptBid(req.RequestID, req.BidID); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "accepted"})
}

// HandleCompleteRequest handles POST /exchange/request/complete
func HandleCompleteRequest(w http.ResponseWriter, r *http.Request) {
	eng := GetEngine()
	if eng == nil {
		http.Error(w, `{"error":"exchange engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		RequestID string `json:"request_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if err := eng.CompleteRequest(req.RequestID); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "completed"})
}

// HandleRateService handles POST /exchange/rate
func HandleRateService(w http.ResponseWriter, r *http.Request) {
	eng := GetEngine()
	if eng == nil {
		http.Error(w, `{"error":"exchange engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		ServiceID string  `json:"service_id"`
		RequestID string  `json:"request_id"`
		RaterID   string  `json:"rater_id"`
		Score     float64 `json:"score"`
		Comment   string  `json:"comment"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	rating, err := eng.RateService(req.ServiceID, req.RequestID, req.RaterID, req.Score, req.Comment)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(rating)
}
