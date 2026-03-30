package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"starclaw.net/extractor/api/internal/handler"
	"starclaw.net/extractor/api/internal/model"
)

func main() {
	dsn := getEnv("EXTRACTOR_DATABASE_DSN", "sqlite:extractor.db")
	var db *gorm.DB
	var err error
	if strings.HasPrefix(dsn, "sqlite:") {
		dbPath := strings.TrimPrefix(dsn, "sqlite:")
		db, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
		log.Printf("Using SQLite: %s", dbPath)
	} else {
		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
		log.Printf("Using PostgreSQL")
	}
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}

	if err := model.AutoMigrate(db); err != nil {
		log.Fatalf("failed to migrate: %v", err)
	}

	model.SeedAccountBindings(db)
	model.SeedRiskRules(db)

	bridgeURL := getEnv("EXTRACTOR_BRIDGE_URL", "http://localhost:8098")

	r := gin.Default()
	handler.Setup(r, db, bridgeURL)

	port := getEnv("EXTRACTOR_PORT", "8097")
	fmt.Printf("🏦 Extractor API starting on :%s (bridge: %s)\n", port, bridgeURL)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
