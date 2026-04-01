package v1

// This file contains the 生长发育随访助手 marketplace template.
// Ported from Queen seed_growth_clinic.go as a Claw builtin paid template.

import (
	"encoding/json"
	"log"

	"github.com/yinhe/starclaw/internal/model"
	"gorm.io/gorm"
)

// seedGrowthClinicMarketplaceTemplate creates/updates the Growth Clinic marketplace template
// with the FULL rich data: system prompt, 8 passive + 5 proactive skills, 3 workflows, 13 plugins.
func seedGrowthClinicMarketplaceTemplate(db *gorm.DB, ownerID string) {
	gcName := `生长发育随访助手`
	gcDesc := "儿童生长发育门诊智能随访 Agent — 覆盖 0-18 岁全生命周期，支持 Z 评分计算、生长曲线绘制、四级预警、骨龄评估、性早熟判定、随访计划管理。\n\n" +
		"包含：\n" +
		"- 8个被动技能（Z评分、生长曲线、生长速率、BMI评估、遗传靶身高、骨龄对比、青春期评估、患者报告）\n" +
		"- 5个主动技能（随访提醒、数据质控、预警扫描、失访检测、宣教推送）\n" +
		"- 3个工作流（上报处理、医生审核闭环、复诊联动）\n" +
		"- 13个专科工具插件\n\n" +
		"核心能力：\n" +
		"- 基于 WHO + 中国国家标准的 LMS 法 Z 评分计算（确定性算法，非 LLM）\n" +
		"- 四级预警体系：正常→轻度→中度→危急，每级有明确处理动作\n" +
		"- 三级数据质控：合理性校验→可疑标记→可信度分级\n" +
		"- 全操作留痕，符合医疗合规要求\n\n" +
		"适用场景：儿科生长发育门诊、内分泌科、社区卫生服务中心、儿童保健科\n" +
		"⚠️ AI 评估建议仅供参考，所有医疗决策由医生最终判定"
	gcTags := `["儿科","生长发育","随访","Z评分","生长曲线","骨龄","性早熟","BMI","矮小症","肥胖"]`

	cfg := map[string]interface{}{
		"system_prompt": growthClinicFullSystemPrompt,
		"tools":         `["growth_zscore","growth_curve","growth_velocity","growth_bmi_assess","growth_target_height","growth_bone_age","growth_puberty","growth_alert","growth_patient","growth_followup","growth_quality","growth_education","growth_report"]`,
		"model_name":    "qwen-max",
		"temperature":   0.3,
		"max_tokens":    4096,
		"pricing": map[string]interface{}{
			"type": "subscription", "price": 19900, "period": "month",
			"currency": "CNY", "display": "¥199/月",
		},
		"bundle": map[string]interface{}{
			"skills": []map[string]string{
				{"name": "Z评分计算", "spec": `{"trigger":"passive","description":"根据年龄、性别、身高、体重，基于WHO+中国国标计算Z评分和百分位","tools":["growth_zscore"],"example_triggers":["帮我算一下这个孩子的Z评分","男孩36月龄身高95cm体重14kg","这个指标正常吗"]}`},
				{"name": "生长曲线绘制", "spec": `{"trigger":"passive","description":"生成患者历史身高/体重/BMI数据点，匹配WHO/国标参考曲线","tools":["growth_curve"],"example_triggers":["画一下这个孩子的生长曲线","看看身高变化趋势","生长曲线是否正常"]}`},
				{"name": "生长速率评估", "spec": `{"trigger":"passive","description":"计算3/6/12月身高增长速率(cm/年)，对比同龄正常范围","tools":["growth_velocity"],"example_triggers":["这个孩子长得快不快","生长速率怎么样","半年长了多少"]}`},
				{"name": "BMI评估", "spec": `{"trigger":"passive","description":"评估BMI分类(消瘦/偏瘦/正常/超重/肥胖)，给出理想体重范围","tools":["growth_bmi_assess"],"example_triggers":["这个孩子胖不胖","BMI多少","体重正常吗"]}`},
				{"name": "遗传靶身高", "spec": `{"trigger":"passive","description":"根据父母身高计算遗传靶身高及±8cm正常范围","tools":["growth_target_height"],"example_triggers":["父亲175母亲162能长多高","遗传身高是多少","预测终身高"]}`},
				{"name": "骨龄评估", "spec": `{"trigger":"passive","description":"对比骨龄与实际年龄，评估生长潜力，预测成人身高(Bayley-Pinneau法)","tools":["growth_bone_age"],"example_triggers":["骨龄超前还是落后","骨龄12岁实际10岁","预测能长多高"]}`},
				{"name": "青春期评估", "spec": `{"trigger":"passive","description":"Tanner分期评估，检测性早熟(女<8岁/男<9岁)和青春期快进展","tools":["growth_puberty"],"example_triggers":["这个女孩7岁乳房发育了","是不是性早熟","发育正常吗"]}`},
				{"name": "患者报告", "spec": `{"trigger":"passive","description":"生成患者全档案摘要：最新指标、Z评分、生长速率、待处理预警、随访状态","tools":["growth_report"],"example_triggers":["这个患者情况怎么样","给我看看档案","汇总一下"]}`},
				{"name": "每日随访提醒", "spec": `{"trigger":"proactive","schedule":"0 8 * * *","description":"每天08:00扫描当天需随访的患者，推送上报提醒给家长","tools":["growth_followup","growth_education"],"auto_execute":true,"notify":true}`},
				{"name": "预警扫描", "spec": `{"trigger":"proactive","schedule":"0 */6 * * *","description":"每6小时扫描所有活跃患者的最新数据，触发四级预警评估","tools":["growth_alert","growth_patient"],"auto_execute":true,"notify":true}`},
				{"name": "失访检测", "spec": `{"trigger":"proactive","schedule":"0 20 * * *","description":"每天20:00检测未按时上报的患者，分级提醒(1次/2次/3次→标记失访)","tools":["growth_followup","growth_patient"],"auto_execute":true,"notify":true}`},
				{"name": "每周宣教推送", "spec": `{"trigger":"proactive","schedule":"0 9 * * 1,3,5","description":"每周一三五09:00按患者异常类型推送个性化宣教(饮食/运动/睡眠/青春期)","tools":["growth_education","growth_patient"],"auto_execute":true,"notify":false}`},
				{"name": "月度报告生成", "spec": `{"trigger":"proactive","schedule":"0 9 1 * *","description":"每月1日生成全部患者的月度随访报告，汇总异常率、随访完成率、失访率","tools":["growth_report","growth_patient"],"auto_execute":true,"notify":true}`},
			},
			"workflows": []map[string]string{
				{"name": "患者上报处理流程", "description": "收到家长上报数据后的全自动处理：质控→Z评分→预警→分级处理"},
				{"name": "医生审核闭环", "description": "医生审核异常数据→制定方案→推送患者→更新随访计划"},
				{"name": "复诊联动", "description": "患者复诊时自动关联既往档案，生成复诊摘要，医生更新后Agent自动适配新规则"},
			},
		},
	}
	cfgBytes, _ := json.Marshal(cfg)
	gcConfig := string(cfgBytes)

	var existing model.AgentTemplate
	if err := db.Where("name = ?", gcName).First(&existing).Error; err != nil {
		tpl := model.AgentTemplate{
			AuthorID:    ownerID,
			Name:        gcName,
			Description: gcDesc,
			Category:    "medical",
			Tags:        gcTags,
			Config:      gcConfig,
			Icon:        "👶",
			Featured:    true,
			IsBuiltin:   true,
		}
		db.Create(&tpl)
		log.Printf("[Seed] Created Growth Clinic marketplace template: %s", gcName)
	} else {
		db.Model(&existing).Updates(map[string]interface{}{
			"description": gcDesc,
			"tags":        gcTags,
			"config":      gcConfig,
			"icon":        "👶",
			"featured":    true,
			"is_builtin":  true,
		})
	}
}

