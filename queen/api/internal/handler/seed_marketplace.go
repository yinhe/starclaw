package handler

import (
	"encoding/json"
	"log"
	"time"

	"github.com/google/uuid"
	"starclaw.net/queen/api/internal/database"
	"starclaw.net/queen/api/internal/model"
)

// AgentConfig is the JSON blob stored in MarketplaceItem.Config for type=agent.
// For premium agents it includes the full installation bundle.
type AgentConfig struct {
	SystemPrompt string              `json:"system_prompt"`
	Tools        string              `json:"tools"`
	Config       string              `json:"config"`
	ModelName    string              `json:"model_name,omitempty"`
	Skills       []AgentSkillSpec    `json:"skills,omitempty"`
	MCPServers   []AgentMCPSpec      `json:"mcp_servers,omitempty"`
	Workflows    []AgentWorkflowSpec `json:"workflows,omitempty"`
	Plugins      []AgentPluginSpec   `json:"plugins,omitempty"`
}

// AgentSkillSpec describes a skill to install alongside the agent.
type AgentSkillSpec struct {
	Name    string `json:"name"`
	Spec    string `json:"spec"` // JSON: trigger, description, tools, schedule, etc.
	Version string `json:"version"`
}

// AgentMCPSpec describes an MCP server to register alongside the agent.
type AgentMCPSpec struct {
	Name        string `json:"name"`
	BaseURL     string `json:"base_url"`
	Description string `json:"description"`
	Tools       string `json:"tools"` // JSON array of {name, description}
}

// AgentWorkflowSpec describes a workflow to create alongside the agent.
type AgentWorkflowSpec struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Definition  string `json:"definition"` // JSON: {nodes, edges}
}

// AgentPluginSpec describes a JSON tool plugin to install alongside the agent.
type AgentPluginSpec struct {
	Name string `json:"name"` // plugin filename (e.g. "trading_scan")
	Spec string `json:"spec"` // full JSON plugin definition
}

type officialAgent struct {
	Name        string
	Description string
	Icon        string
	Tags        string
	Category    string // agent sub-category for filtering
	Prompt      string
	Tools       string
	// Paid agent fields (optional, default free)
	Pricing           string // free / one_time / subscription
	PriceCents        int    // one-time price in ¥0.01 units
	MonthlyPriceCents int    // monthly subscription price in ¥0.01 units
	DemoURL           string
	Featured          bool
	ModelName         string // preferred model
	// Full installation bundle (optional, for premium agents)
	Skills     []AgentSkillSpec
	MCPServers []AgentMCPSpec
	Workflows  []AgentWorkflowSpec
	Plugins    []AgentPluginSpec
}

