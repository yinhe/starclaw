package tool

import (
	"fmt"
	"sort"
	"strings"

	"gorm.io/gorm"
)

type DiagnosticsClinicTool struct {
	db *gorm.DB
}

func NewDiagnosticsClinicTool(db *gorm.DB) *DiagnosticsClinicTool {
	return &DiagnosticsClinicTool{db: db}
}

func (t *DiagnosticsClinicTool) Execute(userID string, params map[string]interface{}) (string, error) {
	action, _ := params["action"].(string)
	switch action {
	case "chest_imaging_review":
		return t.chestImagingReview(params)
	case "fracture_imaging_review":
		return t.fractureImagingReview(params)
	case "abdominal_imaging_review":
		return t.abdominalImagingReview(params)
	case "pathology_report_review":
		return t.pathologyReportReview(params)
	default:
		return "", fmt.Errorf("unknown diagnostics_clinic action: %s", action)
	}
}

func (t *DiagnosticsClinicTool) chestImagingReview(params map[string]interface{}) (string, error) {
	ageYears := toFloat(params["age_years"])
	modality, _ := params["modality"].(string)
	impression, _ := params["impression"].(string)
	findingsInput := parseDiagnosticsList(params["findings"])
	symptoms := parseDiagnosticsList(params["symptoms"])

	if strings.TrimSpace(impression) == "" && len(findingsInput) == 0 && len(symptoms) == 0 {
		return "", fmt.Errorf("chest imaging impression, findings, or symptoms are required")
	}

	findings := []map[string]interface{}{}
	level := "normal"
	urgency := "routine"
	imagingSignals := append([]string{impression, modality}, findingsInput...)

	if containsAnyNormalizedDiagnostics(imagingSignals, "气胸", "pneumothorax", "大量胸腔积液", "large pleural effusion", "纵隔增宽", "mediastinal widening") {
		findings = append(findings, map[string]interface{}{"category": "critical_imaging", "level": "severe", "title": "胸部影像存在高风险线索", "description": "提示需要立即结合呼吸循环情况做线下或急诊评估"})
		level = diagnosticsMaxLevel(level, "severe")
		urgency = diagnosticsMaxUrgency(urgency, "emergency")
	}
	if containsAnyNormalizedDiagnostics(imagingSignals, "肺实变", "consolidation", "肺炎", "infiltrate", "不张", "atelectasis", "胸腔积液", "pleural effusion") {
		findings = append(findings, map[string]interface{}{"category": "infectious_or_airway", "level": "moderate", "title": "胸片提示感染或通气异常线索", "description": "建议结合发热、呼吸频率、血氧和精神状态进一步复核"})
		level = diagnosticsMaxLevel(level, "moderate")
		urgency = diagnosticsMaxUrgency(urgency, "same_day")
	}
	if containsAnyNormalizedDiagnostics(imagingSignals, "过度充气", "hyperinflation", "支气管周围增厚", "peribronchial thickening") {
		findings = append(findings, map[string]interface{}{"category": "airway_pattern", "level": "mild", "title": "影像存在气道高反应线索", "description": "需结合喘息、慢性咳嗽或哮喘控制情况综合判断"})
		level = diagnosticsMaxLevel(level, "mild")
	}
	for _, symptom := range symptoms {
		normalized := normalizeDiagnosticsTerm(symptom)
		switch {
		case containsAnyNormalizedDiagnostics([]string{normalized}, "呼吸困难", "发绀", "胸痛", "不能完整说话", "低氧"):
			findings = append(findings, map[string]interface{}{"category": "clinical_red_flag", "level": "severe", "title": fmt.Sprintf("胸部影像相关红旗：%s", symptom), "description": "建议立即线下或急诊评估"})
			level = diagnosticsMaxLevel(level, "severe")
			urgency = diagnosticsMaxUrgency(urgency, "emergency")
		case containsAnyNormalizedDiagnostics([]string{normalized}, "发热", "咳嗽", "喘息"):
			level = diagnosticsMaxLevel(level, "mild")
		}
	}
	if ageYears > 0 && ageYears < 1 && level != "normal" {
		urgency = diagnosticsMaxUrgency(urgency, "same_day")
	}

	return jsonStr(map[string]interface{}{
		"panel":         "chest_imaging_review",
		"age_years":     ageYears,
		"modality":      strings.TrimSpace(modality),
		"overall_level": level,
		"urgency":       urgency,
		"findings":      findings,
		"followup":      diagnosticsFollowupAdvice("chest_imaging", urgency),
	}), nil
}

