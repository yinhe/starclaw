package tool

import (
	"fmt"
	"sort"
	"strings"

	"gorm.io/gorm"
)

type InfectionClinicTool struct {
	db *gorm.DB
}

func NewInfectionClinicTool(db *gorm.DB) *InfectionClinicTool {
	return &InfectionClinicTool{db: db}
}

func (t *InfectionClinicTool) Execute(userID string, params map[string]interface{}) (string, error) {
	action := normalizeInfectionAction(params)
	switch action {
	case "fever_triage":
		return t.feverTriage(params)
	case "respiratory_triage":
		return t.respiratoryTriage(params)
	case "gastroenteritis_triage":
		return t.gastroenteritisTriage(params)
	case "vaccine_review":
		return t.vaccineReview(params)
	default:
		return "", fmt.Errorf("unknown infection_clinic action: %s", action)
	}
}

func normalizeInfectionAction(params map[string]interface{}) string {
	action, _ := params["action"].(string)
	normalized := strings.TrimSpace(strings.ToLower(action))

	switch normalized {
	case "fever_triage", "respiratory_triage", "gastroenteritis_triage", "vaccine_review":
		return normalized
	case "triage", "assess", "assessment", "review", "":
		return inferInfectionAction(params)
	default:
		inferred := inferInfectionAction(params)
		if inferred != "" {
			return inferred
		}
		return normalized
	}
}

func inferInfectionAction(params map[string]interface{}) string {
	if hasAnyInfectionParam(params, "temperature_c", "fever_days", "mental_status", "intake_status", "urine_output") {
		return "fever_triage"
	}
	if hasAnyInfectionParam(params, "respiratory_rate", "spo2") {
		return "respiratory_triage"
	}
	if hasAnyInfectionParam(params, "vomiting_days", "diarrhea_days", "stool_count_per_day") {
		return "gastroenteritis_triage"
	}
	if hasAnyInfectionParam(params, "vaccine_name", "days_since_vaccine", "reaction_site", "reaction_symptoms") {
		return "vaccine_review"
	}

	for _, symptom := range parseInfectionList(params["symptoms"]) {
		normalized := normalizeInfectionTerm(symptom)
		switch {
		case containsAnyNormalized([]string{normalized}, "发热", "高热", "lethargy", "poor", "cough", "咳嗽"):
			return "fever_triage"
		case containsAnyNormalized([]string{normalized}, "呼吸困难", "dyspnea", "喘息", "wheeze", "低氧", "口唇发绀"):
			return "respiratory_triage"
		case containsAnyNormalized([]string{normalized}, "腹泻", "呕吐", "bloody stool", "腹痛"):
			return "gastroenteritis_triage"
		}
	}

	return ""
}

func hasAnyInfectionParam(params map[string]interface{}, keys ...string) bool {
	for _, key := range keys {
		if value, ok := params[key]; ok && value != nil {
			switch v := value.(type) {
			case string:
				if strings.TrimSpace(v) != "" {
					return true
				}
			default:
				return true
			}
		}
	}
	return false
}

