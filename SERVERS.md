# StarClaw 服务器架构

## 服务器列表

| 服务器 | 域名 | IP | 角色 | SSH |
|--------|------|-----|------|-----|
| A | starclaw.me | (域名直连) | Claw — 开源官网 + app + api | `ssh -i ~/.ssh/claw_deploy root@starclaw.me` |
| B | star-ai.net | 47.103.51.32 | Router — AI 算力平台 (国内) | `ssh -i ~/.ssh/starai_deploy root@47.103.51.32` |
| C | starclaw.net | 43.106.158.26 | Queen + Nydus + Proxy — 中央控制 (新加坡) | `ssh -i ~/.ssh/queen_deploy root@43.106.158.26` |

## 架构全景

```
用户
 │
 ├── starclaw.me ─────────── Server A (Claw)
 │    ├── app.starclaw.me    → claw-web (:3000)
 │    └── api.starclaw.me    → claw-api (:8080)
 │
 ├── star-ai.net ─────────── Server B (Router)
 │    ├── star-ai.net        → star-ai-web (:3096)
 │    ├── api.star-ai.net    → star-ai-api (:8096)
 │    ├── star-ai.net/v1/*   → star-ai-gateway (:8085)  ← API Gateway
 │    └── core.star-ai.net   → star-ai-core (:3097)
 │
 └── starclaw.net ─────────── Server C (Queen + Nydus + Proxy)
      ├── starclaw.net         → queen-web (:8086)
      ├── api.starclaw.net     → queen-api (:8085)
      ├── swarm.starclaw.net   → swarm (:8090)
      ├── core.starclaw.net    → core (:8091)
      ├── bounty.starclaw.net  → bounty (:8092)
      ├── forum.starclaw.net   → forum (:8093)
      ├── arena.starclaw.net   → arena (:8094)
      ├── overseer.starclaw.net → overseer (:8087)       ← 监控面板
      ├── partner.starclaw.net  → partner (:8088)        ← 城市合伙人招募
      ├── city.starclaw.net    → city (:8089)           ← 城市合伙人 CRM
      ├── nydus.starclaw.net   → nydus-server (:8095)   ← Claw 更新备源
      └── proxy.starclaw.net   → proxy (:8000)          ← AI API 中转
```

## Server A — Claw + Site (starclaw.me)

开源 AI Agent 平台 + 官网。

| 服务 | 容器名 | 端口 | 域名 | 说明 |
|------|--------|------|------|------|
| **官网** | — (静态文件) | — | **starclaw.me** | **落地页 + 文档 (10 语言)** |
| Claw Web | starclaw-web | 8081 | app.starclaw.me | React 前端 (产品 Demo) |
| Claw API | starclaw-api | 8080 | api.starclaw.me | Go 后端 |
| MySQL | starclaw-mysql | 3306 | — | 数据库 |
| Redis | starclaw-redis | 6379 | — | 缓存 |

**代码目录：** `claw/` (开源产品) + `queen/site/` (官网，闭源)
**部署：** `docker-compose.prod.yml` (Claw) + 静态文件 `/var/www/starclaw/website/` (官网)
**nginx：** `/etc/nginx/sites-enabled/starclaw`

## Server B — Router (star-ai.net)

AI 算力平台 + API Gateway，面向付费用户和开发者。

| 服务 | 容器名 | 端口 | 说明 |
|------|--------|------|------|
| Star-AI API | star-ai-api | 8096 | Go 后端（计费/路由/LLM 直连） |
| Star-AI Web | star-ai-web | 3096 | React 前端 |
| Star-AI Core | star-ai-core | 3097 | Admin 管理面板 |
| **Gateway** | **star-ai-gateway** | **8085** | **OpenAI 兼容 API 网关 (queen-api)** |
| MySQL | star-ai-mysql | 3306 | 数据库 (star_ai + starclaw_queen) |
| Redis | star-ai-redis | 6379 | 缓存 |
| nginx | nginx | 80/443 | 反向代理 |
| **Nydus Worm** | **systemd** | **8097** | **部署执行 Agent（本地监听）** |

