package sense

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

// ════════════════════════════════════════════════════════════
// SenseClaw v1 HTTP Handlers — /v1/sense/*
// ════════════════════════════════════════════════════════════

// HandleStats handles GET /sense/stats
func HandleStats(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"sense engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"stats":          e.Stats(),
		"config":         e.Config(),
		"health_summary": e.HealthSummary(),
	})
}

// HandleSubmitFeedback handles POST /sense/feedback
func HandleSubmitFeedback(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"sense engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		Type        FeedbackType     `json:"type"`
		Priority    FeedbackPriority `json:"priority"`
		Title       string           `json:"title"`
		Description string           `json:"description"`
		Source      string           `json:"source"`
		Tags        []string         `json:"tags"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if req.Title == "" {
		http.Error(w, `{"error":"title required"}`, http.StatusBadRequest)
		return
	}
	if req.Type == "" {
		req.Type = FeedbackOther
	}
	if req.Priority == "" {
		req.Priority = FBPriMedium
	}
	fb := e.SubmitFeedback(req.Type, req.Priority, req.Title, req.Description, req.Source, req.Tags)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(fb)
}

// HandleListFeedbacks handles GET /sense/feedback?status=new&limit=20
func HandleListFeedbacks(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"sense engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	status := r.URL.Query().Get("status")
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	fbs := e.ListFeedbacks(status, limit)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"feedbacks": fbs,
		"count":     len(fbs),
	})
}

// HandleResolveFeedback handles POST /sense/feedback/resolve
func HandleResolveFeedback(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"sense engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		ID         string `json:"id"`
		Resolution string `json:"resolution"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if err := e.ResolveFeedback(req.ID, req.Resolution); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "id": req.ID})
}

// HandleFireAlert handles POST /sense/alert
func HandleFireAlert(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"sense engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		Name      string            `json:"name"`
		Severity  AlertSeverity     `json:"severity"`
		Service   string            `json:"service"`
		Message   string            `json:"message"`
		Value     float64           `json:"value"`
		Threshold float64           `json:"threshold"`
		Labels    map[string]string `json:"labels"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if req.Severity == "" {
		req.Severity = AlertWarning
	}
	alert := e.FireAlert(req.Name, req.Severity, req.Service, req.Message, req.Value, req.Threshold, req.Labels)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(alert)
}

// HandleListAlerts handles GET /sense/alerts?status=firing
func HandleListAlerts(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"sense engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	status := r.URL.Query().Get("status")
	alerts := e.ListAlerts(status)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"alerts": alerts,
		"count":  len(alerts),
	})
}

// HandleResolveAlert handles POST /sense/alert/resolve
func HandleResolveAlert(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"sense engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if err := e.ResolveAlert(req.ID); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "id": req.ID})
}

// HandleSilenceAlert handles POST /sense/alert/silence
func HandleSilenceAlert(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"sense engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		ID       string `json:"id"`
		Duration int    `json:"duration_min"` // minutes
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if req.Duration <= 0 {
		req.Duration = 60
	}
	if err := e.SilenceAlert(req.ID, time.Duration(req.Duration)*time.Minute); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "id": req.ID, "silenced_min": req.Duration})
}

// HandleHealth handles GET /sense/health
func HandleHealth(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"sense engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"services": e.GetHealth(),
		"summary":  e.HealthSummary(),
	})
}

// HandleUpdateHealth handles POST /sense/health
func HandleUpdateHealth(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"sense engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		Service   string        `json:"service"`
		Status    ServiceStatus `json:"status"`
		LatencyMs float64       `json:"latency_ms"`
		Message   string        `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	e.UpdateHealth(req.Service, req.Status, req.LatencyMs, req.Message)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "service": req.Service})
}

// HandleReportAnomaly handles POST /sense/anomaly
func HandleReportAnomaly(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"sense engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		Type        string  `json:"type"`
		Service     string  `json:"service"`
		Description string  `json:"description"`
		Metric      string  `json:"metric"`
		Current     float64 `json:"current_value"`
		Baseline    float64 `json:"baseline_value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	a := e.ReportAnomaly(req.Type, req.Service, req.Description, req.Metric, req.Current, req.Baseline)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(a)
}

// HandleListAnomalies handles GET /sense/anomalies?limit=20
func HandleListAnomalies(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"sense engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	anomalies := e.ListAnomalies(limit)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"anomalies": anomalies,
		"count":     len(anomalies),
	})
}

// HandleGenerateInsight handles POST /sense/insight
func HandleGenerateInsight(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"sense engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		Category string   `json:"category"`
		Title    string   `json:"title"`
		Summary  string   `json:"summary"`
		Impact   string   `json:"impact"`
		Sources  []string `json:"sources"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	insight := e.GenerateInsight(req.Category, req.Title, req.Summary, req.Impact, req.Sources)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(insight)
}

// HandleListInsights handles GET /sense/insights?limit=20
func HandleListInsights(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"sense engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	insights := e.ListInsights(limit)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"insights": insights,
		"count":    len(insights),
	})
}
