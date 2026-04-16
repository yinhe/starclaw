package tool

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
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
		"father_height_cm":   fatherH,
		"mother_height_cm":   motherH,
		"gender":             gender,
		"target_height_cm":   roundTo(target, 1),
		"range_low_cm":       roundTo(target-8, 1),
		"range_high_cm":      roundTo(target+8, 1),
		"formula":            formulaStr(gender),
		"note":               "遗传靶身高仅供参考，实际身高受营养、运动、睡眠、疾病等多因素影响",
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
	tannerBreast, _ := params["tanner_breast"].(string)  // B1-B5 (female)
	tannerGenital, _ := params["tanner_genital"].(string) // G1-G5 (male)
	tannerPubicHair, _ := params["tanner_pubic_hair"].(string) // PH1-PH5
	menarche, _ := params["menarche"].(bool) // 是否已月经初潮

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
				"title": "BMI异常: " + cat,
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
				"title":    fmt.Sprintf("生长速率偏低: %.1f cm/年", record.GrowthVelocity),
				"normal":   normalVelocityRange(ageYears),
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

// ═══════════════════════════════════════════════════════════════
// 9. Patient Record CRUD
// ═══════════════════════════════════════════════════════════════

func (t *GrowthClinicTool) patientRecord(userID string, params map[string]interface{}) (string, error) {
	op, _ := params["operation"].(string)
	switch op {
	case "create":
		return t.createPatient(userID, params)
	case "get":
		return t.getPatient(userID, params)
	case "list":
		return t.listPatients(userID)
	case "update":
		return t.updatePatient(userID, params)
	case "add_record":
		return t.addGrowthRecord(userID, params)
	default:
		return t.listPatients(userID)
	}
}

func (t *GrowthClinicTool) createPatient(userID string, params map[string]interface{}) (string, error) {
	name, _ := params["name"].(string)
	gender, _ := params["gender"].(string)
	birthStr, _ := params["birth_date"].(string)

	if name == "" || gender == "" || birthStr == "" {
		return "", fmt.Errorf("name, gender, and birth_date are required")
	}

	birthDate, err := time.Parse("2006-01-02", birthStr)
	if err != nil {
		return "", fmt.Errorf("invalid birth_date format, expected YYYY-MM-DD")
	}

	p := model.GCPatient{
		UserID:    userID,
		Name:      name,
		Gender:    gender,
		BirthDate: birthDate,
		Status:    "active",
	}

	if fh := toFloat(params["father_height"]); fh > 0 {
		p.FatherHeight = fh
	}
	if mh := toFloat(params["mother_height"]); mh > 0 {
		p.MotherHeight = mh
	}
	if p.FatherHeight > 0 && p.MotherHeight > 0 {
		if gender == "male" {
			p.TargetHeight = (p.FatherHeight + p.MotherHeight + 13) / 2
		} else {
			p.TargetHeight = (p.FatherHeight + p.MotherHeight - 13) / 2
		}
	}
	if gn, ok := params["guardian_name"].(string); ok {
		p.GuardianName = gn
	}
	if gp, ok := params["guardian_phone"].(string); ok {
		p.GuardianPhone = gp
	}

	if err := t.db.Create(&p).Error; err != nil {
		return "", fmt.Errorf("create patient failed: %v", err)
	}

	return jsonStr(map[string]interface{}{"patient": p, "message": "患者档案创建成功"}), nil
}

func (t *GrowthClinicTool) getPatient(userID string, params map[string]interface{}) (string, error) {
	id, _ := params["patient_id"].(string)
	var p model.GCPatient
	if err := t.db.Where("id = ? AND user_id = ?", id, userID).First(&p).Error; err != nil {
		return "", fmt.Errorf("patient not found")
	}

	var recordCount int64
	t.db.Model(&model.GCGrowthRecord{}).Where("patient_id = ?", id).Count(&recordCount)

	var alertCount int64
	t.db.Model(&model.GCAlert{}).Where("patient_id = ? AND status = ?", id, "pending").Count(&alertCount)

	ageMonths := time.Since(p.BirthDate).Hours() / 24 / 30.4375

	return jsonStr(map[string]interface{}{
		"patient":        p,
		"age_months":     roundTo(ageMonths, 1),
		"age_years":      roundTo(ageMonths/12, 1),
		"record_count":   recordCount,
		"pending_alerts": alertCount,
	}), nil
}

