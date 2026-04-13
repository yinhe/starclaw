package tool

import (
	"fmt"
	"math"
	"time"

	"github.com/yinhe/starclaw/internal/model"
	"gorm.io/gorm"
)

// GrowthClinicTool implements the 13 growth & development clinic tools.
// All calculations use deterministic algorithms (not LLM).
type GrowthClinicTool struct {
	db *gorm.DB
}

func NewGrowthClinicTool(db *gorm.DB) *GrowthClinicTool {
	return &GrowthClinicTool{db: db}
}

// Execute dispatches tool calls by action name.
func (t *GrowthClinicTool) Execute(userID string, params map[string]interface{}) (string, error) {
	action, _ := params["action"].(string)
	switch action {
	case "zscore":
		return t.calcZScore(params)
	case "growth_curve":
		return t.growthCurve(userID, params)
	case "growth_velocity":
		return t.growthVelocity(userID, params)
	case "bmi_assess":
		return t.bmiAssess(params)
	case "target_height":
		return t.targetHeight(params)
	case "bone_age_compare":
		return t.boneAgeCompare(params)
	case "puberty_assess":
		return t.pubertyAssess(params)
	case "alert_evaluate":
		return t.alertEvaluate(userID, params)
	case "patient_record":
		return t.patientRecord(userID, params)
	case "followup_schedule":
		return t.followupSchedule(userID, params)
	case "data_quality_check":
		return t.dataQualityCheck(params)
	case "education_push":
		return t.educationPush(userID, params)
	case "report_summary":
		return t.reportSummary(userID, params)
	default:
		return "", fmt.Errorf("unknown growth_clinic action: %s", action)
	}
}

// ═══════════════════════════════════════════════════════════════
// 1. Z-Score Calculator (WHO + China national standards)
// ═══════════════════════════════════════════════════════════════

func (t *GrowthClinicTool) calcZScore(params map[string]interface{}) (string, error) {
	gender, _ := params["gender"].(string)
	ageMonths := toFloat(params["age_months"])
	height := toFloat(params["height"])
	weight := toFloat(params["weight"])

	if gender == "" || ageMonths <= 0 {
		return "", fmt.Errorf("gender and age_months are required")
	}

	result := map[string]interface{}{
		"age_months": ageMonths,
		"gender":     gender,
	}

	if height > 0 {
		hz, hp := heightForAgeZScore(gender, ageMonths, height)
		result["height_cm"] = height
		result["height_z_score"] = roundTo(hz, 2)
		result["height_percentile"] = roundTo(hp, 1)
		result["height_interpretation"] = interpretZScore(hz)
	}

	if weight > 0 {
		wz, wp := weightForAgeZScore(gender, ageMonths, weight)
		result["weight_kg"] = weight
		result["weight_z_score"] = roundTo(wz, 2)
		result["weight_percentile"] = roundTo(wp, 1)
		result["weight_interpretation"] = interpretZScore(wz)
	}

	if height > 0 && weight > 0 {
		hm := height / 100
		bmi := weight / (hm * hm)
		bz, bp := bmiForAgeZScore(gender, ageMonths, bmi)
		result["bmi"] = roundTo(bmi, 1)
		result["bmi_z_score"] = roundTo(bz, 2)
		result["bmi_percentile"] = roundTo(bp, 1)
		result["bmi_interpretation"] = interpretBMIZScore(bz)
	}

	return jsonStr(result), nil
}

// heightForAgeZScore calculates Z-score using simplified LMS method.
// Based on: 《中国7岁以下儿童生长发育参照标准》(2009) + WHO Child Growth Standards.
func heightForAgeZScore(gender string, ageMonths, height float64) (z, percentile float64) {
	l, m, s := getHeightLMS(gender, ageMonths)
	z = lmsToZ(l, m, s, height)
	percentile = zToPercentile(z)
	return
}

func weightForAgeZScore(gender string, ageMonths, weight float64) (z, percentile float64) {
	l, m, s := getWeightLMS(gender, ageMonths)
	z = lmsToZ(l, m, s, weight)
	percentile = zToPercentile(z)
	return
}

func bmiForAgeZScore(gender string, ageMonths, bmi float64) (z, percentile float64) {
	l, m, s := getBMI_LMS(gender, ageMonths)
	z = lmsToZ(l, m, s, bmi)
	percentile = zToPercentile(z)
	return
}

