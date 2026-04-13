package cocoon

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// ════════════════════════════════════════════════════════════
// Cocoon v1 — Agent 打包与构建引擎
//
// 职责:
//   1. cocoon.yaml 解析与验证
//   2. Agent 构建 (pack): manifest + prompt + tools → 可分发包
//   3. 平台交叉编译: linux/amd64, darwin/arm64, windows/amd64
//   4. 版本管理: semver, 变更日志
//   5. 发布到 Nydus Registry / Forge Marketplace
//   6. 构建历史与状态追踪
// ════════════════════════════════════════════════════════════

// ── Types ──

type BuildStatus string

const (
	BuildPending  BuildStatus = "pending"
	BuildRunning  BuildStatus = "running"
	BuildSuccess  BuildStatus = "success"
	BuildFailed   BuildStatus = "failed"
	BuildCanceled BuildStatus = "canceled"
)

type BuildTarget string

const (
	TargetLinuxAMD64   BuildTarget = "linux/amd64"
	TargetLinuxARM64   BuildTarget = "linux/arm64"
	TargetDarwinAMD64  BuildTarget = "darwin/amd64"
	TargetDarwinARM64  BuildTarget = "darwin/arm64"
	TargetWindowsAMD64 BuildTarget = "windows/amd64"
)

type PublishTarget string

const (
	PublishNydus PublishTarget = "nydus"
	PublishForge PublishTarget = "forge"
	PublishLocal PublishTarget = "local"
)

type PackageType string

const (
	PkgAgent    PackageType = "agent"
	PkgSkill    PackageType = "skill"
	PkgWorkflow PackageType = "workflow"
	PkgPlugin   PackageType = "plugin"
	PkgBundle   PackageType = "bundle"
)

// ── Data Structures ──

type CocoonSpec struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Version     string      `json:"version"`
	Type        PackageType `json:"type"`
	Description string      `json:"description,omitempty"`
	Author      string      `json:"author,omitempty"`
	License     string      `json:"license,omitempty"`
	EntryPoint  string      `json:"entry_point"`
	Dependencies []string   `json:"dependencies,omitempty"`
	Tools       []string    `json:"tools,omitempty"`
	Platforms   []BuildTarget `json:"platforms,omitempty"`
	Valid       bool        `json:"valid"`
	Errors      []string    `json:"errors,omitempty"`
	ParsedAt    time.Time   `json:"parsed_at"`
}

type Build struct {
	ID         string      `json:"id"`
	SpecID     string      `json:"spec_id"`
	Name       string      `json:"name"`
	Version    string      `json:"version"`
	Type       PackageType `json:"type"`
	Target     BuildTarget `json:"target"`
	Status     BuildStatus `json:"status"`
	OutputPath string      `json:"output_path,omitempty"`
	OutputSize int64       `json:"output_size_bytes,omitempty"`
	Checksum   string      `json:"checksum,omitempty"` // sha256
	Logs       []string    `json:"logs,omitempty"`
	Error      string      `json:"error,omitempty"`
	CreatedAt  time.Time   `json:"created_at"`
	StartedAt  *time.Time  `json:"started_at,omitempty"`
	FinishedAt *time.Time  `json:"finished_at,omitempty"`
	Duration   float64     `json:"duration_sec,omitempty"`
}

type PublishRecord struct {
	ID        string        `json:"id"`
	BuildID   string        `json:"build_id"`
	Name      string        `json:"name"`
	Version   string        `json:"version"`
	Target    PublishTarget `json:"target"`
	URL       string        `json:"url,omitempty"`
	Status    string        `json:"status"` // published, failed, pending
	Error     string        `json:"error,omitempty"`
	CreatedAt time.Time     `json:"created_at"`
}

// ── Engine ──

type EngineConfig struct {
	OutputDir       string        `json:"output_dir"`
	DefaultTargets  []BuildTarget `json:"default_targets"`
	MaxConcurrent   int           `json:"max_concurrent_builds"`
	NydusRegistryURL string       `json:"nydus_registry_url"`
	ForgeAPIURL     string        `json:"forge_api_url"`
	SignBuilds      bool          `json:"sign_builds"`
}

