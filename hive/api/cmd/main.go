package main

import (
	"fmt"
	"log"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"starclaw.net/carapace"
	"starclaw.net/carapace/backend"
	"starclaw.net/hive/api/config"
	"starclaw.net/hive/api/handler"
	"starclaw.net/hive/api/model"
	"starclaw.net/hive/api/service"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func main() {
	cfg := config.Load()

	// Connect to Hive's own database (stored in the shared MySQL)
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/hive_controller?charset=utf8mb4&parseTime=true",
		cfg.MySQLRootUser, cfg.MySQLRootPass, cfg.MySQLHost, cfg.MySQLPort)
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("[hive] database connection failed: %v", err)
	}

	// Auto-migrate
	if err := db.AutoMigrate(&model.ClawInstance{}, &model.SubdomainBlacklist{}, &model.Plan{}, &model.Order{}); err != nil {
		log.Fatalf("[hive] auto-migrate failed: %v", err)
	}

	// Seed blacklist + plans
	seedBlacklist(db)
	seedPlans(db)

	// Init services
	dockerSvc, err := service.NewDockerService(cfg)
	if err != nil {
		log.Fatalf("[hive] docker service init failed: %v", err)
	}
	mysqlSvc, err := service.NewMySQLService(cfg)
	if err != nil {
		log.Fatalf("[hive] mysql service init failed: %v", err)
	}
	nginxSvc := service.NewNginxService(cfg)

	vault, err := carapace.New(carapace.Config{
		Backend: backend.NewEnvBackend("HIVE_MASTER_KEY"),
		Service: "hive",
	})
	if err != nil {
		// Fallback to ephemeral vault if master key not set
		log.Printf("[hive] warning: %v — using ephemeral vault", err)
		vault, _ = carapace.New(carapace.Config{Service: "hive"})
	}

	// Ensure Docker network exists
	if err := dockerSvc.EnsureNetwork(); err != nil {
		log.Printf("[hive] warning: ensure network: %v", err)
	}

	// Queen billing service (optional — needed for paid plans)
	var billingSvc *service.BillingService
	if cfg.QueenURL != "" && cfg.QueenToken != "" {
		billingSvc = service.NewBillingService(cfg.QueenURL, cfg.QueenToken)
		log.Printf("[hive] billing service connected to %s", cfg.QueenURL)
	} else {
		log.Printf("[hive] warning: Queen billing not configured (paid plans disabled)")
	}

	// Aliyun ECS service (optional — needed for pro/enterprise plans)
	var ecsSvc *service.ECSService
	if cfg.AliyunAccessKeyID != "" {
		ecsSvc = service.NewECSService(service.ECSConfig{
			AccessKeyID:     cfg.AliyunAccessKeyID,
			AccessKeySecret: cfg.AliyunAccessKeySecret,
			RegionID:        cfg.AliyunRegionID,
			VPCID:           cfg.AliyunVPCID,
			VSwitchID:       cfg.AliyunVSwitchID,
			SecurityGroupID: cfg.AliyunSecurityGroupID,
			ImageID:         cfg.AliyunImageID,
		})
		log.Printf("[hive] ECS service ready (region: %s)", cfg.AliyunRegionID)
	}

	// Aliyun DNS service (optional — auto DNS record management)
	var dnsSvc *service.DNSService
	if cfg.AliyunAccessKeyID != "" && cfg.AliyunDNSDomain != "" {
		dnsSvc = service.NewDNSService(service.DNSConfig{
			AccessKeyID:     cfg.AliyunAccessKeyID,
			AccessKeySecret: cfg.AliyunAccessKeySecret,
			Domain:          cfg.AliyunDNSDomain,
		})
		log.Printf("[hive] DNS service ready (domain: %s)", cfg.AliyunDNSDomain)
	}

	h := handler.NewHiveHandler(db, cfg, dockerSvc, mysqlSvc, nginxSvc, vault, billingSvc, ecsSvc, dnsSvc)

	// Router
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "X-Hive-Token"},
		AllowCredentials: true,
	}))

	// Health
	r.GET("/hive/health", h.Health)

	// Public API (with rate limiting in production)
	hive := r.Group("/hive")
	{
		hive.GET("/plans", h.ListPlans)
		hive.GET("/balance", h.CheckBalance)
		hive.POST("/claws", h.CreateInstance)
		hive.GET("/claws", h.ListInstances)
		hive.GET("/claws/:slug", h.GetInstance)
		hive.DELETE("/claws/:slug", h.DestroyInstance)
		hive.POST("/claws/:slug/stop", h.StopInstance)
		hive.POST("/claws/:slug/start", h.StartInstance)
		hive.POST("/claws/:slug/restart", h.RestartInstance)
	}

	// Admin API (requires admin token)
	admin := r.Group("/hive/admin", adminAuth(cfg.AdminToken))
	{
		admin.GET("/stats", h.GetStats)
		admin.POST("/cleanup", h.CleanupExpired)
		admin.GET("/blacklist", h.ListBlacklist)
		admin.POST("/blacklist", h.AddBlacklist)
	}

	log.Printf("[hive] 🐝 Hive Controller starting on :%d (domain: %s)", cfg.Port, cfg.Domain)
	if err := r.Run(fmt.Sprintf(":%d", cfg.Port)); err != nil {
		log.Fatalf("[hive] server failed: %v", err)
	}
}

func adminAuth(token string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if token == "" {
			c.Next()
			return
		}
		t := c.GetHeader("X-Hive-Token")
		if t == "" {
			t = c.Query("token")
		}
		if t != token {
			c.JSON(401, gin.H{"error": "unauthorized"})
			c.Abort()
			return
		}
		c.Next()
	}
}

func seedBlacklist(db *gorm.DB) {
	var count int64
	db.Model(&model.SubdomainBlacklist{}).Count(&count)
	if count > 0 {
		return
	}
	entries := model.DefaultBlacklist()
	for _, e := range entries {
		db.Create(&e)
	}
	log.Printf("[hive] seeded %d reserved subdomains", len(entries))
}

func seedPlans(db *gorm.DB) {
	plans := model.DefaultPlans()
	for _, p := range plans {
		var existing model.Plan
		if err := db.Where("id = ?", p.ID).First(&existing).Error; err != nil {
			db.Create(&p)
		} else {
			db.Model(&existing).Updates(map[string]interface{}{
				"display_name":  p.DisplayName,
				"deploy_mode":   p.DeployMode,
				"price_daily":   p.PriceDaily,
				"price_monthly": p.PriceMonthly,
				"cpu":           p.CPU,
				"memory_mb":     p.MemoryMB,
				"storage_gb":    p.StorageGB,
				"expire_days":   p.ExpireDays,
				"is_active":     p.IsActive,
			})
		}
	}
	// Deactivate old plan IDs that are no longer in defaults
	defaultIDs := make([]string, len(plans))
	for i, p := range plans {
		defaultIDs[i] = p.ID
	}
	db.Model(&model.Plan{}).Where("id NOT IN ?", defaultIDs).Update("is_active", false)
	log.Printf("[hive] synced %d plans", len(plans))
}
