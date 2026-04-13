package molt

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// ════════════════════════════════════════════════════════════
// Molt Upgrade Protocol — Stage → Apply → Healthcheck → Rollback
// ════════════════════════════════════════════════════════════

// UpgradeState tracks the current upgrade lifecycle
type UpgradeState struct {
	Phase         string    `json:"phase"`          // idle, staging, staged, applying, applied, healthcheck, complete, rollback, failed
	TargetVersion string    `json:"target_version"`
	Source        string    `json:"source"`          // github / nydus
	StagedAt      time.Time `json:"staged_at,omitempty"`
	AppliedAt     time.Time `json:"applied_at,omitempty"`
	Error         string    `json:"error,omitempty"`
	RollbackInfo  string    `json:"rollback_info,omitempty"`
}

var (
	upgradeMu    sync.Mutex
	upgradeState = UpgradeState{Phase: "idle"}

	// Configurable paths
	binaryDir    string // directory of the running binary
	stagingDir   string // temp dir for downloaded update
	backupDir    string // backup of current binary before apply
	healthURL    string // URL to hit for post-upgrade health check
	healthChecks = 3    // number of consecutive health checks
	healthDelay  = 5 * time.Second
)

// InitUpgrade configures the upgrade subsystem paths.
// Called from main.go after determining install directory.
func InitUpgrade(installDir string) {
	binaryDir = installDir
	stagingDir = filepath.Join(installDir, ".molt-staging")
	backupDir = filepath.Join(installDir, ".molt-backup")

	// Default health check URL: self
	healthURL = "http://localhost:9101/health"
	if port := os.Getenv("CLAW_PORT"); port != "" {
		healthURL = "http://localhost:" + port + "/health"
	}

	log.Printf("[molt] upgrade protocol initialized (dir=%s)", installDir)
}

// SetHealthURL overrides the health check endpoint
func SetHealthURL(url string) {
	upgradeMu.Lock()
	healthURL = url
	upgradeMu.Unlock()
}

// GetUpgradeState returns the current upgrade state
func GetUpgradeState() UpgradeState {
	upgradeMu.Lock()
	defer upgradeMu.Unlock()
	return upgradeState
}

// ── Phase 1: Stage ──

// Stage downloads the new version to a staging directory without applying it.
// This allows pre-validation before any service disruption.
func Stage(targetVersion string) error {
	upgradeMu.Lock()
	defer upgradeMu.Unlock()

	if upgradeState.Phase != "idle" && upgradeState.Phase != "failed" && upgradeState.Phase != "complete" {
		return fmt.Errorf("upgrade already in progress (phase=%s)", upgradeState.Phase)
	}

	upgradeState = UpgradeState{
		Phase:         "staging",
		TargetVersion: targetVersion,
	}

	log.Printf("[molt] staging version %s", targetVersion)

	// Clean and create staging dir
	os.RemoveAll(stagingDir)
	if err := os.MkdirAll(stagingDir, 0755); err != nil {
		upgradeState.Phase = "failed"
		upgradeState.Error = err.Error()
		return err
	}

	// Try download from sources
	downloaded := false
	for _, src := range UpdateSources {
		dlURL := buildDownloadURL(src, targetVersion)
		if dlURL == "" {
			continue
		}
		log.Printf("[molt] downloading from %s: %s", src.Name, dlURL)
		if err := downloadFile(dlURL, filepath.Join(stagingDir, "update.tar.gz"), src.Timeout*3); err != nil {
			log.Printf("[molt] download from %s failed: %v", src.Name, err)
			continue
		}
		upgradeState.Source = src.Name
		downloaded = true
		break
	}

	if !downloaded {
		// Fallback: Nydus source tarball
		log.Printf("[molt] trying nydus source fallback: %s", NydusSourceURL)
		if err := downloadFile(NydusSourceURL, filepath.Join(stagingDir, "update.tar.gz"), 30*time.Second); err != nil {
			upgradeState.Phase = "failed"
			upgradeState.Error = "all download sources failed"
			return fmt.Errorf("all download sources failed")
		}
		upgradeState.Source = "nydus-source"
	}

	// Verify staging artifact exists and has reasonable size
	info, err := os.Stat(filepath.Join(stagingDir, "update.tar.gz"))
	if err != nil || info.Size() < 1024 {
		upgradeState.Phase = "failed"
		upgradeState.Error = "staged artifact too small or missing"
		return fmt.Errorf("staged artifact invalid")
	}

	upgradeState.Phase = "staged"
	upgradeState.StagedAt = time.Now()
	log.Printf("[molt] staged version %s (%.1f MB, source=%s)",
		targetVersion, float64(info.Size())/1024/1024, upgradeState.Source)
	return nil
}

