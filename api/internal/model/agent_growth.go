package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// EvolutionPath represents the six evolution directions (Earth creature paths).
type EvolutionPath string

const (
	PathLarva    EvolutionPath = "larva"    // Lv 1-4, 未分化（共用起点）
	PathOcean    EvolutionPath = "ocean"    // 🌊 水域之路 — 海洋霸主 (HP+DEF)
	PathTerrain  EvolutionPath = "terrain"  // 🏔️ 大地之路 — 陆地之王 (ATK+SPD)
	PathSky      EvolutionPath = "sky"      // 🌪️ 天空之路 — 空中统治 (SPD+ATK)
	PathWisdom   EvolutionPath = "wisdom"   // 🧬 智慧之路 — 进化巅峰 (均衡+技能多)
	PathAncient  EvolutionPath = "ancient"  // 🔥 远古之路 — 史前巨兽 (HP+ATK)
	PathSymbiont EvolutionPath = "symbiont" // 🌿 共生之路 — 生态守护 (DEF+治愈)
	// Legacy aliases
	PathAbyss EvolutionPath = "abyss" // deprecated: maps to ocean
)

// RealmPath represents the three spiritual cultivation paths (仙魔妖).
type RealmPath string

const (
	RealmNone     RealmPath = ""         // 凡境 (Mortal)
	RealmImmortal RealmPath = "immortal" // ✨ 仙道 — 守护者 (DEF+HP, 治愈)
	RealmDemon    RealmPath = "demon"    // 🔥 魔道 — 毁灭者 (ATK+SPD, 吸收)
	RealmMonster  RealmPath = "monster"  // 🌿 妖道 — 变化者 (SPD+ATK, 幻术)
)

// SwarmUnitType represents the auto-classified type of a swarm unit (Agent → 虫).
type SwarmUnitType string

const (
	SwarmFinancial SwarmUnitType = "financial" // 🏦 财务官虫
	SwarmCreative  SwarmUnitType = "creative"  // 🎬 创意虫
	SwarmSocial    SwarmUnitType = "social"    // 💬 社交虫
	SwarmEngineer  SwarmUnitType = "engineer"  // 💻 工程虫
	SwarmScout     SwarmUnitType = "scout"     // 🔍 侦察虫
	SwarmScholar   SwarmUnitType = "scholar"   // 🧠 学者虫
	SwarmGeneric   SwarmUnitType = "generic"   // ⚔️ 通用虫
)

// LevelThresholds lists the 12-stage evolution breakpoints.
var LevelThresholds = []int{1, 3, 5, 8, 12, 16, 20, 25, 30, 38, 45, 50}

// LevelTitles maps (path, level) → Chinese title (Earth creatures).
var LevelTitles = map[EvolutionPath]map[int]string{
	PathLarva:    {1: "浮游幼体", 3: "虾苗", 5: "小龙虾"},
	PathOcean:    {1: "浮游幼体", 3: "虾苗", 5: "小龙虾", 8: "帝王蟹", 12: "章鱼", 16: "大白鲨", 20: "海豚", 25: "大王乌贼", 30: "虎鲸", 38: "蓝鲸", 45: "沧龙", 50: "利维坦"},
	PathTerrain:  {1: "浮游幼体", 3: "虾苗", 5: "小龙虾", 8: "蝎子", 12: "科莫多龙", 16: "灰狼", 20: "灰熊", 25: "狮子", 30: "非洲象", 38: "猛犸象", 45: "腕龙", 50: "霸王龙"},
	PathSky:      {1: "浮游幼体", 3: "虾苗", 5: "小龙虾", 8: "蜻蜓", 12: "猫头鹰", 16: "猎隼", 20: "金雕", 25: "安第斯神鹫", 30: "巨型果蝠", 38: "翼龙", 45: "阿根廷巨鹰", 50: "凤凰"},
	PathWisdom:   {1: "浮游幼体", 3: "虾苗", 5: "小龙虾", 8: "乌鸦", 12: "章鱼", 16: "海豚", 20: "大象", 25: "大猩猩", 30: "黑猩猩", 38: "智人", 45: "达·芬奇", 50: "超智体"},
	PathAncient:  {1: "浮游幼体", 3: "虾苗", 5: "小龙虾", 8: "三叶虫", 12: "邓氏鱼", 16: "异齿龙", 20: "帝鳄", 25: "迅猛龙", 30: "棘龙", 38: "霸王龙", 45: "龙", 50: "哥斯拉"},
	PathSymbiont: {1: "浮游幼体", 3: "虾苗", 5: "小龙虾", 8: "蜜蜂", 12: "珊瑚", 16: "灰狼", 20: "红杉", 25: "大象", 30: "灯塔水母", 38: "菌丝网络", 45: "世界树", 50: "盖亚"},
	PathAbyss:    {1: "浮游幼体", 3: "虾苗", 5: "小龙虾", 8: "帝王蟹", 12: "章鱼", 16: "大白鲨", 20: "海豚", 25: "大王乌贼", 30: "虎鲸", 38: "蓝鲸", 45: "沧龙", 50: "利维坦"},
}

