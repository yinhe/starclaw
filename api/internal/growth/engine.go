package growth

import (
	"bytes"
	"encoding/json"
	"log"
	"math"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/yinhe/starclaw/internal/model"
	"gorm.io/gorm"
)

// Stats holds aggregated metrics for a Claw node (all agents combined).
type Stats struct {
	// Raw counts from DB
	Conversations  int64 `json:"conversations"`
	Messages       int64 `json:"messages"`
	Memories       int64 `json:"memories"`
	KnowledgeDocs  int64 `json:"knowledge_docs"`
	TasksCompleted int64 `json:"tasks_completed"`
	TasksFailed    int64 `json:"tasks_failed"`
	GoalsCompleted int64 `json:"goals_completed"`
	ToolsUsed      int64 `json:"tools_used"`
	UniqueTools    int64 `json:"unique_tools"`
	WechatSent     int64 `json:"wechat_sent"`
	ThumbsUp       int64 `json:"thumbs_up"`
	ThumbsDown     int64 `json:"thumbs_down"`

	// Derived
	DaysSinceFirst   int     `json:"days_since_first"`
	DayStreak        int     `json:"day_streak"`
	EXP              int64   `json:"exp"`
	Level            int     `json:"level"`
	LevelProgress    float64 `json:"level_progress"`
	NextLevelEXP     int64   `json:"next_level_exp"`
	SatisfactionRate float64 `json:"satisfaction_rate"`

	// Battle stats (HP/ATK/DEF/SPD derived from usage)
	HP  int `json:"hp"`
	ATK int `json:"atk"`
	DEF int `json:"def"`
	SPD int `json:"spd"`
}

// MilestoneDef defines a milestone check.
type MilestoneDef struct {
	Code  string
	Title string
	Check func(Stats) bool
}

// MilestoneDefs is the full list of milestone definitions.
var MilestoneDefs = []MilestoneDef{
	// 对话类
	{Code: "first_chat", Title: "首次对话", Check: func(s Stats) bool { return s.Conversations >= 1 }},
	{Code: "chat_50", Title: "对话 50 次", Check: func(s Stats) bool { return s.Conversations >= 50 }},
	{Code: "chat_200", Title: "对话 200 次", Check: func(s Stats) bool { return s.Conversations >= 200 }},
	{Code: "chat_1000", Title: "千言万语", Check: func(s Stats) bool { return s.Conversations >= 1000 }},

	// 记忆类
	{Code: "memory_10", Title: "记住 10 件事", Check: func(s Stats) bool { return s.Memories >= 10 }},
	{Code: "memory_50", Title: "记住 50 件事", Check: func(s Stats) bool { return s.Memories >= 50 }},
	{Code: "memory_100", Title: "过目不忘", Check: func(s Stats) bool { return s.Memories >= 100 }},

	// 任务类
	{Code: "task_1", Title: "首个任务完成", Check: func(s Stats) bool { return s.TasksCompleted >= 1 }},
	{Code: "task_10", Title: "任务达人", Check: func(s Stats) bool { return s.TasksCompleted >= 10 }},
	{Code: "task_50", Title: "自主管家", Check: func(s Stats) bool { return s.TasksCompleted >= 50 }},

	// 工具类
	{Code: "tool_first", Title: "首次使用工具", Check: func(s Stats) bool { return s.ToolsUsed >= 1 }},
	{Code: "tool_10", Title: "工具大师", Check: func(s Stats) bool { return s.UniqueTools >= 10 }},
	{Code: "wechat_first", Title: "首次微信消息", Check: func(s Stats) bool { return s.WechatSent >= 1 }},

	// 满意度类
	{Code: "thumbs_10", Title: "10 次好评", Check: func(s Stats) bool { return s.ThumbsUp >= 10 }},
	{Code: "thumbs_100", Title: "百里挑一", Check: func(s Stats) bool { return s.ThumbsUp >= 100 }},

	// 连续使用
	{Code: "streak_7", Title: "连续使用 7 天", Check: func(s Stats) bool { return s.DayStreak >= 7 }},
	{Code: "streak_30", Title: "月度陪伴", Check: func(s Stats) bool { return s.DayStreak >= 30 }},

	// 进化里程碑
	{Code: "evolve_5", Title: "首次进化", Check: func(s Stats) bool { return s.Level >= 5 }},
	{Code: "evolve_10", Title: "二次进化", Check: func(s Stats) bool { return s.Level >= 10 }},
	{Code: "evolve_20", Title: "三次进化", Check: func(s Stats) bool { return s.Level >= 20 }},
	{Code: "evolve_30", Title: "四次进化", Check: func(s Stats) bool { return s.Level >= 30 }},
}