// lmsToZ converts a measurement to Z-score using the LMS method.
// Z = ((X/M)^L - 1) / (L*S) when L != 0
// Z = ln(X/M) / S when L == 0
func lmsToZ(l, m, s, x float64) float64 {
	if m <= 0 || s <= 0 {
		return 0
	}
	if l == 0 {
		return math.Log(x/m) / s
	}
	return (math.Pow(x/m, l) - 1) / (l * s)
}

// zToPercentile converts Z-score to percentile using standard normal CDF approximation.
func zToPercentile(z float64) float64 {
	// Abramowitz and Stegun approximation
	if z < -6 {
		return 0.001
	}
	if z > 6 {
		return 99.999
	}
	t := 1.0 / (1.0 + 0.2316419*math.Abs(z))
	d := 0.3989422804014327 // 1/sqrt(2*pi)
	p := d * math.Exp(-z*z/2.0) * (t * (0.319381530 + t*(-0.356563782+t*(1.781477937+t*(-1.821255978+t*1.330274429)))))
	if z > 0 {
		p = 1.0 - p
	}
	return roundTo(p*100, 1)
}

func interpretZScore(z float64) string {
	switch {
	case z < -3:
		return "严重偏低（<-3SD）— 危急值"
	case z < -2:
		return "偏低（-3SD~-2SD）— 中度异常"
	case z < -1:
		return "略偏低（-2SD~-1SD）— 轻度异常"
	case z <= 1:
		return "正常范围（-1SD~+1SD）"
	case z <= 2:
		return "偏高（+1SD~+2SD）"
	case z <= 3:
		return "明显偏高（+2SD~+3SD）"
	default:
		return "严重偏高（>+3SD）"
	}
}

func interpretBMIZScore(z float64) string {
	switch {
	case z < -2:
		return "消瘦"
	case z < -1:
		return "偏瘦"
	case z <= 1:
		return "正常"
	case z <= 2:
		return "超重"
	default:
		return "肥胖"
	}
}

// ═══════════════════════════════════════════════════════════════
// 2. Growth Curve — returns historical data points for charting
// ═══════════════════════════════════════════════════════════════

func (t *GrowthClinicTool) growthCurve(userID string, params map[string]interface{}) (string, error) {
	patientID, _ := params["patient_id"].(string)
	if patientID == "" {
		return "", fmt.Errorf("patient_id is required")
	}

	var records []model.GCGrowthRecord
	t.db.Where("user_id = ? AND patient_id = ?", userID, patientID).
		Order("record_date ASC").Limit(200).Find(&records)

	type point struct {
		Date       string  `json:"date"`
		AgeMonths  float64 `json:"age_months"`
		Height     float64 `json:"height"`
		Weight     float64 `json:"weight"`
		BMI        float64 `json:"bmi"`
		HeightZ    float64 `json:"height_z"`
		WeightZ    float64 `json:"weight_z"`
		BMIZ       float64 `json:"bmi_z"`
		Percentile float64 `json:"percentile"`
	}

	var patient model.GCPatient
	t.db.Where("id = ? AND user_id = ?", patientID, userID).First(&patient)

	points := make([]point, 0, len(records))
	for _, r := range records {
		points = append(points, point{
			Date:       r.RecordDate.Format("2006-01-02"),
			AgeMonths:  r.AgeMonths,
			Height:     r.Height,
			Weight:     r.Weight,
			BMI:        r.BMI,
			HeightZ:    r.HeightZScore,
			WeightZ:    r.WeightZScore,
			BMIZ:       r.BMIZScore,
			Percentile: r.HeightPercentile,
		})
	}

	// Generate reference curves (P3, P10, P25, P50, P75, P90, P97)
	refCurves := generateRefCurves(patient.Gender, records)

	return jsonStr(map[string]interface{}{
		"patient":    patient.Name,
		"gender":     patient.Gender,
		"points":     points,
		"ref_curves": refCurves,
		"total":      len(points),
	}), nil
}

