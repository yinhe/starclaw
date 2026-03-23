package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/yinhe/starclaw-queen/api/internal/config"
	"github.com/yinhe/starclaw-queen/api/internal/database"
	"github.com/yinhe/starclaw-queen/api/internal/middleware"
	"github.com/yinhe/starclaw-queen/api/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type InvestorHandler struct{}

// ════════════════════════════════════════════════════════════
// Admin: Pool Management
// ════════════════════════════════════════════════════════════

// POST /admin/investor/pool/init — Initialize the investor pool + seed 5 funding rounds (one-time)
func (h *InvestorHandler) InitPool(c *gin.Context) {
	db := database.DB

	var existing model.InvestorPool
	if err := db.First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "investor pool already initialized", "pool": existing})
		return
	}

	pool := model.InvestorPool{
		ID:           uuid.New().String(),
		TotalShares:  0,
		MaxShares:    model.StarDiamondTotal, // 1亿 star diamonds
		PoolBalance:  0,
		SeedTotal:    model.StarDiamondTotal / 10, // 10% = 1000万 star diamonds for spore round
		SeedIssued:   0,
		CurrentRound: "spore",
		SharePrice:   20, // ¥0.20 per star diamond (spore floor price)
		Status:       "active",
	}
	db.Create(&pool)

	// Seed the 5 funding rounds (each = 10% of MaxShares)
	quota := pool.MaxShares / 10 // 10% = 1000万 shares per round
	for _, rc := range model.RoundConfig {
		status := "upcoming"
		if rc.Round == "spore" {
			status = "open"
		}
		db.Create(&model.FundingRound{
			ID:          uuid.New().String(),
			Round:       rc.Round,
			Label:       rc.Label,
			SharePrice:  rc.Price,
			Multiplier:  rc.Multiplier,
			SharesQuota: quota,
			Status:      status,
		})
	}

	log.Printf("[investor] Pool initialized: max=%d shares, 5 rounds seeded, seed round open", pool.MaxShares)
	c.JSON(http.StatusCreated, gin.H{"pool": pool, "rounds": model.RoundConfig})
}

// GET /admin/investor/pool — Get pool status + all rounds
func (h *InvestorHandler) GetPool(c *gin.Context) {
	db := database.DB
	var pool model.InvestorPool
	if err := db.First(&pool).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "investor pool not initialized"})
		return
	}

	var investorCount int64
	db.Model(&model.Investor{}).Where("status = ?", "active").Count(&investorCount)

	var rounds []model.FundingRound
	db.Order("multiplier ASC").Find(&rounds)

	c.JSON(http.StatusOK, gin.H{
		"pool":           pool,
		"rounds":         rounds,
		"investor_count": investorCount,
		"seed_remaining": pool.SeedTotal - pool.SeedIssued,
		"seed_pct":       fmt.Sprintf("%.1f%%", float64(pool.SeedIssued)/float64(pool.SeedTotal)*100),
	})
}

// POST /admin/investor/round/open — Open the next funding round
func (h *InvestorHandler) OpenRound(c *gin.Context) {
	var req struct {
		Round string `json:"round" binding:"required"` // seed / angel / a / b / c
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请指定轮次: spore / larva / zergling / overlord / queen"})
		return
	}

	db := database.DB
	err := db.Transaction(func(tx *gorm.DB) error {
		// Close any currently open round
		tx.Model(&model.FundingRound{}).Where("status = ?", "open").Updates(map[string]interface{}{
			"status":    "closed",
			"closed_at": time.Now(),
		})

		// Open the requested round
		var round model.FundingRound
		if err := tx.Where("round = ?", req.Round).First(&round).Error; err != nil {
			return fmt.Errorf("轮次 %s 不存在", req.Round)
		}
		if round.Status == "sold_out" {
			return fmt.Errorf("%s 已售罄", round.Label)
		}
		now := time.Now()
		tx.Model(&round).Updates(map[string]interface{}{
			"status":    "open",
			"opened_at": &now,
		})

		// Update pool's current round + dynamic price
		var pool model.InvestorPool
		if err := tx.First(&pool).Error; err != nil {
			return fmt.Errorf("合伙人池未初始化")
		}
		dynPrice := pool.CalcPrice(round.SharePrice)
		tx.Model(&pool).Updates(map[string]interface{}{
			"current_round": req.Round,
			"share_price":   dynPrice,
		})

		return nil
	})

	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	var round model.FundingRound
	db.Where("round = ?", req.Round).First(&round)
	log.Printf("[investor] Round opened: %s @ ¥%.2f/份", round.Label, float64(round.SharePrice)/100)
	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("%s 已开启", round.Label), "round": round})
}

// GET /admin/investor/rounds — List all funding rounds
func (h *InvestorHandler) ListRounds(c *gin.Context) {
	var rounds []model.FundingRound
	database.DB.Order("multiplier ASC").Find(&rounds)
	c.JSON(http.StatusOK, gin.H{"rounds": rounds})
}

