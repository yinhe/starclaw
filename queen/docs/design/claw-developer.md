# DevClaw — 虫族团队智能体

> **团队智能体 (Team Agent)** — StarClaw 多 Agent 协作的第一个代表作品。
> 不是一个 AI 在干活，而是一支 AI 团队在协作。
> 两大使命：**帮用户造产品，帮虫后养生态。**

---

## 一、团队智能体 (Team Agent)

### 1.1 从单体到团队

```
单体智能体 (Single Agent):
  用户 → [一个 Agent] → 结果
  局限: 上下文有限、角色单一、无质量把关

团队智能体 (Team Agent):
  用户 → [一支 Agent 团队] → 结果
  优势: 角色分工、并行执行、审查循环、集体智慧
```

| 维度 | 单体 Agent | 团队智能体 |
|------|-----------|-----------|
| **角色** | 1 个全能 Agent | N 个专精 Agent |
| **质量** | 自己写自己看 | 写的人和审的人分离 |
| **并行** | 串行处理 | 多个 Agent 同时工作 |
| **容错** | 一个 Agent 卡住就全卡 | 可替换/重试单个角色 |
| **知识** | 一份上下文 | 共享知识库 + 各自专业知识 |
| **复杂度** | 适合简单任务 | 适合工程级任务 |

### 1.2 团队智能体 = Squad + 角色图 + 协作拓扑

```
┌─ Team Agent（团队智能体）─────────────────────────────────────────────┐
│                                                                        │
│  ┌─ Role Graph（角色图）────────────────────────────────────────────┐  │
│  │  定义: 有哪些角色、各自的 System Prompt + 技能 + 知识库          │  │
│  │  示例: Architect / Drone / Tester / Reviewer / DocBot            │  │
│  └──────────────────────────────────────────────────────────────────┘  │
│                                                                        │
│  ┌─ Collaboration Topology（协作拓扑）─────────────────────────────┐  │
│  │  定义: 角色之间的工作流 — 谁先谁后、并行/串行、审查循环          │  │
│  │  模式: Pipeline / Fan-out / Review Loop / Debate / Hierarchy     │  │
│  └──────────────────────────────────────────────────────────────────┘  │
│                                                                        │
│  ┌─ Shared Workspace（共享工作区）─────────────────────────────────┐  │
│  │  Git 仓库 / 任务看板 / 共享记忆 / RAG 知识库                    │  │
│  └──────────────────────────────────────────────────────────────────┘  │
│                                                                        │
│  ┌─ Runtime = Squad Engine ───────────────────────────────────────┐   │
│  │  Mission 调度 / Sprint 管理 / 跨节点执行 / 回调收集              │  │
│  └──────────────────────────────────────────────────────────────────┘  │
│                                                                        │
└────────────────────────────────────────────────────────────────────────┘
```

**核心洞察：** Squad Engine 是运行时基础设施，Team Agent 是产品抽象。Squad 提供调度+执行，Team Agent 定义角色+拓扑+质量标准。

### 1.3 五种协作拓扑

| 拓扑 | 模式 | 适用场景 | DevClaw 中的应用 |
|------|------|---------|-----------------|
| **Pipeline** | A → B → C | 严格顺序依赖 | 设计 → 编码 → 测试 |
| **Fan-out** | A → [B, C, D] | 可并行的子任务 | 1 个 Architect → 3 个 Drone 同时写不同模块 |
| **Review Loop** | A → B → (pass/retry A) | 需要质量把关 | Drone → Reviewer → 评分<7 退回重写 |
| **Debate** | [A, B] → Arbiter | 需要多视角决策 | 2 个 Architect 提方案 → Queen 选最优 |
| **Hierarchy** | Captain → [Workers] | 复杂分层任务 | Squad Captain 协调所有角色 |

### 1.4 DevClaw 的协作拓扑

DevClaw 组合使用 Pipeline + Fan-out + Review Loop：

