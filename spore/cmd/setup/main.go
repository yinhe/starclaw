package main

import (
	"bufio"
	_ "embed"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"time"
)

const version = "2026.0323.1809"

//go:embed embed/spore_bin
var sporeBin []byte

//go:embed embed/claw.spore
var clawPkg []byte

//go:embed embed/icon.ico
var iconData []byte

func main() {
	// On macOS, catch panics so terminal doesn't flash and close
	if goruntime.GOOS == "darwin" {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("\n  ❌ Unexpected error: %v\n\n", r)
				fmt.Println("  Press Enter to close...")
				bufio.NewReader(os.Stdin).ReadBytes('\n')
			}
		}()
	}

	cliMode := false
	uninstallMode := false
	for _, arg := range os.Args[1:] {
		switch arg {
		case "--cli", "-cli":
			cliMode = true
		case "--uninstall", "-uninstall", "uninstall":
			uninstallMode = true
		}
	}

	// GUI mode (default) — open browser wizard
	if !cliMode && !uninstallMode {
		if err := startGUI(); err != nil {
			fmt.Printf("GUI failed: %v — falling back to CLI\n", err)
			if goruntime.GOOS == "darwin" {
				fmt.Println("\n  If macOS blocked the app, go to System Settings → Privacy & Security → 'Open Anyway'")
				fmt.Println("  Press Enter to close...")
				bufio.NewReader(os.Stdin).ReadBytes('\n')
			}
		} else {
			return
		}
	}

	// CLI uninstall
	if uninstallMode {
		cls()
		uninstall()
		return
	}

	// CLI install (legacy)
	cls()
	cyan := "\033[36m"
	green := "\033[32m"
	yellow := "\033[33m"
	white := "\033[97m"
	reset := "\033[0m"
	fmt.Println()
	fmt.Println(cyan + "  ╔════════════════════════════════════════════╗" + reset)
	fmt.Println(cyan + "  ║                                            ║" + reset)
	fmt.Println(cyan + "  ║   " + white + "StarClaw" + cyan + " — AI Agent Platform Setup     ║" + reset)
	fmt.Println(cyan + "  ║   v" + version + "                                    ║" + reset)
	fmt.Println(cyan + "  ║                                            ║" + reset)
	fmt.Println(cyan + "  ╚════════════════════════════════════════════╝" + reset)
	fmt.Println()

	start := time.Now()

	// Setup default paths
	var defaultDir string
	if goruntime.GOOS == "windows" {
		defaultDir = filepath.Join(os.Getenv("LOCALAPPDATA"), "StarClaw")
	} else {
		home, _ := os.UserHomeDir()
		defaultDir = filepath.Join(home, ".local", "starclaw")
	}

	// Ask user for install directory
	fmt.Printf("  安装目录 (Install directory)\n")
	fmt.Printf("  \u9ed8\u8ba4: "+cyan+"%s"+reset+"\n", defaultDir)
	fmt.Printf("  输入新路径或直接回车使用默认: ")
	reader := bufio.NewReader(os.Stdin)
	inputDir, _ := reader.ReadString('\n')
	inputDir = strings.TrimSpace(inputDir)

	installDir := defaultDir
	if inputDir != "" {
		installDir = inputDir
	}
	fmt.Println()

	var binDir, sporePath string
	binDir = filepath.Join(installDir, "bin")
	if goruntime.GOOS == "windows" {
		sporePath = filepath.Join(binDir, "spore.exe")
	} else {
		sporePath = filepath.Join(binDir, "spore")
	}
	os.MkdirAll(binDir, 0755)

	// Save install path for uninstall
	saveInstallInfo(installDir)

	// Step 1: Extract embedded Spore runtime (instant — no download)
	fmt.Printf(green+"  [1/4]"+reset+" Extracting Spore runtime (%d MB)...", len(sporeBin)/(1024*1024))
	if err := os.WriteFile(sporePath, sporeBin, 0755); err != nil {
		fail("Extract Spore failed: %v", err)
	}
	fmt.Println(" ✓")

	// Step 2: Extract + install Claw package (instant)
	fmt.Printf(green+"  [2/4]"+reset+" Extracting Claw package (%d MB)...", len(clawPkg)/(1024*1024))
	tmpSpore := filepath.Join(os.TempDir(), "claw-setup.spore")
	if err := os.WriteFile(tmpSpore, clawPkg, 0644); err != nil {
		fail("Extract Claw failed: %v", err)
	}
	fmt.Println(" ✓")

	fmt.Print(green + "  [2/4]" + reset + " Installing Claw...")
	cmd := exec.Command(sporePath, "install", tmpSpore)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fail("Install failed: %v", err)
	}
	os.Remove(tmpSpore)
	fmt.Println(" ✓")

	// Step 3: Auto-configure — find available port (Vite-style auto-increment)
	port := findAvailablePort(8080, 8099)
	fmt.Printf("  Using port %s\n", port)

	homeDir, _ := os.UserHomeDir()
	sporeHome := filepath.Join(homeDir, ".spore")

	// Find the actual claw install dir (where binary lives, where claw reads config)
	clawBase := filepath.Join(sporeHome, "installed", "claw")
	clawInstallDir := resolveCurrentDir(clawBase)
	if clawInstallDir == "" {
		clawInstallDir = filepath.Join(clawBase, "v1.0.0")
	}
	// Also ensure data dir exists inside install dir
	os.MkdirAll(filepath.Join(clawInstallDir, "data"), 0755)

	jwtSecret := fmt.Sprintf("sc-%d-%d", time.Now().UnixNano(), os.Getpid())
	config := fmt.Sprintf(`server:
  port: %s

database:
  driver: sqlite
  sqlite_path: "./data/claw.db"

jwt:
  secret: "%s"
`, port, jwtSecret)
	// Write config.yaml directly in the install dir (where viper searches ".")
	os.WriteFile(filepath.Join(clawInstallDir, "config.yaml"), []byte(config), 0644)

	envContent := fmt.Sprintf("GIN_MODE=release\nCLAW_DATA_DIR=./data\nCLAW_PORT=%s\nDEFAULT_PROVIDER=starai\n", port)
	os.WriteFile(filepath.Join(clawInstallDir, ".env"), []byte(envContent), 0644)

	addToPath(binDir)
	fmt.Printf(green+"  [3/4]"+reset+" Configuration saved (port %s, default: StarAI) ✓\n", port)

	// Step 4: Start + Desktop shortcut
	fmt.Print(green + "  [4/4]" + reset + " Starting Claw...")
	startCmd := exec.Command(sporePath, "start", "claw")
	startCmd.Stdout = os.Stdout
	startCmd.Stderr = os.Stderr
	if err := startCmd.Run(); err != nil {
		fmt.Printf("\n"+yellow+"  Warning: Start failed: %v\n"+reset, err)
		fmt.Println(yellow + "  You can start manually later: spore start claw" + reset)
	} else {
		fmt.Println(" ✓")
	}

	url := fmt.Sprintf("http://localhost:%s", port)
	if port == "80" {
		url = "http://localhost"
	}
	// Save icon to install dir
	iconPath := filepath.Join(installDir, "starclaw.ico")
	os.WriteFile(iconPath, iconData, 0644)

	createDesktopShortcut(url, sporePath, iconPath)
	registerAutoStart(sporePath, "claw")

	elapsed := time.Since(start)

	fmt.Println()
	fmt.Println(green + "  ══════════════════════════════════════════════" + reset)
	fmt.Printf(green+"  ✅ Installation Complete! (%s)"+reset+"\n", elapsed.Round(time.Millisecond))
	fmt.Println(green + "  ══════════════════════════════════════════════" + reset)
	fmt.Println()
	fmt.Printf("  🌐 Browser: "+cyan+"%s"+reset+"\n", url)
	fmt.Println("  🖥️  Desktop: " + appName())
	fmt.Println("  🤖 Default AI: " + cyan + "StarAI" + reset)
	fmt.Println()
	fmt.Println("  Commands:")
	fmt.Println("    spore status        — Check status")
	fmt.Println("    spore stop claw     — Stop")
	fmt.Println("    spore start claw    — Start")
	fmt.Printf("  Uninstall:\n")
	fmt.Printf("    %s --uninstall\n", os.Args[0])
	fmt.Println()

	openBrowser(url)

	fmt.Println("  Press Enter to close...")
	bufio.NewReader(os.Stdin).ReadString('\n')
}