func (t *InfectionClinicTool) feverTriage(params map[string]interface{}) (string, error) {
	ageMonths := toFloat(params["age_months"])
	temperature := toFloat(params["temperature_c"])
	feverDays := toFloat(params["fever_days"])
	symptoms := parseInfectionList(params["symptoms"])
	mentalStatus, _ := params["mental_status"].(string)
	intakeStatus, _ := params["intake_status"].(string)
	urineOutput, _ := params["urine_output"].(string)

	if temperature <= 0 && len(symptoms) == 0 {
		return "", fmt.Errorf("temperature_c or symptoms is required")
	}

	findings := []map[string]interface{}{}
	level := "normal"
	urgency := "routine"

	if ageMonths > 0 && ageMonths < 3 && temperature >= 38 {
		findings = append(findings, map[string]interface{}{"category": "age_fever", "level": "severe", "title": "3 月龄以下发热", "description": "低月龄婴儿发热需尽快线下评估"})
		level = infectionMaxLevel(level, "severe")
		urgency = infectionMaxUrgency(urgency, "same_day")
	}
	if temperature >= 40 {
		findings = append(findings, map[string]interface{}{"category": "temperature", "level": "severe", "title": fmt.Sprintf("高热 %.1f℃", temperature), "description": "建议尽快线下复核高热原因和精神状态"})
		level = infectionMaxLevel(level, "severe")
		urgency = infectionMaxUrgency(urgency, "same_day")
	} else if temperature >= 38.5 {
		findings = append(findings, map[string]interface{}{"category": "temperature", "level": "mild", "title": fmt.Sprintf("存在发热 %.1f℃", temperature), "description": "需结合病程和伴随症状评估"})
		level = infectionMaxLevel(level, "mild")
	}
	if feverDays >= 5 {
		findings = append(findings, map[string]interface{}{"category": "duration", "level": "moderate", "title": fmt.Sprintf("发热持续 %.0f 天", feverDays), "description": "持续发热建议尽快复诊复核"})
		level = infectionMaxLevel(level, "moderate")
		urgency = infectionMaxUrgency(urgency, "expedited")
	}

	for _, symptom := range symptoms {
		normalized := normalizeInfectionTerm(symptom)
		switch {
		case containsAnyNormalized([]string{normalized}, "抽搐", "convulsion", "意识差", "lethargy", "呼吸困难", "dyspnea", "胸痛", "petechiae", "紫癜", "无法唤醒"):
			findings = append(findings, map[string]interface{}{"category": "red_flag", "level": "severe", "title": fmt.Sprintf("红旗症状：%s", symptom), "description": "建议立即线下就医或急诊评估"})
			level = infectionMaxLevel(level, "severe")
			urgency = infectionMaxUrgency(urgency, "emergency")
		case containsAnyNormalized([]string{normalized}, "呼吸急促", "喘息", "wheeze", "呕吐", "腹泻", "皮疹", "咽痛", "耳痛"):
			findings = append(findings, map[string]interface{}{"category": "associated_symptom", "level": "mild", "title": fmt.Sprintf("伴随症状：%s", symptom), "description": "建议结合症状持续时间和精神状态继续观察或就诊"})
			level = infectionMaxLevel(level, "mild")
		}
	}

	if containsAnyNormalized([]string{mentalStatus}, "差", "嗜睡", "难唤醒", "poor", "lethargic") {
		findings = append(findings, map[string]interface{}{"category": "mental_status", "level": "severe", "title": "精神状态差", "description": "建议立即线下评估"})
		level = infectionMaxLevel(level, "severe")
		urgency = infectionMaxUrgency(urgency, "emergency")
	}
	if containsAnyNormalized([]string{intakeStatus}, "差", "拒食", "poor", "unabletodrink") || containsAnyNormalized([]string{urineOutput}, "明显减少", "reduced", "8小时无尿", "nourine") {
		findings = append(findings, map[string]interface{}{"category": "dehydration_risk", "level": "moderate", "title": "存在脱水风险线索", "description": "需关注饮水量和尿量变化"})
		level = infectionMaxLevel(level, "moderate")
		urgency = infectionMaxUrgency(urgency, "same_day")
	}

	return jsonStr(map[string]interface{}{
		"panel":         "fever",
		"age_months":    ageMonths,
		"temperature_c": temperature,
		"fever_days":    feverDays,
		"overall_level": level,
		"urgency":       urgency,
		"findings":      findings,
		"followup":      infectionFollowupAdvice("fever", urgency),
	}), nil
}

