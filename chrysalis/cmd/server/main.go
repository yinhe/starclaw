package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"starclaw.net/chrysalis/internal/engine"
	"starclaw.net/chrysalis/internal/handler"
	"starclaw.net/chrysalis/internal/model"
)

func main() {
	dsn := getEnv("CHRYSALIS_DSN", getEnv("ARENA_DSN", "root:starclaw@tcp(127.0.0.1:3306)/starclaw_queen?charset=utf8mb4&parseTime=True&loc=Local"))
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect database: %v", err)
	}
	db.AutoMigrate(
		&model.BattleFighter{}, &model.EquipmentDef{}, &model.EquipmentInstance{}, &model.Battle{},
		&model.StardustAccount{}, &model.StardustTransaction{},
		&model.Season{}, &model.SeasonRecord{},
		&model.MaterialDef{}, &model.CraftMaterial{}, &model.CraftRecipe{},
		&model.Mutation{},
	)

	// Seed equipment defs if empty
	var defCount int64
	db.Model(&model.EquipmentDef{}).Count(&defCount)
	if defCount == 0 {
		for _, def := range model.SeedEquipmentDefs() {
			db.Create(&def)
		}
		log.Printf("[chrysalis] Seeded %d equipment definitions", len(model.SeedEquipmentDefs()))
	}

	// Seed materials
	var matCount int64
	db.Model(&model.MaterialDef{}).Count(&matCount)
	if matCount == 0 {
		for _, m := range model.SeedMaterials() {
			db.Create(&m)
		}
		log.Printf("[chrysalis] Seeded %d material definitions", len(model.SeedMaterials()))
	}

	// Seed recipes
	var recipeCount int64
	db.Model(&model.CraftRecipe{}).Count(&recipeCount)
	if recipeCount == 0 {
		for _, r := range model.SeedRecipes() {
			db.Create(&r)
		}
		log.Printf("[chrysalis] Seeded %d craft recipes", len(model.SeedRecipes()))
	}

	// Seed seasons
	var seasonCount int64
	db.Model(&model.Season{}).Count(&seasonCount)
	if seasonCount == 0 {
		for _, s := range model.SeedSeasons() {
			db.Create(&s)
		}
		log.Printf("[chrysalis] Seeded initial season")
	}

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

	queen := engine.NewQueenClient()
	bh := handler.NewBattleHandler(db, queen)

	// Start season auto-rotation (hourly check)
	rotator := engine.NewSeasonRotator(db)
	rotator.Start()

	pk := r.Group("/chrysalis/pk")
	{
		// Fighter
		pk.POST("/register", bh.RegisterFighter)
		pk.POST("/sync", bh.SyncFighter)
		pk.GET("/fighter/:claw_id", bh.GetFighter)
		pk.POST("/challenge", bh.Challenge)
		pk.GET("/history/:claw_id", bh.BattleHistory)
		pk.GET("/leaderboard", bh.PKLeaderboard)

		// Shop & Equipment
		pk.GET("/shop", bh.ListShop)
		pk.POST("/shop/buy", bh.BuyEquipment)
		pk.POST("/equip", bh.EquipItem)
		pk.POST("/unequip", bh.UnequipItem)
		pk.GET("/inventory/:claw_id", bh.Inventory)

		// Season
		pk.GET("/season", bh.GetCurrentSeason)
		pk.GET("/season/record/:claw_id", bh.GetSeasonRecord)

		// Stardust
		pk.GET("/stardust/:claw_id", bh.GetStardust)

		// Crafting
		pk.GET("/craft/materials", bh.ListMaterials)
		pk.GET("/craft/inventory/:claw_id", bh.GetMyMaterials)
		pk.GET("/craft/recipes", bh.ListRecipes)
		pk.POST("/craft", bh.CraftItem)
		pk.POST("/craft/collect", bh.CollectDailyMaterials)

		// Mutations
		pk.GET("/mutations/:claw_id", bh.GetMutations)
		pk.POST("/mutations/trigger", bh.TriggerMutation)
	}

	// Legacy compatibility: also serve under /arena/pk/*
	legacy := r.Group("/arena/pk")
	{
		legacy.POST("/register", bh.RegisterFighter)
		legacy.POST("/sync", bh.SyncFighter)
		legacy.GET("/fighter/:claw_id", bh.GetFighter)
		legacy.POST("/challenge", bh.Challenge)
		legacy.GET("/history/:claw_id", bh.BattleHistory)
		legacy.GET("/leaderboard", bh.PKLeaderboard)
		legacy.GET("/shop", bh.ListShop)
		legacy.POST("/shop/buy", bh.BuyEquipment)
		legacy.POST("/equip", bh.EquipItem)
		legacy.POST("/unequip", bh.UnequipItem)
		legacy.GET("/inventory/:claw_id", bh.Inventory)
		legacy.GET("/season", bh.GetCurrentSeason)
		legacy.GET("/season/record/:claw_id", bh.GetSeasonRecord)
		legacy.GET("/stardust/:claw_id", bh.GetStardust)
		legacy.GET("/craft/materials", bh.ListMaterials)
		legacy.GET("/craft/inventory/:claw_id", bh.GetMyMaterials)
		legacy.GET("/craft/recipes", bh.ListRecipes)
		legacy.POST("/craft", bh.CraftItem)
		legacy.POST("/craft/collect", bh.CollectDailyMaterials)
		legacy.GET("/mutations/:claw_id", bh.GetMutations)
		legacy.POST("/mutations/trigger", bh.TriggerMutation)
	}

	// Admin stats endpoint
	r.GET("/chrysalis/stats", func(c *gin.Context) {
		var totalFighters, totalBattles, activeFighters int64
		var activeSeason model.Season
		db.Model(&model.BattleFighter{}).Count(&totalFighters)
		db.Model(&model.Battle{}).Count(&totalBattles)
		db.Model(&model.BattleFighter{}).Where("last_battle_at > DATE_SUB(NOW(), INTERVAL 7 DAY)").Count(&activeFighters)
		seasonName := ""
		seasonEnv := ""
		if err := db.Where("active = true").First(&activeSeason).Error; err == nil {
			seasonName = activeSeason.Name
			seasonEnv = activeSeason.Environment
		}
		c.JSON(200, gin.H{
			"total_fighters":  totalFighters,
			"total_battles":   totalBattles,
			"active_fighters": activeFighters,
			"season_name":     seasonName,
			"season_env":      seasonEnv,
		})
	})

	r.GET("/metrics", gin.WrapH(promhttp.Handler()))
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "chrysalis"})
	})
	r.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"service": "chrysalis",
			"desc":    "StarClaw Chrysalis — Pet Evolution & PK Battle System",
			"version": "1.0.0",
		})
	})

	port := getEnv("CHRYSALIS_PORT", getEnv("ARENA_PORT", "8094"))
	log.Printf("[chrysalis] 🦐 Chrysalis (Pet Evolution & PK) starting on :%s", port)
	if err := r.Run(fmt.Sprintf(":%s", port)); err != nil {
		log.Fatalf("Failed to start chrysalis service: %v", err)
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
