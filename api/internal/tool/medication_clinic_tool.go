package tool

import (
	"fmt"
	"sort"
	"strings"

	"gorm.io/gorm"
)

type MedicationClinicTool struct {
	db *gorm.DB
}

type medicationProfile struct {
	Key               string
	DisplayName       string
	ScheduleType      string
	CommonSideEffects []string
	UrgentSymptoms    []string
	MonitoringItems   []string
	GeneralAdvice     []string
}

func NewMedicationClinicTool(db *gorm.DB) *MedicationClinicTool {
	return &MedicationClinicTool{db: db}
}

func (t *MedicationClinicTool) Execute(userID string, params map[string]interface{}) (string, error) {
	action, _ := params["action"].(string)
	switch action {
	case "safety_review":
		return t.safetyReview(params)
	case "interaction_check":
		return t.interactionCheck(params)
	case "adherence_review":
		return t.adherenceReview(params)
	case "side_effect_triage":
		return t.sideEffectTriage(params)
	case "monitoring_plan":
		return t.monitoringPlan(params)
	default:
		return "", fmt.Errorf("unknown medication_clinic action: %s", action)
	}
}

func (t *MedicationClinicTool) safetyReview(params map[string]interface{}) (string, error) {
	profile, err := medicationProfileFromParams(params)
	if err != nil {
		return "", err
	}

	concurrentMeds := parseMedicationList(params["concurrent_meds"])
	symptoms := parseMedicationList(params["symptoms"])
	riskFactors := parseMedicationList(params["risk_factors"])
	doseIU := toFloat(params["dose_iu"])
	fastingGlucose := toFloat(params["fasting_glucose"])
	serumCalcium := toFloat(params["serum_calcium"])

	findings := []map[string]interface{}{}
	overallLevel := "normal"

	interactionAlerts, interactionLevel := buildMedicationInteractions(profile, concurrentMeds)
	for _, alert := range interactionAlerts {
		findings = append(findings, map[string]interface{}{"category": "interaction", "level": interactionLevel, "title": alert, "description": "建议核对服药时间、剂量和联合用药方案"})
	}
	if len(interactionAlerts) > 0 {
		overallLevel = medicationMaxLevel(overallLevel, interactionLevel)
	}

	for _, symptom := range symptoms {
		normalized := normalizeMedicationTerm(symptom)
		if containsNormalized(profile.UrgentSymptoms, normalized) {
			findings = append(findings, map[string]interface{}{"category": "urgent_symptom", "level": "severe", "title": fmt.Sprintf("出现需尽快评估的不良反应信号：%s", symptom), "description": "建议尽快联系医生或线下就医评估"})
			overallLevel = medicationMaxLevel(overallLevel, "severe")
		} else if containsNormalized(profile.CommonSideEffects, normalized) {
			findings = append(findings, map[string]interface{}{"category": "common_side_effect", "level": "mild", "title": fmt.Sprintf("常见不良反应：%s", symptom), "description": "建议记录发生时间、严重程度和持续时间，必要时复诊复核"})
			overallLevel = medicationMaxLevel(overallLevel, "mild")
		}
	}

	switch profile.Key {
	case "growth_hormone":
		if fastingGlucose >= 7 {
			findings = append(findings, map[string]interface{}{"category": "glucose", "level": "severe", "title": fmt.Sprintf("空腹血糖升高 %.2f mmol/L", fastingGlucose), "description": "使用生长激素时需尽快复核糖代谢并由医生评估"})
			overallLevel = medicationMaxLevel(overallLevel, "severe")
		} else if fastingGlucose >= 5.6 {
			findings = append(findings, map[string]interface{}{"category": "glucose", "level": "moderate", "title": fmt.Sprintf("空腹血糖受损 %.2f mmol/L", fastingGlucose), "description": "建议随访糖代谢并评估与治疗方案的关系"})
			overallLevel = medicationMaxLevel(overallLevel, "moderate")
		}
	case "levothyroxine":
		if containsAnyNormalized(riskFactors, "心悸", "palpitations", "手抖", "tremor", "失眠", "insomnia") {
			findings = append(findings, map[string]interface{}{"category": "over_replacement", "level": "moderate", "title": "存在甲状腺素过量提示症状", "description": "建议结合 TSH、FT4 与临床表现尽快复核"})
			overallLevel = medicationMaxLevel(overallLevel, "moderate")
		}
	case "iron":
		if containsAnyNormalized(riskFactors, "剧烈腹痛", "severeabdominalpain", "持续呕吐", "persistentvomiting") {
			findings = append(findings, map[string]interface{}{"category": "gastrointestinal", "level": "moderate", "title": "铁剂相关胃肠道不适需复核", "description": "建议评估剂量、服用方式及是否需要线下复诊"})
			overallLevel = medicationMaxLevel(overallLevel, "moderate")
		}
	case "vitamin_d":
		if doseIU > 4000 {
			findings = append(findings, map[string]interface{}{"category": "dose", "level": "moderate", "title": fmt.Sprintf("维生素D 日剂量偏高 %.0f IU", doseIU), "description": "需核对处方目的、疗程及复查计划"})
			overallLevel = medicationMaxLevel(overallLevel, "moderate")
		}
		if serumCalcium >= 2.75 {
			findings = append(findings, map[string]interface{}{"category": "hypercalcemia", "level": "severe", "title": fmt.Sprintf("血钙偏高 %.2f mmol/L", serumCalcium), "description": "需尽快复核是否存在补充过量或高钙血症风险"})
			overallLevel = medicationMaxLevel(overallLevel, "severe")
		}
	case "metformin":
		if containsAnyNormalized(riskFactors, "脱水", "dehydration", "呼吸困难", "dyspnea", "持续呕吐", "persistentvomiting") {
			findings = append(findings, map[string]interface{}{"category": "red_flag", "level": "severe", "title": "二甲双胍使用期间存在红旗信号", "description": "建议尽快线下评估并复核代谢状态"})
			overallLevel = medicationMaxLevel(overallLevel, "severe")
		}
	case "gnrha":
		if containsAnyNormalized(riskFactors, "漏打", "missedinjection", "持续出血", "persistentbleeding") {
			findings = append(findings, map[string]interface{}{"category": "schedule", "level": "moderate", "title": "GnRHa 注射节律或异常反应需复核", "description": "建议联系专科医生确认后续注射时间和疗效评估"})
			overallLevel = medicationMaxLevel(overallLevel, "moderate")
		}
	}

	classification := "当前用药未见明显安全警示"
	if overallLevel != "normal" {
		classification = "提示需要进行用药安全复核"
	}

	return jsonStr(map[string]interface{}{
		"medication":        profile.DisplayName,
		"medication_key":    profile.Key,
		"overall_level":     overallLevel,
		"classification":    classification,
		"findings":          findings,
		"general_advice":    profile.GeneralAdvice,
		"monitoring_items":  profile.MonitoringItems,
		"concurrent_meds":   concurrentMeds,
		"reported_symptoms": symptoms,
		"risk_factors":      riskFactors,
	}), nil
}

