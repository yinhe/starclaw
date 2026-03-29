package handler

import (
	"fmt"
	"math/rand"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"starclaw.net/chrysalis/internal/engine"
	"starclaw.net/chrysalis/internal/model"
)

type BattleHandler struct {
	db        *gorm.DB
	queen     *engine.QueenClient
	pheromone *engine.PheromonePublisher
}

func NewBattleHandler(db *gorm.DB, queen *engine.QueenClient) *BattleHandler {
	return &BattleHandler{db: db, queen: queen, pheromone: engine.NewPheromonePublisher()}
}

// RegisterFighter registers or updates a Claw node's pet for PK battles.
// POST /arena/pk/register
func (h *BattleHandler) RegisterFighter(c *gin.Context) {
	var req struct {
		ClawID        string `json:"claw_id" binding:"required"`
		Name          string `json:"name" binding:"required"`
		Level         int    `json:"level"`
		EvolutionPath string `json:"evolution_path"`
		FormCode      string `json:"form_code"`
		BaseHP        int    `json:"base_hp"`
		BaseATK       int    `json:"base_atk"`
		BaseDEF       int    `json:"base_def"`
		BaseSPD       int    `json:"base_spd"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var fighter model.BattleFighter
	err := h.db.Where("claw_id = ?", req.ClawID).First(&fighter).Error
	if err != nil {
		// Create new
		fighter = model.BattleFighter{
			ClawID:        req.ClawID,
			Name:          req.Name,
			Level:         req.Level,
			EvolutionPath: req.EvolutionPath,
			FormCode:      req.FormCode,
			BaseHP:        req.BaseHP,
			BaseATK:       req.BaseATK,
			BaseDEF:       req.BaseDEF,
			BaseSPD:       req.BaseSPD,
		}
		if fighter.Level < 1 {
			fighter.Level = 1
		}
		if fighter.BaseHP < 50 {
			fighter.BaseHP = 50
		}
		if fighter.BaseATK < 10 {
			fighter.BaseATK = 10
		}
		if fighter.BaseDEF < 10 {
			fighter.BaseDEF = 10
		}
		if fighter.BaseSPD < 10 {
			fighter.BaseSPD = 10
		}
		h.db.Create(&fighter)
		c.JSON(http.StatusCreated, gin.H{"fighter": fighter})
		return
	}

	// Update existing
	updates := map[string]interface{}{
		"name":           req.Name,
		"level":          req.Level,
		"evolution_path": req.EvolutionPath,
		"form_code":      req.FormCode,
		"base_hp":        req.BaseHP,
		"base_atk":       req.BaseATK,
		"base_def":       req.BaseDEF,
		"base_spd":       req.BaseSPD,
	}
	h.db.Model(&fighter).Updates(updates)
	h.db.First(&fighter, "id = ?", fighter.ID)
	c.JSON(http.StatusOK, gin.H{"fighter": fighter})
}

// SyncFighter updates a registered fighter's stats from Claw growth engine.
// Called by Claw on heartbeat or level-up. Only updates if fighter exists.
// POST /chrysalis/pk/sync
func (h *BattleHandler) SyncFighter(c *gin.Context) {
	var req struct {
		ClawID        string `json:"claw_id" binding:"required"`
		Name          string `json:"name"`
		Level         int    `json:"level"`
		EvolutionPath string `json:"evolution_path"`
		FormCode      string `json:"form_code"`
		BaseHP        int    `json:"base_hp"`
		BaseATK       int    `json:"base_atk"`
		BaseDEF       int    `json:"base_def"`
		BaseSPD       int    `json:"base_spd"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var fighter model.BattleFighter
	if err := h.db.Where("claw_id = ?", req.ClawID).First(&fighter).Error; err != nil {
		// Not registered yet — skip silently (fighter must register first via UI)
		c.JSON(http.StatusOK, gin.H{"synced": false, "reason": "not_registered"})
		return
	}

	// Only update fields that actually changed
	updates := map[string]interface{}{}
	if req.Level > fighter.Level {
		updates["level"] = req.Level
	}
	if req.Name != "" && req.Name != fighter.Name {
		updates["name"] = req.Name
	}
	if req.EvolutionPath != "" && req.EvolutionPath != fighter.EvolutionPath {
		updates["evolution_path"] = req.EvolutionPath
	}
	if req.FormCode != "" && req.FormCode != fighter.FormCode {
		updates["form_code"] = req.FormCode
	}
	if req.BaseHP > 0 {
		updates["base_hp"] = req.BaseHP
	}
	if req.BaseATK > 0 {
		updates["base_atk"] = req.BaseATK
	}
	if req.BaseDEF > 0 {
		updates["base_def"] = req.BaseDEF
	}
	if req.BaseSPD > 0 {
		updates["base_spd"] = req.BaseSPD
	}

	if len(updates) == 0 {
		c.JSON(http.StatusOK, gin.H{"synced": false, "reason": "no_changes"})
		return
	}

	h.db.Model(&fighter).Updates(updates)
	c.JSON(http.StatusOK, gin.H{"synced": true, "updated_fields": len(updates)})
}

// GetFighter returns a fighter's full profile including equipment.
// GET /arena/pk/fighter/:claw_id
func (h *BattleHandler) GetFighter(c *gin.Context) {
	clawID := c.Param("claw_id")
	var fighter model.BattleFighter
	if err := h.db.Where("claw_id = ?", clawID).First(&fighter).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "fighter not found"})
		return
	}

	// Load equipment
	var equipment []model.EquipmentInstance
	h.db.Where("claw_id = ?", clawID).Find(&equipment)

	c.JSON(http.StatusOK, gin.H{"fighter": fighter, "equipment": equipment})
}