**代码目录：** `router/` (api + web + core), Gateway 用 `queen/api`
**部署路径：** `/opt/starclaw/router/` + `/opt/starclaw/gateway/`
**nginx 配置：** `/dnmp/services/nginx/conf.d/starai-router.conf`
**Worm 二进制：** `/opt/nydus-worm`，配置 `/opt/nydus/worm.yaml`，systemd: `nydus-worm.service`

### Gateway 路由

```
star-ai.net/v1/models           → gateway:8085  (模型列表)
star-ai.net/v1/chat/completions → gateway:8085  (聊天代理)
star-ai.net/v1/api-keys         → gateway:8085  (API Key 管理)
star-ai.net/*                   → web:3096      (前端)
api.star-ai.net/*               → api:8096      (Router API)
```

### Gateway 更新流程

**自动（通过 Nydus）：** `git push nydus master` → Nydus Server 通过 SSH 同步 `queen/api` 到 Server B → 调用本地 Worm 构建

```
git push nydus master
  → Nydus Server (Server C)
  → git archive HEAD:queen/api | ssh root@47.103.51.32 tar xf -
  → SSH → curl http://127.0.0.1:8097/deploy
  → Worm: docker compose -f docker-compose.gateway.yml up -d --build
```

**手动（备用）：**

```bash
scp -i ~/.ssh/starai_deploy -r queen/api root@47.103.51.32:/opt/starclaw/gateway/
ssh -i ~/.ssh/starai_deploy root@47.103.51.32 'cd /opt/starclaw/gateway && docker compose -f docker-compose.gateway.yml up -d --build'
```

## Server C — Queen + Nydus + Proxy (starclaw.net)

中央控制服务器（新加坡），管理虫群网络、赏金、社区、代码部署、AI API 中转。

| 服务 | 容器名 | 端口 | 域名 | 说明 |
|------|--------|------|------|------|
| Queen API | starclaw-queen-api | 8085 | api.starclaw.net | 中央 API（用户/计费/星力） |
| Queen Web | starclaw-queen-web | 8086 | starclaw.net | Dashboard 前端 |
| Swarm | starclaw-queen-swarm | 8090 | swarm.starclaw.net | 节点注册/心跳/管理 |
| Core | starclaw-queen-core | 8091 | core.starclaw.net | 管理面板 |
| Bounty | starclaw-queen-bounty | 8092 | bounty.starclaw.net | 赏金市场 |
| Forum | starclaw-queen-forum | 8093 | forum.starclaw.net | 社区论坛 |
| Arena | starclaw-queen-arena | 8094 | arena.starclaw.net | 机器人竞技场 |
| Overseer | starclaw-queen-overseer | 8087 | overseer.starclaw.net | 监控面板 (Prometheus 可视化) |
| Partner | starclaw-queen-partner | 8088 | partner.starclaw.net | 城市合伙人招募页 |
| City | starclaw-queen-city | 8089 | city.starclaw.net | 城市合伙人 CRM |
| **Proxy** | **starclaw-queen-proxy** | **8000** | **proxy.starclaw.net** | **AI API 中转 (OpenAI/Grok/Fal/Runway)** |
| **Redis** | **starclaw-queen-redis** | **6379** | — | **Proxy 队列** |
| Nydus Server | nydus-server | 8095 | nydus.starclaw.net | Git 仓库 + 部署调度 + Claw 更新备源 |
| Nydus Worm | nydus-worm | 8096 | — | 部署执行 Agent |
| MySQL | starclaw-queen-mysql | 3306 | — | 数据库 (starclaw_queen) |
| Prometheus | starclaw-queen-prometheus | 9090 | — | 监控指标 (内网) |

**代码目录：** `queen/` (含 `queen/proxy/`) + `nydus/`
**部署路径：** `/opt/starclaw-queen/` (Queen+Proxy) + `/opt/nydus/` (Nydus)
**部署：** `docker-compose.prod.yml` (Queen+Proxy+Redis) + Nydus (裸进程)
**nginx：** `/etc/nginx/sites-enabled/queen` — 单文件统一管理所有子域名
**SSL：** Let's Encrypt 通配符证书 `*.starclaw.net`（DNS-01 验证）
**状态：** ✅ 容器运行中

