package v1

import (
	"encoding/json"
	"fmt"
)

// Workflow definitions for each built-in specialist agent.
// Each workflow is a JSON string with {nodes, edges} for the visual workflow editor.

const mvWorkflow = `{
  "nodes": [
    {"id":"start","type":"start","position":{"x":400,"y":30},"data":{"label":"开始"}},
    {"id":"branch","type":"condition","position":{"x":400,"y":120},"data":{"label":"音频来源判断","description":"用户上传了音频？→ 跳到分析；没有 → 创作歌曲"}},
    {"id":"gen-1","type":"llm","position":{"x":150,"y":230},"data":{"label":"创作歌词","description":"根据用户需求编写 [verse]/[chorus]/[bridge] 结构歌词"}},
    {"id":"gen-2","type":"tool","position":{"x":150,"y":340},"data":{"label":"生成歌曲","toolName":"music_generation","description":"调用 generate_music，传入歌词和风格标签"}},
    {"id":"gen-3","type":"tool","position":{"x":150,"y":450},"data":{"label":"等待歌曲完成","toolName":"music_generation","description":"轮询 check_status 至 succeeded"}},
    {"id":"analyze","type":"tool","position":{"x":400,"y":450},"data":{"label":"音频分析","toolName":"audio_analysis","description":"analyze: 获取时长/BPM/能量曲线；detect_beats: 获取节拍时间戳"}},
    {"id":"plan","type":"llm","position":{"x":400,"y":560},"data":{"label":"导演策划分镜","description":"根据能量曲线+歌词段落，设计每个镜头的时长/画面/转场/模型选择"}},
    {"id":"generate","type":"tool","position":{"x":400,"y":670},"data":{"label":"批量生成视频场景","toolName":"video_generation","description":"逐个调用 generate_video（veo3/kling/wan/sora2），标注 scene 字段"}},
    {"id":"srt","type":"tool","position":{"x":400,"y":780},"data":{"label":"生成歌词字幕","toolName":"audio_analysis","description":"调用 generate_srt，歌词+时长 → SRT 字幕文件"}},
    {"id":"compose","type":"tool","position":{"x":400,"y":890},"data":{"label":"专业合成MV","toolName":"mv_production","description":"compose_pro: 逐镜头裁剪+节拍同步+转场(cut/crossfade/flash/fadeblack)+字幕烧录"}},
    {"id":"end","type":"end","position":{"x":400,"y":1000},"data":{"label":"完成"}}
  ],
  "edges": [
    {"id":"e-sb","source":"start","target":"branch"},
    {"id":"e-b-gen","source":"branch","target":"gen-1","data":{"label":"需要创作歌曲"}},
    {"id":"e-b-ana","source":"branch","target":"analyze","data":{"label":"已有音频+歌词"}},
    {"id":"e-g12","source":"gen-1","target":"gen-2"},
    {"id":"e-g23","source":"gen-2","target":"gen-3"},
    {"id":"e-g3a","source":"gen-3","target":"analyze"},
    {"id":"e-ap","source":"analyze","target":"plan"},
    {"id":"e-pg","source":"plan","target":"generate"},
    {"id":"e-gs","source":"generate","target":"srt"},
    {"id":"e-sc","source":"srt","target":"compose"},
    {"id":"e-ce","source":"compose","target":"end"}
  ]
}`

const videoWorkflow = `{
  "nodes": [
    {"id":"start","type":"start","position":{"x":300,"y":30},"data":{"label":"开始"}},
    {"id":"step-1","type":"llm","position":{"x":300,"y":120},"data":{"label":"编写分镜脚本","description":"编写场景描述、时长、旁白文字，选择视频模型"}},
    {"id":"step-2","type":"tool","position":{"x":300,"y":230},"data":{"label":"逐场景生成视频","toolName":"video_generation","description":"逐个调用 generate_video（wan/veo3/sora2/kling等）"}},
    {"id":"step-3","type":"tool","position":{"x":300,"y":340},"data":{"label":"等待自动合成","toolName":"video_generation","description":"所有场景完成后系统自动合并"}},
    {"id":"step-4","type":"tool","position":{"x":300,"y":450},"data":{"label":"添加配音字幕","toolName":"dubbing","description":"调用 add_voiceover，选择音色，传入分段旁白"}},
    {"id":"end","type":"end","position":{"x":300,"y":550},"data":{"label":"完成"}}
  ],
  "edges": [
    {"id":"e-s1","source":"start","target":"step-1"},
    {"id":"e-12","source":"step-1","target":"step-2"},
    {"id":"e-23","source":"step-2","target":"step-3"},
    {"id":"e-34","source":"step-3","target":"step-4"},
    {"id":"e-4e","source":"step-4","target":"end"}
  ]
}`

