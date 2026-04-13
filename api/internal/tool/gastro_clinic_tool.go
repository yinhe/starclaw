package tool

import (
	"fmt"
	"sort"
	"strings"

	"gorm.io/gorm"
)

type GastroClinicTool struct {
	db *gorm.DB
}

func NewGastroClinicTool(db *gorm.DB) *GastroClinicTool {
	return &GastroClinicTool{db: db}
}

func (t *GastroClinicTool) Execute(userID string, params map[string]interface{}) (string, error) {
	action, _ := params["action"].(string)
	switch action {
	case "abdominal_pain_triage":
		return t.abdominalPainTriage(params)
	case "constipation_review":
		return t.constipationReview(params)
	case "reflux_review":
		return t.refluxReview(params)
	case "digestive_red_flag_review":
		return t.digestiveRedFlagReview(params)
	default:
		return "", fmt.Errorf("unknown gastro_clinic action: %s", action)
	}
}

func (t *GastroClinicTool) abdominalPainTriage(params map[string]interface{}) (string, error) {
	ageYears := toFloat(params["age_years"])
	durationHours := toFloat(params["duration_hours"])
	painLocation, _ := params["pain_location"].(string)
	painSeverity := toFloat(params["pain_severity"])
	fever := toFloat(params["temperature_c"])
	vomiting := gastroToBool(params["vomiting"])
	diarrhea := gastroToBool(params["diarrhea"])
	symptoms := parseGastroList(params["symptoms"])

	if durationHours <= 0 && painSeverity <= 0 && painLocation == "" && len(symptoms) == 0 {
		return "", fmt.Errorf("abdominal pain inputs are required")
	}

	findings := []map[string]interface{}{}
	level := "normal"
	urgency := "routine"

	if painSeverity >= 8 {
		findings = append(findings, map[string]interface{}{"category": "pain", "level": "severe", "title": fmt.Sprintf("腹痛较重（%.0f/10）", painSeverity), "description": "剧烈腹痛建议尽快线下评估"})
		level = gastroMaxLevel(level, "severe")
		urgency = gastroMaxUrgency(urgency, "same_day")
	} else if painSeverity >= 5 {
		findings = append(findings, map[string]interface{}{"category": "pain", "level": "moderate", "title": fmt.Sprintf("腹痛中度（%.0f/10）", painSeverity), "description": "建议结合部位、病程和伴随症状复核"})
		level = gastroMaxLevel(level, "moderate")
		urgency = gastroMaxUrgency(urgency, "expedited")
	}
	if durationHours >= 24 {
		findings = append(findings, map[string]interface{}{"category": "duration", "level": "moderate", "title": fmt.Sprintf("腹痛持续 %.0f 小时", durationHours), "description": "持续腹痛建议近期线下复核"})
		level = gastroMaxLevel(level, "moderate")
		urgency = gastroMaxUrgency(urgency, "expedited")
	}
	if containsAnyNormalizedGastro([]string{painLocation}, "右下腹", "rlq", "右上腹", "全腹", "固定") {
		findings = append(findings, map[string]interface{}{"category": "location", "level": "moderate", "title": fmt.Sprintf("腹痛部位：%s", painLocation), "description": "固定或特定部位腹痛建议更积极线下评估"})
		level = gastroMaxLevel(level, "moderate")
		urgency = gastroMaxUrgency(urgency, "expedited")
	}
	if fever >= 39 {
		findings = append(findings, map[string]interface{}{"category": "fever", "level": "moderate", "title": fmt.Sprintf("腹痛伴高热 %.1f℃", fever), "description": "建议复核感染或腹部炎症线索"})
		level = gastroMaxLevel(level, "moderate")
		urgency = gastroMaxUrgency(urgency, "same_day")
	}
	if vomiting || diarrhea {
		findings = append(findings, map[string]interface{}{"category": "associated_gi", "level": "mild", "title": "伴随呕吐或腹泻", "description": "需结合脱水、疼痛加重和精神状态继续判断"})
		level = gastroMaxLevel(level, "mild")
	}
	for _, symptom := range symptoms {
		normalized := normalizeGastroTerm(symptom)
		switch {
		case containsAnyNormalizedGastro([]string{normalized}, "血便", "黑便", "胆汁性呕吐", "腹胀明显", "反跳痛", "不能走路", "嗜睡"):
			findings = append(findings, map[string]interface{}{"category": "abdominal_red_flag", "level": "severe", "title": fmt.Sprintf("腹痛红旗：%s", symptom), "description": "建议立即线下或急诊评估"})
			level = gastroMaxLevel(level, "severe")
			urgency = gastroMaxUrgency(urgency, "emergency")
		case containsAnyNormalizedGastro([]string{normalized}, "恶心", "食欲差", "便秘", "腹泻"):
			level = gastroMaxLevel(level, "mild")
		}
	}
	if ageYears > 0 && ageYears < 2 && painSeverity > 0 {
		urgency = gastroMaxUrgency(urgency, "same_day")
	}

	return jsonStr(map[string]interface{}{
		"panel":         "abdominal_pain_triage",
		"age_years":     ageYears,
		"overall_level": level,
		"urgency":       urgency,
		"findings":      findings,
		"followup":      gastroFollowupAdvice("abdominal_pain", urgency),
	}), nil
}