### Nydus 虫道部署系统

一次 `git push`，两台服务器同时部署：

```
git push nydus master
  → SSH → /data/nydus/repos/starclaw.git (bare repo)
  → post-receive hook → Nydus Server (:8095)
  ├─ Target 1: queen-server-c (本地)
  │  → Worm clone + sync queen/ 子目录
  │  → docker compose -f docker-compose.prod.yml up -d --build (~20s)
  └─ Target 2: gateway-server-b (跨服务器 SSH)
     → git archive HEAD:queen/api | ssh → Server B
     → SSH → Worm (:8097) → docker compose gateway (~1s cached)
```

**Nydus remote:** `git remote add nydus git@43.106.158.26:starclaw.git`
**SSH config:** `~/.ssh/config` 中 Host 43.106.158.26 使用 `~/.ssh/queen_deploy`
**跨服务器 SSH:** Server C (`id_ed25519`) → Server B (`authorized_keys`)

### 部署架构图

```
开发者 (git push nydus master)
    │
    ▼
Server C: starclaw.git (bare)
    │ post-receive hook
    ▼
Nydus Server (:8095)
    ├───────────────────────────────────────────────┐
    │ Local (Docker network)                        │ SSH tunnel
    ▼                                               ▼
Worm Server C (:8096)                    Worm Server B (:8097)
    │ clone + sync queen/                    │ code synced via git archive
    ▼                                               ▼
Queen (12 containers)                    Gateway (queen-api)
starclaw.net                             star-ai.net/v1/*
```

## Server D — 已废弃 (~47.237.11.193)

> **已迁移到 Server C（proxy.starclaw.net）。** 原 Proxy 服务已 Docker 化并合入 Queen docker-compose。
> Server D 可安全下线。

## 仓库目录结构

```
starclaw/                        # 私有 monorepo
├── claw/           🦞           # Server A — 开源 Claw
├── router/         ⛽           # Server B — star-ai.net
├── queen/proxy/    🌏           # Server C — AI API 海外中转 (proxy.starclaw.net)
├── queen/          👑           # Server C — Queen 中央控制
├── nydus/          🕳️           # Server C — 虫道代码分发系统
├── .env                         # 环境变量（gitignored，含密钥）
├── .env.production.example      # 环境变量模板（tracked）
├── .env.example                 # Claw 开源版模板
├── docker-compose.yml           # Claw 本地开发
├── docker-compose.prod.yml      # Claw 生产部署
├── SERVERS.md                   # 本文档
└── README.md                    # 开源 README
```

## 环境变量管理

| 文件 | Git 状态 | 用途 |
|------|----------|------|
| `.env` | gitignored | 真实密钥，开发/部署用 |
| `.env.production.example` | tracked | 全量模板（占位符） |
| `.env.example` | tracked | Claw 开源版模板 |

## 网络拓扑

```
                    ┌─────────────┐
                    │   用户/Claw  │
                    └──────┬──────┘
                           │
              ┌────────────┼────────────┐
              │            │            │
     ┌────────▼──────┐  ┌─▼──────────┐ │
     │  Server A     │  │ Server B   │ │
     │  starclaw.me  │  │ star-ai.net│ │
     │  (Claw)       │  │ (Router)   │ │
     └───────┬───────┘  └──┬──────┬──┘ │
             │             │      │    │
             │    ┌────────▼──┐   │    │
             │    │ Server D  │   │    │
             │    │ proxy.    │   │    │
             │    │ star-ai.  │   │    │
             │    │ net       │   │    │
             │    │(海外中转) │   │    │
             │    └───────────┘   │    │
             │                    │    │
             └─────┐    ┌────────┘    │
                   │    │             │
              ┌────▼────▼────┐       │
              │  Server C    │◄──────┘
              │ starclaw.net │
              │  (Queen)     │
              └──────────────┘
```
