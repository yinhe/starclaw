package tool

import (
	"fmt"
	"sort"
	"strings"

	"gorm.io/gorm"
)

type AllergyClinicTool struct {
	db *gorm.DB
}

func NewAllergyClinicTool(db *gorm.DB) *AllergyClinicTool {
	return &AllergyClinicTool{db: db}
}

func (t *AllergyClinicTool) Execute(userID string, params map[string]interface{}) (string, error) {
	action, _ := params["action"].(string)
	switch action {
	case "rash_triage":
		return t.rashTriage(params)
	case "allergic_reaction_review":
		return t.allergicReactionReview(params)
	case "anaphylaxis_review":
		return t.anaphylaxisReview(params)
	case "vaccine_reaction_review":
		return t.vaccineReactionReview(params)
	default:
		return "", fmt.Errorf("unknown allergy_clinic action: %s", action)
	}
}

func (t *AllergyClinicTool) rashTriage(params map[string]interface{}) (string, error) {
	ageMonths := toFloat(params["age_months"])
	durationDays := toFloat(params["duration_days"])
	fever := toFloat(params["temperature_c"])
	rashAreas := parseAllergyList(params["rash_areas"])
	symptoms := parseAllergyList(params["symptoms"])
	appearance, _ := params["appearance"].(string)
	itchSeverity, _ := params["itch_severity"].(string)

	if len(rashAreas) == 0 && len(symptoms) == 0 && appearance == "" {
		return "", fmt.Errorf("rash_areas, symptoms, or appearance is required")
	}

	findings := []map[string]interface{}{}
	level := "normal"
	urgency := "routine"

	if fever >= 39 {
		findings = append(findings, map[string]interface{}{"category": "fever", "level": "moderate", "title": fmt.Sprintf("皮疹伴高热 %.1f℃", fever), "description": "需结合精神状态和皮疹形态尽快线下复核"})
		level = allergyMaxLevel(level, "moderate")
		urgency = allergyMaxUrgency(urgency, "same_day")
	}
	if durationDays >= 7 {
		findings = append(findings, map[string]interface{}{"category": "duration", "level": "mild", "title": fmt.Sprintf("皮疹持续 %.0f 天", durationDays), "description": "持续不缓解建议皮肤科或儿科复核"})
		level = allergyMaxLevel(level, "mild")
		urgency = allergyMaxUrgency(urgency, "expedited")
	}
	if containsAnyNormalizedAllergy([]string{appearance}, "紫癜", "petechiae", "purpura", "水疱", "blister", "大片脱皮", "黏膜", "口腔溃疡") {
		findings = append(findings, map[string]interface{}{"category": "appearance", "level": "severe", "title": "皮疹形态存在红旗信号", "description": "建议立即线下评估，必要时急诊处理"})
		level = allergyMaxLevel(level, "severe")
		urgency = allergyMaxUrgency(urgency, "emergency")
	}
	if containsAnyNormalizedAllergy([]string{itchSeverity}, "重度", "严重", "severe") {
		findings = append(findings, map[string]interface{}{"category": "itch", "level": "mild", "title": "瘙痒明显", "description": "提示过敏或湿疹样反应可能，需要结合诱因判断"})
		level = allergyMaxLevel(level, "mild")
	}
	for _, area := range rashAreas {
		normalized := normalizeAllergyTerm(area)
		switch {
		case containsAnyNormalizedAllergy([]string{normalized}, "口周", "眼周", "面部", "全身", "genital"):
			findings = append(findings, map[string]interface{}{"category": "distribution", "level": "mild", "title": fmt.Sprintf("皮疹分布：%s", area), "description": "需结合是否进展迅速、是否累及黏膜判断"})
			level = allergyMaxLevel(level, "mild")
		}
	}
	for _, symptom := range symptoms {
		normalized := normalizeAllergyTerm(symptom)
		switch {
		case containsAnyNormalizedAllergy([]string{normalized}, "呼吸困难", "声音嘶哑", "嗜睡", "抽搐", "紫癜", "黏膜出血"):
			findings = append(findings, map[string]interface{}{"category": "rash_red_flag", "level": "severe", "title": fmt.Sprintf("伴随红旗症状：%s", symptom), "description": "建议立即就医或急诊处理"})
			level = allergyMaxLevel(level, "severe")
			urgency = allergyMaxUrgency(urgency, "emergency")
		case containsAnyNormalizedAllergy([]string{normalized}, "瘙痒", "咽痛", "关节痛", "腹痛"):
			findings = append(findings, map[string]interface{}{"category": "associated_symptom", "level": "mild", "title": fmt.Sprintf("伴随症状：%s", symptom), "description": "建议结合病程、体温和皮疹形态进一步判断"})
			level = allergyMaxLevel(level, "mild")
		}
	}
	if ageMonths > 0 && ageMonths < 3 && len(rashAreas) > 0 {
		urgency = allergyMaxUrgency(urgency, "same_day")
	}

	return jsonStr(map[string]interface{}{
		"panel":         "rash",
		"age_months":    ageMonths,
		"duration_days": durationDays,
		"temperature_c": fever,
		"overall_level": level,
		"urgency":       urgency,
		"findings":      findings,
		"followup":      allergyFollowupAdvice("rash", urgency),
	}), nil
}