func (t *InfectionClinicTool) respiratoryTriage(params map[string]interface{}) (string, error) {
	ageMonths := toFloat(params["age_months"])
	respiratoryRate := toFloat(params["respiratory_rate"])
	spo2 := toFloat(params["spo2"])
	symptoms := parseInfectionList(params["symptoms"])
	fever := toFloat(params["temperature_c"])

	if respiratoryRate <= 0 && spo2 <= 0 && len(symptoms) == 0 {
		return "", fmt.Errorf("respiratory_rate, spo2, or symptoms is required")
	}

	findings := []map[string]interface{}{}
	level := "normal"
	urgency := "routine"

	if spo2 > 0 {
		if spo2 < 92 {
			findings = append(findings, map[string]interface{}{"category": "spo2", "level": "severe", "title": fmt.Sprintf("血氧偏低 %.0f%%", spo2), "description": "建议尽快急诊或当天线下评估"})
			level = infectionMaxLevel(level, "severe")
			urgency = infectionMaxUrgency(urgency, "emergency")
		} else if spo2 < 95 {
			findings = append(findings, map[string]interface{}{"category": "spo2", "level": "moderate", "title": fmt.Sprintf("血氧边缘偏低 %.0f%%", spo2), "description": "建议尽快复诊复核"})
			level = infectionMaxLevel(level, "moderate")
			urgency = infectionMaxUrgency(urgency, "same_day")
		}
	}
	if respiratoryRate > 0 && respiratoryRate > ageBasedRespUpper(ageMonths) {
		findings = append(findings, map[string]interface{}{"category": "respiratory_rate", "level": "moderate", "title": fmt.Sprintf("呼吸频率偏快 %.0f 次/分", respiratoryRate), "description": "需结合喘息、呼吸费力和血氧判断"})
		level = infectionMaxLevel(level, "moderate")
		urgency = infectionMaxUrgency(urgency, "same_day")
	}
	for _, symptom := range symptoms {
		normalized := normalizeInfectionTerm(symptom)
		switch {
		case containsAnyNormalized([]string{normalized}, "呼吸困难", "dyspnea", "口唇发绀", "cyanosis", "三凹征", "无法完整说话"):
			findings = append(findings, map[string]interface{}{"category": "respiratory_red_flag", "level": "severe", "title": fmt.Sprintf("呼吸红旗信号：%s", symptom), "description": "建议立即线下或急诊评估"})
			level = infectionMaxLevel(level, "severe")
			urgency = infectionMaxUrgency(urgency, "emergency")
		case containsAnyNormalized([]string{normalized}, "喘息", "wheeze", "犬吠样咳嗽", "持续咳嗽", "胸闷"):
			findings = append(findings, map[string]interface{}{"category": "respiratory_symptom", "level": "mild", "title": fmt.Sprintf("呼吸道症状：%s", symptom), "description": "建议结合病程、发热和呼吸费力评估"})
			level = infectionMaxLevel(level, "mild")
		}
	}
	if fever >= 39 && len(symptoms) > 0 {
		urgency = infectionMaxUrgency(urgency, "expedited")
	}

	return jsonStr(map[string]interface{}{
		"panel":            "respiratory",
		"age_months":       ageMonths,
		"respiratory_rate": respiratoryRate,
		"spo2":             spo2,
		"temperature_c":    fever,
		"overall_level":    level,
		"urgency":          urgency,
		"findings":         findings,
		"followup":         infectionFollowupAdvice("respiratory", urgency),
	}), nil
}

