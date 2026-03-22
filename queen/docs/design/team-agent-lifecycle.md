# Team Agent Lifecycle — 团队智能体全生命周期

> 从 Claw 部署到组团、赋能、运行、监控、验收、循环的完整闭环。
> **Squad Engine 是骨架，Team Agent 是灵魂。**

---

## 一、全景流程

```
┌─────────────────────────────────────────────────────────────────────┐
│                    Team Agent 全生命周期                              │
│                                                                     │
│  Phase 0        Phase 1       Phase 2       Phase 3                 │
│  ┌──────┐      ┌──────┐      ┌──────┐      ┌──────┐               │
│  │ 部署  │ ──→ │ 组团  │ ──→ │ 赋能  │ ──→ │ 启动  │               │
│  │Deploy │      │Form  │      │Equip │      │Launch│               │
│  └──────┘      └──────┘      └──────┘      └──┬───┘               │
│                                                 │                   │
│                                                 ▼                   │
│  Phase 7        Phase 6       Phase 5       Phase 4                 │
│  ┌──────┐      ┌──────┐      ┌──────┐      ┌──────┐               │
│  │ 循环  │ ◄── │ 验收  │ ◄── │ 监控  │ ◄── │ 运行  │               │
│  │Loop  │      │Accept│      │Watch │      │ Run  │               │
│  └──┬───┘      └──────┘      └──────┘      └──────┘               │
│     │                                                               │
│     └── 用户反馈/新需求/维护 → 回到 Phase 3                          │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

### 时间线示例

```
T+0s     用户: "帮我做一个宠物电商小程序"
T+2s     Phase 0: Claw 节点已在线 (已部署)
T+3s     Phase 1: 自动选择 DevClaw 模板 → 创建 Squad
T+5s     Phase 2: 实例化 7 个角色 Agent → 加载知识库 → 就绪检查
T+8s     Phase 3: Architect 生成方案 → 用户确认 → MissionStep DAG 生成
T+15s    Phase 4: Drone-A/B/C 并行编码，Tester 待命
T+20min  Phase 5: 仪表盘实时显示进度 60%，已消耗 800⚡
T+45min  Phase 4: 编码完成 → Reviewer 审查 → 评分 8.2 → 通过
T+50min  Phase 6: CI Gate 通过 → Preview URL → 用户验收
T+52min  用户: "客服入口放大一点"
T+53min  Phase 7: 新 Sprint → Drone 修改 → 5 分钟后重新验收 → 完成
```

---

## 二、Phase 0 — 部署与注册 (Deploy)

> 前提条件：Claw 节点已部署并在线。

```
已有流程（无需新增）:

1. 用户部署 Claw 节点
   → claw/api/cmd/server/main.go 启动
   → 加载 .env 配置
   → 初始化 DB, Redis, Router

2. 节点注册到 Hivemind
   → node/hivemind.go: RegisterSelf()
   → 上报 NodeCapability (CPU/GPU/Models/Region)

3. Agent 能力上报（每30秒心跳）
   → AgentCapability[] 随 heartbeat 广播
   → 其他节点知道 "这个节点有哪些 Agent，擅长什么"

4. Squad Engine 启动
   → squad/engine.go: Start()
   → 后台循环每5秒检查待执行 Mission
```

**Team Agent 新增：**

```
5. 注册 Team Agent 能力
   → 节点上报自己可以参与哪些 Team Agent 类型
   → NodeCapability.TeamRoles: ["architect", "drone", "tester", ...]
   → 这样 Hivemind 知道哪些节点可以组成 DevClaw 团队
```

| 已有代码 | 状态 |
|---------|------|
| `node/hivemind.go` NodeCapability + heartbeat | ✅ 已有 |
| `node/hivemind.go` AgentCapability broadcast | ✅ 已有 |
| `squad/engine.go` Start() 后台循环 | ✅ 已有 |
| NodeCapability.TeamRoles 字段 | 🆕 新增 |

---

## 三、Phase 1 — 组团 (Form)

> 从模板创建一支 Team Agent 团队。

### 3.1 触发方式

| 触发 | 场景 | 示例 |
|------|------|------|
| **用户对话** | 用户描述需求，LLM 判断需要团队协作 | "帮我做一个电商小程序" |
| **用户手动** | 用户在 UI 上选择 Team Agent 模板 | 点击 "创建 DevClaw 团队" |
| **Queen 触发** | Evolution Engine 发现需求 | "需要开发 PPT 生成技能" |
| **API 调用** | 外部系统通过 API 触发 | `POST /v1/teams` |

### 3.2 组团流程

```
Input: TeamAgentTemplate + 用户需求描述

Step 1: 选择模板
  ├── 用户指定: "用 DevClaw"
  └── 自动匹配: LLM 分析需求 → 推荐最佳 Team Agent 类型
      "帮我做电商" → DevClaw
      "帮我写营销文案" → MarketClaw
      "帮我调研竞品" → ResearchClaw

Step 2: 创建 Squad
  → model.Squad{
      Name:        "DevClaw-宠物电商",
      CaptainNode: selfNodeID,        // 当前节点做 Captain
      UserID:      userID,
      Status:      "forming",
      Tags:        ["development", "fullstack"],
    }
  → db.Create(&squad)