const musicWorkflow = `{
  "nodes": [
    {"id":"start","type":"start","position":{"x":300,"y":30},"data":{"label":"开始"}},
    {"id":"step-1","type":"llm","position":{"x":300,"y":120},"data":{"label":"理解需求","description":"了解风格、情绪、主题、语言偏好"}},
    {"id":"step-2","type":"llm","position":{"x":300,"y":230},"data":{"label":"创作歌词","description":"使用 [verse]/[chorus]/[bridge] 结构，注意押韵和情感"}},
    {"id":"step-3","type":"tool","position":{"x":300,"y":340},"data":{"label":"生成歌曲","toolName":"music_generation","description":"选择模型(ace-step/minimax/diffrhythm/stable-audio)，设定时长"}},
    {"id":"step-4","type":"tool","position":{"x":300,"y":450},"data":{"label":"检查状态","toolName":"music_generation","description":"轮询 check_status 至完成"}},
    {"id":"end","type":"end","position":{"x":300,"y":550},"data":{"label":"交付结果"}}
  ],
  "edges": [
    {"id":"e-s1","source":"start","target":"step-1"},
    {"id":"e-12","source":"step-1","target":"step-2"},
    {"id":"e-23","source":"step-2","target":"step-3"},
    {"id":"e-34","source":"step-3","target":"step-4"},
    {"id":"e-4e","source":"step-4","target":"end"}
  ]
}`

const codingWorkflow = `{
  "nodes": [
    {"id":"start","type":"start","position":{"x":300,"y":30},"data":{"label":"开始"}},
    {"id":"step-1","type":"llm","position":{"x":300,"y":120},"data":{"label":"需求分析","description":"理解功能需求，选择技术栈"}},
    {"id":"step-2","type":"tool","position":{"x":300,"y":230},"data":{"label":"编写代码","toolName":"code","description":"write_file 创建项目文件和代码"}},
    {"id":"step-3","type":"tool","position":{"x":300,"y":340},"data":{"label":"安装依赖","toolName":"code","description":"run_command 安装包和配置环境"}},
    {"id":"step-4","type":"tool","position":{"x":300,"y":450},"data":{"label":"运行测试","toolName":"code","description":"execute 运行测试，调试修复错误"}},
    {"id":"step-5","type":"tool","position":{"x":300,"y":560},"data":{"label":"部署应用","toolName":"code","description":"start_app 启动应用（监听 PORT 环境变量）"}},
    {"id":"end","type":"end","position":{"x":300,"y":660},"data":{"label":"交付"}}
  ],
  "edges": [
    {"id":"e-s1","source":"start","target":"step-1"},
    {"id":"e-12","source":"step-1","target":"step-2"},
    {"id":"e-23","source":"step-2","target":"step-3"},
    {"id":"e-34","source":"step-3","target":"step-4"},
    {"id":"e-45","source":"step-4","target":"step-5"},
    {"id":"e-5e","source":"step-5","target":"end"}
  ]
}`