func (t *DiagnosticsClinicTool) fractureImagingReview(params map[string]interface{}) (string, error) {
	ageYears := toFloat(params["age_years"])
	injurySite, _ := params["injury_site"].(string)
	impression, _ := params["impression"].(string)
	swelling := diagnosticsToBool(params["swelling"])
	deformity := diagnosticsToBool(params["deformity"])
	neurovascularConcern := diagnosticsToBool(params["neurovascular_concern"])
	symptoms := parseDiagnosticsList(params["symptoms"])

	if strings.TrimSpace(impression) == "" && len(symptoms) == 0 && strings.TrimSpace(injurySite) == "" {
		return "", fmt.Errorf("fracture imaging impression, injury_site, or symptoms are required")
	}

	findings := []map[string]interface{}{}
	level := "normal"
	urgency := "routine"
	imagingSignals := append([]string{impression, injurySite}, symptoms...)

	if containsAnyNormalizedDiagnostics(imagingSignals, "骨折", "fracture", "脱位", "dislocation", "移位", "displaced") {
		findings = append(findings, map[string]interface{}{"category": "fracture", "level": "severe", "title": "影像提示骨折或脱位线索", "description": "建议尽快线下骨科或急诊评估固定与进一步处理"})
		level = diagnosticsMaxLevel(level, "severe")
		urgency = diagnosticsMaxUrgency(urgency, "same_day")
	}
	if containsAnyNormalizedDiagnostics(imagingSignals, "可疑骨折", "possible fracture", "骨皮质不连续", "buckle", "torus", "生长板", "physeal") {
		findings = append(findings, map[string]interface{}{"category": "possible_fracture", "level": "moderate", "title": "影像存在可疑骨伤线索", "description": "建议近期线下复核必要时重复影像或骨科评估"})
		level = diagnosticsMaxLevel(level, "moderate")
		urgency = diagnosticsMaxUrgency(urgency, "expedited")
	}
	if swelling || deformity {
		findings = append(findings, map[string]interface{}{"category": "exam", "level": "moderate", "title": "伴明显肿胀或畸形", "description": "需结合负重能力、疼痛和影像综合判断"})
		level = diagnosticsMaxLevel(level, "moderate")
		urgency = diagnosticsMaxUrgency(urgency, "same_day")
	}
	if neurovascularConcern {
		findings = append(findings, map[string]interface{}{"category": "neurovascular", "level": "severe", "title": "存在神经血运受累线索", "description": "建议立即线下或急诊处理"})
		level = diagnosticsMaxLevel(level, "severe")
		urgency = diagnosticsMaxUrgency(urgency, "emergency")
	}
	for _, symptom := range symptoms {
		if containsAnyNormalizedDiagnostics([]string{symptom}, "麻木", "发凉", "不能活动", "剧痛") {
			findings = append(findings, map[string]interface{}{"category": "fracture_red_flag", "level": "severe", "title": fmt.Sprintf("骨伤红旗：%s", symptom), "description": "建议立即线下评估"})
			level = diagnosticsMaxLevel(level, "severe")
			urgency = diagnosticsMaxUrgency(urgency, "emergency")
		}
	}
	if ageYears > 0 && ageYears < 5 && level != "normal" {
		urgency = diagnosticsMaxUrgency(urgency, "same_day")
	}

	return jsonStr(map[string]interface{}{
		"panel":         "fracture_imaging_review",
		"age_years":     ageYears,
		"injury_site":   strings.TrimSpace(injurySite),
		"overall_level": level,
		"urgency":       urgency,
		"findings":      findings,
		"followup":      diagnosticsFollowupAdvice("fracture_imaging", urgency),
	}), nil
}

