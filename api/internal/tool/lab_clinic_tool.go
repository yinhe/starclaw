package tool

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"gorm.io/gorm"
)

type LabClinicTool struct {
	db *gorm.DB
}

func NewLabClinicTool(db *gorm.DB) *LabClinicTool {
	return &LabClinicTool{db: db}
}

func (t *LabClinicTool) Execute(userID string, params map[string]interface{}) (string, error) {
	action, _ := params["action"].(string)
	switch action {
	case "parse_report":
		return t.parseReport(params)
	case "cbc_assess":
		return t.cbcAssess(params)
	case "iron_assess":
		return t.ironAssess(params)
	case "thyroid_assess":
		return t.thyroidAssess(params)
	case "vitamin_assess":
		return t.vitaminAssess(params)
	case "metabolic_assess":
		return t.metabolicAssess(params)
	default:
		return "", fmt.Errorf("unknown lab_clinic action: %s", action)
	}
}

func (t *LabClinicTool) cbcAssess(params map[string]interface{}) (string, error) {
	parsedMetrics := extractMetricsFromParams(params)
	ageYears := metricOrParsed(params, parsedMetrics, "age_years")
	hemoglobin := metricOrParsed(params, parsedMetrics, "hemoglobin")
	wbc := metricOrParsed(params, parsedMetrics, "wbc")
	neutrophilAbs := metricOrParsed(params, parsedMetrics, "neutrophil_abs")
	platelets := metricOrParsed(params, parsedMetrics, "platelets")
	mcv := metricOrParsed(params, parsedMetrics, "mcv")

	if hemoglobin <= 0 {
		return "", fmt.Errorf("hemoglobin is required")
	}

	findings := []map[string]interface{}{}
	overallLevel := "normal"
	hbLow := pediatricHemoglobinLower(ageYears)
	classification := "未见明显异常"

	if hemoglobin < hbLow {
		severity := "mild"
		title := fmt.Sprintf("血红蛋白偏低 %.1fg/L", hemoglobin)
		desc := fmt.Sprintf("低于该年龄段建议下限 %.1fg/L", hbLow)
		classification = "提示贫血筛查异常"
		if hemoglobin < 90 {
			severity = "severe"
			classification = "重度贫血风险"
		} else if hemoglobin < 110 {
			severity = "moderate"
		}
		findings = append(findings, map[string]interface{}{"category": "hemoglobin", "level": severity, "title": title, "description": desc})
		overallLevel = maxLevel(overallLevel, severity)
	}

	if mcv > 0 && mcv < 80 {
		findings = append(findings, map[string]interface{}{"category": "mcv", "level": "mild", "title": fmt.Sprintf("MCV 偏低 %.1ffL", mcv), "description": "提示小细胞性贫血或缺铁风险"})
		overallLevel = maxLevel(overallLevel, "mild")
	}

	if wbc > 0 {
		if wbc < 4 {
			findings = append(findings, map[string]interface{}{"category": "wbc", "level": "moderate", "title": fmt.Sprintf("白细胞偏低 %.1f x10^9/L", wbc), "description": "建议结合感染史、药物史和复查计划"})
			overallLevel = maxLevel(overallLevel, "moderate")
		} else if wbc > pediatricWBCUpper(ageYears) {
			findings = append(findings, map[string]interface{}{"category": "wbc", "level": "mild", "title": fmt.Sprintf("白细胞偏高 %.1f x10^9/L", wbc), "description": "需结合感染、应激或炎症背景判断"})
			overallLevel = maxLevel(overallLevel, "mild")
		}
	}

	if neutrophilAbs > 0 {
		if neutrophilAbs < 0.5 {
			findings = append(findings, map[string]interface{}{"category": "neutrophil", "level": "severe", "title": fmt.Sprintf("重度中性粒细胞减少 %.2f x10^9/L", neutrophilAbs), "description": "感染风险高，建议尽快复核并结合临床处理"})
			overallLevel = maxLevel(overallLevel, "severe")
		} else if neutrophilAbs < 1.5 {
			findings = append(findings, map[string]interface{}{"category": "neutrophil", "level": "moderate", "title": fmt.Sprintf("中性粒细胞减少 %.2f x10^9/L", neutrophilAbs), "description": "建议结合发热、感染暴露史和近期用药情况"})
			overallLevel = maxLevel(overallLevel, "moderate")
		}
	}

	if platelets > 0 {
		if platelets < 50 {
			findings = append(findings, map[string]interface{}{"category": "platelet", "level": "severe", "title": fmt.Sprintf("重度血小板减少 %.0f x10^9/L", platelets), "description": "出血风险升高，建议尽快处理"})
			overallLevel = maxLevel(overallLevel, "severe")
		} else if platelets < 100 {
			findings = append(findings, map[string]interface{}{"category": "platelet", "level": "moderate", "title": fmt.Sprintf("血小板偏低 %.0f x10^9/L", platelets), "description": "建议复核并评估感染、免疫或药物相关因素"})
			overallLevel = maxLevel(overallLevel, "moderate")
		} else if platelets > 450 {
			findings = append(findings, map[string]interface{}{"category": "platelet", "level": "mild", "title": fmt.Sprintf("血小板偏高 %.0f x10^9/L", platelets), "description": "常见于炎症或缺铁状态，需结合临床判断"})
			overallLevel = maxLevel(overallLevel, "mild")
		}
	}

	return jsonStr(map[string]interface{}{
		"panel":          "cbc",
		"age_years":      ageYears,
		"hemoglobin":     hemoglobin,
		"wbc":            wbc,
		"neutrophil_abs": neutrophilAbs,
		"platelets":      platelets,
		"mcv":            mcv,
		"classification": classification,
		"overall_level":  overallLevel,
		"findings":       findings,
		"followup":       cbcFollowup(overallLevel, hemoglobin, mcv),
	}), nil
}

