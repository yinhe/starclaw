package mcp

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/yinhe/starclaw/internal/tool"
)

const (
	BridgePort         = 9100
	BridgeServerName   = "host"
	bridgeProbeTimeout = 3 * time.Second
)

// DetectBridgeURL determines the MCP Bridge URL based on runtime environment.
// In Docker: use host.docker.internal. On bare metal: use 127.0.0.1.
func DetectBridgeURL() string {
	// Check if running inside Docker (/.dockerenv exists on Linux containers)
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return fmt.Sprintf("http://host.docker.internal:%d", BridgePort)
	}
	return fmt.Sprintf("http://127.0.0.1:%d", BridgePort)
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

// BridgeInstallInstructions returns platform-specific install instructions.
func BridgeInstallInstructions() string {
	switch runtime.GOOS {
	case "windows":
		return `# Windows - 在宿主机 PowerShell 中运行:
cd claw\api
go build -o mcp-bridge.exe .\cmd\mcp-bridge\
.\mcp-bridge.exe -port 9100`
	case "darwin":
		return `# macOS - 在宿主机终端中运行:
cd claw/api
go build -o mcp-bridge ./cmd/mcp-bridge/
./mcp-bridge -port 9100`
	default:
		return `# Linux - 在宿主机终端中运行:
cd claw/api
go build -o mcp-bridge ./cmd/mcp-bridge/
./mcp-bridge -port 9100

# 或使用 systemd 服务:
sudo cp deploy/mcp-bridge.service /etc/systemd/system/
sudo systemctl enable --now mcp-bridge`
	}
}
