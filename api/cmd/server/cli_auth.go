package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"os"

	"github.com/yinhe/starclaw/internal/config"
	"github.com/yinhe/starclaw/internal/database"
	"github.com/yinhe/starclaw/internal/model"
	"golang.org/x/crypto/bcrypt"
)

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