func (t *LabClinicTool) ironAssess(params map[string]interface{}) (string, error) {
	parsedMetrics := extractMetricsFromParams(params)
	ferritin := metricOrParsed(params, parsedMetrics, "ferritin")
	serumIron := metricOrParsed(params, parsedMetrics, "serum_iron")
	tibc := metricOrParsed(params, parsedMetrics, "tibc")
	hemoglobin := metricOrParsed(params, parsedMetrics, "hemoglobin")
	mcv := metricOrParsed(params, parsedMetrics, "mcv")
	crp := metricOrParsed(params, parsedMetrics, "crp")

	if ferritin <= 0 && serumIron <= 0 && tibc <= 0 && hemoglobin <= 0 {
		return "", fmt.Errorf("at least one iron panel marker is required")
	}

	findings := []map[string]interface{}{}
	overallLevel := "normal"
	transferrinSaturation := 0.0
	if serumIron > 0 && tibc > 0 {
		transferrinSaturation = roundTo(serumIron/tibc*100, 1)
	}

	ironDeficiency := false
	if ferritin > 0 && ferritin < 15 {
		ironDeficiency = true
		findings = append(findings, map[string]interface{}{"category": "ferritin", "level": "moderate", "title": fmt.Sprintf("铁蛋白偏低 %.1f ng/mL", ferritin), "description": "提示储存铁不足"})
		overallLevel = maxLevel(overallLevel, "moderate")
	}
	if transferrinSaturation > 0 && transferrinSaturation < 20 {
		ironDeficiency = true
		findings = append(findings, map[string]interface{}{"category": "tsat", "level": "mild", "title": fmt.Sprintf("转铁蛋白饱和度偏低 %.1f%%", transferrinSaturation), "description": "支持缺铁状态"})
		overallLevel = maxLevel(overallLevel, "mild")
	}
	if mcv > 0 && mcv < 80 {
		ironDeficiency = true
	}

	classification := "铁代谢未见明显异常"
	if ironDeficiency {
		classification = "提示缺铁风险"
	}
	if ironDeficiency && hemoglobin > 0 && hemoglobin < 110 {
		classification = "提示缺铁性贫血风险"
		overallLevel = maxLevel(overallLevel, "moderate")
	}
	if ferritin > 0 && ferritin < 8 {
		classification = "高度提示缺铁或缺铁性贫血"
		overallLevel = maxLevel(overallLevel, "severe")
	}

	caveat := ""
	if crp > 10 {
		caveat = "CRP 升高时铁蛋白可受炎症影响，需结合临床复核"
	}

	return jsonStr(map[string]interface{}{
		"panel":                  "iron",
		"ferritin":               ferritin,
		"serum_iron":             serumIron,
		"tibc":                   tibc,
		"transferrin_saturation": transferrinSaturation,
		"hemoglobin":             hemoglobin,
		"mcv":                    mcv,
		"crp":                    crp,
		"classification":         classification,
		"overall_level":          overallLevel,
		"findings":               findings,
		"interpretation_caveat":  caveat,
		"followup":               ironFollowup(classification),
	}), nil
}

