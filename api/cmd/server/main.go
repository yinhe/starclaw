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
	"strings"
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
		case "devices":
			cmdDevices()
			return
		case "approve":
			cmdApproveDevice()
			return
		case "reject":
			cmdRejectDevice()
			return
		case "export-key":
			cmdExportKey()
			return
		case "import-key":
			cmdImportKey()
			return
		case "wallet-info":
			cmdWalletInfo()
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
	fmt.Println("  devices          List all authorized devices")
	fmt.Println("  approve <id>     Approve a pending device (supports ID prefix)")
	fmt.Println("  reject <id>      Reject/revoke a device (supports ID prefix)")
	fmt.Println("  export-key       Export 24-word mnemonic (BIP-39 backup)")
	fmt.Println("  import-key       Restore identity from mnemonic or seed hex")
	fmt.Println("  wallet-info      Show HD wallet addresses and derivation paths")
	fmt.Println("  version          Print version and exit")
	fmt.Println("  help             Show this help")
	fmt.Println("")
	fmt.Println("Examples:")
	fmt.Println("  starclaw get-token")
	fmt.Println("  starclaw reset-token")
	fmt.Println("  starclaw reset-password --password newpass123")
	fmt.Println("  starclaw devices")
	fmt.Println("  starclaw approve a1b2c3d4")
	fmt.Println("  starclaw export-key")
	fmt.Println("  starclaw import-key <24 words or seed-hex>")
	fmt.Println("  starclaw wallet-info")
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

