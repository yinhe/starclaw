package agent

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

// ════════════════════════════════════════════════════════════
//  Bridge Manager — manages agent bridge subprocesses
//  (Python/Node/Go) declared in manifest.yaml bridge: section.
// ════════════════════════════════════════════════════════════

// GlobalBridgeManager is the singleton bridge manager used by the router.
var GlobalBridgeManager = NewBridgeManager()

// BridgeState represents the lifecycle state of a bridge subprocess.
type BridgeState string

const (
	BridgeStopped  BridgeState = "stopped"
	BridgeStarting BridgeState = "starting"
	BridgeRunning  BridgeState = "running"
	BridgeError    BridgeState = "error"

	bridgeHealthTimeout  = 3 * time.Second
	bridgeStartupTimeout = 30 * time.Second
	bridgeHealthInterval = 30 * time.Second
)

// BridgeInstance tracks a running bridge subprocess.
type BridgeInstance struct {
	AgentID      string            `json:"agent_id"`
	Manifest     *ManifestBridge   `json:"-"`
	Dir          string            `json:"-"` // agent directory (cwd for subprocess)
	State        BridgeState       `json:"state"`
	Port         int               `json:"port"`
	PID          int               `json:"pid"`
	URL          string            `json:"url"`
	DashboardURL string            `json:"dashboard_url,omitempty"`
	Error        string            `json:"error,omitempty"`
	StartedAt    *time.Time        `json:"started_at,omitempty"`
	Env          map[string]string `json:"-"`

	cmd    *exec.Cmd
	cancel context.CancelFunc
}

// BridgeManager manages all agent bridge subprocesses.
type BridgeManager struct {
	mu        sync.RWMutex
	instances map[string]*BridgeInstance // agentID → instance
	usedPorts map[int]string             // port → agentID
	stopCh    chan struct{}
}

// NewBridgeManager creates a new bridge manager.
func NewBridgeManager() *BridgeManager {
	return &BridgeManager{
		instances: make(map[string]*BridgeInstance),
		usedPorts: make(map[int]string),
		stopCh:    make(chan struct{}),
	}
}

// StartAll starts all bridges from discovered manifests that have auto_start=true.
func (bm *BridgeManager) StartAll(manifests []AgentManifest) {
	for i := range manifests {
		m := &manifests[i]
		if m.Bridge == nil || !m.Bridge.AutoStart {
			continue
		}
		if err := bm.Start(m.ID, m.Bridge, m.Dir); err != nil {
			log.Printf("[BridgeManager] failed to start %s: %v", m.ID, err)
		}
	}

	// Start health check loop
	go bm.healthCheckLoop()
}