func (t *LabClinicTool) thyroidAssess(params map[string]interface{}) (string, error) {
	parsedMetrics := extractMetricsFromParams(params)
	tsh := metricOrParsed(params, parsedMetrics, "tsh")
	ft4 := metricOrParsed(params, parsedMetrics, "ft4")
	ft3 := metricOrParsed(params, parsedMetrics, "ft3")

	if tsh <= 0 && ft4 <= 0 && ft3 <= 0 {
		return "", fmt.Errorf("tsh, ft4, or ft3 is required")
	}

	classification := "甲状腺功能未见明显异常"
	overallLevel := "normal"
	findings := []map[string]interface{}{}

	if tsh >= 10 || (tsh > 4.5 && ft4 > 0 && ft4 < 12) {
		classification = "提示甲减风险"
		overallLevel = "moderate"
		findings = append(findings, map[string]interface{}{"category": "thyroid", "level": "moderate", "title": fmt.Sprintf("TSH 升高 %.2f mIU/L", tsh), "description": "需结合 FT4 和临床表现判断甲状腺功能减退"})
	}
	if tsh > 20 {
		classification = "高度提示甲减风险"
		overallLevel = "severe"
	}
	if tsh > 4.5 && tsh < 10 && ft4 >= 12 {
		classification = "提示亚临床甲减风险"
		overallLevel = maxLevel(overallLevel, "mild")
	}
	if tsh > 0 && tsh < 0.1 && ((ft4 > 22 && ft4 > 0) || (ft3 > 6.8 && ft3 > 0)) {
		classification = "提示甲亢风险"
		overallLevel = maxLevel(overallLevel, "moderate")
		findings = append(findings, map[string]interface{}{"category": "thyroid", "level": overallLevel, "title": "TSH 抑制伴甲状腺激素升高", "description": "需排查甲亢或相关治疗影响"})
	}
	if ft4 > 0 && ft4 < 12 && tsh > 0 && tsh <= 4.5 {
		classification = "FT4 偏低需复核"
		overallLevel = maxLevel(overallLevel, "moderate")
	}

	return jsonStr(map[string]interface{}{
		"panel":          "thyroid",
		"tsh":            tsh,
		"ft4":            ft4,
		"ft3":            ft3,
		"classification": classification,
		"overall_level":  overallLevel,
		"findings":       findings,
		"followup":       thyroidFollowup(classification),
	}), nil
}

