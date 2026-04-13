package tool

import (
	"fmt"
	"sort"
	"strings"

	"gorm.io/gorm"
)

type NeurodevClinicTool struct {
	db *gorm.DB
}

func NewNeurodevClinicTool(db *gorm.DB) *NeurodevClinicTool {
	return &NeurodevClinicTool{db: db}
}

func (t *NeurodevClinicTool) Execute(userID string, params map[string]interface{}) (string, error) {
	action, _ := params["action"].(string)
	switch action {
	case "developmental_milestone_review":
		return t.developmentalMilestoneReview(params)
	case "language_delay_screen":
		return t.languageDelayScreen(params)
	case "autism_flag_screen":
		return t.autismFlagScreen(params)
	case "attention_behavior_screen":
		return t.attentionBehaviorScreen(params)
	default:
		return "", fmt.Errorf("unknown neurodev_clinic action: %s", action)
	}
}

func (t *NeurodevClinicTool) developmentalMilestoneReview(params map[string]interface{}) (string, error) {
	ageMonths := toFloat(params["age_months"])
	missing := parseNeurodevList(params["missing_milestones"])
	symptoms := parseNeurodevList(params["symptoms"])
	motorConcerns, _ := params["motor_concerns"].(string)
	regression := toBool(params["regression"])

	if ageMonths <= 0 && len(missing) == 0 && len(symptoms) == 0 {
		return "", fmt.Errorf("age_months, missing_milestones, or symptoms is required")
	}

	findings := []map[string]interface{}{}
	level := "normal"
	urgency := "routine"

	if regression {
		findings = append(findings, map[string]interface{}{"category": "regression", "level": "severe", "title": "存在发育倒退线索", "description": "建议尽快发育行为儿科或儿童神经专科线下评估"})
		level = neurodevMaxLevel(level, "severe")
		urgency = neurodevMaxUrgency(urgency, "same_day")
	}
	if len(missing) >= 3 {
		findings = append(findings, map[string]interface{}{"category": "milestones", "level": "moderate", "title": fmt.Sprintf("缺失多个发育里程碑（%d 项）", len(missing)), "description": "建议尽快做系统发育评估"})
		level = neurodevMaxLevel(level, "moderate")
		urgency = neurodevMaxUrgency(urgency, "expedited")
	} else if len(missing) > 0 {
		findings = append(findings, map[string]interface{}{"category": "milestones", "level": "mild", "title": "存在发育里程碑落后线索", "description": "建议结合年龄和家庭观察继续复核"})
		level = neurodevMaxLevel(level, "mild")
	}
	for _, item := range missing {
		normalized := normalizeNeurodevTerm(item)
		switch {
		case ageMonths >= 9 && containsAnyNormalizedNeurodev([]string{normalized}, "不会坐", "不能独坐", "notsit"):
			findings = append(findings, map[string]interface{}{"category": "gross_motor", "level": "severe", "title": "9 月龄后仍不会独坐", "description": "建议尽快线下评估粗大运动发育"})
			level = neurodevMaxLevel(level, "severe")
			urgency = neurodevMaxUrgency(urgency, "same_day")
		case ageMonths >= 18 && containsAnyNormalizedNeurodev([]string{normalized}, "不会走", "不能独走", "notwalk"):
			findings = append(findings, map[string]interface{}{"category": "gross_motor", "level": "severe", "title": "18 月龄后仍不会独走", "description": "建议尽快线下评估发育与神经系统情况"})
			level = neurodevMaxLevel(level, "severe")
			urgency = neurodevMaxUrgency(urgency, "same_day")
		case ageMonths >= 12 && containsAnyNormalizedNeurodev([]string{normalized}, "不会指物", "不模仿", "无应答手势"):
			findings = append(findings, map[string]interface{}{"category": "social_communication", "level": "moderate", "title": "社交沟通里程碑偏晚", "description": "建议进一步筛查语言和社交沟通发育"})
			level = neurodevMaxLevel(level, "moderate")
			urgency = neurodevMaxUrgency(urgency, "expedited")
		}
	}
	if containsAnyNormalizedNeurodev([]string{motorConcerns}, "偏瘫", "肌张力", "僵硬", "不对称", "toe", "尖足") {
		findings = append(findings, map[string]interface{}{"category": "motor_pattern", "level": "moderate", "title": "存在运动模式异常线索", "description": "建议儿保或儿童神经专科复核"})
		level = neurodevMaxLevel(level, "moderate")
		urgency = neurodevMaxUrgency(urgency, "expedited")
	}
	for _, symptom := range symptoms {
		normalized := normalizeNeurodevTerm(symptom)
		if containsAnyNormalizedNeurodev([]string{normalized}, "抽搐", "意识差", "头围倒退", "持续头痛", "走路明显退步") {
			findings = append(findings, map[string]interface{}{"category": "red_flag", "level": "severe", "title": fmt.Sprintf("神经发育红旗：%s", symptom), "description": "建议立即线下评估"})
			level = neurodevMaxLevel(level, "severe")
			urgency = neurodevMaxUrgency(urgency, "emergency")
		}
	}

	return jsonStr(map[string]interface{}{
		"panel":         "developmental_milestone_review",
		"age_months":    ageMonths,
		"overall_level": level,
		"urgency":       urgency,
		"findings":      findings,
		"followup":      neurodevFollowupAdvice("developmental", urgency),
	}), nil
}