// Start launches a bridge subprocess for the given agent.
func (bm *BridgeManager) Start(agentID string, bridge *ManifestBridge, dir string) error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	// Already running?
	if inst, ok := bm.instances[agentID]; ok && inst.State == BridgeRunning {
		return fmt.Errorf("bridge %s already running on port %d", agentID, inst.Port)
	}

	// Allocate port
	port, err := bm.allocatePort(bridge)
	if err != nil {
		return fmt.Errorf("port allocation failed: %w", err)
	}

	// Build command
	cmd, err := buildBridgeCmd(bridge, dir, port)
	if err != nil {
		return fmt.Errorf("build command failed: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cmd.Dir = dir
	cmd.Env = buildBridgeEnv(bridge, port)

	// Log output
	logPrefix := fmt.Sprintf("[Bridge:%s]", agentID)
	cmd.Stdout = &prefixWriter{prefix: logPrefix, w: os.Stdout}
	cmd.Stderr = &prefixWriter{prefix: logPrefix, w: os.Stderr}

	inst := &BridgeInstance{
		AgentID:  agentID,
		Manifest: bridge,
		Dir:      dir,
		State:    BridgeStarting,
		Port:     port,
		URL:      fmt.Sprintf("http://127.0.0.1:%d", port),
		Env:      bridge.Env,
		cmd:      cmd,
		cancel:   cancel,
	}

	if bridge.Dashboard != "" {
		inst.DashboardURL = fmt.Sprintf("http://127.0.0.1:%d%s", port, bridge.Dashboard)
	}

	bm.instances[agentID] = inst
	bm.usedPorts[port] = agentID

	// Start subprocess
	if err := cmd.Start(); err != nil {
		inst.State = BridgeError
		inst.Error = err.Error()
		cancel()
		return fmt.Errorf("start subprocess: %w", err)
	}

	inst.PID = cmd.Process.Pid
	log.Printf("[BridgeManager] starting %s (pid=%d, port=%d)", agentID, inst.PID, port)

	// Wait for health check in background
	go bm.waitForHealthy(ctx, inst, bridge)

	// Watch for process exit
	go bm.watchProcess(inst)

	return nil
}

// Stop gracefully stops a bridge subprocess.
func (bm *BridgeManager) Stop(agentID string) error {
	bm.mu.Lock()
	inst, ok := bm.instances[agentID]
	if !ok {
		bm.mu.Unlock()
		return fmt.Errorf("bridge %s not found", agentID)
	}
	bm.mu.Unlock()

	log.Printf("[BridgeManager] stopping %s (pid=%d)", agentID, inst.PID)

	// Cancel context (signals shutdown)
	if inst.cancel != nil {
		inst.cancel()
	}

	// Try graceful shutdown via HTTP
	if inst.State == BridgeRunning {
		client := &http.Client{Timeout: 3 * time.Second}
		req, _ := http.NewRequest("POST", inst.URL+"/shutdown", nil)
		if req != nil {
			client.Do(req) //nolint:errcheck
		}
		time.Sleep(500 * time.Millisecond)
	}

	// Kill process if still running
	if inst.cmd != nil && inst.cmd.Process != nil {
		inst.cmd.Process.Kill() //nolint:errcheck
	}

	bm.mu.Lock()
	inst.State = BridgeStopped
	delete(bm.usedPorts, inst.Port)
	bm.mu.Unlock()

	log.Printf("[BridgeManager] stopped %s", agentID)
	return nil
}

// Restart stops and restarts a bridge.
func (bm *BridgeManager) Restart(agentID string) error {
	bm.mu.RLock()
	inst, ok := bm.instances[agentID]
	bm.mu.RUnlock()
	if !ok {
		return fmt.Errorf("bridge %s not found", agentID)
	}

	bridge := inst.Manifest
	dir := inst.Dir

	if err := bm.Stop(agentID); err != nil {
		log.Printf("[BridgeManager] stop error on restart: %v", err)
	}
	time.Sleep(1 * time.Second)
	return bm.Start(agentID, bridge, dir)
}

// Status returns the status of a specific bridge.
func (bm *BridgeManager) Status(agentID string) *BridgeInstance {
	bm.mu.RLock()
	defer bm.mu.RUnlock()
	return bm.instances[agentID]
}

// StatusAll returns all bridge statuses.
func (bm *BridgeManager) StatusAll() map[string]*BridgeInstance {
	bm.mu.RLock()
	defer bm.mu.RUnlock()
	result := make(map[string]*BridgeInstance, len(bm.instances))
	for k, v := range bm.instances {
		result[k] = v
	}
	return result
}

// StopAll stops all running bridges (called on shutdown).
func (bm *BridgeManager) StopAll() {
	close(bm.stopCh)
	bm.mu.RLock()
	ids := make([]string, 0, len(bm.instances))
	for id, inst := range bm.instances {
		if inst.State == BridgeRunning || inst.State == BridgeStarting {
			ids = append(ids, id)
		}
	}
	bm.mu.RUnlock()

	for _, id := range ids {
		bm.Stop(id) //nolint:errcheck
	}
	log.Printf("[BridgeManager] all bridges stopped")
}

// ── Internal ─────────────────────────────────────────────────

func (bm *BridgeManager) allocatePort(bridge *ManifestBridge) (int, error) {
	// Preferred port
	preferred := bridge.Port
	if preferred > 0 {
		if _, used := bm.usedPorts[preferred]; !used && isPortFree(preferred) {
			return preferred, nil
		}
	}

	// Try range
	lo, hi := bridge.PortRange[0], bridge.PortRange[1]
	if lo == 0 || hi == 0 {
		lo, hi = 8100, 8199 // default range
	}
	for port := lo; port <= hi; port++ {
		if _, used := bm.usedPorts[port]; used {
			continue
		}
		if isPortFree(port) {
			return port, nil
		}
	}

	return 0, fmt.Errorf("no free port in range %d-%d", lo, hi)
}

func isPortFree(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return false
	}
	ln.Close()
	return true
}

func buildBridgeCmd(bridge *ManifestBridge, dir string, port int) (*exec.Cmd, error) {
	entry := filepath.Join(dir, bridge.Entry)

	switch bridge.Type {
	case "python":
		python := findPython()
		return exec.Command(python, entry), nil

	case "node":
		return exec.Command("node", entry), nil

	case "go":
		// For Go bridges, build and run
		binName := "bridge"
		if runtime.GOOS == "windows" {
			binName = "bridge.exe"
		}
		binPath := filepath.Join(dir, binName)
		// Build if needed
		if _, err := os.Stat(binPath); os.IsNotExist(err) {
			buildCmd := exec.Command("go", "build", "-o", binPath, entry)
			buildCmd.Dir = dir
			if out, err := buildCmd.CombinedOutput(); err != nil {
				return nil, fmt.Errorf("go build failed: %s: %w", string(out), err)
			}
		}
		return exec.Command(binPath), nil

	case "external":
		// External bridge is managed separately, just track status
		return exec.Command(entry), nil

	default:
		return nil, fmt.Errorf("unsupported bridge type: %s", bridge.Type)
	}
}

