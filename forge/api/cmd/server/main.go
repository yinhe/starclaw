package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"runtime"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"starclaw.net/forge/internal/aggregator"
	"starclaw.net/forge/internal/config"
	"starclaw.net/forge/internal/engine"
	"starclaw.net/forge/internal/handler"
	"starclaw.net/forge/internal/model"
)

var version = "dev"

func main() {
	cfg := config.Load()

	// Database
	db, err := gorm.Open(sqlite.Open(cfg.DBPath), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	db.AutoMigrate(model.AllModels()...)

	// Engines
	prdEngine := engine.NewPRDEngine(db, cfg)
	orchestrator := &engine.Orchestrator{DB: db, Cfg: cfg}

	// Aggregators
	nydusAgg := aggregator.NewNydusClient(cfg.NydusURL, cfg.NydusSecret)
	bridgeAgg := aggregator.NewDevBridgeClient(cfg.DevBridgeURL)
	pheromoneAgg, err := aggregator.NewPheromoneClient(cfg.PheromoneNATSURL)
	if err != nil {
		log.Printf("[WARN] Pheromone subscriber disabled: %v", err)
	} else if pheromoneAgg != nil {
		_, err := pheromoneAgg.SubscribeDeployEvents(func(subject string, payload []byte) {
			handlePheromoneDeployEvent(db, subject, payload)
		})
		if err != nil {
			log.Printf("[WARN] Failed to subscribe Pheromone deploy events: %v", err)
		} else {
			log.Printf("[INFO] Pheromone deploy subscriber connected: %s (%s)", cfg.PheromoneNATSURL, aggregator.DeploySubjectPattern)
		}
	}

	// Handlers
	projectH := &handler.ProjectHandler{DB: db}
	issueH := &handler.IssueHandler{DB: db}
	sprintH := &handler.SprintHandler{DB: db}
	dashboardH := &handler.DashboardHandler{DB: db, Cfg: cfg, Nydus: nydusAgg, Bridge: bridgeAgg}

	// Router
	r := gin.Default()
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		AllowCredentials: true,
	}))

	// Auth middleware
	r.Use(handler.AuthMiddleware(cfg))

	// Health (public)
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"service": "forge",
			"version": version,
		})
	})

	api := r.Group("/api")
	{
		// Auth (public)
		api.POST("/auth/login", handler.LoginHandler(cfg))
		api.GET("/auth/me", handler.MeHandler())

		// Projects
		api.GET("/projects", projectH.List)
		api.POST("/projects", projectH.Create)
		api.GET("/projects/:id", projectH.Get)
		api.PUT("/projects/:id", projectH.Update)
		api.DELETE("/projects/:id", projectH.Delete)

		// Issues
		api.GET("/projects/:id/issues", issueH.List)
		api.POST("/projects/:id/issues", issueH.Create)
		api.GET("/projects/:id/board", issueH.Board)
		api.GET("/issues/:id", issueH.Get)
		api.GET("/issues/key/:key", issueH.GetByKey)
		api.PUT("/issues/:id", issueH.Update)
		api.POST("/issues/:id/transition", issueH.Transition)
		api.POST("/issues/:id/comments", issueH.AddComment)

		// Sprints
		api.GET("/projects/:id/sprints", sprintH.List)
		api.POST("/projects/:id/sprints", sprintH.Create)
		api.PUT("/sprints/:sid", sprintH.Update)
		api.GET("/sprints/:sid/burndown", sprintH.Burndown)

		// Dashboard
		api.GET("/dashboard", dashboardH.Overview)
		api.GET("/dashboard/services", dashboardH.Services)
		api.GET("/dashboard/activity", dashboardH.Activity)
		api.GET("/dashboard/branches", dashboardH.Branches)
		api.GET("/dashboard/devclaws", dashboardH.DevClaws)
		api.GET("/dashboard/stats", dashboardH.Stats)
		api.GET("/dashboard/heatmap", dashboardH.Heatmap)
		api.GET("/dashboard/deploys", dashboardH.Deploys)
		api.GET("/dashboard/commits", dashboardH.Commits)

		// PRD (non-streaming, kept for backwards compat)
		api.POST("/prd/generate", func(c *gin.Context) {
			var req struct {
				ProjectID string `json:"project_id" binding:"required"`
				Prompt    string `json:"prompt" binding:"required"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			prd, err := prdEngine.GeneratePRD(req.ProjectID, req.Prompt)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusCreated, gin.H{"prd": prd})
		})

		// PRD generate — streaming (SSE)
		api.POST("/prd/generate/stream", func(c *gin.Context) {
			var req struct {
				ProjectID string `json:"project_id" binding:"required"`
				Prompt    string `json:"prompt" binding:"required"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}

			c.Writer.Header().Set("Content-Type", "text/event-stream")
			c.Writer.Header().Set("Cache-Control", "no-cache")
			c.Writer.Header().Set("Connection", "keep-alive")
			c.Writer.Flush()

			sendSSE := func(chunk engine.StreamChunk) {
				data, _ := json.Marshal(chunk)
				fmt.Fprintf(c.Writer, "data: %s\n\n", data)
				c.Writer.Flush()
			}

			rawContent, err := prdEngine.CallLLMStream(
				prdEngine.GetGenerateSystemPrompt(), req.Prompt, sendSSE,
			)
			if err != nil {
				sendSSE(engine.StreamChunk{Type: "error", Text: err.Error()})
				return
			}

			prd, _ := prdEngine.SaveGeneratedPRD(req.ProjectID, req.Prompt, rawContent)
			prdJSON, _ := json.Marshal(prd)
			sendSSE(engine.StreamChunk{Type: "result", Text: string(prdJSON)})
		})

		// PRD plan — streaming (SSE)
		api.POST("/prd/:id/plan/stream", func(c *gin.Context) {
			var prd model.ForgePRD
			if err := db.First(&prd, "id = ?", c.Param("id")).Error; err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "PRD not found"})
				return
			}

			c.Writer.Header().Set("Content-Type", "text/event-stream")
			c.Writer.Header().Set("Cache-Control", "no-cache")
			c.Writer.Header().Set("Connection", "keep-alive")
			c.Writer.Flush()

			sendSSE := func(chunk engine.StreamChunk) {
				data, _ := json.Marshal(chunk)
				fmt.Fprintf(c.Writer, "data: %s\n\n", data)
				c.Writer.Flush()
			}

			rawContent, err := prdEngine.CallLLMStream(
				prdEngine.GetPlanSystemPrompt(),
				prdEngine.GetPlanUserPrompt(&prd),
				sendSSE,
			)
			if err != nil {
				sendSSE(engine.StreamChunk{Type: "error", Text: err.Error()})
				return
			}

			// Parse and save sprints + issues
			sprints, issues, err := prdEngine.SavePlanResult(&prd, rawContent)
			if err != nil {
				sendSSE(engine.StreamChunk{Type: "error", Text: err.Error()})
				return
			}
			resultJSON, _ := json.Marshal(gin.H{
				"sprints":       sprints,
				"issues":        issues,
				"total_sprints": len(sprints),
				"total_issues":  len(issues),
			})
			sendSSE(engine.StreamChunk{Type: "result", Text: string(resultJSON)})
		})

		// PRD import — 从对话/外部直接导入完整 PRD (不调 LLM)
		api.POST("/prd/import", func(c *gin.Context) {
			var req struct {
				ProjectID          string        `json:"project_id" binding:"required"`
				Title              string        `json:"title" binding:"required"`
				Prompt             string        `json:"prompt"`
				Objective          string        `json:"objective"`
				Features           []interface{} `json:"features"`
				NonFunctional      []string      `json:"non_functional"`
				AcceptanceCriteria []string      `json:"acceptance_criteria"`
				Services           []string      `json:"services"`
				EstimatedSprints   int           `json:"estimated_sprints"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			// Verify project exists
			var project model.ForgeProject
			if err := db.First(&project, "id = ?", req.ProjectID).Error; err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
				return
			}
			prd := model.ForgePRD{
				ProjectID:        req.ProjectID,
				Title:            req.Title,
				Prompt:           req.Prompt,
				Objective:        req.Objective,
				EstimatedSprints: req.EstimatedSprints,
				Status:           "confirmed", // 对话导入的 PRD 直接 confirmed
			}
			if b, err := json.Marshal(req.Features); err == nil {
				prd.Features = string(b)
			}
			if b, err := json.Marshal(req.NonFunctional); err == nil {
				prd.NonFunctional = string(b)
			}
			if b, err := json.Marshal(req.AcceptanceCriteria); err == nil {
				prd.AcceptanceCriteria = string(b)
			}
			if b, err := json.Marshal(req.Services); err == nil {
				prd.Services = string(b)
			}
			if req.EstimatedSprints == 0 {
				prd.EstimatedSprints = 1
			}
			db.Create(&prd)
			c.JSON(http.StatusCreated, gin.H{"prd": prd})
		})

		api.GET("/prd/:id", func(c *gin.Context) {
			var prd model.ForgePRD
			if err := db.First(&prd, "id = ?", c.Param("id")).Error; err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "PRD not found"})
				return
			}
			c.JSON(http.StatusOK, gin.H{"prd": prd})
		})

		api.POST("/prd/:id/confirm", func(c *gin.Context) {
			db.Model(&model.ForgePRD{}).Where("id = ?", c.Param("id")).Update("status", "confirmed")
			c.JSON(http.StatusOK, gin.H{"status": "confirmed"})
		})

		api.POST("/prd/:id/plan", func(c *gin.Context) {
			var prd model.ForgePRD
			if err := db.First(&prd, "id = ?", c.Param("id")).Error; err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "PRD not found"})
				return
			}
			sprints, issues, err := prdEngine.PlanSprints(&prd)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusCreated, gin.H{
				"sprints":       sprints,
				"issues":        issues,
				"total_sprints": len(sprints),
				"total_issues":  len(issues),
			})
		})

		// Orchestrator
		api.POST("/sprints/:sid/start", func(c *gin.Context) {
			if err := orchestrator.StartSprint(c.Param("sid")); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"status": "started"})
		})

		api.GET("/orchestrator/status", func(c *gin.Context) {
			c.JSON(http.StatusOK, orchestrator.Status())
		})

		// Agent registration
		api.POST("/orchestrator/register", func(c *gin.Context) {
			var req struct {
				Name         string `json:"name" binding:"required"`
				Type         string `json:"type"`
				Capabilities string `json:"capabilities"`
				Services     string `json:"services"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			agent := model.ForgeAgent{
				Name:         req.Name,
				Type:         req.Type,
				Capabilities: req.Capabilities,
				Services:     req.Services,
				Status:       "idle",
				LastSeenAt:   time.Now(),
				RegisteredAt: time.Now(),
			}
			// Upsert
			var existing model.ForgeAgent
			if err := db.First(&existing, "name = ?", req.Name).Error; err == nil {
				db.Model(&existing).Updates(map[string]interface{}{
					"type":         req.Type,
					"capabilities": req.Capabilities,
					"services":     req.Services,
					"status":       "idle",
					"last_seen_at": time.Now(),
				})
				c.JSON(http.StatusOK, gin.H{"agent": existing, "action": "updated"})
				return
			}
			db.Create(&agent)
			c.JSON(http.StatusCreated, gin.H{"agent": agent, "action": "registered"})
		})

		api.GET("/orchestrator/agents", func(c *gin.Context) {
			var agents []model.ForgeAgent
			db.Order("registered_at DESC").Find(&agents)
			c.JSON(http.StatusOK, gin.H{"agents": agents})
		})

		// Webhook receivers
		api.POST("/webhooks/nydus", func(c *gin.Context) {
			var payload map[string]interface{}
			c.ShouldBindJSON(&payload)
			event, _ := payload["event"].(string)
			db.Create(&model.ForgeActivity{
				Type:    event,
				Actor:   "nydus",
				Summary: fmt.Sprintf("Nydus event: %s", event),
				Detail:  toJSON(payload),
				Source:  "nydus",
			})
			// If PR merged, check if linked issue should be closed
			if event == "pr.merged" {
				// TODO: resolve linked issues → done → orchestrator.OnIssueComplete
			}
			c.JSON(http.StatusOK, gin.H{"status": "received"})
		})

		api.POST("/webhooks/github", func(c *gin.Context) {
			var payload map[string]interface{}
			c.ShouldBindJSON(&payload)
			action, _ := payload["action"].(string)
			db.Create(&model.ForgeActivity{
				Type:    "ci",
				Actor:   "github",
				Summary: fmt.Sprintf("GitHub event: %s", action),
				Detail:  toJSON(payload),
				Source:  "github",
			})
			c.JSON(http.StatusOK, gin.H{"status": "received"})
		})

		// Issue completion callback (called by Dev Bridge or manually)
		api.POST("/issues/:id/complete", func(c *gin.Context) {
			issueID := c.Param("id")
			now := time.Now()
			db.Model(&model.ForgeIssue{}).Where("id = ?", issueID).Updates(map[string]interface{}{
				"status":    "done",
				"closed_at": &now,
			})
			orchestrator.OnIssueComplete(issueID)
			c.JSON(http.StatusOK, gin.H{"status": "completed"})
		})
	}

	// Banner
	whitelistCount := len(cfg.Whitelist)
	authStatus := "OFF (dev mode)"
	if whitelistCount > 0 {
		authStatus = fmt.Sprintf("ON (%d nodes)", whitelistCount)
	}

	fmt.Printf(`
  ╔══════════════════════════════════════════════════╗
  ║   🔥 StarClaw Forge v%-10s                 ║
  ║   研发管控 + 可视化大屏                          ║
  ╠══════════════════════════════════════════════════╣
  ║   Port:       :%-5s                            ║
  ║   DB:         %-36s  ║
  ║   Auth:       %-36s  ║
  ║   OS:         %-8s  Arch: %-8s            ║
  ╠══════════════════════════════════════════════════╣
  ║   Login:      POST /api/auth/login               ║
  ║   Dashboard:  /api/dashboard                     ║
  ║   Projects:   /api/projects                      ║
  ║   PRD:        /api/prd/generate                  ║
  ║   Sprint:     /api/sprints/:id/start             ║
  ╚══════════════════════════════════════════════════╝
`, version, cfg.Port, cfg.DBPath, authStatus, runtime.GOOS, runtime.GOARCH)

	log.Fatal(r.Run(":" + cfg.Port))
}

