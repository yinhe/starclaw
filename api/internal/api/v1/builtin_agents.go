package v1

import (
	"fmt"
	"log"

	"github.com/yinhe/starclaw/internal/model"
	"gorm.io/gorm"
)

// SeedBuiltinAgents creates system-level built-in agents on startup (visible to all users).
// If an Owner user exists, builtin agents are owned by the Owner.
// Otherwise they use model.SystemUserID as a placeholder (migrated to Owner on setup).
func SeedBuiltinAgents(db *gorm.DB) {
	ownerID := getOwnerOrSystemID(db)

	superDesc := "你的首席 AI 助手。统领全部专业 Agent，简单任务亲自执行，复杂任务智能委派。搜索研究、内容创作、编程开发、文档处理，无所不能。"
	superTools := `["code","system","browser","web_search","http_request","video_generation","dubbing","mv_production","comic_production","music_generation","image_generation","audio_analysis","document","desktop","arena"]`

	// Temporarily disable FK checks for seeding (model_id is NULL for system agents)
	db.Exec("SET FOREIGN_KEY_CHECKS = 0")
	defer db.Exec("SET FOREIGN_KEY_CHECKS = 1")

	// Seed SuperAgent (search by both ownerID and old "system" for backward compat)
	var superAgent model.Agent
	if err := db.Where("(user_id = ? OR user_id = ?) AND name = ?", ownerID, model.SystemUserID, "全能助手").First(&superAgent).Error; err != nil {
		superAgent = model.Agent{
			UserID:       ownerID,
			Name:         "全能助手",
			Description:  superDesc,
			Tools:        superTools,
			Config:       `{"temperature":0.3,"max_tokens":8192}`,
			IsPublic:     true,
			IsBuiltin:    true,
			SystemPrompt: superAgentSystemPrompt,
		}
		db.Create(&superAgent)
		log.Println("[Seed] Created system SuperAgent: 全能助手")
	} else {
		db.Model(&superAgent).Updates(map[string]interface{}{
			"system_prompt": superAgentSystemPrompt,
			"tools":         superTools,
			"description":   superDesc,
			"is_builtin":    true,
			"is_public":     true,
		})
	}

	// Update SuperAgent workflow to 总管决策流程
	updateSuperAgentWorkflow(db, ownerID, superAgent.Name)

	// Clean up deprecated/duplicate activities (daily_review → self_improve, schedule_reminder → remind_check)
	db.Where("template IN ?", []string{"daily_review", "schedule_reminder"}).Delete(&model.Activity{})

	// Seed Q8bot 麒博 marketplace template (replace old English version)
	seedQ8botMarketplaceTemplate(db, ownerID)

	// Seed Cicada 蝉·电话机器人 marketplace template
	seedCicadaMarketplaceTemplate(db, ownerID)

	// Seed 生长发育随访助手 marketplace template
	seedGrowthClinicMarketplaceTemplate(db, ownerID)

	// Clean up garbled and untranslated marketplace templates
	cleanupMarketplaceTemplates(db)

	// ── Aggressive dedup: remove duplicate and garbled agents ──
	cleanupDuplicateAgents(db, ownerID)

	// Seed/update specialist agents (MV, 视频, 音乐, etc.)
	for _, def := range builtinAgents {
		if def.ManifestID != "" {
			var manifestAgent model.Agent
			if err := db.Where("manifest_id = ? AND (user_id = ? OR user_id = ?)", def.ManifestID, ownerID, model.SystemUserID).First(&manifestAgent).Error; err == nil {
				db.Model(&manifestAgent).Updates(map[string]interface{}{
					"is_builtin": true,
					"is_public":  true,
				})
				db.Where("(user_id = ? OR user_id = ?) AND name = ? AND (manifest_id IS NULL OR manifest_id = '') AND id != ?", ownerID, model.SystemUserID, def.Name, manifestAgent.ID).Delete(&model.Agent{})
				continue
			}

			var legacyAgent model.Agent
			if err := db.Where("(user_id = ? OR user_id = ?) AND name = ? AND (manifest_id IS NULL OR manifest_id = '')", ownerID, model.SystemUserID, def.Name).First(&legacyAgent).Error; err == nil {
				db.Model(&legacyAgent).Updates(map[string]interface{}{
					"manifest_id": def.ManifestID,
					"is_builtin":  true,
					"is_public":   true,
				})
				db.Where("(user_id = ? OR user_id = ?) AND name = ? AND (manifest_id IS NULL OR manifest_id = '') AND id != ?", ownerID, model.SystemUserID, def.Name, legacyAgent.ID).Delete(&model.Agent{})
				continue
			}

			continue
		}

		var agent model.Agent
		if err := db.Where("(user_id = ? OR user_id = ?) AND name = ?", ownerID, model.SystemUserID, def.Name).First(&agent).Error; err != nil {
			agent = model.Agent{
				UserID:       ownerID,
				Name:         def.Name,
				Description:  def.Description,
				Tools:        def.Tools,
				SystemPrompt: def.Prompt,
				Config:       `{"temperature":0.5,"max_tokens":8192}`,
				IsPublic:     true,
				IsBuiltin:    true,
			}
			db.Create(&agent)
			log.Printf("[Seed] Created builtin agent: %s", def.Name)
		} else {
			db.Model(&agent).Updates(map[string]interface{}{
				"system_prompt": def.Prompt,
				"tools":         def.Tools,
				"description":   def.Description,
				"is_builtin":    true,
				"is_public":     true,
			})
		}
	}
}

