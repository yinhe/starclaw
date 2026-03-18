package database

import (
	"log"

	"github.com/yinhe/starclaw/internal/model"
	"gorm.io/gorm"
)

// modelSeed defines a model to be seeded
type modelSeed struct {
	Provider    string
	ModelName   string
	DisplayName string
	MaxTokens   int
}

// allModels is the comprehensive list of Alibaba Cloud Bailian models
var allModels = []modelSeed{
	// ── 千问核心系列 ──
	{"qwen", "qwen-max", "千问Max (旗舰)", 32768},
	{"qwen", "qwen-max-latest", "千问Max Latest", 32768},
	{"qwen", "qwen-plus", "千问Plus (均衡)", 131072},
	{"qwen", "qwen-plus-latest", "千问Plus Latest", 131072},
	{"qwen", "qwen-turbo", "千问Turbo (快退", 131072},
	{"qwen", "qwen-turbo-latest", "千问Turbo Latest", 131072},
	{"qwen", "qwen-long", "千问Long (超长上下斀", 1000000},
	// ── 推理模型 ──
	{"qwen", "qwq-plus", "QwQ-Plus (深度推理)", 131072},
	{"qwen", "qwq-plus-latest", "QwQ-Plus Latest", 131072},
	{"qwen", "qwq-32b", "QwQ-32B (推理)", 32768},
	// ── 专业系列 ──
	{"qwen", "qwen-vl-max", "千问VL-Max (视觉旗舰)", 32768},
	{"qwen", "qwen-vl-max-latest", "千问VL-Max Latest", 32768},
	{"qwen", "qwen-vl-plus", "千问VL-Plus (视觉)", 32768},
	{"qwen", "qwen-vl-plus-latest", "千问VL-Plus Latest", 32768},
	{"qwen", "qwen-coder-plus", "千问Coder-Plus (代码)", 131072},
	{"qwen", "qwen-coder-plus-latest", "千问Coder-Plus Latest", 131072},
	{"qwen", "qwen-coder-turbo", "千问Coder-Turbo (代码快退", 131072},
	{"qwen", "qwen-coder-turbo-latest", "千问Coder-Turbo Latest", 131072},
	{"qwen", "qwen-math-plus", "千问Math-Plus (数学)", 4096},
	{"qwen", "qwen-math-plus-latest", "千问Math-Plus Latest", 4096},
	{"qwen", "qwen-math-turbo", "千问Math-Turbo (数学快退", 4096},
	{"qwen", "qwen-math-turbo-latest", "千问Math-Turbo Latest", 4096},
	{"qwen", "qwen-ocr", "千问OCR (文字识别)", 32768},
	{"qwen", "qwen-ocr-latest", "千问OCR Latest", 32768},
	// ── 千问Omni ──
	{"qwen", "qwen-omni-turbo", "千问Omni-Turbo (多模怀", 32768},
	{"qwen", "qwen-omni-turbo-latest", "千问Omni-Turbo Latest", 32768},
	// ── 开源版 Qwen3 ──
	{"qwen", "qwen3-235b-a22b", "Qwen3-235B-A22B (开源旗舀", 131072},
	{"qwen", "qwen3-32b", "Qwen3-32B", 131072},
	{"qwen", "qwen3-14b", "Qwen3-14B", 131072},
	{"qwen", "qwen3-8b", "Qwen3-8B", 131072},
	{"qwen", "qwen3-4b", "Qwen3-4B", 32768},
	{"qwen", "qwen3-1.7b", "Qwen3-1.7B", 32768},
	{"qwen", "qwen3-0.6b", "Qwen3-0.6B", 32768},
	// ── 开源版 Qwen2.5 ──
	{"qwen", "qwen2.5-72b-instruct", "Qwen2.5-72B", 131072},
	{"qwen", "qwen2.5-32b-instruct", "Qwen2.5-32B", 131072},
	{"qwen", "qwen2.5-14b-instruct", "Qwen2.5-14B", 131072},
	{"qwen", "qwen2.5-7b-instruct", "Qwen2.5-7B", 131072},
	{"qwen", "qwen2.5-3b-instruct", "Qwen2.5-3B", 32768},
	{"qwen", "qwen2.5-coder-32b-instruct", "Qwen2.5-Coder-32B", 131072},
	{"qwen", "qwen2.5-coder-14b-instruct", "Qwen2.5-Coder-14B", 131072},
	{"qwen", "qwen2.5-coder-7b-instruct", "Qwen2.5-Coder-7B", 131072},
	// ── 开源版视觉/VL ──
	{"qwen", "qwen-vl-ocr", "Qwen-VL-OCR (视觉OCR)", 32768},
	{"qwen", "qwen2.5-vl-72b-instruct", "Qwen2.5-VL-72B", 32768},
	{"qwen", "qwen2.5-vl-32b-instruct", "Qwen2.5-VL-32B", 32768},
	{"qwen", "qwen2.5-vl-7b-instruct", "Qwen2.5-VL-7B", 32768},
	{"qwen", "qwen2.5-vl-3b-instruct", "Qwen2.5-VL-3B", 32768},
	// ── 第三方模垀(via DashScope) ──
	{"qwen", "deepseek-v3", "DeepSeek-V3 (百炼)", 65536},
	{"qwen", "deepseek-r1", "DeepSeek-R1 (百炼)", 65536},
	{"qwen", "deepseek-r1-distill-qwen-32b", "DeepSeek-R1-Distill-32B", 32768},
	{"qwen", "deepseek-r1-distill-qwen-14b", "DeepSeek-R1-Distill-14B", 32768},
	{"qwen", "deepseek-r1-distill-qwen-7b", "DeepSeek-R1-Distill-7B", 32768},
}

