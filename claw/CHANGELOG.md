# Changelog

All notable changes to StarClaw will be documented in this file.

Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
versioning follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [2026.0313.1806] - 2026-03-13

### Added
- **Ed25519 signature authentication middleware** — `NodeSignatureAuth()` Gin middleware verifies inter-node requests via `X-Node-ID`, `X-Node-PubKey`, `X-Node-Signature`, `X-Node-Timestamp` headers. Protocol: `sign(METHOD\nPATH\nTIMESTAMP\nBODY_SHA256)` with 300-second anti-replay window. Includes `SignRequest()` client helper.
- **Inference router** — New `ContributorRegistry` tracks compute contributor nodes, supported models, GPU capacity, and load. `SelectContributor` routes requests by model match, load ratio, and latency. Background reaper marks contributors offline after 120s. `InferenceRouter` forwards requests to best contributor with Ed25519 signed headers and supports SSE stream proxying.
- **Inference API endpoints** — `POST /v1/inference/register`, `/heartbeat`, `/unregister` (contributor-side, signature-protected); `POST /v1/inference/completions`, `GET /v1/inference/contributors` (user-side, JWT-protected); `GET /v1/inference/status` (public); `POST /v1/inference/execute` (contributor execution endpoint, signature-protected).
- **Gossip v2 protocol** — Header-based Ed25519 signed gossip at `POST /v1/peer/v2/gossip` and `POST /v1/peer/v2/relay`. Falls back to v1 body-signed protocol for backwards compatibility with older nodes.
- **Agent step progress in chat** — Team agent conversations now stream `agent_step` SSE events showing real-time progress (thinking, summarizing) instead of a blank spinner. Frontend displays step name, detail text, and step counter.
- **HD wallet system** — BIP-39 24-word mnemonic, SLIP-0010 HD key derivation for Ed25519 (`m/44'/9001'/account'/change'/index'`). Cold/hot wallet separation. Multi-signature primitives (m-of-n approval).
- **Wallet page** — Frontend wallet management with HP (health points) visualization bar showing Star Credits balance status.
- **HP bar in sidebar** — Clickable HP status indicator in navigation sidebar.
- **Star Credits balance monitoring** — Swarm heartbeat integration for real-time balance updates.
- **Device approval system** — New devices require owner approval via CLI or Web before accessing the instance.
- **CLI subcommands** — `export-key`, `import-key` (identity backup/restore), `get-token`, `reset-token`, `reset-password`, `version`. Host CLI wrapper (`starclaw`/`claw`) with Makefile targets.
- **Messaging integrations** — DingTalk, WeCom, Slack, Discord, and Telegram tool support for agent-driven messaging.

### Changed
- **3-command quick start** — `make up` now auto-creates `.env` from `.env.example` and all `data/` subdirectories. Deployment simplified to `git clone` → `cd starclaw` → `make up`.
- **Gossip engine** — Tries v2 header-signed endpoint first, falls back to v1 body-signed for backwards compatibility.

### Fixed
- **Docker port conflict** — MySQL and Redis no longer expose ports to host by default, preventing conflicts with existing services on port 3306/6379.
- **Deploy stability** — Identity persistence via volume mount, correct nginx port proxy, proper `.env` variable names, safe `make update` target.
- **CLI help** — `starclaw`/`claw` called without args now shows help instead of crashing.

---

## [2026.0310.1838] - 2026-03-11

### Added
- **`/model` slash command** — Type `/model` in chat to quickly switch between configured models without leaving the conversation.
- **No-model-configured guide** — When no models are set up, chat shows a helpful setup link instead of failing silently.
- **WebSocket Nginx proxy** — Production Nginx config now includes WebSocket upgrade headers for real-time push.
- **Auth Token localStorage sync** — Owner token persists correctly across browser sessions.

