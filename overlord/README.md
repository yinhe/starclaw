# StarClaw Overlord 👁️ — 领主企业管理层（闭源）

> 领主（Overlord）是 StarClaw 虫群架构的企业级中间管理节点

## 定位

Overlord 是**付费的企业管理层**，部署在 Claw 之上、Queen 之下，
为企业客户提供多节点管理、任务编排、负载均衡等高级能力。

星际争霸中，领主提供人口上限、侦察视野和运输能力——
StarClaw 的领主同样掌控**资源配额、监控视野和任务分发**。
一个 Overlord 管辖的所有 Claw 集群统称为一个 **Brood（虫群）**。

```
👑 Queen（虫后）         闭源，starclaw.me 中央管控
     │
👁️ Overlord（领主）      闭源，企业付费管理层  ← 本模块
     │  管辖一个 Brood（虫群）
🦞🦞🦞 Claw（小龙虾）    开源，最小执行单元
```

## 已实现功能

| 模块 | 功能 | 数据表 |
|------|------|--------|
| **Claw 管理** | 注册/心跳/配额/调度/解析/审计 | claw_nodes, task_assignments, audit_logs |
| **多租户 RBAC** | 团队隔离 + 4 级角色权限 | teams, admin_users |
| **Nydus 隧道** | TCP/UDP 正向/反向隧道管理 | nydus_tunnels |
| **Molt 更新审批** | 版本提交→审批→滚动更新→自动熔断 | molt_releases, molt_node_statuses |
| **Webhook 通知** | HMAC 签名投递 + 事件驱动 | webhooks, webhook_logs |
| **管理控制台** | 10 页 React SPA（含登录） | — |

### RBAC 角色权限

| 角色 | 权限范围 |
|------|---------|
| `superadmin` | 全部权限 (`*`) |
| `admin` | Claw 全操作、团队管理、隧道管理、Molt 管理、Webhook、审计、统计 |
| `operator` | Claw 读写、隧道管理、Molt 审批、审计只读、统计 |
| `viewer` | 全部只读 |

### API 端点（40+）

| 权限 | 端点 |
|------|------|
| 公开 | `POST /brood/register` · `/heartbeat` · `/auth/login` · `/molt/node-status` |
| viewer+ | `GET /brood/claws` · `/stats` · `/audit` · `/resolve` · `/tunnels` · `/molt/releases` · `/webhooks` |
| operator+ | `PUT /brood/claws/:id/quota` · `POST /task/assign` · 隧道/Molt CRUD |
| admin+ | `DELETE /brood/claws/:id` · 团队 CRUD · Molt 审批 |
| superadmin | `GET/POST/DELETE /brood/admins` |

### Webhook 事件

| 事件 | 触发时机 |
|------|---------|
| `node.online` | Claw 心跳从非在线恢复为在线 |
| `node.feral` | 90 秒无心跳，Claw 进入失控状态 |
| `node.offline` | 5 分钟无心跳，Claw 标记离线 |
| `test` | 手动测试投递 |

### 管理控制台（10 页）

| 页面 | 路由 | 功能 |
|------|------|------|
| 登录 | — | 用户名密码认证，Token 存储 |
| 总览 | `/` | 节点统计、CPU/内存、任务数、Token、团队分布 |
| 节点管理 | `/claws` | 列表/筛选/详情/配额/删除 |
| 团队管理 | `/teams` | 创建/删除团队，配额管理 |
| Nydus 隧道 | `/tunnels` | 隧道 CRUD，流量统计，状态筛选 |
| Molt 更新 | `/molt` | 版本提交/审批/滚动更新，节点进度 |
| Webhook | `/webhooks` | 创建/删除/测试投递，投递日志 |
| 审计日志 | `/audit` | 操作日志（颜色编码） |
| 地址解析 | `/resolve` | claw: ID → 网络地址 |

## 技术栈

| 组件 | 技术 |
|------|------|
| 后端 | Go 1.24 + Gin + GORM + MySQL 8.0 |
| 前端 | React 18 + TypeScript + Vite 6 + TailwindCSS 3 |
| 图标 | Lucide React |
| 路由 | React Router 6 |
| 容器 | Docker (Go multi-stage / node:20-alpine → nginx:alpine) |

## 目录结构

```
overlord/
├── manager/                       # Go 管理服务 (:8095)
│   ├── cmd/server/main.go         # 入口
│   ├── internal/
│   │   ├── handler/
│   │   │   ├── registry.go        # Claw 注册/心跳/配额/调度/解析
│   │   │   ├── team.go            # 团队 CRUD + 管理员 + 登录
│   │   │   ├── nydus.go           # Nydus 隧道 CRUD
│   │   │   ├── molt.go            # 版本发布/审批/滚动更新
│   │   │   └── webhook.go         # Webhook CRUD + HMAC 投递
│   │   ├── middleware/auth.go     # AdminAuth + RBAC + TeamScope
│   │   └── model/                 # GORM 模型（5 文件，10 表）
│   ├── Dockerfile
│   └── go.mod
├── console/                       # React 控制台 (:3095)
│   ├── src/
│   │   ├── api/brood.ts           # 类型化 API 客户端
│   │   ├── pages/                 # 10 个页面
│   │   ├── App.tsx                # 侧边栏 + 路由 + 鉴权
│   │   └── main.tsx
│   ├── Dockerfile
│   └── nginx.conf
├── docker-compose.yml             # 开发环境
├── docker-compose.prod.yml        # 生产环境
└── README.md                      # ← 本文件
```

## 快速启动

```bash
# 开发环境
cd overlord
docker compose up -d

# 访问控制台
open http://localhost:3095

# 默认账号
# 用户名: admin
# 密码:   admin123（可通过 OVERLORD_ADMIN_PASSWORD 环境变量修改）
```

## 生产部署

```bash
# 设置密码
export OVERLORD_MYSQL_PASSWORD=your_secure_password
export OVERLORD_ADMIN_PASSWORD=your_admin_password

# 启动
docker compose -f docker-compose.prod.yml up -d --build
```

## 与 Claw 的关系

- 每个 Overlord 部署时**内嵌一个完整的 Claw 实例**（Overlord 自身也能执行 AI 任务）
- Overlord 管理服务作为独立进程运行在 Claw 旁边
- 企业客户部署：1 个 Overlord + N 个 Claw = 1 个 Brood（虫群）

## 商业模式

- 按管理的 Claw 节点数量收费
- 企业年度订阅制
- 包含技术支持和 SLA 保障
