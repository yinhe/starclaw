package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"github.com/yinhe/starclaw/internal/config"
	"github.com/yinhe/starclaw/internal/mcp"
	"github.com/yinhe/starclaw/internal/molt"
	"github.com/yinhe/starclaw/internal/overlord"
	"github.com/yinhe/starclaw/internal/swarm"
)

// SystemHandler handles system-level settings: swarm, overlord, updates, bounty
type SystemHandler struct {
	cfg            *config.Config
	swarmClient    *swarm.Client
	overlordClient *overlord.Client
}

func NewSystemHandler(cfg *config.Config, sc *swarm.Client, oc ...*overlord.Client) *SystemHandler {
	h := &SystemHandler{cfg: cfg, swarmClient: sc}
	if len(oc) > 0 {
		h.overlordClient = oc[0]
	}
	return h
}

// --- Swarm ---

// GetSwarmStatus returns current swarm connection state
func (h *SystemHandler) GetSwarmStatus(c *gin.Context) {
	nodeID := ""
	if h.swarmClient != nil {
		nodeID = h.swarmClient.NodeID()
	}

	c.JSON(http.StatusOK, gin.H{
		"enabled":   h.cfg.Swarm.Enabled,
		"queen_url": h.cfg.Swarm.QueenURL,
		"node_name": h.cfg.Swarm.NodeName,
		"region":    h.cfg.Swarm.Region,
		"node_id":   nodeID,
		"connected": nodeID != "",
	})
}

