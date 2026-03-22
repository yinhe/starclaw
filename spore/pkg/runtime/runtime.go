package runtime

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/yinhe/starclaw-spore/pkg/manifest"
	"github.com/yinhe/starclaw-spore/pkg/platform"
)

// SporeInstance represents an installed spore with runtime state.
type SporeInstance struct {
	Name       string             `json:"name"`
	Version    string             `json:"version"`
	InstallDir string             `json:"install_dir"`
	DataDir    string             `json:"data_dir"`
	LogDir     string             `json:"log_dir"`
	PidFile    string             `json:"pid_file"`
	Manifest   *manifest.Manifest `json:"manifest"`
	Status     string             `json:"status"` // stopped, running, error
}

// Manager handles spore installation, running, and lifecycle.
type Manager struct {
	sporeHome string
	platform  *platform.Info
}

// NewManager creates a runtime manager.
func NewManager(sporeHome string) *Manager {
	return &Manager{
		sporeHome: sporeHome,
		platform:  platform.Detect(),
	}
}

// InstalledDir returns the base directory for installed spores.
func (m *Manager) InstalledDir() string {
	return filepath.Join(m.sporeHome, "installed")
}

// Install extracts a .spore package and registers it.
func (m *Manager) Install(sporePath string) (*SporeInstance, error) {
	// Read manifest from the package to get name and version
	// For now, we extract to a temp dir first, read manifest, then move
	tmpDir, err := os.MkdirTemp("", "spore-install-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// We need to import archive package - but to avoid circular imports,
	// the caller should pass the extracted manifest or we parse it from the archive.
	// For simplicity, let's assume the archive is already extracted or we handle it at CLI level.
	// This method works with an already-extracted directory.
	return m.InstallFromDir(sporePath)
}

// InstallFromDir installs from an extracted spore directory containing manifest.json.
// An optional customName overrides the manifest name (for multi-instance deployment).
func (m *Manager) InstallFromDir(srcDir string, customName ...string) (*SporeInstance, error) {
	mf, err := manifest.Load(filepath.Join(srcDir, "manifest.json"))
	if err != nil {
		return nil, fmt.Errorf("load manifest: %w", err)
	}

	if !mf.MatchesRuntime() {
		return nil, fmt.Errorf("platform mismatch: package is %s/%s, running on %s/%s",
			mf.Platform.OS, mf.Platform.Arch, m.platform.OS, m.platform.Arch)
	}

	// Use custom name if provided (multi-instance support)
	instName := mf.Name
	if len(customName) > 0 && customName[0] != "" {
		instName = customName[0]
	}

	// Create install directories
	installBase := filepath.Join(m.InstalledDir(), instName)
	versionDir := filepath.Join(installBase, "v"+mf.Version)
	dataDir := filepath.Join(installBase, "data")
	logDir := filepath.Join(installBase, "logs")

	for _, d := range []string{versionDir, dataDir, logDir} {
		if err := os.MkdirAll(d, 0755); err != nil {
			return nil, fmt.Errorf("create dir %s: %w", d, err)
		}
	}

	// Stop THIS instance before overwriting files (Windows: exe locked while running)
	if existing, _ := m.loadInstance(instName); existing != nil && existing.Status == "running" {
		log.Printf("[spore] stopping running %s before upgrade...", instName)
		m.Stop(instName)
		time.Sleep(1 * time.Second)
	}

	// Copy files from srcDir to versionDir (retry once if locked on Windows)
	if err := copyDir(srcDir, versionDir); err != nil {
		if goruntime.GOOS == "windows" {
			log.Printf("[spore] copy failed, retrying after wait: %v", err)
			time.Sleep(2 * time.Second)
			if err2 := copyDir(srcDir, versionDir); err2 != nil {
				return nil, fmt.Errorf("copy files: %w", err2)
			}
		} else {
			return nil, fmt.Errorf("copy files: %w", err)
		}
	}

	// Make binary executable
	binPath := filepath.Join(versionDir, mf.Binary)
	os.Chmod(binPath, 0755)

	// Update "current" symlink
	currentLink := filepath.Join(installBase, "current")
	os.Remove(currentLink)
	if err := createSymlinkOrMarker(versionDir, currentLink); err != nil {
		return nil, fmt.Errorf("create current link: %w", err)
	}

	log.Printf("[spore] installed %s v%s to %s", instName, mf.Version, versionDir)

	return &SporeInstance{
		Name:       instName,
		Version:    mf.Version,
		InstallDir: versionDir,
		DataDir:    dataDir,
		LogDir:     logDir,
		PidFile:    filepath.Join(installBase, instName+".pid"),
		Manifest:   mf,
		Status:     "stopped",
	}, nil
}

// List returns all installed spore instances.
func (m *Manager) List() ([]*SporeInstance, error) {
	installedDir := m.InstalledDir()
	entries, err := os.ReadDir(installedDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var instances []*SporeInstance
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		inst, err := m.loadInstance(entry.Name())
		if err != nil {
			continue
		}
		instances = append(instances, inst)
	}
	return instances, nil
}

// Get returns a specific installed spore.
func (m *Manager) Get(name string) (*SporeInstance, error) {
	return m.loadInstance(name)
}

func (m *Manager) loadInstance(name string) (*SporeInstance, error) {
	installBase := filepath.Join(m.InstalledDir(), name)
	currentDir := resolveCurrentDir(installBase)
	if currentDir == "" {
		return nil, fmt.Errorf("spore %q not installed", name)
	}

	mf, err := manifest.Load(filepath.Join(currentDir, "manifest.json"))
	if err != nil {
		return nil, err
	}

	inst := &SporeInstance{
		Name:       name,
		Version:    mf.Version,
		InstallDir: currentDir,
		DataDir:    filepath.Join(installBase, "data"),
		LogDir:     filepath.Join(installBase, "logs"),
		PidFile:    filepath.Join(installBase, name+".pid"),
		Manifest:   mf,
	}

	inst.Status = m.checkStatus(inst)
	return inst, nil
}

// Start launches a spore process in the background.
func (m *Manager) Start(name string) error {
	return m.StartWithEnv(name, nil)
}

// StartWithEnv launches a spore process with additional environment variable overrides.
// envOverrides format: ["KEY=VALUE", ...]
func (m *Manager) StartWithEnv(name string, envOverrides []string) error {
	inst, err := m.Get(name)
	if err != nil {
		return err
	}

	if inst.Status == "running" {
		return fmt.Errorf("%s is already running", name)
	}

	binPath := filepath.Join(inst.InstallDir, inst.Manifest.Binary)
	if _, err := os.Stat(binPath); err != nil {
		return fmt.Errorf("binary not found: %s", binPath)
	}

	// Build command
	args := inst.Manifest.Args
	cmd := exec.Command(binPath, args...)
	cmd.Dir = inst.InstallDir
	env := append(os.Environ(),
		fmt.Sprintf("SPORE_DATA_DIR=%s", inst.DataDir),
		fmt.Sprintf("SPORE_LOG_DIR=%s", inst.LogDir),
	)
	// Inject custom env vars from manifest
	for k, v := range inst.Manifest.Env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	// Apply CLI env overrides (highest priority)
	env = append(env, envOverrides...)
	cmd.Env = env

	// Redirect output to log file
	logFile, err := os.OpenFile(
		filepath.Join(inst.LogDir, name+".log"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	// Detach process
	cmd.SysProcAttr = detachAttr()

	if err := cmd.Start(); err != nil {
		logFile.Close()
		return fmt.Errorf("start process: %w", err)
	}

	// Write PID file
	os.WriteFile(inst.PidFile, []byte(strconv.Itoa(cmd.Process.Pid)), 0644)

	log.Printf("[spore] started %s (PID %d)", name, cmd.Process.Pid)

	// Release the process so it runs independently
	cmd.Process.Release()
	logFile.Close()

	return nil
}

// Stop sends a signal to gracefully stop a running spore.
func (m *Manager) Stop(name string) error {
	inst, err := m.Get(name)
	if err != nil {
		return err
	}

	pid, err := m.readPid(inst.PidFile)
	if err != nil {
		return fmt.Errorf("%s is not running (no PID file)", name)
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		os.Remove(inst.PidFile)
		return fmt.Errorf("process %d not found", pid)
	}

	// Send SIGTERM (or equivalent)
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		// Process may already be dead
		os.Remove(inst.PidFile)
		return nil
	}

	// Wait up to 10 seconds for graceful shutdown
	for i := 0; i < 20; i++ {
		time.Sleep(500 * time.Millisecond)
		if !isProcessRunning(pid) {
			os.Remove(inst.PidFile)
			log.Printf("[spore] stopped %s (PID %d)", name, pid)
			return nil
		}
	}

	// Force kill
	proc.Kill()
	os.Remove(inst.PidFile)
	log.Printf("[spore] force-killed %s (PID %d)", name, pid)
	return nil
}

// Status returns the status of a spore.
func (m *Manager) checkStatus(inst *SporeInstance) string {
	pid, err := m.readPid(inst.PidFile)
	if err != nil {
		return "stopped"
	}
	if isProcessRunning(pid) {
		return "running"
	}
	// Stale PID file
	os.Remove(inst.PidFile)
	return "stopped"
}

// HealthCheck performs an HTTP health check.
func (m *Manager) HealthCheck(name string) (bool, error) {
	inst, err := m.Get(name)
	if err != nil {
		return false, err
	}

	endpoint := inst.Manifest.Health.Endpoint
	if endpoint == "" {
		return true, nil // no health check configured, assume healthy
	}

	timeout := time.Duration(inst.Manifest.Health.TimeoutSeconds) * time.Second
	if timeout == 0 {
		timeout = 5 * time.Second
	}

	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(endpoint)
	if err != nil {
		return false, nil
	}
	defer resp.Body.Close()

	return resp.StatusCode >= 200 && resp.StatusCode < 400, nil
}

// RunInline runs a spore binary in the foreground (for Docker/container use).
// It reads manifest.json from the given directory, runs the binary as PID 1,
// and restarts it on crash. This blocks until SIGTERM/SIGINT.
func RunInline(dir string, envOverrides []string) error {
	mf, err := manifest.Load(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return fmt.Errorf("load manifest: %w", err)
	}

	binPath := filepath.Join(dir, mf.Binary)
	if _, err := os.Stat(binPath); err != nil {
		return fmt.Errorf("binary not found: %s", binPath)
	}
	os.Chmod(binPath, 0755)

	// Ensure data dir
	dataDir := filepath.Join(dir, "data")
	os.MkdirAll(dataDir, 0755)

	log.Printf("[spore-inline] starting %s v%s (%s)", mf.Name, mf.Version, binPath)

	for {
		cmd := exec.Command(binPath, mf.Args...)
		cmd.Dir = dir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		env := os.Environ()
		env = append(env, fmt.Sprintf("SPORE_DATA_DIR=%s", dataDir))
		for k, v := range mf.Env {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
		env = append(env, envOverrides...)
		cmd.Env = env

		if err := cmd.Run(); err != nil {
			log.Printf("[spore-inline] process exited: %v, restarting in 3s...", err)
			time.Sleep(3 * time.Second)
			continue
		}
		// Clean exit
		log.Printf("[spore-inline] process exited cleanly")
		return nil
	}
}

// Uninstall removes an installed spore.
func (m *Manager) Uninstall(name string) error {
	inst, err := m.Get(name)
	if err != nil {
		return err
	}

	if inst.Status == "running" {
		m.Stop(name)
	}

	installBase := filepath.Join(m.InstalledDir(), name)
	return os.RemoveAll(installBase)
}

// Logs returns the path to the log file.
func (m *Manager) LogPath(name string) string {
	return filepath.Join(m.InstalledDir(), name, "logs", name+".log")
}

// ── Helpers ──

func (m *Manager) readPid(pidFile string) (int, error) {
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(data)))
}

func isProcessRunning(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Windows, Signal(0) always succeeds for any PID from FindProcess.
	// Use a non-blocking wait or tasklist check instead.
	if goruntime.GOOS == "windows" {
		// Try to open the process handle — if it fails, the process is dead
		cmd := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/NH")
		out, err := cmd.Output()
		if err != nil {
			return false
		}
		return strings.Contains(string(out), strconv.Itoa(pid))
	}
	err = proc.Signal(syscall.Signal(0))
	return err == nil
}

func resolveCurrentDir(installBase string) string {
	currentLink := filepath.Join(installBase, "current")

	// Try symlink first
	target, err := os.Readlink(currentLink)
	if err == nil {
		if filepath.IsAbs(target) {
			return target
		}
		return filepath.Join(installBase, target)
	}

	// Try marker file (for platforms without symlink support)
	data, err := os.ReadFile(currentLink)
	if err == nil {
		t := strings.TrimSpace(string(data))
		if filepath.IsAbs(t) {
			return t
		}
		return filepath.Join(installBase, t)
	}

	return ""
}

func createSymlinkOrMarker(target, link string) error {
	// Try symlink first
	if err := os.Symlink(target, link); err == nil {
		return nil
	}
	// Fallback: write target path as a text file (Windows without Developer Mode)
	return os.WriteFile(link, []byte(target), 0644)
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, _ := filepath.Rel(src, path)
		targetPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(targetPath, info.Mode())
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(targetPath, data, info.Mode())
	})
}

// detachAttr is defined in detach_windows.go and detach_unix.go

// IndexEntry is used for the local spore registry index.
type IndexEntry struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	Platform  string `json:"platform"`
	Checksum  string `json:"checksum"`
	Size      int64  `json:"size"`
	CreatedAt string `json:"created_at"`
}

// SaveIndex writes the local index to disk.
func (m *Manager) SaveIndex() error {
	instances, _ := m.List()
	entries := make([]IndexEntry, 0, len(instances))
	for _, inst := range instances {
		entries = append(entries, IndexEntry{
			Name:     inst.Name,
			Version:  inst.Version,
			Platform: fmt.Sprintf("%s/%s", inst.Manifest.Platform.OS, inst.Manifest.Platform.Arch),
		})
	}

	data, _ := json.MarshalIndent(entries, "", "  ")
	indexPath := filepath.Join(m.sporeHome, "registry", "index.json")
	os.MkdirAll(filepath.Dir(indexPath), 0755)
	return os.WriteFile(indexPath, data, 0644)
}
