package billing

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/yinhe/starclaw/internal/tool"
	"gorm.io/gorm"
)

// Revenue split rates — 10% markup on upstream cost
const (
	MarkupRate        = 0.10 // 10% markup on upstream cost
	InvestorShareRate = 0.10 // investor pool always gets 10% of margin
)

// PriceEntry defines pricing for a specific tool + sub-type combination.
// All prices in CNY (元).
type PriceEntry struct {
	ToolName     string  // e.g. "video_generation"
	SubType      string  // e.g. "veo3.1", "sora2", "kling-v3"
	ResourceType string  // tokens, video, image, music, tts, stt, search, plugin_api
	UpstreamCNY  float64 // what we pay upstream (元)
	UserCNY      float64 // what user pays = upstream × 1.10 (auto-calculated)
}

// Gateway is the central billing middleware for tool execution.
// It wraps tool.Registry.Execute with: balance check → execute → cost + deduct + revenue split.
type Gateway struct {
	db          *gorm.DB
	queenClient *QueenClient
	prices      map[string]PriceEntry // key: "toolName" or "toolName:subType"
	priceMu     sync.RWMutex
	clawID      string // this node's claw_id
	enabled     bool
}

// NewGateway creates a new billing gateway.
func NewGateway(db *gorm.DB, queenClient *QueenClient, clawID string) *Gateway {
	g := &Gateway{
		db:          db,
		queenClient: queenClient,
		prices:      make(map[string]PriceEntry),
		clawID:      clawID,
		enabled:     queenClient != nil && queenClient.IsEnabled(),
	}
	g.seedPrices()
	if g.enabled {
		log.Printf("[billing-gateway] Initialized for claw=%s", clawID)
	}
	return g
}

// IsEnabled returns true when the billing gateway is active
func (g *Gateway) IsEnabled() bool {
	return g.enabled
}

// seedPrices initializes the default pricing table.
// Upstream prices in CNY; user price = upstream × 1.10 (auto).
func (g *Gateway) seedPrices() {
	entries := []PriceEntry{
		// Video generation
		{ToolName: "video_generation", SubType: "veo3.1", ResourceType: "video", UpstreamCNY: 2.50},
		{ToolName: "video_generation", SubType: "veo3", ResourceType: "video", UpstreamCNY: 2.50},
		{ToolName: "video_generation", SubType: "sora2", ResourceType: "video", UpstreamCNY: 1.50},
		{ToolName: "video_generation", SubType: "kling-v3", ResourceType: "video", UpstreamCNY: 1.00},
		{ToolName: "video_generation", SubType: "luma", ResourceType: "video", UpstreamCNY: 0.80},
		{ToolName: "video_generation", SubType: "wan2.6-t2v", ResourceType: "video", UpstreamCNY: 0.20},
		{ToolName: "video_generation", SubType: "wan2.6-i2v", ResourceType: "video", UpstreamCNY: 0.20},
		{ToolName: "video_generation", SubType: "", ResourceType: "video", UpstreamCNY: 0.20},
		// Image generation
		{ToolName: "image_generation", SubType: "flux-pro", ResourceType: "image", UpstreamCNY: 0.30},
		{ToolName: "image_generation", SubType: "flux-kontext", ResourceType: "image", UpstreamCNY: 0.20},
		{ToolName: "image_generation", SubType: "", ResourceType: "image", UpstreamCNY: 0.20},
		// Music generation
		{ToolName: "music_generation", SubType: "", ResourceType: "music", UpstreamCNY: 1.00},
		// TTS
		{ToolName: "text_to_speech", SubType: "", ResourceType: "tts", UpstreamCNY: 0.05},
		// Search
		{ToolName: "web_search", SubType: "", ResourceType: "search", UpstreamCNY: 0.01},
		// Default plugin
		{ToolName: "_default_plugin", SubType: "", ResourceType: "plugin_api", UpstreamCNY: 0.005},
	}

	for _, e := range entries {
		e.UserCNY = e.UpstreamCNY * (1 + MarkupRate)
		key := e.ToolName
		if e.SubType != "" {
			key = e.ToolName + ":" + e.SubType
		}
		g.prices[key] = e
	}
}

// getPrice looks up pricing for a tool + sub-type.
func (g *Gateway) getPrice(toolName, subType string) PriceEntry {
	g.priceMu.RLock()
	defer g.priceMu.RUnlock()

	// Try exact match first
	if subType != "" {
		if p, ok := g.prices[toolName+":"+subType]; ok {
			return p
		}
	}
	// Fallback to tool-level default
	if p, ok := g.prices[toolName]; ok {
		return p
	}
	// Fallback to global default
	if p, ok := g.prices["_default_plugin"]; ok {
		return p
	}
	return PriceEntry{}
}

// ExecuteHook is the billing hook that wraps tool execution.
// It is called by tool.Registry.Execute when a hook is set.
func (g *Gateway) ExecuteHook(ctx context.Context, t tool.Tool, name, args string) (string, error) {
	if !g.enabled {
		return t.Execute(ctx, args)
	}

	userID, _ := ctx.Value(tool.CtxKeyUserID).(string)
	if userID == "" {
		// No user context = internal call, skip billing
		return t.Execute(ctx, args)
	}

	// ── Before: Check balance ──
	hasBalance, _, _ := g.queenClient.CheckBalance(userID)
	if !hasBalance {
		return "", fmt.Errorf("余额不足，无法执行 %s，请充值后继续使用", name)
	}

	// ── Execute ──
	start := time.Now()
	result, execErr := t.Execute(ctx, args)
	elapsed := time.Since(start)

	// ── After: Calculate cost + settle ──
	// Only charge for successful generative executions — non-generative actions are free
	if execErr == nil && isBillableAction(name, args) {
		subType := extractSubType(name, args)
		price := g.getPrice(name, subType)

		if price.UpstreamCNY > 0 {
			log.Printf("[billing-gateway] charging: user=%s tool=%s subType=%s upstream=¥%.3f", userID, name, subType, price.UpstreamCNY)
			go g.settle(ctx, userID, name, subType, price, elapsed, execErr)
		} else {
			log.Printf("[billing-gateway] no price found: tool=%s subType=%s, skipping billing", name, subType)
		}
	} else if execErr != nil {
		log.Printf("[billing-gateway] tool %s failed, not charging: %v", name, execErr)
	}

	return result, execErr
}

