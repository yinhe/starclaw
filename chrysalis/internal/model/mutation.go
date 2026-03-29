package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Mutation represents a special trait gained during evolution.
// Triggered when a fighter levels up past certain thresholds.
type Mutation struct {
	ID         string `json:"id" gorm:"type:varchar(36);primaryKey"`
	FighterID  string `json:"fighter_id" gorm:"type:varchar(36);index;not null"`
	ClawID     string `json:"claw_id" gorm:"type:varchar(60);index;not null"`
	Code       string `json:"code" gorm:"type:varchar(50);not null"`
	Name       string `json:"name" gorm:"type:varchar(100);not null"`
	Desc       string `json:"desc" gorm:"type:varchar(300)"`
	Rarity     string `json:"rarity" gorm:"type:varchar(20)"` // common / rare / legendary
	BonusHP    int    `json:"bonus_hp" gorm:"default:0"`
	BonusATK   int    `json:"bonus_atk" gorm:"default:0"`
	BonusDEF   int    `json:"bonus_def" gorm:"default:0"`
	BonusSPD   int    `json:"bonus_spd" gorm:"default:0"`
	SpecialCode string `json:"special_code" gorm:"type:varchar(50)"` // special combat effect
	TriggeredAt time.Time `json:"triggered_at"`
}

func (m *Mutation) BeforeCreate(tx *gorm.DB) error {
	if m.ID == "" {
		m.ID = uuid.New().String()
	}
	return nil
}

// MutationDef defines possible mutations and their trigger conditions.
type MutationDef struct {
	Code        string
	Name        string
	Desc        string
	Rarity      string  // common (60%) / rare (30%) / legendary (10%)
	PathOnly    string  // empty = any path
	MinLevel    int
	BonusHP     int
	BonusATK    int
	BonusDEF    int
	BonusSPD    int
	SpecialCode string
}

// MutationPool is the pool of possible mutations.
var MutationPool = []MutationDef{
	// Common mutations (60% chance)
	{Code: "tough_shell", Name: "硬壳强化", Desc: "外壳变得更加坚硬", Rarity: "common", BonusDEF: 5},
	{Code: "sharp_claw", Name: "利爪突变", Desc: "爪子变得更加锋利", Rarity: "common", BonusATK: 5},
	{Code: "swift_legs", Name: "迅捷步足", Desc: "行动速度微幅提升", Rarity: "common", BonusSPD: 5},
	{Code: "thick_hide", Name: "厚皮增生", Desc: "体质略微增强", Rarity: "common", BonusHP: 20},
	{Code: "keen_sense", Name: "敏锐感知", Desc: "感知能力增强", Rarity: "common", BonusSPD: 3, BonusATK: 2},

	// Rare mutations (30% chance)
	{Code: "abyss_eye", Name: "深渊之眼", Desc: "获得渊系暗视能力，暴击率+5%", Rarity: "rare", PathOnly: "abyss",
		BonusHP: 30, SpecialCode: "crit_5"},
	{Code: "earth_anchor", Name: "大地锚定", Desc: "根植大地，受击时减伤10%", Rarity: "rare", PathOnly: "terrain",
		BonusDEF: 15, SpecialCode: "dmg_reduce_10"},
	{Code: "wind_step", Name: "御风步", Desc: "踏风而行，先手概率大幅提升", Rarity: "rare", PathOnly: "sky",
		BonusSPD: 20, SpecialCode: "first_strike"},
	{Code: "regeneration", Name: "再生能力", Desc: "每回合恢复3%最大HP", Rarity: "rare",
		BonusHP: 50, SpecialCode: "regen_3pct"},
	{Code: "berserker", Name: "狂化本能", Desc: "HP低于30%时ATK+50%", Rarity: "rare",
		BonusATK: 10, SpecialCode: "berserk_30"},

	// Legendary mutations (10% chance)
	{Code: "leviathan_blood", Name: "利维坦之血", Desc: "传说级深渊血脉觉醒", Rarity: "legendary", PathOnly: "abyss",
		BonusHP: 100, BonusDEF: 30, SpecialCode: "leviathan"},
	{Code: "titan_fist", Name: "泰坦之拳", Desc: "传说级大地之力觉醒", Rarity: "legendary", PathOnly: "terrain",
		BonusATK: 50, BonusHP: 50, SpecialCode: "titan"},
	{Code: "phoenix_wing", Name: "凤凰之翼", Desc: "传说级穹天之力觉醒", Rarity: "legendary", PathOnly: "sky",
		BonusSPD: 40, BonusATK: 30, SpecialCode: "phoenix"},
}
