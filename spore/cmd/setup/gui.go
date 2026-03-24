package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"time"
)

//go:embed embed/wizard.html
var wizardHTML []byte

const guiPort = "17890"

func startGUI() error {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(wizardHTML)
	})

	mux.HandleFunc("/api/info", handleInfo)
	mux.HandleFunc("/api/install", handleInstall)
	mux.HandleFunc("/api/uninstall", handleUninstall)

	addr := "127.0.0.1:" + guiPort
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		// Port in use, try random
		listener, err = net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return fmt.Errorf("cannot start GUI server: %w", err)
		}
		addr = listener.Addr().String()
	}

	url := fmt.Sprintf("http://%s", addr)
	fmt.Println()
	fmt.Println("  ✨ StarClaw Setup v" + version + " (GUI)")
	fmt.Println("  🌐 " + url)
	fmt.Println()
	fmt.Println("  Browser will open automatically.")
	fmt.Println("  If not, copy the URL above into your browser.")
	fmt.Println("  Press Ctrl+C to exit.")
	fmt.Println()

	go func() {
		time.Sleep(500 * time.Millisecond)
		openBrowser(url)
	}()

	server := &http.Server{Handler: mux}
	return server.Serve(listener)
}

func handleInfo(w http.ResponseWriter, r *http.Request) {
	var defaultDir string
	if goruntime.GOOS == "windows" {
		defaultDir = filepath.Join(os.Getenv("LOCALAPPDATA"), "StarClaw")
	} else {
		home, _ := os.UserHomeDir()
		defaultDir = filepath.Join(home, ".local", "starclaw")
	}

	homeDir, _ := os.UserHomeDir()
	sporeHome := filepath.Join(homeDir, ".spore")
	existingDir := loadInstallInfo()

	totalMB := (len(sporeBin) + len(clawPkg)) / (1024 * 1024)

	osName := goruntime.GOOS
	switch osName {
	case "windows":
		osName = "Windows"
	case "darwin":
		osName = "macOS"
	case "linux":
		osName = "Linux"
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"version":     version,
		"default_dir": defaultDir,
		"install_dir": existingDir,
		"spore_home":  sporeHome,
		"os":          osName,
		"arch":        goruntime.GOARCH,
		"total_size":  fmt.Sprintf("~%d", totalMB),
	})
}

type sseWriter struct {
	w       http.ResponseWriter
	flusher http.Flusher
}

func newSSE(w http.ResponseWriter) *sseWriter {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	f, _ := w.(http.Flusher)
	return &sseWriter{w: w, flusher: f}
}

func (s *sseWriter) send(data map[string]interface{}) {
	b, _ := json.Marshal(data)
	fmt.Fprintf(s.w, "data: %s\n\n", b)
	if s.flusher != nil {
		s.flusher.Flush()
	}
}

func (s *sseWriter) log(msg string, progress int) {
	s.send(map[string]interface{}{"log": msg, "level": "info", "progress": progress})
}

func (s *sseWriter) logOK(msg string, progress int) {
	s.send(map[string]interface{}{"log": msg, "level": "ok", "progress": progress})
}

func (s *sseWriter) logErr(msg string) {
	s.send(map[string]interface{}{"error": msg})
}

func handleInstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "POST only", 405)
		return
	}

	var req struct {
		Dir  string `json:"dir"`
		Name string `json:"name"`
		Port string `json:"port"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	instName := req.Name
	if instName == "" {
		instName = "claw"
	}

	sse := newSSE(w)

	// Recover from any panic so the SSE stream sends an error instead of silently dying
	defer func() {
		if r := recover(); r != nil {
			sse.logErr(fmt.Sprintf("Internal error: %v", r))
		}
	}()

	start := time.Now()

	installDir := req.Dir
	if installDir == "" {
		if goruntime.GOOS == "windows" {
			installDir = filepath.Join(os.Getenv("LOCALAPPDATA"), "StarClaw")
		} else {
			home, _ := os.UserHomeDir()
			installDir = filepath.Join(home, ".local", "starclaw")
		}
	}

	binDir := filepath.Join(installDir, "bin")
	var sporePath string
	if goruntime.GOOS == "windows" {
		sporePath = filepath.Join(binDir, "spore.exe")
	} else {
		sporePath = filepath.Join(binDir, "spore")
	}
	os.MkdirAll(binDir, 0755)
	saveInstallInfo(installDir)

	// Step 0: Stop existing instance if running (prevents "file in use" on Windows)
	sse.log("Stopping existing instance...", 5)
	if existingSpore := sporePath; true {
		_ = exec.Command(existingSpore, "stop", instName).Run()
	}
	if goruntime.GOOS == "windows" {
		// Direct kill: old spore may have broken SIGTERM, so taskkill claw-api.exe directly
		exec.Command("taskkill", "/IM", "claw-api.exe", "/F").Run()
		time.Sleep(2 * time.Second)
	} else {
		time.Sleep(1 * time.Second)
	}

	// Step 1: Extract Spore runtime
	sse.log(fmt.Sprintf("[1/4] Extracting Spore runtime (%d MB)...", len(sporeBin)/(1024*1024)), 10)
	if err := os.WriteFile(sporePath, sporeBin, 0755); err != nil {
		sse.logErr(fmt.Sprintf("Extract Spore failed: %v", err))
		return
	}
	sse.logOK("[1/4] Spore runtime extracted ✓", 25)

	// Step 2: Extract + install Claw package
	sse.log(fmt.Sprintf("[2/4] Installing Claw package (%d MB)...", len(clawPkg)/(1024*1024)), 35)
	tmpSpore := filepath.Join(os.TempDir(), "claw-setup.spore")
	if err := os.WriteFile(tmpSpore, clawPkg, 0644); err != nil {
		sse.logErr(fmt.Sprintf("Extract Claw failed: %v", err))
		return
	}

	installArgs := []string{"install", tmpSpore}
	if instName != "claw" {
		installArgs = append(installArgs, "--name", instName)
	}
	cmd := exec.Command(sporePath, installArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		sse.logErr(fmt.Sprintf("Install Claw failed: %v", err))
		return
	}
	os.Remove(tmpSpore)
	sse.logOK("[2/4] Claw package installed ✓", 55)

	// Step 3: Configure — use specified port or find available one
	port := req.Port
	if port == "" {
		port = findAvailablePort(8080, 8099)
	}

	homeDir, _ := os.UserHomeDir()
	sporeHome := filepath.Join(homeDir, ".spore")
	clawBase := filepath.Join(sporeHome, "installed", instName)
	clawInstallDir := resolveCurrentDir(clawBase)
	if clawInstallDir == "" {
		clawInstallDir = filepath.Join(clawBase, "v1.0.0")
	}
	os.MkdirAll(filepath.Join(clawInstallDir, "data"), 0755)

	jwtSecret := fmt.Sprintf("sc-%d-%d", time.Now().UnixNano(), os.Getpid())
	config := fmt.Sprintf("server:\n  port: %s\n\ndatabase:\n  driver: sqlite\n  sqlite_path: \"./data/claw.db\"\n\njwt:\n  secret: \"%s\"\n", port, jwtSecret)
	os.WriteFile(filepath.Join(clawInstallDir, "config.yaml"), []byte(config), 0644)

	envContent := fmt.Sprintf("GIN_MODE=release\nCLAW_DATA_DIR=./data\nCLAW_PORT=%s\nDEFAULT_PROVIDER=starai\n", port)
	os.WriteFile(filepath.Join(clawInstallDir, ".env"), []byte(envContent), 0644)

	addToPath(binDir)
	sse.logOK(fmt.Sprintf("[3/4] Configuration saved (port %s) ✓", port), 75)

	// Step 4: Start + Desktop shortcut
	sse.log(fmt.Sprintf("[4/4] Starting %s service...", instName), 85)
	startCmd := exec.Command(sporePath, "start", instName)
	startCmd.Stdout = os.Stdout
	startCmd.Stderr = os.Stderr
	startErr := startCmd.Run()

	url := fmt.Sprintf("http://localhost:%s", port)
	if port == "80" {
		url = "http://localhost"
	}

	iconPath := filepath.Join(installDir, "starclaw.ico")
	os.WriteFile(iconPath, iconData, 0644)
	createDesktopShortcut(url, sporePath, iconPath)
	registerAutoStart(sporePath, instName)

	if startErr != nil {
		sse.log(fmt.Sprintf("Warning: start failed: %v (can start manually later)", startErr), 90)
	} else {
		sse.logOK("[4/4] Claw started ✓", 90)
	}

	// Step 5: Get or initialize owner token from running Claw
	ownerToken := ""
	if startErr == nil {
		sse.log("Retrieving auth token...", 93)
		time.Sleep(2 * time.Second) // wait for Claw to fully start
		apiBase := fmt.Sprintf("http://127.0.0.1:%s/v1", port)
		// Try to get existing token first
		if resp, err := http.Get(apiBase + "/setup/token"); err == nil {
			defer resp.Body.Close()
			var result map[string]interface{}
			if json.NewDecoder(resp.Body).Decode(&result) == nil {
				if t, ok := result["owner_token"].(string); ok && t != "" {
					ownerToken = t
				}
			}
		}
		// If no token (fresh install), run setup to create one
		if ownerToken == "" {
			if resp, err := http.Post(apiBase+"/setup", "application/json", strings.NewReader("{}")); err == nil {
				defer resp.Body.Close()
				var result map[string]interface{}
				if json.NewDecoder(resp.Body).Decode(&result) == nil {
					if t, ok := result["owner_token"].(string); ok {
						ownerToken = t
					}
				}
			}
		}
		if ownerToken != "" {
			sse.logOK("Auth token retrieved ✓", 97)
		}
	}

	elapsed := time.Since(start).Round(time.Millisecond).String()
	sse.send(map[string]interface{}{
		"done":        true,
		"url":         url,
		"name":        instName,
		"owner_token": ownerToken,
		"elapsed":     elapsed,
		"progress":    100,
		"log":         fmt.Sprintf("✅ Installation complete! (%s)", elapsed),
		"level":       "ok",
	})
}

func handleUninstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "POST only", 405)
		return
	}

	sse := newSSE(w)

	installDir := loadInstallInfo()
	homeDir, _ := os.UserHomeDir()
	sporeHome := filepath.Join(homeDir, ".spore")

	if installDir == "" {
		if goruntime.GOOS == "windows" {
			installDir = filepath.Join(os.Getenv("LOCALAPPDATA"), "StarClaw")
		} else {
			installDir = filepath.Join(homeDir, ".local", "starclaw")
		}
	}

	// 1. Stop claw
	sse.log("[1/5] Stopping claw...", 10)
	sporePath := filepath.Join(installDir, "bin", "spore")
	if goruntime.GOOS == "windows" {
		sporePath += ".exe"
	}
	exec.Command(sporePath, "stop", "claw").Run()
	sse.logOK("[1/5] Stopped ✓", 20)

	// 2. Remove shortcuts
	sse.log("[2/5] Removing desktop shortcuts...", 30)
	name := appName()
	desktop := filepath.Join(homeDir, "Desktop")
	if _, err := os.Stat(desktop); os.IsNotExist(err) {
		desktop = filepath.Join(homeDir, "桌面")
	}
	for _, ext := range []string{".url", " - Start.bat", ".command", ".desktop"} {
		os.Remove(filepath.Join(desktop, name+ext))
	}
	sse.logOK("[2/5] Shortcuts removed ✓", 40)

	// 3. Remove spore data
	sse.log("[3/5] Removing Spore data...", 55)
	os.RemoveAll(sporeHome)
	sse.logOK("[3/5] Spore data removed ✓", 65)

	// 4. Remove install dir
	sse.log("[4/5] Removing install directory...", 75)
	os.RemoveAll(installDir)
	sse.logOK("[4/5] Install directory removed ✓", 85)

	// 5. Clean PATH
	sse.log("[5/5] Cleaning PATH...", 90)
	binDir := filepath.Join(installDir, "bin")
	if goruntime.GOOS == "windows" {
		exec.Command("powershell", "-NoProfile", "-Command",
			fmt.Sprintf(`$p = [Environment]::GetEnvironmentVariable('Path','User'); $p2 = ($p -split ';' | Where-Object { $_ -ne '%s' }) -join ';'; [Environment]::SetEnvironmentVariable('Path', $p2, 'User')`, binDir)).Run()
	} else {
		for _, rc := range []string{".bashrc", ".zshrc"} {
			rcPath := filepath.Join(homeDir, rc)
			data, err := os.ReadFile(rcPath)
			if err != nil {
				continue
			}
			lines := strings.Split(string(data), "\n")
			var cleaned []string
			for _, line := range lines {
				if !strings.Contains(line, binDir) {
					cleaned = append(cleaned, line)
				}
			}
			os.WriteFile(rcPath, []byte(strings.Join(cleaned, "\n")), 0644)
		}
	}
	sse.logOK("[5/5] PATH cleaned ✓", 95)

	os.Remove(installInfoPath())

	sse.send(map[string]interface{}{
		"done":     true,
		"progress": 100,
		"log":      "✅ Uninstall complete",
		"level":    "ok",
	})
}
