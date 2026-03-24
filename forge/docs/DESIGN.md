# Forge 🔥 熔炉 — 全局研发管控 + 可视化大屏

> 虫群的战略指挥中心 — 所有项目、任务、进度在此熔炼
> 类似: Jira + Linear + GitHub Projects + Grafana Dashboard

---

## 一、定位

```
starclaw/
├── claw/       🦞 小龙虾 — AI Agent 执行引擎
├── synapse/    ⛽ 突触   — AI 算力网关
├── queen/      👑 虫后   — 中央管控
├── overlord/   👁️ 领主   — 企业管控
├── forge/      🔥 熔炉   — 研发管控 + 可视化大屏
├── nydus/      🕳️ 虫道   — 部署管道
├── spore/      🍄 孢子   — 桌面安装器
├── cerebrate/  🧠 脑虫   — 合伙人生态
├── larva/      🐛 幼虫   — 跨平台客户端
└── carapace/   🛡️ 甲壳   — 加密库
```

### 在虫族层级中的位置

```
                     👑 Queen（虫后）
                          │
     ┌──────────┬─────────┼─────────┬──────────┐
     │          │         │         │          │
👁️ Overlord  🔥 Forge  ⛽ Synapse  🧠 Cerebrate  🐛 Larva
 企业管控     研发管控    算力网关    合伙人生态    用户客户端
 节点/RBAC   项目/看板   LLM路由    拓客/部署     iOS/Android
 订阅计费    Issue/PR    计费/代理
             大屏/燃尽图
```

### 两个使用场景

| 场景 | 用户 | 说明 |
|------|------|------|
| **StarClaw 自身研发** | 创始团队 + AI 军团 | 管理 monorepo 的功能/Bug/Sprint，可视化 12 个服务的健康和进度 |
| **Claw 用户的项目** | 企业用户 + DevClaw 团队 | 团队智能体用 Forge 管理它们开发的软件项目（天气 App、CRM 等） |

---

## 二、全景架构

```
┌─────────────────────────────────────────────────────────────────┐
│                        🔥 Forge                                 │
│              forge-api (:8099)  +  forge-web (:3099)            │
│                                                                 │
│   ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌───────────┐     │
│   │ 项目管理  │  │ Issue/PR │  │ 看板/Sprint│  │ 可视化大屏 │     │
│   │ Projects │  │ Issues   │  │ Board    │  │ Dashboard │     │
│   └────┬─────┘  └────┬─────┘  └────┬─────┘  └─────┬─────┘     │
│        │              │              │              │           │
│        └──────────────┴──────────────┴──────────────┘           │
│                         聚合层                                   │
│   ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐      │
│   │ Nydus    │  │DevBridge │  │ Overlord │  │ GitHub   │      │
│   │ Git/PR   │  │ MCP Tasks│  │ 团队实例  │  │ Actions  │      │
│   │ CI/CD    │  │ 分支管理  │  │ DevClaw  │  │ CI 状态  │      │
│   └──────────┘  └──────────┘  └──────────┘  └──────────┘      │
└─────────────────────────────────────────────────────────────────┘
         │              │              │              │
         ▼              ▼              ▼              ▼
   Nydus (:8095)  Dev Bridge    Overlord         GitHub API
   starclaw.net    (:9102)     (:8098)
```

---

## 三、目录结构

