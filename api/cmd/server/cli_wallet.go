package main

import (
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/yinhe/starclaw/internal/config"
	"github.com/yinhe/starclaw/internal/node"
	"github.com/yinhe/starclaw/internal/swarm"
)

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