func (t *GastroClinicTool) constipationReview(params map[string]interface{}) (string, error) {
	ageYears := toFloat(params["age_years"])
	stoolDays := toFloat(params["days_between_stools"])
	painfulDefecation := gastroToBool(params["painful_defecation"])
	bloodOnStool := gastroToBool(params["blood_on_stool"])
	withholding := gastroToBool(params["withholding_behavior"])
	abdominalDistension := gastroToBool(params["abdominal_distension"])
	symptoms := parseGastroList(params["symptoms"])

	if stoolDays <= 0 && len(symptoms) == 0 {
		return "", fmt.Errorf("constipation inputs are required")
	}

	findings := []map[string]interface{}{}
	level := "normal"
	urgency := "routine"

	if stoolDays >= 4 {
		findings = append(findings, map[string]interface{}{"category": "frequency", "level": "moderate", "title": fmt.Sprintf("排便间隔 %.0f 天", stoolDays), "description": "提示便秘风险较高，建议线下复核管理方案"})
		level = gastroMaxLevel(level, "moderate")
		urgency = gastroMaxUrgency(urgency, "expedited")
	}
	if painfulDefecation || withholding {
		findings = append(findings, map[string]interface{}{"category": "stool_behavior", "level": "mild", "title": "存在排便疼痛或憋便行为", "description": "常提示功能性便秘持续化风险"})
		level = gastroMaxLevel(level, "mild")
	}
	if bloodOnStool {
		findings = append(findings, map[string]interface{}{"category": "blood", "level": "moderate", "title": "便后见血线索", "description": "需区分肛裂、便秘相关损伤或其他原因"})
		level = gastroMaxLevel(level, "moderate")
		urgency = gastroMaxUrgency(urgency, "expedited")
	}
	if abdominalDistension {
		findings = append(findings, map[string]interface{}{"category": "distension", "level": "moderate", "title": "存在腹胀线索", "description": "若腹胀明显或进行性加重，建议尽快复核"})
		level = gastroMaxLevel(level, "moderate")
		urgency = gastroMaxUrgency(urgency, "same_day")
	}
	for _, symptom := range symptoms {
		if containsAnyNormalizedGastro([]string{symptom}, "呕吐", "体重下降", "夜间痛醒", "发热", "血便") {
			findings = append(findings, map[string]interface{}{"category": "constipation_red_flag", "level": "severe", "title": fmt.Sprintf("便秘红旗：%s", symptom), "description": "建议尽快线下评估"})
			level = gastroMaxLevel(level, "severe")
			urgency = gastroMaxUrgency(urgency, "same_day")
		}
	}
	if ageYears > 0 && ageYears < 1 && stoolDays >= 3 {
		urgency = gastroMaxUrgency(urgency, "same_day")
	}

	return jsonStr(map[string]interface{}{
		"panel":         "constipation_review",
		"age_years":     ageYears,
		"overall_level": level,
		"urgency":       urgency,
		"findings":      findings,
		"followup":      gastroFollowupAdvice("constipation", urgency),
	}), nil
}

