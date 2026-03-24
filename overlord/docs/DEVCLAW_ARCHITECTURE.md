# DevClaw 架构设计

## 一、核心理念

> DevClaw 不是"一个 Claw"，而是**一群 Claw 组成的团队**。
> 每个角色 = 一个独立的 Agent（智能体），有自己的身份、模型、技能。
> 技能从 Queen 市场安装，而非硬编码。

```
用户视角:
┌───────────── DevClaw Team ──────────────┐
│                                          │
│   🏗️ 设计虫   ⚡ 编码虫×3   🧪 测试虫   │
│   🔍 审查虫   📝 文档虫                  │
│                                          │
│   每个角色 = 一个 Claw Agent             │
│   每个 Agent 有自己的技能（从市场安装）   │
│   团队通过 DAG 拓扑协作                  │
└──────────────────────────────────────────┘
```

## 二、三层架构

```
┌─────────────────────────────────────────────────────────┐
│  Layer 3: Overlord — 团队编排层                          │
│  职责: 模板管理、实例生命周期、任务分发、进度追踪         │
│  关键: TeamInstance → 绑定 N 个 Claw 节点               │
│                                                          │
│  ┌────────────┐  ┌────────────┐  ┌────────────┐        │
│  │  Template   │  │  Instance  │  │  Mission   │        │
│  │  Registry   │  │  Manager   │  │  Router    │        │
│  └────────────┘  └────────────┘  └────────────┘        │
│         ▲                │              │                │
│         │                ▼              ▼                │
├─────────┼────────────────────────────────────────────────┤
│  Layer 2: Claw — Agent 运行时层                          │
│  职责: 执行单个 Agent 的推理、工具调用、输出              │
│                                                          │
│  ┌────────────────────────────────────────────────────┐  │
│  │  Claw Node (devclaw-local)                         │  │
│  │                                                     │  │
│  │  ┌──────────────────────────────────────────┐      │  │
│  │  │  Squad Engine (协调器)                    │      │  │
│  │  │  mission → plan → dispatch → review       │      │  │
│  │  └──────────────┬───────────────────────────┘      │  │
│  │                 │                                   │  │
│  │  ┌──────┐ ┌──────┐ ┌──────┐ ┌──────┐ ┌──────┐   │  │
│  │  │设计虫│ │编码虫│ │测试虫│ │审查虫│ │文档虫│   │  │
│  │  │Agent │ │Agent │ │Agent │ │Agent │ │Agent │   │  │
│  │  │      │ │  ×3  │ │      │ │      │ │      │   │  │
│  │  └──┬───┘ └──┬───┘ └──┬───┘ └──┬───┘ └──┬───┘   │  │
│  │     │ skills  │ skills  │ skills │ skills │ skills │  │
│  │     ▼        ▼        ▼        ▼        ▼        │  │
│  │  ┌──────────────────────────────────────────┐      │  │
│  │  │  Tool Registry (技能注册表)               │      │  │
│  │  │  每个 Agent 只能调用自己安装的技能         │      │  │
│  │  └──────────────────────────────────────────┘      │  │
│  └────────────────────────────────────────────────────┘  │
│                         ▲                                 │
├─────────────────────────┼─────────────────────────────────┤
│  Layer 1: Queen — 市场与技能层                            │
│  职责: 技能/Agent/工作流的发布、审核、分发                │
│                                                          │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐  │
│  │  Agent   │ │  Skill   │ │ Workflow │ │   MCP    │  │
│  │ Template │ │  Plugin  │ │  Chain   │ │  Server  │  │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘  │
└─────────────────────────────────────────────────────────┘
```

## 三、核心概念定义

### 3.1 Agent（智能体）

每个 Agent 是一个独立的 AI 实体，定义:

```json
{
  "id": "agent-architect-001",
  "name": "设计虫",
  "role_code": "architect",
  "identity": {
    "system_prompt": "你是首席架构师...",
    "model": "gpt-4o",
    "temperature": 0.3
  },
  "skills": [
    { "id": "skill-code-exec",   "source": "queen-marketplace", "version": "1.2.0" },
    { "id": "skill-web-search",  "source": "queen-marketplace", "version": "2.0.1" },
    { "id": "skill-doc-reader",  "source": "queen-marketplace", "version": "1.0.0" }
  ],
  "permissions": {
    "can_read_git": true,
    "can_write_git": false,
    "can_deploy": false,
    "max_tokens_per_turn": 8000
  },
  "memory": {
    "type": "conversation",
    "max_context_tokens": 32000
  }
}
```

