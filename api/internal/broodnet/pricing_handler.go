package broodnet

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// ════════════════════════════════════════════════════════════
// PricingEngine HTTP Handlers — /v1/broodnet/pricing/*
// ════════════════════════════════════════════════════════════

// HandlePricingStats handles GET /broodnet/pricing/stats
func HandlePricingStats(w http.ResponseWriter, r *http.Request) {
	pe := GetPricing()
	if pe == nil {
		http.Error(w, `{"error":"pricing engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"stats":  pe.Stats(),
		"config": pe.PriceConfig(),
	})
}

// HandlePricingQuote handles POST /broodnet/pricing/quote
func HandlePricingQuote(w http.ResponseWriter, r *http.Request) {
	pe := GetPricing()
	if pe == nil {
		http.Error(w, `{"error":"pricing engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}

	var req struct {
		Category       TaskCategory     `json:"category"`
		Requirements   TaskRequirements `json:"requirements"`
		UrgencyMinutes int64            `json:"urgency_minutes"`
		RepScore       float64          `json:"rep_score"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if req.Category == "" {
		req.Category = CatCompute
	}

	quote := pe.Quote(req.Category, req.Requirements, req.UrgencyMinutes, req.RepScore)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(quote)
}

// HandlePricingValidate handles GET /broodnet/pricing/validate?category=...&price=...
func HandlePricingValidate(w http.ResponseWriter, r *http.Request) {
	pe := GetPricing()
	if pe == nil {
		http.Error(w, `{"error":"pricing engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}

	category := TaskCategory(r.URL.Query().Get("category"))
	priceStr := r.URL.Query().Get("price")
	price, err := strconv.ParseInt(priceStr, 10, 64)
	if err != nil {
		http.Error(w, `{"error":"price must be integer"}`, http.StatusBadRequest)
		return
	}

	valid, reason := pe.ValidateBid(category, price)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"valid":  valid,
		"reason": reason,
	})
}

// HandlePricingSettle handles POST /broodnet/pricing/settle
func HandlePricingSettle(w http.ResponseWriter, r *http.Request) {
	pe := GetPricing()
	if pe == nil {
		http.Error(w, `{"error":"pricing engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}

	var req struct {
		Category TaskCategory `json:"category"`
		Price    int64        `json:"price"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}

	pe.RecordSettlement(req.Category, req.Price)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
}
