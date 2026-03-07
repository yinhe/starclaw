package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw-queen/core/internal/handler"
)

func main() {
	swarmURL := getEnv("SWARM_URL", "http://localhost:8090")

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

	dash := handler.NewDashboardHandler(swarmURL)

	// Queen Core management API
	api := r.Group("/api/queen")
	{
		// Dashboard
		api.GET("/stats", dash.GlobalStats)

		// Node management
		api.GET("/nodes", dash.ListNodes)
		api.GET("/nodes/:id", dash.GetNode)
		api.DELETE("/nodes/:id", dash.RemoveNode)

		// Update management (Molt)
		api.POST("/update/notify", dash.NotifyUpdate)
	}

	// Health
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "queen-core"})
	})

	port := getEnv("CORE_PORT", "8091")
	log.Printf("[queen-core] Management API starting on :%s (swarm: %s)", port, swarmURL)
	if err := r.Run(fmt.Sprintf(":%s", port)); err != nil {
		log.Fatalf("Failed to start queen-core: %v", err)
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