**关键区别**: 现在的 `TeamRole` 只是 prompt + hardcoded tool 名。升级后的 Agent 有:
- **安装的技能** (从 Queen 市场下载，有版本号)
- **权限边界** (不是所有 Agent 都能 git push)
- **独立记忆** (每个 Agent 有自己的上下文窗口)

### 3.2 Skill（技能）

技能 = Agent 能调用的工具，来自 Queen 市场:

```json
{
  "id": "skill-code-exec",
  "name": "代码执行",
  "type": "skill",
  "version": "1.2.0",
  "spec": {
    "function_name": "code",
    "description": "在沙箱中执行代码",
    "parameters": { "language": "string", "code": "string" },
    "runtime": "sandbox"
  },
  "installed_by": ["architect", "drone", "tester"]
}
```

**现状 vs 目标**:
| 维度 | 现状 | 目标 |
|------|------|------|
| 技能来源 | 硬编码字符串 `["code", "git"]` | Queen 市场安装，有版本 |
| 技能隔离 | 所有 Agent 共享 ToolRegistry | 每个 Agent 有独立的 skill scope |
| 技能更新 | 改代码重新部署 | 市场一键更新 |
| 自定义技能 | 不支持 | 用户可开发上架 |

### 3.3 Mission（任务）

任务是团队协作的单元:

```
Mission: "开发一个用户登录功能"
  │
  ├── Sprint 1: 设计阶段
  │   └── Step: architect → 输出设计文档
  │
  ├── Sprint 2: 编码阶段
  │   ├── Step: drone[0] → 实现后端 API
  │   ├── Step: drone[1] → 实现前端页面
  │   └── Step: drone[2] → 编写数据库 migration
  │
  ├── Sprint 3: 测试阶段
  │   └── Step: tester → 编写并执行测试用例
  │
  ├── Sprint 4: 审查阶段
  │   └── Step: reviewer → 审查代码质量
  │   (score < 7 → 退回 Sprint 2 重做)
  │
  └── Sprint 5: 文档阶段
      └── Step: docbot → 编写 README + API 文档
```

## 四、数据流

### 4.1 创建 DevClaw 实例

```
用户 (Overlord Console)
  │
  ├── 1. 选择 DevClaw 模板
  ├── 2. 选择 Claw 节点 (devclaw-local)
  ├── 3. 确认创建
  │
  ▼
Overlord API
  │
  ├── 4. 创建 TeamInstance
  ├── 5. 读取模板 roles[]
  ├── 6. 对每个 role:
  │      ├── 从 Queen 市场查询需要的 skills
  │      ├── 调用 Claw API 安装 skills
  │      └── 调用 Claw API 注册 Agent
  ├── 7. 调用 Claw API 创建 Squad
  │      (传入 agents[] + topology DAG)
  │
  ▼
Claw Node (devclaw-local)
  │
  ├── 8. 注册各 Agent (system_prompt + model + skills)
  ├── 9. 初始化 Squad Engine
  ├── 10. 状态 → "active"
  │
  ▼
用户可以开始创建 Mission
```

### 4.2 执行 Mission

```
用户: "开发一个用户登录功能"
  │
  ▼
Overlord → Claw Squad Engine
  │
  ├── Phase 1: Planning
  │   Captain (设计虫) 分析目标 → 输出开发计划
  │   计划包含: steps[], 每个 step 指定执行角色
  │
  ├── Phase 2: Execution (按 DAG 拓扑)
  │   ┌── architect: 设计文档 ──┐
  │   │                         ▼
  │   │   drone[0]: 后端 ────┐
  │   │   drone[1]: 前端 ────┤ fan_out → fan_in
  │   │   drone[2]: DB    ───┘
  │   │                    ▼
  │   │   tester: 测试 ──── reviewer: 审查
  │   │                              │
  │   │        score ≥ 7? ───────────┤
  │   │        yes ──→ docbot: 文档  │
  │   │        no  ──→ 退回 drone    │
  │   └──────────────────────────────┘
  │
  ├── Phase 3: Quality Gate
  │   审查虫打分 → ≥ 7 通过 / < 7 退回重做
  │
  └── Phase 4: Delivery
      文档虫输出 README → Mission 完成
```