Step 3: 节点选择（单节点 or 多节点）
  ├── 单节点模式（大多数场景）:
  │   → 所有角色运行在用户自己的 Claw 节点
  │   → 创建 1 个 SquadMember (self)
  │
  └── 多节点模式（大任务 / Queen 生态）:
      → Hivemind.FindNodesForTeam(template.roles)
      → 为每个角色找到最佳节点
      → 发送 Squad 邀请 → 对方接受 → 创建 SquadMember
      → 已有: squad_peer.go HandleInvite

Step 4: Squad 状态 → "active"
  → db.Model(&squad).Update("status", "active")
```

### 3.3 节点选择策略

```
单节点（默认，适合 90% 场景）:

  用户的 Claw 节点
  ├── Architect Agent  (1 个)
  ├── Drone Agent      (1-3 个，按需并发)
  ├── Tester Agent     (1 个)
  ├── Reviewer Agent   (1 个)
  └── DocBot Agent     (1 个)

  所有 Agent 共享同一节点资源
  优点: 零延迟、无跨节点通信、简单
  限制: 并发受单节点算力限制


多节点（大项目 / Queen 生态维护）:

  Node-A (Captain): Architect + Reviewer
  Node-B (Worker):  Drone × 2
  Node-C (Worker):  Drone + Tester
  Node-D (Worker):  DocBot + Analyst

  Hivemind 选择算法:
  for each role in template.roles:
    candidates = hivemind.FindNodesWithCapability(role.code)
    best = rank by (load↓, latency↓, capability_match↑)
    assign role → best node
```

### 3.4 数据模型（新增）

```go
// TeamAgentTemplate — 团队智能体模板
type TeamAgentTemplate struct {
    ID          string    `json:"id" gorm:"type:varchar(36);primaryKey"`
    Name        string    `json:"name" gorm:"type:varchar(200);not null"`         // "DevClaw"
    Category    string    `json:"category" gorm:"type:varchar(50);index"`         // development | marketing | research | support
    Description string    `json:"description" gorm:"type:text"`
    Roles       string    `json:"roles" gorm:"type:json"`                         // []TeamRole JSON
    Topology    string    `json:"topology" gorm:"type:json"`                      // TopologyConfig JSON
    QualityGate string    `json:"quality_gate" gorm:"type:json"`                  // QualityGateConfig JSON
    Escalation  string    `json:"escalation" gorm:"type:json"`                    // EscalationConfig JSON
    IsOfficial  bool      `json:"is_official" gorm:"default:false"`               // 官方模板
    SourceID    string    `json:"source_id" gorm:"type:varchar(36)"`              // 市场来源
    UserID      string    `json:"user_id" gorm:"type:varchar(36);index"`          // 创建者
    CreatedAt   time.Time `json:"created_at"`
}

// TeamRole — 角色定义（存在 Roles JSON 中）
type TeamRole struct {
    Code         string   `json:"code"`          // "architect", "drone", "tester"
    Name         string   `json:"name"`          // "设计虫"
    SystemPrompt string   `json:"system_prompt"` // 角色专属 System Prompt
    Model        string   `json:"model"`         // 推荐模型
    Tools        []string `json:"tools"`         // ["code", "git", "web_search"]
    KnowledgeBase string  `json:"knowledge_base"` // 知识库 ID（可选）
    MaxInstances int      `json:"max_instances"` // 最大并发实例数
}

// TeamInstance — 运行中的团队实例
type TeamInstance struct {
    ID          string     `json:"id" gorm:"type:varchar(36);primaryKey"`
    TemplateID  string     `json:"template_id" gorm:"type:varchar(36);index;not null"`
    SquadID     string     `json:"squad_id" gorm:"type:varchar(36);index;not null"`   // → Squad
    UserID      string     `json:"user_id" gorm:"type:varchar(36);index;not null"`
    Name        string     `json:"name" gorm:"type:varchar(200)"`                     // "DevClaw-宠物电商"
    Status      string     `json:"status" gorm:"type:varchar(20);default:forming;index"` // forming → ready → running → paused → completed → disbanded
    RoleMap     string     `json:"role_map" gorm:"type:json"`                          // {role_code → agent_id} mapping
    Config      string     `json:"config" gorm:"type:json"`                            // 运行时配置
    EnergyBudget int      `json:"energy_budget" gorm:"default:0"`                     // 星能预算
    EnergyUsed   int      `json:"energy_used" gorm:"default:0"`                       // 已消耗星能
    CreatedAt   time.Time  `json:"created_at"`
    UpdatedAt   time.Time  `json:"updated_at"`
    DisbandedAt *time.Time `json:"disbanded_at"`
}
```

### 3.5 API

```
POST /v1/teams                           — 从模板创建团队
{
  "template_id": "devclaw-v1",            // 或 "auto" 自动匹配
  "name": "宠物电商开发",
  "goal": "做一个宠物用品电商小程序",      // 需求描述
  "energy_budget": 5000,                   // 星能预算（可选）
  "multi_node": false                      // 是否多节点（默认单节点）
}

