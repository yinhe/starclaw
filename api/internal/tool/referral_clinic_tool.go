package tool

import (
	"fmt"
	"sort"
	"strings"

	"gorm.io/gorm"
)

type ReferralClinicTool struct {
	db *gorm.DB
}

type referralRule struct {
	Specialty string
	Keywords  []string
	Checklist []string
}

func NewReferralClinicTool(db *gorm.DB) *ReferralClinicTool {
	return &ReferralClinicTool{db: db}
}

func (t *ReferralClinicTool) Execute(userID string, params map[string]interface{}) (string, error) {
	action, _ := params["action"].(string)
	switch action {
	case "specialty_triage":
		return t.specialtyTriage(params)
	case "urgent_review":
		return t.urgentReview(params)
	case "visit_checklist":
		return t.visitChecklist(params)
	case "multidisciplinary_handoff":
		return t.multidisciplinaryHandoff(params)
	default:
		return "", fmt.Errorf("unknown referral_clinic action: %s", action)
	}
}

func (t *ReferralClinicTool) specialtyTriage(params map[string]interface{}) (string, error) {
	signals := collectReferralSignals(params)
	if len(signals) == 0 {
		return "", fmt.Errorf("at least one symptom, finding, diagnosis, concern, or note_text is required")
	}

	scores, reasons := scoreReferralSpecialties(signals)
	primary, alternatives := rankReferralSpecialties(scores)
	redFlags := detectReferralRedFlags(signals)
	urgency := inferReferralUrgency(redFlags, params)
	if primary == "" {
		primary = "儿科门诊"
	}

	return jsonStr(map[string]interface{}{
		"primary_specialty":       primary,
		"alternative_specialties": alternatives,
		"urgency":                 urgency,
		"signals":                 signals,
		"red_flags":               redFlags,
		"referral_reasons":        reasons[primary],
		"visit_checklist":         specialtyChecklist(primary),
	}), nil
}

func (t *ReferralClinicTool) urgentReview(params map[string]interface{}) (string, error) {
	signals := collectReferralSignals(params)
	if len(signals) == 0 {
		return "", fmt.Errorf("at least one symptom, finding, diagnosis, concern, or note_text is required")
	}
	redFlags := detectReferralRedFlags(signals)
	urgency := inferReferralUrgency(redFlags, params)
	scores, _ := scoreReferralSpecialties(signals)
	primary, alternatives := rankReferralSpecialties(scores)
	recommendation := map[string]string{
		"emergency": "建议立即前往急诊或呼叫急救支持，不要仅依赖线上建议。",
		"same_day":  "建议当天线下就医或尽快到相应专科/急诊复核。",
		"expedited": "建议尽快预约专科门诊，在近期完成复核。",
		"routine":   "可常规预约相应专科，并提前准备既往资料。",
	}[urgency]

	return jsonStr(map[string]interface{}{
		"urgency":               urgency,
		"red_flags":             redFlags,
		"recommended_specialty": primary,
		"backup_specialties":    alternatives,
		"recommendation":        recommendation,
	}), nil
}

func (t *ReferralClinicTool) visitChecklist(params map[string]interface{}) (string, error) {
	targetSpecialty, _ := params["target_specialty"].(string)
	targetSpecialty = strings.TrimSpace(targetSpecialty)
	signals := collectReferralSignals(params)
	if targetSpecialty == "" {
		scores, _ := scoreReferralSpecialties(signals)
		primary, _ := rankReferralSpecialties(scores)
		targetSpecialty = primary
	}
	if targetSpecialty == "" {
		targetSpecialty = "儿科门诊"
	}

	documents := uniqueSortedClinicalStrings([]string{
		"既往门诊病历或出院小结",
		"近期化验单和检查报告",
		"当前用药与补充剂清单",
		"家长记录的主要症状时间线",
	})
	if containsAnyNormalized(signals, "矮小", "生长迟缓", "肥胖", "体重异常", "性早熟", "骨龄") {
		documents = uniqueSortedClinicalStrings(append(documents, "身高体重记录与生长曲线"))
	}

	return jsonStr(map[string]interface{}{
		"target_specialty":             targetSpecialty,
		"documents_to_bring":           documents,
		"specialty_checklist":          specialtyChecklist(targetSpecialty),
		"signals":                      signals,
		"suggested_appointment_reason": summarizeReferralReason(signals, targetSpecialty),
	}), nil
}

