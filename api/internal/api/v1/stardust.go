package v1

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/yinhe/starclaw/internal/model"
	"gorm.io/gorm"
)

type StardustHandler struct {
	db *gorm.DB
}

func NewStardustHandler(db *gorm.DB) *StardustHandler {
	return &StardustHandler{db: db}
}

// Balance returns the user's stardust balance.
func (h *StardustHandler) Balance(c *gin.Context) {
	userID := c.GetString("user_id")

	var growth model.NodeGrowth
	if err := h.db.Where("user_id = ?", userID).First(&growth).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"balance": 0})
		return
	}
	c.JSON(http.StatusOK, gin.H{"balance": growth.StardustBalance})
}

// Transactions returns stardust transaction history.
func (h *StardustHandler) Transactions(c *gin.Context) {
	userID := c.GetString("user_id")

	var txns []model.StardustTransaction
	h.db.Where("user_id = ?", userID).Order("created_at DESC").Limit(100).Find(&txns)
	c.JSON(http.StatusOK, gin.H{"transactions": txns})
}

// EnhanceHero spends stardust to boost the hero's stats.
func (h *StardustHandler) EnhanceHero(c *gin.Context) {
	userID := c.GetString("user_id")

	var req struct {
		Stat   string `json:"stat" binding:"required"` // hp, atk, def, spd
		Amount int    `json:"amount" binding:"required"`
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

	cost := req.Amount * 50 // 50 stardust per hero stat point
	if growth.StardustBalance < cost {
		c.JSON(http.StatusBadRequest, gin.H{"error": "insufficient stardust", "balance": growth.StardustBalance, "cost": cost})
		return
	}

	growth.StardustBalance -= cost
	h.db.Save(&growth)

	h.db.Create(&model.StardustTransaction{
		UserID:   userID,
		Amount:   -cost,
		Type:     "spend_enhance_hero",
		TargetID: growth.ID,
		Note:     req.Stat + " boost",
	})

	c.JSON(http.StatusOK, gin.H{"stardust_remaining": growth.StardustBalance, "stat": req.Stat, "amount": req.Amount})
}

// Hatch spends stardust to hatch a new swarm unit.
func (h *StardustHandler) Hatch(c *gin.Context) {
	userID := c.GetString("user_id")

	var req struct {
		Type string `json:"type"` // optional: financial, creative, social, engineer, scout, scholar
	}
	c.ShouldBindJSON(&req)

	var growth model.NodeGrowth
	if err := h.db.Where("user_id = ?", userID).First(&growth).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "growth profile not found"})
		return
	}

	cost := 100 // normal hatch
	if req.Type != "" {
		cost = 300 // targeted hatch
	}

	if growth.StardustBalance < cost {
		c.JSON(http.StatusBadRequest, gin.H{"error": "insufficient stardust", "balance": growth.StardustBalance, "cost": cost})
		return
	}

	unitType := model.SwarmGeneric
	if req.Type != "" {
		unitType = model.SwarmUnitType(req.Type)
	}

	unit := model.SwarmUnit{
		NodeID:    userID,
		AgentID:   "hatched-" + uuid.New().String()[:8],
		AgentName: "Hatched " + string(unitType),
		UnitType:  unitType,
		Level:     1,
		HP:        12,
		ATK:       12,
		DEF:       6,
		SPD:       12,
		Skill1:    "Basic Attack",
	}
	h.db.Create(&unit)

	growth.StardustBalance -= cost
	h.db.Save(&growth)

	h.db.Create(&model.StardustTransaction{
		UserID:   userID,
		Amount:   -cost,
		Type:     "spend_hatch",
		TargetID: unit.ID,
		Note:     "hatched " + string(unitType),
	})

	c.JSON(http.StatusOK, gin.H{"unit": unit, "stardust_remaining": growth.StardustBalance})
}
