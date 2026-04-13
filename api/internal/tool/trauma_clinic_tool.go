package tool

import (
	"fmt"
	"sort"
	"strings"

	"gorm.io/gorm"
)

type TraumaClinicTool struct {
	db *gorm.DB
}

func NewTraumaClinicTool(db *gorm.DB) *TraumaClinicTool {
	return &TraumaClinicTool{db: db}
}

func (t *TraumaClinicTool) Execute(userID string, params map[string]interface{}) (string, error) {
	action, _ := params["action"].(string)
	switch action {
	case "head_injury_triage":
		return t.headInjuryTriage(params)
	case "fall_impact_review":
		return t.fallImpactReview(params)
	case "limb_injury_triage":
		return t.limbInjuryTriage(params)
	case "wound_burn_review":
		return t.woundBurnReview(params)
	default:
		return "", fmt.Errorf("unknown trauma_clinic action: %s", action)
	}
}

func (t *TraumaClinicTool) headInjuryTriage(params map[string]interface{}) (string, error) {
	ageYears := toFloat(params["age_years"])
	mechanism, _ := params["mechanism"].(string)
	symptoms := parseTraumaList(params["symptoms"])
	vomitingCount := toFloat(params["vomiting_count"])
	lossOfConsciousness := traumaToBool(params["loss_of_consciousness"])
	mentalStatus, _ := params["mental_status"].(string)
	seizure := traumaToBool(params["seizure"])
	scalpSwelling, _ := params["scalp_swelling"].(string)

	if mechanism == "" && len(symptoms) == 0 && vomitingCount <= 0 {
		return "", fmt.Errorf("mechanism, symptoms, or vomiting_count is required")
	}

	findings := []map[string]interface{}{}
	level := "normal"
	urgency := "routine"

	if seizure {
		findings = append(findings, map[string]interface{}{"category": "neurologic_red_flag", "level": "severe", "title": "头部外伤后抽搐", "description": "建议立即急诊评估"})
		level = traumaMaxLevel(level, "severe")
		urgency = traumaMaxUrgency(urgency, "emergency")
	}
	if lossOfConsciousness {
		findings = append(findings, map[string]interface{}{"category": "consciousness", "level": "severe", "title": "存在意识丧失线索", "description": "建议尽快急诊或当天线下评估"})
		level = traumaMaxLevel(level, "severe")
		urgency = traumaMaxUrgency(urgency, "emergency")
	}
	if vomitingCount >= 2 {
		findings = append(findings, map[string]interface{}{"category": "vomiting", "level": "moderate", "title": fmt.Sprintf("头伤后呕吐 %.0f 次", vomitingCount), "description": "反复呕吐提示需要尽快线下复核"})
		level = traumaMaxLevel(level, "moderate")
		urgency = traumaMaxUrgency(urgency, "same_day")
	}
	if containsAnyNormalizedTrauma([]string{mentalStatus}, "嗜睡", "难唤醒", "意识差", "confused", "lethargic") {
		findings = append(findings, map[string]interface{}{"category": "mental_status", "level": "severe", "title": "精神状态异常", "description": "建议立即急诊评估"})
		level = traumaMaxLevel(level, "severe")
		urgency = traumaMaxUrgency(urgency, "emergency")
	}
	if ageYears > 0 && ageYears < 2 && containsAnyNormalizedTrauma([]string{scalpSwelling}, "有", "明显", "frontal", "parietal", "occipital") {
		findings = append(findings, map[string]interface{}{"category": "infant_scalp", "level": "moderate", "title": "低龄儿童头皮肿胀", "description": "婴幼儿头部外伤建议更积极线下复核"})
		level = traumaMaxLevel(level, "moderate")
		urgency = traumaMaxUrgency(urgency, "same_day")
	}
	if containsAnyNormalizedTrauma([]string{mechanism}, "高处", "坠落", "车祸", "高速", "楼梯", "bike", "motor") {
		findings = append(findings, map[string]interface{}{"category": "mechanism", "level": "moderate", "title": "受伤机制偏高风险", "description": "需结合症状尽快评估是否存在颅脑损伤"})
		level = traumaMaxLevel(level, "moderate")
		urgency = traumaMaxUrgency(urgency, "same_day")
	}
	for _, symptom := range symptoms {
		normalized := normalizeTraumaTerm(symptom)
		switch {
		case containsAnyNormalizedTrauma([]string{normalized}, "抽搐", "嗜睡", "瞳孔", "持续头痛", "行为改变", "流血不止", "耳鼻流液"):
			findings = append(findings, map[string]interface{}{"category": "head_red_flag", "level": "severe", "title": fmt.Sprintf("头伤红旗：%s", symptom), "description": "建议立即急诊评估"})
			level = traumaMaxLevel(level, "severe")
			urgency = traumaMaxUrgency(urgency, "emergency")
		case containsAnyNormalizedTrauma([]string{normalized}, "头痛", "头晕", "哭闹", "恶心"):
			findings = append(findings, map[string]interface{}{"category": "head_symptom", "level": "mild", "title": fmt.Sprintf("伴随症状：%s", symptom), "description": "建议继续观察是否加重"})
			level = traumaMaxLevel(level, "mild")
		}
	}

	return jsonStr(map[string]interface{}{
		"panel":         "head_injury_triage",
		"age_years":     ageYears,
		"overall_level": level,
		"urgency":       urgency,
		"findings":      findings,
		"followup":      traumaFollowupAdvice("head", urgency),
	}), nil
}

