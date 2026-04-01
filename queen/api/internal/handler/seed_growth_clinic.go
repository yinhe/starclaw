package handler

// growthClinicAgents returns the Growth & Development Clinic follow-up agent for the marketplace.
func growthClinicAgents() []officialAgent {
	return []officialAgent{
		{
			Name: "生长发育随访助手",
			Description: "儿童生长发育门诊智能随访 Agent — 覆盖 0-18 岁全生命周期，支持 Z 评分计算、生长曲线绘制、四级预警、骨龄评估、性早熟判定、随访计划管理。\n\n" +
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
				"⚠️ AI 评估建议仅供参考，所有医疗决策由医生最终判定",
			Icon:     "Baby",
			Tags:     "儿科,生长发育,随访,Z评分,生长曲线,骨龄,性早熟,BMI,矮小症,肥胖",
			Category: "medical",
			Prompt:   growthClinicSystemPrompt,
			Tools:    `["growth_zscore","growth_curve","growth_velocity","growth_bmi_assess","growth_target_height","growth_bone_age","growth_puberty","growth_alert","growth_patient","growth_followup","growth_quality","growth_education","growth_report"]`,
			// ¥199/月 订阅制
			Pricing:    "subscription",
			PriceCents: 19900,
			DemoURL:    "",
			Featured:   true,
			ModelName:  "qwen-max",

			// ── Full installation bundle ──
			Skills:    growthClinicSkills(),
			Workflows: growthClinicWorkflows(),
			Plugins:   growthClinicPlugins(),
		},
	}
}

// ── 被动技能 ×8 + 主动技能 ×5 ──

