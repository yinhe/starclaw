package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	pheromone "starclaw.net/pheromone/sdk"
	"starclaw.net/queen/api/internal/config"
	"starclaw.net/queen/api/internal/database"
	"starclaw.net/queen/api/internal/handler"
	"starclaw.net/queen/api/internal/model"
	"starclaw.net/queen/api/internal/router"
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
		// City Partner Portal
		&model.CityPartner{},
		&model.CityClient{},
		&model.Commission{},
		&model.Payout{},
		&model.MarketingMaterial{},
		// Team Partner Hub
		&model.TeamPartner{},
		&model.CRMDeal{},
		&model.PartnerCommission{},
		&model.EquityGrant{},
		&model.Deployment{},
		// Settlement Engine
		&model.SettlementBill{},
		&model.SettlementLineItem{},
		&model.SettlementConfig{},
		// Investor Pool
		&model.InvestorPool{},
		&model.Investor{},
		&model.InvestorTransaction{},
		&model.InvestorDividend{},
		&model.PoolDeposit{},
		&model.FundingRound{},
		&model.DiamondOrder{},
		// Team Elections
		&model.TeamVote{},
		&model.TeamElection{},
		// Partner Invites
		&model.PartnerInvite{},
		&model.PartnerInviteUse{},
	)

	seedAdmin()
	handler.SeedOfficialAgents()

	// Connect to Pheromone ESB
	natsURL := os.Getenv("PHEROMONE_NATS_URL")
	if natsURL == "" {
		natsURL = "nats://127.0.0.1:4222"
	}
	ph, err := pheromone.New(natsURL, pheromone.ServiceInfo{
		Name:    "queen",
		Version: "1.0.0",
		Port:    8085,
		Tags:    []string{"community", "marketplace", "billing"},
	})
	if err != nil {
		log.Printf("[queen] pheromone connect failed (non-fatal): %v", err)
	} else {
		ph.StartHeartbeat(30 * time.Second)
		defer ph.Close()
		handler.SetPheromone(ph)
		handler.RegisterPheromoneRPC(ph)
		log.Printf("[queen] pheromone ESB connected (%s)", natsURL)
	}

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
