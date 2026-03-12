package handler

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/yinhe/starclaw-queen/api/internal/database"
	"github.com/yinhe/starclaw-queen/api/internal/middleware"
	"github.com/yinhe/starclaw-queen/api/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// CreditHandler handles star credit (星力) operations
type CreditHandler struct{}

// ─── Constants ───

const (
	StarUnit         = 10000 // 1 Star = 10000 internal units (4 decimal precision)
	NewClawBonus     = 100 * StarUnit // 100 ⭐ welcome bonus
	MaxTransferNoFee = int64(math.MaxInt64) // no fee on transfers currently
)

// ─── Helper: derive claw address from public key ───

func deriveClawAddress(pubKey ed25519.PublicKey) string {
	hash := sha256.Sum256(pubKey)
	return "claw:" + hex.EncodeToString(hash[:])[:40]
}

// ─── Helper: verify Ed25519 signature and extract claw address ───

func verifyClawSignature(clawID, publicKeyHex, message, signatureHex string) error {
	pubBytes, err := hex.DecodeString(publicKeyHex)
	if err != nil || len(pubBytes) != ed25519.PublicKeySize {
		return fmt.Errorf("invalid public key")
	}
	pubKey := ed25519.PublicKey(pubBytes)

	// verify pubkey → claw address match
	derived := deriveClawAddress(pubKey)
	if derived != clawID {
		return fmt.Errorf("public key does not match claw address")
	}

	sigBytes, err := hex.DecodeString(signatureHex)
	if err != nil || len(sigBytes) != ed25519.SignatureSize {
		return fmt.Errorf("invalid signature")
	}

	if !ed25519.Verify(pubKey, []byte(message), sigBytes) {
		return fmt.Errorf("signature verification failed")
	}
	return nil
}

// ─── Helper: ensure credit account exists ───

func ensureCreditAccount(clawID string) *model.CreditAccount {
	var acct model.CreditAccount
	if err := database.DB.Where("claw_id = ?", clawID).First(&acct).Error; err != nil {
		acct = model.CreditAccount{
			ID:     uuid.New().String(),
			ClawID: clawID,
			Status: "active",
		}
		database.DB.Create(&acct)
	}
	return &acct
}

// ─── Public API: GET /v1/credits/balance ───

func (h *CreditHandler) GetBalance(c *gin.Context) {
	clawID := c.Query("claw_id")
	if clawID == "" || !strings.HasPrefix(clawID, "claw:") {
		middleware.Fail(c, http.StatusBadRequest, middleware.CodeBadRequest, "缺少 claw_id 参数")
		return
	}

	acct := ensureCreditAccount(clawID)

	// Calculate HP status
	stars := acct.Balance / int64(StarUnit)
	hpStatus := "hibernated"
	switch {
	case stars > 1000:
		hpStatus = "full"
	case stars > 100:
		hpStatus = "healthy"
	case stars > 10:
		hpStatus = "low"
	case stars > 0:
		hpStatus = "critical"
	}

	middleware.OK(c, gin.H{
		"claw_id":     acct.ClawID,
		"balance":     acct.Balance,
		"balance_stars": float64(acct.Balance) / float64(StarUnit),
		"frozen":      acct.Frozen,
		"frozen_stars": float64(acct.Frozen) / float64(StarUnit),
		"total_in":    acct.TotalIn,
		"total_out":   acct.TotalOut,
		"nonce":       acct.Nonce,
		"status":      acct.Status,
		"hp_status":   hpStatus,
		"trust_level": acct.TrustLevel,
	})
}

// ─── Public API: POST /v1/credits/transfer ───