func (t *LabClinicTool) vitaminAssess(params map[string]interface{}) (string, error) {
	parsedMetrics := extractMetricsFromParams(params)
	vitaminD := metricOrParsed(params, parsedMetrics, "vitamin_d")
	calcium := metricOrParsed(params, parsedMetrics, "calcium")
	phosphorus := metricOrParsed(params, parsedMetrics, "phosphorus")
	alp := metricOrParsed(params, parsedMetrics, "alp")

	if vitaminD <= 0 && calcium <= 0 && phosphorus <= 0 && alp <= 0 {
		return "", fmt.Errorf("at least one bone metabolism marker is required")
	}

	classification := "骨代谢指标未见明显异常"
	overallLevel := "normal"
	findings := []map[string]interface{}{}

	if vitaminD > 0 {
		if vitaminD < 12 {
			classification = "重度维生素D缺乏风险"
			overallLevel = maxLevel(overallLevel, "severe")
		} else if vitaminD < 20 {
			classification = "维生素D缺乏风险"
			overallLevel = maxLevel(overallLevel, "moderate")
		} else if vitaminD < 30 {
			classification = "维生素D不足"
			overallLevel = maxLevel(overallLevel, "mild")
		}
	}
	if calcium > 0 && calcium < 2.1 {
		findings = append(findings, map[string]interface{}{"category": "calcium", "level": "moderate", "title": fmt.Sprintf("血钙偏低 %.2f mmol/L", calcium), "description": "需结合饮食、维生素D和症状复核"})
		overallLevel = maxLevel(overallLevel, "moderate")
	}
	if phosphorus > 0 && phosphorus < 1.3 {
		findings = append(findings, map[string]interface{}{"category": "phosphorus", "level": "mild", "title": fmt.Sprintf("血磷偏低 %.2f mmol/L", phosphorus), "description": "需结合骨代谢和营养状况判断"})
		overallLevel = maxLevel(overallLevel, "mild")
	}
	if alp > 500 {
		findings = append(findings, map[string]interface{}{"category": "alp", "level": "mild", "title": fmt.Sprintf("ALP 升高 %.0f U/L", alp), "description": "生长活跃期可升高，需结合维生素D和骨龄判断"})
		overallLevel = maxLevel(overallLevel, "mild")
	}

	return jsonStr(map[string]interface{}{
		"panel":          "vitamin_bone_metabolism",
		"vitamin_d":      vitaminD,
		"calcium":        calcium,
		"phosphorus":     phosphorus,
		"alp":            alp,
		"classification": classification,
		"overall_level":  overallLevel,
		"findings":       findings,
		"followup":       vitaminFollowup(classification),
	}), nil
}

