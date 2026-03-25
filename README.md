# StarClaw 🦞

**开源 AI Agent 编排平台** — 多模型接入 / 可视化工作流 / RAG 知识库 / MCP 兼容 / Multi-Agent 协作

> 每一个部署的 StarClaw 实例就是一只小龙虾（Claw），它们由领主（Overlord）进行管理，
> 所有领主最终汇聚到虫后（Queen）的中央管控之下。灵感来自星际争霸虫族。

## 项目结构

```
starclaw/
├── claw/ 🦞         开源（免费）— 小龙虾执行单元
│   ├── api/          Go 后端 API 服务
│   ├── web/          React 前端
│   ├── data/         运行时数据
│   ├── deploy/       部署配置
│   ├── scripts/      工具脚本
│   └── docs/         开源文档
├── synapse/ ⛽       闭源（官方运营）— 突触 AI 算力网关
│   ├── api/          Go 后端（star-ai.net）
│   ├── web/          React 用户控制台
│   └── proxy/        海外 AI API 中转
├── larva/ 🐛         闭源（官方运营）— 幼虫跨平台客户端
│   └── lib/          Flutter App（iOS/Android/桌面/Web）
├── overlord/ 👁️     闭源（企业付费）— 领主管理层
│   ├── manager/      领主管理服务
│   └── console/      领主管理控制台
├── queen/ 👑        闭源（官方运营）— 虫后中央管控
│   ├── site/         官网落地页
│   └── docs/         全局架构文档
├── nydus/ 🕳️        闭源（官方运营）— 虫道部署管道
├── spore/ 🍄        闭源（官方运营）— 孢子桌面安装器
├── docker-compose.yml
└── .env.example
```

## 功能概览

| 模块 | 功能 |
|------|------|
| 仪表盘 | 数据统计、Token 用量、快捷操作入口 |
| 对话 | SSE 流式输出、Tool 调用状态、多轮对话历史 |
| Agent 管理 | 创建/编辑 Agent，配置模型、工具、知识库 |
| 模型管理 | 多 Provider，API Key 配置，参数调整 |
| 知识库 (RAG) | 文档上传、智能分块、向量检索、语义搜索 |
| MCP 工具 | 连接外部 MCP 工具服务器，扩展 Agent 能力 |
| 多 Agent 协作 | 顺序/并行/编排三种模式 |
| 工作流编辑器 | React Flow 可视化画布，5 种节点，保存/执行 |
| 编程 Agent | 自主编码、文件操作、代码执行沙箱 |
| 虫群网络 | Claw↔Overlord↔Queen 分布式协作（计划中） |

## 技术栈

| 层级 | 技术 |
|------|------|
| 前端 | React 18 + Vite + TypeScript + TailwindCSS + Zustand + React Flow |
| 后端 | Go 1.24 + Gin + GORM + Viper |
| 数据库 | MySQL 8.0 + Redis 7 |
| AI | Qwen / OpenAI / DeepSeek / Anthropic / Ollama / OpenRouter + StarAI |
| 部署 | Docker Compose + Nginx |

## 快速开始

### Docker Compose（推荐）

```bash
git clone https://github.com/yinhe/starclaw.git
cd starclaw

cp .env.example .env
nano .env  # 设置 JWT_SECRET、DB 密码

docker compose up -d

# 访问: http://localhost
```

### 本地开发

**前置要求：** Go 1.24+, Node.js 20+, MySQL 8.0, Redis 7

```bash
# 1. 基础设施
docker compose up -d mysql redis

# 2. API 服务
cd claw/api
go mod tidy
go run ./cmd/server

# 3. Web 前端（新终端）
cd claw/web
npm install
npm run dev
```

前端访问 http://localhost:5173，Vite 代理 API 到后端 :8080

## API 接口

