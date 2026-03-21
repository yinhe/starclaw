# StarClaw 开发者指南

> 从零开始，为 StarClaw 开发 Agent、技能、工作流和插件。

---

## 一、环境准备

### 1.1 系统要求

| 工具 | 版本 | 用途 |
|------|------|------|
| **Go** | ≥ 1.21 | 后端 API 开发 |
| **Node.js** | ≥ 18 | 前端开发 |
| **pnpm** | ≥ 8 | 前端包管理（推荐） |
| **Docker** | ≥ 24 | 容器化运行 |
| **MySQL** | 8.0 | 主数据库（或用 SQLite 轻量模式） |
| **Redis** | 7.x | 缓存（可选，关闭后用内存缓存） |
| **Git** | ≥ 2.40 | 版本控制 |

### 1.2 获取代码

```bash
git clone https://github.com/yinhe/starclaw.git
cd starclaw
```

### 1.3 环境变量

```bash
cp .env.example .env
```

编辑 `.env`，至少配置：

```bash
# 必须修改
JWT_SECRET=your-random-secret-string

# AI 能力（至少配一个）
OPENAI_API_KEY=sk-xxx              # OpenAI（用于 RAG 嵌入）
PLATFORM_QWEN_API_KEY=sk-xxx       # 通义千问
PLATFORM_DEEPSEEK_API_KEY=sk-xxx   # DeepSeek
```

所有配置项通过 `STARCLAW_` 前缀的环境变量覆盖，映射规则：`STARCLAW_DATABASE_HOST` → `database.host`。

完整配置参考 `claw/api/internal/config/config.go`。

---

## 二、快速启动

提供三种启动方式，按开发场景选择：

### 方式 A：Docker 一键启动（推荐新手）

```bash
# 启动 MySQL + Redis + API + Web 四个容器
docker compose up -d

# 查看日志
docker compose logs -f api
```

访问 `http://localhost` 即可使用。首次启动会自动创建数据库表。

### 方式 B：本地开发模式（推荐开发者）

**终端 1 — 启动数据库：**
```bash
docker compose up -d mysql redis
```

**终端 2 — 启动 Go 后端：**
```bash
cd claw/api
go run ./cmd/server
# 默认监听 :8080
```

**终端 3 — 启动 React 前端：**
```bash
cd claw/web
pnpm install
pnpm dev
# 默认监听 :5173，自动代理 API 到 :8080
```

访问 `http://localhost:5173`。

### 方式 C：SQLite 极简模式（零依赖）

无需 Docker/MySQL/Redis，适合快速验证：

```bash
cd claw/api

# 使用 SQLite + 关闭 Redis
STARCLAW_DATABASE_DRIVER=sqlite \
STARCLAW_DATABASE_SQLITE_PATH=./data/claw.db \
STARCLAW_REDIS_ENABLED=false \
go run ./cmd/server
```

### 首次登录

1. 打开 Web UI → 进入 Setup 页面
2. 设置管理员用户名和密码
3. 进入 **模型配置** → 添加至少一个 LLM 模型（如通义千问）
4. 开始创建 Agent 和对话

---

## 三、项目结构

### 3.1 Claw 后端（`claw/api/`）