func toJSON(v interface{}) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	if len(b) > 5000 {
		return string(b[:5000]) + "...[truncated]"
	}
	return string(b)
}

func handlePheromoneDeployEvent(db *gorm.DB, subject string, payload []byte) {
	if db == nil {
		return
	}

	detail := string(payload)
	eventName := subject
	repo := ""
	actor := "pheromone"

	var p map[string]interface{}
	if err := json.Unmarshal(payload, &p); err == nil {
		if v, ok := p["event"].(string); ok && v != "" {
			eventName = v
		}
		if v, ok := p["repo"].(string); ok {
			repo = v
		}
		if v, ok := p["service"].(string); ok && v != "" {
			repo = v
		}
		if v, ok := p["actor"].(string); ok && v != "" {
			actor = v
		}
	}

	summary := fmt.Sprintf("Pheromone event: %s", eventName)
	if repo != "" {
		summary = fmt.Sprintf("Pheromone deploy: %s (%s)", repo, eventName)
	}

	if err := db.Create(&model.ForgeActivity{
		Type:    "deploy",
		Actor:   actor,
		Summary: summary,
		Detail:  detail,
		Service: repo,
		Source:  "pheromone",
	}).Error; err != nil {
		log.Printf("[WARN] Failed to persist Pheromone event (%s): %v", subject, err)
	}
}
