package handler

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"starclaw.net/carapace"
	"starclaw.net/hive/api/config"
	"starclaw.net/hive/api/model"
	"starclaw.net/hive/api/service"
)

var slugRegex = regexp.MustCompile(`^[a-z][a-z0-9-]{1,28}[a-z0-9]$`)

type HiveHandler struct {
	db      *gorm.DB
	cfg     *config.Config
	docker  *service.DockerService
	mysql   *service.MySQLService
	nginx   *service.NginxService
	vault   *carapace.Vault
	billing *service.BillingService
	ecs     *service.ECSService
	dns     *service.DNSService
	ssh     *service.SSHService
}

func NewHiveHandler(db *gorm.DB, cfg *config.Config, docker *service.DockerService, mysql *service.MySQLService, nginx *service.NginxService, vault *carapace.Vault, billing *service.BillingService, ecs *service.ECSService, dns *service.DNSService, ssh *service.SSHService) *HiveHandler {
	return &HiveHandler{db: db, cfg: cfg, docker: docker, mysql: mysql, nginx: nginx, vault: vault, billing: billing, ecs: ecs, dns: dns, ssh: ssh}
}

// ──── Create Claw Instance ────

type CreateRequest struct {
	Slug        string `json:"slug" binding:"required"`
	DisplayName string `json:"display_name"`
	OwnerEmail  string `json:"owner_email"`
	PlanID      string `json:"plan_id"` // empty = free
	ClawID      string `json:"claw_id"` // payer's claw address (required for paid plans)
}

func (h *HiveHandler) CreateInstance(c *gin.Context) {
	var req CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	slug := strings.ToLower(strings.TrimSpace(req.Slug))

	// Validate slug format
	if !slugRegex.MatchString(slug) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "slug 格式无效：3-30位小写字母/数字/连字符，需以字母开头"})
		return
	}

	// Check blacklist
	var blocked model.SubdomainBlacklist
	if err := h.db.Where("subdomain = ?", slug).First(&blocked).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": fmt.Sprintf("子域名 '%s' 已被保留（%s）", slug, blocked.Reason)})
		return
	}

	// Check uniqueness
	var existing model.ClawInstance
	if err := h.db.Where("slug = ?", slug).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": fmt.Sprintf("子域名 '%s' 已被使用", slug)})
		return
	}

	// Resolve plan
	planID := req.PlanID
	if planID == "" {
		planID = "free"
	}
	var plan model.Plan
	if err := h.db.Where("id = ? AND is_active = ?", planID, true).First(&plan).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("套餐 '%s' 不存在", planID)})
		return
	}

	// Paid plan: verify claw_id and check balance via Queen
	var order *model.Order
	if plan.PriceMonthly > 0 {
		if req.ClawID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "付费套餐需要 claw_id（请先登录 StarAI）"})
			return
		}
		if h.billing == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "支付服务未配置"})
			return
		}

		// Check balance
		bal, err := h.billing.GetBalance(req.ClawID)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "查询余额失败: " + err.Error()})
			return
		}
		if bal.Balance < plan.PriceMonthly {
			c.JSON(http.StatusPaymentRequired, gin.H{
				"error":    "星能不足",
				"required": plan.PriceMonthly,
				"balance":  bal.Balance,
			})
			return
		}

		// Freeze credits for first month
		freeze, err := h.billing.Freeze(req.ClawID, plan.PriceMonthly,
			fmt.Sprintf("Hive %s (%s) 首月", slug, plan.DisplayName))
		if err != nil {
			c.JSON(http.StatusPaymentRequired, gin.H{"error": "冻结星能失败: " + err.Error()})
			return
		}

		// Create order
		now := time.Now()
		order = &model.Order{
			ID:          uuid.New().String(),
			ClawID:      req.ClawID,
			PlanID:      planID,
			Type:        "create",
			Amount:      plan.PriceMonthly,
			FreezeID:    freeze.FreezeID,
			Status:      "pending",
			PeriodStart: now,
			PeriodEnd:   now.AddDate(0, 1, 0),
			CreatedAt:   now,
		}
	}

	// Free tier: check capacity
	if plan.PriceMonthly == 0 {
		var count int64
		h.db.Model(&model.ClawInstance{}).Where("status != 'destroying'").Count(&count)
		if int(count) >= h.cfg.MaxFreeInstances {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "蜂巢已满，请稍后重试或选择付费套餐"})
			return
		}
	}

	// Build instance record from plan
	inst := model.ClawInstance{
		ID:          uuid.New().String(),
		Slug:        slug,
		DisplayName: req.DisplayName,
		OwnerEmail:  req.OwnerEmail,
		OwnerID:     req.ClawID,
		DeployMode:  plan.DeployMode,
		Status:      "creating",
		CPULimit:    plan.CPU,
		MemoryLimit: int64(plan.MemoryMB) * 1024 * 1024,
		StorageMax:  int64(plan.StorageGB) * 1024 * 1024 * 1024,
		JWTSecret:   randomHex(32),
	}

	// Hive/Lite mode: allocate local port (both run on this server)
	if plan.DeployMode == "hive" || plan.DeployMode == "lite" {
		port, err := h.allocatePort()
		if err != nil {
			// Unfreeze if paid
			if order != nil {
				h.billing.Unfreeze(req.ClawID, order.Amount, order.FreezeID)
			}
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "无可用端口"})
			return
		}
		inst.Port = port
	}

	// Set expiry for free tier only
	if plan.ExpireDays > 0 {
		exp := time.Now().AddDate(0, 0, plan.ExpireDays)
		inst.ExpiresAt = &exp
	}

	if err := h.db.Create(&inst).Error; err != nil {
		if order != nil {
			h.billing.Unfreeze(req.ClawID, order.Amount, order.FreezeID)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存实例失败"})
		return
	}

	// Save order with instance ID
	if order != nil {
		order.InstanceID = inst.ID
		h.db.Create(order)
	}

	// Async provisioning
	go h.provisionInstance(&inst, order)

	c.JSON(http.StatusCreated, gin.H{
		"id":     inst.ID,
		"slug":   inst.Slug,
		"url":    fmt.Sprintf("https://%s.%s", inst.Slug, h.cfg.Domain),
		"status": inst.Status,
		"plan":   planID,
		"message": fmt.Sprintf("正在创建 Claw 实例，约 %s后可用", func() string {
			if inst.DeployMode == "lite" {
				return "3 秒"
			}
			return "10 秒"
		}()),
	})
}