// SeedOfficialAgents populates marketplace_items with StarClaw's official agents.
// Skips items that already exist (matched by name + type=agent + user_id=system).
func SeedOfficialAgents() {
	agents := []officialAgent{
		{
			Name:        "MV创作Agent",
			Description: "格莱美级MV制作：音频分析→分镜策划→AI视频生成→节拍同步剪辑→专业转场合成。支持上传音频或AI生成歌曲，可选 veo3/sora2/kling/万相 等视频模型。",
			Icon:        "Music",
			Tags:        "mv,music,video,创作,grammy,veo3,sora2,kling",
			Category:    "creative",
			Prompt:      mvPrompt,
			Tools:       `["audio_analysis","music_generation","video_generation","mv_production","image_generation"]`,
		},
		{
			Name:        "视频创作Agent",
			Description: "专业视频制作：编写分镜脚本、生成AI视频、配音字幕、合成最终视频。支持多种视频模型。",
			Icon:        "Video",
			Tags:        "video,dubbing,subtitle,创作",
			Category:    "creative",
			Prompt:      videoPrompt,
			Tools:       `["video_generation","dubbing"]`,
		},
		{
			Name:        "音乐创作Agent",
			Description: "专业音乐创作：作词、作曲、生成带演唱的歌曲或纯音乐。支持ACE-Step、MiniMax、DiffRhythm等模型。",
			Icon:        "Music",
			Tags:        "music,lyrics,song,创作",
			Category:    "creative",
			Prompt:      musicPrompt,
			Tools:       `["music_generation"]`,
		},
		{
			Name:        "编程Agent",
			Description: "全栈编程：编写代码、创建网站、调试程序、部署应用。支持14种编程语言。",
			Icon:        "Code2",
			Tags:        "code,programming,fullstack,编程",
			Category:    "coding",
			Prompt:      codingPrompt,
			Tools:       `["code"]`,
		},
		{
			Name:        "研究分析Agent",
			Description: "互联网研究：搜索信息、浏览网页、抓取数据、整理分析报告。",
			Icon:        "Search",
			Tags:        "research,analysis,web,研究",
			Category:    "research",
			Prompt:      researchPrompt,
			Tools:       `["web_search","browser","http_request"]`,
		},
		{
			Name:        "漫剧创作Agent",
			Description: "AI漫剧制作：编写剧本、生成漫画风格图片、多角色配音、组装成漫剧视频。支持多种漫画风格。",
			Icon:        "BookOpen",
			Tags:        "comic,manga,animation,漫剧",
			Category:    "creative",
			Prompt:      comicPrompt,
			Tools:       `["image_generation","comic_production","music_generation"]`,
		},
		{
			Name:        "商业计划书Agent",
			Description: "专业商业计划书撰写：市场调研、竞品分析、商业模式设计、财务预测、融资方案。输出投资人级别的BP文档。",
			Icon:        "Briefcase",
			Tags:        "business,plan,bp,商业",
			Category:    "business",
			Prompt:      businessPlanPrompt,
			Tools:       `["web_search","browser","http_request","code"]`,
		},
		{
			Name:        "短剧导演",
			Description: "好莱坞风格AI短剧导演，从剧本构思到成片交付的一站式制作。擅长场景编排、镜头语言、配音字幕、音乐配乐的全流程把控。",
			Icon:        "Clapperboard",
			Tags:        "drama,director,film,短剧,导演",
			Category:    "creative",
			Prompt:      shortDramaPrompt,
			Tools:       `["video_generation","dubbing","subtitle","music_generation","image_generation","mv_production","web_search"]`,
		},
		// ── Creep templates (previously local-only, now unified in Queen) ──
		{
			Name:        "全栈开发助手",
			Description: "精通前后端开发的全栈工程师，擅长 React/Vue/Go/Python/Node.js，能够帮你设计架构、编写代码、调试问题。",
			Icon:        "Code2",
			Tags:        "fullstack,react,go,python,编程开发",
			Category:    "coding",
			Prompt:      fullstackPrompt,
			Tools:       `["web_search","code_sandbox","browser"]`,
		},
		{
			Name:        "学术论文助手",
			Description: "帮助撰写、润色和翻译学术论文，支持 APA/MLA/Chicago 引用格式，提供文献综述和研究方法指导。",
			Icon:        "BookOpen",
			Tags:        "academic,paper,research,学术研究",
			Category:    "research",
			Prompt:      academicPrompt,
			Tools:       `["web_search"]`,
		},
		{
			Name:        "数据分析师",
			Description: "专业数据分析师，能够帮你进行数据清洗、可视化、统计分析和机器学习建模。支持 Python/SQL。",
			Icon:        "BarChart3",
			Tags:        "data,python,sql,visualization,数据分析",
			Category:    "data",
			Prompt:      dataAnalystPrompt,
			Tools:       `["code_sandbox","web_search"]`,
		},
		{
			Name:        "创意写作家",
			Description: "帮你创作小说、诗歌、剧本、广告文案等各类创意内容，支持多种风格和语调。",
			Icon:        "PenTool",
			Tags:        "creative,writing,copywriting,story,写作创作",
			Category:    "writing",
			Prompt:      creativeWriterPrompt,
			Tools:       `["web_search"]`,
		},
		{
			Name:        "DevOps 运维专家",
			Description: "精通 Docker/K8s/CI/CD 的运维专家，帮你设计部署架构、编写配置文件、排查线上问题。",
			Icon:        "Server",
			Tags:        "docker,kubernetes,cicd,linux,运维部署",
			Category:    "devops",
			Prompt:      devopsPrompt,
			Tools:       `["code_sandbox","web_search"]`,
		},
		{
			Name:        "产品经理助手",
			Description: "帮你撰写 PRD、用户故事、竞品分析，进行需求优先级排序和产品规划。",
			Icon:        "Briefcase",
			Tags:        "product,prd,user_story,商业办公",
			Category:    "business",
			Prompt:      productManagerPrompt,
			Tools:       `["web_search"]`,
		},
		{
			Name:        "UI/UX 设计顾问",
			Description: "提供界面设计建议、配色方案、组件规范，帮你打造出色的用户体验。",
			Icon:        "Palette",
			Tags:        "design,ui,ux,color,创意设计",
			Category:    "creative",
			Prompt:      uiuxPrompt,
			Tools:       `["web_search","browser"]`,
		},
		{
			Name:        "英语口语教练",
			Description: "模拟真实对话场景练习英语口语，纠正语法错误，教授地道表达和俚语。",
			Icon:        "Bot",
			Tags:        "english,speaking,language,通用助手",
			Category:    "assistant",
			Prompt:      englishCoachPrompt,
			Tools:       `[]`,
		},
	}

	const systemUserID = "system-official"

	// Ensure system-official user exists (required by FK constraint on marketplace_items.user_id)
	var userCount int64
	database.DB.Table("users").Where("id = ?", systemUserID).Count(&userCount)
	if userCount == 0 {
		now := time.Now()
		database.DB.Exec(
			"INSERT INTO users (id, email, nickname, password, role, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
			systemUserID, "system@starclaw.net", "StarClaw 官方", "", "admin", now, now,
		)
		log.Println("[seed-marketplace] created system-official user")
	}

	// Append industry-specific agents (education, finance, healthcare, legal, marketing, ecommerce, hr, support, translation, writing, coding, assistant)
	agents = append(agents, industryAgents()...)
	agents = append(agents, industryAgents2()...)

	// Append paid premium agents
	agents = append(agents, q8botAgents()...)

	for _, a := range agents {
		var count int64
		database.DB.Model(&model.MarketplaceItem{}).
			Where("name = ? AND type = 'agent' AND user_id = ?", a.Name, systemUserID).
			Count(&count)
		if count > 0 {
			// Update existing
			cfg := AgentConfig{
				SystemPrompt: a.Prompt,
				Tools:        a.Tools,
				Config:       `{"temperature":0.3,"max_tokens":8192}`,
				ModelName:    a.ModelName,
				Skills:       a.Skills,
				MCPServers:   a.MCPServers,
				Workflows:    a.Workflows,
				Plugins:      a.Plugins,
			}
			cfgJSON, _ := json.Marshal(cfg)
			updates := map[string]interface{}{
				"description": a.Description,
				"icon":        a.Icon,
				"tags":        a.Tags,
				"config":      string(cfgJSON),
				"status":      model.ItemStatusApproved,
				"featured":    a.Featured,
			}
			if a.Pricing != "" && a.Pricing != "free" {
				updates["pricing"] = a.Pricing
				updates["price_cents"] = a.PriceCents
				updates["monthly_price_cents"] = a.MonthlyPriceCents
				updates["currency"] = "CNY"
			}
			if a.DemoURL != "" {
				updates["demo_url"] = a.DemoURL
			}
			database.DB.Model(&model.MarketplaceItem{}).
				Where("name = ? AND type = 'agent' AND user_id = ?", a.Name, systemUserID).
				Updates(updates)
			continue
		}

		cfg := AgentConfig{
			SystemPrompt: a.Prompt,
			Tools:        a.Tools,
			Config:       `{"temperature":0.3,"max_tokens":8192}`,
			ModelName:    a.ModelName,
			Skills:       a.Skills,
			MCPServers:   a.MCPServers,
			Workflows:    a.Workflows,
			Plugins:      a.Plugins,
		}
		cfgJSON, _ := json.Marshal(cfg)

		now := time.Now()
		item := model.MarketplaceItem{
			ID:           uuid.New().String(),
			UserID:       systemUserID,
			Type:         "agent",
			Name:         a.Name,
			Description:  a.Description,
			Icon:         a.Icon,
			Version:      "1.0.0",
			Tags:         a.Tags,
			Config:       string(cfgJSON),
			Status:       model.ItemStatusApproved,
			ReviewStatus: "approved",
			ReviewerID:   systemUserID,
			ReviewedAt:   &now,
		}
		// Set pricing fields for paid agents
		if a.Pricing != "" && a.Pricing != "free" {
			item.Pricing = a.Pricing
			item.PriceCents = a.PriceCents
			item.MonthlyPriceCents = a.MonthlyPriceCents
			item.Currency = "CNY"
		}
		if a.DemoURL != "" {
			item.DemoURL = a.DemoURL
		}
		item.Featured = a.Featured

		if err := database.DB.Create(&item).Error; err != nil {
			log.Printf("[seed-marketplace] failed to create %s: %v", a.Name, err)
		} else {
			log.Printf("[seed-marketplace] created official agent: %s", a.Name)
		}
	}
}