func (t *LabClinicTool) metabolicAssess(params map[string]interface{}) (string, error) {
	parsedMetrics := extractMetricsFromParams(params)
	fastingGlucose := metricOrParsed(params, parsedMetrics, "fasting_glucose")
	alt := metricOrParsed(params, parsedMetrics, "alt")
	ast := metricOrParsed(params, parsedMetrics, "ast")
	triglycerides := metricOrParsed(params, parsedMetrics, "triglycerides")
	totalCholesterol := metricOrParsed(params, parsedMetrics, "total_cholesterol")
	ldlC := metricOrParsed(params, parsedMetrics, "ldl_c")
	uricAcid := metricOrParsed(params, parsedMetrics, "uric_acid")
	bmiZ := metricOrParsed(params, parsedMetrics, "bmi_z_score")

	if fastingGlucose <= 0 && alt <= 0 && ast <= 0 && triglycerides <= 0 && totalCholesterol <= 0 && ldlC <= 0 && uricAcid <= 0 {
		return "", fmt.Errorf("at least one metabolic marker is required")
	}

	findings := []map[string]interface{}{}
	overallLevel := "normal"
	tags := []string{}

	if fastingGlucose >= 7 {
		findings = append(findings, map[string]interface{}{"category": "glucose", "level": "severe", "title": fmt.Sprintf("空腹血糖升高 %.2f mmol/L", fastingGlucose), "description": "达到糖尿病风险阈值，需尽快复核"})
		overallLevel = maxLevel(overallLevel, "severe")
		tags = append(tags, "hyperglycemia")
	} else if fastingGlucose >= 5.6 {
		findings = append(findings, map[string]interface{}{"category": "glucose", "level": "moderate", "title": fmt.Sprintf("空腹血糖受损 %.2f mmol/L", fastingGlucose), "description": "建议结合 HbA1c 或复查判断"})
		overallLevel = maxLevel(overallLevel, "moderate")
		tags = append(tags, "impaired_fasting_glucose")
	}
	if alt > 80 {
		findings = append(findings, map[string]interface{}{"category": "liver", "level": "moderate", "title": fmt.Sprintf("ALT 明显升高 %.0f U/L", alt), "description": "需排查脂肪肝、药物或感染相关因素"})
		overallLevel = maxLevel(overallLevel, "moderate")
		tags = append(tags, "liver_injury")
	} else if alt > 40 {
		findings = append(findings, map[string]interface{}{"category": "liver", "level": "mild", "title": fmt.Sprintf("ALT 偏高 %.0f U/L", alt), "description": "建议复核肝功能并结合体重管理"})
		overallLevel = maxLevel(overallLevel, "mild")
		tags = append(tags, "liver_risk")
	}
	if triglycerides > 1.7 {
		findings = append(findings, map[string]interface{}{"category": "lipid", "level": "mild", "title": fmt.Sprintf("甘油三酯偏高 %.2f mmol/L", triglycerides), "description": "提示代谢风险增加"})
		overallLevel = maxLevel(overallLevel, "mild")
		tags = append(tags, "hypertriglyceridemia")
	}
	if totalCholesterol > 5.2 || ldlC > 3.4 {
		findings = append(findings, map[string]interface{}{"category": "lipid", "level": "mild", "title": "胆固醇异常", "description": "建议结合饮食结构、运动和家族史评估"})
		overallLevel = maxLevel(overallLevel, "mild")
		tags = append(tags, "dyslipidemia")
	}
	if uricAcid > 420 {
		findings = append(findings, map[string]interface{}{"category": "uric_acid", "level": "moderate", "title": fmt.Sprintf("尿酸偏高 %.0f umol/L", uricAcid), "description": "需结合肥胖、饮食和肾功能背景判断"})
		overallLevel = maxLevel(overallLevel, "moderate")
		tags = append(tags, "hyperuricemia")
	} else if uricAcid > 360 {
		findings = append(findings, map[string]interface{}{"category": "uric_acid", "level": "mild", "title": fmt.Sprintf("尿酸临界升高 %.0f umol/L", uricAcid), "description": "建议结合饮水和饮食管理"})
		overallLevel = maxLevel(overallLevel, "mild")
	}
	if bmiZ > 2 && len(tags) > 0 {
		tags = append(tags, "obesity_related_risk")
	}

	classification := "代谢风险未见明显异常"
	if len(tags) > 0 {
		classification = "提示代谢异常筛查风险"
	}

	return jsonStr(map[string]interface{}{
		"panel":             "metabolic",
		"fasting_glucose":   fastingGlucose,
		"alt":               alt,
		"ast":               ast,
		"triglycerides":     triglycerides,
		"total_cholesterol": totalCholesterol,
		"ldl_c":             ldlC,
		"uric_acid":         uricAcid,
		"bmi_z_score":       bmiZ,
		"classification":    classification,
		"overall_level":     overallLevel,
		"risk_tags":         tags,
		"findings":          findings,
		"followup":          metabolicFollowup(tags, overallLevel),
	}), nil
}

func (t *LabClinicTool) parseReport(params map[string]interface{}) (string, error) {
	reportText, _ := params["report_text"].(string)
	reportText = strings.TrimSpace(reportText)
	if reportText == "" {
		return "", fmt.Errorf("report_text is required")
	}
	panelHint, _ := params["panel_hint"].(string)
	metrics, matchedAliases := extractLabMetrics(reportText)
	if ageYears := toFloat(params["age_years"]); ageYears > 0 {
		metrics["age_years"] = ageYears
		matchedAliases["age_years"] = "age_years"
	}
	suggestedPanels := suggestLabPanels(metrics, panelHint)
	toolPayloads := buildLabToolPayloads(metrics, suggestedPanels)
	message := "已识别结构化检验指标"
	if len(metrics) == 0 {
		message = "未识别到可结构化检验指标"
	}
	return jsonStr(map[string]interface{}{
		"message":            message,
		"panel_hint":         panelHint,
		"recognized_metrics": metrics,
		"matched_aliases":    matchedAliases,
		"suggested_panels":   suggestedPanels,
		"tool_payloads":      toolPayloads,
		"report_length":      len([]rune(reportText)),
	}), nil
}

func pediatricHemoglobinLower(ageYears float64) float64 {
	switch {
	case ageYears <= 0:
		return 110
	case ageYears < 5:
		return 110
	case ageYears < 12:
		return 115
	case ageYears < 15:
		return 120
	default:
		return 120
	}
}

func pediatricWBCUpper(ageYears float64) float64 {
	switch {
	case ageYears > 0 && ageYears < 6:
		return 15
	default:
		return 12
	}
}