```
                         ┌──────────┐
                         │ Architect │  ← Pipeline 起点: 设计方案
                         └────┬─────┘
                              │ 方案确认
                    ┌─────────┼─────────┐
                    ▼         ▼         ▼
               ┌────────┐┌────────┐┌────────┐
               │Drone-A ││Drone-B ││Drone-C │  ← Fan-out: 并行编码
               │ 后端API ││ 前端页面││ 工具集成│
               └───┬────┘└───┬────┘└───┬────┘
                   └─────────┼─────────┘
                             ▼
                        ┌─────────┐
                        │ Tester  │  ← 测试所有模块
                        └────┬────┘
                             ▼
                        ┌──────────┐
                 ┌──────│ Reviewer │  ← Review Loop
                 │      └────┬─────┘
                 │           │
            score < 7    score ≥ 7
                 │           │
                 ▼           ▼
            退回 Drone   ┌────────┐
            重写(≤3次)   │ DocBot │  ← 更新文档
                         └────┬───┘
                              ▼
                          ✅ 完成
```

### 1.5 Team Agent 数据模型

```
TeamAgentTemplate {
  id:           uuid
  name:         "DevClaw"              // 团队智能体名称
  description:  "全栈 AI 开发团队"
  category:     "development"          // development | marketing | research | support | custom

  // 角色定义
  roles: [
    {
      code:          "architect",
      name:          "设计虫",
      system_prompt: "你是资深软件架构师...",
      model:         "star-ai/pro",
      tools:         ["code", "web_search", "document"],
      knowledge_base: "codebase-rag",   // 可选
      max_instances:  1                  // 该角色最多几个 Agent 同时工作
    },
    {
      code:          "drone",
      name:          "编码虫",
      system_prompt: "你是全栈开发工程师...",
      model:         "star-ai/pro",
      tools:         ["code", "git", "web_search", "http_request"],
      knowledge_base: "codebase-rag",
      max_instances:  5                  // 可以有多个 Drone 并行
    },
    // ... tester, reviewer, docbot, medic, analyst
  ],

  // 协作拓扑
  topology: {
    type: "pipeline_fan_review",         // 预定义拓扑 or 自定义 DAG
    flow: [
      { from: "start",      to: "architect",  type: "pipeline" },
      { from: "architect",   to: "drone",      type: "fan_out"  },
      { from: "drone",       to: "tester",     type: "fan_in"   },
      { from: "tester",      to: "reviewer",   type: "pipeline" },
      { from: "reviewer",    to: "drone",      type: "review_loop", condition: "score < 7", max_retries: 3 },
      { from: "reviewer",    to: "docbot",     type: "pipeline", condition: "score >= 7" },
      { from: "docbot",      to: "end",        type: "pipeline" }
    ]
  },

  // 质量标准
  quality_gate: {
    review_threshold:  7,               // Reviewer 评分阈值
    test_required:     true,            // 必须通过测试
    build_required:    true,            // 必须通过编译
    max_retries:       3                // 最多重试次数
  },

  // 升级策略
  escalation: {
    on_max_retries:    "bounty",        // 超过重试 → 赏金任务
    on_budget_exceed:  "pause_notify",  // 超预算 → 暂停通知
    on_critical_path:  "human_review"   // 关键路径 → 人类审批
  }
}
```

### 1.6 DevClaw 是第一个，但不是唯一一个

团队智能体是通用框架，用户可以创建自己的 Team Agent：

| Team Agent | 角色 | 典型拓扑 | 场景 |
|-----------|------|---------|------|
| **DevClaw** (开发团队) | Architect + Drone + Tester + Reviewer + DocBot | Pipeline + Fan-out + Review | 软件开发 |
| **MarketClaw** (营销团队) | Strategist + Copywriter + Designer + Reviewer | Pipeline + Review | 营销活动策划 |
| **ResearchClaw** (研究团队) | Researcher + Analyst + Writer + Editor | Fan-out + Pipeline | 研究报告 |
| **SupportClaw** (客服团队) | Triager + Specialist + QA | Hierarchy + Review | 客户支持 |
| **ContentClaw** (内容团队) | Planner + Writer + Editor + Publisher | Pipeline + Review | 内容创作 |

```
Team Agent 市场（未来）:
  用户可以在市场上:
  ├── 使用官方 Team Agent（DevClaw 等）
  ├── 购买社区创建的 Team Agent
  └── 自己组装 Team Agent（选角色 + 定拓扑 + 设质量标准）
```

---

## 二、双场景总览

DevClaw（第一个团队智能体）服务两个截然不同的主人，共享同一套基础设施。