func resolveCurrentDir(installBase string) string {
	currentLink := filepath.Join(installBase, "current")
	// Try symlink
	target, err := os.Readlink(currentLink)
	if err == nil {
		if filepath.IsAbs(target) {
			return target
		}
		return filepath.Join(installBase, target)
	}
	// Try marker file (Windows fallback)
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

func isPortAvailable(port string) bool {
	ln, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return false
	}
	ln.Close()
	return true
}

// findAvailablePort tries ports from startPort to endPort, returns first available (Vite-style).
func findAvailablePort(startPort, endPort int) string {
	for p := startPort; p <= endPort; p++ {
		s := fmt.Sprintf("%d", p)
		if isPortAvailable(s) {
			return s
		}
	}
	return fmt.Sprintf("%d", startPort)
}

// appName returns "星爪" for Chinese locale, "StarClaw" otherwise
func appName() string {
	if isChinese() {
		return "星爪"
	}
	return "StarClaw"
}

func isChinese() bool {
	// Check common locale env vars
	for _, key := range []string{"LANG", "LC_ALL", "LANGUAGE"} {
		v := os.Getenv(key)
		if strings.HasPrefix(strings.ToLower(v), "zh") {
			return true
		}
	}
	// Windows: check via PowerShell
	if goruntime.GOOS == "windows" {
		out, err := exec.Command("powershell", "-NoProfile", "-Command",
			"(Get-Culture).TwoLetterISOLanguageName").Output()
		if err == nil && strings.TrimSpace(string(out)) == "zh" {
			return true
		}
	}
	return false
}

