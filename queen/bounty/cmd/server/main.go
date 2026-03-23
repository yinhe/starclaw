package main

import (
	"fmt"
	"log"
	"math"
	"os"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"starclaw.net/queen/bounty/internal/billing"
	"starclaw.net/queen/bounty/internal/handler"
	"starclaw.net/queen/bounty/internal/model"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func main() {
	dsn := getEnv("BOUNTY_DSN", "root:starclaw@tcp(127.0.0.1:3306)/starclaw_queen?charset=utf8mb4&parseTime=True&loc=Local")
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect database: %v", err)
	}
	db.AutoMigrate(&model.Bounty{}, &model.BountyUser{})

	// Init billing client for fund escrow (optional — gracefully disabled if not configured)
	var billingClient *billing.Client
	queenAPIURL := getEnv("QUEEN_API_URL", "")
	nodeToken := getEnv("NODE_TOKEN", "")
	if queenAPIURL != "" && nodeToken != "" {
		billingClient = billing.NewClient(queenAPIURL, nodeToken)
		log.Printf("[bounty] Billing integration enabled (queen-api=%s)", queenAPIURL)
	} else {
		log.Println("[bounty] Billing integration disabled (QUEEN_API_URL or NODE_TOKEN not set)")
	}

	// Expire stale bounties (with billing unfreeze)
	go expireLoop(db, billingClient)

	mode := getEnv("GIN_MODE", "debug")
	if mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.Default()
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	h := handler.NewBountyHandler(db, billingClient)

	bounties := r.Group("/bounties")
	{
		bounties.POST("", h.Create)
		bounties.GET("", h.List)
		bounties.GET("/stats", h.Stats)
		bounties.GET("/categories", h.Categories)
		bounties.GET("/:id", h.Get)
		bounties.POST("/:id/claim", h.Claim)
		bounties.POST("/:id/deliver", h.Deliver)
		bounties.POST("/:id/accept", h.Accept)
		bounties.POST("/:id/cancel", h.Cancel)
		bounties.POST("/:id/dispute", h.Dispute)
	}

	r.GET("/metrics", gin.WrapH(promhttp.Handler()))
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "queen-bounty"})
	})
	r.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"service":   "queen-bounty",
			"version":   "1.0.0",
			"docs":      "https://starclaw.net/docs/bounty",
			"endpoints": []string{"/bounties", "/bounties/:id", "/bounties/:id/claim", "/health"},
		})
	})

	port := getEnv("BOUNTY_PORT", "8092")
	log.Printf("[bounty] Queen Bounty service starting on :%s", port)
	if err := r.Run(fmt.Sprintf(":%s", port)); err != nil {
		log.Fatalf("Failed to start bounty service: %v", err)
	}
}

// expireLoop marks open bounties past their deadline as expired and unfreezes funds
func expireLoop(db *gorm.DB, bc *billing.Client) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		// Find bounties to expire
		var expiring []model.Bounty
		db.Where("status = ? AND deadline IS NOT NULL AND deadline < ?", model.BountyOpen, time.Now()).
			Find(&expiring)

		for _, b := range expiring {
			// Unfreeze funds back to creator
			if bc != nil {
				cents := int64(math.Round(b.Reward * 100))
				if err := bc.Unfreeze(b.UserID, cents, b.ID); err != nil {
					log.Printf("[bounty] Expire unfreeze failed for bounty=%s: %v", b.ID, err)
				}
			}
			db.Model(&b).Update("status", model.BountyExpired)
		}
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