| 维度 | 场景 A: 用户产品开发 | 场景 B: Queen 生态维护 |
|------|---------------------|----------------------|
| **主人** | Claw 用户（付费客户） | Queen（平台自身） |
| **目标** | 造用户的产品 | 维护/进化 StarClaw 平台 |
| **代码仓库** | 用户 Project Repo | StarClaw Monorepo |
| **权限** | 用户代码无限制 | 白名单目录（安全边界） |
| **审批者** | 用户本人 | AI Quality Gate + 可选人类 |
| **部署** | Vercel / 自有服务器 / StarClaw Hosting | Nydus 自动部署到生产 |
| **成本** | 用户星能⚡ | 平台运营预算（月收入×5%） |
| **收益** | 用户赚钱 | 平台竞争力提升 |

### Team Agent → Squad Engine 映射

团队智能体不是新建一套系统，而是复用已有 Squad 基础设施，加上角色图层和拓扑层。

```
Team Agent 概念          Squad Engine 实现                  已有代码
─────────────────────────────────────────────────────────────────────
TeamAgentTemplate     →  Squad + AgentTemplate[]           model/squad.go
Role                  →  AgentTemplate (专用 SystemPrompt)  model/agent.go
Topology.flow         →  MissionStep.Dependencies          model/squad.go
Fan-out               →  多个 MissionStep 无依赖(并行)     engine.go
Review Loop           →  ReviewMatrix + retry 逻辑          sprint_lifecycle.go
Quality Gate          →  CI Gate (merge + LLM quality)      sprint_lifecycle.go
Shared Workspace      →  GitManager (repo per mission)      git.go
Escalation            →  BountyTool (auto post)             bounty_tool.go
Debate                →  多个 Step 产出 → LLM 仲裁 Step     (新增)
```

关键实现点：

```
1. TeamAgentTemplate → 自动生成 Squad + Mission
   用户选择 DevClaw 模板 + 输入需求描述
   → Queen/Claw 根据模板 roles[] 创建 Squad
   → 根据模板 topology.flow[] 生成 MissionStep DAG
   → Squad Engine 按依赖图调度执行（已有）

2. Role → MissionStep.TargetNode 自动分配
   已有: Hivemind.FindBestNodeForSpecialty(specialty)
   新增: specialty = role.code (e.g. "architect", "drone")
   → Hivemind 根据节点上的 AgentCapability 自动选最佳节点

3. Fan-out → 多个平行 MissionStep
   已有: 无依赖的 Step 自动并行 dispatch
   → Architect 输出方案后，生成 N 个 Drone Step（无互相依赖）

4. Review Loop → Sprint Lifecycle 的 ReviewMatrix
   已有: triggerAutoReview → 6 角色评分 → 均分 < 阈值 → retry
   → 完美匹配 Reviewer 角色，max_retries 已实现

5. Escalation → BountyTool
   已有: BountyTool.Execute(post_bounty)
   → DevClaw 连续失败 → 自动调用 bounty 技能发布赏金任务
```

### 与其他系统的关系

| 现有系统 | Team Agent 如何复用 |
|---------|-------------------|
| **Squad Engine** | 运行时：Mission 调度 + Sprint 管理 + 跨节点执行 |
| **Sprint Lifecycle** | 质量引擎：Review + CI Gate + Retrospective |
| **Hivemind** | 节点发现：AgentCapability 匹配最佳节点 |
| **Git Manager** | 共享工作区：每个 Mission 一个 Git 分支 |
| **Instinct Engine** | 触发器：定时扫描 → 生成 Team Agent 任务 |
| **Bounty System** | 升级策略：AI 做不了 → 人类赏金 |
| **Tool Registry** | 技能注册：新开发的 Tool 直接注册 |
| **Memory System** | 知识积累：跨任务经验传承 |
| **CreatorRevenue** | 分成结算：用户产品上架 80/15/5 |

---

## 三、场景 A — 用户产品开发

用户的 Claw 变成一个**全栈开发团队**，帮用户从零造产品。

### 3.1 用户视角

