package v1

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"github.com/yinhe/starclaw/internal/config"
	"github.com/yinhe/starclaw/internal/mcp"
	"github.com/yinhe/starclaw/internal/molt"
	"github.com/yinhe/starclaw/internal/swarm"
)

// SystemHandler handles system-level settings: swarm, updates, bounty
type SystemHandler struct {
	cfg         *config.Config
	swarmClient *swarm.Client
}

func NewSystemHandler(cfg *config.Config, sc *swarm.Client) *SystemHandler {
	return &SystemHandler{cfg: cfg, swarmClient: sc}
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

func performDockerUpdate() error {
	// Step 1: Pull latest image
	log.Println("[molt] pulling latest image...")
	pull := exec.Command("docker", "pull", "ghcr.io/yinhe/starclaw:latest")
	pull.Stdout = os.Stdout
	pull.Stderr = os.Stderr
	if err := pull.Run(); err != nil {
		// Try alternative: docker compose pull
		log.Println("[molt] docker pull failed, trying docker compose pull...")
		composePull := exec.Command("docker", "compose", "pull", "api")
		composePull.Stdout = os.Stdout
		composePull.Stderr = os.Stderr
		if err2 := composePull.Run(); err2 != nil {
			return fmt.Errorf("pull failed: %v / %v", err, err2)
		}
	}

	// Step 2: Restart via docker compose
	log.Println("[molt] restarting service...")
	restart := exec.Command("docker", "compose", "up", "-d", "--no-deps", "api")
	restart.Stdout = os.Stdout
	restart.Stderr = os.Stderr
	if err := restart.Run(); err != nil {
		// If running inside container, request graceful exit and let orchestrator restart
		log.Println("[molt] compose restart failed, sending SIGTERM to self for orchestrator restart...")
		p, _ := os.FindProcess(os.Getpid())
		p.Signal(os.Interrupt)
		return nil
	}

	return nil
}