func (t *GrowthClinicTool) listPatients(userID string) (string, error) {
	var patients []model.GCPatient
	t.db.Where("user_id = ?", userID).Order("created_at DESC").Limit(100).Find(&patients)

	items := make([]map[string]interface{}, 0, len(patients))
	for _, p := range patients {
		ageMonths := time.Since(p.BirthDate).Hours() / 24 / 30.4375
		items = append(items, map[string]interface{}{
			"id":         p.ID,
			"name":       p.Name,
			"gender":     p.Gender,
			"age_months": roundTo(ageMonths, 1),
			"status":     p.Status,
			"diagnosis":  p.Diagnosis,
		})
	}
	return jsonStr(map[string]interface{}{"patients": items, "total": len(items)}), nil
}

func (t *GrowthClinicTool) updatePatient(userID string, params map[string]interface{}) (string, error) {
	id, _ := params["patient_id"].(string)
	var p model.GCPatient
	if err := t.db.Where("id = ? AND user_id = ?", id, userID).First(&p).Error; err != nil {
		return "", fmt.Errorf("patient not found")
	}

	updates := map[string]interface{}{}
	if v, ok := params["diagnosis"].(string); ok {
		updates["diagnosis"] = v
	}
	if v, ok := params["treatment_plan"].(string); ok {
		updates["treatment_plan"] = v
	}
	if v, ok := params["status"].(string); ok {
		updates["status"] = v
	}
	if v, ok := params["notes"].(string); ok {
		updates["notes"] = v
	}

	if len(updates) > 0 {
		t.db.Model(&p).Updates(updates)
	}

	return jsonStr(map[string]interface{}{"message": "patient updated", "patient_id": id}), nil
}

func (t *GrowthClinicTool) addGrowthRecord(userID string, params map[string]interface{}) (string, error) {
	patientID, _ := params["patient_id"].(string)
	if patientID == "" {
		return "", fmt.Errorf("patient_id is required")
	}

	var patient model.GCPatient
	if err := t.db.Where("id = ? AND user_id = ?", patientID, userID).First(&patient).Error; err != nil {
		return "", fmt.Errorf("patient not found")
	}

	now := time.Now()
	ageMonths := now.Sub(patient.BirthDate).Hours() / 24 / 30.4375

	rec := model.GCGrowthRecord{
		UserID:     userID,
		PatientID:  patientID,
		RecordDate: now,
		AgeMonths:  ageMonths,
		Height:     toFloat(params["height"]),
		HeightMorning: toFloat(params["height_morning"]),
		HeightEvening: toFloat(params["height_evening"]),
		Weight:     toFloat(params["weight"]),
		BoneAge:    toFloat(params["bone_age"]),
		SleepHours: toFloat(params["sleep_hours"]),
		ExerciseMinutes: int(toFloat(params["exercise_minutes"])),
		ExerciseWeekly:  int(toFloat(params["exercise_weekly"])),
		DataSource: "manual",
	}

	if ts, ok := params["tanner_stage"].(string); ok {
		rec.TannerStage = ts
	}
	if st, ok := params["sleep_time"].(string); ok {
		rec.SleepTime = st
	}
	if et, ok := params["exercise_type"].(string); ok {
		rec.ExerciseType = et
	}
	if dn, ok := params["diet_note"].(string); ok {
		rec.DietNote = dn
	}
	if ds, ok := params["data_source"].(string); ok {
		rec.DataSource = ds
	}

	// Auto-compute Z-scores
	if rec.Height > 0 {
		hz, hp := heightForAgeZScore(patient.Gender, ageMonths, rec.Height)
		rec.HeightZScore = roundTo(hz, 2)
		rec.HeightPercentile = roundTo(hp, 1)
	}
	if rec.Weight > 0 {
		wz, _ := weightForAgeZScore(patient.Gender, ageMonths, rec.Weight)
		rec.WeightZScore = roundTo(wz, 2)
	}
	if rec.BMI > 0 {
		bz, _ := bmiForAgeZScore(patient.Gender, ageMonths, rec.BMI)
		rec.BMIZScore = roundTo(bz, 2)
	}

	// Growth velocity from previous record
	var prev model.GCGrowthRecord
	if err := t.db.Where("user_id = ? AND patient_id = ? AND height > 0", userID, patientID).
		Order("record_date DESC").First(&prev).Error; err == nil && rec.Height > 0 {
		days := rec.RecordDate.Sub(prev.RecordDate).Hours() / 24
		if days > 30 {
			rec.GrowthVelocity = roundTo((rec.Height-prev.Height)/days*365, 1)
		}
	}

	if err := t.db.Create(&rec).Error; err != nil {
		return "", fmt.Errorf("failed to save record: %v", err)
	}

	return jsonStr(map[string]interface{}{
		"record":           rec,
		"message":          "生长数据记录成功",
		"auto_z_scores":    true,
		"auto_velocity":    rec.GrowthVelocity > 0,
	}), nil
}

