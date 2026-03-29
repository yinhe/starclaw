package handler

import (
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"starclaw.net/queen/api/internal/database"
	"starclaw.net/queen/api/internal/model"
)

// PartnerOptionHandler handles partner option pool investment APIs.
type PartnerOptionHandler struct{}

// POST /v1/partner/option/purchase — Partner buys star diamonds from option pool
// Validates: round limits, quota, partner identity, calculates new commission rate.
func (h *PartnerOptionHandler) Purchase(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	var req struct {
		Amount int64 `json:"amount" binding:"required"` // investment amount (分)
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "amount is required"})
		return
	}
	if req.Amount <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "amount must be positive"})
		return
	}

	db := database.DB

	// ── 1. Identify partner (city or team) ──
	partnerID, partnerType, partnerName := resolvePartnerFromUser(db, userID)
	if partnerID == "" {
		c.JSON(http.StatusForbidden, gin.H{"error": "您不是合伙人，无法购买期权"})
		return
	}

	// ── 2. Get current round from InvestorPool ──
	var pool model.InvestorPool
	if err := db.First(&pool).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "期权池未初始化"})
		return
	}
	if pool.Status != "active" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "期权池当前不可用"})
		return
	}

	roundName := pool.CurrentRound
	rc := model.GetPartnerRoundConfig(roundName)
	if rc == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("当前轮次 %s 不支持合伙人期权", roundName)})
		return
	}

	// ── 3. Check round limits ──
	minInvest, maxInvest := model.PartnerRoundLimits(roundName, partnerType)
	if req.Amount < minInvest {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("最低投资 ¥%.2f", float64(minInvest)/100),
			"min":   minInvest,
		})
		return
	}

	// Check cumulative investment this round against max (single-round cap)
	var existingTotal int64
	db.Model(&model.PartnerOptionInvestment{}).
		Where("partner_id = ? AND partner_type = ? AND round = ? AND status = ?",
			partnerID, partnerType, roundName, "completed").
		Select("COALESCE(SUM(amount), 0)").Scan(&existingTotal)

	if existingTotal+req.Amount > maxInvest {
		remaining := maxInvest - existingTotal
		if remaining <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("您在%s已达投资上限 ¥%.2f", rc.Label, float64(maxInvest)/100),
			})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{
			"error":     fmt.Sprintf("超出本轮上限，还可投 ¥%.2f", float64(remaining)/100),
			"remaining": remaining,
		})
		return
	}

	// ── 4. Check round quota ──
	price := pool.CalcPrice(rc.Price)
	shares := req.Amount / price
	if shares <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "投资金额不足以购买1份星钻"})
		return
	}

	// Check round quota from FundingRound table
	var round model.FundingRound
	if err := db.Where("round = ?", roundName).First(&round).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "轮次配置不存在"})
		return
	}
	if round.Status != "open" {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("%s 尚未开放或已售罄", rc.Label)})
		return
	}
	if round.SharesSold+shares > round.SharesQuota {
		remaining := round.SharesQuota - round.SharesSold
		c.JSON(http.StatusBadRequest, gin.H{
			"error":           fmt.Sprintf("%s 配额不足，剩余 %d 份", rc.Label, remaining),
			"remaining_quota": remaining,
		})
		return
	}

	// ── 5. Execute purchase in transaction ──
	profitCfg := LoadProfitConfig()
	var newRate float64
	investmentID := uuid.New().String()

	err := db.Transaction(func(tx *gorm.DB) error {
		// Lock pool
		var p model.InvestorPool
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&p).Error; err != nil {
			return err
		}

		// Update pool
		p.TotalShares += shares
		p.TotalRaised += req.Amount
		if err := tx.Save(&p).Error; err != nil {
			return err
		}

		// Update round
		tx.Model(&model.FundingRound{}).Where("round = ?", roundName).Updates(map[string]interface{}{
			"shares_sold":    gorm.Expr("shares_sold + ?", shares),
			"amount_raised":  gorm.Expr("amount_raised + ?", req.Amount),
			"investor_count": gorm.Expr("investor_count + 1"),
		})

		// Record investment
		// Calculate the commission rate this investment gives
		var allInvestments []model.PartnerOptionInvestment
		tx.Where("partner_id = ? AND partner_type = ? AND status = ?",
			partnerID, partnerType, "completed").Find(&allInvestments)

		// Add the new investment to the list for rate calculation
		newInv := model.PartnerOptionInvestment{
			ID:          investmentID,
			PartnerID:   partnerID,
			PartnerType: partnerType,
			Round:       roundName,
			Amount:      req.Amount,
			Shares:      shares,
			Price:       price,
			Status:      "completed",
		}
		allInvestments = append(allInvestments, newInv)
		newRate = model.CalcPartnerCommRate(allInvestments, partnerType, profitCfg.BaseCommRate, profitCfg.CityMaxRate, profitCfg.TeamMaxRate)
		newInv.CommRate = newRate

		if err := tx.Create(&newInv).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "购买失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":       "期权购买成功",
		"investment_id": investmentID,
		"partner_id":    partnerID,
		"partner_type":  partnerType,
		"partner_name":  partnerName,
		"round":         roundName,
		"amount":        req.Amount,
		"shares":        shares,
		"price":         price,
		"new_comm_rate": math.Round(newRate*10000) / 10000,
	})
}

