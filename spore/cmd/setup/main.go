package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"sync"
	"time"
)

const (
	version   = "1.0.0"
	nydusBase = "https://nydus.starclaw.net/spore/releases"
)

func getSporeURL() string {
	switch goruntime.GOOS {
	case "windows":
		return nydusBase + "/spore-windows-amd64.exe"
	case "darwin":
		return nydusBase + "/spore-darwin-" + goruntime.GOARCH
	default:
		return nydusBase + "/spore-linux-amd64"
	}
}

func getClawURL() string {
	switch goruntime.GOOS {
	case "windows":
		return nydusBase + "/claw-v1.0.0-windows-amd64.spore"
	case "darwin":
		return nydusBase + "/claw-v1.0.0-darwin-" + goruntime.GOARCH + ".spore"
	default:
		return nydusBase + "/claw-v1.0.0-linux-amd64.spore"
	}
}

func main() {
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

	// Setup paths
	var installDir, binDir, sporePath string
	if goruntime.GOOS == "windows" {
		installDir = filepath.Join(os.Getenv("LOCALAPPDATA"), "StarClaw")
		binDir = filepath.Join(installDir, "bin")
		sporePath = filepath.Join(binDir, "spore.exe")
	} else {
		home, _ := os.UserHomeDir()
		installDir = filepath.Join(home, ".local")
		binDir = filepath.Join(installDir, "bin")
		sporePath = filepath.Join(binDir, "spore")
	}
	os.MkdirAll(binDir, 0755)
	tmpSpore := filepath.Join(os.TempDir(), fmt.Sprintf("claw-%d.spore", time.Now().UnixNano()))

	// Step 1: Parallel download Spore + Claw
	fmt.Printf(green + "  [1/4]" + reset + " Downloading Spore + Claw (parallel)...\n")
	var wg sync.WaitGroup
	var errSpore, errClaw error
	wg.Add(2)
	go func() {
		defer wg.Done()
		errSpore = downloadFile(sporePath, getSporeURL(), "  Spore  ")
	}()
	go func() {
		defer wg.Done()
		errClaw = downloadFile(tmpSpore, getClawURL(), "  Claw   ")
	}()
	wg.Wait()
	fmt.Println()
	if errSpore != nil {
		fail("Download Spore failed: %v", errSpore)
	}
	if errClaw != nil {
		fail("Download Claw failed: %v", errClaw)
	}
	// Make spore executable on Unix
	if goruntime.GOOS != "windows" {
		os.Chmod(sporePath, 0755)
	}
	fmt.Printf(green + "  [1/4]" + reset + " Download complete ✓\n")

	// Step 2: Install
	fmt.Printf(green + "  [2/4]" + reset + " Installing Claw...\n")
	cmd := exec.Command(sporePath, "install", tmpSpore)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fail("Install failed: %v", err)
	}
	os.Remove(tmpSpore)
	fmt.Printf(green + "  [2/4]" + reset + " Claw installed ✓\n")

	// Step 3: Auto-configure (no user input needed)
	port := "80"
	if !isPortAvailable(port) {
		port = "8080"
		if !isPortAvailable(port) {
			port = "8888"
		}
		fmt.Printf(yellow+"  Port 80 is in use, using port %s instead\n"+reset, port)
	}

	homeDir, _ := os.UserHomeDir()
	sporeHome := filepath.Join(homeDir, ".spore")
	configDir := filepath.Join(sporeHome, "installed", "claw", "current", "config")
	os.MkdirAll(configDir, 0755)

	jwtSecret := fmt.Sprintf("sc-%d-%d", time.Now().UnixNano(), os.Getpid())
	config := fmt.Sprintf(`server:
  host: 0.0.0.0
  port: %s

database:
  driver: sqlite
  dsn: "./data/claw.db"

jwt:
  secret: "%s"
`, port, jwtSecret)
	os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(config), 0644)

	envContent := fmt.Sprintf("GIN_MODE=release\nCLAW_DATA_DIR=./data\nCLAW_PORT=%s\nDEFAULT_PROVIDER=qwen\n", port)
	envDir := filepath.Join(sporeHome, "installed", "claw", "current")
	os.WriteFile(filepath.Join(envDir, ".env"), []byte(envContent), 0644)

	// Add to PATH
	addToPath(binDir)
	fmt.Printf(green+"  [3/4]"+reset+" Configuration saved (port %s, default: Qwen) ✓\n", port)

	// Step 4: Start + Desktop shortcut
	fmt.Printf(green + "  [4/4]" + reset + " Starting Claw...\n")
	startCmd := exec.Command(sporePath, "start", "claw")
	startCmd.Stdout = os.Stdout
	startCmd.Stderr = os.Stderr
	if err := startCmd.Run(); err != nil {
		fmt.Printf(yellow+"  Warning: Start failed: %v\n"+reset, err)
		fmt.Println(yellow + "  You can start manually later: spore start claw" + reset)
	} else {
		fmt.Printf(green + "  [4/4]" + reset + " Claw started ✓\n")
	}

	// Create desktop shortcut
	url := fmt.Sprintf("http://localhost:%s", port)
	if port == "80" {
		url = "http://localhost"
	}
	createDesktopShortcut(url, sporePath)

	// Done
	fmt.Println()
	fmt.Println(green + "  ══════════════════════════════════════════════" + reset)
	fmt.Println(green + "  ✅ Installation Complete!" + reset)
	fmt.Println(green + "  ══════════════════════════════════════════════" + reset)
	fmt.Println()
	fmt.Printf("  🌐 Open in browser: "+cyan+"%s"+reset+"\n", url)
	fmt.Println("  🖥️  Desktop shortcut: StarClaw")
	fmt.Println()
	fmt.Println("  Default AI: " + cyan + "Qwen (通义千问)" + reset)
	fmt.Println("  Config: " + configDir + "/config.yaml")
	fmt.Println()
	fmt.Println("  Commands:")
	fmt.Println("    spore status        — Check status")
	fmt.Println("    spore logs claw     — View logs")
	fmt.Println("    spore stop claw     — Stop")
	fmt.Println("    spore restart claw  — Restart")
	fmt.Println()

	// Open browser
	openBrowser(url)

	fmt.Println("  Press Enter to close this window...")
	bufio.NewReader(os.Stdin).ReadString('\n')
}