func (t *NeurodevClinicTool) languageDelayScreen(params map[string]interface{}) (string, error) {
	ageMonths := toFloat(params["age_months"])
	spokenWords := toFloat(params["spoken_words"])
	twoWordPhrases := toBool(params["two_word_phrases"])
	hearingConcern, _ := params["hearing_concern"].(string)
	symptoms := parseNeurodevList(params["symptoms"])
	regression := toBool(params["regression"])

	if ageMonths <= 0 && spokenWords <= 0 && len(symptoms) == 0 {
		return "", fmt.Errorf("age_months, spoken_words, or symptoms is required")
	}

	findings := []map[string]interface{}{}
	level := "normal"
	urgency := "routine"

	if regression {
		findings = append(findings, map[string]interface{}{"category": "regression", "level": "severe", "title": "语言能力出现倒退线索", "description": "建议尽快线下评估听力与发育行为情况"})
		level = neurodevMaxLevel(level, "severe")
		urgency = neurodevMaxUrgency(urgency, "same_day")
	}
	if ageMonths >= 18 && spokenWords < 10 {
		findings = append(findings, map[string]interface{}{"category": "expressive_language", "level": "moderate", "title": fmt.Sprintf("18 月龄后词汇量偏少（约 %.0f 个）", spokenWords), "description": "提示语言表达可能偏慢，建议筛查听力和语言发育"})
		level = neurodevMaxLevel(level, "moderate")
		urgency = neurodevMaxUrgency(urgency, "expedited")
	}
	if ageMonths >= 24 && !twoWordPhrases {
		findings = append(findings, map[string]interface{}{"category": "phrase_language", "level": "severe", "title": "24 月龄后仍缺乏双词短语", "description": "建议尽快做语言发育和社交沟通评估"})
		level = neurodevMaxLevel(level, "severe")
		urgency = neurodevMaxUrgency(urgency, "same_day")
	}
	if containsAnyNormalizedNeurodev([]string{hearingConcern}, "有", "异常", "听不见", "反应差", "poor") {
		findings = append(findings, map[string]interface{}{"category": "hearing", "level": "moderate", "title": "存在听力相关担忧", "description": "建议尽快先做听力评估，再结合语言发育判断"})
		level = neurodevMaxLevel(level, "moderate")
		urgency = neurodevMaxUrgency(urgency, "expedited")
	}
	for _, symptom := range symptoms {
		normalized := normalizeNeurodevTerm(symptom)
		switch {
		case containsAnyNormalizedNeurodev([]string{normalized}, "不回应名字", "无眼神", "不会指物", "不会模仿"):
			findings = append(findings, map[string]interface{}{"category": "social_communication", "level": "moderate", "title": fmt.Sprintf("社交沟通线索：%s", symptom), "description": "建议结合 ASD 红旗进一步筛查"})
			level = neurodevMaxLevel(level, "moderate")
			urgency = neurodevMaxUrgency(urgency, "expedited")
		case containsAnyNormalizedNeurodev([]string{normalized}, "抽搐", "明显退步", "失去已会说的词"):
			findings = append(findings, map[string]interface{}{"category": "language_red_flag", "level": "severe", "title": fmt.Sprintf("语言红旗：%s", symptom), "description": "建议立即线下评估"})
			level = neurodevMaxLevel(level, "severe")
			urgency = neurodevMaxUrgency(urgency, "emergency")
		}
	}

	return jsonStr(map[string]interface{}{
		"panel":            "language_delay_screen",
		"age_months":       ageMonths,
		"spoken_words":     spokenWords,
		"two_word_phrases": twoWordPhrases,
		"overall_level":    level,
		"urgency":          urgency,
		"findings":         findings,
		"followup":         neurodevFollowupAdvice("language", urgency),
	}), nil
}

