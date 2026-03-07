# Changelog

All notable changes to StarClaw will be documented in this file.

Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
versioning follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
