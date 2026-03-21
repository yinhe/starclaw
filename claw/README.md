<h1 align="center">🦞 StarClaw</h1>

<p align="center">
  <strong>Open-Source AI Agent Orchestration Platform</strong><br>
  Multi-Model · Squad Engine · Visual Workflow · RAG · MCP Compatible · P2P Network
</p>

<p align="center">
  <a href="https://github.com/yinhe/starclaw/blob/main/LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="License" /></a>
  <a href="https://github.com/yinhe/starclaw/releases"><img src="https://img.shields.io/github/v/release/yinhe/starclaw?color=orange" alt="Release" /></a>
  <a href="https://github.com/yinhe/starclaw/actions"><img src="https://img.shields.io/github/actions/workflow/status/yinhe/starclaw/ci.yml?label=CI" alt="CI" /></a>
  <img src="https://img.shields.io/badge/Go-1.24-00ADD8?logo=go" alt="Go" />
  <img src="https://img.shields.io/badge/React-18-61DAFB?logo=react" alt="React" />
  <a href="https://github.com/yinhe/starclaw"><img src="https://img.shields.io/github/stars/yinhe/starclaw?style=social" alt="Stars" /></a>
</p>

<p align="center">
  <a href="https://starclaw.me">Website</a> ·
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
| **Squad Engine** | Multi-node mission orchestration — LLM auto-plans tasks, dispatches across nodes, collects results |
| **Code Review Loop** | Automated review with retry — reviewer Agent scores code, rejects < threshold, re-dispatches with feedback (max 3 retries) |
| **Agent Engine** | ReAct reasoning loop, Multi-Agent collaboration, autonomous sub-agent delegation |
| **Visual Workflow** | React Flow canvas with 5 node types: LLM / Condition / HTTP / Code / Merge |
| **RAG Knowledge Base** | Document upload → smart chunking → vector embedding → semantic retrieval |
| **Tool System** | Browser control, code execution, video/music/image generation, TTS, web search |
| **Multi-Model** | Qwen · OpenAI · DeepSeek · Anthropic · Ollama · OpenRouter |
| **Coding Agent** | Autonomous coding, file ops, sandbox for 13 languages |
| **MCP Compatible** | Connect external MCP tool servers to extend Agent capabilities |
| **P2P Network** | Ed25519 identity · Gossip protocol · DHT · NAT traversal · encrypted node-to-node communication |
| **BYOK** | Bring Your Own Key — use your own API keys, completely free |
| **Multimedia** | AI video + music + MV composition + TTS narration |
| **Swarm Network** | Distributed multi-node collaboration (optional, works perfectly standalone) |

## 🚀 Quick Start

### One-Click Install (Linux server)

```bash
curl -fsSL https://starclaw.me/install.sh | bash
```

### One-Click Install (China Mainland)

```bash
curl -fsSL https://starclaw.me/install-cn.sh | bash
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
│   │   ├── squad/          # Squad Engine (mission orchestration)
│   │   ├── tool/           # Tool system
│   │   ├── node/           # P2P node identity + Gossip
│   │   ├── security/       # AES-256-GCM encryption + Merkle audit chain
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
|-------|----------|
| **Frontend** | React 18 + Vite + TypeScript + TailwindCSS + Zustand + React Flow |
| **Backend** | Go 1.24 + Gin + GORM + Viper |
| **Database** | MySQL 8.0 + Redis 7 |
| **AI** | Qwen / OpenAI / DeepSeek / Anthropic / Ollama / OpenRouter + fal.ai |
| **P2P** | Ed25519 + Gossip v2 + DHT + NAT traversal |
| **Security** | AES-256-GCM encryption + Merkle-linked audit chain |
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

We welcome contributions of all kinds! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

- 🐛 [Report a bug](https://github.com/yinhe/starclaw/issues)
- 💡 [Suggest a feature](https://github.com/yinhe/starclaw/discussions)
- 🔧 [Submit a PR](https://github.com/yinhe/starclaw/pulls)

## 📄 License

[MIT](LICENSE)

---

<a id="中文"></a>

## 中文简介

> 每一个部署的 StarClaw 实例就是一只小龙虾（Claw）——灵感来自星际争霸虫族

StarClaw 是一个功能完整的**开源 AI Agent 编排平台**，支持：

- **Squad 引擎** — 多节点组队协作，LLM 自动拆解任务、跨节点分发、代码审查自动重试
- **Agent 引擎** — ReAct 推理、Multi-Agent 协作、自主委派子 Agent
- **可视化工作流** — React Flow 画布，5 种节点类型
- **RAG 知识库** — 文档分块 → 向量嵌入 → 语义检索
- **P2P 网络** — Ed25519 身份 + Gossip 协议 + 加密通信
- **Tool 系统** — 浏览器操控、代码沙箱、视频/音乐/图片生成
- **多模型** — Qwen / OpenAI / DeepSeek / Anthropic / Ollama
- **BYOK** — 自带 API Key，完全免费

### 一键部署

```bash
curl -fsSL https://starclaw.me/install.sh | bash
```

### 一键部署（中国大陆加速版）

```bash
curl -fsSL https://starclaw.me/install-cn.sh | bash
```

或手动安装：

```bash
git clone https://github.com/yinhe/starclaw.git
cd starclaw
cp .env.example .env
docker compose up -d
```

访问 `http://localhost` 开始使用。详细文档见 [docs/](docs/) 目录。
