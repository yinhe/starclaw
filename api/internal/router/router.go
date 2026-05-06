package router

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	agentpkg "github.com/yinhe/starclaw/internal/agent"
	v1 "github.com/yinhe/starclaw/internal/api/v1"
	"github.com/yinhe/starclaw/internal/api/v1/auth"
	billingpkg "github.com/yinhe/starclaw/internal/api/v1/billing"
	"github.com/yinhe/starclaw/internal/api/v1/game"
	"github.com/yinhe/starclaw/internal/api/v1/infra"
	"github.com/yinhe/starclaw/internal/api/v1/knowledge"
	"github.com/yinhe/starclaw/internal/api/v1/market"
	"github.com/yinhe/starclaw/internal/api/v1/media"
	network "github.com/yinhe/starclaw/internal/api/v1/net"
	"github.com/yinhe/starclaw/internal/api/v1/ops"
	squadapi "github.com/yinhe/starclaw/internal/api/v1/squad"
	wf "github.com/yinhe/starclaw/internal/api/v1/workflow"
	"github.com/yinhe/starclaw/internal/config"
	"github.com/yinhe/starclaw/internal/forge"
	"github.com/yinhe/starclaw/internal/inference"
	"github.com/yinhe/starclaw/internal/memory"
	"github.com/yinhe/starclaw/internal/middleware"
	"github.com/yinhe/starclaw/internal/model"
	"github.com/yinhe/starclaw/internal/molt"
	"github.com/yinhe/starclaw/internal/node"
	"github.com/yinhe/starclaw/internal/observe"
	"github.com/yinhe/starclaw/internal/overlord"
	"github.com/yinhe/starclaw/internal/security"
	"github.com/yinhe/starclaw/internal/squad"
	"github.com/yinhe/starclaw/internal/swarm"
	"github.com/yinhe/starclaw/internal/tool"
	"github.com/yinhe/starclaw/internal/web"
	"github.com/yinhe/starclaw/internal/webhook"
	"github.com/yinhe/starclaw/internal/ws"
	"gorm.io/gorm"
)

