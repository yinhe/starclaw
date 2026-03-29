package v1

import (
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw/internal/model"
	"gorm.io/gorm"
)

// MarketplaceHandler manages the Agent economy: listings, purchases, revenue, ratings.
type MarketplaceHandler struct {
	db *gorm.DB
}

// NewMarketplaceHandler creates the marketplace handler.
func NewMarketplaceHandler(db *gorm.DB) *MarketplaceHandler {
	return &MarketplaceHandler{db: db}
}

// ════════════════════════════════════════════════════════════════
//  Creator Profile
// ════════════════════════════════════════════════════════════════

// GetCreatorProfile returns the current user's creator profile.
func (h *MarketplaceHandler) GetCreatorProfile(c *gin.Context) {
	userID := c.GetString("user_id")
	var profile model.CreatorProfile
	if err := h.db.Where("user_id = ?", userID).First(&profile).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "creator profile not found, please register first"})
		return
	}
	c.JSON(http.StatusOK, profile)
}

// RegisterCreator creates or updates a creator profile.
func (h *MarketplaceHandler) RegisterCreator(c *gin.Context) {
	userID := c.GetString("user_id")
	var req struct {
		DisplayName  string `json:"display_name"`
		Bio          string `json:"bio"`
		AvatarURL    string `json:"avatar_url"`
		Website      string `json:"website"`
		PayoutMethod string `json:"payout_method"`
		PayoutInfo   string `json:"payout_info"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var profile model.CreatorProfile
	h.db.Where("user_id = ?", userID).First(&profile)

	profile.UserID = userID
	profile.DisplayName = req.DisplayName
	profile.Bio = req.Bio
	profile.AvatarURL = req.AvatarURL
	profile.Website = req.Website
	if req.PayoutMethod != "" {
		profile.PayoutMethod = req.PayoutMethod
	}
	if req.PayoutInfo != "" {
		profile.PayoutInfo = req.PayoutInfo
	}

	if profile.ID == "" {
		if err := h.db.Create(&profile).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create profile"})
			return
		}
	} else {
		h.db.Save(&profile)
	}

	c.JSON(http.StatusOK, profile)
}

// ════════════════════════════════════════════════════════════════
//  Agent Listings
// ════════════════════════════════════════════════════════════════

// CreateListing creates a new listing for an agent template.
func (h *MarketplaceHandler) CreateListing(c *gin.Context) {
	userID := c.GetString("user_id")
	var req struct {
		TemplateID     string `json:"template_id" binding:"required"`
		Pricing        string `json:"pricing"`
		PriceCents     int    `json:"price_cents"`
		MonthlyPricing int    `json:"monthly_pricing"`
		Screenshots    string `json:"screenshots"`
		DemoURL        string `json:"demo_url"`
		Version        string `json:"version"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Verify template ownership
	var tmpl model.AgentTemplate
	if err := h.db.Where("id = ? AND author_id = ?", req.TemplateID, userID).First(&tmpl).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "template not found or not owned by you"})
		return
	}

	// Check creator profile exists
	var profile model.CreatorProfile
	if err := h.db.Where("user_id = ?", userID).First(&profile).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "please register as a creator first"})
		return
	}

	pricing := model.PricingFree
	if req.Pricing == "one_time" {
		pricing = model.PricingOneTime
	} else if req.Pricing == "subscription" {
		pricing = model.PricingSubscription
	}

	listing := model.AgentListing{
		TemplateID:     req.TemplateID,
		CreatorID:      userID,
		Pricing:        pricing,
		PriceCents:     req.PriceCents,
		MonthlyPricing: req.MonthlyPricing,
		Screenshots:    req.Screenshots,
		DemoURL:        req.DemoURL,
		Version:        req.Version,
		Status:         "pending_review",
	}
	if listing.Version == "" {
		listing.Version = "1.0.0"
	}

	if err := h.db.Create(&listing).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create listing"})
		return
	}

	c.JSON(http.StatusCreated, listing)
}