func growthClinicSkills() []AgentSkillSpec {
	return []AgentSkillSpec{
		// 被动技能 (passive) — 用户/医生提问时触发
		{
			Name:    "Z评分计算",
			Version: "1.0.0",
			Spec: `{"trigger":"passive","description":"根据年龄、性别、身高、体重，基于WHO+中国国标计算Z评分和百分位",` +
				`"tools":["growth_zscore"],` +
				`"example_triggers":["帮我算一下这个孩子的Z评分","男孩36月龄身高95cm体重14kg","这个指标正常吗"]}`,
		},
		{
			Name:    "生长曲线绘制",
			Version: "1.0.0",
			Spec: `{"trigger":"passive","description":"生成患者历史身高/体重/BMI数据点，匹配WHO/国标参考曲线",` +
				`"tools":["growth_curve"],` +
				`"example_triggers":["画一下这个孩子的生长曲线","看看身高变化趋势","生长曲线是否正常"]}`,
		},
		{
			Name:    "生长速率评估",
			Version: "1.0.0",
			Spec: `{"trigger":"passive","description":"计算3/6/12月身高增长速率(cm/年)，对比同龄正常范围",` +
				`"tools":["growth_velocity"],` +
				`"example_triggers":["这个孩子长得快不快","生长速率怎么样","半年长了多少"]}`,
		},
		{
			Name:    "BMI评估",
			Version: "1.0.0",
			Spec: `{"trigger":"passive","description":"评估BMI分类(消瘦/偏瘦/正常/超重/肥胖)，给出理想体重范围",` +
				`"tools":["growth_bmi_assess"],` +
				`"example_triggers":["这个孩子胖不胖","BMI多少","体重正常吗"]}`,
		},
		{
			Name:    "遗传靶身高",
			Version: "1.0.0",
			Spec: `{"trigger":"passive","description":"根据父母身高计算遗传靶身高及±8cm正常范围",` +
				`"tools":["growth_target_height"],` +
				`"example_triggers":["父亲175母亲162能长多高","遗传身高是多少","预测终身高"]}`,
		},
		{
			Name:    "骨龄评估",
			Version: "1.0.0",
			Spec: `{"trigger":"passive","description":"对比骨龄与实际年龄，评估生长潜力，预测成人身高(Bayley-Pinneau法)",` +
				`"tools":["growth_bone_age"],` +
				`"example_triggers":["骨龄超前还是落后","骨龄12岁实际10岁","预测能长多高"]}`,
		},
		{
			Name:    "青春期评估",
			Version: "1.0.0",
			Spec: `{"trigger":"passive","description":"Tanner分期评估，检测性早熟(女<8岁/男<9岁)和青春期快进展",` +
				`"tools":["growth_puberty"],` +
				`"example_triggers":["这个女孩7岁乳房发育了","是不是性早熟","发育正常吗"]}`,
		},
		{
			Name:    "患者报告",
			Version: "1.0.0",
			Spec: `{"trigger":"passive","description":"生成患者全档案摘要：最新指标、Z评分、生长速率、待处理预警、随访状态",` +
				`"tools":["growth_report"],` +
				`"example_triggers":["这个患者情况怎么样","给我看看档案","汇总一下"]}`,
		},
		// 主动技能 (proactive) — 定时自动执行
		{
			Name:    "每日随访提醒",
			Version: "1.0.0",
			Spec: `{"trigger":"proactive","schedule":"0 8 * * *",` +
				`"description":"每天08:00扫描当天需随访的患者，推送上报提醒给家长",` +
				`"tools":["growth_followup","growth_education"],"auto_execute":true,"notify":true}`,
		},
		{
			Name:    "预警扫描",
			Version: "1.0.0",
			Spec: `{"trigger":"proactive","schedule":"0 */6 * * *",` +
				`"description":"每6小时扫描所有活跃患者的最新数据，触发四级预警评估",` +
				`"tools":["growth_alert","growth_patient"],"auto_execute":true,"notify":true}`,
		},
		{
			Name:    "失访检测",
			Version: "1.0.0",
			Spec: `{"trigger":"proactive","schedule":"0 20 * * *",` +
				`"description":"每天20:00检测未按时上报的患者，分级提醒(1次/2次/3次→标记失访)",` +
				`"tools":["growth_followup","growth_patient"],"auto_execute":true,"notify":true}`,
		},
		{
			Name:    "每周宣教推送",
			Version: "1.0.0",
			Spec: `{"trigger":"proactive","schedule":"0 9 * * 1,3,5",` +
				`"description":"每周一三五09:00按患者异常类型推送个性化宣教(饮食/运动/睡眠/青春期)",` +
				`"tools":["growth_education","growth_patient"],"auto_execute":true,"notify":false}`,
		},
		{
			Name:    "月度报告生成",
			Version: "1.0.0",
			Spec: `{"trigger":"proactive","schedule":"0 9 1 * *",` +
				`"description":"每月1日生成全部患者的月度随访报告，汇总异常率、随访完成率、失访率",` +
				`"tools":["growth_report","growth_patient"],"auto_execute":true,"notify":true}`,
		},
	}
}

// ── 3个工作流 ──

