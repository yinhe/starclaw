package database

import (
	"fmt"
	"log"

	"github.com/yinhe/starclaw/internal/config"
	"gorm.io/gorm"
)

// InitDB initializes the database based on the configured driver.
// Supports "mysql" and "sqlite". Defaults to "mysql" for backward compatibility.
func InitDB(cfg *config.Config) (*gorm.DB, error) {
	driver := cfg.Database.Driver
	if driver == "" {
		driver = "mysql"
	}

	switch driver {
	case "sqlite":
		log.Printf("[database] Using SQLite driver (path: %s)", cfg.Database.SQLitePath)
		return InitSQLite(cfg)
	case "mysql":
		log.Printf("[database] Using MySQL driver (%s:%d/%s)", cfg.Database.Host, cfg.Database.Port, cfg.Database.DBName)
		return InitMySQL(cfg)
	default:
		return nil, fmt.Errorf("unsupported database driver: %s (use 'mysql' or 'sqlite')", driver)
	}
}