func (t *TraumaClinicTool) fallImpactReview(params map[string]interface{}) (string, error) {
	ageYears := toFloat(params["age_years"])
	fallHeightCm := toFloat(params["fall_height_cm"])
	landingSurface, _ := params["landing_surface"].(string)
	bodyRegions := parseTraumaList(params["body_regions"])
	symptoms := parseTraumaList(params["symptoms"])
	ableToWalk := traumaToBool(params["able_to_walk"])
	behaviorChange, _ := params["behavior_change"].(string)

	if fallHeightCm <= 0 && len(bodyRegions) == 0 && len(symptoms) == 0 {
		return "", fmt.Errorf("fall_height_cm, body_regions, or symptoms is required")
	}

	findings := []map[string]interface{}{}
	level := "normal"
	urgency := "routine"

	if fallHeightCm >= 150 {
		findings = append(findings, map[string]interface{}{"category": "fall_height", "level": "severe", "title": fmt.Sprintf("跌落高度较高（%.0f cm）", fallHeightCm), "description": "建议尽快线下评估潜在头部、胸腹部或骨伤"})
		level = traumaMaxLevel(level, "severe")
		urgency = traumaMaxUrgency(urgency, "same_day")
	} else if fallHeightCm >= 60 {
		findings = append(findings, map[string]interface{}{"category": "fall_height", "level": "moderate", "title": fmt.Sprintf("存在中高风险跌落（%.0f cm）", fallHeightCm), "description": "建议结合受力部位和症状尽快复核"})
		level = traumaMaxLevel(level, "moderate")
		urgency = traumaMaxUrgency(urgency, "expedited")
	}
	if containsAnyNormalizedTrauma([]string{landingSurface}, "水泥", "台阶", "硬地", "metal", "sharp") {
		findings = append(findings, map[string]interface{}{"category": "surface", "level": "mild", "title": "落地表面较硬", "description": "提示冲击风险增加"})
		level = traumaMaxLevel(level, "mild")
	}
	if !ableToWalk {
		findings = append(findings, map[string]interface{}{"category": "mobility", "level": "moderate", "title": "受伤后不能正常行走/站立", "description": "建议当天线下评估骨伤或疼痛原因"})
		level = traumaMaxLevel(level, "moderate")
		urgency = traumaMaxUrgency(urgency, "same_day")
	}
	if containsAnyNormalizedTrauma([]string{behaviorChange}, "有", "明显", "嗜睡", "烦躁", "异常") {
		findings = append(findings, map[string]interface{}{"category": "behavior", "level": "severe", "title": "跌落后行为改变", "description": "建议尽快头部和神经系统评估"})
		level = traumaMaxLevel(level, "severe")
		urgency = traumaMaxUrgency(urgency, "emergency")
	}
	for _, region := range bodyRegions {
		if containsAnyNormalizedTrauma([]string{region}, "头", "颈", "胸", "腹") {
			findings = append(findings, map[string]interface{}{"category": "impact_region", "level": "moderate", "title": fmt.Sprintf("主要受力部位：%s", region), "description": "该部位受力建议更积极线下复核"})
			level = traumaMaxLevel(level, "moderate")
			urgency = traumaMaxUrgency(urgency, "expedited")
		}
	}
	for _, symptom := range symptoms {
		normalized := normalizeTraumaTerm(symptom)
		switch {
		case containsAnyNormalizedTrauma([]string{normalized}, "呼吸困难", "腹痛加重", "意识差", "呕吐", "抽搐"):
			findings = append(findings, map[string]interface{}{"category": "fall_red_flag", "level": "severe", "title": fmt.Sprintf("跌落红旗：%s", symptom), "description": "建议立即急诊评估"})
			level = traumaMaxLevel(level, "severe")
			urgency = traumaMaxUrgency(urgency, "emergency")
		case containsAnyNormalizedTrauma([]string{normalized}, "疼痛", "瘀青", "哭闹"):
			level = traumaMaxLevel(level, "mild")
		}
	}
	if ageYears > 0 && ageYears < 1 && fallHeightCm > 0 {
		urgency = traumaMaxUrgency(urgency, "same_day")
	}

	return jsonStr(map[string]interface{}{
		"panel":          "fall_impact_review",
		"age_years":      ageYears,
		"fall_height_cm": fallHeightCm,
		"overall_level":  level,
		"urgency":        urgency,
		"findings":       findings,
		"followup":       traumaFollowupAdvice("fall", urgency),
	}), nil
}

