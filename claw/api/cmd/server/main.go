package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	v1 "github.com/yinhe/starclaw/internal/api/v1"
	"github.com/yinhe/starclaw/internal/config"
	"github.com/yinhe/starclaw/internal/database"
	"github.com/yinhe/starclaw/internal/instinct"
	"github.com/yinhe/starclaw/internal/model"
	"github.com/yinhe/starclaw/internal/molt"
	"github.com/yinhe/starclaw/internal/node"
	"github.com/yinhe/starclaw/internal/router"
	"github.com/yinhe/starclaw/internal/swarm"
	"golang.org/x/crypto/bcrypt"
)

func init() {
	// Force China Standard Time globally so time.Now() returns CST
	// everywhere — cron triggers, billing boundaries, dashboard stats, etc.
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = time.FixedZone("CST", 8*3600)
	}
	time.Local = loc
}

func main() {
	// Handle CLI subcommands before starting the full server
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "token", "get-token":
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
		case "balance":
			cmdBalance()
			return
		case "transfer":
			cmdTransfer()
			return
		case "transactions":
			cmdTransactions()
			return
		case "version":
			fmt.Printf("StarClaw v%s\n", molt.Version)
			return
		case "help", "--help", "-h":
			printUsage()
			return
		}
	}

	// Anchor CWD to exe directory if needed (fixes Spore / manual-launch CWD mismatch)
	anchorToExeDir()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Spore upgrade migration: if SPORE_DATA_DIR is set (shared dir) but has no
	// claw.db, and the version-local ./data/ does → migrate data to shared dir.
	// This runs once on the first upgrade from old (version-local) to new (shared) layout.
	migrateSporeData(cfg)

	// Initialize database (MySQL or SQLite based on config)
	db, err := database.InitDB(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Auto migrate database schemas
	if err := database.AutoMigrate(db); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}

	// Create composite indexes for query performance
	database.EnsureIndexes(db)

	// Seed system-level built-in agents (visible to all users)
	v1.SeedBuiltinAgents(db)

	// Seed marketplace templates (coding assistant, translator, etc.)
	v1.SeedBuiltinTemplates(db)

	// Auto-seed missing built-in activities for all users who have a SuperAgent
	// This ensures new instinct templates are available even for existing users
	go func() {
		var superAgents []model.Agent
		db.Where("name = ? AND is_builtin = ?", "全能助手", true).Find(&superAgents)
		for _, sa := range superAgents {
			instinct.SeedBuiltinActivities(db, sa.UserID, sa.ID)
		}
	}()

	// Initialize Redis (optional — nil means in-memory fallback)
	var rdb *redis.Client
	if cfg.Redis.Enabled {
		rdb, err = database.InitRedis(cfg)
		if err != nil {
			log.Printf("[warning] Redis unavailable, using in-memory cache: %v", err)
			rdb = nil
		} else {
			defer rdb.Close()
		}
	} else {
		log.Println("[info] Redis disabled, using in-memory cache")
	}

	// Start Molt version checker
	molt.StartChecker()
	if cfg.Hive.URL != "" {
		molt.SetHiveURL(cfg.Hive.URL)
	}
	log.Printf("StarClaw v%s starting...", molt.Version)

	// Tell node package where data lives (so .node_key is found in data dir)
	node.SetDataDir(cfg.Storage.DataDir)

	// Load crypto identity for claw: address
	identity := node.LoadOrCreateIdentity()
	log.Printf("Node ID: %s", identity.NodeID)

	// Log full boot context for diagnostics
	logBootContext(cfg)

	// Start Swarm client (register with Queen + heartbeat)
	swarmClient := swarm.NewClient(cfg.Swarm)
	swarmClient.SetIdentity(identity) // sets clawID + initializes CreditClient
	if cfg.Node.Address != "" {
		swarmClient.SetAddress(cfg.Node.Address)
	}
	swarmClient.UpdateFunc = v1.PerformDockerUpdate // wire auto-update
	swarmClient.Start()
	defer swarmClient.Stop()

	// Report pending molt update result from previous restart (if any)
	go swarmClient.ReportPendingMolt()

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
	fmt.Println("")
	fmt.Println("Auth:")
	fmt.Println("  token            Show current Owner Token (alias: get-token)")
	fmt.Println("  reset-token      Generate a new Owner Token")
	fmt.Println("  reset-password   Reset the Owner password (--password <pw>)")
	fmt.Println("")
	fmt.Println("Devices:")
	fmt.Println("  devices          List all authorized devices")
	fmt.Println("  approve <id>     Approve a pending device (supports ID prefix)")
	fmt.Println("  reject <id>      Reject/revoke a device (supports ID prefix)")
	fmt.Println("")
	fmt.Println("Identity & Wallet:")
	fmt.Println("  export-key       Export 24-word mnemonic (BIP-39 backup)")
	fmt.Println("  import-key       Restore identity from mnemonic or seed hex")
	fmt.Println("  wallet-info      Show HD wallet addresses and derivation paths")
	fmt.Println("  balance          Show star energy balance and HP status")
	fmt.Println("  transfer         Transfer stars to another claw address")
	fmt.Println("  transactions     Show recent transaction history")
	fmt.Println("")
	fmt.Println("Other:")
	fmt.Println("  version          Print version and exit")
	fmt.Println("  help             Show this help")
	fmt.Println("")
	fmt.Println("Examples:")
	fmt.Println("  starclaw token")
	fmt.Println("  starclaw reset-token")
	fmt.Println("  starclaw reset-password --password newpass123")
	fmt.Println("  starclaw devices")
	fmt.Println("  starclaw approve a1b2c3d4")
	fmt.Println("  starclaw export-key")
	fmt.Println("  starclaw import-key <24 words or seed-hex>")
	fmt.Println("  starclaw wallet-info")
	fmt.Println("  starclaw balance")
	fmt.Println("  starclaw transfer <claw:address> <amount_stars> [remark]")
	fmt.Println("  starclaw transactions [--type transfer] [--page 1]")
}

