# Forge v1.0 PRD — 研发管控平台补全 & 生产上线

> **项目**: StarClaw Forge 🔥 熔炉
> **版本**: v1.0 (首次生产发布)
> **日期**: 2026-03-24
> **状态**: draft

---

## 一、目标

将 Forge 从当前的开发预览状态推进到**生产可用**，补全数据聚合层、Webhook 事件驱动、前端可视化组件，并完成生产环境部署上线。

**一句话**: 让 forge.starclaw.net 成为 StarClaw 研发的实时指挥大屏。

---

## 二、现状盘点

### 已完成 ✅

| 模块 | 内容 |
|------|------|
| **数据模型** | 8 张表: Project, Issue, Sprint, Milestone, Activity, PRD, Comment, Agent |
| **API** | Project/Issue/Sprint CRUD, Board, Burndown, Dashboard×9, PRD 生成/规划 (SSE), Orchestrator |
| **聚合层** | `aggregator/nydus.go` (heatmap/commits/deploys), `aggregator/devbridge.go` (MCP client) |
| **AI 引擎** | PRD 生成器 (LLM streaming), Sprint 拆分器, Orchestrator (DAG 依赖调度) |
| **前端** | 7 页面: Dashboard, Board, PRD, Sprint, Orchestrator, Timeline, Login |
| **代理** | Claw `/v1/forge/*` → forge-api (STARCLAW_FORGE_URL), Overlord `/brood/forge/api/*` → forge-api |
| **部署** | Dockerfile×2, docker-compose, 本地运行正常 |
| **Nydus** | 新增 `/v1/commits/heatmap` 端点 |

### 缺失 ❌

| 编号 | 缺失项 | 影响 |
|------|--------|------|
| G-1 | Webhook 接收器 (Nydus/GitHub/DevBridge) | 大屏活动流无实时数据，Issue 不能自动关联 PR/分支 |
| G-2 | 服务健康聚合 (`aggregator/health.go`) | 大屏"服务健康地图"显示假数据 |
| G-3 | Overlord 聚合 (`aggregator/overlord.go`) | DevClaw 实例状态从 Overlord API 获取而非硬编码 |
| G-4 | GitHub CI 聚合 (`aggregator/github.go`) | CI 构建状态无法显示 |
| G-5 | Dev Bridge 双向同步回写 | Forge Issue 状态变更不能同步回 Dev Bridge |
| G-6 | 前端可视化组件 | 大屏缺少独立图表组件 (热力图/燃尽图/饼图/部署时间线) |
| G-7 | 甘特图 (Gantt) | TimelinePage 只有热力图，缺少 Issue 时间线视图 |
| G-8 | 生产部署 | 未部署到 Server C，无 nginx 配置，Nydus hook 未集成 |

---

## 三、功能需求

### F-1: Webhook 接收器

**优先级**: P0 (核心)
**服务**: forge/api
**文件**: `handler/webhook.go`

接收来自 Nydus、GitHub、Dev Bridge 的事件推送，写入 ForgeActivity 并自动关联 Issue。

```
POST /api/webhooks/nydus     ← Nydus push/pr/deploy 事件
POST /api/webhooks/github    ← GitHub Actions check_run/workflow_run
POST /api/webhooks/devbridge ← Dev Bridge task 状态变更
```

**事件映射**:

| 来源 | 事件 | 动作 |
|------|------|------|
| Nydus push | commit 包含 `SC-42` | → ForgeActivity (type:commit) + Issue 关联 |
| Nydus pr.opened | PR 标题含 `SC-42` | → ForgeActivity (type:pr) + Issue 关联 PR |
| Nydus pr.merged | PR 合并 | → ForgeActivity + Issue 自动 → done |
| Nydus deploy | 部署完成 | → ForgeActivity (type:deploy) |
| GitHub check_run | CI 通过/失败 | → ForgeActivity (type:ci) |
| DevBridge task_update | 任务状态变更 | → 对应 Issue 状态同步 |

**验收标准**:
- Nydus push 带 `SC-42` 的 commit → 30s 内出现在活动流
- PR merged → Issue 自动变为 done
- Webhook 签名验证 (HMAC-SHA256)

---

### F-2: 服务健康聚合

**优先级**: P0
**服务**: forge/api
**文件**: `aggregator/health.go`

定时轮询 StarClaw 所有服务的 `/health` 端点，汇总到 `/api/dashboard/services`。

```go
// 轮询目标 (从 config 读取)
services:
  - name: claw-api    url: http://host.docker.internal:8080/health
  - name: queen-api   url: http://host.docker.internal:8085/health
  - name: synapse-api url: http://host.docker.internal:8096/health
  - name: overlord    url: http://host.docker.internal:8098/health
  - name: nydus       url: https://nydus.starclaw.net/health
  - name: hive        url: http://host.docker.internal:9090/health
  - name: forge-api   url: http://localhost:8099/health
```

