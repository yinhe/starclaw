package handler

import (
	"encoding/json"
	"log"

	"github.com/google/uuid"
	"github.com/yinhe/starclaw-queen/api/internal/database"
	"github.com/yinhe/starclaw-queen/api/internal/model"
)

// AgentConfig is the JSON blob stored in MarketplaceItem.Config for type=agent
type AgentConfig struct {
	SystemPrompt string `json:"system_prompt"`
	Tools        string `json:"tools"`
	Config       string `json:"config"`
	ModelName    string `json:"model_name,omitempty"`
}

type officialAgent struct {
	Name        string
	Description string
	Icon        string
	Tags        string
	Category    string // agent sub-category for filtering
	Prompt      string
	Tools       string
}

// SeedOfficialAgents populates marketplace_items with StarClaw's official agents.
// Skips items that already exist (matched by name + type=agent + user_id=system).
func SeedOfficialAgents() {
	agents := []officialAgent{
		{
			Name:        "MV创作Agent",
			Description: "专业MV制作：创作歌词、生成歌曲、分镜视频、合成最终MV。支持多种音乐风格和视频模型。",
			Icon:        "Music",
			Tags:        "mv,music,video,创作",
			Category:    "creative",
			Prompt:      mvPrompt,
			Tools:       `["music_generation","video_generation","mv_production"]`,
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
	}

	const systemUserID = "system-official"

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
			}
			cfgJSON, _ := json.Marshal(cfg)
			database.DB.Model(&model.MarketplaceItem{}).
				Where("name = ? AND type = 'agent' AND user_id = ?", a.Name, systemUserID).
				Updates(map[string]interface{}{
					"description": a.Description,
					"icon":        a.Icon,
					"tags":        a.Tags,
					"config":      string(cfgJSON),
					"status":      model.ItemStatusApproved,
				})
			continue
		}

		cfg := AgentConfig{
			SystemPrompt: a.Prompt,
			Tools:        a.Tools,
			Config:       `{"temperature":0.3,"max_tokens":8192}`,
		}
		cfgJSON, _ := json.Marshal(cfg)

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
		}
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

const mvPrompt = `你是专业的MV（音乐视频）创作Agent。你的工作是从用户的需求出发，完成一部完整MV的制作。

## 你的工具
- **music_generation**: 生成歌曲（带演唱）或纯音乐
- **video_generation**: 生成视频场景
- **mv_production**: 将视频和音乐合成为最终MV

## MV制作流程（必须严格按顺序执行）
1. 创作歌词（使用结构标签）→ 展示给用户确认
2. 生成歌曲（music_generation）→ 记录 music_id
3. 等待歌曲完成（check_status 轮询）
4. 按歌曲时长规划分镜
5. 逐场景生成视频（video_generation），保持统一视觉风格
6. 等待自动合并
7. 合成最终MV（mv_production.compose_mv）`

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