func (t *AllergyClinicTool) allergicReactionReview(params map[string]interface{}) (string, error) {
	ageMonths := toFloat(params["age_months"])
	exposure, _ := params["exposure"].(string)
	onsetMinutes := toFloat(params["onset_minutes"])
	symptoms := parseAllergyList(params["symptoms"])
	history := parseAllergyList(params["history"])
	temperature := toFloat(params["temperature_c"])

	if exposure == "" && len(symptoms) == 0 {
		return "", fmt.Errorf("exposure or symptoms is required")
	}

	findings := []map[string]interface{}{}
	level := "normal"
	urgency := "routine"

	if onsetMinutes > 0 && onsetMinutes <= 120 {
		findings = append(findings, map[string]interface{}{"category": "timing", "level": "mild", "title": fmt.Sprintf("疑似暴露后 %.0f 分钟内起病", onsetMinutes), "description": "与急性过敏反应时间窗相符，需结合症状判断"})
		level = allergyMaxLevel(level, "mild")
	}
	if exposure != "" {
		findings = append(findings, map[string]interface{}{"category": "exposure", "level": "mild", "title": fmt.Sprintf("可疑诱因：%s", exposure), "description": "需结合既往接触史和症状变化复核"})
		level = allergyMaxLevel(level, "mild")
	}
	if containsAnyNormalizedAllergy(history, "过敏史", "食物过敏", "药物过敏", "湿疹", "哮喘") {
		findings = append(findings, map[string]interface{}{"category": "history", "level": "mild", "title": "存在过敏相关既往史", "description": "提示再次过敏反应风险可能增加"})
		level = allergyMaxLevel(level, "mild")
	}
	for _, symptom := range symptoms {
		normalized := normalizeAllergyTerm(symptom)
		switch {
		case containsAnyNormalizedAllergy([]string{normalized}, "呼吸困难", "喘鸣", "喉头紧", "声音嘶哑", "晕厥", "血压低"):
			findings = append(findings, map[string]interface{}{"category": "anaphylaxis_flag", "level": "severe", "title": fmt.Sprintf("严重过敏线索：%s", symptom), "description": "建议立即急诊评估，不要仅依赖线上建议"})
			level = allergyMaxLevel(level, "severe")
			urgency = allergyMaxUrgency(urgency, "emergency")
		case containsAnyNormalizedAllergy([]string{normalized}, "全身荨麻疹", "面部肿胀", "呕吐", "腹痛"):
			findings = append(findings, map[string]interface{}{"category": "systemic_symptom", "level": "moderate", "title": fmt.Sprintf("系统性反应线索：%s", symptom), "description": "建议尽快线下复核过敏反应程度"})
			level = allergyMaxLevel(level, "moderate")
			urgency = allergyMaxUrgency(urgency, "same_day")
		case containsAnyNormalizedAllergy([]string{normalized}, "局部红斑", "瘙痒", "打喷嚏", "流涕"):
			findings = append(findings, map[string]interface{}{"category": "local_symptom", "level": "mild", "title": fmt.Sprintf("局部或轻度症状：%s", symptom), "description": "可结合诱因回避和症状变化继续观察"})
			level = allergyMaxLevel(level, "mild")
		}
	}
	if temperature >= 39 {
		findings = append(findings, map[string]interface{}{"category": "temperature", "level": "moderate", "title": fmt.Sprintf("伴高热 %.1f℃", temperature), "description": "高热不典型于单纯过敏，建议复核感染或其他病因"})
		level = allergyMaxLevel(level, "moderate")
		urgency = allergyMaxUrgency(urgency, "same_day")
	}
	if ageMonths > 0 && ageMonths < 12 && level != "normal" {
		urgency = allergyMaxUrgency(urgency, "expedited")
	}

	return jsonStr(map[string]interface{}{
		"panel":         "allergic_reaction",
		"age_months":    ageMonths,
		"exposure":      exposure,
		"onset_minutes": onsetMinutes,
		"temperature_c": temperature,
		"overall_level": level,
		"urgency":       urgency,
		"findings":      findings,
		"followup":      allergyFollowupAdvice("allergic_reaction", urgency),
	}), nil
}

