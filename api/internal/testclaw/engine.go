package testclaw

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// ════════════════════════════════════════════════════════════
// TestClaw v1 — 测试与验证引擎
//
// 职责:
//   1. 冒烟测试: 对服务 API 端点执行基本可达性测试
//   2. 回归测试: Agent prompt/tool/workflow 正确性验证
//   3. 部署验证: 部署后全链路健康检查
//   4. 性能基准: 延迟/吞吐量简单测量
//   5. 测试套件: 组织和管理测试用例
//   6. 报告: 测试结果汇总和趋势
// ════════════════════════════════════════════════════════════

// ── Types ──

type TestType string

const (
	TestSmoke      TestType = "smoke"
	TestRegression TestType = "regression"
	TestDeploy     TestType = "deploy_verify"
	TestBenchmark  TestType = "benchmark"
	TestIntegration TestType = "integration"
	TestCustom     TestType = "custom"
)

type TestResult string

const (
	ResultPending TestResult = "pending"
	ResultRunning TestResult = "running"
	ResultPassed  TestResult = "passed"
	ResultFailed  TestResult = "failed"
	ResultSkipped TestResult = "skipped"
	ResultError   TestResult = "error"
)

// ── Data Structures ──

type TestCase struct {
	ID          string     `json:"id"`
	SuiteID     string     `json:"suite_id,omitempty"`
	Type        TestType   `json:"type"`
	Name        string     `json:"name"`
	Description string     `json:"description,omitempty"`
	Target      string     `json:"target"`       // service name or endpoint
	Method      string     `json:"method"`       // GET, POST, etc.
	Endpoint    string     `json:"endpoint"`     // /health, /v1/xxx
	ExpectCode  int        `json:"expect_code"`  // expected HTTP status
	ExpectBody  string     `json:"expect_body,omitempty"` // substring to match
	Timeout     int        `json:"timeout_ms"`
	Result      TestResult `json:"result"`
	StatusCode  int        `json:"status_code,omitempty"`
	LatencyMs   float64    `json:"latency_ms,omitempty"`
	ResponseBody string   `json:"response_body,omitempty"`
	Error       string     `json:"error,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

type TestSuite struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Type        TestType   `json:"type"`
	Description string     `json:"description,omitempty"`
	Cases       []TestCase `json:"cases"`
	Passed      int        `json:"passed"`
	Failed      int        `json:"failed"`
	Skipped     int        `json:"skipped"`
	Errors      int        `json:"errors"`
	Total       int        `json:"total"`
	Duration    float64    `json:"duration_ms"`
	Status      TestResult `json:"status"` // overall suite status
	CreatedAt   time.Time  `json:"created_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

type BenchmarkResult struct {
	ID          string    `json:"id"`
	Target      string    `json:"target"`
	Endpoint    string    `json:"endpoint"`
	Requests    int       `json:"requests"`
	Concurrency int       `json:"concurrency"`
	AvgLatency  float64   `json:"avg_latency_ms"`
	P50Latency  float64   `json:"p50_latency_ms"`
	P95Latency  float64   `json:"p95_latency_ms"`
	P99Latency  float64   `json:"p99_latency_ms"`
	MaxLatency  float64   `json:"max_latency_ms"`
	ThroughputRPS float64 `json:"throughput_rps"`
	ErrorRate   float64   `json:"error_rate"`
	CreatedAt   time.Time `json:"created_at"`
}

type TestReport struct {
	ID           string       `json:"id"`
	SuiteID      string       `json:"suite_id"`
	SuiteName    string       `json:"suite_name"`
	Summary      string       `json:"summary"`
	Passed       int          `json:"passed"`
	Failed       int          `json:"failed"`
	FailedCases  []string     `json:"failed_cases,omitempty"`
	Duration     float64      `json:"duration_ms"`
	Verdict      TestResult   `json:"verdict"`
	CreatedAt    time.Time    `json:"created_at"`
}

// ── Engine ──

type EngineConfig struct {
	DefaultTimeout  int      `json:"default_timeout_ms"`
	MaxConcurrent   int      `json:"max_concurrent_tests"`
	RetainResults   int      `json:"retain_results"`
	RetainReports   int      `json:"retain_reports"`
	AutoVerifyDeploy bool    `json:"auto_verify_deploy"`
	Targets         []string `json:"targets"` // services to test
}

func DefaultEngineConfig() *EngineConfig {
	return &EngineConfig{
		DefaultTimeout:  5000,
		MaxConcurrent:   10,
		RetainResults:   500,
		RetainReports:   100,
		AutoVerifyDeploy: true,
		Targets: []string{
			"queen", "claw", "synapse", "billing", "overlord",
			"gateway", "hive", "pheromone", "nydus", "forge",
		},
	}
}

type Engine struct {
	mu         sync.RWMutex
	nodeID     string
	config     *EngineConfig
	suites     map[string]*TestSuite
	cases      []TestCase
	benchmarks []BenchmarkResult
	reports    []TestReport
	stats      EngineStats
	startAt    time.Time
	nextID     int
}

type EngineStats struct {
	SuitesRun       int       `json:"suites_run"`
	SuitesPassed    int       `json:"suites_passed"`
	SuitesFailed    int       `json:"suites_failed"`
	CasesRun        int       `json:"cases_run"`
	CasesPassed     int       `json:"cases_passed"`
	CasesFailed     int       `json:"cases_failed"`
	BenchmarksRun   int       `json:"benchmarks_run"`
	ReportsGenerated int      `json:"reports_generated"`
	AvgLatency      float64   `json:"avg_latency_ms"`
	Uptime          string    `json:"uptime"`
	LastRun         time.Time `json:"last_run,omitempty"`
}

var (
	globalEngine *Engine
	engineOnce   sync.Once
)

func InitEngine(nodeID string, cfg *EngineConfig) *Engine {
	if cfg == nil {
		cfg = DefaultEngineConfig()
	}
	engineOnce.Do(func() {
		globalEngine = &Engine{
			nodeID:     nodeID,
			config:     cfg,
			suites:     make(map[string]*TestSuite),
			cases:      make([]TestCase, 0),
			benchmarks: make([]BenchmarkResult, 0),
			reports:    make([]TestReport, 0),
			startAt:    time.Now(),
		}
		log.Printf("[testclaw] engine ready (targets=%d, timeout=%dms, concurrent=%d)",
			len(cfg.Targets), cfg.DefaultTimeout, cfg.MaxConcurrent)
	})
	return globalEngine
}

func GetEngine() *Engine {
	return globalEngine
}

func (e *Engine) genID(prefix string) string {
	e.nextID++
	return fmt.Sprintf("%s-%d-%d", prefix, time.Now().Unix(), e.nextID)
}

// ── Suite Management ──

func (e *Engine) CreateSuite(name string, testType TestType, description string) *TestSuite {
	e.mu.Lock()
	defer e.mu.Unlock()

	suite := &TestSuite{
		ID:          e.genID("suite"),
		Name:        name,
		Type:        testType,
		Description: description,
		Cases:       make([]TestCase, 0),
		Status:      ResultPending,
		CreatedAt:   time.Now(),
	}

	e.suites[suite.ID] = suite
	log.Printf("[testclaw] suite created: %s — %s (%s)", suite.ID, name, testType)
	return suite
}

func (e *Engine) AddCase(suiteID string, tc TestCase) (*TestCase, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	suite, ok := e.suites[suiteID]
	if !ok {
		return nil, fmt.Errorf("suite %s not found", suiteID)
	}

	tc.ID = e.genID("case")
	tc.SuiteID = suiteID
	tc.Result = ResultPending
	tc.CreatedAt = time.Now()
	if tc.Timeout <= 0 {
		tc.Timeout = e.config.DefaultTimeout
	}
	if tc.ExpectCode <= 0 {
		tc.ExpectCode = 200
	}
	if tc.Method == "" {
		tc.Method = "GET"
	}

	suite.Cases = append(suite.Cases, tc)
	suite.Total = len(suite.Cases)
	return &suite.Cases[len(suite.Cases)-1], nil
}

func (e *Engine) ListSuites() []*TestSuite {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*TestSuite, 0, len(e.suites))
	for _, s := range e.suites {
		result = append(result, s)
	}
	return result
}