// cmdDevices lists all authorized devices.
func cmdDevices() {
	cfg, err := openCLIDB()
	if err != nil {
		log.Fatal(err)
	}
	db, err := database.InitMySQL(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	var devices []model.AuthorizedDevice
	db.Order("created_at DESC").Find(&devices)

	if len(devices) == 0 {
		fmt.Println("No authorized devices found.")
		return
	}

	fmt.Printf("%-10s %-20s %-10s %-10s %s\n", "ID", "NAME", "APPROVED", "REVOKED", "LAST USED")
	fmt.Println("----------------------------------------------------------------------")
	for _, d := range devices {
		lastUsed := "never"
		if d.LastUsedAt != nil {
			lastUsed = d.LastUsedAt.Format("2006-01-02 15:04")
		}
		status := "pending"
		if d.Revoked {
			status = "revoked"
		} else if d.Approved {
			status = "approved"
		}
		fmt.Printf("%-10s %-20s %-10s %-10s %s\n", d.ID[:8], truncate(d.DeviceName, 20), status, "", lastUsed)
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

// cmdApproveDevice approves a pending device by ID prefix.
func cmdApproveDevice() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: starclaw approve <device-id-prefix>")
		os.Exit(1)
	}
	prefix := os.Args[2]

	cfg, err := openCLIDB()
	if err != nil {
		log.Fatal(err)
	}
	db, err := database.InitMySQL(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	var devices []model.AuthorizedDevice
	db.Where("id LIKE ?", prefix+"%").Find(&devices)

	if len(devices) == 0 {
		log.Fatalf("No device found matching prefix: %s", prefix)
	}
	if len(devices) > 1 {
		fmt.Printf("Multiple devices match prefix '%s':\n", prefix)
		for _, d := range devices {
			fmt.Printf("  %s  %s\n", d.ID[:8], d.DeviceName)
		}
		log.Fatalf("Please provide a more specific prefix.")
	}

	device := devices[0]
	if device.Approved && !device.Revoked {
		fmt.Printf("Device %s (%s) is already approved.\n", device.ID[:8], device.DeviceName)
		return
	}

	db.Model(&device).Updates(map[string]interface{}{"approved": true, "revoked": false})
	fmt.Printf("✓ Device approved: %s (%s)\n", device.ID[:8], device.DeviceName)
}

// cmdRejectDevice rejects/revokes a device by ID prefix.
func cmdRejectDevice() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: starclaw reject <device-id-prefix>")
		os.Exit(1)
	}
	prefix := os.Args[2]

	cfg, err := openCLIDB()
	if err != nil {
		log.Fatal(err)
	}
	db, err := database.InitMySQL(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	var devices []model.AuthorizedDevice
	db.Where("id LIKE ?", prefix+"%").Find(&devices)

	if len(devices) == 0 {
		log.Fatalf("No device found matching prefix: %s", prefix)
	}
	if len(devices) > 1 {
		fmt.Printf("Multiple devices match prefix '%s':\n", prefix)
		for _, d := range devices {
			fmt.Printf("  %s  %s\n", d.ID[:8], d.DeviceName)
		}
		log.Fatalf("Please provide a more specific prefix.")
	}

	device := devices[0]
	db.Model(&device).Updates(map[string]interface{}{"approved": false, "revoked": true})
	fmt.Printf("✓ Device rejected: %s (%s)\n", device.ID[:8], device.DeviceName)
}

// cmdExportKey exports the node identity as a 24-word BIP-39 mnemonic.
func cmdExportKey() {
	id := node.LoadOrCreateIdentity()
	seed := id.PrivateKey.Seed()

	mnemonic, err := node.SeedToMnemonic(seed)
	if err != nil {
		log.Fatalf("Failed to encode mnemonic: %v", err)
	}

	// Build HD wallet to show derived addresses
	w := node.WalletFromSeed(seed, mnemonic)

	fmt.Println("╔══════════════════════════════════════════════════╗")
	fmt.Println("║       StarClaw Node Identity Backup              ║")
	fmt.Println("╠══════════════════════════════════════════════════╣")
	fmt.Printf("║  Node ID (cold): %s\n", w.NodeID)
	fmt.Printf("║  Hot address:    %s\n", w.HotNodeID)
	fmt.Printf("║  Fingerprint:    %s\n", id.Fingerprint())
	fmt.Println("╠══════════════════════════════════════════════════╣")
	fmt.Println("║  24-Word Mnemonic (BIP-39):")
	words := strings.Split(mnemonic, " ")
	for i := 0; i < len(words); i += 4 {
		end := i + 4
		if end > len(words) {
			end = len(words)
		}
		nums := ""
		for j := i; j < end; j++ {
			nums += fmt.Sprintf("  %2d.%-12s", j+1, words[j])
		}
		fmt.Printf("║%s\n", nums)
	}
	fmt.Println("╠══════════════════════════════════════════════════╣")
	fmt.Printf("║  Seed (hex): %s\n", hex.EncodeToString(seed))
	fmt.Println("╚══════════════════════════════════════════════════╝")
	fmt.Println("")
	fmt.Println("Write down the 24 words above and store in a SAFE place.")
	fmt.Println("To restore: starclaw import-key <24 words>")
	fmt.Println("")
	fmt.Println("WARNING: Anyone with these words can control your node and wallet.")
}

// cmdImportKey restores node identity from mnemonic (24 words) or seed hex.
func cmdImportKey() {
	if len(os.Args) < 3 {
		fmt.Println("Usage:")
		fmt.Println("  starclaw import-key word1 word2 word3 ... word24")
		fmt.Println("  starclaw import-key <64-char-hex-seed>")
		os.Exit(1)
	}

	var seed []byte
	var err error

	// Detect mode: hex (single arg, 64 chars) or mnemonic (multiple words)
	if len(os.Args) == 3 && len(os.Args[2]) == 64 {
		// Hex seed mode
		seed, err = hex.DecodeString(os.Args[2])
		if err != nil || len(seed) != 32 {
			log.Fatalf("Invalid hex seed: must be 64 hex characters")
		}
		fmt.Println("Importing from hex seed...")
	} else {
		// Mnemonic mode: collect all remaining args as words
		words := os.Args[2:]
		mnemonic := strings.Join(words, " ")
		seed, err = node.MnemonicToSeed(mnemonic)
		if err != nil {
			log.Fatalf("Invalid mnemonic: %v", err)
		}
		fmt.Printf("Importing from %d-word mnemonic...\n", len(words))
	}

	// Build wallet to show info
	w := node.WalletFromSeed(seed, "")

	// Write key file
	keyFile := os.Getenv("NODE_KEY_PATH")
	if keyFile == "" {
		keyFile = ".node_key"
	}

	// Check if key file already exists
	if _, err := os.Stat(keyFile); err == nil {
		fmt.Printf("WARNING: Key file already exists at %s\n", keyFile)
		fmt.Printf("This will OVERWRITE the current node identity.\n")
		fmt.Printf("Type 'yes' to confirm: ")
		var confirm string
		fmt.Scanln(&confirm)
		if confirm != "yes" {
			fmt.Println("Aborted.")
			return
		}
	}

	if err := node.SaveWalletKey(w, keyFile); err != nil {
		log.Fatalf("Failed to write key file: %v", err)
	}

	fmt.Println("========================================")
	fmt.Println("  Node identity restored!")
	fmt.Printf("  Cold address: %s\n", w.NodeID)
	fmt.Printf("  Hot address:  %s\n", w.HotNodeID)
	fmt.Println("========================================")
	fmt.Println("Restart the server for the new identity to take effect.")
}

// cmdWalletInfo shows HD wallet addresses and derivation paths.
func cmdWalletInfo() {
	id := node.LoadOrCreateIdentity()
	seed := id.PrivateKey.Seed()
	w := node.WalletFromSeed(seed, "")

	fmt.Println("╔══════════════════════════════════════════════════╗")
	fmt.Println("║            StarClaw HD Wallet                     ║")
	fmt.Println("╠══════════════════════════════════════════════════╣")
	fmt.Printf("  Master (cold):  %s\n", w.NodeID)
	fmt.Printf("  Path: m (master key)\n")
	fmt.Println("")

	// Show first 5 derived addresses
	fmt.Println("  Derived addresses (BIP-44 / SLIP-0010):")
	fmt.Println("  ─────────────────────────────────────────")
	for i := uint32(0); i < 5; i++ {
		key := w.DeriveAddress(0, 0, i)
		marker := "  "
		if i == 0 {
			marker = "→ " // current hot wallet
		}
		fmt.Printf("  %s[%d] %s  (%s)\n", marker, i, key.NodeID(), key.Path)
	}

	fmt.Println("")
	fmt.Printf("  Hot wallet:     %s\n", w.HotNodeID)
	fmt.Printf("  Path: m/44'/9001'/0'/0'/0'\n")
	fmt.Println("╚══════════════════════════════════════════════════╝")
	fmt.Println("")
	fmt.Println("Cold address = master wallet (high-value ops, backup with mnemonic)")
	fmt.Println("Hot address  = everyday wallet (transfers, heartbeats)")
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