func (h *HiveHandler) provisionInstance(inst *model.ClawInstance, order *model.Order) {
	updateStatus := func(status string) {
		h.db.Model(inst).Update("status", status)
		inst.Status = status
	}

	// On failure: refund frozen credits if paid order
	refundOnError := func() {
		if order != nil && h.billing != nil && order.Status == "pending" {
			h.billing.Unfreeze(order.ClawID, order.Amount, order.FreezeID)
			h.db.Model(order).Update("status", "failed")
		}
	}

	switch inst.DeployMode {
	case "ecs":
		h.provisionECS(inst, order, updateStatus, refundOnError)
	case "lite":
		h.provisionLite(inst, order, updateStatus, refundOnError)
	default:
		h.provisionHive(inst, order, updateStatus, refundOnError)
	}
}

// provisionHive deploys a Claw instance as a Docker container on the shared Hive server.
func (h *HiveHandler) provisionHive(inst *model.ClawInstance, order *model.Order, updateStatus func(string), refundOnError func()) {
	// Step 1: Create MySQL database
	dbName, dbUser, dbPass, err := h.mysql.CreateDatabase(inst.Slug)
	if err != nil {
		log.Printf("[hive] failed to create DB for %s: %v", inst.Slug, err)
		updateStatus("error")
		refundOnError()
		return
	}
	inst.DBName = dbName
	inst.DBUser = dbUser
	inst.DBPassword = dbPass // plaintext for Docker

	// Step 2: Create data directories
	dataDir := filepath.Join(h.cfg.DataDir, "instances", inst.Slug)
	for _, sub := range []string{"identity", "uploads", "workspaces", "images"} {
		os.MkdirAll(filepath.Join(dataDir, sub), 0755)
	}

	// Step 3: Create and start Docker container (needs plaintext credentials)
	containerID, err := h.docker.CreateContainer(inst)
	if err != nil {
		log.Printf("[hive] failed to create container for %s: %v", inst.Slug, err)
		updateStatus("error")
		refundOnError()
		return
	}
	inst.ContainerID = containerID

	// Encrypt sensitive fields before persisting to DB
	if h.vault != nil {
		if enc, err := h.vault.Seal("db_password", inst.DBPassword); err == nil {
			inst.DBPassword = enc
		}
		if enc, err := h.vault.Seal("jwt_secret", inst.JWTSecret); err == nil {
			inst.JWTSecret = enc
		}
	}
	h.db.Save(inst)

	// Step 4: Wait for health check BEFORE nginx reload (prevents 502)
	if err := h.docker.WaitHealthy(inst.Port, 60*time.Second); err != nil {
		log.Printf("[hive] health check failed for %s: %v", inst.Slug, err)
		updateStatus("error")
		refundOnError()
		return
	}

	// Step 5: Generate nginx config
	if err := h.nginx.WriteConfig(inst.Slug, inst.Port); err != nil {
		log.Printf("[hive] failed to write nginx config for %s: %v", inst.Slug, err)
		updateStatus("error")
		refundOnError()
		return
	}

	// Step 6: Test and reload nginx (container is confirmed healthy, no 502 risk)
	if err := h.nginx.TestConfig(); err != nil {
		log.Printf("[hive] nginx config test failed: %v", err)
		h.nginx.RemoveConfig(inst.Slug)
		updateStatus("error")
		refundOnError()
		return
	}
	if err := h.nginx.Reload(); err != nil {
		log.Printf("[hive] nginx reload failed: %v", err)
	}

	// Step 7: DNS record (if DNS service configured)
	if h.dns != nil && h.cfg.HivePublicIP != "" {
		if recordID, err := h.dns.AddRecord(inst.Slug, h.cfg.HivePublicIP); err != nil {
			log.Printf("[hive] DNS record failed for %s: %v (non-fatal)", inst.Slug, err)
		} else {
			log.Printf("[hive] DNS A record created: %s.%s → %s (id=%s)", inst.Slug, h.cfg.Domain, h.cfg.HivePublicIP, recordID)
		}
	}

	// Done — confirm billing
	h.confirmOrder(inst, order)
	NotifyInstanceCreated(inst.ID, inst.OwnerID, inst.Slug, fmt.Sprintf("%s.%s", inst.Slug, h.cfg.Domain), "hive")
	log.Printf("[hive] instance %s ready at https://%s.%s", inst.Slug, inst.Slug, h.cfg.Domain)
}

