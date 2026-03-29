# Forge + Nydus — AI 原生项目管理与代码协作平台

> 版本: v1.0 | 日期: 2026-03-23 | 状态: 设计确认

## 一、核心洞察

**Squad (战队) = Team Agent (团队智能体)** — 同一事物的两个视角。

- 从 Claw 节点看：这是一个**战队**，多个节点协作
- 从 Overlord 看：这是一个**团队智能体实例**，由管理员创建和管控

一个团队智能体下面有**多个 Claw 节点**，每个节点上跑着不同角色（设计虫、编码虫、测试虫…）。
团队智能体可以**独立工作**，也可以和**其他团队协作**。

## 二、四层架构

```
┌─────────────────────────────────────────────────────────────┐
│                 Overlord (领主 — 企业管理层)                   │
│                                                             │
│  管理多个团队智能体 · 看全局 · 定策略 · 算账                   │
│  节点注册 / 团队创建 / RBAC / 计费 / 合规 / 组织策略          │
└──────────────────────────┬──────────────────────────────────┘
                           │ 创建 & 管控
                           ▼
┌─────────────────────────────────────────────────────────────┐
│          Team Agent / Squad (团队智能体 — 协作层)              │
│                                                             │
│  DevClaw(开发) / MarketClaw(营销) / LegalClaw(法务) ...      │
│  每个实例 = 1个Captain节点 + N个Member节点 + M个角色          │
│  可独立完成项目，也可跨团队协作                                │
│                                                             │
│  项目管理走 → Forge     代码托管走 → Nydus                    │
└──────┬──────────────────────────────┬───────────────────────┘
       │                              │
       ▼                              ▼
┌──────────────────┐    ┌────────────────────────────────────┐
│  Forge (锻炉)     │    │  Nydus (虫道)                       │
│  ≈ Jira           │    │  ≈ GitHub + CI/CD                   │
│                   │    │                                     │
│  每个 Claw 节点上  │    │  中心化部署                          │
│  运行 Forge 模块   │    │  所有团队共享                        │
│                   │    │                                     │
│  Project          │◄──►│  Repo (节点身份 + 权限)              │
│  Issue            │    │  Fork                               │
│  Board (看板)     │    │  Pull Request + Diff                │
│  Sprint           │    │  Code Review                        │
│  Milestone        │    │  Branch Protection                  │
│                   │    │  Webhook → 通知 Forge/Overlord       │
│                   │    │  CI/CD Pipeline                     │
│                   │    │  Release 管理                        │
└──────────────────┘    └────────────────────────────────────┘
```

## 三、统一模型: Squad = Team Agent

### 3.1 当前两套概念的对应关系

| Claw 侧 (Squad) | Overlord 侧 (Team Agent) | 统一后 |
|-----------------|-------------------------|--------|
| Squad | TeamAgentInstance | **同一个**, Squad.ID = Instance.ID |
| SquadMember | Claw Node in Instance | **同一个**, member = node |
| Mission | TeamMission | **同一个**, mission 触发 Forge Sprint |
| MissionStep | — | 演进为 Forge Issue |
| Captain | Instance 的主节点 | **同一个** |
| Member roles | Template 角色定义 | **同一个** |

### 3.2 一个团队智能体的完整结构

```
Team Agent Instance: "DevClaw-产品部" (id: inst-001)
│
├── 模板: DevClaw
├── 管理方: Overlord Admin
│
├── Claw 节点 (Squad Members):
│     ├── Node A (Captain) — 设计虫 Architect
│     ├── Node B — 编码虫 Drone (Backend)
│     ├── Node C — 编码虫 Drone (Frontend)
│     ├── Node D — 测试虫 Tester
│     └── Node E — 审查虫 Reviewer + 文档虫 DocBot
│
├── Forge 项目 (1:N):
│     ├── Project "天气App" → Nydus Repo "weather-app"
│     ├── Project "CRM系统" → Nydus Repo "crm-system"
│     └── Project "内部工具" → Nydus Repo "internal-tools"
│
└── 协作中的其他团队:
      ├── DesignClaw-设计部 → 共同参与 "天气App" 项目
      └── MarketClaw-市场部 → 共同参与 "CRM系统" 项目
```