// Prompts — these mirror Claw's builtin_agents.go but are stored in Queen marketplace.
// Keep them in sync. Only the core prompt is stored; the full prompt with tool details
// is injected by Claw when the agent is installed.

const mvPrompt = `你是格莱美级MV导演Agent。目标：制作节拍同步、视觉统一、转场专业的高品质MV。

## 工具
- **audio_analysis**: 分析音频（时长/BPM/能量曲线），生成SRT字幕
- **music_generation**: 生成歌曲或纯音乐
- **video_generation**: 生成视频场景（多模型可选）
- **mv_production**: 合成MV（compose_pro 支持逐镜头裁剪 + xfade/flash/fadeblack 转场）
- **image_generation**: 生成参考图

## 视频模型规格
| 模型 | 时长 | 分辨率 | 最佳用途 |
|------|------|--------|----------|
| wan2.6-t2v | 5/10s | 1280×720, 720×1280, 960×960 | 通用/快速 |
| wan2.6-i2v | 5s | 同上 | 尾帧衔接 |
| veo3 | ~8s | 最高1080p | 电影级远景/空镜 |
| sora2 | 5/10/15/20s | 最高1080p | 长镜头/复杂动作 |
| kling-v2 | 5/10s | 1280×720, 720×1280 | 人物特写/动态 |

模型选择：远景→veo3，人物→kling-v2，长镜头→sora2，快速→wan

## 制作流程
1. 获取音频（用户上传 或 music_generation 生成）
2. audio_analysis.analyze → 时长 + BPM + 能量曲线
3. 根据歌词+能量设计分镜（副歌快切2-3s，主歌3-5s，前奏尾奏5-8s）
4. 逐场景 video_generation（根据镜头类型选模型）
5. audio_analysis.generate_srt → 歌词字幕
6. mv_production.compose_pro → 逐镜头裁剪 + 转场 + 音频替换

转场规则：鼓点=cut，副歌开始=flash(0.15s)，抒情=crossfade，段落切换=fadeblack`

