# StarClaw - 技术栈方案

## 一、总体架构选型

**架构风格：** 前后端分离 + 微服务（初期可单体，后期拆分）
**部署方式：** Docker Compose（开发 & 生产统一）

---

## 二、前端技术栈

| 层级 | 技术选型 | 理由 |
|------|---------|------|
| 框架 | **React 18+ (Vite)** | 轻量快速的 SPA 构建，HMR 极速，生态成熟 |
| 语言 | **TypeScript** | 类型安全，大型项目必备 |
| 路由 | **React Router v6** | React 生态最主流的路由方案 |
| UI 组件库 | **Ant Design 5** 或 **shadcn/ui + Radix UI** | 企业级组件丰富 / 高度可定制 |
| 样式 | **TailwindCSS** | 原子化 CSS，开发效率高 |
| 状态管理 | **Zustand** | 轻量、简洁，适合中大型应用 |
| 数据请求 | **TanStack Query (React Query)** | 服务端状态管理、缓存、自动重试 |
| 工作流编辑器 | **React Flow** | 成熟的 DAG 可视化拖拽库，Dify/LangFlow 同款 |
| 富文本/Markdown | **Tiptap** 或 **react-markdown** | 支持 Markdown 渲染和编辑 |
| 实时通信 | **SSE (Server-Sent Events)** | 流式输出 LLM 响应，比 WebSocket 更简单 |
| 代码高亮 | **Shiki** | 高质量语法高亮 |
| 图表 | **Recharts** | React 图表库，用于用量监控 |
| 图标 | **Lucide React** | 轻量图标库 |
| 国际化 | **react-i18next** | React 生态最流行的国际化方案 |

---

## 三、后端技术栈

| 层级 | 技术选型 | 理由 |
|------|---------|------|
| 语言 | **Go 1.22+** | 高性能、高并发、编译型语言，部署简单（单二进制） |
| Web 框架 | **Gin** | Go 生态最流行的高性能 HTTP 框架，中间件丰富 |
| ORM | **GORM** | Go 最成熟的 ORM，支持 MySQL、自动迁移 |
| 数据库迁移 | **golang-migrate** | 独立的迁移工具，版本化管理 |
| 任务队列 | **Asynq (Redis-based)** | Go 原生的异步任务队列，轻量高效 |
| 认证 | **JWT (golang-jwt)** + **OAuth2** | 成熟的 Go JWT 库 |
| API 文档 | **Swagger (swaggo/swag)** | 注解自动生成 OpenAPI 文档 |
| 配置管理 | **Viper** | Go 标准配置管理库，支持多格式 |
| 日志 | **Zap** 或 **Zerolog** | 高性能结构化日志 |
| 依赖注入 | **Wire (Google)** | 编译期依赖注入，无运行时开销 |

### Agent 核心引擎

| 组件 | 技术选型 | 理由 |
|------|---------|------|
| Agent 框架 | **自研 Agent Runtime** | 灵活控制，不被第三方框架锁定 |
| 参考方案 | 借鉴 **LangGraph** 的状态机思路 | 可控的 Agent 执行流程 |
| Tool Calling | **统一 Tool 接口协议** | 兼容 OpenAI Function Calling 格式 |
| MCP 支持 | **Model Context Protocol SDK** | 兼容 Anthropic MCP 标准 |
| 代码沙箱 | **Docker 容器隔离** 或 **E2B** | 安全执行用户代码 |
| 浏览器操作 | **Playwright** | Computer Use 场景 |

### 模型统一接入层

```go
// 核心设计：Provider 接口抽象层
type ModelProvider interface {
    // Chat 发送对话请求，支持流式输出
    Chat(ctx context.Context, req *ChatRequest) (<-chan *ChatChunk, error)
    // Embed 文本向量化
    Embed(ctx context.Context, texts []string) ([][]float64, error)
}

// 已规划的 Provider 注册表
var providerRegistry = map[string]ProviderFactory{
    "openai":     NewOpenAIProvider,     // GPT-4o, GPT-4, o1, o3
    "anthropic":  NewAnthropicProvider,  // Claude 3.5/4
    "google":     NewGoogleProvider,     // Gemini 2.0
    "deepseek":   NewDeepSeekProvider,   // DeepSeek-V3/R1
    "ollama":     NewOllamaProvider,     // 本地开源模型 (Llama, Qwen, etc.)
    "openrouter": NewOpenRouterProvider, // 聚合网关，一个 Key 访问所有模型
    "azure":      NewAzureProvider,      // Azure OpenAI
    "zhipu":      NewZhipuProvider,      // 智谱 GLM-4
    "moonshot":   NewMoonshotProvider,   // Kimi / Moonshot
    "custom":     NewOpenAICompatible,   // 任意 OpenAI 兼容 API
}
```

