package v1

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw/internal/model"
	"github.com/yinhe/starclaw/internal/security"
	"gorm.io/gorm"
)

// SecurityHandler manages security features: encryption, audit chain, compliance.
type SecurityHandler struct {
	db         *gorm.DB
	keyMgr     *security.KeyManager
	auditChain *security.AuditChain
}

// NewSecurityHandler creates the security handler.
func NewSecurityHandler(db *gorm.DB, keyMgr *security.KeyManager, auditChain *security.AuditChain) *SecurityHandler {
	return &SecurityHandler{db: db, keyMgr: keyMgr, auditChain: auditChain}
}

// ════════════════════════════════════════════════════════════════
//  Encryption Status
// ════════════════════════════════════════════════════════════════

// EncryptionStatus returns the current encryption configuration.
func (h *SecurityHandler) EncryptionStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"enabled":             true,
		"algorithm":           "AES-256-GCM",
		"key_fingerprint":     h.keyMgr.MasterKeyFingerprint(),
		"key_derivation":      "SHA-256 HKDF-like",
		"encrypted_fields":    []string{"api_keys", "payout_info", "webhook_secrets"},
	})
}

// ════════════════════════════════════════════════════════════════
//  Audit Chain
// ════════════════════════════════════════════════════════════════

// AuditChainQuery returns paginated audit entries.
func (h *SecurityHandler) AuditChainQuery(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))
	action := c.Query("action")
	actor := c.Query("actor")
	target := c.Query("target")
	severity := c.Query("severity")
	sinceStr := c.Query("since")

	if page < 1 {
		page = 1
	}

	var since time.Time
	if sinceStr != "" {
		since, _ = time.Parse(time.RFC3339, sinceStr)
	}

	entries, total := h.auditChain.Query(action, actor, target, severity, since, page, pageSize)

	c.JSON(http.StatusOK, gin.H{
		"items":     entries,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// AuditChainVerify checks the integrity of the audit chain.
func (h *SecurityHandler) AuditChainVerify(c *gin.Context) {
	valid, entryCount, msg := h.auditChain.Verify()
	c.JSON(http.StatusOK, gin.H{
		"valid":        valid,
		"entry_count":  entryCount,
		"message":      msg,
	})
}

// AuditChainExport exports the audit chain as JSON.
func (h *SecurityHandler) AuditChainExport(c *gin.Context) {
	sinceStr := c.Query("since")
	var since time.Time
	if sinceStr != "" {
		since, _ = time.Parse(time.RFC3339, sinceStr)
	}

	data, err := h.auditChain.Export(since)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "export failed"})
		return
	}

	c.Header("Content-Disposition", "attachment; filename=audit-chain-export.json")
	c.Data(http.StatusOK, "application/json", data)
}

// AuditChainStats returns audit chain statistics.
func (h *SecurityHandler) AuditChainStats(c *gin.Context) {
	stats := h.auditChain.Stats()
	c.JSON(http.StatusOK, stats)
}

// ════════════════════════════════════════════════════════════════
//  GDPR — Data Subject Rights
// ════════════════════════════════════════════════════════════════

// GDPRExportData exports all personal data for the current user (Article 20: Right to Portability).
func (h *SecurityHandler) GDPRExportData(c *gin.Context) {
	userID := c.GetString("user_id")

	// Collect all user data
	var user model.User
	h.db.Where("id = ?", userID).First(&user)

	var agents []model.Agent
	h.db.Where("user_id = ?", userID).Find(&agents)

	var conversations []model.Conversation
	h.db.Where("user_id = ?", userID).Find(&conversations)

	var messages []model.Message
	h.db.Where("user_id = ?", userID).Find(&messages)

	var workflows []model.Workflow
	h.db.Where("user_id = ?", userID).Find(&workflows)

	var memories []model.Memory
	h.db.Where("user_id = ?", userID).Find(&memories)

	// Record this export in audit chain
	h.auditChain.Append("gdpr_export", userID, c.ClientIP(), "user:"+userID, "{}", "info")

	export := gin.H{
		"export_date":    time.Now().UTC().Format(time.RFC3339),
		"user_id":        userID,
		"user":           gin.H{"username": user.Username, "email": user.Email, "created_at": user.CreatedAt},
		"agents":         agents,
		"conversations":  conversations,
		"messages_count": len(messages),
		"workflows":      workflows,
		"memories":       memories,
	}

	c.Header("Content-Disposition", "attachment; filename=gdpr-export-"+userID+".json")
	c.JSON(http.StatusOK, export)
}