// ComputeStats aggregates all growth metrics for a Claw node (across all agents).
func ComputeStats(db *gorm.DB, userID string) Stats {
	var s Stats

	// Conversations count (all agents)
	db.Model(&model.Conversation{}).
		Where("user_id = ? AND deleted_at IS NULL", userID).
		Count(&s.Conversations)

	// Messages count
	db.Model(&model.Message{}).
		Joins("JOIN conversations ON conversations.id = messages.conversation_id").
		Where("conversations.user_id = ? AND conversations.deleted_at IS NULL", userID).
		Count(&s.Messages)

	// Thumbs up/down
	db.Model(&model.Message{}).
		Joins("JOIN conversations ON conversations.id = messages.conversation_id").
		Where("conversations.user_id = ? AND conversations.deleted_at IS NULL AND messages.feedback = 1", userID).
		Count(&s.ThumbsUp)
	db.Model(&model.Message{}).
		Joins("JOIN conversations ON conversations.id = messages.conversation_id").
		Where("conversations.user_id = ? AND conversations.deleted_at IS NULL AND messages.feedback = -1", userID).
		Count(&s.ThumbsDown)

	// Tool usage: count messages with non-empty tool_calls
	db.Model(&model.Message{}).
		Joins("JOIN conversations ON conversations.id = messages.conversation_id").
		Where("conversations.user_id = ? AND conversations.deleted_at IS NULL AND messages.tool_calls != '[]' AND messages.tool_calls != ''", userID).
		Count(&s.ToolsUsed)
	s.UniqueTools = s.ToolsUsed

	// Memories (all agents)
	db.Model(&model.Memory{}).
		Where("user_id = ?", userID).
		Count(&s.Memories)

	// Knowledge docs
	var kbDocs int64
	db.Model(&model.KnowledgeBase{}).
		Where("user_id = ? AND deleted_at IS NULL", userID).
		Select("COALESCE(SUM(document_count), 0)").
		Scan(&kbDocs)
	s.KnowledgeDocs = kbDocs

	// Tasks completed / failed (all agents)
	db.Model(&model.Task{}).
		Where("user_id = ? AND status = ?", userID, model.TaskStatusCompleted).
		Count(&s.TasksCompleted)
	db.Model(&model.Task{}).
		Where("user_id = ? AND status = ?", userID, model.TaskStatusFailed).
		Count(&s.TasksFailed)

	// WeChat sent
	db.Model(&model.Task{}).
		Where("user_id = ? AND status = ? AND (title LIKE ? OR title LIKE ?)",
			userID, model.TaskStatusCompleted, "%wechat%", "%微信%").
		Count(&s.WechatSent)

	// First chat date (earliest across all agents)
	var firstConv model.Conversation
	if err := db.Where("user_id = ? AND deleted_at IS NULL", userID).
		Order("created_at ASC").Limit(1).First(&firstConv).Error; err == nil {
		s.DaysSinceFirst = int(time.Since(firstConv.CreatedAt).Hours() / 24)
	}

	// Day streak: consecutive days with any conversation
	s.DayStreak = computeDayStreak(db, userID)

	// Compute EXP
	s.EXP = computeEXP(s)

	// Compute Level: Level = floor(sqrt(EXP / 100))
	s.Level = int(math.Floor(math.Sqrt(float64(s.EXP) / 100.0)))
	if s.Level < 1 && s.Conversations > 0 {
		s.Level = 1
	}

	// Level progress
	currentLevelEXP := int64(s.Level * s.Level * 100)
	nextLevelEXP := int64((s.Level + 1) * (s.Level + 1) * 100)
	s.NextLevelEXP = nextLevelEXP
	if nextLevelEXP > currentLevelEXP {
		s.LevelProgress = float64(s.EXP-currentLevelEXP) / float64(nextLevelEXP-currentLevelEXP)
	}
	if s.LevelProgress < 0 {
		s.LevelProgress = 0
	}
	if s.LevelProgress > 1 {
		s.LevelProgress = 1
	}

	// Satisfaction rate
	total := s.ThumbsUp + s.ThumbsDown
	if total > 0 {
		s.SatisfactionRate = float64(s.ThumbsUp) / float64(total)
	}

	// Battle stats (for Arena PK)
	s.HP = computeBattleStat(s.Memories, 100, 10)       // 记忆→血厚
	s.ATK = computeBattleStat(s.TasksCompleted, 50, 10) // 任务→攻强
	s.DEF = computeBattleStat(s.ThumbsUp, 30, 10)       // 好评→防高
	s.SPD = computeBattleStat(s.Conversations, 200, 10) // 对话→速快

	return s
}

