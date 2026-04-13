package router

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"time"

	agentpkg "github.com/yinhe/starclaw/internal/agent"
	v1 "github.com/yinhe/starclaw/internal/api/v1"
	"github.com/yinhe/starclaw/internal/api/v1/infra"
	"github.com/yinhe/starclaw/internal/billing"
	"github.com/yinhe/starclaw/internal/browser"
	"github.com/yinhe/starclaw/internal/config"
	"github.com/yinhe/starclaw/internal/instinct"
	"github.com/yinhe/starclaw/internal/mcp"
	"github.com/yinhe/starclaw/internal/model"
	"github.com/yinhe/starclaw/internal/node"
	"github.com/yinhe/starclaw/internal/provider"
	"github.com/yinhe/starclaw/internal/rag"
	"github.com/yinhe/starclaw/internal/sandbox"
	"github.com/yinhe/starclaw/internal/tool"
	"github.com/yinhe/starclaw/internal/trading"
	"github.com/yinhe/starclaw/internal/worker"
	"gorm.io/gorm"
)

// Deps holds all shared dependencies created during engine initialization.
// These are passed to route registration functions.
type Deps struct {
	Identity         *node.Identity
	ProviderRegistry *provider.Registry
	ToolRegistry     *tool.Registry
	SandboxMgr       *sandbox.Manager
	ProcessMgr       *sandbox.ProcessManager
	TaskWorker       *worker.TaskWorker
	InstinctEngine   *instinct.Engine
	Embedder         rag.EmbeddingProvider
	Pipeline         *rag.Pipeline
	QueenClient      *billing.QueenClient
}