// JoinSwarm enables swarm and connects to Queen
func (h *SystemHandler) JoinSwarm(c *gin.Context) {
	var req struct {
		QueenURL string `json:"queen_url" binding:"required"`
		NodeName string `json:"node_name"`
		Region   string `json:"region"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Update runtime config
	h.cfg.Swarm.Enabled = true
	h.cfg.Swarm.QueenURL = req.QueenURL
	if req.NodeName != "" {
		h.cfg.Swarm.NodeName = req.NodeName
	}
	if req.Region != "" {
		h.cfg.Swarm.Region = req.Region
	}

	// Persist to config file
	viper.Set("swarm.enabled", true)
	viper.Set("swarm.queen_url", req.QueenURL)
	if req.NodeName != "" {
		viper.Set("swarm.node_name", req.NodeName)
	}
	if req.Region != "" {
		viper.Set("swarm.region", req.Region)
	}
	_ = viper.WriteConfig()

	// Start swarm client with new config
	if h.swarmClient != nil {
		h.swarmClient.Stop()
	}
	h.swarmClient = swarm.NewClient(h.cfg.Swarm)
	h.swarmClient.Start()

	log.Printf("[system] joined swarm: queen=%s node=%s region=%s", req.QueenURL, req.NodeName, req.Region)
	c.JSON(http.StatusOK, gin.H{"message": "已加入虫群", "queen_url": req.QueenURL})
}

// LeaveSwarm disconnects from Queen
func (h *SystemHandler) LeaveSwarm(c *gin.Context) {
	h.cfg.Swarm.Enabled = false
	viper.Set("swarm.enabled", false)
	_ = viper.WriteConfig()

	if h.swarmClient != nil {
		h.swarmClient.Stop()
	}

	// Clean up credentials
	os.Remove(".swarm_credentials")

	log.Println("[system] left swarm")
	c.JSON(http.StatusOK, gin.H{"message": "已退出虫群"})
}

// --- Bounty ---

// GetBountyStatus returns bounty network connection state
func (h *SystemHandler) GetBountyStatus(c *gin.Context) {
	queenURL := h.cfg.Swarm.QueenURL
	connected := h.cfg.Swarm.Enabled && queenURL != ""

	c.JSON(http.StatusOK, gin.H{
		"enabled":   connected,
		"queen_url": queenURL,
		"bounty_url": func() string {
			if queenURL != "" {
				return strings.TrimSuffix(queenURL, "/swarm") + "/bounty"
			}
			return ""
		}(),
	})
}

// --- Update ---

// GetUpdateInfo returns version + update availability info
func (h *SystemHandler) GetUpdateInfo(c *gin.Context) {
	vi := molt.GetVersionInfo()

	// Also include system info
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	c.JSON(http.StatusOK, gin.H{
		"version":       vi,
		"go_version":    runtime.Version(),
		"os":            runtime.GOOS,
		"arch":          runtime.GOARCH,
		"memory_mb":     memStats.Alloc / 1024 / 1024,
		"deploy_mode":   h.cfg.Server.DeployMode,
		"swarm_enabled": h.cfg.Swarm.Enabled,
	})
}

// TriggerUpdate pulls latest Docker image and restarts the container
func (h *SystemHandler) TriggerUpdate(c *gin.Context) {
	vi := molt.GetVersionInfo()
	if !vi.UpdateAvail {
		c.JSON(http.StatusOK, gin.H{"message": "已是最新版本", "version": vi.Current})
		return
	}

	// Pre-check: MCP Bridge must be available (container can't rebuild itself)
	bridgeURL := mcp.DetectBridgeURL()
	if !mcp.ProbeBridge(bridgeURL) {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "MCP Bridge 未运行，无法执行一键更新",
			"message": "请先启动 MCP Bridge，或在服务器手动执行更新命令",
		})
		return
	}

	log.Printf("[molt] user triggered update: %s → %s", vi.Current, vi.Latest)

	// Run update in background
	go func() {
		if err := performDockerUpdate(); err != nil {
			log.Printf("[molt] update failed: %v", err)
		}
	}()

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("正在更新到 v%s，服务将在数秒后重启...", vi.Latest),
		"from":    vi.Current,
		"to":      vi.Latest,
	})
}

// ForceCheck triggers an immediate version check
func (h *SystemHandler) ForceCheck(c *gin.Context) {
	molt.ForceCheck()
	vi := molt.GetVersionInfo()
	c.JSON(http.StatusOK, vi)
}

// GetBridgeStatus returns MCP Bridge connection status and download URLs
func (h *SystemHandler) GetBridgeStatus(c *gin.Context) {
	c.JSON(http.StatusOK, mcp.BridgeStatus())
}

// --- Overlord ---

// GetOverlordStatus returns overlord connection state
func (h *SystemHandler) GetOverlordStatus(c *gin.Context) {
	nodeID := ""
	if h.overlordClient != nil {
		nodeID = h.overlordClient.NodeID()
	}

	c.JSON(http.StatusOK, gin.H{
		"enabled":      h.cfg.Overlord.Enabled,
		"overlord_url": h.cfg.Overlord.OverlordURL,
		"node_name":    h.cfg.Overlord.NodeName,
		"region":       h.cfg.Overlord.Region,
		"node_id":      nodeID,
		"connected":    nodeID != "",
	})
}

// JoinOverlord connects to an Overlord monitoring node
func (h *SystemHandler) JoinOverlord(c *gin.Context) {
	var req struct {
		OverlordURL string `json:"overlord_url" binding:"required"`
		NodeName    string `json:"node_name"`
		Region      string `json:"region"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.cfg.Overlord.Enabled = true
	h.cfg.Overlord.OverlordURL = req.OverlordURL
	if req.NodeName != "" {
		h.cfg.Overlord.NodeName = req.NodeName
	}
	if req.Region != "" {
		h.cfg.Overlord.Region = req.Region
	}

	viper.Set("overlord.enabled", true)
	viper.Set("overlord.overlord_url", req.OverlordURL)
	if req.NodeName != "" {
		viper.Set("overlord.node_name", req.NodeName)
	}
	if req.Region != "" {
		viper.Set("overlord.region", req.Region)
	}
	_ = viper.WriteConfig()

	if h.overlordClient != nil {
		h.overlordClient.Stop()
	}
	h.overlordClient = overlord.NewClient(h.cfg.Overlord)
	h.overlordClient.Start()

	log.Printf("[system] joined overlord: url=%s node=%s region=%s", req.OverlordURL, req.NodeName, req.Region)
	c.JSON(http.StatusOK, gin.H{"message": "已接入领主监控", "overlord_url": req.OverlordURL})
}

// LeaveOverlord disconnects from Overlord
func (h *SystemHandler) LeaveOverlord(c *gin.Context) {
	h.cfg.Overlord.Enabled = false
	viper.Set("overlord.enabled", false)
	_ = viper.WriteConfig()

	if h.overlordClient != nil {
		h.overlordClient.Stop()
	}

	os.Remove(".overlord_credentials")

	log.Println("[system] left overlord")
	c.JSON(http.StatusOK, gin.H{"message": "已退出领主监控"})
}

// execOnHost sends a shell command to the MCP Bridge with proper JSON escaping.
func execOnHost(client *mcp.Client, command string) (string, error) {
	args, _ := json.Marshal(map[string]string{"command": command})
	return client.CallTool(context.Background(), "shell_exec", string(args))
}

func performDockerUpdate() error {
	// MCP Bridge is required — the container cannot rebuild itself
	bridgeURL := mcp.DetectBridgeURL()
	if !mcp.ProbeBridge(bridgeURL) {
		log.Println("[molt] MCP Bridge not available, cannot update from inside container")
		return fmt.Errorf("MCP Bridge 未运行，无法执行一键更新。请在宿主机手动执行 update.sh")
	}

	log.Println("[molt] MCP Bridge detected, updating via host shell...")
	client := mcp.NewClient(mcp.ServerConfig{BaseURL: bridgeURL, Name: "host"})

	// Step 1: Find project root directory
	result, _ := execOnHost(client, `for d in /opt/starclaw /opt/claw /home/*/starclaw /root/starclaw; do [ -d "$d/claw/api" ] && echo "$d" && exit 0; done; echo /opt/starclaw`)
	projectDir := strings.TrimSpace(result)
	if projectDir == "" {
		projectDir = "/opt/starclaw"
	}
	log.Printf("[molt] project dir: %s", projectDir)

	// Step 2: Detect compose file — all compose files use api/web service names
	// Prefer claw/ subdir compose (standalone/monorepo OSS layout), then root
	composeFile := "docker-compose.yml"

	checkResult, _ := execOnHost(client, fmt.Sprintf(
		`[ -f "%s/claw/docker-compose.prod.yml" ] && echo CLAW_PROD || ([ -f "%s/docker-compose.prod.yml" ] && echo ROOT_PROD || echo DEV)`,
		projectDir, projectDir))
	checkResult = strings.TrimSpace(checkResult)
	if strings.Contains(checkResult, "CLAW_PROD") {
		projectDir = projectDir + "/claw"
		composeFile = "docker-compose.prod.yml"
	} else if strings.Contains(checkResult, "ROOT_PROD") {
		composeFile = "docker-compose.prod.yml"
	}
	log.Printf("[molt] compose: %s/%s", projectDir, composeFile)

	// Step 3: Update source code — fetch + reset --hard (not pull, which fails with dirty working tree from tar deploys)
	// Monorepo layout: git may be in claw/ subdir (OSS repo maps claw/ → root)
	// Standalone layout: git is at project root
	pullResult, _ := execOnHost(client, fmt.Sprintf(
		`cd "%s" && if [ -d .git ]; then git fetch origin main 2>&1 && git reset --hard origin/main 2>&1; elif [ -d claw/.git ]; then cd claw && git fetch origin main 2>&1 && git reset --hard origin/main 2>&1; else echo "NO_GIT"; fi`,
		projectDir))
	log.Printf("[molt] source update: %.500s", pullResult)

	if strings.Contains(pullResult, "NO_GIT") {
		log.Println("[molt] WARNING: no git repo on server, source code not updated. Build will use existing code.")
		log.Println("[molt] TIP: for monorepo, run: cd /opt/starclaw/claw && git init && git remote add origin https://github.com/yinhe/starclaw.git && git fetch origin main && git reset --mixed origin/main")
	}

	// Step 4: Build and restart with correct compose file
	updateCmd := fmt.Sprintf(`cd "%s" && docker compose -f %s build api web 2>&1 && docker compose -f %s up -d --no-deps api web 2>&1`,
		projectDir, composeFile, composeFile)
	result, err := execOnHost(client, updateCmd)
	if err != nil {
		log.Printf("[molt] update failed: %v", err)
		return fmt.Errorf("更新失败: %v", err)
	}
	log.Printf("[molt] update result: %.500s", result)
	return nil
}