func growthClinicWorkflows() []AgentWorkflowSpec {
	return []AgentWorkflowSpec{
		{
			Name:        "患者上报处理流程",
			Description: "收到家长上报数据后的全自动处理：质控→Z评分→预警→分级处理",
			Definition: `{"nodes":[` +
				`{"id":"start","type":"start","data":{"label":"收到上报数据"}},` +
				`{"id":"quality","type":"llm","data":{"label":"三级质控","model":"qwen-max","prompt":"对上报数据进行质控检查，使用 growth_quality 工具验证数据合理性、变化率、来源可信度","tools":["growth_quality"]}},` +
				`{"id":"cond_valid","type":"condition","data":{"label":"数据合格?","condition":"output.includes('pass') && !output.includes('invalid')"}},` +
				`{"id":"save","type":"llm","data":{"label":"保存数据+计算Z评分","model":"qwen-max","prompt":"使用 growth_patient 工具(operation=add_record)保存数据，系统自动计算Z评分和生长速率","tools":["growth_patient"]}},` +
				`{"id":"alert","type":"llm","data":{"label":"四级预警评估","model":"qwen-max","prompt":"使用 growth_alert 工具对该患者进行预警评估","tools":["growth_alert"]}},` +
				`{"id":"cond_level","type":"condition","data":{"label":"预警等级","condition":"output.includes('normal') ? 'normal' : output.includes('mild') ? 'mild' : output.includes('severe') ? 'severe' : 'moderate'"}},` +
				`{"id":"archive","type":"llm","data":{"label":"自动归档","model":"qwen-max","prompt":"数据正常，自动归档，推送日常保健建议","tools":["growth_education"]}},` +
				`{"id":"educate","type":"llm","data":{"label":"针对性宣教","model":"qwen-max","prompt":"轻度异常，推送针对性宣教和生活方式调整建议","tools":["growth_education"]}},` +
				`{"id":"doctor_queue","type":"llm","data":{"label":"推入医生待审","model":"qwen-max","prompt":"中度/危急异常，生成预警报告推入医生待审核列表，通知家长","tools":["growth_report"]}},` +
				`{"id":"reject","type":"llm","data":{"label":"引导重新上报","model":"qwen-max","prompt":"数据不合格，引导家长重新测量后再上报","tools":[]}},` +
				`{"id":"end","type":"end","data":{"label":"处理完成"}}` +
				`],"edges":[` +
				`{"source":"start","target":"quality"},` +
				`{"source":"quality","target":"cond_valid"},` +
				`{"source":"cond_valid","target":"save","label":"合格"},` +
				`{"source":"cond_valid","target":"reject","label":"不合格"},` +
				`{"source":"save","target":"alert"},` +
				`{"source":"alert","target":"cond_level"},` +
				`{"source":"cond_level","target":"archive","label":"normal"},` +
				`{"source":"cond_level","target":"educate","label":"mild"},` +
				`{"source":"cond_level","target":"doctor_queue","label":"moderate/severe"},` +
				`{"source":"archive","target":"end"},` +
				`{"source":"educate","target":"end"},` +
				`{"source":"doctor_queue","target":"end"},` +
				`{"source":"reject","target":"end"}` +
				`]}`,
		},
		{
			Name:        "医生审核闭环",
			Description: "医生审核异常数据→制定方案→推送患者→更新随访计划",
			Definition: `{"nodes":[` +
				`{"id":"start","type":"start","data":{"label":"医生开始审核"}},` +
				`{"id":"review","type":"llm","data":{"label":"查看患者全档案","model":"qwen-max","prompt":"使用 growth_report 生成患者完整报告，包含生长曲线、Z评分趋势、历史预警","tools":["growth_report","growth_curve"]}},` +
				`{"id":"decide","type":"llm","data":{"label":"医生决策","model":"qwen-max","prompt":"等待医生输入处理方案：调整饮食/运动/睡眠计划、修改随访频次、安排复诊、或要求补充数据","tools":[]}},` +
				`{"id":"push","type":"llm","data":{"label":"推送方案","model":"qwen-max","prompt":"将医生方案推送至家长端，同步推送针对性宣教内容","tools":["growth_education"]}},` +
				`{"id":"update_plan","type":"llm","data":{"label":"更新随访计划","model":"qwen-max","prompt":"根据医生决策更新随访计划频次和预警阈值","tools":["growth_followup"]}},` +
				`{"id":"end","type":"end","data":{"label":"审核完成"}}` +
				`],"edges":[` +
				`{"source":"start","target":"review"},` +
				`{"source":"review","target":"decide"},` +
				`{"source":"decide","target":"push"},` +
				`{"source":"push","target":"update_plan"},` +
				`{"source":"update_plan","target":"end"}` +
				`]}`,
		},
		{
			Name:        "复诊联动",
			Description: "患者复诊时自动关联既往档案，生成复诊摘要，医生更新后Agent自动适配新规则",
			Definition: `{"nodes":[` +
				`{"id":"start","type":"start","data":{"label":"复诊触发"}},` +
				`{"id":"summary","type":"llm","data":{"label":"生成复诊摘要","model":"qwen-max","prompt":"使用 growth_report 生成复诊摘要：指标变化趋势、预警历史、宣教记录、上次诊疗方案执行情况","tools":["growth_report","growth_curve","growth_velocity"]}},` +
				`{"id":"doctor_update","type":"llm","data":{"label":"医生更新病历","model":"qwen-max","prompt":"等待医生更新诊断、治疗方案、随访计划","tools":["growth_patient"]}},` +
				`{"id":"adapt","type":"llm","data":{"label":"Agent适配新规则","model":"qwen-max","prompt":"根据医生更新的方案，自动调整随访频次和预警阈值","tools":["growth_followup"]}},` +
				`{"id":"end","type":"end","data":{"label":"复诊完成"}}` +
				`],"edges":[` +
				`{"source":"start","target":"summary"},` +
				`{"source":"summary","target":"doctor_update"},` +
				`{"source":"doctor_update","target":"adapt"},` +
				`{"source":"adapt","target":"end"}` +
				`]}`,
		},
	}
}