## 四、完整链路

### 4.1 单团队独立工作

```
Overlord Admin: "产品部 DevClaw，开发一个天气 App"
     │
     ▼
① Overlord 创建 Mission
   → POST /brood/team-agent/instances/inst-001/missions
   → {title: "开发天气App", goal: "...需求描述..."}
     │
     ▼
② Captain (Node A) 接收任务
   → Forge: 创建 Project "天气App"
   → Forge → Nydus: POST /api/repos {name: "weather-app", owner: "claw:nodeA"}
   → Nydus: 创建 bare repo, 返回 git URL
   → Nydus: 为 Squad 所有 Member 授权 write
   → Nydus: 设置 branch protection (main 需 review)
   → Forge: 创建 Sprint #1 (关联 Mission)
     │
     ▼
③ 设计虫 (Node A) 分解需求
   → LLM 分析目标 → 生成 Issue 列表:
     Issue #1 [Epic]  天气App
     Issue #2 [Task]  设计 REST API      → assigned: Node B (Backend)
     Issue #3 [Task]  实现天气页面        → assigned: Node C (Frontend)
     Issue #4 [Task]  数据库模型          → assigned: Node B (Backend)
     Issue #5 [Task]  编写单元测试        → assigned: Node D (Tester)
     Issue #6 [Task]  安全审查            → assigned: Node E (Reviewer)
     Issue #7 [Task]  编写文档            → assigned: Node E (DocBot)
     │
     ▼
④ 编码虫 (Node B) 认领 Issue #2
   → git clone git@nydus:weather-app.git
   → git checkout -b feature/issue-2-api
   → Agent 写代码...
   → git add . && git commit && git push
   → Nydus: POST /api/repos/weather-app/pulls
     {title: "Add weather API", source: "feature/issue-2-api",
      target: "main", linked_issues: ["#2"]}
   → PR #1 created
     │
     ▼
⑤ 审查虫 (Node E) 自动 Review PR #1
   → Nydus: GET /api/repos/weather-app/pulls/1/diff
   → LLM Review (ReviewMatrix: Backend → QA focus)
   → Nydus: POST /api/repos/weather-app/pulls/1/reviews
     {status: "approved", body: "API 设计合理，错误处理完善"}
     │
     ▼
⑥ PR Merged → Webhook 触发
   → Nydus: auto-merge PR #1 into main
   → Nydus → Forge (webhook): {"event": "pr.merged", "pr": 1, "issues": ["#2"]}
   → Forge: Issue #2 状态 → done
   → Forge: Board 自动更新
   → Forge: 检查 Issue #3 的依赖是否满足 → 如满足，通知 Node C 开始
     │
     ▼
⑦ 所有 Issue 完成 → Sprint #1 done
   → Nydus: CI/CD 部署 → 预览 URL
   → Forge → Overlord: 上报 Sprint 完成 + 统计
   → Overlord Console: "天气App 完成, 7/7 Issues, 5 PRs merged, 耗时2天, 消耗18.5⚡"
```

### 4.2 多团队跨团队协作