### Fixed
- **MiniMax model not responding** — Auto-select first provider model when `model_name` is empty or `"default"`. Providers that don't accept generic model names (e.g., MiniMax) now work correctly out of the box.
- **SSE errors swallowed in chat** — LLM/provider errors are now displayed as visible messages in the chat instead of silently hanging. Empty responses show a configuration check prompt.
- **Profile update silent failure** — `UpdateProfile` returns proper HTTP errors (409 for duplicate, 500 for DB errors) instead of always returning 200.
- **Login page shows error instead of setup** — `LoginPage` now checks `setup_completed` and redirects to `/setup` when no owner exists, instead of showing "未找到 Owner 用户".
- **Version always showing "vdev"** — Dockerfile reads `.version` file from build context as primary source. Docker Compose defaults changed from `"dev"` to empty string. `.version` committed to repo and auto-updated by `make tag`.
- **Owner Token login** — `tokenLogin` endpoint now accepts owner tokens in addition to API tokens. `claw reset-token` CLI hint added for token recovery.

### Changed
- Makefile version detection prefers git tags over timestamp generation.
- `make tag` automatically updates `api/.version` and commits before tagging.
- Docker Compose `BUILD_VERSION` defaults to empty string (triggers Dockerfile auto-detect) instead of `"dev"`.

---

## [2026.0310.1430] - 2026-03-10

### Added
- **Single-user Owner mode** — Each Claw instance has exactly one owner. First visit shows a setup page to initialize. Generates a permanent Owner Token (`claw_` + 32 hex chars) stored in browser for automatic login. No registration required in opensource mode.
- **Setup API** — `GET /v1/setup/status` returns setup state and deploy mode. `POST /v1/setup` performs one-time initialization (creates owner user, generates token). Handles both fresh install and upgrade from existing multi-user data.
- **Owner Token authentication** — Auth middleware accepts both JWT and Owner Token (`claw_` prefix) via `ResolveToken()` helper. Owner tokens are permanent (no expiry), validated via DB lookup. WebSocket auth also supports owner tokens.
- **Password recovery login** — `POST /v1/auth/owner-login` allows the owner to recover their token using a password (if set during setup). For instances without a password, CLI reset is available.
- **Setup page** — New frontend page with two-step flow: initialization form (optional password/username) → token display with copy button and backup reminder.
- **Date-based versioning** — Replaced SemVer with `YYYY.MMDD.HHmm` (UTC) timestamp format. Version injected at build time via `-ldflags` (no more hardcoded constants). `make tag` generates and pushes version tag automatically.
- **Cross-platform release binaries** — CI workflow builds StarClaw API + MCP Bridge for 5 targets: linux-amd64, linux-arm64, darwin-amd64, darwin-arm64, windows-amd64. All 10 binaries uploaded as GitHub Release assets.
- **Prometheus metrics** — `/metrics` endpoint exposing `http_requests_total`, `http_request_duration_seconds`, `http_requests_in_flight`, `websocket_connections_active`.
- **Release guide** — `docs/RELEASE_GUIDE.md` documenting version format, release commands, CI pipeline, GitHub Release rules, and dual-repo sync workflow.
- **GitHub Actions CI** — `ci.yml` (lint + build on PR/push) and `release.yml` (build + push Docker images + binaries on tag).

### Changed
- Auth middleware signature updated from `AuthRequired(cfg)` to `AuthRequired(cfg, db)` to support owner token DB lookup
- Login page detects deploy mode: opensource shows simplified password-only recovery, hosted preserves full email/phone/OAuth login
- `PrivateRoute` checks setup status before routing — redirects to `/setup` if not initialized
- 401 interceptor skips redirect when already on `/setup` or `/login` page
- `molt.Version` changed from `const` to `var` for ldflags injection, with `dev` default for local builds
- `mcp-bridge` version also injectable via ldflags
- All Docker Compose files (`docker-compose.yml`, `docker-compose.prod.yml`, `docker-compose.cn.yml`) pass `BUILD_VERSION` build arg
- Makefile `up`/`up-cn` targets pass `BUILD_VERSION` to Docker Compose
- Dockerfile accepts `BUILD_VERSION` ARG and injects into Go binary

