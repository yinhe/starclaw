package v1

import (
	"bufio"
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
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"github.com/yinhe/starclaw/internal/config"
	"github.com/yinhe/starclaw/internal/mcp"
	"github.com/yinhe/starclaw/internal/molt"
	"github.com/yinhe/starclaw/internal/node"
	"github.com/yinhe/starclaw/internal/overlord"
	"github.com/yinhe/starclaw/internal/procutil"
	"github.com/yinhe/starclaw/internal/swarm"
	"github.com/yinhe/starclaw/internal/tool"
	"gorm.io/gorm"
)

// SystemHandler handles system-level settings: swarm, overlord, updates, bounty
type SystemHandler struct {
	cfg            *config.Config
	db             *gorm.DB
	swarmClient    *swarm.Client
	identity       *node.Identity
	overlordClient *overlord.Client
}

func NewSystemHandler(cfg *config.Config, db *gorm.DB, sc *swarm.Client, identity *node.Identity, oc ...*overlord.Client) *SystemHandler {
	h := &SystemHandler{cfg: cfg, db: db, swarmClient: sc, identity: identity}
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
		// Standalone mode: try star-ai.net balance first, fall back to local stardust
		force := c.Query("refresh") == "true"
		if bal := tool.GetStarAIBalance(force); bal != nil {
			hp := "healthy"
			switch bal.StarStatus {
			case "hibernated":
				hp = "hibernated"
			case "":
				// Derive from balance if Queen didn't return status
				if bal.Balance < 100 {
					hp = "critical"
				} else if bal.Balance < 500 {
					hp = "low"
				} else if bal.Balance >= 5000 {
					hp = "full"
				}
			default:
				if bal.Balance < 100 {
					hp = "critical"
				} else if bal.Balance < 500 {
					hp = "low"
				} else if bal.Balance >= 5000 {
					hp = "full"
				}
			}
			c.JSON(http.StatusOK, gin.H{
				"connected":      true,
				"balance":        bal.BalanceRaw, // internal units (1⚡ = 10000) for BillingPage formatEnergy()
				"balance_energy": bal.Balance,    // display ⚡ value for HPBar
				"hp_status":      hp,
				"status":         "synapse",
				"star_status":    bal.StarStatus,
				"message":        fmt.Sprintf("星能余额 %.1f ⚡（star-ai.net）", bal.Balance),
				"updated_at":     bal.UpdatedAt.Format("15:04:05"),
			})
			return
		}
		// StarAI unreachable — show disconnected, never fake balance
		c.JSON(http.StatusOK, gin.H{
			"connected": false,
			"message":   "未连接星能网络（star-ai.net 不可达）",
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

	runtimeMode := detectRuntimeMode(h.cfg.Hive.URL)

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
//   - hive:       notify Hive Controller → rolling upgrade of all instances
//   - docker:     Docker socket pull+up (fast) → MCP Bridge build (fallback)
//   - spore:      download binary → replace → exit → Spore auto-restarts
//   - standalone: download binary → replace → exit
func (h *SystemHandler) TriggerUpdate(c *gin.Context) {
	vi := molt.GetVersionInfo()
	if !vi.UpdateAvail {
		c.JSON(http.StatusOK, gin.H{"message": "已是最新版本", "version": vi.Current})
		return
	}

	resetUpdateLog()
	ulogInfo("开始更新: %s → %s", vi.Current, vi.Latest)

	// Detect runtime mode
	runtimeMode := detectRuntimeMode(h.cfg.Hive.URL)
	ulogInfo("运行模式: %s", runtimeMode)

	switch runtimeMode {
	case "hive":
		// Notify Hive Controller to perform rolling upgrade of all instances
		ulogInfo("Hive 模式，通知 Hive Controller 执行滚动升级")
		go func() {
			defer finishUpdateLog()
			if err := performHiveUpdate(h.cfg.Hive.URL, vi); err != nil {
				ulogError("Hive 升级通知失败: %v", err)
			}
		}()

	case "docker":
		// Try Docker socket first (fast pull), fall back to MCP Bridge (source build)
		if hasDockerSocket() {
			ulogInfo("检测到 Docker Socket，使用镜像拉取更新")
			go func() {
				defer finishUpdateLog()
				if err := performDockerSocketUpdate(vi.Latest); err != nil {
					ulogError("Docker Socket 更新失败: %v，尝试 MCP Bridge...", err)
					if err2 := performDockerUpdate(); err2 != nil {
						ulogError("MCP Bridge 更新也失败: %v", err2)
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
				defer finishUpdateLog()
				if err := performDockerUpdate(); err != nil {
					ulogError("更新失败: %v", err)
				}
			}()
		}

	case "spore":
		ulogInfo("Spore 模式，使用二进制替换更新")
		go func() {
			defer finishUpdateLog()
			if err := performSporeUpdate(vi.Latest); err != nil {
				ulogError("Spore 更新失败: %v", err)
			}
		}()

	default: // standalone
		ulogInfo("独立模式，使用二进制替换更新")
		go func() {
			defer finishUpdateLog()
			if err := performStandaloneUpdate(vi.Latest); err != nil {
				ulogError("独立更新失败: %v", err)
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

	// Kill the bridge process directly (more reliable than JSON-RPC shutdown)
	var killErr error
	if runtime.GOOS == "windows" {
		killErr = procutil.Command("taskkill", "/F", "/IM", "mcp-bridge.exe").Run()
	} else {
		killErr = procutil.Command("pkill", "-f", "mcp-bridge").Run()
	}

	if killErr != nil {
		log.Printf("[bridge] kill failed: %v, trying HTTP shutdown", killErr)
		// Fallback: try HTTP
		client := &http.Client{Timeout: 3 * time.Second}
		req, _ := http.NewRequest("POST", bridgeURL+"/shutdown", nil)
		resp, err := client.Do(req)
		if err == nil {
			resp.Body.Close()
		}
	}

	c.JSON(http.StatusOK, gin.H{"status": "stopped", "message": "MCP Bridge 已断开"})
}

// --- Identity (Key Export/Import) ---

// ExportIdentity returns the node's Ed25519 key as a downloadable backup.
// The key is base64-encoded for safe transport. User should store securely.
func (h *SystemHandler) ExportIdentity(c *gin.Context) {
	if h.identity == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "identity not initialized"})
		return
	}
	keyData, err := h.identity.ExportKey()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "export failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"node_id":     h.identity.NodeID,
		"fingerprint": h.identity.Fingerprint(),
		"key_backup":  string(keyData),
	})
}

// ImportIdentity replaces the current node identity with a backup key.
// After import, the server must be restarted for the new identity to take effect.
func (h *SystemHandler) ImportIdentity(c *gin.Context) {
	var req struct {
		KeyBackup string `json:"key_backup" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "key_backup required"})
		return
	}

	newID, err := node.ImportKey([]byte(req.KeyBackup))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":          "密钥已导入，请重启服务生效",
		"new_node_id":      newID.NodeID,
		"new_fingerprint":  newID.Fingerprint(),
		"restart_required": true,
	})
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
		InviteCode  string `json:"invite_code"`
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
	if req.InviteCode != "" {
		h.cfg.Overlord.InviteCode = req.InviteCode
	}

	viper.Set("overlord.enabled", true)
	viper.Set("overlord.overlord_url", req.OverlordURL)
	if req.NodeName != "" {
		viper.Set("overlord.node_name", req.NodeName)
	}
	if req.Region != "" {
		viper.Set("overlord.region", req.Region)
	}
	if req.InviteCode != "" {
		viper.Set("overlord.invite_code", req.InviteCode)
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

// ── Update log buffer (real-time progress visible in frontend) ──

var (
	updateLogMu    sync.Mutex
	updateLogLines []updateLogEntry
	updateRunning  bool
)

type updateLogEntry struct {
	Time    string `json:"time"`
	Message string `json:"message"`
	Level   string `json:"level"` // info, error, success
}

// ulog writes to both the standard log and the update log buffer.
func ulog(level, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	log.Printf("[molt] %s", msg)
	updateLogMu.Lock()
	updateLogLines = append(updateLogLines, updateLogEntry{
		Time:    time.Now().Format("15:04:05"),
		Message: msg,
		Level:   level,
	})
	updateLogMu.Unlock()
}

func ulogInfo(format string, args ...interface{})    { ulog("info", format, args...) }
func ulogError(format string, args ...interface{})   { ulog("error", format, args...) }
func ulogSuccess(format string, args ...interface{}) { ulog("success", format, args...) }

func resetUpdateLog() {
	updateLogMu.Lock()
	updateLogLines = nil
	updateRunning = true
	updateLogMu.Unlock()
}

func finishUpdateLog() {
	updateLogMu.Lock()
	updateRunning = false
	updateLogMu.Unlock()
}

// GetUpdateLog returns the current update log buffer for the frontend
func (h *SystemHandler) GetUpdateLog(c *gin.Context) {
	updateLogMu.Lock()
	lines := make([]updateLogEntry, len(updateLogLines))
	copy(lines, updateLogLines)
	running := updateRunning
	updateLogMu.Unlock()
	c.JSON(http.StatusOK, gin.H{
		"lines":   lines,
		"running": running,
	})
}

// ── Helpers: runtime detection ──

func detectRuntimeMode(hiveURL string) string {
	if os.Getenv("SPORE_DATA_DIR") != "" {
		return "spore"
	}
	if _, err := os.Stat("/.dockerenv"); err == nil {
		// Hive containers have hive.url configured by the Hive Controller
		if hiveURL != "" {
			return "hive"
		}
		return "docker"
	}
	return "standalone"
}

func hasDockerSocket() bool {
	_, err := os.Stat("/var/run/docker.sock")
	return err == nil
}

// ── Docker Socket update (auto-detect host dir, pull or build, no MCP Bridge needed) ──

func performDockerSocketUpdate(targetVersion string) error {
	hostDir := os.Getenv("STARCLAW_HOST_DIR")

	// Auto-detect host project dir from Docker Compose container labels
	if hostDir == "" {
		ulogInfo("自动检测宿主机项目目录...")
		out, err := procutil.Command("docker", "inspect", "starclaw-api",
			"--format", `{{index .Config.Labels "com.docker.compose.project.working_dir"}}`).CombinedOutput()
		if err == nil {
			detected := strings.TrimSpace(string(out))
			if detected != "" && detected != "<no value>" {
				hostDir = detected
				ulogInfo("检测到项目目录: %s", hostDir)
			}
		}
	}

	// Fallback: search common host paths via Docker
	if hostDir == "" {
		for _, dir := range []string{"/opt/starclaw/claw", "/opt/starclaw", "/opt/claw"} {
			if err := procutil.Command("docker", "run", "--rm",
				"-v", dir+":"+dir+":ro",
				"alpine:latest", "test", "-d", filepath.Join(dir, "api")).Run(); err == nil {
				hostDir = dir
				ulogInfo("找到项目目录: %s", hostDir)
				break
			}
		}
	}

	if hostDir == "" {
		return fmt.Errorf("无法检测宿主机项目目录，请设置 STARCLAW_HOST_DIR 环境变量")
	}

	// Build the update script that runs inside a helper container on the host.
	// Key fixes:
	//  1. Read original project-name from container labels → seamless container replacement
	//  2. Use --project-directory → compose relative paths resolve correctly
	//  3. Verify pull got real images → avoid false-positive when compose has build+image
	//  4. Temp dir for tar → avoid overwrite conflicts
	nydusURL := molt.NydusSourceURL
	script := fmt.Sprintf(`#!/bin/sh
set -e

# Setup mirrors for China network
sed -i 's|dl-cdn.alpinelinux.org|mirrors.aliyun.com|g' /etc/apk/repositories 2>/dev/null || true
apk add --no-cache curl docker-cli docker-cli-compose > /dev/null 2>&1
echo "@@TOOLS_READY"

HOSTDIR="%s"
VER="%s"
NYDUS="%s"
cd "$HOSTDIR"

# ── Detect project layout ──
if [ -f "claw/docker-compose.prod.yml" ]; then
  COMPOSE="$HOSTDIR/claw/docker-compose.prod.yml"
  SRCDIR="$HOSTDIR/claw"
  echo "@@LAYOUT:monorepo"
elif [ -f "docker-compose.prod.yml" ]; then
  COMPOSE="$HOSTDIR/docker-compose.prod.yml"
  SRCDIR="$HOSTDIR"
  echo "@@LAYOUT:standalone"
else
  COMPOSE="$HOSTDIR/docker-compose.yml"
  SRCDIR="$HOSTDIR"
  echo "@@LAYOUT:dev"
fi

COMPOSEDIR=$(dirname "$COMPOSE")

# ── Read original project name from running container labels ──
# This is CRITICAL: docker compose tracks containers by project name.
# Without matching the original, "up" tries to create NEW containers
# instead of replacing existing ones → container name conflict.
PROJECT=$(docker inspect starclaw-api --format '{{index .Config.Labels "com.docker.compose.project"}}' 2>/dev/null || true)
if [ -z "$PROJECT" ]; then
  PROJECT=$(basename "$COMPOSEDIR")
fi

DC="docker compose --project-name $PROJECT --project-directory $COMPOSEDIR -f $COMPOSE"
echo "@@PROJECT:$PROJECT"
echo "@@COMPOSE:$COMPOSE"
echo "@@SRCDIR:$SRCDIR"

# ── Try pulling pre-built images (fast path) ──
NEED_BUILD=true
echo "@@PULL_START"
STARCLAW_VERSION="$VER" $DC pull api web > /dev/null 2>&1 || true

# Verify pull actually got the versioned image (not just exit code 0)
API_IMG="ghcr.io/yinhe/starclaw-api:$VER"
if docker image inspect "$API_IMG" > /dev/null 2>&1; then
  NEED_BUILD=false
  echo "@@PULL_OK"
else
  echo "@@PULL_NOIMAGE"
fi

# ── Build from source if needed ──
if [ "$NEED_BUILD" = "true" ]; then
  # Update source code from Nydus (temp dir avoids tar overwrite conflicts)
  echo "@@SOURCE_START"
  TMP=$(mktemp -d)
  curl -sfL --connect-timeout 10 --max-time 120 "$NYDUS" | tar xz -C "$TMP" --strip-components=1
  cp -rf "$TMP"/. "$SRCDIR"/
  rm -rf "$TMP"
  echo -n "$VER" > "$SRCDIR/api/.version"
  echo "@@SOURCE_OK"

  # Build images
  echo "@@BUILD_START"
  BUILD_VERSION="$VER" STARCLAW_VERSION="$VER" $DC build api web 2>&1
  echo "@@BUILD_OK"
fi

# ── Recreate containers ──
echo "@@UP_START"
BUILD_VERSION="$VER" STARCLAW_VERSION="$VER" $DC up -d --no-deps api web 2>&1
echo "@@UPDATE_COMPLETE"
`, hostDir, targetVersion, nydusURL)

	ulogInfo("启动宿主机构建容器...")
	cmd := procutil.Command("docker", "run", "--rm",
		"-v", hostDir+":"+hostDir,
		"-v", "/var/run/docker.sock:/var/run/docker.sock",
		"--workdir", hostDir,
		"alpine:latest",
		"sh", "-c", script)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("create pipe: %w", err)
	}
	cmd.Stderr = cmd.Stdout // merge stderr into stdout

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start helper container: %w", err)
	}

	// Stream output and translate step markers into real-time log entries
	var lastOutput strings.Builder
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 256*1024)
	for scanner.Scan() {
		line := scanner.Text()
		lastOutput.WriteString(line + "\n")
		// Keep buffer bounded
		if lastOutput.Len() > 8192 {
			s := lastOutput.String()
			lastOutput.Reset()
			lastOutput.WriteString(s[len(s)-4096:])
		}

		switch {
		case line == "@@TOOLS_READY":
			ulogInfo("构建工具就绪")
		case strings.HasPrefix(line, "@@LAYOUT:"):
			ulogInfo("布局: %s", strings.TrimPrefix(line, "@@LAYOUT:"))
		case strings.HasPrefix(line, "@@COMPOSE:"):
			ulogInfo("compose: %s", strings.TrimPrefix(line, "@@COMPOSE:"))
		case strings.HasPrefix(line, "@@SRCDIR:"):
			ulogInfo("源码目录: %s", strings.TrimPrefix(line, "@@SRCDIR:"))
		case line == "@@PULL_START":
			ulogInfo("尝试拉取预构建镜像...")
		case line == "@@PULL_OK":
			ulogInfo("预构建镜像已拉取，跳过源码构建")
		case line == "@@PULL_NOIMAGE":
			ulogInfo("镜像不存在，切换到源码构建...")
		case strings.HasPrefix(line, "@@PROJECT:"):
			ulogInfo("项目名: %s", strings.TrimPrefix(line, "@@PROJECT:"))
		case line == "@@SOURCE_START":
			ulogInfo("正在从 Nydus 下载源码...")
		case line == "@@SOURCE_OK":
			ulogInfo("源码已更新，版本写入完成")
		case line == "@@BUILD_START":
			ulogInfo("正在构建镜像 (可能需要几分钟)...")
		case line == "@@BUILD_OK":
			ulogInfo("镜像构建完成")
		case line == "@@UP_START":
			ulogInfo("正在重建容器...")
		case line == "@@UPDATE_COMPLETE":
			ulogSuccess("✅ Docker 更新到 v%s 完成，服务重启中...", targetVersion)
		case strings.HasPrefix(line, "@@"):
			// skip unknown markers
		default:
			// Log notable build output (docker Step lines, errors)
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "Step ") || strings.HasPrefix(trimmed, "Successfully") ||
				strings.Contains(trimmed, "error") || strings.Contains(trimmed, "Error") {
				if len(trimmed) > 200 {
					trimmed = trimmed[:200] + "..."
				}
				ulogInfo("> %s", trimmed)
			}
		}
	}

	if err := cmd.Wait(); err != nil {
		out := lastOutput.String()
		if len(out) > 500 {
			out = out[len(out)-500:]
		}
		ulogError("构建容器失败: %s", out)
		return fmt.Errorf("helper container failed: %w", err)
	}
	return nil
}

// ── Hive update (notify Hive Controller → rolling upgrade of all instances) ──

func performHiveUpdate(hiveURL string, vi molt.VersionInfo) error {
	ulogInfo("通知 Hive Controller: %s", hiveURL)

	body := fmt.Sprintf(`{"current_version":"%s","latest_version":"%s","source":"claw-ui"}`,
		vi.Current, vi.Latest)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Post(hiveURL+"/hive/upgrade-notify", "application/json",
		strings.NewReader(body))
	if err != nil {
		return fmt.Errorf("连接 Hive Controller 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusAccepted {
		ulogSuccess("✅ Hive Controller 已接受升级请求，将对所有实例执行滚动升级")
		return nil
	}
	return fmt.Errorf("Hive Controller 返回 HTTP %d", resp.StatusCode)
}

// ── Spore update (download binary, replace in-place, exit for auto-restart) ──

func performSporeUpdate(targetVersion string) error {
	ulogInfo("下载 v%s 二进制文件...", targetVersion)

	// Build candidate URLs: Nydus first (faster in China), then GitHub
	urls := sporeDownloadURLs(targetVersion)
	if len(urls) == 0 {
		return fmt.Errorf("unsupported platform: %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	// Try each URL until one succeeds
	var tmpPath string
	var lastErr error
	client := &http.Client{Timeout: 5 * time.Minute}
	for _, binaryURL := range urls {
		ulogInfo("尝试下载: %s", binaryURL)
		tmpFile, err := os.CreateTemp("", "starclaw-update-*")
		if err != nil {
			return fmt.Errorf("create temp file: %w", err)
		}
		tp := tmpFile.Name()

		resp, err := client.Get(binaryURL)
		if err != nil {
			tmpFile.Close()
			os.Remove(tp)
			ulogError("下载失败: %v，尝试下一个镜像...", err)
			lastErr = err
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			tmpFile.Close()
			os.Remove(tp)
			ulogError("下载返回 HTTP %d，尝试下一个镜像...", resp.StatusCode)
			lastErr = fmt.Errorf("HTTP %d from %s", resp.StatusCode, binaryURL)
			continue
		}

		if _, err := io.Copy(tmpFile, resp.Body); err != nil {
			resp.Body.Close()
			tmpFile.Close()
			os.Remove(tp)
			ulogError("写入失败: %v，尝试下一个镜像...", err)
			lastErr = err
			continue
		}
		resp.Body.Close()
		tmpFile.Close()
		os.Chmod(tp, 0755)
		tmpPath = tp
		ulogInfo("下载完成: %s", binaryURL)
		break
	}
	if tmpPath == "" {
		return fmt.Errorf("所有下载镜像均失败: %v", lastErr)
	}

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

	ulogSuccess("✅ 二进制已替换: %s，正在重启...", currentBin)

	// Give the HTTP response time to flush, then restart.
	time.Sleep(1 * time.Second)

	// If supervised by Spore (SPORE_SUPERVISED=1), just exit — supervisor restarts automatically.
	if os.Getenv("SPORE_SUPERVISED") == "1" {
		ulogInfo("Spore 监控模式，退出后自动重启...")
		os.Exit(0)
		return nil
	}

	// Standalone: spawn a detached helper that waits for this process to exit
	// (releasing the port), then starts the new binary.
	cwd, _ := os.Getwd()
	pid := os.Getpid()
	argsStr := strings.Join(os.Args[1:], " ")

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		// PowerShell: wait for old PID to exit, then start new binary
		script := fmt.Sprintf(
			`Start-Sleep -Seconds 2; `+
				`try { Wait-Process -Id %d -Timeout 10 -ErrorAction SilentlyContinue } catch {}; `+
				`Start-Process -FilePath '%s' -ArgumentList '%s' -WorkingDirectory '%s'`,
			pid, currentBin, argsStr, cwd)
		cmd = procutil.Command("powershell", "-WindowStyle", "Hidden", "-Command", script)
	} else {
		// Unix: wait for old PID to exit, then start new binary
		script := fmt.Sprintf(
			`sleep 2; while kill -0 %d 2>/dev/null; do sleep 1; done; cd '%s' && '%s' %s &`,
			pid, cwd, currentBin, argsStr)
		cmd = procutil.Command("sh", "-c", script)
	}
	cmd.SysProcAttr = detachedSysProcAttr()
	if err := cmd.Start(); err != nil {
		ulogError("重启失败: %v（请手动启动）", err)
	} else {
		ulogInfo("延迟重启已调度 (等待 PID %d 退出后启动新进程)...", pid)
		cmd.Process.Release()
	}
	os.Exit(0)
	return nil // unreachable
}

// performStandaloneUpdate downloads and replaces the binary for non-Docker, non-Spore installs.
func performStandaloneUpdate(targetVersion string) error {
	return performSporeUpdate(targetVersion) // same logic: download + replace + exit
}

// sporeDownloadURLs returns candidate download URLs for the given version.
// Nydus Spore releases first (faster in China), then Nydus binary path, then GitHub.
func sporeDownloadURLs(version string) []string {
	os_ := runtime.GOOS
	arch := runtime.GOARCH

	// claw-api binary name pattern: claw-api-{os}-{arch}[.exe]
	name := fmt.Sprintf("claw-api-%s-%s", os_, arch)
	if os_ == "windows" {
		name += ".exe"
	}

	// GitHub release binary name pattern: starclaw-{os}-{arch}[.exe]
	ghName := fmt.Sprintf("starclaw-%s-%s", os_, arch)
	if os_ == "windows" {
		ghName += ".exe"
	}

	return []string{
		// Nydus Spore releases dir (uploaded by build-release.ps1)
		fmt.Sprintf("https://nydus.starclaw.net/spore/releases/%s", name),
		// Nydus binary release path (legacy)
		fmt.Sprintf("https://nydus.starclaw.net/releases/binary/v%s/%s", version, ghName),
		// GitHub release
		fmt.Sprintf("https://github.com/yinhe/starclaw/releases/download/v%s/%s", version, ghName),
	}
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
		ulogError("MCP Bridge 不可用，无法从容器内更新")
		return fmt.Errorf("MCP Bridge 未运行，无法执行一键更新。请在宿主机手动执行 update.sh")
	}

	ulogInfo("MCP Bridge 已连接，通过宿主机 Shell 更新...")
	client := mcp.NewClientWithTimeout(mcp.ServerConfig{BaseURL: bridgeURL, Name: "host"}, 15*time.Minute)

	// Step 1: Find project root directory
	result, _ := execOnHost(client, `for d in /opt/starclaw /opt/claw /home/*/starclaw /root/starclaw; do [ -d "$d/claw/api" ] && echo "$d" && exit 0; done; echo /opt/starclaw`)
	projectDir := strings.TrimSpace(result)
	if projectDir == "" {
		projectDir = "/opt/starclaw"
	}
	ulogInfo("项目目录: %s", projectDir)

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
	ulogInfo("compose: %s/%s", projectDir, composeFile)

	// Step 3: Update source code — try GitHub first, fallback to Nydus tarball
	// Monorepo layout: git may be in claw/ subdir (OSS repo maps claw/ → root)
	// Standalone layout: git is at project root
	pullResult, _ := execOnHost(client, fmt.Sprintf(
		`cd "%s" && if [ -d .git ]; then git fetch origin main 2>&1 && git reset --hard origin/main 2>&1; elif [ -d claw/.git ]; then cd claw && git fetch origin main 2>&1 && git reset --hard origin/main 2>&1; else echo "NO_GIT"; fi`,
		projectDir))
	ulogInfo("源码更新: %.200s", pullResult)

	// Fallback: if git fetch failed or no git, download tarball from Nydus mirror
	gitFailed := strings.Contains(pullResult, "NO_GIT") || strings.Contains(pullResult, "fatal:") || strings.Contains(pullResult, "error:")
	if gitFailed {
		ulogInfo("Git 拉取失败，尝试 Nydus 源码包...")
		nydusResult, nydusErr := execOnHostTimeout(client, fmt.Sprintf(
			`cd "%s" && curl -sfL --connect-timeout 10 --max-time 120 "%s" | tar xz --strip-components=1 2>&1 && echo "NYDUS_OK"`,
			projectDir, molt.NydusSourceURL), 180)
		if nydusErr != nil || !strings.Contains(nydusResult, "NYDUS_OK") {
			ulogError("Nydus 回退也失败: %v", nydusErr)
			ulogError("警告: 源码未更新，将使用现有代码构建")
		} else {
			ulogInfo("源码已通过 Nydus 更新")
		}
	}

	// Step 3.5: Write version to .version file so Dockerfile picks it up during build
	targetVersion := molt.GetVersionInfo().Latest
	execOnHost(client, fmt.Sprintf(`echo -n "%s" > "%s/api/.version"`, targetVersion, projectDir))
	ulogInfo("写入版本 %s 到 api/.version", targetVersion)

	// Step 4: Build and restart with correct compose file (5 min timeout for docker build)
	updateCmd := fmt.Sprintf(`cd "%s" && BUILD_VERSION="%s" docker compose -f %s build api web 2>&1 && BUILD_VERSION="%s" docker compose -f %s up -d --no-deps api web 2>&1`,
		projectDir, targetVersion, composeFile, targetVersion, composeFile)
	result, err := execOnHostTimeout(client, updateCmd, 900)
	if err != nil {
		ulogError("构建/重启失败: %v", err)
		return fmt.Errorf("更新失败: %v", err)
	}
	ulogSuccess("✅ MCP Bridge 更新完成: %.200s", result)
	return nil
}