func (t *TraumaClinicTool) limbInjuryTriage(params map[string]interface{}) (string, error) {
	injuredLimb, _ := params["injured_limb"].(string)
	swelling, _ := params["swelling"].(string)
	painScore := toFloat(params["pain_score"])
	deformity := traumaToBool(params["deformity"])
	canBearWeight := traumaToBool(params["can_bear_weight"])
	neurovascular := parseTraumaList(params["neurovascular_symptoms"])
	symptoms := parseTraumaList(params["symptoms"])

	if injuredLimb == "" && painScore <= 0 && len(symptoms) == 0 {
		return "", fmt.Errorf("injured_limb, pain_score, or symptoms is required")
	}

	findings := []map[string]interface{}{}
	level := "normal"
	urgency := "routine"

	if deformity {
		findings = append(findings, map[string]interface{}{"category": "deformity", "level": "severe", "title": "存在明显畸形", "description": "提示骨折或脱位风险，建议尽快线下评估"})
		level = traumaMaxLevel(level, "severe")
		urgency = traumaMaxUrgency(urgency, "same_day")
	}
	if painScore >= 7 {
		findings = append(findings, map[string]interface{}{"category": "pain", "level": "moderate", "title": fmt.Sprintf("疼痛评分较高（%.0f/10）", painScore), "description": "建议尽快线下复核是否存在骨伤"})
		level = traumaMaxLevel(level, "moderate")
		urgency = traumaMaxUrgency(urgency, "expedited")
	}
	if containsAnyNormalizedTrauma([]string{swelling}, "明显", "快速加重", "严重") {
		findings = append(findings, map[string]interface{}{"category": "swelling", "level": "moderate", "title": "肿胀较明显", "description": "需结合疼痛、活动受限和畸形判断"})
		level = traumaMaxLevel(level, "moderate")
	}
	if !canBearWeight && containsAnyNormalizedTrauma([]string{injuredLimb}, "腿", "膝", "踝", "足", "hip") {
		findings = append(findings, map[string]interface{}{"category": "weight_bearing", "level": "moderate", "title": "下肢受伤后不能负重", "description": "建议当天线下评估"})
		level = traumaMaxLevel(level, "moderate")
		urgency = traumaMaxUrgency(urgency, "same_day")
	}
	if len(neurovascular) > 0 || containsAnyNormalizedTrauma(neurovascular, "麻木", "冰冷", "发白", "无脉", "刺痛") {
		findings = append(findings, map[string]interface{}{"category": "neurovascular", "level": "severe", "title": "存在神经血运受累线索", "description": "建议立即线下评估"})
		level = traumaMaxLevel(level, "severe")
		urgency = traumaMaxUrgency(urgency, "emergency")
	}
	for _, symptom := range symptoms {
		if containsAnyNormalizedTrauma([]string{symptom}, "活动受限", "压痛", "卡住", "弹响") {
			findings = append(findings, map[string]interface{}{"category": "limb_symptom", "level": "mild", "title": fmt.Sprintf("伴随线索：%s", symptom), "description": "建议结合疼痛、负重和畸形进一步判断"})
			level = traumaMaxLevel(level, "mild")
		}
	}

	return jsonStr(map[string]interface{}{
		"panel":         "limb_injury_triage",
		"injured_limb":  injuredLimb,
		"pain_score":    painScore,
		"overall_level": level,
		"urgency":       urgency,
		"findings":      findings,
		"followup":      traumaFollowupAdvice("limb", urgency),
	}), nil
}