// getOwnerOrSystemID returns the Owner's user ID if one exists, otherwise model.SystemUserID.
func getOwnerOrSystemID(db *gorm.DB) string {
	var owner model.User
	if err := db.Where("owner_token IS NOT NULL AND owner_token != ''").First(&owner).Error; err == nil {
		return owner.ID
	}
	return model.SystemUserID
}

// MigrateSystemToOwner reassigns all system-owned agents, templates, workflows,
// tasks, schedules and notifications to the real Owner user.
// Called after Setup completes to clean up the placeholder system user.
func MigrateSystemToOwner(db *gorm.DB, ownerID string) {
	if ownerID == "" || ownerID == model.SystemUserID {
		return
	}
	tables := []string{"agents", "agent_templates", "workflows", "tasks", "schedules", "notifications"}
	for _, table := range tables {
		result := db.Exec("UPDATE "+table+" SET user_id = ? WHERE user_id = ?", ownerID, model.SystemUserID)
		if result.RowsAffected > 0 {
			log.Printf("[Migration] Reassigned %d rows in %s from %s to %s", result.RowsAffected, table, model.SystemUserID, ownerID)
		}
	}
	// Also migrate author_id in agent_templates
	result := db.Exec("UPDATE agent_templates SET author_id = ? WHERE author_id = ?", ownerID, model.SystemUserID)
	if result.RowsAffected > 0 {
		log.Printf("[Migration] Reassigned %d template authors from %s to %s", result.RowsAffected, model.SystemUserID, ownerID)
	}
	// Delete the orphan system user record if it exists
	db.Exec("DELETE FROM users WHERE id = ? AND owner_token IS NULL", model.SystemUserID)
	log.Printf("[Migration] System user cleanup complete for owner %s", ownerID)
}

// ════════════════════════════════════════════════════════════════
//  Marketplace Cleanup: fix garbled + translate English → Chinese
// ════════════════════════════════════════════════════════════════

