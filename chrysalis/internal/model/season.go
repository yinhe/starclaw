package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Season represents a competitive season with environment buffs.
type Season struct {
	ID          string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	Name        string    `json:"name" gorm:"type:varchar(100);not null"`
	Number      int       `json:"number" gorm:"uniqueIndex;not null"`
	Environment string    `json:"environment" gorm:"type:varchar(30);not null"` // abyss / terrain / sky
	Active      bool      `json:"active" gorm:"default:false;index"`
	StartAt     time.Time `json:"start_at"`
	EndAt       time.Time `json:"end_at"`

	// Environment buffs for the favored path (percentage bonus)
	PathATKBonus int `json:"path_atk_bonus" gorm:"default:15"` // e.g. abyss season → abyss fighters get +15% ATK
	PathDEFBonus int `json:"path_def_bonus" gorm:"default:10"`
	PathSPDBonus int `json:"path_spd_bonus" gorm:"default:10"`

	// Decay: inactive fighters lose ELO per season
	InactiveDecay int `json:"inactive_decay" gorm:"default:50"` // ELO points lost if 0 battles in season

	CreatedAt time.Time `json:"created_at"`
}

func (s *Season) BeforeCreate(tx *gorm.DB) error {
	if s.ID == "" {
		s.ID = uuid.New().String()
	}
	return nil
}

// SeasonRecord tracks a fighter's performance in a specific season.
type SeasonRecord struct {
	ID         string `json:"id" gorm:"type:varchar(36);primaryKey"`
	SeasonID   string `json:"season_id" gorm:"type:varchar(36);index;not null"`
	FighterID  string `json:"fighter_id" gorm:"type:varchar(36);index;not null"`
	ClawID     string `json:"claw_id" gorm:"type:varchar(60);index;not null"`
	Wins       int    `json:"wins" gorm:"default:0"`
	Losses     int    `json:"losses" gorm:"default:0"`
	PeakELO    int    `json:"peak_elo" gorm:"default:1000"`
	SeasonRank int    `json:"season_rank" gorm:"default:0"`
	Decayed    bool   `json:"decayed" gorm:"default:false"` // whether inactive decay was applied

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (r *SeasonRecord) BeforeCreate(tx *gorm.DB) error {
	if r.ID == "" {
		r.ID = uuid.New().String()
	}
	return nil
}

// SeedSeasons returns initial seasons.
func SeedSeasons() []Season {
	now := time.Now()
	return []Season{
		{
			ID: "season-1", Name: "深渊觉醒", Number: 1, Environment: "abyss",
			Active: true, StartAt: now, EndAt: now.AddDate(0, 1, 0),
			PathATKBonus: 15, PathDEFBonus: 10, PathSPDBonus: 10, InactiveDecay: 50,
		},
	}
}
