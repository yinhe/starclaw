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

// POST /internal/billing/check — check if user has sufficient balance
func (h *BillingHandler) InternalCheckBalance(c *gin.Context) {
	var req struct {
		UserID       string `json:"user_id" binding:"required"`
		ResourceType string `json:"resource_type"` // tokens / video / image / music
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数不完整"})
		return
	}

	bal := ensureBalance(req.UserID)
	c.JSON(http.StatusOK, gin.H{
		"user_id":   req.UserID,
		"balance":   bal.Balance,
		"has_quota": bal.Balance > 0,
	})
}

// POST /internal/billing/consume — deduct balance for resource usage
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

	// Auto-calculate amount if not provided
	amount := req.Amount
	if amount == 0 {
		amount = calculateCost(req.ResourceType, req.Quantity)
	}

	if amount <= 0 {
		c.JSON(http.StatusOK, gin.H{"deducted": 0, "balance": ensureBalance(req.UserID).Balance})
		return
	}

	db := database.DB
	var newBalance int64

	err := db.Transaction(func(tx *gorm.DB) error {
		var bal model.UserBalance
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ?", req.UserID).First(&bal).Error; err != nil {
			// Create balance if not exists
			bal = model.UserBalance{
				ID:     uuid.New().String(),
				UserID: req.UserID,
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

		remark := req.Remark
		if remark == "" {
			remark = fmt.Sprintf("消费 %s x%d", req.ResourceType, req.Quantity)
		}

		tx.Create(&model.BalanceTransaction{
			ID:     uuid.New().String(),
			UserID: req.UserID,
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

	c.JSON(http.StatusOK, gin.H{
		"deducted": amount,
		"balance":  newBalance,
	})
}

// GET /internal/billing/balance/:user_id — get user balance (for Claw sync)
func (h *BillingHandler) InternalGetBalance(c *gin.Context) {
	userID := c.Param("user_id")
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