```
claw/api/
├── cmd/server/main.go          # 入口：配置加载 → DB → Redis → Router → 启动
├── configs/config.yaml          # 可选配置文件（环境变量优先）
├── plugins/                     # JSON 插件目录（自动加载）
├── Dockerfile                   # 容器构建
└── internal/                    # 核心代码（Go internal 包，不对外暴露）
    ├── config/                  # 配置结构体 + Viper 加载
    ├── database/                # DB 初始化 + 迁移 + 索引 + 缓存
    ├── model/                   # 数据模型（29 个文件）
    │   ├── agent.go             #   Agent 智能体
    │   ├── workflow.go          #   Workflow 工作流
    │   ├── plugin.go            #   PluginListing 插件市场
    │   ├── memory.go            #   Memory 记忆
    │   └── ...                  #   User, Conversation, Message, Task, ...
    ├── router/router.go         # 路由注册（1870 行，所有端点在此注册）
    ├── api/v1/                  # HTTP Handler（49 个文件）
    │   ├── chat.go              #   对话（同步 + SSE 流式）
    │   ├── agent.go             #   Agent CRUD
    │   ├── workflow.go          #   Workflow CRUD + 执行
    │   ├── marketplace.go       #   市场（上架/购买/评分）
    │   ├── developer.go         #   开发者门户（OpenAPI/Playground）
    │   └── ...
    ├── tool/                    # 技能系统（30 个文件）
    │   ├── tool.go              #   Tool 接口 + Registry
    │   ├── plugin.go            #   JSON Plugin 加载器
    │   ├── code_tool.go         #   代码沙箱
    │   ├── browser_tool.go      #   浏览器控制
    │   ├── video_tool.go        #   视频生成
    │   └── ...                  #   24+ 内置技能
    ├── provider/                # LLM Provider 抽象层
    │   ├── provider.go          #   ModelProvider 接口
    │   ├── openai.go            #   OpenAI / 兼容 API
    │   ├── qwen.go              #   通义千问
    │   ├── starai.go            #   StarAI (Synapse) Ed25519 签名
    │   └── ...
    ├── agent/                   # Agent 运行时（tool calling 循环）
    ├── workflow/                 # 工作流 DAG 引擎
    ├── rag/                     # RAG Pipeline（分块/嵌入/检索）
    ├── memory/                  # 记忆系统（提取/召回/生命周期）
    ├── mcp/                     # MCP Server 接入
    ├── webhook/                 # 事件规则引擎
    ├── billing/                 # 计费网关
    ├── sandbox/                 # 代码沙箱
    ├── security/                # 加密 + 审计链
    ├── squad/                   # 战队系统（多 Claw 协作）
    ├── node/                    # 节点身份（Ed25519 密钥对）
    ├── swarm/                   # Queen Swarm 客户端
    ├── observe/                 # 可观测性（日志/指标/告警）
    ├── middleware/               # JWT 鉴权 / 限流 / RBAC / 日志
    └── worker/                  # 后台任务执行器
```

### 3.2 Claw 前端（`claw/web/`）

```
claw/web/src/
├── App.tsx                      # 路由定义（38 页面）
├── main.tsx                     # React 入口
├── components/                  # 共享组件（Layout, ThemeToggle, ...）
├── lib/
│   ├── api.ts                   # API 客户端（所有后端接口的 TypeScript 封装）
│   └── i18n.ts                  # 国际化（中/英）
├── stores/                      # Zustand 状态管理
└── pages/                       # 38 个页面
    ├── ChatPage.tsx             #   AI 对话
    ├── AgentsPage.tsx           #   Agent 管理
    ├── WorkflowPage.tsx         #   可视化工作流编辑器
    ├── SkillsPage.tsx           #   技能管理
    ├── MarketplacePage.tsx      #   Agent 市场
    ├── MCPPage.tsx              #   MCP 服务器管理
    ├── DeveloperPage.tsx        #   开发者门户
    └── ...
```

**技术栈：** React + TypeScript + Vite + TailwindCSS + Lucide Icons + Zustand + React Flow（工作流编辑器）

### 3.3 数据库

自动迁移，无需手动建表。核心表：

| 表名 | 模型 | 说明 |
|------|------|------|
| `users` | User | 用户 |
| `agents` | Agent | 智能体 |
| `conversations` | Conversation | 对话 |
| `messages` | Message | 消息 |
| `workflows` | Workflow | 工作流定义 |
| `workflow_runs` | WorkflowRun | 工作流执行记录 |
| `model_configs` | ModelConfig | LLM 模型配置 |
| `knowledge_bases` | KnowledgeBase | RAG 知识库 |
| `documents` | Document | 知识库文档 |
| `memories` | Memory | Agent 记忆 |
| `tasks` | Task | 后台任务 |
| `plugin_listings` | PluginListing | 插件市场 |
| `squads` | Squad | 战队 |
| `missions` | Mission | 战队任务 |

---

## 四、开发 Agent（智能体）

### 4.1 通过 API 创建