```
用户: "帮我做一个宠物用品电商小程序，带 AI 客服和推荐"

  用户的 Claw 节点
  │
  ├─ Mission: "宠物电商小程序"
  │   ├── Sprint 0: 技术选型 + 脚手架
  │   │   ├─ Architect: 选 Next.js + Tailwind + Supabase
  │   │   └─ Drone: 创建项目 + 基础路由
  │   │
  │   ├── Sprint 1: 商品管理 + 展示
  │   │   ├─ Drone-A: 商品 CRUD API
  │   │   ├─ Drone-B: 商品列表/详情页面
  │   │   └─ Tester: API 测试
  │   │
  │   ├── Sprint 2: AI 客服 + 推荐
  │   │   ├─ Drone: 集成 StarClaw Agent API
  │   │   └─ Drone: 推荐算法
  │   │
  │   └── Sprint 3: 支付 + 部署
  │       ├─ Drone: 支付接入
  │       └─ Drone: 部署上线
  │
  │   每个 Sprint → 预览 URL → 用户反馈 → 下一轮
  └───────────────────────────────────────────
```

### 3.2 三种产品开发模式

| 模式 | 说明 | 典型用户 | 交付物 |
|------|------|---------|--------|
| **自用开发** | 描述需求，DevClaw 全栈开发 | 创业者、中小企业 | Web/移动应用、SaaS、API |
| **Agent 产品** | 设计 Agent + 技能 + 工作流 → 打包发布 | AI 创作者 | 市场上架的 Agent/工作流/插件 |
| **定制外包** | 用户接单，DevClaw 协助交付 | 自由开发者 | 客户的定制项目 |

### 3.3 技术栈自动选择

| 项目类型 | 推荐技术栈 | 部署方式 |
|---------|-----------|---------|
| Web SaaS | Next.js + Tailwind + Supabase | Vercel / StarClaw Hosting |
| 小程序 | UniApp / Taro | 微信/支付宝发布 |
| API 服务 | Go Gin / Node Express | Docker + StarClaw Hosting |
| 数据分析 | Python + Streamlit | StarClaw Hosting |
| 移动 App | Flutter / React Native | App Store / 自建分发 |
| AI Agent 产品 | StarClaw Agent + 自定义技能 | StarClaw 市场 |

### 3.4 用户项目 API

```
POST /v1/projects                    — 创建开发项目
{
  "name": "宠物电商",
  "description": "带 AI 客服的宠物用品电商小程序",
  "type": "web_saas",               // web_saas | miniapp | api | data | mobile | agent
  "tech_stack": "auto",              // auto = DevClaw 自动选, 或指定
  "budget_stars": 5000,              // 星能预算上限
  "deploy_target": "starclaw_host"   // vercel | starclaw_host | self_hosted | marketplace
}

GET  /v1/projects                    — 我的项目列表
GET  /v1/projects/:id                — 项目详情 + Sprint 进度
POST /v1/projects/:id/feedback       — 用户反馈（触发新 Sprint）
GET  /v1/projects/:id/preview        — 最新预览 URL
POST /v1/projects/:id/deploy         — 正式部署
POST /v1/projects/:id/publish        — 发布到市场（Agent 产品）
GET  /v1/projects/:id/cost           — 查看已消耗星能
```

### 3.5 StarClaw Hosting（虫巢托管）

用户产品一键部署到 StarClaw 基础设施：

```
StarClaw Hosting
├── 静态站点 → CDN + 自定义域名（免费额度 + 超出按量）
├── API 服务 → Docker 容器 + 自动扩缩（按请求计费）
├── 数据库  → 共享 MySQL / SQLite（按存储计费）
└── AI 能力 → 直接调用 Claw Agent API（按 token 计费）

全部用星能⚡结算
```

### 3.6 用户产品维护

产品上线不是终点，DevClaw 持续维护：

```
用户产品维护 DevClaw（可选订阅）:

  Medic   — 7×24 监控产品健康，自动修复宕机
  Analyst — 分析用户行为数据，生成运营报告
  Drone   — 用户提新需求 → 自动迭代开发

定价: 按月订阅星能包
  基础版: 500⚡/月  (监控 + 自动重启)
  标准版: 2000⚡/月 (监控 + bug 修复 + 小需求)
  高级版: 5000⚡/月 (全包: 监控 + 开发 + 分析)
```

---

## 四、场景 B — Queen 生态维护

Queen 把 DevClaw 当成自己的**内部工程团队**，7×24 维护 StarClaw 平台。

### 4.1 Queen 的日常

