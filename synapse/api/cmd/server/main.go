package main

import (
	"fmt"
	"log"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw-router/internal/billing"
	"github.com/yinhe/starclaw-router/internal/config"
	"github.com/yinhe/starclaw-router/internal/database"
	"github.com/yinhe/starclaw-router/internal/handler"
	"github.com/yinhe/starclaw-router/internal/middleware"
	"github.com/yinhe/starclaw-router/internal/provider"
	"github.com/yinhe/starclaw-router/internal/proxy"
)

func main() {
	// Load config
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("[star-ai] failed to load config: %v", err)
	}

	if cfg.Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Database
	db, err := database.InitMySQL(cfg)
	if err != nil {
		log.Fatalf("[star-ai] MySQL: %v", err)
	}
	database.AutoMigrate(db)

	// Redis
	rdb, err := database.InitRedis(cfg)
	if err != nil {
		log.Printf("[star-ai] Redis unavailable: %v (rate limiting disabled)", err)
	}

	// Proxy client (Node.js overseas relay)
	proxyClient := proxy.NewClient(cfg.Proxy.URL, cfg.Proxy.SecretKey)

	// Provider registry (load YAML configs)
	reg := provider.NewRegistry()
	if err := reg.LoadDir("./providers"); err != nil {
		log.Printf("[star-ai] warning: failed to load providers: %v", err)
	}

	// Billing meter
	meter := billing.NewMeter(db, reg)

	// Queen credit client (star energy billing for Claw signature auth)
	queenCredit := billing.NewQueenCreditClient(cfg.Queen)
	if queenCredit.Enabled() {
		meter.SetQueenCredit(queenCredit)
		log.Printf("[star-ai] Queen credit client enabled (url=%s)", cfg.Queen.URL)
	} else {
		log.Println("[star-ai] Queen credit client not configured — Claw signature auth will be unavailable")
	}

	// Gin router
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.RequestLogger())
	r.Use(middleware.PrometheusMetrics())
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Authorization", "Content-Type", "X-Claw-ID", "X-Claw-PubKey", "X-Claw-Signature", "X-Claw-Timestamp"},
		ExposeHeaders:    []string{"X-RateLimit-Limit", "X-RateLimit-Remaining"},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	}))

	// Prometheus metrics endpoint (no auth)
	r.GET("/metrics", middleware.MetricsHandler())

	// Health check (no auth)
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "star-ai"})
	})

	// ── Auth routes (no auth required) ──
	authHandler := handler.NewAuthHandler(db, cfg.JWT.Secret, cfg.JWT.ExpireHours)
	if queenCredit.Enabled() {
		authHandler.SetQueenCredit(queenCredit)
	}
	r.POST("/auth/register", authHandler.Register)
	r.POST("/auth/login", authHandler.Login)

	// ── Claw address auth (MetaMask-style, same as Queen community) ──
	clawAuth := handler.NewClawAuthHandler(db, cfg.JWT.Secret, cfg.JWT.ExpireHours)
	r.POST("/auth/claw/challenge", clawAuth.Challenge)
	r.POST("/auth/claw/verify", clawAuth.Verify)

	// ── Dashboard routes (JWT auth) ──
	dash := r.Group("/dash")
	dash.Use(middleware.JWTAuth(cfg.JWT.Secret))
	dash.GET("/profile", authHandler.Profile)
	dash.PUT("/profile", authHandler.UpdateProfile)
	dash.POST("/password", authHandler.ChangePassword)

	// Dashboard can also manage keys/usage/balance via JWT
	dashKeys := handler.NewKeysHandler(db)
	dash.GET("/keys", dashKeys.List)
	dash.POST("/keys", dashKeys.Create)
	dash.DELETE("/keys/:id", dashKeys.Delete)

	dashUsage := handler.NewUsageHandler(db, queenCredit)
	dash.GET("/usage", dashUsage.Query)
	dash.GET("/logs", dashUsage.Logs)
	dash.GET("/balance", dashUsage.Balance)
	dash.GET("/tool-usage", dashUsage.ToolUsage)

	// Payment (JWT auth for creating orders, public for callbacks)
	payHandler := handler.NewPaymentHandler(db, cfg.Alipay, cfg.Wechat, cfg.Queen)
	dash.GET("/pay/packages", payHandler.Packages)
	dash.POST("/pay/alipay", payHandler.CreateAlipay)
	dash.POST("/pay/wechat", payHandler.CreateWechat)
	dash.GET("/pay/orders", payHandler.Orders)
	dash.GET("/pay/query", payHandler.QueryOrder)
	dash.POST("/pay/sync", payHandler.SyncPendingOrders)

	// Payment callbacks (called by Alipay/WeChat servers — no auth)
	r.POST("/pay/callback/alipay", payHandler.CallbackAlipay)
	r.POST("/pay/callback/wechat", payHandler.CallbackWechat)

	// ── Internal routes (service-to-service, token auth) ──
	internal := r.Group("/internal")
	internal.Use(func(c *gin.Context) {
		token := c.GetHeader("X-Internal-Token")
		if token == "" || token != cfg.Queen.Token {
			c.AbortWithStatusJSON(403, gin.H{"error": "unauthorized"})
			return
		}
		c.Next()
	})
	internal.POST("/payment/invest-order", payHandler.CreateInvestOrder)

	// ── Admin routes (JWT + RBAC) ──
	adminHandler := handler.NewAdminHandler(db, reg)
	admin := r.Group("/admin")
	admin.Use(middleware.JWTAuth(cfg.JWT.Secret))
	admin.Use(middleware.RBACAuth(db))
	admin.GET("/overview", middleware.RequirePermission("view_overview"), adminHandler.Overview)
	admin.GET("/users", middleware.RequirePermission("view_users"), adminHandler.ListUsers)
	admin.GET("/users/:id", middleware.RequirePermission("view_users"), adminHandler.GetUser)
	admin.PUT("/users/:id", middleware.RequirePermission("manage_users"), adminHandler.UpdateUser)
	admin.GET("/logs", middleware.RequirePermission("view_logs"), adminHandler.AllLogs)
	admin.GET("/orders", middleware.RequirePermission("view_orders"), adminHandler.AllOrders)
	admin.GET("/providers", middleware.RequirePermission("view_providers"), adminHandler.ListProviders)
	// RBAC management
	admin.GET("/roles", middleware.RequirePermission("manage_roles"), adminHandler.ListRoles)
	admin.GET("/roles/:id", middleware.RequirePermission("manage_roles"), adminHandler.GetRole)
	admin.POST("/users/:id/roles", middleware.RequirePermission("manage_roles"), adminHandler.AssignRole)
	admin.DELETE("/users/:id/roles/:role_id", middleware.RequirePermission("manage_roles"), adminHandler.RevokeRole)
	admin.GET("/permissions", middleware.RequirePermission("manage_roles"), adminHandler.ListPermissions)
	admin.GET("/me", adminHandler.AdminMe)

	// ── Claw API key provisioning (Ed25519 signature auth only) ──
	clawProv := handler.NewClawProvisionHandler(db)
	clawGroup := r.Group("/v1/claw")
	clawGroup.Use(middleware.ClawSignatureAuth())
	clawGroup.POST("/provision", clawProv.Provision)
	clawGroup.POST("/rotate-key", clawProv.RotateKey)
	clawGroup.GET("/sync", clawProv.Sync)

	// ── OpenAI-compatible API routes (API Key OR Claw signature auth) ──
	v1 := r.Group("/v1")
	v1.Use(middleware.DualAuth(db))
	if rdb != nil {
		v1.Use(middleware.RateLimit(rdb, 60, time.Minute)) // 60 req/min per key
	}

	chatHandler := handler.NewChatHandler(db, proxyClient, reg, meter)
	v1.POST("/chat/completions", chatHandler.ChatCompletions)

	modelsHandler := handler.NewModelsHandler(reg)
	v1.GET("/models", modelsHandler.ListModels)

	keysHandler := handler.NewKeysHandler(db)
	v1.GET("/keys", keysHandler.List)
	v1.POST("/keys", keysHandler.Create)
	v1.DELETE("/keys/:id", keysHandler.Delete)

	usageHandler := handler.NewUsageHandler(db, queenCredit)
	v1.GET("/usage", usageHandler.Query)
	v1.GET("/balance", usageHandler.Balance)
	v1.GET("/tool-usage", usageHandler.ToolUsage)

	proxyHandler := handler.NewProxyHandler(proxyClient)
	v1.POST("/images/generations", proxyHandler.Forward)
	v1.POST("/audio/speech", proxyHandler.Forward)
	v1.POST("/audio/transcriptions", proxyHandler.Forward)
	v1.POST("/embeddings", chatHandler.Embeddings)

	// ── Generations tracking ──
	genHandler := handler.NewGenerationHandler(db, reg, meter)
	v1.GET("/generations", genHandler.ListGenerations)
	v1.GET("/generations/stats", genHandler.GenerationStats)
	v1.GET("/generations/:id", genHandler.GetGeneration)

	// Dashboard generations
	dash.GET("/generations", genHandler.ListGenerations)
	dash.GET("/generations/stats", genHandler.GenerationStats)

	// ── Provider Proxy (StarAI super router for all model types) ──
	// Claw tools call /v1/proxy/:provider/*path, Router injects API key and forwards
	providerProxy := handler.NewProviderProxyHandler(reg)
	providerProxy.SetGenerationHandler(genHandler)
	v1.Any("/proxy/:provider/*path", providerProxy.Forward)

	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	log.Printf("[star-ai] API starting on %s (proxy → %s)", addr, cfg.Proxy.URL)
	if err := r.Run(addr); err != nil {
		log.Fatalf("[star-ai] failed to start: %v", err)
	}
}