```bash
# 创建一个研究助手 Agent
curl -X POST http://localhost:8080/v1/agents \
  -H "Authorization: Bearer <your-token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "研究助手",
    "description": "帮你搜索和整理信息",
    "system_prompt": "你是一个专业的研究助手。当用户提出问题时，先用 web_search 搜索最新信息，然后用 browser 打开关键网页获取详情，最后整理成结构化报告。",
    "model_name": "qwen/qwen-max",
    "tools": "[\"web_search\", \"browser\", \"document\"]",
    "config": "{\"temperature\": 0.3, \"max_tokens\": 4096}"
  }'
```

### 4.2 Agent 对话

```bash
# 同步对话
curl -X POST http://localhost:8080/v1/chat \
  -H "Authorization: Bearer <token>" \
  -d '{"conversation_id": "<conv-id>", "message": "帮我调研 2026 年 AI Agent 市场规模"}'

# SSE 流式对话
curl -N http://localhost:8080/v1/chat/stream \
  -H "Authorization: Bearer <token>" \
  -d '{"conversation_id": "<conv-id>", "message": "帮我调研 2026 年 AI Agent 市场规模"}'
```

### 4.3 Agent 运行时原理

```
用户消息
  ↓
Agent Runtime (agent/runtime.go)
  ↓
┌─ LLM 推理循环（最多 10 轮 tool calling）─────────────────────┐
│                                                                │
│  1. 构造 messages: [system_prompt, ...history, user_message]   │
│  2. 调用 LLM (provider.ChatSync / ChatStream)                 │
│  3. LLM 返回 tool_calls? ──→ 是 ──→ 执行 tool → 结果追加     │
│     │                               messages → 回到步骤 2     │
│     └─ 否 ──→ 返回最终文本响应                                 │
│                                                                │
└────────────────────────────────────────────────────────────────┘
```

### 4.4 内置 Agent 类型

| 模板 | 典型 Prompt 关键词 | 推荐技能 |
|------|-------------------|---------|
| 通用助手 | "你是一个有帮助的 AI 助手" | `web_search` `document` |
| 编程助手 | "你是一个编程专家" | `code` `git` `web_search` |
| 影视创作 | "你是一个影视制作人" | `video_generation` `image_generation` `music_generation` `dubbing` `subtitle` `comic_production` `mv_production` `audio_analysis` |
| 数据分析 | "你是一个数据分析师" | `code` `http_request` `document` |
| 企业助手 | "你是企业智能助手" | `system` `wecom`/`dingtalk`/`feishu` `http_request` |
| 全能 Agent | "你可以使用所有可用工具" | 全选 |

---

## 五、开发技能（Tool）

### 5.1 Tool 接口

所有技能实现同一个接口（`claw/api/internal/tool/tool.go`）：

```go
type Tool interface {
    Name() string                                              // 唯一标识
    Description() string                                       // LLM 读取，据此决定何时调用
    Parameters() interface{}                                   // JSON Schema，定义输入参数
    Execute(ctx context.Context, args string) (string, error)  // 执行逻辑
}
```

### 5.2 方式一：Go 内置技能（完整示例）

**Step 1 — 创建技能文件**

```go
// claw/api/internal/tool/stock_tool.go
package tool

import (
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
)

type StockTool struct{}

func NewStockTool() *StockTool { return &StockTool{} }

func (t *StockTool) Name() string { return "stock_price" }

func (t *StockTool) Description() string {
    return "查询股票实时价格。输入股票代码（如 AAPL、600519），返回最新价格和涨跌幅。"
}

func (t *StockTool) Parameters() interface{} {
    return &JSONSchema{
        Type: "object",
        Properties: map[string]Property{
            "symbol": {Type: "string", Description: "股票代码，如 AAPL, 600519.SH"},
        },
        Required: []string{"symbol"},
    }
}

func (t *StockTool) Execute(ctx context.Context, args string) (string, error) {
    // 1. 解析参数
    params, err := ParseArgs[struct {
        Symbol string `json:"symbol"`
    }](args)
    if err != nil {
        return "", err
    }

    // 2. 调用外部 API（示例）
    resp, err := http.Get(fmt.Sprintf("https://api.example.com/stock/%s", params.Symbol))
    if err != nil {
        return "", fmt.Errorf("查询失败: %v", err)
    }
    defer resp.Body.Close()
    body, _ := io.ReadAll(resp.Body)

    // 3. 返回 JSON（LLM 会解析并呈现给用户）
    return string(body), nil
}
```

