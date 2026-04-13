package game

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw/internal/model"
	"gorm.io/gorm"
)

// EndgameHandler manages post-Lv.50 systems: awakening, fusion, rebirth.
type EndgameHandler struct {
	db *gorm.DB
}

func NewEndgameHandler(db *gorm.DB) *EndgameHandler {
	return &EndgameHandler{db: db}
}

// AwakeningEXP defines EXP required for each awakening star.
var AwakeningEXP = map[int]int{
	1: 5000, 2: 10000, 3: 20000, 4: 50000, 5: 100000,
}

// Awaken attempts to advance to the next awakening star.
// POST /v1/growth/awaken
func (h *EndgameHandler) Awaken(c *gin.Context) {
	userID := c.GetString("user_id")

	var growth model.NodeGrowth
	if err := h.db.Where("user_id = ?", userID).First(&growth).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "growth profile not found"})
		return
	}

	if growth.Level < 50 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "must be Lv.50 to awaken"})
		return
	}

	nextStar := growth.AwakeningStars + 1
	required, ok := AwakeningEXP[nextStar]
	if !ok {
		// Beyond 5 stars: each subsequent star costs 100000 * star
		required = 100000 * nextStar
	}

	// For simplicity, check stardust as awakening currency
	if growth.StardustBalance < required/10 { // 1/10 of EXP as stardust cost
		c.JSON(http.StatusBadRequest, gin.H{
			"error":    "insufficient stardust for awakening",
			"required": required / 10,
			"balance":  growth.StardustBalance,
		})
		return
	}

	growth.AwakeningStars = nextStar
	growth.StardustBalance -= required / 10
	h.db.Save(&growth)

	h.db.Create(&model.StardustTransaction{
		UserID: userID,
		Amount: -(required / 10),
		Type:   "spend_awaken",
		Note:   fmt.Sprintf("awakening star %d", nextStar),
	})

	bonusPct := nextStar * 5
	c.JSON(http.StatusOK, gin.H{
		"awakening_stars": nextStar,
		"bonus_pct":       bonusPct,
		"message":         fmt.Sprintf("觉醒 %d 星！全属性 +%d%%", nextStar, bonusPct),
	})
}

// FusionPaths defines available cross-path fusions.
var FusionPaths = map[string]map[string]string{
	"terrain":  {"ocean": "棘龙 (Spinosaurus)", "sky": "翼龙骑士 (Dragon Rider)", "wisdom": "人龙 (Dragonborn)", "ancient": "远古霸王 (Primal Rex)", "symbiont": "森林巨兽 (Forest Titan)"},
	"ocean":    {"terrain": "两栖霸主 (Amphibious Lord)", "sky": "飞鱼龙 (Leviawing)", "wisdom": "克苏鲁 (Cthulhu)", "ancient": "远古海兽 (Ancient Leviathan)", "symbiont": "珊瑚巨人 (Coral Giant)"},
	"sky":      {"terrain": "陆空之王 (Sky Tyrant)", "ocean": "海燕之神 (Storm Petrel)", "wisdom": "天使 (Angel)", "ancient": "始祖鸟 (Archaeopteryx)", "symbiont": "风之精灵 (Wind Spirit)"},
	"wisdom":   {"terrain": "兽王 (Beast Master)", "ocean": "海神 (Poseidon)", "sky": "天神 (Zeus)", "ancient": "时间领主 (Time Lord)", "symbiont": "世界意志 (World Will)"},
	"ancient":  {"terrain": "远古暴君 (Ancient Tyrant)", "ocean": "远古海怪 (Kraken)", "sky": "远古飞龙 (Elder Dragon)", "wisdom": "远古智者 (Ancient Sage)", "symbiont": "远古守护 (Primordial Guardian)"},
	"symbiont": {"terrain": "大地之母 (Earth Mother)", "ocean": "海洋之心 (Ocean Heart)", "sky": "穹天之种 (Sky Seed)", "wisdom": "生命编织 (Life Weaver)", "ancient": "远古之树 (Ancient Tree)"},
}

// Fuse attempts a cross-path fusion.
// POST /v1/growth/fuse
func (h *EndgameHandler) Fuse(c *gin.Context) {
	userID := c.GetString("user_id")

	var req struct {
		TargetPath string `json:"target_path" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var growth model.NodeGrowth
	if err := h.db.Where("user_id = ?", userID).First(&growth).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "growth profile not found"})
		return
	}

	if growth.AwakeningStars < 3 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "need awakening 3+ stars for fusion"})
		return
	}

	currentPath := string(growth.EvolutionPath)
	fusions, ok := FusionPaths[currentPath]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "current path does not support fusion"})
		return
	}

	fusionName, ok := fusions[req.TargetPath]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid fusion target", "available": fusions})
		return
	}

	cost := 1000 // stardust cost for fusion
	if growth.StardustBalance < cost {
		c.JSON(http.StatusBadRequest, gin.H{"error": "insufficient stardust", "required": cost})
		return
	}

	growth.StardustBalance -= cost
	h.db.Save(&growth)

	h.db.Create(&model.StardustTransaction{
		UserID: userID,
		Amount: -cost,
		Type:   "spend_fusion",
		Note:   fusionName,
	})

	c.JSON(http.StatusOK, gin.H{
		"fusion_form": fusionName,
		"source_path": currentPath,
		"target_path": req.TargetPath,
		"message":     "跨路线融合成功！",
	})
}

// Rebirth resets to Lv.1 with a new path, keeping permanent bonuses.
// POST /v1/growth/rebirth
func (h *EndgameHandler) Rebirth(c *gin.Context) {
	userID := c.GetString("user_id")

	var req struct {
		NewPath string `json:"new_path" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var growth model.NodeGrowth
	if err := h.db.Where("user_id = ?", userID).First(&growth).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "growth profile not found"})
		return
	}

	if growth.AwakeningStars < 5 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "need awakening 5 stars for rebirth"})
		return
	}

	validPaths := map[string]bool{"ocean": true, "terrain": true, "sky": true, "wisdom": true, "ancient": true, "symbiont": true}
	if !validPaths[req.NewPath] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid path"})
		return
	}

	oldGen := growth.Generation
	oldPath := growth.EvolutionPath

	// Reset but keep permanent bonuses
	growth.Level = 1
	growth.EvolutionPath = model.EvolutionPath(req.NewPath)
	growth.AwakeningStars = 0
	growth.RealmPath = model.RealmNone
	growth.RealmLevel = 0
	growth.Generation = oldGen + 1
	h.db.Save(&growth)

	isOrigin := growth.Generation >= 6
	msg := fmt.Sprintf("转生第 %d 代！从 %s 转入 %s", growth.Generation, oldPath, req.NewPath)
	if isOrigin {
		msg = "🌟 万物之祖！六道轮回完成！你已超越一切！"
	}

	c.JSON(http.StatusOK, gin.H{
		"generation": growth.Generation,
		"new_path":   req.NewPath,
		"old_path":   string(oldPath),
		"is_origin":  isOrigin,
		"perm_bonus": growth.Generation * 3, // +3% per generation
		"message":    msg,
	})
}
