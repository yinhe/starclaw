package tool

import (
	"fmt"
	"sort"
	"strings"

	"gorm.io/gorm"
)

type WellnessClinicTool struct {
	db *gorm.DB
}

func NewWellnessClinicTool(db *gorm.DB) *WellnessClinicTool {
	return &WellnessClinicTool{db: db}
}

func (t *WellnessClinicTool) Execute(userID string, params map[string]interface{}) (string, error) {
	action, _ := params["action"].(string)
	switch action {
	case "sleep_habit_review":
		return t.sleepHabitReview(params)
	case "nutrition_intake_review":
		return t.nutritionIntakeReview(params)
	case "activity_screen_review":
		return t.activityScreenReview(params)
	case "lifestyle_risk_review":
		return t.lifestyleRiskReview(params)
	default:
		return "", fmt.Errorf("unknown wellness_clinic action: %s", action)
	}
}

func (t *WellnessClinicTool) sleepHabitReview(params map[string]interface{}) (string, error) {
	ageYears := toFloat(params["age_years"])
	sleepHours := toFloat(params["sleep_hours"])
	nightAwakenings := toFloat(params["night_awakenings_per_week"])
	snoring := wellnessToBool(params["snoring"])
	witnessedApnea := wellnessToBool(params["witnessed_apnea"])
	daytimeSleepiness := wellnessToBool(params["daytime_sleepiness"])
	symptoms := parseWellnessList(params["symptoms"])

	if sleepHours <= 0 && len(symptoms) == 0 && !snoring && !witnessedApnea {
		return "", fmt.Errorf("sleep review inputs are required")
	}

	findings := []map[string]interface{}{}
	level := "normal"
	urgency := "routine"

	recommendedSleep := 0.0
	switch {
	case ageYears > 0 && ageYears < 3:
		recommendedSleep = 11
	case ageYears >= 3 && ageYears < 6:
		recommendedSleep = 10
	case ageYears >= 6 && ageYears < 13:
		recommendedSleep = 9
	case ageYears >= 13:
		recommendedSleep = 8
	}

	if recommendedSleep > 0 && sleepHours > 0 && sleepHours < recommendedSleep {
		findings = append(findings, map[string]interface{}{"category": "sleep_duration", "level": "moderate", "title": fmt.Sprintf("睡眠时长偏少（%.1f 小时）", sleepHours), "description": "建议结合年龄、日间功能和作息习惯进一步复核"})
		level = wellnessMaxLevel(level, "moderate")
		urgency = wellnessMaxUrgency(urgency, "expedited")
	}
	if nightAwakenings >= 4 {
		findings = append(findings, map[string]interface{}{"category": "sleep_fragmentation", "level": "mild", "title": fmt.Sprintf("每周夜醒约 %.0f 次", nightAwakenings), "description": "提示睡眠连续性受影响，需结合打鼾和白天状态判断"})
		level = wellnessMaxLevel(level, "mild")
	}
	if snoring {
		findings = append(findings, map[string]interface{}{"category": "snoring", "level": "mild", "title": "存在打鼾线索", "description": "需结合张口呼吸、憋醒和白天困倦判断睡眠呼吸问题风险"})
		level = wellnessMaxLevel(level, "mild")
	}
	if witnessedApnea {
		findings = append(findings, map[string]interface{}{"category": "apnea", "level": "severe", "title": "存在睡眠中呼吸暂停线索", "description": "建议尽快线下评估睡眠呼吸问题"})
		level = wellnessMaxLevel(level, "severe")
		urgency = wellnessMaxUrgency(urgency, "same_day")
	}
	if daytimeSleepiness {
		findings = append(findings, map[string]interface{}{"category": "daytime_function", "level": "moderate", "title": "白天困倦或功能受影响", "description": "提示睡眠质量已影响日间学习或活动"})
		level = wellnessMaxLevel(level, "moderate")
		urgency = wellnessMaxUrgency(urgency, "expedited")
	}
	for _, symptom := range symptoms {
		if containsAnyNormalizedWellness([]string{symptom}, "发绀", "难唤醒", "抽搐", "严重头痛") {
			findings = append(findings, map[string]interface{}{"category": "sleep_red_flag", "level": "severe", "title": fmt.Sprintf("睡眠红旗：%s", symptom), "description": "建议立即线下评估"})
			level = wellnessMaxLevel(level, "severe")
			urgency = wellnessMaxUrgency(urgency, "emergency")
		}
	}

	return jsonStr(map[string]interface{}{
		"panel":         "sleep_habit_review",
		"age_years":     ageYears,
		"overall_level": level,
		"urgency":       urgency,
		"findings":      findings,
		"followup":      wellnessFollowupAdvice("sleep", urgency),
	}), nil
}