**Step 2 — 注册到 Router**

编辑 `claw/api/internal/router/router.go`，在 tool 注册区域添加：

```go
toolRegistry.Register(tool.NewStockTool())
```

**Step 3 — 编译验证**

```bash
cd claw/api
go build ./...   # 编译检查
go run ./cmd/server  # 启动测试
```

**Step 4 — 测试**

在 Web UI 中创建 Agent，勾选 `stock_price` 技能，然后对话："查一下苹果股价"

### 5.3 方式二：JSON 插件（零代码）

在 `claw/api/plugins/` 目录下创建 JSON 文件：

```json
// claw/api/plugins/ip_lookup.json
{
  "name": "ip_lookup",
  "description": "查询 IP 地址的地理位置信息",
  "version": "1.0.0",
  "author": "StarClaw",
  "parameters": {
    "type": "object",
    "properties": {
      "ip": {
        "type": "string",
        "description": "IP 地址，如 8.8.8.8"
      }
    },
    "required": ["ip"]
  },
  "endpoint": {
    "url": "https://ipapi.co/{{ip}}/json/",
    "method": "GET"
  },
  "headers": {
    "Accept": "application/json"
  }
}
```

**特性：**
- `{{param}}` — URL 模板变量，自动替换
- POST 方法 — 参数自动作为 JSON Body 发送
- 响应自动截断到 100KB
- 启动时自动加载，无需编译

### 5.4 方式三：MCP 服务器

**连接已有 MCP 服务器：**
1. Web UI → MCP 页面 → 添加服务器
2. 输入名称和 URL（如 `http://localhost:3001/sse`）
3. Claw 自动发现并注册所有工具

**MCP Bridge（宿主系统访问）：**
```bash
# 下载并运行 MCP Bridge
npx @anthropic/mcp-bridge --port 3001

# Claw 自动检测并注册为 mcp_bridge 工具
# Agent 可通过它执行宿主命令、读写文件
```

**自建 MCP 服务器（Node.js 示例）：**

```javascript
import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";

const server = new McpServer({ name: "my-tools", version: "1.0.0" });

// 注册工具
server.tool("query_database",
  { sql: { type: "string", description: "SQL query to execute" } },
  async ({ sql }) => ({
    content: [{ type: "text", text: JSON.stringify(await db.query(sql)) }],
  })
);

const transport = new StdioServerTransport();
await server.connect(transport);
```

### 5.5 高级模式

**获取用户/对话上下文：**
```go
func (t *MyTool) Execute(ctx context.Context, args string) (string, error) {
    userID, _ := ctx.Value(CtxKeyUserID).(string)
    convID, _ := ctx.Value(CtxKeyConversationID).(string)
    // 可据此做权限控制、数据隔离
}
```

**访问数据库：**
```go
type MyTool struct { db *gorm.DB }

func NewMyTool(db *gorm.DB) *MyTool { return &MyTool{db: db} }

// 在 Execute 中使用 t.db 查询/写入数据
```

**生成文件并返回下载链接：**
```go
// 保存文件
filename := fmt.Sprintf("report_%s.pdf", uuid.New().String()[:8])
os.WriteFile(filepath.Join(dataDir, "reports", filename), data, 0644)

// 返回下载 URL（需在 router.go 中添加对应的 GET 路由）
return toJSON(map[string]interface{}{
    "status": "success",
    "download_url": fmt.Sprintf("/v1/reports/%s", filename),
}), nil
```

---

## 六、开发工作流（Workflow）

### 6.1 工作流定义

工作流是 JSON 定义的 DAG，包含 `nodes`（节点）和 `edges`（连线）：

