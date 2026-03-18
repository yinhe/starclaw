# Claw 战队（Squad）— 多节点协作系统设计

> 版本: v1.0 | 日期: 2026-03-16 | 状态: 已确认，开发中

## 一、概述

多个 Claw 节点通过虫洞链路（Nydus）组成**战队（Squad）**，协作完成复杂任务。

类比：
- **英雄联盟** — 多个英雄组队 PK
- **研发团队** — 产品/设计/开发/测试 各司其职
- **短剧团队** — 编剧/导演/演员/后期 协同制作

每个 Claw 是一个「英雄」，拥有自己擅长的 Agent（技能）。战队队长（Captain）负责接单和编排任务。

## 二、架构

```
┌─────────────────────────────────────────────────────────┐
│                    协作编排层 (新增)                       │
│  Squad → Mission → MissionStep → 跨节点委派              │
├─────────────────────────────────────────────────────────┤
│                    能力发现层 (已有+扩展)                   │
│  Hivemind NodeCapability + AgentCapability (新增)         │
├─────────────────────────────────────────────────────────┤
│                    连接层 (已有)                           │
│  Nydus (STUN/Punch/Relay) + Peer + Swarm                │
├─────────────────────────────────────────────────────────┤
│                    身份层 (已有)                           │
│  Ed25519 Identity + claw:xxx 地址 + 签名验证              │
└─────────────────────────────────────────────────────────┘
```

## 三、现有基础设施复用

| 模块 | 文件 | 复用方式 |
|------|------|---------|
| Nydus NAT 穿透 | `node/nydus.go` | 通信层直接用 |
| Relay 转发 | `node/nydus_relay.go` | `Forward()` 作为 RPC 通道 |
| Peer 节点管理 | `model/peer.go` | 成员节点发现 |
| Hivemind 能力 | `node/hivemind.go` | 扩展 AgentCapability |
| Multi-Agent 编排 | `agent/multi.go` | Orchestrated 模式复用 |
| TaskWorker | `worker/task_worker.go` | 本地任务执行 |
| Ed25519 签名 | `node/identity.go` | 跨节点请求验证 |

## 四、数据模型

### 4.1 Squad（战队）

```go
type Squad struct {
    ID          string     `gorm:"type:varchar(36);primaryKey"`
    Name        string     `gorm:"type:varchar(200);not null"`
    Description string     `gorm:"type:text"`
    CaptainNode string     `gorm:"type:varchar(50);index;not null"` // claw:xxx
    UserID      string     `gorm:"type:varchar(36);index;not null"`
    Status      string     `gorm:"type:varchar(20);default:forming"` // forming/active/disbanded
    MaxMembers  int        `gorm:"default:10"`
    IsPublic    bool       `gorm:"default:false"`
    Tags        string     `gorm:"type:json"` // ["dev","design","video"]
    CreatedAt   time.Time
    UpdatedAt   time.Time
}
```

### 4.2 SquadMember（战队成员）

```go
type SquadMember struct {
    ID          string    `gorm:"type:varchar(36);primaryKey"`
    SquadID     string    `gorm:"type:varchar(36);index;not null"`
    NodeID      string    `gorm:"type:varchar(50);index;not null"` // claw:xxx
    PeerID      string    `gorm:"type:varchar(36)"` // 本地 Peer 表 ID
    Role        string    `gorm:"type:varchar(20);default:member"` // captain/member
    Specialty   string    `gorm:"type:varchar(50)"` // coding/design/video/sales
    AgentExport string    `gorm:"type:json"` // 该节点贡献的 Agent 摘要
    Status      string    `gorm:"type:varchar(20);default:offline"` // online/offline/busy
    JoinedAt    time.Time
}
```

### 4.3 Mission（战队任务）