```
forge/
├── api/                              # Go 后端
│   ├── cmd/
│   │   └── server/main.go            # 入口
│   ├── internal/
│   │   ├── config/config.go          # 配置
│   │   ├── model/                    # 数据模型
│   │   │   ├── project.go            # ForgeProject
│   │   │   ├── issue.go              # ForgeIssue + Comment
│   │   │   ├── sprint.go             # Sprint + Milestone
│   │   │   └── activity.go           # ActivityLog (全局活动流)
│   │   ├── handler/                  # HTTP handlers
│   │   │   ├── project.go            # 项目 CRUD
│   │   │   ├── issue.go              # Issue CRUD + 状态流转
│   │   │   ├── board.go              # 看板视图 API
│   │   │   ├── sprint.go             # Sprint 迭代管理
│   │   │   ├── dashboard.go          # 可视化大屏 API
│   │   │   ├── webhook.go            # 接收 Nydus/GitHub webhook
│   │   │   └── search.go             # 全局搜索
│   │   ├── engine/                   # AI 引擎
│   │   │   ├── decompose.go          # AI 需求分解为 Issues
│   │   │   ├── assign.go             # AI 自动分配
│   │   │   └── triage.go             # AI 自动分类
│   │   └── aggregator/               # 数据聚合层
│   │       ├── nydus.go              # Nydus (git commit/PR/deploy)
│   │       ├── devbridge.go          # Dev Bridge (:9102) 任务同步
│   │       ├── github.go             # GitHub Actions CI 状态
│   │       ├── overlord.go           # Overlord 团队实例/DevClaw
│   │       └── health.go             # 各服务 /health 聚合
│   ├── Dockerfile
│   ├── go.mod
│   └── go.sum
│
├── web/                              # React 前端 (可视化大屏)
│   ├── src/
│   │   ├── pages/
│   │   │   ├── DashboardPage.tsx     # 🔥 可视化大屏 (主页)
│   │   │   ├── ProjectsPage.tsx      # 项目列表
│   │   │   ├── ProjectDetailPage.tsx # 项目详情 (Board + Issues)
│   │   │   ├── BoardPage.tsx         # 看板视图
│   │   │   ├── IssueDetailPage.tsx   # Issue 详情
│   │   │   ├── SprintPage.tsx        # Sprint 迭代视图
│   │   │   ├── TimelinePage.tsx      # 甘特图 / 路线图
│   │   │   ├── CodeActivityPage.tsx  # 代码活跃度 (热力图)
│   │   │   └── ReportsPage.tsx       # 统计报表
│   │   ├── components/
│   │   │   ├── ServiceStatusGrid.tsx # 12 服务健康状态网格
│   │   │   ├── BurndownChart.tsx     # Sprint 燃尽图
│   │   │   ├── ActivityHeatmap.tsx   # Git 提交热力图
│   │   │   ├── DeployTimeline.tsx    # 部署历史时间线
│   │   │   ├── BranchTree.tsx        # 活跃分支拓扑图
│   │   │   ├── IssuePieChart.tsx     # Issue 分类饼图
│   │   │   └── ActivityFeed.tsx      # 实时活动流
│   │   ├── App.tsx
│   │   └── main.tsx
│   ├── Dockerfile
│   ├── package.json
│   └── vite.config.ts
│
├── docs/
│   └── DESIGN.md                     # 本文件
├── docker-compose.yml
└── README.md
```

---

## 四、数据模型

### 4.1 ForgeProject — 项目

```go
type ForgeProject struct {
    ID          string     `json:"id"`
    Name        string     `json:"name"`            // "StarClaw Core" / "天气App"
    Key         string     `json:"key"`             // "SC" / "WEATHER" (Issue前缀: SC-1, SC-2)
    Description string     `json:"description"`
    OwnerType   string     `json:"owner_type"`      // monorepo / team / personal
    OwnerID     string     `json:"owner_id"`        // squad_id 或 user_id
    NydusRepo   string     `json:"nydus_repo"`      // 关联 Nydus 仓库
    Status      string     `json:"status"`          // active / archived
    Visibility  string     `json:"visibility"`      // public / team / private
    Tags        string     `json:"tags"`            // JSON: ["backend", "frontend"]
    CreatedAt   time.Time  `json:"created_at"`
    UpdatedAt   time.Time  `json:"updated_at"`
}
```

对于 StarClaw 自身研发：
```
Project: "StarClaw Core" (key: SC)
  OwnerType: monorepo
  NydusRepo: starclaw
  → SC-1, SC-2, SC-3... 是 monorepo 的全局 Issue
```

### 4.2 ForgeIssue — 工单

```go
type ForgeIssue struct {
    ID            string      `json:"id"`
    ProjectID     string      `json:"project_id"`
    Number        int         `json:"number"`          // 项目内自增
    Key           string      `json:"key"`             // "SC-42" (自动生成)
    Title         string      `json:"title"`
    Body          string      `json:"body"`            // Markdown
    Type          string      `json:"type"`            // epic / story / task / bug / improvement
    Priority      string      `json:"priority"`        // critical / high / medium / low
    Status        string      `json:"status"`          // backlog / todo / in_progress / review / done / closed
    Assignee      string      `json:"assignee"`        // 人名 / AI角色 / claw:xxx / windsurf-1
    Reporter      string      `json:"reporter"`        // 创建者
    Service       string      `json:"service"`         // claw / queen / synapse / forge / ... (monorepo 服务)
    SprintID      string      `json:"sprint_id"`
    MilestoneID   string      `json:"milestone_id"`
    ParentID      string      `json:"parent_id"`       // Epic → Story → Task 层级
    Labels        string      `json:"labels"`          // JSON: ["backend","api"]
    StoryPoints   int         `json:"story_points"`
    Branch        string      `json:"branch"`          // 关联 Git 分支
    PRNumber      int         `json:"pr_number"`       // 关联 Nydus PR
    DevBridgeTask string      `json:"devbridge_task"`  // 关联 Dev Bridge 任务 ID
    DependsOn     string      `json:"depends_on"`      // JSON: ["issue-id"]
    DueDate       *time.Time  `json:"due_date"`
    ClosedAt      *time.Time  `json:"closed_at"`
    CreatedAt     time.Time   `json:"created_at"`
    UpdatedAt     time.Time   `json:"updated_at"`
}
```

### 4.3 ForgeSprint — 迭代

