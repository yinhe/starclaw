package tool

import (
	"fmt"
	"sort"
	"strings"

	"gorm.io/gorm"
)

type MentalHealthClinicTool struct {
	db *gorm.DB
}

func NewMentalHealthClinicTool(db *gorm.DB) *MentalHealthClinicTool {
	return &MentalHealthClinicTool{db: db}
}

func (t *MentalHealthClinicTool) Execute(userID string, params map[string]interface{}) (string, error) {
	action, _ := params["action"].(string)
	switch action {
	case "mood_anxiety_screen":
		return t.moodAnxietyScreen(params)
	case "school_stress_review":
		return t.schoolStressReview(params)
	case "sleep_emotion_review":
		return t.sleepEmotionReview(params)
	case "self_harm_crisis_review":
		return t.selfHarmCrisisReview(params)
	default:
		return "", fmt.Errorf("unknown mental_health_clinic action: %s", action)
	}
}

func (t *MentalHealthClinicTool) moodAnxietyScreen(params map[string]interface{}) (string, error) {
	ageYears := toFloat(params["age_years"])
	durationWeeks := toFloat(params["duration_weeks"])
	functionalImpact, _ := params["functional_impact"].(string)
	panicLikeEpisodes := mentalHealthToBool(params["panic_like_episodes"])
	physicalSymptoms := parseMentalHealthList(params["physical_symptoms"])
	symptoms := parseMentalHealthList(params["symptoms"])

	if durationWeeks <= 0 && len(symptoms) == 0 && len(physicalSymptoms) == 0 {
		return "", fmt.Errorf("mood or anxiety inputs are required")
	}

	findings := []map[string]interface{}{}
	level := "normal"
	urgency := "routine"

	if durationWeeks >= 2 {
		findings = append(findings, map[string]interface{}{"category": "duration", "level": "moderate", "title": fmt.Sprintf("情绪/焦虑问题持续 %.0f 周", durationWeeks), "description": "持续时间较长，建议结合功能影响和安全线索进一步复核"})
		level = mentalHealthMaxLevel(level, "moderate")
		urgency = mentalHealthMaxUrgency(urgency, "expedited")
	}
	if panicLikeEpisodes {
		findings = append(findings, map[string]interface{}{"category": "panic_like", "level": "moderate", "title": "存在惊恐样发作线索", "description": "建议结合胸闷、心悸、过度换气和触发因素进一步复核"})
		level = mentalHealthMaxLevel(level, "moderate")
		urgency = mentalHealthMaxUrgency(urgency, "expedited")
	}
	if containsAnyNormalizedMentalHealth([]string{functionalImpact}, "明显", "严重", "上学困难", "停课", "poor") {
		findings = append(findings, map[string]interface{}{"category": "function", "level": "moderate", "title": "已影响日常功能", "description": "提示需要尽快线下心理/精神专科评估"})
		level = mentalHealthMaxLevel(level, "moderate")
		urgency = mentalHealthMaxUrgency(urgency, "same_day")
	}
	for _, symptom := range append(symptoms, physicalSymptoms...) {
		normalized := normalizeMentalHealthTerm(symptom)
		switch {
		case containsAnyNormalizedMentalHealth([]string{normalized}, "自杀", "轻生", "自伤", "割腕", "绝望", "听到声音", "幻觉"):
			findings = append(findings, map[string]interface{}{"category": "mental_health_red_flag", "level": "severe", "title": fmt.Sprintf("精神心理红旗：%s", symptom), "description": "建议立即线下或急诊评估"})
			level = mentalHealthMaxLevel(level, "severe")
			urgency = mentalHealthMaxUrgency(urgency, "emergency")
		case containsAnyNormalizedMentalHealth([]string{normalized}, "持续低落", "焦虑", "烦躁", "躯体不适", "心悸", "胃痛"):
			level = mentalHealthMaxLevel(level, "mild")
		}
	}
	if ageYears > 0 && ageYears < 12 && level != "normal" {
		urgency = mentalHealthMaxUrgency(urgency, "expedited")
	}

	return jsonStr(map[string]interface{}{
		"panel":         "mood_anxiety_screen",
		"age_years":     ageYears,
		"overall_level": level,
		"urgency":       urgency,
		"findings":      findings,
		"followup":      mentalHealthFollowupAdvice("mood", urgency),
	}), nil
}