func generateRefCurves(gender string, records []model.GCGrowthRecord) map[string][]map[string]float64 {
	if len(records) == 0 {
		return nil
	}
	percentiles := []float64{3, 10, 25, 50, 75, 90, 97}
	zValues := []float64{-1.88, -1.28, -0.67, 0, 0.67, 1.28, 1.88}

	curves := make(map[string][]map[string]float64)
	for i, pct := range percentiles {
		key := fmt.Sprintf("P%.0f", pct)
		var pts []map[string]float64
		minAge := records[0].AgeMonths
		maxAge := records[len(records)-1].AgeMonths
		for age := minAge; age <= maxAge; age += 1 {
			_, m, s := getHeightLMS(gender, age)
			if m > 0 {
				val := m * math.Pow(1+zValues[i]*s, 1) // simplified for L≈1
				pts = append(pts, map[string]float64{"age": age, "height": roundTo(val, 1)})
			}
		}
		curves[key] = pts
	}
	return curves
}

// ═══════════════════════════════════════════════════════════════
// 3. Growth Velocity — cm/year over 3/6/12 months
// ═══════════════════════════════════════════════════════════════

func (t *GrowthClinicTool) growthVelocity(userID string, params map[string]interface{}) (string, error) {
	patientID, _ := params["patient_id"].(string)
	if patientID == "" {
		return "", fmt.Errorf("patient_id is required")
	}

	var records []model.GCGrowthRecord
	t.db.Where("user_id = ? AND patient_id = ? AND height > 0", userID, patientID).
		Order("record_date DESC").Limit(50).Find(&records)

	if len(records) < 2 {
		return jsonStr(map[string]string{"message": "需要至少2次身高记录才能计算生长速率"}), nil
	}

	latest := records[0]
	result := map[string]interface{}{
		"latest_height": latest.Height,
		"latest_date":   latest.RecordDate.Format("2006-01-02"),
	}

	// Calculate velocity for different periods
	for _, months := range []int{3, 6, 12} {
		cutoff := latest.RecordDate.AddDate(0, -months, 0)
		var best model.GCGrowthRecord
		for _, r := range records[1:] {
			if r.RecordDate.Before(cutoff.AddDate(0, 0, 15)) && r.RecordDate.After(cutoff.AddDate(0, 0, -15)) {
				best = r
				break
			}
		}
		if best.Height > 0 {
			days := latest.RecordDate.Sub(best.RecordDate).Hours() / 24
			if days > 30 {
				velocity := (latest.Height - best.Height) / days * 365
				key := fmt.Sprintf("velocity_%dm_cm_year", months)
				result[key] = roundTo(velocity, 1)
			}
		}
	}

	// Also calculate from earliest to latest annualized
	earliest := records[len(records)-1]
	totalDays := latest.RecordDate.Sub(earliest.RecordDate).Hours() / 24
	if totalDays > 60 {
		annualized := (latest.Height - earliest.Height) / totalDays * 365
		result["velocity_overall_cm_year"] = roundTo(annualized, 1)
	}

	// Normal ranges by age
	ageYears := latest.AgeMonths / 12
	result["normal_range"] = normalVelocityRange(ageYears)

	return jsonStr(result), nil
}

func normalVelocityRange(ageYears float64) string {
	switch {
	case ageYears < 1:
		return "25cm/年（0-1岁正常范围）"
	case ageYears < 2:
		return "10-12cm/年（1-2岁正常范围）"
	case ageYears < 3:
		return "8-10cm/年（2-3岁正常范围）"
	case ageYears < 10:
		return "5-7cm/年（3-10岁正常范围）"
	case ageYears < 14:
		return "7-12cm/年（青春期正常范围）"
	default:
		return "1-3cm/年（青春后期正常范围）"
	}
}

// ═══════════════════════════════════════════════════════════════
// 4. BMI Assessment
// ═══════════════════════════════════════════════════════════════