// POST /admin/investor/seed — Issue seed round shares to a user
func (h *InvestorHandler) SeedGrant(c *gin.Context) {
	var req struct {
		UserID string `json:"user_id" binding:"required"`
		Shares int64  `json:"shares" binding:"required"`
		Name   string `json:"name"`
		Email  string `json:"email"`
		Phone  string `json:"phone"`
		Remark string `json:"remark"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数不完整"})
		return
	}
	if req.Shares <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "份额必须大于0"})
		return
	}

	db := database.DB
	err := db.Transaction(func(tx *gorm.DB) error {
		// Lock pool
		var pool model.InvestorPool
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&pool).Error; err != nil {
			return fmt.Errorf("investor pool not initialized")
		}
		if pool.Status != "active" {
			return fmt.Errorf("investor pool is %s", pool.Status)
		}

		// Check seed round budget (tracked in star diamond count, not fen)
		if pool.SeedIssued+req.Shares > pool.SeedTotal {
			return fmt.Errorf("seed round budget exceeded (remaining: %d star diamonds, need: %d)",
				pool.SeedTotal-pool.SeedIssued, req.Shares)
		}

		// Find or create investor
		var investor model.Investor
		if err := tx.Where("user_id = ?", req.UserID).First(&investor).Error; err != nil {
			investor = model.Investor{
				ID:       uuid.New().String(),
				UserID:   req.UserID,
				Name:     req.Name,
				Email:    req.Email,
				Phone:    req.Phone,
				Source:   "seed_grant",
				Status:   "active",
				JoinedAt: time.Now(),
			}
			tx.Create(&investor)
		}

		// Issue shares
		investor.Shares += req.Shares
		tx.Model(&investor).Update("shares", investor.Shares)

		// Update pool
		pool.TotalShares += req.Shares
		pool.SeedIssued += req.Shares
		tx.Save(&pool)

		// Record transaction
		remark := req.Remark
		if remark == "" {
			remark = fmt.Sprintf("空投 %d 星钻", req.Shares)
		}
		tx.Create(&model.InvestorTransaction{
			ID:            uuid.New().String(),
			InvestorID:    investor.ID,
			Type:          "seed_grant",
			Shares:        req.Shares,
			Amount:        0, // seed grant is free
			PricePerShare: 0,
			Remark:        remark,
		})

		log.Printf("[investor] SeedGrant: user=%s shares=%d pool_issued=%d/%d",
			req.UserID, req.Shares, pool.SeedIssued, pool.SeedTotal)
		return nil
	})

	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	var investor model.Investor
	db.Where("user_id = ?", req.UserID).First(&investor)
	c.JSON(http.StatusOK, gin.H{"investor": investor, "message": "空投成功"})
}

// GET /admin/investor/list — List all investors
func (h *InvestorHandler) ListInvestors(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	if page < 1 {
		page = 1
	}

	var total int64
	database.DB.Model(&model.Investor{}).Count(&total)

	var investors []model.Investor
	database.DB.Order("shares DESC").Offset((page - 1) * size).Limit(size).Find(&investors)

	c.JSON(http.StatusOK, gin.H{
		"investors": investors,
		"total":     total,
		"page":      page,
		"size":      size,
	})
}

// POST /admin/investor/distribute — Distribute dividends for a period
func (h *InvestorHandler) Distribute(c *gin.Context) {
	var req struct {
		Period string `json:"period"` // YYYY-MM, defaults to last month
	}
	c.ShouldBindJSON(&req)

	if req.Period == "" {
		req.Period = time.Now().AddDate(0, -1, 0).Format("2006-01")
	}

	db := database.DB

	// Check if already distributed for this period
	var existingCount int64
	db.Model(&model.InvestorDividend{}).Where("period = ?", req.Period).Count(&existingCount)
	if existingCount > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": fmt.Sprintf("dividends already distributed for %s", req.Period)})
		return
	}

	err := db.Transaction(func(tx *gorm.DB) error {
		// Lock pool
		var pool model.InvestorPool
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&pool).Error; err != nil {
			return fmt.Errorf("pool not initialized")
		}

		if pool.PoolBalance <= 0 {
			return fmt.Errorf("pool balance is zero, nothing to distribute")
		}
		if pool.TotalShares <= 0 {
			return fmt.Errorf("no shares issued")
		}

		distribAmount := pool.PoolBalance

		// Get all active investors
		var investors []model.Investor
		tx.Where("status = ? AND shares > 0", "active").Find(&investors)

		if len(investors) == 0 {
			return fmt.Errorf("no active investors with shares")
		}

		var totalDistributed int64
		now := time.Now()

		for _, inv := range investors {
			ratio := float64(inv.Shares) / float64(pool.TotalShares)
			amount := int64(math.Floor(float64(distribAmount) * ratio))
			if amount <= 0 {
				continue
			}
			totalDistributed += amount

			tx.Create(&model.InvestorDividend{
				ID:          uuid.New().String(),
				InvestorID:  inv.ID,
				Period:      req.Period,
				PoolDeposit: distribAmount,
				ShareRatio:  ratio,
				Amount:      amount,
				Shares:      inv.Shares,
				TotalShares: pool.TotalShares,
				Status:      "paid",
				PaidAt:      &now,
			})

			// Update investor total dividends
			tx.Model(&inv).Update("total_dividends", gorm.Expr("total_dividends + ?", amount))
		}

		// Update pool
		pool.PoolBalance -= totalDistributed
		pool.TotalDistrib += totalDistributed
		tx.Save(&pool)

		log.Printf("[investor] Distributed %d分 for %s to %d investors (pool_remaining=%d分)",
			totalDistributed, req.Period, len(investors), pool.PoolBalance)
		return nil
	})

	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	var dividends []model.InvestorDividend
	db.Where("period = ?", req.Period).Order("amount DESC").Find(&dividends)

	c.JSON(http.StatusOK, gin.H{
		"period":    req.Period,
		"dividends": dividends,
		"count":     len(dividends),
	})
}

// GET /admin/investor/dividends — List dividend records
func (h *InvestorHandler) ListDividends(c *gin.Context) {
	period := c.Query("period")
	investorID := c.Query("investor_id")

	query := database.DB.Model(&model.InvestorDividend{})
	if period != "" {
		query = query.Where("period = ?", period)
	}
	if investorID != "" {
		query = query.Where("investor_id = ?", investorID)
	}

	var dividends []model.InvestorDividend
	query.Order("created_at DESC").Limit(100).Find(&dividends)

	c.JSON(http.StatusOK, gin.H{"dividends": dividends})
}

// GET /admin/investor/deposits — List pool deposit records
func (h *InvestorHandler) ListDeposits(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "50"))
	if page < 1 {
		page = 1
	}

	var total int64
	database.DB.Model(&model.PoolDeposit{}).Count(&total)

	var deposits []model.PoolDeposit
	database.DB.Order("created_at DESC").Offset((page - 1) * size).Limit(size).Find(&deposits)

	c.JSON(http.StatusOK, gin.H{
		"deposits": deposits,
		"total":    total,
		"page":     page,
		"size":     size,
	})
}

// ════════════════════════════════════════════════════════════
// Internal: Called by Billing Gateway to deposit profit
// ════════════════════════════════════════════════════════════

// POST /internal/investor/deposit — Deposit 10% profit into investor pool
func (h *InvestorHandler) InternalDeposit(c *gin.Context) {
	var req struct {
		SourceType  string  `json:"source_type" binding:"required"` // tool_usage / token_usage
		SourceID    string  `json:"source_id"`
		Amount      int64   `json:"amount" binding:"required"` // deposit amount (分)
		MarginTotal int64   `json:"margin_total"`              // total margin this came from
		Rate        float64 `json:"rate"`                      // investor rate (default 0.10)
		ClawID      string  `json:"claw_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数不完整"})
		return
	}
	if req.Amount <= 0 {
		c.JSON(http.StatusOK, gin.H{"deposited": 0})
		return
	}
	if req.Rate == 0 {
		req.Rate = 0.10
	}

	db := database.DB
	err := db.Transaction(func(tx *gorm.DB) error {
		var pool model.InvestorPool
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&pool).Error; err != nil {
			// Pool not initialized — silently skip
			log.Printf("[investor] deposit skipped: pool not initialized")
			return nil
		}
		if pool.Status != "active" {
			return nil
		}

		pool.PoolBalance += req.Amount
		pool.TotalDeposited += req.Amount
		tx.Save(&pool)

		tx.Create(&model.PoolDeposit{
			ID:          uuid.New().String(),
			SourceType:  req.SourceType,
			SourceID:    req.SourceID,
			Amount:      req.Amount,
			MarginTotal: req.MarginTotal,
			Rate:        req.Rate,
			ClawID:      req.ClawID,
		})

		return nil
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"deposited": req.Amount})
}

