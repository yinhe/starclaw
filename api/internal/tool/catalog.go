package tool

import "strings"

type CapabilityCatalog struct {
	Category         string   `json:"category"`
	CategoryLabel    string   `json:"category_label"`
	Subcategory      string   `json:"subcategory,omitempty"`
	SubcategoryLabel string   `json:"subcategory_label,omitempty"`
	Industry         string   `json:"industry"`
	Tags             []string `json:"tags,omitempty"`
}

func MetadataFor(t Tool) map[string]interface{} {
	switch tt := t.(type) {
	case *PluginTool:
		return tt.spec.Metadata
	case *LocalPluginTool:
		return tt.spec.Metadata
	default:
		return nil
	}
}

func DescribeCapability(name, typ string, metadata map[string]interface{}) CapabilityCatalog {
	lowerName := strings.ToLower(strings.TrimSpace(name))
	catalog := capability("general_tools", "通用工具", "general", "通用能力", "general", "general")

	switch {
	case typ == "mcp" || strings.HasPrefix(lowerName, "mcp:") || strings.HasPrefix(lowerName, "host.") || strings.HasPrefix(lowerName, "mcp_"):
		catalog = capability("integration", "外部集成", "mcp", "MCP 集成", "general", "integration", "mcp")
	case lowerName == "system":
		catalog = capability("platform_system", "平台系统", "orchestration", "编排与系统", "general", "system", "orchestration")
	case lowerName == "code":
		catalog = capability("engineering", "工程开发", "development", "开发执行", "general", "engineering", "development", "code")
	case lowerName == "deploy_web" || lowerName == "bind_domain" || lowerName == "verify_online":
		catalog = capability("engineering", "工程开发", "deployment", "部署运维", "general", "engineering", "deployment")
	case lowerName == "web_search" || lowerName == "http_request" || lowerName == "browser":
		catalog = capability("research", "信息检索", "web_research", "网页检索", "general", "research", "web")
	case lowerName == "video_generation" || lowerName == "image_generation" || lowerName == "music_generation" || lowerName == "audio_analysis" || lowerName == "mv_production" || lowerName == "dubbing" || lowerName == "comic_production":
		catalog = capability("creative_media", "多媒体创作", "media_generation", "媒体生成", "general", "creative", "media")
	case strings.HasPrefix(lowerName, "growth_"):
		catalog = capability("medical_clinical", "临床医疗", "growth_followup", "生长发育与随访", "medical", "medical", "clinical", "growth")
	case strings.HasPrefix(lowerName, "lab_"):
		catalog = capability("medical_clinical", "临床医疗", "lab_interpretation", "检验解读", "medical", "medical", "clinical", "lab")
	case strings.HasPrefix(lowerName, "medication_"):
		catalog = capability("medical_clinical", "临床医疗", "medication_safety", "药物安全", "medical", "medical", "clinical", "medication")
	case strings.HasPrefix(lowerName, "clinical_"):
		catalog = capability("medical_clinical", "临床医疗", "clinical_documentation", "病历结构化", "medical", "medical", "clinical", "documentation")
	case strings.HasPrefix(lowerName, "referral_"):
		catalog = capability("medical_clinical", "临床医疗", "referral_triage", "转诊分流", "medical", "medical", "clinical", "referral")
	case strings.HasPrefix(lowerName, "infection_"):
		catalog = capability("medical_clinical", "临床医疗", "infection_triage", "感染分流", "medical", "medical", "clinical", "infection")
	case strings.HasPrefix(lowerName, "allergy_"):
		catalog = capability("medical_clinical", "临床医疗", "allergy_immunology", "过敏与免疫", "medical", "medical", "clinical", "allergy")
	case strings.HasPrefix(lowerName, "neurodev_"):
		catalog = capability("medical_clinical", "临床医疗", "neurodevelopment", "神经发育", "medical", "medical", "clinical", "neurodevelopment")
	case strings.HasPrefix(lowerName, "trauma_"):
		catalog = capability("medical_clinical", "临床医疗", "trauma_triage", "外伤分流", "medical", "medical", "clinical", "trauma")
	case strings.HasPrefix(lowerName, "airway_"):
		catalog = capability("medical_clinical", "临床医疗", "airway_respiratory", "呼吸与气道", "medical", "medical", "clinical", "airway")
	case strings.HasPrefix(lowerName, "gastro_"):
		catalog = capability("medical_clinical", "临床医疗", "gastrointestinal", "消化系统", "medical", "medical", "clinical", "gastro")
	case strings.HasPrefix(lowerName, "diagnostics_"):
		catalog = capability("medical_clinical", "临床医疗", "diagnostics_review", "影像与病理", "medical", "medical", "clinical", "diagnostics")
	case strings.HasPrefix(lowerName, "wellness_"):
		catalog = capability("medical_clinical", "临床医疗", "wellness_lifestyle", "睡眠营养与生活方式", "medical", "medical", "clinical", "wellness")
	case strings.HasPrefix(lowerName, "mental_health_"):
		catalog = capability("medical_clinical", "临床医疗", "mental_health", "精神心理", "medical", "medical", "clinical", "mental_health")
	case strings.HasPrefix(lowerName, "device_"):
		catalog = capability("medical_clinical", "临床医疗", "home_monitoring", "家用监测设备", "medical", "medical", "clinical", "device")
	case strings.HasPrefix(lowerName, "pubmed_") || strings.HasPrefix(lowerName, "guideline_") || strings.HasPrefix(lowerName, "clinical_trials_"):
		catalog = capability("research", "科研数据库", "medical_evidence", "文献与循证", "medical", "medical", "research", "evidence")
	case strings.HasPrefix(lowerName, "genetics_") || strings.HasPrefix(lowerName, "variant_") || strings.HasPrefix(lowerName, "protein_") || strings.HasPrefix(lowerName, "drug_target_"):
		catalog = capability("bioinformatics", "生物信息", "medical_databases", "疾病与分子数据库", "medical", "medical", "bioinformatics", "database")
	case strings.HasPrefix(lowerName, "bioinformatics_"):
		catalog = capability("bioinformatics", "生物信息", "sequence_analysis", "序列分析", "medical", "medical", "bioinformatics")
	case strings.HasPrefix(lowerName, "trading_"):
		catalog = capability("finance_trading", "金融交易", "market_analysis", "行情分析", "finance", "finance", "trading")
	}

	catalog = applyMetadata(catalog, metadata)
	return catalog
}

