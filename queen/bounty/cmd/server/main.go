package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw-queen/bounty/internal/handler"
	"github.com/yinhe/starclaw-queen/bounty/internal/model"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func main() {
	dsn := getEnv("BOUNTY_DSN", "root:starclaw@tcp(127.0.0.1:3306)/starclaw_queen?charset=utf8mb4&parseTime=True&loc=Local")
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect database: %v", err)
	}
	db.AutoMigrate(&model.Bounty{}, &model.BountyUser{})

	// Expire stale bounties
	go expireLoop(db)

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

	h := handler.NewBountyHandler(db)

	bounties := r.Group("/bounties")
	{
		bounties.POST("", h.Create)
		bounties.GET("", h.List)
		bounties.GET("/stats", h.Stats)
		bounties.GET("/categories", h.Categories)
		bounties.GET("/:id", h.Get)
		bounties.POST("/:id/claim", h.Claim)
		bounties.POST("/:id/deliver", h.Deliver)
		bounties.POST("/:id/accept", h.Accept)
		bounties.POST("/:id/cancel", h.Cancel)
		bounties.POST("/:id/dispute", h.Dispute)
	}

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "queen-bounty"})
	})

	port := getEnv("BOUNTY_PORT", "8092")
	log.Printf("[bounty] Queen Bounty service starting on :%s", port)
	if err := r.Run(fmt.Sprintf(":%s", port)); err != nil {
		log.Fatalf("Failed to start bounty service: %v", err)
	}
}

// expireLoop marks open bounties past their deadline as expired
func expireLoop(db *gorm.DB) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		db.Model(&model.Bounty{}).
			Where("status = ? AND deadline IS NOT NULL AND deadline < ?", model.BountyOpen, time.Now()).
			Update("status", model.BountyExpired)
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