func (e *Engine) GetSuite(suiteID string) (*TestSuite, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	suite, ok := e.suites[suiteID]
	if !ok {
		return nil, fmt.Errorf("suite %s not found", suiteID)
	}
	return suite, nil
}

// ── Test Execution (simulate — real HTTP testing would be async) ──

func (e *Engine) RecordCaseResult(suiteID, caseID string, result TestResult, statusCode int, latencyMs float64, respBody, errMsg string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	suite, ok := e.suites[suiteID]
	if !ok {
		return fmt.Errorf("suite %s not found", suiteID)
	}

	now := time.Now()
	for i := range suite.Cases {
		if suite.Cases[i].ID == caseID {
			suite.Cases[i].Result = result
			suite.Cases[i].StatusCode = statusCode
			suite.Cases[i].LatencyMs = latencyMs
			suite.Cases[i].ResponseBody = respBody
			suite.Cases[i].Error = errMsg
			suite.Cases[i].CompletedAt = &now

			e.stats.CasesRun++
			switch result {
			case ResultPassed:
				e.stats.CasesPassed++
				suite.Passed++
			case ResultFailed:
				e.stats.CasesFailed++
				suite.Failed++
			case ResultSkipped:
				suite.Skipped++
			case ResultError:
				suite.Errors++
			}

			// Accumulate latency for average
			total := float64(e.stats.CasesRun)
			e.stats.AvgLatency = (e.stats.AvgLatency*(total-1) + latencyMs) / total
			e.stats.LastRun = now
			return nil
		}
	}
	return fmt.Errorf("case %s not found in suite %s", caseID, suiteID)
}