```
06:00  Evolution Engine 日扫描
       ├── 分析昨日 10 万条对话 → "生成PPT" 被提及 230 次
       │   → P1 Feature: "开发 ppt_generation 技能"
       ├── web_search 失败率 5%→18%
       │   → P1 Bugfix: "修复 web_search 超时"
       └── video_tool.go:87 nil pointer × 50/h
           → P0 Incident: "紧急修复 video_tool nil crash"

06:05  Queen 自动组建 Dev Squad
       P0 → Medic 诊断 → Drone 修复 → Tester 验证 → 30 分钟完成
       P1 → Architect 设计 → Drone 编码 → Reviewer 审查 → 4 小时完成

18:00  日报推送管理员: 修复 2 bug, 上线 1 新技能, 消耗 3200⚡
```

### 4.2 四大引擎

| 引擎 | 职责 | 触发方式 |
|------|------|---------|
| **Evolution** | 分析对话/统计/日志/竞品 → 生成 DevTask | Instinct 定时(每日) |
| **Dispatch** | 评估复杂度 → 组建 Dev Squad → 分配 Sprint | DevTask 入队时 |
| **Quality** | 编译 → 测试 → Reviewer 评分 → 合并/拒绝 | Sprint CI Gate |
| **Healing** | 7×24 监控 → 异常诊断 → 修复 → 验证 | 实时事件 |

### 4.3 Evolution Engine 四维分析

```
需求挖掘         质量优化         故障检测         生态扫描
┌──────────┐   ┌──────────┐   ┌──────────┐   ┌──────────┐
│ 用户对话  │   │ 技能统计  │   │ 错误日志  │   │ 竞品动态  │
│ "想做PPT" │   │ 失败率↑   │   │ 500/panic│   │ 新功能   │
│ ×230次/周 │   │ 延迟↑     │   │ OOM      │   │ 社区热点  │
└────┬─────┘   └────┬─────┘   └────┬─────┘   └────┬─────┘
     │ P1 Feature   │ P1 Bugfix    │ P0 Incident   │ P3 Feature
     └──────────────┴──────────────┴───────────────┘
                         ▼
                  DevTask 队列
```

### 4.4 自愈闭环

```
Monitor (Analyst, 每60秒)
  → 5xx 错误率>1% / P99>5s / OOM / 磁盘>90%
  ↓
Diagnose (Medic)
  → 收集日志+最近变更+代码 → LLM 分析根因
  ↓
  ├── 可修复 → P0 Incident → Drone 修复 → Tester 验证
  ├── 需回滚 → 自动回滚 + 通知人类
  └── 需人类 → Bounty 赏金任务 + 告警
  ↓
Verify (Tester)
  → 回归测试 + 30 分钟无复发 → 关闭
```

### 4.5 安全边界（白名单）

| 目录 | 权限 | 说明 |
|------|------|------|
| `claw/api/internal/tool/` | 读写 | 技能开发/修复 |
| `claw/api/plugins/` | 读写 | JSON 插件 |
| `claw/docs/` | 读写 | 技术文档 |
| `claw/web/src/pages/` | 读写 | 前端页面 |
| `claw/api/internal/router/router.go` | **仅追加** | 注册新 Tool 行 |
| `claw/api/internal/middleware/` | **禁止** | 安全关键 |
| `claw/api/internal/security/` | **禁止** | 安全关键 |
| `claw/api/internal/billing/` | **禁止** | 财务关键 |
| `queen/` | **禁止** | 防止自我修改 |
| `synapse/` | **禁止** | 计算层核心 |

### 4.6 Queen DevTask 数据模型

```
QueenDevTask {
  id:             uuid
  type:           feature | bugfix | maintenance | evolution | incident
  title:          "开发 PPT 生成技能"
  description:    "用户高频需求，对话提及 230 次/周"
  priority:       P0 | P1 | P2 | P3
  source:         evolution_demand | evolution_quality | evolution_incident | evolution_scan | human_admin
  status:         queued → designing → coding → testing → reviewing → merging → deployed → verified | failed

  target_paths:   ["claw/api/internal/tool/", "claw/docs/"]
  acceptance:     "go build + 测试覆盖>60% + 评分>7"
  energy_budget:  2000⚡
  max_retries:    3

  squad_id:       → 自动组建的 Dev Squad
  mission_id:     → Squad Mission
  quality_score:  8.5
  energy_consumed: 1250⚡
  duration_min:   240

  created_by:     evolution_engine | human_admin
  approved_by:    auto (score>8) | human:<user_id>
}
```

### 4.7 Queen API

