package handler

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/yinhe/starclaw/hive/config"
	"github.com/yinhe/starclaw/hive/model"
	"github.com/yinhe/starclaw/hive/service"
	"gorm.io/gorm"
)

var slugRegex = regexp.MustCompile(`^[a-z][a-z0-9-]{1,28}[a-z0-9]$`)

type HiveHandler struct {
	db     *gorm.DB
	cfg    *config.Config
	docker *service.DockerService
	mysql  *service.MySQLService
	nginx  *service.NginxService
}

func NewHiveHandler(db *gorm.DB, cfg *config.Config, docker *service.DockerService, mysql *service.MySQLService, nginx *service.NginxService) *HiveHandler {
	return &HiveHandler{db: db, cfg: cfg, docker: docker, mysql: mysql, nginx: nginx}
}

// ──── Create Claw Instance ────

type CreateRequest struct {
	Slug        string `json:"slug" binding:"required"`
	DisplayName string `json:"display_name"`
	OwnerEmail  string `json:"owner_email"`
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

	// Check free tier limit
	var count int64
	h.db.Model(&model.ClawInstance{}).Where("status != 'destroying'").Count(&count)
	if int(count) >= h.cfg.MaxFreeInstances {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "蜂巢已满，请稍后重试或选择云服务器模式"})
		return
	}

	// Allocate port
	port, err := h.allocatePort()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "无可用端口"})
		return
	}

	// Create instance record
	inst := model.ClawInstance{
		ID:          uuid.New().String(),
		Slug:        slug,
		DisplayName: req.DisplayName,
		OwnerEmail:  req.OwnerEmail,
		DeployMode:  "hive",
		Port:        port,
		Status:      "creating",
		CPULimit:    0.5,
		MemoryLimit: 512 * 1024 * 1024,      // 512MB
		StorageMax:  2 * 1024 * 1024 * 1024, // 2GB
		JWTSecret:   randomHex(32),
	}

	// Set expiry for free tier
	if h.cfg.FreeTierExpireDays > 0 {
		exp := time.Now().AddDate(0, 0, h.cfg.FreeTierExpireDays)
		inst.ExpiresAt = &exp
	}

	if err := h.db.Create(&inst).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存实例失败"})
		return
	}

	// Async provisioning
	go h.provisionInstance(&inst)

	c.JSON(http.StatusCreated, gin.H{
		"id":      inst.ID,
		"slug":    inst.Slug,
		"url":     fmt.Sprintf("https://%s.%s", inst.Slug, h.cfg.Domain),
		"status":  inst.Status,
		"message": "正在创建 Claw 实例，约 10 秒后可用",
	})
}

func (h *HiveHandler) provisionInstance(inst *model.ClawInstance) {
	updateStatus := func(status string) {
		h.db.Model(inst).Update("status", status)
		inst.Status = status
	}

	// Step 1: Create MySQL database
	dbName, dbUser, dbPass, err := h.mysql.CreateDatabase(inst.Slug)
	if err != nil {
		log.Printf("[hive] failed to create DB for %s: %v", inst.Slug, err)
		updateStatus("error")
		return
	}
	inst.DBName = dbName
	inst.DBUser = dbUser
	inst.DBPassword = dbPass
	h.db.Save(inst)

	// Step 2: Create data directories
	dataDir := filepath.Join(h.cfg.DataDir, "instances", inst.Slug)
	for _, sub := range []string{"identity", "uploads", "workspaces", "images"} {
		os.MkdirAll(filepath.Join(dataDir, sub), 0755)
	}

	// Step 3: Create and start Docker container
	containerID, err := h.docker.CreateContainer(inst)
	if err != nil {
		log.Printf("[hive] failed to create container for %s: %v", inst.Slug, err)
		updateStatus("error")
		return
	}
	inst.ContainerID = containerID
	h.db.Save(inst)

	// Step 4: Generate nginx config
	if err := h.nginx.WriteConfig(inst.Slug, inst.Port); err != nil {
		log.Printf("[hive] failed to write nginx config for %s: %v", inst.Slug, err)
		updateStatus("error")
		return
	}

	// Step 5: Test and reload nginx
	if err := h.nginx.TestConfig(); err != nil {
		log.Printf("[hive] nginx config test failed: %v", err)
		h.nginx.RemoveConfig(inst.Slug)
		updateStatus("error")
		return
	}
	if err := h.nginx.Reload(); err != nil {
		log.Printf("[hive] nginx reload failed: %v", err)
	}

	// Step 6: Wait for health check
	if err := h.docker.WaitHealthy(inst.Port, 60*time.Second); err != nil {
		log.Printf("[hive] health check failed for %s: %v", inst.Slug, err)
		updateStatus("error")
		return
	}

	// Done!
	updateStatus("running")
	now := time.Now()
	inst.LastActiveAt = &now
	h.db.Save(inst)

	log.Printf("[hive] ✅ instance %s ready at https://%s.%s", inst.Slug, inst.Slug, h.cfg.Domain)
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
	// Find the highest used port
	var maxPort struct{ Port int }
	h.db.Model(&model.ClawInstance{}).
		Where("status != 'destroying' AND deploy_mode = 'hive'").
		Select("COALESCE(MAX(port), ?) as port", h.cfg.PortRangeStart-1).
		Scan(&maxPort)

	next := maxPort.Port + 1
	if next > h.cfg.PortRangeEnd {
		// Try to find a gap
		var usedPorts []int
		h.db.Model(&model.ClawInstance{}).
			Where("status != 'destroying' AND deploy_mode = 'hive'").
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

func randomHex(length int) string {
	b := make([]byte, length)
	rand.Read(b)
	return hex.EncodeToString(b)
}
