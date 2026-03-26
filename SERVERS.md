# StarClaw 服务器架构

## 服务器列表

| 服务器 | 域名 | IP | 角色 | SSH |
|--------|------|-----|------|-----|
| A | starclaw.me | (域名直连) | Claw — 开源官网 + app + api | `ssh -i ~/.ssh/claw_deploy root@starclaw.me` |
| B | star-ai.net | 47.103.51.32 | Synapse — AI 算力平台 (国内) | `ssh -i ~/.ssh/starai_deploy root@47.103.51.32` |
| C | starclaw.net | 43.106.158.26 | Queen + Nydus + Proxy — 中央控制 (新加坡) | `ssh -i ~/.ssh/queen_deploy root@43.106.158.26` |

## 架构全景

```
用户
 │
 ├── starclaw.me ─────────── Server A (Claw)
 │    ├── app.starclaw.me    → claw-web (:3000)
 │    └── api.starclaw.me    → claw-api (:8080)
 │
 ├── star-ai.net ─────────── Server B (Synapse)
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
      ├── overlord.starclaw.net → overlord-api (:8095)    ← 企业 AI 管控
      │                          overlord-console (:3095)
      │                          overlord-web (:3096)
      ├── nydus.starclaw.net   → nydus-api (:8098)      ← 部署调度 + 更新备源
      │                          nydus-web (:8101)       ← Nydus Dashboard
      ├── forge                → forge-api (:8099)       ← CI/CD
      ├── pheromone            → pheromone-api (:8100)   ← ESB 事件总线
      │                          pheromone-nats (:4222)  ← NATS
      └── proxy.starclaw.net   → proxy (:8000)          ← AI API 中转
```

## Server A — Claw + Site (starclaw.me)

开源 AI Agent 平台 + 官网。

| 服务 | 容器名 | 端口 | 域名 | 说明 |
|------|--------|------|------|------|
| **官网** | — (静态文件) | — | **starclaw.me** | **落地页 + 文档 (10 语言)** |
| Claw Web | starclaw-web | 8081 | app.starclaw.me | React 前端 (产品 Demo) |
| Claw API | starclaw-api | 8080 | api.starclaw.me | Go 后端 |
| MySQL | starclaw-mysql | 3306 | — | Claw 数据库 |
| Redis | starclaw-redis | 6379 | — | 缓存 |
| **Hive Web** | **hive-web** | **8082** | — | **多租户控制面板前端** |
| **Hive Controller** | **hive-controller** | **9090** | — | **Hive API (实例管理)** |
| **Hive MySQL** | **hive-mysql** | **3307** | — | **Hive 数据库** |
| **Hive Redis** | **hive-redis** | **6380** | — | **Hive 缓存** |
| Overlord API | starclaw-overlord-api | — | — | Overlord 客户端 (连 Server C 管控) |
| Overlord Console | starclaw-overlord-console | 3095 | — | 管理控制台 |
| Overlord Web | starclaw-overlord-web | 3096 | — | 员工工作台 |
| Overlord MySQL | starclaw-overlord-mysql | 3306 | — | Overlord 数据库 |
| Claw 实例 | claw-*-lite | 9001-9020 | *.starclaw.me | 租户 Claw 实例 (动态分配) |

**代码目录：** `claw/` (开源产品) + `hive/` (多租户控制器) + `queen/site/` (官网，闭源)
**部署路径：** `/opt/starclaw/` (Claw) + `/opt/hive/` (Hive) + `/opt/overlord/` (Overlord)
**部署：** `docker-compose.prod.yml` (Claw) + `docker-compose.hive.yml` (Hive) + 静态文件 `/var/www/starclaw/website/` (官网)
**nginx：** `/etc/nginx/sites-enabled/starclaw`

### 官网部署 (queen/site → starclaw.me)

**自动：** `git push nydus master` 时，若 `queen/site/` 有变更，Nydus hook 会自动通过 Server C SSH 到 Server A 构建并部署。

**手动（备用）：**
```bash
# 从 Server C 中转：取代码 → 传到 Server A → Docker 构建 → 复制静态文件
ssh -i ~/.ssh/queen_deploy root@43.106.158.26 "\
  git --git-dir=/data/nydus/repos/starclaw.git archive HEAD:queen/site | \
  ssh root@starclaw.me 'mkdir -p /tmp/queen-site && cd /tmp/queen-site && rm -rf * && tar xf -'"

ssh -i ~/.ssh/queen_deploy root@43.106.158.26 "\
  ssh root@starclaw.me 'cd /tmp/queen-site && \
    docker build -t queen-site-build . && \
    docker rm -f queen-site-tmp 2>/dev/null; \
    docker create --name queen-site-tmp queen-site-build && \
    rm -rf /var/www/starclaw/website/* && \
    docker cp queen-site-tmp:/usr/share/nginx/html/. /var/www/starclaw/website/ && \
    docker rm queen-site-tmp && rm -rf /tmp/queen-site'"
```

> **注意：** Server A 没有 npm，必须用 Docker 多阶段构建。`queen-web` 容器 (Server C :8086) 是 Queen Dashboard (starclaw.net)，不是官网。

## Server B — Synapse (star-ai.net)

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

**代码目录：** `synapse/` (api + web + core), Gateway 用 `queen/api`
**部署路径：** `/opt/starclaw/synapse/` + `/opt/starclaw/gateway/`
**nginx 配置：** `/dnmp/services/nginx/conf.d/starai-router.conf`
**Worm 二进制：** `/opt/nydus-worm`，配置 `/opt/nydus/worm.yaml`，systemd: `nydus-worm.service`

### Gateway 路由