// GET /v1/partner/option/me — Get current partner's option investment summary
func (h *PartnerOptionHandler) MyOptions(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	db := database.DB
	partnerID, partnerType, partnerName := resolvePartnerFromUser(db, userID)
	if partnerID == "" {
		c.JSON(http.StatusForbidden, gin.H{"error": "您不是合伙人"})
		return
	}

	// Load all investments
	var investments []model.PartnerOptionInvestment
	db.Where("partner_id = ? AND partner_type = ? AND status = ?", partnerID, partnerType, "completed").
		Order("created_at ASC").Find(&investments)

	// Calculate current commission rate
	profitCfg := LoadProfitConfig()
	commRate := model.CalcPartnerCommRate(investments, partnerType, profitCfg.BaseCommRate, profitCfg.CityMaxRate, profitCfg.TeamMaxRate)

	// Check transition status
	var legacyRate float64
	var transitionEnd *time.Time
	if partnerType == "city" {
		var cp model.CityPartner
		if db.Where("id = ?", partnerID).First(&cp).Error == nil {
			legacyRate = cp.LegacyCommRate
			transitionEnd = cp.TransitionEnd
		}
	} else {
		var tp model.TeamPartner
		if db.Where("id = ?", partnerID).First(&tp).Error == nil {
			legacyRate = tp.LegacyCommRate
			transitionEnd = tp.TransitionEnd
		}
	}

	inTransition := transitionEnd != nil && time.Now().Before(*transitionEnd)
	effectiveRate := commRate
	if inTransition && legacyRate > commRate {
		effectiveRate = legacyRate
	}

	// Aggregate by round
	type roundSummary struct {
		Round       string  `json:"round"`
		TotalAmount int64   `json:"total_amount"`
		TotalShares int64   `json:"total_shares"`
		CommRate    float64 `json:"comm_rate"`
	}
	roundMap := make(map[string]*roundSummary)
	var totalAmount, totalShares int64
	for _, inv := range investments {
		rs, ok := roundMap[inv.Round]
		if !ok {
			rs = &roundSummary{Round: inv.Round}
			roundMap[inv.Round] = rs
		}
		rs.TotalAmount += inv.Amount
		rs.TotalShares += inv.Shares
		totalAmount += inv.Amount
		totalShares += inv.Shares
	}
	// Calculate rate per round
	for rn, rs := range roundMap {
		_, maxInvest := model.PartnerRoundLimits(rn, partnerType)
		baseRate := profitCfg.BaseCommRate
		var rateRange float64
		if partnerType == "city" {
			rateRange = profitCfg.CityMaxRate - baseRate
		} else {
			rateRange = profitCfg.TeamMaxRate - baseRate
		}
		ratio := float64(rs.TotalAmount) / float64(maxInvest)
		if ratio > 1.0 {
			ratio = 1.0
		}
		rs.CommRate = baseRate + ratio*rateRange
	}

	rounds := make([]roundSummary, 0, len(roundMap))
	for _, rs := range roundMap {
		rounds = append(rounds, *rs)
	}

	// Current round info
	var pool model.InvestorPool
	db.First(&pool)
	currentRound := pool.CurrentRound
	_, currentMax := model.PartnerRoundLimits(currentRound, partnerType)
	var currentRoundInvested int64
	db.Model(&model.PartnerOptionInvestment{}).
		Where("partner_id = ? AND partner_type = ? AND round = ? AND status = ?",
			partnerID, partnerType, currentRound, "completed").
		Select("COALESCE(SUM(amount), 0)").Scan(&currentRoundInvested)

	c.JSON(http.StatusOK, gin.H{
		"partner_id":     partnerID,
		"partner_type":   partnerType,
		"partner_name":   partnerName,
		"effective_rate": math.Round(effectiveRate*10000) / 10000,
		"option_rate":    math.Round(commRate*10000) / 10000,
		"in_transition":  inTransition,
		"legacy_rate":    legacyRate,
		"total_invested": totalAmount,
		"total_shares":   totalShares,
		"rounds":         rounds,
		"current_round": gin.H{
			"round":     currentRound,
			"invested":  currentRoundInvested,
			"max":       currentMax,
			"remaining": currentMax - currentRoundInvested,
		},
	})
}

// resolvePartnerFromUser finds the partner (city or team) linked to a Queen user.
// Returns (partnerID, partnerType, partnerName). Empty if not a partner.
func resolvePartnerFromUser(db *gorm.DB, userID string) (string, string, string) {
	// Try city partner
	var cityPartner model.CityPartner
	if err := db.Where("user_id = ? AND status = ?", userID, "approved").First(&cityPartner).Error; err == nil {
		return cityPartner.ID, "city", cityPartner.Name
	}

	// Try team partner
	var teamPartner model.TeamPartner
	if err := db.Where("user_id = ? AND status = ?", userID, "active").First(&teamPartner).Error; err == nil {
		return teamPartner.ID, "team", teamPartner.Name
	}

	return "", "", ""
}