// cleanupMarketplaceTemplates removes garbled (mojibake) and untranslated
// English templates from the marketplace — only Chinese content survives.
func cleanupMarketplaceTemplates(db *gorm.DB) {
	// Step 1: Translate known English templates to Chinese FIRST
	// (so they gain CJK characters and survive the step-2 purge)
	type zhTrans struct{ Name, Desc string }
	translations := map[string]zhTrans{
		"Animal Chefs":                 {"动物厨师", "趣味动物厨师，输入食物名称即可获取独特创意食谱。"},
		"AI Doctor":                    {"AI 医生", "利用顶级医疗资源，提供经过验证的健康建议和医学信息查询。"},
		"img2img":                      {"图生图", "上传一张图片，使用 DALL·E 3 进行风格转换和创意重绘。"},
		"Briefly":                      {"一句话精简", "相同含义，更少文字。提交你的文本，帮你精炼浓缩。"},
		"High-Quality Review Analyzer": {"高质量评论分析", "分析网页上的评论内容，提供可执行的反馈和改进建议。"},
		"Prompt Perfect":               {"提示词优化", "自动优化你的 AI 提示词，获得更精准的回复。"},
		"Code Copilot":                 {"代码副驾驶", "智能编程助手，帮你写代码、调试、解释和重构。"},
		"Data Analyst":                 {"数据分析师", "专业数据分析，上传文件即可获得洞察和可视化。"},
		"Creative Writing Coach":       {"创意写作教练", "提升你的写作技巧，从构思到润色全程指导。"},
		"Logo Creator":                 {"Logo 设计师", "专业 Logo 设计，根据描述生成独特的品牌标识。"},
		"SEO Mentor":                   {"SEO 导师", "搜索引擎优化专家，提升网站排名和流量。"},
		"Copywriter":                   {"文案大师", "专业广告文案撰写，吸引眼球的营销内容。"},
		"Math Mentor":                  {"数学导师", "数学学习助手，从基础到高等数学全面辅导。"},
		"Resume Builder":               {"简历优化", "专业简历撰写和优化，助你脱颖而出。"},
		"Travel Guide":                 {"旅行规划师", "全球旅行规划和推荐，打造完美旅程。"},
		"Fitness Coach":                {"健身教练", "个性化健身计划和营养建议。"},
		"Language Tutor":               {"语言学习导师", "多语言学习助手，口语练习和语法纠正。"},
		"Excel Expert":                 {"Excel 专家", "Excel 公式、数据透视表、VBA 宏编程全能助手。"},
		"SQL Expert":                   {"SQL 专家", "数据库查询优化、SQL 编写和数据建模。"},
		"Email Writer":                 {"邮件写手", "专业邮件撰写，商务沟通和客户跟进。"},
		"Presentation Expert":          {"PPT 专家", "专业演示文稿设计和内容策划。"},
		"Story Teller":                 {"故事创作家", "创意故事和小说创作，从大纲到完稿。"},
		"Diagram Wizard":               {"图表向导", "流程图、架构图、思维导图专业制作。"},
		"API Docs":                     {"API 文档助手", "API 文档撰写、测试和交互式示例生成。"},
	}
	for oldName, tr := range translations {
		result := db.Model(&model.AgentTemplate{}).Where("name = ?", oldName).
			Updates(map[string]interface{}{"name": tr.Name, "description": tr.Desc})
		if result.RowsAffected > 0 {
			log.Printf("[Cleanup] Translated: %s → %s", oldName, tr.Name)
		}
	}

	// Step 2: Delete ALL non-builtin templates whose name lacks CJK characters.
	// This catches both garbled/mojibake (CP1252 artifacts) AND remaining English-only.
	var templates []model.AgentTemplate
	db.Where("is_builtin = ?", false).Select("id, name").Find(&templates)

	deleted := 0
	for _, t := range templates {
		if !hasCJK(t.Name) {
			db.Where("template_id = ?", t.ID).Delete(&model.AgentListing{})
			db.Delete(&t)
			deleted++
		}
	}
	if deleted > 0 {
		log.Printf("[Cleanup] Removed %d garbled/English-only marketplace templates", deleted)
	}
}

// hasCJK returns true if s contains at least one CJK Unified Ideograph (U+4E00–U+9FFF).
func hasCJK(s string) bool {
	for _, r := range s {
		if r >= 0x4E00 && r <= 0x9FFF {
			return true
		}
	}
	return false
}