func (t *AllergyClinicTool) anaphylaxisReview(params map[string]interface{}) (string, error) {
	trigger, _ := params["trigger"].(string)
	symptoms := parseAllergyList(params["symptoms"])
	minutesSinceExposure := toFloat(params["minutes_since_exposure"])
	bpConcern, _ := params["bp_concern"].(string)
	breathingStatus, _ := params["breathing_status"].(string)

	if trigger == "" && len(symptoms) == 0 && breathingStatus == "" {
		return "", fmt.Errorf("trigger, symptoms, or breathing_status is required")
	}

	findings := []map[string]interface{}{}
	level := "normal"
	urgency := "routine"

	if trigger != "" {
		findings = append(findings, map[string]interface{}{"category": "trigger", "level": "mild", "title": fmt.Sprintf("可疑诱因：%s", trigger), "description": "需结合暴露后时间窗和多系统症状评估"})
		level = allergyMaxLevel(level, "mild")
	}
	if minutesSinceExposure > 0 && minutesSinceExposure <= 60 {
		findings = append(findings, map[string]interface{}{"category": "timing", "level": "moderate", "title": fmt.Sprintf("暴露后 %.0f 分钟内出现症状", minutesSinceExposure), "description": "与急性严重过敏反应时间窗一致"})
		level = allergyMaxLevel(level, "moderate")
		urgency = allergyMaxUrgency(urgency, "same_day")
	}
	if containsAnyNormalizedAllergy([]string{bpConcern}, "低血压", "头晕", "晕厥", "苍白") {
		findings = append(findings, map[string]interface{}{"category": "circulation", "level": "severe", "title": "存在循环受累线索", "description": "建议立即急诊评估"})
		level = allergyMaxLevel(level, "severe")
		urgency = allergyMaxUrgency(urgency, "emergency")
	}
	if containsAnyNormalizedAllergy([]string{breathingStatus}, "喘憋", "呼吸困难", "喉鸣", "声音嘶哑", "紫绀") {
		findings = append(findings, map[string]interface{}{"category": "airway", "level": "severe", "title": "存在气道或呼吸受累线索", "description": "建议立即急诊评估"})
		level = allergyMaxLevel(level, "severe")
		urgency = allergyMaxUrgency(urgency, "emergency")
	}
	for _, symptom := range symptoms {
		normalized := normalizeAllergyTerm(symptom)
		switch {
		case containsAnyNormalizedAllergy([]string{normalized}, "呼吸困难", "喉头紧", "晕厥", "面色苍白", "持续呕吐"):
			findings = append(findings, map[string]interface{}{"category": "critical_symptom", "level": "severe", "title": fmt.Sprintf("危重信号：%s", symptom), "description": "符合严重过敏反应高风险线索"})
			level = allergyMaxLevel(level, "severe")
			urgency = allergyMaxUrgency(urgency, "emergency")
		case containsAnyNormalizedAllergy([]string{normalized}, "荨麻疹", "面唇肿胀", "腹痛", "呕吐"):
			findings = append(findings, map[string]interface{}{"category": "systemic_symptom", "level": "moderate", "title": fmt.Sprintf("系统症状：%s", symptom), "description": "若与呼吸或循环症状并存需高度警惕"})
			level = allergyMaxLevel(level, "moderate")
			urgency = allergyMaxUrgency(urgency, "same_day")
		}
	}

	return jsonStr(map[string]interface{}{
		"panel":                  "anaphylaxis_review",
		"trigger":                trigger,
		"minutes_since_exposure": minutesSinceExposure,
		"overall_level":          level,
		"urgency":                urgency,
		"findings":               findings,
		"followup":               allergyFollowupAdvice("anaphylaxis_review", urgency),
	}), nil
}

