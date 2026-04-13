package game

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw/internal/model"
	"gorm.io/gorm"
)

type GrowthChoiceHandler struct {
	db *gorm.DB
}

func NewGrowthChoiceHandler(db *gorm.DB) *GrowthChoiceHandler {
	return &GrowthChoiceHandler{db: db}
}

// ChoosePath handles Lv.5 evolution path selection.
// POST /v1/growth/choose-path
func (h *GrowthChoiceHandler) ChoosePath(c *gin.Context) {
	userID := c.GetString("user_id")

	var req struct {
		Path string `json:"path" binding:"required"` // ocean, terrain, sky, wisdom, ancient, symbiont
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate path
	validPaths := map[string]bool{
		"ocean": true, "terrain": true, "sky": true,
		"wisdom": true, "ancient": true, "symbiont": true,
	}
	if !validPaths[req.Path] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid path, must be one of: ocean, terrain, sky, wisdom, ancient, symbiont"})
		return
	}

	var growth model.NodeGrowth
	if err := h.db.Where("user_id = ?", userID).First(&growth).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "growth profile not found"})
		return
	}

	// Must be at least Lv.5 and still on larva path
	if growth.Level < 5 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "must be at least Lv.5 to choose a path"})
		return
	}
	if growth.EvolutionPath != model.PathLarva && growth.Generation == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path already chosen", "current_path": growth.EvolutionPath})
		return
	}

	growth.EvolutionPath = model.EvolutionPath(req.Path)
	h.db.Save(&growth)

	title, titleEN := model.GetTitle(growth.EvolutionPath, growth.Level)
	c.JSON(http.StatusOK, gin.H{
		"path":     req.Path,
		"title":    title,
		"title_en": titleEN,
		"message":  "evolution path chosen!",
	})
}

// ChooseRealm handles awakening 2-star realm path selection (仙/魔/妖).
// POST /v1/growth/choose-realm
func (h *GrowthChoiceHandler) ChooseRealm(c *gin.Context) {
	userID := c.GetString("user_id")

	var req struct {
		Realm string `json:"realm" binding:"required"` // immortal, demon, monster
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	validRealms := map[string]bool{
		"immortal": true, "demon": true, "monster": true,
	}
	if !validRealms[req.Realm] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid realm, must be: immortal, demon, or monster"})
		return
	}

	var growth model.NodeGrowth
	if err := h.db.Where("user_id = ?", userID).First(&growth).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "growth profile not found"})
		return
	}

	// Must be awakening 2+ stars
	if growth.AwakeningStars < 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "must have awakening 2+ stars to choose realm"})
		return
	}
	if growth.RealmPath != model.RealmNone {
		c.JSON(http.StatusBadRequest, gin.H{"error": "realm already chosen", "current_realm": growth.RealmPath})
		return
	}

	growth.RealmPath = model.RealmPath(req.Realm)
	growth.RealmLevel = 2 // starts at realm level 2 (仙徒/魔徒/妖修)
	h.db.Save(&growth)

	realmNames := map[string]string{
		"immortal": "仙道", "demon": "魔道", "monster": "妖道",
	}
	c.JSON(http.StatusOK, gin.H{
		"realm":       req.Realm,
		"realm_name":  realmNames[req.Realm],
		"realm_level": growth.RealmLevel,
		"message":     "realm path chosen!",
	})
}