// ── Phase 2: Apply ──

// Apply backs up the current binary and replaces it with the staged version.
// The service should be restarted after Apply succeeds.
func Apply() error {
	upgradeMu.Lock()
	defer upgradeMu.Unlock()

	if upgradeState.Phase != "staged" {
		return fmt.Errorf("cannot apply: current phase is %s (need 'staged')", upgradeState.Phase)
	}

	upgradeState.Phase = "applying"
	log.Printf("[molt] applying version %s", upgradeState.TargetVersion)

	// Create backup of current binary
	os.RemoveAll(backupDir)
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		upgradeState.Phase = "failed"
		upgradeState.Error = err.Error()
		return err
	}

	// Backup current executables
	currentBinary := getCurrentBinary()
	if currentBinary != "" {
		backupPath := filepath.Join(backupDir, filepath.Base(currentBinary))
		if err := copyFile(currentBinary, backupPath); err != nil {
			log.Printf("[molt] backup warning: %v", err)
			// Non-fatal: continue with apply
		} else {
			upgradeState.RollbackInfo = backupPath
			log.Printf("[molt] backed up %s → %s", currentBinary, backupPath)
		}
	}

	// Extract staged update
	archivePath := filepath.Join(stagingDir, "update.tar.gz")
	extractDir := filepath.Join(stagingDir, "extracted")
	os.MkdirAll(extractDir, 0755)

	if err := extractTarGz(archivePath, extractDir); err != nil {
		upgradeState.Phase = "failed"
		upgradeState.Error = "extract failed: " + err.Error()
		return err
	}

	// Find and copy new binary to install directory
	newBinary := findBinary(extractDir)
	if newBinary == "" {
		upgradeState.Phase = "failed"
		upgradeState.Error = "no binary found in staged update"
		return fmt.Errorf("no binary found in staged update")
	}

	targetPath := filepath.Join(binaryDir, filepath.Base(currentBinary))
	if err := copyFile(newBinary, targetPath); err != nil {
		upgradeState.Phase = "failed"
		upgradeState.Error = "copy binary failed: " + err.Error()
		return err
	}

	// Make executable on Unix
	if runtime.GOOS != "windows" {
		os.Chmod(targetPath, 0755)
	}

	upgradeState.Phase = "applied"
	upgradeState.AppliedAt = time.Now()
	log.Printf("[molt] applied version %s → %s", upgradeState.TargetVersion, targetPath)
	return nil
}

// ── Phase 3: Healthcheck ──

// Healthcheck verifies the new version is running correctly after restart.
// Should be called after the service has been restarted with the new binary.
func Healthcheck() error {
	upgradeMu.Lock()
	upgradeState.Phase = "healthcheck"
	url := healthURL
	checks := healthChecks
	delay := healthDelay
	upgradeMu.Unlock()

	log.Printf("[molt] running %d health checks against %s", checks, url)

	client := &http.Client{Timeout: 10 * time.Second}
	for i := 0; i < checks; i++ {
		time.Sleep(delay)

		resp, err := client.Get(url)
		if err != nil {
			log.Printf("[molt] health check %d/%d failed: %v", i+1, checks, err)
			upgradeMu.Lock()
			upgradeState.Phase = "failed"
			upgradeState.Error = fmt.Sprintf("health check failed: %v", err)
			upgradeMu.Unlock()
			return err
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			err := fmt.Errorf("health check returned %d", resp.StatusCode)
			log.Printf("[molt] health check %d/%d: %v", i+1, checks, err)
			upgradeMu.Lock()
			upgradeState.Phase = "failed"
			upgradeState.Error = err.Error()
			upgradeMu.Unlock()
			return err
		}
		log.Printf("[molt] health check %d/%d: OK", i+1, checks)
	}

	upgradeMu.Lock()
	upgradeState.Phase = "complete"
	upgradeMu.Unlock()

	log.Printf("[molt] upgrade to %s complete — all health checks passed", upgradeState.TargetVersion)

	// Clean up staging and backup
	go func() {
		time.Sleep(5 * time.Minute)
		os.RemoveAll(stagingDir)
		os.RemoveAll(backupDir)
		log.Printf("[molt] cleaned up staging/backup dirs")
	}()

	return nil
}