// ════════════════════════════════════════════════════════════
// User-facing: Investor portal (authenticated users)
//
// Flow: Register → Sign Agreement (1/3/5yr) → Recharge (¥1万 min per txn)
//       → Cumulative ≥ ¥10万 → Activated (profit sharing starts)
//       → ¥100万 = 1% 份额 = 每日收益的 1%
// ════════════════════════════════════════════════════════════

const (
	MinRechargeFen      = 1000000  // ¥10,000 (1万) minimum per recharge
	ActivationThreshold = 10000000 // ¥100,000 (10万) cumulative to activate profit sharing
)

var validTerms = map[int]bool{1: true, 3: true, 5: true}

// GET /v1/investor/pool — Public Star Diamond (星钻) pool info
func (h *InvestorHandler) PublicPoolInfo(c *gin.Context) {
	var pool model.InvestorPool
	if err := database.DB.First(&pool).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "星钻池未启动"})
		return
	}
	var investorCount int64
	database.DB.Model(&model.Investor{}).Where("status = ? AND activated = ?", "active", true).Count(&investorCount)

	// Get current round for floor price
	var round model.FundingRound
	var floorPrice int64 = 50 // default spore ¥0.50
	if err := database.DB.Where("status = ?", "open").First(&round).Error; err == nil {
		floorPrice = round.SharePrice
	}

	nav := pool.CalcNAV()
	dynPrice := pool.CalcPrice(floorPrice)

	minFen, maxFen := model.RoundLimits(pool.CurrentRound)
	// Current round supply info
	roundSupply := round.SharesQuota // 本期总配额 (e.g. 1000万)
	roundIssued := round.SharesSold  // 本期已售出
	if roundSupply == 0 {
		roundSupply = model.StarDiamondTotal / 10 // fallback: 10% per round
	}

	c.JSON(http.StatusOK, gin.H{
		"name":                "星钻 (Star Diamond)",
		"total_supply":        model.StarDiamondTotal,
		"issued":              pool.TotalShares,
		"remaining":           model.StarDiamondTotal - pool.TotalShares,
		"round_supply":        roundSupply,
		"round_issued":        roundIssued,
		"round_remaining":     roundSupply - roundIssued,
		"nav_fen":             nav,
		"nav_yuan":            float64(nav) / 100,
		"floor_price_fen":     floorPrice,
		"floor_price_yuan":    float64(floorPrice) / 100,
		"price_fen":           dynPrice,
		"price_yuan":          float64(dynPrice) / 100,
		"price_driver":        priceDriver(nav, floorPrice),
		"current_round":       pool.CurrentRound,
		"current_round_label": round.Label,
		"active_investors":    investorCount,
		"total_raised_yuan":   float64(pool.TotalRaised) / 100,
		"pool_balance_yuan":   float64(pool.PoolBalance) / 100,
		"min_invest_yuan":     float64(minFen) / 100,
		"max_invest_yuan":     float64(maxFen) / 100,
		"activation_yuan":     float64(ActivationThreshold) / 100,
		"terms_available":     []int{1, 3, 5},
		"payment_available":   config.C.StarAI.URL != "",
		"payment_channels":    paymentChannels(),
	})
}

