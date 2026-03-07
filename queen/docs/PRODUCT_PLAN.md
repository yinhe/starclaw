# StarClaw - AI Agent 平台产品规划

## 一、产品定位

**一句话描述：** StarClaw 是一个开源的多模型 AI Agent 编排平台，让用户通过自然语言或可视化方式创建、组合、运行智能体，完成复杂任务。

**核心差异化：**
- 多模型统一接入（OpenAI / Claude / Gemini / DeepSeek / 开源模型等）
- 可视化 Agent 工作流编排（类似 n8n / Dify 的拖拽体验）
- 内置丰富的 Tool 生态（代码执行、网页浏览、文件处理、API 调用等）
- 支持 Multi-Agent 协作（多智能体协同完成复杂任务）
- 本地部署 + 云端 SaaS 双模式

---

## 二、目标用户

| 用户群体 | 需求 |
|---------|------|
| 开发者 | 通过 SDK/API 快速构建 Agent 应用 |
| 产品经理/运营 | 零代码拖拽式创建业务 Agent |
| 企业客户 | 私有化部署，数据安全，流程自动化 |
| AI 爱好者 | 探索和分享 Agent 玩法 |

---

## 三、产品路线图

### Phase 1 - MVP ✅ 已完成
- [x] 多模型统一接入层（OpenAI / Anthropic / DeepSeek / Ollama / OpenRouter）
- [x] 模型管理：API Key 配置、模型参数调整、用量监控（Dashboard Token 统计）
- [x] 基础 Agent 运行时：System Prompt + Tool Calling + 对话历史
- [x] 内置基础 Tools：
  - Web 搜索（DuckDuckGo API）
  - HTTP 请求（GET / POST）
- [x] Chat UI：SSE 流式对话、历史记录、Markdown 渲染、Tool 调用状态、代码块复制、消息反馈（👍👎）
- [x] 对话管理：重命名 / 删除 / 导出 Markdown / 搜索过滤
- [x] Agent 创建向导：配置 Prompt / 模型 / 工具 / 知识库 + 6 个 System Prompt 模板
- [x] Agent 市场：浏览公开 Agent / 一键克隆
- [x] 用户系统：注册登录（JWT）、个人资料、修改密码、API Key 管理
- [x] 深色模式 + 移动端响应式布局
- [x] Docker Compose 一键部署 + .env.example

### Phase 2 - 工作流引擎 ✅ 已完成
- [x] 可视化工作流编辑器（React Flow 拖拽画布）
- [x] 5 种节点类型：Start / LLM / Tool / Condition / End
- [x] 节点属性编辑面板（模型选择、Prompt 模板、Temperature 等）
- [x] 工作流保存 / 加载 / 执行（后端图遍历引擎）
- [x] 条件分支节点（contains / length / 相等判断）
- [x] `{{input}}` 模板变量系统
- [x] 多 Agent 协作模式：
  - [x] 串行链式调用（Sequential）
  - [x] 并行执行（Parallel）
  - [x] 编排者模式（Orchestrated，管理者 Agent 委托子 Agent）
- [x] Webhook 触发器（HTTP POST 触发工作流，Token 鉴权，无需登录）
- [x] 运行日志（WorkflowRun 记录：状态/耗时/输入输出/错误，UI 面板展示）
- [x] 工作流模板市场（发布/浏览/分类/克隆）
- [x] 定时任务（Cron 触发，Schedule CRUD + 前端管理 UI）

### Phase 3 - 知识库 & RAG ✅ 已完成
- [x] 文档上传（文本文件 + 直接文本输入，支持 .txt/.md/.csv/.json/.py/.go 等）
- [x] 向量化存储 & 检索（OpenAI Embedding + 余弦相似度）
- [x] RAG Pipeline：智能分块（句子/段落边界）→ 批量嵌入 → 存储 → 检索
- [x] 知识库关联 Agent（自动检索注入 System Prompt）
- [x] 知识库管理页面（列表 / 详情 / 搜索 / 上传）
- [x] PDF / Word 文档解析（Go 原生 PDF + DOCX 解析，无需 Python）
- [x] 多模态支持（图片理解、语音输入/输出）