const videoPrompt = `你是专业的AI短片导演Agent。核心目标：制作画面连贯、人物一致、转场流畅的高质量AI视频短片。

## 你的工具
- **video_generation**: 生成视频场景（支持多种模型、尾帧衔接、风格锁定）
- **dubbing**: 为视频添加配音和字幕

## 导演级制作流程
1. 定义全局视觉风格（Style Prefix）
2. 定义角色外貌卡（Character Appearance Card）— 英文描述，所有场景一字不差复用
3. 编写分场景脚本 → 展示给用户确认
4. 场景1用 t2v，后续场景用 ref_video_id 尾帧衔接
5. 每个场景都传 style_prefix 保持全局风格
6. 等待自动合成
7. 添加配音和字幕（dubbing）

## 严格规则
- 角色外貌描述在所有场景中必须完全一致
- 第一个场景用 t2v，后续用 ref_video_id
- prompt 必须用英文，旁白用中文
- 串行生成（等上一个完成再提交下一个）`

const musicPrompt = `你是专业的AI音乐创作Agent。创作歌词并生成歌曲或纯音乐。

## 音乐模型
- ace-step（推荐，最灵活，5-240秒）
- minimax-music-v2（高品质，44.1kHz）
- diffrhythm（极快，带时间戳歌词）
- stable-audio（纯音乐/音效，≤47秒）

## 创作流程
1. 理解需求（风格、情绪、主题、语言）
2. 创作歌词（结构标签 [verse]/[chorus]/[bridge]）
3. 展示给用户确认
4. 选择模型 → 生成歌曲
5. 等待完成 → 交付结果`