func DefaultEngineConfig() *EngineConfig {
	return &EngineConfig{
		OutputDir:        ".cocoon/output",
		DefaultTargets:   []BuildTarget{TargetLinuxAMD64, TargetDarwinARM64, TargetWindowsAMD64},
		MaxConcurrent:    3,
		NydusRegistryURL: "https://nydus.starclaw.net.cn",
		ForgeAPIURL:      "http://127.0.0.1:8087",
		SignBuilds:       true,
	}
}

type Engine struct {
	mu       sync.RWMutex
	nodeID   string
	config   *EngineConfig
	specs    map[string]*CocoonSpec
	builds   []Build
	publishes []PublishRecord
	stats    EngineStats
	startAt  time.Time
	nextID   int
}

type EngineStats struct {
	SpecsParsed     int       `json:"specs_parsed"`
	SpecsValid      int       `json:"specs_valid"`
	SpecsInvalid    int       `json:"specs_invalid"`
	BuildsTotal     int       `json:"builds_total"`
	BuildsSuccess   int       `json:"builds_success"`
	BuildsFailed    int       `json:"builds_failed"`
	PublishesTotal  int       `json:"publishes_total"`
	PublishesSuccess int      `json:"publishes_success"`
	Uptime          string    `json:"uptime"`
	LastBuild       time.Time `json:"last_build,omitempty"`
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
			nodeID:    nodeID,
			config:    cfg,
			specs:     make(map[string]*CocoonSpec),
			builds:    make([]Build, 0),
			publishes: make([]PublishRecord, 0),
			startAt:   time.Now(),
		}
		log.Printf("[cocoon] build engine ready (targets=%v, concurrent=%d)", cfg.DefaultTargets, cfg.MaxConcurrent)
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

// ── Spec Parsing ──

func (e *Engine) ParseSpec(name, version string, pkgType PackageType, description, author, entryPoint string, deps, tools []string, platforms []BuildTarget) *CocoonSpec {
	e.mu.Lock()
	defer e.mu.Unlock()

	spec := &CocoonSpec{
		ID:           e.genID("spec"),
		Name:         name,
		Version:      version,
		Type:         pkgType,
		Description:  description,
		Author:       author,
		EntryPoint:   entryPoint,
		Dependencies: deps,
		Tools:        tools,
		Platforms:    platforms,
		ParsedAt:     time.Now(),
	}

	// Validate
	var errors []string
	if name == "" {
		errors = append(errors, "name is required")
	}
	if version == "" {
		errors = append(errors, "version is required")
	}
	if entryPoint == "" {
		errors = append(errors, "entry_point is required")
	}
	if pkgType == "" {
		errors = append(errors, "type is required")
	}

	spec.Errors = errors
	spec.Valid = len(errors) == 0
	e.specs[spec.ID] = spec
	e.stats.SpecsParsed++
	if spec.Valid {
		e.stats.SpecsValid++
	} else {
		e.stats.SpecsInvalid++
	}

	log.Printf("[cocoon] spec parsed: %s v%s (valid=%v)", name, version, spec.Valid)
	return spec
}

func (e *Engine) GetSpec(specID string) (*CocoonSpec, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	spec, ok := e.specs[specID]
	if !ok {
		return nil, fmt.Errorf("spec %s not found", specID)
	}
	return spec, nil
}

func (e *Engine) ListSpecs() []*CocoonSpec {
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make([]*CocoonSpec, 0, len(e.specs))
	for _, s := range e.specs {
		result = append(result, s)
	}
	return result
}

// ── Build ──

