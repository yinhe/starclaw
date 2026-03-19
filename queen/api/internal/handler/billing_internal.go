package handler

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/yinhe/starclaw-queen/api/internal/database"
	"github.com/yinhe/starclaw-queen/api/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ============================================================
// Internal API — called by Claw nodes (authenticated via node token)
// ============================================================

// energyUnit is the conversion factor: 1 分 (¥0.01) = 1⚡ = 10000 internal units.
const energyUnit = 10000

// userToClawID resolves a Queen user_id to a claw_id for star energy billing.
// Priority: OAuthID (claw wallet login) → NodeBinding.
func userToClawID(userID string) string {
	db := database.DB

	// 1. Check if user logged in via Claw wallet (OAuthProvider = "claw")
	var user struct {
		OAuthProvider string
		OAuthID       string
	}
	if err := db.Table("users").Where("id = ?", userID).
		Select("o_auth_provider, o_auth_id").Scan(&user).Error; err == nil {
		if user.OAuthProvider == "claw" && user.OAuthID != "" {
			return user.OAuthID
		}
	}

	// 2. Fallback: NodeBinding (node_id = claw_id → queen_user_id)
	var binding struct {
		NodeID string
	}
	if err := db.Table("node_bindings").Where("queen_user_id = ? AND status = ?", userID, "active").
		Select("node_id").First(&binding).Error; err == nil {
		return binding.NodeID
	}

	return ""
}

// POST /internal/billing/check — check if user has sufficient star energy
func (h *BillingHandler) InternalCheckBalance(c *gin.Context) {
	var req struct {
		UserID       string `json:"user_id" binding:"required"`
		ResourceType string `json:"resource_type"` // tokens / video / image / music
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数不完整"})
		return
	}

	clawID := userToClawID(req.UserID)
	if clawID == "" {
		// No claw_id found — fallback to UserBalance for backward compat
		bal := ensureBalance(req.UserID)
		c.JSON(http.StatusOK, gin.H{
			"user_id":   req.UserID,
			"balance":   bal.Balance,
			"has_quota": bal.Balance > 0,
		})
		return
	}

	acct := ensureCreditAccount(clawID)
	// Convert energy units to 分 for backward compat response
	balanceFen := acct.Balance / energyUnit
	c.JSON(http.StatusOK, gin.H{
		"user_id":   req.UserID,
		"claw_id":   clawID,
		"balance":   balanceFen,
		"has_quota": acct.Balance > 0,
	})
}

// POST /internal/billing/consume — deduct star energy for resource usage
// Accepts user_id, auto-resolves to claw_id, deducts from CreditAccount.
// Amount is in 分; internally converted to energy units (× 10000).
func (h *BillingHandler) InternalConsume(c *gin.Context) {
	var req struct {
		UserID       string `json:"user_id" binding:"required"`
		ResourceType string `json:"resource_type" binding:"required"` // tokens / video / image / music
		Quantity     int64  `json:"quantity" binding:"required"`
		Amount       int64  `json:"amount"` // cost in 分, if 0 auto-calculate
		Remark       string `json:"remark"`
		NodeID       string `json:"node_id"` // which Claw node
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数不完整"})
		return
	}

	// Auto-calculate amount (in 分) if not provided
	amountFen := req.Amount
	if amountFen == 0 {
		amountFen = calculateCost(req.ResourceType, req.Quantity)
	}
	if amountFen <= 0 {
		c.JSON(http.StatusOK, gin.H{"deducted": 0, "balance": int64(0)})
		return
	}

	// Resolve user_id → claw_id
	clawID := userToClawID(req.UserID)
	if clawID == "" {
		// No claw_id — fallback to legacy UserBalance deduction
		h.legacyConsume(c, req.UserID, amountFen, req.ResourceType, req.Quantity, req.Remark)
		return
	}

	// Convert 分 → energy units (1 分 = 10000 units = 1⚡)
	energy := amountFen * energyUnit

	db := database.DB
	var newBalance int64

	err := db.Transaction(func(tx *gorm.DB) error {
		var acct model.CreditAccount
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("claw_id = ?", clawID).First(&acct).Error; err != nil {
			return fmt.Errorf("星能账户不存在: %s", clawID)
		}

		if acct.Balance < energy {
			return fmt.Errorf("星能不足（需要 %d⚡，可用 %d⚡）", amountFen, acct.Balance/energyUnit)
		}

		tx.Model(&acct).Updates(map[string]interface{}{
			"balance":   gorm.Expr("balance - ?", energy),
			"total_out": gorm.Expr("total_out + ?", energy),
		})

		// Update HP status
		remaining := acct.Balance - energy
		if remaining <= 0 && acct.Status != "hibernated" {
			tx.Model(&acct).Update("status", "hibernated")
		}

		remark := req.Remark
		if remark == "" {
			remark = fmt.Sprintf("%s x%d (¥%.2f)", req.ResourceType, req.Quantity, float64(amountFen)/100)
		}

		tx.Create(&model.CreditTransaction{
			ID:       uuid.New().String(),
			FromClaw: clawID,
			ToClaw:   "system",
			Amount:   energy,
			Type:     "consume",
			Remark:   remark,
			Status:   "confirmed",
		})

		newBalance = remaining
		return nil
	})

	if err != nil {
		c.JSON(http.StatusPaymentRequired, gin.H{"error": err.Error()})
		return
	}

	// Return balance in 分 for backward compat
	c.JSON(http.StatusOK, gin.H{
		"deducted": amountFen,
		"balance":  newBalance / energyUnit,
	})
}

