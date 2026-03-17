package main

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
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

	// Step 1: Download Spore runtime
	fmt.Printf(green + "  [1/5]" + reset + " Downloading Spore runtime (6 MB)...\n")
	if err := downloadFile(sporePath, getSporeURL()); err != nil {
		fail("Download Spore failed: %v", err)
	}
	fmt.Printf(green + "  [1/5]" + reset + " Spore runtime ✓\n")

	// Make spore executable on Unix
	if goruntime.GOOS != "windows" {
		os.Chmod(sporePath, 0755)
	}

	// Step 2: Download Claw package
	fmt.Printf(green + "  [2/5]" + reset + " Downloading Claw package (12 MB)...\n")
	if err := downloadFile(tmpSpore, getClawURL()); err != nil {
		fail("Download Claw failed: %v", err)
	}
	fmt.Printf(green + "  [2/5]" + reset + " Claw package ✓\n")

	// Step 3: Install
	fmt.Printf(green + "  [3/5]" + reset + " Installing Claw...\n")
	cmd := exec.Command(sporePath, "install", tmpSpore)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fail("Install failed: %v", err)
	}
	os.Remove(tmpSpore)
	fmt.Printf(green + "  [3/5]" + reset + " Claw installed ✓\n")

	// Step 4: Configuration
	fmt.Println()
	fmt.Println(yellow + "  ─── Configuration ───" + reset)
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)

	provider := prompt(reader, "  AI Provider [openai/deepseek/qwen/ollama]", "openai")
	var apiKey, apiURL string
	if provider == "ollama" {
		apiURL = prompt(reader, "  Ollama URL", "http://localhost:11434")
	} else {
		apiKey = prompt(reader, "  API Key", "")
		if apiKey == "" {
			fmt.Println(yellow + "  (You can add the API key later in the config file)" + reset)
		}
	}
	port := prompt(reader, "  Server port", "8080")

	// Write config
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

	// Write .env
	envContent := fmt.Sprintf("GIN_MODE=release\nCLAW_DATA_DIR=./data\nCLAW_PORT=%s\n", port)
	if apiKey != "" {
		envContent += fmt.Sprintf("%s_API_KEY=%s\n", strings.ToUpper(provider), apiKey)
	}
	if apiURL != "" {
		envContent += fmt.Sprintf("OLLAMA_URL=%s\n", apiURL)
	}
	envDir := filepath.Join(sporeHome, "installed", "claw", "current")
	os.WriteFile(filepath.Join(envDir, ".env"), []byte(envContent), 0644)

	fmt.Printf(green + "  [4/5]" + reset + " Configuration saved ✓\n")

	// Add to PATH
	addToPath(binDir)

	// Step 5: Start
	fmt.Printf(green + "  [5/5]" + reset + " Starting Claw...\n")
	startCmd := exec.Command(sporePath, "start", "claw")
	startCmd.Stdout = os.Stdout
	startCmd.Stderr = os.Stderr
	if err := startCmd.Run(); err != nil {
		fmt.Printf(yellow+"  Warning: Start failed: %v\n"+reset, err)
		fmt.Println(yellow + "  You can start manually later: spore start claw" + reset)
	} else {
		fmt.Printf(green + "  [5/5]" + reset + " Claw started ✓\n")
	}

	// Done
	fmt.Println()
	fmt.Println(green + "  ══════════════════════════════════════════════" + reset)
	fmt.Println(green + "  ✅ Installation Complete!" + reset)
	fmt.Println(green + "  ══════════════════════════════════════════════" + reset)
	fmt.Println()
	fmt.Printf("  🌐 Open in browser: "+cyan+"http://localhost:%s"+reset+"\n", port)
	fmt.Println()
	fmt.Println("  Commands:")
	fmt.Println("    spore status        — Check status")
	fmt.Println("    spore logs claw     — View logs")
	fmt.Println("    spore stop claw     — Stop")
	fmt.Println("    spore restart claw  — Restart")
	fmt.Println("    spore update claw   — Update to latest")
	fmt.Println()

	// Try to open browser
	openBrowser(fmt.Sprintf("http://localhost:%s", port))

	fmt.Println("  Press Enter to close this window...")
	reader.ReadString('\n')
}

func prompt(reader *bufio.Reader, label, defaultVal string) string {
	if defaultVal != "" {
		fmt.Printf("  %s (default: %s): ", label, defaultVal)
	} else {
		fmt.Printf("  %s: ", label)
	}
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input == "" {
		return defaultVal
	}
	return input
}

func downloadFile(filepath string, url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	size := resp.ContentLength
	written := int64(0)
	buf := make([]byte, 32*1024)
	lastPct := -1

	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			out.Write(buf[:n])
			written += int64(n)
			if size > 0 {
				pct := int(written * 100 / size)
				if pct != lastPct && pct%10 == 0 {
					fmt.Printf("\r         %d%%", pct)
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
	fmt.Print("\r")
	return nil
}

func addToPath(dir string) {
	if goruntime.GOOS == "windows" {
		exec.Command("powershell", "-NoProfile", "-Command",
			fmt.Sprintf(`$p = [Environment]::GetEnvironmentVariable('Path','User'); if ($p -notlike '*%s*') { [Environment]::SetEnvironmentVariable('Path', "$p;%s", 'User') }`, dir, dir)).Run()
	} else {
		// Add to shell profile
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