### Fixed
- Auth handler context key bug — `c.Get("userID")` corrected to `c.Get("user_id")` in `GetAPIToken`, `RegenerateToken`, `ListDevices`, `RevokeDevice` (previously would panic when called)

---

## [0.5.11] - 2026-03-09

### Fixed
- **一键更新 git 脏目录修复** — `git pull` 改为 `git fetch + git reset --hard`，解决 tar 部署后 working tree 脏导致 pull 失败的问题。

---

## [0.5.10] - 2026-03-09

### Fixed
- **一键更新测试版本** — 验证 v0.5.9 → v0.5.10 一键更新流程。

---

## [0.5.9] - 2026-03-09

### Fixed
- **统一 Docker Compose 服务命名** — 所有 compose 文件（含根目录 `docker-compose.prod.yml`）服务名统一为 `api`/`web`，移除旧的 `backend`/`frontend` 命名。一键更新逻辑简化，不再需要动态检测服务名。

---

## [0.5.8] - 2026-03-09

### Fixed
- **一键更新 monorepo 兼容** — 支持 monorepo 布局（git 在 `claw/` 子目录），自动检测 `claw/.git` 并在正确目录执行 `git pull`。

---

## [0.5.7] - 2026-03-09

### Fixed
- **一键更新完全重写** — 修复 4 个导致更新永远无法完成的 bug：
  1. **JSON 转义错误** — Shell 命令中的双引号破坏 MCP Bridge JSON 参数，导致命令为空。改用 `json.Marshal` 正确序列化。
  2. **git pull 失败** — 服务器通过 tar 部署（无 `.git` 目录），`git pull` 必定报错。现在自动检测：有 git 则 pull，无 git 则跳过并提示。
  3. **错误的 compose 文件和服务名** — 硬编码 `docker compose build api web`，但生产环境使用 `-f docker-compose.prod.yml` 且服务名为 `backend/frontend`。现在自动检测 compose 文件和服务名。
  4. **容器内 fallback 无效** — 容器内无 docker CLI，fallback 策略只会自杀不会更新。已移除，改为前置检查 MCP Bridge 可用性。
- **更新前置检查** — 点击一键更新时先检查 MCP Bridge 是否运行，不可用则立即返回错误提示（而非等 15 分钟超时）。
- **前端错误提示** — 更新失败时显示后端返回的具体错误信息（如"MCP Bridge 未运行"）。

---

## [0.5.6] - 2026-03-08

### Fixed
- **MCP Bridge 下载 404** — 桥接下载链接从 `releases/latest/download` 改为使用固定版本号 `releases/download/v{BridgeVersion}/...`，避免新版本发布后因 release 未附带二进制而导致 404。新增 `BridgeVersion` 常量，桥接版本与应用版本解耦。
- **MCP Bridge 平台检测错误** — 修复前端用浏览器 `navigator.userAgent` 检测平台导致显示错误下载版本的问题。改为由后端返回 `host_os`/`host_arch`（服务器宿主机的真实操作系统），前端据此推荐正确的下载版本。

---

## [0.5.5] - 2026-03-08

### Fixed
- **One-click update timeout** — Frontend polling increased from 5 min to 15 min to accommodate full Docker build cycles. Progress bar timing adjusted to match realistic build durations (~3-5 min).
- **Update command improvements** — Auto-detects project directory on host (no longer hardcoded `/opt/starclaw/claw`). Split `docker compose up --build` into separate `build` + `up --no-deps` to avoid rebuilding mysql/redis. Only rebuilds `api` and `web` services.
- **Update timeout message** — Now suggests "构建可能仍在进行中，请稍后刷新页面" instead of generic error.

---

## [0.5.4] - 2026-03-08

