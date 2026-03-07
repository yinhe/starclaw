<p align="center">
  <img src="docs/logo.png" alt="StarClaw" width="120" />
</p>

<h1 align="center">StarClaw 🦞</h1>

<p align="center">
  <strong>开源 AI Agent 编排平台</strong><br>
  多模型接入 · 可视化工作流 · RAG 知识库 · MCP 兼容 · Multi-Agent 协作
</p>

<p align="center">
  <a href="https://github.com/yinhe/starclaw/blob/main/LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="License" /></a>
  <a href="https://github.com/yinhe/starclaw"><img src="https://img.shields.io/github/stars/yinhe/starclaw?style=social" alt="Stars" /></a>
</p>

---

> 每一个部署的 StarClaw 实例就是一只小龙虾（Claw）——灵感来自星际争霸虫族

## ✨ 功能亮点

| 能力 | 说明 |
|------|------|
| **Agent 引擎** | ReAct 推理循环、Multi-Agent 协作、自主委派子 Agent |
| **工作流编排** | React Flow 可视化画布，LLM / 条件 / HTTP / 代码 / 合并 5 种节点 |
| **RAG 知识库** | 文档上传 → 智能分块 → 向量嵌入 → 语义检索 |
| **Tool 系统** | 浏览器操控、代码执行、视频/音乐/图片生成、配音、搜索 |
| **多模型接入** | Qwen · OpenAI · DeepSeek · Anthropic · Ollama · OpenRouter |
| **编程 Agent** | 自主编码、文件操作、13 种语言代码沙箱 |
| **MCP 兼容** | 连接外部 MCP 工具服务器，扩展 Agent 能力 |
| **BYOK** | Bring Your Own Key — 自带 API Key，完全免费使用 |
| **多媒体** | AI 视频生成 + 音乐生成 + MV 合成 + TTS 配音 |
| **虫群网络** | 多节点分布式协作（可选，独立运行完全没问题） |

## 🚀 快速开始

```bash
# 1. 克隆
git clone https://github.com/yinhe/starclaw.git
cd starclaw

# 2. 配置环境变量
cp .env.example .env
# 编辑 .env，设置 JWT_SECRET（必须）和 API Key（按需）

# 3. 启动
docker compose up -d
```

访问 **http://localhost** 开始使用。

首次注册的用户自动成为管理员。

## 📁 项目结构

```
starclaw/
├── api/                    # Go 后端（Gin + GORM）
│   ├── cmd/server/         # 入口
│   ├── configs/            # 配置文件
│   ├── internal/           # 业务逻辑
│   │   ├── agent/          # Agent 引擎（ReAct / Multi-Agent）
│   │   ├── api/v1/         # HTTP Handlers
│   │   ├── provider/       # LLM Provider 适配层
│   │   ├── rag/            # RAG Pipeline
│   │   ├── tool/           # Tool 系统
│   │   └── workflow/       # 工作流引擎
│   ├── plugins/            # JSON 工具插件
│   └── Dockerfile
├── web/                    # React 前端（Vite + TypeScript）
│   ├── src/
│   └── Dockerfile
├── deploy/                 # Nginx 配置
├── docs/                   # 文档
│   ├── README.md           # 详细项目概览
│   ├── DEPLOY.md           # 部署指南
│   └── API.md              # API 接口文档
├── docker-compose.yml      # 开发环境
├── docker-compose.prod.yml # 生产环境
├── .env.example            # 环境变量模板
└── LICENSE
```

## 🛠 技术栈

| 层级 | 技术 |
|------|------|
| **前端** | React 18 + Vite + TypeScript + TailwindCSS + Zustand + React Flow |
| **后端** | Go 1.24 + Gin + GORM + Viper |
| **数据库** | MySQL 8.0 + Redis 7 |
| **AI** | Qwen / OpenAI / DeepSeek / Anthropic / Ollama / OpenRouter + fal.ai |
| **多媒体** | FFmpeg + DashScope TTS + fal.ai（视频/音乐/图片） |
| **部署** | Docker Compose + Nginx |

## 🌐 虫群网络（可选）

每只小龙虾可以独立运行，也可以加入虫群网络获得集体增益：

```yaml
# api/configs/config.yaml
server:
  node_role: claw          # 默认
  queen_url: ""            # 留空 = 独立运行
  auto_update: true        # 自动接收版本更新
```

- **独立模式** — 不连接任何上级节点，所有功能完全正常
- **加入虫群** — 连接后获得共享知识、自动更新、任务分配
- **断网生存** — 网络断开后自动进入 Feral 模式，所有能力不受影响

## 📖 文档

- [详细项目概览](docs/README.md)
- [部署指南](docs/DEPLOY.md)
- [API 接口文档](docs/API.md)

## 🤝 贡献

欢迎提交 Issue 和 PR！

## 📄 License

[MIT](LICENSE)