func (t *DiagnosticsClinicTool) abdominalImagingReview(params map[string]interface{}) (string, error) {
	ageYears := toFloat(params["age_years"])
	modality, _ := params["modality"].(string)
	painLocation, _ := params["pain_location"].(string)
	impression, _ := params["impression"].(string)
	vomiting := diagnosticsToBool(params["vomiting"])
	fever := toFloat(params["temperature_c"])
	symptoms := parseDiagnosticsList(params["symptoms"])

	if strings.TrimSpace(impression) == "" && len(symptoms) == 0 && strings.TrimSpace(painLocation) == "" {
		return "", fmt.Errorf("abdominal imaging impression, pain_location, or symptoms are required")
	}

	findings := []map[string]interface{}{}
	level := "normal"
	urgency := "routine"
	imagingSignals := append([]string{impression, modality, painLocation}, symptoms...)

	if containsAnyNormalizedDiagnostics(imagingSignals, "阑尾炎", "appendicitis", "肠套叠", "intussusception", "肠梗阻", "obstruction", "腹腔游离气体", "free air") {
		findings = append(findings, map[string]interface{}{"category": "surgical_signal", "level": "severe", "title": "腹部影像提示外科急腹症线索", "description": "建议立即线下或急诊评估"})
		level = diagnosticsMaxLevel(level, "severe")
		urgency = diagnosticsMaxUrgency(urgency, "emergency")
	}
	if containsAnyNormalizedDiagnostics(imagingSignals, "肠系膜淋巴结", "mesenteric adenitis", "便秘", "stool burden", "轻度积气", "gas distention") {
		findings = append(findings, map[string]interface{}{"category": "common_pattern", "level": "mild", "title": "腹部影像存在常见非特异线索", "description": "建议结合腹痛病程、排便和红旗症状综合判断"})
		level = diagnosticsMaxLevel(level, "mild")
	}
	if vomiting || fever >= 39 {
		findings = append(findings, map[string]interface{}{"category": "clinical_context", "level": "moderate", "title": "伴呕吐或高热", "description": "提示需要更积极结合影像和临床情况评估"})
		level = diagnosticsMaxLevel(level, "moderate")
		urgency = diagnosticsMaxUrgency(urgency, "same_day")
	}
	for _, symptom := range symptoms {
		if containsAnyNormalizedDiagnostics([]string{symptom}, "黑便", "血便", "胆汁性呕吐", "反跳痛", "腹胀明显") {
			findings = append(findings, map[string]interface{}{"category": "abdominal_red_flag", "level": "severe", "title": fmt.Sprintf("腹部影像相关红旗：%s", symptom), "description": "建议立即线下评估"})
			level = diagnosticsMaxLevel(level, "severe")
			urgency = diagnosticsMaxUrgency(urgency, "emergency")
		}
	}
	if ageYears > 0 && ageYears < 2 && level != "normal" {
		urgency = diagnosticsMaxUrgency(urgency, "same_day")
	}

	return jsonStr(map[string]interface{}{
		"panel":         "abdominal_imaging_review",
		"age_years":     ageYears,
		"modality":      strings.TrimSpace(modality),
		"overall_level": level,
		"urgency":       urgency,
		"findings":      findings,
		"followup":      diagnosticsFollowupAdvice("abdominal_imaging", urgency),
	}), nil
}

