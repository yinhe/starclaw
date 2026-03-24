package mcp

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/yinhe/starclaw/internal/tool"
)

const (
	BridgePort          = 9101
	DevBridgePort       = 9102
	BridgeServerName    = "host"
	DevBridgeServerName = "dev"
	bridgeProbeTimeout  = 3 * time.Second
	owner               = "yinhe"
	repoName            = "starclaw"
	BridgeVersion       = "0.5.6"
)

// DetectBridgeURL determines the MCP Bridge URL based on runtime environment.
// In Docker: use host.docker.internal. On bare metal: use 127.0.0.1.
func DetectBridgeURL() string {
	port := BridgePort
	if v := os.Getenv("STARCLAW_BRIDGE_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			port = p
		}
	}
	// Check if running inside Docker (/.dockerenv exists on Linux containers)
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return fmt.Sprintf("http://host.docker.internal:%d", port)
	}
	return fmt.Sprintf("http://127.0.0.1:%d", port)
}

// ProbeBridge checks if the MCP Bridge is reachable at the given URL.
func ProbeBridge(url string) bool {
	client := &http.Client{Timeout: bridgeProbeTimeout}
	resp, err := client.Post(url, "application/json",
		strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// AutoRegisterBridge detects and registers the MCP Bridge on startup.
// It runs in the background, retrying periodically until the bridge is found.
func AutoRegisterBridge(registry *tool.Registry) {
	go func() {
		bridgeURL := DetectBridgeURL()
		log.Printf("[MCP Bridge] Auto-detect: OS=%s, URL=%s", runtime.GOOS, bridgeURL)

		// Try immediately, then retry every 30s for up to 5 minutes
		for attempt := 0; attempt < 10; attempt++ {
			if attempt > 0 {
				time.Sleep(30 * time.Second)
			}

			if !ProbeBridge(bridgeURL) {
				if attempt == 0 {
					log.Printf("[MCP Bridge] Not detected at %s (will retry in background)", bridgeURL)
				}
				continue
			}

			// Bridge is alive — register all its tools
			cfg := ServerConfig{
				BaseURL: bridgeURL,
				Name:    BridgeServerName,
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			err := RegisterMCPTools(ctx, registry, cfg)
			cancel()

			if err != nil {
				log.Printf("[MCP Bridge] Connected but failed to register tools: %v", err)
				continue
			}

			log.Printf("[MCP Bridge] ✓ Auto-registered from %s", bridgeURL)
			return
		}

		log.Printf("[MCP Bridge] Not found after retries. To enable host control, run: mcp-bridge -port %d", BridgePort)
	}()
}

// DetectDevBridgeURL returns the dev-bridge URL (same logic as host bridge).
func DetectDevBridgeURL() string {
	port := DevBridgePort
	if v := os.Getenv("STARCLAW_DEV_BRIDGE_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			port = p
		}
	}
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return fmt.Sprintf("http://host.docker.internal:%d", port)
	}
	return fmt.Sprintf("http://127.0.0.1:%d", port)
}

// AutoRegisterDevBridge detects and registers the Dev Bridge (development MCP tools).
func AutoRegisterDevBridge(registry *tool.Registry) {
	go func() {
		devURL := DetectDevBridgeURL()
		// Try once on startup, don't retry aggressively (dev-bridge is optional)
		time.Sleep(2 * time.Second) // wait for host bridge first
		if !ProbeBridge(devURL) {
			log.Printf("[Dev Bridge] Not detected at %s (optional — run dev-bridge to enable)", devURL)
			return
		}
		cfg := ServerConfig{
			BaseURL: devURL,
			Name:    DevBridgeServerName,
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err := RegisterMCPTools(ctx, registry, cfg)
		cancel()
		if err != nil {
			log.Printf("[Dev Bridge] Connected but failed to register: %v", err)
			return
		}
		log.Printf("[Dev Bridge] ✓ Auto-registered from %s", devURL)
	}()
}

// BridgeStatus returns the current bridge connection status and download info.
func BridgeStatus() map[string]interface{} {
	bridgeURL := DetectBridgeURL()
	connected := ProbeBridge(bridgeURL)

	result := map[string]interface{}{
		"connected":  connected,
		"bridge_url": bridgeURL,
		"port":       BridgePort,
		"host_os":    runtime.GOOS,
		"host_arch":  runtime.GOARCH,
		"downloads":  BridgeDownloadURLs(),
	}

	if connected {
		client := NewClient(ServerConfig{BaseURL: bridgeURL, Name: BridgeServerName})
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		tools, err := client.ListTools(ctx)
		cancel()
		if err == nil {
			result["tool_count"] = len(tools)
			names := make([]string, len(tools))
			for i, t := range tools {
				names[i] = t.Name
			}
			result["tool_names"] = names
		}
	}

	return result
}

// BridgeDownloadURLs returns download URLs served directly from this API.
func BridgeDownloadURLs() map[string]string {
	return map[string]string{
		"windows_amd64": "/v1/mcp-bridge/download/windows_amd64",
		"darwin_amd64":  "/v1/mcp-bridge/download/darwin_amd64",
		"darwin_arm64":  "/v1/mcp-bridge/download/darwin_arm64",
		"linux_amd64":   "/v1/mcp-bridge/download/linux_amd64",
	}
}

// GenerateInstallScript returns a bash script that auto-detects OS/arch,
// downloads the correct MCP Bridge binary, sets up auto-start, and runs it.
func GenerateInstallScript(serverURL string) string {
	return fmt.Sprintf(`#!/bin/bash
set -e

echo "🦞 StarClaw MCP Bridge Installer"
echo "================================="

# Detect OS and architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$OS" in
  darwin) PLATFORM_OS="darwin" ;;
  linux)  PLATFORM_OS="linux" ;;
  *)      echo "❌ Unsupported OS: $OS"; exit 1 ;;
esac

case "$ARCH" in
  x86_64|amd64)  PLATFORM_ARCH="amd64" ;;
  arm64|aarch64) PLATFORM_ARCH="arm64" ;;
  *)             echo "❌ Unsupported architecture: $ARCH"; exit 1 ;;
esac

PLATFORM="${PLATFORM_OS}_${PLATFORM_ARCH}"
BINARY="mcp-bridge"
INSTALL_DIR="$HOME/.starclaw"
BINARY_PATH="$INSTALL_DIR/$BINARY"

echo "📦 Platform: $PLATFORM_OS/$PLATFORM_ARCH"
echo "📂 Install to: $INSTALL_DIR"
mkdir -p "$INSTALL_DIR"

# Stop existing bridge if running
if curl -sf http://127.0.0.1:9101/health >/dev/null 2>&1; then
  echo "⏹  Stopping existing MCP Bridge..."
  curl -sf -X POST http://127.0.0.1:9101/shutdown >/dev/null 2>&1 || true
  sleep 1
fi

# Download binary
echo "⬇️  Downloading MCP Bridge..."
DOWNLOAD_URL="%s/v1/mcp-bridge/download/${PLATFORM}"
if command -v curl &>/dev/null; then
  curl -fSL "$DOWNLOAD_URL" -o "$BINARY_PATH"
elif command -v wget &>/dev/null; then
  wget -q "$DOWNLOAD_URL" -O "$BINARY_PATH"
else
  echo "❌ curl or wget required"; exit 1
fi
chmod +x "$BINARY_PATH"

# macOS: remove quarantine flag
if [ "$PLATFORM_OS" = "darwin" ]; then
  xattr -d com.apple.quarantine "$BINARY_PATH" 2>/dev/null || true
fi

# --- Set up auto-start as background service ---
if [ "$PLATFORM_OS" = "darwin" ]; then
  # macOS: launchd plist
  PLIST="$HOME/Library/LaunchAgents/com.starclaw.mcp-bridge.plist"
  mkdir -p "$HOME/Library/LaunchAgents"
  cat > "$PLIST" << PLISTEOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key><string>com.starclaw.mcp-bridge</string>
  <key>ProgramArguments</key><array><string>${BINARY_PATH}</string></array>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><true/>
  <key>StandardOutPath</key><string>${INSTALL_DIR}/bridge.log</string>
  <key>StandardErrorPath</key><string>${INSTALL_DIR}/bridge.log</string>
</dict>
</plist>
PLISTEOF
  launchctl unload "$PLIST" 2>/dev/null || true
  launchctl load "$PLIST"
  echo "✅ Installed as macOS service (auto-start on login)"
  echo "   Logs: $INSTALL_DIR/bridge.log"

elif [ "$PLATFORM_OS" = "linux" ]; then
  # Linux: systemd user service
  SVCDIR="$HOME/.config/systemd/user"
  mkdir -p "$SVCDIR"
  cat > "$SVCDIR/mcp-bridge.service" << SVCEOF
[Unit]
Description=StarClaw MCP Bridge
After=network.target

[Service]
ExecStart=${BINARY_PATH}
Restart=always
RestartSec=5

[Install]
WantedBy=default.target
SVCEOF
  systemctl --user daemon-reload
  systemctl --user enable mcp-bridge
  systemctl --user restart mcp-bridge
  echo "✅ Installed as systemd user service (auto-start on login)"
  echo "   Logs: journalctl --user -u mcp-bridge -f"
fi

echo ""
echo "🎉 Done! MCP Bridge is running in the background."
echo "   It will auto-start whenever you log in."
echo "   Go back to your Claw settings page — it should show 'Connected'."
`, serverURL)
}

// GeneratePowerShellInstallScript returns a PowerShell script for Windows.
func GeneratePowerShellInstallScript(serverURL string) string {
	return fmt.Sprintf(`# StarClaw MCP Bridge Installer (Windows)
Write-Host "StarClaw MCP Bridge Installer" -ForegroundColor Cyan
Write-Host "================================="

$InstallDir = "$env:USERPROFILE\.starclaw"
$Binary = "mcp-bridge.exe"
$BinaryPath = "$InstallDir\$Binary"
$DownloadURL = "%s/v1/mcp-bridge/download/windows_amd64"

if (!(Test-Path $InstallDir)) { New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null }

# Stop existing bridge
try { Invoke-RestMethod -Uri "http://127.0.0.1:9101/shutdown" -Method POST -ErrorAction SilentlyContinue } catch {}
Start-Sleep -Seconds 1

# Download
Write-Host "Downloading MCP Bridge..." -ForegroundColor Yellow
Invoke-WebRequest -Uri $DownloadURL -OutFile $BinaryPath -UseBasicParsing

# Auto-start: create startup shortcut
$StartupDir = "$env:APPDATA\Microsoft\Windows\Start Menu\Programs\Startup"
$ShortcutPath = "$StartupDir\MCP Bridge.lnk"
$WshShell = New-Object -ComObject WScript.Shell
$Shortcut = $WshShell.CreateShortcut($ShortcutPath)
$Shortcut.TargetPath = $BinaryPath
$Shortcut.WorkingDirectory = $InstallDir
$Shortcut.WindowStyle = 7  # minimized
$Shortcut.Save()
Write-Host "Auto-start shortcut created" -ForegroundColor Green

# Start as background process
Start-Process -FilePath $BinaryPath -WorkingDirectory $InstallDir -WindowStyle Hidden

Write-Host ""
Write-Host "Done! MCP Bridge is running in the background." -ForegroundColor Green
Write-Host "It will auto-start whenever you log in."
Write-Host "Go back to your Claw settings page - it should show Connected."
`, serverURL)
}

// BridgeBinaryPath returns the local file path for a given platform binary.
func BridgeBinaryPath(platform string) (string, string) {
	m := map[string]string{
		"windows_amd64": "mcp-bridge-windows-amd64.exe",
		"darwin_amd64":  "mcp-bridge-darwin-amd64",
		"darwin_arm64":  "mcp-bridge-darwin-arm64",
		"linux_amd64":   "mcp-bridge-linux-amd64",
	}
	filename, ok := m[platform]
	if !ok {
		return "", ""
	}
	return "/app/mcp-bridge/" + filename, filename
}
