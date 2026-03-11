package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/yinhe/starclaw-queen/swarm/internal/handler"
	"github.com/yinhe/starclaw-queen/swarm/internal/model"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func main() {
	// Database
	dsn := getEnv("SWARM_DSN", "root:starclaw@tcp(127.0.0.1:3306)/starclaw_queen?charset=utf8mb4&parseTime=True&loc=Local")
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect database: %v", err)
	}
	db.AutoMigrate(&model.Node{}, &model.MoltRelease{}, &model.MoltNodeStatus{})

	// Start offline detector (mark nodes offline if heartbeat missed)
	go offlineDetector(db)

	// Router
	mode := getEnv("GIN_MODE", "debug")
	if mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.Default()
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	h := handler.NewSwarmHandler(db)

	swarm := r.Group("/swarm")
	{
		// Node registration & heartbeat (called by Claw/Overlord)
		swarm.POST("/register", h.Register)
		swarm.POST("/heartbeat", h.Heartbeat)
		swarm.GET("/config", h.GetConfig)

		// Management (called by Queen core)
		swarm.GET("/nodes", h.ListNodes)
		swarm.GET("/nodes/:id", h.GetNode)
		swarm.DELETE("/nodes/:id", h.RemoveNode)
		swarm.POST("/update/notify", h.NotifyUpdate)
		swarm.GET("/stats", h.Stats)
		swarm.GET("/resolve", h.Resolve)

		// Molt — version update management
		molt := handler.NewMoltHandler(db)
		swarm.POST("/molt/releases", molt.CreateRelease)
		swarm.GET("/molt/releases", molt.ListReleases)
		swarm.GET("/molt/releases/:id", molt.GetRelease)
		swarm.POST("/molt/releases/:id/start", molt.StartRelease)
		swarm.POST("/molt/releases/:id/pause", molt.PauseRelease)
		swarm.POST("/molt/report", molt.Report)
		swarm.GET("/molt/check", molt.Check)
	}

	// Prometheus metrics
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "queen-swarm"})
	})

	port := getEnv("SWARM_PORT", "8090")
	log.Printf("[swarm] Queen Swarm service starting on :%s", port)
	if err := r.Run(fmt.Sprintf(":%s", port)); err != nil {
		log.Fatalf("Failed to start swarm service: %v", err)
	}
}

// offlineDetector periodically marks nodes as offline if heartbeat is stale
func offlineDetector(db *gorm.DB) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		threshold := time.Now().Add(-90 * time.Second) // 3x heartbeat interval
		db.Model(&model.Node{}).
			Where("status = ? AND last_heartbeat < ?", model.StatusOnline, threshold).
			Update("status", model.StatusFeral)

		// Mark fully offline after 5 minutes
		offlineThreshold := time.Now().Add(-5 * time.Minute)
		db.Model(&model.Node{}).
			Where("status = ? AND last_heartbeat < ?", model.StatusFeral, offlineThreshold).
			Update("status", model.StatusOffline)
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