### 认证
| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/auth/register` | 用户注册 |
| POST | `/api/v1/auth/login` | 用户登录 |

### Agent
| 方法 | 路径 | 说明 |
|------|------|------|
| GET/POST | `/api/v1/agents` | 列表 / 创建 |
| GET/PUT/DELETE | `/api/v1/agents/:id` | 获取 / 更新 / 删除 |

### 对话
| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/chat/completions` | 发送对话（支持 SSE 流式 + Tool Calling） |
| GET | `/api/v1/conversations` | 对话列表 |
| GET | `/api/v1/conversations/:id/messages` | 对话消息历史 |

### 模型配置
| 方法 | 路径 | 说明 |
|------|------|------|
| GET/POST | `/api/v1/models` | 列表 / 创建 |
| PUT/DELETE | `/api/v1/models/:id` | 更新 / 删除 |

### 工作流
| 方法 | 路径 | 说明 |
|------|------|------|
| GET/POST | `/api/v1/workflows` | 列表 / 创建 |
| GET/PUT/DELETE | `/api/v1/workflows/:id` | 获取 / 更新 / 删除 |
| POST | `/api/v1/workflows/:id/run` | 执行工作流 |

### 知识库 (RAG)
| 方法 | 路径 | 说明 |
|------|------|------|
| GET/POST | `/api/v1/knowledge-bases` | 列表 / 创建 |
| GET/DELETE | `/api/v1/knowledge-bases/:id` | 获取（含文档列表）/ 删除 |
| POST | `/api/v1/knowledge-bases/:id/documents` | 上传文件 |
| POST | `/api/v1/knowledge-bases/:id/documents/text` | 添加文本 |
| DELETE | `/api/v1/knowledge-bases/:id/documents/:doc_id` | 删除文档 |
| POST | `/api/v1/knowledge-bases/:id/search` | 语义搜索 |

### MCP
| 方法 | 路径 | 说明 |
|------|------|------|
| GET/POST | `/api/v1/mcp/servers` | 列表 / 添加 |
| DELETE | `/api/v1/mcp/servers/:id` | 删除 |
| POST | `/api/v1/mcp/servers/:id/test` | 测试连接 |

### Multi-Agent / 工具 / 仪表盘 / 设置
| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/multi-agent/run` | 多 Agent 协作运行 |
| GET | `/api/v1/tools` | 可用工具列表 |
| GET | `/api/v1/dashboard/stats` | 统计数据 |
| GET/PUT | `/api/v1/settings/profile` | 用户资料 |
| PUT | `/api/v1/settings/password` | 修改密码 |

## 支持的 LLM Provider

| Provider | 模型示例 |
|----------|---------|
| **OpenAI** | GPT-4o, GPT-4, o1, o3-mini |
| **Anthropic** | Claude 3.5 Sonnet, Claude 4 |
| **DeepSeek** | DeepSeek-V3, DeepSeek-R1 |
| **Ollama** | Llama 3, Qwen, Mistral（本地） |
| **OpenRouter** | 聚合网关，一个 Key 访问所有模型 |

## 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `STARCLAW_SERVER_PORT` | 8080 | 后端端口 |
| `STARCLAW_SERVER_MODE` | debug | 运行模式（debug/release） |
| `STARCLAW_DATABASE_HOST` | localhost | MySQL 地址 |
| `STARCLAW_DATABASE_PASSWORD` | starclaw | MySQL 密码 |
| `STARCLAW_REDIS_HOST` | localhost | Redis 地址 |
| `STARCLAW_JWT_SECRET` | (内置) | JWT 签名密钥（**生产必须修改**） |
| `STARCLAW_OPENAI_API_KEY` | (空) | OpenAI API Key（RAG 嵌入用） |

## 数据库表

| 表名 | 说明 |
|------|------|
| users | 用户账户 |
| agents | AI Agent 配置 |
| conversations | 对话会话 |
| messages | 对话消息 |
| model_configs | LLM 模型配置 |
| workflows | 工作流定义 |
| knowledge_bases | 知识库元数据 |
| documents | 上传的文档 |
| document_chunks | 文档分块 + 向量 |
| mcp_servers | MCP 工具服务器 |

## License

MIT
