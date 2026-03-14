# Nydus — 虫道代码分发系统

> 灵感来自星际争霸虫族的 **Nydus Network（纳德斯虫道网络）**
> 代码从一端推入，瞬间从另一端完成部署

## 架构

```
开发者 (git push nydus master)
    │
    ▼ SSH (宿主机 :22, git 用户)
Server C: /data/nydus/repos/starclaw.git (bare repo)
    │ post-receive hook
    ▼
Nydus Server (:8095, Docker)
    ├───────────────────────────────────────────┐
    │ Local (Docker network)                    │ SSH tunnel
    ▼                                           ▼
Worm Server C (:8096)                 Worm Server B (:8097)
    │ clone + sync queen/                 │ git archive → ssh tar
    ▼                                           ▼
Queen (12 containers)                 Gateway (queen-api)
starclaw.net                          star-ai.net/v1/*
```

## 组件

| 组件 | 位置 | 端口 | 说明 |
|------|------|------|------|
| **Nydus Server** | Server C (Docker) | 8095 | Git 仓库管理 + 部署调度 |
| **Nydus Worm** | Server C (Docker) | 8096 | 本地部署 Agent |
| **Nydus Worm** | Server B (systemd) | 8097 | 远程部署 Agent |

## Monorepo 模式

整个 `starclaw/` monorepo 推送到一个 `starclaw.git` bare repo，通过 `subdir` 字段分发不同子目录到不同服务器：

| Target | 子目录 | 部署服务器 | 方式 |
|--------|--------|-----------|------|
| queen-server-c | `queen/` | Server C | 本地 Worm (Docker) |
| gateway-server-b | `queen/api/` | Server B | SSH + 远程 Worm |

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

对于 `ssh_host` 配置的远程 target，Nydus Server：

1. 从 bare repo 用 `git archive HEAD:<subdir>` 提取子目录
2. 通过 SSH 管道传输到远程服务器的 deploy_path
3. SSH 调用远程 Worm 的 `/deploy` 端点执行构建

优势：**无需在远程服务器开放额外端口**（绕过云安全组限制）

## API

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/health` | 健康检查 |
| GET | `/api/repos` | 列出所有仓库 |
| POST | `/api/repos` | 创建新仓库 |
| GET | `/api/repos/:name` | 仓库详情 |
| DELETE | `/api/repos/:name` | 删除仓库 |
| POST | `/api/repos/:name/deploy?branch=X` | 手动触发部署 |
| GET | `/api/deploys` | 最近部署记录 |

所有 `/api` 请求需要 `X-Nydus-Secret` 头。

## 配置

### nydus.yaml (Server)

```yaml
server:
  port: "8095"
  secret: "your-secret"
  repos_dir: "/data/nydus/repos"

repos:
  starclaw:
    description: "StarClaw monorepo"
    targets:
      # 本地 target：直接调用同网络的 Worm
      - name: "queen-server-c"
        worm_url: "http://nydus-worm:8096"
        deploy_path: "/opt/starclaw-queen"
        deploy_cmd: "docker compose -f docker-compose.prod.yml up -d --build"
        subdir: "queen"
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