func Setup(cfg *config.Config, db *gorm.DB, rdb *redis.Client, swarmClient ...*swarm.Client) *gin.Engine {
	if cfg.Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	// CORS
	corsConfig := cors.Config{
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}
	if cfg.Server.DeployMode == "hosted" {
		corsConfig.AllowOrigins = []string{
			"https://starclaw.me", "https://app.starclaw.me",
			"https://api.starclaw.me", "https://www.starclaw.me",
			"https://star-ai.net", "https://www.star-ai.net",
			"https://app.star-ai.net", "https://api.star-ai.net",
			"https://starclaw.net", "https://www.starclaw.net",
			"https://invest.starclaw.net", "https://overlord.starclaw.net",
			"http://localhost:5173", "http://localhost:3000",
		}
	} else {
		// opensource: allow any origin so self-hosted users don't get CORS 403
		corsConfig.AllowAllOrigins = true
		corsConfig.AllowCredentials = false // AllowAllOrigins requires credentials=false
	}
	r.Use(cors.New(corsConfig))

	// Prometheus metrics
	r.Use(middleware.PrometheusMetrics())

	// Rate limiting
	r.Use(middleware.RateLimit(300, time.Minute, rdb))

	// Initialize all engines, registries, and services
	deps := initDeps(cfg, db)
	identity := deps.Identity
	providerRegistry := deps.ProviderRegistry
	toolRegistry := deps.ToolRegistry
	sandboxMgr := deps.SandboxMgr
	processMgr := deps.ProcessMgr
	taskWorker := deps.TaskWorker
	instinctEngine := deps.InstinctEngine
	embedder := deps.Embedder
	pipeline := deps.Pipeline
	queenClient := deps.QueenClient

	r.Use(middleware.RequestLogger())

	// A2A Agent Card discovery (must be at root, not under /v1)
	a2aCardHandler := network.NewA2AHandler(db, providerRegistry, toolRegistry)
	r.GET("/.well-known/agent.json", a2aCardHandler.AgentCardHandler)

	// Prometheus metrics endpoint
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Health check
	startTime := time.Now()
	var oc *overlord.Client
	r.GET("/health", func(c *gin.Context) {
		dbOk := true
		sqlDB, err := db.DB()
		if err == nil {
			if err := sqlDB.Ping(); err != nil {
				dbOk = false
			}
		} else {
			dbOk = false
		}
		c.JSON(200, gin.H{
			"status":        "ok",
			"service":       "starclaw",
			"version":       molt.Version,
			"uptime_s":      int(time.Since(startTime).Seconds()),
			"qmt_connected": false,
			"db_status":     dbOk,
		})
	})

	// API v1
	apiV1 := r.Group("/v1")
	{
		// Public routes
		authHandler := auth.NewAuthHandler(db, cfg, identity)
		apiV1.POST("/auth/register", authHandler.Register)
		apiV1.POST("/auth/login", authHandler.Login)
		apiV1.POST("/auth/phone/register", authHandler.PhoneRegister)
		apiV1.POST("/auth/phone/login", authHandler.PhoneLogin)
		apiV1.POST("/auth/token/login", authHandler.TokenLogin)

		// Setup (single-user Owner mode, opensource only)
		setupHandler := v1.NewSetupHandler(db, cfg, identity)
		apiV1.GET("/setup/status", setupHandler.Status)
		apiV1.POST("/setup", setupHandler.Setup)
		apiV1.GET("/setup/token", setupHandler.GetToken)
		apiV1.POST("/setup/reset-token", setupHandler.ResetToken)
		apiV1.POST("/setup/reset-password", setupHandler.ResetPassword)
		apiV1.POST("/auth/owner-login", setupHandler.PasswordLogin)

		// OAuth routes (public)
		oauthHandler := auth.NewOAuthHandler(db, cfg)
		apiV1.GET("/auth/oauth/providers", oauthHandler.GetOAuthConfig)
		apiV1.POST("/auth/oauth/github", oauthHandler.GitHubCallback)
		apiV1.POST("/auth/oauth/google", oauthHandler.GoogleCallback)

		// Public identity endpoint (exposes public key only, safe)
		apiV1.GET("/identity/info", func(c *gin.Context) {
			c.JSON(200, gin.H{
				"node_id":    identity.NodeID,
				"public_key": identity.PublicKeyHex(),
			})
		})

		// Recovery (public endpoints — needed on fresh installs before auth exists)
		recoveryQueenURL := ""
		if cfg.Swarm.QueenURL != "" {
			recoveryQueenURL = strings.TrimSuffix(cfg.Swarm.QueenURL, "/api") + "/api"
		}
		recoveryHandler := auth.NewRecoveryHandler(db, identity, recoveryQueenURL)
		apiV1.POST("/recovery/verify-mnemonic", recoveryHandler.VerifyMnemonic)
		apiV1.POST("/recovery/restore", recoveryHandler.Restore)

		// Auth request endpoints (MetaMask-style: public create+poll, protected approve)
		authReqHandler := auth.NewAuthRequestHandler(identity, db)
		apiV1.POST("/identity/auth-request", authReqHandler.Create)
		apiV1.GET("/identity/auth-request/:id", authReqHandler.GetStatus)

		// Deploy mode info (public, no auth needed)
		apiV1.GET("/config", func(c *gin.Context) {
			c.JSON(200, gin.H{
				"deploy_mode": cfg.Server.DeployMode,
			})
		})

		// Version info (public, includes Molt update check)
		apiV1.GET("/version", func(c *gin.Context) {
			c.JSON(200, molt.GetVersionInfo())
		})

		// Drone marketplace import (internal, secret-protected)
		droneImportHandler := market.NewMarketplaceHandler(db)
		apiV1.POST("/marketplace/import", func(c *gin.Context) {
			secret := c.GetHeader("X-Drone-Secret")
			if secret == "" {
				secret = c.Query("secret")
			}
			expected := cfg.JWT.Secret
			if secret != expected {
				c.JSON(401, gin.H{"error": "unauthorized"})
				c.Abort()
				return
			}
			droneImportHandler.AdminBulkImport(c)
		})

		// P9: Developer portal (public — OpenAPI + Swagger UI)
		devHandler := market.NewDeveloperHandler(db)
		apiV1.GET("/developer/openapi.json", devHandler.GetOpenAPISpec)
		apiV1.GET("/developer/docs", devHandler.SwaggerUI)
		apiV1.GET("/developer/plugins/categories", devHandler.PluginCategories)

		// Peer-to-Peer inter-node endpoints (public, signature-verified)
		peerPublicHandler := network.NewPeerHandler(db, cfg)
		apiV1.GET("/peer/handshake", peerPublicHandler.HandleHandshake)
		apiV1.GET("/peer/resolve", peerPublicHandler.HandleResolve)
		apiV1.POST("/peer/gossip", peerPublicHandler.HandleGossip)
		apiV1.POST("/peer/relay", peerPublicHandler.HandleRelayTask)

		// P7: P2P Network + Evolution engines
		dhtEngine := node.NewDHT(identity, cfg.Node.Address, peerPublicHandler.Gossip())
		dhtEngine.Start()
		creepEngine := node.NewCreepEngine(identity.NodeID, peerPublicHandler.Gossip())
		creepEngine.Start(60 * time.Second)
		hivemindEngine := node.NewHivemindEngine(identity.NodeID, peerPublicHandler.Gossip())
		hivemindEngine.SetAgentProvider(func() []node.AgentCapability {
			var agents []model.Agent
			db.Select("id, name, description").Find(&agents)
			caps := make([]node.AgentCapability, 0, len(agents))
			for _, ag := range agents {
				caps = append(caps, node.AgentCapability{
					AgentID:     ag.ID,
					Name:        ag.Name,
					Description: ag.Description,
					Specialty:   "general",
					Available:   true,
				})
			}
			return caps
		})
		hivemindEngine.Start(30 * time.Second)
		evolutionEngine := node.NewEvolutionEngine(node.DefaultEvolutionConfig())
		evolutionEngine.Start()
		p2pHandler := network.NewP2PHandler(dhtEngine, creepEngine, hivemindEngine, evolutionEngine)

		// P8: Observability engine
		observeEngine := observe.NewEngine(db)
		observeEngine.Start()

		// P8: Webhook orchestration engine
		webhookEngine := webhook.NewEngine(db)
		webhookEngine.Start()

		// P9: Security engine
		keyMgr, _ := security.NewKeyManager()
		security.SetGlobalKeyManager(keyMgr)
		auditChain := security.NewAuditChain(db)

		// P10: AI-Native engines
		mmRouter := agentpkg.NewMultimodalRouter()
		proactiveEngine := agentpkg.NewProactiveEngine(db)
		proactiveEngine.Start()
		collabEngine := agentpkg.NewCollaborationEngine(db)
		fineTuneEngine := agentpkg.NewFineTuneEngine(db)
		fineTuneEngine.Start()

		// DHT inter-node RPC (public)
		apiV1.GET("/peer/dht/ping", p2pHandler.HandleDHTPing)
		apiV1.POST("/peer/dht/find_node", p2pHandler.HandleDHTFindNode)
		apiV1.POST("/peer/dht/store", p2pHandler.HandleDHTStore)
		apiV1.POST("/peer/dht/find_value", p2pHandler.HandleDHTFindValue)

		// Creep inter-node sync (public)
		apiV1.POST("/peer/creep/sync", p2pHandler.HandleCreepSync)
		apiV1.POST("/peer/creep/push", p2pHandler.HandleCreepPush)

		// Hivemind inter-node (public)
		apiV1.POST("/peer/hivemind/capability", p2pHandler.HandleHivemindCapability)
		apiV1.POST("/peer/hivemind/execute", p2pHandler.HandleHivemindExecute)

		// Forge: Nydus webhook receiver (HMAC-verified)
		apiV1.POST("/forge/nydus/webhook", forge.HandleNydusWebhook(db))

		// Squad inter-node (public, signature-verified)
		squadPeerHandler := network.NewSquadPeerHandler(db, identity)
		apiV1.POST("/peer/squad/invite", squadPeerHandler.HandleInvite)
		apiV1.POST("/peer/squad/agents", squadPeerHandler.HandleAgents)
		apiV1.POST("/peer/squad/execute", squadPeerHandler.HandleExecute)
		apiV1.POST("/peer/squad/callback", squadPeerHandler.HandleCallback)
		apiV1.POST("/peer/squad/heartbeat", squadPeerHandler.HandleHeartbeat)
		squadPeerHandler.StartCallbackWatcher()

		// Overlord internal endpoints (token-authenticated, for Team Agent orchestration)
		overlordH := squadapi.NewOverlordInternalHandler(db, identity, cfg)
		overlordH.SetProviderRegistry(providerRegistry)
		overlordH.SetToolRegistry(toolRegistry)
		internal := apiV1.Group("/internal")
		internal.Use(overlordH.AuthMiddleware())
		{
			internal.POST("/squad/create", overlordH.CreateSquad)
			internal.POST("/squad/disband", overlordH.DisbandSquad)
			internal.GET("/squad/:id", overlordH.GetSquad)
			internal.POST("/mission/create", overlordH.CreateMission)
			internal.POST("/mission/start", overlordH.StartMission)
			internal.GET("/mission/:id", overlordH.GetMission)
			// Auth exchange (Overlord ↔ Claw token bridge)
			internal.POST("/auth/exchange", overlordH.AuthExchange)
			internal.POST("/auth/verify", overlordH.AuthVerify)
			// Chat proxy (OpenAI-compatible, for Overlord)
			internal.POST("/chat/completions", overlordH.ChatCompletions)
			internal.GET("/models", overlordH.ListModels)
			// Skills & Agents (marketplace integration for Overlord)
			internal.GET("/skills", overlordH.ListSkills)
			internal.GET("/agents", overlordH.ListAgents)
			// Agent Development (DevClaw sandbox + publish)
			internal.POST("/agent-sandbox", overlordH.AgentSandbox)
			internal.POST("/agent-publish", overlordH.AgentPublish)
			// Team Agent Management (Phase 1: Agent 化)
			internal.POST("/agents/register", overlordH.RegisterAgent)
			internal.GET("/agents/team/:instanceId", overlordH.ListTeamAgents)
			internal.DELETE("/agents/:id", overlordH.DeleteAgent)
			internal.POST("/agents/:id/skills", overlordH.InstallSkill)
			internal.DELETE("/agents/:id/skills/:skillName", overlordH.UninstallSkill)
		}

		// Inference Router (public status + signed contributor endpoints)
		inferenceRouter := inference.NewInferenceRouter(identity)
		spotChecker := inference.NewSpotChecker(inferenceRouter.Registry, inferenceRouter, 0.01) // 1% spot-check rate
		inferenceHandler := infra.NewInferenceHandler(inferenceRouter, providerRegistry, spotChecker)
		apiV1.GET("/inference/status", inferenceHandler.RouterStatus)

		// Node-signed endpoints (protected by Ed25519 signature middleware)
		signedRoutes := apiV1.Group("")
		signedRoutes.Use(middleware.NodeSignatureAuth())
		{
			// Inference contributor endpoints
			signedRoutes.POST("/inference/register", inferenceHandler.RegisterContributor)
			signedRoutes.POST("/inference/heartbeat", inferenceHandler.Heartbeat)
			signedRoutes.POST("/inference/unregister", inferenceHandler.UnregisterContributor)
			signedRoutes.POST("/inference/execute", inferenceHandler.Execute)

			// Peer v2 endpoints (middleware-verified, cleaner protocol)
			signedRoutes.POST("/peer/v2/gossip", peerPublicHandler.HandleGossipSigned)
			signedRoutes.POST("/peer/v2/relay", peerPublicHandler.HandleRelayTaskSigned)
		}

		// Start compute contribution service (auto-detects local Ollama and registers with peers)
		contributorCfg := inference.ContributorConfig{
			Enabled:      cfg.Contributor.Enabled,
			OllamaURL:    cfg.Contributor.OllamaURL,
			MaxJobs:      cfg.Contributor.MaxJobs,
			ExternalAddr: cfg.Contributor.ExternalAddr,
			NydusEnabled: cfg.Nydus.Enabled || cfg.Contributor.Enabled, // auto-enable with contributor
		}
		if contributorCfg.ExternalAddr == "" && cfg.Node.Address != "" {
			contributorCfg.ExternalAddr = cfg.Node.Address
		}

		// Create Nydus NAT traversal manager (if contributor enabled and no static external address)
		var nydusManager *node.NydusManager
		if contributorCfg.NydusEnabled {
			nydusCfg := node.NydusConfig{
				STUNServers: cfg.Nydus.STUNServers,
				RelayURLs:   cfg.Nydus.RelayURLs,
			}
			nydusManager = node.NewNydusManager(identity, nydusCfg)
		}

		contributorSvc := inference.NewContributorService(contributorCfg, identity, providerRegistry, peerPublicHandler.PeerAddresses, nydusManager)
		contributorSvc.Start()

		// Squad engine (multi-node team collaboration)
		selfAddr := cfg.Node.Address
		if selfAddr == "" {
			selfAddr = fmt.Sprintf("http://localhost:%d", cfg.Server.Port)
		}
		squadEngine := squad.NewEngine(db, identity, nydusManager, hivemindEngine, providerRegistry, toolRegistry, selfAddr)
		squadEngine.Start()

		// Git HTTP Smart Protocol server (enables remote nodes to clone/push via HTTP)
		gitHTTPHandler := squad.NewGitHTTPHandler(filepath.Join(tool.GetDataDir(), "repos"))
		r.Any("/v1/git/*path", func(c *gin.Context) {
			// Strip /v1/git/ prefix and forward to Git HTTP handler
			c.Request.URL.Path = strings.TrimPrefix(c.Request.URL.Path, "/v1/git/")
			gitHTTPHandler.ServeHTTP(c.Writer, c.Request)
		})

		// Memory lifecycle (decay, purge, capacity enforcement)
		memLifecycle := memory.NewLifecycleManager(db)
		memLifecycle.Start()

		// Wire contributor info into swarm heartbeat for mining reporting
		if len(swarmClient) > 0 && swarmClient[0] != nil {
			sc := swarmClient[0]
			sc.ContributorInfoFunc = func() *swarm.ContributorInfo {
				isC, models, gpu := contributorSvc.GetContributorInfo()
				if !isC {
					return nil
				}
				return &swarm.ContributorInfo{
					IsContributor: true,
					Models:        models,
					GPUInfo:       gpu,
				}
			}
		}

		// A2A (Agent-to-Agent) protocol endpoints (public)
		a2aHandler := network.NewA2AHandler(db, providerRegistry, toolRegistry)
		apiV1.POST("/a2a", a2aHandler.HandleRPC)

		// Seed platform model configs in hosted mode
		if cfg.Server.DeployMode == "hosted" {
			infra.SeedPlatformModels(db)
		}

		// Public webhook trigger (no auth)
		workflowHandler := wf.NewWorkflowHandler(db, providerRegistry, toolRegistry)
		apiV1.POST("/webhooks/workflow/:token", workflowHandler.Webhook)

		// Static file serving (uploads, screenshots, videos, images, docs, MCP bridge)
		registerStaticServeRoutes(apiV1)

		// Sandbox workspace preview (serve static files from workspace)
		// For .md files, render as styled HTML; for others, serve as-is
		apiV1.GET("/preview/:workspace_id/*filepath", func(c *gin.Context) {
			wsID := c.Param("workspace_id")
			filePath := c.Param("filepath")
			if filePath == "" || filePath == "/" {
				filePath = "/index.html"
			}
			ws := sandboxMgr.GetOrCreateWorkspace(wsID)
			absPath := filepath.Join(ws.Path, filepath.Clean(filePath))
			// Security: ensure path is within workspace
			if !strings.HasPrefix(absPath, ws.Path) {
				c.String(403, "forbidden")
				return
			}
			ext := strings.ToLower(filepath.Ext(absPath))
			if ext == ".md" || ext == ".markdown" {
				html, err := renderFileToStyledHTML(absPath)
				if err != nil {
					c.String(500, "render error: "+err.Error())
					return
				}
				c.Data(200, "text/html; charset=utf-8", []byte(html))
				return
			}
			c.File(absPath)
		})

		// Serve static BP style CSS (public, no auth)
		apiV1.GET("/static/bp-style.css", func(c *gin.Context) {
			c.Data(200, "text/css; charset=utf-8", []byte(bpStyleCSS))
		})

		// Convert workspace file (HTML/MD/TXT) to PDF via headless Chromium
		apiV1.GET("/pdf/:workspace_id/*filepath", func(c *gin.Context) {
			wsID := c.Param("workspace_id")
			filePath := c.Param("filepath")
			if filePath == "" || filePath == "/" {
				filePath = "/index.html"
			}
			ws := sandboxMgr.GetOrCreateWorkspace(wsID)
			absPath := filepath.Join(ws.Path, filepath.Clean(filePath))
			if !strings.HasPrefix(absPath, ws.Path) {
				c.String(403, "forbidden")
				return
			}
			if _, err := os.Stat(absPath); os.IsNotExist(err) {
				c.JSON(404, gin.H{"error": "file not found"})
				return
			}

			// Convert file to styled HTML (handles MD, plain HTML, text)
			styledHTML, err := renderFileToStyledHTML(absPath)
			if err != nil {
				c.JSON(500, gin.H{"error": "render failed: " + err.Error()})
				return
			}

			// Write styled HTML to temp file for chromedp
			tmpFile, err := os.CreateTemp("", "bp-*.html")
			if err != nil {
				c.JSON(500, gin.H{"error": "temp file failed: " + err.Error()})
				return
			}
			tmpPath := tmpFile.Name()
			defer os.Remove(tmpPath)
			tmpFile.WriteString(styledHTML)
			tmpFile.Close()

			// Use chromedp to convert to PDF
			opts := append(chromedp.DefaultExecAllocatorOptions[:],
				chromedp.Flag("no-sandbox", true),
				chromedp.Flag("disable-gpu", true),
				chromedp.Flag("disable-dev-shm-usage", true),
			)
			allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
			defer allocCancel()

			ctx, cancel := chromedp.NewContext(allocCtx)
			defer cancel()

			ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
			defer cancel()

			fileURL := "file://" + tmpPath
			var pdfBuf []byte

			if err := chromedp.Run(ctx,
				chromedp.Navigate(fileURL),
				chromedp.WaitReady("body"),
				chromedp.Sleep(500*time.Millisecond),
				chromedp.ActionFunc(func(ctx context.Context) error {
					buf, _, err := page.PrintToPDF().
						WithPrintBackground(true).
						WithPreferCSSPageSize(true).
						WithMarginTop(0.4).
						WithMarginBottom(0.4).
						WithMarginLeft(0.4).
						WithMarginRight(0.4).
						Do(ctx)
					if err != nil {
						return err
					}
					pdfBuf = buf
					return nil
				}),
			); err != nil {
				c.JSON(500, gin.H{"error": "PDF conversion failed: " + err.Error()})
				return
			}

			// Derive PDF filename from original
			baseName := strings.TrimSuffix(filepath.Base(absPath), filepath.Ext(absPath))
			c.Header("Content-Type", "application/pdf")
			c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.pdf"`, baseName))
			c.Data(200, "application/pdf", pdfBuf)
		})

		// Reverse proxy for running web apps
		appProxy := func(c *gin.Context) {
			wsID := c.Param("workspace_id")
			app := processMgr.GetApp(wsID)
			if app == nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "no app running for this workspace"})
				return
			}
			if !app.Ready {
				c.JSON(http.StatusServiceUnavailable, gin.H{"error": "app is still starting up, please wait a moment"})
				return
			}

			target, _ := url.Parse(fmt.Sprintf("http://localhost:%d", app.Port))
			proxy := httputil.NewSingleHostReverseProxy(target)
			proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
				w.WriteHeader(http.StatusBadGateway)
				w.Write([]byte(fmt.Sprintf(`{"error":"app unreachable: %v"}`, err)))
			}

			// Strip the /v1/app/:workspace_id prefix
			originalPath := c.Param("path")
			if originalPath == "" {
				originalPath = "/"
			}
			c.Request.URL.Path = originalPath
			c.Request.Host = target.Host
			proxy.ServeHTTP(c.Writer, c.Request)
		}
		apiV1.GET("/app/:workspace_id/*path", appProxy)
		apiV1.POST("/app/:workspace_id/*path", appProxy)
		apiV1.PUT("/app/:workspace_id/*path", appProxy)
		apiV1.DELETE("/app/:workspace_id/*path", appProxy)

		// WebSocket endpoint (authenticated via query param token — supports JWT and Owner Token)
		wsHub := ws.GetHub()
		apiV1.GET("/ws", func(c *gin.Context) {
			token := c.Query("token")
			if token == "" {
				c.JSON(401, gin.H{"error": "token required"})
				return
			}
			claims, err := middleware.ResolveToken(token, cfg, db)
			if err != nil {
				c.JSON(401, gin.H{"error": "invalid token"})
				return
			}
			ws.HandleWS(wsHub, claims.UserID, c.Writer, c.Request)
		})

		// Public agent sharing (no auth)
		sharedAgentHandler := v1.NewAgentHandler(db)
		apiV1.GET("/agents/shared/:id", sharedAgentHandler.GetShared)

		// Protected routes
		protected := apiV1.Group("")
		protected.Use(middleware.AuthRequired(cfg, db))
		{
			// Agents
			agentHandler := v1.NewAgentHandler(db)
			protected.GET("/agents", agentHandler.List)
			protected.POST("/agents", agentHandler.Create)
			protected.GET("/agents/:id", agentHandler.Get)
			protected.PUT("/agents/:id", agentHandler.Update)
			protected.DELETE("/agents/:id", agentHandler.Delete)
			protected.GET("/agents/marketplace", agentHandler.ListPublic)
			protected.POST("/agents/:id/clone", agentHandler.Clone)
			protected.GET("/agents/:id/export", agentHandler.Export)
			protected.GET("/agents/:id/workflow", agentHandler.GetWorkflow)
			protected.POST("/agents/import", agentHandler.Import)
			protected.POST("/agents/:id/share", agentHandler.Share)
			protected.POST("/agents/super-agent", agentHandler.EnsureSuperAgent)
			protected.GET("/agents/installed-source-ids", agentHandler.InstalledSourceIDs)
			protected.POST("/agents/install-marketplace", agentHandler.InstallFromMarketplace)
			protected.DELETE("/agents/uninstall/:source_id", agentHandler.UninstallBySourceID)

			// Glands (腺体 — agent runtime configuration)
			glandHandler := infra.NewGlandHandler(db)
			protected.GET("/glands", glandHandler.List)
			protected.GET("/glands/:id", glandHandler.Get)
			protected.POST("/glands", glandHandler.Create)
			protected.PUT("/glands/:id", glandHandler.Update)
			protected.DELETE("/glands/:id", glandHandler.Delete)
			protected.POST("/glands/batch", glandHandler.BatchUpsert)
			protected.GET("/glands/decrypt", glandHandler.GetDecrypted)

			// Swarm (虫群)
			swarmHandler := v1.NewSwarmHandler(db)
			protected.GET("/swarm", swarmHandler.List)
			protected.GET("/swarm/:id", swarmHandler.Get)
			protected.POST("/swarm/:id/invest", swarmHandler.Invest)

			// Stardust (星尘)
			stardustHandler := v1.NewStardustHandler(db)
			protected.GET("/stardust", stardustHandler.Balance)
			protected.GET("/stardust/transactions", stardustHandler.Transactions)
			protected.POST("/stardust/enhance-hero", stardustHandler.EnhanceHero)
			protected.POST("/stardust/hatch", stardustHandler.Hatch)

			// Evolution path + realm choice
			growthChoiceH := game.NewGrowthChoiceHandler(db)
			protected.POST("/growth/choose-path", growthChoiceH.ChoosePath)
			protected.POST("/growth/choose-realm", growthChoiceH.ChooseRealm)

			// Endgame (awakening, fusion, rebirth)
			endgameH := game.NewEndgameHandler(db)
			protected.POST("/growth/awaken", endgameH.Awaken)
			protected.POST("/growth/fuse", endgameH.Fuse)
			protected.POST("/growth/rebirth", endgameH.Rebirth)

			// Agent Templates (Creep Marketplace)
			tplHandler := billingpkg.NewTemplateHandler(db, queenClient)
			protected.GET("/templates", tplHandler.List)
			protected.GET("/templates/categories", tplHandler.Categories)
			protected.GET("/templates/:id", tplHandler.Get)
			protected.POST("/templates", tplHandler.Publish)
			protected.POST("/templates/:id/install", tplHandler.Install)
			protected.POST("/templates/install-remote", tplHandler.InstallRemote)
			protected.GET("/templates/community", tplHandler.CommunityList)
			protected.GET("/templates/community/:id", tplHandler.CommunityGet)
			protected.POST("/templates/:id/rate", tplHandler.Rate)
			protected.POST("/templates/community/:id/purchase", tplHandler.Purchase)
			protected.GET("/templates/community/:id/access", tplHandler.CheckAccess)
			protected.GET("/templates/purchases", tplHandler.ListPurchases)
			protected.GET("/templates/purchases/:order_no/status", tplHandler.PollPurchaseStatus)

			// Inference (user-facing: route to contributors)
			protected.POST("/inference/completions", inferenceHandler.Infer)
			protected.GET("/inference/contributors", inferenceHandler.ListContributors)

			// Chat
			chatHandler := v1.NewChatHandler(db, providerRegistry, toolRegistry, embedder)
			protected.POST("/chat/completions", middleware.UserRateLimit(30, time.Minute, rdb), chatHandler.Chat)
			protected.GET("/conversations", chatHandler.ListConversations)
			protected.GET("/conversations/:id/messages", chatHandler.GetMessages)
			protected.PUT("/conversations/:id", chatHandler.RenameConversation)
			protected.DELETE("/conversations/:id", chatHandler.DeleteConversation)
			protected.GET("/conversations/:id/export", chatHandler.ExportConversation)
			protected.POST("/conversations/:id/pin", chatHandler.PinConversation)
			protected.POST("/conversations/batch-delete", chatHandler.BatchDeleteConversations)
			protected.PUT("/conversations/:id/messages/:msg_id/feedback", chatHandler.FeedbackMessage)
			protected.POST("/conversations/:id/messages/:msg_id/truncate", chatHandler.TruncateMessages)

			// Conversation context (linked tasks, workflows, videos)
			convCtxHandler := v1.NewConversationContextHandler(db)
			protected.GET("/conversations/:id/context", convCtxHandler.GetContext)

			// Tools / Skills
			skillsHandler := v1.NewSkillsHandler(db, toolRegistry)
			protected.GET("/tools", skillsHandler.ListTools)
			protected.GET("/skills", skillsHandler.ListSkills)

			// Models
			modelHandler := infra.NewModelHandler(db, providerRegistry, cfg.Server.DeployMode)
			protected.GET("/models", modelHandler.List)
			protected.GET("/models/available", modelHandler.AvailableModels)
			protected.POST("/models", modelHandler.Create)
			protected.PUT("/models/:id", modelHandler.Update)
			protected.DELETE("/models/:id", modelHandler.Delete)

			// Workflows (reuses workflowHandler from public scope)
			protected.GET("/workflows", workflowHandler.List)
			protected.POST("/workflows", workflowHandler.Create)
			protected.GET("/workflows/:id", workflowHandler.Get)
			protected.PUT("/workflows/:id", workflowHandler.Update)
			protected.DELETE("/workflows/:id", workflowHandler.Delete)
			protected.POST("/workflows/:id/run", workflowHandler.Run)
			protected.GET("/workflows/:id/runs", workflowHandler.ListRuns)
			protected.POST("/workflows/:id/webhook/enable", workflowHandler.EnableWebhook)
			protected.POST("/workflows/:id/webhook/disable", workflowHandler.DisableWebhook)
			protected.POST("/workflows/sync-project", workflowHandler.SyncProject)
			protected.GET("/workflows/projects", workflowHandler.ListProjects)

			// Knowledge Bases (RAG)
			kbHandler := knowledge.NewKnowledgeHandler(db, pipeline, embedder)
			protected.GET("/knowledge-bases", kbHandler.ListKBs)
			protected.POST("/knowledge-bases", kbHandler.CreateKB)
			protected.GET("/knowledge-bases/:id", kbHandler.GetKB)
			protected.DELETE("/knowledge-bases/:id", kbHandler.DeleteKB)
			protected.POST("/knowledge-bases/:id/documents", kbHandler.UploadDocument)
			protected.POST("/knowledge-bases/:id/documents/text", kbHandler.UploadText)
			protected.DELETE("/knowledge-bases/:id/documents/:doc_id", kbHandler.DeleteDocument)
			protected.POST("/knowledge-bases/:id/search", kbHandler.Search)

			// MCP Servers
			mcpHandler := infra.NewMCPHandler(db, toolRegistry)
			protected.GET("/mcp/servers", mcpHandler.ListServers)
			protected.POST("/mcp/servers", mcpHandler.AddServer)
			protected.DELETE("/mcp/servers/:id", mcpHandler.DeleteServer)
			protected.POST("/mcp/servers/:id/test", mcpHandler.TestServer)

			// Integrations (messaging platforms: Feishu, DingTalk, Slack, etc.)
			integrationHandler := infra.NewIntegrationHandler(db)
			protected.GET("/integrations", integrationHandler.List)
			protected.POST("/integrations", integrationHandler.Create)
			protected.PUT("/integrations/:id", integrationHandler.Update)
			protected.DELETE("/integrations/:id", integrationHandler.Delete)
			protected.POST("/integrations/:id/test", integrationHandler.Test)

			// Multi-Agent
			multiAgentHandler := v1.NewMultiAgentHandler(db, providerRegistry, toolRegistry)
			protected.POST("/multi-agent/run", multiAgentHandler.Run)

			// Teams — local multi-agent collaboration (Hexad layer)
			teamHandler := squadapi.NewTeamHandler(db)
			protected.GET("/teams", teamHandler.List)
			protected.POST("/teams", teamHandler.Create)
			protected.GET("/team-templates", teamHandler.ListTemplates)
			protected.GET("/teams/:id", teamHandler.Get)
			protected.PUT("/teams/:id", teamHandler.Update)
			protected.DELETE("/teams/:id", teamHandler.Delete)
			protected.POST("/teams/:id/members", teamHandler.AddMember)
			protected.DELETE("/teams/:id/members/:member_id", teamHandler.RemoveMember)

			// Squads (multi-node team collaboration)
			squadHandler := squadapi.NewSquadHandler(db, identity)
			protected.POST("/squads", squadHandler.CreateSquad)
			protected.GET("/squads", squadHandler.ListSquads)
			protected.GET("/squads/:id", squadHandler.GetSquad)
			protected.PUT("/squads/:id", squadHandler.UpdateSquad)
			protected.DELETE("/squads/:id", squadHandler.DeleteSquad)
			protected.POST("/squads/:id/invite", squadHandler.InviteMember)
			protected.GET("/squads/:id/members", squadHandler.ListMembers)
			protected.DELETE("/squads/:id/members/:nodeId", squadHandler.RemoveMember)
			protected.POST("/squads/:id/missions", squadHandler.CreateMission)
			protected.GET("/squads/:id/missions", squadHandler.ListMissions)
			protected.GET("/missions/:id", squadHandler.GetMission)
			protected.POST("/missions/:id/start", squadHandler.StartMission)
			protected.POST("/missions/:id/cancel", squadHandler.CancelMission)
			protected.GET("/missions", squadHandler.ListAllMissions)
			protected.GET("/missions/:id/steps", squadHandler.ListMissionSteps)
			protected.GET("/missions/:id/sprints", squadHandler.ListSprints)
			protected.POST("/missions/:id/feedback", squadHandler.SubmitFeedback)
			protected.GET("/missions/:id/reviews", squadHandler.ListStepReviews)

			// Forge (AI-Native Project Management)
			// When STARCLAW_FORGE_URL is set, proxy all /v1/forge/* to standalone forge-api.
			// Otherwise, use local Claw DB handlers (backward compat).
			if cfg.Forge.URL != "" {
				log.Printf("[router] forge proxy mode → %s", cfg.Forge.URL)
				protected.Any("/forge/*path", forgeProxy(cfg.Forge.URL))
			} else {
				forgeHandler := infra.NewForgeHandler(db)
				protected.POST("/forge/projects", forgeHandler.CreateProject)
				protected.GET("/forge/projects", forgeHandler.ListProjects)
				protected.GET("/forge/projects/:id", forgeHandler.GetProject)
				protected.PUT("/forge/projects/:id", forgeHandler.UpdateProject)
				protected.POST("/forge/projects/:id/issues", forgeHandler.CreateIssue)
				protected.GET("/forge/projects/:id/issues", forgeHandler.ListIssues)
				protected.GET("/forge/projects/:id/issues/:number", forgeHandler.GetIssue)
				protected.PUT("/forge/projects/:id/issues/:number", forgeHandler.UpdateIssue)
				protected.POST("/forge/projects/:id/issues/:number/comments", forgeHandler.AddIssueComment)
				protected.POST("/forge/projects/:id/milestones", forgeHandler.CreateMilestone)
				protected.GET("/forge/projects/:id/milestones", forgeHandler.ListMilestones)
				protected.POST("/forge/milestones/:ms_id/close", forgeHandler.CloseMilestone)
				protected.GET("/forge/projects/:id/board", forgeHandler.GetBoard)
			}

			// Dashboard
			dashboardHandler := infra.NewDashboardHandler(db)
			protected.GET("/dashboard/stats", dashboardHandler.Stats)

			// Node Growth System (one pet per Claw node)
			growthHandler := game.NewGrowthHandler(db, providerRegistry, identity)
			protected.GET("/growth", growthHandler.GetGrowth)
			protected.GET("/growth/milestones", growthHandler.GetMilestones)
			protected.GET("/growth/milestones/new", growthHandler.GetNewMilestones)
			protected.GET("/growth/daily-report", growthHandler.GetDailyReport)
			protected.GET("/growth/curve", growthHandler.GetGrowthCurve)
			protected.GET("/assets/overview", growthHandler.GetAssets)

			// Identity Recovery (protected — requires auth)
			protected.GET("/recovery/status", recoveryHandler.Status)
			protected.GET("/recovery/mnemonic", recoveryHandler.GetMnemonic)
			protected.POST("/recovery/confirm-mnemonic", recoveryHandler.ConfirmMnemonic)
			protected.POST("/recovery/bind-phone", recoveryHandler.BindPhone)
			protected.POST("/recovery/verify-phone", recoveryHandler.VerifyPhone)
			protected.POST("/recovery/backup", recoveryHandler.Backup)
			protected.GET("/recovery/address", recoveryHandler.Address)

			// Arena PK (proxy to Queen web → Arena service)
			arenaTarget := cfg.Swarm.ArenaURL
			if arenaTarget == "" {
				arenaTarget = cfg.Swarm.QueenURL
			}
			if arenaTarget != "" {
				log.Printf("[router] arena proxy → %s", arenaTarget)
				protected.Any("/arena/*path", arenaProxy(arenaTarget))
			}

			// Auth request management (protected — user approves/rejects on their Claw UI)
			protected.GET("/identity/auth-requests", authReqHandler.List)
			protected.POST("/identity/auth-request/:id/approve", authReqHandler.Approve)
			protected.POST("/identity/auth-request/:id/reject", authReqHandler.Reject)

			// Direct sign-challenge (protected — fallback for Claw-to-Queen internal use)
			protected.POST("/identity/sign-challenge", func(c *gin.Context) {
				var req struct {
					Challenge string `json:"challenge" binding:"required"`
				}
				if err := c.ShouldBindJSON(&req); err != nil {
					c.JSON(400, gin.H{"error": "challenge required"})
					return
				}
				sig := identity.Sign([]byte(req.Challenge))
				c.JSON(200, gin.H{
					"node_id":    identity.NodeID,
					"public_key": identity.PublicKeyHex(),
					"signature":  fmt.Sprintf("%x", sig),
					"challenge":  req.Challenge,
				})
			})

			// Settings
			settingsHandler := infra.NewSettingsHandler(db)
			protected.GET("/settings/profile", settingsHandler.GetProfile)
			protected.PUT("/settings/profile", settingsHandler.UpdateProfile)
			protected.PUT("/settings/password", settingsHandler.ChangePassword)
			protected.GET("/settings/api-keys", settingsHandler.GetAPIKeys)
			protected.GET("/auth/token", authHandler.GetAPIToken)
			protected.POST("/auth/token/regenerate", authHandler.RegenerateToken)
			protected.GET("/auth/devices", authHandler.ListDevices)
			protected.POST("/auth/devices/:deviceID/revoke", authHandler.RevokeDevice)

			// System: Swarm, Bounty, Updates
			var sc *swarm.Client
			if len(swarmClient) > 0 {
				sc = swarmClient[0]
			}
			if cfg.Overlord.Enabled && cfg.Overlord.OverlordURL != "" {
				oc = overlord.NewClient(cfg.Overlord)
				if identity != nil {
					oc.SetClawID(identity.NodeID)
				}
				if cfg.Node.Address != "" {
					oc.SetAddress(cfg.Node.Address)
				}
				if cfg.Node.WebURL != "" {
					oc.SetWebURL(cfg.Node.WebURL)
				}
				oc.TaskCountFunc = func() overlord.TaskStats {
					var running, queued int64
					db.Model(&model.Mission{}).Where("status IN ?", []string{"executing", "reviewing"}).Count(&running)
					db.Model(&model.Mission{}).Where("status = ?", "planning").Count(&queued)
					return overlord.TaskStats{Running: int(running), Queued: int(queued)}
				}
				oc.Start()
				log.Printf("[router] overlord client started: url=%s trading=%v", cfg.Overlord.OverlordURL, cfg.Trading.Enabled)
			}
			systemHandler := v1.NewSystemHandler(cfg, db, sc, identity, oc)
			protected.GET("/system/update", systemHandler.GetUpdateInfo)
			protected.POST("/system/update", systemHandler.TriggerUpdate)
			protected.POST("/system/update/check", systemHandler.ForceCheck)
			protected.GET("/system/update-log", systemHandler.GetUpdateLog)
			protected.GET("/system/identity/export", systemHandler.ExportIdentity)
			protected.POST("/system/identity/import", systemHandler.ImportIdentity)
			protected.GET("/system/bridge", systemHandler.GetBridgeStatus)
			protected.POST("/system/bridge/stop", systemHandler.StopBridge)
			protected.GET("/system/overlord", systemHandler.GetOverlordStatus)
			protected.POST("/system/overlord/join", systemHandler.JoinOverlord)
			protected.POST("/system/overlord/leave", systemHandler.LeaveOverlord)
			protected.GET("/system/swarm", systemHandler.GetSwarmStatus)
			protected.POST("/system/swarm/join", systemHandler.JoinSwarm)
			protected.POST("/system/swarm/leave", systemHandler.LeaveSwarm)
			protected.GET("/system/credits", systemHandler.GetCredits)
			protected.POST("/system/credits/transfer", systemHandler.TransferCredits)
			protected.GET("/system/credits/transactions", systemHandler.ListCreditTransactions)
			protected.GET("/system/bounty", systemHandler.GetBountyStatus)
			protected.GET("/system/mining", systemHandler.GetMiningStatus)
			protected.POST("/system/mining/toggle", systemHandler.ToggleMining)

			// Device Management
			deviceHandler := auth.NewDeviceHandler(db)
			protected.GET("/devices", deviceHandler.ListDevices)
			protected.POST("/devices/:id/approve", deviceHandler.ApproveDevice)
			protected.POST("/devices/:id/reject", deviceHandler.RejectDevice)
			protected.POST("/devices/:id/revoke", deviceHandler.RevokeDevice)

			// Queen Account Linking
			queenHandler := billingpkg.NewQueenAccountHandler(cfg, sc, identity)
			protected.GET("/queen/status", queenHandler.GetStatus)
			protected.POST("/queen/link", queenHandler.Link)
			protected.POST("/queen/link-claw", queenHandler.LinkWithClaw)
			protected.POST("/queen/auto-register", queenHandler.AutoRegister)
			protected.POST("/queen/unlink", queenHandler.Unlink)

			// Node Identity & Peer Networking
			var scOpt []interface{}
			if len(swarmClient) > 0 && swarmClient[0] != nil {
				scOpt = append(scOpt, swarmClient[0])
			}
			peerHandler := network.NewPeerHandler(db, cfg, scOpt...)
			protected.GET("/node/info", peerHandler.GetNodeInfo)
			protected.PUT("/node/config", peerHandler.UpdateNodeConfig)
			protected.POST("/node/auto-setup", peerHandler.AutoSetupNode)
			protected.GET("/peers/resolve", peerHandler.ResolveNode)
			protected.GET("/peers", peerHandler.ListPeers)
			protected.POST("/peers", peerHandler.AddPeer)
			protected.DELETE("/peers/:id", peerHandler.RemovePeer)
			protected.POST("/peers/:id/ping", peerHandler.PingPeer)

			// P7: P2P + Evolution (authenticated endpoints)
			protected.GET("/p2p/overview", p2pHandler.HandleP2POverview)
			protected.GET("/p2p/dht/stats", p2pHandler.HandleDHTStats)
			protected.GET("/p2p/creep/get", p2pHandler.HandleCreepGet)
			protected.POST("/p2p/creep/set", p2pHandler.HandleCreepSet)
			protected.GET("/p2p/creep/stats", p2pHandler.HandleCreepStats)
			protected.POST("/p2p/hivemind/route", p2pHandler.HandleHivemindRoute)
			protected.GET("/p2p/hivemind/stats", p2pHandler.HandleHivemindStats)
			protected.POST("/p2p/evolution/seed", p2pHandler.HandleEvolutionSeed)
			protected.POST("/p2p/evolution/eval", p2pHandler.HandleEvolutionEval)
			protected.GET("/p2p/evolution/best", p2pHandler.HandleEvolutionBest)
			protected.POST("/p2p/evolution/evolve", p2pHandler.HandleEvolutionEvolve)
			protected.GET("/p2p/evolution/stats", p2pHandler.HandleEvolutionStats)

			// P8: Agent Economy (marketplace listings, purchases, revenue, ratings)
			marketplaceHandler := market.NewMarketplaceHandler(db)
			// Public-ish browse (still auth required for user context)
			protected.GET("/marketplace/listings", marketplaceHandler.ListPublished)
			protected.GET("/marketplace/listings/:id", marketplaceHandler.GetListing)
			protected.GET("/marketplace/trending", marketplaceHandler.Trending)
			protected.GET("/marketplace/listings/:id/ratings", marketplaceHandler.ListRatings)
			protected.GET("/marketplace/listings/:id/access", marketplaceHandler.CheckAccess)
			// Purchasing
			protected.POST("/marketplace/listings/:id/purchase", marketplaceHandler.PurchaseAgent)
			protected.GET("/marketplace/purchases", marketplaceHandler.MyPurchases)
			protected.POST("/marketplace/listings/:id/rate", marketplaceHandler.CreateRating)
			// Creator
			protected.GET("/marketplace/creator/profile", marketplaceHandler.GetCreatorProfile)
			protected.POST("/marketplace/creator/register", marketplaceHandler.RegisterCreator)
			protected.GET("/marketplace/creator/dashboard", marketplaceHandler.CreatorDashboard)
			protected.GET("/marketplace/creator/revenue", marketplaceHandler.CreatorRevenueList)
			protected.GET("/marketplace/creator/listings", marketplaceHandler.MyListings)
			protected.POST("/marketplace/creator/listings", marketplaceHandler.CreateListing)
			protected.PUT("/marketplace/creator/listings/:id", marketplaceHandler.UpdateListing)
			protected.POST("/marketplace/creator/listings/:id/version", marketplaceHandler.PublishVersion)

			// P8: Observability (traces, alerts, logs)
			observeHandler := ops.NewObserveHandler(observeEngine, db)
			protected.GET("/observe/stats", observeHandler.ObserveStats)
			protected.GET("/observe/traces/:trace_id", observeHandler.GetTrace)
			protected.GET("/observe/spans", observeHandler.QuerySpans)
			protected.GET("/observe/logs", observeHandler.QueryLogs)
			// Alert rules
			protected.GET("/observe/alerts/rules", observeHandler.ListAlertRules)
			protected.POST("/observe/alerts/rules", observeHandler.CreateAlertRule)
			protected.PUT("/observe/alerts/rules/:id", observeHandler.UpdateAlertRule)
			protected.POST("/observe/alerts/rules/:id/toggle", observeHandler.ToggleAlertRule)
			protected.DELETE("/observe/alerts/rules/:id", observeHandler.DeleteAlertRule)
			// Alert history
			protected.GET("/observe/alerts/history", observeHandler.ListAlertHistory)
			protected.POST("/observe/alerts/history/:id/resolve", observeHandler.ResolveAlert)

			// P8: Webhook orchestration (event rules, logs, dead letter queue)
			webhookRuleHandler := ops.NewWebhookRuleHandler(webhookEngine, db)
			protected.GET("/webhooks/rules", webhookRuleHandler.ListRules)
			protected.POST("/webhooks/rules", webhookRuleHandler.CreateRule)
			protected.PUT("/webhooks/rules/:id", webhookRuleHandler.UpdateRule)
			protected.POST("/webhooks/rules/:id/toggle", webhookRuleHandler.ToggleRule)
			protected.DELETE("/webhooks/rules/:id", webhookRuleHandler.DeleteRule)
			protected.GET("/webhooks/logs", webhookRuleHandler.ListLogs)
			protected.POST("/webhooks/logs/:id/retry", webhookRuleHandler.RetryDeadLetter)
			protected.GET("/webhooks/stats", webhookRuleHandler.Stats)
			protected.GET("/webhooks/event-types", webhookRuleHandler.EventTypes)
			protected.POST("/webhooks/test", webhookRuleHandler.TestRule)

			// P9: Developer portal (plugins, playground)
			protected.GET("/developer/plugins", devHandler.ListPlugins)
			protected.GET("/developer/plugins/:id", devHandler.GetPlugin)
			protected.POST("/developer/plugins", devHandler.PublishPlugin)
			protected.GET("/developer/plugins/mine", devHandler.MyPlugins)
			protected.POST("/developer/plugins/:id/install", devHandler.InstallPlugin)
			protected.DELETE("/developer/plugins/:id/install", devHandler.UninstallPlugin)
			protected.GET("/developer/plugins/installed", devHandler.MyInstalled)
			protected.POST("/developer/plugins/:id/rate", devHandler.RatePlugin)
			protected.POST("/developer/playground/execute", devHandler.PlaygroundExecute)
			protected.GET("/developer/playground/history", devHandler.PlaygroundHistory)
			protected.GET("/developer/stats", devHandler.DeveloperStats)

			// P9: Security (encryption, audit chain, GDPR, compliance)
			securityHandler := ops.NewSecurityHandler(db, keyMgr, auditChain)
			protected.GET("/security/encryption", securityHandler.EncryptionStatus)
			protected.GET("/security/overview", securityHandler.SecurityOverview)
			protected.GET("/security/audit", securityHandler.AuditChainQuery)
			protected.GET("/security/audit/verify", securityHandler.AuditChainVerify)
			protected.GET("/security/audit/export", securityHandler.AuditChainExport)
			protected.GET("/security/audit/stats", securityHandler.AuditChainStats)
			protected.GET("/security/gdpr/export", securityHandler.GDPRExportData)
			protected.POST("/security/gdpr/delete", securityHandler.GDPRDeleteData)
			protected.GET("/security/gdpr/consent", securityHandler.GDPRConsentStatus)
			protected.GET("/security/compliance", securityHandler.ComplianceChecklist)

			// P10: AI-Native (multimodal, proactive goals, multi-agent collaboration)
			advancedHandler := v1.NewAgentAdvancedHandler(db, mmRouter, proactiveEngine, collabEngine)
			// Multimodal
			protected.POST("/multimodal/chat", advancedHandler.MultimodalChat)
			protected.GET("/multimodal/modalities", advancedHandler.SupportedModalities)
			// Proactive goals
			protected.POST("/goals", advancedHandler.CreateGoal)
			protected.GET("/goals", advancedHandler.ListGoals)
			protected.GET("/goals/:id", advancedHandler.GetGoal)
			protected.POST("/goals/:id/activate", advancedHandler.ActivateGoal)
			protected.POST("/goals/:id/cancel", advancedHandler.CancelGoal)
			protected.GET("/goals/stats", advancedHandler.GoalStats)
			protected.GET("/goals/decomposition-prompt", advancedHandler.DecompositionPrompt)
			// Multi-agent collaboration
			protected.POST("/collaborations", advancedHandler.CreateCollaboration)
			protected.GET("/collaborations", advancedHandler.ListCollaborations)
			protected.POST("/collaborations/:id/join", advancedHandler.JoinCollaboration)
			protected.GET("/collaborations/:id/members", advancedHandler.CollaborationMembers)
			protected.GET("/collaborations/:id/messages", advancedHandler.CollaborationMessages)
			protected.POST("/collaborations/:id/messages", advancedHandler.SendCollaborationMessage)
			protected.POST("/collaborations/:id/vote", advancedHandler.SubmitVote)

			// P10: Fine-tuning & Knowledge Distillation
			fineTuneHandler := infra.NewFineTuneHandler(db, fineTuneEngine)
			protected.GET("/finetune/adapters", fineTuneHandler.ListAdapters)
			protected.POST("/finetune/adapters", fineTuneHandler.CreateAdapter)
			protected.GET("/finetune/adapters/:id", fineTuneHandler.GetAdapter)
			protected.DELETE("/finetune/adapters/:id", fineTuneHandler.DeleteAdapter)
			protected.POST("/finetune/adapters/:id/train", fineTuneHandler.StartTraining)
			protected.GET("/finetune/adapters/:id/export", fineTuneHandler.ExportSamples)
			protected.GET("/finetune/adapters/:id/samples", fineTuneHandler.ListSamples)
			protected.POST("/finetune/adapters/:id/samples", fineTuneHandler.AddSample)
			protected.POST("/finetune/adapters/:id/samples/batch", fineTuneHandler.AddSamplesBatch)
			protected.DELETE("/finetune/samples/:sample_id", fineTuneHandler.DeleteSample)
			protected.GET("/finetune/distillation", fineTuneHandler.ListDistillationJobs)
			protected.POST("/finetune/distillation", fineTuneHandler.CreateDistillationJob)
			protected.GET("/finetune/distillation/:id", fineTuneHandler.GetDistillationJob)
			protected.POST("/finetune/distillation/:id/cancel", fineTuneHandler.CancelDistillationJob)
			protected.GET("/finetune/distillation/prompt", fineTuneHandler.DistillationPrompt)
			protected.GET("/finetune/stats", fineTuneHandler.FineTuneStats)

			// Workflow Templates (Marketplace)
			wfTemplateHandler := wf.NewWorkflowTemplateHandler(db)
			protected.GET("/workflow-templates", wfTemplateHandler.List)
			protected.POST("/workflow-templates", wfTemplateHandler.Publish)
			protected.POST("/workflow-templates/:id/clone", wfTemplateHandler.Clone)
			protected.DELETE("/workflow-templates/:id", wfTemplateHandler.Delete)

			// Schedules (Cron)
			scheduleHandler := wf.NewScheduleHandler(db)
			protected.GET("/schedules", scheduleHandler.List)
			protected.POST("/schedules", scheduleHandler.Create)
			protected.DELETE("/schedules/batch", scheduleHandler.BatchDelete)
			protected.POST("/schedules/:id/toggle", scheduleHandler.Toggle)
			protected.DELETE("/schedules/:id", scheduleHandler.Delete)

			// Activities (Instinct — proactive behavior system)
			activityHandler := wf.NewActivityHandler(db, instinctEngine)
			protected.GET("/activities", activityHandler.List)
			protected.GET("/activities/templates", activityHandler.Templates)
			protected.POST("/activities/seed", activityHandler.Seed)
			protected.POST("/activities/batch-disable", activityHandler.BatchDisable)
			protected.POST("/activities", activityHandler.Create)
			protected.GET("/activities/:id", activityHandler.Get)
			protected.PUT("/activities/:id", activityHandler.Update)
			protected.POST("/activities/:id/toggle", activityHandler.Toggle)
			protected.DELETE("/activities/:id", activityHandler.Delete)
			protected.GET("/activities/:id/logs", activityHandler.Logs)
			protected.POST("/activities/events/:event", activityHandler.FireEvent)

			// Audit Logs
			auditHandler := ops.NewAuditHandler(db)
			protected.GET("/audit-logs", auditHandler.List)

			// Agent Evaluation
			evalHandler := infra.NewEvalHandler(db, providerRegistry, toolRegistry)
			protected.GET("/eval/test-cases", evalHandler.ListTestCases)
			protected.POST("/eval/test-cases", evalHandler.CreateTestCase)
			protected.DELETE("/eval/test-cases/:id", evalHandler.DeleteTestCase)
			protected.POST("/eval/test-cases/:id/run", evalHandler.RunTestCase)
			protected.GET("/eval/runs", evalHandler.ListTestRuns)

			// Long-term Memory
			memoryHandler := knowledge.NewMemoryHandler(db)
			protected.GET("/memories", memoryHandler.List)
			protected.POST("/memories", memoryHandler.Create)
			protected.PUT("/memories/:id", memoryHandler.Update)
			protected.DELETE("/memories/:id", memoryHandler.Delete)
			protected.DELETE("/memories", memoryHandler.Clear)
			protected.GET("/memories/stats", memoryHandler.Stats)
			protected.GET("/memories/recall/:agent_id", memoryHandler.Recall)

			// Multimodal (image upload, STT, TTS)
			multimodalHandler := media.NewMultimodalHandler(cfg, db)
			protected.POST("/multimodal/upload-image", multimodalHandler.UploadImage)
			protected.POST("/multimodal/stt", multimodalHandler.SpeechToText)
			protected.POST("/multimodal/tts", multimodalHandler.TextToSpeech)

			// File upload (general: documents, audio, video, code, etc.)
			protected.POST("/upload", media.UploadFile)

			// Coding Agent
			codingHandler := infra.NewCodingHandler(db, sandboxMgr, providerRegistry, toolRegistry)
			protected.POST("/coding/run", codingHandler.Run)
			protected.POST("/coding/execute", codingHandler.ExecuteCode)
			protected.POST("/coding/run-file", codingHandler.RunFile)
			protected.POST("/coding/run-command", codingHandler.RunCommand)
			protected.POST("/coding/stop", codingHandler.StopExecution)
			protected.GET("/coding/workspace/:workspace_id/files", codingHandler.ListWorkspaceFiles)
			protected.GET("/coding/workspace/:workspace_id/file", codingHandler.ReadWorkspaceFile)

			// Tasks (autonomous background execution)
			taskHandler := wf.NewTaskHandler(db, taskWorker)
			protected.GET("/tasks", taskHandler.ListTasks)
			protected.POST("/tasks", taskHandler.CreateTask)
			protected.GET("/tasks/visualization", taskHandler.Visualization)
			protected.GET("/tasks/:id", taskHandler.GetTask)
			protected.POST("/tasks/batch-cancel", taskHandler.BatchCancelTasks)
			protected.POST("/tasks/:id/cancel", taskHandler.CancelTask)
			protected.POST("/tasks/:id/pause", taskHandler.PauseTask)
			protected.POST("/tasks/:id/resume", taskHandler.ResumeTask)
			protected.POST("/tasks/worker/pause", taskHandler.WorkerPause)
			protected.POST("/tasks/worker/resume", taskHandler.WorkerResume)
			protected.POST("/tasks/worker/stop", taskHandler.WorkerStop)
			protected.GET("/tasks/worker/status", taskHandler.WorkerStatus)

			// Notifications
			protected.GET("/notifications", taskHandler.ListNotifications)
			protected.POST("/notifications/read", taskHandler.MarkNotificationsRead)
			protected.GET("/notifications/unread-count", taskHandler.UnreadCount)

			// Videos (generated video gallery)
			videoHandler := media.NewVideoHandler(db, toolRegistry)
			protected.GET("/videos", videoHandler.List)
			protected.POST("/videos/generate", videoHandler.Generate)
			protected.DELETE("/videos/:id", videoHandler.Delete)
			protected.POST("/videos/:id/cancel", videoHandler.Cancel)
			protected.POST("/videos/:id/retry", videoHandler.Retry)
			protected.POST("/videos/:id/regenerate", videoHandler.Regenerate)
			protected.POST("/videos/merge", videoHandler.MergeByTaskIDs)
			protected.POST("/videos/:id/remerge", videoHandler.Remerge)
			protected.POST("/videos/:id/dub", videoHandler.Dub)
			protected.POST("/videos/:id/add-music", videoHandler.AddMusic)
			protected.GET("/videos/voices", videoHandler.ListVoices)
			// Archive a generated video (Seedance TOS URL) into docs/<project>/production/<ep>/clips_v2/
			protected.POST("/videos/archive", videoHandler.Archive)
			// One-shot backfill: scan all _generated_urls.json ledgers and write local_url to DB.
			protected.POST("/videos/backfill-archived", videoHandler.BackfillArchivedURLs)
			// Soft-delete succeeded clips whose TOS URL has expired and have no local_url (废片).
			protected.POST("/videos/cleanup-expired", videoHandler.CleanupExpiredOrphans)
			// Publish picked takes to docs/<project>/episodes/<ep>/scenes/ + regenerate README.
			// Called when the user confirms an episode is ready (all scenes picked) so the
			// canonical script directory tree matches the finished episodes.
			protected.POST("/videos/publish-episode", videoHandler.PublishEpisode)

			// Project manifest helpers (used by workflow preflight self-check "一键修复")
			projectManifestH := media.NewProjectManifestHandler(db)
			protected.POST("/projects/:project/ref/suggest", projectManifestH.SuggestRef)
			// Prune a single candidate image file under /entities/* (used by the
			// LocalCandidateBar right-click "删除" action). Safe-guarded server-side
			// against path traversal, wrong extensions, and deleting files still
			// referenced by manifest.json.
			protected.POST("/projects/:project/ref/delete", projectManifestH.DeleteRef)
			protected.PUT("/projects/:project/manifest/characters/:key", projectManifestH.SetCharacterRef)
			// Partial update of character or prop fields (tos_url / cdn_url /
			// appearance_card / description / ref_clip). Called by NodePropertyPanel
			// after launderTOS/resignTOS succeeds so the newly generated URL actually
			// persists to manifest.json instead of only living in React state.
			protected.PATCH("/projects/:project/manifest/:kind/:key", projectManifestH.PatchManifestEntity)
			// v2: promote a candidate image (raw/nano/variants or external URL) into
			// entities/<kind>/<key>/sheets/unified_sheet_v<N+1>.png and patch manifest.ref.
			protected.POST("/projects/:project/entities/:kind/:key/promote", projectManifestH.PromoteToSheet)

			// nano-banana generate via StarAI provider: writes output to
			// docs/<project>/entities/<kind>/<key>/nano/<ts>_<slug>.png + sidecar.
			nanoH := media.NewNanoHandler(db)
			protected.POST("/nano/generate", nanoH.Generate)

			// Images, Music, Documents (media gallery)
			mediaHandler := media.NewMediaHandler(db, toolRegistry)
			protected.GET("/images", mediaHandler.ListImages)
			protected.POST("/images/generate", mediaHandler.GenerateImage)
			protected.DELETE("/images/:id", mediaHandler.DeleteImage)

			// Character Studio helpers (AI appearance card + CDN upload + TOS launder)
			charStudioHandler := media.NewCharacterStudioHandler(cfg, db, providerRegistry)
			protected.POST("/characters/generate-appearance", charStudioHandler.GenerateAppearance)
			protected.POST("/cdn/upload", charStudioHandler.CDNUpload)
			// Same as /cdn/upload but body is { image_url, drama, asset_type, filename }
			// — no multipart, reads bytes server-side. Used by the workflow
			// NodePropertyPanel's "🔼 同步本地图到 CDN" button.
			protected.POST("/cdn/upload-from-local", charStudioHandler.UploadFromLocal)
			protected.POST("/cdn/launder-tos", charStudioHandler.LaunderTOSURL)
			protected.POST("/cdn/resign-tos", charStudioHandler.ResignTOSURL)
			protected.POST("/cdn/promote-tos", charStudioHandler.PromoteToOwnedTOS)

			// Drama Writer Agent（短剧编剧 AI · 多维度审稿 + 发布文案）
			writerHandler := media.NewDramaWriterHandler(db, providerRegistry)
			protected.POST("/drama/writer/review", writerHandler.Review)
			protected.POST("/drama/writer/promo", writerHandler.GeneratePromo)
			protected.GET("/music", mediaHandler.ListMusic)
			protected.DELETE("/music/:id", mediaHandler.DeleteMusic)
			protected.GET("/documents", mediaHandler.ListDocuments)
			protected.GET("/documents/:workspace/*filepath", mediaHandler.GetDocument)
			protected.DELETE("/documents/:workspace/*filepath", mediaHandler.DeleteDocument)

			// Workspace folders (document tree browsing)
			wsHandler := infra.NewWorkspaceHandler(db)
			protected.GET("/doc-folders", wsHandler.ListFolders)
			protected.GET("/doc-folders/:conv_id", wsHandler.ListFolderFiles)
			protected.DELETE("/doc-folders/:conv_id", wsHandler.DeleteFolder)
			protected.POST("/doc-folders/:conv_id/lock", wsHandler.LockFolder)
			protected.POST("/doc-folders/:conv_id/unlock", wsHandler.UnlockFolder)

			// Billing & Tenant
			billingHandler := v1.NewBillingHandler(db)
			billingHandler.SeedPlans()
			protected.GET("/billing/plans", billingHandler.ListPlans)
			protected.GET("/billing/plan", billingHandler.GetCurrentPlan)
			protected.POST("/billing/recharge", billingHandler.Recharge)
			protected.GET("/billing/usage", billingHandler.GetUsageHistory)
			protected.GET("/billing/usage/daily", billingHandler.GetDailyUsage)
			protected.GET("/billing/transactions", billingHandler.ListTransactions)
			protected.GET("/tenant", billingHandler.GetTenant)
			protected.PUT("/tenant", billingHandler.UpdateTenant)
			protected.POST("/tenant/members", billingHandler.AddMember)
			protected.DELETE("/tenant/members/:user_id", billingHandler.RemoveMember)
			protected.PUT("/tenant/members/:user_id/role", billingHandler.UpdateMemberRole)

			// Admin routes (require admin role)
			admin := protected.Group("")
			admin.Use(middleware.RequireAdmin())
			{
				adminHandler := ops.NewAdminHandler(db)
				admin.GET("/admin/users", adminHandler.ListUsers)
				admin.PUT("/admin/users/:id/role", adminHandler.UpdateUserRole)
				admin.DELETE("/admin/users/:id", adminHandler.DeleteUser)
				admin.GET("/admin/stats", adminHandler.SystemStats)

				// P8: Marketplace admin review
				admin.GET("/admin/marketplace/pending", marketplaceHandler.AdminListPending)
				admin.POST("/admin/marketplace/listings/:id/review", marketplaceHandler.AdminReviewListing)
				admin.POST("/admin/marketplace/import", marketplaceHandler.AdminBulkImport)

				// P9: Plugin admin review
				admin.GET("/admin/plugins/pending", devHandler.AdminListPendingPlugins)
				admin.POST("/admin/plugins/:id/review", devHandler.AdminReviewPlugin)
			}
		}
	}

	// Serve embedded web frontend (SPA with fallback to index.html)
	web.RegisterRoutes(r)

	return r
}

