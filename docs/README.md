# StarClaw 🦞 — 开源 AI Agent 编排平台

> 每一个部署的 StarClaw 实例就是一只小龙虾（Claw）

StarClaw 是一个功能完整的 AI Agent 编排平台，支持多模型接入、工作流编排、
RAG 知识库、Tool Calling、代码沙箱、视频/音乐/图片生成等能力。

## 核心能力

| 能力 | 说明 |
|------|------|
| **Agent 引擎** | ReAct / Multi-Agent / 自定义 Runtime |
| **工作流编排** | 可视化 DAG 工作流（React Flow） |
| **RAG 知识库** | 文档分块 → 嵌入 → 向量检索 |
| **Tool 系统** | 浏览器操控、代码执行、视频/音乐/图片生成、配音、网页搜索 |
| **多模型接入** | Qwen / OpenAI / DeepSeek / Anthropic / Ollama / OpenRouter |
| **代码沙箱** | Python / JavaScript / Go / Bash 安全执行 |
| **MCP 兼容** | Model Context Protocol 工具协议 |
| **BYOK** | Bring Your Own Key，自带 API Key 完全免费 |

## 目录结构

```
claw/
├── api/                # Go 后端 API 服务
│   ├── cmd/server/     # 入口
│   ├── configs/        # 配置文件
│   ├── internal/       # 业务逻辑
│   │   ├── agent/      # Agent 引擎
│   │   ├── api/v1/     # HTTP Handlers
│   │   ├── browser/    # 无头浏览器
│   │   ├── provider/   # LLM Provider
│   │   ├── rag/        # RAG Pipeline
│   │   ├── tool/       # Tool 系统
│   │   ├── worker/     # 异步任务
│   │   └── workflow/   # 工作流引擎
│   ├── plugins/        # JSON 工具插件
│   ├── Dockerfile
│   └── go.mod
├── web/                # React 前端
│   ├── src/
│   ├── Dockerfile
│   └── nginx.conf
├── data/               # 运行时数据（视频/图片/音乐/工作区）
├── deploy/             # 部署配置
├── scripts/            # 工具脚本
└── docs/               # ← 本目录
    ├── README.md       # 项目概览（本文件）
    ├── API.md          # API 接口文档（核心）
    ├── API_P8_P10.md   # API 接口文档（P8-P10 扩展，97 个新接口）
    ├── USER_GUIDE.md   # 用户手册
    ├── DEPLOY.md       # 部署指南
    └── SKILL_DEVELOPMENT.md  # 技能开发指南
```

## 快速开始

详见 [DEPLOY.md](./DEPLOY.md)

```bash
# 1. 克隆
git clone https://github.com/yinhe/starclaw.git
cd starclaw

# 2. 配置
cp .env.example .env
nano .env  # 设置 JWT_SECRET、DB 密码等

# 3. 启动
docker compose up -d
```

访问 `http://localhost` 即可使用。

## 虫群网络（可选）

每只小龙虾可以独立运行，也可以加入虫群网络获得集体增益：

```yaml
# configs/config.yaml
server:
  node_role: claw              # claw（默认）
  queen_url: ""                # 留空 = 独立运行，填写 = 加入虫群
  auto_update: true            # 自动接收版本更新（Molt 蜕皮）
```

- **独立模式**：不连接任何上级节点，所有功能正常
- **加入虫群**：连接到管理节点，获得共享知识、自动更新、任务分配
- **企业管理**：升级为管理节点后可管辖其他 Claw 节点（需企业订阅）

## 技术栈

| 层级 | 技术 |
|------|------|
| **前端** | React 18 + Vite + TypeScript + TailwindCSS + Zustand |
| **后端** | Go 1.24 + Gin + GORM + Viper |
| **数据库** | MySQL 8.0 + Redis 7 |
| **AI** | Qwen / OpenAI / DeepSeek / Anthropic / Ollama |
| **部署** | Docker Compose |

## License

MIT