```json
{
  "nodes": [
    {
      "id": "1", "type": "start",
      "position": {"x": 0, "y": 0}, "data": {}
    },
    {
      "id": "2", "type": "llm",
      "position": {"x": 200, "y": 0},
      "data": {
        "model": "qwen/qwen-max",
        "prompt": "将以下文本翻译成英文：\n\n{{input}}",
        "temperature": 0.3,
        "maxTokens": 2048
      }
    },
    {
      "id": "3", "type": "end",
      "position": {"x": 400, "y": 0}, "data": {}
    }
  ],
  "edges": [
    { "id": "e1", "source": "1", "target": "2" },
    { "id": "e2", "source": "2", "target": "3" }
  ]
}
```

### 6.2 节点类型

| 类型 | 说明 | `data` 字段 |
|------|------|------------|
| `start` | 入口，接收 input | — |
| `end` | 出口，输出最终结果 | `outputMapping`: 可选模板 |
| `llm` | LLM 推理 | `model`, `prompt` (`{{input}}`模板), `temperature`, `maxTokens` |
| `tool` | 调用技能 | `toolName`, `argsTemplate` (`{{input}}`模板) |
| `condition` | 条件分支 | `expression`: 如 `input.contains("退款")`, `input.length > 100` |

### 6.3 条件表达式

| 表达式 | 说明 |
|--------|------|
| `input.contains("关键词")` | 输入包含关键词 |
| `input.length > N` | 输入长度大于 N |
| `input == "某个值"` | 精确匹配 |
| 空/其他 | 输入非空则 true |

条件节点有两个输出 handle：`true` 和 `false`，分别连向不同的后续节点。

### 6.4 通过 API 操作工作流

```bash
# 创建工作流
curl -X POST http://localhost:8080/v1/workflows \
  -H "Authorization: Bearer <token>" \
  -d '{"name": "翻译工作流", "definition": "<上面的JSON>"}'

# 执行工作流
curl -X POST http://localhost:8080/v1/workflows/<id>/run \
  -H "Authorization: Bearer <token>" \
  -d '{"input": "今天天气真好"}'

# 生成 Webhook Token（外部触发）
curl -X POST http://localhost:8080/v1/workflows/<id>/webhook \
  -H "Authorization: Bearer <token>"
# 返回: { "webhook_url": "/v1/webhooks/workflow/<token>" }

# 通过 Webhook 触发（无需认证）
curl -X POST http://localhost:8080/v1/webhooks/workflow/<token> \
  -d '{"input": "今天天气真好"}'
```

### 6.5 前端工作流编辑器

Web UI 的 WorkflowPage 使用 React Flow 实现可视化编辑：
- 拖拽添加节点
- 连线定义执行顺序
- 实时预览执行结果
- 支持保存为模板上架市场

---

## 七、Webhook 事件系统

### 7.1 创建事件规则

```bash
curl -X POST http://localhost:8080/v1/webhooks/rules \
  -H "Authorization: Bearer <token>" \
  -d '{
    "name": "Agent 报错通知钉钉",
    "event_type": "agent.error",
    "filter": "{\"agent_name\": \"客服机器人\"}",
    "action_type": "webhook",
    "action_config": "{\"url\": \"https://oapi.dingtalk.com/robot/send?access_token=xxx\", \"method\": \"POST\"}",
    "enabled": true
  }'
```

### 7.2 支持的事件类型

| 事件 | 说明 |
|------|------|
| `agent.complete` | Agent 完成任务 |
| `agent.error` | Agent 执行出错 |
| `workflow.complete` | 工作流执行完成 |
| `chat.message` | 收到新消息 |
| `task.complete` | 后台任务完成 |
| `squad.step.done` | 战队步骤完成 |

### 7.3 事件规则 API

| 方法 | 端点 | 说明 |
|------|------|------|
| `GET` | `/v1/webhooks/rules` | 列出所有规则 |
| `POST` | `/v1/webhooks/rules` | 创建规则 |
| `PUT` | `/v1/webhooks/rules/:id` | 更新规则 |
| `POST` | `/v1/webhooks/rules/:id/toggle` | 启用/禁用 |
| `DELETE` | `/v1/webhooks/rules/:id` | 删除规则 |
| `GET` | `/v1/webhooks/logs` | 查看事件日志 |
| `POST` | `/v1/webhooks/logs/:id/retry` | 重试失败事件 |
| `POST` | `/v1/webhooks/test` | 测试规则 |