// ── 13 个工具插件 ──

func growthClinicPlugins() []AgentPluginSpec {
	return []AgentPluginSpec{
		{Name: "growth_zscore", Spec: `{"name":"growth_zscore","description":"Calculate Z-score and percentile for height/weight/BMI based on WHO + China national growth standards.","version":"1.0.0","author":"StarClaw Growth Clinic","parameters":{"type":"object","properties":{"action":{"type":"string","const":"zscore"},"gender":{"type":"string"},"age_months":{"type":"number"},"height":{"type":"number"},"weight":{"type":"number"}},"required":["action","gender","age_months"]},"handler":"growth_clinic","metadata":{"category":"medical","timeout":10}}`},
		{Name: "growth_curve", Spec: `{"name":"growth_curve","description":"Generate growth curve data with WHO/China reference curves (P3-P97).","version":"1.0.0","author":"StarClaw Growth Clinic","parameters":{"type":"object","properties":{"action":{"type":"string","const":"growth_curve"},"patient_id":{"type":"string"}},"required":["action","patient_id"]},"handler":"growth_clinic","metadata":{"category":"medical","timeout":10}}`},
		{Name: "growth_velocity", Spec: `{"name":"growth_velocity","description":"Calculate growth velocity (cm/year) over 3/6/12 months.","version":"1.0.0","author":"StarClaw Growth Clinic","parameters":{"type":"object","properties":{"action":{"type":"string","const":"growth_velocity"},"patient_id":{"type":"string"}},"required":["action","patient_id"]},"handler":"growth_clinic","metadata":{"category":"medical","timeout":10}}`},
		{Name: "growth_bmi_assess", Spec: `{"name":"growth_bmi_assess","description":"Assess BMI category and ideal weight range.","version":"1.0.0","author":"StarClaw Growth Clinic","parameters":{"type":"object","properties":{"action":{"type":"string","const":"bmi_assess"},"gender":{"type":"string"},"age_months":{"type":"number"},"height":{"type":"number"},"weight":{"type":"number"}},"required":["action","height","weight"]},"handler":"growth_clinic","metadata":{"category":"medical","timeout":10}}`},
		{Name: "growth_target_height", Spec: `{"name":"growth_target_height","description":"Calculate genetic target height from parents heights.","version":"1.0.0","author":"StarClaw Growth Clinic","parameters":{"type":"object","properties":{"action":{"type":"string","const":"target_height"},"gender":{"type":"string"},"father_height":{"type":"number"},"mother_height":{"type":"number"}},"required":["action","gender","father_height","mother_height"]},"handler":"growth_clinic","metadata":{"category":"medical","timeout":10}}`},
		{Name: "growth_bone_age", Spec: `{"name":"growth_bone_age","description":"Compare bone age vs chronological age, predict adult height.","version":"1.0.0","author":"StarClaw Growth Clinic","parameters":{"type":"object","properties":{"action":{"type":"string","const":"bone_age_compare"},"gender":{"type":"string"},"bone_age":{"type":"number"},"chrono_age":{"type":"number"},"height":{"type":"number"}},"required":["action","bone_age","chrono_age"]},"handler":"growth_clinic","metadata":{"category":"medical","timeout":10}}`},
		{Name: "growth_puberty", Spec: `{"name":"growth_puberty","description":"Assess Tanner staging, detect precocious puberty and rapid progression.","version":"1.0.0","author":"StarClaw Growth Clinic","parameters":{"type":"object","properties":{"action":{"type":"string","const":"puberty_assess"},"gender":{"type":"string"},"age_years":{"type":"number"},"tanner_breast":{"type":"string"},"tanner_genital":{"type":"string"},"tanner_pubic_hair":{"type":"string"},"menarche":{"type":"boolean"}},"required":["action","gender","age_years"]},"handler":"growth_clinic","metadata":{"category":"medical","timeout":10}}`},
		{Name: "growth_alert", Spec: `{"name":"growth_alert","description":"Four-level alert evaluation: Normal/Mild/Moderate/Severe.","version":"1.0.0","author":"StarClaw Growth Clinic","parameters":{"type":"object","properties":{"action":{"type":"string","const":"alert_evaluate"},"patient_id":{"type":"string"}},"required":["action","patient_id"]},"handler":"growth_clinic","metadata":{"category":"medical","timeout":15}}`},
		{Name: "growth_patient", Spec: `{"name":"growth_patient","description":"Patient record CRUD and growth data recording with auto Z-score computation.","version":"1.0.0","author":"StarClaw Growth Clinic","parameters":{"type":"object","properties":{"action":{"type":"string","const":"patient_record"},"operation":{"type":"string"},"patient_id":{"type":"string"},"name":{"type":"string"},"gender":{"type":"string"},"birth_date":{"type":"string"},"height":{"type":"number"},"weight":{"type":"number"}},"required":["action","operation"]},"handler":"growth_clinic","metadata":{"category":"medical","timeout":10}}`},
		{Name: "growth_followup", Spec: `{"name":"growth_followup","description":"Follow-up plan management with smart frequency.","version":"1.0.0","author":"StarClaw Growth Clinic","parameters":{"type":"object","properties":{"action":{"type":"string","const":"followup_schedule"},"operation":{"type":"string"},"patient_id":{"type":"string"},"frequency_days":{"type":"number"}},"required":["action"]},"handler":"growth_clinic","metadata":{"category":"medical","timeout":10}}`},
		{Name: "growth_quality", Spec: `{"name":"growth_quality","description":"Three-level data quality check: range/change-rate/credibility.","version":"1.0.0","author":"StarClaw Growth Clinic","parameters":{"type":"object","properties":{"action":{"type":"string","const":"data_quality_check"},"height":{"type":"number"},"weight":{"type":"number"},"age_months":{"type":"number"},"prev_height":{"type":"number"},"prev_days_ago":{"type":"number"}},"required":["action"]},"handler":"growth_clinic","metadata":{"category":"medical","timeout":10}}`},
		{Name: "growth_education", Spec: `{"name":"growth_education","description":"Push health education content (diet/exercise/sleep/puberty).","version":"1.0.0","author":"StarClaw Growth Clinic","parameters":{"type":"object","properties":{"action":{"type":"string","const":"education_push"},"patient_id":{"type":"string"},"category":{"type":"string"}},"required":["action"]},"handler":"growth_clinic","metadata":{"category":"medical","timeout":10}}`},
		{Name: "growth_report", Spec: `{"name":"growth_report","description":"Generate patient summary report for doctor review.","version":"1.0.0","author":"StarClaw Growth Clinic","parameters":{"type":"object","properties":{"action":{"type":"string","const":"report_summary"},"patient_id":{"type":"string"}},"required":["action","patient_id"]},"handler":"growth_clinic","metadata":{"category":"medical","timeout":10}}`},
	}
}

const growthClinicSystemPrompt = `你是一位专业的儿童生长发育随访助理，服务于儿科生长发育门诊。

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