// computeEXP calculates total EXP from raw stats.
func computeEXP(s Stats) int64 {
	var exp int64
	exp += s.Conversations * 10  // 每次对话 +10
	exp += s.ThumbsUp * 20       // 好评 +20
	exp += s.Memories * 5        // 记忆 +5
	exp += s.TasksCompleted * 30 // 任务完成 +30
	exp += s.GoalsCompleted * 50 // 目标完成 +50
	exp -= s.ThumbsDown * 5      // 差评 -5
	if exp < 0 {
		exp = 0
	}
	return exp
}

// computeBattleStat converts a raw count to a battle stat using log scaling.
// Base is the minimum stat, scale controls the growth rate.
func computeBattleStat(count int64, scale float64, base int) int {
	if count <= 0 {
		return base
	}
	return base + int(math.Log2(float64(count)+1)*scale/10.0)
}

// computeDayStreak counts consecutive days (ending today/yesterday) with any conversation.
func computeDayStreak(db *gorm.DB, userID string) int {
	type DayRow struct {
		Day string
	}
	var days []DayRow
	db.Model(&model.Conversation{}).
		Select("DATE(created_at) as day").
		Where("user_id = ? AND deleted_at IS NULL", userID).
		Group("DATE(created_at)").
		Order("day DESC").
		Limit(90).
		Find(&days)

	if len(days) == 0 {
		return 0
	}

	today := time.Now().Truncate(24 * time.Hour)
	streak := 0
	for i, d := range days {
		t, err := time.Parse("2006-01-02", d.Day)
		if err != nil {
			break
		}
		expected := today.AddDate(0, 0, -i)
		// Allow starting from today or yesterday
		if i == 0 && t.Before(expected.AddDate(0, 0, -1)) {
			break
		}
		if i == 0 {
			// Adjust expected based on actual start
			expected = t.Truncate(24 * time.Hour)
		}
		if i > 0 {
			prevT, _ := time.Parse("2006-01-02", days[i-1].Day)
			if prevT.Sub(t).Hours() > 48 { // more than 1 day gap
				break
			}
		}
		streak++
	}
	return streak
}

// DetermineEvolutionPath picks the path based on usage stats.
func DetermineEvolutionPath(s Stats) model.EvolutionPath {
	abyssScore := float64(s.Memories) + float64(s.KnowledgeDocs)
	terrainScore := float64(s.TasksCompleted) + float64(s.ToolsUsed)
	skyScore := float64(s.Conversations)*0.1 + float64(s.ThumbsUp)

	if abyssScore >= terrainScore && abyssScore >= skyScore {
		return model.PathAbyss
	}
	if terrainScore >= skyScore {
		return model.PathTerrain
	}
	return model.PathSky
}

// GrowthProfile is the full response object for the growth API (node-level).
type GrowthProfile struct {
	Stats         Stats               `json:"stats"`
	EvolutionPath model.EvolutionPath `json:"evolution_path"`
	FormCode      string              `json:"form_code"`
	Title         string              `json:"title"`
	TitleEN       string              `json:"title_en"`
	PathEmoji     string              `json:"path_emoji"`
	PathName      string              `json:"path_name"`
	DaysWith      int                 `json:"days_with"`
	FirstChat     *time.Time          `json:"first_chat"`
	AgentCount    int64               `json:"agent_count"`
	Milestones    []model.Milestone   `json:"milestones"`
	NewMilestones []model.Milestone   `json:"new_milestones"`
}

