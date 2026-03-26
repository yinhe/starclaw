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
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"starclaw.net/overlord/api/internal/handler"
	"starclaw.net/overlord/api/internal/middleware"
	"starclaw.net/overlord/api/internal/model"
	"starclaw.net/overlord/api/internal/ws"
	pheromone "starclaw.net/pheromone/sdk"
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
		&model.Plan{}, &model.Subscription{}, &model.UsageRecord{}, &model.UsageDailySummary{}, &model.BudgetAlert{},
		&model.SSOProvider{}, &model.SSOSession{},
		// P4: White-label + License
		&model.BrandConfig{}, &model.FeatureToggle{}, &model.LicenseKey{},
		&model.ComplianceLog{}, &model.SensitiveWordRule{}, &model.DataFlowRecord{},
		// Team Agent
		&model.TeamAgentTemplate{}, &model.TeamInstance{}, &model.TeamMission{},
		&model.Conversation{}, &model.ChatMessage{},
		&model.EmployeeInvite{}, &model.InstanceAccess{},
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
	billingH := handler.NewBillingHandler(db)
	billingH.SeedDefaultPlans()
	ssoH := handler.NewSSOHandler(db)
	brandH := handler.NewBrandHandler(db)
	brandH.SeedFeatures()
	complianceH := handler.NewComplianceHandler(db)
	wsHub := ws.GetHub()
	teamAgentH := handler.NewTeamAgentHandler(db)
	teamAgentH.SetWSHub(wsHub)
	teamAgentH.SeedOfficialTemplates()
	teamAgentH.StartStatusSyncer()
	provisionH := handler.NewProvisionHandler(db)

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

		// --- Auth: login + registration ---
		brood.POST("/auth/login", teamH.Login)
		brood.POST("/auth/node-login", teamAgentH.NodeLogin)
		brood.POST("/auth/register", teamAgentH.RegisterWithInvite)

		// --- Read endpoints (viewer+) ---
		read := brood.Group("")
		read.Use(middleware.RequirePermission("claws.read"))
		{
			read.GET("/claws", regH.ListClaws)
			read.GET("/claws/:id", regH.GetClaw)
			read.GET("/stats", regH.Stats)
			read.GET("/audit", regH.AuditLogs)
			read.GET("/resolve", regH.Resolve)
			read.GET("/models", teamAgentH.ListModels)
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

		// --- Forge (Nydus proxy + Forge API proxy) ---
		forgeH := handler.NewForgeHandler()
		forgeRead := brood.Group("/forge")
		forgeRead.Use(middleware.RequirePermission("nydus.read"))
		{
			forgeRead.GET("/summary", forgeH.ForgeSummary)
			forgeRead.GET("/repos", forgeH.ListRepos)
			forgeRead.GET("/nodes", forgeH.ListNodes)
			forgeRead.GET("/repos/:name/pulls", forgeH.ListPRs)
			forgeRead.GET("/repos/:name/forks", forgeH.ListForks)
			forgeRead.GET("/repos/:name/webhooks", forgeH.ListWebhooks)
			forgeRead.GET("/repos/:name/branches/protect", forgeH.ListBranchProtections)
			// Nydus data aggregation
			forgeRead.GET("/heatmap", forgeH.Heatmap)
			forgeRead.GET("/commits", forgeH.Commits)
			forgeRead.GET("/deploys", forgeH.Deploys)
			// Forge API proxy (projects, issues, sprints, dashboard, PRD, etc.)
			forgeRead.Any("/api/*path", forgeH.ForgeAPIProxy)
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

		// --- Billing ---
		billingRead := brood.Group("/billing")
		billingRead.Use(middleware.RequirePermission("billing.read"))
		{
			billingRead.GET("/plans", billingH.ListPlans)
			billingRead.GET("/plans/:id", billingH.GetPlan)
			billingRead.GET("/subscriptions", billingH.ListSubscriptions)
			billingRead.GET("/subscriptions/:id", billingH.GetSubscription)
			billingRead.GET("/overview", billingH.BillingOverview)
			billingRead.GET("/usage/stats", billingH.UsageStats)
			billingRead.GET("/usage/by-model", billingH.UsageByModel)
			billingRead.GET("/usage/by-user", billingH.UsageByUser)
			billingRead.GET("/usage/recent", billingH.UsageRecent)
			billingRead.GET("/alerts", billingH.ListBudgetAlerts)
		}
		billingWrite := brood.Group("/billing")
		billingWrite.Use(middleware.RequirePermission("billing.write"))
		{
			billingWrite.POST("/plans", billingH.CreatePlan)
			billingWrite.PUT("/plans/:id", billingH.UpdatePlan)
			billingWrite.POST("/subscriptions", billingH.CreateSubscription)
			billingWrite.POST("/subscriptions/:id/cancel", billingH.CancelSubscription)
			billingWrite.POST("/usage", billingH.RecordUsage)
			billingWrite.POST("/alerts", billingH.CreateBudgetAlert)
			billingWrite.PUT("/alerts/:id", billingH.UpdateBudgetAlert)
			billingWrite.DELETE("/alerts/:id", billingH.DeleteBudgetAlert)
		}

		// --- SSO ---
		// Public SSO endpoints (no auth required — used during login flow)
		brood.GET("/sso/oauth2/authorize", ssoH.OAuth2Authorize)
		brood.POST("/sso/oauth2/callback", ssoH.OAuth2Callback)
		brood.POST("/sso/ldap/login", ssoH.LDAPLogin)

		ssoRead := brood.Group("/sso")
		ssoRead.Use(middleware.RequirePermission("teams.read"))
		{
			ssoRead.GET("/providers", ssoH.ListProviders)
			ssoRead.GET("/providers/:id", ssoH.GetProvider)
			ssoRead.GET("/sessions", ssoH.ListSessions)
		}
		ssoWrite := brood.Group("/sso")
		ssoWrite.Use(middleware.RequirePermission("teams.write"))
		{
			ssoWrite.POST("/providers", ssoH.CreateProvider)
			ssoWrite.PUT("/providers/:id", ssoH.UpdateProvider)
			ssoWrite.DELETE("/providers/:id", ssoH.DeleteProvider)
			ssoWrite.POST("/providers/:id/test", ssoH.TestProvider)
		}

		// --- Brand / White-Label (public read, admin write) ---
		brood.GET("/brand", brandH.GetBrandConfig) // public — web frontend reads this

		brandRead := brood.Group("/brand")
		brandRead.Use(middleware.RequirePermission("brand.read"))
		{
			brandRead.GET("/config", brandH.GetBrandConfig)
		}
		brandWrite := brood.Group("/brand")
		brandWrite.Use(middleware.RequirePermission("brand.write"))
		brandWrite.Use(middleware.RequireTier(db, model.TierWhiteLabel))
		{
			brandWrite.PUT("/config", brandH.UpdateBrandConfig)
		}

		// --- License ---
		licRead := brood.Group("/license")
		licRead.Use(middleware.RequirePermission("license.read"))
		{
			licRead.GET("", brandH.GetLicense)
		}
		licWrite := brood.Group("/license")
		licWrite.Use(middleware.RequirePermission("license.write"))
		{
			licWrite.POST("/activate", brandH.ActivateLicense)
			licWrite.POST("", brandH.CreateLicense)
			licWrite.POST("/:id/revoke", brandH.RevokeLicense)
		}

		// --- Features ---
		featRead := brood.Group("/features")
		featRead.Use(middleware.RequirePermission("features.read"))
		{
			featRead.GET("", brandH.ListFeatures)
		}
		featWrite := brood.Group("/features")
		featWrite.Use(middleware.RequirePermission("features.write"))
		{
			featWrite.PUT("/:id", brandH.UpdateFeature)
		}

		// --- Direct Chat (no template/instance required) ---
		chatRead := brood.Group("/chat")
		chatRead.Use(middleware.RequirePermission("team_agent.read"))
		{
			chatRead.GET("/history", teamAgentH.GetDirectChatHistory)
		}
		chatSubmit := brood.Group("/chat")
		chatSubmit.Use(middleware.RequirePermission("team_agent.submit"))
		{
			chatSubmit.POST("", teamAgentH.SendDirectChat)
		}

		// --- Team Agent ---
		taRead := brood.Group("/team-agent")
		taRead.Use(middleware.RequirePermission("team_agent.read"))
		{
			taRead.GET("/templates", teamAgentH.ListTemplates)
			taRead.GET("/templates/:id", teamAgentH.GetTemplate)
			taRead.GET("/instances", teamAgentH.ListInstances)
			taRead.GET("/instances/:id", teamAgentH.GetInstance)
			taRead.GET("/instances/:id/dashboard", teamAgentH.GetDashboard)
			taRead.GET("/instances/:id/missions", teamAgentH.ListMissions)
			taRead.GET("/instances/:id/missions/:mid", teamAgentH.GetMission)
			taRead.GET("/stats", teamAgentH.Stats)
			taRead.GET("/instances/:id/chat", teamAgentH.GetChatHistory)
			taRead.GET("/instances/:id/conversations", teamAgentH.ListConversations)
			taRead.GET("/instances/:id/access", teamAgentH.ListInstanceAccess)
			taRead.GET("/node-models/:nodeId", teamAgentH.NodeModels)
			taRead.GET("/node-skills/:nodeId", teamAgentH.NodeSkills)
			taRead.GET("/node-agents/:nodeId", teamAgentH.NodeAgents)
			taRead.GET("/usage/by-user", teamAgentH.UsageByUser)
			taRead.GET("/instances/:id/agents", teamAgentH.ListInstanceAgents)
		}
		// --- Team Agent submit (viewer+) — chat + mission creation ---
		taSubmit := brood.Group("/team-agent")
		taSubmit.Use(middleware.RequirePermission("team_agent.submit"))
		{
			taSubmit.POST("/instances/:id/missions", teamAgentH.CreateMission)
			taSubmit.POST("/instances/:id/missions/:mid/cancel", teamAgentH.CancelMission)
			taSubmit.DELETE("/instances/:id/missions/:mid", teamAgentH.DeleteMission)
			taSubmit.POST("/instances/:id/chat", teamAgentH.SendChat)
			taSubmit.POST("/instances/:id/conversations", teamAgentH.CreateConversation)
			taSubmit.DELETE("/instances/:id/conversations/:cid", teamAgentH.DeleteConversation)
		}
		// --- Team Agent write (operator+) — instance lifecycle + provisioning ---
		taWrite := brood.Group("/team-agent")
		taWrite.Use(middleware.RequirePermission("team_agent.write"))
		{
			taWrite.POST("/instances", teamAgentH.CreateInstance)
			taWrite.POST("/instances/:id/disband", teamAgentH.DisbandInstance)
			taWrite.PUT("/instances/:id/publish", teamAgentH.PublishInstance)
			taWrite.PUT("/instances/:id/roles", teamAgentH.UpdateInstanceRoles)
			taWrite.POST("/instances/:id/agent-sandbox", teamAgentH.AgentSandbox)
			taWrite.POST("/instances/:id/agent-publish", teamAgentH.AgentPublish)
			taWrite.POST("/instances/:id/access", teamAgentH.GrantInstanceAccess)
			taWrite.DELETE("/instances/:id/access/:uid", teamAgentH.RevokeInstanceAccess)
			taWrite.POST("/provision-node", provisionH.ProvisionNode)
			taWrite.GET("/provision-status", provisionH.ProvisionStatus)
		}
		// --- Employee invite (admin only) ---
		inviteWrite := brood.Group("/admins")
		inviteWrite.Use(middleware.RequirePermission("teams.write"))
		{
			inviteWrite.POST("/invite", teamAgentH.CreateInvite)
			inviteWrite.GET("/employees", teamAgentH.ListEmployees)
		}

		// --- Compliance ---
		compRead := brood.Group("/compliance")
		compRead.Use(middleware.RequirePermission("compliance.read"))
		compRead.Use(middleware.RequireTier(db, model.TierEnterprise))
		{
			compRead.GET("/logs", complianceH.ListComplianceLogs)
			compRead.GET("/stats", complianceH.ComplianceStats)
			compRead.GET("/export", complianceH.ExportComplianceLogs)
			compRead.GET("/words", complianceH.ListSensitiveWords)
			compRead.GET("/flows", complianceH.ListDataFlows)
		}
		compWrite := brood.Group("/compliance")
		compWrite.Use(middleware.RequirePermission("compliance.write"))
		compWrite.Use(middleware.RequireTier(db, model.TierEnterprise))
		{
			compWrite.POST("/logs", complianceH.CreateComplianceLog)
			compWrite.POST("/logs/:id/resolve", complianceH.ResolveComplianceLog)
			compWrite.POST("/words", complianceH.CreateSensitiveWord)
			compWrite.PUT("/words/:id", complianceH.UpdateSensitiveWord)
			compWrite.DELETE("/words/:id", complianceH.DeleteSensitiveWord)
			compWrite.POST("/flows", complianceH.CreateDataFlow)
			compWrite.PUT("/flows/:id", complianceH.UpdateDataFlow)
			compWrite.DELETE("/flows/:id", complianceH.DeleteDataFlow)
		}
	}

	// WebSocket endpoint for real-time Team Agent updates
	r.GET("/ws/team-agent", func(c *gin.Context) {
		teamID := c.Query("team_id")
		if teamID == "" {
			teamID = "global"
		}
		ws.HandleWS(wsHub, teamID, c.Writer, c.Request)
	})

	r.GET("/metrics", gin.WrapH(promhttp.Handler()))
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "overlord-api"})
	})

	// Connect to Pheromone ESB
	natsURL := getEnv("PHEROMONE_NATS_URL", "nats://127.0.0.1:4222")
	ph, phErr := pheromone.New(natsURL, pheromone.ServiceInfo{
		Name:    "overlord",
		Version: "1.0.0",
		Port:    8095,
		Tags:    []string{"management", "monitoring", "team-agent"},
	})
	if phErr != nil {
		log.Printf("[overlord] pheromone connect failed (non-fatal): %v", phErr)
	} else {
		ph.StartHeartbeat(30 * time.Second)
		defer ph.Close()
		handler.SubscribePheromoneEvents(ph)
		log.Printf("[overlord] pheromone ESB connected (%s)", natsURL)
	}

	port := getEnv("OVERLORD_PORT", "8095")
	log.Printf("[overlord] API service starting on :%s", port)
	if err := r.Run(fmt.Sprintf(":%s", port)); err != nil {
		log.Fatalf("Failed to start overlord api: %v", err)
	}
}

func seedSuperAdmin(db *gorm.DB) {
	username := getEnv("OVERLORD_ADMIN_USERNAME", "admin")
	password := getEnv("OVERLORD_ADMIN_PASSWORD", "admin123")

	var user model.AdminUser
	if err := db.Where("username = ?", username).First(&user).Error; err == nil {
		// User exists — update password if env changed (compare hash)
		newHash := middleware.HashTokenExported(password)
		if user.PasswordHash != newHash {
			db.Model(&user).Update("password_hash", newHash)
			log.Printf("[overlord] superadmin '%s' password updated from env", username)
		}
		return
	}

	// No admin user with this username — check if any admin exists
	var count int64
	db.Model(&model.AdminUser{}).Count(&count)
	if count > 0 {
		return
	}

	db.Create(&model.AdminUser{
		Username:     username,
		PasswordHash: middleware.HashTokenExported(password),
		Role:         "superadmin",
		Email:        username + "@overlord.local",
	})
	log.Printf("[overlord] Default superadmin created (username: %s)", username)
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