// LevelTitlesEN maps (path, level) → English title.
var LevelTitlesEN = map[EvolutionPath]map[int]string{
	PathLarva:    {1: "Nauplius", 3: "Shrimplet", 5: "Crayfish"},
	PathOcean:    {1: "Nauplius", 3: "Shrimplet", 5: "Crayfish", 8: "King Crab", 12: "Octopus", 16: "Great White", 20: "Dolphin", 25: "Giant Squid", 30: "Orca", 38: "Blue Whale", 45: "Mosasaurus", 50: "Leviathan"},
	PathTerrain:  {1: "Nauplius", 3: "Shrimplet", 5: "Crayfish", 8: "Scorpion", 12: "Komodo Dragon", 16: "Gray Wolf", 20: "Grizzly Bear", 25: "Lion", 30: "African Elephant", 38: "Mammoth", 45: "Brachiosaurus", 50: "T-Rex"},
	PathSky:      {1: "Nauplius", 3: "Shrimplet", 5: "Crayfish", 8: "Dragonfly", 12: "Owl", 16: "Peregrine Falcon", 20: "Golden Eagle", 25: "Andean Condor", 30: "Giant Bat", 38: "Pteranodon", 45: "Argentavis", 50: "Phoenix"},
	PathWisdom:   {1: "Nauplius", 3: "Shrimplet", 5: "Crayfish", 8: "Crow", 12: "Octopus", 16: "Dolphin", 20: "Elephant", 25: "Gorilla", 30: "Chimpanzee", 38: "Homo Sapiens", 45: "Da Vinci", 50: "Superintelligence"},
	PathAncient:  {1: "Nauplius", 3: "Shrimplet", 5: "Crayfish", 8: "Trilobite", 12: "Dunkleosteus", 16: "Dimetrodon", 20: "Sarcosuchus", 25: "Velociraptor", 30: "Spinosaurus", 38: "T-Rex", 45: "Dragon", 50: "Godzilla"},
	PathSymbiont: {1: "Nauplius", 3: "Shrimplet", 5: "Crayfish", 8: "Honeybee", 12: "Coral", 16: "Wolf", 20: "Redwood", 25: "Elephant", 30: "Immortal Jellyfish", 38: "Mycelium Network", 45: "Yggdrasil", 50: "Gaia"},
	PathAbyss:    {1: "Nauplius", 3: "Shrimplet", 5: "Crayfish", 8: "King Crab", 12: "Octopus", 16: "Great White", 20: "Dolphin", 25: "Giant Squid", 30: "Orca", 38: "Blue Whale", 45: "Mosasaurus", 50: "Leviathan"},
}

// LevelFormCodes maps (path, level) → form code for SVG asset lookup.
var LevelFormCodes = map[EvolutionPath]map[int]string{
	PathLarva:    {1: "nauplius", 3: "shrimplet", 5: "crayfish"},
	PathOcean:    {1: "nauplius", 3: "shrimplet", 5: "crayfish", 8: "king_crab", 12: "octopus", 16: "great_white", 20: "dolphin", 25: "giant_squid", 30: "orca", 38: "blue_whale", 45: "mosasaurus", 50: "leviathan"},
	PathTerrain:  {1: "nauplius", 3: "shrimplet", 5: "crayfish", 8: "scorpion", 12: "komodo", 16: "wolf", 20: "grizzly", 25: "lion", 30: "elephant", 38: "mammoth", 45: "brachiosaurus", 50: "trex"},
	PathSky:      {1: "nauplius", 3: "shrimplet", 5: "crayfish", 8: "dragonfly", 12: "owl", 16: "falcon", 20: "eagle", 25: "condor", 30: "bat", 38: "pteranodon", 45: "argentavis", 50: "phoenix"},
	PathWisdom:   {1: "nauplius", 3: "shrimplet", 5: "crayfish", 8: "crow", 12: "octopus", 16: "dolphin", 20: "elephant", 25: "gorilla", 30: "chimp", 38: "sapiens", 45: "davinci", 50: "superintelligence"},
	PathAncient:  {1: "nauplius", 3: "shrimplet", 5: "crayfish", 8: "trilobite", 12: "dunkleosteus", 16: "dimetrodon", 20: "sarcosuchus", 25: "raptor", 30: "spinosaurus", 38: "trex", 45: "dragon", 50: "godzilla"},
	PathSymbiont: {1: "nauplius", 3: "shrimplet", 5: "crayfish", 8: "bee", 12: "coral", 16: "wolf", 20: "redwood", 25: "elephant", 30: "jellyfish", 38: "mycelium", 45: "yggdrasil", 50: "gaia"},
	PathAbyss:    {1: "nauplius", 3: "shrimplet", 5: "crayfish", 8: "king_crab", 12: "octopus", 16: "great_white", 20: "dolphin", 25: "giant_squid", 30: "orca", 38: "blue_whale", 45: "mosasaurus", 50: "leviathan"},
}