// settle handles post-execution billing: deduct + revenue split + record.
func (g *Gateway) settle(ctx context.Context, userID, toolName, subType string, price PriceEntry, elapsed time.Duration, execErr error) {
	upstream := price.UpstreamCNY
	margin := upstream * MarkupRate
	userCost := upstream + margin

	// 1. Deduct from user balance (amount in 分)
	costFen := int64(userCost * 100)
	remark := fmt.Sprintf("%s(%s) upstream=¥%.3f", toolName, subType, upstream)
	if _, err := g.queenClient.Consume(userID, price.ResourceType, 1, costFen, remark); err != nil {
		log.Printf("[billing-gateway] consume failed: user=%s tool=%s err=%v", userID, toolName, err)
		// Still create a zero-cost tracking record so the usage appears in consumption history
		trackRemark := fmt.Sprintf("%s(%s) [unfunded]", toolName, subType)
		g.queenClient.Consume(userID, price.ResourceType, 1, 0, trackRemark)
	}

	// 2. Revenue split
	investorAmount := int64(margin * InvestorShareRate * 100)

	// Resolve partner chain
	cityID, coreID := "", ""
	if g.queenClient != nil {
		cityID, coreID = g.queenClient.ResolvePartners(g.clawID)
	}

	var cityAmount, coreAmount, platformAmount int64
	marginFen := int64(margin * 100)

	if cityID != "" && coreID != "" {
		// City + Core: 30/30/30/10
		cityAmount = int64(float64(marginFen) * 0.30)
		coreAmount = int64(float64(marginFen) * 0.30)
		platformAmount = int64(float64(marginFen) * 0.30)
	} else if cityID != "" {
		// Only City: 30/0/60/10
		cityAmount = int64(float64(marginFen) * 0.30)
		platformAmount = int64(float64(marginFen) * 0.60)
	} else if coreID != "" {
		// Only Core: 0/70/20/10
		coreAmount = int64(float64(marginFen) * 0.70)
		platformAmount = int64(float64(marginFen) * 0.20)
	} else {
		// Direct: 0/0/90/10
		platformAmount = marginFen - investorAmount
	}

	// 3. Deposit investor share to Queen pool
	if investorAmount > 0 {
		g.queenClient.DepositInvestorPool(toolName, "", investorAmount, marginFen, g.clawID)
	}

	// 4. Record locally (for analytics/audit)
	success := execErr == nil
	errMsg := ""
	if execErr != nil {
		errMsg = execErr.Error()
	}

	convID, _ := ctx.Value(tool.CtxKeyConversationID).(string)

	record := &ToolUsageRecord{
		UserID:           userID,
		ConversationID:   convID,
		ClawID:           g.clawID,
		ToolName:         toolName,
		SubType:          subType,
		ResourceType:     price.ResourceType,
		CostFen:          costFen,
		UpstreamFen:      int64(upstream * 100),
		MarginFen:        marginFen,
		CityPartnerID:    cityID,
		CorePartnerID:    coreID,
		CityShareFen:     cityAmount,
		CoreShareFen:     coreAmount,
		PlatformShareFen: platformAmount,
		InvestorShareFen: investorAmount,
		DurationMs:       elapsed.Milliseconds(),
		Success:          success,
		ErrorMsg:         errMsg,
	}

	if g.db != nil {
		if err := g.db.Create(record).Error; err != nil {
			log.Printf("[billing-gateway] record failed: %v", err)
		}
	}

	log.Printf("[billing-gateway] settled: user=%s tool=%s(%s) cost=¥%.3f margin=¥%.3f city=%d core=%d platform=%d investor=%d",
		userID, toolName, subType, userCost, margin, cityAmount, coreAmount, platformAmount, investorAmount)
}

// isBillableAction checks if a tool action should be billed.
// Only generative actions are billed; status checks, listings, etc. are free.
func isBillableAction(toolName, args string) bool {
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(args), &parsed); err != nil {
		return true // can't parse → bill by default
	}
	action, _ := parsed["action"].(string)
	if action == "" {
		return true // no action field → bill by default
	}

	// Non-billable actions per tool
	freeActions := map[string]map[string]bool{
		"video_generation": {"check_status": true, "list_models": true, "merge_videos": true, "extract_last_frame": true, "list_videos": true},
		"music_generation": {"check_status": true, "list_voices": true},
		"audio_analysis":   {"detect_beats": true, "get_energy_curve": true, "generate_srt": true},
		"mv_production":    {}, // compose_mv and compose_pro are both billable
	}

	if free, ok := freeActions[toolName]; ok {
		return !free[action]
	}
	return true
}

// extractSubType extracts the sub-type (e.g. model name) from tool args.
func extractSubType(toolName, args string) string {
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(args), &parsed); err != nil {
		return ""
	}
	// Try common field names for model/sub-type
	for _, key := range []string{"model", "model_name", "engine", "provider", "voice"} {
		if v, ok := parsed[key]; ok {
			if s, ok := v.(string); ok {
				return s
			}
		}
	}
	return ""
}