// anchorToExeDir changes the working directory to the executable's directory
// when CWD doesn't contain a config file but the exe directory does.
// This fixes the #1 cause of "wrong database / missing node_key / setup page"
// when users launch claw-api.exe from the wrong directory.
func anchorToExeDir() {
	exeDir := config.ExeDir()
	if exeDir == "" {
		return // can't determine (go run), stay in CWD
	}

	cwd, _ := os.Getwd()
	if cwd == exeDir {
		return // already there
	}

	// If exe directory has a config file, always chdir there.
	// The exe's directory IS the intended runtime context (set by Spore, Docker, etc.).
	// This ensures .node_key, data/, web/, config.yaml all resolve correctly.
	for _, p := range []string{
		filepath.Join(exeDir, "config.yaml"),
		filepath.Join(exeDir, "configs", "config.yaml"),
	} {
		if _, err := os.Stat(p); err == nil {
			log.Printf("[boot] Anchoring CWD to exe directory: %s (was: %s)", exeDir, cwd)
			os.Chdir(exeDir)
			return
		}
	}
}

// logBootContext prints the full runtime binding at startup for diagnostics.
// This makes CWD/DB/data path issues immediately visible in logs.
func logBootContext(cfg *config.Config) {
	cwd, _ := os.Getwd()
	exeDir := config.ExeDir()

	dbPath := cfg.Database.SQLitePath
	if cfg.Database.Driver == "sqlite" && dbPath != "" {
		if abs, err := filepath.Abs(dbPath); err == nil {
			dbPath = abs
		}
	}

	dataAbs := cfg.Storage.DataDir
	if abs, err := filepath.Abs(dataAbs); err == nil {
		dataAbs = abs
	}

	nodeKeyPath := os.Getenv("NODE_KEY_PATH")
	if nodeKeyPath == "" {
		nodeKeyPath = ".node_key"
	}
	if abs, err := filepath.Abs(nodeKeyPath); err == nil {
		nodeKeyPath = abs
	}

	log.Printf("[boot] CWD=%s", cwd)
	log.Printf("[boot] ExeDir=%s", exeDir)
	if sd := os.Getenv("SPORE_DATA_DIR"); sd != "" {
		log.Printf("[boot] SPORE_DATA_DIR=%s (shared, upgrade-safe)", sd)
	}
	if cfg.Database.Driver == "sqlite" {
		log.Printf("[boot] DB=%s (sqlite)", dbPath)
	} else {
		log.Printf("[boot] DB=%s@%s:%d/%s", cfg.Database.Driver, cfg.Database.Host, cfg.Database.Port, cfg.Database.DBName)
	}
	log.Printf("[boot] DataDir=%s (abs=%s)", cfg.Storage.DataDir, dataAbs)
	log.Printf("[boot] NodeKey=%s", nodeKeyPath)
}