func (h *CreditHandler) Transfer(c *gin.Context) {
	var req struct {
		FromClaw  string `json:"from_claw" binding:"required"`
		ToClaw    string `json:"to_claw" binding:"required"`
		Amount    int64  `json:"amount" binding:"required"`  // in internal units
		Nonce     int64  `json:"nonce" binding:"required"`
		PublicKey string `json:"public_key" binding:"required"` // hex
		Signature string `json:"signature" binding:"required"`  // hex
		Remark    string `json:"remark"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, http.StatusBadRequest, middleware.CodeBadRequest, "参数不完整")
		return
	}

	if req.Amount <= 0 {
		middleware.Fail(c, http.StatusBadRequest, middleware.CodeBadRequest, "转账金额必须大于 0")
		return
	}
	if req.FromClaw == req.ToClaw {
		middleware.Fail(c, http.StatusBadRequest, middleware.CodeBadRequest, "不能转账给自己")
		return
	}

	// Verify signature: message = "transfer|{from}|{to}|{amount}|{nonce}"
	message := fmt.Sprintf("transfer|%s|%s|%d|%d", req.FromClaw, req.ToClaw, req.Amount, req.Nonce)
	if err := verifyClawSignature(req.FromClaw, req.PublicKey, message, req.Signature); err != nil {
		middleware.Fail(c, http.StatusUnauthorized, middleware.CodeUnauthorized, "签名验证失败: "+err.Error())
		return
	}

	db := database.DB
	var txnID string
	var newBalance int64

	err := db.Transaction(func(tx *gorm.DB) error {
		// Lock sender account
		var from model.CreditAccount
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("claw_id = ?", req.FromClaw).First(&from).Error; err != nil {
			return fmt.Errorf("发送方账户不存在")
		}

		// Check nonce (must be > current nonce)
		if req.Nonce <= from.Nonce {
			return fmt.Errorf("nonce 过期（当前: %d，提交: %d）", from.Nonce, req.Nonce)
		}

		// Check balance
		if from.Balance < req.Amount {
			return fmt.Errorf("余额不足（可用: %d，需要: %d）", from.Balance, req.Amount)
		}

		// Store public key if not set
		if from.PublicKey == "" {
			tx.Model(&from).Update("public_key", req.PublicKey)
		}

		// Deduct from sender
		tx.Model(&from).Updates(map[string]interface{}{
			"balance":   gorm.Expr("balance - ?", req.Amount),
			"total_out": gorm.Expr("total_out + ?", req.Amount),
			"nonce":     req.Nonce,
		})

		// Credit receiver (auto-create if needed)
		var to model.CreditAccount
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("claw_id = ?", req.ToClaw).First(&to).Error; err != nil {
			to = model.CreditAccount{
				ID:     uuid.New().String(),
				ClawID: req.ToClaw,
				Status: "active",
			}
			tx.Create(&to)
		}
		tx.Model(&to).Updates(map[string]interface{}{
			"balance":  gorm.Expr("balance + ?", req.Amount),
			"total_in": gorm.Expr("total_in + ?", req.Amount),
		})

		// Record transaction
		txnID = uuid.New().String()
		txn := model.CreditTransaction{
			ID:        txnID,
			FromClaw:  req.FromClaw,
			ToClaw:    req.ToClaw,
			Amount:    req.Amount,
			Type:      "transfer",
			Nonce:     req.Nonce,
			Signature: req.Signature,
			Remark:    req.Remark,
			Status:    "confirmed",
		}
		tx.Create(&txn)

		newBalance = from.Balance - req.Amount
		return nil
	})

	if err != nil {
		log.Printf("[credit] transfer failed: %v", err)
		middleware.Fail(c, http.StatusBadRequest, middleware.CodeBillingInsufficient, err.Error())
		return
	}

	middleware.OK(c, gin.H{
		"txn_id":        txnID,
		"from":          req.FromClaw,
		"to":            req.ToClaw,
		"amount":        req.Amount,
		"amount_stars":  float64(req.Amount) / float64(StarUnit),
		"new_balance":   newBalance,
	})
}

// ─── Public API: GET /v1/credits/transactions ───

func (h *CreditHandler) ListTransactions(c *gin.Context) {
	clawID := c.Query("claw_id")
	if clawID == "" {
		middleware.Fail(c, http.StatusBadRequest, middleware.CodeBadRequest, "缺少 claw_id")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize > 100 {
		pageSize = 100
	}

	var txns []model.CreditTransaction
	var total int64

	db := database.DB.Model(&model.CreditTransaction{}).
		Where("from_claw = ? OR to_claw = ?", clawID, clawID)

	if txnType := c.Query("type"); txnType != "" {
		db = db.Where("type = ?", txnType)
	}

	db.Count(&total)
	db.Order("created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&txns)

	middleware.OK(c, gin.H{
		"transactions": txns,
		"total":        total,
		"page":         page,
		"page_size":    pageSize,
	})
}

// ─── Internal API: POST /internal/credits/grant ───
// Queen grants credits to a claw (welcome bonus, mining rewards, etc.)

func (h *CreditHandler) InternalGrant(c *gin.Context) {
	var req struct {
		ClawID    string `json:"claw_id" binding:"required"`
		Amount    int64  `json:"amount" binding:"required"`
		Type      string `json:"type" binding:"required"` // grant / mining_reward / bounty / referral
		Remark    string `json:"remark"`
		PublicKey string `json:"public_key"` // optional, store if provided
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数不完整"})
		return
	}
	if req.Amount <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "金额必须大于 0"})
		return
	}

	db := database.DB
	var newBalance int64

	err := db.Transaction(func(tx *gorm.DB) error {
		var acct model.CreditAccount
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("claw_id = ?", req.ClawID).First(&acct).Error; err != nil {
			acct = model.CreditAccount{
				ID:        uuid.New().String(),
				ClawID:    req.ClawID,
				PublicKey: req.PublicKey,
				Status:    "active",
			}
			tx.Create(&acct)
		}

		if req.PublicKey != "" && acct.PublicKey == "" {
			tx.Model(&acct).Update("public_key", req.PublicKey)
		}

		tx.Model(&acct).Updates(map[string]interface{}{
			"balance":  gorm.Expr("balance + ?", req.Amount),
			"total_in": gorm.Expr("total_in + ?", req.Amount),
		})

		txn := model.CreditTransaction{
			ID:       uuid.New().String(),
			FromClaw: "system",
			ToClaw:   req.ClawID,
			Amount:   req.Amount,
			Type:     req.Type,
			Remark:   req.Remark,
			Status:   "confirmed",
		}
		tx.Create(&txn)

		newBalance = acct.Balance + req.Amount
		return nil
	})

	if err != nil {
		log.Printf("[credit] grant failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"claw_id":     req.ClawID,
		"granted":     req.Amount,
		"new_balance": newBalance,
	})
}

// ─── Internal API: POST /internal/credits/consume ───
// Deduct credits for API usage (called by Router after serving a request)

func (h *CreditHandler) InternalConsume(c *gin.Context) {
	var req struct {
		ClawID       string `json:"claw_id" binding:"required"`
		Amount       int64  `json:"amount" binding:"required"`
		ResourceType string `json:"resource_type"` // tokens / image / video / sandbox
		Quantity     int64  `json:"quantity"`       // e.g. token count
		Remark       string `json:"remark"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数不完整"})
		return
	}

	// Auto-calculate if amount not provided
	if req.Amount == 0 && req.Quantity > 0 {
		req.Amount = calculateStarCost(req.ResourceType, req.Quantity)
	}
	if req.Amount <= 0 {
		c.JSON(http.StatusOK, gin.H{"deducted": 0, "balance": ensureCreditAccount(req.ClawID).Balance})
		return
	}

	db := database.DB
	var newBalance int64

	err := db.Transaction(func(tx *gorm.DB) error {
		var acct model.CreditAccount
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("claw_id = ?", req.ClawID).First(&acct).Error; err != nil {
			return fmt.Errorf("账户不存在: %s", req.ClawID)
		}
		if acct.Balance < req.Amount {
			return fmt.Errorf("余额不足")
		}

		tx.Model(&acct).Updates(map[string]interface{}{
			"balance":   gorm.Expr("balance - ?", req.Amount),
			"total_out": gorm.Expr("total_out + ?", req.Amount),
		})

		// Update HP status
		remaining := acct.Balance - req.Amount
		newStatus := "active"
		if remaining <= 0 {
			newStatus = "hibernated"
		}
		if acct.Status != newStatus {
			tx.Model(&acct).Update("status", newStatus)
		}

		remark := req.Remark
		if remark == "" {
			remark = fmt.Sprintf("%s x %d", req.ResourceType, req.Quantity)
		}

		txn := model.CreditTransaction{
			ID:       uuid.New().String(),
			FromClaw: req.ClawID,
			ToClaw:   "system",
			Amount:   req.Amount,
			Type:     "consume",
			Remark:   remark,
			Status:   "confirmed",
		}
		tx.Create(&txn)

		newBalance = remaining
		return nil
	})

	if err != nil {
		log.Printf("[credit] consume failed: %v", err)
		c.JSON(http.StatusPaymentRequired, gin.H{"error": err.Error(), "code": middleware.CodeBillingInsufficient})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"claw_id":  req.ClawID,
		"deducted": req.Amount,
		"balance":  newBalance,
	})
}