// ListPublished returns all published listings (public marketplace).
func (h *MarketplaceHandler) ListPublished(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	category := c.Query("category")
	pricing := c.Query("pricing")
	sort := c.DefaultQuery("sort", "popular") // popular, newest, rating, price_asc, price_desc
	search := c.Query("q")

	if page < 1 {
		page = 1
	}
	if pageSize > 50 {
		pageSize = 50
	}

	query := h.db.Model(&model.AgentListing{}).Where("status = ?", "published")
	query = query.Preload("Template").Preload("Creator")

	if category != "" {
		query = query.Joins("JOIN agent_templates ON agent_templates.id = agent_listings.template_id").
			Where("agent_templates.category = ?", category)
	}
	if pricing != "" {
		query = query.Where("agent_listings.pricing = ?", pricing)
	}
	if search != "" {
		query = query.Joins("JOIN agent_templates t2 ON t2.id = agent_listings.template_id").
			Where("t2.name LIKE ? OR t2.description LIKE ?", "%"+search+"%", "%"+search+"%")
	}

	var total int64
	query.Count(&total)

	switch sort {
	case "newest":
		query = query.Order("agent_listings.created_at DESC")
	case "rating":
		query = query.Joins("JOIN agent_templates t3 ON t3.id = agent_listings.template_id").
			Order("t3.rating DESC")
	case "price_asc":
		query = query.Order("agent_listings.price_cents ASC")
	case "price_desc":
		query = query.Order("agent_listings.price_cents DESC")
	default: // popular
		query = query.Order("agent_listings.sales_count DESC")
	}

	var listings []model.AgentListing
	query.Offset((page - 1) * pageSize).Limit(pageSize).Find(&listings)

	c.JSON(http.StatusOK, gin.H{
		"items":       listings,
		"total":       total,
		"page":        page,
		"page_size":   pageSize,
		"total_pages": int(math.Ceil(float64(total) / float64(pageSize))),
	})
}

// GetListing returns a single listing detail.
func (h *MarketplaceHandler) GetListing(c *gin.Context) {
	id := c.Param("id")
	var listing model.AgentListing
	if err := h.db.Preload("Template").Preload("Creator").Where("id = ?", id).First(&listing).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "listing not found"})
		return
	}

	// Get ratings summary
	var avgScore float64
	var ratingCount int64
	h.db.Model(&model.AgentRating{}).Where("listing_id = ?", id).Count(&ratingCount)
	if ratingCount > 0 {
		h.db.Model(&model.AgentRating{}).Where("listing_id = ?", id).Select("AVG(score)").Scan(&avgScore)
	}

	// Get version history
	var versions []model.AgentVersion
	h.db.Where("listing_id = ?", id).Order("created_at DESC").Limit(10).Find(&versions)

	c.JSON(http.StatusOK, gin.H{
		"listing":      listing,
		"avg_rating":   avgScore,
		"rating_count": ratingCount,
		"versions":     versions,
	})
}

// MyListings returns the current creator's listings.
func (h *MarketplaceHandler) MyListings(c *gin.Context) {
	userID := c.GetString("user_id")
	var listings []model.AgentListing
	h.db.Preload("Template").Where("creator_id = ?", userID).Order("created_at DESC").Find(&listings)
	c.JSON(http.StatusOK, listings)
}