// migrateSporeData handles one-time migration from version-local data to shared data dir.
// Old layout: each version has its own ./data/ (claw.db, uploads, workspaces, .node_key)
// New layout: SPORE_DATA_DIR points to a shared dir that persists across upgrades.
// This function copies data from version-local to shared on first upgrade.
func migrateSporeData(cfg *config.Config) {
	sporeDataDir := os.Getenv("SPORE_DATA_DIR")
	if sporeDataDir == "" {
		return // not running under Spore
	}

	// Check if version-local ./data/ has data to migrate
	localDB := filepath.Join(".", "data", "claw.db")
	localInfo, err := os.Stat(localDB)
	if err != nil {
		return // no local data, fresh install
	}

	// Compare with shared dir: migrate if shared db is missing or smaller
	// (a smaller shared db means it's an empty scaffold from initial install)
	sharedDB := filepath.Join(sporeDataDir, "claw.db")
	sharedInfo, sharedErr := os.Stat(sharedDB)
	if sharedErr == nil && sharedInfo.Size() >= localInfo.Size() {
		return // shared dir already has equal or more data, skip
	}

	log.Printf("[boot] Spore upgrade migration: copying data from ./data/ to %s (local=%d bytes, shared=%d bytes)",
		sporeDataDir, localInfo.Size(), func() int64 {
			if sharedErr == nil {
				return sharedInfo.Size()
			}
			return 0
		}())
	os.MkdirAll(sporeDataDir, 0755)

	// Copy all files from version-local data/ to shared data/
	localDataDir := filepath.Join(".", "data")
	entries, err := os.ReadDir(localDataDir)
	if err != nil {
		log.Printf("[boot] migration: failed to read local data dir: %v", err)
		return
	}

	for _, entry := range entries {
		src := filepath.Join(localDataDir, entry.Name())
		dst := filepath.Join(sporeDataDir, entry.Name())
		if entry.IsDir() {
			if err := copyDirRecursive(src, dst); err != nil {
				log.Printf("[boot] migration: failed to copy dir %s: %v", entry.Name(), err)
			}
		} else {
			if err := copyFile(src, dst); err != nil {
				log.Printf("[boot] migration: failed to copy %s: %v", entry.Name(), err)
			}
		}
	}

	// Also migrate .node_key from CWD to shared data dir
	if _, err := os.Stat(".node_key"); err == nil {
		if err := copyFile(".node_key", filepath.Join(sporeDataDir, ".node_key")); err != nil {
			log.Printf("[boot] migration: failed to copy .node_key: %v", err)
		}
	}

	log.Printf("[boot] Spore migration complete: data now in %s", sporeDataDir)
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

func copyDirRecursive(src, dst string) error {
	os.MkdirAll(dst, 0755)
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		s := filepath.Join(src, entry.Name())
		d := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			if err := copyDirRecursive(s, d); err != nil {
				return err
			}
		} else {
			if err := copyFile(s, d); err != nil {
				return err
			}
		}
	}
	return nil
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
	db, err := database.InitDB(cfg)
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
	db, err := database.InitDB(cfg)
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
	db, err := database.InitDB(cfg)
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
	db, err := database.InitDB(cfg)
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
	db, err := database.InitDB(cfg)
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

// migrateIdentity calls Queen's identity migration API to transfer balance/bindings
// from an old claw address to a new one. Runs async, non-fatal on failure.
var _ = migrateIdentity // suppress unused lint — called conditionally at startup

func migrateIdentity(queenURL, token, oldClawID, newClawID string) {
	body := fmt.Sprintf(`{"old_claw_id":"%s","new_claw_id":"%s"}`, oldClawID, newClawID)
	url := strings.TrimSuffix(queenURL, "/swarm") + "/internal/identity/migrate"

	req, _ := http.NewRequest("POST", url, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Node-Token", token)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[identity] migration request failed (non-fatal): %v", err)
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusOK {
		log.Printf("[identity] ✅ migration successful: %s → %s: %s", oldClawID, newClawID, string(respBody))
	} else {
		log.Printf("[identity] migration returned %d: %s (non-fatal)", resp.StatusCode, string(respBody))
	}
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

// cmdBalance queries and displays the star energy balance from Queen.
func cmdBalance() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	identity := node.LoadOrCreateIdentity()
	if cfg.Swarm.QueenURL == "" {
		log.Fatalf("queen_url not configured in swarm settings")
	}

	cc := swarm.NewCreditClient(cfg.Swarm.QueenURL, identity)
	balance, err := cc.QueryBalance()
	if err != nil {
		log.Fatalf("Failed to query balance: %v", err)
	}

	hpIcon := map[string]string{
		"full": "\u2764\ufe0f", "healthy": "\U0001f49a", "low": "\U0001f49b",
		"critical": "\u2764\ufe0f\u200d\U0001fa79", "hibernated": "\U0001f480",
	}[balance.HPStatus]
	if hpIcon == "" {
		hpIcon = "\u2753"
	}

	fmt.Println("\u2554\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2557")
	fmt.Println("\u2551       StarClaw Star Energy                     \u2551")
	fmt.Println("\u2560\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2563")
	fmt.Printf("  Claw ID:     %s\n", identity.NodeID)
	fmt.Printf("  Balance:     %.2f Stars\n", balance.BalanceEnergy)
	fmt.Printf("  Frozen:      %.2f Stars\n", balance.FrozenEnergy)
	fmt.Printf("  Total In:    %d units\n", balance.TotalIn)
	fmt.Printf("  Total Out:   %d units\n", balance.TotalOut)
	fmt.Printf("  Nonce:       %d\n", balance.Nonce)
	fmt.Printf("  HP Status:   %s %s\n", hpIcon, balance.HPStatus)
	fmt.Printf("  Trust Level: %s\n", balance.TrustLevel)
	fmt.Printf("  Status:      %s\n", balance.Status)
	fmt.Println("\u255a\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u255d")
}

