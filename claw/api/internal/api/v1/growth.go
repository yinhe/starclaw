package v1

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw/internal/growth"
	"github.com/yinhe/starclaw/internal/model"
	"github.com/yinhe/starclaw/internal/node"
	"github.com/yinhe/starclaw/internal/provider"
	"gorm.io/gorm"
)

type GrowthHandler struct {
	db               *gorm.DB
	providerRegistry *provider.Registry
	identity         *node.Identity
}

func NewGrowthHandler(db *gorm.DB, providerRegistry *provider.Registry, identity *node.Identity) *GrowthHandler {
	return &GrowthHandler{db: db, providerRegistry: providerRegistry, identity: identity}
}

// GetGrowth returns the full growth profile for this Claw node.
// GET /v1/growth
func (h *GrowthHandler) GetGrowth(c *gin.Context) {
	userID := c.GetString("user_id")

	profile, err := growth.BuildProfile(h.db, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to compute growth profile"})
		return
	}

	// Sync fighter stats to Chrysalis (async, fire-and-forget)
	if h.identity != nil && h.identity.NodeID != "" {
		growth.SyncToChrysalis(profile, h.identity.NodeID)
	}

	c.JSON(http.StatusOK, profile)
}

// GetMilestones returns milestones for this Claw node.
// GET /v1/growth/milestones
func (h *GrowthHandler) GetMilestones(c *gin.Context) {
	userID := c.GetString("user_id")

	var milestones []model.Milestone
	h.db.Where("user_id = ?", userID).
		Order("achieved_at ASC").Find(&milestones)

	// Mark unnotified milestones as notified
	now := time.Now()
	for i := range milestones {
		if milestones[i].NotifiedAt == nil {
			h.db.Model(&milestones[i]).Update("notified_at", now)
			milestones[i].NotifiedAt = &now
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"milestones": milestones,
		"total":      len(milestones),
	})
}

// GetDailyReport returns a daily activity report for this Claw node.
// GET /v1/growth/daily-report?date=2026-03-27
func (h *GrowthHandler) GetDailyReport(c *gin.Context) {
	userID := c.GetString("user_id")

	// Parse date (default: yesterday)
	date := time.Now().Add(-24 * time.Hour)
	if dateStr := c.Query("date"); dateStr != "" {
		if parsed, err := time.Parse("2006-01-02", dateStr); err == nil {
			date = parsed
		}
	}

	// Try to get a provider for LLM summary
	var p provider.ModelProvider
	if h.providerRegistry != nil {
		if starAI, ok := h.providerRegistry.Get("star-ai"); ok {
			p = starAI
		}
	}

	report, err := growth.GenerateDailyReport(h.db, p, userID, date)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate report"})
		return
	}

	c.JSON(http.StatusOK, report)
}

// GetGrowthCurve returns daily stats for recent days.
// GET /v1/growth/curve?days=7
func (h *GrowthHandler) GetGrowthCurve(c *gin.Context) {
	userID := c.GetString("user_id")

	days := 7
	if d := c.Query("days"); d != "" {
		if n, err := strconv.Atoi(d); err == nil && n > 0 && n <= 90 {
			days = n
		}
	}

	curve := growth.GrowthCurve(h.db, userID, days)
	c.JSON(http.StatusOK, gin.H{"curve": curve, "days": days})
}

// GetAssets returns the user's digital asset overview.
// GET /v1/assets/overview
func (h *GrowthHandler) GetAssets(c *gin.Context) {
	userID := c.GetString("user_id")

	clawID := ""
	onlineDays := 0
	if h.identity != nil {
		clawID = h.identity.NodeID
	}

	overview := growth.BuildAssetOverview(h.db, userID, clawID, onlineDays)
	c.JSON(http.StatusOK, overview)
}

// GetNewMilestones returns only unnotified milestones (for popup).
// GET /v1/growth/milestones/new
func (h *GrowthHandler) GetNewMilestones(c *gin.Context) {
	userID := c.GetString("user_id")

	var milestones []model.Milestone
	h.db.Where("user_id = ? AND notified_at IS NULL", userID).
		Order("achieved_at ASC").Find(&milestones)

	// Mark as notified
	now := time.Now()
	for i := range milestones {
		h.db.Model(&milestones[i]).Update("notified_at", now)
		milestones[i].NotifiedAt = &now
	}

	c.JSON(http.StatusOK, gin.H{
		"milestones": milestones,
		"total":      len(milestones),
	})
}