// priceDriver returns which factor is driving the current price.
func priceDriver(nav, floorPrice int64) string {
	if nav > floorPrice {
		return "NAV"
	}
	return "期次地板价"
}

// paymentChannels returns which payment channels are configured via StarAI.
func paymentChannels() []string {
	if config.C.StarAI.URL == "" {
		return []string{}
	}
	return []string{"alipay", "wechatpay"}
}

// POST /v1/investor/register — Self-register as investor
func (h *InvestorHandler) Register(c *gin.Context) {
	userID := c.GetString("user_id")
	db := database.DB

	var existing model.Investor
	if err := db.Where("user_id = ?", userID).First(&existing).Error; err == nil {
		c.JSON(http.StatusOK, gin.H{"investor": existing, "message": "已注册"})
		return
	}

	var user struct {
		Username string
		Email    string
		Phone    string
	}
	db.Table("users").Where("id = ?", userID).Select("username, email, phone").Scan(&user)

	investor := model.Investor{
		ID:       uuid.New().String(),
		UserID:   userID,
		Name:     user.Username,
		Email:    user.Email,
		Phone:    user.Phone,
		Source:   "self_register",
		Status:   "active",
		JoinedAt: time.Now(),
	}
	db.Create(&investor)

	log.Printf("[investor] Registered: user=%s name=%s", userID, user.Username)
	c.JSON(http.StatusCreated, gin.H{
		"investor": investor,
		"message":  "注册成功，请先签署合伙人协议（选择 1/3/5 年期限），再进行购买",
		"next":     "POST /v1/investor/agree",
	})
}

// POST /v1/investor/agree — Sign investor agreement (choose 1/3/5 year term)
func (h *InvestorHandler) SignAgreement(c *gin.Context) {
	userID := c.GetString("user_id")
	var req struct {
		Term int `json:"term" binding:"required"` // 1, 3, or 5 years
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请选择投资期限: 1 / 3 / 5 年"})
		return
	}
	if !validTerms[req.Term] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "期限只能选择 1、3 或 5 年", "valid": []int{1, 3, 5}})
		return
	}

	db := database.DB
	var investor model.Investor
	if err := db.Where("user_id = ?", userID).First(&investor).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "请先注册为合伙人"})
		return
	}
	if investor.AgreementTerm > 0 {
		c.JSON(http.StatusConflict, gin.H{
			"error":    fmt.Sprintf("已签署 %d 年期协议，到期: %s", investor.AgreementTerm, investor.AgreementExpiresAt.Format("2006-01-02")),
			"investor": investor,
		})
		return
	}

	now := time.Now()
	expires := now.AddDate(req.Term, 0, 0)
	db.Model(&investor).Updates(map[string]interface{}{
		"agreement_term":       req.Term,
		"agreement_signed_at":  &now,
		"agreement_expires_at": &expires,
	})
	investor.AgreementTerm = req.Term
	investor.AgreementSignedAt = &now
	investor.AgreementExpiresAt = &expires

	log.Printf("[investor] Agreement signed: user=%s term=%d years expires=%s", userID, req.Term, expires.Format("2006-01-02"))
	c.JSON(http.StatusOK, gin.H{
		"message":  fmt.Sprintf("已签署 %d 年合伙人协议，可开始购买星钻（每次 ¥1万 起，累计 ¥10万 激活分润）", req.Term),
		"investor": investor,
		"next":     "POST /v1/investor/recharge",
	})
}

