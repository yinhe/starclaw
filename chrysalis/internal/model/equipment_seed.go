package model

// SeedEquipmentDefs returns the initial set of equipment definitions.
func SeedEquipmentDefs() []EquipmentDef {
	return []EquipmentDef{
		// ── 🗡️ Weapons ──
		{ID: "w-white-1", Name: "虫爪", Slot: "weapon", Quality: "white", BonusATK: 8, PriceStar: 50,
			SpecialDesc: "基础武器"},
		{ID: "w-green-1", Name: "腐蚀刺针", Slot: "weapon", Quality: "green", BonusATK: 20, PriceStar: 200,
			SpecialCode: "armor_pierce_5", SpecialDesc: "5%概率穿甲（无视DEF）"},
		{ID: "w-blue-abyss", Name: "深渊三叉戟", Slot: "weapon", Quality: "blue", PathOnly: "abyss",
			BonusATK: 40, CritRateBonus: 10, PriceStar: 500,
			SpecialCode: "abyss_crit", SpecialDesc: "🌊渊专属：暴击+10%"},
		{ID: "w-blue-terrain", Name: "裂地巨锤", Slot: "weapon", Quality: "blue", PathOnly: "terrain",
			BonusATK: 35, BonusHP: 50, PriceStar: 500,
			SpecialCode: "terrain_lifesteal", SpecialDesc: "🏔️陆专属：每次攻击回复5HP"},
		{ID: "w-blue-sky", Name: "穹风之翼", Slot: "weapon", Quality: "blue", PathOnly: "sky",
			BonusATK: 30, BonusSPD: 20, PriceStar: 500,
			SpecialCode: "sky_double_strike", SpecialDesc: "🌪️穹专属：30%概率二连击"},
		{ID: "w-purple-1", Name: "毁灭之爪", Slot: "weapon", Quality: "purple",
			BonusATK: 80, PriceDust: 2000,
			SpecialCode: "crit_dmg_250", SpecialDesc: "暴击伤害 ×2.5（默认×2）"},

		// ── 🛡️ Armor ──
		{ID: "a-white-1", Name: "硬壳外骨骼", Slot: "armor", Quality: "white",
			BonusDEF: 6, BonusHP: 30, PriceStar: 50, SpecialDesc: "基础护甲"},
		{ID: "a-green-1", Name: "菌毯披风", Slot: "armor", Quality: "green",
			BonusDEF: 15, BonusHP: 80, PriceStar: 200,
			SpecialCode: "regen_3pct", SpecialDesc: "每回合回复 3% HP"},
		{ID: "a-blue-abyss", Name: "渊鳞甲", Slot: "armor", Quality: "blue", PathOnly: "abyss",
			BonusDEF: 40, BonusHP: 150, PriceStar: 500,
			SpecialCode: "abyss_crit_resist", SpecialDesc: "🌊渊专属：受到暴击时减伤50%"},
		{ID: "a-blue-terrain", Name: "大地之铠", Slot: "armor", Quality: "blue", PathOnly: "terrain",
			BonusDEF: 50, BonusHP: 100, PriceStar: 500,
			SpecialCode: "terrain_last_stand", SpecialDesc: "🏔️陆专属：HP<30%时DEF翻倍"},
		{ID: "a-blue-sky", Name: "天蛾薄翼", Slot: "armor", Quality: "blue", PathOnly: "sky",
			BonusDEF: 20, BonusSPD: 30, PriceStar: 500,
			SpecialCode: "sky_dodge_20", SpecialDesc: "🌪️穹专属：20%概率闪避攻击"},
		{ID: "a-purple-1", Name: "利维坦之鳞", Slot: "armor", Quality: "purple",
			BonusDEF: 120, BonusHP: 500, PriceDust: 2000,
			SpecialCode: "cheat_death", SpecialDesc: "免疫一次致死伤害（每场一次）"},

		// ── 💍 Trinkets ──
		{ID: "t-white-1", Name: "突触加速器", Slot: "trinket", Quality: "white",
			BonusSPD: 8, PriceStar: 50, SpecialDesc: "基础饰品"},
		{ID: "t-green-1", Name: "利爪勋章", Slot: "trinket", Quality: "green",
			CritRateBonus: 8, PriceStar: 200, SpecialDesc: "暴击率+8%"},
		{ID: "t-purple-1", Name: "虫后祝福", Slot: "trinket", Quality: "purple",
			BonusHP: 30, BonusATK: 30, BonusDEF: 30, BonusSPD: 30, PriceDust: 2000,
			SpecialCode: "queen_blessing", SpecialDesc: "全属性+30，每3回合释放路线技能"},
	}
}