// provisionLite deploys a Claw Lite instance (Spark tier) — single container, SQLite, no MySQL/Redis.
func (h *HiveHandler) provisionLite(inst *model.ClawInstance, order *model.Order, updateStatus func(string), refundOnError func()) {
	// Step 1: Create data directories
	dataDir := filepath.Join(h.cfg.DataDir, "instances", inst.Slug)
	for _, sub := range []string{"data", "identity"} {
		os.MkdirAll(filepath.Join(dataDir, sub), 0755)
	}

	// Step 2: Create and start Docker container (lite image, no MySQL needed)
	containerID, err := h.docker.CreateLiteContainer(inst)
	if err != nil {
		log.Printf("[hive] failed to create lite container for %s: %v", inst.Slug, err)
		updateStatus("error")
		refundOnError()
		return
	}
	inst.ContainerID = containerID

	// Encrypt JWT secret before DB save
	if h.vault != nil {
		if enc, err := h.vault.Seal("jwt_secret", inst.JWTSecret); err == nil {
			inst.JWTSecret = enc
		}
	}
	h.db.Save(inst)

	// Step 3: Wait for health check BEFORE nginx reload (prevents 502 on first visit)
	// Use container name on Docker network since controller runs in container too
	containerName := fmt.Sprintf("claw-%s-lite", inst.Slug)
	if err := h.docker.WaitHealthyByName(containerName, 8080, 15*time.Second); err != nil {
		log.Printf("[hive] health check failed for lite %s: %v", inst.Slug, err)
		updateStatus("error")
		refundOnError()
		return
	}

	// Step 4: Generate nginx config
	if err := h.nginx.WriteConfig(inst.Slug, inst.Port); err != nil {
		log.Printf("[hive] failed to write nginx config for %s: %v", inst.Slug, err)
		updateStatus("error")
		refundOnError()
		return
	}

	// Step 5: Test and reload nginx (container is confirmed healthy, no 502 risk)
	if err := h.nginx.TestConfig(); err != nil {
		log.Printf("[hive] nginx config test failed: %v", err)
		h.nginx.RemoveConfig(inst.Slug)
		updateStatus("error")
		refundOnError()
		return
	}
	if err := h.nginx.Reload(); err != nil {
		log.Printf("[hive] nginx reload failed: %v", err)
	}

	// Step 6: DNS record
	if h.dns != nil && h.cfg.HivePublicIP != "" {
		if recordID, err := h.dns.AddRecord(inst.Slug, h.cfg.HivePublicIP); err != nil {
			log.Printf("[hive] DNS record failed for %s: %v (non-fatal)", inst.Slug, err)
		} else {
			log.Printf("[hive] DNS A record created: %s.%s → %s (id=%s)", inst.Slug, h.cfg.Domain, h.cfg.HivePublicIP, recordID)
		}
	}

	// Done — no billing for free tier
	h.confirmOrder(inst, order)
	NotifyInstanceCreated(inst.ID, inst.OwnerID, inst.Slug, fmt.Sprintf("%s.%s", inst.Slug, h.cfg.Domain), "lite")
	log.Printf("[hive] lite instance %s ready at https://%s.%s (3s deploy)", inst.Slug, inst.Slug, h.cfg.Domain)
}

