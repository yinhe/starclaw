# Nydus 🕳️ — 虫道代码分发系统

> 灵感来自星际争霸虫族的 **Nydus Network（纳德斯虫道网络）**
> 代码从一端推入，瞬间从另一端完成部署

## 架构

```
开发者 (git push)
    │
    ▼ SSH (:2222)
┌──────────────────┐
│  Nydus Server    │  bare repos + post-receive hook
│  :8095           │  nydus.starclaw.net
└──────┬───────────┘
       │ webhook
       ▼
┌──────────────────┐     ┌──────────────────┐
│  Nydus Worm      │     │  Nydus Worm      │
│  Server C :8096  │     │  Server B :8096  │
│  → Queen deploy  │     │  → Gateway deploy│
└──────────────────┘     └──────────────────┘
```

## 组件

| 组件 | 说明 | 端口 |
|------|------|------|
| **Nydus Server** | Git 仓库管理 + 部署调度 | 8095 (API) + 2222 (SSH) |
| **Nydus Worm** | 部署执行 Agent（每台目标服务器一个） | 8096 |

## 快速开始

### 1. 部署 Nydus Server + Worm

```bash
cd nydus
docker compose up -d --build
```

### 2. 添加 SSH 公钥

```bash
# 在 .env 中设置
NYDUS_SSH_PUBKEY="ssh-ed25519 AAAA... your-key"

# 或直接挂载
docker exec nydus-server sh -c 'echo "ssh-ed25519 AAAA..." >> /home/git/.ssh/authorized_keys'
```

### 3. 本地添加 remote 并推送

```bash
# 在 queen/ 目录中
git remote add nydus ssh://git@nydus.starclaw.net:2222/data/nydus/repos/queen.git
git push nydus main
```

推送后自动触发：
1. `post-receive` hook → 通知 Nydus Server
2. Nydus Server → 按 `nydus.yaml` 配置调用目标 Worm
3. Worm → `docker compose up -d --build`

### 4. 手动触发部署

```bash
curl -X POST http://nydus.starclaw.net:8095/api/repos/queen/deploy \
  -H "X-Nydus-Secret: <secret>"
```

## API

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/health` | 健康检查 |
| GET | `/api/repos` | 列出所有仓库 |
| POST | `/api/repos` | 创建新仓库 |
| GET | `/api/repos/:name` | 仓库详情 |
| DELETE | `/api/repos/:name` | 删除仓库 |
| POST | `/api/repos/:name/deploy` | 手动触发部署 |
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
  queen:
    description: "Queen 中央控制"
    targets:
      - name: "queen-server-c"
        worm_url: "http://127.0.0.1:8096"
        deploy_path: "/opt/starclaw-queen"
        deploy_cmd: "docker compose -f docker-compose.prod.yml up -d --build"
        branch: "main"
```

### worm.yaml (Agent)

```yaml
port: "8096"
secret: "same-secret-as-server"
```

## 虫族命名体系

| 名称 | 角色 |
|------|------|
| **Nydus** | 虫道入口 — Git 仓库 + 调度中心 |
| **Worm** | 虫道出口 — 部署执行 Agent |
| **Canal** | 虫道通道 — 一次 push → deploy 的完整链路 |