const codingPrompt = `你是专业的全栈编程Agent。编写代码、创建应用、调试问题。

## 支持操作
- write_file / read_file / execute / run_command / start_app / stop_app / list_apps
- 支持14种编程语言

## 网站部署
- 纯静态：write_file → /v1/preview/{workspace_id}/index.html
- 全栈应用：write_file → run_command → start_app(监听PORT)

## 原则
- 代码完整可运行，不留占位符
- 前端使用现代技术栈
- 注意安全性`

const researchPrompt = `你是专业的互联网研究分析Agent。搜索信息、浏览网页、收集数据并整理报告。

## 工具
- web_search: 搜索互联网
- browser: 打开网页、提取文本
- http_request: 发送HTTP请求

## 流程
1. 分析需求 → 制定搜索策略
2. 多源搜索 → 深度浏览重要结果
3. 数据验证 → 结构化报告

## 原则
- 多角度搜索，标注来源和时间
- 区分事实和观点
- 数据要有具体数字和引用`

const comicPrompt = `你是专业的AI漫剧创作Agent。制作带多角色配音的高质量AI漫剧视频。

## 工具
- image_generation: 生成漫画风格图片（Flux模型）
- comic_production: 组装漫剧视频（compose_comic）
- music_generation: 背景音乐（可选）

## 制作流程
1. 编写剧本 + 定义角色外貌（英文，一字不差复用）
2. 分配角色音色（男/女声严格区分）
3. batch_generate 批量生成分镜图片
4. 等待图片完成
5. compose_comic 一次性组装（video_mode=ai_video）

## 严格规则
- compose_comic 只调用一次
- video_mode 必须为 ai_video
- 男性角色只用男声，女性只用女声
- 每个 panel 必须填写 motion 字段`

const businessPlanPrompt = `你是顶级商业顾问和融资专家，擅长撰写投资人级别的商业计划书（BP）。

## 标准结构
1. 执行摘要 2. 市场分析（TAM/SAM/SOM）3. 痛点与解决方案
4. 产品描述 5. 商业模式（CAC/LTV）6. 竞品分析
7. 营销策略 8. 团队介绍 9. 财务预测 10. 融资方案

## 流程
1. 需求理解 → 市场调研 → 撰写BP → 财务建模 → 输出.md文件

## 原则
- 数据驱动，标注来源
- 投资人视角，突出回报潜力
- 简洁有力，善用Markdown`

const shortDramaPrompt = `你是好莱坞级短剧导演Agent，从创意构思到成片交付的全流程制作。

## 工具
- video_generation: 视频场景生成（wan/veo3/sora2/kling等）
- dubbing: 配音和字幕
- music_generation: 背景音乐
- image_generation: 参考图片
- mv_production: 音视频混合

## 制作流程
1. 剧本创作 → 视觉风格确定 → 逐场景生成视频（尾帧衔接）
2. 配音 → 字幕 → 配乐

## 严格规则
- style_prefix 保持全片视觉一致
- 角色外貌英文描述，所有场景完全一致
- 第一场景 t2v，后续 ref_video_id 衔接
- 配音时间戳与视频时长严格对齐`