func (t *GrowthClinicTool) bmiAssess(params map[string]interface{}) (string, error) {
	height := toFloat(params["height"])
	weight := toFloat(params["weight"])
	gender, _ := params["gender"].(string)
	ageMonths := toFloat(params["age_months"])

	if height <= 0 || weight <= 0 {
		return "", fmt.Errorf("height and weight are required")
	}

	hm := height / 100
	bmi := weight / (hm * hm)

	result := map[string]interface{}{
		"height_cm": height,
		"weight_kg": weight,
		"bmi":       roundTo(bmi, 1),
	}

	if gender != "" && ageMonths > 0 {
		z, pct := bmiForAgeZScore(gender, ageMonths, bmi)
		result["z_score"] = roundTo(z, 2)
		result["percentile"] = roundTo(pct, 1)
		result["category"] = interpretBMIZScore(z)

		// Ideal weight range
		_, m, s := getBMI_LMS(gender, ageMonths)
		idealBMILow := m * math.Pow(1-1*s, 1)
		idealBMIHigh := m * math.Pow(1+1*s, 1)
		result["ideal_bmi_range"] = fmt.Sprintf("%.1f - %.1f", idealBMILow, idealBMIHigh)
		result["ideal_weight_range_kg"] = fmt.Sprintf("%.1f - %.1f", idealBMILow*hm*hm, idealBMIHigh*hm*hm)
	}

	return jsonStr(result), nil
}

// ═══════════════════════════════════════════════════════════════
// 5. Target Height (遗传靶身高)
// ═══════════════════════════════════════════════════════════════

func (t *GrowthClinicTool) targetHeight(params map[string]interface{}) (string, error) {
	fatherH := toFloat(params["father_height"])
	motherH := toFloat(params["mother_height"])
	gender, _ := params["gender"].(string)

	if fatherH <= 0 || motherH <= 0 || gender == "" {
		return "", fmt.Errorf("father_height, mother_height, and gender are required")
	}

	// FPH (Familial target height) formula:
	// Male:   (father + mother + 13) / 2
	// Female: (father + mother - 13) / 2
	var target float64
	if gender == "male" {
		target = (fatherH + motherH + 13) / 2
	} else {
		target = (fatherH + motherH - 13) / 2
	}

	// ±8cm range (2SD)
	result := map[string]interface{}{
		"father_height_cm":    fatherH,
		"mother_height_cm":    motherH,
		"gender":              gender,
		"target_height_cm":    roundTo(target, 1),
		"range_low_cm":        roundTo(target-8, 1),
		"range_high_cm":       roundTo(target+8, 1),
		"formula":             formulaStr(gender),
		"note":                "遗传靶身高仅供参考，实际身高受营养、运动、睡眠、疾病等多因素影响",
		"deviation_threshold": "偏离遗传靶身高>8cm需重点关注",
	}

	return jsonStr(result), nil
}

func formulaStr(gender string) string {
	if gender == "male" {
		return "(父亲身高 + 母亲身高 + 13) / 2"
	}
	return "(父亲身高 + 母亲身高 - 13) / 2"
}

// ═══════════════════════════════════════════════════════════════
// 6. Bone Age Compare
// ═══════════════════════════════════════════════════════════════

func (t *GrowthClinicTool) boneAgeCompare(params map[string]interface{}) (string, error) {
	boneAge := toFloat(params["bone_age"])
	chronoAge := toFloat(params["chrono_age"])
	gender, _ := params["gender"].(string)
	height := toFloat(params["height"])

	if boneAge <= 0 || chronoAge <= 0 {
		return "", fmt.Errorf("bone_age and chrono_age (in years) are required")
	}

	diff := boneAge - chronoAge

	var status, interpretation string
	switch {
	case diff < -2:
		status = "明显落后"
		interpretation = "骨龄显著落后于实际年龄，生长潜力较大，但需排除生长激素缺乏、甲减等病因"
	case diff < -1:
		status = "轻度落后"
		interpretation = "骨龄略落后于实际年龄，提示生长潜力尚可"
	case diff <= 1:
		status = "正常"
		interpretation = "骨龄与实际年龄匹配，发育进度正常"
	case diff <= 2:
		status = "轻度超前"
		interpretation = "骨龄略超前于实际年龄，需关注性早熟可能"
	default:
		status = "明显超前"
		interpretation = "骨龄显著超前，强烈建议排查性早熟或肾上腺疾病，可能影响终身高"
	}

	// Predicted adult height (simplified Bayley-Pinneau method)
	var predictedHeight float64
	if height > 0 {
		// Approximate: current height / (bone age maturity %)
		maturity := boneAgeMaturityPercent(gender, boneAge)
		if maturity > 0 {
			predictedHeight = height / (maturity / 100)
		}
	}

	result := map[string]interface{}{
		"bone_age_years":   boneAge,
		"chrono_age_years": chronoAge,
		"difference_years": roundTo(diff, 1),
		"status":           status,
		"interpretation":   interpretation,
	}
	if predictedHeight > 0 {
		result["predicted_adult_height_cm"] = roundTo(predictedHeight, 1)
		result["prediction_note"] = "预测身高基于简化Bayley-Pinneau法，仅供参考"
	}

	return jsonStr(result), nil
}