func (t *MedicationClinicTool) interactionCheck(params map[string]interface{}) (string, error) {
	profile, err := medicationProfileFromParams(params)
	if err != nil {
		return "", err
	}
	concurrentMeds := parseMedicationList(params["concurrent_meds"])
	alerts, level := buildMedicationInteractions(profile, concurrentMeds)
	classification := "未识别到明显相互作用"
	if len(alerts) > 0 {
		classification = "存在需提醒的联合用药注意事项"
	}
	return jsonStr(map[string]interface{}{
		"medication":         profile.DisplayName,
		"concurrent_meds":    concurrentMeds,
		"overall_level":      level,
		"classification":     classification,
		"interaction_alerts": alerts,
		"separation_advice":  buildSeparationAdvice(profile.Key, concurrentMeds),
	}), nil
}

func (t *MedicationClinicTool) adherenceReview(params map[string]interface{}) (string, error) {
	profile, err := medicationProfileFromParams(params)
	if err != nil {
		return "", err
	}
	missedDoses := int(toFloat(params["missed_doses"]))
	missedDays := int(toFloat(params["missed_days"]))
	lateDays := int(toFloat(params["late_days"]))
	overallLevel := "normal"
	status := "依从性良好"
	nextStep := "维持当前用药节律并按计划复诊。"

	switch profile.ScheduleType {
	case "daily":
		if missedDays >= 3 || missedDoses >= 5 {
			overallLevel = "moderate"
			status = "依从性下降"
			nextStep = "建议复盘漏服原因，必要时简化提醒方式并与医生确认是否需要调整方案。"
		} else if missedDays >= 1 || missedDoses >= 2 {
			overallLevel = "mild"
			status = "存在间断漏服"
			nextStep = "建议加强日常提醒并记录漏服时点。"
		}
	case "monthly_injection", "quarterly_injection":
		if lateDays > 28 {
			overallLevel = "severe"
			status = "注射超期明显"
			nextStep = "建议尽快联系专科门诊确认下一次注射与疗效评估安排。"
		} else if lateDays > 7 {
			overallLevel = "moderate"
			status = "注射节律延迟"
			nextStep = "建议尽快补预约并核对后续注射周期。"
		}
	}

	return jsonStr(map[string]interface{}{
		"medication":    profile.DisplayName,
		"schedule_type": profile.ScheduleType,
		"missed_doses":  missedDoses,
		"missed_days":   missedDays,
		"late_days":     lateDays,
		"overall_level": overallLevel,
		"status":        status,
		"next_step":     nextStep,
	}), nil
}

