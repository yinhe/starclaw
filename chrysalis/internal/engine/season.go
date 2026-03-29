package engine

import (
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"
	"starclaw.net/chrysalis/internal/model"
)

// SeasonRotator manages automatic season rotation and end-of-season rewards.
type SeasonRotator struct {
	db        *gorm.DB
	pheromone *PheromonePublisher
	stopCh    chan struct{}
}

// NewSeasonRotator creates a new season rotator.
func NewSeasonRotator(db *gorm.DB) *SeasonRotator {
	return &SeasonRotator{
		db:        db,
		pheromone: NewPheromonePublisher(),
		stopCh:    make(chan struct{}),
	}
}

// Start begins the season check loop (runs every hour).
func (r *SeasonRotator) Start() {
	go func() {
		// Check immediately on start
		r.checkAndRotate()

		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				r.checkAndRotate()
			case <-r.stopCh:
				return
			}
		}
	}()
	log.Printf("[chrysalis] season rotator started (check interval: 1h)")
}

// Stop halts the season rotator.
func (r *SeasonRotator) Stop() {
	close(r.stopCh)
}

// checkAndRotate checks if the current season has ended and rotates.
func (r *SeasonRotator) checkAndRotate() {
	var current model.Season
	if err := r.db.Where("active = true").First(&current).Error; err != nil {
		log.Printf("[season] no active season found")
		return
	}

	if time.Now().Before(current.EndAt) {
		return // Season still active
	}

	log.Printf("[season] Season %q (#%d) ended, rotating...", current.Name, current.Number)

	// 1. End current season
	r.endSeason(&current)

	// 2. Create next season
	r.createNextSeason(&current)
}

// endSeason finalizes the current season: rank fighters, apply rewards/decay.
func (r *SeasonRotator) endSeason(season *model.Season) {
	// Deactivate current season
	r.db.Model(season).Update("active", false)

	// Rank all fighters who participated this season by peak ELO
	var records []model.SeasonRecord
	r.db.Where("season_id = ?", season.ID).Order("peak_elo DESC").Find(&records)

	for i := range records {
		records[i].SeasonRank = i + 1
		r.db.Model(&records[i]).Update("season_rank", i+1)
	}

	// Apply inactive decay: fighters with 0 battles lose ELO
	var allFighters []model.BattleFighter
	r.db.Find(&allFighters)

	participantIDs := make(map[string]bool)
	for _, rec := range records {
		participantIDs[rec.FighterID] = true
	}

	decayed := 0
	for _, f := range allFighters {
		if participantIDs[f.ID] {
			continue // Participated, no decay
		}
		newELO := f.ELO - season.InactiveDecay
		if newELO < 800 {
			newELO = 800 // Floor
		}
		r.db.Model(&f).Update("elo", newELO)
		decayed++
	}

	// Grant season rewards (stardust) to top 10
	for i, rec := range records {
		if i >= 10 {
			break
		}
		reward := seasonReward(i + 1)
		if reward > 0 {
			var account model.StardustAccount
			if err := r.db.Where("claw_id = ?", rec.ClawID).First(&account).Error; err != nil {
				continue
			}
			r.db.Model(&account).Update("balance", gorm.Expr("balance + ?", reward))
			r.db.Create(&model.StardustTransaction{
				ClawID: rec.ClawID,
				Amount: int64(reward),
				Type:   "season_reward",
				Remark: fmt.Sprintf("赛季 #%d 排名第 %d 奖励", season.Number, i+1),
			})
		}
	}

	// Publish season end event
	r.pheromone.Publish("chrysalis.season_end", SeasonEndEvent{
		SeasonID:   season.ID,
		SeasonName: season.Name,
		Timestamp:  time.Now().UTC().Format("2006-01-02T15:04:05Z"),
	})

	log.Printf("[season] Season #%d ended: %d participants ranked, %d inactive decayed",
		season.Number, len(records), decayed)
}

// createNextSeason creates the next season with rotating environment.
func (r *SeasonRotator) createNextSeason(prev *model.Season) {
	environments := []struct {
		env  string
		name string
	}{
		{"abyss", "深渊"},
		{"terrain", "大地"},
		{"sky", "天空"},
	}

	nextNum := prev.Number + 1
	envIdx := (nextNum - 1) % 3
	env := environments[envIdx]

	seasonNames := []string{
		"觉醒", "狂潮", "裂变", "回响", "吞噬", "升华",
		"风暴", "寂灭", "重生", "天启", "混沌", "永恒",
	}
	nameIdx := (nextNum - 1) % len(seasonNames)
	name := fmt.Sprintf("%s·%s", env.name, seasonNames[nameIdx])

	now := time.Now()
	next := model.Season{
		Name:          name,
		Number:        nextNum,
		Environment:   env.env,
		Active:        true,
		StartAt:       now,
		EndAt:         now.AddDate(0, 1, 0), // 1 month
		PathATKBonus:  15,
		PathDEFBonus:  10,
		PathSPDBonus:  10,
		InactiveDecay: 50,
	}
	r.db.Create(&next)

	log.Printf("[season] New season #%d %q (%s) created, ends %s",
		nextNum, name, env.env, next.EndAt.Format("2006-01-02"))
}

// seasonReward returns stardust reward for a given rank.
func seasonReward(rank int) int {
	switch rank {
	case 1:
		return 500
	case 2:
		return 300
	case 3:
		return 200
	case 4, 5:
		return 100
	case 6, 7, 8, 9, 10:
		return 50
	default:
		return 0
	}
}