// UpdateListing updates a listing's metadata.
func (h *MarketplaceHandler) UpdateListing(c *gin.Context) {
	userID := c.GetString("user_id")
	id := c.Param("id")

	var listing model.AgentListing
	if err := h.db.Where("id = ? AND creator_id = ?", id, userID).First(&listing).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "listing not found"})
		return
	}

	var req struct {
		Pricing        *string `json:"pricing"`
		PriceCents     *int    `json:"price_cents"`
		MonthlyPricing *int    `json:"monthly_pricing"`
		Screenshots    *string `json:"screenshots"`
		DemoURL        *string `json:"demo_url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Pricing != nil {
		listing.Pricing = model.PricingModel(*req.Pricing)
	}
	if req.PriceCents != nil {
		listing.PriceCents = *req.PriceCents
	}
	if req.MonthlyPricing != nil {
		listing.MonthlyPricing = *req.MonthlyPricing
	}
	if req.Screenshots != nil {
		listing.Screenshots = *req.Screenshots
	}
	if req.DemoURL != nil {
		listing.DemoURL = *req.DemoURL
	}

	h.db.Save(&listing)
	c.JSON(http.StatusOK, listing)
}

// ════════════════════════════════════════════════════════════════
//  Purchasing
// ════════════════════════════════════════════════════════════════

// PurchaseAgent processes a purchase of a listed agent.
func (h *MarketplaceHandler) PurchaseAgent(c *gin.Context) {
	userID := c.GetString("user_id")
	listingID := c.Param("id")

	var listing model.AgentListing
	if err := h.db.Where("id = ? AND status = ?", listingID, "published").First(&listing).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "listing not found or not published"})
		return
	}

	// Don't allow self-purchase
	if listing.CreatorID == userID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot purchase your own agent"})
		return
	}

	// Check for existing purchase
	var existing model.AgentPurchase
	if err := h.db.Where("listing_id = ? AND buyer_id = ? AND status = ?", listingID, userID, "completed").First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "already purchased", "purchase_id": existing.ID})
		return
	}

	// Determine price
	priceCents := listing.PriceCents
	if listing.Pricing == model.PricingSubscription {
		priceCents = listing.MonthlyPricing
	}

	// Create purchase
	purchase := model.AgentPurchase{
		ListingID:  listingID,
		BuyerID:    userID,
		CreatorID:  listing.CreatorID,
		PriceCents: priceCents,
		Currency:   listing.Currency,
		Status:     "completed",
	}
	if listing.Pricing == model.PricingSubscription {
		expiry := time.Now().AddDate(0, 1, 0)
		purchase.ExpiresAt = &expiry
	}

	tx := h.db.Begin()

	if err := tx.Create(&purchase).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "purchase failed"})
		return
	}

	// Update sales count and revenue
	tx.Model(&listing).Updates(map[string]interface{}{
		"sales_count": gorm.Expr("sales_count + 1"),
		"revenue":     gorm.Expr("revenue + ?", priceCents),
	})

	// Update template install count
	tx.Model(&model.AgentTemplate{}).Where("id = ?", listing.TemplateID).
		Update("install_count", gorm.Expr("install_count + 1"))

	// Create revenue record (80/15/5 split)
	if priceCents > 0 {
		platformFee := priceCents * 15 / 100
		referralFee := priceCents * 5 / 100
		netAmount := priceCents - platformFee - referralFee

		revenue := model.CreatorRevenue{
			CreatorID:   listing.CreatorID,
			PurchaseID:  purchase.ID,
			ListingID:   listingID,
			GrossAmount: priceCents,
			PlatformFee: platformFee,
			ReferralFee: referralFee,
			NetAmount:   netAmount,
			Currency:    listing.Currency,
			Status:      "pending",
		}
		tx.Create(&revenue)

		// Update creator profile total earned
		tx.Model(&model.CreatorProfile{}).Where("user_id = ?", listing.CreatorID).
			Update("total_earned", gorm.Expr("total_earned + ?", netAmount))
	}

	tx.Commit()

	c.JSON(http.StatusCreated, gin.H{
		"purchase_id": purchase.ID,
		"status":      "completed",
		"price":       priceCents,
	})
}

// MyPurchases returns the current user's purchased agents.
func (h *MarketplaceHandler) MyPurchases(c *gin.Context) {
	userID := c.GetString("user_id")
	var purchases []model.AgentPurchase
	h.db.Preload("Listing").Preload("Listing.Template").
		Where("buyer_id = ? AND status = ?", userID, "completed").
		Order("created_at DESC").Find(&purchases)
	c.JSON(http.StatusOK, purchases)
}

// CheckAccess checks if the user has access to a listing (purchased or free).
func (h *MarketplaceHandler) CheckAccess(c *gin.Context) {
	userID := c.GetString("user_id")
	listingID := c.Param("id")

	var listing model.AgentListing
	if err := h.db.Where("id = ?", listingID).First(&listing).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "listing not found"})
		return
	}

	// Creator always has access
	if listing.CreatorID == userID {
		c.JSON(http.StatusOK, gin.H{"has_access": true, "reason": "creator"})
		return
	}

	// Free agents are always accessible
	if listing.Pricing == model.PricingFree {
		c.JSON(http.StatusOK, gin.H{"has_access": true, "reason": "free"})
		return
	}

	// Check purchase
	var purchase model.AgentPurchase
	err := h.db.Where("listing_id = ? AND buyer_id = ? AND status = ?", listingID, userID, "completed").First(&purchase).Error
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"has_access": false})
		return
	}

	// Check subscription expiry
	if listing.Pricing == model.PricingSubscription && purchase.ExpiresAt != nil {
		if purchase.ExpiresAt.Before(time.Now()) {
			c.JSON(http.StatusOK, gin.H{"has_access": false, "reason": "subscription_expired"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"has_access": true, "reason": "purchased"})
}

// ════════════════════════════════════════════════════════════════
//  Revenue
// ════════════════════════════════════════════════════════════════

// CreatorDashboard returns creator's revenue overview.
func (h *MarketplaceHandler) CreatorDashboard(c *gin.Context) {
	userID := c.GetString("user_id")

	var profile model.CreatorProfile
	if err := h.db.Where("user_id = ?", userID).First(&profile).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not a creator"})
		return
	}

	// Total revenue
	var totalRevenue int64
	h.db.Model(&model.CreatorRevenue{}).Where("creator_id = ?", userID).
		Select("COALESCE(SUM(net_amount),0)").Scan(&totalRevenue)

	// Pending payout
	var pendingPayout int64
	h.db.Model(&model.CreatorRevenue{}).Where("creator_id = ? AND status = ?", userID, "pending").
		Select("COALESCE(SUM(net_amount),0)").Scan(&pendingPayout)

	// This month's revenue
	monthStart := time.Now().Format("2006-01") + "-01"
	var monthRevenue int64
	h.db.Model(&model.CreatorRevenue{}).Where("creator_id = ? AND created_at >= ?", userID, monthStart).
		Select("COALESCE(SUM(net_amount),0)").Scan(&monthRevenue)

	// Total sales
	var totalSales int64
	h.db.Model(&model.AgentPurchase{}).Where("creator_id = ?", userID).Count(&totalSales)

	// Active listings
	var activeListings int64
	h.db.Model(&model.AgentListing{}).Where("creator_id = ? AND status = ?", userID, "published").Count(&activeListings)

	// Average rating across all listings
	var avgRating float64
	h.db.Model(&model.AgentRating{}).
		Joins("JOIN agent_listings ON agent_listings.id = agent_ratings.listing_id").
		Where("agent_listings.creator_id = ?", userID).
		Select("COALESCE(AVG(score),0)").Scan(&avgRating)

	c.JSON(http.StatusOK, gin.H{
		"profile":         profile,
		"total_revenue":   totalRevenue,
		"pending_payout":  pendingPayout,
		"month_revenue":   monthRevenue,
		"total_sales":     totalSales,
		"active_listings": activeListings,
		"avg_rating":      avgRating,
	})
}

// CreatorRevenueList returns paginated revenue records.
func (h *MarketplaceHandler) CreatorRevenueList(c *gin.Context) {
	userID := c.GetString("user_id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	if page < 1 {
		page = 1
	}

	var total int64
	h.db.Model(&model.CreatorRevenue{}).Where("creator_id = ?", userID).Count(&total)

	var records []model.CreatorRevenue
	h.db.Where("creator_id = ?", userID).Order("created_at DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&records)

	c.JSON(http.StatusOK, gin.H{
		"items":     records,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// ════════════════════════════════════════════════════════════════
//  Ratings
// ════════════════════════════════════════════════════════════════

// CreateRating submits a rating for a purchased agent.
func (h *MarketplaceHandler) CreateRating(c *gin.Context) {
	userID := c.GetString("user_id")
	listingID := c.Param("id")

	var req struct {
		Score   int    `json:"score" binding:"required,min=1,max=5"`
		Title   string `json:"title"`
		Comment string `json:"comment"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Must have purchased (or be free)
	var listing model.AgentListing
	if err := h.db.Where("id = ? AND status = ?", listingID, "published").First(&listing).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "listing not found"})
		return
	}

	if listing.Pricing != model.PricingFree {
		var purchase model.AgentPurchase
		if err := h.db.Where("listing_id = ? AND buyer_id = ? AND status = ?", listingID, userID, "completed").First(&purchase).Error; err != nil {
			c.JSON(http.StatusForbidden, gin.H{"error": "must purchase before rating"})
			return
		}
	}

	// Check existing rating
	var existing model.AgentRating
	if err := h.db.Where("listing_id = ? AND user_id = ?", listingID, userID).First(&existing).Error; err == nil {
		// Update existing
		existing.Score = req.Score
		existing.Title = req.Title
		existing.Comment = req.Comment
		h.db.Save(&existing)
		h.updateListingRating(listingID)
		c.JSON(http.StatusOK, existing)
		return
	}

	rating := model.AgentRating{
		ListingID: listingID,
		UserID:    userID,
		Score:     req.Score,
		Title:     req.Title,
		Comment:   req.Comment,
	}
	h.db.Create(&rating)
	h.updateListingRating(listingID)

	c.JSON(http.StatusCreated, rating)
}

