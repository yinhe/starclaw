package engine

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
)

// Fighter holds the runtime combat state for one side of a battle.
type Fighter struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Path string `json:"path"` // abyss / terrain / sky / larva
	Lv   int    `json:"lv"`

	MaxHP int `json:"max_hp"`
	HP    int `json:"hp"`
	ATK   int `json:"atk"`
	DEF   int `json:"def"`
	SPD   int `json:"spd"`

	CritRate       int `json:"crit_rate"`       // percentage (default 10)
	CritMultiplier int `json:"crit_multiplier"` // percentage (default 200 = 2x)

	Skill1Used bool `json:"-"`
	Skill2Used bool `json:"-"`

	// Temporary buffs
	DEFBuff   int  `json:"-"` // remaining rounds of DEF×2
	DodgeNext bool `json:"-"` // dodge the next attack
}

// RoundLog records what happened in one round.
type RoundLog struct {
	Round   int    `json:"round"`
	Actor   string `json:"actor"` // fighter name
	ActorID string `json:"actor_id"`
	Action  string `json:"action"` // attack / skill_1 / skill_2
	Detail  string `json:"detail"`
	Damage  int    `json:"damage"`
	Crit    bool   `json:"crit"`
	Counter bool   `json:"counter"` // path advantage
}

// BattleResult is the output of ExecuteBattle.
type BattleResult struct {
	WinnerID string     `json:"winner_id"`
	Rounds   int        `json:"rounds"`
	Log      []RoundLog `json:"log"`
	FinalHPA int        `json:"final_hp_a"`
	FinalHPB int        `json:"final_hp_b"`
}

// NewFighter creates a Fighter with base stats + path bonus + equipment bonus.
func NewFighter(id, name, path string, lv, baseHP, baseATK, baseDEF, baseSPD int,
	equipHP, equipATK, equipDEF, equipSPD, equipCrit int) *Fighter {

	hp := baseHP + equipHP
	atk := baseATK + equipATK
	def := baseDEF + equipDEF
	spd := baseSPD + equipSPD

	// Path bonus
	switch path {
	case "abyss": // 🌊 tank: HP+20%, DEF+15%
		hp = hp * 120 / 100
		def = def * 115 / 100
	case "terrain": // 🏔️ dps: ATK+20%, SPD+10%
		atk = atk * 120 / 100
		spd = spd * 110 / 100
	case "sky": // 🌪️ speed: SPD+25%, ATK+10%
		spd = spd * 125 / 100
		atk = atk * 110 / 100
	}

	critRate := 10 + equipCrit
	if critRate > 80 {
		critRate = 80
	}

	return &Fighter{
		ID:             id,
		Name:           name,
		Path:           path,
		Lv:             lv,
		MaxHP:          hp,
		HP:             hp,
		ATK:            atk,
		DEF:            def,
		SPD:            spd,
		CritRate:       critRate,
		CritMultiplier: 200,
	}
}

// SeasonBuff holds environment buffs from the active season.
type SeasonBuff struct {
	Environment string // abyss / terrain / sky
	ATKBonus    int    // percentage, e.g. 15 = +15%
	DEFBonus    int
	SPDBonus    int
}

// ApplySeasonBuff applies season environment buffs to a fighter whose path matches.
func (f *Fighter) ApplySeasonBuff(sb *SeasonBuff) {
	if sb == nil || sb.Environment == "" {
		return
	}
	if f.Path != sb.Environment {
		return
	}
	f.ATK = f.ATK * (100 + sb.ATKBonus) / 100
	f.DEF = f.DEF * (100 + sb.DEFBonus) / 100
	f.SPD = f.SPD * (100 + sb.SPDBonus) / 100
}