func (t *InfectionClinicTool) gastroenteritisTriage(params map[string]interface{}) (string, error) {
	ageMonths := toFloat(params["age_months"])
	vomitingDays := toFloat(params["vomiting_days"])
	diarrheaDays := toFloat(params["diarrhea_days"])
	stoolCount := toFloat(params["stool_count_per_day"])
	symptoms := parseInfectionList(params["symptoms"])
	urineOutput, _ := params["urine_output"].(string)
	intakeStatus, _ := params["intake_status"].(string)

	if vomitingDays <= 0 && diarrheaDays <= 0 && len(symptoms) == 0 {
		return "", fmt.Errorf("vomiting_days, diarrhea_days, or symptoms is required")
	}

	findings := []map[string]interface{}{}
	level := "normal"
	urgency := "routine"

	if vomitingDays >= 2 {
		findings = append(findings, map[string]interface{}{"category": "vomiting", "level": "mild", "title": fmt.Sprintf("呕吐持续 %.0f 天", vomitingDays), "description": "需关注脱水风险和进食情况"})
		level = infectionMaxLevel(level, "mild")
	}
	if diarrheaDays >= 3 || stoolCount >= 8 {
		findings = append(findings, map[string]interface{}{"category": "diarrhea", "level": "moderate", "title": fmt.Sprintf("腹泻风险较高，持续 %.0f 天", diarrheaDays), "description": "建议关注尿量、精神状态和口服补液情况"})
		level = infectionMaxLevel(level, "moderate")
		urgency = infectionMaxUrgency(urgency, "expedited")
	}
	for _, symptom := range symptoms {
		normalized := normalizeInfectionTerm(symptom)
		switch {
		case containsAnyNormalized([]string{normalized}, "血便", "bloody stool", "喷射性呕吐", "持续腹痛", "严重腹痛", "昏睡"):
			findings = append(findings, map[string]interface{}{"category": "gastro_red_flag", "level": "severe", "title": fmt.Sprintf("胃肠红旗信号：%s", symptom), "description": "建议立即线下或急诊评估"})
			level = infectionMaxLevel(level, "severe")
			urgency = infectionMaxUrgency(urgency, "emergency")
		case containsAnyNormalized([]string{normalized}, "发热", "呕吐", "腹泻", "腹胀"):
			level = infectionMaxLevel(level, "mild")
		}
	}
	if containsAnyNormalized([]string{urineOutput}, "明显减少", "reduced", "8小时无尿", "nourine") || containsAnyNormalized([]string{intakeStatus}, "拒饮", "poor", "unabletodrink") {
		findings = append(findings, map[string]interface{}{"category": "dehydration", "level": "severe", "title": "存在明显脱水风险", "description": "需尽快线下处理补液和复核"})
		level = infectionMaxLevel(level, "severe")
		urgency = infectionMaxUrgency(urgency, "same_day")
	}
	if ageMonths > 0 && ageMonths < 6 && (vomitingDays > 0 || diarrheaDays > 0) {
		urgency = infectionMaxUrgency(urgency, "expedited")
	}

	return jsonStr(map[string]interface{}{
		"panel":               "gastroenteritis",
		"age_months":          ageMonths,
		"vomiting_days":       vomitingDays,
		"diarrhea_days":       diarrheaDays,
		"stool_count_per_day": stoolCount,
		"overall_level":       level,
		"urgency":             urgency,
		"findings":            findings,
		"followup":            infectionFollowupAdvice("gastroenteritis", urgency),
	}), nil
}