func (t *GastroClinicTool) refluxReview(params map[string]interface{}) (string, error) {
	ageMonths := toFloat(params["age_months"])
	postFeedSpitup := gastroToBool(params["post_feed_spitup"])
	backArching := gastroToBool(params["back_arching"])
	poorWeightGain := gastroToBool(params["poor_weight_gain"])
	respiratorySymptoms := gastroToBool(params["respiratory_symptoms"])
	biliousVomiting := gastroToBool(params["bilious_vomiting"])
	bloodInVomit := gastroToBool(params["blood_in_vomit"])
	symptoms := parseGastroList(params["symptoms"])

	if !postFeedSpitup && len(symptoms) == 0 && !backArching && !poorWeightGain {
		return "", fmt.Errorf("reflux-related inputs are required")
	}

	findings := []map[string]interface{}{}
	level := "normal"
	urgency := "routine"

	if postFeedSpitup {
		findings = append(findings, map[string]interface{}{"category": "spitup", "level": "mild", "title": "餐后溢奶/反流线索", "description": "需结合月龄、生长情况和警示症状判断"})
		level = gastroMaxLevel(level, "mild")
	}
	if backArching {
		findings = append(findings, map[string]interface{}{"category": "discomfort", "level": "moderate", "title": "存在弓背/明显不适线索", "description": "提示反流相关不适可能，需要进一步复核"})
		level = gastroMaxLevel(level, "moderate")
		urgency = gastroMaxUrgency(urgency, "expedited")
	}
	if poorWeightGain {
		findings = append(findings, map[string]interface{}{"category": "growth", "level": "severe", "title": "反流伴体重增长不佳线索", "description": "建议尽快线下评估喂养和消化问题"})
		level = gastroMaxLevel(level, "severe")
		urgency = gastroMaxUrgency(urgency, "same_day")
	}
	if respiratorySymptoms {
		findings = append(findings, map[string]interface{}{"category": "airway", "level": "moderate", "title": "伴呼吸道症状", "description": "需注意反流相关咳嗽或误吸风险"})
		level = gastroMaxLevel(level, "moderate")
		urgency = gastroMaxUrgency(urgency, "expedited")
	}
	if biliousVomiting || bloodInVomit {
		findings = append(findings, map[string]interface{}{"category": "vomit_red_flag", "level": "severe", "title": "存在胆汁性呕吐或呕血线索", "description": "建议立即线下或急诊评估"})
		level = gastroMaxLevel(level, "severe")
		urgency = gastroMaxUrgency(urgency, "emergency")
	}
	for _, symptom := range symptoms {
		if containsAnyNormalizedGastro([]string{symptom}, "拒奶", "哭闹", "反复呛咳", "窒息样") {
			findings = append(findings, map[string]interface{}{"category": "associated", "level": "moderate", "title": fmt.Sprintf("伴随线索：%s", symptom), "description": "建议结合喂养和体重情况进一步判断"})
			level = gastroMaxLevel(level, "moderate")
			urgency = gastroMaxUrgency(urgency, "expedited")
		}
	}
	if ageMonths > 0 && ageMonths < 6 && postFeedSpitup && !poorWeightGain && !biliousVomiting && !bloodInVomit {
		level = gastroMaxLevel(level, "mild")
	}

	return jsonStr(map[string]interface{}{
		"panel":         "reflux_review",
		"age_months":    ageMonths,
		"overall_level": level,
		"urgency":       urgency,
		"findings":      findings,
		"followup":      gastroFollowupAdvice("reflux", urgency),
	}), nil
}

