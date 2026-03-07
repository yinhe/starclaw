package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw-overlord/manager/internal/handler"
	"github.com/yinhe/starclaw-overlord/manager/internal/model"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func main() {
	dsn := getEnv("OVERLORD_DSN", "root:starclaw@tcp(127.0.0.1:3306)/starclaw_overlord?charset=utf8mb4&parseTime=True&loc=Local")
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect database: %v", err)
	}
	db.AutoMigrate(&model.ClawNode{}, &model.TaskAssignment{}, &model.AuditLog{})

	// Offline detector
	go offlineDetector(db)

	mode := getEnv("GIN_MODE", "debug")
	if mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.Default()
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "X-Admin-User"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	h := handler.NewRegistryHandler(db)

	brood := r.Group("/brood")
	{
		// Claw registration & heartbeat
		brood.POST("/register", h.Register)
		brood.POST("/heartbeat", h.Heartbeat)

		// Management
		brood.GET("/claws", h.ListClaws)
		brood.GET("/claws/:id", h.GetClaw)
		brood.PUT("/claws/:id/quota", h.UpdateQuota)
		brood.DELETE("/claws/:id", h.RemoveClaw)

		// Scheduler
		brood.POST("/task/assign", h.AssignTask)

		// Stats & audit
		brood.GET("/stats", h.Stats)
		brood.GET("/audit", h.AuditLogs)
	}

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "overlord-manager"})
	})

	port := getEnv("OVERLORD_PORT", "8095")
	log.Printf("[overlord] Manager service starting on :%s", port)
	if err := r.Run(fmt.Sprintf(":%s", port)); err != nil {
		log.Fatalf("Failed to start overlord manager: %v", err)
	}
}

func offlineDetector(db *gorm.DB) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		threshold := time.Now().Add(-90 * time.Second)
		db.Model(&model.ClawNode{}).
			Where("status = ? AND last_heartbeat < ?", "online", threshold).
			Update("status", "feral")

		offlineThreshold := time.Now().Add(-5 * time.Minute)
		db.Model(&model.ClawNode{}).
			Where("status = ? AND last_heartbeat < ?", "feral", offlineThreshold).
			Update("status", "offline")
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
