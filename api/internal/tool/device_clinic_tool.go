package tool

import (
	"fmt"
	"sort"
	"strings"

	"gorm.io/gorm"
)

type DeviceClinicTool struct {
	db *gorm.DB
}

func NewDeviceClinicTool(db *gorm.DB) *DeviceClinicTool {
	return &DeviceClinicTool{db: db}
}

func (t *DeviceClinicTool) Execute(userID string, params map[string]interface{}) (string, error) {
	action, _ := params["action"].(string)
	switch action {
	case "pulse_oximeter_review":
		return t.pulseOximeterReview(params)
	case "home_bp_review":
		return t.homeBPReview(params)
	case "wearable_monitor_review":
		return t.wearableMonitorReview(params)
	case "device_safety_event_review":
		return t.deviceSafetyEventReview(params)
	default:
		return "", fmt.Errorf("unknown device_clinic action: %s", action)
	}
}

func (t *DeviceClinicTool) pulseOximeterReview(params map[string]interface{}) (string, error) {
	spo2 := toFloat(params["spo2"])
	heartRate := toFloat(params["heart_rate"])
	poorSignal := deviceToBool(params["poor_signal"])
	coldExtremities := deviceToBool(params["cold_extremities"])
	symptoms := parseDeviceList(params["symptoms"])

	if spo2 <= 0 && heartRate <= 0 && len(symptoms) == 0 {
		return "", fmt.Errorf("pulse oximeter inputs are required")
	}

	findings := []map[string]interface{}{}
	level := "normal"
	urgency := "routine"

	if spo2 > 0 {
		if spo2 < 92 {
			findings = append(findings, map[string]interface{}{"category": "spo2", "level": "severe", "title": fmt.Sprintf("血氧偏低 %.0f%%", spo2), "description": "建议立即线下或急诊评估"})
			level = deviceMaxLevel(level, "severe")
			urgency = deviceMaxUrgency(urgency, "emergency")
		} else if spo2 < 95 {
			findings = append(findings, map[string]interface{}{"category": "spo2", "level": "moderate", "title": fmt.Sprintf("血氧边缘偏低 %.0f%%", spo2), "description": "建议尽快线下复核，并结合呼吸状态判断"})
			level = deviceMaxLevel(level, "moderate")
			urgency = deviceMaxUrgency(urgency, "same_day")
		}
	}
	if heartRate > 0 && (heartRate >= 180 || heartRate <= 45) {
		findings = append(findings, map[string]interface{}{"category": "heart_rate", "level": "severe", "title": fmt.Sprintf("设备记录心率异常 %.0f 次/分", heartRate), "description": "需立即结合症状和测量质量线下评估"})
		level = deviceMaxLevel(level, "severe")
		urgency = deviceMaxUrgency(urgency, "emergency")
	} else if heartRate > 0 && (heartRate >= 140 || heartRate <= 55) {
		findings = append(findings, map[string]interface{}{"category": "heart_rate", "level": "moderate", "title": fmt.Sprintf("心率偏离常见范围 %.0f 次/分", heartRate), "description": "建议复测并结合发热、活动和症状进一步判断"})
		level = deviceMaxLevel(level, "moderate")
	}
	if poorSignal || coldExtremities {
		findings = append(findings, map[string]interface{}{"category": "measurement_quality", "level": "mild", "title": "存在测量质量受影响线索", "description": "提示设备读数可能不稳定，建议规范复测"})
		level = deviceMaxLevel(level, "mild")
	}
	for _, symptom := range symptoms {
		if containsAnyNormalizedDevice([]string{symptom}, "呼吸困难", "发绀", "胸痛", "意识差") {
			findings = append(findings, map[string]interface{}{"category": "device_red_flag", "level": "severe", "title": fmt.Sprintf("设备监测红旗：%s", symptom), "description": "建议立即线下或急诊评估"})
			level = deviceMaxLevel(level, "severe")
			urgency = deviceMaxUrgency(urgency, "emergency")
		}
	}

	return jsonStr(map[string]interface{}{
		"panel":         "pulse_oximeter_review",
		"overall_level": level,
		"urgency":       urgency,
		"findings":      findings,
		"followup":      deviceFollowupAdvice("pulse_oximeter", urgency),
	}), nil
}