// legacyConsume is the fallback for users without a claw_id (deducts from UserBalance).
func (h *BillingHandler) legacyConsume(c *gin.Context, userID string, amount int64, resourceType string, quantity int64, remark string) {
	db := database.DB
	var newBalance int64

	err := db.Transaction(func(tx *gorm.DB) error {
		var bal model.UserBalance
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ?", userID).First(&bal).Error; err != nil {
			bal = model.UserBalance{
				ID:     uuid.New().String(),
				UserID: userID,
			}
			tx.Create(&bal)
		}

		if bal.Balance < amount {
			return fmt.Errorf("余额不足")
		}

		before := bal.Balance
		bal.Balance -= amount
		bal.TotalOut += amount
		newBalance = bal.Balance
		if err := tx.Save(&bal).Error; err != nil {
			return err
		}

		if remark == "" {
			remark = fmt.Sprintf("消费 %s x%d", resourceType, quantity)
		}
		tx.Create(&model.BalanceTransaction{
			ID:     uuid.New().String(),
			UserID: userID,
			Type:   "consume",
			Amount: -amount,
			Before: before,
			After:  bal.Balance,
			Remark: remark,
		})

		return nil
	})

	if err != nil {
		c.JSON(http.StatusPaymentRequired, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deducted": amount, "balance": newBalance})
}

// GET /internal/billing/balance/:user_id — get user balance (for Claw sync)
func (h *BillingHandler) InternalGetBalance(c *gin.Context) {
	userID := c.Param("user_id")

	clawID := userToClawID(userID)
	if clawID != "" {
		acct := ensureCreditAccount(clawID)
		c.JSON(http.StatusOK, gin.H{
			"user_id":   userID,
			"claw_id":   clawID,
			"balance":   acct.Balance / energyUnit,
			"total_in":  acct.TotalIn / energyUnit,
			"total_out": acct.TotalOut / energyUnit,
		})
		return
	}

	bal := ensureBalance(userID)
	c.JSON(http.StatusOK, gin.H{
		"user_id":   userID,
		"balance":   bal.Balance,
		"total_in":  bal.TotalIn,
		"total_out": bal.TotalOut,
	})
}

// POST /internal/billing/freeze — freeze user balance for bounty escrow
func (h *BillingHandler) InternalFreeze(c *gin.Context) {
	var req struct {
		UserID   string `json:"user_id" binding:"required"`
		Amount   int64  `json:"amount" binding:"required"`
		BountyID string `json:"bounty_id"`
		Remark   string `json:"remark"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数不完整"})
		return
	}
	if req.Amount <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "冻结金额必须大于0"})
		return
	}

	db := database.DB
	var newBalance, newFrozen int64

	err := db.Transaction(func(tx *gorm.DB) error {
		var bal model.UserBalance
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ?", req.UserID).First(&bal).Error; err != nil {
			bal = model.UserBalance{
				ID:     uuid.New().String(),
				UserID: req.UserID,
			}
			tx.Create(&bal)
		}

		if bal.Balance < req.Amount {
			return fmt.Errorf("余额不足（可用 %d 分，需冻结 %d 分）", bal.Balance, req.Amount)
		}

		before := bal.Balance
		bal.Balance -= req.Amount
		bal.Frozen += req.Amount
		newBalance = bal.Balance
		newFrozen = bal.Frozen
		if err := tx.Save(&bal).Error; err != nil {
			return err
		}

		remark := req.Remark
		if remark == "" {
			remark = fmt.Sprintf("赏金冻结 bounty=%s", req.BountyID)
		}
		tx.Create(&model.BalanceTransaction{
			ID:      uuid.New().String(),
			UserID:  req.UserID,
			OrderNo: req.BountyID,
			Type:    "freeze",
			Amount:  -req.Amount,
			Before:  before,
			After:   bal.Balance,
			Remark:  remark,
		})

		log.Printf("[billing] Frozen %d for user=%s bounty=%s", req.Amount, req.UserID, req.BountyID)
		return nil
	})

	if err != nil {
		c.JSON(http.StatusPaymentRequired, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"frozen": req.Amount, "balance": newBalance, "total_frozen": newFrozen})
}

// POST /internal/billing/unfreeze — unfreeze (refund) frozen amount back to user (bounty cancel/expire)
func (h *BillingHandler) InternalUnfreeze(c *gin.Context) {
	var req struct {
		UserID   string `json:"user_id" binding:"required"`
		Amount   int64  `json:"amount" binding:"required"`
		BountyID string `json:"bounty_id"`
		Remark   string `json:"remark"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数不完整"})
		return
	}

	db := database.DB
	var newBalance, newFrozen int64

	err := db.Transaction(func(tx *gorm.DB) error {
		var bal model.UserBalance
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ?", req.UserID).First(&bal).Error; err != nil {
			return fmt.Errorf("用户余额不存在")
		}

		if bal.Frozen < req.Amount {
			return fmt.Errorf("冻结余额不足（冻结 %d 分，需解冻 %d 分）", bal.Frozen, req.Amount)
		}

		before := bal.Balance
		bal.Frozen -= req.Amount
		bal.Balance += req.Amount
		newBalance = bal.Balance
		newFrozen = bal.Frozen
		if err := tx.Save(&bal).Error; err != nil {
			return err
		}

		remark := req.Remark
		if remark == "" {
			remark = fmt.Sprintf("赏金解冻（退回） bounty=%s", req.BountyID)
		}
		tx.Create(&model.BalanceTransaction{
			ID:      uuid.New().String(),
			UserID:  req.UserID,
			OrderNo: req.BountyID,
			Type:    "unfreeze",
			Amount:  req.Amount,
			Before:  before,
			After:   bal.Balance,
			Remark:  remark,
		})

		log.Printf("[billing] Unfrozen %d for user=%s bounty=%s", req.Amount, req.UserID, req.BountyID)
		return nil
	})

	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"unfrozen": req.Amount, "balance": newBalance, "total_frozen": newFrozen})
}