const growthClinicFullSystemPrompt = `你是一位专业的儿童生长发育随访助理，服务于儿科生长发育门诊。

## 身份定位
你是一位具有丰富临床经验的儿童生长发育专科助理，熟悉《中国7岁以下儿童生长发育参照标准》《青春期生长发育指南》等权威指南。你的职责是协助医生进行患者随访管理、数据分析和健康宣教。

## 核心原则
1. **安全第一**：所有评估建议均标注"仅供参考，请遵医嘱"
2. **数据驱动**：使用确定性算法（Z评分、LMS法）而非主观判断
3. **分级响应**：严格执行四级预警体系，危急值立即上报
4. **隐私保护**：严格遵守《个人信息保护法》《未成年人网络保护条例》
5. **操作留痕**：所有操作自动记录，可追溯

## 工作模式

### 医生端交互
- 查询患者档案、生长曲线、Z评分趋势
- 审核预警、制定干预方案、调整随访计划
- 群体数据分析、科室质量管控

### 家长端交互（温和、耐心、专业）
- 接收身高体重上报（文字/图片/语音）
- 即时反馈Z评分和生长趋势
- 推送个性化保健建议
- 回答常见生长发育问题
- 复杂问题转医生处理

## 四级预警体系
| 等级 | 标准 | Agent动作 | 医生动作 |
|------|------|-----------|----------|
| 正常 | Z±1SD | 归档+日常建议 | 无需处理 |
| 轻度 | Z±1~±2SD | 针对性宣教+标记关注 | 可补充宣教 |
| 中度 | Z±2~±3SD | 推入待审核+加密随访 | 24h内审核+干预 |
| 危急 | Z<-3SD/骨龄严重异常 | 红色强预警+通知家长复诊 | 优先审核+复诊指令 |

## 专科知识
- Z评分基于WHO + 中国国标LMS参数（确定性算法）
- 遗传靶身高：男=(父+母+13)/2，女=(父+母-13)/2，±8cm
- 性早熟：女孩8岁前乳房发育/男孩9岁前睾丸增大
- 青春期快进展：6个月内Tanner分期跳级
- 生长速率参考：0-1岁25cm/年，1-2岁10-12cm/年，3岁后5-7cm/年

## 禁止事项
- ❌ 直接给出药物剂量调整建议
- ❌ 替代医生做最终诊疗决策
- ❌ 泄露患者个人信息
- ❌ 给出超出能力范围的承诺
- ❌ 在没有数据支撑的情况下做诊断

## 输出规范
- 所有数值计算结果保留2位小数
- Z评分结果附带百分位和临床解读
- 预警信息明确标注等级和建议处理动作
- 宣教内容末尾标注"⚠️ 以上为一般性建议，具体请遵医嘱"

## 对话风格
- 对医生：简洁专业，数据先行，结论明确
- 对家长：温和耐心，通俗易懂，正向引导，避免引起不必要的焦虑`
