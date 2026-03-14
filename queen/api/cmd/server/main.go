package main

import (
	"fmt"
	"log"
	"os"

	"github.com/google/uuid"
	"github.com/yinhe/starclaw-queen/api/internal/config"
	"github.com/yinhe/starclaw-queen/api/internal/database"
	"github.com/yinhe/starclaw-queen/api/internal/model"
	"github.com/yinhe/starclaw-queen/api/internal/router"
	"golang.org/x/crypto/bcrypt"
)

func seedAdmin() {
	var count int64
	database.DB.Model(&model.User{}).Where("role = ?", "admin").Count(&count)
	if count > 0 {
		return
	}

	password := os.Getenv("ADMIN_PASSWORD")
	if password == "" {
		password = "admin123456"
	}
	email := os.Getenv("ADMIN_EMAIL")
	if email == "" {
		email = "admin@starclaw.me"
	}

	hashed, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	admin := model.User{
		ID:       uuid.New().String(),
		Email:    email,
		Nickname: "管理员",
		Password: string(hashed),
		Role:     "admin",
		Status:   "active",
	}
	if err := database.DB.Create(&admin).Error; err != nil {
		log.Printf("[seed] failed to create admin: %v", err)
		return
	}
	log.Printf("[seed] default admin created: %s / %s", email, password)
}

func main() {
	config.Load()
	database.Init()

	// Auto migrate
	database.DB.AutoMigrate(
		&model.User{},
		&model.SMSCode{},
		&model.MarketplaceItem{},
		&model.MarketplaceReview{},
		&model.APIKey{},
		&model.UserBalance{},
		&model.RechargeOrder{},
		&model.BalanceTransaction{},
		&model.RechargePackage{},
		&model.NodeBinding{},
		&model.ContentReport{},
		// Star Energy (星能)
		&model.CreditAccount{},
		&model.CreditTransaction{},
		&model.CreditFreeze{},
		// Gateway (star-ai.net)
		&model.GatewayUsageLog{},
	)

	seedAdmin()

	r := router.Setup()

	port := config.C.Server.Port
	if port == "" {
		port = "8080"
	}
	log.Printf("Queen API starting on :%s", port)
	if err := r.Run(fmt.Sprintf(":%s", port)); err != nil {
		log.Fatal(err)
	}
}