// GetTitle returns the evolution title for a given path and level.
func GetTitle(path EvolutionPath, level int) (string, string) {
	titles, ok := LevelTitles[path]
	if !ok {
		titles = LevelTitles[PathAbyss]
	}
	titlesEN := LevelTitlesEN[path]

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

// NodeGrowth tracks the growth path for this Claw node (龙虾英雄).
// EXP is computed on-the-fly from existing data, NOT stored here.
type NodeGrowth struct {
	ID            string        `json:"id" gorm:"type:varchar(36);primaryKey"`
	UserID        string        `json:"user_id" gorm:"type:varchar(36);uniqueIndex;not null"`
	EvolutionPath EvolutionPath `json:"evolution_path" gorm:"type:varchar(20);default:larva"`
	Level         int           `json:"level" gorm:"default:0"`
	FirstChat     *time.Time    `json:"first_chat"`
	// v2: Realm system (仙魔妖)
	RealmPath      RealmPath `json:"realm_path" gorm:"type:varchar(20);default:''"`
	RealmLevel     int       `json:"realm_level" gorm:"default:0"`     // 0=凡, 1=人, 2=仙徒/魔徒/妖修, 3=仙人/魔将/妖王, 4=神/魔神/妖皇, 5=圣
	AwakeningStars int       `json:"awakening_stars" gorm:"default:0"` // 0-5+, post-Lv.50 growth
	Generation     int       `json:"generation" gorm:"default:0"`      // rebirth count (0=first life)
	// v2: Stardust
	StardustBalance int       `json:"stardust_balance" gorm:"default:0"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// SwarmUnit represents a single swarm member (one per Agent).
type SwarmUnit struct {
	ID               string        `json:"id" gorm:"type:varchar(36);primaryKey"`
	NodeID           string        `json:"node_id" gorm:"type:varchar(64);index;not null"`
	AgentID          string        `json:"agent_id" gorm:"type:varchar(36);index;not null"`
	AgentName        string        `json:"agent_name" gorm:"type:varchar(200)"`
	UnitType         SwarmUnitType `json:"unit_type" gorm:"type:varchar(20);default:generic"`
	Level            int           `json:"level" gorm:"default:1"`
	Exp              int           `json:"exp" gorm:"default:0"`
	HP               int           `json:"hp" gorm:"default:10"`
	ATK              int           `json:"atk" gorm:"default:10"`
	DEF              int           `json:"def" gorm:"default:5"`
	SPD              int           `json:"spd" gorm:"default:10"`
	Skill1           string        `json:"skill_1" gorm:"type:varchar(100)"`
	Skill2           string        `json:"skill_2" gorm:"type:varchar(100)"`
	Skill3           string        `json:"skill_3" gorm:"type:varchar(100)"`
	StardustInvested int           `json:"stardust_invested" gorm:"default:0"`
	Skin             string        `json:"skin" gorm:"type:varchar(50)"`
	CreatedAt        time.Time     `json:"created_at"`
	UpdatedAt        time.Time     `json:"updated_at"`
}

func (s *SwarmUnit) BeforeCreate(tx *gorm.DB) error {
	if s.ID == "" {
		s.ID = uuid.New().String()
	}
	return nil
}

// ClassifySwarmUnit determines the unit type based on Agent's tools.
func ClassifySwarmUnit(tools string) SwarmUnitType {
	if tools == "" {
		return SwarmGeneric
	}
	switch {
	case containsAny(tools, "trading_scan", "trading_buy", "trading_sell"):
		return SwarmFinancial
	case containsAny(tools, "video_generation", "image_generation", "music_generation", "mv_production", "comic_production"):
		return SwarmCreative
	case containsAny(tools, "wechat", "telegram", "slack", "discord", "dingtalk", "feishu", "wecom"):
		return SwarmSocial
	case containsAny(tools, "code", "git"):
		return SwarmEngineer
	case containsAny(tools, "web_search", "browser", "http_request"):
		return SwarmScout
	case containsAny(tools, "knowledge", "rag", "document"):
		return SwarmScholar
	default:
		return SwarmGeneric
	}
}

// StardustTransaction records stardust income/spending.
type StardustTransaction struct {
	ID        string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	UserID    string    `json:"user_id" gorm:"type:varchar(36);index;not null"`
	Amount    int       `json:"amount" gorm:"not null"`            // positive=earn, negative=spend
	Type      string    `json:"type" gorm:"type:varchar(30)"`      // earn_daily, earn_chat, earn_task, spend_enhance, spend_hatch
	TargetID  string    `json:"target_id" gorm:"type:varchar(36)"` // target unit/hero ID
	Note      string    `json:"note" gorm:"type:varchar(200)"`
	CreatedAt time.Time `json:"created_at"`
}

func (t *StardustTransaction) BeforeCreate(tx *gorm.DB) error {
	if t.ID == "" {
		t.ID = uuid.New().String()
	}
	return nil
}

// helper
func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if len(s) > 0 && len(sub) > 0 {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
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
