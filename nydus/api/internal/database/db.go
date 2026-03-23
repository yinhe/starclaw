package database

import (
	"log"
	"os"
	"path/filepath"

	"github.com/yinhe/starclaw/nydus/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

// Init opens (or creates) the SQLite database and auto-migrates models.
func Init(dbPath string) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Fatalf("[nydus] failed to create db dir %s: %v", dir, err)
	}

	var err error
	DB, err = gorm.Open(sqlite.Open(dbPath+"?_journal_mode=WAL&_busy_timeout=5000"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Fatalf("[nydus] failed to open database %s: %v", dbPath, err)
	}

	if err := DB.AutoMigrate(
		&model.NydusNode{},
		&model.NydusRepo{},
		&model.RepoAccess{},
	); err != nil {
		log.Fatalf("[nydus] auto-migrate failed: %v", err)
	}

	log.Printf("[nydus] database ready: %s", dbPath)
}