func (t *NeurodevClinicTool) autismFlagScreen(params map[string]interface{}) (string, error) {
	ageMonths := toFloat(params["age_months"])
	socialFlags := parseNeurodevList(params["social_flags"])
	repetitive := parseNeurodevList(params["repetitive_behaviors"])
	languageFlags := parseNeurodevList(params["language_flags"])
	regression := toBool(params["regression"])

	if ageMonths <= 0 && len(socialFlags) == 0 && len(repetitive) == 0 && len(languageFlags) == 0 {
		return "", fmt.Errorf("age_months or autism-related flags are required")
	}

	findings := []map[string]interface{}{}
	level := "normal"
	urgency := "routine"
	flagCount := len(socialFlags) + len(repetitive) + len(languageFlags)

	if regression {
		findings = append(findings, map[string]interface{}{"category": "regression", "level": "severe", "title": "存在社交或语言倒退线索", "description": "建议尽快发育行为儿科线下评估"})
		level = neurodevMaxLevel(level, "severe")
		urgency = neurodevMaxUrgency(urgency, "same_day")
	}
	if flagCount >= 4 {
		findings = append(findings, map[string]interface{}{"category": "screen_positive", "level": "severe", "title": fmt.Sprintf("ASD 红旗较多（%d 项）", flagCount), "description": "建议尽快做标准化发育行为筛查与专科评估"})
		level = neurodevMaxLevel(level, "severe")
		urgency = neurodevMaxUrgency(urgency, "same_day")
	} else if flagCount >= 2 {
		findings = append(findings, map[string]interface{}{"category": "screen_positive", "level": "moderate", "title": fmt.Sprintf("存在多项 ASD 红旗（%d 项）", flagCount), "description": "建议近期完成进一步筛查"})
		level = neurodevMaxLevel(level, "moderate")
		urgency = neurodevMaxUrgency(urgency, "expedited")
	}
	if ageMonths >= 18 && containsAnyNormalizedNeurodev(socialFlags, "不回应名字", "无共同关注", "不看人") {
		findings = append(findings, map[string]interface{}{"category": "social_core_flag", "level": "moderate", "title": "核心社交线索异常", "description": "建议结合语言和重复行为做进一步 ASD 筛查"})
		level = neurodevMaxLevel(level, "moderate")
	}
	if containsAnyNormalizedNeurodev(repetitive, "刻板动作", "转圈", "排列", "拍手", "强烈固定兴趣") {
		findings = append(findings, map[string]interface{}{"category": "repetitive_behavior", "level": "mild", "title": "存在重复刻板行为线索", "description": "需结合社交沟通情况整体判断"})
		level = neurodevMaxLevel(level, "mild")
	}
	if containsAnyNormalizedNeurodev(languageFlags, "无功能语言", "重复语言", "不会指物", "不会表达需求") {
		findings = append(findings, map[string]interface{}{"category": "language_related_flag", "level": "moderate", "title": "存在语言相关红旗", "description": "建议结合语言筛查和听力评估"})
		level = neurodevMaxLevel(level, "moderate")
	}

	return jsonStr(map[string]interface{}{
		"panel":         "autism_flag_screen",
		"age_months":    ageMonths,
		"flag_count":    flagCount,
		"overall_level": level,
		"urgency":       urgency,
		"findings":      findings,
		"followup":      neurodevFollowupAdvice("autism", urgency),
	}), nil
}