// initDeps initializes all engines, registries, and services.
// This is called once during Setup and the returned Deps is used for route registration.
func initDeps(cfg *config.Config, db *gorm.DB) *Deps {
	// Node identity (created early so star-ai provider can use it)
	identity := node.LoadOrCreateIdentity()

	// Provider registry
	providerRegistry := provider.NewRegistry()

	// Register star-ai provider with node identity (Ed25519 signature auth, no API key needed)
	providerRegistry.Register("star-ai", provider.NewStarAIProvider(provider.StarAIConfig{
		Identity: identity,
	}))

	// Initialize StarAI proxy for tools (video/image/music route through StarAI Router)
	starAIBaseURL := "https://api.star-ai.net/v1"
	if envURL := os.Getenv("STAR_AI_BASE_URL"); envURL != "" {
		starAIBaseURL = envURL
	}
	tool.InitStarAIProxy(identity, starAIBaseURL, db)

	// Tool registry
	browserMgr := browser.NewManager()
	// Initialize data directory from config (must be before tool creation)
	tool.SetDataDir(cfg.Storage.DataDir)
	sandbox.SetDataDir(cfg.Storage.DataDir)

	sandboxMgr := sandbox.NewManager()
	processMgr := sandbox.NewProcessManager()
	toolRegistry := tool.NewRegistry()
	toolRegistry.Register(tool.NewWebSearchTool(tool.WebSearchConfig{}))
	toolRegistry.Register(tool.NewHTTPRequestTool())
	toolRegistry.Register(tool.NewBrowserTool(browserMgr))
	toolRegistry.Register(tool.NewCodeTool(sandboxMgr, processMgr, db))
	toolRegistry.Register(tool.NewGitTool())
	videoTool := tool.NewVideoTool(db)
	toolRegistry.Register(videoTool)
	toolRegistry.Register(tool.NewDubbingTool(db))
	toolRegistry.Register(tool.NewSubtitleTool(db))
	toolRegistry.Register(tool.NewMVTool(db))
	toolRegistry.Register(tool.NewComicTool(db))
	toolRegistry.Register(tool.NewMusicTool(db))
	toolRegistry.Register(tool.NewAudioTool(db))
	toolRegistry.Register(tool.NewImageTool(db))
	toolRegistry.Register(tool.NewDocumentTool(db))
	toolRegistry.Register(tool.NewBountyTool(cfg.Swarm))
	toolRegistry.Register(tool.NewArenaTool(cfg.Swarm))
	toolRegistry.Register(tool.NewDeployWebTool())
	toolRegistry.Register(tool.NewBindDomainTool())
	toolRegistry.Register(tool.NewVerifyOnlineTool())
	toolRegistry.Register(tool.NewFeishuTool(db))
	toolRegistry.Register(tool.NewDingtalkTool(db))
	toolRegistry.Register(tool.NewWeComTool(db))
	toolRegistry.Register(tool.NewSlackTool(db))
	toolRegistry.Register(tool.NewDiscordTool(db))
	toolRegistry.Register(tool.NewTelegramTool(db))
	toolRegistry.Register(tool.NewWeChatCSTool(db, cfg.JWT.Secret, cfg.Server.Port))
	toolRegistry.Register(tool.NewDesktopTool())

	// Generate thumbnails for existing videos on startup
	go videoTool.GenerateMissingThumbnails()

	// Build delegate function for agent-to-agent delegation (breaks circular import)
	delegateFunc := func(ctx context.Context, ag model.Agent, modelCfg model.ModelConfig, message string) (*tool.DelegateResult, error) {
		// Timeout: prevent delegation from hanging forever on slow/unreachable providers
		// 5 min for complex delegations (video editing, multi-tool workflows via star-ai.net)
		delegateCtx, cancel := context.WithTimeout(ctx, 300*time.Second)
		defer cancel()

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
		start := time.Now()
		result, err := rt.Run(delegateCtx, runReq)
		log.Printf("[delegate] agent=%s model=%s provider=%s duration=%v err=%v",
			ag.Name, modelCfg.ModelName, modelCfg.Provider, time.Since(start), err)
		if err != nil {
			return &tool.DelegateResult{Error: err.Error()}, nil
		}
		return &tool.DelegateResult{Content: result.Content}, nil
	}
	toolRegistry.Register(tool.NewSystemTool(db, providerRegistry, delegateFunc))

	// Load JSON tool plugins from plugins/ directory (includes trading_*.json when present)
	_ = tool.LoadPluginsFromDir(toolRegistry, "plugins")

	// Trading plugin (Extractor quantitative trading)
	if cfg.Trading.Enabled {
		tradingCfg := trading.Config{
			Enabled:   cfg.Trading.Enabled,
			Role:      cfg.Trading.Role,
			BridgeURL: cfg.Trading.BridgeURL,
			Mode:      cfg.Trading.Mode,
			Master: trading.MasterConfig{
				HeartbeatURL:      cfg.Trading.Master.HeartbeatURL,
				HeartbeatInterval: cfg.Trading.Master.HeartbeatInterval,
				HeartbeatTimeout:  cfg.Trading.Master.HeartbeatTimeout,
				AutoAutonomous:    cfg.Trading.Master.AutoAutonomous,
			},
			Auto: trading.AutoPolicy{
				AllowNewPositions: cfg.Trading.Auto.AllowNewPositions,
				MaxPositionPct:    cfg.Trading.Auto.MaxPositionPct,
				StopLossPct:       cfg.Trading.Auto.StopLossPct,
				MinConfidence:     cfg.Trading.Auto.MinConfidence,
				ScanInterval:      cfg.Trading.Auto.ScanInterval,
			},
		}
		tradingPlugin := trading.NewPlugin(tradingCfg)
		tradingPlugin.Start()
		log.Printf("[router] trading plugin started: role=%s mode=%s", tradingCfg.Role, tradingCfg.Mode)
	}

	// Auto-detect and register MCP Bridge (host control) + Dev Bridge (development tools)
	mcp.AutoRegisterBridge(toolRegistry)
	mcp.AutoRegisterDevBridge(toolRegistry)

	// Reload user-saved MCP servers from DB (survives restarts)
	infra.ReloadSavedServers(db, toolRegistry)

	// Billing Gateway: wraps all tool execution with cost tracking + revenue split
	// Prefer dedicated swarm.node_token for Queen internal API; fall back to jwt.secret
	queenNodeToken := cfg.Swarm.NodeToken
	if queenNodeToken == "" {
		queenNodeToken = cfg.JWT.Secret
	}
	queenClient := billing.NewQueenClient(cfg.Swarm.QueenURL, queenNodeToken, identity.NodeID)
	billingGW := billing.NewGateway(db, queenClient, identity.NodeID)
	if billingGW.IsEnabled() {
		toolRegistry.SetExecuteHook(billingGW.ExecuteHook)
		v1.SetBillingQueenClient(queenClient)
		log.Printf("[router] Billing gateway enabled for node %s", identity.NodeID)
	}

	// Auto-migrate task & notification tables
	db.AutoMigrate(&model.Task{}, &model.Notification{}, &model.MusicRecord{}, &model.ImageRecord{}, &model.AgentTemplate{}, &model.Peer{}, &model.Memory{}, &model.Activity{}, &model.ActivityLog{}, &model.Squad{}, &model.SquadMember{}, &model.Mission{}, &model.MissionStep{}, &model.Sprint{}, &model.StepReview{}, &model.WeChatWatch{}, &billing.ToolUsageRecord{}, &model.NodeGrowth{}, &model.Milestone{}, &model.MarketplacePurchase{})

	// Drop FK constraint on agents.model_id so agents can be created without a model
	db.Exec("ALTER TABLE agents DROP FOREIGN KEY fk_agents_model")

	// NOTE: Built-in agent templates are now in Queen marketplace (seed_marketplace.go).
	// Local SeedBuiltinTemplates is no longer called.

	// NOTE: star-ai model seeding now only happens on user creation (setup.go).
	// Removed SeedStarAIForAllUsers from startup — it was re-creating configs
	// that users intentionally deleted.

	// Start background task worker (7x24 autonomous execution)
	taskWorker := worker.NewTaskWorker(db, providerRegistry, toolRegistry, 2)
	taskWorker.Start()

	// Start Instinct engine (proactive behavior system)
	instinctEngine := instinct.NewEngine(db)
	instinctEngine.Start()

	// RAG embedding provider (configured via env or config)
	embedder := rag.NewOpenAIEmbedding(rag.OpenAIEmbeddingConfig{
		APIKey: cfg.OpenAI.APIKey,
		Model:  "text-embedding-3-small",
	})
	pipeline := rag.NewPipeline(db, embedder)

	return &Deps{
		Identity:         identity,
		ProviderRegistry: providerRegistry,
		ToolRegistry:     toolRegistry,
		SandboxMgr:       sandboxMgr,
		ProcessMgr:       processMgr,
		TaskWorker:       taskWorker,
		InstinctEngine:   instinctEngine,
		Embedder:         embedder,
		Pipeline:         pipeline,
		QueenClient:      queenClient,
	}
}
