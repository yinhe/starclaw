package router

import (
	"context"
	"encoding/json"
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
	"github.com/yinhe/starclaw/internal/browser"
	"github.com/yinhe/starclaw/internal/config"
	"github.com/yinhe/starclaw/internal/mcp"
	"github.com/yinhe/starclaw/internal/middleware"
	"github.com/yinhe/starclaw/internal/model"
	"github.com/yinhe/starclaw/internal/molt"
	"github.com/yinhe/starclaw/internal/node"
	"github.com/yinhe/starclaw/internal/provider"
	"github.com/yinhe/starclaw/internal/rag"
	"github.com/yinhe/starclaw/internal/sandbox"
	"github.com/yinhe/starclaw/internal/swarm"
	"github.com/yinhe/starclaw/internal/tool"
	"github.com/yinhe/starclaw/internal/worker"
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
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:5173", "http://localhost:3000", "http://localhost", "https://starclaw.me", "https://app.starclaw.me", "https://api.starclaw.me", "https://www.starclaw.me"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Prometheus metrics
	r.Use(middleware.PrometheusMetrics())

	// Rate limiting
	r.Use(middleware.RateLimit(300, time.Minute, rdb))

	// Provider registry
	providerRegistry := provider.NewRegistry()

	// Tool registry
	browserMgr := browser.NewManager()
	sandboxMgr := sandbox.NewManager()
	processMgr := sandbox.NewProcessManager()
	toolRegistry := tool.NewRegistry()
	toolRegistry.Register(tool.NewWebSearchTool(tool.WebSearchConfig{}))
	toolRegistry.Register(tool.NewHTTPRequestTool())
	toolRegistry.Register(tool.NewBrowserTool(browserMgr))
	toolRegistry.Register(tool.NewCodeTool(sandboxMgr, processMgr, db))
	videoTool := tool.NewVideoTool(db)
	toolRegistry.Register(videoTool)
	toolRegistry.Register(tool.NewDubbingTool(db))
	toolRegistry.Register(tool.NewMVTool(db))
	toolRegistry.Register(tool.NewComicTool(db))
	toolRegistry.Register(tool.NewMusicTool(db))
	toolRegistry.Register(tool.NewImageTool(db))
	toolRegistry.Register(tool.NewDocumentTool(db))
	toolRegistry.Register(tool.NewBountyTool(cfg.Swarm))
	toolRegistry.Register(tool.NewDeployWebTool())
	toolRegistry.Register(tool.NewBindDomainTool())
	toolRegistry.Register(tool.NewVerifyOnlineTool())
	toolRegistry.Register(tool.NewFeishuTool(db))
	toolRegistry.Register(tool.NewDingtalkTool(db))
	toolRegistry.Register(tool.NewWeComTool(db))
	toolRegistry.Register(tool.NewSlackTool(db))
	toolRegistry.Register(tool.NewDiscordTool(db))
	toolRegistry.Register(tool.NewTelegramTool(db))

	// Generate thumbnails for existing videos on startup
	go videoTool.GenerateMissingThumbnails()

	// Build delegate function for agent-to-agent delegation (breaks circular import)
	delegateFunc := func(ctx context.Context, ag model.Agent, modelCfg model.ModelConfig, message string) (*tool.DelegateResult, error) {
		p := provider.CreateFromConfig(providerRegistry, modelCfg)
		var enabledTools []string
		if ag.Tools != "" {
			json.Unmarshal([]byte(ag.Tools), &enabledTools)
		}
		messages := []provider.ChatMessage{
			{Role: "system", Content: ag.SystemPrompt},
			{Role: "user", Content: message},
		}
		rt := agentpkg.NewRuntime(p, toolRegistry)
		runReq := &agentpkg.RunRequest{
			Model:       modelCfg.ModelName,
			Messages:    messages,
			Tools:       enabledTools,
			Temperature: modelCfg.Temperature,
			MaxTokens:   modelCfg.MaxTokens,
		}
		result, err := rt.Run(ctx, runReq)
		if err != nil {
			return &tool.DelegateResult{Error: err.Error()}, nil
		}
		return &tool.DelegateResult{Content: result.Content}, nil
	}
	toolRegistry.Register(tool.NewSystemTool(db, providerRegistry, delegateFunc))

	// Load JSON tool plugins from plugins/ directory
	_ = tool.LoadPluginsFromDir(toolRegistry, "plugins")

	// Auto-detect and register MCP Bridge (host control)
	mcp.AutoRegisterBridge(toolRegistry)

	// Auto-migrate task & notification tables
	db.AutoMigrate(&model.Task{}, &model.Notification{}, &model.MusicRecord{}, &model.ImageRecord{}, &model.AgentTemplate{}, &model.Peer{})

	// Seed built-in agent templates (Creep marketplace)
	v1.SeedBuiltinTemplates(db)

	// Start background task worker (7x24 autonomous execution)
	taskWorker := worker.NewTaskWorker(db, providerRegistry, toolRegistry, 2)
	taskWorker.Start()

	// RAG embedding provider (configured via env or config)
	embedder := rag.NewOpenAIEmbedding(rag.OpenAIEmbeddingConfig{
		APIKey: cfg.OpenAI.APIKey,
		Model:  "text-embedding-3-small",
	})
	pipeline := rag.NewPipeline(db, embedder)

	r.Use(middleware.RequestLogger())

	// A2A Agent Card discovery (must be at root, not under /v1)
	a2aCardHandler := v1.NewA2AHandler(db, providerRegistry, toolRegistry)
	r.GET("/.well-known/agent.json", a2aCardHandler.AgentCardHandler)

	// Prometheus metrics endpoint
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Health check
	startTime := time.Now()
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
			"status":    "ok",
			"service":   "starclaw",
			"uptime_s":  int(time.Since(startTime).Seconds()),
			"db_status": dbOk,
		})
	})

	// API v1
	apiV1 := r.Group("/v1")
	{
		// Public routes
		identity := node.LoadOrCreateIdentity()
		authHandler := v1.NewAuthHandler(db, cfg, identity)
		apiV1.POST("/auth/register", authHandler.Register)
		apiV1.POST("/auth/login", authHandler.Login)
		apiV1.POST("/auth/phone/register", authHandler.PhoneRegister)
		apiV1.POST("/auth/phone/login", authHandler.PhoneLogin)
		apiV1.POST("/auth/token/login", authHandler.TokenLogin)

		// Setup (single-user Owner mode, opensource only)
		setupHandler := v1.NewSetupHandler(db, cfg, identity)
		apiV1.GET("/setup/status", setupHandler.Status)
		apiV1.POST("/setup", setupHandler.Setup)
		apiV1.POST("/auth/owner-login", setupHandler.PasswordLogin)

		// OAuth routes (public)
		oauthHandler := v1.NewOAuthHandler(db, cfg)
		apiV1.GET("/auth/oauth/providers", oauthHandler.GetOAuthConfig)
		apiV1.POST("/auth/oauth/github", oauthHandler.GitHubCallback)
		apiV1.POST("/auth/oauth/google", oauthHandler.GoogleCallback)

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

		// Peer-to-Peer inter-node endpoints (public, signature-verified)
		peerPublicHandler := v1.NewPeerHandler(db, cfg)
		apiV1.GET("/peer/handshake", peerPublicHandler.HandleHandshake)
		apiV1.GET("/peer/resolve", peerPublicHandler.HandleResolve)
		apiV1.POST("/peer/gossip", peerPublicHandler.HandleGossip)
		apiV1.POST("/peer/relay", peerPublicHandler.HandleRelayTask)

		// A2A (Agent-to-Agent) protocol endpoints (public)
		a2aHandler := v1.NewA2AHandler(db, providerRegistry, toolRegistry)
		apiV1.POST("/a2a", a2aHandler.HandleRPC)

		// Seed platform model configs in hosted mode
		if cfg.Server.DeployMode == "hosted" {
			v1.SeedPlatformModels(db)
		}

		// Public webhook trigger (no auth)
		workflowHandler := v1.NewWorkflowHandler(db, providerRegistry, toolRegistry)
		apiV1.POST("/webhooks/workflow/:token", workflowHandler.Webhook)

		// Uploaded files (public, secured by UUID filename)
		apiV1.GET("/uploads/:filename", v1.ServeUploadedFile)

		// Browser screenshots (public, secured by UUID)
		apiV1.GET("/screenshots/:id", v1.ServeScreenshot)

		// Merged videos (public, secured by UUID filename)
		apiV1.GET("/videos/merged/:filename", func(c *gin.Context) {
			filename := c.Param("filename")
			filePath := "/app/merged_videos/" + filename
			if _, err := os.Stat(filePath); os.IsNotExist(err) {
				c.JSON(404, gin.H{"error": "merged video not found"})
				return
			}
			c.Header("Content-Disposition", "attachment; filename="+filename)
			c.File(filePath)
		})

		// Video thumbnails (public, secured by UUID filename)
		apiV1.GET("/videos/thumbnails/:filename", func(c *gin.Context) {
			filename := c.Param("filename")
			filePath := "/app/thumbnails/" + filename
			if _, err := os.Stat(filePath); os.IsNotExist(err) {
				c.JSON(404, gin.H{"error": "thumbnail not found"})
				return
			}
			c.Header("Cache-Control", "public, max-age=86400")
			c.File(filePath)
		})

		// Generated images (public, secured by UUID filename)
		apiV1.GET("/images/:filename", func(c *gin.Context) {
			filename := c.Param("filename")
			filePath := "/app/images/" + filename
			if _, err := os.Stat(filePath); os.IsNotExist(err) {
				c.JSON(404, gin.H{"error": "image not found"})
				return
			}
			c.Header("Cache-Control", "public, max-age=86400")
			c.File(filePath)
		})

		// Generated Word documents (public, secured by UUID filename)
		apiV1.GET("/docx/:filename", func(c *gin.Context) {
			filename := c.Param("filename")
			if !strings.HasSuffix(filename, ".docx") {
				c.JSON(400, gin.H{"error": "invalid document format"})
				return
			}
			filePath := "/app/data/documents/" + filename
			if _, err := os.Stat(filePath); os.IsNotExist(err) {
				c.JSON(404, gin.H{"error": "document not found"})
				return
			}
			c.Header("Content-Disposition", "attachment; filename="+filename)
			c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.wordprocessingml.document")
			c.File(filePath)
		})

		// Music files (public, secured by UUID filename)
		apiV1.GET("/music/:filename", func(c *gin.Context) {
			filename := c.Param("filename")
			filePath := "/app/music/" + filename
			if _, err := os.Stat(filePath); os.IsNotExist(err) {
				c.JSON(404, gin.H{"error": "music file not found"})
				return
			}
			c.Header("Cache-Control", "public, max-age=86400")
			c.File(filePath)
		})

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

			// Agent Templates (Creep Marketplace)
			tplHandler := v1.NewTemplateHandler(db)
			protected.GET("/templates", tplHandler.List)
			protected.GET("/templates/categories", tplHandler.Categories)
			protected.GET("/templates/:id", tplHandler.Get)
			protected.POST("/templates", tplHandler.Publish)
			protected.POST("/templates/:id/install", tplHandler.Install)
			protected.POST("/templates/:id/rate", tplHandler.Rate)

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
			protected.GET("/tools", func(c *gin.Context) {
				c.JSON(200, gin.H{"tools": toolRegistry.List()})
			})
			protected.GET("/skills", func(c *gin.Context) {
				type SkillInfo struct {
					Name        string `json:"name"`
					Description string `json:"description"`
					Type        string `json:"type"`
					Status      string `json:"status"`
				}
				var skills []SkillInfo

				// Built-in tools
				builtinNames := []string{"system", "code", "web_search", "http_request", "browser", "video_generation", "deploy_web", "bind_domain", "verify_online"}
				for _, name := range toolRegistry.List() {
					t, ok := toolRegistry.Get(name)
					if !ok {
						continue
					}
					typ := "builtin"
					for _, bn := range builtinNames {
						if name == bn {
							typ = "builtin"
							break
						}
					}
					// Check if it's a plugin (starts with plugin prefix or not in builtin list)
					isBuiltin := false
					for _, bn := range builtinNames {
						if name == bn {
							isBuiltin = true
							break
						}
					}
					if !isBuiltin && name != "system" {
						if strings.HasPrefix(name, "mcp_") {
							typ = "mcp"
						} else {
							typ = "plugin"
						}
					}
					skills = append(skills, SkillInfo{
						Name:        name,
						Description: t.Description(),
						Type:        typ,
						Status:      "active",
					})
				}

				// MCP server tools
				userID := c.GetString("user_id")
				var mcpServers []model.MCPServer
				db.Where("user_id = ?", userID).Find(&mcpServers)
				for _, srv := range mcpServers {
					skills = append(skills, SkillInfo{
						Name:        "mcp:" + srv.Name,
						Description: fmt.Sprintf("MCP 外部服务: %s (%s)", srv.Name, srv.BaseURL),
						Type:        "mcp",
						Status:      srv.Status,
					})
				}

				// Count by type
				builtinCount, pluginCount, mcpCount := 0, 0, 0
				for _, s := range skills {
					switch s.Type {
					case "builtin":
						builtinCount++
					case "plugin":
						pluginCount++
					case "mcp":
						mcpCount++
					}
				}

				c.JSON(200, gin.H{
					"skills": skills,
					"summary": gin.H{
						"total":   len(skills),
						"builtin": builtinCount,
						"plugin":  pluginCount,
						"mcp":     mcpCount,
					},
				})
			})

			// Models
			modelHandler := v1.NewModelHandler(db, providerRegistry, cfg.Server.DeployMode)
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

			// Knowledge Bases (RAG)
			kbHandler := v1.NewKnowledgeHandler(db, pipeline, embedder)
			protected.GET("/knowledge-bases", kbHandler.ListKBs)
			protected.POST("/knowledge-bases", kbHandler.CreateKB)
			protected.GET("/knowledge-bases/:id", kbHandler.GetKB)
			protected.DELETE("/knowledge-bases/:id", kbHandler.DeleteKB)
			protected.POST("/knowledge-bases/:id/documents", kbHandler.UploadDocument)
			protected.POST("/knowledge-bases/:id/documents/text", kbHandler.UploadText)
			protected.DELETE("/knowledge-bases/:id/documents/:doc_id", kbHandler.DeleteDocument)
			protected.POST("/knowledge-bases/:id/search", kbHandler.Search)

			// MCP Servers
			mcpHandler := v1.NewMCPHandler(db, toolRegistry)
			protected.GET("/mcp/servers", mcpHandler.ListServers)
			protected.POST("/mcp/servers", mcpHandler.AddServer)
			protected.DELETE("/mcp/servers/:id", mcpHandler.DeleteServer)
			protected.POST("/mcp/servers/:id/test", mcpHandler.TestServer)

			// Integrations (messaging platforms: Feishu, DingTalk, Slack, etc.)
			integrationHandler := v1.NewIntegrationHandler(db)
			protected.GET("/integrations", integrationHandler.List)
			protected.POST("/integrations", integrationHandler.Create)
			protected.PUT("/integrations/:id", integrationHandler.Update)
			protected.DELETE("/integrations/:id", integrationHandler.Delete)
			protected.POST("/integrations/:id/test", integrationHandler.Test)

			// Multi-Agent
			multiAgentHandler := v1.NewMultiAgentHandler(db, providerRegistry, toolRegistry)
			protected.POST("/multi-agent/run", multiAgentHandler.Run)

			// Teams (lightweight orchestrator mapping)
			teamHandler := v1.NewTeamHandler(db)
			protected.GET("/teams/:id/orchestrator", teamHandler.GetOrchestrator)

			// Dashboard
			dashboardHandler := v1.NewDashboardHandler(db)
			protected.GET("/dashboard/stats", dashboardHandler.Stats)

			// Settings
			settingsHandler := v1.NewSettingsHandler(db)
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
			systemHandler := v1.NewSystemHandler(cfg, sc)
			protected.GET("/system/update", systemHandler.GetUpdateInfo)
			protected.POST("/system/update", systemHandler.TriggerUpdate)
			protected.POST("/system/update/check", systemHandler.ForceCheck)
			protected.GET("/system/bridge", systemHandler.GetBridgeStatus)
			protected.GET("/system/overlord", systemHandler.GetOverlordStatus)
			protected.POST("/system/overlord/join", systemHandler.JoinOverlord)
			protected.POST("/system/overlord/leave", systemHandler.LeaveOverlord)
			protected.GET("/system/swarm", systemHandler.GetSwarmStatus)
			protected.POST("/system/swarm/join", systemHandler.JoinSwarm)
			protected.POST("/system/swarm/leave", systemHandler.LeaveSwarm)
			protected.GET("/system/credits", systemHandler.GetCredits)
			protected.GET("/system/bounty", systemHandler.GetBountyStatus)

			// Device Management
			deviceHandler := v1.NewDeviceHandler(db)
			protected.GET("/devices", deviceHandler.ListDevices)
			protected.POST("/devices/:id/approve", deviceHandler.ApproveDevice)
			protected.POST("/devices/:id/reject", deviceHandler.RejectDevice)
			protected.POST("/devices/:id/revoke", deviceHandler.RevokeDevice)

			// Queen Account Linking
			queenHandler := v1.NewQueenAccountHandler(cfg, sc, identity)
			protected.GET("/queen/status", queenHandler.GetStatus)
			protected.POST("/queen/link", queenHandler.Link)
			protected.POST("/queen/unlink", queenHandler.Unlink)

			// Node Identity & Peer Networking
			var scOpt []interface{}
			if len(swarmClient) > 0 && swarmClient[0] != nil {
				scOpt = append(scOpt, swarmClient[0])
			}
			peerHandler := v1.NewPeerHandler(db, cfg, scOpt...)
			protected.GET("/node/info", peerHandler.GetNodeInfo)
			protected.PUT("/node/config", peerHandler.UpdateNodeConfig)
			protected.POST("/node/auto-setup", peerHandler.AutoSetupNode)
			protected.GET("/peers/resolve", peerHandler.ResolveNode)
			protected.GET("/peers", peerHandler.ListPeers)
			protected.POST("/peers", peerHandler.AddPeer)
			protected.DELETE("/peers/:id", peerHandler.RemovePeer)
			protected.POST("/peers/:id/ping", peerHandler.PingPeer)

			// Workflow Templates (Marketplace)
			wfTemplateHandler := v1.NewWorkflowTemplateHandler(db)
			protected.GET("/workflow-templates", wfTemplateHandler.List)
			protected.POST("/workflow-templates", wfTemplateHandler.Publish)
			protected.POST("/workflow-templates/:id/clone", wfTemplateHandler.Clone)
			protected.DELETE("/workflow-templates/:id", wfTemplateHandler.Delete)

			// Schedules (Cron)
			scheduleHandler := v1.NewScheduleHandler(db)
			protected.GET("/schedules", scheduleHandler.List)
			protected.POST("/schedules", scheduleHandler.Create)
			protected.POST("/schedules/:id/toggle", scheduleHandler.Toggle)
			protected.DELETE("/schedules/:id", scheduleHandler.Delete)

			// Audit Logs
			auditHandler := v1.NewAuditHandler(db)
			protected.GET("/audit-logs", auditHandler.List)

			// Agent Evaluation
			evalHandler := v1.NewEvalHandler(db, providerRegistry, toolRegistry)
			protected.GET("/eval/test-cases", evalHandler.ListTestCases)
			protected.POST("/eval/test-cases", evalHandler.CreateTestCase)
			protected.DELETE("/eval/test-cases/:id", evalHandler.DeleteTestCase)
			protected.POST("/eval/test-cases/:id/run", evalHandler.RunTestCase)
			protected.GET("/eval/runs", evalHandler.ListTestRuns)

			// Long-term Memory
			memoryHandler := v1.NewMemoryHandler(db)
			protected.GET("/memories", memoryHandler.List)
			protected.POST("/memories", memoryHandler.Create)
			protected.PUT("/memories/:id", memoryHandler.Update)
			protected.DELETE("/memories/:id", memoryHandler.Delete)
			protected.GET("/memories/recall/:agent_id", memoryHandler.Recall)

			// Multimodal (image upload, STT, TTS)
			multimodalHandler := v1.NewMultimodalHandler(cfg, db)
			protected.POST("/multimodal/upload-image", multimodalHandler.UploadImage)
			protected.POST("/multimodal/stt", multimodalHandler.SpeechToText)
			protected.POST("/multimodal/tts", multimodalHandler.TextToSpeech)

			// File upload (general: documents, audio, video, code, etc.)
			protected.POST("/upload", v1.UploadFile)

			// Coding Agent
			codingHandler := v1.NewCodingHandler(db, sandboxMgr, providerRegistry, toolRegistry)
			protected.POST("/coding/run", codingHandler.Run)
			protected.POST("/coding/execute", codingHandler.ExecuteCode)
			protected.POST("/coding/run-file", codingHandler.RunFile)
			protected.POST("/coding/run-command", codingHandler.RunCommand)
			protected.POST("/coding/stop", codingHandler.StopExecution)
			protected.GET("/coding/workspace/:workspace_id/files", codingHandler.ListWorkspaceFiles)
			protected.GET("/coding/workspace/:workspace_id/file", codingHandler.ReadWorkspaceFile)

			// Tasks (autonomous background execution)
			taskHandler := v1.NewTaskHandler(db, taskWorker)
			protected.GET("/tasks", taskHandler.ListTasks)
			protected.POST("/tasks", taskHandler.CreateTask)
			protected.GET("/tasks/visualization", taskHandler.Visualization)
			protected.GET("/tasks/:id", taskHandler.GetTask)
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
			protected.GET("/videos", func(c *gin.Context) {
				userID := c.GetString("user_id")
				var records []model.VideoRecord
				db.Where("user_id = ?", userID).Order("created_at DESC").Limit(500).Find(&records)

				// Background: generate missing thumbnails, retry narration, check for stuck merges
				go func() {
					vt, ok := toolRegistry.Get("video_generation")
					if !ok {
						return
					}
					// Generate thumbnails for videos that don't have one
					if vTool, ok := vt.(*tool.VideoTool); ok {
						vTool.GenerateMissingThumbnails()
					}
					// Retry narration for clips with narration text but no narrated_url
					if vTool, ok := vt.(*tool.VideoTool); ok {
						vTool.RetryNarration(userID)
					}
					// Find conversations with succeeded clips but no merged video
					type convRow struct{ ConversationID string }
					var convs []convRow
					db.Model(&model.VideoRecord{}).
						Select("DISTINCT conversation_id").
						Where("user_id = ? AND (type = 'clip' OR type = '') AND status = 'succeeded' AND conversation_id != ''", userID).
						Find(&convs)
					for _, cr := range convs {
						var mergeCount int64
						db.Model(&model.VideoRecord{}).Where("user_id = ? AND conversation_id = ? AND type = 'merged'", userID, cr.ConversationID).Count(&mergeCount)
						if mergeCount == 0 {
							// No merged video yet  check if all clips are done
							var running int64
							db.Model(&model.VideoRecord{}).Where("user_id = ? AND conversation_id = ? AND (type = 'clip' OR type = '') AND status IN ('running','pending')", userID, cr.ConversationID).Count(&running)
							if running == 0 {
								// All clips done, no merge  trigger via reflection
								if vTool, ok := vt.(*tool.VideoTool); ok {
									vTool.TryAutoMerge(userID, cr.ConversationID)
								}
							}
						}
					}
				}()

				c.JSON(200, gin.H{"videos": records})
			})
			protected.DELETE("/videos/:id", func(c *gin.Context) {
				id := c.Param("id")
				userID := c.GetString("user_id")
				result := db.Where("id = ? AND user_id = ?", id, userID).Delete(&model.VideoRecord{})
				if result.RowsAffected == 0 {
					c.JSON(404, gin.H{"error": "video not found"})
					return
				}
				c.JSON(200, gin.H{"message": "deleted"})
			})
			protected.POST("/videos/:id/cancel", func(c *gin.Context) {
				id := c.Param("id")
				userID := c.GetString("user_id")
				result := db.Model(&model.VideoRecord{}).
					Where("id = ? AND user_id = ? AND status IN ?", id, userID, []string{"running", "pending"}).
					Update("status", "cancelled")
				if result.RowsAffected == 0 {
					c.JSON(400, gin.H{"error": "video not found or cannot be cancelled"})
					return
				}
				c.JSON(200, gin.H{"status": "cancelled"})
			})
			protected.POST("/videos/:id/retry", func(c *gin.Context) {
				id := c.Param("id")
				userID := c.GetString("user_id")
				var rec model.VideoRecord
				if err := db.Where("id = ? AND user_id = ? AND status IN ?", id, userID, []string{"failed", "cancelled"}).First(&rec).Error; err != nil {
					c.JSON(400, gin.H{"error": "video not found or cannot be retried"})
					return
				}
				// Resubmit to the video tool
				videoTool, ok := toolRegistry.Get("video_generation")
				if !ok {
					c.JSON(500, gin.H{"error": "video tool not available"})
					return
				}
				// Reset status
				db.Model(&rec).Updates(map[string]interface{}{"status": "running", "video_url": ""})
				// Re-generate in background
				go func() {
					argsJSON := fmt.Sprintf(`{"action":"generate_video","prompt":%q,"model":%q,"size":%q,"duration":"%d","scene":%q}`,
						rec.Prompt, rec.Model, rec.Size, rec.Duration, rec.Scene)
					ctx := context.WithValue(context.Background(), tool.CtxKeyUserID, rec.UserID)
					ctx = context.WithValue(ctx, tool.CtxKeyConversationID, rec.ConversationID)
					// Delete old record before re-generating (new one will be created)
					db.Where("id = ?", rec.ID).Delete(&model.VideoRecord{})
					videoTool.Execute(ctx, argsJSON)
				}()
				c.JSON(200, gin.H{"status": "retrying"})
			})

			// Regenerate a clip (works for any status including succeeded)
			protected.POST("/videos/:id/regenerate", func(c *gin.Context) {
				id := c.Param("id")
				userID := c.GetString("user_id")
				var rec model.VideoRecord
				if err := db.Where("id = ? AND user_id = ?", id, userID).First(&rec).Error; err != nil {
					c.JSON(400, gin.H{"error": "video not found"})
					return
				}
				if rec.Type == "merged" || rec.Type == "mv" || rec.Type == "narrated" {
					c.JSON(400, gin.H{"error": "cannot regenerate a merged/mv/narrated video, use remerge instead"})
					return
				}
				videoTool, ok := toolRegistry.Get("video_generation")
				if !ok {
					c.JSON(500, gin.H{"error": "video tool not available"})
					return
				}
				// Delete old record and re-generate with same params
				oldConvID := rec.ConversationID
				oldUserID := rec.UserID
				go func() {
					argsJSON := fmt.Sprintf(`{"action":"generate_video","prompt":%q,"model":%q,"size":%q,"duration":"%d","scene":%q}`,
						rec.Prompt, rec.Model, rec.Size, rec.Duration, rec.Scene)
					ctx := context.WithValue(context.Background(), tool.CtxKeyUserID, oldUserID)
					ctx = context.WithValue(ctx, tool.CtxKeyConversationID, oldConvID)
					db.Where("id = ?", rec.ID).Delete(&model.VideoRecord{})
					videoTool.Execute(ctx, argsJSON)
				}()
				c.JSON(200, gin.H{"status": "regenerating", "message": "片段正在重新生成"})
			})

			// Re-merge a composite video (only the original clip_ids, not all clips in conversation)
			protected.POST("/videos/:id/remerge", func(c *gin.Context) {
				id := c.Param("id")
				userID := c.GetString("user_id")
				var rec model.VideoRecord
				if err := db.Where("id = ? AND user_id = ? AND type IN ?", id, userID, []string{"merged", "mv"}).First(&rec).Error; err != nil {
					c.JSON(400, gin.H{"error": "merged video not found"})
					return
				}
				videoTool, ok := toolRegistry.Get("video_generation")
				if !ok {
					c.JSON(500, gin.H{"error": "video tool not available"})
					return
				}
				convID := rec.ConversationID
				if convID == "" {
					c.JSON(400, gin.H{"error": "no conversation_id on this merged video"})
					return
				}
				// Parse original clip IDs from the merged record
				var clipIDs []string
				if err := json.Unmarshal([]byte(rec.ClipIDs), &clipIDs); err != nil || len(clipIDs) == 0 {
					c.JSON(400, gin.H{"error": "no clip_ids found in merged video"})
					return
				}
				// Get task_ids for these specific clips
				var clips []model.VideoRecord
				db.Where("id IN ?", clipIDs).Find(&clips)
				var taskIDs []string
				for _, clip := range clips {
					if clip.TaskID != "" {
						taskIDs = append(taskIDs, clip.TaskID)
					}
				}
				if len(taskIDs) == 0 {
					c.JSON(400, gin.H{"error": "no valid clips found for remerge"})
					return
				}
				// Delete old merged record
				db.Where("id = ?", rec.ID).Delete(&model.VideoRecord{})
				// Re-merge in background with specific task_ids
				taskIDsStr := strings.Join(taskIDs, ",")
				go func() {
					argsJSON := fmt.Sprintf(`{"action":"merge_videos","task_ids":%q}`, taskIDsStr)
					ctx := context.WithValue(context.Background(), tool.CtxKeyUserID, userID)
					ctx = context.WithValue(ctx, tool.CtxKeyConversationID, convID)
					videoTool.Execute(ctx, argsJSON)
				}()
				c.JSON(200, gin.H{"status": "remerging", "message": "正在重新合成视频"})
			})

			// Dub a video clip (add voiceover + subtitles)
			protected.POST("/videos/:id/dub", func(c *gin.Context) {
				id := c.Param("id")
				userID := c.GetString("user_id")

				var req struct {
					Text          string `json:"text"`
					Voice         string `json:"voice"`
					SubtitleStyle string `json:"subtitle_style"`
				}
				if err := c.ShouldBindJSON(&req); err != nil || req.Text == "" {
					c.JSON(400, gin.H{"error": "text (配音文案) is required"})
					return
				}

				var rec model.VideoRecord
				if err := db.Where("id = ? AND user_id = ?", id, userID).First(&rec).Error; err != nil {
					c.JSON(400, gin.H{"error": "video not found"})
					return
				}
				if rec.Status != "succeeded" || rec.VideoURL == "" {
					c.JSON(400, gin.H{"error": "video not ready yet"})
					return
				}

				dubbingTool, ok := toolRegistry.Get("dubbing")
				if !ok {
					c.JSON(500, gin.H{"error": "dubbing tool not available"})
					return
				}

				// Auto-split text into timed segments
				segments := tool.SplitNarrationToSegments(req.Text, 0, float64(rec.Duration), 15)
				segJSON, _ := json.Marshal(segments)

				voice := req.Voice
				if voice == "" {
					voice = "longyuan"
				}
				subStyle := req.SubtitleStyle
				if subStyle == "" {
					subStyle = "auto"
				}

				go func() {
					argsJSON := fmt.Sprintf(`{"action":"add_voiceover","video_id":%q,"narrations":%s,"voice":%q,"subtitle_style":%q}`,
						rec.ID, string(segJSON), voice, subStyle)
					ctx := context.WithValue(context.Background(), tool.CtxKeyUserID, userID)
					ctx = context.WithValue(ctx, tool.CtxKeyConversationID, rec.ConversationID)
					result, err := dubbingTool.Execute(ctx, argsJSON)
					if err != nil {
						log.Printf("[DubAPI] failed for video %s: %v", rec.ID, err)
					} else {
						log.Printf("[DubAPI] success for video %s: %s", rec.ID, result)
					}
				}()

				c.JSON(200, gin.H{
					"status":   "dubbing",
					"message":  fmt.Sprintf("配音任务已开始，音色: %s，共%d段旁白", voice, len(segments)),
					"segments": len(segments),
				})
			})

			// Add music to a video (compose MV)
			protected.POST("/videos/:id/add-music", func(c *gin.Context) {
				id := c.Param("id")
				userID := c.GetString("user_id")

				var req struct {
					MusicID       string `json:"music_id"`
					LyricsSRT     string `json:"lyrics_srt"`
					SubtitleStyle string `json:"subtitle_style"`
				}
				if err := c.ShouldBindJSON(&req); err != nil || req.MusicID == "" {
					c.JSON(400, gin.H{"error": "music_id is required"})
					return
				}

				var rec model.VideoRecord
				if err := db.Where("id = ? AND user_id = ?", id, userID).First(&rec).Error; err != nil {
					c.JSON(400, gin.H{"error": "video not found"})
					return
				}
				if rec.Status != "succeeded" || rec.VideoURL == "" {
					c.JSON(400, gin.H{"error": "video not ready yet"})
					return
				}

				mvTool, ok := toolRegistry.Get("mv_production")
				if !ok {
					c.JSON(500, gin.H{"error": "mv tool not available"})
					return
				}

				go func() {
					argsJSON := fmt.Sprintf(`{"action":"compose_mv","video_id":%q,"music_id":%q,"lyrics_srt":%q,"subtitle_style":%q}`,
						rec.ID, req.MusicID, req.LyricsSRT, req.SubtitleStyle)
					ctx := context.WithValue(context.Background(), tool.CtxKeyUserID, userID)
					ctx = context.WithValue(ctx, tool.CtxKeyConversationID, rec.ConversationID)
					result, err := mvTool.Execute(ctx, argsJSON)
					if err != nil {
						log.Printf("[AddMusicAPI] failed for video %s: %v", rec.ID, err)
					} else {
						log.Printf("[AddMusicAPI] success for video %s: %s", rec.ID, result)
					}
				}()

				c.JSON(200, gin.H{"status": "composing", "message": "正在合成配乐视频"})
			})

			// List available voices for dubbing
			protected.GET("/videos/voices", func(c *gin.Context) {
				dubbingTool, ok := toolRegistry.Get("dubbing")
				if !ok {
					c.JSON(500, gin.H{"error": "dubbing tool not available"})
					return
				}
				result, err := dubbingTool.Execute(context.Background(), `{"action":"list_voices"}`)
				if err != nil {
					c.JSON(500, gin.H{"error": err.Error()})
					return
				}
				var data map[string]interface{}
				json.Unmarshal([]byte(result), &data)
				c.JSON(200, data)
			})

			// Images (generated image gallery)
			protected.GET("/images", func(c *gin.Context) {
				userID := c.GetString("user_id")
				var records []model.ImageRecord
				db.Where("user_id = ?", userID).Order("created_at DESC").Limit(500).Find(&records)
				c.JSON(200, gin.H{"images": records})
			})
			protected.DELETE("/images/:id", func(c *gin.Context) {
				id := c.Param("id")
				userID := c.GetString("user_id")
				result := db.Where("id = ? AND user_id = ?", id, userID).Delete(&model.ImageRecord{})
				if result.RowsAffected == 0 {
					c.JSON(404, gin.H{"error": "image not found"})
					return
				}
				c.JSON(200, gin.H{"message": "deleted"})
			})

			// Music (generated music gallery)
			protected.GET("/music", func(c *gin.Context) {
				userID := c.GetString("user_id")
				var records []model.MusicRecord
				db.Where("user_id = ?", userID).Order("created_at DESC").Limit(500).Find(&records)
				c.JSON(200, gin.H{"music": records})
			})
			protected.DELETE("/music/:id", func(c *gin.Context) {
				id := c.Param("id")
				userID := c.GetString("user_id")
				result := db.Where("id = ? AND user_id = ?", id, userID).Delete(&model.MusicRecord{})
				if result.RowsAffected == 0 {
					c.JSON(404, gin.H{"error": "music not found"})
					return
				}
				c.JSON(200, gin.H{"message": "deleted"})
			})

			// Documents (workspace files) - DB-backed with filesystem fallback
			protected.GET("/documents", func(c *gin.Context) {
				baseDir := "/app/workspaces"
				userID := c.GetString("user_id")
				convFilter := c.Query("conversation_id")

				type DocFile struct {
					Name           string `json:"name"`
					Path           string `json:"path"`
					Workspace      string `json:"workspace"`
					Size           int64  `json:"size"`
					ModTime        string `json:"mod_time"`
					URL            string `json:"url"`
					Category       string `json:"category"`
					ConversationID string `json:"conversation_id"`
					ConvTitle      string `json:"conv_title"`
				}

				type ConvSummary struct {
					ID    string `json:"id"`
					Title string `json:"title"`
				}

				// 1. Query tracked files from DB (user-isolated)
				var dbFiles []model.WorkspaceFile
				q := db.Where("user_id = ?", userID).Order("created_at DESC")
				if convFilter != "" {
					q = q.Where("conversation_id = ?", convFilter)
				}
				q.Find(&dbFiles)

				// Build a set of tracked file keys (workspace:path) for dedup
				trackedKeys := map[string]bool{}
				// Collect unique conversation IDs
				convIDs := map[string]bool{}
				var docs []DocFile

				cst := time.FixedZone("CST", 8*3600)

				for _, f := range dbFiles {
					key := f.WorkspaceID + ":" + f.Path
					trackedKeys[key] = true
					if f.ConversationID != "" {
						convIDs[f.ConversationID] = true
					}

					// Get actual file size and mod_time from filesystem
					absPath := filepath.Join(baseDir, f.WorkspaceID, f.Path)
					modTime := f.UpdatedAt.In(cst).Format("2006-01-02 15:04:05")
					size := f.Size
					if info, err := os.Stat(absPath); err == nil {
						size = info.Size()
						modTime = info.ModTime().In(cst).Format("2006-01-02 15:04:05")
					}

					docs = append(docs, DocFile{
						Name:           f.Name,
						Path:           f.Path,
						Workspace:      f.WorkspaceID,
						Size:           size,
						ModTime:        modTime,
						URL:            fmt.Sprintf("/v1/documents/%s/%s", f.WorkspaceID, f.Path),
						Category:       f.Category,
						ConversationID: f.ConversationID,
					})
				}

				// 2. If no conversation filter, also scan filesystem for untracked files (user's workspace only)
				if convFilter == "" {
					skipExts := map[string]bool{
						".pyc": true, ".class": true, ".o": true, ".exe": true,
					}
					codeExts := map[string]bool{
						".py": true, ".js": true, ".ts": true, ".jsx": true, ".tsx": true,
						".go": true, ".java": true, ".c": true, ".cpp": true, ".h": true,
						".rs": true, ".rb": true, ".php": true, ".sh": true, ".bat": true,
						".ps1": true, ".sql": true, ".r": true, ".m": true, ".swift": true,
						".kt": true, ".scala": true, ".lua": true, ".pl": true, ".dart": true,
						".css": true, ".scss": true, ".less": true, ".vue": true, ".svelte": true,
						".json": true, ".yaml": true, ".yml": true, ".toml": true, ".xml": true,
						".ini": true, ".cfg": true, ".conf": true, ".env": true,
						".html": true, ".htm": true,
					}
					// Only scan user's own workspace directory
					wsID := userID
					wsPath := filepath.Join(baseDir, wsID)
					if _, err := os.Stat(wsPath); err == nil {
						filepath.Walk(wsPath, func(path string, info os.FileInfo, err error) error {
							if err != nil || info.IsDir() {
								return nil
							}
							name := info.Name()
							if strings.HasPrefix(name, "_exec") || name == "Main.class" {
								return nil
							}
							ext := strings.ToLower(filepath.Ext(name))
							if skipExts[ext] {
								return nil
							}
							relPath, _ := filepath.Rel(wsPath, path)
							key := wsID + ":" + relPath
							if trackedKeys[key] {
								return nil // already from DB
							}
							category := "document"
							if codeExts[ext] {
								category = "code"
							}
							docs = append(docs, DocFile{
								Name:      name,
								Path:      relPath,
								Workspace: wsID,
								Size:      info.Size(),
								ModTime:   info.ModTime().In(cst).Format("2006-01-02 15:04:05"),
								URL:       fmt.Sprintf("/v1/documents/%s/%s", wsID, relPath),
								Category:  category,
							})
							return nil
						})
					}
				}

				// 3. Fetch conversation titles for grouping
				var conversations []ConvSummary
				if len(convIDs) > 0 {
					ids := make([]string, 0, len(convIDs))
					for id := range convIDs {
						ids = append(ids, id)
					}
					var convs []model.Conversation
					db.Where("id IN ?", ids).Find(&convs)
					convMap := map[string]string{}
					for _, cv := range convs {
						convMap[cv.ID] = cv.Title
						conversations = append(conversations, ConvSummary{ID: cv.ID, Title: cv.Title})
					}
					// Backfill conv_title into docs
					for i := range docs {
						if docs[i].ConversationID != "" {
							docs[i].ConvTitle = convMap[docs[i].ConversationID]
						}
					}
				}

				c.JSON(200, gin.H{
					"documents":     docs,
					"conversations": conversations,
				})
			})
			protected.GET("/documents/:workspace/*filepath", func(c *gin.Context) {
				wsID := c.Param("workspace")
				filePath := strings.TrimPrefix(c.Param("filepath"), "/")
				baseDir := "/app/workspaces"
				absPath := filepath.Join(baseDir, wsID, filePath)
				// Security: ensure path stays within workspace
				if !strings.HasPrefix(absPath, filepath.Join(baseDir, wsID)) {
					c.JSON(403, gin.H{"error": "forbidden"})
					return
				}
				if _, err := os.Stat(absPath); os.IsNotExist(err) {
					c.JSON(404, gin.H{"error": "file not found"})
					return
				}
				c.File(absPath)
			})
			protected.DELETE("/documents/:workspace/*filepath", func(c *gin.Context) {
				wsID := c.Param("workspace")
				filePath := strings.TrimPrefix(c.Param("filepath"), "/")
				baseDir := "/app/workspaces"
				absPath := filepath.Join(baseDir, wsID, filePath)
				if !strings.HasPrefix(absPath, filepath.Join(baseDir, wsID)) {
					c.JSON(403, gin.H{"error": "forbidden"})
					return
				}
				if err := os.Remove(absPath); err != nil {
					c.JSON(404, gin.H{"error": "file not found"})
					return
				}
				c.JSON(200, gin.H{"message": "deleted"})
			})

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
				adminHandler := v1.NewAdminHandler(db)
				admin.GET("/admin/users", adminHandler.ListUsers)
				admin.PUT("/admin/users/:id/role", adminHandler.UpdateUserRole)
				admin.DELETE("/admin/users/:id", adminHandler.DeleteUser)
				admin.GET("/admin/stats", adminHandler.SystemStats)
			}
		}
	}

	return r
}
