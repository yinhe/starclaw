package handler

import (
	"log"
	"math"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"starclaw.net/queen/bounty/internal/billing"
	"starclaw.net/queen/bounty/internal/model"
	"gorm.io/gorm"
)

const platformFeeRate = 0.05 // 5% platform service fee

type BountyHandler struct {
	db      *gorm.DB
	billing *billing.Client // nil = billing disabled (no fund escrow)
}

func NewBountyHandler(db *gorm.DB, billingClient *billing.Client) *BountyHandler {
	return &BountyHandler{db: db, billing: billingClient}
}

// rewardToCents converts CNY float to 分 int64
func rewardToCents(reward float64) int64 {
	return int64(math.Round(reward * 100))
}

// ---------- POST /bounties — Create (posted by Claw via BountyTool) ----------

type CreateRequest struct {
	NodeID        string  `json:"node_id" binding:"required"`
	UserID        string  `json:"user_id" binding:"required"`
	Title         string  `json:"title" binding:"required"`
	Description   string  `json:"description"`
	Category      string  `json:"category"`
	Requirements  string  `json:"requirements"`
	Reward        float64 `json:"reward" binding:"required,gt=0"`
	DeadlineHours int     `json:"deadline_hours"` // hours from now, 0 = no deadline
}

func (h *BountyHandler) Create(c *gin.Context) {
	var req CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	bounty := model.Bounty{
		NodeID:       req.NodeID,
		UserID:       req.UserID,
		Title:        req.Title,
		Description:  req.Description,
		Category:     model.BountyCategory(req.Category),
		Requirements: req.Requirements,
		Reward:       req.Reward,
		Status:       model.BountyOpen,
	}
	if req.DeadlineHours > 0 {
		dl := time.Now().Add(time.Duration(req.DeadlineHours) * time.Hour)
		bounty.Deadline = &dl
	}
	if bounty.Category == "" {
		bounty.Category = model.CatOther
	}

	// Freeze reward from creator's balance (if billing enabled)
	if h.billing != nil {
		cents := rewardToCents(bounty.Reward)
		if err := h.billing.Freeze(req.UserID, cents, bounty.ID); err != nil {
			log.Printf("[bounty] Billing freeze failed: %v", err)
			c.JSON(http.StatusPaymentRequired, gin.H{"error": "余额不足，无法冻结赏金: " + err.Error()})
			return
		}
	}

	if err := h.db.Create(&bounty).Error; err != nil {
		// Rollback freeze if DB create fails
		if h.billing != nil {
			_ = h.billing.Unfreeze(req.UserID, rewardToCents(bounty.Reward), bounty.ID)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create bounty"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"bounty": bounty})
}

// ---------- GET /bounties — List (marketplace for humans) ----------

func (h *BountyHandler) List(c *gin.Context) {
	status := c.DefaultQuery("status", "open")
	category := c.Query("category")

	q := h.db.Order("reward DESC, created_at DESC")
	if status != "all" {
		q = q.Where("status = ?", status)
	}
	if category != "" {
		q = q.Where("category = ?", category)
	}

	var bounties []model.Bounty
	if err := q.Limit(100).Find(&bounties).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list bounties"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"bounties": bounties, "total": len(bounties)})
}

// ---------- GET /bounties/:id ----------

func (h *BountyHandler) Get(c *gin.Context) {
	id := c.Param("id")
	var bounty model.Bounty
	if err := h.db.First(&bounty, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "bounty not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"bounty": bounty})
}

// ---------- POST /bounties/:id/claim — Human claims a bounty ----------

func (h *BountyHandler) Claim(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		UserID string `json:"user_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var bounty model.Bounty
	if err := h.db.First(&bounty, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "bounty not found"})
		return
	}
	if bounty.Status != model.BountyOpen {
		c.JSON(http.StatusConflict, gin.H{"error": "bounty is not open for claiming"})
		return
	}

	now := time.Now()
	h.db.Model(&bounty).Updates(map[string]interface{}{
		"status":     model.BountyClaimed,
		"claimed_by": req.UserID,
		"claimed_at": &now,
	})

	c.JSON(http.StatusOK, gin.H{"message": "bounty claimed", "bounty_id": id})
}

// ---------- POST /bounties/:id/deliver — Human submits deliverable ----------

func (h *BountyHandler) Deliver(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		UserID       string `json:"user_id" binding:"required"`
		DeliveryNote string `json:"delivery_note"`
		DeliveryURL  string `json:"delivery_url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var bounty model.Bounty
	if err := h.db.First(&bounty, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "bounty not found"})
		return
	}
	if bounty.Status != model.BountyClaimed {
		c.JSON(http.StatusConflict, gin.H{"error": "bounty is not in claimed state"})
		return
	}
	if bounty.ClaimedBy != req.UserID {
		c.JSON(http.StatusForbidden, gin.H{"error": "only the claimer can deliver"})
		return
	}

	now := time.Now()
	h.db.Model(&bounty).Updates(map[string]interface{}{
		"status":        model.BountyDelivered,
		"delivery_note": req.DeliveryNote,
		"delivery_url":  req.DeliveryURL,
		"delivered_at":  &now,
	})

	c.JSON(http.StatusOK, gin.H{"message": "delivery submitted", "bounty_id": id})
}

// ---------- POST /bounties/:id/accept — Claw accepts delivery ----------

func (h *BountyHandler) Accept(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		NodeID   string `json:"node_id" binding:"required"`
		Rating   int    `json:"rating"` // 1-5
		Feedback string `json:"feedback"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var bounty model.Bounty
	if err := h.db.First(&bounty, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "bounty not found"})
		return
	}
	if bounty.Status != model.BountyDelivered {
		c.JSON(http.StatusConflict, gin.H{"error": "bounty has no pending delivery"})
		return
	}

	now := time.Now()
	rating := req.Rating
	if rating < 1 {
		rating = 5
	}

	// Settle funds: frozen amount from creator → completer (minus platform fee)
	if h.billing != nil && bounty.ClaimedBy != "" {
		cents := rewardToCents(bounty.Reward)
		if err := h.billing.Settle(bounty.UserID, bounty.ClaimedBy, cents, platformFeeRate, bounty.ID); err != nil {
			log.Printf("[bounty] Billing settle failed: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "资金结算失败: " + err.Error()})
			return
		}
	}

	h.db.Model(&bounty).Updates(map[string]interface{}{
		"status":       model.BountyCompleted,
		"completed_at": &now,
		"rating":       rating,
		"feedback":     req.Feedback,
	})

	// Update claimer stats
	if bounty.ClaimedBy != "" {
		h.db.Model(&model.BountyUser{}).Where("id = ?", bounty.ClaimedBy).
			UpdateColumns(map[string]interface{}{
				"completed_count": gorm.Expr("completed_count + 1"),
				"total_earned":    gorm.Expr("total_earned + ?", bounty.Reward),
			})
	}

	c.JSON(http.StatusOK, gin.H{"message": "delivery accepted, bounty completed", "bounty_id": id})
}

// ---------- POST /bounties/:id/cancel — Cancel bounty ----------

func (h *BountyHandler) Cancel(c *gin.Context) {
	id := c.Param("id")

	var bounty model.Bounty
	if err := h.db.First(&bounty, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "bounty not found"})
		return
	}
	if bounty.Status == model.BountyCompleted {
		c.JSON(http.StatusConflict, gin.H{"error": "cannot cancel a completed bounty"})
		return
	}

	// Unfreeze funds back to creator
	if h.billing != nil {
		cents := rewardToCents(bounty.Reward)
		if err := h.billing.Unfreeze(bounty.UserID, cents, bounty.ID); err != nil {
			log.Printf("[bounty] Billing unfreeze on cancel failed: %v", err)
			// Don't block cancellation — log and continue
		}
	}

	h.db.Model(&bounty).Update("status", model.BountyCancelled)
	c.JSON(http.StatusOK, gin.H{"message": "bounty cancelled", "bounty_id": id})
}

// ---------- POST /bounties/:id/dispute — Raise dispute ----------

func (h *BountyHandler) Dispute(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Reason string `json:"reason" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var bounty model.Bounty
	if err := h.db.First(&bounty, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "bounty not found"})
		return
	}

	h.db.Model(&bounty).Update("status", model.BountyDisputed)
	c.JSON(http.StatusOK, gin.H{"message": "dispute raised", "bounty_id": id})
}