func (e *Engine) CompleteSuite(suiteID string) (*TestReport, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	suite, ok := e.suites[suiteID]
	if !ok {
		return nil, fmt.Errorf("suite %s not found", suiteID)
	}

	now := time.Now()
	suite.CompletedAt = &now
	suite.Duration = now.Sub(suite.CreatedAt).Seconds() * 1000

	if suite.Failed > 0 || suite.Errors > 0 {
		suite.Status = ResultFailed
		e.stats.SuitesFailed++
	} else {
		suite.Status = ResultPassed
		e.stats.SuitesPassed++
	}
	e.stats.SuitesRun++

	// Collect failed case names
	var failedNames []string
	for _, c := range suite.Cases {
		if c.Result == ResultFailed || c.Result == ResultError {
			failedNames = append(failedNames, c.Name)
		}
	}

	verdict := ResultPassed
	if len(failedNames) > 0 {
		verdict = ResultFailed
	}

	report := TestReport{
		ID:          e.genID("report"),
		SuiteID:     suiteID,
		SuiteName:   suite.Name,
		Summary:     fmt.Sprintf("%d passed, %d failed, %d skipped out of %d", suite.Passed, suite.Failed, suite.Skipped, suite.Total),
		Passed:      suite.Passed,
		Failed:      suite.Failed,
		FailedCases: failedNames,
		Duration:    suite.Duration,
		Verdict:     verdict,
		CreatedAt:   now,
	}

	e.reports = append(e.reports, report)
	if len(e.reports) > e.config.RetainReports {
		e.reports = e.reports[1:]
	}
	e.stats.ReportsGenerated++

	log.Printf("[testclaw] suite completed: %s — %s (%s)", suiteID, suite.Name, verdict)
	return &report, nil
}

// ── Benchmark ──

func (e *Engine) RecordBenchmark(target, endpoint string, requests, concurrency int, avg, p50, p95, p99, max, rps, errorRate float64) *BenchmarkResult {
	e.mu.Lock()
	defer e.mu.Unlock()

	b := BenchmarkResult{
		ID:            e.genID("bench"),
		Target:        target,
		Endpoint:      endpoint,
		Requests:      requests,
		Concurrency:   concurrency,
		AvgLatency:    avg,
		P50Latency:    p50,
		P95Latency:    p95,
		P99Latency:    p99,
		MaxLatency:    max,
		ThroughputRPS: rps,
		ErrorRate:     errorRate,
		CreatedAt:     time.Now(),
	}

	e.benchmarks = append(e.benchmarks, b)
	if len(e.benchmarks) > 100 {
		e.benchmarks = e.benchmarks[1:]
	}
	e.stats.BenchmarksRun++
	log.Printf("[testclaw] benchmark: %s%s — avg=%.1fms p99=%.1fms rps=%.0f err=%.1f%%",
		target, endpoint, avg, p99, rps, errorRate*100)
	return &b
}

func (e *Engine) ListBenchmarks(limit int) []BenchmarkResult {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if limit <= 0 || limit > len(e.benchmarks) {
		limit = len(e.benchmarks)
	}
	if limit == 0 {
		return nil
	}
	start := len(e.benchmarks) - limit
	result := make([]BenchmarkResult, limit)
	copy(result, e.benchmarks[start:])
	return result
}

// ── Reports ──

func (e *Engine) ListReports(limit int) []TestReport {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if limit <= 0 || limit > len(e.reports) {
		limit = len(e.reports)
	}
	if limit == 0 {
		return nil
	}
	start := len(e.reports) - limit
	result := make([]TestReport, limit)
	copy(result, e.reports[start:])
	return result
}

// ── Stats ──

func (e *Engine) Stats() *EngineStats {
	e.mu.RLock()
	defer e.mu.RUnlock()
	s := e.stats
	s.Uptime = time.Since(e.startAt).Round(time.Second).String()
	return &s
}

func (e *Engine) Config() *EngineConfig {
	return e.config
}