```
场景: 开发一款游戏
  → GameClaw (策划+美术) 负责游戏设计
  → DevClaw (开发) 负责技术实现
  → MarketClaw (营销) 负责上线推广

── Step 1: Overlord 创建协作项目 ──────────────

Overlord Admin:
  → 创建 Forge Project "space-game" (跨团队项目)
  → Nydus: 创建 Repo "space-game"
  → 授权 3 个团队:
      GameClaw Captain  → admin (项目发起方)
      DevClaw Captain   → write
      MarketClaw Captain → read (后期加入时升 write)

── Step 2: GameClaw 团队工作 ──────────────────

GameClaw 设计虫:
  → Forge: 创建 Issues
    #1 [Epic] 太空游戏
    #2 [Task] 关卡设计文档
    #3 [Task] 角色原画
    #4 [Task] 音效设计

GameClaw 各角色:
  → 认领 Issue → Nydus 分支开发 → 提 PR
  → 产出: 设计文档 / 原画素材 / 音效文件

── Step 3: DevClaw 团队加入 ─────────────────

DevClaw 收到通知: "space-game 项目有新 Issue 需要开发"

DevClaw 设计虫:
  → 读取 GameClaw 已完成的设计文档 (在 Nydus repo main 分支)
  → Forge: 创建开发 Issues
    #10 [Task] 游戏引擎框架
    #11 [Task] 关卡加载器 (depends: #2)
    #12 [Task] 角色渲染 (depends: #3)
    #13 [Task] 音效系统 (depends: #4)

DevClaw 各角色:
  → 从 Nydus clone → 基于 GameClaw 已合并的资源开发
  → 提 PR → GameClaw 审查虫 review 功能完整性
         → DevClaw 审查虫 review 代码质量
  → 两个团队的 Reviewer 都 approve → merge

── Step 4: MarketClaw 团队加入 ────────────────

MarketClaw:
  → Forge: 创建 Issues
    #20 [Task] 应用商店描述
    #21 [Task] 推广素材
    #22 [Task] 上线 checklist

── Step 5: Overlord 全局视图 ─────────────────

Overlord Console 看到:
  Project "space-game":
    ├── 3 个团队参与: GameClaw / DevClaw / MarketClaw
    ├── 22 个 Issue: 15 done / 5 in_progress / 2 todo
    ├── 18 个 PR: 15 merged / 2 open / 1 changes_requested
    ├── Sprint #1: done (设计阶段)
    ├── Sprint #2: in_progress (开发阶段)
    ├── 预览: https://space-game.preview.starclaw.me
    ├── 总消耗: 85.3⚡ (GameClaw 30 + DevClaw 45 + MarketClaw 10)
    └── 预计完成: 3天后
```

### 4.3 Nydus CI/CD 在链路中的位置

```
代码合并到 main 后:

Nydus post-receive hook 触发:
  │
  ├── ① Webhook 通知
  │     → Forge: Issue 状态更新
  │     → Overlord: 进度上报
  │     → 其他订阅者 (Slack/钉钉等)
  │
  ├── ② CI 检查 (新增)
  │     → 运行测试 (如果 repo 配置了 test command)
  │     → 代码质量扫描 (lint)
  │     → 安全扫描 (敏感信息检测)
  │     → 结果写回 PR status check
  │
  ├── ③ CD 部署 (已有, 增强)
  │     → 识别变更目录 → 选择性部署到目标服务器
  │     → 生成预览 URL
  │     → 部署结果写回 Forge Sprint
  │
  └── ④ Release (已有)
        → Tag push → 创建 Release
        → 构建制品 → 上传到 /releases/
        → 通知 Spore 更新检查
```

## 五、数据模型

### 5.1 Nydus 侧 (新增, SQLite)