// SeedModelsForUser creates model configs for a user if they don't exist yet.
// It reuses the API key and base_url from any existing qwen model config for that user.
func SeedModelsForUser(db *gorm.DB, userID string) {
	// Find existing qwen config to get API key
	var existing model.ModelConfig
	if err := db.Where("user_id = ? AND provider = ?", userID, "qwen").First(&existing).Error; err != nil {
		return // no existing config, can't seed without API key
	}
	if existing.APIKey == "" {
		return
	}

	seeded := 0
	for _, m := range allModels {
		var count int64
		db.Model(&model.ModelConfig{}).Where("user_id = ? AND model_name = ?", userID, m.ModelName).Count(&count)
		if count > 0 {
			continue // already exists
		}
		cfg := model.ModelConfig{
			UserID:      userID,
			Provider:    m.Provider,
			ModelName:   m.ModelName,
			DisplayName: m.DisplayName,
			APIKey:      existing.APIKey,
			BaseURL:     existing.BaseURL,
			MaxTokens:   m.MaxTokens,
			Temperature: 0.7,
			IsEnabled:   true,
		}
		if err := db.Create(&cfg).Error; err != nil {
			log.Printf("[SeedModels] Failed to create %s: %v", m.ModelName, err)
		} else {
			seeded++
		}
	}
	if seeded > 0 {
		log.Printf("[SeedModels] Created %d model configs for user %s", seeded, userID)
	}
}

// SeedStarAIModels ensures exactly ONE star-ai provider config exists for a user.
// Available models come from StarAIProvider.Models() — no need for individual model rows.
func SeedStarAIModels(db *gorm.DB, userID string) {
	// Fix legacy base_url missing api. prefix
	db.Model(&model.ModelConfig{}).
		Where("user_id = ? AND provider = ? AND base_url = ?", userID, "star-ai", "https://star-ai.net/v1").
		Update("base_url", "https://api.star-ai.net/v1")

	var count int64
	db.Model(&model.ModelConfig{}).Where("user_id = ? AND provider = ?", userID, "star-ai").Count(&count)
	if count == 1 {
		return // already has exactly one, nothing to do
	}

	if count > 1 {
		// Deduplicate: keep the first one, delete the rest
		var configs []model.ModelConfig
		db.Where("user_id = ? AND provider = ?", userID, "star-ai").Order("created_at ASC").Find(&configs)
		for i := 1; i < len(configs); i++ {
			db.Delete(&configs[i])
		}
		log.Printf("[SeedStarAI] Deduplicated star-ai configs for user %s: kept 1, removed %d", userID, len(configs)-1)
		return
	}

	// count == 0: create one
	cfg := model.ModelConfig{
		UserID:      userID,
		Provider:    "star-ai",
		ModelName:   "default",
		DisplayName: "Star AI",
		APIKey:      "claw-identity", // marker: use Ed25519 signature auth
		BaseURL:     "https://api.star-ai.net/v1",
		MaxTokens:   131072,
		Temperature: 0.7,
		IsEnabled:   true,
	}
	if err := db.Create(&cfg).Error; err != nil {
		log.Printf("[SeedStarAI] Failed to create star-ai config for user %s: %v", userID, err)
	} else {
		log.Printf("[SeedStarAI] Created star-ai config for user %s", userID)
	}
}

// SeedStarAIForAllUsers ensures one star-ai config per user (idempotent, deduplicates on every startup)
func SeedStarAIForAllUsers(db *gorm.DB) {
	var userIDs []string
	db.Model(&model.User{}).Pluck("id", &userIDs)
	for _, uid := range userIDs {
		SeedStarAIModels(db, uid)
	}
}

// SeedModelsForAllUsers seeds models for all users that have at least one qwen config
func SeedModelsForAllUsers(db *gorm.DB) {
	var userIDs []string
	db.Model(&model.ModelConfig{}).Where("provider = ?", "qwen").Distinct("user_id").Pluck("user_id", &userIDs)
	for _, uid := range userIDs {
		SeedModelsForUser(db, uid)
	}
}
