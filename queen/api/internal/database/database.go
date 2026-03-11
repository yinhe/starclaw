package database

import (
	"log"

	"github.com/yinhe/starclaw-queen/api/internal/config"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func Init() {
	var logLevel logger.LogLevel
	if config.C.Server.Mode == "debug" {
		logLevel = logger.Info
	} else {
		logLevel = logger.Silent
	}

	var err error
	DB, err = gorm.Open(mysql.Open(config.C.Database.DSN), &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
	})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	sqlDB, _ := DB.DB()
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
}
