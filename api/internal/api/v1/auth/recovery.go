package auth

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw/internal/model"
	"github.com/yinhe/starclaw/internal/node"
	"gorm.io/gorm"
)

// RecoveryHandler handles identity recovery, mnemonic backup, and phone binding.
type RecoveryHandler struct {
	db       *gorm.DB
	identity *node.Identity
	queenURL string
}

func NewRecoveryHandler(db *gorm.DB, identity *node.Identity, queenURL string) *RecoveryHandler {
	return &RecoveryHandler{db: db, identity: identity, queenURL: queenURL}
}

// GET /v1/recovery/status — check recovery setup progress
func (h *RecoveryHandler) Status(c *gin.Context) {
	status := node.LoadRecoveryStatus()

	// Also check if Queen has a backup for this node
	if h.queenURL != "" && !status.BackupExists {
		status.BackupExists = h.checkQueenBackup()
	}

	c.JSON(200, gin.H{"status": status})
}

// GET /v1/recovery/mnemonic — show 24-word BIP-39 mnemonic for current identity
func (h *RecoveryHandler) GetMnemonic(c *gin.Context) {
	mnemonic, err := node.IdentityMnemonic(h.identity)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to derive mnemonic: " + err.Error()})
		return
	}

	words := strings.Fields(mnemonic)
	c.JSON(200, gin.H{
		"mnemonic":   mnemonic,
		"words":      words,
		"word_count": len(words),
		"node_id":    h.identity.NodeID,
		"warning":    "请将助记词抄写在纸上，妥善保管。任何持有助记词的人都可以恢复您的 Claw 身份。",
	})
}