```go
type ForgeSprint struct {
    ID          string     `json:"id"`
    ProjectID   string     `json:"project_id"`
    Name        string     `json:"name"`           // "Sprint 2026-W13"
    Goal        string     `json:"goal"`           // Sprint 目标描述
    Status      string     `json:"status"`         // planned / active / completed
    StartDate   *time.Time `json:"start_date"`
    EndDate     *time.Time `json:"end_date"`
    Velocity    int        `json:"velocity"`       // 已完成 story points
    CreatedAt   time.Time  `json:"created_at"`
}
```

### 4.4 ForgeMilestone — 里程碑

```go
type ForgeMilestone struct {
    ID          string     `json:"id"`
    ProjectID   string     `json:"project_id"`
    Title       string     `json:"title"`          // "v2026.04 Release"
    Description string     `json:"description"`
    DueDate     *time.Time `json:"due_date"`
    Status      string     `json:"status"`         // open / closed
    Progress    int        `json:"progress"`       // 0-100%
    CreatedAt   time.Time  `json:"created_at"`
}
```

### 4.5 ForgeActivity — 活动流

```go
type ForgeActivity struct {
    ID        string    `json:"id"`
    ProjectID string    `json:"project_id"`
    Type      string    `json:"type"`     // commit / pr / issue / deploy / ci / devbridge / devclaw
    Actor     string    `json:"actor"`    // 人/AI/服务
    Summary   string    `json:"summary"`  // "merged feat/claw/memory-v2 → master"
    Detail    string    `json:"detail"`   // JSON 详情
    Service   string    `json:"service"`  // claw / queen / ...
    Source    string    `json:"source"`   // nydus / github / devbridge / overlord
    CreatedAt time.Time `json:"created_at"`
}
```

---

## 五、API 设计

### 5.1 项目管理

```
POST   /api/projects                  创建项目
GET    /api/projects                  项目列表
GET    /api/projects/:id              项目详情 (含统计)
PUT    /api/projects/:id              更新项目
DELETE /api/projects/:id              归档项目
```

### 5.2 Issue 工单

```
POST   /api/projects/:id/issues      创建 Issue
GET    /api/projects/:id/issues      Issue 列表 (?status=&assignee=&service=&sprint=&type=)
GET    /api/issues/:key               按 Key 查询 (SC-42)
GET    /api/issues/:id                Issue 详情
PUT    /api/issues/:id                更新 Issue
POST   /api/issues/:id/transition     状态流转 { status: "in_progress" }
POST   /api/issues/:id/assign         分配 { assignee: "windsurf-1" }
POST   /api/issues/:id/comments       添加评论
GET    /api/issues/:id/comments       评论列表
POST   /api/issues/:id/link-branch    关联 Git 分支
POST   /api/issues/:id/link-pr        关联 PR
```

### 5.3 看板

```
GET    /api/projects/:id/board        看板视图 (Issues 按 status 列分组)
PUT    /api/projects/:id/board        更新看板配置 (列/泳道)
PATCH  /api/board/move                拖拽移动 { issue_id, new_status, position }
```

### 5.4 Sprint

```
POST   /api/projects/:id/sprints      创建 Sprint
GET    /api/projects/:id/sprints      Sprint 列表
PUT    /api/sprints/:id               更新 Sprint (开始/结束)
GET    /api/sprints/:id/burndown      燃尽图数据
GET    /api/sprints/:id/report        Sprint 报告 (完成率/velocity)
```

### 5.5 可视化大屏

```
GET    /api/dashboard                  大屏总览
GET    /api/dashboard/services         12 服务健康状态
GET    /api/dashboard/activity         最近活动流
GET    /api/dashboard/branches         活跃分支列表
GET    /api/dashboard/ci               CI 构建状态
GET    /api/dashboard/deploys          最近部署记录
GET    /api/dashboard/heatmap          提交热力图 (7天/30天)
GET    /api/dashboard/stats            统计数据 (Issue/PR/commit 计数)
GET    /api/dashboard/devclaws         DevClaw 实例状态
```

### 5.6 Webhook 接收

```
POST   /api/webhooks/nydus            接收 Nydus 事件 (push/pr/deploy)
POST   /api/webhooks/github           接收 GitHub Actions 事件
POST   /api/webhooks/devbridge        接收 Dev Bridge 任务变更
```

### 5.7 AI 自动化

```
POST   /api/projects/:id/ai/decompose     AI 需求分解为 Issues
POST   /api/issues/:id/ai/auto-assign     AI 自动分配到最佳角色
POST   /api/projects/:id/ai/sprint-plan   AI 规划 Sprint
POST   /api/projects/:id/ai/retrospective AI Sprint 回顾总结
```

---

## 六、可视化大屏设计

### 6.1 布局