func createDesktopShortcut(url, sporePath, iconPath string) {
	name := appName()
	homeDir, _ := os.UserHomeDir()
	desktop := filepath.Join(homeDir, "Desktop")
	if _, err := os.Stat(desktop); os.IsNotExist(err) {
		desktop = filepath.Join(homeDir, "桌面")
		if _, err := os.Stat(desktop); os.IsNotExist(err) {
			return
		}
	}

	switch goruntime.GOOS {
	case "windows":
		// .url shortcut with custom icon
		content := fmt.Sprintf("[InternetShortcut]\nURL=%s\nIconFile=%s\nIconIndex=0\n", url, iconPath)
		os.WriteFile(filepath.Join(desktop, name+".url"), []byte(content), 0644)
		// .bat to start service + open browser
		batContent := fmt.Sprintf("@echo off\r\nstart \"\" \"%s\" start claw 2>nul\r\ntimeout /t 2 /nobreak >nul\r\nstart %s\r\n", sporePath, url)
		os.WriteFile(filepath.Join(desktop, name+" - Start.bat"), []byte(batContent), 0644)
	case "darwin":
		content := fmt.Sprintf("#!/bin/sh\n%s start claw 2>/dev/null\nsleep 1\nopen \"%s\"\n", sporePath, url)
		os.WriteFile(filepath.Join(desktop, name+".command"), []byte(content), 0755)
	default:
		content := fmt.Sprintf("[Desktop Entry]\nType=Application\nName=%s\nComment=AI Agent Platform\nIcon=%s\nExec=sh -c '%s start claw; sleep 1; xdg-open %s'\nTerminal=false\nCategories=Development;\n", name, iconPath, sporePath, url)
		os.WriteFile(filepath.Join(desktop, name+".desktop"), []byte(content), 0755)
	}
}

func registerAutoStart(sporePath, instName string) {
	// Delegate to spore autostart — single source of truth for all platforms
	exec.Command(sporePath, "autostart", "enable", instName).Run()
}

func addToPath(dir string) {
	if goruntime.GOOS == "windows" {
		exec.Command("powershell", "-NoProfile", "-Command",
			fmt.Sprintf(`$p = [Environment]::GetEnvironmentVariable('Path','User'); if ($p -notlike '*%s*') { [Environment]::SetEnvironmentVariable('Path', "$p;%s", 'User') }`, dir, dir)).Run()
	} else {
		shellRC := filepath.Join(os.Getenv("HOME"), ".bashrc")
		if _, err := os.Stat(filepath.Join(os.Getenv("HOME"), ".zshrc")); err == nil {
			shellRC = filepath.Join(os.Getenv("HOME"), ".zshrc")
		}
		line := fmt.Sprintf("\nexport PATH=\"%s:$PATH\"\n", dir)
		f, err := os.OpenFile(shellRC, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err == nil {
			f.WriteString(line)
			f.Close()
		}
	}
}

func openBrowser(url string) {
	switch goruntime.GOOS {
	case "windows":
		exec.Command("cmd", "/c", "start", url).Start()
	case "darwin":
		exec.Command("open", url).Start()
	default:
		exec.Command("xdg-open", url).Start()
	}
}

func cls() {
	if goruntime.GOOS == "windows" {
		cmd := exec.Command("cmd", "/c", "cls")
		cmd.Stdout = os.Stdout
		cmd.Run()
		exec.Command("cmd", "/c", "chcp 65001 >nul 2>&1").Run()
	} else {
		fmt.Print("\033[H\033[2J")
	}
}

func fail(format string, args ...interface{}) {
	fmt.Printf("\n  ❌ Error: "+format+"\n", args...)
	fmt.Println("\n  Press Enter to close...")
	bufio.NewReader(os.Stdin).ReadString('\n')
	os.Exit(1)
}

// installInfoPath returns the path to the install info file.
func installInfoPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".starclaw_install")
}