func (t *DiagnosticsClinicTool) pathologyReportReview(params map[string]interface{}) (string, error) {
	specimenType, _ := params["specimen_type"].(string)
	diagnosisSummary, _ := params["diagnosis_summary"].(string)
	marginStatus, _ := params["margin_status"].(string)
	keywords := parseDiagnosticsList(params["keywords"])
	urgentFlags := parseDiagnosticsList(params["urgent_flags"])

	if strings.TrimSpace(diagnosisSummary) == "" && len(keywords) == 0 && len(urgentFlags) == 0 {
		return "", fmt.Errorf("pathology diagnosis_summary, keywords, or urgent_flags are required")
	}

	findings := []map[string]interface{}{}
	level := "normal"
	urgency := "routine"
	pathSignals := append([]string{specimenType, diagnosisSummary, marginStatus}, keywords...)
	pathSignals = append(pathSignals, urgentFlags...)

	if containsAnyNormalizedDiagnostics(pathSignals, "恶性", "malignant", "癌", "carcinoma", "肉瘤", "sarcoma", "高度异型", "high grade") {
		findings = append(findings, map[string]interface{}{"category": "pathology_high_risk", "level": "severe", "title": "病理提示高风险结果线索", "description": "建议尽快由相关专科医生线下解释与制定下一步计划"})
		level = diagnosticsMaxLevel(level, "severe")
		urgency = diagnosticsMaxUrgency(urgency, "same_day")
	}
	if containsAnyNormalizedDiagnostics(pathSignals, "不典型增生", "dysplasia", "可疑", "suspicious", "阳性边缘", "positive margin") {
		findings = append(findings, map[string]interface{}{"category": "pathology_attention", "level": "moderate", "title": "病理存在需重点复核线索", "description": "建议近期带完整报告至相关专科复核"})
		level = diagnosticsMaxLevel(level, "moderate")
		urgency = diagnosticsMaxUrgency(urgency, "expedited")
	}
	if containsAnyNormalizedDiagnostics(pathSignals, "良性", "benign", "炎症", "inflammation", "反应性") {
		findings = append(findings, map[string]interface{}{"category": "benign_or_inflammation", "level": "mild", "title": "病理更偏向良性或炎症性改变", "description": "仍需结合临床背景、影像和专科意见综合判断"})
		level = diagnosticsMaxLevel(level, "mild")
	}
	for _, flag := range urgentFlags {
		if containsAnyNormalizedDiagnostics([]string{flag}, "大出血", "坏死", "感染", "穿孔", "急诊") {
			findings = append(findings, map[string]interface{}{"category": "urgent_flag", "level": "severe", "title": fmt.Sprintf("病理红旗：%s", flag), "description": "建议立即线下评估"})
			level = diagnosticsMaxLevel(level, "severe")
			urgency = diagnosticsMaxUrgency(urgency, "emergency")
		}
	}

	return jsonStr(map[string]interface{}{
		"panel":         "pathology_report_review",
		"specimen_type": strings.TrimSpace(specimenType),
		"overall_level": level,
		"urgency":       urgency,
		"findings":      findings,
		"followup":      diagnosticsFollowupAdvice("pathology_report", urgency),
	}), nil
}

func parseDiagnosticsList(v interface{}) []string {
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
	return uniqueSortedDiagnosticsStrings(items)
}

func normalizeDiagnosticsTerm(v string) string {
	replacer := strings.NewReplacer(" ", "", "-", "", "_", "", ".", "", "/", "", "（", "(", "）", ")")
	return strings.ToLower(strings.TrimSpace(replacer.Replace(v)))
}

func uniqueSortedDiagnosticsStrings(items []string) []string {
	lookup := map[string]string{}
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		key := normalizeDiagnosticsTerm(trimmed)
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

func diagnosticsMaxLevel(current, next string) string {
	levels := map[string]int{"normal": 0, "mild": 1, "moderate": 2, "severe": 3}
	if levels[next] > levels[current] {
		return next
	}
	return current
}

func diagnosticsMaxUrgency(current, next string) string {
	levels := map[string]int{"routine": 0, "expedited": 1, "same_day": 2, "emergency": 3}
	if levels[next] > levels[current] {
		return next
	}
	return current
}

func diagnosticsFollowupAdvice(panel, urgency string) string {
	if urgency == "emergency" {
		return "建议立即线下就医或急诊评估，不要仅依赖线上建议。"
	}
	if urgency == "same_day" {
		return "建议当天线下复核，携带完整影像或病理报告、既往检查和症状时间线。"
	}
	switch panel {
	case "pathology_report":
		return "建议携带完整病理原文、标本信息和既往影像资料，由相关专科医生线下解释。"
	case "fracture_imaging":
		return "建议记录疼痛、负重能力、肿胀和肢端颜色温度变化，必要时尽快骨科复诊。"
	default:
		return "建议携带影像报告原文，并记录发热、疼痛、呼吸或消化道症状变化，必要时尽快复诊。"
	}
}

func containsAnyNormalizedDiagnostics(values []string, patterns ...string) bool {
	for _, value := range values {
		normalized := normalizeDiagnosticsTerm(value)
		for _, pattern := range patterns {
			if strings.Contains(normalized, normalizeDiagnosticsTerm(pattern)) {
				return true
			}
		}
	}
	return false
}

func diagnosticsToBool(v interface{}) bool {
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
