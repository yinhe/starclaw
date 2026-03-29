package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"starclaw.net/queen/arena/internal/handler"
	"starclaw.net/queen/arena/internal/model"
)

func main() {
	dsn := getEnv("ARENA_DSN", "root:starclaw@tcp(127.0.0.1:3306)/starclaw_queen?charset=utf8mb4&parseTime=True&loc=Local")
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect database: %v", err)
	}
	db.AutoMigrate(
		&model.ArenaAgent{}, &model.ArenaThread{}, &model.ArenaReply{},
	)

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

	h := handler.NewArenaHandler(db)

	arena := r.Group("/arena")
	{
		// Agent registration & leaderboard (read by humans too)
		arena.POST("/agents", h.RegisterAgent)
		arena.GET("/leaderboard", h.Leaderboard)

		// Threads (only agents can post, humans can read)
		arena.POST("/threads", h.CreateThread)
		arena.GET("/threads", h.ListThreads)
		arena.GET("/threads/:id", h.GetThread)
		arena.POST("/threads/:id/replies", h.CreateReply)

		arena.GET("/stats", h.Stats)
	}

	r.GET("/metrics", gin.WrapH(promhttp.Handler()))
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "queen-arena"})
	})
	r.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"service":   "queen-arena",
			"version":   "1.1.0",
			"desc":      "Robot Forum (PK moved to Chrysalis)",
			"endpoints": []string{"/arena/agents", "/arena/leaderboard", "/arena/threads", "/health"},
		})
	})

	port := getEnv("ARENA_PORT", "8095")
	log.Printf("[arena] Queen Arena (Robot Forum) starting on :%s", port)
	if err := r.Run(fmt.Sprintf(":%s", port)); err != nil {
		log.Fatalf("Failed to start arena service: %v", err)
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
