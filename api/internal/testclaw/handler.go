package testclaw

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// ════════════════════════════════════════════════════════════
// TestClaw v1 HTTP Handlers — /v1/testclaw/*
// ════════════════════════════════════════════════════════════

// HandleStats handles GET /testclaw/stats
func HandleStats(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"testclaw engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"stats":  e.Stats(),
		"config": e.Config(),
	})
}

// HandleCreateSuite handles POST /testclaw/suites
func HandleCreateSuite(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"testclaw engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		Name        string   `json:"name"`
		Type        TestType `json:"type"`
		Description string   `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		http.Error(w, `{"error":"name required"}`, http.StatusBadRequest)
		return
	}
	if req.Type == "" {
		req.Type = TestSmoke
	}
	suite := e.CreateSuite(req.Name, req.Type, req.Description)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(suite)
}

// HandleListSuites handles GET /testclaw/suites
func HandleListSuites(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"testclaw engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	suites := e.ListSuites()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"suites": suites,
		"count":  len(suites),
	})
}

// HandleGetSuite handles GET /testclaw/suites/detail?id=xxx
func HandleGetSuite(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"testclaw engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	suiteID := r.URL.Query().Get("id")
	if suiteID == "" {
		http.Error(w, `{"error":"id required"}`, http.StatusBadRequest)
		return
	}
	suite, err := e.GetSuite(suiteID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(suite)
}

// HandleAddCase handles POST /testclaw/cases
func HandleAddCase(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"testclaw engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		SuiteID    string   `json:"suite_id"`
		Type       TestType `json:"type"`
		Name       string   `json:"name"`
		Target     string   `json:"target"`
		Method     string   `json:"method"`
		Endpoint   string   `json:"endpoint"`
		ExpectCode int      `json:"expect_code"`
		ExpectBody string   `json:"expect_body"`
		Timeout    int      `json:"timeout_ms"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	tc := TestCase{
		Type:       req.Type,
		Name:       req.Name,
		Target:     req.Target,
		Method:     req.Method,
		Endpoint:   req.Endpoint,
		ExpectCode: req.ExpectCode,
		ExpectBody: req.ExpectBody,
		Timeout:    req.Timeout,
	}
	result, err := e.AddCase(req.SuiteID, tc)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(result)
}

// HandleRecordResult handles POST /testclaw/results
func HandleRecordResult(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"testclaw engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		SuiteID      string     `json:"suite_id"`
		CaseID       string     `json:"case_id"`
		Result       TestResult `json:"result"`
		StatusCode   int        `json:"status_code"`
		LatencyMs    float64    `json:"latency_ms"`
		ResponseBody string     `json:"response_body"`
		Error        string     `json:"error"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if err := e.RecordCaseResult(req.SuiteID, req.CaseID, req.Result, req.StatusCode, req.LatencyMs, req.ResponseBody, req.Error); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
}

// HandleCompleteSuite handles POST /testclaw/suites/complete
func HandleCompleteSuite(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"testclaw engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		SuiteID string `json:"suite_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	report, err := e.CompleteSuite(req.SuiteID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(report)
}

// HandleRecordBenchmark handles POST /testclaw/benchmarks
func HandleRecordBenchmark(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"testclaw engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		Target      string  `json:"target"`
		Endpoint    string  `json:"endpoint"`
		Requests    int     `json:"requests"`
		Concurrency int     `json:"concurrency"`
		AvgLatency  float64 `json:"avg_latency_ms"`
		P50Latency  float64 `json:"p50_latency_ms"`
		P95Latency  float64 `json:"p95_latency_ms"`
		P99Latency  float64 `json:"p99_latency_ms"`
		MaxLatency  float64 `json:"max_latency_ms"`
		RPS         float64 `json:"throughput_rps"`
		ErrorRate   float64 `json:"error_rate"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	b := e.RecordBenchmark(req.Target, req.Endpoint, req.Requests, req.Concurrency,
		req.AvgLatency, req.P50Latency, req.P95Latency, req.P99Latency, req.MaxLatency,
		req.RPS, req.ErrorRate)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(b)
}

// HandleListBenchmarks handles GET /testclaw/benchmarks?limit=10
func HandleListBenchmarks(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"testclaw engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	limit := 10
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	benchmarks := e.ListBenchmarks(limit)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"benchmarks": benchmarks,
		"count":      len(benchmarks),
	})
}

// HandleListReports handles GET /testclaw/reports?limit=10
func HandleListReports(w http.ResponseWriter, r *http.Request) {
	e := GetEngine()
	if e == nil {
		http.Error(w, `{"error":"testclaw engine not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	limit := 10
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	reports := e.ListReports(limit)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"reports": reports,
		"count":   len(reports),
	})
}