// POST /internal/billing/settle — settle bounty: release frozen amount from creator to completer
func (h *BillingHandler) InternalSettle(c *gin.Context) {
	var req struct {
		FromUserID string  `json:"from_user_id" binding:"required"` // bounty creator
		ToUserID   string  `json:"to_user_id" binding:"required"`   // bounty completer
		Amount     int64   `json:"amount" binding:"required"`
		FeeRate    float64 `json:"fee_rate"` // platform fee (e.g. 0.05 = 5%)
		BountyID   string  `json:"bounty_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数不完整"})
		return
	}

	fee := int64(float64(req.Amount) * req.FeeRate)
	payout := req.Amount - fee

	db := database.DB
	err := db.Transaction(func(tx *gorm.DB) error {
		// 1. Deduct from creator's frozen balance
		var fromBal model.UserBalance
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ?", req.FromUserID).First(&fromBal).Error; err != nil {
			return fmt.Errorf("发布者余额不存在")
		}
		if fromBal.Frozen < req.Amount {
			return fmt.Errorf("冻结余额不足")
		}

		fromBefore := fromBal.Balance
		fromBal.Frozen -= req.Amount
		fromBal.TotalOut += req.Amount
		if err := tx.Save(&fromBal).Error; err != nil {
			return err
		}

		tx.Create(&model.BalanceTransaction{
			ID:      uuid.New().String(),
			UserID:  req.FromUserID,
			OrderNo: req.BountyID,
			Type:    "bounty_pay",
			Amount:  -req.Amount,
			Before:  fromBefore,
			After:   fromBal.Balance,
			Remark:  fmt.Sprintf("赏金结算（支出） bounty=%s", req.BountyID),
		})

		// 2. Credit to completer's available balance
		var toBal model.UserBalance
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ?", req.ToUserID).First(&toBal).Error; err != nil {
			toBal = model.UserBalance{
				ID:     uuid.New().String(),
				UserID: req.ToUserID,
			}
			tx.Create(&toBal)
		}

		toBefore := toBal.Balance
		toBal.Balance += payout
		toBal.TotalIn += payout
		if err := tx.Save(&toBal).Error; err != nil {
			return err
		}

		tx.Create(&model.BalanceTransaction{
			ID:      uuid.New().String(),
			UserID:  req.ToUserID,
			OrderNo: req.BountyID,
			Type:    "bounty_earn",
			Amount:  payout,
			Before:  toBefore,
			After:   toBal.Balance,
			Remark:  fmt.Sprintf("赏金收入 bounty=%s", req.BountyID),
		})

		// 3. Record platform fee (if any)
		if fee > 0 {
			tx.Create(&model.BalanceTransaction{
				ID:      uuid.New().String(),
				UserID:  "platform",
				OrderNo: req.BountyID,
				Type:    "bounty_fee",
				Amount:  fee,
				Before:  0,
				After:   0,
				Remark:  fmt.Sprintf("赏金平台服务费 %.0f%% bounty=%s", req.FeeRate*100, req.BountyID),
			})
		}

		log.Printf("[billing] Settled bounty=%s: from=%s amount=%d, to=%s payout=%d, fee=%d",
			req.BountyID, req.FromUserID, req.Amount, req.ToUserID, payout, fee)
		return nil
	})

	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"settled": req.Amount, "payout": payout, "fee": fee})
}

// GET /internal/billing/resolve-partners?claw_id=xxx
// Resolves the partner chain for a given Claw node: claw_id → user → CityPartner → TeamPartner
// Used by Billing Gateway for revenue split routing.
func (h *BillingHandler) InternalResolvePartners(c *gin.Context) {
	clawID := c.Query("claw_id")
	if clawID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "claw_id required"})
		return
	}

	db := database.DB
	result := gin.H{
		"claw_id":           clawID,
		"city_partner_id":   "",
		"city_partner_name": "",
		"core_partner_id":   "",
		"core_partner_name": "",
	}

	// 1. NodeBinding: claw_id → queen user_id
	var binding model.NodeBinding
	if err := db.Where("node_id = ? AND status = ?", clawID, "active").First(&binding).Error; err != nil {
		// No binding — might be direct connection without Queen account
		c.JSON(http.StatusOK, result)
		return
	}
	userID := binding.QueenUserID

	// 2. CityClient: user_id → partner_id (CityPartner)
	var cityClient model.CityClient
	if err := db.Where("user_id = ?", userID).First(&cityClient).Error; err != nil {
		// No city partner attribution — direct user
		c.JSON(http.StatusOK, result)
		return
	}

	// 3. CityPartner: get the city partner
	var cityPartner model.CityPartner
	if err := db.Where("id = ? AND status = ?", cityClient.PartnerID, "approved").First(&cityPartner).Error; err != nil {
		c.JSON(http.StatusOK, result)
		return
	}
	result["city_partner_id"] = cityPartner.ID
	result["city_partner_name"] = cityPartner.Name

	// 4. TeamPartner: explicit link first, then fallback to region match
	var corePartner model.TeamPartner
	if cityPartner.TeamPartnerID != "" {
		// Explicit link (set when core partner adds city partner via AddCityPartnerClaw)
		if err := db.Where("id = ? AND status = ?", cityPartner.TeamPartnerID, "active").First(&corePartner).Error; err == nil {
			result["core_partner_id"] = corePartner.ID
			result["core_partner_name"] = corePartner.Name
		}
	} else if cityPartner.City != "" {
		// Fallback: region match
		if err := db.Where("region = ? AND status = ?", cityPartner.City, "active").First(&corePartner).Error; err == nil {
			result["core_partner_id"] = corePartner.ID
			result["core_partner_name"] = corePartner.Name
		}
	}

	log.Printf("[billing] resolve-partners: claw=%s → city=%s core=%s", clawID, result["city_partner_id"], result["core_partner_id"])
	c.JSON(http.StatusOK, result)
}

// Resource pricing: amount in 分 per unit
func calculateCost(resourceType string, quantity int64) int64 {
	// Pricing in 分
	prices := map[string]float64{
		"tokens": 0.001, // ¥0.01 per 1K tokens = 1分/1K = 0.001分/token
		"video":  200,   // ¥2 per video = 200分
		"image":  50,    // ¥0.5 per image = 50分
		"music":  100,   // ¥1 per music = 100分
	}
	price, ok := prices[resourceType]
	if !ok {
		return 0
	}
	return int64(float64(quantity) * price)
}

// ============================================================
// Admin API — called by Core (operations center)
// ============================================================

// GET /admin/billing/stats — revenue overview
func (h *BillingHandler) AdminBillingStats(c *gin.Context) {
	db := database.DB

	// Total revenue (paid orders)
	var totalRevenue int64
	db.Model(&model.RechargeOrder{}).Where("status = ?", "paid").
		Select("COALESCE(SUM(amount), 0)").Scan(&totalRevenue)

	// Total orders
	var totalOrders int64
	db.Model(&model.RechargeOrder{}).Where("status = ?", "paid").Count(&totalOrders)

	// Today's revenue
	today := time.Now().Format("2006-01-02")
	var todayRevenue int64
	db.Model(&model.RechargeOrder{}).Where("status = ? AND DATE(paid_at) = ?", "paid", today).
		Select("COALESCE(SUM(amount), 0)").Scan(&todayRevenue)

	var todayOrders int64
	db.Model(&model.RechargeOrder{}).Where("status = ? AND DATE(paid_at) = ?", "paid", today).Count(&todayOrders)

	// Total users with balance
	var totalUsers int64
	db.Model(&model.UserBalance{}).Count(&totalUsers)

	// Total balance across all users
	var totalBalance int64
	db.Model(&model.UserBalance{}).Select("COALESCE(SUM(balance), 0)").Scan(&totalBalance)

	// Total consumed
	var totalConsumed int64
	db.Model(&model.UserBalance{}).Select("COALESCE(SUM(total_out), 0)").Scan(&totalConsumed)

	// This month revenue
	monthStart := time.Now().Format("2006-01") + "-01"
	var monthRevenue int64
	db.Model(&model.RechargeOrder{}).Where("status = ? AND paid_at >= ?", "paid", monthStart).
		Select("COALESCE(SUM(amount), 0)").Scan(&monthRevenue)

	c.JSON(http.StatusOK, gin.H{
		"total_revenue":  totalRevenue,
		"total_orders":   totalOrders,
		"today_revenue":  todayRevenue,
		"today_orders":   todayOrders,
		"month_revenue":  monthRevenue,
		"total_users":    totalUsers,
		"total_balance":  totalBalance,
		"total_consumed": totalConsumed,
	})
}

// GET /admin/billing/orders — list all orders (paginated)
func (h *BillingHandler) AdminListOrders(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	status := c.Query("status")
	if page < 1 {
		page = 1
	}

	db := database.DB
	query := db.Model(&model.RechargeOrder{})
	if status != "" {
		query = query.Where("status = ?", status)
	}

	var total int64
	query.Count(&total)

	var orders []model.RechargeOrder
	query.Order("created_at DESC").Offset((page - 1) * size).Limit(size).Find(&orders)

	c.JSON(http.StatusOK, gin.H{
		"orders": orders,
		"total":  total,
		"page":   page,
		"size":   size,
	})
}

// GET /admin/billing/balances — list all user balances
func (h *BillingHandler) AdminListBalances(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	if page < 1 {
		page = 1
	}

	var total int64
	database.DB.Model(&model.UserBalance{}).Count(&total)

	var balances []model.UserBalance
	database.DB.Order("balance DESC").Offset((page - 1) * size).Limit(size).Find(&balances)

	c.JSON(http.StatusOK, gin.H{
		"balances": balances,
		"total":    total,
		"page":     page,
		"size":     size,
	})
}

// POST /admin/billing/adjust — manually adjust user balance (admin tool)
func (h *BillingHandler) AdminAdjustBalance(c *gin.Context) {
	var req struct {
		UserID string `json:"user_id" binding:"required"`
		Amount int64  `json:"amount" binding:"required"` // positive=add, negative=deduct
		Remark string `json:"remark" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数不完整"})
		return
	}

	db := database.DB
	err := db.Transaction(func(tx *gorm.DB) error {
		var bal model.UserBalance
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ?", req.UserID).First(&bal).Error; err != nil {
			bal = model.UserBalance{
				ID:     uuid.New().String(),
				UserID: req.UserID,
			}
			tx.Create(&bal)
		}

		before := bal.Balance
		bal.Balance += req.Amount
		if bal.Balance < 0 {
			bal.Balance = 0
		}
		if req.Amount > 0 {
			bal.TotalIn += req.Amount
		} else {
			bal.TotalOut += -req.Amount
		}
		tx.Save(&bal)

		tx.Create(&model.BalanceTransaction{
			ID:     uuid.New().String(),
			UserID: req.UserID,
			Type:   "admin_adjust",
			Amount: req.Amount,
			Before: before,
			After:  bal.Balance,
			Remark: req.Remark,
		})

		log.Printf("[billing] Admin adjusted balance: user=%s, amount=%d, remark=%s", req.UserID, req.Amount, req.Remark)
		return nil
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	bal := ensureBalance(req.UserID)
	c.JSON(http.StatusOK, gin.H{
		"message": "余额已调整",
		"balance": bal.Balance,
	})
}

// GET /admin/billing/packages — list all packages (including disabled)
func (h *BillingHandler) AdminListPackages(c *gin.Context) {
	var packages []model.RechargePackage
	database.DB.Order("sort_order ASC").Find(&packages)
	c.JSON(http.StatusOK, gin.H{"packages": packages})
}

// PUT /admin/billing/packages/:id — update package
func (h *BillingHandler) AdminUpdatePackage(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Name        *string  `json:"name"`
		Amount      *int64   `json:"amount"`
		BonusAmount *int64   `json:"bonus_amount"`
		BonusRate   *float64 `json:"bonus_rate"`
		SortOrder   *int     `json:"sort_order"`
		Enabled     *bool    `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	updates := map[string]interface{}{}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Amount != nil {
		updates["amount"] = *req.Amount
	}
	if req.BonusAmount != nil {
		updates["bonus_amount"] = *req.BonusAmount
	}
	if req.BonusRate != nil {
		updates["bonus_rate"] = *req.BonusRate
	}
	if req.SortOrder != nil {
		updates["sort_order"] = *req.SortOrder
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}

	database.DB.Model(&model.RechargePackage{}).Where("id = ?", id).Updates(updates)
	c.JSON(http.StatusOK, gin.H{"message": "套餐已更新"})
}