```go
// NydusNode — Claw 节点注册 (≈ GitHub User)
type NydusNode struct {
    ID          string    // claw:xxxx (Ed25519 公钥哈希)
    Name        string    // 可读名称, 如 "spark6"
    PublicKey   string    // Ed25519 公钥 (验签用)
    SSHPubKey   string    // SSH 公钥 (git push 用)
    Role        string    // owner / member / readonly
    TeamID      string    // 所属 Team Agent Instance ID
    LastSeen    time.Time
    RegisteredAt time.Time
}

// NydusRepo — 仓库 (≈ GitHub Repo, 动态创建, 持久化)
type NydusRepo struct {
    ID             string
    Name           string    // unique, 如 "weather-app"
    OwnerNodeID    string    // 创建者节点
    OwnerTeamID    string    // 所属团队 (可空=个人)
    Visibility     string    // public / private
    DefaultBranch  string    // "main"
    Description    string
    ForkedFrom     string    // fork 源 repo ID (可空)
    Settings       string    // JSON: branch protection, CI config...
    CreatedAt      time.Time
    UpdatedAt      time.Time
}

// RepoAccess — 仓库权限 (≈ GitHub Collaborator)
type RepoAccess struct {
    ID         string
    RepoID     string
    NodeID     string    // 单个节点
    TeamID     string    // 或整个团队 (二选一)
    Permission string    // read / write / admin
    GrantedBy  string
    GrantedAt  time.Time
}

// PullRequest — 合并请求
type PullRequest struct {
    ID            string
    RepoID        string
    Number        int       // 自增: 1, 2, 3...
    Title         string
    Description   string
    SourceBranch  string
    TargetBranch  string
    SourceRepoID  string    // fork PR 时不同于 RepoID
    AuthorNodeID  string    // 提交者节点
    AuthorRole    string    // "Drone-Backend" 等角色名
    Status        string    // open / merged / closed
    LinkedIssues  string    // JSON: ["issue-uuid-1", "issue-uuid-2"]
    MergedBy      string
    MergedAt      *time.Time
    CreatedAt     time.Time
    UpdatedAt     time.Time
}

// PRReview — PR 审查
type PRReview struct {
    ID           string
    PRID         string
    ReviewerNode string
    ReviewerRole string    // "Reviewer", "Tester" 等
    Status       string    // pending / approved / changes_requested
    Body         string    // 审查意见
    CreatedAt    time.Time
}

// PRComment — PR 评论 (支持行内)
type PRComment struct {
    ID         string
    PRID       string
    AuthorNode string
    Body       string
    FilePath   string    // 行内评论: 文件路径 (可空)
    LineNumber int       // 行内评论: 行号 (可空)
    CreatedAt  time.Time
}

// RepoWebhook — 可配置 Webhook
type RepoWebhook struct {
    ID       string
    RepoID   string
    URL      string    // 回调地址
    Events   string    // JSON: ["push", "pr.opened", "pr.merged"]
    Secret   string    // HMAC 签名密钥
    Active   bool
}
```

### 5.2 Claw Forge 侧 (新增)

```go
// ForgeProject — 项目 (长期存在, 关联 Nydus Repo)
type ForgeProject struct {
    ID            string
    Name          string
    Description   string
    TeamID        string    // Squad/TeamAgent Instance ID
    NydusRepoName string    // 对应 Nydus 上的 repo name
    NydusRepoURL  string    // git clone URL
    Status        string    // active / archived
    UserID        string
    CreatedAt     time.Time
    UpdatedAt     time.Time
}

// ForgeIssue — 工单 (演进自 MissionStep)
type ForgeIssue struct {
    ID            string
    ProjectID     string
    Number        int       // 项目内自增: #1, #2, #3
    Type          string    // epic / story / task / bug
    Title         string
    Description   string
    Status        string    // backlog / todo / in_progress / review / done
    Priority      string    // critical / high / medium / low
    AssigneeNode  string    // claw:xxx
    AssigneeRole  string    // "Drone-Backend" 等
    SprintID      string    // 所属 Sprint
    MilestoneID   string    // 所属 Milestone
    ParentID      string    // Epic → Story 父子关系
    Labels        string    // JSON: ["backend", "api"]
    LinkedPR      int       // 关联的 Nydus PR number
    LinkedBranch  string    // 关联的 git branch
    DependsOn     string    // JSON: ["issue-id-1"]
    Estimate      int       // 预估工时 (小时)
    CreatedBy     string    // 创建者节点
    CreatedAt     time.Time
    UpdatedAt     time.Time
    ClosedAt      *time.Time
}

// ForgeIssueComment — 工单评论
type ForgeIssueComment struct {
    ID         string
    IssueID    string
    AuthorNode string
    AuthorRole string
    Body       string
    CreatedAt  time.Time
}

// ForgeSprint — 迭代 (演进自现有 Sprint, 关联 Mission)
// 复用现有 model.Sprint, 新增字段:
//   ProjectID  string  // 所属项目
//   MissionID  string  // 触发的 Mission (已有)

// ForgeMilestone — 里程碑
type ForgeMilestone struct {
    ID          string
    ProjectID   string
    Title       string
    Description string
    DueDate     *time.Time
    Status      string    // open / closed
    CreatedAt   time.Time
}

// ForgeBoardConfig — 看板配置
type ForgeBoardConfig struct {
    ID        string
    ProjectID string
    Columns   string    // JSON: [{"name":"Backlog","status":"backlog"},...]
    Swimlanes string    // JSON: "none" / "assignee" / "type"
}
```