```
┌─────────────────────────────────────────────────────────────────────┐
│  🔥 StarClaw Forge — 研发指挥中心                     v2026.03  [⛶]  │
├──────────────────┬──────────────────┬───────────────────────────────┤
│                  │                  │                               │
│  服务健康地图      │  Sprint 燃尽图    │     本周活跃度热力图             │
│                  │                  │                               │
│  🟢 Claw API    │  ████████████   │  Mon ██████████               │
│  🟢 Claw Web    │  ██████████     │  Tue ████████                 │
│  🟢 Queen API   │  ████████       │  Wed ████████████████         │
│  🟢 Synapse     │  ██████         │  Thu ██████████               │
│  🟡 Hive        │                  │  Fri ████                     │
│  🟢 Overlord    │  Sprint W13      │                               │
│  🟢 Nydus       │  Velocity: 24pt  │  52 commits / 8 PRs          │
│  🟢 Forge       │  Done: 73%       │  3 deploys                    │
│  🟢 Spore       │                  │                               │
│                  │                  │                               │
├──────────────────┴──────────────────┴───────────────────────────────┤
│                                                                     │
│  看板 (Board)  [Backlog: 5]  [Todo: 3]  [Doing: 4]  [Done: 11]    │
│  ┌───────────┐ ┌───────────┐ ┌───────────┐ ┌───────────┐          │
│  │ SC-20     │ │ SC-15     │ │ SC-18     │ │ SC-10     │          │
│  │ 记忆向量化 │ │ Arena排名 │ │ 熔炉大屏  │ │ 并行开发  │          │
│  │ claw/     │ │ queen/    │ │ forge/    │ │ ✅ merged  │          │
│  │ 🔴 high   │ │ 🟡 medium │ │ 🔵 WS-1  │ │           │          │
│  ├───────────┤ ├───────────┤ ├───────────┤ ├───────────┤          │
│  │ SC-21     │ │ SC-16     │ │ SC-19     │ │ SC-11     │          │
│  │ 市场V2    │ │ 计费网关  │ │ DC-2 🤖  │ │ CI 工作流 │          │
│  │ claw/     │ │ queen/    │ │ 翻译虫    │ │ ✅ merged  │          │
│  └───────────┘ └───────────┘ └───────────┘ └───────────┘          │
│                                                                     │
├─────────────────────────────────────────────────────────────────────┤
│  最近活动                                                            │
│  14:30 🔀 WS-1 merged feat/claw/memory-v2 → master  [claw]        │
│  14:25 🤖 DC-1 published 药理虫 Agent  [overlord]                   │
│  14:20 ✅ CI passed: queen-api, synapse-api  [github]               │
│  13:50 🚀 Deployed claw/ → Server A  [nydus]                       │
│  13:40 📝 WS-2 pushed 3 commits to feat/queen/arena  [nydus]       │
│  13:00 📋 Task T003 created: 实现 medical_catalog  [devbridge]      │
└─────────────────────────────────────────────────────────────────────┘
```

### 6.2 大屏数据源

| 区块 | 数据源 | 刷新频率 |
|------|--------|----------|
| **服务健康地图** | 各服务 /health 端点 | 30s |
| **Sprint 燃尽图** | Forge DB (Issue status changes) | 实时 |
| **活跃度热力图** | Nydus API (commit history) | 5min |
| **看板** | Forge DB (Issues) | 实时 |
| **活动流** | Forge ActivityLog | Webhook 推送 |
| **CI 状态** | GitHub Actions API | 1min |
| **分支列表** | Nydus API / Dev Bridge | 1min |
| **DevClaw 实例** | Overlord API | 1min |

---

## 七、数据聚合层

Forge 不直接管代码/部署，而是**聚合**各模块的数据：

### 7.1 Nydus 集成

```
Nydus (:8095) ──webhook──→ Forge (:8099)

事件:
  push         → ForgeActivity (type: commit)
  pr.opened    → ForgeActivity (type: pr) + 关联 Issue
  pr.merged    → ForgeActivity + Issue 自动 → done
  deploy       → ForgeActivity (type: deploy)
  release      → ForgeMilestone 进度更新
```

### 7.2 Dev Bridge 集成

```
Dev Bridge (:9102) ←→ Forge (:8099)

同步方向:
  Dev Bridge task_create → 自动创建 Forge Issue (type: task, assignee 映射)
  Forge Issue 状态变更   → 同步回 Dev Bridge task_update
  Dev Bridge git_create_branch → Forge Issue 关联 branch
  Dev Bridge git_merge   → Forge Issue 自动 → done
```

### 7.3 Overlord 集成

```
Overlord (:8098) ──API──→ Forge (:8099)

聚合:
  GET /brood/team-agent/instances → DevClaw 实例列表和状态
  GET /brood/team-agent/missions  → Mission 进度
  Forge 大屏显示 DevClaw 工作进展
```

### 7.4 GitHub Actions 集成

```
GitHub ──webhook──→ Forge (:8099)

事件:
  check_run.completed  → CI 状态更新
  workflow_run.completed → 构建结果
  Forge 大屏显示 CI 状态徽章
```

### 7.5 服务健康检查