// ═══════════════════════════════════════════════════════════════
// 10. Follow-up Schedule Management
// ═══════════════════════════════════════════════════════════════

func (t *GrowthClinicTool) followupSchedule(userID string, params map[string]interface{}) (string, error) {
	op, _ := params["operation"].(string)
	switch op {
	case "create":
		return t.createPlan(userID, params)
	case "update":
		return t.updatePlan(userID, params)
	case "list":
		return t.listPlans(userID, params)
	case "next":
		return t.nextFollowups(userID)
	default:
		return t.nextFollowups(userID)
	}
}

func (t *GrowthClinicTool) createPlan(userID string, params map[string]interface{}) (string, error) {
	patientID, _ := params["patient_id"].(string)
	if patientID == "" {
		return "", fmt.Errorf("patient_id is required")
	}

	days := int(toFloat(params["frequency_days"]))
	if days <= 0 {
		days = 90 // default 3 months
	}
	mode, _ := params["frequency_mode"].(string)
	if mode == "" {
		mode = "smart"
	}

	next := time.Now().AddDate(0, 0, days)
	plan := model.GCFollowupPlan{
		UserID:         userID,
		PatientID:      patientID,
		FrequencyMode:  mode,
		FrequencyDays:  days,
		Status:         "active",
		NextFollowupAt: &next,
	}

	if err := t.db.Create(&plan).Error; err != nil {
		return "", fmt.Errorf("create plan failed: %v", err)
	}

	return jsonStr(map[string]interface{}{
		"plan":    plan,
		"message": fmt.Sprintf("随访计划创建成功，下次随访: %s", next.Format("2006-01-02")),
	}), nil
}

func (t *GrowthClinicTool) updatePlan(userID string, params map[string]interface{}) (string, error) {
	planID, _ := params["plan_id"].(string)
	var plan model.GCFollowupPlan
	if err := t.db.Where("id = ? AND user_id = ?", planID, userID).First(&plan).Error; err != nil {
		return "", fmt.Errorf("plan not found")
	}

	updates := map[string]interface{}{}
	if v, ok := params["status"].(string); ok {
		updates["status"] = v
	}
	if v := int(toFloat(params["frequency_days"])); v > 0 {
		updates["frequency_days"] = v
		next := time.Now().AddDate(0, 0, v)
		updates["next_followup_at"] = &next
	}
	if v, ok := params["change_reason"].(string); ok {
		updates["change_reason"] = v
	}

	t.db.Model(&plan).Updates(updates)
	return jsonStr(map[string]interface{}{"message": "plan updated", "plan_id": planID}), nil
}

func (t *GrowthClinicTool) listPlans(userID string, params map[string]interface{}) (string, error) {
	patientID, _ := params["patient_id"].(string)
	q := t.db.Where("user_id = ?", userID)
	if patientID != "" {
		q = q.Where("patient_id = ?", patientID)
	}
	var plans []model.GCFollowupPlan
	q.Order("next_followup_at ASC").Limit(50).Find(&plans)
	return jsonStr(map[string]interface{}{"plans": plans, "total": len(plans)}), nil
}