func (t *DeviceClinicTool) homeBPReview(params map[string]interface{}) (string, error) {
	systolic := toFloat(params["systolic"])
	diastolic := toFloat(params["diastolic"])
	repeatReadings := toFloat(params["repeat_readings"])
	cuffMismatch := deviceToBool(params["cuff_size_mismatch"])
	symptoms := parseDeviceList(params["symptoms"])

	if systolic <= 0 && diastolic <= 0 && len(symptoms) == 0 {
		return "", fmt.Errorf("home blood pressure inputs are required")
	}

	findings := []map[string]interface{}{}
	level := "normal"
	urgency := "routine"

	if systolic >= 140 || diastolic >= 90 {
		findings = append(findings, map[string]interface{}{"category": "blood_pressure", "level": "severe", "title": fmt.Sprintf("家用血压明显升高 %.0f/%.0f mmHg", systolic, diastolic), "description": "建议当天线下复核，必要时急诊评估"})
		level = deviceMaxLevel(level, "severe")
		urgency = deviceMaxUrgency(urgency, "same_day")
	} else if systolic >= 130 || diastolic >= 80 {
		findings = append(findings, map[string]interface{}{"category": "blood_pressure", "level": "moderate", "title": fmt.Sprintf("家用血压偏高 %.0f/%.0f mmHg", systolic, diastolic), "description": "建议在规范条件下重复测量并尽快线下复核"})
		level = deviceMaxLevel(level, "moderate")
		urgency = deviceMaxUrgency(urgency, "expedited")
	}
	if repeatReadings >= 2 {
		findings = append(findings, map[string]interface{}{"category": "repeatability", "level": "mild", "title": "已进行重复测量", "description": "需结合坐位静息、袖带合适性和设备质量综合判断"})
		level = deviceMaxLevel(level, "mild")
	}
	if cuffMismatch {
		findings = append(findings, map[string]interface{}{"category": "cuff", "level": "mild", "title": "存在袖带尺寸不匹配线索", "description": "提示家测血压可能失真，建议规范复测"})
		level = deviceMaxLevel(level, "mild")
	}
	for _, symptom := range symptoms {
		if containsAnyNormalizedDevice([]string{symptom}, "严重头痛", "视物模糊", "抽搐", "胸痛") {
			findings = append(findings, map[string]interface{}{"category": "bp_red_flag", "level": "severe", "title": fmt.Sprintf("血压相关红旗：%s", symptom), "description": "建议立即线下或急诊评估"})
			level = deviceMaxLevel(level, "severe")
			urgency = deviceMaxUrgency(urgency, "emergency")
		}
	}

	return jsonStr(map[string]interface{}{
		"panel":         "home_bp_review",
		"overall_level": level,
		"urgency":       urgency,
		"findings":      findings,
		"followup":      deviceFollowupAdvice("home_bp", urgency),
	}), nil
}

func (t *DeviceClinicTool) wearableMonitorReview(params map[string]interface{}) (string, error) {
	alertType, _ := params["alert_type"].(string)
	readings := parseDeviceList(params["readings"])
	symptoms := parseDeviceList(params["symptoms"])
	recurrentAlerts := toFloat(params["recurrent_alerts_7d"])
	deviceRemoved := deviceToBool(params["device_removed_during_event"])

	if strings.TrimSpace(alertType) == "" && len(readings) == 0 && len(symptoms) == 0 {
		return "", fmt.Errorf("wearable monitor inputs are required")
	}

	findings := []map[string]interface{}{}
	level := "normal"
	urgency := "routine"
	allSignals := append([]string{alertType}, readings...)
	allSignals = append(allSignals, symptoms...)

	if containsAnyNormalizedDevice(allSignals, "心律失常", "arrhythmia", "持续低氧", "fall detected", "摔倒", "afib") {
		findings = append(findings, map[string]interface{}{"category": "critical_alert", "level": "severe", "title": "穿戴设备提示高风险警报", "description": "建议尽快结合真实症状线下评估"})
		level = deviceMaxLevel(level, "severe")
		urgency = deviceMaxUrgency(urgency, "same_day")
	}
	if recurrentAlerts >= 3 {
		findings = append(findings, map[string]interface{}{"category": "recurrent_alerts", "level": "moderate", "title": fmt.Sprintf("近 7 天反复报警 %.0f 次", recurrentAlerts), "description": "提示需要复核设备准确性与真实临床意义"})
		level = deviceMaxLevel(level, "moderate")
		urgency = deviceMaxUrgency(urgency, "expedited")
	}
	if deviceRemoved {
		findings = append(findings, map[string]interface{}{"category": "data_gap", "level": "mild", "title": "事件期间设备数据可能不完整", "description": "提示报警解释需要谨慎"})
		level = deviceMaxLevel(level, "mild")
	}
	for _, symptom := range symptoms {
		if containsAnyNormalizedDevice([]string{symptom}, "晕厥", "胸痛", "呼吸困难", "意识丧失") {
			findings = append(findings, map[string]interface{}{"category": "wearable_red_flag", "level": "severe", "title": fmt.Sprintf("穿戴设备伴临床红旗：%s", symptom), "description": "建议立即线下或急诊评估"})
			level = deviceMaxLevel(level, "severe")
			urgency = deviceMaxUrgency(urgency, "emergency")
		}
	}

	return jsonStr(map[string]interface{}{
		"panel":         "wearable_monitor_review",
		"overall_level": level,
		"urgency":       urgency,
		"findings":      findings,
		"followup":      deviceFollowupAdvice("wearable_monitor", urgency),
	}), nil
}

