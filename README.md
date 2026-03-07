<h1 align="center">🦞 StarClaw</h1>

<p align="center">
  <strong>Open-Source AI Agent Orchestration Platform</strong><br>
  Multi-Model · Visual Workflow · RAG · MCP Compatible · Multi-Agent Collaboration
</p>

<p align="center">
  <a href="https://github.com/yinhe/starclaw/blob/main/LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="License" /></a>
  <a href="https://github.com/yinhe/starclaw"><img src="https://img.shields.io/github/stars/yinhe/starclaw?style=social" alt="Stars" /></a>
</p>

<p align="center">
  <a href="#-quick-start">Quick Start</a> ·
  <a href="docs/DEPLOY_EN.md">Deploy Guide</a> ·
  <a href="docs/API_EN.md">API Docs</a> ·
  <a href="#中文">中文文档</a>
</p>

---

> Every deployed StarClaw instance is a Claw 🦞 — inspired by the StarCraft Zerg swarm

## ✨ Features

| Feature | Description |
|---------|-------------|
| **Agent Engine** | ReAct reasoning loop, Multi-Agent collaboration, autonomous sub-agent delegation |
| **Visual Workflow** | React Flow canvas with 5 node types: LLM / Condition / HTTP / Code / Merge |
| **RAG Knowledge Base** | Document upload → smart chunking → vector embedding → semantic retrieval |
| **Tool System** | Browser control, code execution, video/music/image generation, TTS, web search |
| **Multi-Model** | Qwen · OpenAI · DeepSeek · Anthropic · Ollama · OpenRouter |
| **Coding Agent** | Autonomous coding, file ops, sandbox for 13 languages |
| **MCP Compatible** | Connect external MCP tool servers to extend Agent capabilities |
| **BYOK** | Bring Your Own Key — use your own API keys, completely free |
| **Multimedia** | AI video + music + MV composition + TTS narration |
| **Swarm Network** | Distributed multi-node collaboration (optional, works perfectly standalone) |

## 🚀 Quick Start

### One-Click Install (Linux server)

```bash
curl -fsSL https://raw.githubusercontent.com/yinhe/starclaw/main/scripts/install.sh | bash
```

This will automatically install Docker (if needed), clone the repo, generate secure config, and start all services.

### Manual Install

```bash
# 1. Clone
git clone https://github.com/yinhe/starclaw.git
cd starclaw

# 2. Configure
cp .env.example .env
# Edit .env: set JWT_SECRET (required) and API keys (as needed)

# 3. Launch
docker compose up -d
```

Visit **http://localhost** (or your server IP) to get started.

The first registered user automatically becomes admin.

## 📁 Project Structure

```
starclaw/
├── api/                    # Go backend (Gin + GORM)
│   ├── cmd/server/         # Entrypoint
│   ├── configs/            # Configuration
│   ├── internal/           # Business logic
│   │   ├── agent/          # Agent engine (ReAct / Multi-Agent)
│   │   ├── api/v1/         # HTTP Handlers
│   │   ├── provider/       # LLM Provider adapters
│   │   ├── rag/            # RAG Pipeline
│   │   ├── tool/           # Tool system
│   │   └── workflow/       # Workflow engine
│   ├── plugins/            # JSON tool plugins
│   └── Dockerfile
├── web/                    # React frontend (Vite + TypeScript)
│   ├── src/
│   └── Dockerfile
├── deploy/                 # Nginx config
├── docs/                   # Documentation
├── docker-compose.yml      # Development
├── docker-compose.prod.yml # Production
├── .env.example            # Environment template
└── LICENSE
```

## 🛠 Tech Stack

| Layer | Technology |
|-------|-----------|
| **Frontend** | React 18 + Vite + TypeScript + TailwindCSS + Zustand + React Flow |
| **Backend** | Go 1.24 + Gin + GORM + Viper |
| **Database** | MySQL 8.0 + Redis 7 |
| **AI** | Qwen / OpenAI / DeepSeek / Anthropic / Ollama / OpenRouter + fal.ai |
| **Multimedia** | FFmpeg + DashScope TTS + fal.ai (video/music/image) |
| **Deploy** | Docker Compose + Nginx |

## 🌐 Swarm Network (Optional)

Each Claw can run independently or join the swarm for collective benefits:

```yaml
# api/configs/config.yaml
server:
  node_role: claw          # default
  queen_url: ""            # empty = standalone
  auto_update: true        # auto-receive version updates
```

- **Standalone** — all features work without connecting to any upstream node
- **Join Swarm** — shared knowledge, auto-updates, task distribution
- **Feral Mode** — network disconnected? all capabilities remain unaffected

## 📖 Documentation

**English:**
- [Deploy Guide](docs/DEPLOY_EN.md)
- [API Reference](docs/API_EN.md)

**中文:**
- [部署指南](docs/DEPLOY.md)
- [API 接口文档](docs/API.md)
- [详细项目概览](docs/README.md)

## 🤝 Contributing

Issues and PRs are welcome!

## 📄 License

[MIT](LICENSE)

---

<a id="中文"></a>

## 中文简介

> 每一个部署的 StarClaw 实例就是一只小龙虾（Claw）——灵感来自星际争霸虫族

StarClaw 是一个功能完整的**开源 AI Agent 编排平台**，支持：

- **Agent 引擎** — ReAct 推理、Multi-Agent 协作、自主委派子 Agent
- **可视化工作流** — React Flow 画布，5 种节点类型
- **RAG 知识库** — 文档分块 → 向量嵌入 → 语义检索
- **Tool 系统** — 浏览器操控、代码沙箱、视频/音乐/图片生成
- **多模型** — Qwen / OpenAI / DeepSeek / Anthropic / Ollama
- **BYOK** — 自带 API Key，完全免费

### 一键部署

```bash
curl -fsSL https://raw.githubusercontent.com/yinhe/starclaw/main/scripts/install.sh | bash
```

或手动安装：

```bash
git clone https://github.com/yinhe/starclaw.git
cd starclaw
cp .env.example .env
docker compose up -d
```

访问 `http://localhost` 开始使用。详细文档见 [docs/](docs/) 目录。
