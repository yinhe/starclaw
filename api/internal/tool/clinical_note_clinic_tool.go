package tool

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"gorm.io/gorm"
)

type ClinicalNoteClinicTool struct {
	db *gorm.DB
}

func NewClinicalNoteClinicTool(db *gorm.DB) *ClinicalNoteClinicTool {
	return &ClinicalNoteClinicTool{db: db}
}

func (t *ClinicalNoteClinicTool) Execute(userID string, params map[string]interface{}) (string, error) {
	action, _ := params["action"].(string)
	switch action {
	case "parse_note":
		return t.parseNote(params)
	case "extract_followup":
		return t.extractFollowup(params)
	case "discharge_summary":
		return t.dischargeSummary(params)
	case "handoff_brief":
		return t.handoffBrief(params)
	default:
		return "", fmt.Errorf("unknown clinical_note_clinic action: %s", action)
	}
}

func (t *ClinicalNoteClinicTool) parseNote(params map[string]interface{}) (string, error) {
	noteText, err := requireNoteText(params)
	if err != nil {
		return "", err
	}
	sections := extractNoteSections(noteText)
	diagnoses := extractListFromSections(sections, []string{"诊断", "出院诊断", "初步诊断", "assessment", "problem list"})
	medications := extractMedicationMentions(noteText)
	labItems := extractLabMentions(noteText)
	followupItems := extractListFromSections(sections, []string{"随访计划", "复诊计划", "出院医嘱", "plan", "follow up"})
	redFlags := extractRedFlags(noteText)
	noteType := detectNoteType(noteText, sections)

	return jsonStr(map[string]interface{}{
		"note_type":       noteType,
		"chief_complaint": firstSectionValue(sections, "主诉", "chief complaint", "chiefcomplaint"),
		"history_present": firstSectionValue(sections, "现病史", "history of present illness", "hpi"),
		"assessment":      firstSectionValue(sections, "评估", "assessment"),
		"diagnoses":       diagnoses,
		"medications":     medications,
		"lab_items":       labItems,
		"followup_items":  followupItems,
		"red_flags":       redFlags,
		"section_keys":    sortedSectionKeys(sections),
		"sections":        sections,
		"text_length":     len([]rune(noteText)),
	}), nil
}

func (t *ClinicalNoteClinicTool) extractFollowup(params map[string]interface{}) (string, error) {
	noteText, err := requireNoteText(params)
	if err != nil {
		return "", err
	}
	sections := extractNoteSections(noteText)
	followupItems := extractListFromSections(sections, []string{"随访计划", "复诊计划", "出院医嘱", "plan", "follow up"})
	medications := extractMedicationMentions(noteText)
	labItems := extractLabMentions(noteText)
	nextVisitWindow := detectFollowupWindow(noteText)
	homeTasks := extractHomeTasks(noteText)

	return jsonStr(map[string]interface{}{
		"note_type":         detectNoteType(noteText, sections),
		"next_visit_window": nextVisitWindow,
		"followup_items":    followupItems,
		"home_tasks":        homeTasks,
		"labs_to_recheck":   labItems,
		"medications":       medications,
		"care_points":       uniqueSortedClinicalStrings(append(followupItems, homeTasks...)),
	}), nil
}

func (t *ClinicalNoteClinicTool) dischargeSummary(params map[string]interface{}) (string, error) {
	noteText, err := requireNoteText(params)
	if err != nil {
		return "", err
	}
	sections := extractNoteSections(noteText)
	diagnoses := extractListFromSections(sections, []string{"出院诊断", "诊断", "assessment"})
	dischargeAdvice := extractListFromSections(sections, []string{"出院医嘱", "医嘱", "plan", "follow up"})
	medications := extractMedicationMentions(noteText)
	labItems := extractLabMentions(noteText)

	return jsonStr(map[string]interface{}{
		"admission_reason":  firstSectionValue(sections, "入院原因", "入院情况", "主诉", "chief complaint"),
		"hospital_course":   firstSectionValue(sections, "住院经过", "病程经过", "现病史", "hospital course"),
		"discharge_status":  firstSectionValue(sections, "出院情况", "出院状态", "condition on discharge"),
		"diagnoses":         diagnoses,
		"medications":       medications,
		"lab_items":         labItems,
		"discharge_advice":  dischargeAdvice,
		"next_visit_window": detectFollowupWindow(noteText),
	}), nil
}