// provisionECS deploys a Claw instance on a dedicated Aliyun ECS server.
func (h *HiveHandler) provisionECS(inst *model.ClawInstance, order *model.Order, updateStatus func(string), refundOnError func()) {
	if h.ecs == nil {
		log.Printf("[hive] ECS service not configured for %s", inst.Slug)
		updateStatus("error")
		refundOnError()
		return
	}

	var plan model.Plan
	if order != nil {
		h.db.Where("id = ?", order.PlanID).First(&plan)
	}

	// Step 1: Create ECS instance
	log.Printf("[hive] creating ECS instance for %s (%.0fC/%dMB)", inst.Slug, inst.CPULimit, inst.MemoryLimit/(1024*1024))
	result, err := h.ecs.CreateInstance(inst.Slug, inst.CPULimit, int(inst.MemoryLimit/(1024*1024)), plan.BandwidthMB)
	if err != nil {
		log.Printf("[hive] ECS creation failed for %s: %v", inst.Slug, err)
		updateStatus("error")
		refundOnError()
		return
	}
	inst.ECSID = result.InstanceID
	h.db.Model(inst).Update("ecs_id", result.InstanceID)

	// Step 2: Allocate public IP
	ip, err := h.ecs.AllocatePublicIP(result.InstanceID)
	if err != nil {
		log.Printf("[hive] allocate IP failed for %s: %v", inst.Slug, err)
		h.ecs.DeleteInstance(result.InstanceID)
		updateStatus("error")
		refundOnError()
		return
	}
	inst.PublicIP = ip
	h.db.Model(inst).Update("public_ip", ip)

	// Step 3: Start ECS
	if err := h.ecs.StartInstance(result.InstanceID); err != nil {
		log.Printf("[hive] start ECS failed for %s: %v", inst.Slug, err)
		h.ecs.DeleteInstance(result.InstanceID)
		updateStatus("error")
		refundOnError()
		return
	}

	// Step 4: DNS record
	if h.dns != nil {
		if recordID, err := h.dns.AddRecord(inst.Slug, ip); err != nil {
			log.Printf("[hive] DNS record failed for %s: %v", inst.Slug, err)
		} else {
			log.Printf("[hive] DNS A record: %s.%s → %s (id=%s)", inst.Slug, h.cfg.Domain, ip, recordID)
		}
	}

	// Step 5: Wait for ECS to become Running + SSH/health check
	log.Printf("[hive] waiting for ECS %s to start...", inst.Slug)
	for i := 0; i < 30; i++ {
		time.Sleep(5 * time.Second)
		info, err := h.ecs.DescribeInstance(result.InstanceID)
		if err == nil && info.Status == "Running" {
			break
		}
	}

	// Encrypt JWT secret
	if h.vault != nil {
		if enc, err := h.vault.Seal("jwt_secret", inst.JWTSecret); err == nil {
			inst.JWTSecret = enc
		}
	}
	h.db.Save(inst)

	// Done — confirm billing
	h.confirmOrder(inst, order)
	NotifyInstanceCreated(inst.ID, inst.OwnerID, inst.Slug, fmt.Sprintf("%s.%s", inst.Slug, h.cfg.Domain), "ecs")
	log.Printf("[hive] ECS instance %s ready at https://%s.%s (IP: %s)", inst.Slug, inst.Slug, h.cfg.Domain, ip)
}

// confirmOrder marks an order as paid and confirms the credit deduction.
func (h *HiveHandler) confirmOrder(inst *model.ClawInstance, order *model.Order) {
	now := time.Now()
	inst.LastActiveAt = &now
	h.db.Model(inst).Updates(map[string]interface{}{"status": "running", "last_active_at": now})

	if order != nil && h.billing != nil {
		// Consume the frozen credits (freeze → consume)
		_, err := h.billing.Consume(order.ClawID, order.Amount, "hive_instance",
			fmt.Sprintf("Hive %s 首月费用", inst.Slug))
		if err != nil {
			log.Printf("[hive] billing consume failed for %s: %v (credits still frozen)", inst.Slug, err)
		}
		paidAt := time.Now()
		h.db.Model(order).Updates(map[string]interface{}{"status": "paid", "paid_at": paidAt})
	}
}

// ──── Plans ────

func (h *HiveHandler) ListPlans(c *gin.Context) {
	var plans []model.Plan
	h.db.Where("is_active = ?", true).Order("price_monthly ASC").Find(&plans)
	c.JSON(http.StatusOK, gin.H{"plans": plans})
}

// ──── Balance Check ────

func (h *HiveHandler) CheckBalance(c *gin.Context) {
	clawID := c.Query("claw_id")
	if clawID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 claw_id 参数"})
		return
	}
	if h.billing == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "支付服务未配置"})
		return
	}
	bal, err := h.billing.GetBalance(clawID)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, bal)
}

// ──── List Instances ────