> **Go 调用 LLM 的方案：** 大多数 LLM 厂商都提供 REST API，Go 通过 `net/http` 原生调用即可。对于需要 Python 生态的 RAG 场景（文档解析、Embedding），可用独立的 Python 微服务处理，通过 gRPC/HTTP 与 Go 主服务通信。

---

## 四、数据存储

| 用途 | 技术选型 | 理由 |
|------|---------|------|
| 主数据库 | **MySQL 8.0+** | 成熟稳定，社区生态丰富，运维经验广泛 |
| 缓存 | **Redis 7** | 会话缓存、限流、Pub/Sub、任务队列 |
| 向量数据库 | **Milvus** 或 **Qdrant** | RAG 场景的向量检索 |
| 对象存储 | **MinIO** (自部署) / **S3** (云端) | 文件、文档、图片存储 |
| 消息队列 | **Redis Streams** 或 **RabbitMQ** | 异步任务、事件驱动 |
| 搜索引擎 | **Meilisearch** (可选) | 全文搜索（Agent 市场、文档搜索） |

---

## 五、RAG Pipeline

```
文档上传 → 格式解析 → 文本分块 → 向量化 → 存储
                                              ↓
用户查询 → Query 改写 → 向量检索 → 重排序 → 注入 Prompt → LLM 生成
```

| 环节 | 技术选型 |
|------|---------|
| 文档解析 | **Python 微服务** (Unstructured / PyMuPDF) 或 **Go: goldmark / unioffice** |
| 文本分块 | 自研（支持按段落/语义/Token 分块） |
| Embedding | 多模型支持（OpenAI text-embedding-3 / BGE / Jina），通过 HTTP API 调用 |
| 向量存储 | Milvus（推荐） / Qdrant |
| 重排序 | **Cohere Rerank** / **BGE Reranker**（通过 API 调用） |

---

## 六、基础设施 & DevOps

| 层级 | 技术选型 |
|------|---------|
| 容器化 & 编排 | **Docker + Docker Compose** |
| CI/CD | **GitHub Actions** |
| 监控 | **Prometheus + Grafana** |
| 日志 | **Loki** 或 **ELK** |
| 链路追踪 | **LangSmith** 或 **Langfuse** (LLM 专用追踪) |
| 反向代理 | **Nginx** 或 **Traefik** |

---

## 七、实际项目目录结构

> 详见 [ARCHITECTURE.md](./ARCHITECTURE.md) 获取完整架构说明（含蜂群架构、开源策略等）

```
starclaw/                          # 私有 Monorepo
│
├── api/                           # Go API 后端（原 backend/）
│   ├── cmd/server/main.go         # 入口（优雅停机）
│   ├── configs/config.yaml        # Viper 配置文件
│   ├── internal/
│   │   ├── api/v1/               # API Handlers
│   │   ├── agent/                # Agent 引擎（runtime/react/multi）
│   │   ├── browser/              # 无头浏览器（Computer Use）
│   │   ├── config/               # 配置结构体
│   │   ├── database/             # 数据库迁移 & Seed
│   │   ├── middleware/           # 认证/限流/RBAC/日志
│   │   ├── model/                # GORM 数据模型
│   │   ├── provider/             # LLM Provider（Qwen/OpenAI/DeepSeek/Ollama…）
│   │   ├── rag/                  # RAG Pipeline（分块/嵌入/检索）
│   │   ├── router/               # 路由注册（70+ 端点）
│   │   ├── sandbox/              # 代码沙箱
│   │   ├── tool/                 # Tool 系统（浏览器/代码/视频/音乐/配音…）
│   │   ├── worker/               # 异步任务 Worker
│   │   └── workflow/             # 工作流引擎
│   ├── plugins/                  # JSON 工具插件
│   ├── go.mod                    # module github.com/yinhe/starclaw
│   └── Dockerfile
│
├── web/                           # React 前端（原 frontend/）
│   ├── src/
│   │   ├── pages/                # 页面（Chat/Agents/Workflow/Coding/Videos…）
│   │   ├── components/           # 公共组件（Layout/CommandPalette/…）
│   │   ├── lib/                  # API 层 & i18n
│   │   └── stores/               # Zustand 状态管理
│   ├── Dockerfile / nginx.conf
│   └── package.json              # name: starclaw-web
│
├── mobile/                        # Flutter 移动端
│   ├── lib/
│   │   ├── screens/ / services/
│   │   └── main.dart
│   └── pubspec.yaml
│
├── site/                          # 官网落地页（原 website/，闭源）
├── docs/                          # 技术文档
│   ├── ARCHITECTURE.md            # 系统架构（蜂群/开源/计费）
│   ├── PRODUCT_PLAN.md
│   ├── TECH_STACK.md
│   └── DEPLOY.md
├── deploy/                        # Nginx 等部署配置
├── scripts/                       # 工具脚本（sync-oss.sh 等）
│
├── docker-compose.yml             # MySQL + Redis + api + web
├── docker-compose.prod.yml        # 生产环境
├── .env.example
└── README.md
```