func (t *TraumaClinicTool) woundBurnReview(params map[string]interface{}) (string, error) {
	injuryType, _ := params["injury_type"].(string)
	burnAreaPercent := toFloat(params["burn_area_percent"])
	woundDepth, _ := params["wound_depth"].(string)
	location, _ := params["location"].(string)
	bleedingStatus, _ := params["bleeding_status"].(string)
	symptoms := parseTraumaList(params["symptoms"])

	if injuryType == "" && burnAreaPercent <= 0 && location == "" && len(symptoms) == 0 {
		return "", fmt.Errorf("injury_type, burn_area_percent, location, or symptoms is required")
	}

	findings := []map[string]interface{}{}
	level := "normal"
	urgency := "routine"

	if containsAnyNormalizedTrauma([]string{bleedingStatus}, "无法止血", "喷射", "持续大量", "soaking") {
		findings = append(findings, map[string]interface{}{"category": "bleeding", "level": "severe", "title": "出血控制困难", "description": "建议立即线下或急诊处理"})
		level = traumaMaxLevel(level, "severe")
		urgency = traumaMaxUrgency(urgency, "emergency")
	}
	if burnAreaPercent >= 10 {
		findings = append(findings, map[string]interface{}{"category": "burn_area", "level": "severe", "title": fmt.Sprintf("烧烫伤面积较大（%.0f%%）", burnAreaPercent), "description": "建议尽快线下处理"})
		level = traumaMaxLevel(level, "severe")
		urgency = traumaMaxUrgency(urgency, "same_day")
	} else if burnAreaPercent >= 3 {
		findings = append(findings, map[string]interface{}{"category": "burn_area", "level": "moderate", "title": fmt.Sprintf("存在一定面积烧烫伤（%.0f%%）", burnAreaPercent), "description": "建议尽快复核深度和处理方式"})
		level = traumaMaxLevel(level, "moderate")
		urgency = traumaMaxUrgency(urgency, "expedited")
	}
	if containsAnyNormalizedTrauma([]string{woundDepth}, "深", "焦痂", "发白", "起大疱", "full") {
		findings = append(findings, map[string]interface{}{"category": "depth", "level": "moderate", "title": "伤口/烧伤深度可疑偏深", "description": "建议尽快线下复核"})
		level = traumaMaxLevel(level, "moderate")
		urgency = traumaMaxUrgency(urgency, "same_day")
	}
	if containsAnyNormalizedTrauma([]string{location}, "脸", "面", "眼", "口", "手", "足", "会阴", "关节") {
		findings = append(findings, map[string]interface{}{"category": "location", "level": "moderate", "title": "受伤部位较特殊", "description": "面部、手足、会阴或关节附近建议更积极线下评估"})
		level = traumaMaxLevel(level, "moderate")
		urgency = traumaMaxUrgency(urgency, "same_day")
	}
	for _, symptom := range symptoms {
		switch {
		case containsAnyNormalizedTrauma([]string{symptom}, "发热", "红肿加重", "恶臭", "黑色", "麻木"):
			findings = append(findings, map[string]interface{}{"category": "wound_red_flag", "level": "moderate", "title": fmt.Sprintf("伤口红旗：%s", symptom), "description": "提示感染或组织损伤风险，建议尽快复诊"})
			level = traumaMaxLevel(level, "moderate")
			urgency = traumaMaxUrgency(urgency, "same_day")
		case containsAnyNormalizedTrauma([]string{symptom}, "疼痛", "水疱", "渗液"):
			level = traumaMaxLevel(level, "mild")
		}
	}

	return jsonStr(map[string]interface{}{
		"panel":             "wound_burn_review",
		"injury_type":       injuryType,
		"burn_area_percent": burnAreaPercent,
		"overall_level":     level,
		"urgency":           urgency,
		"findings":          findings,
		"followup":          traumaFollowupAdvice("wound", urgency),
	}), nil
}

