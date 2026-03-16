package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/yinhe/starclaw-overlord/manager/internal/handler"
	"github.com/yinhe/starclaw-overlord/manager/internal/middleware"
	"github.com/yinhe/starclaw-overlord/manager/internal/model"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func main() {
	dsn := getEnv("OVERLORD_DSN", "root:starclaw@tcp(127.0.0.1:3306)/starclaw_overlord?charset=utf8mb4&parseTime=True&loc=Local")
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect database: %v", err)
	}

	// Auto-migrate all models
	db.AutoMigrate(
		&model.ClawNode{}, &model.TaskAssignment{}, &model.AuditLog{},
		&model.Team{}, &model.AdminUser{},
		&model.NydusTunnel{},
		&model.MoltRelease{}, &model.MoltNodeStatus{},
		&model.Webhook{}, &model.WebhookLog{},
	)

	// Seed default superadmin if none exists
	seedSuperAdmin(db)

	mode := getEnv("GIN_MODE", "debug")
	if mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.Default()
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "X-Admin-User", "X-Admin-Token"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Handlers
	regH := handler.NewRegistryHandler(db)
	regH.MaxNodes = getEnvInt("OVERLORD_MAX_NODES", 10) // Community=10; 0=unlimited
	teamH := handler.NewTeamHandler(db)
	nydusH := handler.NewNydusHandler(db)
	moltH := handler.NewMoltHandler(db)
	webhookH := handler.NewWebhookHandler(db)

	// Wire webhook dispatcher into registry handler
	regH.Dispatcher = webhookH

	// Offline detector (with webhook dispatch)
	go offlineDetector(db, webhookH)

	brood := r.Group("/brood")
	brood.Use(middleware.AdminAuth(db))
	{
		// --- Public Claw-facing endpoints (no auth required) ---
		brood.POST("/register", regH.Register)
		brood.POST("/heartbeat", regH.Heartbeat)

		// --- Auth: login ---
		brood.POST("/auth/login", teamH.Login)

		// --- Read endpoints (viewer+) ---
		read := brood.Group("")
		read.Use(middleware.RequirePermission("claws.read"))
		{
			read.GET("/claws", regH.ListClaws)
			read.GET("/claws/:id", regH.GetClaw)
			read.GET("/stats", regH.Stats)
			read.GET("/audit", regH.AuditLogs)
			read.GET("/resolve", regH.Resolve)
		}

		// --- Write endpoints (operator+) ---
		write := brood.Group("")
		write.Use(middleware.RequirePermission("claws.write"))
		{
			write.PUT("/claws/:id/quota", regH.UpdateQuota)
			write.POST("/task/assign", regH.AssignTask)
		}

		// --- Delete endpoints (admin+) ---
		del := brood.Group("")
		del.Use(middleware.RequirePermission("claws.delete"))
		{
			del.DELETE("/claws/:id", regH.RemoveClaw)
		}

		// --- Teams (admin+) ---
		teams := brood.Group("/teams")
		teams.Use(middleware.RequirePermission("teams.read"))
		{
			teams.GET("", teamH.ListTeams)
			teams.GET("/:id", teamH.GetTeam)
		}
		teamsWrite := brood.Group("/teams")
		teamsWrite.Use(middleware.RequirePermission("teams.write"))
		{
			teamsWrite.POST("", teamH.CreateTeam)
			teamsWrite.PUT("/:id", teamH.UpdateTeam)
			teamsWrite.DELETE("/:id", teamH.DeleteTeam)
		}

		// --- Admin users (superadmin only) ---
		admins := brood.Group("/admins")
		admins.Use(middleware.RequirePermission("*"))
		{
			admins.GET("", teamH.ListAdmins)
			admins.POST("", teamH.CreateAdmin)
			admins.DELETE("/:id", teamH.DeleteAdmin)
		}

		// --- Nydus Tunnels ---
		tunnelsRead := brood.Group("/tunnels")
		tunnelsRead.Use(middleware.RequirePermission("nydus.read"))
		{
			tunnelsRead.GET("", nydusH.ListTunnels)
			tunnelsRead.GET("/:id", nydusH.GetTunnel)
		}
		tunnelsWrite := brood.Group("/tunnels")
		tunnelsWrite.Use(middleware.RequirePermission("nydus.write"))
		{
			tunnelsWrite.POST("", nydusH.CreateTunnel)
			tunnelsWrite.PUT("/:id/status", nydusH.UpdateTunnelStatus)
			tunnelsWrite.PUT("/:id/metrics", nydusH.UpdateTunnelMetrics)
			tunnelsWrite.DELETE("/:id", nydusH.DeleteTunnel)
		}

		// --- Molt update management ---
		moltRead := brood.Group("/molt")
		moltRead.Use(middleware.RequirePermission("molt.read"))
		{
			moltRead.GET("/releases", moltH.ListReleases)
			moltRead.GET("/releases/:id", moltH.GetRelease)
		}
		moltWrite := brood.Group("/molt")
		moltWrite.Use(middleware.RequirePermission("molt.write"))
		{
			moltWrite.POST("/releases", moltH.CreateRelease)
			moltWrite.POST("/releases/:id/rollout", moltH.StartRollout)
		}
		moltApprove := brood.Group("/molt")
		moltApprove.Use(middleware.RequirePermission("molt.approve"))
		{
			moltApprove.POST("/releases/:id/review", moltH.ApproveRelease)
		}
		// Claw-facing: report node update status (no auth — uses node token in heartbeat)
		brood.POST("/molt/node-status", moltH.ReportNodeStatus)

		// --- Webhooks ---
		whRead := brood.Group("/webhooks")
		whRead.Use(middleware.RequirePermission("webhook.read"))
		{
			whRead.GET("", webhookH.ListWebhooks)
			whRead.GET("/:id", webhookH.GetWebhook)
		}
		whWrite := brood.Group("/webhooks")
		whWrite.Use(middleware.RequirePermission("webhook.write"))
		{
			whWrite.POST("", webhookH.CreateWebhook)
			whWrite.PUT("/:id", webhookH.UpdateWebhook)
			whWrite.DELETE("/:id", webhookH.DeleteWebhook)
			whWrite.POST("/:id/test", webhookH.TestWebhook)
		}
	}

	r.GET("/metrics", gin.WrapH(promhttp.Handler()))
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "overlord-manager"})
	})

	port := getEnv("OVERLORD_PORT", "8095")
	log.Printf("[overlord] Manager service starting on :%s", port)
	if err := r.Run(fmt.Sprintf(":%s", port)); err != nil {
		log.Fatalf("Failed to start overlord manager: %v", err)
	}
}

