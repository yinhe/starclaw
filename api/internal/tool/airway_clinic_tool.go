package tool

import (
	"fmt"
	"sort"
	"strings"

	"gorm.io/gorm"
)

type AirwayClinicTool struct {
	db *gorm.DB
}

func NewAirwayClinicTool(db *gorm.DB) *AirwayClinicTool {
	return &AirwayClinicTool{db: db}
}

func (t *AirwayClinicTool) Execute(userID string, params map[string]interface{}) (string, error) {
	action, _ := params["action"].(string)
	switch action {
	case "chronic_cough_triage":
		return t.chronicCoughTriage(params)
	case "asthma_control_review":
		return t.asthmaControlReview(params)
	case "wheeze_risk_review":
		return t.wheezeRiskReview(params)
	case "allergic_rhinitis_review":
		return t.allergicRhinitisReview(params)
	default:
		return "", fmt.Errorf("unknown airway_clinic action: %s", action)
	}
}

func (t *AirwayClinicTool) chronicCoughTriage(params map[string]interface{}) (string, error) {
	ageYears := toFloat(params["age_years"])
	coughWeeks := toFloat(params["cough_duration_weeks"])
	fever := toFloat(params["temperature_c"])
	nocturnalCough := airwayToBool(params["nocturnal_cough"])
	exerciseTrigger := airwayToBool(params["exercise_trigger"])
	weightLoss := airwayToBool(params["weight_loss"])
	symptoms := parseAirwayList(params["symptoms"])

	if coughWeeks <= 0 && len(symptoms) == 0 {
		return "", fmt.Errorf("cough_duration_weeks or symptoms is required")
	}

	findings := []map[string]interface{}{}
	level := "normal"
	urgency := "routine"

	if coughWeeks >= 4 {
		findings = append(findings, map[string]interface{}{"category": "duration", "level": "moderate", "title": fmt.Sprintf("咳嗽持续 %.0f 周", coughWeeks), "description": "慢性咳嗽建议线下复核诱因、听诊和必要检查"})
		level = airwayMaxLevel(level, "moderate")
		urgency = airwayMaxUrgency(urgency, "expedited")
	}
	if nocturnalCough {
		findings = append(findings, map[string]interface{}{"category": "nocturnal", "level": "mild", "title": "存在夜间咳嗽", "description": "提示气道高反应性、鼻后滴流或反流等可能"})
		level = airwayMaxLevel(level, "mild")
	}
	if exerciseTrigger {
		findings = append(findings, map[string]interface{}{"category": "exercise", "level": "moderate", "title": "运动诱发咳嗽", "description": "提示喘息或哮喘样气道问题可能，需要进一步复核"})
		level = airwayMaxLevel(level, "moderate")
		urgency = airwayMaxUrgency(urgency, "expedited")
	}
	if fever >= 39 {
		findings = append(findings, map[string]interface{}{"category": "fever", "level": "moderate", "title": fmt.Sprintf("咳嗽伴高热 %.1f℃", fever), "description": "建议线下复核感染性原因和呼吸状态"})
		level = airwayMaxLevel(level, "moderate")
		urgency = airwayMaxUrgency(urgency, "same_day")
	}
	if weightLoss {
		findings = append(findings, map[string]interface{}{"category": "systemic", "level": "severe", "title": "咳嗽伴体重下降线索", "description": "建议尽快专科或线下进一步评估"})
		level = airwayMaxLevel(level, "severe")
		urgency = airwayMaxUrgency(urgency, "same_day")
	}
	for _, symptom := range symptoms {
		normalized := normalizeAirwayTerm(symptom)
		switch {
		case containsAnyNormalizedAirway([]string{normalized}, "咯血", "发绀", "呼吸困难", "胸痛", "喘不过气", "低氧"):
			findings = append(findings, map[string]interface{}{"category": "red_flag", "level": "severe", "title": fmt.Sprintf("慢性咳嗽红旗：%s", symptom), "description": "建议立即线下或急诊评估"})
			level = airwayMaxLevel(level, "severe")
			urgency = airwayMaxUrgency(urgency, "emergency")
		case containsAnyNormalizedAirway([]string{normalized}, "喘息", "鼻塞", "清嗓", "反酸", "后鼻滴流"):
			findings = append(findings, map[string]interface{}{"category": "associated", "level": "mild", "title": fmt.Sprintf("伴随线索：%s", symptom), "description": "提示需结合鼻炎、哮喘或反流等因素综合判断"})
			level = airwayMaxLevel(level, "mild")
		}
	}
	if ageYears > 0 && ageYears < 1 && coughWeeks >= 2 {
		urgency = airwayMaxUrgency(urgency, "same_day")
	}

	return jsonStr(map[string]interface{}{
		"panel":                "chronic_cough_triage",
		"age_years":            ageYears,
		"cough_duration_weeks": coughWeeks,
		"overall_level":        level,
		"urgency":              urgency,
		"findings":             findings,
		"followup":             airwayFollowupAdvice("cough", urgency),
	}), nil
}