### Phase 4 - 生态 & 企业级 ✅ 已完成
- [x] MCP (Model Context Protocol) 兼容：JSON-RPC 2.0 Client + MCPTool 适配器
- [x] MCP 服务器管理（添加 / 删除 / 测试连接）
- [x] Dashboard 仪表盘（数据统计 + 快捷操作）
- [x] 设置页面（个人资料 + 密码 + API Key 管理）
- [x] Agent 市场（公开 Agent 浏览 + 克隆）
- [x] Toast 通知系统 + 全局 Error Boundary
- [x] /health 健康检查（含 DB 状态 + 运行时间）
- [x] 审计日志（AuditLog 模型 + List API + Settings 页面展示）
- [x] API 速率限制（per-IP + per-user 限流 + X-RateLimit headers）
- [x] Agent 导入/导出 JSON
- [x] 全局命令面板（Ctrl+K 搜索 / Ctrl+N 新对话）
- [x] 对话置顶 + 批量删除
- [x] Dashboard 用量图表（7 日柱状图 + Agent 排行榜）
- [x] i18n 国际化框架（中/英切换）
- [x] 优雅停机 + 结构化请求日志
- [x] 登录页增强（密码可见切换 + 记住我）
- [x] 加载骨架屏 + 深色模式全覆盖
- [x] 插件/Tool 开发 SDK（JSON 插件规范 + 动态加载 + 示例插件）
- [x] RBAC 角色权限（admin/user 角色 + RequireAdmin 中间件 + 用户管理 API）
- [x] 计费系统 / 多租户（Tenant 多租户 + Plan 套餐 + UsageRecord 用量追踪 + 成员管理 + 配额检查）

### Phase 5 - 自主 Agent 🔨 部分完成
- [x] 长期记忆系统（Memory 模型 + CRUD + Recall API，按重要性/访问次数排序）
- [x] Agent 自主规划 & 反思（ReAct Runtime：Think → Act → Observe 循环，最多 8 步）
- [x] Computer Use（浏览器操作：headless Chrome + rod，navigate/click/type/screenshot/extract/scroll）
- [x] 自主编程 Agent（沙箱 workspace + CodeTool + 代码执行 + IDE 式前端）
- [x] Agent 评估 & 基准测试框架（TestCase + TestRun + 关键词评分）
- [x] SystemTool 系统管理工具（create_agent/list_agents/list_models/create_workflow/schedule_task）
- [x] 全能助手 SuperAgent（自动创建 + 全工具预装 + 智能系统提示 + 用户 context 注入）
- [x] 沙箱网站预览（/api/v1/preview/:workspace_id/*filepath 静态文件服务）
- [x] 对话工具卡片（可折叠 + 类型图标 + 状态徽章 + 展开查看输出 + HTML 预览链接）

---

## 四、核心功能架构

> 完整架构文档详见 [ARCHITECTURE.md](./ARCHITECTURE.md)

```
┌─────────────────────────────────────────────────┐
│            客户端层 (Client)                      │
│  web/ (React)  │  mobile/ (Flutter)  │  sdk/    │
├─────────────────────────────────────────────────┤
│            API 网关 (api/)                        │
│      认证 │ 限流 │ RBAC │ 路由 │ 日志             │
├─────────────────────────────────────────────────┤
│            业务服务层 (Services)                   │
│ ┌──────────┐ ┌──────────┐ ┌──────────┐          │
│ │ Agent    │ │ Workflow │ │ RAG      │          │
│ │ Engine   │ │ Engine   │ │ Pipeline │          │
│ └──────────┘ └──────────┘ └──────────┘          │
│ ┌──────────┐ ┌──────────┐ ┌──────────┐          │
│ │ Tool     │ │ Sandbox  │ │ Worker   │          │
│ │ System   │ │ (Code)   │ │ (Async)  │          │
│ └──────────┘ └──────────┘ └──────────┘          │
├─────────────────────────────────────────────────┤
│            模型统一接入层 (Provider)               │
│  Qwen │ OpenAI │ DeepSeek │ Ollama │ fal.ai    │
├─────────────────────────────────────────────────┤
│            基础设施层 (Infrastructure)             │
│  MySQL │ Redis │ FFmpeg │ Chromium │ DashScope  │
├─────────────────────────────────────────────────┤
│            虫群层 (Swarm) — 计划中                  │
│  Queen (core/) │ Overlord │ Claw │ billing/     │
└─────────────────────────────────────────────────┘
```

---

## 五、竞品分析

| 特性 | Dify | Coze | AutoGPT | LangFlow | **StarClaw** |
|------|------|------|---------|----------|-------------|
| 多模型支持 | ✅ | ❌(字节系) | ✅ | ✅ | ✅ |
| 可视化编排 | ✅ | ✅ | ❌ | ✅ | ✅ |
| Multi-Agent | ⚠️ | ⚠️ | ✅ | ⚠️ | ✅ |
| RAG 知识库 | ✅ | ✅ | ❌ | ⚠️ | ✅ |
| 开源 | ✅ | ❌ | ✅ | ✅ | ✅ |
| 自主 Agent | ❌ | ❌ | ✅ | ❌ | ✅ |
| 本地部署 | ✅ | ❌ | ✅ | ✅ | ✅ |
| Tool 生态 | ⚠️ | ✅ | ⚠️ | ⚠️ | ✅ |
| MCP 兼容 | ⚠️ | ❌ | ❌ | ❌ | ✅ |

**StarClaw 的核心优势：多模型 + 多 Agent 协作 + MCP 兼容 + 可视化编排 + 完全开源**

