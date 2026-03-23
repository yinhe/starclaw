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
		fatal("usage: spore install <path-to-.spore-or-dir> [--name <instance-name>]")
	}
	src := os.Args[2]

	// Parse --name flag for multi-instance support
	customName := ""
	for i := 3; i < len(os.Args); i++ {
		if os.Args[i] == "--name" && i+1 < len(os.Args) {
			customName = os.Args[i+1]
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

	fmt.Printf("✅ Installed %s v%s\n", inst.Name, inst.Version)
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

Commands:
  install <path>    Install from .spore package or directory
  run-inline <dir>  Run in foreground (Docker/container mode)
  start <name>      Start a spore (background)
  stop <name>       Stop a running spore
  restart <name>    Restart a spore
  status            Show status of all installed spores
  list              List installed spores
  info <name>       Show detailed info about a spore
  logs <name>       View spore logs
  autostart <enable|disable|status> [name]  Manage boot autostart
  update            Check and apply spore runtime updates
  uninstall <name>  Remove an installed spore
  version           Show spore version
  platform          Show detected platform info
  help              Show this help

`, version)
}

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
	os.Exit(1)
}