// boneAgeMaturityPercent returns approximate skeletal maturity percentage.
func boneAgeMaturityPercent(gender string, boneAge float64) float64 {
	// Simplified lookup: percentage of adult height achieved at given bone age
	// Based on Bayley-Pinneau tables
	type bp struct{ age, male, female float64 }
	table := []bp{
		{1, 42.2, 44.7}, {2, 49.5, 52.8}, {3, 53.8, 57.0},
		{4, 58.0, 61.8}, {5, 61.8, 66.2}, {6, 65.2, 70.3},
		{7, 69.0, 74.0}, {8, 72.0, 77.5}, {9, 75.0, 80.7},
		{10, 78.0, 84.4}, {11, 81.1, 88.4}, {12, 84.2, 92.9},
		{13, 87.3, 96.5}, {14, 91.5, 98.3}, {15, 95.1, 99.1},
		{16, 97.8, 99.6}, {17, 99.0, 100}, {18, 99.6, 100},
	}
	for _, row := range table {
		if boneAge <= row.age {
			if gender == "male" {
				return row.male
			}
			return row.female
		}
	}
	return 100
}

// ═══════════════════════════════════════════════════════════════
// 7. Puberty Assessment (Tanner Staging + Precocious Puberty)
// ═══════════════════════════════════════════════════════════════

func (t *GrowthClinicTool) pubertyAssess(params map[string]interface{}) (string, error) {
	gender, _ := params["gender"].(string)
	ageYears := toFloat(params["age_years"])
	tannerBreast, _ := params["tanner_breast"].(string)        // B1-B5 (female)
	tannerGenital, _ := params["tanner_genital"].(string)      // G1-G5 (male)
	tannerPubicHair, _ := params["tanner_pubic_hair"].(string) // PH1-PH5
	menarche, _ := params["menarche"].(bool)                   // 是否已月经初潮

	if gender == "" || ageYears <= 0 {
		return "", fmt.Errorf("gender and age_years are required")
	}

	result := map[string]interface{}{
		"gender":    gender,
		"age_years": ageYears,
	}

	var alerts []string
	var stage string

	if gender == "female" {
		stage = tannerBreast
		if stage == "" {
			stage = "未评估"
		}
		result["tanner_breast"] = stage
		result["tanner_pubic_hair"] = tannerPubicHair
		result["menarche"] = menarche

		// 性早熟判定: 女孩8岁前乳房发育
		if ageYears < 8 && stage != "B1" && stage != "" && stage != "未评估" {
			alerts = append(alerts, "⚠️ 性早熟警告：女孩8岁前出现乳房发育（"+stage+"），建议立即内分泌科就诊")
		}
		if ageYears < 10 && menarche {
			alerts = append(alerts, "⚠️ 性早熟警告：10岁前月经初潮，建议立即就诊")
		}
	} else {
		stage = tannerGenital
		if stage == "" {
			stage = "未评估"
		}
		result["tanner_genital"] = stage
		result["tanner_pubic_hair"] = tannerPubicHair

		// 男孩9岁前开始发育
		if ageYears < 9 && stage != "G1" && stage != "" && stage != "未评估" {
			alerts = append(alerts, "⚠️ 性早熟警告：男孩9岁前出现睾丸增大（"+stage+"），建议立即内分泌科就诊")
		}
	}

	// 快进展判断(6个月内进入下一Tanner阶段)
	lastAssess, _ := params["last_tanner_stage"].(string)
	lastDate, _ := params["last_assess_date"].(string)
	if lastAssess != "" && lastDate != "" {
		if lt, err := time.Parse("2006-01-02", lastDate); err == nil {
			monthsDiff := time.Since(lt).Hours() / 24 / 30
			if monthsDiff < 6 && tannerAdvanced(lastAssess, stage) {
				alerts = append(alerts, "⚠️ 青春期快进展：不到6个月内Tanner分期从"+lastAssess+"进展到"+stage+"，建议评估GnRHa干预")
			}
		}
	}

	if len(alerts) > 0 {
		result["alerts"] = alerts
		result["alert_level"] = "moderate"
	} else {
		result["assessment"] = "发育进度正常"
		result["alert_level"] = "normal"
	}

	return jsonStr(result), nil
}