```
Forge 定期轮询:
  GET claw-api:8080/health       → Claw API 状态
  GET queen-api:8085/health      → Queen API 状态
  GET synapse-api:8096/health    → Synapse API 状态
  GET overlord-api:8098/health   → Overlord 状态
  GET nydus-api:8095/health      → Nydus 状态
  GET hive:9090/health           → Hive 状态
  ...
  → 汇总到 /api/dashboard/services
```

---

## 八、与现有 Forge 代码的关系

### 现状

| 位置 | 内容 | 迁移计划 |
|------|------|----------|
| `claw/api/internal/model/forge.go` | ForgeProject/Issue/Comment 模型 | → 迁移到 `forge/api/internal/model/` 并增强 |
| `claw/api/internal/api/v1/forge.go` | Forge CRUD API | → 迁移到 `forge/api/internal/handler/` |
| `claw/api/internal/forge/bridge.go` | Nydus Bridge | → 迁移到 `forge/api/internal/aggregator/nydus.go` |
| `claw/api/internal/forge/webhook_receiver.go` | Webhook | → 迁移到 `forge/api/internal/handler/webhook.go` |
| `claw/api/internal/forge/nydus_client.go` | Nydus HTTP client | → 迁移到 `forge/api/internal/aggregator/` |
| `overlord/api/internal/handler/forge.go` | Nydus proxy | → 改为 proxy 到 forge-api |
| `overlord/console/src/pages/ForgePage.tsx` | 基础 UI | → 保留，增加 iframe 嵌入 forge-web 大屏 |

### 迁移策略: 渐进式

```
Phase 0 (当前): Forge 代码散在 Claw + Overlord 里
Phase 1: 创建 forge/ 独立服务，先做大屏 + 聚合层
Phase 2: 逐步迁移 Claw 内的 Forge 模型和 API 到 forge/
Phase 3: Claw 的 /v1/forge/* 变为 proxy 到 forge-api
Phase 4: 完整独立，Claw/Overlord 只做客户端
```

---

## 九、与 Dev Bridge 的关系

```
Dev Bridge (:9102)                Forge (:8099)
──────────────────               ──────────────
MCP 工具 (编辑器直接调用)          Web 平台 (浏览器可视化)
JSON 文件存储                     SQLite/PostgreSQL
15 个开发工具                     完整项目管理
短期任务 (DevClaw↔编辑器)          长期项目 (Sprint/Milestone)
面向 AI Agent                    面向人 + AI

    Dev Bridge tasks ──同步──→ Forge Issues
    Forge Issues ──同步──→ Dev Bridge tasks (可选)
    Dev Bridge git ops ──webhook──→ Forge Activity
```

**分工明确**: Dev Bridge 是 MCP 工具层（AI Agent 和编辑器调用），Forge 是 Web 可视化层（人看大屏和管理项目）。

---

## 十、部署

### 10.1 服务配置

| 服务 | 端口 | 服务器 | 域名 |
|------|------|--------|------|
| forge-api | :8099 | Server C (starclaw.net) | forge.starclaw.net/api |
| forge-web | :3099 | Server C (starclaw.net) | forge.starclaw.net |

### 10.2 Nydus 部署集成

```
# nydus hook 新增 forge/ 检测:
forge/ 变更 → Server C 本地 docker compose build forge-api forge-web && up -d
```

### 10.3 docker-compose.yml

```yaml
services:
  forge-api:
    build: ./api
    ports: ["8099:8099"]
    environment:
      - FORGE_DB_PATH=/data/forge.db
      - NYDUS_URL=https://nydus.starclaw.net
      - NYDUS_SECRET=${NYDUS_SECRET}
      - OVERLORD_URL=http://overlord-api:8098
      - DEVBRIDGE_URL=http://localhost:9102
      - GITHUB_TOKEN=${GITHUB_TOKEN}
    volumes:
      - forge-data:/data

  forge-web:
    build: ./web
    ports: ["3099:80"]

volumes:
  forge-data:
```

---

## 十一、实施分期

### Phase 1: 基础框架 + 大屏 (3天)