// cleanupDuplicateAgents removes duplicate and garbled agents from the database.
// Rules:
//  1. For each manifest_id, keep only the LATEST agent, delete the rest.
//  2. For each agent name, keep only ONE (prefer manifest_id > is_builtin > newest).
//  3. Remove agents whose name has no CJK characters AND no manifest_id (garbled/English orphans).
func cleanupDuplicateAgents(db *gorm.DB, ownerID string) {
	totalDeleted := 0

	// Step 1: Dedup by manifest_id — keep only the latest per manifest_id
	type midGroup struct {
		ManifestID string
		Cnt        int64
	}
	var groups []midGroup
	db.Model(&model.Agent{}).
		Select("manifest_id, COUNT(*) as cnt").
		Where("manifest_id IS NOT NULL AND manifest_id != '' AND (user_id = ? OR user_id = ?)", ownerID, model.SystemUserID).
		Group("manifest_id").Having("cnt > 1").Find(&groups)

	for _, g := range groups {
		var agents []model.Agent
		db.Where("manifest_id = ? AND (user_id = ? OR user_id = ?)", g.ManifestID, ownerID, model.SystemUserID).
			Order("updated_at DESC").Find(&agents)
		if len(agents) > 1 {
			for _, a := range agents[1:] {
				db.Unscoped().Where("id = ?", a.ID).Delete(&model.Agent{})
				totalDeleted++
			}
		}
	}

	// Step 2: Dedup by name — keep the one with manifest_id, delete others
	type nameGroup struct {
		Name string
		Cnt  int64
	}
	var nameGroups []nameGroup
	db.Model(&model.Agent{}).
		Select("name, COUNT(*) as cnt").
		Where("(user_id = ? OR user_id = ?)", ownerID, model.SystemUserID).
		Group("name").Having("cnt > 1").Find(&nameGroups)

	for _, g := range nameGroups {
		var agents []model.Agent
		db.Where("name = ? AND (user_id = ? OR user_id = ?)", g.Name, ownerID, model.SystemUserID).
			Order("CASE WHEN manifest_id IS NOT NULL AND manifest_id != '' THEN 0 ELSE 1 END, is_builtin DESC, updated_at DESC").
			Find(&agents)
		if len(agents) > 1 {
			for _, a := range agents[1:] {
				db.Unscoped().Where("id = ?", a.ID).Delete(&model.Agent{})
				totalDeleted++
			}
		}
	}

	// Step 3: Remove garbled agents (no CJK in name, no manifest_id, not the SuperAgent)
	var allAgents []model.Agent
	db.Where("(manifest_id IS NULL OR manifest_id = '') AND (user_id = ? OR user_id = ?)", ownerID, model.SystemUserID).
		Select("id, name, description").Find(&allAgents)
	for _, a := range allAgents {
		if !hasCJK(a.Name) && a.Name != "全能助手" {
			db.Unscoped().Where("id = ?", a.ID).Delete(&model.Agent{})
			totalDeleted++
		}
	}

	// Step 4: Remove agents with empty/null description AND no manifest_id AND no system_prompt
	db.Unscoped().Where("(manifest_id IS NULL OR manifest_id = '') AND (description IS NULL OR description = '') AND (system_prompt IS NULL OR system_prompt = '') AND (user_id = ? OR user_id = ?)", ownerID, model.SystemUserID).
		Delete(&model.Agent{})

	// Step 5: Purge all previously soft-deleted agents
	result := db.Unscoped().Where("deleted_at IS NOT NULL AND (user_id = ? OR user_id = ?)", ownerID, model.SystemUserID).Delete(&model.Agent{})
	if result.RowsAffected > 0 {
		log.Printf("[Cleanup] Purged %d soft-deleted agent rows", result.RowsAffected)
	}

	if totalDeleted > 0 {
		log.Printf("[Cleanup] Removed %d duplicate/garbled agents", totalDeleted)
	}
}

// Built-in specialist agent definitions
// Each agent has a focused prompt + specific tools for its domain

type builtinAgentDef struct {
	Name        string
	Description string
	Tools       string // JSON array
	Prompt      string
	ManifestID  string
	Workflow    string // JSON workflow definition {nodes, edges}  auto-created for agent
}