// POST /v1/recovery/confirm-mnemonic — user confirms they saved the mnemonic
func (h *RecoveryHandler) ConfirmMnemonic(c *gin.Context) {
	var req struct {
		Mnemonic string `json:"mnemonic"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}

	// Verify the mnemonic matches current identity
	id, err := node.IdentityFromMnemonic(req.Mnemonic)
	if err != nil {
		c.JSON(400, gin.H{"error": "助记词无效"})
		return
	}
	if id.NodeID != h.identity.NodeID {
		c.JSON(400, gin.H{"error": "助记词与当前节点不匹配"})
		return
	}

	status := node.LoadRecoveryStatus()
	status.MnemonicSaved = true
	node.SaveRecoveryStatus(status)

	c.JSON(200, gin.H{"message": "助记词确认成功", "mnemonic_saved": true})
}

// POST /v1/recovery/bind-phone — bind phone number for recovery verification
func (h *RecoveryHandler) BindPhone(c *gin.Context) {
	var req struct {
		Phone string `json:"phone"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Phone == "" {
		c.JSON(400, gin.H{"error": "请输入手机号"})
		return
	}

	if h.queenURL == "" {
		c.JSON(500, gin.H{"error": "未配置 Queen 服务器"})
		return
	}

	// Send phone binding request to Queen
	body, _ := json.Marshal(map[string]string{
		"node_id": h.identity.NodeID,
		"phone":   req.Phone,
	})
	resp, err := http.Post(h.queenURL+"/recovery/bind-phone", "application/json", strings.NewReader(string(body)))
	if err != nil {
		c.JSON(502, gin.H{"error": "连接 Queen 失败"})
		return
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if resp.StatusCode != 200 {
		msg, _ := result["error"].(string)
		c.JSON(resp.StatusCode, gin.H{"error": msg})
		return
	}

	c.JSON(200, gin.H{"message": "验证码已发送", "expires_in": 300})
}

// POST /v1/recovery/verify-phone — verify SMS code to complete phone binding
func (h *RecoveryHandler) VerifyPhone(c *gin.Context) {
	var req struct {
		Phone string `json:"phone"`
		Code  string `json:"code"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Phone == "" || req.Code == "" {
		c.JSON(400, gin.H{"error": "请输入手机号和验证码"})
		return
	}

	body, _ := json.Marshal(map[string]string{
		"node_id": h.identity.NodeID,
		"phone":   req.Phone,
		"code":    req.Code,
	})
	resp, err := http.Post(h.queenURL+"/recovery/verify-phone", "application/json", strings.NewReader(string(body)))
	if err != nil {
		c.JSON(502, gin.H{"error": "连接 Queen 失败"})
		return
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if resp.StatusCode != 200 {
		msg, _ := result["error"].(string)
		c.JSON(resp.StatusCode, gin.H{"error": msg})
		return
	}

	// Update local status
	status := node.LoadRecoveryStatus()
	status.PhoneBound = true
	status.Phone = maskPhone(req.Phone)
	node.SaveRecoveryStatus(status)

	c.JSON(200, gin.H{"message": "手机绑定成功", "phone": maskPhone(req.Phone)})
}

// POST /v1/recovery/backup — encrypt and upload backup to Queen
func (h *RecoveryHandler) Backup(c *gin.Context) {
	if h.queenURL == "" {
		c.JSON(500, gin.H{"error": "未配置 Queen 服务器"})
		return
	}

	// Get mnemonic for encryption
	mnemonic, err := node.IdentityMnemonic(h.identity)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to derive mnemonic"})
		return
	}

	// Collect agent data for backup
	var agents []model.Agent
	h.db.Find(&agents)
	backupAgents := make([]node.BackupAgent, 0, len(agents))
	for _, ag := range agents {
		backupAgents = append(backupAgents, node.BackupAgent{
			Name:         ag.Name,
			Description:  ag.Description,
			SystemPrompt: ag.SystemPrompt,
			Tools:        ag.Tools,
			Config:       ag.Config,
			SourceID:     ag.SourceID,
		})
	}

	// Build payload
	payload := node.BuildBackupPayload(h.identity, backupAgents, nil)
	payloadJSON, _ := json.Marshal(payload)

	// Encrypt with mnemonic-derived key
	encrypted, err := node.EncryptBackup(payloadJSON, mnemonic)
	if err != nil {
		c.JSON(500, gin.H{"error": "encryption failed"})
		return
	}

	// Upload to Queen
	lookupKey := node.BackupLookupKey(mnemonic)
	uploadBody, _ := json.Marshal(map[string]interface{}{
		"lookup_key": lookupKey,
		"node_id":    h.identity.NodeID,
		"data":       base64.StdEncoding.EncodeToString(encrypted),
		"data_size":  len(encrypted),
		"version":    1,
	})

	resp, err := http.Post(h.queenURL+"/recovery/backup", "application/json", strings.NewReader(string(uploadBody)))
	if err != nil {
		c.JSON(502, gin.H{"error": "上传备份失败"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		c.JSON(resp.StatusCode, gin.H{"error": "Queen 备份存储失败"})
		return
	}

	// Update local status
	status := node.LoadRecoveryStatus()
	status.BackupExists = true
	status.BackupTime = time.Now().Format("2006-01-02 15:04")
	node.SaveRecoveryStatus(status)

	c.JSON(200, gin.H{
		"message":     "备份成功",
		"agents":      len(backupAgents),
		"backup_size": len(encrypted),
		"backup_time": status.BackupTime,
	})
}

// POST /v1/recovery/restore — restore identity and data from mnemonic
// This is called on a FRESH install when the user wants to recover.
func (h *RecoveryHandler) Restore(c *gin.Context) {
	var req struct {
		Mnemonic string `json:"mnemonic"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Mnemonic == "" {
		c.JSON(400, gin.H{"error": "请输入 24 个助记词"})
		return
	}

	// 1. Verify mnemonic and derive identity
	restoredID, err := node.RestoreIdentityFromMnemonic(req.Mnemonic)
	if err != nil {
		c.JSON(400, gin.H{"error": "助记词无效: " + err.Error()})
		return
	}

	// 2. Try to download backup from Queen
	agentsRestored := 0
	if h.queenURL != "" {
		lookupKey := node.BackupLookupKey(req.Mnemonic)
		resp, err := http.Get(h.queenURL + "/recovery/backup?lookup_key=" + lookupKey)
		if err == nil && resp.StatusCode == 200 {
			defer resp.Body.Close()
			var result struct {
				Data string `json:"data"`
			}
			if json.NewDecoder(resp.Body).Decode(&result) == nil && result.Data != "" {
				encrypted, err := base64.StdEncoding.DecodeString(result.Data)
				if err == nil {
					plaintext, err := node.DecryptBackup(encrypted, req.Mnemonic)
					if err == nil {
						var payload node.BackupPayload
						if json.Unmarshal(plaintext, &payload) == nil {
							agentsRestored = h.restoreAgents(payload.Agents)
						}
					}
				}
			}
		}
	}

	c.JSON(200, gin.H{
		"message":          "身份恢复成功",
		"node_id":          restoredID.NodeID,
		"agents_restored":  agentsRestored,
		"restart_required": true,
		"note":             "请重启 Claw 以使新身份完全生效",
	})
}

// restoreAgents imports backup agents into the database.
func (h *RecoveryHandler) restoreAgents(agents []node.BackupAgent) int {
	count := 0
	// Get the owner user (first user)
	var user model.User
	if h.db.First(&user).Error != nil {
		return 0
	}

	for _, ba := range agents {
		agent := model.Agent{
			UserID:       user.ID,
			Name:         ba.Name,
			Description:  ba.Description,
			SystemPrompt: ba.SystemPrompt,
			Tools:        ba.Tools,
			Config:       ba.Config,
			SourceID:     ba.SourceID,
		}
		if h.db.Create(&agent).Error == nil {
			count++
		}
	}
	return count
}

// checkQueenBackup checks if a backup exists on Queen for this node.
func (h *RecoveryHandler) checkQueenBackup() bool {
	mnemonic, err := node.IdentityMnemonic(h.identity)
	if err != nil {
		return false
	}
	lookupKey := node.BackupLookupKey(mnemonic)
	resp, err := http.Get(h.queenURL + "/recovery/backup/exists?lookup_key=" + lookupKey)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200
}

// GET /v1/recovery/address — show the node's claw address in a user-friendly format
func (h *RecoveryHandler) Address(c *gin.Context) {
	seed := h.identity.PrivateKey.Seed()
	w := node.WalletFromSeed(seed, "")

	c.JSON(200, gin.H{
		"node_id":     h.identity.NodeID,
		"fingerprint": h.identity.Fingerprint(),
		"public_key":  h.identity.PublicKeyHex(),
		"hot_address": w.HotNodeID,
		"hd_path":     "m/44'/9001'/0'/0'/0'",
	})
}

// POST /v1/recovery/verify-mnemonic — verify a mnemonic without restoring
// (used in recovery flow to check if mnemonic is valid before proceeding)
func (h *RecoveryHandler) VerifyMnemonic(c *gin.Context) {
	var req struct {
		Mnemonic string `json:"mnemonic"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Mnemonic == "" {
		c.JSON(400, gin.H{"error": "请输入助记词"})
		return
	}

	id, err := node.IdentityFromMnemonic(req.Mnemonic)
	if err != nil {
		c.JSON(400, gin.H{"error": "助记词无效", "valid": false})
		return
	}

	// Check if backup exists on Queen
	hasBackup := false
	if h.queenURL != "" {
		lookupKey := node.BackupLookupKey(req.Mnemonic)
		resp, err := http.Get(h.queenURL + "/recovery/backup/exists?lookup_key=" + lookupKey)
		if err == nil {
			hasBackup = resp.StatusCode == 200
			resp.Body.Close()
		}
	}

	c.JSON(200, gin.H{
		"valid":      true,
		"node_id":    id.NodeID,
		"has_backup": hasBackup,
	})
}

func maskPhone(phone string) string {
	if len(phone) < 7 {
		return phone
	}
	return phone[:3] + "****" + phone[len(phone)-4:]
}
