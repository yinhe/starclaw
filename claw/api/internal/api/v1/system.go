package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"github.com/yinhe/starclaw/internal/config"
	"github.com/yinhe/starclaw/internal/mcp"
	"github.com/yinhe/starclaw/internal/molt"
	"github.com/yinhe/starclaw/internal/node"
	"github.com/yinhe/starclaw/internal/overlord"
	"github.com/yinhe/starclaw/internal/swarm"
)

// SystemHandler handles system-level settings: swarm, overlord, updates, bounty
type SystemHandler struct {
	cfg            *config.Config
	swarmClient    *swarm.Client
	identity       *node.Identity
	overlordClient *overlord.Client
}

func NewSystemHandler(cfg *config.Config, sc *swarm.Client, identity *node.Identity, oc ...*overlord.Client) *SystemHandler {
	h := &SystemHandler{cfg: cfg, swarmClient: sc, identity: identity}
	if len(oc) > 0 {
		h.overlordClient = oc[0]
	}
	return h
}

// --- Swarm ---

// GetSwarmStatus returns current swarm connection state including feral mode
func (h *SystemHandler) GetSwarmStatus(c *gin.Context) {
	nodeID := ""
	state := "disconnected"
	var consecutiveFails int
	var lastHeartbeat, feralSince *time.Time

	if h.swarmClient != nil {
		nodeID = h.swarmClient.NodeID()
		state = h.swarmClient.State()
		consecutiveFails = h.swarmClient.ConsecutiveFails()
		if lh := h.swarmClient.LastHeartbeat(); !lh.IsZero() {
			lastHeartbeat = &lh
		}
		if fs := h.swarmClient.FeralSince(); !fs.IsZero() {
			feralSince = &fs
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"enabled":           h.cfg.Swarm.Enabled,
		"queen_url":         h.cfg.Swarm.QueenURL,
		"node_name":         h.cfg.Swarm.NodeName,
		"region":            h.cfg.Swarm.Region,
		"node_id":           nodeID,
		"connected":         state == "connected",
		"state":             state,
		"consecutive_fails": consecutiveFails,
		"last_heartbeat":    lastHeartbeat,
		"feral_since":       feralSince,
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

	// Normalize claw:// protocol to http(s)://
	queenURL := swarm.NormalizeQueenURL(req.QueenURL)

	// Update runtime config
	h.cfg.Swarm.Enabled = true
	h.cfg.Swarm.QueenURL = queenURL
	if req.NodeName != "" {
		h.cfg.Swarm.NodeName = req.NodeName
	}
	if req.Region != "" {
		h.cfg.Swarm.Region = req.Region
	}

	// Persist to config file (store normalized URL)
	viper.Set("swarm.enabled", true)
	viper.Set("swarm.queen_url", queenURL)
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
	if h.identity != nil {
		h.swarmClient.SetIdentity(h.identity)
	}
	if h.cfg.Node.Address != "" {
		h.swarmClient.SetAddress(h.cfg.Node.Address)
	}
	h.swarmClient.Start()

	log.Printf("[system] joined swarm: queen=%s (raw: %s) node=%s region=%s", queenURL, req.QueenURL, req.NodeName, req.Region)
	c.JSON(http.StatusOK, gin.H{"message": "已加入虫群", "queen_url": queenURL})
}

// LeaveSwarm disconnects from Queen
func (h *SystemHandler) LeaveSwarm(c *gin.Context) {
	h.cfg.Swarm.Enabled = false
	viper.Set("swarm.enabled", false)
	_ = viper.WriteConfig()

	if h.swarmClient != nil {
		h.swarmClient.Stop()
		h.swarmClient = nil
	}

	// Clean up credentials
	os.Remove(".swarm_credentials")

	log.Println("[system] left swarm")
	c.JSON(http.StatusOK, gin.H{"message": "已退出虫群"})
}

// --- Credits (星能) ---

// GetCredits returns cached star energy balance from Queen (updated via heartbeat).
// If ?refresh=true, queries Queen directly for latest balance.
func (h *SystemHandler) GetCredits(c *gin.Context) {
	if h.swarmClient == nil || !h.swarmClient.Connected() {
		c.JSON(http.StatusOK, gin.H{
			"connected": false,
			"message":   "未连接虫群，无法获取星能余额",
		})
		return
	}

	// Direct query if requested
	if c.Query("refresh") == "true" {
		if cc := h.swarmClient.CreditClient(); cc != nil {
			if balance, err := cc.QueryBalance(); err == nil {
				c.JSON(http.StatusOK, gin.H{
					"connected":      true,
					"balance":        balance.Balance,
					"balance_energy": balance.BalanceEnergy,
					"frozen":         balance.Frozen,
					"frozen_energy":  balance.FrozenEnergy,
					"total_in":       balance.TotalIn,
					"total_out":      balance.TotalOut,
					"nonce":          balance.Nonce,
					"status":         balance.Status,
					"hp_status":      balance.HPStatus,
					"hp":             string(cc.HP()),
					"trust_level":    balance.TrustLevel,
					"updated_at":     balance.UpdatedAt,
				})
				return
			}
		}
	}

	credits := h.swarmClient.Credits()
	if credits == nil {
		c.JSON(http.StatusOK, gin.H{
			"connected": true,
			"credits":   nil,
			"message":   "等待心跳同步余额...",
		})
		return
	}

	hp := "unknown"
	if cc := h.swarmClient.CreditClient(); cc != nil {
		hp = string(cc.HP())
	}

	c.JSON(http.StatusOK, gin.H{
		"connected":      true,
		"balance":        credits.Balance,
		"balance_energy": credits.BalanceEnergy,
		"frozen":         credits.Frozen,
		"frozen_energy":  credits.FrozenEnergy,
		"total_in":       credits.TotalIn,
		"total_out":      credits.TotalOut,
		"nonce":          credits.Nonce,
		"status":         credits.Status,
		"hp_status":      credits.HPStatus,
		"hp":             hp,
		"trust_level":    credits.TrustLevel,
		"updated_at":     credits.UpdatedAt,
	})
}

// TransferCredits handles POST /system/credits/transfer — Ed25519-signed transfer
func (h *SystemHandler) TransferCredits(c *gin.Context) {
	cc := h.getCreditClient(c)
	if cc == nil {
		return
	}

	var req struct {
		ToClaw string  `json:"to_claw" binding:"required"`
		Amount float64 `json:"amount" binding:"required"` // in Stars
		Remark string  `json:"remark"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数不完整: to_claw, amount (Stars) 必填"})
		return
	}

	amountUnits := int64(req.Amount * 10000)
	result, err := cc.Transfer(swarm.TransferRequest{
		ToClaw: req.ToClaw,
		Amount: amountUnits,
		Remark: req.Remark,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"txn_id":        result.TxnID,
		"from":          result.From,
		"to":            result.To,
		"amount":        result.Amount,
		"amount_energy": result.AmountEnergy,
		"new_balance":   result.NewBalance,
	})
}

// ListCreditTransactions handles GET /system/credits/transactions
func (h *SystemHandler) ListCreditTransactions(c *gin.Context) {
	cc := h.getCreditClient(c)
	if cc == nil {
		return
	}

	page := 1
	pageSize := 20
	if v := c.Query("page"); v != "" {
		fmt.Sscanf(v, "%d", &page)
	}
	if v := c.Query("page_size"); v != "" {
		fmt.Sscanf(v, "%d", &pageSize)
	}
	txnType := c.Query("type")

	list, err := cc.ListTransactions(page, pageSize, txnType)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"transactions": list.Transactions,
		"total":        list.Total,
		"page":         list.Page,
		"page_size":    list.PageSize,
	})
}

func (h *SystemHandler) getCreditClient(c *gin.Context) *swarm.CreditClient {
	if h.swarmClient == nil || !h.swarmClient.Connected() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "未连接虫群"})
		return nil
	}
	cc := h.swarmClient.CreditClient()
	if cc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "CreditClient 未初始化"})
		return nil
	}
	return cc
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

	// Detect runtime mode: spore (managed by Spore), docker, or standalone
	runtimeMode := "standalone"
	if os.Getenv("SPORE_DATA_DIR") != "" {
		runtimeMode = "spore"
	} else if _, err := os.Stat("/.dockerenv"); err == nil {
		runtimeMode = "docker"
	}

	c.JSON(http.StatusOK, gin.H{
		"version":       vi,
		"go_version":    runtime.Version(),
		"os":            runtime.GOOS,
		"arch":          runtime.GOARCH,
		"memory_mb":     memStats.Alloc / 1024 / 1024,
		"deploy_mode":   h.cfg.Server.DeployMode,
		"runtime_mode":  runtimeMode,
		"swarm_enabled": h.cfg.Swarm.Enabled,
	})
}

// TriggerUpdate detects runtime mode and dispatches to the appropriate update path:
//   - docker: Docker socket pull+up (fast) → MCP Bridge build (fallback)
//   - spore:  download binary → replace → exit → Spore auto-restarts
//   - standalone: download binary → replace → exit
func (h *SystemHandler) TriggerUpdate(c *gin.Context) {
	vi := molt.GetVersionInfo()
	if !vi.UpdateAvail {
		c.JSON(http.StatusOK, gin.H{"message": "已是最新版本", "version": vi.Current})
		return
	}

	log.Printf("[molt] user triggered update: %s → %s", vi.Current, vi.Latest)

	// Detect runtime mode
	runtimeMode := detectRuntimeMode()

	switch runtimeMode {
	case "docker":
		// Try Docker socket first (fast pull), fall back to MCP Bridge (source build)
		if hasDockerSocket() {
			log.Println("[molt] Docker socket available, using pull-based update")
			go func() {
				if err := performDockerSocketUpdate(vi.Latest); err != nil {
					log.Printf("[molt] Docker socket update failed: %v, trying MCP Bridge...", err)
					if err2 := performDockerUpdate(); err2 != nil {
						log.Printf("[molt] MCP Bridge update also failed: %v", err2)
					}
				}
			}()
		} else {
			bridgeURL := mcp.DetectBridgeURL()
			if !mcp.ProbeBridge(bridgeURL) {
				c.JSON(http.StatusServiceUnavailable, gin.H{
					"error":   "Docker socket 和 MCP Bridge 均不可用",
					"message": "请确保 docker-compose.yml 已挂载 /var/run/docker.sock，或启动 MCP Bridge",
				})
				return
			}
			go func() {
				if err := performDockerUpdate(); err != nil {
					log.Printf("[molt] update failed: %v", err)
				}
			}()
		}

	case "spore":
		go func() {
			if err := performSporeUpdate(vi.Latest); err != nil {
				log.Printf("[molt] Spore update failed: %v", err)
			}
		}()

	default: // standalone
		go func() {
			if err := performStandaloneUpdate(vi.Latest); err != nil {
				log.Printf("[molt] standalone update failed: %v", err)
			}
		}()
	}

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("正在更新到 v%s，服务将在数秒后重启...", vi.Latest),
		"from":    vi.Current,
		"to":      vi.Latest,
		"method":  runtimeMode,
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

// StopBridge sends a shutdown command to the MCP Bridge process
func (h *SystemHandler) StopBridge(c *gin.Context) {
	bridgeURL := mcp.DetectBridgeURL()
	if !mcp.ProbeBridge(bridgeURL) {
		c.JSON(http.StatusOK, gin.H{"status": "not_running", "message": "Bridge is not running"})
		return
	}
	client := &http.Client{Timeout: 3 * time.Second}
	req, _ := http.NewRequest("POST", bridgeURL+"/shutdown", nil)
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"status": "stopped", "message": "Bridge shutdown signal sent"})
		return
	}
	resp.Body.Close()
	c.JSON(http.StatusOK, gin.H{"status": "stopped", "message": "Bridge shutdown signal sent"})
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
	if h.identity != nil {
		h.overlordClient.SetClawID(h.identity.NodeID)
	}
	if h.cfg.Node.Address != "" {
		h.overlordClient.SetAddress(h.cfg.Node.Address)
	}
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

// --- Mining (算力共享) ---

// GetMiningStatus returns the current node's compute contribution status
func (h *SystemHandler) GetMiningStatus(c *gin.Context) {
	result := gin.H{
		"enabled":   h.cfg.Contributor.Enabled,
		"connected": false,
	}

	// Get contributor info from swarm client callback
	if h.swarmClient != nil && h.swarmClient.ContributorInfoFunc != nil {
		if info := h.swarmClient.ContributorInfoFunc(); info != nil && info.IsContributor {
			result["connected"] = h.swarmClient.Connected()
			result["is_contributing"] = true
			result["models"] = info.Models
			result["gpu_info"] = info.GPUInfo
		}
	}

	// Get cached credit balance for earnings display
	if h.swarmClient != nil {
		if cb := h.swarmClient.Credits(); cb != nil {
			result["balance"] = cb.Balance
			result["balance_energy"] = cb.BalanceEnergy
			result["hp_status"] = cb.HPStatus
		}
	}

	c.JSON(http.StatusOK, result)
}

// ToggleMining enables or disables compute contribution
func (h *SystemHandler) ToggleMining(c *gin.Context) {
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.cfg.Contributor.Enabled = req.Enabled
	viper.Set("contributor.enabled", req.Enabled)
	if err := viper.WriteConfig(); err != nil {
		log.Printf("[mining] failed to save config: %v", err)
	}

	status := "已关闭"
	if req.Enabled {
		status = "已开启（重启后生效）"
	}

	c.JSON(http.StatusOK, gin.H{
		"enabled": req.Enabled,
		"message": "算力共享" + status,
	})
}

// execOnHost sends a shell command to the MCP Bridge with proper JSON escaping.
func execOnHost(client *mcp.Client, command string) (string, error) {
	args, _ := json.Marshal(map[string]string{"command": command})
	return client.CallTool(context.Background(), "shell_exec", string(args))
}

// execOnHostTimeout sends a shell command with a custom timeout (for long-running ops like docker build).
func execOnHostTimeout(client *mcp.Client, command string, timeoutSec int) (string, error) {
	args, _ := json.Marshal(map[string]interface{}{"command": command, "timeout_seconds": timeoutSec})
	return client.CallTool(context.Background(), "shell_exec", string(args))
}

// ── Helpers: runtime detection ──

func detectRuntimeMode() string {
	if os.Getenv("SPORE_DATA_DIR") != "" {
		return "spore"
	}
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return "docker"
	}
	return "standalone"
}

func hasDockerSocket() bool {
	_, err := os.Stat("/var/run/docker.sock")
	return err == nil
}

// ── Docker Socket update (pull pre-built images, no MCP Bridge needed) ──

func performDockerSocketUpdate(targetVersion string) error {
	hostDir := os.Getenv("STARCLAW_HOST_DIR")
	composeFile := os.Getenv("STARCLAW_COMPOSE_FILE")
	if composeFile == "" {
		composeFile = "docker-compose.prod.yml"
	}

	// If STARCLAW_HOST_DIR is set, use docker compose from the host project dir
	// (mounted at same path for correct volume resolution)
	if hostDir != "" {
		composePath := filepath.Join(hostDir, composeFile)
		log.Printf("[molt] Docker socket update via compose: %s", composePath)

		// Pull latest images
		pullCmd := exec.Command("docker", "compose", "-f", composePath, "pull", "api", "web")
		pullCmd.Env = append(os.Environ(), "STARCLAW_VERSION="+targetVersion)
		if out, err := pullCmd.CombinedOutput(); err != nil {
			log.Printf("[molt] compose pull output: %s", string(out))
			return fmt.Errorf("docker compose pull failed: %w", err)
		}

		// Recreate containers with new images
		upCmd := exec.Command("docker", "compose", "-f", composePath, "up", "-d", "--no-deps", "api", "web")
		upCmd.Env = append(os.Environ(), "STARCLAW_VERSION="+targetVersion)
		if out, err := upCmd.CombinedOutput(); err != nil {
			log.Printf("[molt] compose up output: %s", string(out))
			return fmt.Errorf("docker compose up failed: %w", err)
		}
		log.Printf("[molt] ✅ Docker socket update to v%s complete", targetVersion)
		return nil
	}

	// Fallback: pull images directly by name (no compose file needed)
	log.Println("[molt] STARCLAW_HOST_DIR not set, pulling images directly...")
	imageTag := targetVersion
	if imageTag == "" {
		imageTag = "latest"
	}
	apiImage := fmt.Sprintf("ghcr.io/yinhe/starclaw-api:%s", imageTag)
	webImage := fmt.Sprintf("ghcr.io/yinhe/starclaw-web:%s", imageTag)

	for _, img := range []string{apiImage, webImage} {
		log.Printf("[molt] pulling %s...", img)
		if out, err := exec.Command("docker", "pull", img).CombinedOutput(); err != nil {
			log.Printf("[molt] pull %s failed: %s", img, string(out))
			return fmt.Errorf("docker pull %s failed: %w", img, err)
		}
	}

	// Tag as :latest so existing compose references work
	exec.Command("docker", "tag", apiImage, "ghcr.io/yinhe/starclaw-api:latest").Run()
	exec.Command("docker", "tag", webImage, "ghcr.io/yinhe/starclaw-web:latest").Run()

	// Restart containers using their existing names
	for _, name := range []string{"starclaw-api", "starclaw-web"} {
		exec.Command("docker", "stop", "-t", "10", name).Run()
		exec.Command("docker", "rm", name).Run()
	}

	// Without compose file we can't recreate with full config — try compose in common paths
	for _, dir := range []string{"/opt/starclaw/claw", "/opt/starclaw", "/opt/claw"} {
		for _, cf := range []string{"docker-compose.prod.yml", "docker-compose.yml"} {
			cp := filepath.Join(dir, cf)
			if _, err := os.Stat(cp); err == nil {
				log.Printf("[molt] found compose at %s, using it for up", cp)
				upCmd := exec.Command("docker", "compose", "-f", cp, "up", "-d", "--no-deps", "api", "web")
				upCmd.Env = append(os.Environ(), "STARCLAW_VERSION="+imageTag)
				if out, err := upCmd.CombinedOutput(); err != nil {
					log.Printf("[molt] compose up output: %s", string(out))
					continue
				}
				log.Printf("[molt] ✅ Docker socket update to v%s complete", targetVersion)
				return nil
			}
		}
	}

	return fmt.Errorf("images pulled but no compose file found to recreate containers; set STARCLAW_HOST_DIR")
}

// ── Spore update (download binary, replace in-place, exit for auto-restart) ──

func performSporeUpdate(targetVersion string) error {
	log.Printf("[molt] Spore update: downloading v%s binary...", targetVersion)

	// Determine download URL based on platform
	binaryURL := sporeDownloadURL(targetVersion)
	if binaryURL == "" {
		return fmt.Errorf("unsupported platform: %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	// Download to temp file
	tmpFile, err := os.CreateTemp("", "starclaw-update-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(binaryURL)
	if err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		tmpFile.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("download returned HTTP %d", resp.StatusCode)
	}

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("write binary: %w", err)
	}
	tmpFile.Close()
	os.Chmod(tmpPath, 0755)

	log.Printf("[molt] downloaded %s to %s", binaryURL, tmpPath)

	// Find current binary path
	currentBin, err := os.Executable()
	if err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("find executable: %w", err)
	}
	currentBin, _ = filepath.EvalSymlinks(currentBin)

	// Replace: on Linux/macOS rename over the running binary (inode-based, safe)
	// On Windows: rename old → .old, move new → current
	if runtime.GOOS == "windows" {
		oldPath := currentBin + ".old"
		os.Remove(oldPath)
		if err := os.Rename(currentBin, oldPath); err != nil {
			os.Remove(tmpPath)
			return fmt.Errorf("rename old binary: %w", err)
		}
		if err := os.Rename(tmpPath, currentBin); err != nil {
			os.Rename(oldPath, currentBin) // rollback
			return fmt.Errorf("move new binary: %w", err)
		}
	} else {
		if err := os.Rename(tmpPath, currentBin); err != nil {
			os.Remove(tmpPath)
			return fmt.Errorf("replace binary: %w", err)
		}
	}

	log.Printf("[molt] ✅ binary replaced at %s, exiting for restart...", currentBin)

	// Give the HTTP response time to flush, then exit.
	// Spore runtime's restart loop (or systemd/launchd) will restart with new binary.
	time.Sleep(1 * time.Second)
	os.Exit(0)
	return nil // unreachable
}

// performStandaloneUpdate downloads and replaces the binary for non-Docker, non-Spore installs.
func performStandaloneUpdate(targetVersion string) error {
	return performSporeUpdate(targetVersion) // same logic: download + replace + exit
}

func sporeDownloadURL(version string) string {
	os_ := runtime.GOOS
	arch := runtime.GOARCH

	// GitHub release binary name pattern: starclaw-{os}-{arch}[.exe]
	name := fmt.Sprintf("starclaw-%s-%s", os_, arch)
	if os_ == "windows" {
		name += ".exe"
	}

	// Try Nydus mirror first (faster in China), then GitHub
	nydusURL := fmt.Sprintf("https://nydus.starclaw.net/releases/binary/v%s/%s", version, name)
	ghURL := fmt.Sprintf("https://github.com/yinhe/starclaw/releases/download/v%s/%s", version, name)

	// Quick probe: try Nydus HEAD
	client := &http.Client{Timeout: 5 * time.Second}
	if resp, err := client.Head(nydusURL); err == nil {
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			return nydusURL
		}
	}
	return ghURL
}

// ── MCP Bridge update (source build, legacy fallback) ──

// PerformDockerUpdate executes a full Docker-based self-update via MCP Bridge.
// Exported so it can be called from the swarm client's auto-update flow.
func PerformDockerUpdate() error {
	return performDockerUpdate()
}

func performDockerUpdate() error {
	// MCP Bridge is required — the container cannot rebuild itself
	bridgeURL := mcp.DetectBridgeURL()
	if !mcp.ProbeBridge(bridgeURL) {
		log.Println("[molt] MCP Bridge not available, cannot update from inside container")
		return fmt.Errorf("MCP Bridge 未运行，无法执行一键更新。请在宿主机手动执行 update.sh")
	}

	log.Println("[molt] MCP Bridge detected, updating via host shell...")
	client := mcp.NewClientWithTimeout(mcp.ServerConfig{BaseURL: bridgeURL, Name: "host"}, 15*time.Minute)

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

	// Step 3: Update source code — try GitHub first, fallback to Nydus tarball
	// Monorepo layout: git may be in claw/ subdir (OSS repo maps claw/ → root)
	// Standalone layout: git is at project root
	pullResult, _ := execOnHost(client, fmt.Sprintf(
		`cd "%s" && if [ -d .git ]; then git fetch origin main 2>&1 && git reset --hard origin/main 2>&1; elif [ -d claw/.git ]; then cd claw && git fetch origin main 2>&1 && git reset --hard origin/main 2>&1; else echo "NO_GIT"; fi`,
		projectDir))
	log.Printf("[molt] source update: %.500s", pullResult)

	// Fallback: if git fetch failed or no git, download tarball from Nydus mirror
	gitFailed := strings.Contains(pullResult, "NO_GIT") || strings.Contains(pullResult, "fatal:") || strings.Contains(pullResult, "error:")
	if gitFailed {
		log.Printf("[molt] GitHub git failed, trying Nydus source tarball fallback...")
		nydusResult, nydusErr := execOnHostTimeout(client, fmt.Sprintf(
			`cd "%s" && curl -sfL --connect-timeout 10 --max-time 120 "%s" | tar xz --strip-components=1 2>&1 && echo "NYDUS_OK"`,
			projectDir, molt.NydusSourceURL), 180)
		if nydusErr != nil || !strings.Contains(nydusResult, "NYDUS_OK") {
			log.Printf("[molt] Nydus fallback also failed: %v %.500s", nydusErr, nydusResult)
			log.Println("[molt] WARNING: source code not updated. Build will use existing code.")
		} else {
			log.Printf("[molt] source updated via Nydus tarball")
		}
	}

	// Step 3.5: Write version to .version file so Dockerfile picks it up during build
	targetVersion := molt.GetVersionInfo().Latest
	execOnHost(client, fmt.Sprintf(`echo -n "%s" > "%s/api/.version"`, targetVersion, projectDir))
	log.Printf("[molt] wrote version %s to api/.version", targetVersion)

	// Step 4: Build and restart with correct compose file (5 min timeout for docker build)
	updateCmd := fmt.Sprintf(`cd "%s" && docker compose -f %s build api web 2>&1 && docker compose -f %s up -d --no-deps api web 2>&1`,
		projectDir, composeFile, composeFile)
	result, err := execOnHostTimeout(client, updateCmd, 900)
	if err != nil {
		log.Printf("[molt] update failed: %v", err)
		return fmt.Errorf("更新失败: %v", err)
	}
	log.Printf("[molt] update result: %.500s", result)
	return nil
}