### 5.3 Overlord 侧 (扩展已有模型)

```go
// TeamAgentInstance 扩展字段:
//   ForgeProjectIDs  string  // JSON: 该团队管理的 Forge 项目 ID 列表
//   NydusRepoNames   string  // JSON: 该团队拥有的 Nydus 仓库列表

// 新增: 跨团队协作记录
type TeamCollaboration struct {
    ID           string
    ProjectID    string    // Forge Project ID
    NydusRepo    string    // Nydus Repo name
    OwnerTeamID  string    // 项目所有方团队
    GuestTeamID  string    // 参与协作的团队
    Permission   string    // read / write
    Status       string    // invited / active / ended
    CreatedAt    time.Time
}
```

## 六、API 设计

### 6.1 Nydus 新增 API

```
── 节点身份 ──────────────────────────────────────
POST   /api/nodes/register            Claw 节点注册 (Ed25519 公钥)
GET    /api/nodes                     已注册节点列表
GET    /api/nodes/:id                 节点详情
PUT    /api/nodes/:id/ssh-keys        更新 SSH 公钥

── 仓库增强 ──────────────────────────────────────
POST   /api/repos                     创建 Repo (需节点身份)
POST   /api/repos/:name/fork          Fork 仓库
PUT    /api/repos/:name/settings      设置 (分支保护等)
GET    /api/repos/:name/access        协作者/团队列表
POST   /api/repos/:name/access        添加权限
DELETE /api/repos/:name/access/:id    移除权限

── Pull Request ──────────────────────────────────
POST   /api/repos/:name/pulls         创建 PR
GET    /api/repos/:name/pulls         PR 列表 (?status=open)
GET    /api/repos/:name/pulls/:num    PR 详情
GET    /api/repos/:name/pulls/:num/diff   文件 Diff
POST   /api/repos/:name/pulls/:num/reviews  提交 Review
POST   /api/repos/:name/pulls/:num/merge    合并
PATCH  /api/repos/:name/pulls/:num    更新 PR (标题/描述)
POST   /api/repos/:name/pulls/:num/comments 评论 (支持行内)

── Diff / Compare ────────────────────────────────
GET    /api/repos/:name/compare/:base...:head  比较两个 ref
GET    /api/repos/:name/commits/:sha           commit 详情 + diff
GET    /api/repos/:name/blob/:ref/*path        文件内容 (带语法高亮)

── Webhook ───────────────────────────────────────
POST   /api/repos/:name/webhooks      创建 Webhook
GET    /api/repos/:name/webhooks      列表
DELETE /api/repos/:name/webhooks/:id  删除

── 认证方式 ──────────────────────────────────────
方式1: X-Claw-ID + X-Claw-Timestamp + X-Claw-Signature (Ed25519)
方式2: Authorization: Bearer {node-token} (简化版)
方式3: X-Nydus-Secret (向后兼容, 仅管理操作)
```

### 6.2 Claw Forge 新增 API

```
── 项目 ──────────────────────────────────────────
POST   /v1/forge/projects             创建项目 (自动创建 Nydus Repo)
GET    /v1/forge/projects             项目列表
GET    /v1/forge/projects/:id         项目详情
PUT    /v1/forge/projects/:id         更新
DELETE /v1/forge/projects/:id         归档

── 工单 ──────────────────────────────────────────
POST   /v1/forge/projects/:id/issues  创建 Issue
GET    /v1/forge/projects/:id/issues  Issue 列表 (?status=&assignee=&sprint=)
GET    /v1/forge/issues/:id           Issue 详情
PUT    /v1/forge/issues/:id           更新 (状态/分配/优先级)
POST   /v1/forge/issues/:id/comments  添加评论
POST   /v1/forge/issues/:id/assign    分配给角色/节点
POST   /v1/forge/issues/:id/transition 状态流转

── 看板 ──────────────────────────────────────────
GET    /v1/forge/projects/:id/board   看板视图 (按列分组的 Issues)
PUT    /v1/forge/projects/:id/board   更新看板配置
GET    /v1/forge/projects/:id/stats   项目统计 (Issue/PR/Sprint 数据)

── Sprint ────────────────────────────────────────
POST   /v1/forge/projects/:id/sprints 创建 Sprint
GET    /v1/forge/projects/:id/sprints Sprint 列表
PUT    /v1/forge/sprints/:id          更新 Sprint
GET    /v1/forge/sprints/:id/burndown 燃尽图数据

── AI 自动化 (内部调用) ──────────────────────────
POST   /v1/forge/projects/:id/decompose   AI 分解需求为 Issues
POST   /v1/forge/issues/:id/auto-assign   AI 自动分配
POST   /v1/forge/projects/:id/auto-triage AI 自动分类新 Issue
```