func (t *GrowthClinicTool) nextFollowups(userID string) (string, error) {
	var plans []model.GCFollowupPlan
	t.db.Where("user_id = ? AND status = ? AND next_followup_at <= ?",
		userID, "active", time.Now().AddDate(0, 0, 7)).
		Order("next_followup_at ASC").Limit(20).Find(&plans)

	items := make([]map[string]interface{}, 0, len(plans))
	for _, p := range plans {
		var patient model.GCPatient
		t.db.Where("id = ?", p.PatientID).First(&patient)
		due := "upcoming"
		if p.NextFollowupAt != nil && p.NextFollowupAt.Before(time.Now()) {
			due = "overdue"
		}
		items = append(items, map[string]interface{}{
			"plan_id":      p.ID,
			"patient_name": patient.Name,
			"patient_id":   p.PatientID,
			"next_date":    p.NextFollowupAt,
			"status":       due,
		})
	}
	return jsonStr(map[string]interface{}{"upcoming_followups": items, "total": len(items)}), nil
}

// ═══════════════════════════════════════════════════════════════
// 11. Data Quality Check (三级质控)
// ═══════════════════════════════════════════════════════════════

func (t *GrowthClinicTool) dataQualityCheck(params map[string]interface{}) (string, error) {
	height := toFloat(params["height"])
	weight := toFloat(params["weight"])
	ageMonths := toFloat(params["age_months"])
	prevHeight := toFloat(params["prev_height"])
	prevDaysAgo := toFloat(params["prev_days_ago"])

	issues := []string{}
	quality := "normal"

	// Level 1: Range validation
	if height > 0 {
		if height < 30 || height > 250 {
			issues = append(issues, "身高超出合理范围(30-250cm)")
			quality = "invalid"
		}
		if ageMonths > 0 && ageMonths < 60 && height > 150 {
			issues = append(issues, "5岁以下身高不应超过150cm，请复核")
			quality = "suspect"
		}
	}

	if weight > 0 {
		if weight < 1 || weight > 200 {
			issues = append(issues, "体重超出合理范围(1-200kg)")
			quality = "invalid"
		}
	}

	// Level 2: Change rate validation
	if prevHeight > 0 && height > 0 && prevDaysAgo > 0 {
		monthlyGrowth := (height - prevHeight) / prevDaysAgo * 30
		if monthlyGrowth > 5 {
			issues = append(issues, fmt.Sprintf("月增长%.1fcm，超过正常范围(≤2cm/月)，标记待复核", monthlyGrowth))
			if quality == "normal" {
				quality = "suspect"
			}
		}
		if monthlyGrowth < -1 {
			issues = append(issues, fmt.Sprintf("身高减少%.1fcm，可能测量误差，建议复测", -monthlyGrowth))
			if quality == "normal" {
				quality = "suspect"
			}
		}
	}

	// Level 3: Source credibility
	source, _ := params["data_source"].(string)
	credibility := "medium"
	switch source {
	case "device":
		credibility = "high"
	case "his", "ocr":
		credibility = "medium"
	case "manual":
		credibility = "low"
	}

	result := map[string]interface{}{
		"quality":     quality,
		"credibility": credibility,
		"issues":      issues,
		"pass":        quality != "invalid",
	}

	if quality == "suspect" {
		result["recommendation"] = "建议家长复测后重新上报"
	} else if quality == "invalid" {
		result["recommendation"] = "数据无效，请重新输入正确数据"
	}

	return jsonStr(result), nil
}

// ═══════════════════════════════════════════════════════════════
// 12. Education Push
// ═══════════════════════════════════════════════════════════════

func (t *GrowthClinicTool) educationPush(userID string, params map[string]interface{}) (string, error) {
	patientID, _ := params["patient_id"].(string)
	category, _ := params["category"].(string) // diet/exercise/sleep/puberty/general
	alertLevel, _ := params["alert_level"].(string)

	if category == "" {
		category = "general"
	}

	content := getEducationContent(category, alertLevel)

	if patientID != "" {
		now := time.Now()
		edu := model.GCEducation{
			UserID:    userID,
			PatientID: patientID,
			Category:  category,
			Title:     content.title,
			Content:   content.body,
			Channel:   "app",
			Status:    "sent",
			SentAt:    &now,
		}
		t.db.Create(&edu)
	}

	return jsonStr(map[string]interface{}{
		"category": category,
		"title":    content.title,
		"content":  content.body,
		"sent":     patientID != "",
	}), nil
}

type eduContent struct {
	title string
	body  string
}