func (t *ReferralClinicTool) multidisciplinaryHandoff(params map[string]interface{}) (string, error) {
	signals := collectReferralSignals(params)
	if len(signals) == 0 {
		return "", fmt.Errorf("at least one symptom, finding, diagnosis, concern, or note_text is required")
	}
	scores, reasons := scoreReferralSpecialties(signals)
	primary, alternatives := rankReferralSpecialties(scores)
	departments := []string{}
	if primary != "" {
		departments = append(departments, primary)
	}
	departments = append(departments, alternatives...)
	if len(departments) > 3 {
		departments = departments[:3]
	}
	medications := parseReferralList(params["current_medications"])
	if noteText, _ := params["note_text"].(string); strings.TrimSpace(noteText) != "" {
		medications = uniqueSortedClinicalStrings(append(medications, extractMedicationMentions(noteText)...))
	}
	redFlags := detectReferralRedFlags(signals)
	urgency := inferReferralUrgency(redFlags, params)

	summaryParts := []string{}
	if len(departments) > 0 {
		summaryParts = append(summaryParts, "建议科室: "+strings.Join(departments, "、"))
	}
	if len(reasons[primary]) > 0 {
		summaryParts = append(summaryParts, "转诊依据: "+strings.Join(reasons[primary], "；"))
	}
	if len(medications) > 0 {
		summaryParts = append(summaryParts, "当前用药: "+strings.Join(medications, "、"))
	}
	if len(redFlags) > 0 {
		summaryParts = append(summaryParts, "风险提示: "+strings.Join(redFlags, "；"))
	}

	return jsonStr(map[string]interface{}{
		"departments":         departments,
		"primary_specialty":   primary,
		"urgency":             urgency,
		"handoff_summary":     strings.Join(summaryParts, " | "),
		"signals":             signals,
		"current_medications": medications,
		"red_flags":           redFlags,
	}), nil
}

func collectReferralSignals(params map[string]interface{}) []string {
	signals := []string{}
	for _, key := range []string{"symptoms", "findings", "diagnoses", "concerns"} {
		signals = append(signals, parseReferralList(params[key])...)
	}
	if noteText, _ := params["note_text"].(string); strings.TrimSpace(noteText) != "" {
		signals = append(signals, extractReferralMentions(noteText)...)
		signals = append(signals, extractRedFlags(noteText)...)
	}
	return uniqueSortedClinicalStrings(signals)
}

func parseReferralList(v interface{}) []string {
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
	return uniqueSortedClinicalStrings(items)
}

func extractReferralMentions(noteText string) []string {
	catalog := []string{
		"矮小", "生长迟缓", "生长激素", "骨龄提前", "骨龄落后", "性早熟", "甲状腺", "肥胖", "贫血", "铁缺乏", "中性粒细胞减少", "血小板减少",
		"维生素D缺乏", "低钙", "抽搐", "头痛", "视物模糊", "呼吸困难", "胸痛", "心悸", "大出血", "持续呕吐", "严重腹痛", "跛行", "髋痛",
		"血尿", "蛋白尿", "水肿", "高血压", "反复发热", "高热不退", "肝酶升高", "血脂异常", "尿酸升高", "发育倒退",
	}
	return extractClinicalMentions(noteText, catalog)
}

func detectReferralRedFlags(signals []string) []string {
	redFlagCatalog := []string{
		"胸痛", "呼吸困难", "抽搐", "大出血", "持续呕吐", "严重腹痛", "高热不退", "明显脱水", "视物模糊", "严重头痛", "意识差", "跛行", "髋痛",
	}
	flags := []string{}
	for _, signal := range signals {
		if containsAnyNormalized([]string{signal}, redFlagCatalog...) {
			flags = append(flags, signal)
		}
	}
	return uniqueSortedClinicalStrings(flags)
}

func inferReferralUrgency(redFlags []string, params map[string]interface{}) string {
	if len(redFlags) > 0 {
		if containsAnyNormalized(redFlags, "胸痛", "呼吸困难", "抽搐", "大出血", "意识差") {
			return "emergency"
		}
		return "same_day"
	}
	severityHint, _ := params["severity_hint"].(string)
	switch strings.ToLower(strings.TrimSpace(severityHint)) {
	case "severe", "urgent", "紧急":
		return "same_day"
	case "moderate", "expedited", "尽快":
		return "expedited"
	default:
		return "routine"
	}
}