func (t *DeviceClinicTool) deviceSafetyEventReview(params map[string]interface{}) (string, error) {
	deviceType, _ := params["device_type"].(string)
	eventDescription, _ := params["event_description"].(string)
	medicationConnection := deviceToBool(params["medication_connection"])
	injuryOccurred := deviceToBool(params["injury_occurred"])
	interruptionHours := toFloat(params["interruption_hours"])
	symptoms := parseDeviceList(params["symptoms"])

	if strings.TrimSpace(deviceType) == "" && strings.TrimSpace(eventDescription) == "" && len(symptoms) == 0 {
		return "", fmt.Errorf("device safety event inputs are required")
	}

	findings := []map[string]interface{}{}
	level := "normal"
	urgency := "routine"
	eventSignals := append([]string{deviceType, eventDescription}, symptoms...)

	if injuryOccurred {
		findings = append(findings, map[string]interface{}{"category": "harm", "level": "severe", "title": "设备事件已导致损伤或明显不适", "description": "建议立即线下评估并保留设备与记录"})
		level = deviceMaxLevel(level, "severe")
		urgency = deviceMaxUrgency(urgency, "same_day")
	}
	if medicationConnection || containsAnyNormalizedDevice(eventSignals, "剂量错误", "输注中断", "报警失效", "校准失败") {
		findings = append(findings, map[string]interface{}{"category": "device_failure", "level": "moderate", "title": "存在可能影响治疗的设备异常", "description": "建议尽快线下复核设备状态和治疗连续性"})
		level = deviceMaxLevel(level, "moderate")
		urgency = deviceMaxUrgency(urgency, "expedited")
	}
	if interruptionHours >= 4 {
		findings = append(findings, map[string]interface{}{"category": "interruption", "level": "moderate", "title": fmt.Sprintf("设备中断约 %.0f 小时", interruptionHours), "description": "需结合依赖该设备的临床场景尽快复核"})
		level = deviceMaxLevel(level, "moderate")
	}
	for _, symptom := range symptoms {
		if containsAnyNormalizedDevice([]string{symptom}, "抽搐", "昏迷", "严重低血糖", "呼吸困难") {
			findings = append(findings, map[string]interface{}{"category": "safety_red_flag", "level": "severe", "title": fmt.Sprintf("设备安全红旗：%s", symptom), "description": "建议立即线下或急诊评估"})
			level = deviceMaxLevel(level, "severe")
			urgency = deviceMaxUrgency(urgency, "emergency")
		}
	}

	return jsonStr(map[string]interface{}{
		"panel":         "device_safety_event_review",
		"device_type":   strings.TrimSpace(deviceType),
		"overall_level": level,
		"urgency":       urgency,
		"findings":      findings,
		"followup":      deviceFollowupAdvice("device_safety", urgency),
	}), nil
}

func parseDeviceList(v interface{}) []string {
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
	return uniqueSortedDeviceStrings(items)
}

func normalizeDeviceTerm(v string) string {
	replacer := strings.NewReplacer(" ", "", "-", "", "_", "", ".", "", "/", "", "（", "(", "）", ")")
	return strings.ToLower(strings.TrimSpace(replacer.Replace(v)))
}

func uniqueSortedDeviceStrings(items []string) []string {
	lookup := map[string]string{}
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		key := normalizeDeviceTerm(trimmed)
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

func deviceMaxLevel(current, next string) string {
	levels := map[string]int{"normal": 0, "mild": 1, "moderate": 2, "severe": 3}
	if levels[next] > levels[current] {
		return next
	}
	return current
}

func deviceMaxUrgency(current, next string) string {
	levels := map[string]int{"routine": 0, "expedited": 1, "same_day": 2, "emergency": 3}
	if levels[next] > levels[current] {
		return next
	}
	return current
}

func deviceFollowupAdvice(panel, urgency string) string {
	if urgency == "emergency" {
		return "建议立即线下就医或急诊评估，不要仅依赖设备读数或线上建议。"
	}
	if urgency == "same_day" {
		return "建议当天线下复核，携带设备截图、报警记录和症状时间线。"
	}
	switch panel {
	case "home_bp":
		return "建议在安静坐位、合适袖带条件下重复测量，并记录多次结果供线下复核。"
	case "device_safety":
		return "建议保留设备报警、故障截图和相关记录，并尽快联系线下医生或设备支持渠道。"
	default:
		return "建议保留设备截图、报警记录和症状变化，必要时尽快线下复核。"
	}
}

func containsAnyNormalizedDevice(values []string, patterns ...string) bool {
	for _, value := range values {
		normalized := normalizeDeviceTerm(value)
		for _, pattern := range patterns {
			if strings.Contains(normalized, normalizeDeviceTerm(pattern)) {
				return true
			}
		}
	}
	return false
}

func deviceToBool(v interface{}) bool {
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
