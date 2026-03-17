package creep

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/yinhe/starclaw-spore/pkg/manifest"
	"github.com/yinhe/starclaw-spore/pkg/platform"
	rtPkg "github.com/yinhe/starclaw-spore/pkg/runtime"
)

// DeviceReport is sent from Creep Agent to Queen periodically.
type DeviceReport struct {
	DeviceID   string          `json:"device_id"`
	Hostname   string          `json:"hostname"`
	Platform   platform.Info   `json:"platform"`
	Resources  DeviceResources `json:"resources"`
	Spores     []SporeStatus   `json:"spores"`
	Status     string          `json:"status"` // online, updating
	ReportedAt string          `json:"reported_at"`
	AgentVer   string          `json:"agent_version"`
}

// DeviceResources reports hardware utilization.
type DeviceResources struct {
	CPUCores   int   `json:"cpu_cores"`
	MemTotalMB int64 `json:"mem_total_mb"`
	MemUsedMB  int64 `json:"mem_used_mb"`
	DiskTotalMB int64 `json:"disk_total_mb"`
	DiskUsedMB  int64 `json:"disk_used_mb"`
}

// SporeStatus reports the state of an installed spore.
type SporeStatus struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Status  string `json:"status"` // running, stopped, error
	Healthy bool   `json:"healthy"`
	PID     int    `json:"pid,omitempty"`
}

// Command is received from Queen to perform an action.
type Command struct {
	Action  string `json:"action"`  // deploy, update, rollback, stop, start, uninstall
	Name    string `json:"name"`    // spore name
	Version string `json:"version,omitempty"`
	URL     string `json:"url,omitempty"` // download URL for deploy/update
}

// Agent is the Creep device agent that reports to Queen.
type Agent struct {
	deviceID   string
	queenURL   string
	sporeHome  string
	manager    *rtPkg.Manager
	platInfo   *platform.Info
	interval   time.Duration
	httpC      *http.Client

	mu     sync.Mutex
	stopCh chan struct{}
}

// NewAgent creates a new Creep agent.
func NewAgent(deviceID, queenURL, sporeHome string) *Agent {
	return &Agent{
		deviceID:  deviceID,
		queenURL:  queenURL,
		sporeHome: sporeHome,
		manager:   rtPkg.NewManager(sporeHome),
		platInfo:  platform.Detect(),
		interval:  60 * time.Second,
		httpC:     &http.Client{Timeout: 10 * time.Second},
		stopCh:    make(chan struct{}),
	}
}

// Start begins the periodic reporting loop.
func (a *Agent) Start() {
	go a.loop()
	log.Printf("[creep] agent started (device=%s, queen=%s, interval=%s)", a.deviceID, a.queenURL, a.interval)
}

// Stop halts the agent.
func (a *Agent) Stop() {
	select {
	case <-a.stopCh:
	default:
		close(a.stopCh)
	}
}

func (a *Agent) loop() {
	// Report immediately on start
	a.report()

	ticker := time.NewTicker(a.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			a.report()
		case <-a.stopCh:
			return
		}
	}
}

func (a *Agent) report() {
	a.mu.Lock()
	defer a.mu.Unlock()

	report := a.buildReport()

	data, err := json.Marshal(report)
	if err != nil {
		log.Printf("[creep] marshal report: %v", err)
		return
	}

	url := fmt.Sprintf("%s/v1/creep/report", a.queenURL)
	resp, err := a.httpC.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		log.Printf("[creep] report failed: %v", err)
		return
	}
	defer resp.Body.Close()

	// Check for pending commands from Queen
	if resp.StatusCode == http.StatusOK {
		var commands []Command
		if err := json.NewDecoder(resp.Body).Decode(&commands); err == nil {
			for _, cmd := range commands {
				go a.executeCommand(cmd)
			}
		}
	}
}

func (a *Agent) buildReport() *DeviceReport {
	spores := a.collectSporeStatus()

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	return &DeviceReport{
		DeviceID: a.deviceID,
		Hostname: a.platInfo.Hostname,
		Platform: *a.platInfo,
		Resources: DeviceResources{
			CPUCores:   runtime.NumCPU(),
			MemTotalMB: int64(memStats.Sys / (1024 * 1024)),
		},
		Spores:     spores,
		Status:     "online",
		ReportedAt: time.Now().UTC().Format(time.RFC3339),
		AgentVer:   "0.1.0",
	}
}

func (a *Agent) collectSporeStatus() []SporeStatus {
	instances, err := a.manager.List()
	if err != nil {
		return nil
	}

	statuses := make([]SporeStatus, 0, len(instances))
	for _, inst := range instances {
		healthy := false
		if inst.Status == "running" {
			healthy, _ = a.manager.HealthCheck(inst.Name)
		}
		statuses = append(statuses, SporeStatus{
			Name:    inst.Name,
			Version: inst.Version,
			Status:  inst.Status,
			Healthy: healthy,
		})
	}
	return statuses
}

