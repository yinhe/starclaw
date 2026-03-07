package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	v1 "github.com/yinhe/starclaw/internal/api/v1"
	"github.com/yinhe/starclaw/internal/config"
	"github.com/yinhe/starclaw/internal/database"
	"github.com/yinhe/starclaw/internal/molt"
	"github.com/yinhe/starclaw/internal/router"
	"github.com/yinhe/starclaw/internal/swarm"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize database
	db, err := database.InitMySQL(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to MySQL: %v", err)
	}

	// Auto migrate database schemas
	if err := database.AutoMigrate(db); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}

	// Seed system-level built-in agents (visible to all users)
	v1.SeedBuiltinAgents(db)

	// Initialize Redis
	rdb, err := database.InitRedis(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer rdb.Close()

	// Start Molt version checker
	molt.StartChecker()
	log.Printf("StarClaw v%s starting...", molt.Version)

	// Start Swarm client (register with Queen + heartbeat)
	swarmClient := swarm.NewClient(cfg.Swarm)
	swarmClient.Start()
	defer swarmClient.Stop()

	// Setup router
	r := router.Setup(cfg, db, rdb, swarmClient)

	// Graceful shutdown
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	srv := &http.Server{
		Addr:    addr,
		Handler: r,
	}

	go func() {
		log.Printf("StarClaw server starting on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	// Close DB connection pool
	sqlDB, _ := db.DB()
	if sqlDB != nil {
		sqlDB.Close()
	}

	log.Println("Server exited gracefully")
}