---

## 八、前端开发

### 8.1 开发环境

```bash
cd claw/web
pnpm install
pnpm dev          # 开发服务器 :5173
pnpm build        # 生产构建
pnpm tsc --noEmit # 类型检查
```

### 8.2 添加新页面

**1. 创建页面组件：**

```tsx
// claw/web/src/pages/MyPage.tsx
import { useState, useEffect } from 'react'
import { myAPI } from '../lib/api'

export default function MyPage() {
  const [data, setData] = useState([])

  useEffect(() => {
    myAPI.list().then(setData)
  }, [])

  return (
    <div className="p-6">
      <h1 className="text-2xl font-bold">My Page</h1>
      {/* ... */}
    </div>
  )
}
```

**2. 注册路由（`App.tsx`）：**

```tsx
import MyPage from './pages/MyPage'

// 在 routes 数组中添加
{ path: '/my-page', element: <MyPage /> }
```

**3. 添加导航（`components/Layout.tsx`）：**

```tsx
// 在导航项数组中添加
{ icon: Sparkles, label: t('nav.myPage'), path: '/my-page' }
```

**4. 添加国际化（`lib/i18n.ts`）：**

```ts
nav: {
  myPage: { zh: '我的页面', en: 'My Page' },
}
```

### 8.3 API 调用约定

所有 API 调用封装在 `claw/web/src/lib/api.ts`：

```ts
// 添加新的 API 方法
export const myAPI = {
  list: () => api.get('/my-resource').then(r => r.data),
  create: (data: any) => api.post('/my-resource', data).then(r => r.data),
  update: (id: string, data: any) => api.put(`/my-resource/${id}`, data).then(r => r.data),
  delete: (id: string) => api.delete(`/my-resource/${id}`),
}
```

---

## 九、LLM Provider 开发

### 9.1 Provider 接口

```go
// claw/api/internal/provider/provider.go
type ModelProvider interface {
    ChatSync(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
    ChatStream(ctx context.Context, req *ChatRequest) (<-chan StreamEvent, error)
}
```

### 9.2 添加新 Provider

```go
// claw/api/internal/provider/my_provider.go
type MyProvider struct {
    apiKey  string
    baseURL string
}

func (p *MyProvider) ChatSync(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
    // 将 ChatRequest 转换为目标 API 格式
    // 发送 HTTP 请求
    // 将响应转换回 ChatResponse
}

func (p *MyProvider) ChatStream(ctx context.Context, req *ChatRequest) (<-chan StreamEvent, error) {
    // SSE 流式实现
}
```

注册到 `router.go`：
```go
providerRegistry.Register("my-provider", provider.NewMyProvider(apiKey, baseURL))
```

### 9.3 现有 Provider

| Provider | 支持的模型 | 认证方式 |
|----------|----------|---------|
| `openai` | GPT-4o, GPT-4, GPT-3.5 | API Key |
| `qwen` | Qwen-Max, Qwen-Plus, Qwen-Turbo | API Key |
| `deepseek` | DeepSeek-V3, DeepSeek-Coder | API Key |
| `google` | Gemini Pro, Gemini Flash | API Key |
| `anthropic` | Claude 3.5, Claude 3 | API Key |
| `ollama` | 本地模型 (llama, mistral, ...) | 无需 |
| `star-ai` | 通过 Synapse 代理所有模型 | Ed25519 签名 |

---

## 十、CLI 工具

Claw 服务端自带 CLI 子命令，无需额外安装：