func maxLevel(current, next string) string {
	levels := map[string]int{"normal": 0, "mild": 1, "moderate": 2, "severe": 3}
	if levels[next] > levels[current] {
		return next
	}
	return current
}

func cbcFollowup(level string, hemoglobin, mcv float64) string {
	if level == "severe" {
		return "建议尽快复查血常规并由医生评估是否需要紧急处理。"
	}
	if hemoglobin > 0 && hemoglobin < 110 && mcv > 0 && mcv < 80 {
		return "建议结合铁蛋白、转铁蛋白饱和度进一步评估缺铁性贫血风险。"
	}
	if level == "normal" {
		return "结合临床症状常规随访即可。"
	}
	return "建议结合症状、感染史、营养情况与复查结果综合判断。"
}

func ironFollowup(classification string) string {
	switch classification {
	case "高度提示缺铁或缺铁性贫血":
		return "建议尽快复核铁蛋白、血常规并由医生判断是否需要进一步干预。"
	case "提示缺铁性贫血风险":
		return "建议结合 CBC、铁蛋白和饮食史完善评估，并安排复查。"
	case "提示缺铁风险":
		return "建议关注膳食铁摄入并结合复查结果判断是否持续缺铁。"
	default:
		return "若临床仍怀疑缺铁，可结合炎症指标和复查结果动态评估。"
	}
}

func thyroidFollowup(classification string) string {
	switch classification {
	case "高度提示甲减风险", "提示甲减风险":
		return "建议尽快复核 TSH、FT4，并结合身高增长、乏力、便秘等症状由专科医生评估。"
	case "提示甲亢风险":
		return "建议结合心率、体重变化和复查甲功指标进一步评估。"
	case "提示亚临床甲减风险", "FT4 偏低需复核":
		return "建议短期复查甲状腺功能并结合抗体、超声或临床表现综合判断。"
	default:
		return "当前甲功解读仅供筛查参考，需结合临床情况判断。"
	}
}

func vitaminFollowup(classification string) string {
	switch classification {
	case "重度维生素D缺乏风险", "维生素D缺乏风险":
		return "建议结合日照、饮食和复查计划，由医生决定是否需要进一步补充或检查。"
	case "维生素D不足":
		return "建议优化户外活动、饮食结构并按计划复查。"
	default:
		return "如伴骨痛、抽搐、骨龄异常等情况，需进一步结合临床评估。"
	}
}

func metabolicFollowup(tags []string, level string) string {
	if level == "severe" {
		return "建议尽快复查关键代谢指标，并由医生评估是否需要进一步处理。"
	}
	if len(tags) == 0 {
		return "结合 BMI、生活方式和家族史常规随访即可。"
	}
	return "建议结合体重管理、饮食运动干预和后续复查，动态评估代谢风险。"
}

type labMetricDef struct {
	Key     string
	Aliases []string
}

func extractMetricsFromParams(params map[string]interface{}) map[string]float64 {
	reportText, _ := params["report_text"].(string)
	reportText = strings.TrimSpace(reportText)
	if reportText == "" {
		return map[string]float64{}
	}
	metrics, _ := extractLabMetrics(reportText)
	if ageYears := toFloat(params["age_years"]); ageYears > 0 {
		metrics["age_years"] = ageYears
	}
	return metrics
}

func metricOrParsed(params map[string]interface{}, parsed map[string]float64, key string) float64 {
	if value := toFloat(params[key]); value != 0 {
		return value
	}
	return parsed[key]
}