// ListRatings returns ratings for a listing.
func (h *MarketplaceHandler) ListRatings(c *gin.Context) {
	listingID := c.Param("id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	if page < 1 {
		page = 1
	}

	var total int64
	h.db.Model(&model.AgentRating{}).Where("listing_id = ?", listingID).Count(&total)

	var ratings []model.AgentRating
	h.db.Preload("User").Where("listing_id = ?", listingID).
		Order("created_at DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&ratings)

	c.JSON(http.StatusOK, gin.H{
		"items":     ratings,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// updateListingRating recalculates the average rating for a listing's template.
func (h *MarketplaceHandler) updateListingRating(listingID string) {
	var avgScore float64
	var count int64
	h.db.Model(&model.AgentRating{}).Where("listing_id = ?", listingID).Count(&count)
	if count > 0 {
		h.db.Model(&model.AgentRating{}).Where("listing_id = ?", listingID).Select("AVG(score)").Scan(&avgScore)
	}

	// Update template rating
	var listing model.AgentListing
	if h.db.Where("id = ?", listingID).First(&listing).Error == nil {
		h.db.Model(&model.AgentTemplate{}).Where("id = ?", listing.TemplateID).Updates(map[string]interface{}{
			"rating":       avgScore,
			"rating_count": count,
		})
	}
}

// ════════════════════════════════════════════════════════════════
//  Versions
// ════════════════════════════════════════════════════════════════

// PublishVersion publishes a new version of a listed agent.
func (h *MarketplaceHandler) PublishVersion(c *gin.Context) {
	userID := c.GetString("user_id")
	listingID := c.Param("id")

	var listing model.AgentListing
	if err := h.db.Where("id = ? AND creator_id = ?", listingID, userID).First(&listing).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "listing not found"})
		return
	}

	var req struct {
		Version   string `json:"version" binding:"required"`
		Changelog string `json:"changelog"`
		Config    string `json:"config"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	version := model.AgentVersion{
		ListingID: listingID,
		Version:   req.Version,
		Changelog: req.Changelog,
		Config:    req.Config,
	}
	h.db.Create(&version)

	// Update listing version
	listing.Version = req.Version
	h.db.Save(&listing)

	c.JSON(http.StatusCreated, version)
}

// ════════════════════════════════════════════════════════════════
//  Admin (listing review)
// ════════════════════════════════════════════════════════════════

// AdminListPending returns listings pending review.
func (h *MarketplaceHandler) AdminListPending(c *gin.Context) {
	var listings []model.AgentListing
	h.db.Preload("Template").Preload("Creator").
		Where("status = ?", "pending_review").
		Order("created_at ASC").Find(&listings)
	c.JSON(http.StatusOK, listings)
}

// AdminReviewListing approves or rejects a listing.
func (h *MarketplaceHandler) AdminReviewListing(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Action string `json:"action" binding:"required"` // approve, reject
		Note   string `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var listing model.AgentListing
	if err := h.db.Where("id = ?", id).First(&listing).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "listing not found"})
		return
	}

	switch req.Action {
	case "approve":
		listing.Status = "published"
	case "reject":
		listing.Status = "draft"
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "action must be approve or reject"})
		return
	}
	listing.ReviewNote = req.Note
	h.db.Save(&listing)

	// Update creator agent count
	if req.Action == "approve" {
		var count int64
		h.db.Model(&model.AgentListing{}).Where("creator_id = ? AND status = ?", listing.CreatorID, "published").Count(&count)
		h.db.Model(&model.CreatorProfile{}).Where("user_id = ?", listing.CreatorID).Update("agent_count", count)
	}

	c.JSON(http.StatusOK, gin.H{"status": listing.Status})
}

