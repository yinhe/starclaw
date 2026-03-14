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
	BridgePort         = 9101
	BridgeServerName   = "host"
	bridgeProbeTimeout = 3 * time.Second
	owner              = "yinhe"
	repoName           = "starclaw"
	BridgeVersion      = "0.5.6"
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
// downloads the correct MCP Bridge binary, makes it executable, and runs it.
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
BINARY="mcp-bridge-${PLATFORM_OS}-${PLATFORM_ARCH}"
INSTALL_DIR="$HOME/.starclaw"
BINARY_PATH="$INSTALL_DIR/$BINARY"

echo "📦 Platform: $PLATFORM_OS/$PLATFORM_ARCH"
echo "📂 Install to: $INSTALL_DIR"

# Create install directory
mkdir -p "$INSTALL_DIR"

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

# Make executable
chmod +x "$BINARY_PATH"

# macOS: remove quarantine flag (Gatekeeper)
if [ "$PLATFORM_OS" = "darwin" ]; then
  xattr -d com.apple.quarantine "$BINARY_PATH" 2>/dev/null || true
  echo "✅ macOS Gatekeeper bypass applied"
fi

echo "✅ MCP Bridge installed to $BINARY_PATH"
echo ""
echo "🚀 Starting MCP Bridge..."
echo "   (Press Ctrl+C to stop)"
echo ""

# Run the bridge
exec "$BINARY_PATH"
`, serverURL)
}

// GeneratePowerShellInstallScript returns a PowerShell script for Windows.
func GeneratePowerShellInstallScript(serverURL string) string {
	return fmt.Sprintf(`# StarClaw MCP Bridge Installer (Windows)
Write-Host "🦞 StarClaw MCP Bridge Installer" -ForegroundColor Cyan
Write-Host "================================="

$InstallDir = "$env:USERPROFILE\.starclaw"
$Binary = "mcp-bridge-windows-amd64.exe"
$BinaryPath = "$InstallDir\$Binary"
$DownloadURL = "%s/v1/mcp-bridge/download/windows_amd64"

# Create install directory
if (!(Test-Path $InstallDir)) { New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null }

# Download binary
Write-Host "Downloading MCP Bridge..." -ForegroundColor Yellow
Invoke-WebRequest -Uri $DownloadURL -OutFile $BinaryPath -UseBasicParsing

Write-Host "MCP Bridge installed to $BinaryPath" -ForegroundColor Green
Write-Host ""
Write-Host "Starting MCP Bridge..." -ForegroundColor Cyan
Write-Host "   (Press Ctrl+C to stop)"
Write-Host ""

# Run the bridge
& $BinaryPath
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