GET  /v1/teams                           — 我的团队列表
GET  /v1/teams/:id                       — 团队详情
POST /v1/teams/:id/disband               — 解散团队
GET  /v1/team-templates                  — 可用模板列表
GET  /v1/team-templates/:id              — 模板详情
```

---

## 四、Phase 2 — 赋予职能 (Equip)

> 为团队中每个角色创建专属 Agent 实例，加载工具和知识库。

### 4.1 角色实例化流程

```
Input: TeamInstance + TeamAgentTemplate.Roles[]

for each role in template.roles:

  Step 1: 创建 Agent 实例
    agent = model.Agent{
      UserID:       teamInstance.UserID,
      Name:         fmt.Sprintf("%s-%s", teamInstance.Name, role.Name),
      SystemPrompt: role.SystemPrompt,
      ModelName:    role.Model,       // 或用户覆盖
      Tools:        role.Tools,       // JSON array
      KnowledgeBaseID: role.KnowledgeBase,
      Config:       json.Marshal({
        "team_id":   teamInstance.ID,
        "role_code": role.Code,
        "is_team_agent": true,
      }),
    }
    db.Create(&agent)

  Step 2: 绑定工具集
    for each tool in role.Tools:
      verify tool exists in toolRegistry
      // code, git, web_search, http_request, document, bounty

  Step 3: 绑定知识库
    if role.KnowledgeBase != "":
      verify knowledge base exists
      agent.KnowledgeBaseID = role.KnowledgeBase
    else:
      // 自动创建项目级知识库
      kb = createProjectKnowledgeBase(teamInstance)
      agent.KnowledgeBaseID = kb.ID

  Step 4: 注册为 SquadMember
    member = model.SquadMember{
      SquadID:     teamInstance.SquadID,
      NodeID:      selfNodeID,
      Role:        role.Code,
      Specialty:   role.Code,
      AgentExport: json.Marshal(agentSummary),
      Status:      "online",
    }
    db.Create(&member)

  Step 5: 更新 RoleMap
    roleMap[role.Code] = agent.ID
```

### 4.2 角色专属 System Prompt 模板

```
Architect (设计虫):
  "你是 {{team_name}} 的首席架构师。
   你负责技术方案设计、架构决策、技术选型。
   输出格式: 结构化设计文档 + 技术栈选择 + 模块划分 + API 设计。
   你的设计方案会交给编码虫执行，所以必须足够具体可执行。"

Drone (编码虫):
  "你是 {{team_name}} 的全栈开发工程师。
   你负责编写实际可运行的代码。
   工作流程: 读取设计方案 → 编码 → git add → git commit → git push。
   你的代码会被审查虫审查，注意代码质量和可测试性。"

Tester (测试虫):
  "你是 {{team_name}} 的测试工程师。
   你负责编写和执行测试用例。
   覆盖: 单元测试 + 集成测试 + 边界条件。
   输出: 测试代码 + 测试报告 + 覆盖率。"

Reviewer (审查虫):
  "你是 {{team_name}} 的代码审查专家。
   审查标准: 安全性 > 正确性 > 可读性 > 性能。
   输出 JSON: { verdict: approved/changes_requested, score: 1-10, issues: [], suggestions: [] }
   评分 ≥ 7 通过，< 7 退回重写。严格但务实。"

DocBot (文档虫):
  "你是 {{team_name}} 的技术文档工程师。
   你负责编写 README、API 文档、部署文档。
   风格: 简洁、有示例、面向新手。"

Medic (医疗虫):
  "你是 {{team_name}} 的运维工程师。
   你负责监控系统健康、诊断问题、执行修复。
   优先: 快速恢复服务 > 根因分析 > 长期优化。"

Analyst (分析虫):
  "你是 {{team_name}} 的数据分析师。
   你负责分析用户行为、系统性能、错误趋势。
   输出: 数据报告 + 可视化 + 改进建议。"
```

### 4.3 就绪检查 (Readiness Check)

```
组团完成前，执行就绪检查:

✅ 每个角色的 Agent 已创建
✅ 每个 Agent 的工具可用 (toolRegistry.Has)
✅ 知识库已绑定（如有）
✅ LLM 模型可用 (provider 可创建)
✅ Git 可用（code 工具依赖）
✅ 预算充足 (energy_budget > 预估最低消耗)

全部通过 → TeamInstance.Status = "ready"
任一失败 → 返回失败原因 → 用户修复后重试
```

### 4.4 API

```
GET  /v1/teams/:id/roles                — 查看角色列表及状态
POST /v1/teams/:id/equip                — 手动触发赋能（通常自动）
GET  /v1/teams/:id/readiness            — 就绪检查结果
POST /v1/teams/:id/roles/:code/config   — 修改角色配置（模型/Prompt/工具）
```

---

## 五、Phase 3 — 启动任务 (Launch)

> 用户描述需求 → Architect 生成方案 → 用户确认 → 生成 MissionStep DAG。

### 5.1 任务启动流程

```
Input: 用户需求描述 (goal)

Step 1: 创建 Mission
  mission = model.Mission{
    SquadID:     teamInstance.SquadID,
    Title:       "宠物电商小程序",
    Goal:        userGoal,
    Status:      "planning",
    CaptainNode: selfNodeID,
    UserID:      userID,
    MaxSprints:  4,
  }
  → db.Create(&mission)