// Challenge initiates a PK battle against another fighter.
// POST /arena/pk/challenge
func (h *BattleHandler) Challenge(c *gin.Context) {
	var req struct {
		ChallengerClawID string `json:"challenger_claw_id" binding:"required"`
		OpponentClawID   string `json:"opponent_claw_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.ChallengerClawID == req.OpponentClawID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot challenge yourself"})
		return
	}

	// Load both fighters
	var fighterA, fighterB model.BattleFighter
	if err := h.db.Where("claw_id = ?", req.ChallengerClawID).First(&fighterA).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "challenger not registered"})
		return
	}
	if err := h.db.Where("claw_id = ?", req.OpponentClawID).First(&fighterB).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "opponent not registered"})
		return
	}

	// Compute equipment bonuses
	eqA := h.computeEquipBonus(fighterA.ClawID)
	eqB := h.computeEquipBonus(fighterB.ClawID)

	// Build runtime fighters
	a := engine.NewFighter(fighterA.ID, fighterA.Name, fighterA.EvolutionPath, fighterA.Level,
		fighterA.BaseHP, fighterA.BaseATK, fighterA.BaseDEF, fighterA.BaseSPD,
		eqA.hp, eqA.atk, eqA.def, eqA.spd, eqA.crit)
	b := engine.NewFighter(fighterB.ID, fighterB.Name, fighterB.EvolutionPath, fighterB.Level,
		fighterB.BaseHP, fighterB.BaseATK, fighterB.BaseDEF, fighterB.BaseSPD,
		eqB.hp, eqB.atk, eqB.def, eqB.spd, eqB.crit)

	// Apply season environment buffs
	var season model.Season
	if err := h.db.Where("active = true").First(&season).Error; err == nil {
		sb := &engine.SeasonBuff{
			Environment: season.Environment,
			ATKBonus:    season.PathATKBonus,
			DEFBonus:    season.PathDEFBonus,
			SPDBonus:    season.PathSPDBonus,
		}
		a.ApplySeasonBuff(sb)
		b.ApplySeasonBuff(sb)
	}

	// Execute battle
	result := engine.ExecuteBattle(a, b)

	// ELO changes
	var eloA, eloB int
	if result.WinnerID != "" {
		if result.WinnerID == fighterA.ID {
			eloA, eloB = engine.CalcELOChange(fighterA.ELO, fighterB.ELO)
		} else {
			eloB, eloA = engine.CalcELOChange(fighterB.ELO, fighterA.ELO)
		}
	}

	// Update fighter stats
	h.db.Model(&fighterA).Updates(map[string]interface{}{
		"elo":            gorm.Expr("elo + ?", eloA),
		"last_battle_at": gorm.Expr("NOW()"),
	})
	h.db.Model(&fighterB).Updates(map[string]interface{}{
		"elo":            gorm.Expr("elo + ?", eloB),
		"last_battle_at": gorm.Expr("NOW()"),
	})

	if result.WinnerID == fighterA.ID {
		h.db.Model(&fighterA).Updates(map[string]interface{}{
			"win_count":  gorm.Expr("win_count + 1"),
			"win_streak": gorm.Expr("win_streak + 1"),
		})
		h.db.Model(&fighterB).Updates(map[string]interface{}{
			"lose_count": gorm.Expr("lose_count + 1"),
			"win_streak": 0,
		})
	} else if result.WinnerID == fighterB.ID {
		h.db.Model(&fighterB).Updates(map[string]interface{}{
			"win_count":  gorm.Expr("win_count + 1"),
			"win_streak": gorm.Expr("win_streak + 1"),
		})
		h.db.Model(&fighterA).Updates(map[string]interface{}{
			"lose_count": gorm.Expr("lose_count + 1"),
			"win_streak": 0,
		})
	}

	// Save battle record
	battle := model.Battle{
		FighterAID:   fighterA.ID,
		FighterAName: fighterA.Name,
		FighterAPath: fighterA.EvolutionPath,
		FighterALv:   fighterA.Level,
		FighterBID:   fighterB.ID,
		FighterBName: fighterB.Name,
		FighterBPath: fighterB.EvolutionPath,
		FighterBLv:   fighterB.Level,
		WinnerID:     result.WinnerID,
		Rounds:       result.Rounds,
		ELOChangeA:   eloA,
		ELOChangeB:   eloB,
		Log:          engine.SerializeLog(result.Log),
	}
	h.db.Create(&battle)

	// Battle material drops for both fighters
	dropsA := h.grantBattleDrops(fighterA.ClawID, result.WinnerID == fighterA.ID)
	dropsB := h.grantBattleDrops(fighterB.ClawID, result.WinnerID == fighterB.ID)

	// Publish battle event to Pheromone
	loserID := fighterB.ID
	if result.WinnerID == fighterB.ID {
		loserID = fighterA.ID
	}
	h.pheromone.Publish("chrysalis.battle_complete", engine.BattleCompleteEvent{
		ChallengerID: fighterA.ClawID,
		OpponentID:   fighterB.ClawID,
		WinnerID:     result.WinnerID,
		LoserID:      loserID,
		Rounds:       result.Rounds,
		Timestamp:    battle.CreatedAt.Format("2006-01-02T15:04:05Z"),
	})

	c.JSON(http.StatusOK, gin.H{
		"battle":     battle,
		"result":     result,
		"elo_change": gin.H{"a": eloA, "b": eloB},
		"drops":      gin.H{"a": dropsA, "b": dropsB},
	})
}

// grantBattleDrops gives materials after a battle. Winners get more.
func (h *BattleHandler) grantBattleDrops(clawID string, isWinner bool) []gin.H {
	// Common drop pool: shell + claw (always)
	commonPool := []struct {
		matID  string
		maxQty int
	}{
		{"mat-shell", 2},
		{"mat-claw", 2},
	}
	// Uncommon pool: crystal/feather/stone (winner: 40%, loser: 15%)
	uncommonPool := []string{"mat-crystal", "mat-feather", "mat-stone"}

	var drops []gin.H

	// Always grant 1-2 common materials
	for _, cm := range commonPool {
		qty := 1
		if isWinner && rand.Intn(2) == 0 {
			qty = 2
		}
		h.addMaterial(clawID, cm.matID, qty)
		drops = append(drops, gin.H{"material_id": cm.matID, "quantity": qty})
	}

	// Uncommon chance
	uncommonChance := 15
	if isWinner {
		uncommonChance = 40
	}
	if rand.Intn(100) < uncommonChance {
		matID := uncommonPool[rand.Intn(len(uncommonPool))]
		h.addMaterial(clawID, matID, 1)
		drops = append(drops, gin.H{"material_id": matID, "quantity": 1})
	}

	// Rare essence: winner only, 10% chance
	if isWinner && rand.Intn(100) < 10 {
		h.addMaterial(clawID, "mat-essence", 1)
		drops = append(drops, gin.H{"material_id": "mat-essence", "quantity": 1})
	}

	return drops
}

// addMaterial adds quantity to existing material or creates new record.
func (h *BattleHandler) addMaterial(clawID, materialID string, qty int) {
	var existing model.CraftMaterial
	if err := h.db.Where("claw_id = ? AND material_id = ?", clawID, materialID).First(&existing).Error; err != nil {
		existing = model.CraftMaterial{ClawID: clawID, MaterialID: materialID, Quantity: qty}
		h.db.Create(&existing)
	} else {
		h.db.Model(&existing).Update("quantity", gorm.Expr("quantity + ?", qty))
	}
}

// BattleHistory returns recent battles for a fighter.
// GET /arena/pk/history/:claw_id
func (h *BattleHandler) BattleHistory(c *gin.Context) {
	clawID := c.Param("claw_id")

	var fighter model.BattleFighter
	if err := h.db.Where("claw_id = ?", clawID).First(&fighter).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "fighter not found"})
		return
	}

	var battles []model.Battle
	h.db.Where("fighter_a_id = ? OR fighter_b_id = ?", fighter.ID, fighter.ID).
		Order("created_at DESC").Limit(20).Find(&battles)

	c.JSON(http.StatusOK, gin.H{"battles": battles, "total": len(battles)})
}

// PKLeaderboard returns top fighters by ELO.
// GET /arena/pk/leaderboard
func (h *BattleHandler) PKLeaderboard(c *gin.Context) {
	var fighters []model.BattleFighter
	h.db.Order("elo DESC").Limit(50).Find(&fighters)
	c.JSON(http.StatusOK, gin.H{"leaderboard": fighters})
}

// ListShop returns available equipment definitions.
// GET /arena/pk/shop
func (h *BattleHandler) ListShop(c *gin.Context) {
	var defs []model.EquipmentDef
	h.db.Order("slot, quality, name").Find(&defs)
	c.JSON(http.StatusOK, gin.H{"items": defs})
}

// BuyEquipment purchases an equipment item (deducts star energy / stardust).
// POST /arena/pk/shop/buy
func (h *BattleHandler) BuyEquipment(c *gin.Context) {
	var req struct {
		ClawID string `json:"claw_id" binding:"required"`
		DefID  string `json:"def_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var def model.EquipmentDef
	if err := h.db.First(&def, "id = ?", req.DefID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "equipment not found"})
		return
	}

	if def.PriceStar == 0 && def.PriceDust == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "item not available for purchase"})
		return
	}

	// Deduct star energy via Queen billing
	if def.PriceStar > 0 {
		_, err := h.queen.ConsumeStarEnergy(req.ClawID, int64(def.PriceStar), fmt.Sprintf("arena_buy:%s", def.Name))
		if err != nil {
			c.JSON(http.StatusPaymentRequired, gin.H{"error": err.Error()})
			return
		}
		// Co-generate stardust (spend N star → earn N/CoGenRatio stardust)
		h.coGenStardust(req.ClawID, int64(def.PriceStar), "cogen", fmt.Sprintf("购买%s伴生", def.Name))
	}

	// Deduct stardust for purple+ items
	if def.PriceDust > 0 {
		if err := h.deductStardust(req.ClawID, int64(def.PriceDust), "shop_spend", fmt.Sprintf("购买%s", def.Name)); err != nil {
			c.JSON(http.StatusPaymentRequired, gin.H{"error": err.Error()})
			return
		}
	}

	inst := model.EquipmentInstance{
		ClawID:   req.ClawID,
		DefID:    def.ID,
		BonusHP:  def.BonusHP,
		BonusATK: def.BonusATK,
		BonusDEF: def.BonusDEF,
		BonusSPD: def.BonusSPD,
	}
	h.db.Create(&inst)

	c.JSON(http.StatusCreated, gin.H{"item": inst, "def": def})
}

