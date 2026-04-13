package game

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/yinhe/starclaw/internal/model"
)

// BattleResult represents the outcome of a swarm battle.
type BattleResult struct {
	Winner        string        `json:"winner"`         // "attacker" or "defender"
	AttackerScore int           `json:"attacker_score"` // rounds won
	DefenderScore int           `json:"defender_score"`
	Rounds        []BattleRound `json:"rounds"`
	EXPAbsorbed   int           `json:"exp_absorbed"` // EXP transferred to winner
	EXPLost       int           `json:"exp_lost"`     // EXP lost by loser
	StardustWon   int           `json:"stardust_won"`
}

// BattleRound represents one round of combat.
type BattleRound struct {
	Number     int          `json:"number"`
	Type       string       `json:"type"`   // "hero" or "swarm"
	Winner     string       `json:"winner"` // "attacker" or "defender"
	Log        []string     `json:"log"`
	Combatants []CombatUnit `json:"combatants,omitempty"`
}

// CombatUnit is a unit participating in battle.
type CombatUnit struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	HP     int    `json:"hp"`
	ATK    int    `json:"atk"`
	DEF    int    `json:"def"`
	SPD    int    `json:"spd"`
	Skill  string `json:"skill"`
	IsHero bool   `json:"is_hero"`
	Side   string `json:"side"` // "attacker" or "defender"
	Alive  bool   `json:"alive"`
}

// SwarmBattle executes a full 3-round battle between two players.
// Round 1: Hero vs Hero
// Round 2: Swarm vs Swarm (3v3)
// Round 3: Tiebreaker (hero rematch if needed)
func SwarmBattle(
	attackerHero model.NodeGrowth, attackerUnits []model.SwarmUnit,
	defenderHero model.NodeGrowth, defenderUnits []model.SwarmUnit,
	attackerStats, defenderStats map[string]int, // hp, atk, def, spd from growth API
) BattleResult {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	result := BattleResult{}

	// Round 1: Hero vs Hero
	r1 := heroRound(attackerStats, defenderStats, rng)
	r1.Number = 1
	result.Rounds = append(result.Rounds, r1)
	if r1.Winner == "attacker" {
		result.AttackerScore++
	} else {
		result.DefenderScore++
	}

	// Round 2: Swarm vs Swarm (up to 3 units each)
	r2 := swarmRound(attackerUnits, defenderUnits, rng)
	r2.Number = 2
	result.Rounds = append(result.Rounds, r2)
	if r2.Winner == "attacker" {
		result.AttackerScore++
	} else {
		result.DefenderScore++
	}

	// Round 3: Tiebreaker if needed
	if result.AttackerScore == result.DefenderScore {
		r3 := heroRound(attackerStats, defenderStats, rng)
		r3.Number = 3
		r3.Type = "tiebreaker"
		// Add randomness to tiebreaker
		result.Rounds = append(result.Rounds, r3)
		if r3.Winner == "attacker" {
			result.AttackerScore++
		} else {
			result.DefenderScore++
		}
	}

	// Determine winner
	if result.AttackerScore > result.DefenderScore {
		result.Winner = "attacker"
	} else {
		result.Winner = "defender"
	}

	return result
}

func heroRound(aStats, dStats map[string]int, rng *rand.Rand) BattleRound {
	round := BattleRound{Type: "hero", Log: []string{}}

	aHP := aStats["hp"] * 10 // scale up for battle
	dHP := dStats["hp"] * 10
	aATK := aStats["atk"]
	dATK := dStats["atk"]
	aDEF := aStats["def"]
	dDEF := dStats["def"]
	aSPD := aStats["spd"]
	dSPD := dStats["spd"]

	for turn := 0; turn < 20 && aHP > 0 && dHP > 0; turn++ {
		// Faster goes first
		if aSPD >= dSPD {
			dmg := maxInt(1, aATK-dDEF/2+rng.Intn(5))
			dHP -= dmg
			round.Log = append(round.Log, fmt.Sprintf("攻方龙虾英雄 攻击 → %d 伤害", dmg))
			if dHP <= 0 {
				break
			}
			dmg = maxInt(1, dATK-aDEF/2+rng.Intn(5))
			aHP -= dmg
			round.Log = append(round.Log, fmt.Sprintf("守方龙虾英雄 攻击 → %d 伤害", dmg))
		} else {
			dmg := maxInt(1, dATK-aDEF/2+rng.Intn(5))
			aHP -= dmg
			round.Log = append(round.Log, fmt.Sprintf("守方龙虾英雄 攻击 → %d 伤害", dmg))
			if aHP <= 0 {
				break
			}
			dmg = maxInt(1, aATK-dDEF/2+rng.Intn(5))
			dHP -= dmg
			round.Log = append(round.Log, fmt.Sprintf("攻方龙虾英雄 攻击 → %d 伤害", dmg))
		}
	}

	if aHP > dHP {
		round.Winner = "attacker"
	} else {
		round.Winner = "defender"
	}
	return round
}

func swarmRound(aUnits, dUnits []model.SwarmUnit, rng *rand.Rand) BattleRound {
	round := BattleRound{Type: "swarm", Log: []string{}}

	// Take up to 3 units per side
	aTeam := takeUnits(aUnits, 3)
	dTeam := takeUnits(dUnits, 3)

	if len(aTeam) == 0 && len(dTeam) == 0 {
		round.Winner = "attacker" // both empty = attacker wins by default
		round.Log = append(round.Log, "双方均无虫群出战")
		return round
	}
	if len(aTeam) == 0 {
		round.Winner = "defender"
		round.Log = append(round.Log, "攻方无虫群出战")
		return round
	}
	if len(dTeam) == 0 {
		round.Winner = "attacker"
		round.Log = append(round.Log, "守方无虫群出战")
		return round
	}

	// Simple combat: total stats comparison with skill effects
	aTotalATK, aTotalHP := 0, 0
	for _, u := range aTeam {
		aTotalATK += u.ATK + rng.Intn(5)
		aTotalHP += u.HP
		if u.Skill1 != "" {
			round.Log = append(round.Log, fmt.Sprintf("攻方 %s 使用 %s", u.AgentName, u.Skill1))
		}
	}

	dTotalATK, dTotalHP := 0, 0
	for _, u := range dTeam {
		dTotalATK += u.ATK + rng.Intn(5)
		dTotalHP += u.HP
		if u.Skill1 != "" {
			round.Log = append(round.Log, fmt.Sprintf("守方 %s 使用 %s", u.AgentName, u.Skill1))
		}
	}

	// Simulate 5 exchange rounds
	for i := 0; i < 5 && aTotalHP > 0 && dTotalHP > 0; i++ {
		dTotalHP -= maxInt(1, aTotalATK/3)
		aTotalHP -= maxInt(1, dTotalATK/3)
		round.Log = append(round.Log, fmt.Sprintf("交锋 %d: 攻方HP=%d 守方HP=%d", i+1, maxInt(0, aTotalHP), maxInt(0, dTotalHP)))
	}

	if aTotalHP > dTotalHP {
		round.Winner = "attacker"
		round.Log = append(round.Log, "攻方虫群获胜")
	} else {
		round.Winner = "defender"
		round.Log = append(round.Log, "守方虫群获胜")
	}

	return round
}

func takeUnits(units []model.SwarmUnit, n int) []model.SwarmUnit {
	if len(units) <= n {
		return units
	}
	// Take highest level units
	result := make([]model.SwarmUnit, n)
	copy(result, units[:n])
	return result
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