// BuildProfile computes the full growth profile for this Claw node.
func BuildProfile(db *gorm.DB, userID string) (*GrowthProfile, error) {
	// Get or create NodeGrowth record (one per user/node)
	var ng model.NodeGrowth
	err := db.Where("user_id = ?", userID).First(&ng).Error
	if err != nil {
		ng = model.NodeGrowth{
			ID:            uuid.New().String(),
			UserID:        userID,
			EvolutionPath: model.PathLarva,
		}
		db.Create(&ng)
	}

	// Compute stats (all agents combined)
	stats := ComputeStats(db, userID)

	// Update evolution path if level >= 5
	path := ng.EvolutionPath
	oldLevel := ng.Level
	if stats.Level >= 5 {
		newPath := DetermineEvolutionPath(stats)
		if path != newPath {
			oldPath := path
			path = newPath
			db.Model(&ng).Update("evolution_path", path)
			// Publish evolution event
			PublishGrowthEvent("growth.evolution", EvolutionEvent{
				UserID:  userID,
				OldPath: string(oldPath),
				NewPath: string(path),
				Level:   stats.Level,
			})
		}
	}
	if path == model.PathLarva && stats.Level >= 1 {
		path = DetermineEvolutionPath(stats)
	}

	// Detect level-up and persist
	if stats.Level > oldLevel {
		db.Model(&ng).Update("level", stats.Level)
	}

	// Update first_chat if not set
	if ng.FirstChat == nil && stats.Conversations > 0 {
		var firstConv model.Conversation
		if err := db.Where("user_id = ? AND deleted_at IS NULL", userID).
			Order("created_at ASC").Limit(1).First(&firstConv).Error; err == nil {
			db.Model(&ng).Update("first_chat", firstConv.CreatedAt)
			ng.FirstChat = &firstConv.CreatedAt
		}
	}

	// Agent count
	var agentCount int64
	db.Model(&model.Agent{}).Where("user_id = ? AND deleted_at IS NULL", userID).Count(&agentCount)

	// Titles
	title, titleEN := model.GetTitle(path, stats.Level)
	formCode := model.GetFormCode(path, stats.Level)

	pathEmoji := "🌊"
	pathName := "渊·鲲之路"
	switch path {
	case model.PathTerrain:
		pathEmoji = "🏔️"
		pathName = "陆·兽之路"
	case model.PathSky:
		pathEmoji = "🌪️"
		pathName = "穹·鹏之路"
	}

	// Check milestones
	newMilestones := CheckMilestones(db, userID, stats, path)

	// Fetch all milestones
	var milestones []model.Milestone
	db.Where("user_id = ?", userID).
		Order("achieved_at ASC").Find(&milestones)

	return &GrowthProfile{
		Stats:         stats,
		EvolutionPath: path,
		FormCode:      formCode,
		Title:         title,
		TitleEN:       titleEN,
		PathEmoji:     pathEmoji,
		PathName:      pathName,
		DaysWith:      stats.DaysSinceFirst,
		FirstChat:     ng.FirstChat,
		AgentCount:    agentCount,
		Milestones:    milestones,
		NewMilestones: newMilestones,
	}, nil
}

// SyncToChrysalis pushes current growth stats to the Chrysalis PK service.
// Called after BuildProfile when stats change. Runs async, errors are logged.
func SyncToChrysalis(profile *GrowthProfile, clawID string) {
	chrysalisURL := os.Getenv("CHRYSALIS_URL")
	if chrysalisURL == "" {
		// Try via Queen proxy (default path for most deployments)
		queenURL := os.Getenv("QUEEN_URL")
		if queenURL == "" {
			return // No Chrysalis or Queen configured, skip
		}
		chrysalisURL = queenURL + "/v1/arena/pk"
	} else {
		chrysalisURL = chrysalisURL + "/chrysalis/pk"
	}

	go func() {
		payload := map[string]interface{}{
			"claw_id":        clawID,
			"name":           profile.Title,
			"level":          profile.Stats.Level,
			"evolution_path": string(profile.EvolutionPath),
			"form_code":      profile.FormCode,
			"base_hp":        profile.Stats.HP,
			"base_atk":       profile.Stats.ATK,
			"base_def":       profile.Stats.DEF,
			"base_spd":       profile.Stats.SPD,
		}
		body, _ := json.Marshal(payload)
		resp, err := http.Post(chrysalisURL+"/sync", "application/json", bytes.NewReader(body))
		if err != nil {
			log.Printf("[growth] chrysalis sync failed: %v", err)
			return
		}
		resp.Body.Close()
	}()
}

// CheckMilestones evaluates all milestone definitions and creates new ones.
// Returns the list of newly created milestones (for notification).
func CheckMilestones(db *gorm.DB, userID string, stats Stats, path model.EvolutionPath) []model.Milestone {
	// Get existing milestone codes
	var existing []model.Milestone
	db.Where("user_id = ?", userID).Find(&existing)
	existingCodes := make(map[string]bool, len(existing))
	for _, m := range existing {
		existingCodes[m.Code] = true
	}

	var newMilestones []model.Milestone
	now := time.Now()

	for _, def := range MilestoneDefs {
		if existingCodes[def.Code] {
			continue
		}
		if !def.Check(stats) {
			continue
		}

		// Dynamic title for evolution milestones
		title := def.Title
		if len(def.Code) > 7 && def.Code[:7] == "evolve_" {
			lvTitle, _ := model.GetTitle(path, stats.Level)
			emoji := "🌊"
			switch path {
			case model.PathTerrain:
				emoji = "🏔️"
			case model.PathSky:
				emoji = "🌪️"
			}
			title = "进化为" + lvTitle + " " + emoji
		}

		m := model.Milestone{
			ID:         uuid.New().String(),
			UserID:     userID,
			Code:       def.Code,
			Title:      title,
			AchievedAt: now,
		}
		db.Create(&m)
		newMilestones = append(newMilestones, m)
	}

	return newMilestones
}
