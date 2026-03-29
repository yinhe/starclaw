package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// BattleFighter represents a Claw node's pet registered for PK battles.
// One per node (identified by NodeID / ClawID).
type BattleFighter struct {
	ID            string `json:"id" gorm:"type:varchar(36);primaryKey"`
	ClawID        string `json:"claw_id" gorm:"type:varchar(60);uniqueIndex;not null"`
	Name          string `json:"name" gorm:"type:varchar(100);not null"`
	Level         int    `json:"level" gorm:"default:1"`
	EvolutionPath string `json:"evolution_path" gorm:"type:varchar(20);default:larva"` // abyss/terrain/sky/larva
	FormCode      string `json:"form_code" gorm:"type:varchar(20)"`

	// Base stats (synced from Claw growth system)
	BaseHP  int `json:"base_hp" gorm:"default:50"`
	BaseATK int `json:"base_atk" gorm:"default:10"`
	BaseDEF int `json:"base_def" gorm:"default:10"`
	BaseSPD int `json:"base_spd" gorm:"default:10"`

	// ELO rating
	ELO      int `json:"elo" gorm:"default:1000"`
	WinCount int `json:"win_count" gorm:"default:0"`
	LoseCount int `json:"lose_count" gorm:"default:0"`
	WinStreak int `json:"win_streak" gorm:"default:0"`

	// Equipment slots (FK to EquipmentInstance)
	WeaponID  string `json:"weapon_id" gorm:"type:varchar(36)"`
	ArmorID   string `json:"armor_id" gorm:"type:varchar(36)"`
	TrinketID string `json:"trinket_id" gorm:"type:varchar(36)"`

	LastBattleAt *time.Time `json:"last_battle_at"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

func (f *BattleFighter) BeforeCreate(tx *gorm.DB) error {
	if f.ID == "" {
		f.ID = uuid.New().String()
	}
	return nil
}

// EquipmentDef defines a type of equipment available in the shop.
type EquipmentDef struct {
	ID       string `json:"id" gorm:"type:varchar(36);primaryKey"`
	Name     string `json:"name" gorm:"type:varchar(100);not null"`
	Slot     string `json:"slot" gorm:"type:varchar(20);not null;index"` // weapon / armor / trinket
	Quality  string `json:"quality" gorm:"type:varchar(20);not null"`    // white / green / blue / purple / orange
	PathOnly string `json:"path_only" gorm:"type:varchar(20)"`           // empty = universal, else abyss/terrain/sky

	BonusHP  int `json:"bonus_hp" gorm:"default:0"`
	BonusATK int `json:"bonus_atk" gorm:"default:0"`
	BonusDEF int `json:"bonus_def" gorm:"default:0"`
	BonusSPD int `json:"bonus_spd" gorm:"default:0"`

	CritRateBonus int `json:"crit_rate_bonus" gorm:"default:0"` // percentage points

	SpecialCode string `json:"special_code" gorm:"type:varchar(50)"` // special effect code
	SpecialDesc string `json:"special_desc" gorm:"type:varchar(200)"`

	PriceStar int `json:"price_star" gorm:"default:0"` // star energy cost (0 = not buyable with star)
	PriceDust int `json:"price_dust" gorm:"default:0"` // stardust cost (0 = not buyable with dust)

	CreatedAt time.Time `json:"created_at"`
}

// EquipmentInstance is an owned equipment item.
type EquipmentInstance struct {
	ID       string `json:"id" gorm:"type:varchar(36);primaryKey"`
	ClawID   string `json:"claw_id" gorm:"type:varchar(60);index;not null"`
	DefID    string `json:"def_id" gorm:"type:varchar(36);not null"`
	Equipped bool   `json:"equipped" gorm:"default:false"`

	// Actual stats (may vary ±15% from def base for crafted items)
	BonusHP  int `json:"bonus_hp" gorm:"default:0"`
	BonusATK int `json:"bonus_atk" gorm:"default:0"`
	BonusDEF int `json:"bonus_def" gorm:"default:0"`
	BonusSPD int `json:"bonus_spd" gorm:"default:0"`

	EnhanceLevel int `json:"enhance_level" gorm:"default:0"` // 0-3

	CreatedAt time.Time `json:"created_at"`
}

func (e *EquipmentInstance) BeforeCreate(tx *gorm.DB) error {
	if e.ID == "" {
		e.ID = uuid.New().String()
	}
	return nil
}

// Battle records a completed PK battle.
type Battle struct {
	ID string `json:"id" gorm:"type:varchar(36);primaryKey"`

	FighterAID   string `json:"fighter_a_id" gorm:"type:varchar(36);index;not null"`
	FighterAName string `json:"fighter_a_name" gorm:"type:varchar(100)"`
	FighterAPath string `json:"fighter_a_path" gorm:"type:varchar(20)"`
	FighterALv   int    `json:"fighter_a_lv"`

	FighterBID   string `json:"fighter_b_id" gorm:"type:varchar(36);index;not null"`
	FighterBName string `json:"fighter_b_name" gorm:"type:varchar(100)"`
	FighterBPath string `json:"fighter_b_path" gorm:"type:varchar(20)"`
	FighterBLv   int    `json:"fighter_b_lv"`

	WinnerID string `json:"winner_id" gorm:"type:varchar(36);index"`
	Rounds   int    `json:"rounds"`

	ELOChangeA int `json:"elo_change_a"` // positive = gained
	ELOChangeB int `json:"elo_change_b"`

	Log string `json:"log" gorm:"type:longtext"` // JSON battle log

	CreatedAt time.Time `json:"created_at"`
}

func (b *Battle) BeforeCreate(tx *gorm.DB) error {
	if b.ID == "" {
		b.ID = uuid.New().String()
	}
	return nil
}