func seedSuperAdmin(db *gorm.DB) {
	var count int64
	db.Model(&model.AdminUser{}).Count(&count)
	if count > 0 {
		return
	}
	password := getEnv("OVERLORD_ADMIN_PASSWORD", "admin123")
	db.Create(&model.AdminUser{
		Username:     "admin",
		PasswordHash: middleware.HashTokenExported(password),
		Role:         "superadmin",
		Email:        "admin@overlord.local",
	})
	log.Printf("[overlord] Default superadmin created (username: admin)")
}

func offlineDetector(db *gorm.DB, dispatcher *handler.WebhookHandler) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		// Detect feral nodes (online → feral after 90s no heartbeat)
		threshold := time.Now().Add(-90 * time.Second)
		var feralNodes []model.ClawNode
		db.Where("status = ? AND last_heartbeat < ?", "online", threshold).Find(&feralNodes)
		if len(feralNodes) > 0 {
			db.Model(&model.ClawNode{}).
				Where("status = ? AND last_heartbeat < ?", "online", threshold).
				Update("status", "feral")
			for _, n := range feralNodes {
				dispatcher.Dispatch("node.feral", map[string]interface{}{
					"node_id": n.ID, "name": n.Name, "claw_id": n.ClawID,
					"team": n.Team, "last_heartbeat": n.LastHeartbeat,
				})
			}
		}

		// Detect offline nodes (feral → offline after 5min)
		offlineThreshold := time.Now().Add(-5 * time.Minute)
		var offlineNodes []model.ClawNode
		db.Where("status = ? AND last_heartbeat < ?", "feral", offlineThreshold).Find(&offlineNodes)
		if len(offlineNodes) > 0 {
			db.Model(&model.ClawNode{}).
				Where("status = ? AND last_heartbeat < ?", "feral", offlineThreshold).
				Update("status", "offline")
			for _, n := range offlineNodes {
				dispatcher.Dispatch("node.offline", map[string]interface{}{
					"node_id": n.ID, "name": n.Name, "claw_id": n.ClawID,
					"team": n.Team, "last_heartbeat": n.LastHeartbeat,
				})
			}
		}
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