func (t *InfectionClinicTool) vaccineReview(params map[string]interface{}) (string, error) {
	ageMonths := toFloat(params["age_months"])
	vaccineName, _ := params["vaccine_name"].(string)
	lastDoseMonths := toFloat(params["last_dose_months_ago"])
	missedDoses := toFloat(params["missed_doses"])
	contraindications := parseInfectionList(params["contraindications"])
	currentSymptoms := parseInfectionList(params["current_symptoms"])

	if vaccineName == "" && ageMonths <= 0 {
		return "", fmt.Errorf("vaccine_name or age_months is required")
	}

	findings := []map[string]interface{}{}
	level := "normal"
	status := "可结合免疫程序常规评估接种安排"

	if missedDoses > 0 {
		findings = append(findings, map[string]interface{}{"category": "schedule", "level": "moderate", "title": fmt.Sprintf("存在漏种 %.0f 针", missedDoses), "description": "建议补种前核对既往免疫接种记录"})
		level = infectionMaxLevel(level, "moderate")
		status = "建议尽快核对免疫程序并安排补种"
	}
	if containsAnyNormalized(contraindications, "严重过敏", "anaphylaxis", "免疫抑制", "高热") {
		findings = append(findings, map[string]interface{}{"category": "contraindication", "level": "severe", "title": "存在需线下确认的接种禁忌或慎用情况", "description": "建议由接种门诊或医生当面评估"})
		level = infectionMaxLevel(level, "severe")
		status = "存在接种前需当面复核的风险因素"
	}
	if containsAnyNormalized(currentSymptoms, "高热", "持续发热", "急性严重感染") {
		findings = append(findings, map[string]interface{}{"category": "current_illness", "level": "moderate", "title": "当前急性症状可能影响接种安排", "description": "建议待急性症状稳定后再评估接种"})
		level = infectionMaxLevel(level, "moderate")
	}
	if ageMonths > 0 && ageMonths < 2 && vaccineName == "" {
		findings = append(findings, map[string]interface{}{"category": "infant_schedule", "level": "mild", "title": "低月龄婴儿需严格核对免疫程序", "description": "建议携带接种本线下核对疫苗安排"})
		level = infectionMaxLevel(level, "mild")
	}
	if lastDoseMonths >= 12 && vaccineName != "" {
		findings = append(findings, map[string]interface{}{"category": "interval", "level": "mild", "title": fmt.Sprintf("距离上次接种已 %.0f 个月", lastDoseMonths), "description": "建议结合免疫程序核对是否需要加强针或补种"})
		level = infectionMaxLevel(level, "mild")
	}

	return jsonStr(map[string]interface{}{
		"panel":                "vaccine_review",
		"age_months":           ageMonths,
		"vaccine_name":         vaccineName,
		"missed_doses":         missedDoses,
		"last_dose_months_ago": lastDoseMonths,
		"overall_level":        level,
		"status":               status,
		"findings":             findings,
		"followup":             "建议携带疫苗接种本、既往不良反应记录和近期病史，到接种门诊或儿科进一步核对。",
	}), nil
}

func parseInfectionList(v interface{}) []string {
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
	return uniqueSortedInfectionStrings(items)
}

func normalizeInfectionTerm(v string) string {
	replacer := strings.NewReplacer(" ", "", "-", "", "_", "", ".", "", "/", "", "（", "(", "）", ")")
	return strings.ToLower(strings.TrimSpace(replacer.Replace(v)))
}

func uniqueSortedInfectionStrings(items []string) []string {
	lookup := map[string]string{}
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		key := normalizeInfectionTerm(trimmed)
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

func infectionMaxLevel(current, next string) string {
	levels := map[string]int{"normal": 0, "mild": 1, "moderate": 2, "severe": 3}
	if levels[next] > levels[current] {
		return next
	}
	return current
}

func infectionMaxUrgency(current, next string) string {
	levels := map[string]int{"routine": 0, "expedited": 1, "same_day": 2, "emergency": 3}
	if levels[next] > levels[current] {
		return next
	}
	return current
}

func ageBasedRespUpper(ageMonths float64) float64 {
	switch {
	case ageMonths <= 2:
		return 60
	case ageMonths <= 12:
		return 50
	case ageMonths <= 60:
		return 40
	default:
		return 30
	}
}

func infectionFollowupAdvice(panel, urgency string) string {
	if urgency == "emergency" {
		return "建议立即线下就医或急诊评估，不要仅依赖线上建议。"
	}
	if urgency == "same_day" {
		return "建议当天线下就医，携带体温、尿量、精神状态和症状变化记录。"
	}
	switch panel {
	case "respiratory":
		return "建议记录呼吸频率、发热、咳嗽/喘息变化，如加重尽快复诊。"
	case "gastroenteritis":
		return "建议关注饮水量、尿量、精神状态和大便次数，必要时尽快复诊。"
	case "vaccine_review":
		return "建议携带接种本和既往不良反应记录到接种门诊进一步核对。"
	default:
		return "建议记录体温、精神状态、饮水和症状变化，按病程复诊复核。"
	}
}
