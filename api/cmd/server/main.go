package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
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
	"github.com/yinhe/starclaw/internal/model"
	"github.com/yinhe/starclaw/internal/molt"
	"github.com/yinhe/starclaw/internal/node"
	"github.com/yinhe/starclaw/internal/router"
	"github.com/yinhe/starclaw/internal/swarm"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	// Handle CLI subcommands before starting the full server
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "get-token":
			cmdGetToken()
			return
		case "reset-token":
			cmdResetToken()
			return
		case "reset-password":
			cmdResetPassword()
			return
		case "version":
			fmt.Printf("StarClaw v%s\n", molt.Version)
			return
		case "help", "--help", "-h":
			printUsage()
			return
		}
	}

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

	// Load crypto identity for claw: address
	identity := node.LoadOrCreateIdentity()
	log.Printf("Node ID: %s", identity.NodeID)

	// Start Swarm client (register with Queen + heartbeat)
	swarmClient := swarm.NewClient(cfg.Swarm)
	swarmClient.SetClawID(identity.NodeID)
	if cfg.Node.Address != "" {
		swarmClient.SetAddress(cfg.Node.Address)
	}
	swarmClient.Start()
	defer swarmClient.Stop()

	// Initialize Queen billing client (for hosted mode centralized billing)
	billingClient := swarm.NewBillingClient(cfg.Swarm.QueenURL, cfg.JWT.Secret)
	v1.SetQueenBilling(billingClient)

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

func printUsage() {
	fmt.Println("Usage: starclaw [command]")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  (none)           Start the API server")
	fmt.Println("  get-token        Show current Owner Token (read-only)")
	fmt.Println("  reset-token      Generate a new Owner Token (prints to stdout)")
	fmt.Println("  reset-password   Reset the Owner password (reads from --password flag)")
	fmt.Println("  version          Print version and exit")
	fmt.Println("  help             Show this help")
	fmt.Println("")
	fmt.Println("Examples:")
	fmt.Println("  starclaw get-token")
	fmt.Println("  starclaw reset-token")
	fmt.Println("  starclaw reset-password --password newpass123")
}

func openCLIDB() (*config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	return cfg, nil
}

// cmdGetToken prints the current owner token without modifying it.
func cmdGetToken() {
	cfg, err := openCLIDB()
	if err != nil {
		log.Fatal(err)
	}
	db, err := database.InitMySQL(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	var user model.User
	if err := db.Where("owner_token IS NOT NULL").First(&user).Error; err != nil {
		log.Fatalf("No owner user found. Run initial setup first.")
	}

	fmt.Println("========================================")
	fmt.Printf("Owner: %s (id: %s)\n", user.Username, user.ID)
	fmt.Printf("Owner Token: %s\n", *user.OwnerToken)
	fmt.Println("========================================")
}

// cmdResetToken regenerates the owner token and prints it.
func cmdResetToken() {
	cfg, err := openCLIDB()
	if err != nil {
		log.Fatal(err)
	}
	db, err := database.InitMySQL(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	var user model.User
	if err := db.Where("owner_token IS NOT NULL").First(&user).Error; err != nil {
		log.Fatalf("No owner user found. Run initial setup first.")
	}

	tokenBytes := make([]byte, 16)
	if _, err := rand.Read(tokenBytes); err != nil {
		log.Fatalf("Failed to generate token: %v", err)
	}
	newToken := hex.EncodeToString(tokenBytes)

	if err := db.Model(&user).Update("owner_token", newToken).Error; err != nil {
		log.Fatalf("Failed to update token: %v", err)
	}

	fmt.Println("========================================")
	fmt.Printf("Owner: %s (id: %s)\n", user.Username, user.ID)
	fmt.Printf("New Owner Token: %s\n", newToken)
	fmt.Println("========================================")
	fmt.Println("Use this token to log in via the Auth Token tab.")
}

// cmdResetPassword resets the owner password.
func cmdResetPassword() {
	password := ""
	for i, arg := range os.Args {
		if arg == "--password" && i+1 < len(os.Args) {
			password = os.Args[i+1]
		}
	}
	if password == "" {
		fmt.Println("Usage: starclaw reset-password --password <new-password>")
		os.Exit(1)
	}
	if len(password) < 6 {
		fmt.Println("Error: password must be at least 6 characters")
		os.Exit(1)
	}

	cfg, err := openCLIDB()
	if err != nil {
		log.Fatal(err)
	}
	db, err := database.InitMySQL(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	var user model.User
	if err := db.Where("owner_token IS NOT NULL").First(&user).Error; err != nil {
		log.Fatalf("No owner user found. Run initial setup first.")
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("Failed to hash password: %v", err)
	}

	if err := db.Model(&user).Update("password", string(hashed)).Error; err != nil {
		log.Fatalf("Failed to update password: %v", err)
	}

	fmt.Println("========================================")
	fmt.Printf("Owner: %s (id: %s)\n", user.Username, user.ID)
	fmt.Println("Password has been reset successfully.")
	fmt.Println("========================================")
}