// saveInstallInfo saves the install directory for later uninstall.
func saveInstallInfo(installDir string) {
	os.WriteFile(installInfoPath(), []byte(installDir), 0644)
}

// loadInstallInfo reads the saved install directory.
func loadInstallInfo() string {
	data, err := os.ReadFile(installInfoPath())
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func uninstall() {
	red := "\033[31m"
	cyan := "\033[36m"
	yellow := "\033[33m"
	green := "\033[32m"
	reset := "\033[0m"

	fmt.Println()
	fmt.Println(red + "  ╔════════════════════════════════════════════╗" + reset)
	fmt.Println(red + "  ║     StarClaw — Uninstall                  ║" + reset)
	fmt.Println(red + "  ╚════════════════════════════════════════════╝" + reset)
	fmt.Println()

	installDir := loadInstallInfo()
	homeDir, _ := os.UserHomeDir()
	sporeHome := filepath.Join(homeDir, ".spore")

	if installDir == "" {
		// Guess default
		if goruntime.GOOS == "windows" {
			installDir = filepath.Join(os.Getenv("LOCALAPPDATA"), "StarClaw")
		} else {
			installDir = filepath.Join(homeDir, ".local", "starclaw")
		}
	}

	fmt.Println("  将删除以下内容 (The following will be removed):")
	fmt.Println()
	fmt.Printf("    "+cyan+"1."+reset+" 安装目录:   %s\n", installDir)
	fmt.Printf("    "+cyan+"2."+reset+" Spore 数据: %s\n", sporeHome)
	fmt.Println("    " + cyan + "3." + reset + " 桌面快捷方式")
	fmt.Println("    " + cyan + "4." + reset + " PATH 环境变量条目")
	fmt.Println()
	fmt.Print(yellow + "  确认卸载？(y/N): " + reset)

	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))
	if answer != "y" && answer != "yes" {
		fmt.Println("\n  取消卸载。")
		fmt.Println("\n  Press Enter to close...")
		reader.ReadString('\n')
		return
	}
	fmt.Println()

	// 1. Stop claw
	fmt.Print("  [1/5] Stopping claw...")
	sporePath := filepath.Join(installDir, "bin", "spore")
	if goruntime.GOOS == "windows" {
		sporePath += ".exe"
	}
	exec.Command(sporePath, "stop", "claw").Run()
	fmt.Println(" ✓")

	// 2. Remove desktop shortcuts
	fmt.Print("  [2/5] Removing desktop shortcuts...")
	name := appName()
	desktop := filepath.Join(homeDir, "Desktop")
	if _, err := os.Stat(desktop); os.IsNotExist(err) {
		desktop = filepath.Join(homeDir, "桌面")
	}
	for _, ext := range []string{".url", " - Start.bat", ".command", ".desktop"} {
		os.Remove(filepath.Join(desktop, name+ext))
	}
	fmt.Println(" ✓")

	// 3. Remove spore data
	fmt.Print("  [3/5] Removing Spore data...")
	os.RemoveAll(sporeHome)
	fmt.Println(" ✓")

	// 4. Remove install directory
	fmt.Print("  [4/5] Removing install directory...")
	os.RemoveAll(installDir)
	fmt.Println(" ✓")

	// 5. Clean PATH
	fmt.Print("  [5/5] Cleaning PATH...")
	binDir := filepath.Join(installDir, "bin")
	if goruntime.GOOS == "windows" {
		exec.Command("powershell", "-NoProfile", "-Command",
			fmt.Sprintf(`$p = [Environment]::GetEnvironmentVariable('Path','User'); $p2 = ($p -split ';' | Where-Object { $_ -ne '%s' }) -join ';'; [Environment]::SetEnvironmentVariable('Path', $p2, 'User')`, binDir)).Run()
	} else {
		// Remove PATH entries from shell rc files
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
	fmt.Println(" ✓")

	// Remove install info file
	os.Remove(installInfoPath())

	fmt.Println()
	fmt.Println(green + "  ✅ StarClaw 已完全卸载 (Uninstall complete)" + reset)
	fmt.Println()
	fmt.Println("  Press Enter to close...")
	reader.ReadString('\n')
}