// POST /v1/investor/recharge — Purchase Star Diamonds (星钻) using Star Energy (星能)
// Per-round min/max limits. Cumulative ≥ ¥10万 activates profit sharing.
// Price = max(NAV, round floor price). Payment via StarAI star energy.
func (h *InvestorHandler) Recharge(c *gin.Context) {
	userID := c.GetString("user_id")
	var req struct {
		Amount int64 `json:"amount" binding:"required"` // CNY in 分
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请指定购买金额 (分)"})
		return
	}

	db := database.DB
	var resultInvestor model.Investor
	var resultRound model.FundingRound
	var justActivated bool

	err := db.Transaction(func(tx *gorm.DB) error {
		var investor model.Investor
		if err := tx.Where("user_id = ?", userID).First(&investor).Error; err != nil {
			return fmt.Errorf("请先注册为合伙人")
		}
		if investor.Status != "active" {
			return fmt.Errorf("合伙人账户状态: %s", investor.Status)
		}
		if investor.AgreementTerm == 0 {
			return fmt.Errorf("请先签署合伙人协议")
		}

		var pool model.InvestorPool
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&pool).Error; err != nil {
			return fmt.Errorf("合伙人池未初始化")
		}
		if pool.Status != "active" {
			return fmt.Errorf("合伙人池暂停中")
		}

		// Get current open round
		var round model.FundingRound
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("status = ?", "open").First(&round).Error; err != nil {
			return fmt.Errorf("当前没有开放的融资轮次")
		}

		// Dynamic price = max(NAV, round floor price)
		dynPrice := pool.CalcPrice(round.SharePrice)

		// Check round quota
		shares := req.Amount / dynPrice
		if shares <= 0 {
			return fmt.Errorf("金额不足以购买星钻（当前价格 ¥%.2f/份，%s）", float64(dynPrice)/100, round.Label)
		}
		remaining := round.SharesQuota - round.SharesSold
		if remaining <= 0 {
			return fmt.Errorf("%s 份额已售罄", round.Label)
		}
		if shares > remaining {
			shares = remaining // cap to remaining quota
		}
		cost := shares * dynPrice

		// Per-round min/max validation
		minFen, maxFen := model.RoundLimits(round.Round)
		if req.Amount < minFen {
			return fmt.Errorf("%s 最低购买 ¥%.0f", round.Label, float64(minFen)/100)
		}
		if req.Amount > maxFen {
			return fmt.Errorf("%s 最高购买 ¥%.0f", round.Label, float64(maxFen)/100)
		}

		// Resolve claw_id for star energy deduction
		clawID := userToClawID(userID)
		if clawID == "" {
			return fmt.Errorf("未绑定 StarAI 账户，请先在 star-ai.net 注册并充值星能")
		}

		// Deduct from CreditAccount (star energy)
		energy := cost * int64(EnergyUnit) // 分 → energy units
		var acct model.CreditAccount
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("claw_id = ?", clawID).First(&acct).Error; err != nil {
			return fmt.Errorf("星能账户不存在，请先在 star-ai.net 充值")
		}
		if acct.Balance < energy {
			return fmt.Errorf("星能不足（需要 %d⚡，可用 %d⚡），请先在 star-ai.net 充值",
				cost, acct.Balance/int64(EnergyUnit))
		}

		// Deduct star energy
		tx.Model(&acct).Updates(map[string]interface{}{
			"balance":   gorm.Expr("balance - ?", energy),
			"total_out": gorm.Expr("total_out + ?", energy),
		})

		// Issue shares to investor
		investor.Shares += shares
		investor.TotalInvested += cost
		if !investor.Activated && investor.TotalInvested >= ActivationThreshold {
			now := time.Now()
			investor.Activated = true
			investor.ActivatedAt = &now
			justActivated = true
		}
		tx.Save(&investor)

		// Update pool
		pool.TotalShares += shares
		pool.TotalRaised += cost
		tx.Save(&pool)

		// Update round stats
		round.SharesSold += shares
		round.AmountRaised += cost
		round.InvestorCount++ // simplified; could deduplicate
		if round.SharesSold >= round.SharesQuota {
			round.Status = "sold_out"
			now := time.Now()
			round.ClosedAt = &now

			// Auto-advance to next round
			if next := model.NextRound(round.Round); next != "" {
				var nextRound model.FundingRound
				if err := tx.Where("round = ? AND status = ?", next, "upcoming").First(&nextRound).Error; err == nil {
					nextRound.Status = "open"
					nextRound.OpenedAt = &now
					tx.Save(&nextRound)
					pool.CurrentRound = next
					dynPrice = pool.CalcPrice(nextRound.SharePrice)
					log.Printf("[investor] Auto-advanced: %s sold out → %s opened @ ¥%.2f", round.Label, nextRound.Label, float64(nextRound.SharePrice)/100)
				}
			} else {
				pool.CurrentRound = "closed"
				log.Printf("[investor] All rounds sold out, pool rounds closed")
			}
		}
		tx.Save(&round)

		// Update pool share price to dynamic price
		pool.SharePrice = dynPrice

		// Record transactions
		tx.Create(&model.InvestorTransaction{
			ID:            uuid.New().String(),
			InvestorID:    investor.ID,
			Type:          "recharge",
			Shares:        shares,
			Amount:        cost,
			PricePerShare: dynPrice,
			Remark:        fmt.Sprintf("%s 购买 %d 星钻 @ ¥%.2f/份 = ¥%.0f", round.Label, shares, float64(dynPrice)/100, float64(cost)/100),
		})
		tx.Create(&model.CreditTransaction{
			ID:       uuid.New().String(),
			FromClaw: clawID,
			ToClaw:   "system:invest",
			Amount:   energy,
			Type:     "invest",
			Remark:   fmt.Sprintf("%s 购买 %d 星钻 @ ¥%.2f/份", round.Label, shares, float64(dynPrice)/100),
			Status:   "confirmed",
		})

		resultInvestor = investor
		resultRound = round
		log.Printf("[investor] Recharge: user=%s round=%s shares=%d price=¥%.2f cost=¥%.0f cumulative=¥%.0f activated=%v",
			userID, round.Label, shares, float64(dynPrice)/100, float64(cost)/100, float64(investor.TotalInvested)/100, investor.Activated)
		return nil
	})

	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	var pool model.InvestorPool
	db.First(&pool)

	nav := pool.CalcNAV()
	dynPrice := pool.CalcPrice(resultRound.SharePrice)

	resp := gin.H{
		"message":               "购买星钻成功",
		"investor":              resultInvestor,
		"round":                 resultRound.Label,
		"price_yuan":            float64(dynPrice) / 100,
		"nav_yuan":              float64(nav) / 100,
		"price_driver":          priceDriver(nav, resultRound.SharePrice),
		"total_invested_yuan":   float64(resultInvestor.TotalInvested) / 100,
		"portfolio_yuan":        float64(resultInvestor.Shares*dynPrice) / 100,
		"activated":             resultInvestor.Activated,
		"remaining_to_activate": int64(0),
		"round_remaining":       resultRound.SharesQuota - resultRound.SharesSold,
	}
	if !resultInvestor.Activated {
		resp["remaining_to_activate"] = ActivationThreshold - resultInvestor.TotalInvested
		resp["message"] = fmt.Sprintf("购买星钻成功（%s），还需充值 ¥%.0f 即可激活分润",
			resultRound.Label, float64(ActivationThreshold-resultInvestor.TotalInvested)/100)
	}
	if justActivated {
		resp["message"] = fmt.Sprintf("购买星钻成功（%s），已达 ¥10万 门槛，分润已激活！", resultRound.Label)
	}
	if pool.TotalShares > 0 {
		resp["share_percent"] = fmt.Sprintf("%.4f%%", float64(resultInvestor.Shares)/float64(pool.TotalShares)*100)
	}

	c.JSON(http.StatusOK, resp)
}