// ExecuteBattle runs a turn-based PK battle. Returns the result.
func ExecuteBattle(a, b *Fighter) BattleResult {
	var logs []RoundLog
	round := 0

	for a.HP > 0 && b.HP > 0 && round < 20 {
		round++

		// Determine turn order by SPD
		first, second := a, b
		if b.SPD > a.SPD || (b.SPD == a.SPD && rand.Intn(2) == 0) {
			first, second = b, a
		}

		// First attacks
		rlog := executeAttack(round, first, second)
		logs = append(logs, rlog)
		if second.HP <= 0 {
			break
		}

		// Second attacks
		rlog = executeAttack(round, second, first)
		logs = append(logs, rlog)
	}

	result := BattleResult{
		Rounds:   round,
		Log:      logs,
		FinalHPA: max(a.HP, 0),
		FinalHPB: max(b.HP, 0),
	}

	if a.HP > 0 && b.HP <= 0 {
		result.WinnerID = a.ID
	} else if b.HP > 0 && a.HP <= 0 {
		result.WinnerID = b.ID
	} else if a.HP > b.HP {
		result.WinnerID = a.ID
	} else if b.HP > a.HP {
		result.WinnerID = b.ID
	}
	// else draw: WinnerID stays empty

	return result
}

func executeAttack(round int, attacker, defender *Fighter) RoundLog {
	rlog := RoundLog{
		Round:   round,
		Actor:   attacker.Name,
		ActorID: attacker.ID,
		Action:  "attack",
	}

	// Check dodge
	if defender.DodgeNext {
		defender.DodgeNext = false
		rlog.Detail = fmt.Sprintf("%s 闪避了攻击！", defender.Name)
		rlog.Damage = 0
		return rlog
	}

	// Try to use skill (AI logic: use skills when available at good timing)
	if tryUseSkill(round, attacker, defender, &rlog) {
		return rlog
	}

	// Normal attack
	dmg := calcDamage(attacker, defender, &rlog)
	defender.HP -= dmg
	rlog.Damage = dmg

	detail := fmt.Sprintf("%s 攻击 %s → -%d HP", attacker.Name, defender.Name, dmg)
	if rlog.Crit {
		detail += " 暴击！"
	}
	if rlog.Counter {
		detail += " (克制+25%)"
	}
	rlog.Detail = detail

	// Tick DEF buff
	if defender.DEFBuff > 0 {
		defender.DEFBuff--
	}

	return rlog
}

func calcDamage(atk, def *Fighter, rlog *RoundLog) int {
	effectiveDEF := def.DEF
	if def.DEFBuff > 0 {
		effectiveDEF *= 2
	}

	baseDmg := max(1, atk.ATK-effectiveDEF/2)

	// Counter bonus (+25%)
	if isCounter(atk.Path, def.Path) {
		baseDmg = baseDmg * 125 / 100
		rlog.Counter = true
	}

	// Crit check
	if rand.Intn(100) < atk.CritRate {
		baseDmg = baseDmg * atk.CritMultiplier / 100
		rlog.Crit = true
	}

	// Random ±10%
	baseDmg = baseDmg * (90 + rand.Intn(21)) / 100

	if baseDmg < 1 {
		baseDmg = 1
	}
	return baseDmg
}

// isCounter returns true if attacker's path counters defender's path.
// 🌪️穹 → 🏔️陆, 🏔️陆 → 🌊渊, 🌊渊 → 🌪️穹
func isCounter(atkPath, defPath string) bool {
	switch atkPath {
	case "sky":
		return defPath == "terrain"
	case "terrain":
		return defPath == "abyss"
	case "abyss":
		return defPath == "sky"
	}
	return false
}

// tryUseSkill attempts to use a skill. Returns true if skill was used.
func tryUseSkill(round int, attacker, defender *Fighter, rlog *RoundLog) bool {
	// Skill 1 unlocks at Lv 5, Skill 2 at Lv 10
	// Use skill 1 when HP < 60% (defensive) or round >= 3
	// Use skill 2 when HP < 40% or opponent HP < 40%

	hpRatio := float64(attacker.HP) / float64(attacker.MaxHP)
	defHPRatio := float64(defender.HP) / float64(defender.MaxHP)

	// Skill 2 (Lv 10+, once per battle)
	if attacker.Lv >= 10 && !attacker.Skill2Used && (hpRatio < 0.4 || defHPRatio < 0.4 || round >= 6) {
		attacker.Skill2Used = true
		return applySkill2(attacker, defender, rlog)
	}

	// Skill 1 (Lv 5+, once per battle)
	if attacker.Lv >= 5 && !attacker.Skill1Used && (hpRatio < 0.6 || round >= 3) {
		attacker.Skill1Used = true
		return applySkill1(attacker, defender, rlog)
	}

	return false
}

