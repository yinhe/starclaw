package partner

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// ════════════════════════════════════════════════════════════
// Partner v1 HTTP Handlers — /v1/partner/*
// ════════════════════════════════════════════════════════════

// HandleStats handles GET /partner/stats
func HandleStats(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"partner engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"stats":  e.Stats(),
		"config": e.Config(),
	})
}

// HandleRegisterPartner handles POST /partner/register
func HandleRegisterPartner(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"partner engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		Name     string       `json:"name"`
		Level    PartnerLevel `json:"level"`
		Email    string       `json:"email"`
		Phone    string       `json:"phone"`
		ParentID string       `json:"parent_id"`
		CityCode string       `json:"city_code"`
		Equity   float64      `json:"equity"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		http.Error(w, `{"error":"name required"}`, http.StatusBadRequest)
		return
	}
	if req.Level == "" {
		req.Level = LevelCity
	}
	p := e.RegisterPartner(req.Name, req.Level, req.Email, req.Phone, req.ParentID, req.CityCode, req.Equity)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(p)
}

// HandleListPartners handles GET /partner/list?level=city&status=active
func HandleListPartners(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"partner engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	level := r.URL.Query().Get("level")
	status := r.URL.Query().Get("status")
	partners := e.ListPartners(level, status)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"partners": partners,
		"count":    len(partners),
	})
}

// HandleGetPartner handles GET /partner/detail?id=xxx
func HandleGetPartner(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"partner engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, `{"error":"id required"}`, http.StatusBadRequest)
		return
	}
	p, err := e.GetPartner(id)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(p)
}

// HandleApprovePartner handles POST /partner/approve
func HandleApprovePartner(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"partner engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		PartnerID string `json:"partner_id"`
		Approver  string `json:"approver"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if req.Approver == "" {
		req.Approver = "admin"
	}
	if err := e.ApprovePartner(req.PartnerID, req.Approver); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "partner_id": req.PartnerID})
}

// HandleSuspendPartner handles POST /partner/suspend
func HandleSuspendPartner(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"partner engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		PartnerID string `json:"partner_id"`
		Reason    string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if err := e.SuspendPartner(req.PartnerID, req.Reason); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
}

// HandleRecordCommission handles POST /partner/commission
func HandleRecordCommission(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"partner engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		PartnerID  string  `json:"partner_id"`
		CustomerID string  `json:"customer_id"`
		OrderID    string  `json:"order_id"`
		Revenue    float64 `json:"revenue"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	rec, err := e.RecordCommission(req.PartnerID, req.CustomerID, req.OrderID, req.Revenue)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(rec)
}

// HandleListCommissions handles GET /partner/commissions?partner_id=xxx&limit=20
func HandleListCommissions(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"partner engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	partnerID := r.URL.Query().Get("partner_id")
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	comms := e.ListCommissions(partnerID, limit)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"commissions": comms,
		"count":       len(comms),
	})
}

// HandleCreateSettlement handles POST /partner/settlement
func HandleCreateSettlement(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"partner engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		PartnerID string `json:"partner_id"`
		Period    string `json:"period"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	s, err := e.CreateSettlement(req.PartnerID, req.Period)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(s)
}

// HandleListSettlements handles GET /partner/settlements?partner_id=xxx
func HandleListSettlements(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"partner engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	partnerID := r.URL.Query().Get("partner_id")
	settlements := e.ListSettlements(partnerID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"settlements": settlements,
		"count":       len(settlements),
	})
}

// HandleAudit handles GET /partner/audit?limit=20
func HandleAudit(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"partner engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	entries := e.ListAudit(limit)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"audit": entries,
		"count": len(entries),
	})
}
