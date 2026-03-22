package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"

	"github.com/yinhe/starclaw-router/internal/config"
	"github.com/yinhe/starclaw-router/internal/database"
	"github.com/yinhe/starclaw-router/internal/model"
)

// seed creates a test user and API key for development/testing
func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	db, err := database.InitMySQL(cfg)
	if err != nil {
		log.Fatalf("mysql: %v", err)
	}

	database.AutoMigrate(db)

	// Create test user
	user := model.User{
		ID:        "test-user-001",
		Email:     "test@star-ai.net",
		Name:      "Test User",
		Balance:   100000, // 1000 CNY = 100000 分
		FreeQuota: 1000000,
		Status:    "active",
	}

	if err := db.Where("id = ?", user.ID).FirstOrCreate(&user).Error; err != nil {
		log.Fatalf("create user: %v", err)
	}
	fmt.Printf("User: %s (%s)\n", user.Name, user.ID)

	// Generate and store API key
	rawKey := model.GenerateAPIKey()
	hash := sha256.Sum256([]byte(rawKey))
	keyHash := hex.EncodeToString(hash[:])

	apiKey := model.APIKey{
		UserID:    user.ID,
		Name:      "Test Key",
		KeyHash:   keyHash,
		KeyPrefix: rawKey[:16] + "...",
		IsEnabled: true,
	}

	if err := db.Create(&apiKey).Error; err != nil {
		log.Fatalf("create key: %v", err)
	}

	fmt.Println("========================================")
	fmt.Printf("API Key (save this, shown once only):\n\n  %s\n\n", rawKey)
	fmt.Println("========================================")
	fmt.Println("Test with:")
	fmt.Printf("  curl -H \"Authorization: Bearer %s\" http://localhost:8096/v1/models\n", rawKey)
}