---

## 八、关键技术决策总结

| 决策项 | 选择 | 备选方案 |
|--------|------|---------|
| 前端框架 | React + Vite | Next.js, Vue + Nuxt |
| 后端语言 | Go + Gin | Python (FastAPI), Node.js |
| 数据库 | MySQL 8.0 | PostgreSQL, TiDB |
| Agent 框架 | 自研 (Go) | LangChain (需 Python sidecar) |
| 向量库 | Milvus | Qdrant, Weaviate, Pinecone |
| 工作流 UI | React Flow | X6 (蚂蚁), JointJS |
| 部署 | Docker Compose | Kubernetes, Vercel + Railway |

### 为什么 Agent 框架选择自研？

1. **灵活性** - LangChain 等框架抽象过重，遇到复杂场景难以定制
2. **性能** - Go 的 goroutine 天然适合并发 Agent 执行，内存占用低
3. **可控性** - 核心逻辑不依赖第三方，升级迭代自主可控
4. **差异化** - 自研才能做出真正的产品差异
5. **部署简单** - Go 编译为单二进制文件，无运行时依赖

### 为什么用 Go 而不是 Python？

| 对比项 | Go | Python |
|--------|----|---------|
| 并发模型 | goroutine（轻量、高效） | asyncio（复杂、GIL 限制） |
| 性能 | 编译型，接近 C | 解释型，慢 10-100x |
| 部署 | 单二进制，无依赖 | 需要虚拟环境 + 依赖安装 |
| 内存 | 低占用 | 高占用 |
| LLM 调用 | HTTP API 调用，无差别 | 有更多 SDK，但非必须 |
| AI 生态 | 较弱（RAG 可用 Python 微服务补充） | 最强 |

> **折中方案：** 主服务用 Go（API、Agent Runtime、工作流引擎），RAG 文档处理用独立的 Python 微服务（通过 gRPC 通信），两全其美。

### 初期可以借鉴的核心概念

- **LangGraph** 的状态图 (StateGraph) 执行模型
- **OpenAI Assistants API** 的 Tool/Thread/Run 抽象
- **Anthropic MCP** 的工具协议标准
- **Dify** 的工作流节点设计

---

## 九、MVP 开发排期建议

| 周次 | 任务 | 交付物 |
|------|------|--------|
| W1 | 项目初始化、数据库设计、模型接入层 | 可调用多模型的 API |
| W2 | Agent Runtime 核心 + 基础 Tools | 命令行可运行的 Agent |
| W3 | Chat API (SSE 流式) + 前端 Chat UI | 可对话的 Web 界面 |
| W4 | Agent 配置管理 + 用户系统 | 可创建/管理 Agent |
| W5 | Tool Calling 完善 + 代码沙箱 | Agent 可调用工具 |
| W6 | Memory 系统 + 对话历史 | 有记忆的多轮对话 |
| W7 | UI 打磨 + 文档 + Docker 部署 | 可 docker-compose up 的 MVP |
| W8 | 测试 + Bug 修复 + 开源发布 | GitHub 首个 Release |

---

## 十、商业模式（可选）

1. **开源免费** - 社区版完全开源，吸引开发者
2. **云端 SaaS** - 托管版本按用量收费（Token 消耗 + 存储）
3. **企业版** - 私有化部署 + 高级功能（SSO、审计、SLA）
4. **市场抽成** - Agent/Plugin 市场交易分成