Step 2: Architect 设计方案（新增，替代直接 LLM 规划）
  ┌─────────────────────────────────────────────┐
  │ 与已有 engine.generatePlan() 的区别:         │
  │                                             │
  │ 已有: LLM 自由规划 + 按 specialty 分配       │
  │ 新增: Architect Agent 规划 + 按 topology 分配│
  │                                             │
  │ Architect Agent 知道:                        │
  │ - 团队有哪些角色 (从 template.roles)         │
  │ - 协作拓扑 (从 template.topology)            │
  │ - 质量标准 (从 template.quality_gate)        │
  │ → 生成更精准的执行方案                       │
  └─────────────────────────────────────────────┘

  architectAgent = roleMap["architect"]
  designTask = model.Task{
    AgentID: architectAgent.ID,
    Goal:    buildArchitectPrompt(goal, template),
  }
  → 执行 → 获得设计方案 (JSON)

Step 3: 用户确认（场景 A）或 自动确认（场景 B）
  场景 A: → WebSocket 推送方案给用户
          → 用户查看: 技术栈、模块划分、预估工时/成本
          → 用户: "确认" / "修改XX" / "取消"

  场景 B: → Queen 自动审批（score > 8 直接通过）
          → 或管理员审批

Step 4: 方案 → MissionStep DAG
  根据 topology.flow[] 生成步骤:

  flow: [
    { from: "start",    to: "architect",  type: "pipeline" },
    { from: "architect", to: "drone",      type: "fan_out"  },
    ...
  ]

  → 生成 MissionStep[]:
    Step 0: Architect 细化设计 (depends: [])
    Step 1: Drone-A 后端 API  (depends: [Step 0])
    Step 2: Drone-B 前端页面  (depends: [Step 0])     ← Fan-out
    Step 3: Drone-C 工具集成  (depends: [Step 0])     ← Fan-out
    Step 4: Tester 测试       (depends: [1, 2, 3])    ← Fan-in
    Step 5: Reviewer 审查     (depends: [4])
    Step 6: DocBot 文档       (depends: [5], condition: score ≥ 7)

Step 5: 创建 Sprint 0 + 分配 Git 分支
  → 已有: engine.planAndDispatch() 逻辑
  → Sprint 0 开始执行
```

### 5.2 Architect 规划 Prompt

```
你是 {{team_name}} 的首席架构师。请为以下需求设计技术方案和执行计划。

## 需求
{{user_goal}}

## 你的团队
{{for role in roles}}
- {{role.Name}} ({{role.Code}}): {{role.SystemPrompt[:50]}}... 可用工具: {{role.Tools}}
  最大并发: {{role.MaxInstances}}
{{end}}

## 协作拓扑
{{topology.type}}: {{topology.flow 描述}}

## 输出格式 (JSON)
{
  "tech_stack": {
    "frontend": "Next.js + Tailwind",
    "backend": "Supabase",
    "deployment": "StarClaw Hosting"
  },
  "modules": [
    { "name": "商品管理", "drone_count": 1, "estimated_hours": 2 }
  ],
  "steps": [
    {
      "title": "后端 API",
      "task": "详细任务描述...",
      "role": "drone",                    // ← 指定角色而非 specialty
      "depends_on": [],
      "estimated_energy": 500
    }
  ],
  "total_estimated_energy": 2000,
  "total_estimated_hours": 4
}
```

### 5.3 API

```
POST /v1/teams/:id/missions              — 启动新任务
{
  "goal": "做一个宠物用品电商小程序，带 AI 客服和推荐",
  "auto_confirm": false                    // true = 跳过用户确认
}

GET  /v1/teams/:id/missions               — 任务列表
GET  /v1/teams/:id/missions/:mid          — 任务详情
POST /v1/teams/:id/missions/:mid/confirm  — 确认方案
POST /v1/teams/:id/missions/:mid/cancel   — 取消任务
POST /v1/teams/:id/missions/:mid/feedback — 用户反馈
```

---

## 六、Phase 4 — 运行与协作 (Run)

> 拓扑驱动调度，角色间协作，Review Loop 保证质量。

### 6.1 拓扑驱动调度

```
已有: engine.advanceMission() — 基于 DependsOn 的依赖图调度
新增: 拓扑模式驱动，更智能的调度

Pipeline (A → B → C):
  → 已有逻辑完美支持：Step B depends_on [Step A]

Fan-out (A → [B, C, D]):
  → 已有逻辑完美支持：Steps B/C/D depends_on [Step A]，无互相依赖
  → 已有: engine.dispatchStep() 用 go 协程并发 dispatch

Fan-in ([B, C, D] → E):
  → 已有逻辑完美支持：Step E depends_on [B, C, D]

Review Loop (A → B → A):
  → 已有部分: triggerAutoReview → approved/changes_requested
  → 需增强: changes_requested 时自动重新 dispatch（当前只是 log）
```

### 6.2 Review Loop 增强（关键改动）

```go
// 当前行为（sprint_lifecycle.go:193-198）:
// changes_requested → 只 log，然后 auto-approve
// 这需要改为真正的 retry 循环