func (t *ClinicalNoteClinicTool) handoffBrief(params map[string]interface{}) (string, error) {
	noteText, err := requireNoteText(params)
	if err != nil {
		return "", err
	}
	sections := extractNoteSections(noteText)
	diagnoses := extractListFromSections(sections, []string{"诊断", "出院诊断", "assessment"})
	medications := extractMedicationMentions(noteText)
	redFlags := extractRedFlags(noteText)
	followupItems := extractListFromSections(sections, []string{"随访计划", "复诊计划", "出院医嘱", "plan", "follow up"})

	brief := []string{}
	if complaint := firstSectionValue(sections, "主诉", "chief complaint"); complaint != "" {
		brief = append(brief, "主诉: "+complaint)
	}
	if len(diagnoses) > 0 {
		brief = append(brief, "诊断: "+strings.Join(diagnoses, "；"))
	}
	if len(medications) > 0 {
		brief = append(brief, "当前用药: "+strings.Join(medications, "、"))
	}
	if len(followupItems) > 0 {
		brief = append(brief, "随访要点: "+strings.Join(followupItems, "；"))
	}
	if len(redFlags) > 0 {
		brief = append(brief, "风险提示: "+strings.Join(redFlags, "；"))
	}

	return jsonStr(map[string]interface{}{
		"note_type":      detectNoteType(noteText, sections),
		"handoff_brief":  strings.Join(brief, " | "),
		"diagnoses":      diagnoses,
		"medications":    medications,
		"followup_items": followupItems,
		"red_flags":      redFlags,
	}), nil
}

func requireNoteText(params map[string]interface{}) (string, error) {
	noteText, _ := params["note_text"].(string)
	noteText = strings.TrimSpace(noteText)
	if noteText == "" {
		return "", fmt.Errorf("note_text is required")
	}
	return noteText, nil
}

func extractNoteSections(noteText string) map[string]string {
	text := strings.ReplaceAll(noteText, "\r", "\n")
	lines := strings.Split(text, "\n")
	sections := map[string]string{}
	currentKey := "全文"
	buffer := []string{}

	flush := func() {
		value := strings.TrimSpace(strings.Join(buffer, "\n"))
		if value != "" {
			if existing := strings.TrimSpace(sections[currentKey]); existing != "" {
				sections[currentKey] = existing + "\n" + value
			} else {
				sections[currentKey] = value
			}
		}
		buffer = []string{}
	}

	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		if key, value, ok := parseSectionHeader(line); ok {
			flush()
			currentKey = key
			if value != "" {
				buffer = append(buffer, value)
			}
			continue
		}
		buffer = append(buffer, line)
	}
	flush()
	return sections
}

func parseSectionHeader(line string) (string, string, bool) {
	patterns := []string{
		`^(主诉|现病史|既往史|体格检查|辅助检查|化验结果|诊断|初步诊断|出院诊断|评估|处理|治疗经过|住院经过|出院医嘱|复诊计划|随访计划|用药|目前用药|入院原因|出院情况|家长沟通|风险提示)\s*[:：]\s*(.*)$`,
		`^(chief complaint|history of present illness|hpi|assessment|plan|follow up|discharge diagnosis|hospital course|medications?)\s*[:：]\s*(.*)$`,
	}
	for _, pattern := range patterns {
		re := regexp.MustCompile(`(?i)` + pattern)
		if matches := re.FindStringSubmatch(line); len(matches) >= 3 {
			return strings.TrimSpace(matches[1]), strings.TrimSpace(matches[2]), true
		}
	}
	return "", "", false
}

func extractListFromSections(sections map[string]string, keys []string) []string {
	items := []string{}
	for key, value := range sections {
		if sectionMatches(key, keys) {
			items = append(items, splitClinicalItems(value)...)
		}
	}
	return uniqueSortedClinicalStrings(items)
}

func firstSectionValue(sections map[string]string, keys ...string) string {
	for key, value := range sections {
		if sectionMatches(key, keys) {
			return strings.TrimSpace(firstClinicalSentence(value))
		}
	}
	return ""
}

