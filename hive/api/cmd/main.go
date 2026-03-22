package main

import (
	"fmt"
	"log"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw/carapace"
	"github.com/yinhe/starclaw/carapace/backend"
	"github.com/yinhe/starclaw/hive/api/config"
	"github.com/yinhe/starclaw/hive/api/handler"
	"github.com/yinhe/starclaw/hive/api/model"
	"github.com/yinhe/starclaw/hive/api/service"
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
	if err := db.AutoMigrate(&model.ClawInstance{}, &model.SubdomainBlacklist{}); err != nil {
		log.Fatalf("[hive] auto-migrate failed: %v", err)
	}

	// Seed blacklist
	seedBlacklist(db)

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

	h := handler.NewHiveHandler(db, cfg, dockerSvc, mysqlSvc, nginxSvc, vault)

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
