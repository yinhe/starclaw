package handler

import (
	"fmt"
	"log"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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
		SeedTotal:    model.StarDiamondTotal / 10, // 10% = 1000万 star diamonds for seed round
		SeedIssued:   0,
		CurrentRound: "seed",
		SharePrice:   20, // ¥0.20 per star diamond (seed floor price)
		Status:       "active",
	}
	db.Create(&pool)

	// Seed the 5 funding rounds (each = 10% of MaxShares)
	quota := pool.MaxShares / 10 // 10% = 1000万 shares per round
	for _, rc := range model.RoundConfig {
		status := "upcoming"
		if rc.Round == "seed" {
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "请指定轮次: seed / angel / a / b / c"})
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
			return fmt.Errorf("投资人池未初始化")
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
	var floorPrice int64 = 20 // default seed ¥0.20
	if err := database.DB.Where("status = ?", "open").First(&round).Error; err == nil {
		floorPrice = round.SharePrice
	}

	nav := pool.CalcNAV()
	dynPrice := pool.CalcPrice(floorPrice)

	c.JSON(http.StatusOK, gin.H{
		"name":                "星钻 (Star Diamond)",
		"total_supply":        model.StarDiamondTotal,
		"issued":              pool.TotalShares,
		"remaining":           model.StarDiamondTotal - pool.TotalShares,
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
		"min_recharge_yuan":   float64(MinRechargeFen) / 100,
		"activation_yuan":     float64(ActivationThreshold) / 100,
		"terms_available":     []int{1, 3, 5},
	})
}

// priceDriver returns which factor is driving the current price.
func priceDriver(nav, floorPrice int64) string {
	if nav > floorPrice {
		return "NAV"
	}
	return "轮次地板价"
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
		"message":  "注册成功，请先签署投资人协议（选择 1/3/5 年期限），再进行充值",
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
		c.JSON(http.StatusNotFound, gin.H{"error": "请先注册为投资人"})
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
		"message":  fmt.Sprintf("已签署 %d 年投资人协议，可开始充值（每次 ¥1万 起，累计 ¥10万 激活分润）", req.Term),
		"investor": investor,
		"next":     "POST /v1/investor/recharge",
	})
}

// POST /v1/investor/recharge — Purchase Star Diamonds (星钻)
// ¥1万 min per txn, cumulative ≥ ¥10万 activates profit sharing.
// Price = max(NAV, round floor price). Dynamic dual-driven pricing.
func (h *InvestorHandler) Recharge(c *gin.Context) {
	userID := c.GetString("user_id")
	var req struct {
		Amount int64 `json:"amount" binding:"required"` // CNY in 分
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请指定充值金额 (分)"})
		return
	}
	if req.Amount < MinRechargeFen {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      fmt.Sprintf("每次最低充值 ¥%.0f", float64(MinRechargeFen)/100),
			"min_amount": MinRechargeFen,
		})
		return
	}

	db := database.DB
	var resultInvestor model.Investor
	var resultRound model.FundingRound
	var justActivated bool

	err := db.Transaction(func(tx *gorm.DB) error {
		var investor model.Investor
		if err := tx.Where("user_id = ?", userID).First(&investor).Error; err != nil {
			return fmt.Errorf("请先注册为投资人")
		}
		if investor.Status != "active" {
			return fmt.Errorf("投资人账户状态: %s", investor.Status)
		}
		if investor.AgreementTerm == 0 {
			return fmt.Errorf("请先签署投资人协议（POST /v1/investor/agree）")
		}

		var pool model.InvestorPool
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&pool).Error; err != nil {
			return fmt.Errorf("投资人池未初始化")
		}
		if pool.Status != "active" {
			return fmt.Errorf("投资人池暂停中")
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

		// Deduct from user balance
		var balance model.UserBalance
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ?", userID).First(&balance).Error; err != nil {
			return fmt.Errorf("余额账户不存在，请先充值到平台余额")
		}
		if balance.Balance < cost {
			return fmt.Errorf("余额不足（当前 ¥%.2f，需要 ¥%.2f）",
				float64(balance.Balance)/100, float64(cost)/100)
		}

		// Deduct balance
		balance.Balance -= cost
		balance.TotalOut += cost
		tx.Save(&balance)

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
		tx.Create(&model.BalanceTransaction{
			ID:     uuid.New().String(),
			UserID: userID,
			Type:   "invest",
			Amount: -cost,
			Before: balance.Balance + cost,
			After:  balance.Balance,
			Remark: fmt.Sprintf("%s 购买 %d 星钻", round.Label, shares),
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
	var floorPrice int64 = 20 // default seed ¥0.20
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
		"min_recharge_yuan": float64(MinRechargeFen) / 100,
		"activation_yuan":   float64(ActivationThreshold) / 100,
		"dividends":         dividends,
		"transactions":      transactions,
	})
}
