package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// CraftMaterial represents a crafting material owned by a node.
type CraftMaterial struct {
	ID        string `json:"id" gorm:"type:varchar(36);primaryKey"`
	ClawID    string `json:"claw_id" gorm:"type:varchar(60);index;not null"`
	MaterialID string `json:"material_id" gorm:"type:varchar(36);index;not null"`
	Quantity  int    `json:"quantity" gorm:"default:0"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (m *CraftMaterial) BeforeCreate(tx *gorm.DB) error {
	if m.ID == "" {
		m.ID = uuid.New().String()
	}
	return nil
}

// MaterialDef defines a type of crafting material.
type MaterialDef struct {
	ID       string `json:"id" gorm:"type:varchar(36);primaryKey"`
	Name     string `json:"name" gorm:"type:varchar(100);not null"`
	Icon     string `json:"icon" gorm:"type:varchar(10)"`
	Rarity   string `json:"rarity" gorm:"type:varchar(20)"` // common / uncommon / rare / epic
	Source   string `json:"source" gorm:"type:varchar(50)"` // battle_drop / daily_collect / season_reward
	Desc     string `json:"desc" gorm:"type:varchar(200)"`
}

// CraftRecipe defines how to craft an equipment from materials.
type CraftRecipe struct {
	ID       string `json:"id" gorm:"type:varchar(36);primaryKey"`
	ResultID string `json:"result_id" gorm:"type:varchar(36);not null"` // EquipmentDef.ID
	ResultName string `json:"result_name" gorm:"type:varchar(100)"`

	// Materials needed (JSON: [{"material_id":"x","quantity":N}, ...])
	Materials string `json:"materials" gorm:"type:text;not null"`

	DustCost  int `json:"dust_cost" gorm:"default:0"`  // additional stardust cost
	LevelReq  int `json:"level_req" gorm:"default:0"`  // minimum fighter level
}

// SeedMaterials returns initial material definitions.
func SeedMaterials() []MaterialDef {
	return []MaterialDef{
		{ID: "mat-shell", Name: "硬壳碎片", Icon: "🐚", Rarity: "common", Source: "battle_drop", Desc: "战斗中掉落的甲壳碎片"},
		{ID: "mat-claw", Name: "利爪残片", Icon: "🦀", Rarity: "common", Source: "battle_drop", Desc: "锋利的爪部残片"},
		{ID: "mat-crystal", Name: "深海晶核", Icon: "💎", Rarity: "uncommon", Source: "battle_drop", Desc: "蕴含深渊能量的水晶"},
		{ID: "mat-feather", Name: "穹风羽毛", Icon: "🪶", Rarity: "uncommon", Source: "battle_drop", Desc: "高空巡风者的羽翼"},
		{ID: "mat-stone", Name: "大地岩心", Icon: "🪨", Rarity: "uncommon", Source: "battle_drop", Desc: "大地深处的坚硬岩石"},
		{ID: "mat-essence", Name: "进化精华", Icon: "✨", Rarity: "rare", Source: "daily_collect", Desc: "每日签到可获得的珍稀精华"},
		{ID: "mat-star", Name: "星尘残渣", Icon: "⭐", Rarity: "rare", Source: "season_reward", Desc: "赛季结束时发放的奖励材料"},
		{ID: "mat-dragon", Name: "龙虾至尊壳", Icon: "🦞", Rarity: "epic", Source: "season_reward", Desc: "赛季前三名专属材料"},
	}
}

// SeedRecipes returns initial crafting recipes.
func SeedRecipes() []CraftRecipe {
	return []CraftRecipe{
		// Green tier: simple recipes
		{ID: "r-w-green-1", ResultID: "w-green-1", ResultName: "腐蚀刺针",
			Materials: `[{"material_id":"mat-claw","quantity":5},{"material_id":"mat-shell","quantity":3}]`, DustCost: 50},
		{ID: "r-a-green-1", ResultID: "a-green-1", ResultName: "菌毯披风",
			Materials: `[{"material_id":"mat-shell","quantity":8}]`, DustCost: 50},
		// Blue tier: path materials
		{ID: "r-w-blue-abyss", ResultID: "w-blue-abyss", ResultName: "深渊三叉戟",
			Materials: `[{"material_id":"mat-crystal","quantity":5},{"material_id":"mat-claw","quantity":10}]`, DustCost: 200, LevelReq: 5},
		{ID: "r-w-blue-terrain", ResultID: "w-blue-terrain", ResultName: "裂地巨锤",
			Materials: `[{"material_id":"mat-stone","quantity":5},{"material_id":"mat-claw","quantity":10}]`, DustCost: 200, LevelReq: 5},
		{ID: "r-w-blue-sky", ResultID: "w-blue-sky", ResultName: "穹风之翼",
			Materials: `[{"material_id":"mat-feather","quantity":5},{"material_id":"mat-claw","quantity":10}]`, DustCost: 200, LevelReq: 5},
		// Purple tier: requires rare materials
		{ID: "r-w-purple-1", ResultID: "w-purple-1", ResultName: "毁灭之爪",
			Materials: `[{"material_id":"mat-essence","quantity":10},{"material_id":"mat-crystal","quantity":8},{"material_id":"mat-claw","quantity":20}]`, DustCost: 1000, LevelReq: 10},
		{ID: "r-a-purple-1", ResultID: "a-purple-1", ResultName: "利维坦之鳞",
			Materials: `[{"material_id":"mat-essence","quantity":10},{"material_id":"mat-stone","quantity":8},{"material_id":"mat-shell","quantity":20}]`, DustCost: 1000, LevelReq: 10},
		{ID: "r-t-purple-1", ResultID: "t-purple-1", ResultName: "虫后祝福",
			Materials: `[{"material_id":"mat-dragon","quantity":1},{"material_id":"mat-star","quantity":5},{"material_id":"mat-essence","quantity":5}]`, DustCost: 1500, LevelReq: 15},
	}
}