func tannerAdvanced(old, new string) bool {
	stages := map[string]int{"B1": 1, "B2": 2, "B3": 3, "B4": 4, "B5": 5,
		"G1": 1, "G2": 2, "G3": 3, "G4": 4, "G5": 5,
		"PH1": 1, "PH2": 2, "PH3": 3, "PH4": 4, "PH5": 5}
	return stages[new] > stages[old]
}

// ═══════════════════════════════════════════════════════════════
// 8. Four-Level Alert Evaluation
// ═══════════════════════════════════════════════════════════════

func (t *GrowthClinicTool) alertEvaluate(userID string, params map[string]interface{}) (string, error) {
	patientID, _ := params["patient_id"].(string)
	if patientID == "" {
		return "", fmt.Errorf("patient_id is required")
	}

	// Get latest record
	var record model.GCGrowthRecord
	if err := t.db.Where("user_id = ? AND patient_id = ?", userID, patientID).
		Order("record_date DESC").First(&record).Error; err != nil {
		return "", fmt.Errorf("no growth records found")
	}

	var patient model.GCPatient
	t.db.Where("id = ?", patientID).First(&patient)

	alerts := []map[string]interface{}{}
	overallLevel := model.GCAlertNormal

	// Height Z-score check
	if record.HeightZScore != 0 {
		level, title, desc := classifyZAlert(record.HeightZScore, "身高")
		if level != model.GCAlertNormal {
			alerts = append(alerts, map[string]interface{}{
				"category": "height", "level": string(level),
				"title": title, "description": desc,
				"z_score": record.HeightZScore, "percentile": record.HeightPercentile,
			})
			if alertSeverity(level) > alertSeverity(overallLevel) {
				overallLevel = level
			}
		}
	}

	// Weight Z-score check
	if record.WeightZScore != 0 {
		level, title, desc := classifyZAlert(record.WeightZScore, "体重")
		if level != model.GCAlertNormal {
			alerts = append(alerts, map[string]interface{}{
				"category": "weight", "level": string(level),
				"title": title, "description": desc,
			})
			if alertSeverity(level) > alertSeverity(overallLevel) {
				overallLevel = level
			}
		}
	}

	// BMI check
	if record.BMIZScore != 0 {
		if record.BMIZScore > 2 || record.BMIZScore < -2 {
			level := model.GCAlertModerate
			cat := "超重/肥胖"
			if record.BMIZScore < -2 {
				cat = "消瘦"
			}
			alerts = append(alerts, map[string]interface{}{
				"category": "bmi", "level": string(level),
				"title":   "BMI异常: " + cat,
				"z_score": record.BMIZScore,
			})
			if alertSeverity(level) > alertSeverity(overallLevel) {
				overallLevel = level
			}
		}
	}

	// Bone age check
	if record.BoneAge > 0 {
		chronoAge := record.AgeMonths / 12
		diff := record.BoneAge - chronoAge
		if diff > 2 || diff < -2 {
			level := model.GCAlertModerate
			if diff > 3 || diff < -3 {
				level = model.GCAlertSevere
			}
			alerts = append(alerts, map[string]interface{}{
				"category": "bone_age", "level": string(level),
				"title":    fmt.Sprintf("骨龄偏差: %.1f年", diff),
				"bone_age": record.BoneAge, "chrono_age": chronoAge,
			})
			if alertSeverity(level) > alertSeverity(overallLevel) {
				overallLevel = level
			}
		}
	}

	// Growth velocity check
	if record.GrowthVelocity > 0 {
		ageYears := record.AgeMonths / 12
		if isVelocityLow(ageYears, record.GrowthVelocity) {
			level := model.GCAlertMild
			alerts = append(alerts, map[string]interface{}{
				"category": "velocity", "level": string(level),
				"title":  fmt.Sprintf("生长速率偏低: %.1f cm/年", record.GrowthVelocity),
				"normal": normalVelocityRange(ageYears),
			})
			if alertSeverity(level) > alertSeverity(overallLevel) {
				overallLevel = level
			}
		}
	}

	// Target height deviation
	if patient.TargetHeight > 0 && record.Height > 0 {
		// Predict adult height from current trajectory
		_, m, _ := getHeightLMS(patient.Gender, record.AgeMonths)
		if m > 0 {
			ratio := record.Height / m
			predictedAdult := ratio * adultMedianHeight(patient.Gender)
			deviation := predictedAdult - patient.TargetHeight
			if math.Abs(deviation) > 8 {
				alerts = append(alerts, map[string]interface{}{
					"category":  "target_height",
					"level":     string(model.GCAlertMild),
					"title":     fmt.Sprintf("预测身高偏离遗传靶身高 %.1fcm", deviation),
					"predicted": roundTo(predictedAdult, 1),
					"target":    patient.TargetHeight,
				})
			}
		}
	}

	// Agent recommended actions per level
	agentAction := agentActionForLevel(overallLevel)

	// Persist alerts
	for _, a := range alerts {
		alert := model.GCAlert{
			UserID:      userID,
			PatientID:   patientID,
			RecordID:    record.ID,
			Level:       model.GCAlertLevel(a["level"].(string)),
			Category:    a["category"].(string),
			Title:       a["title"].(string),
			AgentAction: agentAction,
			Status:      "pending",
		}
		t.db.Create(&alert)
	}

	return jsonStr(map[string]interface{}{
		"patient":       patient.Name,
		"overall_level": string(overallLevel),
		"alerts":        alerts,
		"agent_action":  agentAction,
		"record_date":   record.RecordDate.Format("2006-01-02"),
	}), nil
}