func (t *AirwayClinicTool) asthmaControlReview(params map[string]interface{}) (string, error) {
	daytimeDays := toFloat(params["daytime_symptom_days_per_week"])
	nighttimeNights := toFloat(params["nighttime_symptom_nights_per_month"])
	relieverDays := toFloat(params["reliever_days_per_week"])
	activityLimitation, _ := params["activity_limitation"].(string)
	oralSteroidCourses := toFloat(params["recent_oral_steroid_courses"])
	edVisits := toFloat(params["recent_ed_visits"])
	symptoms := parseAirwayList(params["symptoms"])

	if daytimeDays <= 0 && nighttimeNights <= 0 && relieverDays <= 0 && len(symptoms) == 0 {
		return "", fmt.Errorf("control indicators or symptoms are required")
	}

	findings := []map[string]interface{}{}
	level := "normal"
	urgency := "routine"
	control := "well_controlled"

	if daytimeDays > 2 {
		findings = append(findings, map[string]interface{}{"category": "daytime", "level": "moderate", "title": fmt.Sprintf("每周白天症状 %.0f 天", daytimeDays), "description": "提示控制可能不足"})
		level = airwayMaxLevel(level, "moderate")
		urgency = airwayMaxUrgency(urgency, "expedited")
		control = "partly_controlled"
	}
	if nighttimeNights >= 2 {
		findings = append(findings, map[string]interface{}{"category": "nighttime", "level": "moderate", "title": fmt.Sprintf("每月夜间症状 %.0f 次", nighttimeNights), "description": "夜间症状提示控制欠佳"})
		level = airwayMaxLevel(level, "moderate")
		urgency = airwayMaxUrgency(urgency, "expedited")
		control = "partly_controlled"
	}
	if relieverDays > 2 {
		findings = append(findings, map[string]interface{}{"category": "reliever", "level": "moderate", "title": fmt.Sprintf("每周缓解药使用 %.0f 天", relieverDays), "description": "缓解药使用偏频繁，建议复核长期控制方案"})
		level = airwayMaxLevel(level, "moderate")
		urgency = airwayMaxUrgency(urgency, "expedited")
		control = "partly_controlled"
	}
	if containsAnyNormalizedAirway([]string{activityLimitation}, "有", "明显", "不能运动", "影响", "poor") {
		findings = append(findings, map[string]interface{}{"category": "activity", "level": "moderate", "title": "活动受限", "description": "提示控制欠佳，需要线下复核"})
		level = airwayMaxLevel(level, "moderate")
		urgency = airwayMaxUrgency(urgency, "same_day")
		control = "partly_controlled"
	}
	if oralSteroidCourses >= 1 || edVisits >= 1 {
		findings = append(findings, map[string]interface{}{"category": "exacerbation_history", "level": "severe", "title": "近期存在急性加重线索", "description": "提示高风险，建议尽快复核控制策略"})
		level = airwayMaxLevel(level, "severe")
		urgency = airwayMaxUrgency(urgency, "same_day")
		control = "poorly_controlled"
	}
	for _, symptom := range symptoms {
		if containsAnyNormalizedAirway([]string{symptom}, "呼吸困难", "喘息加重", "说话困难", "发绀") {
			findings = append(findings, map[string]interface{}{"category": "asthma_red_flag", "level": "severe", "title": fmt.Sprintf("哮喘红旗：%s", symptom), "description": "建议立即线下或急诊评估"})
			level = airwayMaxLevel(level, "severe")
			urgency = airwayMaxUrgency(urgency, "emergency")
			control = "poorly_controlled"
		}
	}

	return jsonStr(map[string]interface{}{
		"panel":         "asthma_control_review",
		"control":       control,
		"overall_level": level,
		"urgency":       urgency,
		"findings":      findings,
		"followup":      airwayFollowupAdvice("asthma", urgency),
	}), nil
}