func extractLabMetrics(reportText string) (map[string]float64, map[string]string) {
	text := strings.ReplaceAll(reportText, "\r", "\n")
	text = strings.ReplaceAll(text, "\t", " ")
	metrics := map[string]float64{}
	aliases := map[string]string{}

	defs := []labMetricDef{
		{Key: "hemoglobin", Aliases: []string{"血红蛋白", "hemoglobin", "hgb"}},
		{Key: "wbc", Aliases: []string{"白细胞", "white blood cell", "wbc"}},
		{Key: "neutrophil_abs", Aliases: []string{"中性粒细胞绝对值", "中性粒细胞绝对计数", "anc", "neutrophil abs", "中性粒细胞#"}},
		{Key: "platelets", Aliases: []string{"血小板", "platelet", "plt"}},
		{Key: "mcv", Aliases: []string{"平均红细胞体积", "mcv"}},
		{Key: "ferritin", Aliases: []string{"铁蛋白", "ferritin"}},
		{Key: "serum_iron", Aliases: []string{"血清铁", "serum iron"}},
		{Key: "tibc", Aliases: []string{"总铁结合力", "tibc"}},
		{Key: "crp", Aliases: []string{"c反应蛋白", "crp", "c-reactive protein"}},
		{Key: "tsh", Aliases: []string{"促甲状腺激素", "tsh"}},
		{Key: "ft4", Aliases: []string{"游离甲状腺素", "游离t4", "ft4"}},
		{Key: "ft3", Aliases: []string{"游离三碘甲状腺原氨酸", "游离t3", "ft3"}},
		{Key: "vitamin_d", Aliases: []string{"25-oh维生素d", "25-oh vitamin d", "25(oh)d", "25羟维生素d", "维生素d"}},
		{Key: "calcium", Aliases: []string{"血钙", "serum calcium", "calcium"}},
		{Key: "phosphorus", Aliases: []string{"血磷", "phosphorus", "phosphate"}},
		{Key: "alp", Aliases: []string{"碱性磷酸酶", "alkaline phosphatase", "alp"}},
		{Key: "fasting_glucose", Aliases: []string{"空腹血糖", "fasting glucose", "fpg", "glu"}},
		{Key: "alt", Aliases: []string{"谷丙转氨酶", "丙氨酸氨基转移酶", "alt"}},
		{Key: "ast", Aliases: []string{"谷草转氨酶", "天门冬氨酸氨基转移酶", "ast"}},
		{Key: "triglycerides", Aliases: []string{"甘油三酯", "triglycerides", "tg"}},
		{Key: "total_cholesterol", Aliases: []string{"总胆固醇", "total cholesterol", "tc"}},
		{Key: "ldl_c", Aliases: []string{"低密度脂蛋白胆固醇", "ldl-c", "ldl"}},
		{Key: "uric_acid", Aliases: []string{"尿酸", "uric acid", "ua"}},
		{Key: "bmi_z_score", Aliases: []string{"bmi z score", "bmi-z", "bmi z"}},
	}

	for _, def := range defs {
		if value, alias, ok := findLabMetricValue(text, def.Aliases); ok {
			metrics[def.Key] = value
			aliases[def.Key] = alias
		}
	}

	agePatterns := []string{
		`(?:年龄|age)\s*[:：]?\s*(\d+(?:\.\d+)?)\s*(?:岁|years?|y)?`,
	}
	for _, pattern := range agePatterns {
		re := regexp.MustCompile(`(?i)` + pattern)
		if matches := re.FindStringSubmatch(text); len(matches) >= 2 {
			metrics["age_years"] = toFloat(matches[1])
			aliases["age_years"] = "年龄"
			break
		}
	}

	return metrics, aliases
}

func findLabMetricValue(reportText string, aliases []string) (float64, string, bool) {
	for _, alias := range aliases {
		pattern := `(?is)` + aliasRegexPattern(alias) + `\s*(?:\([^\)]*\))?\s*[:：=]?\s*([<>]?)\s*(-?\d+(?:\.\d+)?)`
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(reportText)
		if len(matches) >= 3 {
			return toFloat(matches[2]), alias, true
		}
	}
	return 0, "", false
}

func aliasRegexPattern(alias string) string {
	if regexp.MustCompile(`^[A-Za-z0-9._-]+$`).MatchString(alias) {
		return `\b` + regexp.QuoteMeta(alias) + `\b`
	}
	return regexp.QuoteMeta(alias)
}