// 新行为:
func (e *Engine) completeReview(reviewID string, step, mission, verdict, comments, diffSummary) {
    if verdict == "approved" {
        e.finalizeStepDone(step, mission)
    } else {
        // 检查重试次数
        var retryCount int64
        e.db.Model(&StepReview{}).Where("step_id = ?", step.ID).Count(&retryCount)

        if retryCount >= maxReviewRetries {
            // 超过重试次数 → 升级策略
            e.handleEscalation(step, mission, comments)
            return
        }

        // 将 review 反馈注入到 step 的 context 中
        feedbackContext := fmt.Sprintf(
            "## 审查反馈 (第 %d 次)\n审查结果: 需要修改\n%s\n\n请修正以上问题后重新提交。",
            retryCount, comments,
        )

        // 重置 step 状态，带上反馈 context
        e.db.Model(&step).Updates(map[string]interface{}{
            "status": "pending",
            "input":  step.Input + "\n\n" + feedbackContext,
        })

        // 重新触发 dispatch
        e.advanceMission(mission.ID)
    }
}
```

### 6.3 升级策略 (Escalation)

```
当 Review Loop 达到 max_retries:

template.escalation.on_max_retries 决定行为:

  "bounty"        → BountyTool.Execute(post_bounty, {
                      title: step.Task,
                      description: "AI 尝试 3 次未通过审查: " + comments,
                      category: "development",
                      reward: estimatedEnergy,
                    })

  "pause_notify"  → TeamInstance.Status = "paused"
                  → WebSocket 通知用户: "步骤 X 需要人工介入"

  "human_review"  → 创建人类审批任务
                  → 等待人类指令: 重写/跳过/手动修复
```

### 6.4 Handoff 协议（角色间上下文传递）

```
每个 MissionStep 执行时，自动注入上游 context:

Step 0 (Architect) 输出设计方案
  ↓ output 存入 step.Output
  ↓
Step 1 (Drone-A) 的 input = Step 0 的 output
  → Drone 看到完整设计方案 → 只做自己负责的模块
  ↓ output 存入 step.Output
  ↓
Step 4 (Tester) 的 input = Step 1 + 2 + 3 的 output
  → Tester 看到所有代码 → 写全面的测试
  ↓
Step 5 (Reviewer) 的 input = Step 4 output + 所有代码 diff
  → Reviewer 看到测试结果 + 代码 → 综合评审

已有: engine.advanceMission() 中 contextParts 收集上游 output
增强: 按角色注入不同的 context 模板
```

### 6.5 共享工作区

```
已有:
  - GitManager: InitMissionRepo → bare repo
  - CloneForStep: 每个 step 一个分支
  - MergeBranches: CI Gate 时合并所有分支

新增:
  - 角色感知的分支命名: sprint-0/architect-design, sprint-0/drone-a-backend
  - 共享 RAG: 项目代码自动索引到知识库
  - 共享记忆: 设计决策、踩坑记录存入 Memory
```

---

## 七、Phase 5 — 监控 (Watch)

> 实时追踪团队运行状态、进度、成本。

### 7.1 仪表盘数据源

```
已有: HiveBroadcaster (每5秒推送)
  → HiveStepUpdate:  每个 step 的状态/分支/输出
  → HiveSprintStatus: sprint 进度/目标/完成率

新增: TeamDashboard (增强 HiveBroadcaster)
  → TeamStatus:     团队整体状态
  → RoleActivity:   每个角色当前在做什么
  → EnergyTracker:  实时星能消耗
  → QualityMetrics: 审查通过率/重试次数
  → Timeline:       事件时间线
```

### 7.2 TeamDashboard 数据结构

```go
type TeamDashboard struct {
    TeamID       string              `json:"team_id"`
    TeamName     string              `json:"team_name"`
    Status       string              `json:"status"`        // forming/ready/running/paused/completed
    MissionID    string              `json:"mission_id"`
    MissionTitle string              `json:"mission_title"`

    // 进度
    SprintNumber int                 `json:"sprint_number"`
    TotalSteps   int                 `json:"total_steps"`
    DoneSteps    int                 `json:"done_steps"`
    Progress     int                 `json:"progress"`       // 0-100

    // 角色状态
    Roles        []RoleStatus        `json:"roles"`

    // 成本
    EnergyBudget int                 `json:"energy_budget"`
    EnergyUsed   int                 `json:"energy_used"`
    EnergyRate   float64             `json:"energy_rate"`     // ⚡/min

    // 质量
    ReviewScore  float64             `json:"review_score"`    // 最新审查评分
    RetryCount   int                 `json:"retry_count"`
    TestsPassed  bool                `json:"tests_passed"`

    // 预览
    PreviewURL   string              `json:"preview_url"`

    // 时间线
    Timeline     []TimelineEvent     `json:"timeline"`
}

type RoleStatus struct {
    Code        string `json:"code"`         // "architect"
    Name        string `json:"name"`         // "设计虫"
    Status      string `json:"status"`       // idle / working / reviewing / done
    CurrentTask string `json:"current_task"` // 当前任务描述
    Progress    int    `json:"progress"`     // 0-100
}