```
POST /internal/devtask/create         — 创建 DevTask
GET  /internal/devtask/queue          — 待处理队列
POST /internal/devtask/:id/complete   — 标记完成
POST /internal/devtask/:id/fail       — 标记失败
GET  /internal/devtask/stats          — 统计报表

POST /internal/evolution/scan         — 手动触发进化扫描
GET  /internal/evolution/suggestions  — 查看进化建议
POST /internal/evolution/approve/:id  — 确认建议 → 创建 DevTask
```

---

## 五、DevClaw 角色（双场景共用）

同一套角色，根据场景加载不同知识库。

### 5.1 角色定义

| 角色 | 代号 | 技能 | 场景 A（用户产品） | 场景 B（Queen 生态） |
|------|------|------|-------------------|---------------------|
| **设计虫** | Architect | `web_search` `code` `document` | 产品架构 + 技术选型 | Tool 接口设计 |
| **编码虫** | Drone | `code` `git` `web_search` `http_request` | 用户产品全栈代码 | Go Tool / React 页面 |
| **测试虫** | Tester | `code` `git` | 产品功能测试 | go test + 回归测试 |
| **审查虫** | Reviewer | `code` `git` `web_search` | 代码质量把关 | 安全/性能/规范审查 |
| **文档虫** | DocBot | `code` `git` `document` | 产品文档 + README | SKILL_DEVELOPMENT.md 等 |
| **医疗虫** | Medic | `code` `git` `http_request` `web_search` | 用户产品运维 | 平台 7×24 自愈 |
| **分析虫** | Analyst | `code` `http_request` | 产品用户行为分析 | 技能统计 + 错误检测 |

### 5.2 知识库差异

```
场景 A — 用户产品 DevClaw:
├── Level 1: LLM 预训练（全栈技术）
├── Level 2: 用户项目 RAG（用户的代码 + 需求文档）
└── Level 3: 项目记忆（Sprint 历史 + 用户偏好）

场景 B — Queen 生态 DevClaw:
├── Level 1: LLM 预训练（全栈技术）
├── Level 2: StarClaw 代码库 RAG（源码 + 架构文档 + 编码规范）
└── Level 3: 开发记忆（DevTask 历史 + 踩坑记录）
```

### 5.3 代码库 RAG 自动索引（场景 B）

```
git push 触发 Nydus post-receive hook
  → 扫描变更文件
  → 提取函数签名 / 结构体 / 接口
  → 更新 RAG 向量库
  → DevClaw 编码时查询: "Tool 接口怎么定义的？" → 按规范编写
```

---

## 六、收益模型与分成

### 6.1 全景图

```
┌────────────────────────────────────────────────────────────────────────┐
│                      StarClaw DevClaw 收益全景                          │
│                                                                        │
│  ┌─ 场景 A: 用户产品 ────────────────────────────────────────────────┐ │
│  │                                                                    │ │
│  │  ① 开发费: 用户→平台                                              │ │
│  │     用户消耗星能⚡(LLM token + 工具调用) → 平台收入 100%           │ │
│  │                                                                    │ │
│  │  ② 产品变现: 三种路径                                              │ │
│  │                                                                    │ │
│  │     A. 上架市场（Agent/工作流/插件）:                               │ │
│  │        买家付费 → 创作者 80% / 平台 15% / 推荐人 5%                │ │
│  │        (已有: CreatorRevenue)                                      │ │
│  │                                                                    │ │
│  │     B. 托管运营（SaaS 产品通过 StarClaw Hosting）:                 │ │
│  │        终端用户付费 → 产品主 90% / 平台 10%                        │ │
│  │        (新增: HostingRevenue)                                      │ │
│  │                                                                    │ │
│  │     C. 自有部署:                                                   │ │
│  │        用户自行运营，平台不抽成                                     │ │
│  │        若调用 StarClaw Agent API → 按量计费（已有 Billing）         │ │
│  │                                                                    │ │
│  │  ③ 维护费: 用户→平台                                              │ │
│  │     月度订阅: 500⚡(基础) / 2000⚡(标准) / 5000⚡(高级)           │ │
│  │                                                                    │ │
│  └────────────────────────────────────────────────────────────────────┘ │
│                                                                        │
│  ┌─ 场景 B: Queen 生态 ──────────────────────────────────────────────┐ │
│  │                                                                    │ │
│  │  投入: 月度 DevClaw 预算 = 平台月收入 × 5%                        │ │
│  │                                                                    │ │
│  │  产出（间接收益）:                                                  │ │
│  │  ├── 新技能上线 → 用户满意度↑ → 留存↑ → 收入↑                     │ │
│  │  ├── Bug 自愈  → 稳定性↑ → 口碑↑ → 新用户↑                       │ │
│  │  └── 文档更新  → 开发者体验↑ → 生态贡献者↑                        │ │
│  │                                                                    │ │
│  │  自动调节:                                                         │ │
│  │  ├── ROI > 3x → 预算增到 8%                                      │ │
│  │  └── ROI < 1x → 预算降到 3%                                      │ │
│  │                                                                    │ │
│  └────────────────────────────────────────────────────────────────────┘ │
└────────────────────────────────────────────────────────────────────────┘
```