// coGenStardust generates stardust from star energy spending.
func (h *BattleHandler) coGenStardust(clawID string, starSpent int64, txType, remark string) {
	dustEarned := starSpent / int64(model.CoGenRatio)
	if dustEarned <= 0 {
		return
	}

	// Ensure account exists
	var acct model.StardustAccount
	if err := h.db.Where("claw_id = ?", clawID).First(&acct).Error; err != nil {
		acct = model.StardustAccount{ClawID: clawID}
		h.db.Create(&acct)
	}

	h.db.Model(&acct).Updates(map[string]interface{}{
		"balance":  gorm.Expr("balance + ?", dustEarned),
		"total_in": gorm.Expr("total_in + ?", dustEarned),
	})
	h.db.Create(&model.StardustTransaction{
		ClawID: clawID, Amount: dustEarned, Type: txType, Remark: remark,
	})
}

// deductStardust deducts stardust from a node's account.
func (h *BattleHandler) deductStardust(clawID string, amount int64, txType, remark string) error {
	var acct model.StardustAccount
	if err := h.db.Where("claw_id = ?", clawID).First(&acct).Error; err != nil {
		return fmt.Errorf("星尘账户不存在")
	}
	if acct.Balance < amount {
		return fmt.Errorf("星尘不足 (需要 %d, 余额 %d)", amount, acct.Balance)
	}

	h.db.Model(&acct).Updates(map[string]interface{}{
		"balance":   gorm.Expr("balance - ?", amount),
		"total_out": gorm.Expr("total_out + ?", amount),
	})
	h.db.Create(&model.StardustTransaction{
		ClawID: clawID, Amount: -amount, Type: txType, Remark: remark,
	})
	return nil
}

