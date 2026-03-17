# StarClaw Overlord 👁️ — 企业 AI 智能体管控平台

> 让每个企业都拥有自己的 AI 智能体 — 开源 · 私有 · 可管控

---

## 概述

Overlord 是 StarClaw 虫群架构的**企业级管控平台**，为企业客户提供 AI 智能体的全生命周期管理：节点编排、团队隔离、用量计费、SSO 集成、预算告警。

```
👑 Queen（虫后）         starclaw.me 中央管控
     │
👁️ Overlord（领主）      企业 AI 管控平台  ← 本模块
     │  管辖一个 Brood（虫群）
🦞🦞🦞 Claw（小龙虾）    开源 AI Agent 引擎
```

## 功能模块

| 模块 | 功能 | 数据表 |
|------|------|--------|
| **Claw 节点管理** | 注册/心跳/配额/调度/解析/审计 | claw_nodes, task_assignments, audit_logs |
| **多租户 RBAC** | 团队隔离 + 4 级角色 + 权限矩阵 | teams, admin_users |
| **Nydus 隧道** | TCP/UDP 正反向隧道管理 | nydus_tunnels |
| **Molt 更新** | 版本提交→审批→滚动更新→自动熔断 | molt_releases, molt_node_statuses |
| **Webhook** | HMAC 签名投递 + 事件驱动 | webhooks, webhook_logs |
| **用量计费** | 按团队/人/模型统计 + 套餐订阅 + 预算告警 | plans, subscriptions, usage_records, budget_alerts |
| **SSO 集成** | OAuth2/OIDC 通用 + LDAP 目录 + 自动用户配置 | sso_providers, sso_sessions |
| **管理控制台** | 12 页 React SPA（计费 + 分析） | — |
| **员工工作台** | AI 对话 + Agent 市场 + 工具集 + 个人中心 | — |

### 订阅套餐

| 版本 | 月付 | 节点上限 | 团队 | 核心特性 |
|------|:----:|:-------:|:----:|---------|
| **Community** | 免费 | ≤10 | 1 | 基础用量统计 |
| **Starter** | ¥499 | ≤20 | 3 | + 预算告警 |
| **Pro** | ¥1,999 | ≤100 | 不限 | + SSO + 审计日志 + 高级分析 |
| **Enterprise** | ¥4,999 | ≤500 | 不限 | + 合规面板 + SLA 99.9% |
| **White-Label** | ¥9,999+ | 不限 | 不限 | + 品牌定制 + 自定义域名 |

### RBAC 角色

| 角色 | 权限 |
|------|------|
| `superadmin` | 全部 (`*`) |
| `admin` | 节点/团队/隧道/Molt/Webhook/审计/统计/计费读写 |
| `operator` | 节点读写/隧道/Molt 审批/审计只读/计费只读 |
| `viewer` | 全部只读 |

### API 端点（60+）

| 分类 | 端点数 | 说明 |
|------|:------:|------|
| 节点管理 | 8 | 注册/心跳/配额/调度/解析/审计 |
| 团队管理 | 8 | 团队 CRUD + 管理员 CRUD + 登录 |
| Nydus 隧道 | 6 | CRUD + 状态/指标更新 |
| Molt 更新 | 6 | 发布/审批/滚动更新/节点状态 |
| Webhook | 6 | CRUD + 测试投递 |
| 计费管理 | 18 | 套餐/订阅/用量统计/预算告警/概览 |
| SSO 集成 | 10 | Provider CRUD + OAuth2 流程 + LDAP 登录 + 测试 |

### 管理控制台（12 页）

| 页面 | 路由 | 功能 |
|------|------|------|
| 登录 | — | 用户名密码 / SSO 认证 |
| 总览 | `/` | 节点/CPU/内存/任务/Token/团队分布 |
| 节点管理 | `/claws` | 列表/筛选/详情/配额/删除 |
| 团队管理 | `/teams` | 团队 CRUD + 配额管理 |
| Nydus 隧道 | `/tunnels` | CRUD + 流量统计 |
| Molt 更新 | `/molt` | 版本审批/滚动更新/节点进度 |
| Webhook | `/webhooks` | CRUD + 测试 + 投递日志 |
| **计费管理** | `/billing` | 套餐切换/订阅管理/预算告警 |
| **用量分析** | `/analytics` | 每日趋势/模型分布/用户排行 |
| 审计日志 | `/audit` | 操作日志（颜色编码） |
| 地址解析 | `/resolve` | Claw ID → 网络地址 |