func (t *GastroClinicTool) digestiveRedFlagReview(params map[string]interface{}) (string, error) {
	ageYears := toFloat(params["age_years"])
	poorIntake := gastroToBool(params["poor_intake"])
	urineReduction := gastroToBool(params["urine_reduction"])
	weightLoss := gastroToBool(params["weight_loss"])
	abdominalDistension := gastroToBool(params["abdominal_distension"])
	symptoms := parseGastroList(params["symptoms"])
	recentMeds := parseGastroList(params["recent_medications"])

	if len(symptoms) == 0 && !poorIntake && !urineReduction && !weightLoss && !abdominalDistension {
		return "", fmt.Errorf("digestive red flag inputs are required")
	}

	findings := []map[string]interface{}{}
	level := "normal"
	urgency := "routine"

	if poorIntake || urineReduction {
		findings = append(findings, map[string]interface{}{"category": "hydration", "level": "moderate", "title": "存在进食差或尿量减少线索", "description": "提示脱水风险，需要尽快复核"})
		level = gastroMaxLevel(level, "moderate")
		urgency = gastroMaxUrgency(urgency, "same_day")
	}
	if weightLoss {
		findings = append(findings, map[string]interface{}{"category": "weight", "level": "severe", "title": "存在体重下降线索", "description": "建议尽快线下评估"})
		level = gastroMaxLevel(level, "severe")
		urgency = gastroMaxUrgency(urgency, "same_day")
	}
	if abdominalDistension {
		findings = append(findings, map[string]interface{}{"category": "distension", "level": "severe", "title": "腹胀明显", "description": "明显腹胀伴呕吐或疼痛建议立即评估"})
		level = gastroMaxLevel(level, "severe")
		urgency = gastroMaxUrgency(urgency, "same_day")
	}
	if len(recentMeds) > 0 {
		findings = append(findings, map[string]interface{}{"category": "medications", "level": "mild", "title": "近期存在用药史", "description": "需结合药物不良反应或胃肠刺激进一步判断"})
		level = gastroMaxLevel(level, "mild")
	}
	for _, symptom := range symptoms {
		normalized := normalizeGastroTerm(symptom)
		switch {
		case containsAnyNormalizedGastro([]string{normalized}, "胆汁性呕吐", "呕血", "黑便", "血便", "持续剧痛", "昏睡"):
			findings = append(findings, map[string]interface{}{"category": "digestive_red_flag", "level": "severe", "title": fmt.Sprintf("消化道红旗：%s", symptom), "description": "建议立即线下或急诊评估"})
			level = gastroMaxLevel(level, "severe")
			urgency = gastroMaxUrgency(urgency, "emergency")
		case containsAnyNormalizedGastro([]string{normalized}, "恶心", "腹泻", "便秘", "食欲差"):
			level = gastroMaxLevel(level, "mild")
		}
	}
	if ageYears > 0 && ageYears < 1 && (poorIntake || urineReduction || abdominalDistension) {
		urgency = gastroMaxUrgency(urgency, "same_day")
	}

	return jsonStr(map[string]interface{}{
		"panel":         "digestive_red_flag_review",
		"age_years":     ageYears,
		"overall_level": level,
		"urgency":       urgency,
		"findings":      findings,
		"followup":      gastroFollowupAdvice("digestive_red_flag", urgency),
	}), nil
}

func parseGastroList(v interface{}) []string {
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
	return uniqueSortedGastroStrings(items)
}

func normalizeGastroTerm(v string) string {
	replacer := strings.NewReplacer(" ", "", "-", "", "_", "", ".", "", "/", "", "（", "(", "）", ")")
	return strings.ToLower(strings.TrimSpace(replacer.Replace(v)))
}

func uniqueSortedGastroStrings(items []string) []string {
	lookup := map[string]string{}
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		key := normalizeGastroTerm(trimmed)
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

func gastroMaxLevel(current, next string) string {
	levels := map[string]int{"normal": 0, "mild": 1, "moderate": 2, "severe": 3}
	if levels[next] > levels[current] {
		return next
	}
	return current
}

func gastroMaxUrgency(current, next string) string {
	levels := map[string]int{"routine": 0, "expedited": 1, "same_day": 2, "emergency": 3}
	if levels[next] > levels[current] {
		return next
	}
	return current
}

func gastroFollowupAdvice(panel, urgency string) string {
	if urgency == "emergency" {
		return "建议立即线下就医或急诊评估，不要仅依赖线上建议。"
	}
	if urgency == "same_day" {
		return "建议当天线下就医，携带病程、体温、进食饮水、排便和呕吐情况记录。"
	}
	switch panel {
	case "constipation":
		return "建议记录排便频率、便形、排便疼痛和憋便行为变化，必要时尽快复诊。"
	case "reflux":
		return "建议记录喂养量、餐后反流、体重和呛咳变化，必要时尽快线下复核。"
	default:
		return "建议记录腹痛部位、病程、排便/呕吐和进食变化，必要时尽快复诊。"
	}
}

func containsAnyNormalizedGastro(values []string, patterns ...string) bool {
	for _, value := range values {
		normalized := normalizeGastroTerm(value)
		for _, pattern := range patterns {
			if strings.Contains(normalized, normalizeGastroTerm(pattern)) {
				return true
			}
		}
	}
	return false
}

func gastroToBool(v interface{}) bool {
	switch value := v.(type) {
	case bool:
		return value
	case string:
		normalized := strings.ToLower(strings.TrimSpace(value))
		return normalized == "true" || normalized == "1" || normalized == "yes" || normalized == "y" || normalized == "是"
	case float64:
		return value != 0
	case int:
		return value != 0
	default:
		return false
	}
}