const researchWorkflow = `{
  "nodes": [
    {"id":"start","type":"start","position":{"x":300,"y":30},"data":{"label":"开始"}},
    {"id":"step-1","type":"llm","position":{"x":300,"y":120},"data":{"label":"明确研究问题","description":"拆解问题，制定搜索策略"}},
    {"id":"step-2","type":"tool","position":{"x":300,"y":230},"data":{"label":"多源搜索","toolName":"web_search","description":"搜索引擎检索关键信息"}},
    {"id":"step-3","type":"tool","position":{"x":300,"y":340},"data":{"label":"深度浏览","toolName":"browser","description":"浏览重点网页，提取详细数据"}},
    {"id":"step-4","type":"tool","position":{"x":300,"y":450},"data":{"label":"API数据采集","toolName":"http_request","description":"调用公开API获取结构化数据"}},
    {"id":"step-5","type":"llm","position":{"x":300,"y":560},"data":{"label":"分析整理","description":"交叉验证、数据分析、生成结论"}},
    {"id":"step-6","type":"llm","position":{"x":300,"y":660},"data":{"label":"输出报告","description":"撰写结构化研究报告"}},
    {"id":"end","type":"end","position":{"x":300,"y":760},"data":{"label":"完成"}}
  ],
  "edges": [
    {"id":"e-s1","source":"start","target":"step-1"},
    {"id":"e-12","source":"step-1","target":"step-2"},
    {"id":"e-23","source":"step-2","target":"step-3"},
    {"id":"e-34","source":"step-3","target":"step-4"},
    {"id":"e-45","source":"step-4","target":"step-5"},
    {"id":"e-56","source":"step-5","target":"step-6"},
    {"id":"e-6e","source":"step-6","target":"end"}
  ]
}`

const comicWorkflow = `{
  "nodes": [
    {"id":"start","type":"start","position":{"x":300,"y":30},"data":{"label":"开始"}},
    {"id":"step-1","type":"llm","position":{"x":300,"y":110},"data":{"label":"编写剧本","description":"编写分镜剧本 + 定义角色外貌描述 + 分配音色"}},
    {"id":"step-2","type":"tool","position":{"x":300,"y":210},"data":{"label":"批量生成分镜图","toolName":"image_generation","description":"batch_generate 一次提交所有分镜，角色外貌一致"}},
    {"id":"step-3","type":"tool","position":{"x":300,"y":310},"data":{"label":"等待图片完成","toolName":"image_generation","description":"list_images 确认所有图片 succeeded"}},
    {"id":"step-4","type":"tool","position":{"x":300,"y":410},"data":{"label":"可选：生成BGM","toolName":"music_generation","description":"（可选）生成背景音乐"}},
    {"id":"step-5","type":"tool","position":{"x":300,"y":520},"data":{"label":"组装漫剧视频","toolName":"comic_production","description":"compose_comic: 图片+配音+动效→漫剧视频"}},
    {"id":"end","type":"end","position":{"x":300,"y":620},"data":{"label":"完成"}}
  ],
  "edges": [
    {"id":"e-s1","source":"start","target":"step-1"},
    {"id":"e-12","source":"step-1","target":"step-2"},
    {"id":"e-23","source":"step-2","target":"step-3"},
    {"id":"e-34","source":"step-3","target":"step-4"},
    {"id":"e-45","source":"step-4","target":"step-5"},
    {"id":"e-5e","source":"step-5","target":"end"}
  ]
}`