```go
type Mission struct {
    ID          string     `gorm:"type:varchar(36);primaryKey"`
    SquadID     string     `gorm:"type:varchar(36);index;not null"`
    Title       string     `gorm:"type:varchar(500);not null"`
    Goal        string     `gorm:"type:text;not null"` // 总目标描述
    Status      string     `gorm:"type:varchar(20);default:planning"` // planning/executing/reviewing/completed/failed
    CaptainNode string     `gorm:"type:varchar(50)"` // 编排节点
    Plan        string     `gorm:"type:json"` // 编排 Agent 生成的计划
    FinalResult string     `gorm:"type:longtext"`
    TotalSteps  int        `gorm:"default:0"`
    DoneSteps   int        `gorm:"default:0"`
    UserID      string     `gorm:"type:varchar(36);index"`
    CreatedAt   time.Time
    UpdatedAt   time.Time
    CompletedAt *time.Time
}
```

### 4.4 MissionStep（任务步骤）

```go
type MissionStep struct {
    ID           string     `gorm:"type:varchar(36);primaryKey"`
    MissionID    string     `gorm:"type:varchar(36);index;not null"`
    TargetNode   string     `gorm:"type:varchar(50)"` // claw:xxx
    TargetAgent  string     `gorm:"type:varchar(200)"` // Agent 名称
    Task         string     `gorm:"type:text;not null"` // 子任务描述
    Input        string     `gorm:"type:longtext"` // 上游输出作为上下文
    Output       string     `gorm:"type:longtext"` // 执行结果
    Status       string     `gorm:"type:varchar(20);default:pending"` // pending/dispatched/running/done/failed
    ErrorMsg     string     `gorm:"type:text"`
    DependsOn    string     `gorm:"type:json"` // ["step-id-1","step-id-2"]
    Sequence     int        `gorm:"default:0"`
    DispatchedAt *time.Time
    CompletedAt  *time.Time
    CreatedAt    time.Time
}
```

## 五、API 设计

### 5.1 用户端（JWT 认证）

| Method | Path | 描述 |
|--------|------|------|
| POST | `/v1/squads` | 创建战队 |
| GET | `/v1/squads` | 我的战队列表 |
| GET | `/v1/squads/:id` | 战队详情 |
| PUT | `/v1/squads/:id` | 更新战队信息 |
| DELETE | `/v1/squads/:id` | 解散战队 |
| POST | `/v1/squads/:id/invite` | 邀请 Peer 加入 |
| POST | `/v1/squads/:id/join` | 接受邀请加入 |
| DELETE | `/v1/squads/:id/members/:nodeId` | 移除成员 |
| GET | `/v1/squads/:id/members` | 成员列表 + 能力 |
| POST | `/v1/squads/:id/missions` | 创建 Mission |
| GET | `/v1/squads/:id/missions` | Mission 列表 |
| GET | `/v1/missions/:id` | Mission 详情 + Steps |
| POST | `/v1/missions/:id/start` | 开始执行 Mission |
| POST | `/v1/missions/:id/cancel` | 取消 Mission |

### 5.2 跨节点端（Ed25519 签名验证）

| Method | Path | 描述 |
|--------|------|------|
| POST | `/v1/peer/squad/invite` | 接收战队邀请 |
| POST | `/v1/peer/squad/agents` | 查询本节点可用 Agent |
| POST | `/v1/peer/squad/execute` | 接收并执行委派任务 |
| POST | `/v1/peer/squad/callback` | 接收执行结果回调 |
| POST | `/v1/peer/squad/heartbeat` | 战队内心跳 |

## 六、核心执行流程