// arenaProxy returns a Gin handler that reverse-proxies /v1/arena/* to the Arena service via Queen web.
// Path mapping: /v1/arena/threads → queen-web /arena/threads (queen-web nginx proxies /arena/ to arena:8095)
func arenaProxy(arenaBaseURL string) gin.HandlerFunc {
	target, err := url.Parse(arenaBaseURL)
	if err != nil {
		log.Printf("[arena-proxy] invalid arena URL %q: %v", arenaBaseURL, err)
		return func(c *gin.Context) {
			c.JSON(http.StatusBadGateway, gin.H{"error": "arena proxy misconfigured"})
		}
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte(`{"error":"arena service unreachable"}`))
	}
	return func(c *gin.Context) {
		subPath := c.Param("path")
		c.Request.URL.Path = "/arena" + subPath
		c.Request.Host = target.Host
		proxy.ServeHTTP(c.Writer, c.Request)
	}
}

// forgeProxy returns a Gin handler that reverse-proxies /v1/forge/* to the standalone forge-api.
// Path mapping: /v1/forge/projects → forge-api /api/projects
func forgeProxy(forgeURL string) gin.HandlerFunc {
	target, err := url.Parse(forgeURL)
	if err != nil {
		log.Printf("[forge-proxy] invalid forge URL %q: %v", forgeURL, err)
		return func(c *gin.Context) {
			c.JSON(http.StatusBadGateway, gin.H{"error": "forge proxy misconfigured"})
		}
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	return func(c *gin.Context) {
		// /v1/forge/projects → /api/projects
		subPath := c.Param("path")
		c.Request.URL.Path = "/api" + subPath
		c.Request.Host = target.Host
		proxy.ServeHTTP(c.Writer, c.Request)
	}
}