func (t *WellnessClinicTool) nutritionIntakeReview(params map[string]interface{}) (string, error) {
	ageYears := toFloat(params["age_years"])
	mealsPerDay := toFloat(params["meals_per_day"])
	foodVarietyScore := toFloat(params["food_variety_score"])
	poorAppetite := wellnessToBool(params["poor_appetite"])
	weightLoss := wellnessToBool(params["weight_loss"])
	feedingDifficulty := wellnessToBool(params["feeding_difficulty"])
	symptoms := parseWellnessList(params["symptoms"])

	if mealsPerDay <= 0 && foodVarietyScore <= 0 && len(symptoms) == 0 && !poorAppetite {
		return "", fmt.Errorf("nutrition review inputs are required")
	}

	findings := []map[string]interface{}{}
	level := "normal"
	urgency := "routine"

	if mealsPerDay > 0 && mealsPerDay < 3 {
		findings = append(findings, map[string]interface{}{"category": "meal_pattern", "level": "moderate", "title": fmt.Sprintf("每日进食次数偏少（%.0f 次）", mealsPerDay), "description": "建议结合年龄、加餐、病程和体重变化继续复核"})
		level = wellnessMaxLevel(level, "moderate")
		urgency = wellnessMaxUrgency(urgency, "expedited")
	}
	if foodVarietyScore > 0 && foodVarietyScore <= 3 {
		findings = append(findings, map[string]interface{}{"category": "diet_variety", "level": "mild", "title": "饮食种类较单一", "description": "提示营养均衡风险，建议结合偏食和生长情况评估"})
		level = wellnessMaxLevel(level, "mild")
	}
	if poorAppetite || feedingDifficulty {
		findings = append(findings, map[string]interface{}{"category": "intake_issue", "level": "moderate", "title": "存在食欲差或进食困难线索", "description": "需关注病程、脱水和体重变化"})
		level = wellnessMaxLevel(level, "moderate")
		urgency = wellnessMaxUrgency(urgency, "expedited")
	}
	if weightLoss {
		findings = append(findings, map[string]interface{}{"category": "weight_change", "level": "severe", "title": "存在体重下降线索", "description": "建议尽快线下复核营养和基础疾病因素"})
		level = wellnessMaxLevel(level, "severe")
		urgency = wellnessMaxUrgency(urgency, "same_day")
	}
	for _, symptom := range symptoms {
		normalized := normalizeWellnessTerm(symptom)
		switch {
		case containsAnyNormalizedWellness([]string{normalized}, "持续呕吐", "吞咽困难", "便血", "黑便", "明显脱水"):
			findings = append(findings, map[string]interface{}{"category": "nutrition_red_flag", "level": "severe", "title": fmt.Sprintf("营养红旗：%s", symptom), "description": "建议立即线下评估"})
			level = wellnessMaxLevel(level, "severe")
			urgency = wellnessMaxUrgency(urgency, "emergency")
		case containsAnyNormalizedWellness([]string{normalized}, "偏食", "挑食", "腹痛", "便秘"):
			level = wellnessMaxLevel(level, "mild")
		}
	}
	if ageYears > 0 && ageYears < 2 && level != "normal" {
		urgency = wellnessMaxUrgency(urgency, "same_day")
	}

	return jsonStr(map[string]interface{}{
		"panel":         "nutrition_intake_review",
		"age_years":     ageYears,
		"overall_level": level,
		"urgency":       urgency,
		"findings":      findings,
		"followup":      wellnessFollowupAdvice("nutrition", urgency),
	}), nil
}