### 6.3 Overlord 新增 API

```
── 项目总览 ──────────────────────────────────────
GET    /brood/forge/projects          所有团队的项目列表 (聚合各 Claw 节点)
GET    /brood/forge/projects/:id      项目详情 (proxy 到 Claw Forge)
GET    /brood/forge/dashboard         全局大盘 (Issue/PR/Sprint 统计)

── 代码审计 ──────────────────────────────────────
GET    /brood/nydus/repos             所有 Nydus Repo (proxy 到 Nydus)
GET    /brood/nydus/pulls             所有 Open PR (聚合)
GET    /brood/nydus/activity          提交活跃度 (按团队/按节点)

── 团队协作 ──────────────────────────────────────
POST   /brood/collaborations          创建跨团队协作
GET    /brood/collaborations          协作列表
DELETE /brood/collaborations/:id      结束协作

── 组织策略 ──────────────────────────────────────
GET    /brood/policies                当前策略
PUT    /brood/policies                更新策略 (下发到 Nydus)
```

## 七、关键集成点

### 7.1 Mission → Forge Project + Sprint

```
Overlord 创建 Mission
  → Claw 接收 Mission
  → 检查: 该团队是否已有对应 Project?
    → 有: 在现有 Project 下创建新 Sprint
    → 没有: 创建新 Project → Nydus CreateRepo → 创建 Sprint
  → 设计虫分解目标 → 创建 Issues (挂在 Sprint 下)
```

### 7.2 Issue ↔ Nydus Branch + PR

```
Agent 认领 Issue:
  → 自动创建 branch: feature/issue-{number}-{slug}
  → Agent 在此 branch 开发
  → 完成后自动创建 PR (linked_issues: [issue_id])

PR Merged (Nydus Webhook):
  → Forge 收到通知 → 关联 Issue 状态 → done
  → Board 自动更新
  → 检查依赖: 被阻塞的 Issue 是否可以开始

PR Changes Requested:
  → Issue 状态 → in_progress (退回)
  → Agent 收到反馈 → 修改代码 → 重新 push
  → 最多 3 轮 (复用 maxReviewRetries)
```

### 7.3 Nydus Webhook → 全链路通知

```
Nydus 事件:
  push         → Forge (commit 记录) + Overlord (活跃度)
  pr.opened    → Forge (Issue 关联) + Overlord (PR 大盘)
  pr.reviewed  → Forge (Review 状态) + 提交者 Agent (反馈)
  pr.merged    → Forge (Issue 关闭) + CI/CD (部署)
  pr.closed    → Forge (Issue 退回)
  release      → Overlord (版本发布通知)
```

### 7.4 跨团队协作流程

```
Team A (GameClaw, 项目发起方):
  → Forge: 创建 Project "space-game"
  → Nydus: 创建 Repo, Team A = admin

Team A 邀请 Team B (DevClaw):
  → Overlord: POST /brood/collaborations
    {project: "space-game", guest_team: "devclaw-inst-002", permission: "write"}
  → Nydus: POST /api/repos/space-game/access
    {team_id: "devclaw-inst-002", permission: "write"}
  → Team B Captain 收到通知

Team B 加入协作:
  → Forge: 同步 Project 元数据 (Issues 可见)
  → Team B 的 Agent 可以:
    - 创建 Issues (标记为 Team B 创建)
    - Clone repo / 创建 branch / Push / 提 PR
    - Review Team A 的 PR
  → 不能:
    - 修改 Repo settings (只有 Team A admin)
    - 删除其他团队的 Issues

看板视图 (两种):
  Team A 看板: 只显示 Team A 的 Issues + 依赖的 Team B Issues
  全局看板: 显示所有 Issues (Overlord Console 可见)
```