// ─── Internal API: POST /internal/credits/freeze ───

func (h *CreditHandler) InternalFreeze(c *gin.Context) {
	var req struct {
		ClawID string `json:"claw_id" binding:"required"`
		Amount int64  `json:"amount" binding:"required"`
		Reason string `json:"reason" binding:"required"` // bounty / deposit
		RefID  string `json:"ref_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数不完整"})
		return
	}

	db := database.DB
	var freezeID string

	err := db.Transaction(func(tx *gorm.DB) error {
		var acct model.CreditAccount
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("claw_id = ?", req.ClawID).First(&acct).Error; err != nil {
			return fmt.Errorf("账户不存在")
		}
		if acct.Balance < req.Amount {
			return fmt.Errorf("可用余额不足，无法冻结")
		}

		tx.Model(&acct).Updates(map[string]interface{}{
			"balance": gorm.Expr("balance - ?", req.Amount),
			"frozen":  gorm.Expr("frozen + ?", req.Amount),
		})

		freezeID = uuid.New().String()
		freeze := model.CreditFreeze{
			ID:     freezeID,
			ClawID: req.ClawID,
			Amount: req.Amount,
			Reason: req.Reason,
			RefID:  req.RefID,
			Status: "frozen",
		}
		tx.Create(&freeze)

		txn := model.CreditTransaction{
			ID:       uuid.New().String(),
			FromClaw: req.ClawID,
			ToClaw:   "frozen",
			Amount:   req.Amount,
			Type:     "freeze",
			Remark:   fmt.Sprintf("%s freeze: %s", req.Reason, req.RefID),
			Status:   "confirmed",
		}
		tx.Create(&txn)

		return nil
	})

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"freeze_id": freezeID, "frozen": req.Amount})
}

// ─── Internal API: POST /internal/credits/unfreeze ───

func (h *CreditHandler) InternalUnfreeze(c *gin.Context) {
	var req struct {
		FreezeID string `json:"freeze_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数不完整"})
		return
	}

	db := database.DB

	err := db.Transaction(func(tx *gorm.DB) error {
		var freeze model.CreditFreeze
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND status = ?", req.FreezeID, "frozen").First(&freeze).Error; err != nil {
			return fmt.Errorf("冻结记录不存在或已解冻")
		}

		now := time.Now()
		tx.Model(&freeze).Updates(map[string]interface{}{
			"status":      "released",
			"released_at": &now,
		})

		tx.Model(&model.CreditAccount{}).Where("claw_id = ?", freeze.ClawID).Updates(map[string]interface{}{
			"balance": gorm.Expr("balance + ?", freeze.Amount),
			"frozen":  gorm.Expr("frozen - ?", freeze.Amount),
		})

		txn := model.CreditTransaction{
			ID:       uuid.New().String(),
			FromClaw: "frozen",
			ToClaw:   freeze.ClawID,
			Amount:   freeze.Amount,
			Type:     "unfreeze",
			Remark:   fmt.Sprintf("unfreeze %s: %s", freeze.Reason, freeze.RefID),
			Status:   "confirmed",
		}
		tx.Create(&txn)

		return nil
	})

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "released"})
}

