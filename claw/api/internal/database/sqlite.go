package database

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/glebarez/sqlite"
	"github.com/yinhe/starclaw/internal/config"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func InitSQLite(cfg *config.Config) (*gorm.DB, error) {
	dbPath := cfg.Database.SQLitePath
	if dbPath == "" {
		dbPath = "./data/claw.db"
	}

	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory %s: %w", dir, err)
	}

	logLevel := logger.Info
	if cfg.Server.Mode == "release" {
		logLevel = logger.Warn
	}

	// SQLite pragmas for better performance
	dsn := fmt.Sprintf("%s?_journal_mode=WAL&_busy_timeout=5000&_synchronous=NORMAL&_cache_size=1000000000&_foreign_keys=ON", dbPath)

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open SQLite database: %w", err)
	}

	// SQLite single-connection mode for write safety
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(1)

	return db, nil
}