// generateWorkflowFromTools creates a workflow definition JSON from an agent's tool list.
// This is used for user-created agents that don't have a built-in workflow.
func generateWorkflowFromTools(agentName string, toolsJSON string) string {
	var tools []string
	if err := json.Unmarshal([]byte(toolsJSON), &tools); err != nil || len(tools) == 0 {
		return ""
	}

	// Tool display names
	toolNames := map[string]string{
		"video_generation": "视频生成", "dubbing": "配音字幕", "mv_production": "MV合成",
		"comic_production": "漫剧制作", "music_generation": "音乐生成", "image_generation": "图片生成",
		"code": "代码执行", "web_search": "网页搜索", "browser": "浏览器",
		"http_request": "HTTP请求", "system": "系统管理",
	}
	// Tool descriptions for workflow steps
	toolDescs := map[string]string{
		"video_generation": "使用 AI 生成视频片段",
		"dubbing":          "为视频添加配音和字幕",
		"mv_production":    "将视频和音乐合成 MV",
		"comic_production": "将图片组装成漫剧视频",
		"music_generation": "生成歌曲或背景音乐",
		"image_generation": "生成 AI 图片素材",
		"code":             "编写和执行代码",
		"web_search":       "搜索互联网获取信息",
		"browser":          "浏览网页提取内容",
		"http_request":     "调用外部 API 接口",
		"system":           "管理系统和任务",
	}

	type nodeData struct {
		Label       string `json:"label"`
		Description string `json:"description,omitempty"`
		ToolName    string `json:"toolName,omitempty"`
	}
	type position struct {
		X int `json:"x"`
		Y int `json:"y"`
	}
	type node struct {
		ID       string   `json:"id"`
		Type     string   `json:"type"`
		Position position `json:"position"`
		Data     nodeData `json:"data"`
	}
	type edge struct {
		ID     string `json:"id"`
		Source string `json:"source"`
		Target string `json:"target"`
	}

	var nodes []node
	var edges []edge
	y := 30

	// Start node
	nodes = append(nodes, node{ID: "start", Type: "start", Position: position{X: 300, Y: y}, Data: nodeData{Label: "开始"}})
	y += 100

	// Analysis step
	nodes = append(nodes, node{ID: "step-0", Type: "llm", Position: position{X: 300, Y: y}, Data: nodeData{Label: "理解需求", Description: "分析用户需求，规划执行步骤"}})
	edges = append(edges, edge{ID: "e-s0", Source: "start", Target: "step-0"})
	prevID := "step-0"
	y += 110

	// One step per tool
	for i, t := range tools {
		stepID := fmt.Sprintf("step-%d", i+1)
		label := toolNames[t]
		if label == "" {
			label = t
		}
		desc := toolDescs[t]
		if desc == "" {
			desc = "使用 " + t + " 工具"
		}
		nodes = append(nodes, node{ID: stepID, Type: "tool", Position: position{X: 300, Y: y}, Data: nodeData{Label: label, ToolName: t, Description: desc}})
		edges = append(edges, edge{ID: fmt.Sprintf("e-%s-%s", prevID, stepID), Source: prevID, Target: stepID})
		prevID = stepID
		y += 110
	}

	// Summary step
	summaryID := fmt.Sprintf("step-%d", len(tools)+1)
	nodes = append(nodes, node{ID: summaryID, Type: "llm", Position: position{X: 300, Y: y}, Data: nodeData{Label: "总结输出", Description: "整理结果，交付给用户"}})
	edges = append(edges, edge{ID: fmt.Sprintf("e-%s-%s", prevID, summaryID), Source: prevID, Target: summaryID})
	y += 100

	// End node
	nodes = append(nodes, node{ID: "end", Type: "end", Position: position{X: 300, Y: y}, Data: nodeData{Label: "完成"}})
	edges = append(edges, edge{ID: fmt.Sprintf("e-%s-end", summaryID), Source: summaryID, Target: "end"})

	result := struct {
		Nodes []node `json:"nodes"`
		Edges []edge `json:"edges"`
	}{Nodes: nodes, Edges: edges}

	b, _ := json.Marshal(result)
	return string(b)
}

const businessPlanWorkflow = `{
  "nodes": [
    {"id":"start","type":"start","position":{"x":300,"y":30},"data":{"label":"开始"}},
    {"id":"step-1","type":"llm","position":{"x":300,"y":110},"data":{"label":"需求梳理","description":"明确行业、产品定位、目标市场、融资需求"}},
    {"id":"step-2","type":"tool","position":{"x":300,"y":210},"data":{"label":"市场调研","toolName":"web_search","description":"搜索行业报告、市场规模、增长趋势"}},
    {"id":"step-3","type":"tool","position":{"x":300,"y":310},"data":{"label":"竞品分析","toolName":"browser","description":"浏览竞品网站，分析产品差异化"}},
    {"id":"step-4","type":"llm","position":{"x":300,"y":410},"data":{"label":"商业模式设计","description":"价值主张、收入模式、成本结构"}},
    {"id":"step-5","type":"tool","position":{"x":300,"y":510},"data":{"label":"财务建模","toolName":"code","description":"Python生成财务预测和图表"}},
    {"id":"step-6","type":"llm","position":{"x":300,"y":610},"data":{"label":"撰写BP文档","description":"结构化输出完整商业计划书"}},
    {"id":"end","type":"end","position":{"x":300,"y":710},"data":{"label":"交付BP"}}
  ],
  "edges": [
    {"id":"e-s1","source":"start","target":"step-1"},
    {"id":"e-12","source":"step-1","target":"step-2"},
    {"id":"e-23","source":"step-2","target":"step-3"},
    {"id":"e-34","source":"step-3","target":"step-4"},
    {"id":"e-45","source":"step-4","target":"step-5"},
    {"id":"e-56","source":"step-5","target":"step-6"},
    {"id":"e-6e","source":"step-6","target":"end"}
  ]
}`