```
用户在 Captain 节点创建 Mission "开发一个天气App"
    │
    ▼
① Captain 的编排 Agent 分析目标 + 战队成员能力
    │
    ▼
② 生成 Plan (MissionStep[]):
    Step 1: [Claw-B 设计Agent] → "设计 UI 原型"
    Step 2: [Claw-C 编码Agent] → "实现前端代码" (依赖 Step 1)
    Step 3: [Claw-D 测试Agent] → "编写测试用例" (依赖 Step 2)
    Step 4: [Captain 全能助手]  → "整合 + 发布"  (依赖 Step 2,3)
    │
    ▼
③ 按依赖顺序，通过 Nydus 委派:
    POST {claw-b}/v1/peer/squad/execute
    {
      "mission_id": "...", "step_id": "step-1",
      "task": "设计天气App首页UI",
      "context": "产品需求...",
      "callback": "{captain}/v1/peer/squad/callback",
      "signature": "ed25519签名"
    }
    │
    ▼
④ Claw-B 验签 → 找本地设计 Agent → 创建 Task 执行
    │
    ▼
⑤ 完毕 → 回调 Captain:
    POST {callback}
    { "step_id": "step-1", "output": "UI方案...", "status": "done" }
    │
    ▼
⑥ Captain 收到 → 检查依赖 → 触发下一批 Steps
    │
    ▼
⑦ 全部完成 → 编排 Agent 汇总 → Mission completed
```

## 七、编排引擎（squad/engine.go）

```go
type SquadEngine struct {
    db            *gorm.DB
    nydus         *node.NydusManager
    identity      *node.Identity
    taskWorker    *worker.TaskWorker    // 本地任务执行
    providerReg   *provider.Registry
    toolRegistry  *tool.Registry
}

// PlanMission — Captain 编排 Agent 分解任务
func (e *SquadEngine) PlanMission(mission *Mission, members []SquadMember) error

// DispatchStep — 通过 Nydus Relay 委派步骤到目标节点
func (e *SquadEngine) DispatchStep(step *MissionStep) error

// HandleCallback — 处理远程节点回调的执行结果
func (e *SquadEngine) HandleCallback(stepID, output, status string) error

// CheckAndAdvance — 检查依赖完成情况，触发下一批步骤
func (e *SquadEngine) CheckAndAdvance(missionID string) error

// ExecuteLocalStep — 接收远程委派，在本地执行
func (e *SquadEngine) ExecuteLocalStep(req *ExecuteRequest) error
```

## 八、能力广播扩展

```go
// AgentCapability — 新增，在 Hivemind 心跳中广播
type AgentCapability struct {
    AgentID     string   `json:"agent_id"`
    Name        string   `json:"name"`
    Specialty   string   `json:"specialty"`
    Skills      []string `json:"skills"`
    Description string   `json:"description"` // 50字以内
    Available   bool     `json:"available"`
}
```

战队心跳时上报 `AgentCapability[]`，Captain 据此智能分配。

## 九、安全机制

| 环节 | 措施 | 状态 |
|------|------|------|
| 节点身份 | Ed25519 + claw: 地址 | ✅ 已有 |
| 请求签名 | 所有跨节点请求签名验证 | ✅ 已有 |
| 战队准入 | 邀请制 + 被邀方确认 | 🆕 新增 |
| 任务权限 | 仅执行同战队 Captain 的任务 | 🆕 新增 |
| 结果验证 | 回调结果签名验证 | 🆕 新增 |

## 十、前端 UI

新增 `SquadPage.tsx`，放在侧栏「虫群」分组：

| Tab | 内容 |
|-----|------|
| 我的战队 | 战队卡片、创建/解散/邀请 |
| 任务看板 | Mission Kanban（planning/executing/done） |
| 成员 | 在线状态、Agent 能力矩阵 |
| 执行详情 | MissionStep DAG 图、实时进度 |

## 十一、开发计划

| Phase | 内容 | 文件范围 | 预估 |
|-------|------|---------|------|
| P1 | 数据模型 + CRUD API + DB 迁移 | `model/squad.go` `api/v1/squad.go` `router.go` | 1天 |
| P2 | 跨节点协议 5 端点 + 签名验证 | `api/v1/p2p.go` 扩展 | 1-2天 |
| P3 | 编排引擎 + 任务委派 + 回调 | `squad/engine.go` (新) | 1-2天 |
| P4 | AgentCapability 能力广播 | `node/hivemind.go` 扩展 | 0.5天 |
| P5 | 前端 SquadPage + 看板 | `web/src/pages/SquadPage.tsx` | 1-2天 |
| P6 | 集成测试 | 双 Claw 联调 | 0.5天 |

**总计: 5-8 天**
