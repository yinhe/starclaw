// Package sdk provides the Chrysalis SDK — interfaces and types for integrating
// the pet evolution & PK system with external services (e.g. Claw growth engine).
//
// Claw imports this SDK to report growth stats to the Chrysalis service.
// Chrysalis imports this SDK for shared type definitions.
package sdk

// GrowthStats represents the aggregated growth statistics from a Claw node.
// Claw computes these locally and sends to Chrysalis for fighter sync.
type GrowthStats struct {
	ClawID        string `json:"claw_id"`
	Level         int    `json:"level"`
	EvolutionPath string `json:"evolution_path"` // abyss / terrain / sky / larva
	FormCode      string `json:"form_code"`      // e.g. "abyss_2" = 蛟
	Title         string `json:"title"`

	// Four combat dimensions (base values, before equipment)
	BaseHP  int `json:"base_hp"`
	BaseATK int `json:"base_atk"`
	BaseDEF int `json:"base_def"`
	BaseSPD int `json:"base_spd"`

	// Activity metrics (used for stat computation)
	TotalConversations int     `json:"total_conversations"`
	TotalMemories      int     `json:"total_memories"`
	TotalTasks         int     `json:"total_tasks"`
	SatisfactionRate   float64 `json:"satisfaction_rate"` // 0.0 - 1.0
}

// FighterRegistration is the payload sent to Chrysalis to register/sync a fighter.
type FighterRegistration struct {
	ClawID        string `json:"claw_id"`
	Name          string `json:"name"`
	Level         int    `json:"level"`
	EvolutionPath string `json:"evolution_path"`
	FormCode      string `json:"form_code"`
	BaseHP        int    `json:"base_hp"`
	BaseATK       int    `json:"base_atk"`
	BaseDEF       int    `json:"base_def"`
	BaseSPD       int    `json:"base_spd"`
}

// BattleDrop represents a material dropped after battle.
type BattleDrop struct {
	MaterialID string `json:"material_id"`
	Quantity   int    `json:"quantity"`
}

// SeasonInfo represents the current active season.
type SeasonInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Environment string `json:"environment"` // abyss / terrain / sky
	ATKBonus    int    `json:"atk_bonus"`
	DEFBonus    int    `json:"def_bonus"`
	SPDBonus    int    `json:"spd_bonus"`
	Active      bool   `json:"active"`
}

// MutationInfo represents a mutation trait.
type MutationInfo struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Desc   string `json:"desc"`
	Rarity string `json:"rarity"` // common / rare / legendary
	Effect string `json:"effect"` // JSON
}