### 员工工作台（5 页）

| 页面 | 路由 | 功能 |
|------|------|------|
| 登录 | `/login` | 用户名密码 / SSO |
| AI 对话 | `/` | Agent 选择 + 多轮对话 |
| Agent 市场 | `/agents` | 模板浏览/搜索/分类 |
| 工具集 | `/tools` | MCP 工具目录 |
| 个人中心 | `/profile` | 用量统计/API Key/设置 |

## 技术栈

| 组件 | 技术 |
|------|------|
| 后端 API | Go 1.24 + Gin + GORM + MySQL 8.0 |
| 管理控制台 | React 18 + TypeScript + Vite 6 + TailwindCSS 3 |
| 员工工作台 | React 18 + TypeScript + Vite 6 + TailwindCSS 3 |
| 图标 | Lucide React |
| 路由 | React Router 6 |
| 容器 | Docker (Go multi-stage / node:20 → nginx:alpine) |
| 编排 | Docker Compose |

## 目录结构

```
overlord/
├── api/                           # Go 后端 API (:8095)
│   ├── cmd/server/main.go         # 入口 + 路由注册
│   ├── internal/
│   │   ├── handler/
│   │   │   ├── registry.go        # Claw 注册/心跳/配额/调度
│   │   │   ├── team.go            # 团队 + 管理员 + 登录
│   │   │   ├── nydus.go           # Nydus 隧道
│   │   │   ├── molt.go            # 版本更新
│   │   │   ├── webhook.go         # Webhook + HMAC
│   │   │   ├── billing.go         # 计费 + 用量 + 预算告警
│   │   │   └── sso.go             # OAuth2 + LDAP SSO
│   │   ├── middleware/auth.go     # AdminAuth + RBAC
│   │   └── model/
│   │       ├── brood.go           # ClawNode, TaskAssignment, AuditLog
│   │       ├── team.go            # Team, AdminUser, RolePermissions
│   │       ├── nydus.go           # NydusTunnel
│   │       ├── molt.go            # MoltRelease, MoltNodeStatus
│   │       ├── webhook.go         # Webhook, WebhookLog
│   │       ├── billing.go         # Plan, Subscription, UsageRecord, BudgetAlert
│   │       └── sso.go             # SSOProvider, SSOSession
│   ├── Dockerfile
│   └── go.mod
├── console/                       # 管理控制台 (:3095)
│   ├── src/
│   │   ├── api/brood.ts           # 类型化 API 客户端（60+ 方法）
│   │   ├── pages/                 # 12 个页面
│   │   ├── App.tsx                # 侧边栏 + 路由
│   │   └── main.tsx
│   ├── Dockerfile
│   └── nginx.conf
├── web/                           # 员工工作台 (:3096)
│   ├── src/
│   │   ├── api/client.ts          # API 客户端
│   │   ├── pages/                 # 5 个页面
│   │   ├── App.tsx                # 侧边栏 + 路由
│   │   └── main.tsx
│   ├── Dockerfile
│   └── nginx.conf
├── docker-compose.yml             # 开发环境
├── docker-compose.prod.yml        # 生产环境
├── .env.example                   # 环境变量模板
└── README.md                      # ← 本文件
```

## 快速启动

### 方式一：Docker Compose（推荐）

```bash
# 1. 克隆仓库
git clone <repo-url> && cd overlord

# 2. 复制环境变量
cp .env.example .env
# 编辑 .env 设置密码等

# 3. 一键启动
docker compose up -d --build

# 4. 访问
# 管理控制台: http://localhost:3095
# 员工工作台: http://localhost:3096
# API:        http://localhost:8095/health

# 默认管理员账号
# 用户名: admin
# 密码:   admin123
```

### 方式二：本地开发

```bash
# 后端（需要 Go 1.24 + MySQL）
cd api
export OVERLORD_DSN="root:password@tcp(127.0.0.1:3306)/starclaw_overlord?charset=utf8mb4&parseTime=True&loc=Local"
go run ./cmd/server

# 管理控制台（需要 Node 20+）
cd console
npm install && npm run dev    # → http://localhost:3095

# 员工工作台
cd web
npm install && npm run dev    # → http://localhost:3096
```