type TimelineEvent struct {
    Time    time.Time `json:"time"`
    Role    string    `json:"role"`
    Action  string    `json:"action"`   // "started", "completed", "review_passed", "review_failed", "error"
    Detail  string    `json:"detail"`
}
```

### 7.3 WebSocket 事件

```
新增 WebSocket 事件类型:

EventTeamStatus    = "team:status"       // 团队整体状态变更
EventTeamRole      = "team:role"         // 角色状态变更
EventTeamEnergy    = "team:energy"       // 能量消耗更新
EventTeamTimeline  = "team:timeline"     // 时间线新事件
EventTeamPreview   = "team:preview"      // 预览 URL 可用

已有（继续使用）:
EventHiveStepUpdate = "hive:step_update" // Step 状态更新
EventHiveSprint     = "hive:sprint"      // Sprint 进度
```

### 7.4 预算告警

```
实时监控星能消耗:

  energy_used / energy_budget > 60% → 黄色告警 (WebSocket)
  energy_used / energy_budget > 80% → 橙色告警 + 评估是否暂停
  energy_used / energy_budget > 100% → 红色告警 + 暂停执行

  escalation.on_budget_exceed = "pause_notify"
  → TeamInstance.Status = "paused"
  → 通知用户: "预算即将耗尽，是否追加？"
  → 用户: 追加 / 完成当前步骤后停止 / 立即停止
```

### 7.5 API

```
GET  /v1/teams/:id/dashboard             — 完整仪表盘数据
GET  /v1/teams/:id/timeline              — 事件时间线
GET  /v1/teams/:id/energy                — 能量消耗详情
WS   /v1/ws?team_id=xxx                  — WebSocket 实时推送
```

---

## 八、Phase 6 — 验收 (Accept)

> CI Gate → 预览 → 用户验收 → 部署。

### 8.1 自动验收流程 (CI Gate)

```
已有 (sprint_lifecycle.go):

Sprint 所有 Step 完成
  → checkSprintComplete()
  → runCIGate():
     Stage 1: Git Merge (所有分支合入 master)
     Stage 2: LLM Quality Check (代码质量评分)
     Stage 3: Auto Preview (启动预览服务器)
  → runRetrospective():
     分析成功率、失败原因、质量问题
     → 决定: 下一个 Sprint or 完成 Mission

新增 (Team Agent 层):

CI Gate 通过后 → Team Agent 验收流程:

  Step 1: 自动验收检查
    ├── quality_gate.review_threshold ≤ 7? → 需要人工验收
    ├── quality_gate.test_required → 测试必须通过
    └── quality_gate.build_required → 编译必须通过

  Step 2: 用户验收（场景 A）
    → WebSocket 推送: "Sprint 0 完成，请验收"
    → 用户看到: Preview URL + 代码 diff + 测试报告 + 审查评分
    → 用户操作:
      ├── "通过" → 进入部署
      ├── "修改意见" → 存入 sprint.UserFeedback → 新 Sprint
      └── "不满意，重做" → 重新规划

  Step 3: 自动验收（场景 B）
    → Queen 自动判断: score ≥ 8 + tests passed + build passed
    → 自动合并 → 自动部署
    → 通知管理员（异步）
```

### 8.2 交付物清单

```
任务完成时的交付物:

├── 代码
│   ├── Git 仓库 (master 分支，已 merge)
│   ├── 所有源文件
│   └── 测试文件
│
├── 文档
│   ├── README.md (DocBot 生成)
│   ├── API 文档 (如有)
│   └── 部署说明
│
├── 质量报告
│   ├── 审查评分: 8.2/10
│   ├── 测试覆盖率: 75%
│   ├── Sprint 回顾报告
│   └── 每个 Step 的 Review 记录
│
├── 运行环境
│   ├── Preview URL (已运行)
│   └── 部署配置 (Docker/Vercel)
│
└── 成本报告
    ├── 总消耗: 2350⚡
    ├── 按角色: Architect 200⚡, Drone 1500⚡, Tester 300⚡, Reviewer 200⚡, DocBot 150⚡
    └── 按 Sprint: Sprint 0: 2000⚡, Sprint 1: 350⚡
```

### 8.3 部署

```
验收通过后的部署:

场景 A（用户产品）:
  ├── StarClaw Hosting → 自动部署到平台
  ├── Vercel          → 推送到用户 Vercel 账户
  ├── 自有服务器       → 提供 Docker image + 部署脚本
  └── 市场发布         → 打包为 Agent/Plugin → 上架

场景 B（Queen 生态）:
  └── Nydus 部署 → git push nydus master → post-receive hook → 自动重启
```

### 8.4 API

```
POST /v1/teams/:id/missions/:mid/accept   — 验收通过
POST /v1/teams/:id/missions/:mid/reject   — 退回修改
{
  "feedback": "客服入口放大一点，首页加个轮播图"
}
POST /v1/teams/:id/missions/:mid/deploy   — 部署
{
  "target": "starclaw_hosting",            // starclaw_hosting | vercel | docker | marketplace
  "config": { "domain": "pet-shop.example.com" }
}
GET  /v1/teams/:id/missions/:mid/deliverables — 交付物清单
GET  /v1/teams/:id/missions/:mid/report       — 完整报告
```

---

## 九、Phase 7 — 循环 (Loop)

> Sprint 迭代、用户反馈、持续维护、团队进化。

### 9.1 Sprint 迭代循环

```
已有 (sprint_lifecycle.go):

