package game

import (
	"github.com/yinhe/starclaw/internal/model"
)

// RealmInfo describes a realm tier with its bonuses and skills.
type RealmInfo struct {
	Level    int      `json:"level"`
	Name     string   `json:"name"`
	NameEN   string   `json:"name_en"`
	Aura     string   `json:"aura"` // gold, crimson, emerald
	BonusDEF int      `json:"bonus_def"`
	BonusHP  int      `json:"bonus_hp"`
	BonusATK int      `json:"bonus_atk"`
	BonusSPD int      `json:"bonus_spd"`
	Skills   []string `json:"skills"`
}

// RealmTiers defines all tiers for each realm path.
var RealmTiers = map[model.RealmPath][]RealmInfo{
	model.RealmImmortal: {
		{Level: 2, Name: "仙徒", NameEN: "Immortal Disciple", Aura: "gold", BonusDEF: 8, BonusHP: 5, Skills: []string{"治愈之光"}},
		{Level: 3, Name: "仙人", NameEN: "Immortal", Aura: "gold", BonusDEF: 15, BonusHP: 10, Skills: []string{"治愈之光", "天罡护盾", "群体祝福"}},
		{Level: 4, Name: "神", NameEN: "God", Aura: "gold", BonusDEF: 20, BonusHP: 15, Skills: []string{"治愈之光", "天罡护盾", "群体祝福", "不死金身", "万物复苏"}},
	},
	model.RealmDemon: {
		{Level: 2, Name: "魔徒", NameEN: "Demon Disciple", Aura: "crimson", BonusATK: 8, BonusSPD: 5, Skills: []string{"嗜血之爪"}},
		{Level: 3, Name: "魔将", NameEN: "Demon General", Aura: "crimson", BonusATK: 15, BonusSPD: 10, Skills: []string{"嗜血之爪", "恐惧光环", "灵魂吞噬"}},
		{Level: 4, Name: "魔神", NameEN: "Demon God", Aura: "crimson", BonusATK: 20, BonusSPD: 15, Skills: []string{"嗜血之爪", "恐惧光环", "灵魂吞噬", "毁灭之息", "深渊吞噬"}},
	},
	model.RealmMonster: {
		{Level: 2, Name: "妖修", NameEN: "Monster Cultivator", Aura: "emerald", BonusSPD: 8, BonusATK: 5, Skills: []string{"幻影步"}},
		{Level: 3, Name: "妖王", NameEN: "Monster King", Aura: "emerald", BonusSPD: 15, BonusATK: 10, Skills: []string{"幻影步", "分身术", "化形"}},
		{Level: 4, Name: "妖皇", NameEN: "Monster Emperor", Aura: "emerald", BonusSPD: 20, BonusATK: 15, Skills: []string{"幻影步", "分身术", "化形", "万化归一", "天地同寿"}},
	},
}

// SaintTier is the unified tier at awakening 5 stars (transcends all paths).
var SaintTier = RealmInfo{
	Level: 5, Name: "圣", NameEN: "Saint", Aura: "prismatic",
	BonusDEF: 25, BonusHP: 25, BonusATK: 25, BonusSPD: 25,
	Skills: []string{"三道合一", "天地法则"},
}

// GetRealmInfo returns the current realm info for a growth profile.
func GetRealmInfo(growth model.NodeGrowth) *RealmInfo {
	if growth.RealmPath == model.RealmNone || growth.RealmLevel < 2 {
		return nil // mortal or human realm, no special info
	}
	if growth.RealmLevel >= 5 {
		return &SaintTier
	}
	tiers, ok := RealmTiers[growth.RealmPath]
	if !ok {
		return nil
	}
	for _, t := range tiers {
		if t.Level == growth.RealmLevel {
			return &t
		}
	}
	return nil
}

// GetRealmBattleBonus returns stat bonuses from realm for battle calculations.
func GetRealmBattleBonus(growth model.NodeGrowth) (hp, atk, def, spd int) {
	info := GetRealmInfo(growth)
	if info == nil {
		return 0, 0, 0, 0
	}
	return info.BonusHP, info.BonusATK, info.BonusDEF, info.BonusSPD
}

// GetRealmPressureBonus returns the stat bonus for realm level difference in PK.
// Higher realm gets +3% per level difference; lower realm gets +10% crit chance.
func GetRealmPressureBonus(attackerRealm, defenderRealm int) (attackerBonus, defenderBonus int) {
	diff := attackerRealm - defenderRealm
	if diff > 0 {
		return diff * 3, 0 // attacker has higher realm
	}
	if diff < 0 {
		return 0, -diff * 3 // defender has higher realm
	}
	return 0, 0
}

// GetFullIdentity returns the display name combining evolution + realm.
// e.g. "魔道·霸王龙" or "仙道·凤凰"
func GetFullIdentity(growth model.NodeGrowth) string {
	title, _ := model.GetTitle(growth.EvolutionPath, growth.Level)
	if growth.RealmPath == model.RealmNone {
		return title
	}
	realmName := ""
	info := GetRealmInfo(growth)
	if info != nil {
		realmName = info.Name
	} else {
		switch growth.RealmPath {
		case model.RealmImmortal:
			realmName = "仙"
		case model.RealmDemon:
			realmName = "魔"
		case model.RealmMonster:
			realmName = "妖"
		}
	}
	return realmName + "·" + title
}
