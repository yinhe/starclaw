package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// EvolutionPath represents the three evolution directions.
type EvolutionPath string

const (
	PathLarva   EvolutionPath = "larva"   // Lv 1-4, 未分化
	PathAbyss   EvolutionPath = "abyss"   // 🌊 渊·鲲之路 — 知识深潜型
	PathTerrain EvolutionPath = "terrain" // 🏔️ 陆·兽之路 — 执行驱动型
	PathSky     EvolutionPath = "sky"     // 🌪️ 穹·鹏之路 — 沟通创意型
)

// LevelThresholds lists the evolution breakpoints.
var LevelThresholds = []int{1, 5, 10, 20, 30, 50}

// LevelTitles maps (path, level) → title.
var LevelTitles = map[EvolutionPath]map[int]string{
	PathAbyss: {
		1: "小龙虾", 5: "章鱼", 10: "蛟", 20: "鲲", 30: "利维坦", 50: "渊皇",
	},
	PathTerrain: {
		1: "跳虫", 5: "刺蛇", 10: "潜伏者", 20: "雷兽", 30: "泰坦", 50: "陆皇",
	},
	PathSky: {
		1: "翼龙", 5: "阿根廷巨鹰", 10: "飞龙", 20: "鹏", 30: "守护者", 50: "穹皇",
	},
}

// LevelTitlesEN maps (path, level) → English title.
var LevelTitlesEN = map[EvolutionPath]map[int]string{
	PathAbyss: {
		1: "Claw", 5: "Octopus", 10: "Jiao", 20: "Kun", 30: "Leviathan", 50: "Abyssal",
	},
	PathTerrain: {
		1: "Zergling", 5: "Hydralisk", 10: "Lurker", 20: "Ultralisk", 30: "Titan", 50: "Colossus",
	},
	PathSky: {
		1: "Pterosaur", 5: "Argentavis", 10: "Mutalisk", 20: "Peng", 30: "Guardian", 50: "Skyward",
	},
}

// LevelFormCodes maps (path, level) → form code for SVG asset lookup.
var LevelFormCodes = map[EvolutionPath]map[int]string{
	PathAbyss: {
		1: "claw", 5: "octopus", 10: "jiao", 20: "kun", 30: "leviathan", 50: "abyssal",
	},
	PathTerrain: {
		1: "zergling", 5: "hydralisk", 10: "lurker", 20: "ultralisk", 30: "titan", 50: "colossus",
	},
	PathSky: {
		1: "pterosaur", 5: "argentavis", 10: "mutalisk", 20: "peng", 30: "guardian", 50: "skyward",
	},
}

// GetTitle returns the evolution title for a given path and level.
func GetTitle(path EvolutionPath, level int) (string, string) {
	titles, ok := LevelTitles[path]
	if !ok {
		titles = LevelTitles[PathAbyss]
	}
	titlesEN, _ := LevelTitlesEN[path]

	bestLv := 1
	for _, lv := range LevelThresholds {
		if level >= lv {
			bestLv = lv
		}
	}
	return titles[bestLv], titlesEN[bestLv]
}

// GetFormCode returns the SVG form code for a given path and level.
func GetFormCode(path EvolutionPath, level int) string {
	forms, ok := LevelFormCodes[path]
	if !ok {
		forms = LevelFormCodes[PathAbyss]
	}
	bestLv := 1
	for _, lv := range LevelThresholds {
		if level >= lv {
			bestLv = lv
		}
	}
	return forms[bestLv]
}

// NodeGrowth tracks the growth path for this Claw node (one pet per node).
// EXP is computed on-the-fly from existing data, NOT stored here.
type NodeGrowth struct {
	ID            string        `json:"id" gorm:"type:varchar(36);primaryKey"`
	UserID        string        `json:"user_id" gorm:"type:varchar(36);uniqueIndex;not null"`
	EvolutionPath EvolutionPath `json:"evolution_path" gorm:"type:varchar(20);default:larva"`
	Level         int           `json:"level" gorm:"default:0"`
	FirstChat     *time.Time    `json:"first_chat"`
	CreatedAt     time.Time     `json:"created_at"`
	UpdatedAt     time.Time     `json:"updated_at"`
}

func (g *NodeGrowth) BeforeCreate(tx *gorm.DB) error {
	if g.ID == "" {
		g.ID = uuid.New().String()
	}
	return nil
}

// Milestone records a completed growth milestone for this Claw node.
type Milestone struct {
	ID         string     `json:"id" gorm:"type:varchar(36);primaryKey"`
	UserID     string     `json:"user_id" gorm:"type:varchar(36);index;not null"`
	Code       string     `json:"code" gorm:"type:varchar(50);not null"`
	Title      string     `json:"title" gorm:"type:varchar(200);not null"`
	AchievedAt time.Time  `json:"achieved_at"`
	NotifiedAt *time.Time `json:"notified_at"`
}

func (m *Milestone) BeforeCreate(tx *gorm.DB) error {
	if m.ID == "" {
		m.ID = uuid.New().String()
	}
	return nil
}
