package v1

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw/internal/model"
	"gorm.io/gorm"
)

type SwarmHandler struct {
	db *gorm.DB
}

func NewSwarmHandler(db *gorm.DB) *SwarmHandler {
	return &SwarmHandler{db: db}
}

// List returns all swarm units for the current user's node.
func (h *SwarmHandler) List(c *gin.Context) {
	userID := c.GetString("user_id")

	var units []model.SwarmUnit
	h.db.Where("node_id = ?", userID).Order("level DESC, created_at ASC").Find(&units)

	// Calculate total power
	totalHP, totalATK, totalDEF, totalSPD := 0, 0, 0, 0
	for _, u := range units {
		totalHP += u.HP
		totalATK += u.ATK
		totalDEF += u.DEF
		totalSPD += u.SPD
	}
	countBonus := len(units) * 2 // +2% per unit

	c.JSON(http.StatusOK, gin.H{
		"units":       units,
		"count":       len(units),
		"total_hp":    totalHP,
		"total_atk":   totalATK,
		"total_def":   totalDEF,
		"total_spd":   totalSPD,
		"count_bonus": countBonus,
	})
}

// Get returns a single swarm unit by ID.
func (h *SwarmHandler) Get(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetString("user_id")

	var unit model.SwarmUnit
	if err := h.db.Where("id = ? AND node_id = ?", id, userID).First(&unit).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "unit not found"})
		return
	}
	c.JSON(http.StatusOK, unit)
}

// Invest allocates stardust to a swarm unit.
func (h *SwarmHandler) Invest(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetString("user_id")

	var req struct {
		Stat   string `json:"stat" binding:"required"` // hp, atk, def, spd
		Amount int    `json:"amount" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check stardust balance
	var growth model.NodeGrowth
	if err := h.db.Where("user_id = ?", userID).First(&growth).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "growth profile not found"})
		return
	}

	cost := req.Amount * 30 // 30 stardust per stat point
	if growth.StardustBalance < cost {
		c.JSON(http.StatusBadRequest, gin.H{"error": "insufficient stardust", "balance": growth.StardustBalance, "cost": cost})
		return
	}

	var unit model.SwarmUnit
	if err := h.db.Where("id = ? AND node_id = ?", id, userID).First(&unit).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "unit not found"})
		return
	}

	// Apply stat boost
	switch req.Stat {
	case "hp":
		unit.HP += req.Amount
	case "atk":
		unit.ATK += req.Amount
	case "def":
		unit.DEF += req.Amount
	case "spd":
		unit.SPD += req.Amount
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid stat, must be hp/atk/def/spd"})
		return
	}
	unit.StardustInvested += cost

	h.db.Save(&unit)
	h.db.Model(&growth).Update("stardust_balance", growth.StardustBalance-cost)

	// Record transaction
	h.db.Create(&model.StardustTransaction{
		UserID:   userID,
		Amount:   -cost,
		Type:     "spend_enhance_unit",
		TargetID: unit.ID,
		Note:     req.Stat + " +" + string(rune('0'+req.Amount)),
	})

	c.JSON(http.StatusOK, gin.H{"unit": unit, "stardust_remaining": growth.StardustBalance - cost})
}

// CreateFromAgent auto-creates a swarm unit when an Agent is created.
// Called internally, not exposed as API endpoint.
func CreateSwarmUnitFromAgent(db *gorm.DB, userID string, agent model.Agent) {
	unitType := model.ClassifySwarmUnit(agent.Tools)

	unit := model.SwarmUnit{
		NodeID:    userID,
		AgentID:   agent.ID,
		AgentName: agent.Name,
		UnitType:  unitType,
		Level:     1,
		Exp:       0,
		HP:        10,
		ATK:       10,
		DEF:       5,
		SPD:       10,
	}

	// Type-specific stat bonuses
	switch unitType {
	case model.SwarmFinancial:
		unit.ATK += 5
		unit.Skill1 = "经济压制"
	case model.SwarmCreative:
		unit.SPD += 5
		unit.Skill1 = "灵感爆发"
	case model.SwarmSocial:
		unit.DEF += 5
		unit.Skill1 = "外交斡旋"
	case model.SwarmEngineer:
		unit.DEF += 3
		unit.HP += 3
		unit.Skill1 = "代码壁垒"
	case model.SwarmScout:
		unit.SPD += 5
		unit.Skill1 = "先手打击"
	case model.SwarmScholar:
		unit.HP += 5
		unit.Skill1 = "知识护盾"
	}

	db.Create(&unit)
}