func scoreReferralSpecialties(signals []string) (map[string]int, map[string][]string) {
	rules := []referralRule{
		{Specialty: "小儿内分泌科", Keywords: []string{"矮小", "生长迟缓", "生长激素", "骨龄提前", "骨龄落后", "性早熟", "甲状腺", "肥胖", "维生素D缺乏"}, Checklist: []string{"近期身高体重记录", "骨龄片或报告", "相关激素/甲功/骨代谢化验"}},
		{Specialty: "小儿血液科", Keywords: []string{"贫血", "铁缺乏", "中性粒细胞减少", "血小板减少", "白细胞异常", "出血倾向"}, Checklist: []string{"CBC 或血常规报告", "铁蛋白/铁代谢报告", "出血或感染情况记录"}},
		{Specialty: "小儿神经科", Keywords: []string{"抽搐", "严重头痛", "视物模糊", "发育倒退", "意识差"}, Checklist: []string{"症状发生时间线", "既往影像/脑电图资料", "用药和诱发因素记录"}},
		{Specialty: "急诊科", Keywords: []string{"胸痛", "呼吸困难", "大出血", "意识差", "高热不退", "明显脱水"}, Checklist: []string{"优先立即就医", "准备既往病史和当前用药信息"}},
		{Specialty: "小儿骨科", Keywords: []string{"跛行", "髋痛", "骨痛", "关节肿痛"}, Checklist: []string{"相关影像资料", "疼痛部位和持续时间记录"}},
		{Specialty: "小儿肾脏科", Keywords: []string{"血尿", "蛋白尿", "水肿", "高血压"}, Checklist: []string{"尿常规/肾功能报告", "家庭血压记录", "浮肿照片或变化记录"}},
		{Specialty: "小儿消化科", Keywords: []string{"持续呕吐", "严重腹痛", "肝酶升高", "慢性腹泻"}, Checklist: []string{"肝功能/腹部检查资料", "饮食与症状关系记录", "排便情况记录"}},
		{Specialty: "儿童营养门诊", Keywords: []string{"喂养困难", "营养不良", "体重异常", "肥胖", "维生素D缺乏"}, Checklist: []string{"饮食记录", "体重身高变化", "补充剂使用情况"}},
	}

	scores := map[string]int{}
	reasons := map[string][]string{}
	for _, rule := range rules {
		for _, signal := range signals {
			if containsAnyNormalized([]string{signal}, rule.Keywords...) {
				scores[rule.Specialty]++
				reasons[rule.Specialty] = append(reasons[rule.Specialty], signal)
			}
		}
	}
	for specialty, items := range reasons {
		reasons[specialty] = uniqueSortedClinicalStrings(items)
	}
	return scores, reasons
}

func rankReferralSpecialties(scores map[string]int) (string, []string) {
	type scoreItem struct {
		Specialty string
		Score     int
	}
	items := []scoreItem{}
	for specialty, score := range scores {
		if score > 0 {
			items = append(items, scoreItem{Specialty: specialty, Score: score})
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Score == items[j].Score {
			return items[i].Specialty < items[j].Specialty
		}
		return items[i].Score > items[j].Score
	})
	if len(items) == 0 {
		return "", nil
	}
	primary := items[0].Specialty
	alternatives := []string{}
	for _, item := range items[1:] {
		alternatives = append(alternatives, item.Specialty)
	}
	return primary, alternatives
}

func specialtyChecklist(specialty string) []string {
	lookup := map[string][]string{
		"小儿内分泌科": {"近期身高体重记录", "骨龄片或报告", "甲功/骨代谢/生长相关化验", "当前用药与补充剂信息"},
		"小儿血液科":  {"血常规/CBC 报告", "铁蛋白或铁代谢报告", "感染或出血情况记录", "既往治疗记录"},
		"小儿神经科":  {"症状发生时间线", "既往影像或脑电图资料", "发作视频或描述", "当前用药信息"},
		"急诊科":    {"立即携带既往病史", "当前用药和过敏史", "近期检查资料", "优先就近就医"},
		"小儿骨科":   {"相关影像资料", "疼痛部位与持续时间记录", "步态异常说明"},
		"小儿肾脏科":  {"尿常规/肾功能报告", "血压记录", "水肿或尿色变化说明"},
		"小儿消化科":  {"腹部检查与肝功能资料", "饮食和症状记录", "排便情况记录"},
		"儿童营养门诊": {"饮食记录", "身高体重变化", "补充剂/零食摄入信息"},
	}
	if items, ok := lookup[specialty]; ok {
		return items
	}
	return []string{"既往病历", "近期检查资料", "当前用药与家长观察记录"}
}

func summarizeReferralReason(signals []string, specialty string) string {
	if len(signals) == 0 {
		return "为进一步明确病情与制定后续计划，建议预约相应专科。"
	}
	items := signals
	if len(items) > 3 {
		items = items[:3]
	}
	return fmt.Sprintf("因%s等问题，建议预约%s进一步评估。", strings.Join(items, "、"), specialty)
}