// cmdTransfer sends star energy to another claw address.
func cmdTransfer() {
	if len(os.Args) < 4 {
		fmt.Println("Usage: starclaw transfer <claw:address> <amount_stars> [remark]")
		fmt.Println("  amount is in Stars (e.g. 10.5 = 10.5 Stars)")
		os.Exit(1)
	}

	target := os.Args[2]
	amountStr := os.Args[3]
	remark := ""
	if len(os.Args) > 4 {
		remark = strings.Join(os.Args[4:], " ")
	}

	if !strings.HasPrefix(target, "claw:") {
		log.Fatalf("Invalid target address: must start with claw:")
	}

	amountEnergy, err := strconv.ParseFloat(amountStr, 64)
	if err != nil || amountEnergy <= 0 {
		log.Fatalf("Invalid amount: must be a positive number")
	}
	amountUnits := int64(amountEnergy * 10000) // 1 Star = 10000 units

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	identity := node.LoadOrCreateIdentity()
	if cfg.Swarm.QueenURL == "" {
		log.Fatalf("queen_url not configured in swarm settings")
	}

	cc := swarm.NewCreditClient(cfg.Swarm.QueenURL, identity)

	fmt.Printf("Transferring %.2f Stars (%d units) to %s...\n", amountEnergy, amountUnits, target)

	result, err := cc.Transfer(swarm.TransferRequest{
		ToClaw: target,
		Amount: amountUnits,
		Remark: remark,
	})
	if err != nil {
		log.Fatalf("Transfer failed: %v", err)
	}

	fmt.Println("========================================")
	fmt.Printf("  Transaction ID: %s\n", result.TxnID)
	fmt.Printf("  From:           %s\n", result.From)
	fmt.Printf("  To:             %s\n", result.To)
	fmt.Printf("  Amount:         %.2f Stars\n", result.AmountEnergy)
	fmt.Printf("  New Balance:    %d units\n", result.NewBalance)
	fmt.Println("========================================")
}

// cmdTransactions lists recent transaction history.
func cmdTransactions() {
	page := 1
	pageSize := 20
	txnType := ""

	for i := 2; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "--type":
			if i+1 < len(os.Args) {
				txnType = os.Args[i+1]
				i++
			}
		case "--page":
			if i+1 < len(os.Args) {
				page, _ = strconv.Atoi(os.Args[i+1])
				i++
			}
		case "--size":
			if i+1 < len(os.Args) {
				pageSize, _ = strconv.Atoi(os.Args[i+1])
				i++
			}
		}
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	identity := node.LoadOrCreateIdentity()
	if cfg.Swarm.QueenURL == "" {
		log.Fatalf("queen_url not configured in swarm settings")
	}

	cc := swarm.NewCreditClient(cfg.Swarm.QueenURL, identity)
	list, err := cc.ListTransactions(page, pageSize, txnType)
	if err != nil {
		log.Fatalf("Failed to list transactions: %v", err)
	}

	if len(list.Transactions) == 0 {
		fmt.Println("No transactions found.")
		return
	}

	fmt.Printf("Transactions (page %d, total %d):\n", list.Page, list.Total)
	fmt.Printf("%-12s %-10s %-16s %-16s %12s  %s\n", "TYPE", "STATUS", "FROM", "TO", "AMOUNT", "TIME")
	fmt.Println(strings.Repeat("-", 90))

	for _, txn := range list.Transactions {
		from := truncate(txn.FromClaw, 16)
		to := truncate(txn.ToClaw, 16)
		stars := fmt.Sprintf("%.2f", float64(txn.Amount)/10000)
		t := txn.CreatedAt.Format("01-02 15:04")
		fmt.Printf("%-12s %-10s %-16s %-16s %10s \u2b50  %s\n", txn.Type, txn.Status, from, to, stars, t)
	}
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
	db, err := database.InitDB(cfg)
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