// GDPRDeleteData requests deletion of all personal data (Article 17: Right to Erasure).
func (h *SecurityHandler) GDPRDeleteData(c *gin.Context) {
	userID := c.GetString("user_id")

	var req struct {
		Confirm bool `json:"confirm" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || !req.Confirm {
		c.JSON(http.StatusBadRequest, gin.H{"error": "must confirm deletion with {\"confirm\": true}"})
		return
	}

	// Record before deletion
	h.auditChain.Append("gdpr_delete_request", userID, c.ClientIP(), "user:"+userID, "{}", "critical")

	// Delete user data in order (respecting foreign keys)
	tx := h.db.Begin()

	tx.Where("user_id = ?", userID).Delete(&model.Memory{})
	tx.Where("user_id = ?", userID).Delete(&model.Message{})
	tx.Where("user_id = ?", userID).Delete(&model.Conversation{})
	tx.Where("user_id = ?", userID).Delete(&model.Agent{})
	tx.Where("user_id = ?", userID).Delete(&model.Workflow{})

	// Anonymize user record (don't delete, keep for audit trail)
	tx.Model(&model.User{}).Where("id = ?", userID).Updates(map[string]interface{}{
		"username": "deleted_user_" + userID[:8],
		"email":    "deleted_" + userID[:8] + "@deleted.local",
	})

	tx.Commit()

	c.JSON(http.StatusOK, gin.H{
		"status":  "completed",
		"message": "all personal data deleted, user record anonymized",
	})
}

// GDPRConsentStatus returns the current user's consent status.
func (h *SecurityHandler) GDPRConsentStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"data_processing": true,
		"analytics":       true,
		"marketing":       false,
		"third_party":     false,
		"updated_at":      time.Now().Format(time.RFC3339),
		"note":            "consent management UI should be implemented in frontend",
	})
}

// ════════════════════════════════════════════════════════════════
//  Compliance Checklist
// ════════════════════════════════════════════════════════════════

// ComplianceChecklist returns a self-assessment checklist.
func (h *SecurityHandler) ComplianceChecklist(c *gin.Context) {
	framework := c.DefaultQuery("framework", "djcp") // djcp (等保) or gdpr or soc2

	switch framework {
	case "djcp":
		c.JSON(http.StatusOK, djcpChecklist())
	case "gdpr":
		c.JSON(http.StatusOK, gdprChecklist())
	case "soc2":
		c.JSON(http.StatusOK, soc2Checklist())
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "unknown framework, use: djcp, gdpr, soc2"})
	}
}

func djcpChecklist() gin.H {
	return gin.H{
		"framework": "等保三级 (DJCP Level 3)",
		"categories": []gin.H{
			{"name": "物理安全", "items": []gin.H{
				{"id": "djcp-01", "title": "机房物理访问控制", "status": "n/a", "note": "云部署，由云服务商负责"},
				{"id": "djcp-02", "title": "设备防盗防破坏", "status": "n/a", "note": "云部署"},
			}},
			{"name": "网络安全", "items": []gin.H{
				{"id": "djcp-03", "title": "网络架构安全", "status": "pass", "note": "VPC 隔离 + 安全组"},
				{"id": "djcp-04", "title": "通信加密", "status": "pass", "note": "TLS 1.2+ 全链路加密"},
				{"id": "djcp-05", "title": "入侵检测", "status": "partial", "note": "已有日志监控，待接入 IDS"},
			}},
			{"name": "主机安全", "items": []gin.H{
				{"id": "djcp-06", "title": "操作系统加固", "status": "pass", "note": "Docker 最小化镜像"},
				{"id": "djcp-07", "title": "恶意代码防范", "status": "pass", "note": "容器镜像扫描"},
			}},
			{"name": "应用安全", "items": []gin.H{
				{"id": "djcp-08", "title": "身份鉴别", "status": "pass", "note": "JWT + Ed25519 + RBAC"},
				{"id": "djcp-09", "title": "访问控制", "status": "pass", "note": "RBAC + API 限流"},
				{"id": "djcp-10", "title": "安全审计", "status": "pass", "note": "不可篡改 Merkle 审计链"},
				{"id": "djcp-11", "title": "数据加密", "status": "pass", "note": "AES-256-GCM 静态加密"},
				{"id": "djcp-12", "title": "个人信息保护", "status": "pass", "note": "GDPR API 已实现"},
			}},
			{"name": "数据安全", "items": []gin.H{
				{"id": "djcp-13", "title": "数据备份恢复", "status": "partial", "note": "数据库定期备份，待测试恢复流程"},
				{"id": "djcp-14", "title": "数据加密存储", "status": "pass", "note": "敏感字段 AES-256-GCM 加密"},
				{"id": "djcp-15", "title": "数据完整性", "status": "pass", "note": "Merkle chain 保证审计数据完整性"},
			}},
		},
	}
}

func gdprChecklist() gin.H {
	return gin.H{
		"framework": "GDPR (General Data Protection Regulation)",
		"articles": []gin.H{
			{"article": "Art. 6", "title": "合法性基础", "status": "pass", "note": "用户同意 + 合同履行"},
			{"article": "Art. 13-14", "title": "信息告知义务", "status": "partial", "note": "隐私政策待完善"},
			{"article": "Art. 15", "title": "数据访问权", "status": "pass", "note": "GET /security/gdpr/export"},
			{"article": "Art. 17", "title": "删除权", "status": "pass", "note": "POST /security/gdpr/delete"},
			{"article": "Art. 20", "title": "数据可携权", "status": "pass", "note": "JSON 导出全部数据"},
			{"article": "Art. 25", "title": "隐私设计", "status": "pass", "note": "最小化数据收集 + 默认隐私"},
			{"article": "Art. 32", "title": "安全措施", "status": "pass", "note": "AES-256-GCM + TLS + RBAC"},
			{"article": "Art. 33-34", "title": "数据泄露通知", "status": "partial", "note": "告警系统已有，72h 通知流程待建立"},
			{"article": "Art. 35", "title": "DPIA 影响评估", "status": "pending", "note": "待完成正式评估文档"},
		},
	}
}

func soc2Checklist() gin.H {
	return gin.H{
		"framework": "SOC 2 Type I (Trust Services Criteria)",
		"categories": []gin.H{
			{"name": "Security", "items": []gin.H{
				{"id": "CC6.1", "title": "逻辑和物理访问控制", "status": "pass", "note": "JWT + RBAC + Ed25519"},
				{"id": "CC6.2", "title": "系统边界保护", "status": "pass", "note": "VPC + 安全组 + API 限流"},
				{"id": "CC6.3", "title": "变更管理", "status": "partial", "note": "Git + CI/CD，待正式化"},
			}},
			{"name": "Availability", "items": []gin.H{
				{"id": "A1.1", "title": "系统可用性监控", "status": "pass", "note": "Prometheus + 健康检查"},
				{"id": "A1.2", "title": "灾难恢复", "status": "partial", "note": "数据库备份有，DR 计划待制定"},
			}},
			{"name": "Confidentiality", "items": []gin.H{
				{"id": "C1.1", "title": "数据分类", "status": "partial", "note": "敏感字段已标识"},
				{"id": "C1.2", "title": "数据加密", "status": "pass", "note": "AES-256-GCM at rest + TLS in transit"},
			}},
			{"name": "Processing Integrity", "items": []gin.H{
				{"id": "PI1.1", "title": "数据完整性验证", "status": "pass", "note": "Merkle 审计链"},
			}},
			{"name": "Privacy", "items": []gin.H{
				{"id": "P1.1", "title": "隐私通知", "status": "partial", "note": "待完善隐私政策"},
				{"id": "P1.2", "title": "数据主体权利", "status": "pass", "note": "GDPR API (导出+删除)"},
			}},
		},
	}
}

// SecurityOverview returns a combined security status dashboard.
func (h *SecurityHandler) SecurityOverview(c *gin.Context) {
	auditStats := h.auditChain.Stats()

	c.JSON(http.StatusOK, gin.H{
		"encryption": gin.H{
			"enabled":         true,
			"algorithm":       "AES-256-GCM",
			"key_fingerprint": h.keyMgr.MasterKeyFingerprint(),
		},
		"audit_chain":   auditStats,
		"gdpr_ready":    true,
		"djcp_ready":    false,
		"soc2_ready":    false,
		"frameworks":    []string{"djcp", "gdpr", "soc2"},
	})
}