### 6.2 分成总览

| 场景 | 环节 | 谁付钱 | 分成 | 实现 |
|------|------|--------|------|------|
| A | 开发费 | 用户⚡ | 平台 100% | Billing Gateway |
| A | 市场售卖 | 买家⚡ | 创作者 80% / 平台 15% / 推荐 5% | CreatorRevenue |
| A | 托管运营 | 终端用户 | 产品主 90% / 平台 10% | **新增** HostingRevenue |
| A | API 调用 | 用户产品的用户 | 平台 100%（按量） | Billing Gateway |
| A | 维护订阅 | 用户⚡ | 平台 100% | Subscription |
| B | 生态投入 | 平台预算 | — | Queen Budget |

### 6.3 城市合伙人联动

```
已有: 城市合伙人推广 StarClaw → CommissionTier 佣金
      Bronze 40% → Silver 45% → Gold 50% → Diamond 55%

新增联动:
  城市合伙人推荐企业客户 → 企业用 DevClaw 开发产品
  ├── 开发费: 合伙人获 15% 佣金（首年），10%（续费）
  ├── 托管费: 合伙人获 5% 持续分成
  └── 市场售卖: 不叠加（走创作者分成体系）

示例:
  合伙人推荐餐饮企业 → 企业用 DevClaw 开发外卖小程序
  → 开发费 10000⚡ × 15% = 1500⚡ 给合伙人
  → 每月托管费 2000⚡ × 5% = 100⚡/月 给合伙人
  → 合伙人年收入: 1500 + 100×12 = 2700⚡ 从单客户
```

### 6.4 用户升级路径（漏斗）

```
免费用户 (0⚡)
  │  体验 AI 对话
  ▼
充值用户 (首充 100⚡)
  │  使用更多技能、RAG、工作流
  ▼
DevClaw 用户 (项目开发 1000⚡+)
  │  让 AI 帮做产品
  ▼
创作者 (上架市场，月收入 5000⚡+)
  │  产品卖给其他用户
  ▼
企业客户 (月消耗 10000⚡+)
  │  DevClaw 全栈开发 + 托管 + 维护
  ▼
城市合伙人 (推荐 10+ 企业客户)
     持续佣金收入
```

---

## 七、实施路线

### 7.1 场景 A（用户产品开发）

| 阶段 | 内容 | 依赖 |
|------|------|------|
| **A0** | Project 数据模型 + CRUD API | Claw API |
| **A1** | Project → Mission 自动转换 | Squad Engine |
| **A2** | 技术栈模板库（6 种类型的脚手架） | Drone Agent |
| **A3** | StarClaw Hosting（静态站 + Docker 服务） | 基础设施 |
| **A4** | HostingRevenue 分成结算 | Queen Settlement |
| **A5** | 维护订阅（Medic + Analyst + Drone 持续服务） | Instinct |
| **A6** | 用户项目 Dashboard（进度/成本/预览） | Claw Web |

### 7.2 场景 B（Queen 生态维护）

| 阶段 | 内容 | 依赖 |
|------|------|------|
| **B0** | QueenDevTask 模型 + 状态机 + API | Queen API |
| **B1** | 7 个 DevClaw Agent 模板 + 知识库 | Claw Agent + RAG |
| **B2** | DevTask → Dev Squad 自动编排 | Squad Engine |
| **B3** | Quality Gate（编译+测试+评分+合并） | Sprint CI |
| **B4** | Evolution Engine（对话/统计/日志/竞品分析） | Queen Instinct |
| **B5** | Healing Engine（7×24 监控+诊断+修复+验证） | B0-B3 |
| **B6** | 技能自进化（DevClaw 开发新 Tool/Plugin） | B0-B4 |
| **B7** | Queen Dashboard（DevTask/效率/成本/ROI） | Queen Web |