func (a *Agent) executeCommand(cmd Command) {
	log.Printf("[creep] executing command: %s %s (version=%s)", cmd.Action, cmd.Name, cmd.Version)

	var err error
	switch cmd.Action {
	case "start":
		err = a.manager.Start(cmd.Name)
	case "stop":
		err = a.manager.Stop(cmd.Name)
	case "restart":
		a.manager.Stop(cmd.Name)
		err = a.manager.Start(cmd.Name)
	case "uninstall":
		err = a.manager.Uninstall(cmd.Name)
	case "deploy", "update":
		err = a.deployOrUpdate(cmd)
	case "rollback":
		err = a.rollback(cmd)
	default:
		log.Printf("[creep] unknown command action: %s", cmd.Action)
		return
	}

	if err != nil {
		log.Printf("[creep] command %s %s failed: %v", cmd.Action, cmd.Name, err)
		a.reportCommandResult(cmd, "failed", err.Error())
	} else {
		log.Printf("[creep] command %s %s succeeded", cmd.Action, cmd.Name)
		a.reportCommandResult(cmd, "success", "")
	}
}

func (a *Agent) deployOrUpdate(cmd Command) error {
	if cmd.URL == "" {
		return fmt.Errorf("no download URL provided")
	}

	// Download .spore file
	cacheDir := fmt.Sprintf("%s/cache", a.sporeHome)
	os.MkdirAll(cacheDir, 0755)

	tmpFile := fmt.Sprintf("%s/%s-%s.spore", cacheDir, cmd.Name, cmd.Version)
	if err := downloadFile(a.httpC, cmd.URL, tmpFile); err != nil {
		return fmt.Errorf("download: %w", err)
	}

	// Stop if running
	a.manager.Stop(cmd.Name)

	// Install
	inst, err := a.manager.Install(tmpFile)
	if err != nil {
		return fmt.Errorf("install: %w", err)
	}

	// Start
	if err := a.manager.Start(inst.Name); err != nil {
		return fmt.Errorf("start: %w", err)
	}

	// Health check with retries
	for i := 0; i < 6; i++ {
		time.Sleep(5 * time.Second)
		healthy, _ := a.manager.HealthCheck(inst.Name)
		if healthy {
			return nil
		}
	}

	return fmt.Errorf("health check failed after deploy")
}

func (a *Agent) rollback(cmd Command) error {
	// List version directories and switch to the previous one
	installBase := fmt.Sprintf("%s/installed/%s", a.sporeHome, cmd.Name)
	entries, err := os.ReadDir(installBase)
	if err != nil {
		return err
	}

	var versions []string
	for _, e := range entries {
		if e.IsDir() && len(e.Name()) > 1 && e.Name()[0] == 'v' {
			versions = append(versions, e.Name())
		}
	}

	if len(versions) < 2 {
		return fmt.Errorf("no previous version to rollback to")
	}

	// Stop current
	a.manager.Stop(cmd.Name)

	// Switch current symlink to previous version
	prevVersion := versions[len(versions)-2]
	currentLink := fmt.Sprintf("%s/current", installBase)
	os.Remove(currentLink)
	target := fmt.Sprintf("%s/%s", installBase, prevVersion)
	os.Symlink(target, currentLink)

	// Start
	return a.manager.Start(cmd.Name)
}

func (a *Agent) reportCommandResult(cmd Command, status, errMsg string) {
	result := map[string]string{
		"device_id": a.deviceID,
		"action":    cmd.Action,
		"name":      cmd.Name,
		"status":    status,
		"error":     errMsg,
	}
	data, _ := json.Marshal(result)
	url := fmt.Sprintf("%s/v1/creep/command-result", a.queenURL)
	a.httpC.Post(url, "application/json", bytes.NewReader(data))
}

// ── Helpers ──

func downloadFile(client *http.Client, url, dest string) error {
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = copyWithProgress(f, resp.Body, resp.ContentLength)
	return err
}

func copyWithProgress(dst *os.File, src interface{ Read([]byte) (int, error) }, total int64) (int64, error) {
	buf := make([]byte, 32*1024)
	var written int64
	for {
		n, err := src.Read(buf)
		if n > 0 {
			nw, ew := dst.Write(buf[:n])
			if nw > 0 {
				written += int64(nw)
			}
			if ew != nil {
				return written, ew
			}
		}
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return written, err
		}
	}
	return written, nil
}

// ── Manifest helper ──

// SporeManifest is just re-exported for convenience.
type SporeManifest = manifest.Manifest