**行为**:
- 每 30s 轮询一次
- 状态: `up` (200 + <2s) / `degraded` (200 + >2s) / `down` (非200 或超时)
- 缓存结果，API 直接返回缓存

**验收标准**:
- `/api/dashboard/services` 返回所有服务实时状态
- 某服务宕机后 ≤60s 大屏显示红色

---

### F-3: Overlord DevClaw 聚合

**优先级**: P1
**服务**: forge/api
**文件**: `aggregator/overlord.go`

从 Overlord API 获取 DevClaw 实例列表和 Mission 进度，替代 dashboard.go 中的硬编码 fallback。

```go
type OverlordClient struct {
    BaseURL string
    Token   string
}

// 获取 DevClaw 实例 → GET /brood/team-agent/instances
// 获取 Mission 列表 → GET /brood/team-agent/missions
```

**验收标准**:
- 大屏 DevClaw 卡片显示真实实例名、状态、当前 Mission

---

### F-4: GitHub CI 聚合

**优先级**: P2
**服务**: forge/api
**文件**: `aggregator/github.go`

从 GitHub API 获取 Actions workflow runs 状态。

```go
type GitHubClient struct {
    Token string
    Repo  string // "yinhe/starclaw"
}

// GET /repos/{owner}/{repo}/actions/runs?per_page=10
```

**验收标准**:
- 大屏显示最近 CI 构建状态 (pass/fail/running)
- 可选: webhook 替代轮询

---

### F-5: Dev Bridge 双向同步回写

**优先级**: P1
**服务**: forge/api
**文件**: `handler/issue.go` (Transition 方法增强)

当 Forge Issue 状态通过 `POST /api/issues/:id/transition` 变更时，自动同步回 Dev Bridge。

```
Issue in_progress → Dev Bridge task_update(status: "in_progress")
Issue done        → Dev Bridge task_update(status: "done")
Issue closed      → Dev Bridge task_update(status: "closed")
```

**验收标准**:
- Forge 看板拖拽 Issue → Dev Bridge 任务状态同步变更

---

### F-6: 前端可视化组件

**优先级**: P1
**服务**: forge/web
**文件**: `src/components/*.tsx`

将 DashboardPage 中的内联渲染抽取为独立可复用组件:

| 组件 | 功能 | 数据源 |
|------|------|--------|
| `ServiceStatusGrid.tsx` | 12 服务健康状态网格 (绿/黄/红) | `/dashboard/services` |
| `CommitHeatmap.tsx` | Git 提交热力图 (类 GitHub 贡献图) | `/dashboard/heatmap` |
| `DeployTimeline.tsx` | 部署历史时间线 (成功/失败标记) | `/dashboard/deploys` |
| `ActivityFeed.tsx` | 实时活动流 (commit/pr/deploy/issue) | `/dashboard/activity` |
| `IssuePieChart.tsx` | Issue 按类型/状态饼图 | `/dashboard/stats` |
| `BranchList.tsx` | 活跃分支列表 | `/dashboard/branches` |

**技术选型**:
- 热力图: CSS Grid + Tailwind 色阶 (已在 TimelinePage 验证)
- 饼图/图表: 纯 CSS 或 recharts (按需引入)
- 活动流: 列表 + 自动刷新 (setInterval 10s)

**验收标准**:
- DashboardPage 使用独立组件拼装
- 每个组件可在其他页面复用

---

### F-7: Issue 甘特图 (Gantt)

**优先级**: P2
**服务**: forge/web
**文件**: `src/pages/TimelinePage.tsx` (增强)

在 TimelinePage 中新增 **Issue 甘特图** tab，按 Sprint 分组展示 Issue 时间线:

```
Sprint 1: 向量基础 (3/25 - 3/27)
  ████ SC-51 embedding 字段     (3/25 - 3/25)
  ██████ SC-52 Provider 接口    (3/25 - 3/26)
      ████████ SC-53 自动生成    (3/26 - 3/27) ← 依赖 51+52
```

**数据**: 从 `/api/projects/:id/sprints` + `/api/projects/:id/issues` 组合。

**验收标准**:
- Issue 按时间线水平排列，不同状态不同颜色
- 依赖关系用箭头/线连接
- 可切换 Sprint 查看

---

### F-8: 生产部署上线

**优先级**: P0
**服务**: 部署/运维
**文件**: `deploy/nginx-forge.conf`, `nydus/hooks/post-receive`

| 步骤 | 内容 |
|------|------|
| 8a | 配置 nginx: `forge.starclaw.net` → forge-web:3099, `/api` → forge-api:8099 |
| 8b | Nydus `post-receive` hook 加入 `forge/` 变更检测 → 自动 build + deploy |
| 8c | `.env.production` 配置: NYDUS_URL, NYDUS_SECRET, OVERLORD_URL, LLM_BASE_URL |
| 8d | 在 Nydus 注册 webhook → forge-api:8099/api/webhooks/nydus |
| 8e | 验收: 浏览器访问 `forge.starclaw.net` 可用 |