// ─── Internal API: POST /internal/credits/settle ───
// Settle a frozen amount to another claw (bounty completion, etc.)

func (h *CreditHandler) InternalSettle(c *gin.Context) {
	var req struct {
		FreezeID string `json:"freeze_id" binding:"required"`
		ToClaw   string `json:"to_claw" binding:"required"`
		FeeRate  float64 `json:"fee_rate"` // 0.05 = 5% platform fee
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数不完整"})
		return
	}

	db := database.DB

	err := db.Transaction(func(tx *gorm.DB) error {
		var freeze model.CreditFreeze
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND status = ?", req.FreezeID, "frozen").First(&freeze).Error; err != nil {
			return fmt.Errorf("冻结记录不存在或已结算")
		}

		fee := int64(float64(freeze.Amount) * req.FeeRate)
		payout := freeze.Amount - fee

		now := time.Now()
		tx.Model(&freeze).Updates(map[string]interface{}{
			"status":      "settled",
			"released_at": &now,
		})

		// Deduct frozen from sender
		tx.Model(&model.CreditAccount{}).Where("claw_id = ?", freeze.ClawID).Updates(map[string]interface{}{
			"frozen":    gorm.Expr("frozen - ?", freeze.Amount),
			"total_out": gorm.Expr("total_out + ?", freeze.Amount),
		})

		// Credit receiver
		var to model.CreditAccount
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("claw_id = ?", req.ToClaw).First(&to).Error; err != nil {
			to = model.CreditAccount{
				ID:     uuid.New().String(),
				ClawID: req.ToClaw,
				Status: "active",
			}
			tx.Create(&to)
		}
		tx.Model(&to).Updates(map[string]interface{}{
			"balance":  gorm.Expr("balance + ?", payout),
			"total_in": gorm.Expr("total_in + ?", payout),
		})

		// Record settlement transaction
		txn := model.CreditTransaction{
			ID:       uuid.New().String(),
			FromClaw: freeze.ClawID,
			ToClaw:   req.ToClaw,
			Amount:   payout,
			Fee:      fee,
			Type:     "settle",
			Remark:   fmt.Sprintf("settle %s: %s (fee: %d)", freeze.Reason, freeze.RefID, fee),
			Status:   "confirmed",
		}
		tx.Create(&txn)

		return nil
	})

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "settled"})
}

