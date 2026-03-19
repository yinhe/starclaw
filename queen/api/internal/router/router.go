package router

import (
	"os"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw-queen/api/internal/config"
	"github.com/yinhe/starclaw-queen/api/internal/handler"
	"github.com/yinhe/starclaw-queen/api/internal/middleware"
)

func Setup() *gin.Engine {
	if config.C.Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Recovery())

	// Access logger
	r.Use(middleware.AccessLogger())

	// CORS
	corsConfig := cors.DefaultConfig()
	corsConfig.AllowOrigins = config.C.CORS.Origins
	corsConfig.AllowHeaders = append(corsConfig.AllowHeaders, "Authorization", "X-Claw-ID", "X-Claw-Timestamp", "X-Claw-Signature", "X-Claw-PublicKey")
	corsConfig.AllowCredentials = true
	r.Use(cors.New(corsConfig))

	// Prometheus metrics
	r.Use(middleware.PrometheusMetrics())
	r.GET("/metrics", middleware.MetricsHandler())

	// Global rate limit: 200 requests per minute per IP
	globalRL := middleware.NewRateLimiter(200, 1*time.Minute)
	r.Use(globalRL.Middleware())

	// Write rate limit: 30 requests per minute per user (applied to authed write routes)
	writeRL := middleware.NewRateLimiter(30, 1*time.Minute)

	// Health
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "queen-api"})
	})

	v1 := r.Group("/v1")

	// Auth rate limit: 10 requests per minute per IP (anti-brute-force)
	authRL := middleware.NewRateLimiter(10, 1*time.Minute)

	// ---- Auth (public) ----
	auth := &handler.AuthHandler{}
	v1.POST("/auth/register", authRL.Middleware(), auth.Register)
	v1.POST("/auth/login", authRL.Middleware(), auth.Login)
	v1.POST("/auth/oauth/google", auth.OAuthGoogle)
	v1.POST("/auth/oauth/github", auth.OAuthGitHub)

	// ---- Sign-In with Claw (public) ----
	clawAuth := handler.NewClawAuthHandler()
	v1.POST("/auth/claw/challenge", authRL.Middleware(), clawAuth.Challenge)
	v1.POST("/auth/claw/verify", authRL.Middleware(), clawAuth.Verify)
	v1.POST("/auth/claw-register", authRL.Middleware(), clawAuth.ClawRegister)

	// ---- Marketplace (public read) ----
	mp := &handler.MarketplaceHandler{}
	v1.GET("/marketplace/items", mp.List)
	v1.GET("/marketplace/items/:id", mp.Get)
	v1.GET("/marketplace/stats", mp.Stats)

	// ---- Billing (init clients) ----
	billing := handler.NewBillingHandler()

	// ---- Payment webhooks (public, no auth) ----
	pay := r.Group("/pay")
	{
		pay.POST("/webhook/alipay", billing.AlipayWebhook)
		pay.POST("/webhook/wechatpay", billing.WechatPayWebhook)
	}

	// ---- Billing (public read) ----
	v1.GET("/pay/packages", billing.ListPackages)
	v1.GET("/pay/methods", billing.PayMethods)

	// ---- Authenticated routes ----
	authed := v1.Group("")
	authed.Use(middleware.AuthRequired())
	{
		// User
		user := &handler.UserHandler{}
		authed.GET("/user/profile", user.GetProfile)
		authed.PUT("/user/profile", writeRL.UserRateLimit(), user.UpdateProfile)
		authed.PUT("/user/password", writeRL.UserRateLimit(), user.ChangePassword)

		// Marketplace (write)
		authed.POST("/marketplace/items", writeRL.UserRateLimit(), mp.Create)
		authed.PUT("/marketplace/items/:id", writeRL.UserRateLimit(), mp.Update)
		authed.DELETE("/marketplace/items/:id", writeRL.UserRateLimit(), mp.Delete)
		authed.GET("/marketplace/my", mp.My)
		authed.POST("/marketplace/items/:id/submit", writeRL.UserRateLimit(), mp.Submit)

		// Node binding
		nb := &handler.NodeBindingHandler{}
		authed.POST("/user/nodes", writeRL.UserRateLimit(), nb.BindNode)
		authed.GET("/user/nodes", nb.ListNodes)
		authed.DELETE("/user/nodes/:node_id", writeRL.UserRateLimit(), nb.UnbindNode)

		// Content reports
		rpt := &handler.ReportHandler{}
		authed.POST("/reports", writeRL.UserRateLimit(), rpt.Create)
		authed.GET("/reports/mine", rpt.MyReports)
		authed.GET("/reports/reasons", rpt.Reasons)

		// Billing
		authed.GET("/pay/balance", billing.GetBalance)
		authed.GET("/pay/transactions", billing.ListTransactions)
		authed.GET("/pay/orders", billing.ListOrders)
		authed.POST("/pay/create", writeRL.UserRateLimit(), billing.CreateOrder)
		authed.POST("/pay/convert-energy", writeRL.UserRateLimit(), billing.ConvertToEnergy)
		authed.GET("/pay/order/:order_no/status", billing.QueryOrderStatus)
	}

	// ---- Admin routes (require admin role) ----
	admin := v1.Group("/admin")
	admin.Use(middleware.AuthRequired(), middleware.AdminRequired())
	{
		// Billing management
		admin.GET("/billing/stats", billing.AdminBillingStats)
		admin.GET("/billing/orders", billing.AdminListOrders)
		admin.GET("/billing/balances", billing.AdminListBalances)
		admin.POST("/billing/adjust", billing.AdminAdjustBalance)
		admin.GET("/billing/packages", billing.AdminListPackages)
		admin.PUT("/billing/packages/:id", billing.AdminUpdatePackage)

		// Content moderation
		rptAdmin := &handler.ReportHandler{}
		admin.GET("/reports", rptAdmin.AdminList)
		admin.GET("/reports/stats", rptAdmin.AdminStats)
		admin.PUT("/reports/:id", rptAdmin.AdminReview)
		admin.POST("/reports/:id/action", rptAdmin.AdminAction)

		// User management
		adminUser := &handler.AdminUserHandler{}
		admin.GET("/users", adminUser.List)
		admin.GET("/users/stats", adminUser.Stats)
		admin.GET("/users/:id", adminUser.Get)
		admin.PUT("/users/:id/role", adminUser.UpdateRole)
		admin.PUT("/users/:id/status", adminUser.UpdateStatus)

		// Swarm / node management (proxied to swarm service)
		dash := handler.NewDashboardHandler()
		admin.GET("/stats", dash.GlobalStats)
		admin.GET("/nodes", dash.ListNodes)
		admin.GET("/nodes/:id", dash.GetNode)
		admin.DELETE("/nodes/:id", dash.RemoveNode)
		admin.POST("/update/notify", dash.NotifyUpdate)

		// Molt — version release management (proxied to swarm)
		admin.POST("/molt/releases", dash.CreateRelease)
		admin.GET("/molt/releases", dash.ListReleases)
		admin.GET("/molt/releases/:id", dash.GetRelease)
		admin.POST("/molt/releases/:id/start", dash.StartRelease)
		admin.POST("/molt/releases/:id/pause", dash.PauseRelease)

		// Marketplace review (developer center)
		admin.GET("/marketplace/pending", mp.AdminListPending)
		admin.GET("/marketplace/stats", mp.AdminReviewStats)
		admin.PUT("/marketplace/items/:id/approve", mp.AdminApprove)
		admin.PUT("/marketplace/items/:id/reject", mp.AdminReject)
		admin.PUT("/marketplace/items/:id/remove", mp.AdminRemove)

		// Service proxies (bounty / forum / arena)
		proxy := handler.NewAdminProxyHandler()
		admin.GET("/bounty/stats", proxy.BountyStats)
		admin.GET("/bounty/tasks", proxy.BountyTasks)
		admin.GET("/forum/stats", proxy.ForumStats)
		admin.GET("/forum/posts", proxy.ForumPosts)
		admin.DELETE("/forum/posts/:id", proxy.ForumDeletePost)
		admin.GET("/arena/stats", proxy.ArenaStats)
		admin.GET("/arena/threads", proxy.ArenaThreads)
		admin.GET("/arena/leaderboard", proxy.ArenaLeaderboard)
	}

	// ---- Overseer (monitoring dashboard API) ----
	overseer := handler.NewOverseerHandler()
	admin.GET("/overseer/dashboard", overseer.Dashboard)
	admin.GET("/overseer/nodes", overseer.Nodes)
	admin.GET("/overseer/nodes/:id", overseer.NodeDetail)
	admin.GET("/overseer/services", overseer.Services)
	admin.GET("/overseer/energy", overseer.Energy)
	admin.GET("/overseer/metrics/query", overseer.MetricsQuery)
	admin.GET("/overseer/metrics/query_range", overseer.MetricsQueryRange)
	admin.GET("/overseer/alerts", overseer.Alerts)

	// ---- Core Partner Hub ----
	ph := &handler.PartnerHandler{}

	// Partner portal (core partners only)
	partnerPortal := v1.Group("/partner")
	partnerPortal.Use(middleware.AuthRequired(), handler.TeamPartnerRequired())
	{
		partnerPortal.GET("/dashboard", ph.Dashboard)
		partnerPortal.GET("/deals", ph.ListDeals)
		partnerPortal.POST("/deals", writeRL.UserRateLimit(), ph.CreateDeal)
		partnerPortal.GET("/deals/:id", ph.GetDeal)
		partnerPortal.PUT("/deals/:id", writeRL.UserRateLimit(), ph.UpdateDeal)
		partnerPortal.GET("/city-partners", ph.ListCityPartners)
		partnerPortal.PUT("/city-partners/:id", writeRL.UserRateLimit(), ph.ReviewCityPartner)
		partnerPortal.GET("/nodes", ph.ListNodes)
		partnerPortal.GET("/nodes/my", ph.ListMyNodes)
		partnerPortal.GET("/nodes/:id", ph.GetNode)
		partnerPortal.GET("/commissions", ph.ListCommissions)
		partnerPortal.GET("/equity", ph.GetEquity)
		partnerPortal.GET("/deployments", ph.ListDeployments)
		partnerPortal.POST("/deployments", writeRL.UserRateLimit(), ph.CreateDeployment)
		partnerPortal.GET("/deployments/:id", ph.GetDeployment)
		partnerPortal.POST("/deployments/:id/stop", writeRL.UserRateLimit(), ph.StopDeployment)
		partnerPortal.POST("/city-partners/claw", writeRL.UserRateLimit(), ph.AddCityPartnerClaw)
		partnerPortal.DELETE("/city-partners/:id/claw", writeRL.UserRateLimit(), ph.RemoveCityPartnerClaw)
		// Cerebrate elections (team partners vote)
		election := &handler.ElectionHandler{}
		partnerPortal.GET("/election", election.GetCurrentElection)
		partnerPortal.GET("/election/candidates", election.ListCandidates)
		partnerPortal.POST("/election/vote", writeRL.UserRateLimit(), election.Vote)
	}

	// Admin: team partner management
	admin.GET("/partners", ph.AdminListPartners)
	admin.POST("/partners", ph.AdminCreatePartner)
	admin.PUT("/partners/:id", ph.AdminUpdatePartner)
	admin.POST("/partners/:id/equity", ph.AdminGrantEquity)
	// Admin: Cerebrate elections
	electionAdmin := &handler.ElectionHandler{}
	admin.POST("/election", electionAdmin.AdminCreateElection)
	admin.GET("/election", electionAdmin.AdminListElections)
	admin.POST("/election/:id/close", electionAdmin.AdminCloseElection)
	admin.DELETE("/election/:id", electionAdmin.AdminCancelElection)

	// ---- City Partner Portal ----
	city := &handler.CityHandler{}

	// Apply to become a partner (any authenticated user)
	authed.POST("/city/apply", writeRL.UserRateLimit(), city.Apply)

	// City partner portal (approved partners only)
	cityPortal := v1.Group("/city")
	cityPortal.Use(middleware.AuthRequired(), handler.CityPartnerRequired())
	{
		cityPortal.GET("/dashboard", city.Dashboard)
		cityPortal.GET("/clients", city.ListClients)
		cityPortal.POST("/clients", writeRL.UserRateLimit(), city.AddClient)
		cityPortal.PUT("/clients/:id", writeRL.UserRateLimit(), city.UpdateClient)
		cityPortal.GET("/client-stats", city.ClientStats)
		cityPortal.GET("/commissions", city.ListCommissions)
		cityPortal.GET("/payouts", city.ListPayouts)
		cityPortal.GET("/materials", city.ListMaterials)
		cityPortal.GET("/ref-link", city.RefLink)
	}

	// Admin: city partner management
	admin.GET("/city/partners", city.AdminListPartners)
	admin.PUT("/city/partners/:id", city.AdminReviewPartner)
	admin.GET("/city/commissions", city.AdminListCommissions)
	admin.PUT("/city/commissions/:id", city.AdminApproveCommission)
	admin.POST("/city/materials", city.AdminCreateMaterial)

	// ---- Settlement Engine ----
	settle := &handler.SettlementHandler{}
	admin.POST("/settlement/generate", settle.GenerateBills)
	admin.GET("/settlement/bills", settle.ListBills)
	admin.GET("/settlement/bills/:id", settle.GetBill)
	admin.POST("/settlement/bills/:id/approve", settle.ApproveBill)
	admin.POST("/settlement/bills/:id/reject", settle.RejectBill)
	admin.POST("/settlement/bills/:id/pay", settle.MarkPaid)
	admin.DELETE("/settlement/bills/:id", settle.DeleteBill)
	admin.GET("/settlement/stats", settle.SettlementStats)

	// ---- Admin Analytics (GMV/MRR/ARR) ----
	analytics := &handler.AdminAnalyticsHandler{}
	admin.GET("/analytics", analytics.QueenAnalytics)
	admin.GET("/clients", analytics.AdminListAllClients)
	admin.GET("/partners/performance", analytics.AdminPartnerPerformance)

	// ---- API Gateway (star-ai.net) ----
	gw := handler.NewGatewayHandler()

	// API key management (authenticated)
	authed.POST("/api-keys", writeRL.UserRateLimit(), gw.CreateKey)
	authed.GET("/api-keys", gw.ListKeys)
	authed.DELETE("/api-keys/:id", writeRL.UserRateLimit(), gw.DeleteKey)
	authed.GET("/api-keys/usage", gw.Usage)

	// OpenAI-compatible gateway (API key auth, not JWT)
	v1.POST("/chat/completions", gw.ChatCompletions)
	v1.GET("/models", gw.ListModels)

	// ---- Star Energy (星能) — public API for claw wallets ----
	credit := &handler.CreditHandler{}
	v1.GET("/credits/balance", credit.GetBalance)
	v1.GET("/credits/transactions", credit.ListTransactions)
	v1.POST("/credits/transfer", writeRL.Middleware(), credit.Transfer)

	// ---- Investor Pool (投资人池) ----
	investor := &handler.InvestorHandler{}
	// Public: pool info (no auth needed)
	v1.GET("/investor/pool", investor.PublicPoolInfo)
	// Investor portal (authenticated)
	authed.POST("/investor/register", writeRL.UserRateLimit(), investor.Register)
	authed.POST("/investor/agree", writeRL.UserRateLimit(), investor.SignAgreement)
	authed.POST("/investor/recharge", writeRL.UserRateLimit(), investor.Recharge)
	authed.GET("/investor/me", investor.MyProfile)
	authed.GET("/investor/earnings", investor.DailyEarnings)
	// Admin investor management
	admin.POST("/investor/pool/init", investor.InitPool)
	admin.GET("/investor/pool", investor.GetPool)
	admin.POST("/investor/airdrop", investor.Airdrop)
	admin.GET("/investor/list", investor.ListInvestors)
	admin.POST("/investor/distribute", investor.Distribute)
	admin.GET("/investor/dividends", investor.ListDividends)
	admin.GET("/investor/deposits", investor.ListDeposits)
	admin.POST("/investor/round/open", investor.OpenRound)
	admin.GET("/investor/rounds", investor.ListRounds)

	// ---- Internal API (for Claw nodes, authenticated via X-Node-Token header) ----
	internal := r.Group("/internal")
	internal.Use(nodeTokenAuth())
	{
		internal.POST("/billing/check", billing.InternalCheckBalance)
		internal.POST("/billing/consume", billing.InternalConsume)
		internal.GET("/billing/balance/:user_id", billing.InternalGetBalance)
		internal.POST("/billing/freeze", billing.InternalFreeze)
		internal.POST("/billing/unfreeze", billing.InternalUnfreeze)
		internal.POST("/billing/settle", billing.InternalSettle)
		internal.GET("/billing/resolve-partners", billing.InternalResolvePartners)

		// Star Energy (internal — for Router/Swarm services)
		internal.POST("/credits/grant", credit.InternalGrant)
		internal.POST("/credits/consume", credit.InternalConsume)
		internal.GET("/credits/balance/:claw_id", credit.InternalGetBalance)
		internal.POST("/credits/freeze", credit.InternalFreeze)
		internal.POST("/credits/unfreeze", credit.InternalUnfreeze)
		internal.POST("/credits/settle", credit.InternalSettle)
		internal.POST("/inference/settle", credit.InternalInferenceSettle)

		// Node binding (internal)
		nbInternal := &handler.NodeBindingHandler{}
		internal.POST("/user/bind", nbInternal.InternalBind)
		internal.GET("/user/resolve/:node_id", nbInternal.InternalResolve)
		internal.POST("/user/heartbeat", nbInternal.InternalHeartbeat)

		// Investor pool (internal — for Billing Gateway profit deposit)
		internal.POST("/investor/deposit", investor.InternalDeposit)
	}

	return r
}

// nodeTokenAuth validates X-Node-Token header against INTERNAL_API_SECRET env or JWT secret
func nodeTokenAuth() gin.HandlerFunc {
	// Prefer dedicated env var; fall back to JWT secret from config
	secret := os.Getenv("INTERNAL_API_SECRET")
	src := "INTERNAL_API_SECRET"
	if secret == "" {
		secret = os.Getenv("JWT_SECRET")
		src = "JWT_SECRET"
	}
	if secret == "" {
		secret = config.C.JWT.Secret
		src = "config.jwt.secret"
	}
	_ = src // secret source logged at debug level only
	return func(c *gin.Context) {
		token := c.GetHeader("X-Node-Token")
		if token == "" || token != secret {
			middleware.Fail(c, 401, middleware.CodeUnauthorized, "unauthorized node")
			c.Abort()
			return
		}
		c.Next()
	}
}