func (h *HiveHandler) ListInstances(c *gin.Context) {
	var instances []model.ClawInstance
	query := h.db.Where("status != 'destroying'").Order("created_at DESC")

	// Filter by owner if provided
	if email := c.Query("owner"); email != "" {
		query = query.Where("owner_email = ?", email)
	}

	if err := query.Find(&instances).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"instances": instances, "total": len(instances)})
}

// ──── Get Instance ────

func (h *HiveHandler) GetInstance(c *gin.Context) {
	slug := c.Param("slug")
	var inst model.ClawInstance
	if err := h.db.Where("slug = ?", slug).First(&inst).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "实例不存在"})
		return
	}
	c.JSON(http.StatusOK, inst)
}

// ──── Stop / Start / Restart ────

func (h *HiveHandler) StopInstance(c *gin.Context) {
	inst, ok := h.findInstance(c)
	if !ok {
		return
	}
	if inst.ContainerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无容器"})
		return
	}
	if err := h.docker.StopContainer(inst.ContainerID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.db.Model(&inst).Update("status", "stopped")
	NotifyInstanceStopped(inst.ID, inst.Slug)
	c.JSON(http.StatusOK, gin.H{"message": "已停止", "slug": inst.Slug})
}

func (h *HiveHandler) StartInstance(c *gin.Context) {
	inst, ok := h.findInstance(c)
	if !ok {
		return
	}
	if inst.ContainerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无容器"})
		return
	}
	if err := h.docker.StartContainer(inst.ContainerID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.db.Model(&inst).Update("status", "running")
	NotifyInstanceStarted(inst.ID, inst.Slug)
	c.JSON(http.StatusOK, gin.H{"message": "已启动", "slug": inst.Slug})
}

func (h *HiveHandler) RestartInstance(c *gin.Context) {
	inst, ok := h.findInstance(c)
	if !ok {
		return
	}
	if inst.ContainerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无容器"})
		return
	}
	if err := h.docker.RestartContainer(inst.ContainerID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.db.Model(&inst).Update("status", "running")
	c.JSON(http.StatusOK, gin.H{"message": "已重启", "slug": inst.Slug})
}

// ──── Destroy Instance ────

func (h *HiveHandler) DestroyInstance(c *gin.Context) {
	inst, ok := h.findInstance(c)
	if !ok {
		return
	}

	h.db.Model(&inst).Update("status", "destroying")

	go func() {
		// Remove container
		if inst.ContainerID != "" {
			if err := h.docker.RemoveContainer(inst.ContainerID); err != nil {
				log.Printf("[hive] warning: remove container %s: %v", inst.Slug, err)
			}
		}
		// Remove nginx config
		h.nginx.RemoveConfig(inst.Slug)
		h.nginx.Reload()
		// Drop database
		h.mysql.DropDatabase(inst.Slug)
		// Remove data directory
		dataDir := filepath.Join(h.cfg.DataDir, "instances", inst.Slug)
		os.RemoveAll(dataDir)
		// Soft delete record
		h.db.Delete(&inst)
		NotifyInstanceDeleted(inst.ID, inst.Slug)
		log.Printf("[hive] 🗑️ instance %s destroyed", inst.Slug)
	}()

	c.JSON(http.StatusOK, gin.H{"message": "正在销毁", "slug": inst.Slug})
}

// ──── Admin: Stats ────

func (h *HiveHandler) GetStats(c *gin.Context) {
	var total, running, stopped, errCount int64
	h.db.Model(&model.ClawInstance{}).Count(&total)
	h.db.Model(&model.ClawInstance{}).Where("status = 'running'").Count(&running)
	h.db.Model(&model.ClawInstance{}).Where("status = 'stopped'").Count(&stopped)
	h.db.Model(&model.ClawInstance{}).Where("status = 'error'").Count(&errCount)

	c.JSON(http.StatusOK, gin.H{
		"total":    total,
		"running":  running,
		"stopped":  stopped,
		"error":    errCount,
		"capacity": h.cfg.MaxFreeInstances,
		"port_range": gin.H{
			"start": h.cfg.PortRangeStart,
			"end":   h.cfg.PortRangeEnd,
		},
	})
}

// ──── Admin: Cleanup expired instances ────

func (h *HiveHandler) CleanupExpired(c *gin.Context) {
	var expired []model.ClawInstance
	h.db.Where("expires_at IS NOT NULL AND expires_at < ? AND status != 'destroying'", time.Now()).Find(&expired)

	cleaned := 0
	for _, inst := range expired {
		h.db.Model(&inst).Update("status", "destroying")
		go func(i model.ClawInstance) {
			if i.ContainerID != "" {
				h.docker.RemoveContainer(i.ContainerID)
			}
			h.nginx.RemoveConfig(i.Slug)
			h.mysql.DropDatabase(i.Slug)
			dataDir := filepath.Join(h.cfg.DataDir, "instances", i.Slug)
			os.RemoveAll(dataDir)
			h.db.Delete(&i)
			log.Printf("[hive] cleaned expired instance %s", i.Slug)
		}(inst)
		cleaned++
	}

	if cleaned > 0 {
		h.nginx.Reload()
	}

	c.JSON(http.StatusOK, gin.H{"cleaned": cleaned})
}