```
star-ai.net/v1/models           → gateway:8085  (模型列表)
star-ai.net/v1/chat/completions → gateway:8085  (聊天代理)
star-ai.net/v1/api-keys         → gateway:8085  (API Key 管理)
star-ai.net/*                   → web:3096      (前端)
api.star-ai.net/*               → api:8096      (Synapse API)
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
| **Overlord API** | **starclaw-overlord-api** | **8095** | **overlord.starclaw.net** | **企业 AI 管控平台 API** |
| **Overlord Console** | **starclaw-overlord-console** | **3095** | **overlord.starclaw.net** | **管理控制台 (12 页)** |
| **Overlord Web** | **starclaw-overlord-web** | **3096** | **overlord.starclaw.net/app** | **员工工作台 (5 页)** |
| Overlord MySQL | starclaw-overlord-mysql | 3306 | — | Overlord 独立数据库 (内网) |
| Nydus API | nydus-api | 8098 | nydus.starclaw.net | Git 仓库 + 部署调度 + Claw 更新备源 |
| Nydus Web | nydus-web | 8101 | nydus.starclaw.net | Nydus Dashboard 前端 |
| Nydus Worm | nydus-worm | — | — | 部署执行 Agent (Docker 内网) |
| **Forge API** | **forge-api** | **8099** | — | **CI/CD 构建系统** |
| **Pheromone API** | **pheromone-api** | **8100** | — | **ESB 事件总线** |
| **Pheromone Web** | **pheromone-web** | **3110** | — | **ESB Dashboard** |
| **Pheromone NATS** | **pheromone-nats** | **4222/8880** | **nats.starclaw.net** | **消息队列 (TCP + WebSocket)** |
| MySQL | starclaw-queen-mysql | 3306 | — | 数据库 (starclaw_queen) |
| Prometheus | starclaw-queen-prometheus | 9090 | — | 监控指标 (内网) |

**代码目录：** `queen/` (含 `queen/proxy/`) + `nydus/` + `overlord/` + `forge/` + `pheromone/`
**部署路径：** `/opt/queen/` + `/opt/nydus/` + `/opt/overlord/` + `/opt/forge/` + `/opt/pheromone/`
**部署：** 各服务独立 `docker-compose` 文件
**nginx：** `/etc/nginx/sites-enabled/queen` — 单文件统一管理所有子域名
**SSL：** Let's Encrypt 通配符证书 `*.starclaw.net`（DNS-01 验证）
**状态：** ✅ 容器运行中

### Nydus 虫道部署系统

一次 `git push`，两台服务器同时部署：

```
git push nydus master
  → SSH → /data/nydus/repos/starclaw.git (bare repo)
  → post-receive hook → Nydus API (:8098)
  ├─ Pre-sync: pheromone-sdk → queen/overlord/synapse/hive 构建目录
  ├─ queen-server-c     (本地 Worm, git archive + docker build)
  ├─ overlord-server-c  (本地 Worm)
  ├─ pheromone-server-c (本地 Worm)
  ├─ cerebrate-server-c (本地 Worm)
  ├─ gateway-server-b   (SSH → Server B, git archive + Worm)
  ├─ synapse-server-b   (SSH → Server B)
  ├─ hive-server-a      (SSH → Server A)
  └─ claw-starclaw-me   (SSH → Server A)
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
Nydus API (:8098)
    ├─────────────────────────────────────┐──────────────┐
    │ Local (Docker network)                   │ SSH + archive  │ SSH + archive
    ▼                                          ▼              ▼
Worm C (git archive → deploy)          Server B          Server A
    │                                    (Synapse+GW)    (Hive+Claw)
    ├→ Queen     (starclaw.net)
    ├→ Overlord  (overlord.starclaw.net)
    ├→ Pheromone (ESB)
    └→ Cerebrate
```

## Server D — 已废弃 (~47.237.11.193)

> **已迁移到 Server C（proxy.starclaw.net）。** 原 Proxy 服务已 Docker 化并合入 Queen docker-compose。
> Server D 可安全下线。

## 仓库目录结构

```
starclaw/                        # 私有 monorepo
├── claw/           🦞           # Server A — 开源 Claw (starclaw.me)
├── hive/           🐝           # Server A — 多租户 Claw 控制器
├── synapse/        ⛽           # Server B — star-ai.net AI 算力平台
├── queen/          👑           # Server C — Queen 中央控制 (starclaw.net)
├── queen/proxy/    🌏           # Server C — AI API 海外中转 (proxy.starclaw.net)
├── overlord/       👁️           # Server C+A — 企业 AI 管控 (overlord.starclaw.net)
├── nydus/          🕳️           # Server C — 虫道部署系统
├── pheromone/      📡           # Server C — ESB 事件总线 (NATS)
├── forge/          🔨           # Server C — CI/CD 构建系统
├── cerebrate/      🧠           # Server C — 合伙人生态
├── carapace/       🛡️           # Server A — 共享 Go 库
├── spore/          🌱           # 桌面客户端 (Spore)
├── larva/          🐛           # CLI 工具
├── .env                         # 环境变量（gitignored，含密钥）
├── .env.production.example      # 环境变量模板（tracked）
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
           ┌─────────────────┼─────────────────┐
           │                 │                 │
  ┌────────▼──────┐  ┌──────▼───────┐  ┌──────▼──────────┐
  │  Server A     │  │  Server B    │  │  Server C       │
  │  starclaw.me  │  │  star-ai.net │  │  starclaw.net   │
  │  Claw+Hive    │  │  Synapse     │  │  Queen+Overlord │
  │  +Overlord    │  │  +Gateway    │  │  +Nydus+Pheromone│
  └───────┬───────┘  └──────┬───────┘  │  +Forge+Proxy   │
          │                 │          └────────┬────────┘
          │   Pheromone ESB (NATS)              │
          └──────────────┼──────────────────────┘
                    TCP :4222 / WSS :8880
```
