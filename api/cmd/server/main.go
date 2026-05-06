package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/yinhe/starclaw/internal/abathur"
	agentpkg "github.com/yinhe/starclaw/internal/agent"
	v1 "github.com/yinhe/starclaw/internal/api/v1"
	"github.com/yinhe/starclaw/internal/autonomy"
	"github.com/yinhe/starclaw/internal/broodmind"
	"github.com/yinhe/starclaw/internal/broodnet"
	"github.com/yinhe/starclaw/internal/chitin"
	"github.com/yinhe/starclaw/internal/cocoon"
	"github.com/yinhe/starclaw/internal/config"
	"github.com/yinhe/starclaw/internal/database"
	"github.com/yinhe/starclaw/internal/exchange"
	"github.com/yinhe/starclaw/internal/federation"
	"github.com/yinhe/starclaw/internal/hydralisk"
	"github.com/yinhe/starclaw/internal/hydralisk_v"
	"github.com/yinhe/starclaw/internal/instinct"
	"github.com/yinhe/starclaw/internal/lair"
	"github.com/yinhe/starclaw/internal/model"
	"github.com/yinhe/starclaw/internal/molt"
	"github.com/yinhe/starclaw/internal/mutalisk"
	"github.com/yinhe/starclaw/internal/nerve"
	"github.com/yinhe/starclaw/internal/node"
	"github.com/yinhe/starclaw/internal/partner"
	"github.com/yinhe/starclaw/internal/roach"
	"github.com/yinhe/starclaw/internal/router"
	"github.com/yinhe/starclaw/internal/security"
	"github.com/yinhe/starclaw/internal/sense"
	"github.com/yinhe/starclaw/internal/swarm"
	"github.com/yinhe/starclaw/internal/swarmctl"
	"github.com/yinhe/starclaw/internal/testclaw"
	"github.com/yinhe/starclaw/internal/wiring"
	"github.com/yinhe/starclaw/internal/zergling"
	"gorm.io/gorm"
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

	// Hide console window on Windows (no-op elsewhere)
	hideConsole()

	// Anchor CWD to exe directory if needed (fixes Spore / manual-launch CWD mismatch)
	anchorToExeDir()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Warn if JWT secret is the insecure default (block in release mode)
	if cfg.JWT.Secret == "starclaw-secret-key-change-me" {
		if cfg.Server.Mode == "release" {
			log.Fatalf("[SECURITY] JWT secret is the insecure default. Set JWT_SECRET env var or jwt.secret in config before running in release mode.")
		}
		log.Println("[WARNING] JWT secret is the insecure default 'starclaw-secret-key-change-me'. Set JWT_SECRET for production use.")
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

	// Discover and seed manifest-based agents from agents/ directory
	seedManifestAgents(db)

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

	// Start Molt version checker + upgrade protocol
	molt.InitUpgrade(cfg.Storage.DataDir)
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

	// Initialize BroodMind cognitive engine
	broodmind.Init(identity.NodeID)

	// Initialize security guard (sandbox + trust levels)
	security.InitGuard(security.DefaultSandboxConfig())

	// Initialize swarm formation engine (Phase 3C cross-node coordination)
	swarm.InitFormation(identity.NodeID, identity.NodeID, cfg.Node.Address)

	// Initialize Hydralisk heavy worker (Phase 3C batch processing)
	hydralisk.InitWorker(identity.NodeID, hydralisk.DefaultWorkerConfig())

	// Initialize BroodOS Network (Phase 4A-4/5 + Phase 4B-1/2/3)
	broodnet.InitMarket(identity.NodeID, nil)
	broodnet.InitOrchestrator(identity.NodeID)
	broodnet.InitReputation(nil)
	broodnet.InitPricing(nil)
	broodnet.InitGossip(identity.NodeID, cfg.Node.Address, nil, nil, nil)

	// Initialize Phase 4C engines
	abathur.InitEngine(identity.NodeID, nil)
	sense.InitEngine(identity.NodeID, nil)
	testclaw.InitEngine(identity.NodeID, nil)
	cocoon.InitEngine(identity.NodeID, nil)
	chitin.InitEngine(identity.NodeID, nil)
	lair.InitEngine(identity.NodeID, nil)
	partner.InitEngine(identity.NodeID, nil)

	// Initialize physical variant adapters (Phase 4B)
	zergling.InitAdapter(identity.NodeID, nil)
	mutalisk.InitAdapter(identity.NodeID, nil)
	roach.InitAdapter(identity.NodeID, nil)
	hydralisk_v.InitAdapter(identity.NodeID, nil)

	// Initialize Nerve Bus — cross-engine wiring (Phase 4D)
	nerveBus := nerve.InitBus()
	nerve.RegisterAllWorkers(nerveBus)
	nerve.RegisterCrossEngineSubscriptions(nerveBus)

	// Initialize Phase 5 engines (with DB persistence)
	autonomy.InitEngine(identity.NodeID, nil, db)
	exchange.InitEngine(identity.NodeID, nil, db)
	federation.InitEngine(identity.NodeID, nil, db)
	swarmctl.InitEngine(identity.NodeID, db)

	// Wire Phase 5 engines to Nerve Bus
	wiring.WirePhase5(nerveBus)

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

// seedManifestAgents discovers agents from the agents/ directory and seeds them into the DB.
// This allows manifest-defined agents (like pitch_master) to auto-register on startup.
func seedManifestAgents(db *gorm.DB) {
	// Determine owner ID (same logic as SeedBuiltinAgents)
	ownerID := model.SystemUserID
	var owner model.User
	if err := db.Where("owner_token IS NOT NULL AND owner_token != ''").First(&owner).Error; err == nil {
		ownerID = owner.ID
	}

	// Try multiple candidate paths for agents/ directory
	exe, _ := os.Executable()
	exeDir := filepath.Dir(exe)
	candidates := []string{
		filepath.Join(exeDir, "agents"),
		filepath.Join(exeDir, "..", "agents"),
		filepath.Join(exeDir, "..", "..", "agents"),
		"agents",
		filepath.Join("..", "agents"),
	}
	if v := os.Getenv("CLAW_AGENTS_DIR"); v != "" {
		candidates = append([]string{v}, candidates...)
	}

	var agentsDir string
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && info.IsDir() {
			agentsDir = c
			break
		}
	}
	if agentsDir == "" {
		log.Println("[discovery] No agents/ directory found, skipping manifest discovery")
		return
	}

	manifests, err := agentpkg.ScanAgentsDir(agentsDir)
	if err != nil {
		log.Printf("[discovery] Failed to scan agents dir: %v", err)
		return
	}
	if len(manifests) == 0 {
		return
	}

	agentpkg.GlobalManifests = manifests
	agentpkg.SeedFromManifests(db, manifests, ownerID)
	log.Printf("[discovery] Seeded %d manifest-based agents", len(manifests))
}