func findPython() string {
	// Try python3 first, then python
	for _, name := range []string{"python3", "python"} {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	return "python"
}

func buildBridgeEnv(bridge *ManifestBridge, port int) []string {
	env := os.Environ()
	env = append(env, fmt.Sprintf("PORT=%d", port))
	env = append(env, fmt.Sprintf("BRIDGE_PORT=%d", port))
	for k, v := range bridge.Env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	return env
}

func (bm *BridgeManager) waitForHealthy(ctx context.Context, inst *BridgeInstance, bridge *ManifestBridge) {
	healthURL := fmt.Sprintf("http://127.0.0.1:%d%s", inst.Port, bridge.HealthCheck)
	if bridge.HealthCheck == "" {
		healthURL = fmt.Sprintf("http://127.0.0.1:%d/health", inst.Port)
	}

	deadline := time.Now().Add(bridgeStartupTimeout)
	client := &http.Client{Timeout: bridgeHealthTimeout}

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return
		default:
		}

		resp, err := client.Get(healthURL)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				bm.mu.Lock()
				inst.State = BridgeRunning
				now := time.Now()
				inst.StartedAt = &now
				inst.Error = ""
				bm.mu.Unlock()
				log.Printf("[BridgeManager] ✓ %s healthy (port=%d, pid=%d)", inst.AgentID, inst.Port, inst.PID)
				return
			}
		}
		time.Sleep(1 * time.Second)
	}

	bm.mu.Lock()
	inst.State = BridgeError
	inst.Error = "health check timeout after " + bridgeStartupTimeout.String()
	bm.mu.Unlock()
	log.Printf("[BridgeManager] ✗ %s health check timeout", inst.AgentID)
}

func (bm *BridgeManager) watchProcess(inst *BridgeInstance) {
	if inst.cmd == nil || inst.cmd.Process == nil {
		return
	}
	err := inst.cmd.Wait()

	bm.mu.Lock()
	defer bm.mu.Unlock()

	if inst.State == BridgeStopped {
		return // intentional stop
	}

	inst.State = BridgeError
	if err != nil {
		inst.Error = fmt.Sprintf("process exited: %v", err)
	} else {
		inst.Error = "process exited unexpectedly"
	}
	log.Printf("[BridgeManager] %s process exited: %v", inst.AgentID, err)
}

func (bm *BridgeManager) healthCheckLoop() {
	ticker := time.NewTicker(bridgeHealthInterval)
	defer ticker.Stop()

	for {
		select {
		case <-bm.stopCh:
			return
		case <-ticker.C:
			bm.checkAllHealth()
		}
	}
}

func (bm *BridgeManager) checkAllHealth() {
	bm.mu.RLock()
	running := make([]*BridgeInstance, 0)
	for _, inst := range bm.instances {
		if inst.State == BridgeRunning {
			running = append(running, inst)
		}
	}
	bm.mu.RUnlock()

	client := &http.Client{Timeout: bridgeHealthTimeout}
	for _, inst := range running {
		healthPath := "/health"
		if inst.Manifest != nil && inst.Manifest.HealthCheck != "" {
			healthPath = inst.Manifest.HealthCheck
		}
		healthURL := fmt.Sprintf("http://127.0.0.1:%d%s", inst.Port, healthPath)

		resp, err := client.Get(healthURL)
		if err != nil || resp.StatusCode != http.StatusOK {
			bm.mu.Lock()
			inst.State = BridgeError
			inst.Error = "health check failed"
			bm.mu.Unlock()
			log.Printf("[BridgeManager] %s health check failed, marking as error", inst.AgentID)
			if resp != nil {
				resp.Body.Close()
			}
		} else {
			resp.Body.Close()
		}
	}
}

// ── prefixWriter ────────────────────────────────────────────

type prefixWriter struct {
	prefix string
	w      *os.File
}

func (pw *prefixWriter) Write(p []byte) (n int, err error) {
	// Prepend prefix to each line for log readability
	fmt.Fprintf(pw.w, "%s %s", pw.prefix, string(p))
	return len(p), nil
}