func getEducationContent(category, alertLevel string) eduContent {
	contents := map[string]eduContent{
		"diet": {
			title: "儿童营养膳食指导",
			body: "【均衡饮食建议】\n" +
				"1. 每日奶量：3-6岁 300-400ml，7-12岁 300ml\n" +
				"2. 优质蛋白：每日1个鸡蛋 + 适量鱼/虾/瘦肉\n" +
				"3. 蔬果搭配：每日蔬菜200-300g（深色蔬菜占半），水果100-200g\n" +
				"4. 主食粗细搭配：杂粮占1/3\n" +
				"5. 避免：含糖饮料、油炸食品、过度零食\n" +
				"6. 钙质补充：奶制品+绿叶蔬菜+适量豆制品\n\n" +
				"⚠️ 以上为一般性建议，具体请遵医嘱",
		},
		"exercise": {
			title: "儿童运动促长方案",
			body: "【运动指导】\n" +
				"1. 推荐运动：跳绳、篮球、游泳、跑步、摸高跳\n" +
				"2. 运动频次：每周≥5次，每次≥30分钟\n" +
				"3. 运动强度：中等强度为主（微微出汗、心率加快）\n" +
				"4. 跳绳方案：每天500-1000个，可分组完成\n" +
				"5. 注意事项：\n" +
				"   - 避免空腹运动和饭后立即运动\n" +
				"   - 运动后补充水分和营养\n" +
				"   - 关节不适时及时休息\n" +
				"   - 雨天可进行室内拉伸、跳绳等替代运动\n\n" +
				"⚠️ 以上为一般性建议，具体请遵医嘱",
		},
		"sleep": {
			title: "儿童睡眠管理指导",
			body: "【睡眠管理】\n" +
				"生长激素分泌高峰在深睡眠期（22:00-2:00），充足睡眠对身高至关重要。\n\n" +
				"推荐睡眠时长：\n" +
				"- 3-5岁：10-13小时（含午睡）\n" +
				"- 6-12岁：9-12小时\n" +
				"- 13-18岁：8-10小时\n\n" +
				"改善建议：\n" +
				"1. 固定作息：每天同一时间入睡和起床\n" +
				"2. 睡前1小时避免电子屏幕\n" +
				"3. 睡前避免剧烈运动和大量进食\n" +
				"4. 营造安静、黑暗、凉爽的睡眠环境\n" +
				"5. 建议21:00前入睡（学龄儿童）\n\n" +
				"⚠️ 以上为一般性建议，具体请遵医嘱",
		},
		"puberty": {
			title: "青春期发育科普",
			body: "【青春期发育知识】\n\n" +
				"正常青春期启动年龄：\n" +
				"- 女孩：8-13岁开始乳房发育\n" +
				"- 男孩：9-14岁开始睾丸增大\n\n" +
				"需要关注的信号：\n" +
				"1. 性早熟：女孩8岁前/男孩9岁前出现第二性征\n" +
				"2. 快进展：6个月内Tanner分期跳级\n" +
				"3. 骨龄超前：骨龄大于实际年龄2年以上\n\n" +
				"家长应做到：\n" +
				"1. 定期记录身高变化和发育表现\n" +
				"2. 发现异常及时就诊内分泌科\n" +
				"3. 给予孩子心理关怀和正确引导\n" +
				"4. 避免接触含雌激素的食物和环境\n\n" +
				"⚠️ 以上为一般性科普，具体请遵医嘱",
		},
		"general": {
			title: "儿童生长发育综合指导",
			body: "【健康成长四要素】\n\n" +
				"1. 营养：均衡饮食，每日奶蛋肉蔬果\n" +
				"2. 运动：每天30分钟以上中等强度运动\n" +
				"3. 睡眠：早睡早起，保证充足深睡眠\n" +
				"4. 心理：良好的情绪和家庭环境\n\n" +
				"定期监测：\n" +
				"- 每3个月测量身高体重\n" +
				"- 每年评估一次骨龄（必要时）\n" +
				"- 关注生长曲线是否沿正常轨迹\n\n" +
				"⚠️ 如有异常请及时就诊，以上仅供参考",
		},
	}

	c, ok := contents[category]
	if !ok {
		c = contents["general"]
	}
	return c
}