func (t *NeurodevClinicTool) attentionBehaviorScreen(params map[string]interface{}) (string, error) {
	ageYears := toFloat(params["age_years"])
	settingsAffected := toFloat(params["settings_affected"])
	symptoms := parseNeurodevList(params["symptoms"])
	schoolImpact, _ := params["school_impact"].(string)
	sleepIssue, _ := params["sleep_issue"].(string)
	safetyConcerns := parseNeurodevList(params["safety_concerns"])

	if ageYears <= 0 && len(symptoms) == 0 && settingsAffected <= 0 {
		return "", fmt.Errorf("age_years, symptoms, or settings_affected is required")
	}

	findings := []map[string]interface{}{}
	level := "normal"
	urgency := "routine"

	if settingsAffected >= 2 && len(symptoms) >= 4 {
		findings = append(findings, map[string]interface{}{"category": "cross_setting", "level": "moderate", "title": "注意力/行为问题涉及多个场景", "description": "提示需要更系统的行为评估"})
		level = neurodevMaxLevel(level, "moderate")
		urgency = neurodevMaxUrgency(urgency, "expedited")
	} else if len(symptoms) > 0 {
		findings = append(findings, map[string]interface{}{"category": "behavior", "level": "mild", "title": "存在注意力或行为线索", "description": "建议结合家庭和学校观察继续记录"})
		level = neurodevMaxLevel(level, "mild")
	}
	if containsAnyNormalizedNeurodev([]string{schoolImpact}, "明显", "学习困难", "被投诉", "退步", "严重") {
		findings = append(findings, map[string]interface{}{"category": "school_impact", "level": "moderate", "title": "已影响学习或课堂功能", "description": "建议近期发育行为或儿童心理门诊复核"})
		level = neurodevMaxLevel(level, "moderate")
		urgency = neurodevMaxUrgency(urgency, "expedited")
	}
	if containsAnyNormalizedNeurodev([]string{sleepIssue}, "有", "明显", "入睡困难", "打鼾", "poor") {
		findings = append(findings, map[string]interface{}{"category": "sleep", "level": "mild", "title": "存在睡眠相关线索", "description": "睡眠问题可加重注意力和行为表现，建议同步关注"})
		level = neurodevMaxLevel(level, "mild")
	}
	if len(safetyConcerns) > 0 {
		findings = append(findings, map[string]interface{}{"category": "safety", "level": "severe", "title": "存在安全风险线索", "description": "如冲动离家、明显攻击、自伤或危险行为，建议尽快线下评估"})
		level = neurodevMaxLevel(level, "severe")
		urgency = neurodevMaxUrgency(urgency, "same_day")
	}
	if ageYears > 0 && ageYears < 4 && len(symptoms) > 0 {
		findings = append(findings, map[string]interface{}{"category": "age_context", "level": "mild", "title": "学龄前需结合年龄阶段判断", "description": "不宜仅凭单次线上描述下结论，建议连续观察"})
		level = neurodevMaxLevel(level, "mild")
	}

	return jsonStr(map[string]interface{}{
		"panel":             "attention_behavior_screen",
		"age_years":         ageYears,
		"settings_affected": settingsAffected,
		"overall_level":     level,
		"urgency":           urgency,
		"findings":          findings,
		"followup":          neurodevFollowupAdvice("attention", urgency),
	}), nil
}

func parseNeurodevList(v interface{}) []string {
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
	return uniqueSortedNeurodevStrings(items)
}

func normalizeNeurodevTerm(v string) string {
	replacer := strings.NewReplacer(" ", "", "-", "", "_", "", ".", "", "/", "", "（", "(", "）", ")")
	return strings.ToLower(strings.TrimSpace(replacer.Replace(v)))
}

func uniqueSortedNeurodevStrings(items []string) []string {
	lookup := map[string]string{}
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		key := normalizeNeurodevTerm(trimmed)
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

func neurodevMaxLevel(current, next string) string {
	levels := map[string]int{"normal": 0, "mild": 1, "moderate": 2, "severe": 3}
	if levels[next] > levels[current] {
		return next
	}
	return current
}

func neurodevMaxUrgency(current, next string) string {
	levels := map[string]int{"routine": 0, "expedited": 1, "same_day": 2, "emergency": 3}
	if levels[next] > levels[current] {
		return next
	}
	return current
}

func neurodevFollowupAdvice(panel, urgency string) string {
	if urgency == "emergency" {
		return "建议立即线下就医或急诊评估，不要仅依赖线上建议。"
	}
	if urgency == "same_day" {
		return "建议当天线下评估，携带年龄、病程、家庭观察和既往发育记录。"
	}
	switch panel {
	case "language":
		return "建议记录词汇量、叫名反应、指物/模仿表现，并尽快结合听力与语言发育评估。"
	case "autism":
		return "建议记录眼神、共同关注、叫名反应、重复行为与语言变化，尽快完成标准化筛查。"
	case "attention":
		return "建议同步收集家庭和学校观察，记录持续时间、影响场景、睡眠与安全风险。"
	default:
		return "建议按年龄连续记录发育表现、已掌握与未掌握里程碑，并按需尽快复诊。"
	}
}

func containsAnyNormalizedNeurodev(values []string, patterns ...string) bool {
	for _, value := range values {
		normalized := normalizeNeurodevTerm(value)
		for _, pattern := range patterns {
			if strings.Contains(normalized, normalizeNeurodevTerm(pattern)) {
				return true
			}
		}
	}
	return false
}

func toBool(v interface{}) bool {
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
