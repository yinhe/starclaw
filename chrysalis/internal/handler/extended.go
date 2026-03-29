package handler

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"starclaw.net/chrysalis/internal/engine"
	"starclaw.net/chrysalis/internal/model"
)

// ─── Season API ───

// GetCurrentSeason returns the active season.
// GET /arena/pk/season
func (h *BattleHandler) GetCurrentSeason(c *gin.Context) {
	var season model.Season
	if err := h.db.Where("active = true").First(&season).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"season": nil, "message": "no active season"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"season": season})
}

// GetSeasonRecord returns a fighter's season record.
// GET /arena/pk/season/record/:claw_id
func (h *BattleHandler) GetSeasonRecord(c *gin.Context) {
	clawID := c.Param("claw_id")

	var season model.Season
	if err := h.db.Where("active = true").First(&season).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"record": nil})
		return
	}

	var record model.SeasonRecord
	if err := h.db.Where("season_id = ? AND claw_id = ?", season.ID, clawID).First(&record).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"record": nil, "season": season})
		return
	}

	c.JSON(http.StatusOK, gin.H{"record": record, "season": season})
}

// ─── Stardust API ───

// GetStardust returns a node's stardust balance.
// GET /arena/pk/stardust/:claw_id
func (h *BattleHandler) GetStardust(c *gin.Context) {
	clawID := c.Param("claw_id")

	var acct model.StardustAccount
	if err := h.db.Where("claw_id = ?", clawID).First(&acct).Error; err != nil {
		// Create account on first access
		acct = model.StardustAccount{ClawID: clawID}
		h.db.Create(&acct)
	}

	var txns []model.StardustTransaction
	h.db.Where("claw_id = ?", clawID).Order("created_at DESC").Limit(20).Find(&txns)

	c.JSON(http.StatusOK, gin.H{"account": acct, "transactions": txns})
}

// ─── Crafting API ───

// ListMaterials returns material definitions.
// GET /arena/pk/craft/materials
func (h *BattleHandler) ListMaterials(c *gin.Context) {
	var defs []model.MaterialDef
	h.db.Find(&defs)
	c.JSON(http.StatusOK, gin.H{"materials": defs})
}

// GetMyMaterials returns owned materials for a claw node.
// GET /arena/pk/craft/inventory/:claw_id
func (h *BattleHandler) GetMyMaterials(c *gin.Context) {
	clawID := c.Param("claw_id")

	type MatWithDef struct {
		model.CraftMaterial
		Name   string `json:"name"`
		Icon   string `json:"icon"`
		Rarity string `json:"rarity"`
	}

	var items []MatWithDef
	h.db.Table("craft_materials").
		Select("craft_materials.*, material_defs.name, material_defs.icon, material_defs.rarity").
		Joins("LEFT JOIN material_defs ON material_defs.id = craft_materials.material_id").
		Where("craft_materials.claw_id = ? AND craft_materials.quantity > 0", clawID).
		Find(&items)

	c.JSON(http.StatusOK, gin.H{"materials": items})
}

// ListRecipes returns available crafting recipes.
// GET /arena/pk/craft/recipes
func (h *BattleHandler) ListRecipes(c *gin.Context) {
	var recipes []model.CraftRecipe
	h.db.Find(&recipes)
	c.JSON(http.StatusOK, gin.H{"recipes": recipes})
}

