# Changelog

All notable changes to StarClaw will be documented in this file.

Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
versioning follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.5.0] - 2026-03-07

### Added
- **Agent Template Marketplace (Creep 菌毯)** — Browse, install, publish, and rate AI Agent templates. 8 built-in templates across 8 categories (coding, writing, data, creative, devops, research, business, assistant). Backend: `AgentTemplate` model, `/v1/templates` CRUD + install/rate/categories endpoints. Frontend: redesigned MarketplacePage with category filter, featured section, search, ratings, install counts.
- **Redis-Based Rate Limiting** — Upgraded rate limiter from in-memory to Redis-backed for distributed deployments. Falls back to in-memory when Redis unavailable. Per-IP (300/min global) and per-user (30/min chat) limits with `X-RateLimit-*` headers.
- **Makefile** — Full Docker lifecycle management: `make up/stop/restart/down/destroy/logs/ps/stats/health/backup/restore-db/shell-*/prune/init`. China mirror variant: `make up-cn/update-cn`.

### Changed
- **Dark mode polish** — Added `dark:` variants across Dashboard, Marketplace, Layout notifications, index.css utilities (input-sm, scrollbar). All key pages now properly support dark theme.
- All Docker services now share `starclaw` bridge network for inter-container DNS resolution
- Nginx upstream fixed: `backend` → `api` to match Docker Compose service name
- DEPLOY.md ops section rewritten with Makefile command reference

---

## [0.4.0] - 2026-03-07

### Added
- **WebSocket Real-Time Push** — `ws` package with Hub/Client pattern, `/v1/ws?token=` endpoint, auto-reconnect frontend client. Replaces polling for notifications, task updates, agent status.
- **Agent Share Links** — `POST /agents/:id/share` generates public URL, `GET /agents/shared/:id` returns agent JSON without auth. One-click share any agent.
- **ParseToken Helper** — Reusable JWT parsing in `middleware.ParseToken()`, used by WebSocket auth and future features.

### Changed
- Dockerfile `REGISTRY` build arg now correctly overrides ALL base images (`golang`, `alpine`, `node`, `nginx`) for China mirror
- `docker-compose.cn.yml` overrides MySQL/Redis images + build args for complete China mirror coverage

---

## [0.3.0] - 2026-03-07

### Added
- **A2A Protocol** — Google Agent-to-Agent protocol support: Agent Card at `/.well-known/agent.json`, JSON-RPC task endpoints (`tasks/send`, `tasks/get`, `tasks/cancel`, `tasks/sendSubscribe` with SSE streaming)
- **First-Run Onboarding Wizard** — Step-by-step guide for new users: select provider, enter API key, start chatting. Auto-detects when no models are configured.

### Changed
- Docker services renamed: `backend` → `api`, `frontend` → `web`
- All data volumes now use `./data/` bind mounts (mysql, redis, sandbox, media)
- Added `docker-compose.cn.yml` for China Docker mirror acceleration
- Install script auto-detects China network and configures Docker mirrors

---

## [0.2.0] - 2026-03-07

### Added
- **Google Gemini Provider** — Full support via OpenAI-compatible endpoint (Gemini 2.5 Pro/Flash, 2.0, 1.5)
- **OAuth Login** — GitHub and Google social login (auto-create account, link existing by email)
- **Molt Version Checker** — Background GitHub Release checker, `/v1/version` endpoint, frontend update banner
- **One-Click Install** — `curl | bash` install script for Linux servers

### Changed
- README now bilingual (English first, Chinese at bottom)
- Added English documentation (DEPLOY_EN.md, API_EN.md)

---

## [0.1.0] - 2026-03-07

### Added
- **Agent Engine** — ReAct reasoning loop, Multi-Agent collaboration, autonomous sub-agent delegation
- **Visual Workflow** — React Flow canvas with 5 node types (LLM / Condition / HTTP / Code / Merge)
- **RAG Knowledge Base** — Document upload, smart chunking, vector embedding, semantic retrieval
- **Tool System** — Browser control, code execution, video/music/image generation, TTS, web search
- **Multi-Model Support** — Qwen, OpenAI, DeepSeek, Anthropic, Ollama, OpenRouter
- **Coding Agent** — Autonomous coding, file operations, sandbox for 13 languages
- **MCP Compatible** — Connect external MCP tool servers
- **BYOK Mode** — Bring Your Own Key, completely free to use
- **Multimedia Pipeline** — AI video + music + MV composition + TTS narration
- **Swarm Network** — Optional distributed multi-node collaboration
- **Feral Mode** — Full offline capability when disconnected
- **i18n** — Chinese + English UI
- **Admin Panel** — User management, system stats, RBAC
- **Plugin SDK** — JSON-based tool plugin system
- **Workflow Marketplace** — Publish, browse, clone workflow templates
- **Long-term Memory** — Cross-session memory with importance scoring