var builtinAgents = []builtinAgentDef{
	{
		Name:        "剪辑师",
		Description: "短剧团队·后期工坊(视频)。FFmpeg标准化拼接·concat copy无损·H.264/720×1280/24fps/AAC 44.1kHz·转场·字幕烧录·多平台适配。",
		Tools:       `["audio_analysis","video_generation","mv_production","image_generation","code"]`,
		Prompt:      mvAgentPrompt,
	},
	{
		Name:        "摄影指导",
		Description: "短剧团队·制作工坊。Seedance 2.0链式生成·[图N]标签角色一致性·角色sheet→TOS转换·尾帧链式·中文prompt·720×1280竖屏。",
		Tools:       `["video_generation","image_generation","dubbing","code"]`,
		Prompt:      videoAgentPrompt,
	},
	{
		Name:        "音乐总监",
		Description: "短剧团队·后期工坊(音频)。BGM配乐·amix normalize=0·alimiter混音·Seedance原声复用·loudnorm I=-14拖音标准。",
		Tools:       `["music_generation","code"]`,
		Prompt:      musicAgentPrompt,
	},
	{
		Name:        "编程Agent",
		Description: "全栈编程：编写代码、创建网站、调试程序、部署应用。支持14种编程语言。",
		Tools:       `["code"]`,
		Prompt:      codingAgentPrompt,
	},
	{
		Name:        "研究分析Agent",
		Description: "互联网研究：搜索信息、浏览网页、抓取数据、整理分析报告。",
		Tools:       `["web_search","browser","http_request"]`,
		Prompt:      researchAgentPrompt,
	},
	{
		Name:        "漫剧创作Agent",
		Description: "AI漫剧制作：编写剧本、生成漫画风格图片、多角色配音、组装成漫剧视频。支持多种漫画风格。",
		Tools:       `["image_generation","comic_production","music_generation"]`,
		Prompt:      comicAgentPrompt,
	},
	{
		Name:        "商业计划书Agent",
		Description: "专业商业计划书撰写：市场调研、竞品分析、商业模式设计、财务预测、融资方案。输出投资人级别的BP文档。",
		Tools:       `["web_search","browser","http_request","code"]`,
		Prompt:      businessPlanAgentPrompt,
	},
	{
		Name:        "总导演",
		Description: "AI短剧全链路制作团队。角色工坊→剧本工坊→制作工坊→后期工坊。Seedance 2.0 + [图N]标签角色一致性 + 尾帧链式生成。EP01-EP04四集实战经验编码。",
		Tools:       `["video_generation","dubbing","music_generation","image_generation","mv_production","web_search","browser","code","desktop"]`,
		Prompt:      shortDramaAgentPrompt,
		ManifestID:  "short_drama",
	},
	{
		Name:       "抖音爆款导演",
		ManifestID: "douyin_viral",
	},
}

// updateSuperAgentWorkflow ensures the SuperAgent's workflow contains the 总管决策流程 nodes.
func updateSuperAgentWorkflow(db *gorm.DB, ownerID, agentName string) {
	wfTag := fmt.Sprintf("[agent:%s]", agentName)
	superWorkflowDef := `{"nodes":[{"id":"start","type":"start","position":{"x":400,"y":30},"data":{"label":"开始"}},{"id":"step-1","type":"llm","position":{"x":400,"y":130},"data":{"label":"理解意图","description":"分析用户需求的类型和复杂度"}},{"id":"step-2","type":"condition","position":{"x":400,"y":250},"data":{"label":"路由决策","description":"简单任务直接执行 / 专业任务委派 Agent"}},{"id":"step-3a","type":"tool","position":{"x":200,"y":370},"data":{"label":"直接执行","toolName":"system","description":"调用合适的工具组合完成任务"}},{"id":"step-3b","type":"tool","position":{"x":600,"y":370},"data":{"label":"委派 Agent","toolName":"system","description":"delegate_to_agent 委派给专业 Agent"}},{"id":"step-4","type":"llm","position":{"x":400,"y":490},"data":{"label":"质量检查","description":"验证执行结果，确保符合用户预期"}},{"id":"step-5","type":"llm","position":{"x":400,"y":600},"data":{"label":"交付汇报","description":"展示结果，提供后续建议"}},{"id":"step-6","type":"tool","position":{"x":400,"y":710},"data":{"label":"记忆归档","toolName":"document","description":"提取对话关键信息存入长期记忆"}},{"id":"end","type":"end","position":{"x":400,"y":820},"data":{"label":"完成"}}],"edges":[{"id":"e-s1","source":"start","target":"step-1"},{"id":"e-12","source":"step-1","target":"step-2"},{"id":"e-2a","source":"step-2","target":"step-3a","data":{"label":"简单任务"}},{"id":"e-2b","source":"step-2","target":"step-3b","data":{"label":"专业任务"}},{"id":"e-a4","source":"step-3a","target":"step-4"},{"id":"e-b4","source":"step-3b","target":"step-4"},{"id":"e-45","source":"step-4","target":"step-5"},{"id":"e-56","source":"step-5","target":"step-6"},{"id":"e-6e","source":"step-6","target":"end"}]}`

	var wf model.Workflow
	if err := db.Where("user_id = ? AND description LIKE ?", ownerID, "%"+wfTag+"%").First(&wf).Error; err == nil {
		db.Model(&wf).Updates(map[string]interface{}{
			"definition": superWorkflowDef,
			"name":       agentName + " 工作流",
		})
		log.Printf("[Seed] Updated SuperAgent workflow: %s", wf.ID)
	}
}
