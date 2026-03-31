package v1

import (
	"log"
	"time"

	"github.com/yinhe/starclaw/internal/model"
	"gorm.io/gorm"
)

// StardustEngine handles automatic stardust rewards for user actions.
// Called from chat handler, task worker, and daily cron.
type StardustEngine struct {
	db *gorm.DB
}

func NewStardustEngine(db *gorm.DB) *StardustEngine {
	return &StardustEngine{db: db}
}

// RewardChat gives stardust for completing a conversation turn.
// Called after each assistant message is generated.
func (e *StardustEngine) RewardChat(userID string) {
	e.reward(userID, 1, "earn_chat", "conversation turn")
}

// RewardTask gives stardust for completing a task.
func (e *StardustEngine) RewardTask(userID string) {
	e.reward(userID, 3, "earn_task", "task completed")
}

// RewardThumbsUp gives stardust for a positive rating.
func (e *StardustEngine) RewardThumbsUp(userID string) {
	e.reward(userID, 5, "earn_thumbs_up", "positive feedback")
}

// RewardAgentCreated gives stardust for creating a new Agent.
func (e *StardustEngine) RewardAgentCreated(userID string) {
	e.reward(userID, 20, "earn_agent_created", "new agent created")
}

// RewardDailyLogin gives stardust for daily login.
// Should be called once per day per user.
func (e *StardustEngine) RewardDailyLogin(userID string) {
	// Check if already rewarded today
	today := time.Now().Format("2006-01-02")
	var count int64
	e.db.Model(&model.StardustTransaction{}).
		Where("user_id = ? AND type = ? AND DATE(created_at) = ?", userID, "earn_daily", today).
		Count(&count)
	if count > 0 {
		return // already rewarded today
	}
	e.reward(userID, 5, "earn_daily", "daily login")
}

// RewardStreak gives bonus stardust for consecutive usage days.
func (e *StardustEngine) RewardStreak(userID string, streakDays int) {
	if streakDays <= 1 {
		return
	}
	bonus := streakDays * 2
	if bonus > 30 {
		bonus = 30 // cap at 30
	}
	e.reward(userID, bonus, "earn_streak", "streak bonus")
}

// RewardMilestone gives a large stardust reward for milestone achievements.
func (e *StardustEngine) RewardMilestone(userID string, milestoneTitle string) {
	e.reward(userID, 50, "earn_milestone", milestoneTitle)
}

// RewardPKWin gives stardust for winning a PK battle.
func (e *StardustEngine) RewardPKWin(userID string, opponentStrength int) {
	reward := 10
	if opponentStrength > 30 {
		reward = 20
	}
	if opponentStrength > 40 {
		reward = 30
	}
	e.reward(userID, reward, "earn_pk_win", "arena victory")
}

// AbsorbEXP transfers EXP from loser to winner after PK.
// Returns (winner_gain, loser_loss).
func (e *StardustEngine) AbsorbEXP(winnerID, loserID string, winnerStreak int) (int, int) {
	var loserGrowth model.NodeGrowth
	if err := e.db.Where("user_id = ?", loserID).First(&loserGrowth).Error; err != nil {
		return 0, 0
	}

	// Base absorption: 5% of loser's level-equivalent EXP
	baseEXP := loserGrowth.Level * 100 // approximate EXP value
	absorbPct := 5
	if winnerStreak >= 3 {
		absorbPct = 7
	}
	if winnerStreak >= 5 {
		absorbPct = 10
	}

	loserLoss := baseEXP * absorbPct / 100
	if loserLoss > 500 {
		loserLoss = 500 // cap
	}
	winnerGain := loserLoss * 150 / 100 // 1.5x non-zero-sum

	return winnerGain, loserLoss
}

// PunishPKLoss applies EXP loss and potential demotion for PK losers.
func (e *StardustEngine) PunishPKLoss(userID string, consecutiveLosses int) {
	var growth model.NodeGrowth
	if err := e.db.Where("user_id = ?", userID).First(&growth).Error; err != nil {
		return
	}

	// 3% EXP loss (won't drop level)
	// Consecutive losses: 3=darken, 5=weaken, 10=demote
	if consecutiveLosses >= 10 {
		// Demote one evolution stage
		currentIdx := 0
		for i, lv := range model.LevelThresholds {
			if growth.Level >= lv {
				currentIdx = i
			}
		}
		if currentIdx > 2 { // don't demote below Lv.5 (crayfish)
			newLevel := model.LevelThresholds[currentIdx-1]
			growth.Level = newLevel
			e.db.Save(&growth)
			log.Printf("[stardust] DEMOTED user %s from stage %d to Lv.%d", userID, currentIdx, newLevel)
		}
	}
}

// internal helper
func (e *StardustEngine) reward(userID string, amount int, txnType, note string) {
	// Update balance
	e.db.Model(&model.NodeGrowth{}).
		Where("user_id = ?", userID).
		Update("stardust_balance", gorm.Expr("stardust_balance + ?", amount))

	// Record transaction
	e.db.Create(&model.StardustTransaction{
		UserID: userID,
		Amount: amount,
		Type:   txnType,
		Note:   note,
	})
}
