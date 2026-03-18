# Nydus — 虫道代码分发系统

> 灵感来自星际争霸虫族的 **Nydus Network（纳德斯虫道网络）**
> 代码从一端推入，瞬间从另一端完成部署

## 项目结构

```
nydus/
├── api/                    # Go 后端 (Gin)
│   ├── cmd/
│   │   ├── server/         # nydus-server 入口
│   │   └── worm/           # nydus-worm 入口
│   ├── internal/
│   │   ├── config/         # 配置加载
│   │   ├── handler/        # API 处理器 (repo, release, deploy)
│   │   ├── middleware/     # 认证中间件
│   │   └── router/         # 路由注册
│   ├── configs/            # nydus.yaml + worm.yaml
│   ├── scripts/            # entrypoint.sh
│   ├── Dockerfile          # Server 镜像
│   └── Dockerfile.worm     # Worm 镜像
├── web/                    # React 前端 (Vite + TailwindCSS)
│   ├── src/
│   │   ├── App.tsx         # Dashboard 主页
│   │   └── lib/api.ts      # API 客户端
│   ├── nginx.conf          # 反向代理配置
│   └── Dockerfile
├── scripts/                # 运维脚本
├── docker-compose.yml      # api + web + worm 三服务编排
└── README.md
```

## 架构

```
开发者 (git push nydus master)
    │
    ▼ SSH (宿主机 :22, git 用户)
Server C: /data/nydus/repos/starclaw.git (bare repo)
    │ post-receive hook
    ▼
Nydus API (:8095, Docker)  ←──  Nydus Web (:80, Docker/Nginx)
    ├───────────────────────────────────────────┐───────────────────┐
    │ Local (Docker network)                    │ SSH tunnel         │ SSH direct
    ▼                                           ▼                    ▼
Worm Server C (:8096)                 Worm Server B (:8097)     starclaw.me
    │ clone + sync queen/                 │ git archive → ssh      │ git archive → ssh
    ▼                                     ▼                         ▼
Queen (12 containers)              Gateway (queen-api)        Claw (api+web)
starclaw.net                       star-ai.net/v1/*           starclaw.me
```

## 组件

| 组件 | 位置 | 端口 | 说明 |
|------|------|------|------|
| **Nydus API** | Server C (Docker) | 8095 | Git 仓库管理 + 部署调度 + Release API |
| **Nydus Web** | Server C (Docker) | 80 | React Dashboard + Nginx 反向代理 |
| **Nydus Worm** | Server C (Docker) | 8096 | 本地部署 Agent |
| **Nydus Worm** | Server B (systemd) | 8097 | 远程部署 Agent |

## Monorepo 模式

整个 `starclaw/` monorepo 推送到一个 `starclaw.git` bare repo，通过 `subdir` 字段分发不同子目录到不同服务器：

| Target | 子目录 | 部署服务器 | 方式 |
|--------|--------|-----------|------|
| queen-server-c | `queen/` | Server C | 本地 Worm (Docker) |
| overlord-server-c | `overlord/` | Server C | 本地 Worm (Docker) |
| gateway-server-b | `queen/api/` | Server B | SSH + 远程 Worm |
| claw-starclaw-me | `claw/` | starclaw.me | SSH direct（无 Worm） |

## 快速开始

### 1. 本地添加 remote

```bash
git remote add nydus git@43.106.158.26:starclaw.git
```

需要在 `~/.ssh/config` 中配置：

```
Host 43.106.158.26
    IdentityFile ~/.ssh/queen_deploy
    User git
    StrictHostKeyChecking no
```

### 2. 推送代码（自动部署）

```bash
git push nydus master
```

推送后自动触发：
1. `post-receive` hook → 通知 Nydus Server
2. Nydus Server 按 `nydus.yaml` 配置分发到所有 target
3. **本地 target**: Worm clone → sync subdir → docker compose build
4. **远程 target**: git archive subdir → SSH pipe → 远程 Worm deploy

### 3. 手动触发部署

```bash
# 在 Server C 上
curl -X POST 'http://127.0.0.1:8095/api/repos/starclaw/deploy?branch=master' \
  -H 'X-Nydus-Secret: <secret>'
```

## 跨服务器部署（SSH 模式）

对于 `ssh_host` 配置的远程 target，支持两种部署模式：

### Worm 模式（`worm_url` 已配置）

1. 从 bare repo 用 `git archive HEAD:<subdir>` 提取子目录
2. 通过 SSH 管道传输到远程服务器的 deploy_path
3. SSH 调用远程 Worm 的 `/deploy` 端点执行构建

### Direct SSH 模式（`worm_url` 为空）

1. 从 bare repo 用 `git archive HEAD:<subdir>` 提取子目录
2. 通过 SSH 管道传输到远程服务器的 deploy_path
3. SSH 直接在 deploy_path 中执行 `deploy_cmd`（无需安装 Worm）

适用于不需要 Worm Agent 的简单服务器，如 Claw 实例。

优势：**无需在远程服务器开放额外端口**（绕过云安全组限制）

## 安全模型

Nydus 区分 **公开仓库** 和 **私有仓库**，通过 `public` 字段控制可见性：

```yaml
repos:
  starclaw:
    public: false    # 私有 — 公开 API 完全不可见
  claw:
    public: true     # 公开 — 和 GitHub 内容一致
```

### 路由分层