### 4.3 技能安装流程

```
Overlord
  │ "设计虫需要 code + web_search + doc_reader"
  │
  ├── 1. GET Queen /marketplace/items?type=skill&name=code
  │      → 返回 skill spec + version
  │
  ├── 2. POST Claw /v1/internal/install-skill
  │      body: { agent_id: "architect", skill_id: "xxx", spec: {...} }
  │
  ▼
Claw Node
  │
  ├── 3. 下载 skill spec (如果是 MCP 类型，启动 MCP server)
  ├── 4. 注册到该 Agent 的 ToolRegistry scope
  └── 5. 返回 installed: true
```

## 五、关键设计决策

### 5.1 单 Claw vs 多 Claw

| 方案 | 优点 | 缺点 |
|------|------|------|
| **单 Claw 多 Agent** (Phase 1) | 轻量，一套 MySQL/Redis | Agent 间无进程隔离 |
| **多 Claw 集群** (Phase 2) | 真隔离，可扩缩容 | 每个需 MySQL/Redis，资源重 |
| **混合模式** (Phase 3) | 灵活，按需扩展 | 复杂度高 |

**推荐**: Phase 1 先做单 Claw 多 Agent（当前节点 devclaw-local:8080），架构预留多 Claw 扩展能力。

### 5.2 技能隔离

```go
// 现在: 全局 ToolRegistry，所有 Agent 共享
toolRegistry.Execute("code", params) // 任何人都能调用

// 目标: Agent-scoped ToolRegistry
agentScope := toolRegistry.ScopeFor("architect") // 只含 architect 安装的技能
agentScope.Execute("code", params) // OK
agentScope.Execute("git_push", params) // ❌ architect 没安装 git_push
```

### 5.3 与现有代码的对接

```
现有组件          →  角色
─────────────────────────────
Squad Engine      →  Mission 执行引擎 (已有 plan/dispatch/track)
Agent Runtime     →  单个 Agent 的 ReAct 循环 (已有)
Overlord Client   →  Claw ↔ Overlord 通信 (已有)
Queen Marketplace →  技能/Agent 的商店 (已有 MarketplaceItem)
Tool Registry     →  技能注册表 (需改造: 全局 → Agent 级)
```

**最小改动点**:
1. `tool.Registry` — 增加 `ScopeFor(agentID)` 方法
2. `squad.Engine` — 增加 `agent_id` 到 step 分发
3. Overlord `TeamAgentHandler` — 增加 skill 安装调用
4. Queen `MarketplaceHandler` — 增加 install API

## 六、实现路线图

### Phase 1: Agent 化 (当前迭代)

**目标**: 让 DevClaw 的每个角色成为真正的 Agent，技能从市场安装

```
1. Claw 增加 /v1/internal/agents API
   - POST /agents — 注册 Agent (system_prompt + model + skill_ids)
   - GET  /agents — 列出已注册 Agent
   - POST /agents/:id/execute — 用指定 Agent 执行一个 turn

2. Claw Tool Registry 增加 Agent Scope
   - ScopeFor(agentID) → 只返回该 Agent 安装的技能

3. Queen 增加 install API
   - POST /marketplace/install — 返回 skill spec 供 Claw 注册

4. Overlord CreateInstance 流程升级
   - 读取 template roles → 对每个 role:
     - 查 Queen 市场匹配 skills
     - 调 Claw 安装 skills + 注册 Agent
   - 创建 Squad (agents + topology)
```

### Phase 2: Mission 执行引擎

**目标**: 在 Overlord Console 创建任务，自动按 DAG 拓扑分发执行

```
1. Overlord → Claw: dispatch mission
   - POST /v1/internal/squad/missions — 创建 mission
   - Claw Squad Engine 接管: plan → dispatch → track

2. 实时进度
   - WebSocket 推送: step 状态变更 → Overlord → Console
   - 每个 step: pending → running → review → done/retry

3. Quality Gate
   - 审查虫打分 → Overlord 判断是否通过
   - < 阈值 → 退回指定 step 重做
```

