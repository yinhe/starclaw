package v1

// This file contains the Cicada 🪰 蝉·电话机器人 marketplace template.
// It is EXCLUDED from OSS sync (proprietary telephony agent).

import (
	"encoding/json"
	"log"

	"github.com/yinhe/starclaw/internal/model"
	"gorm.io/gorm"
)

// seedCicadaMarketplaceTemplate creates/updates the Cicada 蝉 marketplace template
// with the FULL rich data: complete system prompt, 8 skills, 6 instincts,
// MCP Cicada Bridge, and outbound call workflows.
func seedCicadaMarketplaceTemplate(db *gorm.DB, ownerID string) {
	db.Where("name = ?", "Cicada 蝉·电话机器人").Delete(&model.AgentTemplate{})

	cicadaName := `Cicada 蝉·电话机器人`
	cicadaDesc := "AI电话外呼智能体 — 日呼800-1000通、A-F六级意向自动分类、通话录音+文字保存。替代人工坐席，效率提升10倍。\n\n" +
		"包含：\n" +
		"- 8个技能（拨号、语音识别、语音合成、CRM查询/更新/跟进、录音保存、短信）\n" +
		"- 6个本能（自动外呼、通话分类、日报、跟进提醒、话术优化、黑名单同步）\n" +
		"- MCP Cicada Bridge（ASR/TTS/SIP语音管道）\n" +
		"- 2个工作流（标准外呼流程、客户跟进流程）\n\n" +
		"6大行业内置话术：房产、教育、金融、装修、招商、医美\n" +
		"要求：需配置容联云账号 + DashScope API Key\n" +
		"数据安全：客户号码加密存储在您自己的节点"
	cicadaTags := `["电话机器人","外呼","CRM","销售","AI语音","ASR","TTS"]`

	cfg := map[string]interface{}{
		"system_prompt": cicadaFullSystemPrompt,
		"tools":         `["cicada_phone_call","cicada_phone_listen","cicada_phone_speak","cicada_crm_query","cicada_crm_update","cicada_crm_schedule","cicada_record_save","cicada_sms_send"]`,
		"model_name":    "qwen-turbo",
		"temperature":   0.3,
		"max_tokens":    200,
		"pricing": map[string]interface{}{
			"type": "subscription", "price": 98000, "period": "month",
			"currency": "CNY", "display": "¥980/月起",
			"tiers": []map[string]interface{}{
				{"name": "入门版", "price": 98000, "features": "200通/天, 3路并发, 3000分钟"},
				{"name": "专业版", "price": 248000, "features": "500通/天, 5路并发, 8000分钟"},
				{"name": "企业版", "price": 498000, "features": "1000通/天, 10路并发, 20000分钟"},
				{"name": "旗舰版", "price": 980000, "features": "3000通/天, 30路并发, 60000分钟"},
			},
		},
		"bundle": map[string]interface{}{
			"skills": []map[string]string{
				{"name": "电话拨号", "spec": `{"trigger":"passive","description":"发起外呼电话、挂断通话、转接人工坐席","tools":["cicada_phone_call"],"example_triggers":["拨打这个号码","给客户打电话","挂断","转人工"]}`},
				{"name": "语音识别", "spec": `{"trigger":"passive","description":"实时语音识别，将客户语音转为文字（流式ASR）","tools":["cicada_phone_listen"],"example_triggers":["开始识别","听客户说什么"]}`},
				{"name": "语音合成", "spec": `{"trigger":"passive","description":"将AI回复文字转为语音播放给客户（流式TTS）","tools":["cicada_phone_speak"],"example_triggers":["回复客户","播放语音"]}`},
				{"name": "客户查询", "spec": `{"trigger":"passive","description":"查询客户信息、历史通话记录、意向等级","tools":["cicada_crm_query"],"example_triggers":["查一下这个客户","看看历史通话","客户意向怎么样"]}`},
				{"name": "客户更新", "spec": `{"trigger":"passive","description":"更新客户分类、标签、备注、意向等级","tools":["cicada_crm_update"],"example_triggers":["把这个客户标记为A类","更新客户备注","加个标签"]}`},
				{"name": "跟进安排", "spec": `{"trigger":"passive","description":"设置客户跟进提醒，安排下次回访时间","tools":["cicada_crm_schedule"],"example_triggers":["三天后再打","安排跟进","设置提醒"]}`},
				{"name": "录音保存", "spec": `{"trigger":"passive","description":"保存通话录音文件和ASR转写文字","tools":["cicada_record_save"],"example_triggers":["保存录音","存下这通电话"]}`},
				{"name": "短信发送", "spec": `{"trigger":"passive","description":"通话后发送资料短信给客户","tools":["cicada_sms_send"],"example_triggers":["发个短信","把资料发给客户"]}`},
			},
			"instincts": []map[string]string{
				{"name": "自动外呼", "spec": `{"trigger":"proactive","schedule":"0 0 9,14 * * 1-6","description":"在设定时段（工作日9:00和14:00）自动开始批量外呼任务","auto_execute":true,"notify":true}`},
				{"name": "通话分类", "spec": `{"trigger":"proactive","event":"call_ended","description":"通话结束后，分析完整对话文本，自动判定A-F意向等级","auto_execute":true,"notify":false}`},
				{"name": "日报生成", "spec": `{"trigger":"proactive","schedule":"0 30 18 * * *","description":"每日18:30汇总今日外呼数据，生成日报（呼叫量/接通率/A类客户数）","auto_execute":true,"notify":true}`},
				{"name": "跟进提醒", "spec": `{"trigger":"proactive","schedule":"0 0 9 * * *","description":"每日09:00检查今日需跟进的客户列表，发送提醒","auto_execute":true,"notify":true}`},
				{"name": "话术优化", "spec": `{"trigger":"proactive","schedule":"0 0 22 * * 0","description":"每周日22:00分析本周通话数据，优化话术模板和应答策略","auto_execute":true,"notify":true}`},
				{"name": "黑名单同步", "spec": `{"trigger":"proactive","event":"customer_unsubscribe","description":"客户要求退订时，立即加入黑名单，72小时内同步到所有外呼任务","auto_execute":true,"notify":false}`},
			},
			"mcp_servers": []map[string]string{
				{"name": "Cicada Bridge", "base_url": "http://localhost:8099", "description": "Cicada 蝉 电话桥接层 — ASR语音识别 + TTS语音合成 + SIP电话管道 + CRM客户管理"},
			},
			"workflows": []map[string]string{
				{"name": "标准外呼工作流", "description": "导入号码 → 去重+合规检查 → 创建外呼任务 → 自动拨号 → 语音对话 → 意向分类 → 保存录音 → 更新CRM → A/B类客户通知业务员 → 日报"},
				{"name": "客户跟进工作流", "description": "检查跟进队列 → 调取历史通话 → 生成个性化话术 → 拨号 → 对话 → 更新状态"},
			},
		},
	}
	cfgBytes, _ := json.Marshal(cfg)
	cicadaConfig := string(cfgBytes)

	var existing model.AgentTemplate
	if err := db.Where("name = ?", cicadaName).First(&existing).Error; err != nil {
		tpl := model.AgentTemplate{
			AuthorID:    ownerID,
			Name:        cicadaName,
			Description: cicadaDesc,
			Category:    "sales",
			Tags:        cicadaTags,
			Config:      cicadaConfig,
			Icon:        "\U0001FAB0",
			Featured:    true,
			IsBuiltin:   true,
		}
		db.Create(&tpl)
		log.Printf("[Seed] Created Cicada marketplace template: %s", cicadaName)
	} else {
		db.Model(&existing).Updates(map[string]interface{}{
			"description": cicadaDesc,
			"tags":        cicadaTags,
			"config":      cicadaConfig,
			"icon":        "\U0001FAB0",
			"featured":    true,
			"is_builtin":  true,
		})
	}
}

