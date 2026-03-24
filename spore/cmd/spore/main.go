package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"time"

	"starclaw.net/spore/pkg/archive"
	"starclaw.net/spore/pkg/platform"
	"starclaw.net/spore/pkg/runtime"
)

const version = "2026.0323.1809"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	info := platform.Detect()
	mgr := runtime.NewManager(info.SporeHome)

	switch os.Args[1] {
	case "install":
		cmdInstall(mgr)
	case "run":
		cmdRun(mgr)
	case "run-inline":
		cmdRunInline()
	case "start":
		cmdStart(mgr)
	case "stop":
		cmdStop(mgr)
	case "restart":
		cmdRestart(mgr)
	case "status":
		cmdStatus(mgr)
	case "list":
		cmdList(mgr)
	case "logs":
		cmdLogs(mgr)
	case "autostart":
		cmdAutostart(mgr)
	case "uninstall":
		cmdUninstall(mgr)
	case "info":
		cmdInfo(mgr)
	case "token", "get-token":
		cmdToken(mgr)
	case "reset-token":
		cmdResetToken(mgr)
	case "reset-password":
		cmdResetPassword(mgr)
	case "update":
		cmdUpdate()
	case "version":
		fmt.Printf("spore v%s (%s/%s)\n", version, info.OS, info.Arch)
	case "platform":
		fmt.Println(info)
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func cmdInstall(mgr *runtime.Manager) {
	if len(os.Args) < 3 {
		fatal("usage: spore install <path-to-.spore-or-dir> [--name <name>] [--port <port>]")
	}
	src := os.Args[2]

	// Parse flags for multi-instance support
	customName := ""
	customPort := ""
	for i := 3; i < len(os.Args); i++ {
		if os.Args[i] == "--name" && i+1 < len(os.Args) {
			customName = os.Args[i+1]
			i++
		}
		if os.Args[i] == "--port" && i+1 < len(os.Args) {
			customPort = os.Args[i+1]
			i++
		}
	}

	// Check if it's a .spore archive or a directory
	fi, err := os.Stat(src)
	if err != nil {
		fatal("cannot access %s: %v", src, err)
	}

	installDir := src
	if !fi.IsDir() && strings.HasSuffix(src, ".spore") {
		// Extract archive to temp dir, then install
		tmpDir, err := os.MkdirTemp("", "spore-extract-*")
		if err != nil {
			fatal("create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		fmt.Printf("📦 Extracting %s...\n", filepath.Base(src))
		checksum, err := archive.Unpack(src, tmpDir)
		if err != nil {
			fatal("extract: %v", err)
		}
		fmt.Printf("   Checksum: %s\n", checksum)
		installDir = tmpDir
	}

	inst, err := mgr.InstallFromDir(installDir, customName)
	if err != nil {
		fatal("install: %v", err)
	}

	// Auto-generate config if --port is specified (multi-instance)
	if customPort != "" {
		jwtSecret := fmt.Sprintf("sc-%d-%d", time.Now().UnixNano(), os.Getpid())
		config := fmt.Sprintf("server:\n  port: %s\n\ndatabase:\n  driver: sqlite\n  sqlite_path: \"./data/claw.db\"\n\njwt:\n  secret: \"%s\"\n", customPort, jwtSecret)
		os.MkdirAll(filepath.Join(inst.InstallDir, "data"), 0755)
		os.WriteFile(filepath.Join(inst.InstallDir, "config.yaml"), []byte(config), 0644)
		envContent := fmt.Sprintf("GIN_MODE=release\nCLAW_DATA_DIR=./data\nCLAW_PORT=%s\nDEFAULT_PROVIDER=starai\n", customPort)
		os.WriteFile(filepath.Join(inst.InstallDir, ".env"), []byte(envContent), 0644)
		fmt.Printf("   Port: %s\n", customPort)
	}

	fmt.Printf(" Installed %s v%s\n", inst.Name, inst.Version)
	fmt.Printf("   Location: %s\n", inst.InstallDir)
	fmt.Printf("   Start with: spore start %s\n", inst.Name)
}

func cmdRun(mgr *runtime.Manager) {
	if len(os.Args) < 3 {
		fatal("usage: spore run <name>")
	}
	// Run in foreground — delegate to Start for now but could exec directly
	fmt.Println("🔄 Use 'spore start' for background mode, or run the binary directly for foreground.")
	inst, err := mgr.Get(os.Args[2])
	if err != nil {
		fatal("%v", err)
	}
	binPath := filepath.Join(inst.InstallDir, inst.Manifest.Binary)
	fmt.Printf("Binary: %s %s\n", binPath, strings.Join(inst.Manifest.Args, " "))
}

func cmdRunInline() {
	if len(os.Args) < 3 {
		fatal("usage: spore run-inline <dir> [--env KEY=VALUE ...]")
	}
	dir := os.Args[2]

	var envOverrides []string
	for i := 3; i < len(os.Args); i++ {
		if os.Args[i] == "--env" && i+1 < len(os.Args) {
			envOverrides = append(envOverrides, os.Args[i+1])
			i++
		}
	}

	if err := runtime.RunInline(dir, envOverrides); err != nil {
		fatal("run-inline: %v", err)
	}
}

func cmdStart(mgr *runtime.Manager) {
	if len(os.Args) < 3 {
		fatal("usage: spore start <name> [--env KEY=VALUE ...]")
	}
	name := os.Args[2]

	// Parse --env flags for runtime env overrides
	var envOverrides []string
	for i := 3; i < len(os.Args); i++ {
		if os.Args[i] == "--env" && i+1 < len(os.Args) {
			envOverrides = append(envOverrides, os.Args[i+1])
			i++
		}
	}

	fmt.Printf("🚀 Starting %s...\n", name)
	if err := mgr.StartWithEnv(name, envOverrides); err != nil {
		fatal("start: %v", err)
	}
	fmt.Printf("✅ %s started\n", name)
}

func cmdStop(mgr *runtime.Manager) {
	if len(os.Args) < 3 {
		fatal("usage: spore stop <name>")
	}
	name := os.Args[2]
	fmt.Printf("⏹️  Stopping %s...\n", name)
	if err := mgr.Stop(name); err != nil {
		fatal("stop: %v", err)
	}
	fmt.Printf("✅ %s stopped\n", name)
}

func cmdRestart(mgr *runtime.Manager) {
	if len(os.Args) < 3 {
		fatal("usage: spore restart <name>")
	}
	name := os.Args[2]
	fmt.Printf("🔄 Restarting %s...\n", name)
	mgr.Stop(name) // ignore error if not running
	if err := mgr.Start(name); err != nil {
		fatal("start: %v", err)
	}
	fmt.Printf("✅ %s restarted\n", name)
}

func cmdStatus(mgr *runtime.Manager) {
	instances, err := mgr.List()
	if err != nil {
		fatal("list: %v", err)
	}
	if len(instances) == 0 {
		fmt.Println("No spores installed.")
		return
	}

	fmt.Printf("%-20s %-12s %-10s %s\n", "NAME", "VERSION", "STATUS", "LOCATION")
	fmt.Println(strings.Repeat("─", 70))
	for _, inst := range instances {
		statusIcon := "⏹"
		switch inst.Status {
		case "running":
			statusIcon = "🟢"
		case "error":
			statusIcon = "🔴"
		}
		fmt.Printf("%-20s %-12s %s %-8s %s\n", inst.Name, inst.Version, statusIcon, inst.Status, inst.InstallDir)
	}
}

func cmdList(mgr *runtime.Manager) {
	cmdStatus(mgr)
}

func cmdLogs(mgr *runtime.Manager) {
	if len(os.Args) < 3 {
		fatal("usage: spore logs <name>")
	}
	logPath := mgr.LogPath(os.Args[2])
	data, err := os.ReadFile(logPath)
	if err != nil {
		fatal("read logs: %v", err)
	}
	// Show last 100 lines
	lines := strings.Split(string(data), "\n")
	start := 0
	if len(lines) > 100 {
		start = len(lines) - 100
	}
	for _, line := range lines[start:] {
		fmt.Println(line)
	}
}

func cmdInfo(mgr *runtime.Manager) {
	if len(os.Args) < 3 {
		fatal("usage: spore info <name>")
	}
	inst, err := mgr.Get(os.Args[2])
	if err != nil {
		fatal("%v", err)
	}
	fmt.Printf("Name:        %s\n", inst.Name)
	fmt.Printf("Version:     %s\n", inst.Version)
	fmt.Printf("Status:      %s\n", inst.Status)
	fmt.Printf("Platform:    %s/%s\n", inst.Manifest.Platform.OS, inst.Manifest.Platform.Arch)
	fmt.Printf("Binary:      %s\n", inst.Manifest.Binary)
	fmt.Printf("Install Dir: %s\n", inst.InstallDir)
	fmt.Printf("Data Dir:    %s\n", inst.DataDir)
	fmt.Printf("Log Dir:     %s\n", inst.LogDir)
	if inst.Manifest.Health.Endpoint != "" {
		healthy, _ := mgr.HealthCheck(inst.Name)
		status := "❌ unhealthy"
		if healthy {
			status = "✅ healthy"
		}
		fmt.Printf("Health:      %s (%s)\n", status, inst.Manifest.Health.Endpoint)
	}
	if len(inst.Manifest.Network.Ports) > 0 {
		ports := make([]string, len(inst.Manifest.Network.Ports))
		for i, p := range inst.Manifest.Network.Ports {
			ports[i] = fmt.Sprintf("%d/%s", p.Port, p.Protocol)
		}
		fmt.Printf("Ports:       %s\n", strings.Join(ports, ", "))
	}
}

func cmdUninstall(mgr *runtime.Manager) {
	if len(os.Args) < 3 {
		fatal("usage: spore uninstall <name>")
	}
	name := os.Args[2]
	fmt.Printf("🗑️  Uninstalling %s...\n", name)
	if err := mgr.Uninstall(name); err != nil {
		fatal("uninstall: %v", err)
	}
	fmt.Printf("✅ %s uninstalled\n", name)
}

func cmdAutostart(mgr *runtime.Manager) {
	if len(os.Args) < 3 {
		fatal("usage: spore autostart <enable|disable|status> [name]")
	}
	action := os.Args[2]
	name := "claw"
	if len(os.Args) >= 4 {
		name = os.Args[3]
	}

	switch action {
	case "enable":
		if err := mgr.EnableAutostart(name); err != nil {
			fatal("enable autostart: %v", err)
		}
		fmt.Printf("✅ %s 已设为开机自启动\n", name)
	case "disable":
		if err := mgr.DisableAutostart(name); err != nil {
			fatal("disable autostart: %v", err)
		}
		fmt.Printf("✅ %s 已取消开机自启动\n", name)
	case "status":
		if mgr.IsAutostartEnabled(name) {
			fmt.Printf("🟢 %s 开机自启动: 已启用\n", name)
		} else {
			fmt.Printf("⚪ %s 开机自启动: 未启用\n", name)
		}
	default:
		fatal("unknown autostart action: %s (use enable/disable/status)", action)
	}
}

func cmdToken(mgr *runtime.Manager) {
	nameArg := ""
	if len(os.Args) >= 3 {
		nameArg = os.Args[2]
	}
	name, port := resolveClawPort(mgr, nameArg)

	apiBase := fmt.Sprintf("http://127.0.0.1:%s/v1", port)
	resp, err := http.Get(apiBase + "/setup/token")
	if err != nil {
		fatal("cannot connect to %s: %v (is it running?)", name, err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fatal("parse response: %v", err)
	}

	if token, ok := result["owner_token"].(string); ok && token != "" {
		fmt.Printf("Owner Token: %s\n", token)
		fmt.Printf("Login URL:   http://localhost:%s/login?token=%s\n", port, token)
	} else {
		fmt.Println("No owner token found. Run setup first by opening the web UI.")
	}
}

// resolveClawPort reads the port for a named instance from .env or defaults to 8080.
func resolveClawPort(mgr *runtime.Manager, nameOverride string) (string, string) {
	name := "claw"
	if nameOverride != "" {
		name = nameOverride
	}
	inst, err := mgr.Get(name)
	if err != nil {
		fatal("%v", err)
	}
	port := "8080"
	if envData, err := os.ReadFile(filepath.Join(inst.InstallDir, ".env")); err == nil {
		for _, line := range strings.Split(string(envData), "\n") {
			if strings.HasPrefix(line, "CLAW_PORT=") {
				port = strings.TrimPrefix(line, "CLAW_PORT=")
				port = strings.TrimSpace(port)
				break
			}
		}
	}
	return name, port
}

func cmdResetToken(mgr *runtime.Manager) {
	nameArg := ""
	if len(os.Args) >= 3 {
		nameArg = os.Args[2]
	}
	name, port := resolveClawPort(mgr, nameArg)

	apiBase := fmt.Sprintf("http://127.0.0.1:%s/v1", port)
	resp, err := http.Post(apiBase+"/setup/reset-token", "application/json", strings.NewReader("{}"))
	if err != nil {
		fatal("cannot connect to %s: %v (is it running?)", name, err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fatal("parse response: %v", err)
	}

	if token, ok := result["owner_token"].(string); ok && token != "" {
		fmt.Printf("New Owner Token: %s\n", token)
		fmt.Printf("Login URL:       http://localhost:%s/login?token=%s\n", port, token)
		fmt.Println("Previous token is now invalid.")
	} else if errMsg, ok := result["error"].(string); ok {
		fatal("%s", errMsg)
	} else {
		fatal("unexpected response")
	}
}

func cmdResetPassword(mgr *runtime.Manager) {
	password := ""
	nameArg := ""
	for i := 2; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "--password":
			if i+1 < len(os.Args) {
				password = os.Args[i+1]
				i++
			}
		default:
			if nameArg == "" {
				nameArg = os.Args[i]
			}
		}
	}
	if password == "" {
		fmt.Println("Usage: spore reset-password [name] --password <new-password>")
		os.Exit(1)
	}
	if len(password) < 6 {
		fatal("password must be at least 6 characters")
	}

	name, port := resolveClawPort(mgr, nameArg)
	apiBase := fmt.Sprintf("http://127.0.0.1:%s/v1", port)

	body := fmt.Sprintf(`{"password":"%s"}`, password)
	resp, err := http.Post(apiBase+"/setup/reset-password", "application/json", strings.NewReader(body))
	if err != nil {
		fatal("cannot connect to %s: %v (is it running?)", name, err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fatal("parse response: %v", err)
	}

	if msg, ok := result["message"].(string); ok {
		fmt.Println(msg)
	} else if errMsg, ok := result["error"].(string); ok {
		fatal("%s", errMsg)
	} else {
		fmt.Println("Password reset successfully.")
	}
}

func cmdUpdate() {
	fmt.Println("🔍 Checking for updates...")

	// 1. Fetch latest version
	resp, err := http.Get("https://nydus.starclaw.net/releases/latest")
	if err != nil {
		fatal("check update: %v", err)
	}
	defer resp.Body.Close()

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		fatal("parse release: %v", err)
	}

	latestVer := strings.TrimPrefix(release.TagName, "v")
	fmt.Printf("   Current: v%s\n", version)
	fmt.Printf("   Latest:  v%s\n", latestVer)

	if latestVer <= version {
		fmt.Println("✅ Already up to date.")
		return
	}

	// 2. Determine download URL
	ext := ""
	if goruntime.GOOS == "windows" {
		ext = ".exe"
	}
	binName := fmt.Sprintf("spore-%s-%s%s", goruntime.GOOS, goruntime.GOARCH, ext)
	downloadURL := "https://nydus.starclaw.net/spore/releases/" + binName

	fmt.Printf("⬇️  Downloading %s...\n", binName)

	dlResp, err := http.Get(downloadURL)
	if err != nil {
		fatal("download: %v", err)
	}
	defer dlResp.Body.Close()

	if dlResp.StatusCode != http.StatusOK {
		fatal("download failed: HTTP %d", dlResp.StatusCode)
	}

	// 3. Write to temp file
	exePath, err := os.Executable()
	if err != nil {
		fatal("locate self: %v", err)
	}
	exePath, _ = filepath.EvalSymlinks(exePath)

	tmpPath := exePath + ".new"
	tmpFile, err := os.Create(tmpPath)
	if err != nil {
		fatal("create temp: %v", err)
	}

	written, err := io.Copy(tmpFile, dlResp.Body)
	tmpFile.Close()
	if err != nil {
		os.Remove(tmpPath)
		fatal("write: %v", err)
	}
	fmt.Printf("   Downloaded %.1f MB\n", float64(written)/1024/1024)

	// 4. Make executable (unix)
	os.Chmod(tmpPath, 0755)

	// 5. Replace binary (Windows: rename old first)
	oldPath := exePath + ".old"
	os.Remove(oldPath) // clean up previous .old

	if goruntime.GOOS == "windows" {
		// Windows can't overwrite running exe, rename first
		if err := os.Rename(exePath, oldPath); err != nil {
			os.Remove(tmpPath)
			fatal("rename old: %v", err)
		}
		if err := os.Rename(tmpPath, exePath); err != nil {
			os.Rename(oldPath, exePath) // rollback
			fatal("replace: %v", err)
		}
	} else {
		if err := os.Rename(tmpPath, exePath); err != nil {
			os.Remove(tmpPath)
			fatal("replace: %v", err)
		}
	}

	fmt.Printf("✅ Updated spore: v%s → v%s\n", version, latestVer)
	fmt.Println("   Run 'spore update-spores' to also update installed spores.")
}

func printUsage() {
	fmt.Printf(`Spore v%s — StarClaw Ultra-Lightweight Deployment Runtime

Usage: spore <command> [args]

Lifecycle:
  install <path>    Install from .spore package or directory
  start <name>      Start a spore (background)
  stop <name>       Stop a running spore
  restart <name>    Restart a spore
  status            Show status of all installed spores
  list              List installed spores
  info <name>       Show detailed info about a spore
  logs <name>       View spore logs
  uninstall <name>  Remove an installed spore
  autostart <enable|disable|status> [name]  Manage boot autostart

Auth (same as claw-api CLI):
  token [name]          Show owner token for a running instance
  reset-token [name]    Regenerate owner token (invalidates old one)
  reset-password [name] --password <pw>  Reset owner password

Other:
  update            Check and apply spore runtime updates
  run-inline <dir>  Run in foreground (Docker/container mode)
  version           Show spore version
  platform          Show detected platform info
  help              Show this help

`, version)
}

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
	os.Exit(1)
}