func capability(category, categoryLabel, subcategory, subcategoryLabel, industry string, tags ...string) CapabilityCatalog {
	return CapabilityCatalog{
		Category:         category,
		CategoryLabel:    categoryLabel,
		Subcategory:      subcategory,
		SubcategoryLabel: subcategoryLabel,
		Industry:         industry,
		Tags:             appendUnique(nil, tags...),
	}
}

func applyMetadata(c CapabilityCatalog, metadata map[string]interface{}) CapabilityCatalog {
	if len(metadata) == 0 {
		return c
	}

	if c.Category == "general_tools" {
		switch strings.ToLower(readString(metadata["category"])) {
		case "medical":
			c = capability("medical_clinical", "临床医疗", "general_medical", "医疗通用", "medical", c.Tags...)
		case "bioinformatics":
			c = capability("bioinformatics", "生物信息", "general_bioinformatics", "生信通用", "medical", c.Tags...)
		case "trading":
			c = capability("finance_trading", "金融交易", "market_analysis", "行情分析", "finance", c.Tags...)
		case "research":
			c = capability("research", "信息检索", "general_research", "通用检索", "general", c.Tags...)
		}
	}

	if industry := strings.TrimSpace(readString(metadata["industry"])); industry != "" {
		c.Industry = industry
	}
	if subcategory := strings.TrimSpace(readString(metadata["subcategory"])); subcategory != "" {
		c.Subcategory = subcategory
		if c.SubcategoryLabel == "" {
			c.SubcategoryLabel = subcategory
		}
	}

	metaCategory := strings.TrimSpace(readString(metadata["category"]))
	if metaCategory != "" {
		c.Tags = appendUnique(c.Tags, strings.ToLower(metaCategory))
	}
	c.Tags = appendUnique(c.Tags, readStringSlice(metadata["tags"])...)
	return c
}

func readString(v interface{}) string {
	s, _ := v.(string)
	return s
}

func readStringSlice(v interface{}) []string {
	switch vv := v.(type) {
	case []string:
		return vv
	case []interface{}:
		out := make([]string, 0, len(vv))
		for _, item := range vv {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				out = append(out, strings.TrimSpace(s))
			}
		}
		return out
	default:
		return nil
	}
}

func appendUnique(dst []string, values ...string) []string {
	seen := make(map[string]bool, len(dst))
	for _, item := range dst {
		key := strings.ToLower(strings.TrimSpace(item))
		if key != "" {
			seen[key] = true
		}
	}
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		key := strings.ToLower(trimmed)
		if trimmed == "" || seen[key] {
			continue
		}
		dst = append(dst, trimmed)
		seen[key] = true
	}
	return dst
}