// isPortAvailable checks if a TCP port is free
func isPortAvailable(port string) bool {
	ln, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return false
	}
	ln.Close()
	return true
}

// createDesktopShortcut creates a clickable shortcut on the desktop
func createDesktopShortcut(url, sporePath string) {
	homeDir, _ := os.UserHomeDir()
	desktop := filepath.Join(homeDir, "Desktop")
	if _, err := os.Stat(desktop); os.IsNotExist(err) {
		// Try localized desktop folder name
		desktop = filepath.Join(homeDir, "桌面")
		if _, err := os.Stat(desktop); os.IsNotExist(err) {
			return
		}
	}

	switch goruntime.GOOS {
	case "windows":
		// Create a .url shortcut (simplest, works everywhere)
		content := fmt.Sprintf("[InternetShortcut]\nURL=%s\nIconIndex=0\n", url)
		os.WriteFile(filepath.Join(desktop, "StarClaw.url"), []byte(content), 0644)

		// Also create a .bat to start + open browser
		batContent := fmt.Sprintf("@echo off\r\nstart \"\" \"%s\" start claw 2>nul\r\ntimeout /t 2 /nobreak >nul\r\nstart %s\r\n", sporePath, url)
		os.WriteFile(filepath.Join(desktop, "StarClaw-Start.bat"), []byte(batContent), 0644)

	case "darwin":
		// Create a .command file
		content := fmt.Sprintf("#!/bin/sh\n%s start claw 2>/dev/null\nsleep 1\nopen \"%s\"\n", sporePath, url)
		path := filepath.Join(desktop, "StarClaw.command")
		os.WriteFile(path, []byte(content), 0755)

	default:
		// Create a .desktop file
		content := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=StarClaw
Comment=AI Agent Platform
Exec=sh -c '%s start claw; sleep 1; xdg-open %s'
Terminal=false
Categories=Development;
`, sporePath, url)
		path := filepath.Join(desktop, "StarClaw.desktop")
		os.WriteFile(path, []byte(content), 0755)
	}
}

func downloadFile(dest string, url string, label string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	size := resp.ContentLength
	written := int64(0)
	buf := make([]byte, 64*1024) // 64KB buffer for speed
	lastPct := -1

	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			out.Write(buf[:n])
			written += int64(n)
			if size > 0 {
				pct := int(written * 100 / size)
				if pct != lastPct && pct%5 == 0 {
					fmt.Printf("\r  %s %3d%% (%d/%d MB)", label, pct, written/(1024*1024), size/(1024*1024))
					lastPct = pct
				}
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}
	fmt.Printf("\r  %s 100%% ✓                    \n", label)
	return nil
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