func parseTraumaList(v interface{}) []string {
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
	return uniqueSortedTraumaStrings(items)
}

func normalizeTraumaTerm(v string) string {
	replacer := strings.NewReplacer(" ", "", "-", "", "_", "", ".", "", "/", "", "（", "(", "）", ")")
	return strings.ToLower(strings.TrimSpace(replacer.Replace(v)))
}

func uniqueSortedTraumaStrings(items []string) []string {
	lookup := map[string]string{}
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		key := normalizeTraumaTerm(trimmed)
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

func traumaMaxLevel(current, next string) string {
	levels := map[string]int{"normal": 0, "mild": 1, "moderate": 2, "severe": 3}
	if levels[next] > levels[current] {
		return next
	}
	return current
}

func traumaMaxUrgency(current, next string) string {
	levels := map[string]int{"routine": 0, "expedited": 1, "same_day": 2, "emergency": 3}
	if levels[next] > levels[current] {
		return next
	}
	return current
}

func traumaFollowupAdvice(panel, urgency string) string {
	if urgency == "emergency" {
		return "建议立即线下就医或急诊评估，不要仅依赖线上建议。"
	}
	if urgency == "same_day" {
		return "建议当天线下就医，携带受伤时间、受伤机制、症状变化和处理经过。"
	}
	switch panel {
	case "limb":
		return "建议记录疼痛、肿胀、活动度和负重变化，必要时尽快复诊拍片复核。"
	case "wound":
		return "建议记录伤口范围、渗液、疼痛和发热变化，必要时尽快换药复诊。"
	default:
		return "建议记录受伤机制、精神状态、呕吐/疼痛变化和家庭观察结果，必要时尽快复诊。"
	}
}

func containsAnyNormalizedTrauma(values []string, patterns ...string) bool {
	for _, value := range values {
		normalized := normalizeTraumaTerm(value)
		for _, pattern := range patterns {
			if strings.Contains(normalized, normalizeTraumaTerm(pattern)) {
				return true
			}
		}
	}
	return false
}

func traumaToBool(v interface{}) bool {
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