func (t *AllergyClinicTool) vaccineReactionReview(params map[string]interface{}) (string, error) {
	vaccineName, _ := params["vaccine_name"].(string)
	hoursSinceShot := toFloat(params["hours_since_shot"])
	temperature := toFloat(params["temperature_c"])
	symptoms := parseAllergyList(params["symptoms"])
	injectionSite, _ := params["injection_site_reaction"].(string)

	if vaccineName == "" && len(symptoms) == 0 && injectionSite == "" {
		return "", fmt.Errorf("vaccine_name, symptoms, or injection_site_reaction is required")
	}

	findings := []map[string]interface{}{}
	level := "normal"
	urgency := "routine"

	if vaccineName != "" {
		findings = append(findings, map[string]interface{}{"category": "vaccine", "level": "mild", "title": fmt.Sprintf("近期接种：%s", vaccineName), "description": "需结合接种后时间窗和症状判断常见反应或异常反应"})
		level = allergyMaxLevel(level, "mild")
	}
	if hoursSinceShot > 0 && hoursSinceShot <= 48 {
		findings = append(findings, map[string]interface{}{"category": "timing", "level": "mild", "title": fmt.Sprintf("接种后 %.0f 小时内出现症状", hoursSinceShot), "description": "处于常见接种后反应或急性反应观察窗口"})
		level = allergyMaxLevel(level, "mild")
	}
	if temperature >= 39 {
		findings = append(findings, map[string]interface{}{"category": "temperature", "level": "moderate", "title": fmt.Sprintf("接种后高热 %.1f℃", temperature), "description": "建议尽快线下复核是否属于异常反应或合并感染"})
		level = allergyMaxLevel(level, "moderate")
		urgency = allergyMaxUrgency(urgency, "same_day")
	}
	if containsAnyNormalizedAllergy([]string{injectionSite}, "红肿", "硬结", "疼痛") {
		findings = append(findings, map[string]interface{}{"category": "local_reaction", "level": "mild", "title": "存在接种部位局部反应", "description": "常见于接种后局部反应，需关注范围是否持续扩大"})
		level = allergyMaxLevel(level, "mild")
	}
	for _, symptom := range symptoms {
		normalized := normalizeAllergyTerm(symptom)
		switch {
		case containsAnyNormalizedAllergy([]string{normalized}, "呼吸困难", "面唇肿胀", "抽搐", "持续高热", "意识差"):
			findings = append(findings, map[string]interface{}{"category": "reaction_red_flag", "level": "severe", "title": fmt.Sprintf("接种后红旗症状：%s", symptom), "description": "建议立即线下评估或急诊处理"})
			level = allergyMaxLevel(level, "severe")
			urgency = allergyMaxUrgency(urgency, "emergency")
		case containsAnyNormalizedAllergy([]string{normalized}, "荨麻疹", "呕吐", "明显哭闹", "发热"):
			findings = append(findings, map[string]interface{}{"category": "systemic_reaction", "level": "moderate", "title": fmt.Sprintf("接种后系统症状：%s", symptom), "description": "建议结合病程和精神状态尽快复核"})
			level = allergyMaxLevel(level, "moderate")
			urgency = allergyMaxUrgency(urgency, "expedited")
		}
	}

	return jsonStr(map[string]interface{}{
		"panel":            "vaccine_reaction_review",
		"vaccine_name":     vaccineName,
		"hours_since_shot": hoursSinceShot,
		"temperature_c":    temperature,
		"overall_level":    level,
		"urgency":          urgency,
		"findings":         findings,
		"followup":         allergyFollowupAdvice("vaccine_reaction_review", urgency),
	}), nil
}