// ═══════════════════════════════════════════════════════════════
// 13. Report Summary — patient overview for doctor
// ═══════════════════════════════════════════════════════════════

func (t *GrowthClinicTool) reportSummary(userID string, params map[string]interface{}) (string, error) {
	patientID, _ := params["patient_id"].(string)
	if patientID == "" {
		return "", fmt.Errorf("patient_id is required")
	}

	var patient model.GCPatient
	if err := t.db.Where("id = ? AND user_id = ?", patientID, userID).First(&patient).Error; err != nil {
		return "", fmt.Errorf("patient not found")
	}

	ageMonths := time.Since(patient.BirthDate).Hours() / 24 / 30.4375

	// Latest record
	var latest model.GCGrowthRecord
	t.db.Where("patient_id = ?", patientID).Order("record_date DESC").First(&latest)

	// Record count
	var totalRecords int64
	t.db.Model(&model.GCGrowthRecord{}).Where("patient_id = ?", patientID).Count(&totalRecords)

	// Pending alerts
	var pendingAlerts int64
	t.db.Model(&model.GCAlert{}).Where("patient_id = ? AND status = ?", patientID, "pending").Count(&pendingAlerts)

	// Active plan
	var plan model.GCFollowupPlan
	t.db.Where("patient_id = ? AND status = ?", patientID, "active").First(&plan)

	// Education count
	var eduCount int64
	t.db.Model(&model.GCEducation{}).Where("patient_id = ?", patientID).Count(&eduCount)

	summary := map[string]interface{}{
		"patient": map[string]interface{}{
			"name":          patient.Name,
			"gender":        patient.Gender,
			"age_months":    roundTo(ageMonths, 1),
			"age_display":   formatAge(ageMonths),
			"diagnosis":     patient.Diagnosis,
			"target_height": patient.TargetHeight,
			"status":        patient.Status,
		},
		"latest_record": map[string]interface{}{
			"date":       latest.RecordDate.Format("2006-01-02"),
			"height":     latest.Height,
			"weight":     latest.Weight,
			"bmi":        latest.BMI,
			"height_z":   latest.HeightZScore,
			"percentile": latest.HeightPercentile,
			"velocity":   latest.GrowthVelocity,
			"bone_age":   latest.BoneAge,
		},
		"statistics": map[string]interface{}{
			"total_records":   totalRecords,
			"pending_alerts":  pendingAlerts,
			"education_sent":  eduCount,
			"followup_active": plan.ID != "",
		},
	}

	if plan.ID != "" && plan.NextFollowupAt != nil {
		summary["next_followup"] = plan.NextFollowupAt.Format("2006-01-02")
	}

	return jsonStr(summary), nil
}

func formatAge(months float64) string {
	years := int(months / 12)
	mo := int(months) % 12
	if years > 0 {
		return fmt.Sprintf("%d岁%d月", years, mo)
	}
	return fmt.Sprintf("%d月", int(months))
}

// ═══════════════════════════════════════════════════════════════
// LMS Reference Data (WHO + China National Standards)
// Simplified: interpolated from key age points
// ═══════════════════════════════════════════════════════════════

// getHeightLMS returns L, M, S for height-for-age.
// Source: WHO Child Growth Standards + 中国7岁以下儿童生长发育参照标准(2009)
func getHeightLMS(gender string, ageMonths float64) (l, m, s float64) {
	// Key reference points: {ageMonths, L, M, S}
	var table []lmsRow
	if gender == "male" {
		table = maleHeightLMS
	} else {
		table = femaleHeightLMS
	}
	return interpolateLMS(table, ageMonths)
}

func getWeightLMS(gender string, ageMonths float64) (l, m, s float64) {
	var table []lmsRow
	if gender == "male" {
		table = maleWeightLMS
	} else {
		table = femaleWeightLMS
	}
	return interpolateLMS(table, ageMonths)
}

func getBMI_LMS(gender string, ageMonths float64) (l, m, s float64) {
	var table []lmsRow
	if gender == "male" {
		table = maleBMI_LMS
	} else {
		table = femaleBMI_LMS
	}
	return interpolateLMS(table, ageMonths)
}

type lmsRow struct {
	age  float64 // months
	l, m, s float64
}

