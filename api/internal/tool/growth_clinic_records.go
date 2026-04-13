package tool

import (
	"fmt"
	"time"

	"github.com/yinhe/starclaw/internal/model"
)

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
		UserID:          userID,
		PatientID:       patientID,
		RecordDate:      now,
		AgeMonths:       ageMonths,
		Height:          toFloat(params["height"]),
		HeightMorning:   toFloat(params["height_morning"]),
		HeightEvening:   toFloat(params["height_evening"]),
		Weight:          toFloat(params["weight"]),
		BoneAge:         toFloat(params["bone_age"]),
		SleepHours:      toFloat(params["sleep_hours"]),
		ExerciseMinutes: int(toFloat(params["exercise_minutes"])),
		ExerciseWeekly:  int(toFloat(params["exercise_weekly"])),
		DataSource:      "manual",
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
		"record":        rec,
		"message":       "生长数据记录成功",
		"auto_z_scores": true,
		"auto_velocity": rec.GrowthVelocity > 0,
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

func getEducationContent(category, _ string) eduContent {
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