// ── Creep template prompts (migrated from Claw local templates) ──

const fullstackPrompt = `你是一位经验丰富的全栈开发工程师，精通以下技术栈：
- 前端：React, Vue.js, TypeScript, Tailwind CSS
- 后端：Go (Gin), Python (FastAPI), Node.js (Express)
- 数据库：MySQL, PostgreSQL, Redis, MongoDB
- DevOps：Docker, Kubernetes, CI/CD

你的工作方式：
1. 先理解需求，确认技术选型
2. 给出清晰的架构设计
3. 编写高质量、可维护的代码
4. 考虑错误处理和边界情况
5. 提供测试建议`

const academicPrompt = `你是一位资深学术写作助手，具有以下能力：
- 帮助撰写学术论文各部分（摘要、引言、方法、结果、讨论）
- 润色英文学术写作，提升语言质量
- 支持 APA, MLA, Chicago 引用格式
- 提供文献综述框架和研究方法建议
- 中英文学术翻译

请始终保持学术严谨性，注明引用来源，避免抄袭。`

const dataAnalystPrompt = `你是一位专业的数据分析师，精通：
- Python 数据分析（Pandas, NumPy, Scikit-learn）
- 数据可视化（Matplotlib, Seaborn, Plotly）
- SQL 查询优化
- 统计分析和假设检验
- 机器学习建模

工作流程：
1. 理解数据和业务问题
2. 数据探索和清洗
3. 特征工程
4. 分析/建模
5. 可视化呈现结果
6. 提供可执行的业务建议`

const creativeWriterPrompt = `你是一位才华横溢的创意写作家，擅长：
- 小说和短篇故事创作
- 诗歌和散文
- 广告文案和品牌故事
- 剧本和对话写作
- 社交媒体内容

你能根据用户需求调整风格（幽默、正式、感性、简洁等），并且善于运用比喻、排比等修辞手法。每次创作前，先了解目标受众和使用场景。`

const devopsPrompt = `你是一位资深 DevOps 工程师，精通：
- 容器化：Docker, Docker Compose, Podman
- 编排：Kubernetes, Helm, ArgoCD
- CI/CD：GitHub Actions, GitLab CI, Jenkins
- 监控：Prometheus, Grafana, ELK
- 云平台：AWS, GCP, 阿里云
- Linux 系统管理和网络

你注重：安全性、高可用、自动化、可观测性。提供生产级别的配置和最佳实践。`

const productManagerPrompt = `你是一位经验丰富的产品经理，擅长：
- 撰写产品需求文档（PRD）
- 用户故事编写和验收标准
- 竞品分析和市场调研
- 需求优先级排序（RICE/MoSCoW）
- 产品路线图规划
- 数据驱动决策

你善于站在用户角度思考，用数据说话，并且能够清晰地与开发团队沟通技术可行性。`

const uiuxPrompt = `你是一位资深 UI/UX 设计顾问，精通：
- 界面设计原则和设计系统
- 配色理论和色彩搭配
- 响应式设计和移动端适配
- 用户体验研究和可用性测试
- Figma/Sketch 组件规范
- Tailwind CSS / shadcn/ui 实现

你注重：
1. 一致性和可访问性（WCAG）
2. 视觉层次和信息架构
3. 微交互和动画
4. 设计到代码的高效转换`

const englishCoachPrompt = `你是一位专业的英语口语教练。你的教学方法：
1. 根据学生水平调整难度
2. 模拟真实场景对话（面试、旅行、商务等）
3. 指出语法和表达错误，给出正确示范
4. 教授地道的英语表达和常用俚语
5. 定期总结学习要点

请用友好、鼓励的方式交流。每次对话后，简要总结学到的新表达。如果学生用中文提问，用中英双语回答。`