func interpolateLMS(table []lmsRow, age float64) (l, m, s float64) {
	if len(table) == 0 {
		return 1, 100, 0.04
	}
	if age <= table[0].age {
		return table[0].l, table[0].m, table[0].s
	}
	if age >= table[len(table)-1].age {
		return table[len(table)-1].l, table[len(table)-1].m, table[len(table)-1].s
	}
	for i := 1; i < len(table); i++ {
		if age <= table[i].age {
			t := (age - table[i-1].age) / (table[i].age - table[i-1].age)
			l = table[i-1].l + t*(table[i].l-table[i-1].l)
			m = table[i-1].m + t*(table[i].m-table[i-1].m)
			s = table[i-1].s + t*(table[i].s-table[i-1].s)
			return
		}
	}
	last := table[len(table)-1]
	return last.l, last.m, last.s
}

// WHO + China merged LMS tables (key age points, linearly interpolated)
// Male height-for-age
var maleHeightLMS = []lmsRow{
	{0, 1, 49.9, 0.0379}, {1, 1, 54.7, 0.0364}, {3, 1, 61.4, 0.0349},
	{6, 1, 67.6, 0.0338}, {9, 1, 72.0, 0.0333}, {12, 1, 75.7, 0.0330},
	{18, 1, 82.3, 0.0328}, {24, 1, 87.8, 0.0325}, {36, 1, 96.1, 0.0321},
	{48, 1, 103.3, 0.0319}, {60, 1, 110.0, 0.0318}, {72, 1, 116.0, 0.0418},
	{84, 1, 121.7, 0.0418}, {96, 1, 127.3, 0.0420}, {108, 1, 132.6, 0.0424},
	{120, 1, 137.8, 0.0430}, {132, 1, 143.1, 0.0440}, {144, 1, 149.1, 0.0450},
	{156, 1, 156.0, 0.0450}, {168, 1, 163.2, 0.0440}, {180, 1, 168.5, 0.0430},
	{192, 1, 171.0, 0.0420}, {204, 1, 172.1, 0.0410}, {216, 1, 172.1, 0.0400},
}

// Female height-for-age
var femaleHeightLMS = []lmsRow{
	{0, 1, 49.1, 0.0379}, {1, 1, 53.7, 0.0364}, {3, 1, 59.8, 0.0349},
	{6, 1, 65.7, 0.0338}, {9, 1, 70.1, 0.0333}, {12, 1, 74.0, 0.0330},
	{18, 1, 80.7, 0.0328}, {24, 1, 86.4, 0.0325}, {36, 1, 95.1, 0.0321},
	{48, 1, 102.7, 0.0319}, {60, 1, 109.4, 0.0318}, {72, 1, 115.1, 0.0418},
	{84, 1, 120.8, 0.0420}, {96, 1, 126.6, 0.0424}, {108, 1, 132.2, 0.0430},
	{120, 1, 137.8, 0.0438}, {132, 1, 143.8, 0.0446}, {144, 1, 149.8, 0.0448},
	{156, 1, 154.6, 0.0440}, {168, 1, 157.8, 0.0430}, {180, 1, 159.4, 0.0420},
	{192, 1, 160.0, 0.0410}, {204, 1, 160.1, 0.0400}, {216, 1, 160.1, 0.0390},
}

// Male weight-for-age
var maleWeightLMS = []lmsRow{
	{0, 0.35, 3.3, 0.121}, {1, 0.24, 4.5, 0.131}, {3, 0.13, 6.4, 0.131},
	{6, 0.01, 7.9, 0.124}, {9, -0.06, 9.2, 0.119}, {12, -0.1, 10.2, 0.116},
	{18, -0.13, 11.5, 0.113}, {24, -0.15, 12.7, 0.112}, {36, -0.15, 14.3, 0.112},
	{48, -0.13, 16.3, 0.113}, {60, -0.1, 18.3, 0.115}, {72, -0.07, 20.5, 0.120},
	{84, -0.04, 22.9, 0.126}, {96, 0, 25.6, 0.133}, {108, 0.04, 28.6, 0.139},
	{120, 0.08, 32.0, 0.145}, {132, 0.1, 35.6, 0.149}, {144, 0.1, 39.9, 0.150},
	{156, 0.1, 45.0, 0.148}, {168, 0.08, 50.5, 0.143}, {180, 0.06, 55.5, 0.138},
	{192, 0.04, 59.5, 0.133}, {204, 0.02, 62.5, 0.128}, {216, 0, 64.0, 0.125},
}