func applySkill1(attacker, defender *Fighter, rlog *RoundLog) bool {
	rlog.Action = "skill_1"
	switch attacker.Path {
	case "abyss": // 深渊吞噬: 回复 30% 最大 HP
		heal := attacker.MaxHP * 30 / 100
		attacker.HP = min(attacker.HP+heal, attacker.MaxHP)
		rlog.Detail = fmt.Sprintf("%s 使用【深渊吞噬】回复 %d HP！", attacker.Name, heal)
	case "terrain": // 裂地猛击: 无视防御攻击
		dmg := max(1, attacker.ATK) * (90 + rand.Intn(21)) / 100
		if isCounter(attacker.Path, defender.Path) {
			dmg = dmg * 125 / 100
			rlog.Counter = true
		}
		defender.HP -= dmg
		rlog.Damage = dmg
		rlog.Detail = fmt.Sprintf("%s 使用【裂地猛击】无视防御 → -%d HP！", attacker.Name, dmg)
	case "sky": // 天穹突袭: 先手 + 伤害 ×1.5
		dmg := calcDamage(attacker, defender, rlog)
		dmg = dmg * 150 / 100
		defender.HP -= dmg
		rlog.Damage = dmg
		rlog.Detail = fmt.Sprintf("%s 使用【天穹突袭】伤害×1.5 → -%d HP！", attacker.Name, dmg)
	default: // larva — basic power attack
		dmg := calcDamage(attacker, defender, rlog)
		defender.HP -= dmg
		rlog.Damage = dmg
		rlog.Detail = fmt.Sprintf("%s 使用【蓄力攻击】→ -%d HP", attacker.Name, dmg)
	}
	return true
}

func applySkill2(attacker, defender *Fighter, rlog *RoundLog) bool {
	rlog.Action = "skill_2"
	switch attacker.Path {
	case "abyss": // 知识壁垒: DEF×2 持续 2 回合
		attacker.DEFBuff = 2
		rlog.Detail = fmt.Sprintf("%s 使用【知识壁垒】DEF翻倍 2回合！", attacker.Name)
	case "terrain": // 狂暴冲锋: ATK×2 但自伤 10% HP
		selfDmg := attacker.MaxHP * 10 / 100
		attacker.HP -= selfDmg
		dmg := attacker.ATK * 2 * (90 + rand.Intn(21)) / 100
		if isCounter(attacker.Path, defender.Path) {
			dmg = dmg * 125 / 100
			rlog.Counter = true
		}
		defender.HP -= dmg
		rlog.Damage = dmg
		rlog.Detail = fmt.Sprintf("%s 使用【狂暴冲锋】ATK×2 → -%d HP（自伤 %d）！", attacker.Name, dmg, selfDmg)
	case "sky": // 羽翼庇护: 闪避下一次攻击
		attacker.DodgeNext = true
		rlog.Detail = fmt.Sprintf("%s 使用【羽翼庇护】完全闪避下一次攻击！", attacker.Name)
	default:
		dmg := calcDamage(attacker, defender, rlog)
		defender.HP -= dmg
		rlog.Damage = dmg
		rlog.Detail = fmt.Sprintf("%s 使用【猛击】→ -%d HP", attacker.Name, dmg)
	}
	return true
}

// CalcELOChange computes ELO rating changes for winner and loser.
func CalcELOChange(winnerELO, loserELO int) (int, int) {
	K := 32.0
	expectedWin := 1.0 / (1.0 + math.Pow(10.0, float64(loserELO-winnerELO)/400.0))
	change := int(math.Round(K * (1.0 - expectedWin)))
	if change < 1 {
		change = 1
	}
	return change, -change
}

// SerializeLog converts battle log to JSON string.
func SerializeLog(logs []RoundLog) string {
	data, _ := json.Marshal(logs)
	return string(data)
}