```bash
cd claw/api

# 查看版本
go run ./cmd/server version

# 获取 API Token（用于开发调试）
go run ./cmd/server get-token

# 重置 Token
go run ./cmd/server reset-token

# 重置密码
go run ./cmd/server reset-password

# 查看已注册设备
go run ./cmd/server devices

# 审批/拒绝设备
go run ./cmd/server approve <device-id>
go run ./cmd/server reject <device-id>

# 钱包操作
go run ./cmd/server wallet-info     # 查看钱包信息
go run ./cmd/server balance         # 查看星能余额
go run ./cmd/server transfer        # 转账
go run ./cmd/server transactions    # 交易记录

# 密钥管理
go run ./cmd/server export-key      # 导出私钥
go run ./cmd/server import-key      # 导入私钥
```

---

## 十一、测试与调试

### 11.1 后端测试

```bash
cd claw/api

# 编译检查
go build ./...

# 静态分析
go vet ./...

# 运行测试
go test ./...
```

### 11.2 前端测试

```bash
cd claw/web

# 类型检查
pnpm tsc --noEmit

# 构建检查
pnpm build

# E2E 测试（Playwright）
pnpm playwright test
```

### 11.3 API 调试

**Swagger UI：** 启动后访问 `http://localhost:8080/v1/developer/docs`

**API Playground：** Web UI → 开发者 → Playground

**curl 快速测试：**
```bash
# 获取 Token
TOKEN=$(curl -s http://localhost:8080/v1/auth/login \
  -d '{"username":"admin","password":"xxx"}' | jq -r '.token')

# 列出 Agent
curl http://localhost:8080/v1/agents -H "Authorization: Bearer $TOKEN"

# 列出技能
curl http://localhost:8080/v1/skills -H "Authorization: Bearer $TOKEN"

# 健康检查
curl http://localhost:8080/health
```

### 11.4 常见问题

| 问题 | 解决 |
|------|------|
| `tool not found: xxx` | 检查 `router.go` 是否注册了该 tool |
| Agent 不调用技能 | 检查 Agent 的 `tools` 字段是否包含该技能名 |
| LLM 返回空 | 检查模型配置的 API Key 是否有效 |
| CORS 403 | 开源模式下 CORS 默认允许所有来源，检查是否设为 hosted 模式 |
| SQLite 模式报错 | 确认 `STARCLAW_DATABASE_DRIVER=sqlite` 且 data 目录可写 |

---

## 十二、贡献流程

### 12.1 开发流程

```
1. Fork 仓库 → Clone 到本地
2. 创建分支: git checkout -b feature/my-tool
3. 开发 + 测试: go build ./... && go vet ./...
4. 提交: git commit -m "feat(tool): add stock_price tool"
5. Push: git push origin feature/my-tool
6. 创建 Pull Request
```

### 12.2 Commit 规范

```
<type>(<scope>): <description>

type:
  feat     — 新功能
  fix      — 修复 Bug
  docs     — 文档
  refactor — 重构
  test     — 测试
  chore    — 构建/工具

scope:
  tool     — 技能相关
  agent    — Agent 相关
  workflow — 工作流相关
  web      — 前端相关
  api      — API/路由相关
  provider — LLM Provider
  plugin   — 插件系统

示例:
  feat(tool): add stock_price tool
  fix(agent): fix tool calling loop limit
  docs: update developer guide
```

### 12.3 代码规范

- **Go**: 遵循标准 Go 风格，`gofmt` 格式化
- **TypeScript**: 遵循项目 ESLint 配置
- **注释**: 中英文均可，保持与周围代码一致
- **错误消息**: 面向用户的错误用中文，系统日志用英文
- **Tool 返回值**: 始终返回 JSON 格式，便于 LLM 解析

### 12.4 添加新技能的 Checklist

- [ ] 实现 `Tool` 接口（Name/Description/Parameters/Execute）
- [ ] 在 `router.go` 中注册
- [ ] `go build ./...` 编译通过
- [ ] `go vet ./...` 无警告
- [ ] 在 Web UI 中创建 Agent 勾选该技能并测试对话
- [ ] （可选）在 `SkillsPage.tsx` 中添加图标
- [ ] （可选）更新 `SKILL_DEVELOPMENT.md` 技能列表

---

## 十三、API 速查表

### 认证

| 方法 | 端点 | 说明 |
|------|------|------|
| POST | `/v1/auth/register` | 注册 |
| POST | `/v1/auth/login` | 登录 → JWT |
| POST | `/v1/setup` | 首次设置（开源模式） |