// GET /v1/investor/earnings — Daily earnings for last 30 days
func (h *InvestorHandler) DailyEarnings(c *gin.Context) {
	userID := c.GetString("user_id")
	db := database.DB

	var investor model.Investor
	if err := db.Where("user_id = ?", userID).First(&investor).Error; err != nil {
		middleware.Fail(c, http.StatusNotFound, middleware.CodeNotFound, "not an investor")
		return
	}

	var pool model.InvestorPool
	db.First(&pool)

	var ratio float64
	if pool.TotalShares > 0 {
		ratio = float64(investor.Shares) / float64(pool.TotalShares)
	}

	// Daily pool deposits (last 30 days)
	type dayRow struct {
		Date  string
		Total int64
		Count int64
	}
	var rows []dayRow
	db.Model(&model.PoolDeposit{}).
		Select("DATE(created_at) as date, SUM(amount) as total, COUNT(*) as count").
		Where("created_at >= ?", time.Now().AddDate(0, 0, -30)).
		Group("DATE(created_at)").Order("date DESC").
		Find(&rows)

	type earning struct {
		Date     string  `json:"date"`
		PoolFen  int64   `json:"pool_fen"`
		PoolYuan float64 `json:"pool_yuan"`
		MyFen    int64   `json:"my_fen"`
		MyYuan   float64 `json:"my_yuan"`
		Txns     int64   `json:"transactions"`
	}
	var earnings []earning
	var total30d int64
	var todayFen int64
	today := time.Now().Format("2006-01-02")

	for _, r := range rows {
		myFen := int64(float64(r.Total) * ratio)
		total30d += myFen
		if r.Date == today {
			todayFen = myFen
		}
		earnings = append(earnings, earning{
			Date:     r.Date,
			PoolFen:  r.Total,
			PoolYuan: float64(r.Total) / 100,
			MyFen:    myFen,
			MyYuan:   float64(myFen) / 100,
			Txns:     r.Count,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"investor":        investor,
		"share_ratio":     ratio,
		"share_percent":   fmt.Sprintf("%.4f%%", ratio*100),
		"today_earning":   float64(todayFen) / 100,
		"last_30d_total":  float64(total30d) / 100,
		"daily_earnings":  earnings,
		"pool_balance":    pool.PoolBalance,
		"total_deposited": pool.TotalDeposited,
	})
}

// GET /v1/investor/me — Full investor profile + dividends + transactions
func (h *InvestorHandler) MyProfile(c *gin.Context) {
	userID := c.GetString("user_id")

	var investor model.Investor
	if err := database.DB.Where("user_id = ?", userID).First(&investor).Error; err != nil {
		middleware.Fail(c, http.StatusNotFound, middleware.CodeNotFound, "not an investor")
		return
	}

	var pool model.InvestorPool
	database.DB.First(&pool)

	var sharePercent float64
	if pool.TotalShares > 0 {
		sharePercent = float64(investor.Shares) / float64(pool.TotalShares) * 100
	}

	var dividends []model.InvestorDividend
	database.DB.Where("investor_id = ?", investor.ID).Order("created_at DESC").Limit(12).Find(&dividends)

	var transactions []model.InvestorTransaction
	database.DB.Where("investor_id = ?", investor.ID).Order("created_at DESC").Limit(20).Find(&transactions)

	// Get current round for floor price
	var round model.FundingRound
	var floorPrice int64 = 50 // default spore ¥0.50
	if err := database.DB.Where("status = ?", "open").First(&round).Error; err == nil {
		floorPrice = round.SharePrice
	}
	nav := pool.CalcNAV()
	dynPrice := pool.CalcPrice(floorPrice)

	// Portfolio value = shares × current price
	portfolioFen := investor.Shares * dynPrice

	c.JSON(http.StatusOK, gin.H{
		"investor":          investor,
		"share_percent":     fmt.Sprintf("%.4f%%", sharePercent),
		"portfolio_yuan":    float64(portfolioFen) / 100,
		"nav_yuan":          float64(nav) / 100,
		"price_yuan":        float64(dynPrice) / 100,
		"price_driver":      priceDriver(nav, floorPrice),
		"pool_total_shares": pool.TotalShares,
		"pool_balance":      pool.PoolBalance,
		"min_invest_yuan":   float64(func() int64 { m, _ := model.RoundLimits(pool.CurrentRound); return m }()) / 100,
		"max_invest_yuan":   float64(func() int64 { _, m := model.RoundLimits(pool.CurrentRound); return m }()) / 100,
		"activation_yuan":   float64(ActivationThreshold) / 100,
		"dividends":         dividends,
		"transactions":      transactions,
	})
}

// ════════════════════════════════════════════════════════════
// Direct Payment: Buy Star Diamonds via Alipay/WeChat
//
// Flow: CreatePurchaseOrder → user pays → webhook → CompleteDiamondOrder → shares issued
// ════════════════════════════════════════════════════════════

// POST /v1/investor/purchase — Create a payment order for buying Star Diamonds
func (h *InvestorHandler) CreatePurchaseOrder(c *gin.Context) {
	userID := c.GetString("user_id")
	var req struct {
		Amount    int64  `json:"amount" binding:"required"`     // CNY in 分
		PayMethod string `json:"pay_method" binding:"required"` // alipay / wechatpay
		PayForm   string `json:"pay_form"`                      // pc / h5 / native
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请指定金额和支付方式"})
		return
	}
	// Per-round min/max validation
	minFen, maxFen := model.RoundLimits(func() string {
		var p model.InvestorPool
		if database.DB.First(&p).Error == nil {
			return p.CurrentRound
		}
		return "spore"
	}())
	if req.Amount < minFen {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("本期最低购买 ¥%.0f", float64(minFen)/100)})
		return
	}
	if req.Amount > maxFen {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("本期最高购买 ¥%.0f", float64(maxFen)/100)})
		return
	}
	if req.PayForm == "" {
		req.PayForm = "h5"
	}

	db := database.DB

	// Validate investor
	var investor model.Investor
	if err := db.Where("user_id = ?", userID).First(&investor).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "请先注册为合伙人"})
		return
	}
	if investor.Status != "active" {
		c.JSON(http.StatusConflict, gin.H{"error": "合伙人账户状态: " + investor.Status})
		return
	}
	if investor.AgreementTerm == 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "请先签署合伙人协议"})
		return
	}

	// Get pool + current round
	var pool model.InvestorPool
	if err := db.First(&pool).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "星钻池未初始化"})
		return
	}
	var round model.FundingRound
	if err := db.Where("status = ?", "open").First(&round).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "当前没有开放的融资轮次"})
		return
	}

	dynPrice := pool.CalcPrice(round.SharePrice)
	shares := req.Amount / dynPrice
	if shares <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("金额不足以购买星钻（当前价格 ¥%.2f/份）", float64(dynPrice)/100)})
		return
	}
	remaining := round.SharesQuota - round.SharesSold
	if shares > remaining {
		shares = remaining
	}
	cost := shares * dynPrice

	// Create diamond order (prefix SD = Star Diamond)
	orderNo := fmt.Sprintf("SD%s%04d", time.Now().Format("20060102150405"), time.Now().Nanosecond()/1000000)
	expire := time.Now().Add(30 * time.Minute)
	order := model.DiamondOrder{
		ID:            uuid.New().String(),
		OrderNo:       orderNo,
		UserID:        userID,
		InvestorID:    investor.ID,
		Amount:        cost,
		Shares:        shares,
		PricePerShare: dynPrice,
		Round:         round.Round,
		PayMethod:     req.PayMethod,
		PayForm:       req.PayForm,
		Status:        "pending",
		Subject:       fmt.Sprintf("星钻购买 - %s %d份 @ ¥%.2f", round.Label, shares, float64(dynPrice)/100),
		ExpireAt:      &expire,
	}
	if err := db.Create(&order).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建订单失败"})
		return
	}

	// Call payment gateway
	switch req.PayMethod {
	case "alipay", "wechatpay":
		h.createDiamondPayOrder(c, &order)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "不支持的支付方式，请使用 alipay 或 wechatpay"})
	}
}