// CraftItem crafts an equipment from materials + stardust.
// POST /arena/pk/craft
func (h *BattleHandler) CraftItem(c *gin.Context) {
	var req struct {
		ClawID   string `json:"claw_id" binding:"required"`
		RecipeID string `json:"recipe_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var recipe model.CraftRecipe
	if err := h.db.First(&recipe, "id = ?", req.RecipeID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "recipe not found"})
		return
	}

	// Check fighter level requirement
	if recipe.LevelReq > 0 {
		var fighter model.BattleFighter
		if err := h.db.Where("claw_id = ?", req.ClawID).First(&fighter).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "fighter not registered"})
			return
		}
		if fighter.Level < recipe.LevelReq {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("需要 Lv.%d (当前 Lv.%d)", recipe.LevelReq, fighter.Level)})
			return
		}
	}

	// Parse materials
	type MatReq struct {
		MaterialID string `json:"material_id"`
		Quantity   int    `json:"quantity"`
	}
	var matReqs []MatReq
	json.Unmarshal([]byte(recipe.Materials), &matReqs)

	// Check and deduct materials (in transaction)
	err := h.db.Transaction(func(tx *gorm.DB) error {
		for _, mr := range matReqs {
			var owned model.CraftMaterial
			if err := tx.Where("claw_id = ? AND material_id = ?", req.ClawID, mr.MaterialID).First(&owned).Error; err != nil {
				return fmt.Errorf("缺少材料: %s", mr.MaterialID)
			}
			if owned.Quantity < mr.Quantity {
				return fmt.Errorf("材料不足: %s (需要 %d, 拥有 %d)", mr.MaterialID, mr.Quantity, owned.Quantity)
			}
			tx.Model(&owned).Update("quantity", gorm.Expr("quantity - ?", mr.Quantity))
		}
		return nil
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Deduct stardust
	if recipe.DustCost > 0 {
		if err := h.deductStardust(req.ClawID, int64(recipe.DustCost), "craft_spend", fmt.Sprintf("打造%s", recipe.ResultName)); err != nil {
			c.JSON(http.StatusPaymentRequired, gin.H{"error": err.Error()})
			return
		}
	}

	// Create the equipment with ±15% stat variance for crafted items
	var def model.EquipmentDef
	h.db.First(&def, "id = ?", recipe.ResultID)

	inst := model.EquipmentInstance{
		ClawID:   req.ClawID,
		DefID:    def.ID,
		BonusHP:  applyVariance(def.BonusHP),
		BonusATK: applyVariance(def.BonusATK),
		BonusDEF: applyVariance(def.BonusDEF),
		BonusSPD: applyVariance(def.BonusSPD),
	}
	h.db.Create(&inst)

	c.JSON(http.StatusCreated, gin.H{"item": inst, "def": def, "recipe": recipe})
}

// applyVariance adds ±15% random variance to a stat.
func applyVariance(base int) int {
	if base <= 0 {
		return 0
	}
	variance := float64(base) * 0.15
	result := float64(base) + (rand.Float64()*2-1)*variance
	if result < 1 {
		result = 1
	}
	return int(result)
}

// CollectDailyMaterials grants daily free materials.
// POST /arena/pk/craft/collect
func (h *BattleHandler) CollectDailyMaterials(c *gin.Context) {
	var req struct {
		ClawID string `json:"claw_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Grant 1-3 common materials + chance of uncommon
	grants := []struct {
		matID string
		qty   int
	}{
		{"mat-shell", 1 + rand.Intn(3)},
		{"mat-claw", 1 + rand.Intn(2)},
	}
	// 30% chance of uncommon material
	if rand.Intn(100) < 30 {
		uncommon := []string{"mat-crystal", "mat-feather", "mat-stone"}
		grants = append(grants, struct {
			matID string
			qty   int
		}{uncommon[rand.Intn(len(uncommon))], 1})
	}
	// 10% chance of rare essence
	if rand.Intn(100) < 10 {
		grants = append(grants, struct {
			matID string
			qty   int
		}{"mat-essence", 1})
	}

	var collected []gin.H
	for _, g := range grants {
		var existing model.CraftMaterial
		if err := h.db.Where("claw_id = ? AND material_id = ?", req.ClawID, g.matID).First(&existing).Error; err != nil {
			existing = model.CraftMaterial{ClawID: req.ClawID, MaterialID: g.matID, Quantity: g.qty}
			h.db.Create(&existing)
		} else {
			h.db.Model(&existing).Update("quantity", gorm.Expr("quantity + ?", g.qty))
		}
		collected = append(collected, gin.H{"material_id": g.matID, "quantity": g.qty})
	}

	c.JSON(http.StatusOK, gin.H{"collected": collected})
}

// ─── Mutation API ───

// GetMutations returns all mutations for a fighter.
// GET /arena/pk/mutations/:claw_id
func (h *BattleHandler) GetMutations(c *gin.Context) {
	clawID := c.Param("claw_id")
	var mutations []model.Mutation
	h.db.Where("claw_id = ?", clawID).Order("triggered_at ASC").Find(&mutations)
	c.JSON(http.StatusOK, gin.H{"mutations": mutations})
}

// TriggerMutation attempts to trigger a mutation for a fighter (called on level up).
// POST /arena/pk/mutations/trigger
func (h *BattleHandler) TriggerMutation(c *gin.Context) {
	var req struct {
		ClawID string `json:"claw_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var fighter model.BattleFighter
	if err := h.db.Where("claw_id = ?", req.ClawID).First(&fighter).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "fighter not found"})
		return
	}

	// Filter eligible mutations
	var existing []model.Mutation
	h.db.Where("fighter_id = ?", fighter.ID).Find(&existing)
	existingCodes := make(map[string]bool)
	for _, m := range existing {
		existingCodes[m.Code] = true
	}

	var eligible []model.MutationDef
	for _, def := range model.MutationPool {
		if existingCodes[def.Code] {
			continue
		}
		if def.PathOnly != "" && def.PathOnly != fighter.EvolutionPath {
			continue
		}
		if def.MinLevel > 0 && fighter.Level < def.MinLevel {
			continue
		}
		eligible = append(eligible, def)
	}

	if len(eligible) == 0 {
		c.JSON(http.StatusOK, gin.H{"mutation": nil, "message": "no eligible mutations"})
		return
	}

	// Roll rarity: 60% common, 30% rare, 10% legendary
	roll := rand.Intn(100)
	targetRarity := "common"
	if roll >= 90 {
		targetRarity = "legendary"
	} else if roll >= 60 {
		targetRarity = "rare"
	}

	// Filter by target rarity, fallback to any
	var pool []model.MutationDef
	for _, d := range eligible {
		if d.Rarity == targetRarity {
			pool = append(pool, d)
		}
	}
	if len(pool) == 0 {
		pool = eligible
	}

	// Pick random from pool
	picked := pool[rand.Intn(len(pool))]

	mutation := model.Mutation{
		FighterID:   fighter.ID,
		ClawID:      req.ClawID,
		Code:        picked.Code,
		Name:        picked.Name,
		Desc:        picked.Desc,
		Rarity:      picked.Rarity,
		BonusHP:     picked.BonusHP,
		BonusATK:    picked.BonusATK,
		BonusDEF:    picked.BonusDEF,
		BonusSPD:    picked.BonusSPD,
		SpecialCode: picked.SpecialCode,
	}
	h.db.Create(&mutation)

	// Publish mutation event to Pheromone
	h.pheromone.Publish("chrysalis.mutation", engine.MutationEvent{
		ClawID:       req.ClawID,
		MutationName: picked.Name,
		Rarity:       picked.Rarity,
		Timestamp:    time.Now().UTC().Format("2006-01-02T15:04:05Z"),
	})

	c.JSON(http.StatusCreated, gin.H{"mutation": mutation})
}