func (t *MentalHealthClinicTool) schoolStressReview(params map[string]interface{}) (string, error) {
	ageYears := toFloat(params["age_years"])
	schoolAvoidance := mentalHealthToBool(params["school_avoidance"])
	peerConflict := mentalHealthToBool(params["peer_conflict"])
	academicDecline := mentalHealthToBool(params["academic_decline"])
	bullyingConcern := mentalHealthToBool(params["bullying_concern"])
	symptoms := parseMentalHealthList(params["symptoms"])

	if !schoolAvoidance && !peerConflict && !academicDecline && !bullyingConcern && len(symptoms) == 0 {
		return "", fmt.Errorf("school stress inputs are required")
	}

	findings := []map[string]interface{}{}
	level := "normal"
	urgency := "routine"

	if schoolAvoidance || academicDecline {
		findings = append(findings, map[string]interface{}{"category": "school_function", "level": "moderate", "title": "存在拒学或学业明显受影响线索", "description": "建议尽快结合情绪、睡眠和安全风险复核"})
		level = mentalHealthMaxLevel(level, "moderate")
		urgency = mentalHealthMaxUrgency(urgency, "expedited")
	}
	if peerConflict || bullyingConcern {
		findings = append(findings, map[string]interface{}{"category": "social_stress", "level": "moderate", "title": "存在同伴冲突或校园欺凌担忧", "description": "需尽快与监护人和学校支持系统共同复核"})
		level = mentalHealthMaxLevel(level, "moderate")
		urgency = mentalHealthMaxUrgency(urgency, "same_day")
	}
	for _, symptom := range symptoms {
		if containsAnyNormalizedMentalHealth([]string{symptom}, "自伤", "离家", "攻击他人", "极端恐惧") {
			findings = append(findings, map[string]interface{}{"category": "school_crisis", "level": "severe", "title": fmt.Sprintf("校园压力红旗：%s", symptom), "description": "建议立即线下评估并启动安全支持"})
			level = mentalHealthMaxLevel(level, "severe")
			urgency = mentalHealthMaxUrgency(urgency, "emergency")
		}
	}
	if ageYears >= 6 && level == "normal" && len(symptoms) > 0 {
		level = mentalHealthMaxLevel(level, "mild")
	}

	return jsonStr(map[string]interface{}{
		"panel":         "school_stress_review",
		"age_years":     ageYears,
		"overall_level": level,
		"urgency":       urgency,
		"findings":      findings,
		"followup":      mentalHealthFollowupAdvice("school", urgency),
	}), nil
}

func (t *MentalHealthClinicTool) sleepEmotionReview(params map[string]interface{}) (string, error) {
	sleepHours := toFloat(params["sleep_hours"])
	insomniaDays := toFloat(params["insomnia_days_per_week"])
	nightmares := mentalHealthToBool(params["nightmares"])
	daytimeIrritability := mentalHealthToBool(params["daytime_irritability"])
	symptoms := parseMentalHealthList(params["symptoms"])

	if sleepHours <= 0 && insomniaDays <= 0 && len(symptoms) == 0 && !nightmares {
		return "", fmt.Errorf("sleep emotion inputs are required")
	}

	findings := []map[string]interface{}{}
	level := "normal"
	urgency := "routine"

	if sleepHours > 0 && sleepHours < 7 {
		findings = append(findings, map[string]interface{}{"category": "sleep_hours", "level": "moderate", "title": fmt.Sprintf("睡眠时长偏少（%.1f 小时）", sleepHours), "description": "需结合入睡困难、白天情绪和学习功能复核"})
		level = mentalHealthMaxLevel(level, "moderate")
		urgency = mentalHealthMaxUrgency(urgency, "expedited")
	}
	if insomniaDays >= 3 {
		findings = append(findings, map[string]interface{}{"category": "insomnia", "level": "moderate", "title": fmt.Sprintf("每周失眠约 %.0f 天", insomniaDays), "description": "提示睡眠问题已较频繁"})
		level = mentalHealthMaxLevel(level, "moderate")
	}
	if nightmares || daytimeIrritability {
		findings = append(findings, map[string]interface{}{"category": "emotion_link", "level": "mild", "title": "睡眠与情绪互相影响线索", "description": "建议结合压力事件、学校状态和安全风险复核"})
		level = mentalHealthMaxLevel(level, "mild")
	}
	for _, symptom := range symptoms {
		if containsAnyNormalizedMentalHealth([]string{symptom}, "幻觉", "极端激越", "自伤", "惊厥") {
			findings = append(findings, map[string]interface{}{"category": "sleep_emotion_red_flag", "level": "severe", "title": fmt.Sprintf("睡眠情绪红旗：%s", symptom), "description": "建议立即线下评估"})
			level = mentalHealthMaxLevel(level, "severe")
			urgency = mentalHealthMaxUrgency(urgency, "emergency")
		}
	}

	return jsonStr(map[string]interface{}{
		"panel":         "sleep_emotion_review",
		"overall_level": level,
		"urgency":       urgency,
		"findings":      findings,
		"followup":      mentalHealthFollowupAdvice("sleep_emotion", urgency),
	}), nil
}