// EquipItem equips an owned item to a fighter slot.
// POST /arena/pk/equip
func (h *BattleHandler) EquipItem(c *gin.Context) {
	var req struct {
		ClawID string `json:"claw_id" binding:"required"`
		ItemID string `json:"item_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Verify ownership
	var inst model.EquipmentInstance
	if err := h.db.Where("id = ? AND claw_id = ?", req.ItemID, req.ClawID).First(&inst).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "item not found or not owned"})
		return
	}

	// Get the def to know slot
	var def model.EquipmentDef
	h.db.First(&def, "id = ?", inst.DefID)

	// Get or verify fighter exists
	var fighter model.BattleFighter
	if err := h.db.Where("claw_id = ?", req.ClawID).First(&fighter).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "fighter not registered"})
		return
	}

	// Check path restriction
	if def.PathOnly != "" && def.PathOnly != fighter.EvolutionPath {
		c.JSON(http.StatusBadRequest, gin.H{"error": "item is restricted to " + def.PathOnly + " path"})
		return
	}

	// Unequip current item in that slot
	slotField := ""
	switch def.Slot {
	case "weapon":
		slotField = "weapon_id"
		if fighter.WeaponID != "" {
			h.db.Model(&model.EquipmentInstance{}).Where("id = ?", fighter.WeaponID).Update("equipped", false)
		}
	case "armor":
		slotField = "armor_id"
		if fighter.ArmorID != "" {
			h.db.Model(&model.EquipmentInstance{}).Where("id = ?", fighter.ArmorID).Update("equipped", false)
		}
	case "trinket":
		slotField = "trinket_id"
		if fighter.TrinketID != "" {
			h.db.Model(&model.EquipmentInstance{}).Where("id = ?", fighter.TrinketID).Update("equipped", false)
		}
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "unknown slot"})
		return
	}

	// Equip new item
	h.db.Model(&inst).Update("equipped", true)
	h.db.Model(&fighter).Update(slotField, inst.ID)

	c.JSON(http.StatusOK, gin.H{"equipped": def.Slot, "item": inst})
}

// UnequipItem removes an equipped item.
// POST /arena/pk/unequip
func (h *BattleHandler) UnequipItem(c *gin.Context) {
	var req struct {
		ClawID string `json:"claw_id" binding:"required"`
		Slot   string `json:"slot" binding:"required"` // weapon / armor / trinket
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var fighter model.BattleFighter
	if err := h.db.Where("claw_id = ?", req.ClawID).First(&fighter).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "fighter not registered"})
		return
	}

	var itemID string
	slotField := ""
	switch req.Slot {
	case "weapon":
		itemID = fighter.WeaponID
		slotField = "weapon_id"
	case "armor":
		itemID = fighter.ArmorID
		slotField = "armor_id"
	case "trinket":
		itemID = fighter.TrinketID
		slotField = "trinket_id"
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid slot"})
		return
	}

	if itemID != "" {
		h.db.Model(&model.EquipmentInstance{}).Where("id = ?", itemID).Update("equipped", false)
	}
	h.db.Model(&fighter).Update(slotField, "")

	c.JSON(http.StatusOK, gin.H{"unequipped": req.Slot})
}

// Inventory returns all owned equipment for a claw node.
// GET /arena/pk/inventory/:claw_id
func (h *BattleHandler) Inventory(c *gin.Context) {
	clawID := c.Param("claw_id")

	type ItemWithDef struct {
		model.EquipmentInstance
		DefName     string `json:"def_name"`
		DefSlot     string `json:"def_slot"`
		DefQuality  string `json:"def_quality"`
		SpecialDesc string `json:"special_desc"`
	}

	var items []ItemWithDef
	h.db.Table("equipment_instances").
		Select("equipment_instances.*, equipment_defs.name as def_name, equipment_defs.slot as def_slot, equipment_defs.quality as def_quality, equipment_defs.special_desc").
		Joins("LEFT JOIN equipment_defs ON equipment_defs.id = equipment_instances.def_id").
		Where("equipment_instances.claw_id = ?", clawID).
		Find(&items)

	c.JSON(http.StatusOK, gin.H{"items": items})
}

// equipBonus aggregates equipment stat bonuses.
type equipBonus struct {
	hp, atk, def, spd, crit int
}

func (h *BattleHandler) computeEquipBonus(clawID string) equipBonus {
	var bonus equipBonus
	var items []model.EquipmentInstance
	h.db.Where("claw_id = ? AND equipped = true", clawID).Find(&items)

	for _, it := range items {
		bonus.hp += it.BonusHP
		bonus.atk += it.BonusATK
		bonus.def += it.BonusDEF
		bonus.spd += it.BonusSPD

		// Get crit from def
		var def model.EquipmentDef
		if err := h.db.First(&def, "id = ?", it.DefID).Error; err == nil {
			bonus.crit += def.CritRateBonus
		}
	}
	return bonus
}