func sectionMatches(sectionKey string, targets []string) bool {
	normalizedKey := normalizeClinicalKey(sectionKey)
	for _, target := range targets {
		if normalizedKey == normalizeClinicalKey(target) {
			return true
		}
	}
	return false
}

func normalizeClinicalKey(v string) string {
	replacer := strings.NewReplacer(" ", "", "_", "", "-", "", ".", "", "（", "(", "）", ")")
	return strings.ToLower(strings.TrimSpace(replacer.Replace(v)))
}

func splitClinicalItems(value string) []string {
	replacer := strings.NewReplacer("\n", "；", ";", "；", "。", "；")
	parts := strings.Split(replacer.Replace(value), "；")
	items := []string{}
	for _, part := range parts {
		item := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(part, "-"), "1."))
		if item != "" {
			items = append(items, item)
		}
	}
	return items
}

func firstClinicalSentence(value string) string {
	replacer := strings.NewReplacer("\n", "。", ";", "。", "；", "。")
	parts := strings.Split(replacer.Replace(value), "。")
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			return trimmed
		}
	}
	return strings.TrimSpace(value)
}

func extractMedicationMentions(noteText string) []string {
	catalog := []string{"生长激素", "左甲状腺素", "优甲乐", "铁剂", "硫酸亚铁", "维生素D", "骨化三醇", "二甲双胍", "GnRHa", "亮丙瑞林", "曲普瑞林", "戈舍瑞林", "钙剂"}
	return extractClinicalMentions(noteText, catalog)
}

func extractLabMentions(noteText string) []string {
	catalog := []string{"血常规", "铁蛋白", "TSH", "FT4", "FT3", "25-OH维生素D", "维生素D", "血钙", "血磷", "ALP", "空腹血糖", "HbA1c", "肝功能", "血脂", "尿酸"}
	return extractClinicalMentions(noteText, catalog)
}

func extractClinicalMentions(noteText string, catalog []string) []string {
	mentions := []string{}
	text := strings.ToLower(noteText)
	for _, item := range catalog {
		if strings.Contains(text, strings.ToLower(item)) {
			mentions = append(mentions, item)
		}
	}
	return uniqueSortedClinicalStrings(mentions)
}

func extractRedFlags(noteText string) []string {
	catalog := []string{"立即就医", "尽快就医", "胸痛", "呼吸困难", "抽搐", "大出血", "持续呕吐", "明显脱水", "严重头痛", "视物模糊", "高热不退"}
	return extractClinicalMentions(noteText, catalog)
}

func detectFollowupWindow(noteText string) string {
	patterns := []string{
		`(\d+\s*(?:天|周|月)后复诊)`,
		`(\d+\s*(?:天|周|月)复查)`,
		`(复诊时间\s*[:：]?\s*[^\n。；]+)`,
	}
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(noteText); len(matches) >= 2 {
			return strings.TrimSpace(matches[1])
		}
	}
	if strings.Contains(noteText, "按月复诊") {
		return "按月复诊"
	}
	return "建议结合病历中的复诊计划安排"
}

func extractHomeTasks(noteText string) []string {
	catalog := []string{"规律服药", "按时复诊", "监测身高体重", "记录症状变化", "控制饮食", "加强运动", "保持睡眠", "完成化验复查", "按时注射", "家长观察不良反应"}
	return extractClinicalMentions(noteText, catalog)
}

func detectNoteType(noteText string, sections map[string]string) string {
	lower := strings.ToLower(noteText)
	switch {
	case strings.Contains(lower, "出院") || hasSection(sections, "出院诊断") || hasSection(sections, "出院医嘱"):
		return "discharge_summary"
	case hasSection(sections, "主诉") || hasSection(sections, "现病史") || hasSection(sections, "诊断"):
		return "outpatient_note"
	default:
		return "clinical_note"
	}
}

func hasSection(sections map[string]string, target string) bool {
	for key := range sections {
		if normalizeClinicalKey(key) == normalizeClinicalKey(target) {
			return true
		}
	}
	return false
}

func sortedSectionKeys(sections map[string]string) []string {
	keys := make([]string, 0, len(sections))
	for key := range sections {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func uniqueSortedClinicalStrings(items []string) []string {
	lookup := map[string]string{}
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		key := normalizeClinicalKey(trimmed)
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