func (t *MedicationClinicTool) sideEffectTriage(params map[string]interface{}) (string, error) {
	profile, err := medicationProfileFromParams(params)
	if err != nil {
		return "", err
	}
	symptoms := parseMedicationList(params["symptoms"])
	urgent := []string{}
	common := []string{}
	other := []string{}
	level := "normal"

	for _, symptom := range symptoms {
		normalized := normalizeMedicationTerm(symptom)
		switch {
		case containsNormalized(profile.UrgentSymptoms, normalized):
			urgent = append(urgent, symptom)
			level = medicationMaxLevel(level, "severe")
		case containsNormalized(profile.CommonSideEffects, normalized):
			common = append(common, symptom)
			level = medicationMaxLevel(level, "mild")
		default:
			other = append(other, symptom)
			level = medicationMaxLevel(level, "mild")
		}
	}

	triage := "未见明显药物不良反应信号"
	if len(urgent) > 0 {
		triage = "存在需要立即评估的不良反应信号"
	} else if len(common) > 0 || len(other) > 0 {
		triage = "建议结合症状持续时间和严重程度继续观察或复诊"
	}

	return jsonStr(map[string]interface{}{
		"medication":      profile.DisplayName,
		"overall_level":   level,
		"triage":          triage,
		"urgent_symptoms": urgent,
		"common_symptoms": common,
		"other_symptoms":  other,
	}), nil
}

func (t *MedicationClinicTool) monitoringPlan(params map[string]interface{}) (string, error) {
	profile, err := medicationProfileFromParams(params)
	if err != nil {
		return "", err
	}
	return jsonStr(map[string]interface{}{
		"medication":       profile.DisplayName,
		"schedule_type":    profile.ScheduleType,
		"monitoring_items": profile.MonitoringItems,
		"general_advice":   profile.GeneralAdvice,
		"followup_window":  medicationFollowupWindow(profile.Key),
	}), nil
}

func medicationProfileFromParams(params map[string]interface{}) (medicationProfile, error) {
	name, _ := params["medication_name"].(string)
	name = strings.TrimSpace(name)
	if name == "" {
		return medicationProfile{}, fmt.Errorf("medication_name is required")
	}
	profile, ok := lookupMedicationProfile(name)
	if !ok {
		return medicationProfile{
			Key:             "generic",
			DisplayName:     name,
			ScheduleType:    "daily",
			MonitoringItems: []string{"复核适应证", "核对剂量和疗程", "关注不适症状变化"},
			GeneralAdvice:   []string{"未知药物仅可做一般性提醒", "需结合正式处方和药品说明书核对"},
		}, nil
	}
	return profile, nil
}