func (t *MentalHealthClinicTool) selfHarmCrisisReview(params map[string]interface{}) (string, error) {
	selfHarmThoughts := mentalHealthToBool(params["self_harm_thoughts"])
	attemptHistory := mentalHealthToBool(params["attempt_history"])
	planAccess := mentalHealthToBool(params["plan_or_means_access"])
	agitation := mentalHealthToBool(params["severe_agitation"])
	psychosis := mentalHealthToBool(params["psychosis_like_symptoms"])
	symptoms := parseMentalHealthList(params["symptoms"])

	if !selfHarmThoughts && !attemptHistory && !planAccess && !agitation && !psychosis && len(symptoms) == 0 {
		return "", fmt.Errorf("crisis review inputs are required")
	}

	findings := []map[string]interface{}{}
	level := "normal"
	urgency := "routine"

	if selfHarmThoughts {
		findings = append(findings, map[string]interface{}{"category": "self_harm", "level": "severe", "title": "存在自伤/轻生想法线索", "description": "需要立即由监护人陪同线下评估安全风险"})
		level = mentalHealthMaxLevel(level, "severe")
		urgency = mentalHealthMaxUrgency(urgency, "emergency")
	}
	if attemptHistory || planAccess {
		findings = append(findings, map[string]interface{}{"category": "high_lethality", "level": "severe", "title": "存在既往尝试史或计划/手段可及性", "description": "提示危机风险高，建议立即线下或急诊处置"})
		level = mentalHealthMaxLevel(level, "severe")
		urgency = mentalHealthMaxUrgency(urgency, "emergency")
	}
	if agitation || psychosis {
		findings = append(findings, map[string]interface{}{"category": "acute_state", "level": "severe", "title": "存在严重激越或精神病性线索", "description": "提示需要立即线下评估与安全保护"})
		level = mentalHealthMaxLevel(level, "severe")
		urgency = mentalHealthMaxUrgency(urgency, "emergency")
	}
	for _, symptom := range symptoms {
		if containsAnyNormalizedMentalHealth([]string{symptom}, "割伤", "吞药", "离家出走", "持刀", "威胁他人") {
			findings = append(findings, map[string]interface{}{"category": "crisis_signal", "level": "severe", "title": fmt.Sprintf("危机信号：%s", symptom), "description": "建议立即线下或急诊评估"})
			level = mentalHealthMaxLevel(level, "severe")
			urgency = mentalHealthMaxUrgency(urgency, "emergency")
		}
	}

	return jsonStr(map[string]interface{}{
		"panel":         "self_harm_crisis_review",
		"overall_level": level,
		"urgency":       urgency,
		"findings":      findings,
		"followup":      mentalHealthFollowupAdvice("crisis", urgency),
	}), nil
}

func parseMentalHealthList(v interface{}) []string {
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
	return uniqueSortedMentalHealthStrings(items)
}

func normalizeMentalHealthTerm(v string) string {
	replacer := strings.NewReplacer(" ", "", "-", "", "_", "", ".", "", "/", "", "（", "(", "）", ")")
	return strings.ToLower(strings.TrimSpace(replacer.Replace(v)))
}

func uniqueSortedMentalHealthStrings(items []string) []string {
	lookup := map[string]string{}
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		key := normalizeMentalHealthTerm(trimmed)
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

func mentalHealthMaxLevel(current, next string) string {
	levels := map[string]int{"normal": 0, "mild": 1, "moderate": 2, "severe": 3}
	if levels[next] > levels[current] {
		return next
	}
	return current
}

func mentalHealthMaxUrgency(current, next string) string {
	levels := map[string]int{"routine": 0, "expedited": 1, "same_day": 2, "emergency": 3}
	if levels[next] > levels[current] {
		return next
	}
	return current
}

func mentalHealthFollowupAdvice(panel, urgency string) string {
	if urgency == "emergency" {
		return "建议立即联系监护人并尽快前往线下急诊/精神心理专科评估，不要仅依赖线上建议。"
	}
	if urgency == "same_day" {
		return "建议当天线下复核，尽量由监护人陪同，并记录近期情绪、睡眠和安全风险变化。"
	}
	switch panel {
	case "school":
		return "建议记录上学意愿、同伴冲突、学业变化和家庭沟通情况，必要时尽快寻求线下支持。"
	case "sleep_emotion":
		return "建议记录入睡、夜醒、噩梦和白天情绪变化，必要时尽快线下复核。"
	default:
		return "建议记录情绪、睡眠、功能受影响情况和安全线索，必要时尽快线下复核。"
	}
}

func containsAnyNormalizedMentalHealth(values []string, patterns ...string) bool {
	for _, value := range values {
		normalized := normalizeMentalHealthTerm(value)
		for _, pattern := range patterns {
			if strings.Contains(normalized, normalizeMentalHealthTerm(pattern)) {
				return true
			}
		}
	}
	return false
}

func mentalHealthToBool(v interface{}) bool {
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