### Added
- **Grok (xAI) Provider** — New model provider for xAI's Grok-3/Grok-2 family. OpenAI-compatible API at `api.x.ai/v1`. Backend factory supports both `grok` and `xai` names. Added to Models page, Onboarding wizard.
- **Grok-style Chat Input** — Redesigned input bar as unified pill shape: paperclip attach on left, textarea center, send + mic buttons on right. Clean, modern UI inspired by Grok.
- **Smart Voice Input (STT)** — Backend STT now auto-selects provider from DB config (priority: Qwen → OpenAI → DeepSeek → any). Qwen uses `whisper-large-v3` model via DashScope. Voice recording auto-stops after 2s silence — no manual stop button needed.
- **Video Gallery Tabs** — Split gallery into "合成视频" and "视频片段" tabs with count badges and status indicators.
- **Smart Video Merge** — `ffmpegMergeClips` auto-detects clip resolutions via `ffprobe`. Same resolution uses fast concat; mixed resolutions auto-normalize with scale+pad (letterbox/pillarbox) to majority resolution.

### Fixed
- **Nginx proxy for media files** — Added `/api/v1/` location block with rewrite to strip `/api` prefix. Fixes video/image/music files not loading in gallery and resource center (URLs stored as `/api/v1/...` but nginx only proxied `/v1/`).
- **STT API 404** — Backend STT was hardcoded to `cfg.OpenAI.APIKey` from config file. Now queries DB-configured providers dynamically. Fixed Qwen STT using wrong model name (`whisper-1` → `whisper-large-v3`).
- **Video merge crash on mixed resolutions** — Clips with different aspect ratios (e.g., 1280×720 + 720×1280) no longer produce corrupt output. Auto-normalized before concat.
- **Chat input alignment** — Fixed vertical misalignment between attach button and textarea.

### Changed
- Collapsed 4 chat input buttons (file, image, KB, voice) into single paperclip attach menu
- Textarea auto-expands vertically with content (max 120px), supports manual resize-y
- Removed Web Speech API dependency for voice input — backend STT is now the sole reliable path
- Removed recording overlay and manual stop button — silence detection handles stop automatically

---

## [0.5.3] - 2026-03-08

### Added
- **Overlord Monitoring** — Config, background heartbeat client, API endpoints (`/v1/overlord/status`, `/v1/overlord/stats`), Settings UI card for connecting Claw to an Overlord management node.
- **Remote Update via MCP Bridge** — One-click update from Settings page using MCP Bridge host shell. 4-step progress bar (pull/build/restart/verify), frontend polls version endpoint until complete.

### Fixed
- MCP tool classification: `mcp_*` tools now correctly show as MCP type in summary counts
- API Keys page shows green active badge instead of masked key value
- MySQL `ONLY_FULL_GROUP_BY` error on model config listing — removed `Group(provider)`, use code-level dedup instead
- API Keys display uses custom response struct (APIKey model has `json:"-"` tag)

### Changed
- Renamed "机器人竞技场" → "机器人社区" for clearer community positioning
- Renamed Overlord section to "Brood Network" with enterprise-oriented UX copy
- Swarm description updated to emphasize ecosystem benefits
- Translated Overlord UI labels to Chinese, show built-in MCP Bridge on MCP page
- Removed multi-agent and coding-agent from sidebar (integrated into SuperAgent)

---

## [0.5.2] - 2026-03-07

### Added
- **MCP Bridge** — One-click download, bridge status API, Settings UI card. Auto-detection of running bridge, systemd service template, Makefile targets (`make bridge-install/bridge-start/bridge-stop`), Docker `extra_hosts` for host network access.

### Changed
- Docker Compose uses `DB_PASSWORD` env var, updated `.env.example`
- Web port configurable via `WEB_PORT` env var
- Named volumes (`starclaw_mysql`, `starclaw_redis`, `starclaw_sandbox`) to preserve data across rebuilds

---

## [0.5.1] - 2026-03-07

### Fixed
- Bug fixes and stability improvements

---

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