func lookupMedicationProfile(name string) (medicationProfile, bool) {
	normalized := normalizeMedicationTerm(name)
	catalog := []struct {
		aliases []string
		profile medicationProfile
	}{
		{[]string{"生长激素", "重组人生长激素", "somatropin", "gh"}, medicationProfile{Key: "growth_hormone", DisplayName: "生长激素", ScheduleType: "daily", CommonSideEffects: []string{"水肿", "注射部位疼痛", "轻度头痛", "关节痛", "jointpain"}, UrgentSymptoms: []string{"严重头痛", "视物模糊", "髋痛", "跛行", "呼吸困难", "呼吸急促"}, MonitoringItems: []string{"身高增长速率", "BMI 与体重变化", "糖代谢指标", "骨龄或青春期进展"}, GeneralAdvice: []string{"规律注射并记录时间", "如出现严重头痛、视物异常或髋膝疼痛需尽快复诊"}}},
		{[]string{"左甲状腺素", "优甲乐", "levothyroxine", "lt4"}, medicationProfile{Key: "levothyroxine", DisplayName: "左甲状腺素", ScheduleType: "daily", CommonSideEffects: []string{"心悸", "失眠", "手抖", "palpitations", "insomnia"}, UrgentSymptoms: []string{"胸痛", "明显心动过速", "呼吸困难"}, MonitoringItems: []string{"TSH", "FT4", "心率与体重变化", "症状变化"}, GeneralAdvice: []string{"通常建议空腹、固定时段服用", "与铁剂、钙剂尽量错开服用"}}},
		{[]string{"铁剂", "补铁", "ferrous", "iron", "硫酸亚铁", "右旋糖酐铁"}, medicationProfile{Key: "iron", DisplayName: "铁剂", ScheduleType: "daily", CommonSideEffects: []string{"恶心", "腹痛", "便秘", "黑便", "nausea", "constipation"}, UrgentSymptoms: []string{"持续呕吐", "剧烈腹痛", "便血"}, MonitoringItems: []string{"血红蛋白", "铁蛋白", "胃肠道耐受情况", "依从性"}, GeneralAdvice: []string{"可记录胃肠道反应以便复诊时调整", "与钙剂或甲状腺素建议错峰服用"}}},
		{[]string{"维生素d", "vitamind", "骨化三醇", "vd"}, medicationProfile{Key: "vitamin_d", DisplayName: "维生素D/骨代谢补充", ScheduleType: "daily", CommonSideEffects: []string{"便秘", "轻度腹胀", "constipation"}, UrgentSymptoms: []string{"明显口渴", "频繁呕吐", "精神差", "多尿"}, MonitoringItems: []string{"25-OH 维生素D", "血钙/血磷", "骨代谢相关症状", "补充剂量与疗程"}, GeneralAdvice: []string{"避免重复补充多个高剂量维生素D制剂", "长期高剂量补充建议配合复查"}}},
		{[]string{"二甲双胍", "metformin", "格华止"}, medicationProfile{Key: "metformin", DisplayName: "二甲双胍", ScheduleType: "daily", CommonSideEffects: []string{"腹泻", "恶心", "食欲下降", "diarrhea", "nausea"}, UrgentSymptoms: []string{"呼吸困难", "明显乏力", "持续呕吐", "脱水"}, MonitoringItems: []string{"空腹血糖或 HbA1c", "体重/BMI 变化", "胃肠道耐受性", "脱水风险"}, GeneralAdvice: []string{"胃肠道反应常见，需观察是否持续", "脱水、持续呕吐时建议尽快线下评估"}}},
		{[]string{"gnrha", "亮丙瑞林", "曲普瑞林", "戈舍瑞林", "促性腺激素释放激素激动剂"}, medicationProfile{Key: "gnrha", DisplayName: "GnRHa 抑制治疗", ScheduleType: "monthly_injection", CommonSideEffects: []string{"注射部位疼痛", "情绪波动", "moodchange"}, UrgentSymptoms: []string{"持续出血", "严重过敏", "呼吸困难"}, MonitoringItems: []string{"注射节律", "骨龄与青春期进展", "身高速度", "不良反应"}, GeneralAdvice: []string{"需尽量保持注射周期稳定", "如出现持续出血或明显异常反应需尽快复诊"}}},
	}

	for _, item := range catalog {
		for _, alias := range item.aliases {
			if normalizeMedicationTerm(alias) == normalized {
				return item.profile, true
			}
		}
	}
	return medicationProfile{}, false
}

func buildMedicationInteractions(profile medicationProfile, concurrentMeds []string) ([]string, string) {
	alerts := []string{}
	level := "normal"
	for _, med := range concurrentMeds {
		normalized := normalizeMedicationTerm(med)
		switch profile.Key {
		case "levothyroxine":
			if containsAnyNormalized([]string{normalized}, "iron", "铁剂", "ferrous", "calcium", "钙剂") {
				alerts = append(alerts, fmt.Sprintf("%s 与 %s 同服可能影响吸收", profile.DisplayName, med))
				level = medicationMaxLevel(level, "moderate")
			}
		case "iron":
			if containsAnyNormalized([]string{normalized}, "levothyroxine", "左甲状腺素", "优甲乐", "calcium", "钙剂") {
				alerts = append(alerts, fmt.Sprintf("铁剂与 %s 建议错峰服用", med))
				level = medicationMaxLevel(level, "moderate")
			}
		case "vitamin_d":
			if containsAnyNormalized([]string{normalized}, "calcium", "钙剂") {
				alerts = append(alerts, "维生素D 与钙剂联用时需注意补充总量和高钙风险")
				level = medicationMaxLevel(level, "mild")
			}
		case "growth_hormone":
			if containsAnyNormalized([]string{normalized}, "metformin", "二甲双胍") {
				alerts = append(alerts, "生长激素与二甲双胍联用时建议同步关注糖代谢变化")
				level = medicationMaxLevel(level, "mild")
			}
		case "metformin":
			if containsAnyNormalized([]string{normalized}, "growthhormone", "gh", "生长激素") {
				alerts = append(alerts, "二甲双胍与生长激素联用时建议关注血糖和体重变化")
				level = medicationMaxLevel(level, "mild")
			}
		}
	}
	return uniqueMedicationStrings(alerts), level
}