// createDiamondPayOrder proxies the payment creation through StarAI Router's
// Alipay/WeChat channels. Router creates the actual payment and calls back Queen
// when the user completes payment.
func (h *InvestorHandler) createDiamondPayOrder(c *gin.Context, order *model.DiamondOrder) {
	starAI := config.C.StarAI
	if starAI.URL == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "支付通道尚未配置"})
		return
	}

	// Build callback URL (Queen's internal endpoint, reachable from Router's network)
	callbackBase := strings.TrimRight(starAI.CallbackBase, "/")
	if callbackBase == "" {
		// Fallback: assume same-host Docker with port
		if port := config.C.Server.Port; port != "" {
			callbackBase = fmt.Sprintf("http://queen-api:%s", port)
		} else {
			callbackBase = "http://queen-api:8085"
		}
	}
	callbackURL := callbackBase + "/internal/investor/payment-confirmed"

	body, _ := json.Marshal(map[string]interface{}{
		"channel":           order.PayMethod,
		"amount_cents":      order.Amount,
		"subject":           order.Subject,
		"external_order_no": order.OrderNo,
		"callback_url":      callbackURL,
		"pay_form":          order.PayForm,
	})

	req, _ := http.NewRequest("POST", starAI.URL+"/internal/payment/invest-order", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if starAI.Token != "" {
		req.Header.Set("X-Internal-Token", starAI.Token)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[investor] StarAI payment proxy error: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "支付通道暂不可用"})
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		log.Printf("[investor] StarAI payment returned %d: %s", resp.StatusCode, string(respBody))
		var errResp map[string]interface{}
		json.Unmarshal(respBody, &errResp)
		errMsg := "创建支付失败"
		if e, ok := errResp["error"].(string); ok {
			errMsg = e
		}
		c.JSON(http.StatusBadGateway, gin.H{"error": errMsg})
		return
	}

	var result map[string]interface{}
	json.Unmarshal(respBody, &result)

	// Build response for frontend
	out := gin.H{
		"order_no":    order.OrderNo,
		"shares":      order.Shares,
		"price_yuan":  float64(order.PricePerShare) / 100,
		"amount_yuan": float64(order.Amount) / 100,
		"round":       order.Round,
		"pay_method":  order.PayMethod,
	}
	if v, ok := result["pay_url"]; ok {
		out["pay_url"] = v
	}
	if v, ok := result["code_url"]; ok {
		out["code_url"] = v
	}
	c.JSON(http.StatusOK, out)
}