| Task | 内容 |
|------|------|
| F-1 | 创建 `forge/api/` Go 项目骨架 (go.mod, main.go, config, model) |
| F-2 | 数据模型: Project + Issue + Sprint + Milestone + Activity |
| F-3 | 基础 CRUD: Project + Issue |
| F-4 | 聚合层: health checker (12 服务轮询) |
| F-5 | 大屏 API: /api/dashboard/* |
| F-6 | 创建 `forge/web/` React 项目 + 大屏 UI 组件 |

### Phase 2: 看板 + Sprint (3天)

| Task | 内容 |
|------|------|
| F-7 | Board API + 拖拽排序 |
| F-8 | Sprint CRUD + 燃尽图 |
| F-9 | 看板前端 (DnD 拖拽) |
| F-10 | Sprint 页面 + 燃尽图组件 |

### Phase 3: 数据聚合 (3天)

| Task | 内容 |
|------|------|
| F-11 | Nydus Webhook 接收 (push/PR/deploy) |
| F-12 | GitHub Actions CI 状态聚合 |
| F-13 | Dev Bridge 任务双向同步 |
| F-14 | Overlord DevClaw 实例聚合 |
| F-15 | 提交热力图 + 活动流 |

### Phase 4: AI 引擎 + 迁移 (4天)

| Task | 内容 |
|------|------|
| F-16 | AI 需求分解 (Issue 自动拆分) |
| F-17 | AI Sprint 规划 |
| F-18 | 从 Claw 迁移 Forge 模型和 API |
| F-19 | Overlord ForgeHandler 改为 proxy |
| F-20 | 甘特图 / 路线图页面 |

### Phase 5: 部署 + 上线 (1天)

| Task | 内容 |
|------|------|
| F-21 | Dockerfile + docker-compose |
| F-22 | Nydus hook 加入 forge/ 检测 |
| F-23 | Nginx 路由配置 |
| F-24 | 上线验收 |

**总计: ~14 天**

---

## 十二、StarClaw 自身项目管理示例

上线后，首先为 StarClaw monorepo 创建项目：

```
Project: "StarClaw Core" (key: SC)
  OwnerType: monorepo
  NydusRepo: starclaw

Sprint: "2026-W13" (3/24 - 3/30)
  SC-1  [epic]  并行开发体系                     ✅ done
  SC-2  [task]  CI workflow (ci.yml)             ✅ done
  SC-3  [task]  Dev Bridge MCP                   ✅ done
  SC-4  [task]  parallel-dev.md 文档              ✅ done
  SC-5  [epic]  Forge 熔炉系统                    🔨 in_progress
  SC-6  [task]  Forge 设计文档                    🔨 in_progress
  SC-7  [task]  Forge API 基础框架                ⏳ todo
  SC-8  [task]  Forge 可视化大屏                   ⏳ todo
  SC-9  [task]  记忆系统 v2 向量化                  ⏳ backlog
  SC-10 [task]  Arena 排名算法                     ⏳ backlog
  SC-11 [bug]   Synapse timeout 偶发              ⏳ backlog
```

---

## 十三、一键开发流程 — PRD → Sprint → 自动调度

> **核心理念**: 你只需输入需求，Forge 自动生成 PRD、拆分 Sprint、调度 AI 军团开发。
> **当前阶段**: 主 Windsurf 会话作为调度大脑（人 + AI 协作）
> **终极形态**: 替换为最优秀的 LLM 作为自治调度引擎

### 13.1 完整链路

```
你: "实现记忆系统向量化"
  │
  ▼ ① PRD 生成
Forge AI → 结构化 PRD (目标/功能/非功能/验收标准)
  │
  ▼ ② 你确认 PRD [确认] / [修改]
  │
  ▼ ③ Sprint 拆分
Forge AI → Epic → Sprint × N → Issue × M (含依赖关系/服务标注/Story Points)
  │
  ▼ ④ 你确认计划 [🚀 一键开始]
  │
  ▼ ⑤ Orchestrator 自动调度
  │
  ├── 代码任务 → Dev Bridge → Windsurf/Cursor (分支开发)
  ├── Agent 任务 → Overlord → DevClaw 实例 (五虫协作)
  └── 配置/文档 → Dev Bridge → 直接执行
  │
  ▼ ⑥ 各端完成 → Webhook → Forge 更新看板 → 自动解锁下游任务
  │
  ▼ ⑦ Sprint 完成 → CI/CD 部署 → Sprint 回顾 → 询问启动下一 Sprint
  │
  ▼ ⑧ 所有 Sprint 完成 → Epic 关闭 → 需求交付 ✅
```

### 13.2 PRD 生成器

```
输入: 自然语言需求描述
输出: 结构化 PRD

POST /api/prd/generate
{
  "prompt": "实现 Claw 记忆系统向量化，支持语义搜索，替代现有 LIKE 匹配",
  "project_id": "sc-core"
}

返回:
{
  "id": "prd-001",
  "title": "记忆系统 v2 — 向量语义搜索",
  "objective": "将记忆召回从关键词匹配升级为向量语义搜索",
  "features": [
    { "id": "F1", "title": "记忆向量化", "desc": "存储时生成 embedding", "service": "claw/api" },
    { "id": "F2", "title": "语义搜索", "desc": "对话前用 query embedding 召回 top-K", "service": "claw/api" },
    { "id": "F3", "title": "混合召回", "desc": "向量 + 关键词加权融合", "service": "claw/api" },
    { "id": "F4", "title": "前端展示", "desc": "MemoryPage 显示相似度分数", "service": "claw/web" }
  ],
  "non_functional": [
    "延迟 < 200ms (embedding 查询)",
    "存量记忆自动补 embedding",
    "embedding 模型可配置"
  ],
  "acceptance_criteria": [
    "\"我喜欢吃什么\" 能召回 \"用户偏好川菜\"",
    "1000 条记忆搜索 < 100ms",
    "旧数据无缝迁移"
  ],
  "estimated_sprints": 2,
  "services": ["claw/api", "claw/web"]
}
```

### 13.3 Sprint 拆分器

```
POST /api/prd/:id/plan

PRD → AI 自动拆分为:

Epic: SC-50 记忆系统 v2 — 向量语义搜索
│
├── Sprint 1: "向量基础" (3天)
│   ├── SC-51 [task] 添加 embedding 字段到 Memory 模型
│   │   service: claw/api  type: code  points: 2  depends: []
│   ├── SC-52 [task] 实现 EmbeddingProvider 接口
│   │   service: claw/api  type: code  points: 3  depends: []
│   ├── SC-53 [task] 记忆存储时自动生成 embedding
│   │   service: claw/api  type: code  points: 3  depends: [SC-51, SC-52]
│   ├── SC-54 [task] 向量相似度搜索
│   │   service: claw/api  type: code  points: 3  depends: [SC-53]
│   └── SC-55 [task] 存量记忆后台补 embedding
│       service: claw/api  type: code  points: 2  depends: [SC-53]
│
└── Sprint 2: "前端 + 混合召回" (3天)
    ├── SC-56 [task] 混合召回引擎 (向量 + 关键词加权)
    │   service: claw/api  type: code  points: 3  depends: [SC-54]
    ├── SC-57 [task] MemoryPage 显示相似度分数
    │   service: claw/web  type: code  points: 2  depends: [SC-56]
    ├── SC-58 [task] 记忆搜索 API 返回 similarity score
    │   service: claw/api  type: code  points: 2  depends: [SC-54]
    └── SC-59 [task] 性能测试 + 验收
        service: claw/api  type: code  points: 2  depends: [SC-56, SC-57, SC-58]
```

### 13.4 Task Router — 任务路由器

每个 Issue 根据 `type` 和 `service` 自动路由到正确的执行端：

```
┌──────────────────────────────────────────────────────────────┐
│                    Task Router (任务路由器)                    │
│                                                              │
│  type: code + service in monorepo:                           │
│    → Dev Bridge: git_create_branch + task_create             │
│    → 分配给空闲的 Windsurf/Cursor 会话                        │
│                                                              │
│  type: agent / skill / workflow:                             │
│    → Overlord API: 创建 DevClaw Mission                      │
│    → DevClaw 五虫协作 → 沙箱测试 → 上架                       │
│                                                              │
│  type: config / doc:                                         │
│    → Dev Bridge: file_write (简单配置)                        │
│    → 或分配给 Windsurf (复杂文档)                             │
│                                                              │
│  type: design / review / approve:                            │
│    → 通知人类 → Forge 大屏待确认                               │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

### 13.5 Orchestrator — 调度引擎

```go
// forge/api/internal/engine/orchestrator.go

Orchestrator 核心逻辑:

1. 启动 Sprint
   → 扫描所有 Issue，构建依赖 DAG (有向无环图)
   → 找出所有 "无依赖" 的 Issue → 立即调度

2. 并行调度
   → SC-51 (无依赖) → 调度到 Windsurf-1
   → SC-52 (无依赖) → 调度到 Windsurf-2
   → SC-53 (依赖 SC-51+52) → 排队等待

3. 完成回调
   → Webhook: SC-51 done
   → 检查 SC-53 的依赖: SC-51 ✅ SC-52 ❓ → 还不能启动
   → Webhook: SC-52 done
   → 检查 SC-53 的依赖: SC-51 ✅ SC-52 ✅ → 全部满足 → 调度 SC-53

4. Sprint 完成
   → 所有 Issue done → 生成 Sprint 回顾
   → 触发 CI/CD → 通知人类
   → 询问: "Sprint 1 完成，是否开始 Sprint 2？"
```

### 13.6 调度大脑演进路线

```
Phase 0 (当前):
  大脑 = 主 Windsurf 会话 (你 + Cascade AI)
  你看 Forge 大屏 → 手动确认 PRD → 手动点一键开始
  Windsurf 执行代码任务 → 你审查 → 合并
  优点: 人类全程可控
  缺点: 需要你在线

Phase 1 (近期):
  大脑 = Forge Orchestrator (Go 代码)
  自动调度已确认的 Sprint
  依赖解锁 + 并行派发 + Webhook 回调
  人类确认点: PRD / Sprint 启动 / 代码审查
  优点: 减少人类操作
  缺点: 调度逻辑硬编码

Phase 2 (中期):
  大脑 = Forge Orchestrator + LLM 决策
  LLM 参与: PRD 生成 / Sprint 规划 / 任务分配 / 代码审查
  人类确认点: 只审核关键节点
  优点: 更智能的决策
  缺点: LLM 成本

Phase 3 (终极):
  大脑 = 最优秀的 LLM (GPT-5 / Claude / 未来模型)
  全自治: 需求输入 → 交付产出，人类只做最终验收
  Forge 大屏从"操作台"变为"监控台"
  人类: 战略决策 + 最终验收
  AI: 所有执行 + 战术决策

  ┌─────────────────────────────────┐
  │          你 (CEO)               │
  │    "下个月做 XX 功能"            │
  │    "Q2 目标是 YY"               │
  └──────────────┬──────────────────┘
                 │ 战略指令
                 ▼
  ┌─────────────────────────────────┐
  │       LLM 调度大脑               │
  │  PRD → Sprint → 调度 → 监控     │
  │  审查 → 部署 → 回顾 → 优化      │
  └──────────────┬──────────────────┘
                 │ 自动调度
        ┌────────┼────────┐
        ▼        ▼        ▼
    Windsurf  DevClaw   Cursor
    (代码)    (Agent)   (代码)
```

### 13.7 人在回路 (Human-in-the-loop)

即使到终极形态，保留 3 个人工确认点确保安全：

| 确认点 | 阶段 | 为什么需要人类 |
|--------|------|---------------|
| **PRD 确认** | 需求定义后 | 确保 AI 理解的需求和你想要的一致 |
| **Sprint 启动** | 拆分计划后 | 确认优先级、排期、资源分配合理 |
| **部署审批** | Sprint 完成后 | 代码审查通过 + 验收测试通过才上线 |

### 13.8 新增 API

```
── PRD ──────────────────────────────────────────
POST   /api/prd/generate              AI 生成 PRD { prompt: "..." }
GET    /api/prd/:id                   PRD 详情
PUT    /api/prd/:id                   修改 PRD
POST   /api/prd/:id/confirm           确认 PRD

── Sprint 规划 ──────────────────────────────────
POST   /api/prd/:id/plan              AI 拆分为 Sprint + Issues
PUT    /api/prd/:id/plan              调整 Sprint 计划
POST   /api/prd/:id/plan/confirm      确认计划

── Sprint 执行 ──────────────────────────────────
POST   /api/sprints/:id/start         🚀 一键开始 Sprint
POST   /api/sprints/:id/pause         暂停 Sprint
POST   /api/sprints/:id/resume        恢复 Sprint
GET    /api/sprints/:id/progress      实时进度 (SSE)
POST   /api/sprints/:id/retro         AI 生成 Sprint 回顾

── 调度状态 ──────────────────────────────────────
GET    /api/orchestrator/status        调度器状态 (队列/执行中/完成)
GET    /api/orchestrator/agents        可用 Windsurf/DevClaw 列表
POST   /api/orchestrator/register      编辑器会话注册 (Windsurf/Cursor)
```

### 13.9 Windsurf 会话注册

```
多个编辑器需要向 Forge 注册，才能被调度:

编辑器启动时:
  POST /api/orchestrator/register
  {
    "name": "windsurf-1",
    "type": "windsurf",            // windsurf / cursor / vscode
    "capabilities": ["go", "react", "docker"],
    "services": ["claw/api", "claw/web"],   // 擅长的服务
    "status": "idle"               // idle / busy / offline
  }

Forge 维护可用列表:
  windsurf-1  idle    [go, react]    → 分配 SC-51 (claw/api)
  cursor-1    idle    [go]           → 分配 SC-52 (claw/api)
  windsurf-2  busy    [react]        → 队列中，等完成后分配
```

### 13.10 大屏一键开发视图

```
┌─────────────────────────────────────────────────────────────────┐
│  🔥 Sprint 1: "向量基础"  [🚀 开始] [⏸ 暂停]  进度: 2/5 (40%)  │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  依赖图 (DAG)                          编辑器池                  │
│                                                                 │
│  SC-51 ✅ ──┐                          windsurf-1: SC-53 🔨     │
│              ├──→ SC-53 🔨 ──→ SC-54 ⏳   cursor-1: idle ⏳      │
│  SC-52 ✅ ──┘              └──→ SC-55 ⏳   windsurf-2: offline ❌ │
│                                                                 │
│  ✅ = done    🔨 = running    ⏳ = waiting    ❌ = blocked       │
│                                                                 │
├─────────────────────────────────────────────────────────────────┤
│  活动流                                                          │
│  11:30 ✅ SC-52 done by cursor-1 (3 commits, 2 files)          │
│  11:25 🔨 SC-53 started → windsurf-1 (feat/claw/memory-embed)  │
│  11:00 ✅ SC-51 done by windsurf-1 (1 commit, 1 file)          │
│  10:55 🚀 Sprint started — 2 tasks dispatched in parallel       │
└─────────────────────────────────────────────────────────────────┘
```

---

## 十四、技术栈

| 层 | 技术 |
|----|------|
| **后端** | Go 1.24, Gin, GORM, SQLite (→ PostgreSQL) |
| **前端** | React 19, TypeScript, Vite, TailwindCSS, shadcn/ui |
| **图表** | Recharts (燃尽图/饼图), react-heatmap-grid (热力图) |
| **看板** | @hello-pangea/dnd (拖拽) |
| **时间线** | gantt-task-react (甘特图) |
| **实时** | SSE (Server-Sent Events) 推送活动流 |