| 路由前缀 | 认证方式 | 可见范围 | 说明 |
|----------|---------|---------|------|
| `/v1/*` | 无需认证 | 仅 `public: true` 仓库 | Web UI + 外部消费者 |
| `/api/*` | `X-Nydus-Secret` | 全部仓库 | 管理操作（CRUD、部署） |
| `/releases/*` | 无需认证 | 仅 `claw.git` | Claw 自动更新回退源 |
| `/hooks/*` | `X-Nydus-Secret` | 全部仓库 | Git post-receive 钩子 |

### 保护机制

- **公开 API** (`/v1/*`) 路由设置 `public_only` 上下文标记
- 所有 handler 检查该标记 — 私有仓库返回 404（不泄露是否存在）
- `ListRepos` 过滤私有仓库
- `ListDeploys` 过滤私有仓库的部署记录
- `GetServerStats` 仅统计公开仓库
- **管理 API** (`/api/*`) 不设该标记，可访问全部仓库

### 安全审计清单

| 端点 | 对外是否安全 | 说明 |
|------|------------|------|
| `/health` | 安全 | 仅返回状态 |
| `/releases/*` | 安全 | 只读 claw.git（OSS 镜像） |
| `/v1/repos` | 安全 | 仅列出 public 仓库 |
| `/v1/repos/:name` | 安全 | 私有仓库返回 404 |
| `/v1/repos/:name/tree` | 安全 | 私有仓库返回 404 |
| `/v1/repos/:name/readme` | 安全 | 私有仓库返回 404 |
| `/v1/repos/:name/branches` | 安全 | 私有仓库返回 404 |
| `/v1/repos/:name/tags` | 安全 | 私有仓库返回 404 |
| `/v1/commits?repo=X` | 安全 | 私有仓库返回 404 |
| `/v1/deploys` | 安全 | 仅显示 public 仓库部署 |
| `/v1/stats` | 安全 | 仅统计 public 仓库 |
| `/api/*` | 需密钥 | 完整访问，受 Secret 保护 |

## API

### 公开接口（无需认证，仅返回 `public: true` 仓库数据）

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/health` | 健康检查 |
| GET | `/v1/repos` | 列出公开仓库（含统计） |
| GET | `/v1/repos/:name` | 仓库详情 |
| GET | `/v1/repos/:name/tree?path=&ref=HEAD` | 文件树浏览 |
| GET | `/v1/repos/:name/readme` | README 内容 |
| GET | `/v1/repos/:name/branches` | 分支列表 |
| GET | `/v1/repos/:name/tags` | 标签列表 |
| GET | `/v1/commits?repo=claw&limit=20` | 最近提交记录 |
| GET | `/v1/deploys` | 最近部署记录 |
| GET | `/v1/stats` | 服务器统计 |
| GET | `/v1/releases/latest` | 最新版本信息 |
| GET | `/releases/latest` | 最新版本信息（兼容） |
| GET | `/releases/source.tar.gz` | 源码包下载 |

### 管理接口（需要 `X-Nydus-Secret` 头，可访问全部仓库）

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/repos` | 列出所有仓库（含私有） |
| POST | `/api/repos` | 创建新仓库 |
| GET | `/api/repos/:name` | 仓库详情（含私有） |
| DELETE | `/api/repos/:name` | 删除仓库 |
| POST | `/api/repos/:name/deploy?branch=X` | 手动触发部署 |
| GET | `/api/deploys` | 全部部署记录 |

## 配置

### nydus.yaml (Server)

```yaml
server:
  port: "8095"
  secret: "your-secret"
  repos_dir: "/data/nydus/repos"

repos:
  # 私有仓库 — 公开 API 不可见
  starclaw:
    description: "StarClaw monorepo"
    public: false
    targets:
      # 本地 target：直接调用同网络的 Worm
      - name: "queen-server-c"
        worm_url: "http://nydus-worm:8096"
        deploy_path: "/opt/starclaw-queen"
        deploy_cmd: "docker compose -f docker-compose.prod.yml up -d --build"
        subdir: "queen"
        branch: "master"

      # Overlord：企业 AI 管控平台（本地 Worm）
      - name: "overlord-server-c"
        worm_url: "http://nydus-worm:8096"
        deploy_path: "/opt/starclaw-overlord"
        deploy_cmd: "docker compose -f docker-compose.prod.yml up -d --build api console web"
        subdir: "overlord"
        branch: "master"

      # 远程 target：通过 SSH 同步代码 + 调用远程 Worm
      - name: "gateway-server-b"
        ssh_host: "root@47.103.51.32"
        ssh_key: "/root/.ssh/id_ed25519"
        worm_url: "http://127.0.0.1:8097"
        deploy_path: "/opt/starclaw/gateway"
        deploy_cmd: "docker compose -f docker-compose.gateway.yml up -d --build"
        subdir: "queen/api"
        branch: "master"

  # 公开仓库 — Web UI 可见，和 GitHub 内容一致
  claw:
    description: "StarClaw open-source AI agent platform"
    public: true
    targets: []
```

### worm.yaml (Agent)

```yaml
port: "8096"          # Server C: 8096, Server B: 8097
secret: "same-secret-as-server"
deploy_dirs:
  starclaw: /opt/starclaw-queen
```

## 虫族命名体系

| 名称 | 角色 |
|------|------|
| **Nydus** | 虫道入口 — Git 仓库 + 调度中心 |
| **Worm** | 虫道出口 — 部署执行 Agent |
| **Canal** | 虫道通道 — 一次 push → deploy 的完整链路 |