// ──── Blacklist ────

func (h *HiveHandler) ListBlacklist(c *gin.Context) {
	var list []model.SubdomainBlacklist
	h.db.Order("reason, subdomain").Find(&list)
	c.JSON(http.StatusOK, gin.H{"blacklist": list, "total": len(list)})
}

func (h *HiveHandler) AddBlacklist(c *gin.Context) {
	var req struct {
		Subdomain string `json:"subdomain" binding:"required"`
		Reason    string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	entry := model.SubdomainBlacklist{
		Subdomain: strings.ToLower(req.Subdomain),
		Reason:    req.Reason,
		CreatedAt: time.Now(),
	}
	if err := h.db.Create(&entry).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "已存在"})
		return
	}
	c.JSON(http.StatusCreated, entry)
}

// ──── Molt → Hive: Upgrade Notification ────

// upgradeState tracks debouncing for Molt-triggered upgrades
var (
	upgradeMu       sync.Mutex
	upgradeRunning  bool
	lastUpgradeVer  string
	lastUpgradeTime time.Time
)

// UpgradeNotify receives a version update notification from a Claw container's Molt checker.
// Debounces: ignores duplicate notifications for the same version within 10 minutes.
// Triggers async rolling upgrade of all running hive/lite instances.
func (h *HiveHandler) UpgradeNotify(c *gin.Context) {
	var req struct {
		CurrentVersion string `json:"current_version"`
		LatestVersion  string `json:"latest_version"`
		Source         string `json:"source"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.LatestVersion == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "latest_version required"})
		return
	}

	upgradeMu.Lock()
	// Debounce: same version within 10 minutes → skip
	if req.LatestVersion == lastUpgradeVer && time.Since(lastUpgradeTime) < 10*time.Minute {
		upgradeMu.Unlock()
		c.JSON(http.StatusOK, gin.H{"message": "already scheduled", "version": req.LatestVersion})
		return
	}
	if upgradeRunning {
		upgradeMu.Unlock()
		c.JSON(http.StatusAccepted, gin.H{"message": "upgrade in progress"})
		return
	}
	upgradeRunning = true
	lastUpgradeVer = req.LatestVersion
	lastUpgradeTime = time.Now()
	upgradeMu.Unlock()

	log.Printf("[hive] 🔄 upgrade notification: %s → %s (source: %s)", req.CurrentVersion, req.LatestVersion, req.Source)

	// Pull latest image first, then rolling upgrade in background
	go h.rollingUpgrade(req.LatestVersion)

	c.JSON(http.StatusAccepted, gin.H{
		"message": "upgrade started",
		"version": req.LatestVersion,
	})
}

// rollingUpgrade pulls the latest Docker image and upgrades instances one by one.
// Handles all deploy modes: hive/lite (local Docker) and ecs (remote SSH).
func (h *HiveHandler) rollingUpgrade(version string) {
	defer func() {
		upgradeMu.Lock()
		upgradeRunning = false
		upgradeMu.Unlock()
	}()

	// Step 1: Try pulling latest images (non-fatal — local images built by Nydus hook are fine)
	log.Printf("[hive] attempting to pull latest images (non-fatal if local-only)...")
	for _, img := range []string{h.cfg.ClawImage, h.cfg.ClawLiteImage} {
		out, err := exec.Command("docker", "pull", img).CombinedOutput()
		if err != nil {
			log.Printf("[hive] docker pull %s skipped (local image will be used): %s", img, strings.TrimSpace(string(out)))
		} else {
			log.Printf("[hive] pulled %s", img)
		}
	}

	// Step 1.5: Discover orphaned containers not tracked in DB
	h.discoverOrphanedContainers()

	// Step 2: Find ALL running instances (hive, lite, AND ecs)
	var instances []model.ClawInstance
	h.db.Where("status = 'running' AND deploy_mode IN ('hive','lite','ecs')").Find(&instances)
	if len(instances) == 0 {
		log.Printf("[hive] no running instances to upgrade")
		return
	}

	log.Printf("[hive] rolling upgrade: %d instances → v%s", len(instances), version)

	upgraded, failed := 0, 0
	for _, inst := range instances {
		log.Printf("[hive] upgrading %s (mode=%s)...", inst.Slug, inst.DeployMode)

		var err error
		switch inst.DeployMode {
		case "ecs":
			err = h.upgradeECSInstance(&inst, version)
		default:
			err = h.upgradeLocalInstance(&inst, version)
		}

		if err != nil {
			log.Printf("[hive] ❌ upgrade failed for %s: %v", inst.Slug, err)
			h.db.Model(&inst).Update("status", "error")
			failed++
		} else {
			h.db.Model(&inst).Update("version", version)
			upgraded++
			log.Printf("[hive] ✅ %s upgraded (%d/%d)", inst.Slug, upgraded+failed, len(instances))
		}

		// Small delay between instances to avoid thundering herd
		time.Sleep(2 * time.Second)
	}

	log.Printf("[hive] 🏁 rolling upgrade complete: %d upgraded, %d failed", upgraded, failed)
}

// upgradeLocalInstance upgrades a hive/lite container on the local Docker host.
func (h *HiveHandler) upgradeLocalInstance(inst *model.ClawInstance, version string) error {
	// Stop and remove old container
	if inst.ContainerID != "" {
		h.docker.StopContainer(inst.ContainerID)
		h.docker.RemoveContainer(inst.ContainerID)
	}

	// Recreate with latest image
	decrypted := h.decryptInstance(inst)
	var containerID string
	var err error
	if inst.DeployMode == "lite" {
		containerID, err = h.docker.CreateLiteContainer(decrypted)
	} else {
		containerID, err = h.docker.CreateContainer(decrypted)
	}
	if err != nil {
		return fmt.Errorf("recreate container: %w", err)
	}

	h.db.Model(inst).Update("container_id", containerID)

	// Wait for health before moving to next
	if err := h.docker.WaitHealthy(inst.Port, 60*time.Second); err != nil {
		log.Printf("[hive] ⚠️ health check timeout for %s (container started)", inst.Slug)
	}
	return nil
}

// upgradeECSInstance upgrades a remote ECS Claw via SSH.
func (h *HiveHandler) upgradeECSInstance(inst *model.ClawInstance, version string) error {
	if h.ssh == nil {
		return fmt.Errorf("SSH service not configured")
	}
	if inst.PublicIP == "" {
		return fmt.Errorf("no public IP for ECS instance %s", inst.Slug)
	}
	return h.ssh.UpgradeECS(inst.PublicIP, inst.Slug)
}

// ──── Admin: Upgrade all instances to latest starclaw-api image ────

func (h *HiveHandler) UpgradeInstances(c *gin.Context) {
	// Discover orphaned containers first
	h.discoverOrphanedContainers()

	var instances []model.ClawInstance
	h.db.Where("status = 'running' AND deploy_mode IN ('hive','lite')").Find(&instances)

	if len(instances) == 0 {
		c.JSON(http.StatusOK, gin.H{"message": "没有需要升级的实例", "upgraded": 0})
		return
	}

	type result struct {
		Slug   string `json:"slug"`
		Status string `json:"status"`
		Error  string `json:"error,omitempty"`
	}
	results := make([]result, 0, len(instances))

	for _, inst := range instances {
		log.Printf("[hive] upgrading instance %s (mode=%s, container=%s)", inst.Slug, inst.DeployMode, inst.ContainerID)

		// Stop and remove old container
		if inst.ContainerID != "" {
			h.docker.StopContainer(inst.ContainerID)
			h.docker.RemoveContainer(inst.ContainerID)
		}

		// Recreate with latest image
		decrypted := h.decryptInstance(&inst)
		var containerID string
		var err error
		if inst.DeployMode == "lite" {
			containerID, err = h.docker.CreateLiteContainer(decrypted)
		} else {
			containerID, err = h.docker.CreateContainer(decrypted)
		}

		if err != nil {
			log.Printf("[hive] upgrade failed for %s: %v", inst.Slug, err)
			h.db.Model(&inst).Update("status", "error")
			results = append(results, result{Slug: inst.Slug, Status: "error", Error: err.Error()})
			continue
		}

		h.db.Model(&inst).Update("container_id", containerID)

		// Wait for health
		healthErr := h.docker.WaitHealthy(inst.Port, 60*time.Second)
		if healthErr != nil {
			log.Printf("[hive] health check failed after upgrade for %s: %v", inst.Slug, healthErr)
			results = append(results, result{Slug: inst.Slug, Status: "warning", Error: "容器已创建但健康检查超时"})
		} else {
			results = append(results, result{Slug: inst.Slug, Status: "ok"})
		}

		log.Printf("[hive] ✅ instance %s upgraded (new container: %s)", inst.Slug, containerID[:12])
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  fmt.Sprintf("已升级 %d 个实例", len(instances)),
		"upgraded": len(instances),
		"results":  results,
	})
}

// ──── Container Discovery ────

// discoverOrphanedContainers scans Docker for running claw-* containers that are not tracked
// in the database. This happens when the DB is reset/recreated but containers survive.
// Discovered containers are registered with status=running so upgrades can find them.
func (h *HiveHandler) discoverOrphanedContainers() {
	out, err := exec.Command("docker", "ps", "--filter", "name=claw-", "--format", "{{.ID}}\t{{.Names}}\t{{.Image}}\t{{.Ports}}").CombinedOutput()
	if err != nil {
		log.Printf("[hive] container discovery failed: %v", err)
		return
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	discovered := 0
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 3 {
			continue
		}
		containerID, containerName, image := parts[0], parts[1], parts[2]

		// Extract slug from container name: claw-{slug}-api or claw-{slug}-lite
		slug := containerName
		slug = strings.TrimPrefix(slug, "claw-")
		slug = strings.TrimSuffix(slug, "-api")
		slug = strings.TrimSuffix(slug, "-lite")
		if slug == "" || slug == containerName {
			continue
		}

		// Check if already tracked in DB
		var count int64
		h.db.Model(&model.ClawInstance{}).Where("slug = ?", slug).Count(&count)
		if count > 0 {
			continue
		}

		// Determine deploy mode from container name/image
		deployMode := "hive"
		if strings.HasSuffix(containerName, "-lite") || strings.Contains(image, "lite") {
			deployMode = "lite"
		}

		// Parse port from docker ps output (e.g., "127.0.0.1:9001->8080/tcp")
		port := 0
		if len(parts) >= 4 {
			portStr := parts[3]
			if idx := strings.Index(portStr, ":"); idx >= 0 {
				portStr = portStr[idx+1:]
			}
			if idx := strings.Index(portStr, "->"); idx >= 0 {
				portStr = portStr[:idx]
			}
			fmt.Sscanf(portStr, "%d", &port)
		}

		inst := model.ClawInstance{
			ID:          uuid.New().String(),
			Slug:        slug,
			DisplayName: slug,
			DeployMode:  deployMode,
			Port:        port,
			ContainerID: containerID,
			Status:      "running",
			CPULimit:    0.25,
			MemoryLimit: 268435456,
			StorageMax:  1073741824,
			JWTSecret:   randomHex(32),
		}
		if err := h.db.Create(&inst).Error; err != nil {
			log.Printf("[hive] failed to register orphan %s: %v", slug, err)
			continue
		}
		discovered++
		log.Printf("[hive] discovered orphan container: %s (mode=%s, port=%d, id=%s)", slug, deployMode, port, containerID[:12])
	}

	if discovered > 0 {
		log.Printf("[hive] 🔍 discovered %d orphaned containers", discovered)
	}
}

// ──── Health ────

func (h *HiveHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"service": "hive-controller",
		"status":  "ok",
		"domain":  h.cfg.Domain,
	})
}

// ──── Helpers ────

func (h *HiveHandler) findInstance(c *gin.Context) (model.ClawInstance, bool) {
	slug := c.Param("slug")
	var inst model.ClawInstance
	if err := h.db.Where("slug = ?", slug).First(&inst).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "实例不存在"})
		return inst, false
	}
	return inst, true
}

func (h *HiveHandler) allocatePort() (int, error) {
	// Find the highest used port (both hive and lite modes use local ports)
	var maxPort struct{ Port int }
	h.db.Model(&model.ClawInstance{}).
		Where("status != 'destroying' AND deploy_mode IN ('hive','lite')").
		Select("COALESCE(MAX(port), ?) as port", h.cfg.PortRangeStart-1).
		Scan(&maxPort)

	next := maxPort.Port + 1
	if next > h.cfg.PortRangeEnd {
		// Try to find a gap
		var usedPorts []int
		h.db.Model(&model.ClawInstance{}).
			Where("status != 'destroying' AND deploy_mode IN ('hive','lite')").
			Pluck("port", &usedPorts)
		used := make(map[int]bool)
		for _, p := range usedPorts {
			used[p] = true
		}
		for p := h.cfg.PortRangeStart; p <= h.cfg.PortRangeEnd; p++ {
			if !used[p] {
				return p, nil
			}
		}
		return 0, fmt.Errorf("no ports available in range %d-%d", h.cfg.PortRangeStart, h.cfg.PortRangeEnd)
	}
	return next, nil
}

// decryptInstance returns a copy of the instance with sensitive fields decrypted.
// Use this when you need plaintext credentials (e.g., recreating a container).
func (h *HiveHandler) decryptInstance(inst *model.ClawInstance) *model.ClawInstance {
	if h.vault == nil {
		return inst
	}
	copy := *inst
	if dec, err := h.vault.Unseal("db_password", copy.DBPassword); err == nil {
		copy.DBPassword = dec
	}
	if dec, err := h.vault.Unseal("jwt_secret", copy.JWTSecret); err == nil {
		copy.JWTSecret = dec
	}
	return &copy
}

func randomHex(length int) string {
	b := make([]byte, length)
	rand.Read(b)
	return hex.EncodeToString(b)
}