shouldStartNextSprint():
  ├── 用户有反馈 → 必须迭代
  ├── Retro 有改进建议 + 质量问题 → 迭代
  └── 达到 MaxSprints → 完成

startNextSprint():
  → 生成改进计划 (incorporating feedback + retro hints)
  → 创建新 Sprint + Steps
  → dispatch → 执行 → 审查 → CI → Retro → 再判断

已有循环:
  Mission → Sprint 0 → CI → Retro → Sprint 1 → CI → Retro → ... → Complete
```

### 9.2 用户反馈驱动迭代

```
用户在任何时候可以提交反馈:

POST /v1/teams/:id/missions/:mid/feedback
{
  "feedback": "客服入口放大，首页加轮播图"
}

→ engine.SubmitFeedback() (已有)
→ 存入 sprint.UserFeedback
→ Retro 时: shouldStartNextSprint() = true
→ startNextSprint(): 将反馈注入新 Sprint 的 goal
```

### 9.3 持续维护模式 (Maintenance Mode)

```
任务完成后，团队不解散，进入维护模式:

TeamInstance.Status = "maintenance"

维护模式下:
  Medic  — 定期健康检查 (Instinct: @idle 每30分钟)
           → 检查服务是否正常响应
           → 检查错误日志
           → 发现问题 → 自动创建修复 Mission

  Analyst — 周报 (Instinct: @cron 每周一 9:00)
           → 分析用户访问数据
           → 生成运营报告 → 推送给用户

  Drone  — 待命，用户提新需求时激活
           → 用户: "加一个优惠券功能"
           → 自动创建新 Mission → Sprint → 执行

维护成本:
  → 按月收取维护费 (星能订阅)
  → 无活动时几乎零消耗（仅心跳检查）
  → 有事件时按需消耗
```

### 9.4 团队进化

```
团队运行越久，越聪明:

知识积累:
  每个 Sprint 完成后:
  → 设计决策 → 存入项目 Memory
  → 踩坑记录 → 存入项目 Memory
  → 代码结构 → 更新 RAG 索引

角色优化:
  Retro 分析角色表现:
  → Drone-A 效率高 → 下次优先分配复杂任务
  → Drone-B 经常被审查退回 → 调整 System Prompt 加强代码规范
  → Reviewer 过于严格 → 微调评分阈值

模板进化:
  多次使用后统计:
  → 平均消耗、平均时长、成功率
  → 自动优化模板参数
  → 版本化: DevClaw v1 → v2 → v3
```

---

## 十、状态机

### 10.1 TeamInstance 状态

```
forming ──→ ready ──→ running ──→ paused ──→ running
                        │                      │
                        ├──→ maintenance ───────┤
                        │                      │
                        ├──→ completed         │
                        │                      │
                        └──→ disbanded ◄───────┘
```

| 状态 | 含义 | 触发 |
|------|------|------|
| `forming` | 正在组团 + 赋能 | 创建 TeamInstance |
| `ready` | 就绪检查通过，等待任务 | Readiness Check 全部 ✅ |
| `running` | 正在执行任务 | Mission 开始 |
| `paused` | 暂停（预算/人工介入） | 预算超限 / 用户暂停 |
| `maintenance` | 维护模式 | Mission 完成 + 用户选择维护 |
| `completed` | 所有任务完成，团队闲置 | Mission 完成 |
| `disbanded` | 解散 | 用户解散 / 超时自动解散 |

### 10.2 Mission 状态（已有 + 增强）

```
planning ──→ confirming ──→ executing ──→ reviewing ──→ completed
                │                            │
                ├── cancelled                 ├── failed
                │                            │
                └── (用户修改方案后           └── (CI 失败 → 新 Sprint)
                     回到 planning)
```

新增 `confirming` 状态：Architect 出方案后等待用户确认。

### 10.3 完整执行流序列图

```
用户          TeamEngine       Architect    Drone(s)     Tester    Reviewer    DocBot
 │                │                │           │           │          │          │
 │─── 创建任务 ──→│                │           │           │          │          │
 │                │── 创建Mission─→│           │           │          │          │
 │                │                │           │           │          │          │
 │                │◄─ 设计方案 ────│           │           │          │          │
 │◄── 请确认 ─────│                │           │           │          │          │
 │─── 确认 ──────→│                │           │           │          │          │
 │                │                │           │           │          │          │
 │                │── DAG 生成 ───→│           │           │          │          │
 │                │                │           │           │          │          │
 │                │── Fan-out ────────→ dispatch ──────┐   │          │          │
 │                │                │    │  │  │        │   │          │          │
 │                │                │   D-A D-B D-C     │   │          │          │
 │                │                │    │  │  │        │   │          │          │
 │◄── 进度 60% ──│                │    ▼  ▼  ▼        │   │          │          │
 │                │◄─ Fan-in ──────────────────────────┘   │          │          │
 │                │                │           │           │          │          │
 │                │── dispatch ───────────────────────────→│          │          │
 │                │                │           │           │          │          │
 │                │◄─ 测试结果 ────────────────────────────│          │          │
 │                │                │           │           │          │          │
 │                │── dispatch ──────────────────────────────→│       │          │
 │                │                │           │           │  │       │          │
 │                │              ┌─────── Review Loop ────────┘       │          │
 │                │              │ score < 7 → retry Drone            │          │
 │                │              │ score ≥ 7 → pass ──────────────────│──→ dispatch
 │                │              └────────────────────────────────────│          │
 │                │                │           │           │          │          │
 │                │◄─ CI Gate ────────────────────────────────────────│──────────│
 │                │  merge + quality check                            │          │
 │                │                │           │           │          │          │
 │◄── 验收请求 ──│                │           │           │          │          │
 │── 通过 ───────→│                │           │           │          │          │
 │                │── deploy ─────→            │           │          │          │
 │◄── 完成 ──────│                │           │           │          │          │
 │                │                │           │           │          │          │