### Phase 3: 多 Claw 集群

**目标**: 横向扩展，不同角色跑在不同 Claw 节点

```
1. Overlord 管理 Claw 节点池
   - 每个节点有 capabilities (GPU / memory / installed skills)
   - 根据 role 需求自动分配节点

2. Claw 间通信
   - 通过 Nydus 隧道或 HTTP 直连
   - Git 仓库作为共享工作区

3. 弹性扩缩容
   - 编码虫需求大 → 自动扩 Claw 节点
   - 空闲 → 缩容回收资源
```

## 七、数据模型变更

### Overlord 侧

```sql
-- 现有 team_instances 表增加字段
ALTER TABLE team_instances ADD COLUMN squad_id VARCHAR(36);
-- squad_id 对应 Claw 上创建的 Squad

-- 新增: Agent 安装记录
CREATE TABLE team_agent_installs (
  id          VARCHAR(36) PRIMARY KEY,
  instance_id VARCHAR(36) NOT NULL,     -- DevClaw 实例 ID
  role_code   VARCHAR(50) NOT NULL,     -- architect / drone / tester
  claw_agent_id VARCHAR(36),            -- Claw 上注册的 Agent ID
  skills      TEXT,                      -- JSON: 已安装的 skill IDs
  status      VARCHAR(20) DEFAULT 'installing',
  created_at  TIMESTAMP
);
```

### Claw 侧

```sql
-- 新增: 注册的 Agent
CREATE TABLE registered_agents (
  id            VARCHAR(36) PRIMARY KEY,
  name          VARCHAR(100) NOT NULL,
  role_code     VARCHAR(50),
  system_prompt TEXT,
  model         VARCHAR(100),
  config        TEXT,              -- JSON: temperature, max_tokens, etc.
  status        VARCHAR(20) DEFAULT 'active',
  created_at    TIMESTAMP
);

-- 新增: Agent 已安装的技能
CREATE TABLE agent_skills (
  id          VARCHAR(36) PRIMARY KEY,
  agent_id    VARCHAR(36) NOT NULL,
  skill_id    VARCHAR(36) NOT NULL,     -- Queen marketplace item ID
  skill_name  VARCHAR(100),
  skill_spec  TEXT,                      -- JSON: function spec
  version     VARCHAR(20),
  installed_at TIMESTAMP
);
```

### Queen 侧

```sql
-- 现有 marketplace_items 表已够用
-- type = 'skill' 的记录即为可安装技能
-- config 字段存 JSON function spec
```

## 八、API 设计

### Claw 新增 Internal API

```
POST   /v1/internal/agents                     — 注册 Agent
GET    /v1/internal/agents                     — 列出 Agent
DELETE /v1/internal/agents/:id                 — 删除 Agent
POST   /v1/internal/agents/:id/skills          — 为 Agent 安装技能
DELETE /v1/internal/agents/:id/skills/:skillId — 卸载技能
POST   /v1/internal/agents/:id/execute         — 用 Agent 执行推理
GET    /v1/internal/agents/:id/status          — 获取 Agent 状态
```

### Overlord 新增

```
POST   /brood/team-agent/instances/:id/provision  — 初始化实例 (安装 agents + skills)
GET    /brood/team-agent/instances/:id/agents      — 查看实例内的 Agent 列表
POST   /brood/team-agent/instances/:id/execute     — 触发 Mission 执行
```

### Queen 新增

```
POST   /marketplace/items/:id/install-spec   — 获取安装规格 (供 Claw 注册)
GET    /marketplace/skills/search            — 按功能搜索技能
```

## 九、一句话总结

**DevClaw = Overlord 编排 + Claw 运行 + Queen 赋能**

- **Overlord** 是项目经理：决定谁做什么、什么顺序、质量标准
- **Claw** 是工位：每个 Agent 坐在自己的工位上干活
- **Queen** 是工具仓库：Agent 需要什么技能就去领取
- **Mission** 是工单：从需求到交付的全流程自动化