const cicadaFullSystemPrompt = `你是 Cicada 蝉·AI电话机器人智能体。

## 身份定位
你是一个专业的电话外呼AI，替代人工坐席完成批量外呼任务。你的对话自然流畅，像真人一样亲切，同时高效识别客户意向。

## 核心规则
1. 每句话不超过30字，语速适中，像真人一样自然
2. 不主动透露自己是机器人，但被问到时如实回答
3. 客户说"不需要"/"别打了"时，礼貌结束通话并说再见
4. 根据客户提问深度判断意向等级(A-F)
5. 始终引导客户说出需求关键词
6. 禁止承诺任何不确定的信息
7. 禁止使用"最好""第一""绝对"等绝对化用语

## 意向分类标准
- A(80-100分): 主动询问价格/付款/预约，≥3个深度问题 → 立即通知业务员
- B(60-79分): 询问具体细节(位置/配置/时间)，≥2个问题 → 48小时跟进
- C(40-59分): 有兴趣但不深入，1个问题 → 3天后二次外呼
- D(20-39分): 未拒绝但无实质问题 → 7天后二次外呼
- E(1-19分): 明确拒绝 → 30天不再外呼
- F(0分): 无效号码 → 不再外呼

## 对话策略
1. 开场白简短有力，10秒内说明来意
2. 根据行业话术模板引导对话
3. 客户提问时优先从QA库匹配回答
4. 遇到异议按异议处理模板应对
5. 识别到购买信号时及时引导预约/成交
6. 通话结束前根据意向选择合适的结束语

## 合规要求
- 遵守《个人信息保护法》，号码来源需合法授权
- 遵守《反电信诈骗法》，开场明确身份
- 提供退订机制（按9退订）
- 禁止虚假宣传和绝对化用语`