**验收标准**:
- `forge.starclaw.net` 可访问，登录正常
- `git push nydus master` 带 forge/ 变更 → 自动部署
- 大屏数据实时更新

---

## 四、非功能需求

| 项目 | 要求 |
|------|------|
| **响应时间** | Dashboard API < 500ms, Webhook 处理 < 2s |
| **可用性** | 服务健康轮询不影响主 API 性能 (goroutine + 缓存) |
| **安全** | Webhook HMAC 签名验证, API Bearer token 认证 |
| **兼容** | Claw/Overlord proxy 模式向后兼容 (无 FORGE_URL 时走本地) |

---

## 五、实施计划

### Sprint 1: 数据管道 (2天)

| ID | Task | 类型 | 服务 | Points |
|----|------|------|------|--------|
| SC-101 | Webhook 接收器 — Nydus (push/pr/deploy) | code | forge/api | 3 |
| SC-102 | Webhook 接收器 — Issue 自动关联 (commit msg 解析 SC-XX) | code | forge/api | 2 |
| SC-103 | 服务健康聚合 (health.go + 定时轮询 + 缓存) | code | forge/api | 3 |
| SC-104 | Overlord 聚合 (overlord.go + DevClaw 实例/Mission) | code | forge/api | 2 |
| SC-105 | Dev Bridge 双向回写 (Issue transition → task_update) | code | forge/api | 2 |

### Sprint 2: 可视化增强 (2天)

| ID | Task | 类型 | 服务 | Points |
|----|------|------|------|--------|
| SC-106 | ServiceStatusGrid 组件 (服务健康网格) | code | forge/web | 2 |
| SC-107 | CommitHeatmap 组件 (GitHub 风格贡献图) | code | forge/web | 2 |
| SC-108 | DeployTimeline + ActivityFeed 组件 | code | forge/web | 2 |
| SC-109 | IssuePieChart + BranchList 组件 | code | forge/web | 2 |
| SC-110 | DashboardPage 重构 (使用独立组件) | code | forge/web | 2 |
| SC-111 | Issue 甘特图 (TimelinePage 增强) | code | forge/web | 3 |

### Sprint 3: 上线 (1天)

| ID | Task | 类型 | 服务 | Points |
|----|------|------|------|--------|
| SC-112 | nginx 配置 forge.starclaw.net | config | deploy | 1 |
| SC-113 | Nydus hook 集成 forge/ 自动部署 | config | nydus | 2 |
| SC-114 | .env.production + Nydus webhook 注册 | config | deploy | 1 |
| SC-115 | GitHub CI 聚合 (github.go, 可选) | code | forge/api | 2 |
| SC-116 | 上线验收 + 修 bug | test | all | 2 |

---

## 六、依赖关系

```
SC-101 ──→ SC-102 (webhook 先于 issue 关联)
SC-103 ──→ SC-106 (health API 先于前端组件)
SC-104 ──→ SC-110 (overlord 数据先于大屏重构)
SC-105 (独立)
SC-106~111 (前端组件可并行)
SC-112~114 (部署配置可并行)
SC-116 依赖全部完成
```

---

## 七、风险

| 风险 | 影响 | 缓解 |
|------|------|------|
| Nydus webhook 格式未标准化 | 事件解析出错 | 先查看现有 Nydus webhook 代码确认 payload |
| GitHub Token 权限不足 | CI 状态获取失败 | GitHub CI 聚合标记 P2，可延后 |
| Server C 资源不足 | forge 容器启动失败 | forge 很轻量 (SQLite, 单 Go binary)，风险低 |
| Ollama 本地模型响应慢 | PRD 生成超时 | 已配 300s timeout, qwen3-coder:30b 实测够快 |

---

## 八、交付物

- [ ] `forge/api/internal/handler/webhook.go` — 3 个 webhook 端点
- [ ] `forge/api/internal/aggregator/health.go` — 服务健康轮询
- [ ] `forge/api/internal/aggregator/overlord.go` — Overlord 聚合
- [ ] `forge/api/internal/aggregator/github.go` — GitHub CI 聚合 (P2)
- [ ] `forge/web/src/components/` — 6 个可视化组件
- [ ] `forge/web/src/pages/TimelinePage.tsx` — 甘特图增强
- [ ] `forge/deploy/nginx-forge.conf` — nginx 配置
- [ ] `nydus/hooks/post-receive` — forge 部署集成
- [ ] `forge.starclaw.net` 线上可访问

**预计工期**: 5 天 (3 个 Sprint)
**总 Story Points**: 31