func (t *AirwayClinicTool) wheezeRiskReview(params map[string]interface{}) (string, error) {
	ageYears := toFloat(params["age_years"])
	episodes := toFloat(params["wheeze_episodes_12m"])
	triggers := parseAirwayList(params["triggers"])
	eczemaOrAllergy := airwayToBool(params["eczema_or_allergy"])
	familyHistory := parseAirwayList(params["family_history"])
	symptoms := parseAirwayList(params["symptoms"])

	if episodes <= 0 && len(triggers) == 0 && len(symptoms) == 0 {
		return "", fmt.Errorf("wheeze_episodes_12m, triggers, or symptoms is required")
	}

	findings := []map[string]interface{}{}
	level := "normal"
	urgency := "routine"

	if episodes >= 3 {
		findings = append(findings, map[string]interface{}{"category": "recurrence", "level": "moderate", "title": fmt.Sprintf("近 12 个月喘息发作 %.0f 次", episodes), "description": "反复喘息建议尽快复核诱因和长期管理"})
		level = airwayMaxLevel(level, "moderate")
		urgency = airwayMaxUrgency(urgency, "expedited")
	}
	if eczemaOrAllergy {
		findings = append(findings, map[string]interface{}{"category": "atopy", "level": "mild", "title": "存在湿疹或过敏体质线索", "description": "提示哮喘样气道问题风险可能升高"})
		level = airwayMaxLevel(level, "mild")
	}
	if containsAnyNormalizedAirway(familyHistory, "哮喘", "过敏", "鼻炎", "atopy") {
		findings = append(findings, map[string]interface{}{"category": "family_history", "level": "mild", "title": "存在过敏/哮喘家族史", "description": "提示反复喘息风险增加"})
		level = airwayMaxLevel(level, "mild")
	}
	for _, trigger := range triggers {
		if containsAnyNormalizedAirway([]string{trigger}, "运动", "夜间", "冷空气", "感冒", "过敏原") {
			findings = append(findings, map[string]interface{}{"category": "trigger", "level": "mild", "title": fmt.Sprintf("常见诱因：%s", trigger), "description": "建议结合诱因回避和规范随访"})
			level = airwayMaxLevel(level, "mild")
		}
	}
	for _, symptom := range symptoms {
		if containsAnyNormalizedAirway([]string{symptom}, "呼吸困难", "胸凹", "发绀", "说话困难", "低氧") {
			findings = append(findings, map[string]interface{}{"category": "wheeze_red_flag", "level": "severe", "title": fmt.Sprintf("喘息红旗：%s", symptom), "description": "建议立即线下或急诊评估"})
			level = airwayMaxLevel(level, "severe")
			urgency = airwayMaxUrgency(urgency, "emergency")
		} else if containsAnyNormalizedAirway([]string{symptom}, "咳嗽", "喘鸣", "胸闷") {
			level = airwayMaxLevel(level, "mild")
		}
	}
	if ageYears > 0 && ageYears < 3 && episodes >= 2 {
		urgency = airwayMaxUrgency(urgency, "expedited")
	}

	return jsonStr(map[string]interface{}{
		"panel":               "wheeze_risk_review",
		"age_years":           ageYears,
		"wheeze_episodes_12m": episodes,
		"overall_level":       level,
		"urgency":             urgency,
		"findings":            findings,
		"followup":            airwayFollowupAdvice("wheeze", urgency),
	}), nil
}