func (t *WellnessClinicTool) activityScreenReview(params map[string]interface{}) (string, error) {
	ageYears := toFloat(params["age_years"])
	dailyActivityMinutes := toFloat(params["daily_activity_minutes"])
	screenTimeHours := toFloat(params["screen_time_hours"])
	outdoorDays := toFloat(params["outdoor_days_per_week"])
	exerciseLimitation := wellnessToBool(params["exercise_limitation"])
	symptoms := parseWellnessList(params["symptoms"])

	if dailyActivityMinutes <= 0 && screenTimeHours <= 0 && len(symptoms) == 0 {
		return "", fmt.Errorf("activity review inputs are required")
	}

	findings := []map[string]interface{}{}
	level := "normal"
	urgency := "routine"

	if dailyActivityMinutes > 0 && dailyActivityMinutes < 60 {
		findings = append(findings, map[string]interface{}{"category": "activity", "level": "moderate", "title": fmt.Sprintf("每日活动时间偏少（%.0f 分钟）", dailyActivityMinutes), "description": "提示活动不足，建议结合体重、睡眠和日常作息复核"})
		level = wellnessMaxLevel(level, "moderate")
	}
	if screenTimeHours >= 3 {
		findings = append(findings, map[string]interface{}{"category": "screen", "level": "mild", "title": fmt.Sprintf("屏幕时间偏长（%.1f 小时/日）", screenTimeHours), "description": "需结合睡眠、体力活动和学习节律综合判断"})
		level = wellnessMaxLevel(level, "mild")
	}
	if outdoorDays > 0 && outdoorDays <= 2 {
		findings = append(findings, map[string]interface{}{"category": "outdoor", "level": "mild", "title": "户外活动频率偏少", "description": "建议适度增加户外和规律运动安排"})
		level = wellnessMaxLevel(level, "mild")
	}
	if exerciseLimitation {
		findings = append(findings, map[string]interface{}{"category": "exercise_limitation", "level": "moderate", "title": "存在运动受限线索", "description": "建议结合气道症状、关节疼痛或心血管线索进一步复核"})
		level = wellnessMaxLevel(level, "moderate")
		urgency = wellnessMaxUrgency(urgency, "expedited")
	}
	for _, symptom := range symptoms {
		if containsAnyNormalizedWellness([]string{symptom}, "胸痛", "晕厥", "呼吸困难", "发绀") {
			findings = append(findings, map[string]interface{}{"category": "exercise_red_flag", "level": "severe", "title": fmt.Sprintf("运动相关红旗：%s", symptom), "description": "建议立即线下评估"})
			level = wellnessMaxLevel(level, "severe")
			urgency = wellnessMaxUrgency(urgency, "emergency")
		}
	}

	return jsonStr(map[string]interface{}{
		"panel":         "activity_screen_review",
		"age_years":     ageYears,
		"overall_level": level,
		"urgency":       urgency,
		"findings":      findings,
		"followup":      wellnessFollowupAdvice("activity", urgency),
	}), nil
}

