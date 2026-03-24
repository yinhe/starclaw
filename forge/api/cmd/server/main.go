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
	prdEngine := &engine.PRDEngine{DB: db, Cfg: cfg}
	orchestrator := &engine.Orchestrator{DB: db, Cfg: cfg}

	// Handlers
	projectH := &handler.ProjectHandler{DB: db}
	issueH := &handler.IssueHandler{DB: db}
	sprintH := &handler.SprintHandler{DB: db}
	dashboardH := &handler.DashboardHandler{DB: db, Cfg: cfg}

	// Router
	r := gin.Default()
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		AllowCredentials: true,
	}))

	// Health
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"service": "forge",
			"version": version,
		})
	})

	api := r.Group("/api")
	{
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

		// PRD
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
	fmt.Printf(`
  ╔══════════════════════════════════════════════════╗
  ║   🔥 StarClaw Forge v%-10s                 ║
  ║   研发管控 + 可视化大屏                          ║
  ╠══════════════════════════════════════════════════╣
  ║   Port:       :%-5s                            ║
  ║   DB:         %-36s  ║
  ║   OS:         %-8s  Arch: %-8s            ║
  ╠══════════════════════════════════════════════════╣
  ║   Dashboard:  /api/dashboard                     ║
  ║   Projects:   /api/projects                      ║
  ║   Issues:     /api/projects/:id/issues           ║
  ║   Board:      /api/projects/:id/board            ║
  ║   PRD:        /api/prd/generate                  ║
  ║   Sprint:     /api/sprints/:id/start             ║
  ╚══════════════════════════════════════════════════╝
`, version, cfg.Port, cfg.DBPath, runtime.GOOS, runtime.GOARCH)

	log.Fatal(r.Run(":" + cfg.Port))
}

func toJSON(v interface{}) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	if len(b) > 5000 {
		return string(b[:5000]) + "...[truncated]"
	}
	return string(b)
}