// AdminBulkImport batch-imports AgentTemplates from the Drone harvester.
// POST /v1/admin/marketplace/import
func (h *MarketplaceHandler) AdminBulkImport(c *gin.Context) {
	var req struct {
		Templates []struct {
			Name         string `json:"name"`
			Description  string `json:"description"`
			Category     string `json:"category"`
			Tags         string `json:"tags"`
			SystemPrompt string `json:"system_prompt"`
			Tools        string `json:"tools"`
			Config       string `json:"config"`
			Icon         string `json:"icon"`
			AuthorID     string `json:"author_id"`
			IsBuiltin    bool   `json:"is_builtin"`
		} `json:"templates"`
		Source string `json:"source"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	imported, skipped := 0, 0
	for _, t := range req.Templates {
		if t.Name == "" || t.SystemPrompt == "" {
			skipped++
			continue
		}

		// Dedup by name + source (check config for source_id)
		var existing model.AgentTemplate
		if err := h.db.Where("name = ? AND is_builtin = ?", t.Name, false).First(&existing).Error; err == nil {
			skipped++
			continue
		}

		authorID := t.AuthorID
		if authorID == "" || authorID == "system" {
			// Use first admin user as author
			var admin model.User
			if err := h.db.Where("role = ?", "owner").First(&admin).Error; err == nil {
				authorID = admin.ID
			} else {
				authorID = "00000000-0000-0000-0000-000000000000"
			}
		}

		template := model.AgentTemplate{
			AuthorID:     authorID,
			Name:         t.Name,
			Description:  t.Description,
			Category:     t.Category,
			Tags:         t.Tags,
			SystemPrompt: t.SystemPrompt,
			Tools:        t.Tools,
			Config:       t.Config,
			Icon:         t.Icon,
			IsBuiltin:    false,
		}

		if template.Tags == "" {
			template.Tags = "[]"
		}
		if template.Tools == "" {
			template.Tools = "[]"
		}
		if template.Config == "" {
			template.Config = "{}"
		}

		if err := h.db.Create(&template).Error; err != nil {
			skipped++
			continue
		}
		imported++
	}

	c.JSON(http.StatusOK, gin.H{
		"imported": imported,
		"skipped":  skipped,
		"source":   req.Source,
		"message":  "batch import completed",
	})
}

// Trending returns top agents by various signals.
func (h *MarketplaceHandler) Trending(c *gin.Context) {
	period := c.DefaultQuery("period", "week") // week, month, all

	var since time.Time
	switch period {
	case "week":
		since = time.Now().AddDate(0, 0, -7)
	case "month":
		since = time.Now().AddDate(0, -1, 0)
	default:
		since = time.Time{}
	}

	// Top by sales
	var topSales []model.AgentListing
	q := h.db.Preload("Template").Where("status = ?", "published")
	if !since.IsZero() {
		q = q.Where("updated_at >= ?", since)
	}
	q.Order("sales_count DESC").Limit(10).Find(&topSales)

	// Top rated
	var topRated []model.AgentListing
	q2 := h.db.Preload("Template").
		Joins("JOIN agent_templates ON agent_templates.id = agent_listings.template_id").
		Where("agent_listings.status = ? AND agent_templates.rating_count >= 3", "published")
	q2.Order("agent_templates.rating DESC").Limit(10).Find(&topRated)

	// Featured
	var featured []model.AgentListing
	h.db.Preload("Template").Where("status = ? AND featured = ?", "published", true).
		Limit(6).Find(&featured)

	c.JSON(http.StatusOK, gin.H{
		"top_sales": topSales,
		"top_rated": topRated,
		"featured":  featured,
		"period":    period,
	})
}