func classifyZAlert(z float64, metric string) (model.GCAlertLevel, string, string) {
	absZ := math.Abs(z)
	direction := "偏低"
	if z > 0 {
		direction = "偏高"
	}
	switch {
	case absZ < 1:
		return model.GCAlertNormal, "", ""
	case absZ < 2:
		return model.GCAlertMild,
			fmt.Sprintf("%s略%s (Z=%.2f)", metric, direction, z),
			fmt.Sprintf("%sZ评分在±1SD~±2SD之间，建议关注", metric)
	case absZ < 3:
		return model.GCAlertModerate,
			fmt.Sprintf("%s%s (Z=%.2f)", metric, direction, z),
			fmt.Sprintf("%sZ评分在±2SD~±3SD之间，需要干预", metric)
	default:
		return model.GCAlertSevere,
			fmt.Sprintf("%s严重%s (Z=%.2f) — 危急值", metric, direction, z),
			fmt.Sprintf("%sZ评分超过±3SD，需紧急处理", metric)
	}
}

func alertSeverity(l model.GCAlertLevel) int {
	switch l {
	case model.GCAlertNormal:
		return 0
	case model.GCAlertMild:
		return 1
	case model.GCAlertModerate:
		return 2
	case model.GCAlertSevere:
		return 3
	}
	return 0
}

func agentActionForLevel(level model.GCAlertLevel) string {
	switch level {
	case model.GCAlertNormal:
		return "自动归档数据，推送日常保健建议"
	case model.GCAlertMild:
		return "自动推送针对性宣教和生活方式调整建议，标记'关注'"
	case model.GCAlertModerate:
		return "推入医生'待审核'列表，自动建议加密随访频次，提醒家长重视"
	case model.GCAlertSevere:
		return "触发红色强预警，推入医生紧急处理列表，同步推送家长'立即复诊'提醒"
	}
	return ""
}

func isVelocityLow(ageYears, velocity float64) bool {
	switch {
	case ageYears < 1:
		return velocity < 20
	case ageYears < 2:
		return velocity < 8
	case ageYears < 3:
		return velocity < 6
	case ageYears < 10:
		return velocity < 4
	case ageYears < 14:
		return velocity < 5
	default:
		return velocity < 1
	}
}

func adultMedianHeight(gender string) float64 {
	if gender == "male" {
		return 172.1 // China male adult median
	}
	return 160.1 // China female adult median
}