func (t *WellnessClinicTool) lifestyleRiskReview(params map[string]interface{}) (string, error) {
	ageYears := toFloat(params["age_years"])
	sugaryDrinkDaily := toFloat(params["sugary_drink_daily"])
	fastFoodWeekly := toFloat(params["fast_food_weekly"])
	bedtimeRegular := wellnessToBool(params["bedtime_regular"])
	sleepHours := toFloat(params["sleep_hours"])
	symptoms := parseWellnessList(params["symptoms"])

	if sugaryDrinkDaily <= 0 && fastFoodWeekly <= 0 && sleepHours <= 0 && len(symptoms) == 0 {
		return "", fmt.Errorf("lifestyle risk inputs are required")
	}

	findings := []map[string]interface{}{}
	level := "normal"
	urgency := "routine"

	if sugaryDrinkDaily >= 2 {
		findings = append(findings, map[string]interface{}{"category": "dietary_sugar", "level": "moderate", "title": fmt.Sprintf("含糖饮料摄入偏多（%.0f 次/日）", sugaryDrinkDaily), "description": "提示代谢和体重管理风险升高，建议结合 BMI 和家庭习惯复核"})
		level = wellnessMaxLevel(level, "moderate")
	}
	if fastFoodWeekly >= 3 {
		findings = append(findings, map[string]interface{}{"category": "fast_food", "level": "mild", "title": fmt.Sprintf("快餐摄入偏多（%.0f 次/周）", fastFoodWeekly), "description": "建议复核饮食结构和家庭执行情况"})
		level = wellnessMaxLevel(level, "mild")
	}
	if !bedtimeRegular && sleepHours > 0 {
		findings = append(findings, map[string]interface{}{"category": "bedtime", "level": "mild", "title": "作息规律性不足", "description": "提示睡眠节律可能紊乱"})
		level = wellnessMaxLevel(level, "mild")
	}
	if sleepHours > 0 && sleepHours < 8 {
		findings = append(findings, map[string]interface{}{"category": "insufficient_sleep", "level": "moderate", "title": fmt.Sprintf("睡眠时长偏少（%.1f 小时）", sleepHours), "description": "提示生活方式整体风险增加"})
		level = wellnessMaxLevel(level, "moderate")
		urgency = wellnessMaxUrgency(urgency, "expedited")
	}
	for _, symptom := range symptoms {
		if containsAnyNormalizedWellness([]string{symptom}, "明显体重增加", "高血压", "黑棘皮", "严重头痛") {
			findings = append(findings, map[string]interface{}{"category": "metabolic_red_flag", "level": "moderate", "title": fmt.Sprintf("生活方式相关风险线索：%s", symptom), "description": "建议近期线下复核代谢或血压等指标"})
			level = wellnessMaxLevel(level, "moderate")
			urgency = wellnessMaxUrgency(urgency, "expedited")
		}
	}
	if ageYears >= 12 && level != "normal" {
		urgency = wellnessMaxUrgency(urgency, "expedited")
	}

	return jsonStr(map[string]interface{}{
		"panel":         "lifestyle_risk_review",
		"age_years":     ageYears,
		"overall_level": level,
		"urgency":       urgency,
		"findings":      findings,
		"followup":      wellnessFollowupAdvice("lifestyle", urgency),
	}), nil
}

func parseWellnessList(v interface{}) []string {
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
	return uniqueSortedWellnessStrings(items)
}

func normalizeWellnessTerm(v string) string {
	replacer := strings.NewReplacer(" ", "", "-", "", "_", "", ".", "", "/", "", "（", "(", "）", ")")
	return strings.ToLower(strings.TrimSpace(replacer.Replace(v)))
}

func uniqueSortedWellnessStrings(items []string) []string {
	lookup := map[string]string{}
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		key := normalizeWellnessTerm(trimmed)
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

func wellnessMaxLevel(current, next string) string {
	levels := map[string]int{"normal": 0, "mild": 1, "moderate": 2, "severe": 3}
	if levels[next] > levels[current] {
		return next
	}
	return current
}

func wellnessMaxUrgency(current, next string) string {
	levels := map[string]int{"routine": 0, "expedited": 1, "same_day": 2, "emergency": 3}
	if levels[next] > levels[current] {
		return next
	}
	return current
}

func wellnessFollowupAdvice(panel, urgency string) string {
	if urgency == "emergency" {
		return "建议立即线下就医或急诊评估，不要仅依赖线上建议。"
	}
	if urgency == "same_day" {
		return "建议当天线下复核，携带睡眠、饮食、活动和症状记录。"
	}
	switch panel {
	case "nutrition":
		return "建议记录进食量、饮水、排便和体重变化，必要时尽快复诊。"
	case "activity":
		return "建议记录活动时间、屏幕时间和运动相关不适，必要时尽快线下复核。"
	default:
		return "建议记录作息、饮食、活动和家庭执行情况变化，必要时尽快线下复核。"
	}
}

func containsAnyNormalizedWellness(values []string, patterns ...string) bool {
	for _, value := range values {
		normalized := normalizeWellnessTerm(value)
		for _, pattern := range patterns {
			if strings.Contains(normalized, normalizeWellnessTerm(pattern)) {
				return true
			}
		}
	}
	return false
}

func wellnessToBool(v interface{}) bool {
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