func buildSeparationAdvice(medicationKey string, concurrentMeds []string) []string {
	advice := []string{}
	for _, med := range concurrentMeds {
		normalized := normalizeMedicationTerm(med)
		switch medicationKey {
		case "levothyroxine":
			if containsAnyNormalized([]string{normalized}, "iron", "铁剂", "ferrous", "calcium", "钙剂") {
				advice = append(advice, fmt.Sprintf("左甲状腺素与 %s 建议间隔至少 4 小时", med))
			}
		case "iron":
			if containsAnyNormalized([]string{normalized}, "calcium", "钙剂") {
				advice = append(advice, "铁剂与钙剂可考虑间隔 2 小时以上")
			}
			if containsAnyNormalized([]string{normalized}, "levothyroxine", "左甲状腺素", "优甲乐") {
				advice = append(advice, "铁剂与左甲状腺素建议间隔至少 4 小时")
			}
		}
	}
	return uniqueMedicationStrings(advice)
}

func medicationFollowupWindow(key string) string {
	switch key {
	case "levothyroxine":
		return "通常 6-8 周复核甲功一次，后续根据稳定性调整"
	case "iron":
		return "通常 4-8 周复核 CBC/铁蛋白与耐受情况"
	case "vitamin_d":
		return "通常 8-12 周复核 25-OH 维生素D 或相关骨代谢指标"
	case "metformin":
		return "通常 4-12 周复核血糖、体重和胃肠道耐受性"
	case "growth_hormone":
		return "通常 1-3 个月复核生长速度、体重和相关实验室指标"
	case "gnrha":
		return "按注射周期随访，并定期复核骨龄与青春期进展"
	default:
		return "建议结合处方目的和医生要求安排复查时间"
	}
}

func parseMedicationList(v interface{}) []string {
	result := []string{}
	switch items := v.(type) {
	case []string:
		for _, item := range items {
			if trimmed := strings.TrimSpace(item); trimmed != "" {
				result = append(result, trimmed)
			}
		}
	case []interface{}:
		for _, item := range items {
			if trimmed := strings.TrimSpace(fmt.Sprintf("%v", item)); trimmed != "" {
				result = append(result, trimmed)
			}
		}
	case string:
		replacer := strings.NewReplacer("\n", ",", "；", ",", ";", ",", "、", ",", "，", ",")
		for _, part := range strings.Split(replacer.Replace(items), ",") {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				result = append(result, trimmed)
			}
		}
	}
	return uniqueMedicationStrings(result)
}

func normalizeMedicationTerm(v string) string {
	replacer := strings.NewReplacer(" ", "", "-", "", "_", "", "（", "(", "）", ")", "/", "", ".", "")
	return strings.ToLower(strings.TrimSpace(replacer.Replace(v)))
}

func containsNormalized(items []string, normalized string) bool {
	for _, item := range items {
		if normalizeMedicationTerm(item) == normalized {
			return true
		}
	}
	return false
}

func containsAnyNormalized(items []string, candidates ...string) bool {
	for _, item := range items {
		normalized := normalizeMedicationTerm(item)
		for _, candidate := range candidates {
			if normalized == normalizeMedicationTerm(candidate) {
				return true
			}
		}
	}
	return false
}

func medicationMaxLevel(current, next string) string {
	levels := map[string]int{"normal": 0, "mild": 1, "moderate": 2, "severe": 3}
	if levels[next] > levels[current] {
		return next
	}
	return current
}

func uniqueMedicationStrings(items []string) []string {
	lookup := map[string]string{}
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		key := normalizeMedicationTerm(trimmed)
		if _, ok := lookup[key]; !ok {
			lookup[key] = trimmed
		}
	}
	result := make([]string, 0, len(lookup))
	for _, value := range lookup {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