// Female weight-for-age
var femaleWeightLMS = []lmsRow{
	{0, 0.38, 3.2, 0.115}, {1, 0.28, 4.2, 0.127}, {3, 0.17, 5.8, 0.127},
	{6, 0.06, 7.3, 0.121}, {9, -0.01, 8.6, 0.117}, {12, -0.05, 9.5, 0.115},
	{18, -0.08, 11.0, 0.113}, {24, -0.1, 12.1, 0.113}, {36, -0.1, 14.0, 0.113},
	{48, -0.08, 16.1, 0.114}, {60, -0.05, 18.2, 0.117}, {72, -0.02, 20.2, 0.122},
	{84, 0.01, 22.4, 0.128}, {96, 0.04, 25.0, 0.134}, {108, 0.07, 28.0, 0.140},
	{120, 0.1, 31.5, 0.146}, {132, 0.12, 36.0, 0.150}, {144, 0.12, 40.5, 0.150},
	{156, 0.1, 45.0, 0.148}, {168, 0.08, 49.0, 0.143}, {180, 0.06, 52.0, 0.138},
	{192, 0.04, 53.5, 0.133}, {204, 0.02, 54.5, 0.128}, {216, 0, 55.0, 0.125},
}

// Male BMI-for-age
var maleBMI_LMS = []lmsRow{
	{0, 0.5, 13.4, 0.091}, {3, -0.2, 16.5, 0.083}, {6, -0.8, 17.3, 0.079},
	{12, -1.2, 17.2, 0.078}, {24, -1.5, 16.5, 0.079}, {36, -1.6, 15.9, 0.079},
	{48, -1.7, 15.5, 0.080}, {60, -1.8, 15.3, 0.081}, {72, -1.9, 15.3, 0.085},
	{84, -2.0, 15.5, 0.089}, {96, -2.0, 15.7, 0.094}, {108, -2.0, 16.0, 0.100},
	{120, -2.0, 16.4, 0.105}, {132, -1.9, 17.0, 0.110}, {144, -1.8, 17.6, 0.112},
	{156, -1.7, 18.4, 0.112}, {168, -1.5, 19.2, 0.110}, {180, -1.3, 20.0, 0.108},
	{192, -1.1, 20.7, 0.105}, {204, -0.9, 21.2, 0.102}, {216, -0.7, 21.5, 0.100},
}

// Female BMI-for-age
var femaleBMI_LMS = []lmsRow{
	{0, 0.5, 13.3, 0.092}, {3, -0.2, 16.2, 0.085}, {6, -0.7, 17.0, 0.080},
	{12, -1.0, 16.7, 0.079}, {24, -1.3, 16.0, 0.080}, {36, -1.4, 15.5, 0.080},
	{48, -1.5, 15.2, 0.081}, {60, -1.5, 15.0, 0.082}, {72, -1.6, 15.0, 0.087},
	{84, -1.7, 15.2, 0.092}, {96, -1.7, 15.5, 0.097}, {108, -1.7, 15.9, 0.103},
	{120, -1.6, 16.4, 0.108}, {132, -1.5, 17.1, 0.112}, {144, -1.4, 17.8, 0.113},
	{156, -1.2, 18.6, 0.112}, {168, -1.0, 19.3, 0.110}, {180, -0.8, 19.9, 0.107},
	{192, -0.6, 20.3, 0.104}, {204, -0.4, 20.6, 0.101}, {216, -0.3, 20.8, 0.100},
}

// ═══════════════════════════════════════════════════════════════
// Utility functions
// ═══════════════════════════════════════════════════════════════

func toFloat(v interface{}) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case json.Number:
		f, _ := n.Float64()
		return f
	case string:
		var f float64
		fmt.Sscanf(n, "%f", &f)
		return f
	}
	return 0
}

func roundTo(v float64, decimals int) float64 {
	pow := math.Pow(10, float64(decimals))
	return math.Round(v*pow) / pow
}

func jsonStr(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// suppress unused import warning for strings
var _ = strings.Contains