// ─── Internal API: GET /internal/credits/balance/:claw_id ───

func (h *CreditHandler) InternalGetBalance(c *gin.Context) {
	clawID := c.Param("claw_id")
	if clawID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing claw_id"})
		return
	}
	// URL param may be encoded, reconstruct claw:xxx format
	if !strings.HasPrefix(clawID, "claw:") {
		clawID = "claw:" + clawID
	}

	acct := ensureCreditAccount(clawID)
	c.JSON(http.StatusOK, gin.H{
		"claw_id":   acct.ClawID,
		"balance":   acct.Balance,
		"frozen":    acct.Frozen,
		"status":    acct.Status,
		"has_quota": acct.Balance > 0,
	})
}

// ─── Pricing: calculate star cost by resource type ───

func calculateStarCost(resourceType string, quantity int64) int64 {
	switch resourceType {
	case "input_tokens":
		// 0.5 ⭐ per 1K tokens = 5000 units per 1K = 5 units per token
		return quantity * 5
	case "output_tokens":
		// 1 ⭐ per 1K tokens = 10000 units per 1K = 10 units per token
		return quantity * 10
	case "image":
		return quantity * 5 * int64(StarUnit) // 5 ⭐ per image
	case "image_hd":
		return quantity * 10 * int64(StarUnit) // 10 ⭐ per HD image
	case "video_short":
		return quantity * 50 * int64(StarUnit) // 50 ⭐ per short video
	case "video_long":
		return quantity * 200 * int64(StarUnit) // 200 ⭐ per long video
	case "sandbox_min":
		return quantity * 1 * int64(StarUnit) // 1 ⭐ per minute
	default:
		return quantity // raw units
	}
}