// POST /internal/investor/payment-confirmed — Called by StarAI Router when payment succeeds
func (h *InvestorHandler) PaymentConfirmed(c *gin.Context) {
	var req struct {
		ExternalOrderNo string `json:"external_order_no" binding:"required"`
		RouterOrderNo   string `json:"router_order_no"`
		TradeNo         string `json:"trade_no"`
		AmountCents     int64  `json:"amount_cents"`
		Channel         string `json:"channel"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid callback payload"})
		return
	}

	log.Printf("[investor] Payment confirmed from StarAI: queen_order=%s router_order=%s trade=%s amount=%d channel=%s",
		req.ExternalOrderNo, req.RouterOrderNo, req.TradeNo, req.AmountCents, req.Channel)

	callbackRaw, _ := json.Marshal(req)
	CompleteDiamondOrder(req.ExternalOrderNo, req.TradeNo, string(callbackRaw))

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// CompleteDiamondOrder is called by payment webhooks when a diamond purchase succeeds.
// It issues shares to the investor and updates the pool/round state.
func CompleteDiamondOrder(orderNo, tradeNo, callbackRaw string) {
	db := database.DB

	err := db.Transaction(func(tx *gorm.DB) error {
		var order model.DiamondOrder
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("order_no = ?", orderNo).First(&order).Error; err != nil {
			return fmt.Errorf("diamond order not found: %s", orderNo)
		}
		if order.Status == "paid" {
			return nil // idempotent
		}

		now := time.Now()
		order.Status = "paid"
		order.TradeNo = tradeNo
		order.PaidAt = &now
		if callbackRaw != "" {
			order.CallbackRaw = callbackRaw
		}
		tx.Save(&order)

		// Lock pool
		var pool model.InvestorPool
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&pool).Error; err != nil {
			return fmt.Errorf("pool not initialized")
		}

		// Lock current round
		var round model.FundingRound
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("round = ?", order.Round).First(&round).Error; err != nil {
			return fmt.Errorf("round %s not found", order.Round)
		}

		// Recalculate shares at current price (price may have changed since order)
		dynPrice := pool.CalcPrice(round.SharePrice)
		shares := order.Amount / dynPrice
		if shares <= 0 {
			return fmt.Errorf("price increased, insufficient amount for shares")
		}
		remaining := round.SharesQuota - round.SharesSold
		if shares > remaining {
			shares = remaining
		}

		// Load investor
		var investor model.Investor
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", order.InvestorID).First(&investor).Error; err != nil {
			return fmt.Errorf("investor %s not found", order.InvestorID)
		}

		// Issue shares
		investor.Shares += shares
		investor.TotalInvested += order.Amount
		if !investor.Activated && investor.TotalInvested >= ActivationThreshold {
			investor.Activated = true
			investor.ActivatedAt = &now
		}
		tx.Save(&investor)

		// Update pool
		pool.TotalShares += shares
		pool.TotalRaised += order.Amount
		tx.Save(&pool)

		// Update round
		round.SharesSold += shares
		round.AmountRaised += order.Amount
		round.InvestorCount++
		if round.SharesSold >= round.SharesQuota {
			round.Status = "sold_out"
			round.ClosedAt = &now
			if next := model.NextRound(round.Round); next != "" {
				var nextRound model.FundingRound
				if err := tx.Where("round = ? AND status = ?", next, "upcoming").First(&nextRound).Error; err == nil {
					nextRound.Status = "open"
					nextRound.OpenedAt = &now
					tx.Save(&nextRound)
					pool.CurrentRound = next
					log.Printf("[investor] Auto-advanced: %s sold out → %s opened", round.Label, nextRound.Label)
				}
			} else {
				pool.CurrentRound = "closed"
			}
		}
		tx.Save(&round)

		// Update pool share price
		pool.SharePrice = pool.CalcPrice(model.RoundFloorPrice(pool.CurrentRound))
		tx.Save(&pool)

		// Record transaction
		tx.Create(&model.InvestorTransaction{
			ID:            uuid.New().String(),
			InvestorID:    investor.ID,
			Type:          "purchase",
			Shares:        shares,
			Amount:        order.Amount,
			PricePerShare: dynPrice,
			Remark:        fmt.Sprintf("%s 购买 %d 星钻 @ ¥%.2f/份 (订单 %s)", model.RoundLabel(round.Round), shares, float64(dynPrice)/100, orderNo),
		})

		// Update order with actual shares issued
		tx.Model(&order).Update("shares", shares)

		log.Printf("[investor] DiamondOrder completed: order=%s user=%s round=%s shares=%d price=¥%.2f amount=¥%.0f",
			orderNo, order.UserID, round.Label, shares, float64(dynPrice)/100, float64(order.Amount)/100)
		return nil
	})

	if err != nil {
		log.Printf("[investor] CompleteDiamondOrder error: %v", err)
	}
}

// IsDiamondOrder checks if an order number belongs to a diamond purchase (prefix "SD").
func IsDiamondOrder(orderNo string) bool {
	return strings.HasPrefix(orderNo, "SD")
}

// GET /v1/investor/order/:order_no — Query diamond order status
func (h *InvestorHandler) QueryDiamondOrder(c *gin.Context) {
	userID := c.GetString("user_id")
	orderNo := c.Param("order_no")

	var order model.DiamondOrder
	if err := database.DB.Where("order_no = ? AND user_id = ?", orderNo, userID).First(&order).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "订单不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"order_no":        order.OrderNo,
		"status":          order.Status,
		"amount_yuan":     float64(order.Amount) / 100,
		"shares":          order.Shares,
		"price_per_share": float64(order.PricePerShare) / 100,
		"round":           order.Round,
		"round_label":     model.RoundLabel(order.Round),
		"paid_at":         order.PaidAt,
		"created_at":      order.CreatedAt,
	})
}

// GET /v1/investor/orders — List my diamond orders
func (h *InvestorHandler) ListDiamondOrders(c *gin.Context) {
	userID := c.GetString("user_id")
	var orders []model.DiamondOrder
	database.DB.Where("user_id = ?", userID).Order("created_at DESC").Limit(50).Find(&orders)

	items := make([]gin.H, 0, len(orders))
	for _, o := range orders {
		items = append(items, gin.H{
			"order_no":    o.OrderNo,
			"status":      o.Status,
			"amount_yuan": float64(o.Amount) / 100,
			"shares":      o.Shares,
			"round":       o.Round,
			"round_label": model.RoundLabel(o.Round),
			"paid_at":     o.PaidAt,
			"created_at":  o.CreatedAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{"orders": items})
}