## 生产部署

### 1. 准备环境变量

```bash
cp .env.example .env
```

编辑 `.env` 文件：

```env
OVERLORD_MYSQL_PASSWORD=<强密码>
OVERLORD_ADMIN_PASSWORD=<管理员密码>
OVERLORD_MAX_NODES=100          # 按套餐设置
OVERLORD_CONSOLE_PORT=3095
OVERLORD_WEB_PORT=3096
```

### 2. 启动服务

```bash
docker compose -f docker-compose.prod.yml up -d --build
```

### 3. 验证

```bash
# 健康检查
curl http://localhost:8095/health
# → {"service":"overlord-api","status":"ok"}

# 管理控制台
open http://localhost:3095

# 员工工作台
open http://localhost:3096
```

### 4. 配置 SSO（可选）

通过管理控制台或 API 创建 SSO Provider：

```bash
# OAuth2 示例（企业微信）
curl -X POST http://localhost:8095/brood/sso/providers \
  -H "X-Admin-Token: <token>" \
  -d '{
    "name": "企业微信",
    "type": "oauth2",
    "provider": "wecom",
    "client_id": "<client_id>",
    "client_secret": "<client_secret>",
    "auth_url": "https://open.weixin.qq.com/connect/oauth2/authorize",
    "token_url": "https://qyapi.weixin.qq.com/cgi-bin/gettoken",
    "userinfo_url": "https://qyapi.weixin.qq.com/cgi-bin/user/getuserinfo",
    "redirect_url": "http://your-domain/sso/callback",
    "default_role": "viewer"
  }'

# LDAP 示例
curl -X POST http://localhost:8095/brood/sso/providers \
  -H "X-Admin-Token: <token>" \
  -d '{
    "name": "公司 LDAP",
    "type": "ldap",
    "ldap_host": "ldap.example.com:389",
    "ldap_base_dn": "dc=example,dc=com",
    "ldap_bind_dn": "cn=admin,dc=example,dc=com",
    "ldap_bind_pass": "<password>",
    "ldap_user_filter": "(uid=%s)",
    "default_role": "viewer"
  }'
```

### 5. 配置反向代理（生产推荐）

```nginx
# /etc/nginx/sites-available/overlord
server {
    listen 443 ssl;
    server_name overlord.example.com;

    ssl_certificate     /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    # 管理控制台
    location / {
        proxy_pass http://127.0.0.1:3095;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}

server {
    listen 443 ssl;
    server_name app.example.com;

    ssl_certificate     /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    # 员工工作台
    location / {
        proxy_pass http://127.0.0.1:3096;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

## 数据表概览（17 表）

| 分类 | 表名 | 说明 |
|------|------|------|
| 节点 | claw_nodes | Claw 节点注册信息 + 指标 |
| 节点 | task_assignments | 任务分配记录 |
| 节点 | audit_logs | 操作审计日志 |
| 团队 | teams | 多租户团队 |
| 团队 | admin_users | 管理员/用户账号 |
| 隧道 | nydus_tunnels | Nydus 隧道实例 |
| 更新 | molt_releases | 版本发布 |
| 更新 | molt_node_statuses | 节点更新状态 |
| 通知 | webhooks | Webhook 配置 |
| 通知 | webhook_logs | 投递日志 |
| 计费 | plans | 订阅套餐定义 |
| 计费 | subscriptions | 团队订阅关系 |
| 计费 | usage_records | 逐条用量记录 |
| 计费 | usage_daily_summaries | 每日汇总 |
| 计费 | budget_alerts | 预算告警规则 |
| SSO | sso_providers | 身份提供商配置 |
| SSO | sso_sessions | SSO 登录会话 |

## 架构关系

- 1 个 Overlord + N 个 Claw = 1 个 Brood（虫群）
- 管理控制台供管理员使用（节点/团队/计费/分析）
- 员工工作台供企业员工使用（AI 对话/Agent/工具）
- API 同时服务 Claw 节点上报和前端请求

## 商业模式

- **平台订阅费**：按节点数量分级收费（Community 免费 → Enterprise ¥4,999/月）
- **星能消耗费**：非 BYOK 用户按 AI 推理用量收费
- **增值服务**：私有部署实施、定制开发、培训、技术支持
- **白牌定制**：品牌/域名/功能全定制
