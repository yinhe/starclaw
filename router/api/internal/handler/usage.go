package handler

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw-router/internal/model"
	"gorm.io/gorm"
)

type UsageHandler struct {
	db *gorm.DB
}

func NewUsageHandler(db *gorm.DB) *UsageHandler {
	return &UsageHandler{db: db}
}

// Query returns usage records for the authenticated user
func (h *UsageHandler) Query(c *gin.Context) {
	userID := c.GetString("user_id")

	// Default to last 7 days
	days := 7
	if d := c.Query("days"); d != "" {
		if _, err := time.ParseDuration(d + "h"); err == nil {
			// ignore invalid
		}
	}

	since := time.Now().AddDate(0, 0, -days)

	var records []model.UsageRecord
	h.db.Where("user_id = ? AND created_at >= ?", userID, since).
		Order("created_at DESC").
		Limit(500).
		Find(&records)

	// Summary
	var totalTokens int64
	var totalCost int64
	var totalRequests int64
	for _, r := range records {
		totalTokens += int64(r.TotalTokens)
		totalCost += r.CostCents
		totalRequests++
	}

	c.JSON(http.StatusOK, gin.H{
		"records":        records,
		"total_tokens":   totalTokens,
		"total_cost":     totalCost,
		"total_requests": totalRequests,
		"days":           days,
	})
}

// Logs returns detailed per-request logs with pagination and filtering
func (h *UsageHandler) Logs(c *gin.Context) {
	userID := c.GetString("user_id")

	page := 1
	pageSize := 50
	if p := c.Query("page"); p != "" {
		fmt.Sscanf(p, "%d", &page)
	}
	if ps := c.Query("page_size"); ps != "" {
		fmt.Sscanf(ps, "%d", &pageSize)
	}
	if page < 1 {
		page = 1
	}
	if pageSize > 200 {
		pageSize = 200
	}

	query := h.db.Where("user_id = ?", userID)

	// Filter by model
	if m := c.Query("model"); m != "" {
		query = query.Where("model LIKE ?", "%"+m+"%")
	}
	// Filter by status
	if s := c.Query("status"); s != "" {
		query = query.Where("status = ?", s)
	}
	// Filter by date range
	if from := c.Query("from"); from != "" {
		if t, err := time.Parse("2006-01-02", from); err == nil {
			query = query.Where("created_at >= ?", t)
		}
	}
	if to := c.Query("to"); to != "" {
		if t, err := time.Parse("2006-01-02", to); err == nil {
			query = query.Where("created_at < ?", t.AddDate(0, 0, 1))
		}
	}

	var total int64
	query.Model(&model.UsageRecord{}).Count(&total)

	var records []model.UsageRecord
	query.Order("created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&records)

	c.JSON(http.StatusOK, gin.H{
		"logs":      records,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"pages":     (total + int64(pageSize) - 1) / int64(pageSize),
	})
}

// Balance returns the user's current balance
func (h *UsageHandler) Balance(c *gin.Context) {
	userID := c.GetString("user_id")

	var user model.User
	if err := h.db.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"balance_cents": user.Balance,
		"free_quota":    user.FreeQuota,
	})
}