func parseAllergyList(v interface{}) []string {
	items := []string{}
	switch value := v.(type) {
	case []string:
		for _, item := range value {
			if trimmed := strings.TrimSpace(item); trimmed != "" {
				items = append(items, trimmed)
			}
		}
	case []interface{}:
		for _, item := range value {
			if trimmed := strings.TrimSpace(fmt.Sprintf("%v", item)); trimmed != "" {
				items = append(items, trimmed)
			}
		}
	case string:
		replacer := strings.NewReplacer("\n", ",", "；", ",", ";", ",", "、", ",", "，", ",")
		for _, part := range strings.Split(replacer.Replace(value), ",") {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				items = append(items, trimmed)
			}
		}
	}
	return uniqueSortedAllergyStrings(items)
}

func normalizeAllergyTerm(v string) string {
	replacer := strings.NewReplacer(" ", "", "-", "", "_", "", ".", "", "/", "", "（", "(", "）", ")")
	return strings.ToLower(strings.TrimSpace(replacer.Replace(v)))
}

func uniqueSortedAllergyStrings(items []string) []string {
	lookup := map[string]string{}
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		key := normalizeAllergyTerm(trimmed)
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

func allergyMaxLevel(current, next string) string {
	levels := map[string]int{"normal": 0, "mild": 1, "moderate": 2, "severe": 3}
	if levels[next] > levels[current] {
		return next
	}
	return current
}

func allergyMaxUrgency(current, next string) string {
	levels := map[string]int{"routine": 0, "expedited": 1, "same_day": 2, "emergency": 3}
	if levels[next] > levels[current] {
		return next
	}
	return current
}

func allergyFollowupAdvice(panel, urgency string) string {
	if urgency == "emergency" {
		return "建议立即线下就医或急诊评估，不要仅依赖线上建议。"
	}
	if urgency == "same_day" {
		return "建议当天线下就医，携带诱因、起病时间、体温和症状变化记录。"
	}
	switch panel {
	case "allergic_reaction":
		return "建议记录可疑诱因、起病时间和症状变化，避免再次暴露并按需复诊。"
	case "vaccine_reaction_review":
		return "建议记录接种时间、体温、局部反应范围和系统症状变化，必要时尽快复诊。"
	default:
		return "建议记录皮疹形态、范围、体温和伴随症状变化，必要时尽快复诊。"
	}
}

func containsAnyNormalizedAllergy(values []string, patterns ...string) bool {
	for _, value := range values {
		normalized := normalizeAllergyTerm(value)
		for _, pattern := range patterns {
			if strings.Contains(normalized, normalizeAllergyTerm(pattern)) {
				return true
			}
		}
	}
	return false
}