### 7.3 优先级建议

```
Phase 1 (MVP): A0 + A1 + B0 + B1
  → 用户可以通过对话让 Claw 开发项目
  → Queen 可以手动创建 DevTask 给 DevClaw 执行

Phase 2 (自动化): A2 + A3 + B2 + B3
  → 用户项目有脚手架 + 可托管部署
  → Queen DevTask 全自动 Squad 编排 + 质量门

Phase 3 (进化): A4 + A5 + B4 + B5
  → 用户产品可变现 + 持续维护
  → Queen 自动发现生态缺口 + 自愈

Phase 4 (飞轮): A6 + B6 + B7
  → 完整 Dashboard
  → 技能自进化闭环
```

---

## 八、安全与治理

### 8.1 场景 A 安全

| 风险 | 措施 |
|------|------|
| 用户项目包含恶意代码 | 沙箱执行 + 托管时 Docker 隔离 |
| 用户预算超支 | 硬上限 + 实时余额检查 |
| 用户产品抄袭 | 市场上架前人工审核 |
| 用户产品违规内容 | AI 内容审核 + 举报机制 |

### 8.2 场景 B 安全

| 禁区 | 原因 | 处理 |
|------|------|------|
| 修改认证/鉴权代码 | 安全关键 | 必须人类审批 |
| 修改计费/支付代码 | 财务关键 | 必须人类审批 |
| 删除数据库数据 | 不可逆 | 必须人类审批 |
| 修改 DevClaw 自身逻辑 | 防止失控 | **绝对禁止** |
| 访问生产数据库 | 隐私合规 | 仅只读，需审批 |

### 8.3 共用护栏

```
1. 星能预算硬上限: 单任务 > 2× 预算 → 强制暂停
2. 回滚保险: 部署后 30 分钟关键指标下降 > 10% → 自动回滚
3. 人类升级: DevClaw 连续失败 3 次 → Bounty 赏金任务 + 人类通知
4. 审计链: 所有变更记录到 AuditChain（不可篡改）
```

---

## 相关文档

| 文档 | 内容 |
|------|------|
| [team-agent-lifecycle.md](./team-agent-lifecycle.md) | Team Agent 全生命周期：部署→组团→赋能→运行→监控→验收→循环 |
| [overlord-team-agent.md](./overlord-team-agent.md) | Team Agent 作为 Overlord 企业级主打功能的完整方案 |

---

## 附：虫族类比

```
Team Agent  =  虫群协作范式（不是一只虫在干，而是一支虫队在协作）
Squad       =  虫队的驱体（执行框架：调度 + 执行 + 回调）
Topology    =  虫队的阵型（谁先谁后、谁并行、谁审查谁）
DevClaw     =  第一支虫队（开发团队 — 代表作）
MarketClaw  =  营销虫队（文案 + 设计 + 分析 — 企业级延伸）
SupportClaw =  客服虫队（调度 + 应答 + 升级 — 企业级延伸）
Overlord    =  企业领主（管控 + 编排 → Team Agent 是其尖刀功能）
用户        =  委托人（出钱出需求，DevClaw 帮他造巢）
Queen       =  虫后（双重身份：帮用户调度 + 维护自己的虫群）
Evolution   =  自然选择（淘汰劣质技能，进化优质技能）
Self-Heal   =  免疫系统（检测病变 → 消灭 → 产生抗体）
Marketplace =  虫群集市（用户造的巢 + 虫队模板 都可交易）
Hosting     =  虫后的领地（用户产品可以住在虫后地盘，交租金）

产品三层:
  个人版 (Claw):     免费开源，Squad 自由组队
  企业版 (Overlord): Team Agent 模板化团队 + 管控 + 审批 + 月报
  平台版 (Queen):    虫后生态维护 + 市场运营 + 全局调度

最终目标:
  用户说一句话 → 一支 AI 团队帮他造产品 → 产品在虫群集市卖钱
  企业花 ¥2,000/月 → 雇 3 支 AI 团队 → 替代 30% 重复性人力
  虫后自动发现缺陷 → 一支 AI 团队自愈自进化 → 虫群越来越强
  DevClaw 是第一个团队智能体，但不会是最后一个。
```