// ── Phase 4: Rollback ──

// Rollback restores the previous binary from backup.
// Should be called when healthcheck fails or upgrade causes issues.
func Rollback() error {
	upgradeMu.Lock()
	defer upgradeMu.Unlock()

	if upgradeState.RollbackInfo == "" {
		return fmt.Errorf("no rollback info available (no backup was made)")
	}

	backupPath := upgradeState.RollbackInfo
	currentBinary := getCurrentBinary()
	if currentBinary == "" {
		return fmt.Errorf("cannot determine current binary path")
	}

	upgradeState.Phase = "rollback"
	log.Printf("[molt] rolling back: restoring %s from %s", currentBinary, backupPath)

	if err := copyFile(backupPath, currentBinary); err != nil {
		upgradeState.Phase = "failed"
		upgradeState.Error = "rollback failed: " + err.Error()
		return err
	}

	if runtime.GOOS != "windows" {
		os.Chmod(currentBinary, 0755)
	}

	upgradeState.Phase = "idle"
	upgradeState.Error = ""
	log.Printf("[molt] rollback complete — restart required to use previous version")
	return nil
}

// ── HTTP Handlers (registered in claw-api routes) ──

// HandleGetUpgradeState returns the current upgrade state.
// GET /molt/status
func HandleGetUpgradeState(w http.ResponseWriter, r *http.Request) {
	state := GetUpgradeState()
	state.Phase = upgradeState.Phase // include latest
	vi := GetVersionInfo()

	resp := map[string]interface{}{
		"version":       vi,
		"upgrade_state": state,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// HandleStage starts staging an upgrade.
// POST /molt/stage {"version": "2026.0411.0000"}
func HandleStage(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Version == "" {
		http.Error(w, `{"error":"version required"}`, http.StatusBadRequest)
		return
	}

	go func() {
		if err := Stage(req.Version); err != nil {
			log.Printf("[molt] stage failed: %v", err)
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status":"staging","target_version":"%s"}`, req.Version)
}

// HandleApply applies the staged upgrade.
// POST /molt/apply
func HandleApply(w http.ResponseWriter, r *http.Request) {
	if err := Apply(); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		fmt.Fprintf(w, `{"error":"%s"}`, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, `{"status":"applied","message":"restart service to use new version"}`)
}

// HandleHealthcheck runs post-upgrade health checks.
// POST /molt/healthcheck
func HandleHealthcheck(w http.ResponseWriter, r *http.Request) {
	go func() {
		if err := Healthcheck(); err != nil {
			log.Printf("[molt] healthcheck failed, initiating rollback: %v", err)
			if rbErr := Rollback(); rbErr != nil {
				log.Printf("[molt] auto-rollback also failed: %v", rbErr)
			}
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, `{"status":"healthcheck_started"}`)
}

// HandleRollback manually triggers a rollback.
// POST /molt/rollback
func HandleRollback(w http.ResponseWriter, r *http.Request) {
	if err := Rollback(); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		fmt.Fprintf(w, `{"error":"%s"}`, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, `{"status":"rolled_back","message":"restart service to revert"}`)
}

// ── Utility functions ──

func buildDownloadURL(src UpdateSource, version string) string {
	switch src.Name {
	case "github":
		return fmt.Sprintf("https://github.com/%s/%s/releases/download/v%s/claw-%s-%s.tar.gz",
			owner, repo, version, runtime.GOOS, runtime.GOARCH)
	case "nydus":
		return fmt.Sprintf("https://nydus.starclaw.net/releases/v%s/claw-%s-%s.tar.gz",
			version, runtime.GOOS, runtime.GOARCH)
	}
	return ""
}

func downloadFile(url, destPath string, timeout time.Duration) error {
	client := &http.Client{Timeout: timeout}
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "StarClaw-Molt/"+Version)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned %d", resp.StatusCode)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func getCurrentBinary() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	resolved, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return exe
	}
	return resolved
}

func extractTarGz(archivePath, destDir string) error {
	// Use system tar if available (most reliable)
	cmd := exec.Command("tar", "xzf", archivePath, "-C", destDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("tar extract: %v (%s)", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func findBinary(dir string) string {
	// Look for claw-api or claw-api.exe
	names := []string{"claw-api", "claw-api.exe", "mcp-bridge", "mcp-bridge.exe"}
	var found string
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		for _, name := range names {
			if info.Name() == name {
				found = path
				return filepath.SkipAll
			}
		}
		return nil
	})
	return found
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