func (e *Engine) StartBuild(specID string, target BuildTarget) (*Build, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	spec, ok := e.specs[specID]
	if !ok {
		return nil, fmt.Errorf("spec %s not found", specID)
	}
	if !spec.Valid {
		return nil, fmt.Errorf("spec %s is invalid: %v", specID, spec.Errors)
	}

	// Concurrency limit
	running := 0
	for _, b := range e.builds {
		if b.Status == BuildRunning {
			running++
		}
	}
	if running >= e.config.MaxConcurrent {
		return nil, fmt.Errorf("max concurrent builds (%d) reached", e.config.MaxConcurrent)
	}

	now := time.Now()
	build := Build{
		ID:        e.genID("build"),
		SpecID:    specID,
		Name:      spec.Name,
		Version:   spec.Version,
		Type:      spec.Type,
		Target:    target,
		Status:    BuildRunning,
		CreatedAt: now,
		StartedAt: &now,
		Logs:      []string{fmt.Sprintf("[%s] build started for %s v%s → %s", now.Format("15:04:05"), spec.Name, spec.Version, target)},
	}

	e.builds = append(e.builds, build)
	if len(e.builds) > 200 {
		e.builds = e.builds[1:]
	}
	e.stats.BuildsTotal++
	e.stats.LastBuild = now

	log.Printf("[cocoon] build started: %s → %s/%s@%s", build.ID, spec.Name, spec.Version, target)
	return &build, nil
}

func (e *Engine) FinishBuild(buildID string, success bool, outputPath, checksum string, outputSize int64, errMsg string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	for i := range e.builds {
		if e.builds[i].ID == buildID {
			now := time.Now()
			e.builds[i].FinishedAt = &now
			if e.builds[i].StartedAt != nil {
				e.builds[i].Duration = now.Sub(*e.builds[i].StartedAt).Seconds()
			}
			if success {
				e.builds[i].Status = BuildSuccess
				e.builds[i].OutputPath = outputPath
				e.builds[i].Checksum = checksum
				e.builds[i].OutputSize = outputSize
				e.stats.BuildsSuccess++
			} else {
				e.builds[i].Status = BuildFailed
				e.builds[i].Error = errMsg
				e.stats.BuildsFailed++
			}
			e.builds[i].Logs = append(e.builds[i].Logs,
				fmt.Sprintf("[%s] build %s (%.1fs)", now.Format("15:04:05"), e.builds[i].Status, e.builds[i].Duration))
			return nil
		}
	}
	return fmt.Errorf("build %s not found", buildID)
}

func (e *Engine) ListBuilds(limit int) []Build {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if limit <= 0 || limit > len(e.builds) {
		limit = len(e.builds)
	}
	if limit == 0 {
		return nil
	}
	start := len(e.builds) - limit
	result := make([]Build, limit)
	copy(result, e.builds[start:])
	return result
}

// ── Publish ──

func (e *Engine) Publish(buildID string, target PublishTarget) (*PublishRecord, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	var build *Build
	for i := range e.builds {
		if e.builds[i].ID == buildID {
			build = &e.builds[i]
			break
		}
	}
	if build == nil {
		return nil, fmt.Errorf("build %s not found", buildID)
	}
	if build.Status != BuildSuccess {
		return nil, fmt.Errorf("build %s not successful (status=%s)", buildID, build.Status)
	}

	pub := PublishRecord{
		ID:        e.genID("pub"),
		BuildID:   buildID,
		Name:      build.Name,
		Version:   build.Version,
		Target:    target,
		Status:    "published",
		CreatedAt: time.Now(),
	}

	switch target {
	case PublishNydus:
		pub.URL = fmt.Sprintf("%s/packages/%s/%s", e.config.NydusRegistryURL, build.Name, build.Version)
	case PublishForge:
		pub.URL = fmt.Sprintf("%s/v1/packages/%s/%s", e.config.ForgeAPIURL, build.Name, build.Version)
	case PublishLocal:
		pub.URL = build.OutputPath
	}

	e.publishes = append(e.publishes, pub)
	if len(e.publishes) > 200 {
		e.publishes = e.publishes[1:]
	}
	e.stats.PublishesTotal++
	e.stats.PublishesSuccess++

	log.Printf("[cocoon] published: %s v%s → %s (%s)", build.Name, build.Version, target, pub.URL)
	return &pub, nil
}

func (e *Engine) ListPublishes(limit int) []PublishRecord {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if limit <= 0 || limit > len(e.publishes) {
		limit = len(e.publishes)
	}
	if limit == 0 {
		return nil
	}
	start := len(e.publishes) - limit
	result := make([]PublishRecord, limit)
	copy(result, e.publishes[start:])
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