// ---------- GET /bounties/stats ----------

func (h *BountyHandler) Stats(c *gin.Context) {
	var total, open, claimed, completed, cancelled int64
	var totalReward float64

	h.db.Model(&model.Bounty{}).Count(&total)
	h.db.Model(&model.Bounty{}).Where("status = ?", model.BountyOpen).Count(&open)
	h.db.Model(&model.Bounty{}).Where("status = ?", model.BountyClaimed).Count(&claimed)
	h.db.Model(&model.Bounty{}).Where("status = ?", model.BountyCompleted).Count(&completed)
	h.db.Model(&model.Bounty{}).Where("status = ?", model.BountyCancelled).Count(&cancelled)

	h.db.Model(&model.Bounty{}).Where("status = ?", model.BountyCompleted).
		Select("COALESCE(SUM(reward), 0)").Scan(&totalReward)

	c.JSON(http.StatusOK, gin.H{
		"total":             total,
		"open":              open,
		"claimed":           claimed,
		"completed":         completed,
		"cancelled":         cancelled,
		"total_reward_paid": totalReward,
	})
}

// ---------- GET /bounties/categories ----------

func (h *BountyHandler) Categories(c *gin.Context) {
	cats := []gin.H{
		{"id": "data_labeling", "name": "数据标注", "name_en": "Data Labeling"},
		{"id": "content_review", "name": "内容审核", "name_en": "Content Review"},
		{"id": "creative_design", "name": "创意设计", "name_en": "Creative Design"},
		{"id": "real_world", "name": "现实操作", "name_en": "Real-World Task"},
		{"id": "expert_consult", "name": "专业咨询", "name_en": "Expert Consultation"},
		{"id": "code_review", "name": "代码审查", "name_en": "Code Review"},
		{"id": "other", "name": "其他", "name_en": "Other"},
	}
	c.JSON(http.StatusOK, gin.H{"categories": cats})
}