func (t *AirwayClinicTool) allergicRhinitisReview(params map[string]interface{}) (string, error) {
	symptoms := parseAirwayList(params["symptoms"])
	seasonality, _ := params["seasonality"].(string)
	sleepImpact, _ := params["sleep_impact"].(string)
	eyeSymptoms := airwayToBool(params["eye_symptoms"])
	snoring := airwayToBool(params["snoring"])
	mouthBreathing := airwayToBool(params["mouth_breathing"])
	schoolImpact, _ := params["school_impact"].(string)

	if len(symptoms) == 0 && seasonality == "" {
		return "", fmt.Errorf("symptoms or seasonality is required")
	}

	findings := []map[string]interface{}{}
	level := "normal"
	urgency := "routine"

	if len(symptoms) > 0 {
		findings = append(findings, map[string]interface{}{"category": "symptoms", "level": "mild", "title": "存在鼻炎相关症状", "description": "建议结合季节性、眼症状和睡眠影响综合判断"})
		level = airwayMaxLevel(level, "mild")
	}
	if seasonality != "" {
		findings = append(findings, map[string]interface{}{"category": "seasonality", "level": "mild", "title": fmt.Sprintf("季节性线索：%s", seasonality), "description": "提示过敏性鼻炎可能，需要结合环境诱因分析"})
		level = airwayMaxLevel(level, "mild")
	}
	if eyeSymptoms {
		findings = append(findings, map[string]interface{}{"category": "eye", "level": "mild", "title": "伴眼部过敏线索", "description": "提示过敏性结膜炎或上气道过敏共病可能"})
		level = airwayMaxLevel(level, "mild")
	}
	if snoring || mouthBreathing {
		findings = append(findings, map[string]interface{}{"category": "sleep_breathing", "level": "moderate", "title": "存在打鼾或张口呼吸", "description": "建议线下复核鼻阻塞、腺样体或睡眠影响"})
		level = airwayMaxLevel(level, "moderate")
		urgency = airwayMaxUrgency(urgency, "expedited")
	}
	if containsAnyNormalizedAirway([]string{sleepImpact}, "明显", "严重", "差", "poor") || containsAnyNormalizedAirway([]string{schoolImpact}, "明显", "影响", "注意力差", "poor") {
		findings = append(findings, map[string]interface{}{"category": "function", "level": "moderate", "title": "已影响睡眠或日常功能", "description": "建议近期线下复核控制策略"})
		level = airwayMaxLevel(level, "moderate")
		urgency = airwayMaxUrgency(urgency, "expedited")
	}
	for _, symptom := range symptoms {
		if containsAnyNormalizedAirway([]string{symptom}, "鼻塞", "打喷嚏", "流清涕", "鼻痒", "清嗓") {
			level = airwayMaxLevel(level, "mild")
		}
	}

	return jsonStr(map[string]interface{}{
		"panel":         "allergic_rhinitis_review",
		"overall_level": level,
		"urgency":       urgency,
		"findings":      findings,
		"followup":      airwayFollowupAdvice("rhinitis", urgency),
	}), nil
}

func parseAirwayList(v interface{}) []string {
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
	return uniqueSortedAirwayStrings(items)
}

func normalizeAirwayTerm(v string) string {
	replacer := strings.NewReplacer(" ", "", "-", "", "_", "", ".", "", "/", "", "（", "(", "）", ")")
	return strings.ToLower(strings.TrimSpace(replacer.Replace(v)))
}

func uniqueSortedAirwayStrings(items []string) []string {
	lookup := map[string]string{}
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		key := normalizeAirwayTerm(trimmed)
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

func airwayMaxLevel(current, next string) string {
	levels := map[string]int{"normal": 0, "mild": 1, "moderate": 2, "severe": 3}
	if levels[next] > levels[current] {
		return next
	}
	return current
}

func airwayMaxUrgency(current, next string) string {
	levels := map[string]int{"routine": 0, "expedited": 1, "same_day": 2, "emergency": 3}
	if levels[next] > levels[current] {
		return next
	}
	return current
}

func airwayFollowupAdvice(panel, urgency string) string {
	if urgency == "emergency" {
		return "建议立即线下就医或急诊评估，不要仅依赖线上建议。"
	}
	if urgency == "same_day" {
		return "建议当天线下就医，携带症状日记、既往用药和诱因记录。"
	}
	switch panel {
	case "asthma":
		return "建议记录白天/夜间症状、缓解药使用、活动受限和诱因变化，按需尽快复诊。"
	case "rhinitis":
		return "建议记录鼻塞、喷嚏、睡眠和环境诱因变化，必要时尽快线下复核。"
	default:
		return "建议记录咳嗽/喘息频率、夜间症状、运动诱发和环境诱因变化，按需尽快复诊。"
	}
}

func containsAnyNormalizedAirway(values []string, patterns ...string) bool {
	for _, value := range values {
		normalized := normalizeAirwayTerm(value)
		for _, pattern := range patterns {
			if strings.Contains(normalized, normalizeAirwayTerm(pattern)) {
				return true
			}
		}
	}
	return false
}

func airwayToBool(v interface{}) bool {
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