```

---

## 十一、实现优先级

### Phase 0 (MVP — 最小可行)

```
目标: 用户说需求 → 一支 AI 团队帮他开发 → 交付代码

需要做:
  1. TeamAgentTemplate 模型 + DevClaw 官方模板 (内置)
  2. TeamInstance 模型 + 创建/查询 API
  3. Phase 1-2 自动化: 选模板 → 创建 Squad → 实例化 Agent
  4. Phase 3 增强: Architect Agent 规划 (替代 generatePlan)
  5. Phase 6 增强: Review Loop 真正 retry (替代 auto-approve)

不需要做 (MVP 简化):
  - 多节点模式 (单节点即可)
  - 自定义模板 (只提供 DevClaw 官方模板)
  - 维护模式 (任务完成即结束)
  - Team Agent 市场
```

已有代码复用率:

| 组件 | 状态 | 复用 |
|------|------|------|
| Squad + SquadMember | ✅ 完全复用 | model/squad.go |
| Mission + Sprint + MissionStep | ✅ 完全复用 | model/squad.go |
| Engine.planAndDispatch | 🔄 增强 | 用 Architect Agent 替代 |
| Engine.advanceMission | ✅ 完全复用 | 依赖图调度 |
| Engine.dispatchStep | ✅ 完全复用 | 含本地/远程 |
| Engine.executeLocal | ✅ 完全复用 | Task → TaskWorker |
| triggerAutoReview | 🔄 增强 | 真正 retry |
| runCIGate | ✅ 完全复用 | merge + quality |
| runRetrospective | ✅ 完全复用 | Sprint 回顾 |
| startNextSprint | ✅ 完全复用 | 迭代循环 |
| HiveBroadcaster | 🔄 增强 | 加 Team 级别推送 |
| GitManager | ✅ 完全复用 | 分支管理 |

**预估新增代码量: ~800 行 Go + ~200 行前端**

### Phase 1 (增强)

```
  6. 用户确认流程 (confirming 状态 + WebSocket 推送方案)
  7. TeamDashboard 增强 + 前端页面
  8. 预算控制 + 告警
  9. 交付物清单 + 报告生成
```

### Phase 2 (扩展)

```
  10. 多节点模式
  11. MarketClaw / ResearchClaw 等模板
  12. 维护模式 (Medic + Analyst 持续运行)
  13. 自定义模板 + Team Agent 市场
  14. 团队进化 (模板版本化 + 参数自动优化)
```

---

## 十二、关键设计决策

### Q1: 为什么单节点优先？

```
90% 的用户任务在单节点即可完成。
单节点 = 零网络延迟 + 零跨节点通信 + 简单可靠。
角色并发通过 goroutine 实现，不需要物理节点分离。
多节点留给大项目和 Queen 生态维护。
```

### Q2: TeamAgentTemplate vs 直接用 Squad？

```
Squad 是通用的多节点协作框架，没有"角色"概念。
TeamAgentTemplate 加了一层:
  - 角色图 (谁做什么)
  - 协作拓扑 (谁先谁后)
  - 质量标准 (什么算完成)
  - 升级策略 (失败了怎么办)

类比: Squad = HTTP 框架, TeamAgent = 电商网站
```

### Q3: Architect Agent vs LLM 直接规划？

```
现有: engine.generatePlan() 用一次性 LLM 调用生成步骤
新增: Architect Agent 是一个持久化的 Agent，有 System Prompt + 知识库

优势:
  1. Architect 知道团队构成（不是泛化的 "coding/design"）
  2. Architect 有项目知识库（了解已有代码和架构）
  3. Architect 输出可以被用户审查和修改
  4. Architect 的经验在 Memory 中积累
```

### Q4: 与 Workflow Engine 的关系？

```
Workflow Engine (workflow/engine.go): 执行 DAG 节点（LLM/Tool/Condition）
Squad Engine (squad/engine.go): 执行跨节点 Mission + Sprint

Team Agent 用 Squad Engine，不用 Workflow Engine。
原因: Team Agent 需要 Git 协作、Sprint 迭代、跨节点调度
      这些 Workflow Engine 不支持

未来: Team Agent 的某个 Step 内部可以调用 Workflow
      例如: Drone 编码时，用 Workflow 自动化 lint → test → format
```