func suggestLabPanels(metrics map[string]float64, panelHint string) []string {
	panelKeys := map[string][]string{
		"cbc":       {"hemoglobin", "wbc", "neutrophil_abs", "platelets", "mcv"},
		"iron":      {"ferritin", "serum_iron", "tibc", "hemoglobin", "mcv", "crp"},
		"thyroid":   {"tsh", "ft4", "ft3"},
		"vitamin":   {"vitamin_d", "calcium", "phosphorus", "alp"},
		"metabolic": {"fasting_glucose", "alt", "ast", "triglycerides", "total_cholesterol", "ldl_c", "uric_acid", "bmi_z_score"},
	}

	type panelScore struct {
		name  string
		score int
	}
	var scores []panelScore
	for panel, keys := range panelKeys {
		score := 0
		for _, key := range keys {
			if _, ok := metrics[key]; ok {
				score++
			}
		}
		if score > 0 || (panelHint != "" && strings.EqualFold(panelHint, panel)) {
			scores = append(scores, panelScore{name: panel, score: score})
		}
	}

	sort.Slice(scores, func(i, j int) bool {
		if strings.EqualFold(scores[i].name, panelHint) {
			return true
		}
		if strings.EqualFold(scores[j].name, panelHint) {
			return false
		}
		if scores[i].score == scores[j].score {
			return scores[i].name < scores[j].name
		}
		return scores[i].score > scores[j].score
	})

	result := make([]string, 0, len(scores))
	for _, score := range scores {
		result = append(result, score.name)
	}
	return result
}

func buildLabToolPayloads(metrics map[string]float64, panels []string) map[string]map[string]interface{} {
	payloads := map[string]map[string]interface{}{}
	for _, panel := range panels {
		switch panel {
		case "cbc":
			if hasAnyMetric(metrics, "hemoglobin", "wbc", "neutrophil_abs", "platelets", "mcv") {
				payloads["lab_cbc_assess"] = map[string]interface{}{"action": "cbc_assess"}
				for _, key := range []string{"age_years", "hemoglobin", "wbc", "neutrophil_abs", "platelets", "mcv"} {
					if value, ok := metrics[key]; ok {
						payloads["lab_cbc_assess"][key] = value
					}
				}
			}
		case "iron":
			if hasAnyMetric(metrics, "ferritin", "serum_iron", "tibc", "hemoglobin", "mcv", "crp") {
				payloads["lab_iron_assess"] = map[string]interface{}{"action": "iron_assess"}
				for _, key := range []string{"ferritin", "serum_iron", "tibc", "hemoglobin", "mcv", "crp"} {
					if value, ok := metrics[key]; ok {
						payloads["lab_iron_assess"][key] = value
					}
				}
			}
		case "thyroid":
			if hasAnyMetric(metrics, "tsh", "ft4", "ft3") {
				payloads["lab_thyroid_assess"] = map[string]interface{}{"action": "thyroid_assess"}
				for _, key := range []string{"tsh", "ft4", "ft3"} {
					if value, ok := metrics[key]; ok {
						payloads["lab_thyroid_assess"][key] = value
					}
				}
			}
		case "vitamin":
			if hasAnyMetric(metrics, "vitamin_d", "calcium", "phosphorus", "alp") {
				payloads["lab_vitamin_assess"] = map[string]interface{}{"action": "vitamin_assess"}
				for _, key := range []string{"vitamin_d", "calcium", "phosphorus", "alp"} {
					if value, ok := metrics[key]; ok {
						payloads["lab_vitamin_assess"][key] = value
					}
				}
			}
		case "metabolic":
			if hasAnyMetric(metrics, "fasting_glucose", "alt", "ast", "triglycerides", "total_cholesterol", "ldl_c", "uric_acid", "bmi_z_score") {
				payloads["lab_metabolic_assess"] = map[string]interface{}{"action": "metabolic_assess"}
				for _, key := range []string{"fasting_glucose", "alt", "ast", "triglycerides", "total_cholesterol", "ldl_c", "uric_acid", "bmi_z_score"} {
					if value, ok := metrics[key]; ok {
						payloads["lab_metabolic_assess"][key] = value
					}
				}
			}
		}
	}
	return payloads
}

func hasAnyMetric(metrics map[string]float64, keys ...string) bool {
	for _, key := range keys {
		if _, ok := metrics[key]; ok {
			return true
		}
	}
	return false
}