## 八、实施分期

### Phase 1: Nydus 基础 (N0-N2) — 3天

| Task | 内容 | 文件 |
|------|------|------|
| N0 | NydusNode 模型 + Ed25519 认证中间件 + 节点注册 API | `handler/node.go`, `middleware/claw_auth.go`, `model/` |
| N1 | NydusRepo 动态持久化 (SQLite) + RepoAccess 权限 | `handler/repo.go` 扩展, `model/repo.go` |
| N2 | Fork 机制 + 团队级权限 | `handler/repo.go` 扩展 |

### Phase 2: Nydus PR 系统 (N3-N5) — 4天

| Task | 内容 | 文件 |
|------|------|------|
| N3 | PullRequest 模型 + CRUD + Diff API (git diff) | `handler/pull.go`, `model/pull.go` |
| N4 | PR Review + Merge + Branch Protection | `handler/pull.go` 扩展 |
| N5 | Webhook 系统 (事件定义 + HTTP 分发 + HMAC 签名) | `handler/webhook.go`, `model/webhook.go` |

### Phase 3: Claw Forge (F1-F4) — 6天

| Task | 内容 | 文件 |
|------|------|------|
| F1 | ForgeProject + ForgeIssue + ForgeMilestone 模型 | `model/forge.go` |
| F2 | Issue CRUD + 角色分配 + 状态流转 + Comment | `api/v1/forge_issue.go` |
| F3 | Project CRUD (自动创建 Nydus Repo) + Board API | `api/v1/forge_project.go`, `api/v1/forge_board.go` |
| F4 | ForgeEngine — AI 分解需求/自动认领/自动流转 | `squad/forge_engine.go` |

### Phase 4: 集成 (T1-T3) — 4天

| Task | 内容 | 文件 |
|------|------|------|
| T1 | Squad Engine 改造: Mission → 自动创建 Project/Sprint + Nydus Repo | `squad/engine.go` 改造 |
| T2 | Step 执行改造: Clone from Nydus → 提 PR → Webhook 驱动流转 | `squad/engine.go` + `sprint_lifecycle.go` |
| T3 | Overlord Console: 项目总览 + 代码审计 + 协作管理页面 | `console/src/pages/` |

### Phase 5: 前端 (W1-W2) — 3天

| Task | 内容 | 文件 |
|------|------|------|
| W1 | Claw Web: ForgePage (项目列表) + ProjectPage (Board + Issues + PRs) | `web/src/pages/` |
| W2 | Claw Web: CodeBrowser (文件树 + Diff + PR 详情) — proxy 到 Nydus | `web/src/pages/` |

**总计: ~20 天**

## 九、与现有系统的兼容

### 不破坏现有功能

| 现有功能 | 处理方式 |
|---------|---------|
| Squad CRUD API | 保留, 内部关联 Team Agent Instance |
| Mission/Sprint/MissionStep | 保留, Step 可选升级为 Issue |
| GitManager (本地 repo) | 保留, 单节点项目仍用本地 repo; 多节点走 Nydus |
| Git Smart HTTP | 保留, 跨节点传输仍可走 Claw HTTP |
| Sprint Lifecycle (review/CI/retro) | 保留, Review 增加 Nydus PR 路径 |
| HiveBroadcaster | 保留, 增广播 Issue 认领状态 |
| Nydus 现有 API | 完全保留, 新增 API 不影响旧端点 |
| Nydus YAML config | 保留, SQLite 做补充; config 定义的 repo 优先 |

### 渐进式迁移

```
阶段 1: 新项目走 Forge + Nydus PR 流程
阶段 2: 旧 Mission 可选绑定到 Forge Project
阶段 3: 前端默认展示 Forge 视图, 旧 Squad 页面保留
```