### Agent

| 方法 | 端点 | 说明 |
|------|------|------|
| GET | `/v1/agents` | 列出 Agent |
| POST | `/v1/agents` | 创建 Agent |
| PUT | `/v1/agents/:id` | 更新 Agent |
| DELETE | `/v1/agents/:id` | 删除 Agent |

### 对话

| 方法 | 端点 | 说明 |
|------|------|------|
| GET | `/v1/conversations` | 列出对话 |
| POST | `/v1/chat` | 发送消息（同步） |
| POST | `/v1/chat/stream` | 发送消息（SSE 流式） |

### 工作流

| 方法 | 端点 | 说明 |
|------|------|------|
| GET | `/v1/workflows` | 列出工作流 |
| POST | `/v1/workflows` | 创建工作流 |
| POST | `/v1/workflows/:id/run` | 执行工作流 |
| POST | `/v1/workflows/:id/webhook` | 生成 Webhook Token |

### 技能

| 方法 | 端点 | 说明 |
|------|------|------|
| GET | `/v1/skills` | 列出所有已注册技能 |

### 知识库

| 方法 | 端点 | 说明 |
|------|------|------|
| GET | `/v1/knowledge-bases` | 列出知识库 |
| POST | `/v1/knowledge-bases` | 创建知识库 |
| POST | `/v1/knowledge-bases/:id/documents` | 上传文档 |
| POST | `/v1/knowledge-bases/:id/search` | 语义搜索 |

### 市场

| 方法 | 端点 | 说明 |
|------|------|------|
| GET | `/v1/marketplace/items` | 浏览市场 |
| POST | `/v1/marketplace/items/:id/install` | 安装 |
| POST | `/v1/marketplace/creator` | 注册创作者 |

### 开发者

| 方法 | 端点 | 说明 |
|------|------|------|
| GET | `/v1/developer/docs` | Swagger UI |
| GET | `/v1/developer/openapi.json` | OpenAPI 3.0 规范 |

---

## 附录：架构全景图

```
┌─────────────────────────────────────────────────────────────────────┐
│                        StarClaw Architecture                         │
│                                                                      │
│  ┌─ Web UI ────────┐    ┌─ API Server ─────────────────────────┐    │
│  │  React + Vite   │───→│  Gin Router                           │    │
│  │  38 Pages       │    │    ├─ Auth (JWT / Ed25519 / OAuth)    │    │
│  │  React Flow     │    │    ├─ Chat (sync + SSE stream)        │    │
│  └─────────────────┘    │    ├─ Agent CRUD                      │    │
│                          │    ├─ Workflow (DAG engine)           │    │
│  ┌─ Mobile App ────┐    │    ├─ Skills / Plugins / MCP          │    │
│  │  (optional)     │───→│    ├─ Marketplace                     │    │
│  └─────────────────┘    │    ├─ Webhook Rules                   │    │
│                          │    ├─ Knowledge Base (RAG)            │    │
│                          │    └─ Developer Portal                │    │
│                          │                                       │    │
│                          │  ┌─ Core Services ──────────────┐    │    │
│                          │  │  Agent Runtime (tool calling) │    │    │
│                          │  │  Tool Registry (24+ tools)    │    │    │
│                          │  │  Provider Registry (7+ LLMs)  │    │    │
│                          │  │  Memory (extract + recall)    │    │    │
│                          │  │  Billing Gateway              │    │    │
│                          │  │  Task Worker (async)          │    │    │
│                          │  └───────────────────────────────┘    │    │
│                          └───────────────────────────────────────┘    │
│                                       │                               │
│                             ┌─────────┼─────────┐                    │
│                             ▼         ▼         ▼                    │
│                          MySQL     Redis     AI Gateway               │
│                          (data)    (cache)   (LLM proxy)              │
└─────────────────────────────────────────────────────────────────────┘
```

---

> **更多文档：**
> - 技能开发详细指南 → `docs/SKILL_DEVELOPMENT.md`
> - 部署指南 → `docs/DEPLOY.md`
> - API 参考 → `docs/API.md`
