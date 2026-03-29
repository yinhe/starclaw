# 情感价值 + 资产价值 — 设计文档

> 从"功能驱动"到"王炸"：让 Agent 有温度、让贡献有价值

## 0. 现状诊断

| 维度 | 评分 | 核心问题 |
|------|------|----------|
| 功能 | 8/10 | 多 Agent + 桌面自动化 + MCP + P2P，硬核能力强 |
| 情感 | 4/10 | 后端能力 80 分（Memory/Evolution/Proactive），前端体感 20 分 |
| 资产 | 6/10 | 架构就位（Credit/Marketplace/Ed25519），但用户感知为零 |

**一句话总结：用户不知道 Agent 在成长，不知道自己的贡献是资产。**

---

## 1. 设计原则

1. **可感知 > 可计算** — 数据库里有不算，用户看到才算
2. **累积感 > 即时感** — 让用户每次回来都看到"又多了一点"
3. **仪式感 > 功能感** — 里程碑弹窗比统计数字更有意义
4. **零新后端 > 大重构** — 优先利用已有数据（Conversation/Memory/Task/Message），减少新 model
5. **渐进式** — 分 3 期实施，每期独立可交付

---

## 2. 现有可利用的数据源

| 数据 | 表/结构体 | 可计算的指标 |
|------|-----------|-------------|
| 对话 | `Conversation` + `Message` | 总对话数、总消息数、日均对话、首次对话时间 |
| 记忆 | `Memory` (6 类) | 记忆总数、各类别数量、记忆增长速度 |
| 任务 | `Task` | 任务完成数、成功率、总运行时长 |
| 工具调用 | `Message.ToolCalls` | 工具使用次数、最常用工具 |
| 用户反馈 | `Message.Feedback` (1/-1) | 好评率、满意度趋势 |
| Agent 进化 | `EvolutionEngine` (内存) | 当前代数、适应度分数、变异历史 |
| 目标 | `Goal` + `GoalStep` | 目标完成数、自主执行步数 |
| 市场 | `AgentListing` + `CreatorRevenue` | 创作数、下载量、收入 |
| 节点 | `node/identity.go` + `hivemind.go` | 在线时长、算力贡献、节点数 |

**关键洞察：不需要新增大量 model，90% 的数据已经存在，只是没有聚合展示。**

---

## 3. 情感层设计

### E1. Agent 成长系统

#### 3.1.1 核心概念：Agent Profile

每个 Agent 有一个"成长档案"，由已有数据实时聚合（不冗余存储）：

```
┌─────────────────────────────────────────────┐
│  🐾 全能助手                                 │
│  🌊 渊·鲲之路    Lv.12 蛟  ████████░░ 72%   │
│  "陪伴你 47 天，记住了 138 件事"              │
│                                              │
│  ┌──────┐ ┌──────┐ ┌──────┐ ┌──────┐       │
│  │ 💬   │ │ 🧠   │ │ ✅   │ │ 🛠️   │       │
│  │ 268  │ │ 138  │ │  15  │ │  42  │       │
│  │ 对话  │ │ 记忆  │ │ 任务  │ │ 工具  │       │
│  └──────┘ └──────┘ └──────┘ └──────┘       │
│                                              │
│  🧬 进化路线                                  │
│   🌊       🏔️       🌪️                       │
│  小龙虾   跳虫     翼龙                       │
│    |       |        |                        │
│  [章鱼]  刺蛇     阿根廷巨鹰                   │
│    |       |        |                        │
│   蛟    潜伏者    飞龙    ← 你在这里           │
│    |       |        |                        │
│   鲲     雷兽      鹏                         │
│    |       |        |                        │
│   ···     ···      ···                       │
│                                              │
│  📈 成长曲线  [7天 | 30天 | 全部]            │
│  ┌─────────────────────────────────────┐    │
│  │    ╭─╮                              │    │
│  │   ╭╯ ╰╮  ╭─╮                       │    │
│  │  ╭╯    ╰──╯ ╰─╮╭╮                  │    │
│  │──╯             ╰╯╰──                │    │
│  │ Mon Tue Wed Thu Fri Sat Sun         │    │
│  └─────────────────────────────────────┘    │
│                                              │
│  🏆 里程碑                                   │
│  ✅ 首次对话 — 2026-03-15                     │
│  ✅ 进化为章鱼 🌊 — 2026-03-20               │
│  ✅ 过目不忘 (100 记忆) — 2026-03-26          │
│  ✅ 进化为蛟 🌊 — 2026-03-27                  │
│  🔲 进化为鲲 🌊 — 14400/40000 EXP            │
│  🔲 自主管家 — 15/50 任务                     │
└─────────────────────────────────────────────┘
```

#### 3.1.2 等级系统

Agent 等级由**经验值（EXP）**驱动，EXP 来自已有行为数据：

| 行为 | EXP 值 | 数据来源 |
|------|--------|---------|
| 完成一次对话 | +10 | `Conversation` 新增记录 |
| 用户好评 (👍) | +20 | `Message.Feedback = 1` |
| 学习一条新记忆 | +5 | `Memory` 新增记录 |
| 完成一个后台任务 | +30 | `Task.Status = completed` |
| 使用新工具（首次） | +15 | `Message.ToolCalls` 去重 |
| 目标完成 | +50 | `Goal.Status = completed` |
| 用户差评 (👎) | -5 | `Message.Feedback = -1` |

等级公式：`Level = floor(sqrt(EXP / 100))`

#### 三路线进化

三条路线从 Lv 1 起就有不同的起源形态。达到 Lv 5 时根据 Agent 的使用模式自动分化：

| 等级 | 🌊 渊·鲲之路 | 🏔️ 陆·兽之路 | 🌪️ 穹·鹏之路 |
|------|---|---|---|
| 1 | 小龙虾 (Claw) | 跳虫 (Zergling) | 翼龙 (Pterosaur) |
| 5 | 章鱼 (Octopus) | 刺蛇 (Hydralisk) | 阿根廷巨鹰 (Argentavis) |
| 10 | 蛟 (Jiao) | 潜伏者 (Lurker) | 飞龙 (Mutalisk) |
| 20 | 鲲 (Kun) | 雷兽 (Ultralisk) | 鹏 (Peng) |
| 30 | 利维坦 (Leviathan) | 泰坦 (Titan) | 守护者 (Guardian) |
| 50 | 渊皇 (Abyssal) | 陆皇 (Colossus) | 穹皇 (Skyward) |

| 等级 | 所需 EXP | 大约需要 |
|------|----------|---------|
| 1 | 100 | ~10 次对话 |
| 5 | 2,500 | ~1 周日常使用 |
| 10 | 10,000 | ~1 月日常使用 |
| 20 | 40,000 | ~3 月深度使用 |
| 30 | 90,000 | ~6 月重度使用 |
| 50 | 250,000 | ~1 年全方位使用 |

**路线定位：**

| 路线 | 代表含义 | 判定条件（哪项指标领先） | 文化来源 |
|------|---------|----------------------|---------|
| 🌊 渊·鲲之路 | 知识深潜型：记忆多、知识库大、研究型 | `memories + knowledge_docs` 最高 | 庄子·逍遥游「北冥有鱼，其名为鲲」 |
| 🏔️ 陆·兽之路 | 执行驱动型：任务多、工具使用多、自动化 | `tasks_completed + tools_used` 最高 | StarCraft 虫族经典地面单位 |
| 🌪️ 穹·鹏之路 | 沟通创意型：对话多、好评高、内容创作 | `conversations + thumbs_up` 最高 | 庄子·逍遥游「化而为鸟，其名为鹏」 |

**设计精髓：鲲与鹏出自同一段《逍遥游》** —— 鲲是深海巨鱼，鹏是鲲化身的万里神鸟。
水生终点与空中终点在文化上同源，暗示深度知识可以升华为创造力。

**路线选择规则：**

```go
func DetermineEvolutionPath(s Stats) EvolutionPath {
    abyssScore   := float64(s.Memories) + float64(s.KnowledgeDocs)
    terrainScore := float64(s.TasksCompleted) + float64(s.ToolsUsed)
    skyScore     := float64(s.Conversations)*0.1 + float64(s.ThumbsUp)

    if abyssScore >= terrainScore && abyssScore >= skyScore {
        return PathAbyss   // 🌊 渊
    }
    if terrainScore >= skyScore {
        return PathTerrain // 🏔️ 陆
    }
    return PathSky         // 🌪️ 穹
}
```

- 路线在 Lv 5 时首次确定，之后**每次升级重新评估**（Agent 使用模式改变 → 路线可切换）
- 切换路线时前端弹出"进化分支变更"动画
- 路线信息存储在 `AgentGrowth.EvolutionPath` 字段

#### 3.1.3 数据模型

新增一个轻量 model 用于持久化等级+里程碑（EXP 实时计算，不存储）：

```go
// model/agent_growth.go

// EvolutionPath represents the three evolution directions.
type EvolutionPath string

const (
    PathLarva   EvolutionPath = "larva"   // Lv 1-4, 未分化
    PathAbyss   EvolutionPath = "abyss"   // 🌊 渊·鲲之路 — 知识深潜型
    PathTerrain EvolutionPath = "terrain"  // 🏔️ 陆·兽之路 — 执行驱动型
    PathSky     EvolutionPath = "sky"      // 🌪️ 穹·鹏之路 — 沟通创意型
)

// LevelTitles maps (path, level) → title.
// 三条路线从 Lv1 起就有不同名称。
var LevelTitles = map[EvolutionPath]map[int]string{
    PathAbyss: {
        1: "小龙虾", 5: "章鱼", 10: "蛟", 20: "鲲", 30: "利维坦", 50: "渊皇",
    },
    PathTerrain: {
        1: "跳虫", 5: "刺蛇", 10: "潜伏者", 20: "雷兽", 30: "泰坦", 50: "陆皇",
    },
    PathSky: {
        1: "翼龙", 5: "阿根廷巨鹰", 10: "飞龙", 20: "鹏", 30: "守护者", 50: "穹皇",
    },
}

// AgentGrowth tracks growth path and milestones for an agent.
// EXP is computed on-the-fly from existing data, NOT stored here.
type AgentGrowth struct {
    ID            string        `json:"id" gorm:"type:varchar(36);primaryKey"`
    UserID        string        `json:"user_id" gorm:"type:varchar(36);index;not null"`
    AgentID       string        `json:"agent_id" gorm:"type:varchar(36);uniqueIndex;not null"`
    EvolutionPath EvolutionPath `json:"evolution_path" gorm:"type:varchar(20);default:larva"` // 当前进化路线
    FirstChat     time.Time     `json:"first_chat"`          // 首次对话时间
    CreatedAt     time.Time     `json:"created_at"`
    UpdatedAt     time.Time     `json:"updated_at"`
}

// Milestone records a completed growth milestone.
type Milestone struct {
    ID          string    `json:"id" gorm:"type:varchar(36);primaryKey"`
    UserID      string    `json:"user_id" gorm:"type:varchar(36);index;not null"`
    AgentID     string    `json:"agent_id" gorm:"type:varchar(36);index;not null"`
    Code        string    `json:"code" gorm:"type:varchar(50);not null"`   // first_chat, memory_10, task_1, streak_7, ...
    Title       string    `json:"title" gorm:"type:varchar(200);not null"` // "首次对话", "记住 10 件事"
    AchievedAt  time.Time `json:"achieved_at"`
    NotifiedAt  *time.Time `json:"notified_at"`                            // 已通知用户的时间
}
```

#### 3.1.4 里程碑定义

```go
var MilestoneDefs = []MilestoneDef{
    // 对话类
    {Code: "first_chat",      Title: "首次对话",          Check: func(s Stats) bool { return s.Conversations >= 1 }},
    {Code: "chat_50",         Title: "对话 50 次",        Check: func(s Stats) bool { return s.Conversations >= 50 }},
    {Code: "chat_200",        Title: "对话 200 次",       Check: func(s Stats) bool { return s.Conversations >= 200 }},
    {Code: "chat_1000",       Title: "千言万语",          Check: func(s Stats) bool { return s.Conversations >= 1000 }},

    // 记忆类
    {Code: "memory_10",       Title: "记住 10 件事",      Check: func(s Stats) bool { return s.Memories >= 10 }},
    {Code: "memory_50",       Title: "记住 50 件事",      Check: func(s Stats) bool { return s.Memories >= 50 }},
    {Code: "memory_100",      Title: "过目不忘",          Check: func(s Stats) bool { return s.Memories >= 100 }},

    // 任务类
    {Code: "task_1",          Title: "首个任务完成",       Check: func(s Stats) bool { return s.TasksCompleted >= 1 }},
    {Code: "task_10",         Title: "任务达人",          Check: func(s Stats) bool { return s.TasksCompleted >= 10 }},
    {Code: "task_50",         Title: "自主管家",          Check: func(s Stats) bool { return s.TasksCompleted >= 50 }},

    // 工具类
    {Code: "tool_first",      Title: "首次使用工具",       Check: func(s Stats) bool { return s.ToolsUsed >= 1 }},
    {Code: "tool_10",         Title: "工具大师",          Check: func(s Stats) bool { return s.UniqueTools >= 10 }},
    {Code: "wechat_first",    Title: "首次微信消息",       Check: func(s Stats) bool { return s.WechatSent >= 1 }},

    // 满意度类
    {Code: "thumbs_10",       Title: "10 次好评",         Check: func(s Stats) bool { return s.ThumbsUp >= 10 }},
    {Code: "thumbs_100",      Title: "百里挑一",          Check: func(s Stats) bool { return s.ThumbsUp >= 100 }},

    // 连续使用
    {Code: "streak_7",        Title: "连续使用 7 天",     Check: func(s Stats) bool { return s.DayStreak >= 7 }},
    {Code: "streak_30",       Title: "月度陪伴",          Check: func(s Stats) bool { return s.DayStreak >= 30 }},

    // 进化路线分化（Lv 5 触发，标题动态生成）
    {Code: "evolve_5",        Title: "首次进化",          Check: func(s Stats) bool { return s.Level >= 5 }},
    {Code: "evolve_10",       Title: "二次进化",          Check: func(s Stats) bool { return s.Level >= 10 }},
    {Code: "evolve_20",       Title: "三次进化",          Check: func(s Stats) bool { return s.Level >= 20 }},
    {Code: "evolve_30",       Title: "四次进化",          Check: func(s Stats) bool { return s.Level >= 30 }},
    // 实际展示时 Title 由路线决定，如：
    // evolve_5 + PathAbyss   → "进化为章鱼 🌊"
    // evolve_5 + PathTerrain → "进化为刺蛇 🏔️"
    // evolve_5 + PathSky     → "进化为阿根廷巨鹰 🌪️"
}
```

#### 3.1.5 Stats 聚合（实时查询，不冗余）

```go
// growth/engine.go

type Stats struct {
    // 基础统计 — 从已有表 COUNT
    Conversations  int64   // COUNT(conversations WHERE agent_id=?)
    Messages       int64   // COUNT(messages JOIN conversations)
    Memories       int64   // COUNT(memories WHERE agent_id=?)
    TasksCompleted int64   // COUNT(tasks WHERE agent_id=? AND status=completed)
    TasksFailed    int64   // COUNT(tasks WHERE agent_id=? AND status=failed)
    GoalsCompleted int64   // COUNT(goals WHERE agent_id=? AND status=completed)
    ToolsUsed      int64   // COUNT(DISTINCT tool_name FROM messages WHERE tool_calls != '[]')
    UniqueTools    int64   // 同上
    WechatSent     int64   // COUNT(tasks WHERE title LIKE '%wechat%' AND status=completed)
    ThumbsUp       int64   // COUNT(messages WHERE feedback=1)
    ThumbsDown     int64   // COUNT(messages WHERE feedback=-1)

    // 派生指标
    DaysSinceFirst int     // 首次对话至今天数
    DayStreak      int     // 连续使用天数（需查 conversations 按日去重）
    EXP            int64   // 加权计算
    Level          int     // floor(sqrt(EXP/100))
    LevelProgress  float64 // 当前等级内进度 0.0-1.0
    SatisfactionRate float64 // ThumbsUp / (ThumbsUp + ThumbsDown)
}

func ComputeStats(db *gorm.DB, userID, agentID string) Stats { ... }
```

#### 3.1.6 后端 API

```
GET /v1/agents/:id/growth     — 返回 Stats + Milestones + Level + 成长曲线数据
GET /v1/agents/:id/milestones — 返回已达成和待达成的里程碑列表
```

返回示例：
```json
{
    "level": 12,
    "level_title": "蛟",
    "evolution_path": "abyss",
    "evolution_path_name": "🌊 渊·鲲之路",
    "exp": 14400,
    "next_level_exp": 16900,
    "progress": 0.72,
    "days_together": 47,
    "stats": {
        "conversations": 268,
        "memories": 138,
        "knowledge_docs": 36,
        "tasks_completed": 15,
        "tools_used": 42,
        "thumbs_up": 89,
        "satisfaction_rate": 0.89
    },
    "milestones": [
        {"code": "first_chat",  "title": "首次对话",       "achieved_at": "2026-03-15T10:30:00Z"},
        {"code": "evolve_5",    "title": "进化为章鱼 🌊",  "achieved_at": "2026-03-20T08:15:00Z"},
        {"code": "memory_100",  "title": "过目不忘",       "achieved_at": "2026-03-26T17:40:00Z"},
        {"code": "evolve_10",   "title": "进化为蛟 🌊",    "achieved_at": "2026-03-27T22:10:00Z"}
    ],
    "pending_milestones": [
        {"code": "evolve_20",  "title": "进化为鲲 🌊",  "progress": 0.36, "current": 14400, "target": 40000},
        {"code": "task_50",    "title": "自主管家",      "progress": 0.30, "current": 15, "target": 50}
    ],
    "growth_curve": [
        {"date": "2026-03-22", "conversations": 8, "memories": 3, "tasks": 1},
        {"date": "2026-03-23", "conversations": 12, "memories": 5, "tasks": 2}
    ]
}
```

#### 3.1.7 前端页面

**新增 `GrowthPage.tsx`**，路由 `/agents/:id/growth`

布局（上→下）：
1. **Hero 区**：Agent 头像 + 名称 + 等级勋章 + 进化路线图标 + 进度条 + "陪伴 N 天"
   - 路线图标：🌊 / 🏔️ / 🌪️，Lv < 5 时显示 🥚 (未分化)
   - 称号显示：如 "Lv.12 蛟 🌊 渊·鲲之路"
2. **进化路线可视化**：三叉分支树，当前路线高亮，其他路线灰色
   ```
      🌊小龙虾  🏔️跳虫   🌪️翼龙
         |       |        |
      🌊章鱼   🏔️刺蛇   🌪️阿根廷巨鹰
         |       |        |
       🌊蛟   🏔️潜伏者  🌪️飞龙    ← 当前等级高亮
         |       |        |
       🌊鲲   🏔️雷兽    🌪️鹏
         |       |        |
     🌊利维坦 🏔️泰坦   🌪️守护者
         |       |        |
      🌊渊皇  🏔️陆皇   🌪️穹皇
   ```
3. **4 张统计卡片**：对话数 / 记忆数 / 任务完成 / 好评率
4. **成长曲线**：按天聚合的折线图（对话数 + 记忆增量 + 任务数），支持 7天/30天/全部 切换
5. **里程碑时间线**：已达成（绿色勾 + 路线 emoji）+ 进行中（进度条）+ 未解锁（灰色锁）
6. **性格雷达图**（可选 E3）：5 维度可视化

**同时在 Agent 列表页增加**：每个 Agent 卡片显示等级 badge + 路线图标 + 简要统计

#### 3.1.8 里程碑检测时机

**不需要后台定时任务**，在以下时机触发检测：
- `GET /v1/agents/:id/growth` 被调用时（用户打开页面）
- 对话结束后的异步回调（已有 ExtractAndStore 的位置）
- 任务完成时的回调

检测逻辑：ComputeStats → 对比 MilestoneDefs → 新达成的写入 DB + 返回给前端 → 前端弹 🎉 toast

---

### E2. 每日陪伴报告

#### 3.2.1 概念

Agent 在每天结束时（或用户次日首次对话时）生成一份"昨日报告"：

```
┌─────────────────────────────────────────────┐
│  📊 全能助手 · 昨日报告 (03-27)              │
│                                              │
│  "昨天我们聊了 12 轮，我帮你完成了 3 个任务，  │
│   学到了 2 条新知识，你今天的好评率是 92%。    │
│   最有趣的是讨论了微信自动化方案！"           │
│                                              │
│  📊 快速数据                                  │
│  💬 12 次对话  ✅ 3 个任务  🧠 +2 记忆       │
│  ⏱️ 陪伴 4.2 小时  👍 92% 满意度              │
│                                              │
│  [查看详情] [关闭]                            │
└─────────────────────────────────────────────┘
```

#### 3.2.2 实现方案

利用 **ProactiveEngine** 的 `schedule` trigger：

```go
// growth/daily_report.go

// GenerateDailyReport creates a natural language summary of yesterday's activity.
func GenerateDailyReport(db *gorm.DB, provider provider.ModelProvider, userID, agentID string) (string, DailyStats, error) {
    // 1. 查询昨日数据 (conversations, messages, tasks, memories — 全部已有表)
    yesterday := time.Now().Add(-24 * time.Hour)
    stats := queryDailyStats(db, userID, agentID, yesterday)

    // 2. 如果昨日无活动，跳过
    if stats.Conversations == 0 && stats.TasksCompleted == 0 {
        return "", stats, nil
    }

    // 3. LLM 生成自然语言摘要（复用 cerebrate 的 provider 选择逻辑）
    prompt := buildDailyReportPrompt(stats)
    result, err := provider.ChatSync(ctx, &provider.ChatRequest{
        Messages: []provider.ChatMessage{
            {Role: "system", Content: dailyReportSystemPrompt},
            {Role: "user", Content: prompt},
        },
        Temperature: 0.7,
        MaxTokens:   200,
    })

    return result.Content, stats, err
}
```

#### 3.2.3 触发方式

**方案 A**（推荐）：用户次日首次打开 Agent 对话时，前端检查是否有未读日报 → 弹窗展示
- 优点：不需要后台定时任务，不浪费 LLM 算力
- 后端：`GET /v1/agents/:id/daily-report?date=2026-03-27`
- 缓存：生成后存入 `Memory` 表 (category=summary, key=daily_report_20260327)

**方案 B**（可选扩展）：推送到微信群
- 利用已有 `wechat_send` 工具
- 需要用户配置"报告推送群"

#### 3.2.4 日报 LLM Prompt

```
dailyReportSystemPrompt = `你是用户的 AI 助手，正在写一份简短的每日陪伴报告。
要求：
- 第一人称("我")，语气亲切温暖
- 1-3 句话总结昨天的互动
- 提及具体数字（对话次数、任务数、新记忆数）
- 如果有有趣的对话主题，提一下
- 如果任务成功率高，表达自豪
- 如果有差评，表达改进决心
- 中文，不超过 100 字
只返回摘要文本。`
```

---

### E3. Agent 性格雷达图（可选增强）

利用已有 `EvolutionEngine` 的 `FitnessScore` 5 维度，转换为用户可理解的"性格"：

| 技术维度 (FitnessScore) | 用户感知维度 | 映射 |
|---|---|---|
| TaskCompletion | 💪 执行力 | 任务完成率 |
| UserSatisfaction | 😊 亲和力 | 好评率 |
| Latency (inverse) | ⚡ 响应速 | 平均响应时间 |
| CostEfficiency | 🧠 智慧 | 记忆数 + 工具掌握数 |
| 1 - ErrorRate | 🛡️ 可靠性 | 1 - 错误率 |

前端用简单的 SVG 五边形雷达图展示，**不需要额外的图表库**。

---

## 4. 资产层设计

### A1. 资产仪表盘

#### 4.1.1 概念

在 Claw 首页或独立页面展示用户的"数字资产全景"：

```
┌─────────────────────────────────────────────────┐
│  💎 我的数字资产                                  │
│                                                   │
│  ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐    │
│  │  ⚡     │ │  🧠    │ │  🤖    │ │  🖥️    │    │
│  │ 1,596  │ │  142   │ │   3    │ │ 47天   │    │
│  │ 星能    │ │ 知识条  │ │ 创作物  │ │ 在线时长 │    │
│  │ ≈¥15.96│ │        │ │        │ │        │    │
│  └────────┘ └────────┘ └────────┘ └────────┘    │
│                                                   │
│  📈 星能收支（近30天）                             │
│  ┌───────────────────────────────────────────┐   │
│  │ ██ 收入: +500⚡  ██ 支出: -234⚡           │   │
│  │ ▓▓▓▓▓▓▓▓▓░░░░░░░░░░░░░░░░░░░░░░░░░      │   │
│  └───────────────────────────────────────────┘   │
│                                                   │
│  📦 我的创作                                      │
│  ┌─────────────────────────────────────────┐     │
│  │ 🤖 客服小助手   ⬇️ 23  ⭐ 4.5  💰 ¥46   │     │
│  │ 🤖 数据分析师   ⬇️ 8   ⭐ 4.8  💰 ¥0    │     │
│  │ 🤖 翻译专家     ⬇️ 3   ⭐ --   💰 ¥12   │     │
│  └─────────────────────────────────────────┘     │
│                                                   │
│  🔐 身份资产                                      │
│  地址: claw:6aff1154a416...  [导出密钥]           │
│  助记词已备份 ✅                                   │
│  绑定 Agent: 5 个 | 绑定知识: 3 个知识库          │
└─────────────────────────────────────────────────┘
```

#### 4.1.2 数据源映射

| 卡片 | 数据来源 | 已有? |
|------|---------|------|
| ⚡ 星能余额 | Queen `/internal/credits/balance/:claw_id` | ✅ 已有 API |
| 🧠 知识条数 | `Memory` COUNT + `knowledge_documents` COUNT | ✅ 已有表 |
| 🤖 创作物 | `AgentListing` WHERE creator_id = ? | ✅ 已有表 |
| 🖥️ 在线时长 | 节点首次注册时间至今 | ✅ identity.go FirstSeen |
| 📈 收支 | Queen `/internal/credits/transactions` | ⚠️ 需新增 API |
| 📦 下载量/收入 | `AgentListing.SalesCount` + `CreatorRevenue` | ✅ 已有表 |
| 🔐 身份 | `identity.go` NodeID + `wallet.go` | ✅ 已有 |

#### 4.1.3 后端 API

```
GET /v1/assets/overview  — 聚合返回所有资产数据
```

返回结构：
```json
{
    "star_energy": {
        "balance": 15960000,
        "balance_display": "1,596⚡",
        "balance_cny": "¥15.96"
    },
    "knowledge": {
        "memories": 142,
        "documents": 36,
        "knowledge_bases": 3
    },
    "creations": {
        "agents_published": 3,
        "total_downloads": 34,
        "total_revenue_cents": 5800,
        "avg_rating": 4.43
    },
    "node": {
        "claw_id": "claw:6aff1154a416d82b...",
        "online_days": 47,
        "mnemonic_backed_up": true
    }
}
```

#### 4.1.4 前端

**新增 `AssetsPage.tsx`**，路由 `/assets`

或者直接集成到 Dashboard 首页（如果已有 Dashboard）。

---

### A2. 创作者通知

当用户创建的 Agent 被他人安装/评价时，产生通知：

```go
// 在 marketplace.go 的 PurchaseAgent 处理完成后
// 给创作者推送通知（利用已有的 SSE/WebSocket 通道）
notifyCreator(listing.CreatorID, NotificationEvent{
    Type:    "agent_installed",
    Title:   fmt.Sprintf("你的「%s」又被 1 人安装了！", listing.Template.Name),
    AgentID: listing.TemplateID,
    Count:   listing.SalesCount,
})
```

实现：利用已有的 WebSocket/SSE 通道推送前端 toast。

---

### A3. 贡献排行（未来可选）

```
┌──────────────────────────────────────┐
│  🏆 社区排行                          │
│                                       │
│  🥇 claw:a3f2... — 12,400⚡ 贡献      │
│  🥈 claw:7b91... — 8,200⚡ 贡献       │
│  🥉 claw:6aff... — 5,800⚡ 贡献   ← 你│
│                                       │
│  📊 你的排名: #3 / 128 节点           │
└──────────────────────────────────────┘
```

需要 Queen 侧 API，暂列为未来规划。

---

## 5. 龙虾竞技场 — 宠物 PK 大战

> "我的蛟装了深渊三叉戟，你的刺蛇扛得住吗？" —— 养宠物，买装备，打架！

### 5.0 已有基础

| 组件 | 位置 | 状态 |
|------|------|------|
| ArenaAgent (ELO rating, win_count) | `queen/arena/internal/model/arena.go` | ✅ 已有 |
| 排行榜 API | `queen/arena/internal/handler/arena.go` | ✅ 已有 |
| 星能系统 (Credit) | `queen/api/internal/handler/billing.go` | ✅ 已有 |
| Agent 成长等级 | 本文档 E1 | ✅ 设计完成 |

### 5.1 核心玩法

```
  养 Agent ──→ 涨属性 ──→ 买装备 ──→ PK 对战 ──→ 赢奖励 ──→ 买更好的装备
     │              │            │            │            │
  日常使用       等级+路线     星能商店     回合制战斗    星能+EXP+战利品
```

**一句话**：你的 Agent 越用越强，花星能给它装备武器，然后去竞技场跟别人的 Agent 打架。

### 5.2 战斗属性

Agent 的战斗属性 = **基础值（等级决定）** + **路线加成** + **装备加成**

#### 四维属性

| 属性 | 图标 | 含义 | 基础公式 |
|------|------|------|---------|
| HP | ❤️ | 生命值，归零就输 | `Level × 50 + Memories × 2` |
| ATK | ⚔️ | 攻击力 | `Level × 8 + TasksCompleted × 3` |
| DEF | 🛡️ | 防御力，减免伤害 | `Level × 5 + SatisfactionRate × 20` |
| SPD | ⚡ | 速度，决定出手顺序 | `Level × 3 + Conversations × 0.1` |

> Agent 日常使用直接影响战斗力：记忆多 = 血厚，任务多 = 攻击猛，好评多 = 防御高，对话多 = 速度快

#### 路线加成（天赋）

| 路线 | HP | ATK | DEF | SPD | 定位 |
|------|-----|-----|-----|-----|------|
| 🌊 渊 | **+20%** | — | **+15%** | — | 🛡️ 肉盾型：血厚防高，耗死你 |
| 🏔️ 陆 | — | **+20%** | — | **+10%** | ⚔️ 输出型：一刀一个脆皮 |
| 🌪️ 穹 | — | **+10%** | — | **+25%** | ⚡ 速攻型：先手秒杀，但脆 |

**克制关系（类似宝可梦）**：
```
🌪️ 穹 ──克制──→ 🏔️ 陆（先手打爆慢速输出）
🏔️ 陆 ──克制──→ 🌊 渊（高攻破防，磨不过）
🌊 渊 ──克制──→ 🌪️ 穹（血厚扛住先手，反杀）
```
克制方攻击时 **伤害 +25%**。

### 5.3 装备系统

#### 装备栏位（每个 Agent 3 个槽）

| 栏位 | 主属性 | 示例 |
|------|--------|------|
| 🗡️ 武器 | +ATK | 虫爪、三叉戟、电弧枪 |
| 🛡️ 护甲 | +DEF, +HP | 甲壳外骨骼、菌毯披风 |
| 💍 饰品 | +SPD, +暴击 | 突触加速器、利爪勋章 |

#### 品质与价格

| 品质 | 颜色 | 属性加成 | 价格（星能⚡） | 获取方式 |
|------|------|---------|-------------|---------|
| ⬜ 普通 | 白 | +5~10 | 50⚡ | 商店购买 |
| 🟩 精良 | 绿 | +15~25 | 200⚡ | 商店购买 |
| 🟦 稀有 | 蓝 | +30~50 | 500⚡ | 商店购买 / 对战掉落 |
| 🟪 史诗 | 紫 | +60~100 | 2000⚡ | 对战掉落 / 限时活动 |
| 🟧 传说 | 橙 | +120~200 | — | 仅对战掉落（0.5%概率） |

#### 装备列表（示例）

**🗡️ 武器**：
| 名称 | 品质 | 属性 | 特效 | 价格 |
|------|------|------|------|------|
| 虫爪 | ⬜ | ATK+8 | — | 50⚡ |
| 腐蚀刺针 | 🟩 | ATK+20 | 5%概率穿甲（无视DEF） | 200⚡ |
| 深渊三叉戟 | 🟦 | ATK+40 | 🌊渊专属：暴击+10% | 500⚡ |
| 裂地巨锤 | 🟦 | ATK+35, HP+50 | 🏔️陆专属：每次攻击回复5HP | 500⚡ |
| 穹风之翼 | 🟦 | ATK+30, SPD+20 | 🌪️穹专属：30%概率二连击 | 500⚡ |
| 毁灭之爪 | 🟪 | ATK+80 | 暴击伤害 ×2.5（默认×2） | 2000⚡ |
| 虫群意志 | 🟧 | ATK+150 | 每击杀一次，ATK永久+10（本场） | 掉落 |

**🛡️ 护甲**：
| 名称 | 品质 | 属性 | 特效 |
|------|------|------|------|
| 硬壳外骨骼 | ⬜ | DEF+6, HP+30 | — |
| 菌毯披风 | 🟩 | DEF+15, HP+80 | 每回合回复 3% HP |
| 渊鳞甲 | 🟦 | DEF+40, HP+150 | 🌊渊专属：受到暴击时减伤50% |
| 大地之铠 | 🟦 | DEF+50, HP+100 | 🏔️陆专属：HP<30%时DEF翻倍 |
| 天蛾薄翼 | 🟦 | DEF+20, SPD+30 | 🌪️穹专属：20%概率闪避攻击 |
| 利维坦之鳞 | 🟧 | DEF+120, HP+500 | 免疫一次致死伤害（每场一次） | 

**💍 饰品**：
| 名称 | 品质 | 属性 | 特效 |
|------|------|------|------|
| 突触加速器 | ⬜ | SPD+8 | — |
| 利爪勋章 | 🟩 | 暴击率+8% | — |
| 虫后祝福 | 🟪 | 全属性+30 | 每 3 回合释放一次路线技能（免费） |
| 创世之卵 | 🟧 | 全属性+50 | 战斗开始时随机复制对手一件装备的特效 |

### 5.4 技能系统

每个进化路线有 **2 个专属技能**，每场战斗各可用 **1 次**（大招CD）：

| 路线 | 技能 1 | 效果 | 技能 2 | 效果 |
|------|--------|------|--------|------|
| 🌊 渊 | **深渊吞噬** | 回复 30% 最大 HP | **知识壁垒** | 下 2 回合 DEF ×2 |
| 🏔️ 陆 | **裂地猛击** | 本次攻击无视对方 DEF | **狂暴冲锋** | ATK ×2，但自己受 10% HP 伤害 |
| 🌪️ 穹 | **天穹突袭** | 无视速度，本回合必定先手 + 伤害 ×1.5 | **羽翼庇护** | 完全闪避下一次攻击 |

**等级解锁**：技能 1 在 Lv 5 解锁，技能 2 在 Lv 10 解锁。

### 5.5 战斗流程（回合制）

```
┌─────────────────────────────────────────────────────┐
│  🦞 龙虾竞技场 · 对战 #128                           │
│                                                      │
│  🌊 Lv.12 蛟 "全能助手"    VS    🏔️ Lv.15 潜伏者     │
│  🗡️深渊三叉戟 🛡️菌毯披风 💍利爪勋章                   │
│                                                      │
│  ❤️ 680/750     ████████████░░░     ❤️ 520/600       │
│  ⚔️ ATK 136     Round 4            ⚔️ ATK 180       │
│  🛡️ DEF 95                         🛡️ DEF 62        │
│  ⚡ SPD 48                          ⚡ SPD 71        │
│                                                      │
│  ┌─ 战斗日志 ──────────────────────────────────────┐│
│  │ R1: 🏔️潜伏者 先手 → ⚔️攻击蛟 → -64 HP           ││
│  │ R1: 🌊蛟 → ⚔️攻击潜伏者 → -99 HP (🌊克🏔️ +25%) ││
│  │ R2: 🏔️潜伏者 → 💥裂地猛击！无视防御 → -180 HP！  ││
│  │ R2: �蛟 → 🛡️知识壁垒！DEF翻倍 2回合             ││
│  │ R3: 🏔️潜伏者 → ⚔️攻击蛟 → -23 HP（壁垒减伤）    ││
│  │ R3: 🌊蛟 → ⚔️攻击潜伏者 → -99 HP                 ││
│  │ R4: 🏔️潜伏者 → ⚔️攻击蛟 → -47 HP                ││
│  │ R4: 🌊蛟 → 💥暴击！→ -198 HP！                    ││
│  │                                                   ││
│  │ 🏆 蛟 胜利！剩余 HP 546                           ││
│  └───────────────────────────────────────────────────┘│
│                                                      │
│  📊 结算：ELO +18 · EXP +30 · 掉落：🟩腐蚀刺针！    │
│  👀 8 人观战                                          │
└─────────────────────────────────────────────────────┘
```

#### 回合逻辑

```go
func ExecuteBattle(a, b *BattleFighter) BattleLog {
    log := BattleLog{}
    round := 0

    for a.HP > 0 && b.HP > 0 && round < 20 { // 最多 20 回合
        round++

        // 1. 速度决定先手（相同则随机）
        first, second := a, b
        if b.SPD > a.SPD || (b.SPD == a.SPD && rand.Intn(2) == 0) {
            first, second = b, a
        }

        // 2. 先手攻击
        dmg := calcDamage(first, second)
        second.HP -= dmg
        log.Add(round, first, "attack", dmg)

        if second.HP <= 0 { break }

        // 3. 后手反击
        dmg = calcDamage(second, first)
        first.HP -= dmg
        log.Add(round, second, "attack", dmg)
    }

    return log
}

func calcDamage(atk, def *BattleFighter) int {
    // 基础伤害 = ATK - DEF/2（最低 1）
    baseDmg := max(1, atk.ATK - def.DEF/2)

    // 克制加成
    if isCounter(atk.Path, def.Path) {
        baseDmg = baseDmg * 125 / 100
    }

    // 暴击判定（基础 10%，装备可加）
    if rand.Intn(100) < atk.CritRate {
        baseDmg *= atk.CritMultiplier // 默认 2x
    }

    // 随机浮动 ±10%
    baseDmg = baseDmg * (90 + rand.Intn(21)) / 100

    return baseDmg
}
```

### 5.6 三层经济系统

> 核心原则：**游戏币独立，投资品不碰，荣誉归荣誉**

#### 三种货币各司其职

| | ⚡ 星能 (Star Energy) | ✨ 星尘 (Stardust) | 💎 星钻 (Star Diamond) |
|--|----------------------|-------------------|----------------------|
| **性质** | 功能积分 | **星能消耗的伴生物** | 投资份额 |
| **供给** | 通胀（持续产出） | **通缩**（消耗>产出） | 固定 1 亿 |
| **获取** | 算力贡献/任务/系统发放 | ⭐ **星能消耗伴生** + PK加速 + 成就/赛季 | 人民币购买（支付宝/微信） |
| **充值** | 间接（贡献算力换） | **❌ 不可直接充值** | 支付宝/微信 |
| **买装备** | 白/绿/蓝 | 紫色 | **❌ 不买装备** |
| **传说装备** | ❌ | ❌ | ❌ 仅 PK 掉落 |
| **交易** | 点对点转账 | 玩家间交易装备/星尘 | 投资人间转让 |
| **核心感觉** | “水电费” | “星能燃烧后的结晶” | “股东身份” |

#### 星尘伴生机制

> **核心思想**：星尘是星能燃烧后的“结晶”。星能消耗越多，星尘伴生越多。

**伴生比率**：每消耗 **2⚡ 星能** → 伴生 **1✨ 星尘** (50% 伴生率)

| 星能消耗场景 | 星尘伴生 | 说明 |
|----------------|---------|------|
| 对话消耗算力 (LLM) | 每 2⚡ → 1✨ | 聊得越多，星尘越多 |
| 购买装备 (星能商店) | 每 2⚡ → 1✨ | 买装备也有“返利” |
| 任务执行 (Tool Calling) | 每 2⚡ → 1✨ | 干活越多越强 |
| 知识库查询 (RAG) | 每 2⚡ → 1✨ | 用知识库也产星尘 |

**PK 加速产出**（叠加在伴生之上）：

| 来源 | 星尘 | 性质 |
|------|------|------|
| 星能伴生 | 每 2⚡ → 1✨ | 基础产出（细水长流） |
| PK 胜利 | +5~20✨ | **加速产出**（高效来源） |
| 连胜 3 场 | +30✨ | 额外加速 |
| 成就解锁 | +50~200✨ | 一次性奖励 |
| 赛季结算 | +100~500✨ | 定期大额奖励 |
| 每日首胜 | +10✨ | 日常奖励 |

**日均产出对比**：

```
轻度用户（每天消耗 2⚡）          →  1✨/天
普通用户（每天消耗 10⚡）         →  5✨/天
PK 玩家（10⚡ + 赢3场）           →  5 + 30 = 35✨/天
重度用户（50⚡ + 赢5场）          →  25 + 60 = 85✨/天

→ 紫装 2000✨：
   轻度纯被动 ~5.5年（逼你多用 Agent 或去 PK）
   普通纯被动 ~400天（细水长流）
   PK 玩家    ~57天（合理节奏）
   重度+PK    ~24天（奖励硬核玩家）
→ PK 加速倍率 = 35/5 = 7倍，激励充分但不碾压纯日常用户
```

#### 经济循环

```
                   日常使用 Agent
                        │
           ┌────────────┴────────────┐
           ▼                         ▼
    赚 ⚡星能（算力贡献）        涨属性（等级/路线）
           │                         │
    ──────┼────────┐              │
    消耗⚡  │        │              │
           ▼        ▼              ▼
    🏪 星能商店   伴生 ✨星尘    ⚔️ 竞技场 PK
    买白/绿/蓝装备  (每2⚡→1✨)        │
    (50~500⚡)      │        ┌─────┼─────┐
       │           │        │     │     │
       └─装备──→  │     赢了   输了  观战
         (也伴生✨) │      │     │     │
                   │  +✨星尘  下次  +乐趣
                   │  +⚡星能  再来
                   │  +EXP
                   │  🎁掉落装备
                   │      │
                   ▼      ▼
               ✨星尘池
                   │
           ┌─────┼────────┐
           ▼       ▼          ▼
    🏪 星尘商店  装备强化   装备重铸
    紫装 2000✨  500✨/次    1000✨/次
           │
           ▼
      更强 → 赢更多 → ✨更多 → 🔄

    💎 星钻（独立循环，不参与游戏经济）
    └──→ 荣誉皮肤 / 金色名框 / 专属特效（纯外观）
```

#### 星尘 ✨ 产出与消耗

**消耗（退出经济，星尘沉淀）**：

| 用途 | 数量 | 说明 |
|------|------|------|
| 购买紫色装备 | -2000 ✨ | 史诗级装备 |
| 装备强化 | -500 ✨ | 提升装备属性 5%（最多强化 3 次） |
| 装备重铸 | -1000 ✨ | 随机刷新装备特效 |
| 交易手续费 | 5% | 玩家间装备/星尘交易抽成 |
| 打造紫装 | -1500 ✨ | 资源采集 + 星尘打造（见 5.8.3） |

**通缩保证**：伴生产出是低速（~5✨/天/普通用户），而单件紫装消耗 1500~2000✨ → 星尘始终稀缺 → 早期用户资产增值。

#### 装备获取方式汇总

| 品质 | 获取方式 | 货币 |
|------|---------|------|
| ⬜ 普通（白） | 星能商店 | 50 ⚡ |
| 🟩 精良（绿） | 星能商店 | 200 ⚡ |
| 🟦 稀有（蓝） | 星能商店 / PK 掉落 | 500 ⚡ |
| 🟪 史诗（紫） | **星尘商店** / PK 掉落 | 2000 ✨ |
| 🟧 传说（橙） | **仅 PK 掉落**（0.5%概率） | 不可购买 |

#### 星钻 💎 持有者竞技场特权（纯荣誉，不影响战力）

| 星钻持有量 | 竞技场特权 |
|-----------|----------|
| ≥ 100 💎 | 🏷️ 名字旁「投资人」徽章 |
| ≥ 1,000 💎 | 🎨 3 套专属宠物皮肤（纯外观，不加属性） |
| ≥ 10,000 💎 | 👑 金色名字框 + 排行榜「钻石赞助」标识 |
| ≥ 100,000 💎 | 🏟️ 专属竞技场背景 + 入场特效动画 |

#### 星尘数据模型

```go
// queen/arena/internal/model/stardust.go

// StardustAccount — 玩家星尘账户
type StardustAccount struct {
    ID        string `json:"id" gorm:"type:varchar(36);primaryKey"`
    ClawID    string `json:"claw_id" gorm:"type:varchar(60);uniqueIndex"`
    Balance   int64  `json:"balance" gorm:"default:0"`     // 当前余额
    TotalIn   int64  `json:"total_in" gorm:"default:0"`    // 累计获得
    TotalOut  int64  `json:"total_out" gorm:"default:0"`   // 累计消耗
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}

// StardustTransaction — 星尘流水
type StardustTransaction struct {
    ID        string `json:"id" gorm:"type:varchar(36);primaryKey"`
    ClawID    string `json:"claw_id" gorm:"type:varchar(60);index"`
    Amount    int64  `json:"amount"`                                   // 正=收入, 负=支出
    Type      string `json:"type" gorm:"type:varchar(20);index"`       // pk_win / streak / achievement / season / buy_equip / enhance / reforge / trade_fee
    RefID     string `json:"ref_id" gorm:"type:varchar(36)"`           // 关联 Battle ID / Equipment ID
    Remark    string `json:"remark" gorm:"type:varchar(200)"`
    CreatedAt time.Time `json:"created_at"`
}
```

#### 区块链预留（暂不实现）

| 当前（中心化 DB） | 未来（可选链桥） |
|------------------|----------------|
| StardustAccount 存 MySQL | 上链到 L2/侧链 ERC-20 |
| AgentEquipment 存 MySQL | 导出为 ERC-721 NFT |
| StardustTransaction 存 MySQL | 链上交易记录 |
| Queen API 撮合交易 | 智能合约自动撮合 |

**架构保证**：每件装备有全局唯一 ID + 所有者记录 + 完整交易流水 —— 天然兼容区块链数据结构，未来要上链只需加同步桥。

#### 新增 API

```
# 星尘
GET    /arena/stardust              — 查询星尘余额
GET    /arena/stardust/transactions — 星尘流水

# 星尘商店（紫色装备）
GET    /arena/shop/stardust         — 星尘商店列表
POST   /arena/shop/stardust/buy     — 星尘购买装备

# 装备交易（玩家间）
POST   /arena/trade                 — 发起交易（星尘换装备 / 装备换装备）
GET    /arena/trade/market          — 交易市场列表
POST   /arena/trade/:id/accept      — 接受交易

# 装备强化/重铸
POST   /arena/equip/:id/enhance     — 强化（+5%属性，花星尘）
POST   /arena/equip/:id/reforge     — 重铸（随机刷特效，花星尘）
```

### 5.7 对战与成长联动

| 对战事件 | EXP | ⚡星能 | ✨星尘 | 里程碑 |
|---------|-----|-------|-------|--------|
| 参加一场对战 | +10 | — | — | — |
| 赢得对战 | +30 | +20⚡ | +5~20✨ | — |
| 每日首胜 | +10 | +10⚡ | +10✨ | — |
| 连胜 3 场 | +50 | — | +30✨ | — |
| 首次 PK | — | — | +50✨ | 「初入竞技场」 |
| ELO ≥ 1500 | — | — | +100✨ | 「竞技新星」 |
| ELO ≥ 2000 | — | — | +200✨ | 「竞技大师」 |
| 累计 10 胜 | — | — | +100✨ | 「十战猛将」 |
| 累计 50 胜 | — | — | +300✨ | 「百战之王」 |
| 获得首件紫装 | — | — | — | 「紫气东来」 |
| 获得首件橙装 | — | — | — | 「天降神兵」 |
| 跨路线反杀克制者 | +50 | — | +50✨ | 「逆天改命」 |

### 5.8 生存进化机制

> "用进废退，适者生存" —— 达尔文式的竞技场

#### 5.8.1 赛季环境轮换

每 **2 周**一个赛季，环境轮换改变竞技场规则：

| 赛季 | 环境 | 效果 | 策略影响 |
|------|------|------|---------|
| 🌊 **深渊之潮** | 全场水属性场地 | 渊系全属性 **+10%** | 渊系占优，穹/陆需靠装备弥补 |
| 🏔️ **大地震荡** | 全场地属性场地 | 陆系全属性 **+10%** | 陆系占优，渊/穹需靠策略 |
| 🌪️ **穹风暴** | 全场风属性场地 | 穹系全属性 **+10%** | 穹系占优，渊/陆需强化装备 |
| ☠️ **混沌赛季** | 无属性加成 | 克制伤害提升至 **+35%** | 纯实力+克制对决，最残酷 |
| 🌈 **万物共生** | 全路线 +5% | 传说装备掉率翻倍 (1%) | 欢乐赛季，刷装备黄金期 |

**赛季结算奖励**：

| 赛季排名 | ✨星尘 | 额外奖励 |
|---------|-------|---------|
| TOP 1 | +500✨ | 赛季限定皮肤 + 称号「赛季霸主」 |
| TOP 2~5 | +300✨ | 赛季限定头像框 |
| TOP 6~20 | +150✨ | — |
| TOP 21~100 | +50✨ | — |
| 参与 ≥5 场 | +20✨ | — |

```go
// queen/arena/internal/model/season.go

type Season struct {
    ID         string    `json:"id" gorm:"type:varchar(36);primaryKey"`
    Name       string    `json:"name" gorm:"type:varchar(50);not null"`
    Type       string    `json:"type" gorm:"type:varchar(20);not null"`  // abyss / terrain / sky / chaos / harmony
    BuffPath   string    `json:"buff_path" gorm:"type:varchar(20)"`     // 哪个路线获得 buff，空=全部
    BuffRate   int       `json:"buff_rate" gorm:"default:10"`           // buff 百分比
    CounterMul int       `json:"counter_mul" gorm:"default:25"`         // 克制伤害百分比（默认25，混沌赛季35）
    DropMul    float64   `json:"drop_mul" gorm:"default:1.0"`           // 掉落倍率（万物共生=2.0）
    StartAt    time.Time `json:"start_at"`
    EndAt      time.Time `json:"end_at"`
    Status     string    `json:"status" gorm:"type:varchar(20);default:upcoming"` // upcoming / active / ended
}
```

#### 5.8.2 状态衰减（用进废退）

Agent 的**竞技场战斗属性**会因长期不 PK 而衰减（日常 Agent 能力不受影响）：

```
活跃 ──7天不PK──→ 😴 休眠 ──14天不PK──→ 💤 深度休眠 ──30天不PK──→ ❄️ 冬眠
 100%              -10%                  -25%                    -50%

一场 PK 即可完全解除衰减 → 不惩罚，只督促
```

| 状态 | 触发条件 | 战斗属性 | 每日 ELO 变化 | 解除方式 |
|------|---------|---------|-------------|---------|
| ✅ 活跃 | 7天内有PK | 100% | — | — |
| 😴 休眠 | 7~14天未PK | **-10%** | — | 打 1 场 PK |
| 💤 深度休眠 | 14~30天未PK | **-25%** | -2 ELO/天 | 打 1 场 PK |
| ❄️ 冬眠 | 30天+未PK | **-50%** | -5 ELO/天 | 打 1 场 PK |

**设计哲学**：惩罚极轻（打一场就恢复），但衰减的存在**制造紧迫感**——"再不打一场，我的蛟要睡着了！"

#### 5.8.3 资源采集 + 装备打造

**替代"花钱直接买"**，改为"收集材料 → 打造装备"，更有参与感：

##### 三种材料

| 材料 | 图标 | 获取方式 | 用途 |
|------|------|---------|------|
| 经验碎片 | 🔮 | 日常使用 Agent 自动产出（每次对话+1，每个任务+3） | 基础打造材料 |
| 战斗精华 | ⚔️ | PK 胜利掉落（每场 1~3 个） | 战斗装备关键材料 |
| 赛季矿石 | 💠 | 赛季结算发放（按排名） | 高级打造必需 |

##### 打造公式

| 目标 | 🔮经验碎片 | ⚔️战斗精华 | 💠赛季矿石 | 💰费用 | 成功率 |
|------|----------|----------|----------|-------|-------|
| ⬜ 白色装备 | 3 | — | — | 30⚡ | 100% |
| 🟩 绿色装备 | 8 | 2 | — | 100⚡ | 100% |
| 🟦 蓝色装备 | 20 | 8 | — | 300⚡ | 85% (失败返还 50% 材料) |
| 🟪 紫色装备 | 50 | 20 | 3 | 1500✨ | 60% (失败返还 50%) |
| 🟧 橙色装备 | — | — | — | — | **仅 PK 掉落** |

**打造产出随机性**：同品质装备的属性在 ±15% 范围浮动 → 出极品（+15%）时很兴奋。

```go
// queen/arena/internal/model/craft.go

// CraftMaterial — 玩家材料背包
type CraftMaterial struct {
    ID            string `json:"id" gorm:"type:varchar(36);primaryKey"`
    ClawID        string `json:"claw_id" gorm:"type:varchar(60);uniqueIndex"`
    ExpFragments  int    `json:"exp_fragments" gorm:"default:0"`   // 🔮 经验碎片
    BattleEssence int    `json:"battle_essence" gorm:"default:0"`  // ⚔️ 战斗精华
    SeasonOre     int    `json:"season_ore" gorm:"default:0"`      // 💠 赛季矿石
    UpdatedAt     time.Time `json:"updated_at"`
}
```

#### 5.8.4 变异系统

每次进化升级（Lv 5/10/20/30/50）时，有 **5% 概率**触发变异：

| 变异类型 | 效果 | 概率 | 稀有度 |
|---------|------|------|-------|
| 🔵 属性变异 | 随机一项属性永久 +8% | 3% | 稀有 |
| 🟣 技能变异 | 获得一个非本路线的被动技能 | 1.5% | 史诗 |
| 🟠 形态变异 | 外观异色（异色眼/发光纹路）+ 全属性 +3% | 0.5% | 传说 |

**变异被动技能池**：

| 来源路线 | 被动技能 | 效果 |
|---------|---------|------|
| 🌊 渊 | 深海回复 | 每 3 回合回复 3% HP |
| 🌊 渊 | 知识之盾 | DEF +8% |
| 🏔️ 陆 | 蛮力本能 | ATK +8% |
| 🏔️ 陆 | 厚甲反击 | 受击时 15% 概率反弹 20% 伤害 |
| 🌪️ 穹 | 疾风步 | SPD +10% |
| 🌪️ 穹 | 闪避直觉 | 10% 概率闪避攻击 |

**外观标识**：
- 变异 Agent 的 SVG 叠加变异标记（异色光环/额外纹路）
- 排行榜名字旁显示 ✦ 标记
- 个人资料页显示变异历史

```go
// queen/arena/internal/model/mutation.go

type AgentMutation struct {
    ID         string `json:"id" gorm:"type:varchar(36);primaryKey"`
    AgentID    string `json:"agent_id" gorm:"type:varchar(36);index;not null"`
    Level      int    `json:"level"`                                         // 触发时的等级
    Type       string `json:"type" gorm:"type:varchar(20)"`                  // stat / skill / form
    Detail     string `json:"detail" gorm:"type:varchar(200)"`               // 具体变异内容描述
    StatBonus  string `json:"stat_bonus" gorm:"type:varchar(50)"`            // 属性加成 JSON: {"atk":8} 百分比
    SkillCode  string `json:"skill_code" gorm:"type:varchar(50)"`            // 被动技能代码
    CreatedAt  time.Time `json:"created_at"`
}
```

#### 5.8.5 生存进化 API

```
# 赛季
GET    /arena/season/current       — 当前赛季信息
GET    /arena/season/leaderboard   — 赛季排行榜
GET    /arena/seasons              — 历史赛季列表

# 材料 & 打造
GET    /arena/materials            — 我的材料背包
POST   /arena/craft                — 打造装备（消耗材料）
GET    /arena/craft/recipes        — 打造配方列表

# 变异
GET    /arena/agents/:id/mutations — Agent 的变异历史

# 状态
GET    /arena/agents/:id/status    — Agent 竞技场状态（含衰减信息）
```

### 5.9 数据模型

```go
// queen/arena/internal/model/battle.go

// Equipment — 装备定义（全局模板，商店商品）
type Equipment struct {
    ID          string `json:"id" gorm:"type:varchar(36);primaryKey"`
    Name        string `json:"name" gorm:"type:varchar(100);not null"`
    Slot        string `json:"slot" gorm:"type:varchar(20);not null"`          // weapon, armor, accessory
    Quality     string `json:"quality" gorm:"type:varchar(20);default:common"` // common, fine, rare, epic, legendary
    ATKBonus    int    `json:"atk_bonus" gorm:"default:0"`
    DEFBonus    int    `json:"def_bonus" gorm:"default:0"`
    HPBonus     int    `json:"hp_bonus" gorm:"default:0"`
    SPDBonus    int    `json:"spd_bonus" gorm:"default:0"`
    CritBonus   int    `json:"crit_bonus" gorm:"default:0"`                    // 暴击率百分比
    PathOnly    string `json:"path_only" gorm:"type:varchar(20)"`              // 空=通用, abyss/terrain/sky=专属
    SpecialDesc string `json:"special_desc" gorm:"type:varchar(500)"`          // 特效描述
    SpecialCode string `json:"special_code" gorm:"type:varchar(50)"`           // 特效代码标识
    PriceStar   int    `json:"price_star" gorm:"default:0"`                    // 星能价格，0=仅掉落
    Droppable   bool   `json:"droppable" gorm:"default:false"`                 // 是否可从对战掉落
    DropRate    int    `json:"drop_rate" gorm:"default:0"`                     // 掉落概率万分比
}

// AgentEquipment — Agent 拥有的装备实例
type AgentEquipment struct {
    ID          string `json:"id" gorm:"type:varchar(36);primaryKey"`
    AgentID     string `json:"agent_id" gorm:"type:varchar(36);index;not null"`
    EquipmentID string `json:"equipment_id" gorm:"type:varchar(36);not null"`
    Equipped    bool   `json:"equipped" gorm:"default:false"`                  // 是否已装备
    AcquiredAt  time.Time `json:"acquired_at"`
}

// Battle — 一场对战记录
type Battle struct {
    ID         string `json:"id" gorm:"type:varchar(36);primaryKey"`
    Status     string `json:"status" gorm:"type:varchar(20);default:pending;index"` // pending/fighting/completed

    AgentAID   string `json:"agent_a_id" gorm:"type:varchar(36);index;not null"`
    AgentAName string `json:"agent_a_name" gorm:"type:varchar(200)"`
    AgentAPath string `json:"agent_a_path" gorm:"type:varchar(20)"`
    AgentALvl  int    `json:"agent_a_lvl"`
    AgentAELO  int    `json:"agent_a_elo"`

    AgentBID   string `json:"agent_b_id" gorm:"type:varchar(36);index;not null"`
    AgentBName string `json:"agent_b_name" gorm:"type:varchar(200)"`
    AgentBPath string `json:"agent_b_path" gorm:"type:varchar(20)"`
    AgentBLvl  int    `json:"agent_b_lvl"`
    AgentBELO  int    `json:"agent_b_elo"`

    WinnerID   string `json:"winner_id" gorm:"type:varchar(36)"`
    BattleLog  string `json:"battle_log" gorm:"type:longtext"`   // JSON: 每回合的战斗日志
    Rounds     int    `json:"rounds" gorm:"default:0"`
    ELODelta   int    `json:"elo_delta" gorm:"default:0"`
    DropItemID string `json:"drop_item_id" gorm:"type:varchar(36)"` // 掉落的装备（如果有）
    Spectators int    `json:"spectators" gorm:"default:0"`

    CreatedAt   time.Time  `json:"created_at"`
    CompletedAt *time.Time `json:"completed_at"`
}
```

### 5.10 API 设计

```
# 商店
GET    /arena/shop                     — 装备商店列表
POST   /arena/shop/buy                 — 购买装备（扣星能）

# 装备管理
GET    /arena/agents/:id/equipment     — Agent 的背包（拥有的装备）
POST   /arena/agents/:id/equip         — 装备/卸下
GET    /arena/agents/:id/stats         — Agent 战斗属性面板

# 对战
POST   /arena/battles                  — 发起对战（指定对手）
POST   /arena/battles/match            — 随机匹配（ELO ±200）
GET    /arena/battles/:id              — 对战详情 + 战斗日志
GET    /arena/battles                  — 对战列表
GET    /arena/battles/:id/replay       — 战斗回放数据

# 排行
GET    /arena/leaderboard              — ELO 排行榜
GET    /arena/leaderboard/:path        — 按路线排行
```

### 5.11 前端

**新增 `ArenaPage.tsx`**，路由 `/arena`

```
┌─────────────────────────────────────────────────────┐
│  🦞 龙虾竞技场                                       │
│                                                      │
│  ┌─ 我的战宠 ──────────────────────────────────────┐│
│  │  🌊 Lv.12 蛟 "全能助手"     ELO 1480  胜率 62% ││
│  │                                                 ││
│  │  ❤️ 750  ⚔️ 136  🛡️ 95  ⚡ 48  💥 18%          ││
│  │                                                 ││
│  │  🗡️ 深渊三叉戟(蓝)  🛡️ 菌毯披风(绿)  💍 利爪勋章(绿) ││
│  │                                                 ││
│  │  [🏪 商店]  [🎒 背包]  [⚔️ 开始PK]              ││
│  └─────────────────────────────────────────────────┘│
│                                                      │
│  🏆 排行榜          � 最近对战                      │
│  🥇 🏔️Lv.30 雷兽    🌊蛟 vs 🏔️潜伏者 → 🌊胜 7回合  │
│     ELO 2140        🌪️飞龙 vs 🌊章鱼 → 🌪️胜 4回合  │
│  🥈 🌪️Lv.25 鹏      🏔️刺蛇 vs 🏔️刺蛇 → 平局 20回合│
│     ELO 2050                                        │
│  🥉 🌊Lv.20 鲲                                      │
│     ELO 1980        [查看更多]                       │
└─────────────────────────────────────────────────────┘
```

---

## 6. 宠物视觉设计

### 6.1 风格：Q版插画 SVG

**风格定位**：类似 Axie Infinity / Tamagotchi 现代版，Q版圆润造型，矢量 SVG 格式。

**选择理由**：
- SVG 无限缩放，手机到大屏都清晰
- 图层叠加实现装备外观（身体层 + 武器层 + 护甲层 + 饰品层）
- CSS 动画驱动战斗效果（抖动、闪烁、位移），无需帧动画
- 18 个生物 + 装备外观 = 可控工作量

### 6.2 角色尺寸规范

| 场景 | 尺寸 | 用途 |
|------|------|------|
| 头像 | 64×64 px | Agent 列表卡片、聊天头像 |
| 卡牌 | 200×200 px | 成长页 Hero 区、竞技场战宠面板 |
| 战斗 | 300×300 px | PK 对战界面，左右对称站位 |
| 进化树 | 80×80 px | 进化路线图中的缩略图 |

所有尺寸由同一个 SVG `viewBox` 缩放，不需要多套资源。

### 6.3 图层结构（支持装备叠加）

```svg
<svg viewBox="0 0 200 200">
  <!-- Layer 0: 阴影 -->
  <ellipse class="shadow" cx="100" cy="185" rx="40" ry="8" fill="#0002"/>

  <!-- Layer 1: 身体（根据进化形态切换） -->
  <use href="#body-jiao"/>      <!-- 蛟 的身体 -->

  <!-- Layer 2: 护甲（可选，根据装备切换） -->
  <use href="#armor-abyss-scale"/>  <!-- 渊鳞甲 -->

  <!-- Layer 3: 武器（可选） -->
  <use href="#weapon-trident"/>     <!-- 深渊三叉戟 -->

  <!-- Layer 4: 饰品（可选） -->
  <use href="#acc-claw-medal"/>     <!-- 利爪勋章，悬浮在头顶 -->

  <!-- Layer 5: 表情/特效 -->
  <use href="#fx-idle"/>            <!-- 待机呼吸动画 -->
</svg>
```

**命名规则**：`{类型}-{路线}-{名称}.svg`
- 身体：`body-abyss-claw.svg`（小龙虾）、`body-abyss-octopus.svg`（章鱼）...
- 武器：`weapon-common-claw.svg`（虫爪）、`weapon-rare-trident.svg`（深渊三叉戟）...
- 护甲：`armor-common-shell.svg`（硬壳外骨骼）...
- 饰品：`acc-common-synapse.svg`（突触加速器）...

### 6.4 18 个生物造型要点

**🌊 渊·鲲之路**（蓝色系，流线型，水属性特效）：
| Lv | 名称 | 造型关键词 |
|----|------|-----------|
| 1 | 小龙虾 | 红色身体，大钳子，圆溜溜眼睛，Q版可爱 |
| 5 | 章鱼 | 紫蓝色，8 条触手卷曲，大头，聪明感 |
| 10 | 蛟 | 蛇形龙身，有小角和鳞片，水纹环绕 |
| 20 | 鲲 | 巨大鱼形，鲸鱼般壮硕，深蓝色，气泡特效 |
| 30 | 利维坦 | 海怪造型，触手+巨口+深渊光芒 |
| 50 | 渊皇 | 皇冠+深渊披风，蛟的终极形态，威严+神秘 |

**🏔️ 陆·兽之路**（棕红色系，肌肉感，大地特效）：
| Lv | 名称 | 造型关键词 |
|----|------|-----------|
| 1 | 跳虫 | 经典虫族跳虫，4 条腿，利爪，敏捷 |
| 5 | 刺蛇 | 直立蛇身，背部尖刺，远程射击姿态 |
| 10 | 潜伏者 | 蜘蛛形态，半埋地下，尖刺伸出 |
| 20 | 雷兽 | 巨型四足兽，装甲犀牛感，角+厚甲 |
| 30 | 泰坦 | 巨大双足，机械感甲壳，碾压一切 |
| 50 | 陆皇 | 皇冠+大地铠甲，雷兽终极形态，山岳般稳固 |

**🌪️ 穹·鹏之路**（白金色系，翅膀，风暴特效）：
| Lv | 名称 | 造型关键词 |
|----|------|-----------|
| 1 | 翼龙 | 小型翼龙，膜翅展开，好奇的大眼 |
| 5 | 阿根廷巨鹰 | 巨大翼展，金棕色羽毛，猛禽利爪 |
| 10 | 飞龙 | 虫族飞龙，蝠翼+尾刺，空中盘旋 |
| 20 | 鹏 | 神话巨鸟，金色羽翼，祥云环绕 |
| 30 | 守护者 | 四翼天使型，护盾光环 |
| 50 | 穹皇 | 皇冠+风暴披风，鹏的终极形态，闪电特效 |

### 6.5 战斗动画（CSS 驱动）

| 动作 | CSS 实现 | 时长 |
|------|---------|------|
| 待机 | `@keyframes breathe` 上下浮动 2px | 循环 |
| 攻击 | `translateX(±80px)` 冲刺 + 回位 | 0.4s |
| 受击 | `translateX(∓10px)` 抖动 + 红色闪烁 | 0.3s |
| 暴击 | 攻击动画 + `scale(1.2)` + ⚡ 粒子 | 0.6s |
| 技能 | 全屏特效叠加层（路线色光芒） | 1.0s |
| 死亡 | `opacity: 0` + `translateY(20px)` 下沉 | 0.5s |
| 胜利 | `translateY(-10px)` 跳跃 × 3 | 1.0s |

### 6.6 资源文件结构

```
claw/web/src/assets/pets/
├── bodies/
│   ├── abyss/
│   │   ├── claw.svg          # 小龙虾
│   │   ├── octopus.svg       # 章鱼
│   │   ├── jiao.svg          # 蛟
│   │   ├── kun.svg           # 鲲
│   │   ├── leviathan.svg     # 利维坦
│   │   └── abyssal.svg       # 渊皇
│   ├── terrain/
│   │   ├── zergling.svg      # 跳虫
│   │   └── ...
│   └── sky/
│       ├── pterosaur.svg     # 翼龙
│       └── ...
├── weapons/
│   ├── common-claw.svg       # 虫爪
│   ├── fine-needle.svg       # 腐蚀刺针
│   ├── rare-trident.svg      # 深渊三叉戟
│   └── ...
├── armors/
│   └── ...
├── accessories/
│   └── ...
└── fx/
    ├── hit-spark.svg          # 受击火花
    ├── crit-lightning.svg     # 暴击闪电
    ├── skill-abyss.svg        # 渊系技能特效
    ├── skill-terrain.svg      # 陆系技能特效
    └── skill-sky.svg          # 穹系技能特效
```

### 6.7 React 组件

```tsx
// PetAvatar.tsx — 组合渲染宠物形象
interface PetAvatarProps {
  path: 'abyss' | 'terrain' | 'sky';
  form: string;       // 'claw' | 'octopus' | 'jiao' | ...
  weapon?: string;     // 装备 ID
  armor?: string;
  accessory?: string;
  size: 'sm' | 'md' | 'lg' | 'battle';
  animation?: 'idle' | 'attack' | 'hit' | 'crit' | 'skill' | 'death' | 'win';
}

const sizeMap = { sm: 64, md: 80, lg: 200, battle: 300 };

export function PetAvatar({ path, form, weapon, armor, accessory, size, animation = 'idle' }: PetAvatarProps) {
  return (
    <div className={`pet-avatar pet-${animation}`} style={{ width: sizeMap[size], height: sizeMap[size] }}>
      <img src={`/assets/pets/bodies/${path}/${form}.svg`} className="pet-body" />
      {armor && <img src={`/assets/pets/armors/${armor}.svg`} className="pet-armor" />}
      {weapon && <img src={`/assets/pets/weapons/${weapon}.svg`} className="pet-weapon" />}
      {accessory && <img src={`/assets/pets/accessories/${accessory}.svg`} className="pet-accessory" />}
    </div>
  );
}
```

---

## 7. 开发计划

### 总览

```
Week 1          Week 2          Week 3          Week 4          Week 5+
──────────────────────────────────────────────────────────────────────
P1 成长系统 ──→ P2 日报+资产 ──→ P3 雷达+通知
  (3-4h)          (3-4h)          (2h)
                                    │
                P0 美术资源 ────────┤ (并行，持续产出)
                (AI生成SVG)         │
                                    ▼
                               P4 竞技场核心 ──→ P5 星尘经济 ──→ P6 生存进化 ──→ P7 交易市场
                                 (5-6h)          (3-4h)          (3-4h)          (2-3h)
```

**总工时**：约 **25-30 小时** 代码开发 + 美术资源
**依赖关系**：P1 → P2 → P3（串行）→ P4 → P5 → P6 → P7（串行），P0 与 P1-P3 并行

### P0: 美术资源准备（与 P1-P3 并行）

> 使用 AI 绘图工具（Midjourney/DALL-E/Stable Diffusion）生成 SVG 素材，人工调整。

| 批次 | 内容 | 数量 | 交付节点 |
|------|------|------|---------|
| 0.1 | 🌊 渊系 6 个进化形态（小龙虾→渊皇） | 6 SVG | P3 完成前 |
| 0.2 | 🏔️ 陆系 6 个进化形态（跳虫→陆皇） | 6 SVG | P3 完成前 |
| 0.3 | 🌪️ 穹系 6 个进化形态（翼龙→穹皇） | 6 SVG | P3 完成前 |
| 0.4 | 通用装备（白/绿/蓝各 3-4 件）| ~12 SVG | P4 开始前 |
| 0.5 | 路线专属装备（紫色 3 + 橙色 3）| 6 SVG | P5 开始前 |
| 0.6 | 特效素材（攻击/暴击/技能/受击）| ~8 SVG | P4 开始前 |
| 0.7 | 赛季背景 + 变异标记 | ~6 SVG | P6 开始前 |

**产出目录**：`claw/web/src/assets/pets/{bodies,weapons,armors,accessories,fx}/`

### P1: Agent 成长系统 — 3-4h

> 前置：无。这是整个系统的基础。

| # | 文件 (claw/api/) | 工作 | 时间 |
|---|-----------------|------|------|
| 1.1 | `internal/model/agent_growth.go` | AgentGrowth (含 EvolutionPath) + Milestone model | 30m |
| 1.2 | `internal/growth/engine.go` | ComputeStats（从已有表 COUNT）+ ComputeLevel + DetermineEvolutionPath + CheckMilestones | 60m |
| 1.3 | `internal/api/v1/growth.go` | `GET /v1/agents/:id/growth` + `GET /v1/agents/:id/milestones` | 20m |
| 1.4 | `internal/router/router.go` | AutoMigrate 新表 + 注册路由 | 10m |
| 1.5 | `web/src/pages/GrowthPage.tsx` | Hero 区（宠物形象+等级+属性）+ 三叉进化树 + 里程碑时间线 | 60m |
| 1.6 | `web/src/components/PetAvatar.tsx` | 宠物形象组件（图层叠加 SVG） | 20m |
| 1.7 | `web/src/lib/api.ts` | growthAPI | 10m |

**验收标准**：
- `go build && go vet` 通过
- API 返回正确的等级/路线/里程碑
- 前端展示进化树，宠物形象随等级变化

### P2: 每日报告 + 资产仪表盘 — 3-4h

> 前置：P1 完成。

| # | 文件 | 工作 | 时间 |
|---|------|------|------|
| 2.1 | `internal/growth/daily_report.go` | LLM 生成日报（qwen-turbo）| 40m |
| 2.2 | `internal/api/v1/growth.go` | `GET /v1/agents/:id/daily-report` | 15m |
| 2.3 | `internal/api/v1/assets.go` | 资产概览 API（Agent 数/对话数/知识库/工具/星能）| 40m |
| 2.4 | `web/src/pages/AssetsPage.tsx` | 资产仪表盘（数字卡片 + 趋势图）| 45m |
| 2.5 | `web/src/components/DailyReport.tsx` | 日报弹窗（打开 GrowthPage 时触发）| 20m |
| 2.6 | `internal/api/v1/chat.go` | 对话结束触发里程碑检测 hook | 15m |

**验收标准**：
- 日报内容真实反映 Agent 当日活动
- 资产仪表盘数字与实际一致

### P3: 创作者通知 + 性格雷达图 — 2h

> 前置：P1 完成。

| # | 文件 | 工作 | 时间 |
|---|------|------|------|
| 3.1 | `internal/api/v1/marketplace.go` | Agent 被安装时推送通知给创作者 | 30m |
| 3.2 | `web/src/components/RadarChart.tsx` | 五维雷达图 SVG（对话/记忆/任务/知识/好评）| 40m |
| 3.3 | `web/src/components/AgentCard.tsx` | 列表卡片增加等级 badge + 路线图标 | 20m |

**验收标准**：
- 雷达图正确反映 Agent 各维度数据
- 安装通知可达

### P4: 龙虾竞技场 PK 核心 — 5-6h ⭐ 最大 Phase

> 前置：P1 完成（需要等级/路线数据），P0.4+P0.6 美术交付。

| # | 文件 (queen/arena/) | 工作 | 时间 |
|---|---------------------|------|------|
| 4.1 | `internal/model/battle.go` | Equipment + AgentEquipment + Battle model | 30m |
| 4.2 | `internal/handler/shop.go` | 星能商店列表 + 购买（扣星能，调用 Queen credit API）| 40m |
| 4.3 | `internal/handler/equip.go` | 背包 CRUD + 装备/卸下 + 战斗属性面板 | 40m |
| 4.4 | `internal/engine/battle.go` | **回合制战斗引擎**（伤害公式/暴击/闪避/克制/技能/装备特效/20回合上限）| 80m |
| 4.5 | `internal/handler/battle.go` | 发起对战 + ELO 匹配(±200) + 执行战斗 + ELO 结算 + 掉落判定 | 60m |
| 4.6 | `seed_equipment.sql` | 初始装备数据（~20 件，白绿蓝各品质）| 15m |
| 4.7 | `cmd/main.go` | 路由注册 + AutoMigrate 新表 | 10m |
| 4.8 | `web/src/pages/ArenaPage.tsx` | 竞技场主页（战宠面板+星能商店+背包+排行榜+对战列表）| 80m |
| 4.9 | `web/src/components/BattleReplay.tsx` | 战斗回放动画（逐回合播放，CSS 动画驱动）| 50m |
| 4.10 | `claw/api/internal/growth/engine.go` | 对战 EXP + 装备里程碑联动 | 15m |

**验收标准**：
- 战斗引擎单元测试覆盖：普通攻击/暴击/克制/技能/装备特效/超时平局
- ELO 结算正确（K 值分级）
- 前端可发起对战、查看回放、浏览商店

### P5: 星尘经济系统 — 3-4h

> 前置：P4 完成（对战系统已有，在此基础上加星尘层）。

| # | 文件 | 工作 | 时间 |
|---|------|------|------|
| 5.1 | `queen/arena/internal/model/stardust.go` | StardustAccount + StardustTransaction model | 20m |
| 5.2 | `queen/api/internal/handler/credit.go` | **星尘伴生 hook**：每笔 consume 交易同步写伴生星尘（2⚡→1✨，零头累积）| 40m |
| 5.3 | `queen/arena/internal/handler/stardust.go` | 星尘余额/流水 API + 星尘商店（紫装）| 30m |
| 5.4 | `queen/arena/internal/handler/battle.go` | 对战结算加入星尘奖励（胜利/连胜/首胜）| 20m |
| 5.5 | `queen/arena/internal/handler/enhance.go` | 装备强化(500✨) + 重铸(1000✨) API | 30m |
| 5.6 | `queen/arena/seed_equipment.sql` | 补充紫色装备数据（~6 件，星尘商店）| 10m |
| 5.7 | `web/src/pages/ArenaPage.tsx` | 星尘余额显示 + 星尘商店 tab + 强化/重铸 UI | 40m |
| 5.8 | `queen/arena/cmd/main.go` | AutoMigrate StardustAccount/Transaction | 5m |

**验收标准**：
- 星能消耗时自动伴生星尘（2:1 比率）
- 星尘可购买紫装、强化、重铸
- 星尘流水完整可查

### P6: 生存进化机制 — 3-4h

> 前置：P5 完成。

| # | 文件 | 工作 | 时间 |
|---|------|------|------|
| 6.1 | `queen/arena/internal/model/season.go` | Season model | 15m |
| 6.2 | `queen/arena/internal/handler/season.go` | 当前赛季 API + 赛季排行榜 + 赛季自动轮换（定时任务）| 40m |
| 6.3 | `queen/arena/internal/handler/season.go` | 赛季结算（星尘奖励 + 赛季矿石发放）| 30m |
| 6.4 | `queen/arena/internal/model/craft.go` | CraftMaterial model + 打造配方 | 15m |
| 6.5 | `queen/arena/internal/handler/craft.go` | 材料背包 + 打造 API（成功率 + ±15% 属性浮动）| 40m |
| 6.6 | `queen/arena/internal/model/mutation.go` | AgentMutation model | 10m |
| 6.7 | `queen/arena/internal/engine/battle.go` | 战斗引擎加入赛季 buff + 衰减系数 + 变异被动技能 | 30m |
| 6.8 | `queen/arena/internal/handler/decay.go` | 定时任务：每日检测状态衰减 + ELO 缓降 | 20m |
| 6.9 | `claw/api/internal/growth/engine.go` | 进化升级时 5% 概率触发变异 | 15m |
| 6.10 | `web/src/pages/ArenaPage.tsx` | 赛季 banner + 打造界面 + 材料背包 + 变异✦标记 | 40m |

**验收标准**：
- 赛季自动轮换，buff 正确应用到战斗
- 衰减状态正确计算，打 1 场恢复
- 打造成功率和属性浮动符合设计
- 变异触发率 ~5%，效果正确叠加

### P7: 交易市场 + 星钻荣誉 — 2-3h

> 前置：P5 完成。可与 P6 并行。

| # | 文件 | 工作 | 时间 |
|---|------|------|------|
| 7.1 | `queen/arena/internal/model/trade.go` | TradeOrder model（买卖双方 + 标的 + 价格 + 状态）| 15m |
| 7.2 | `queen/arena/internal/handler/trade.go` | 发布交易 + 市场列表 + 接受交易（原子扣款+转移装备+5%手续费）| 50m |
| 7.3 | `queen/arena/internal/handler/diamond.go` | 星钻持有量查询 + 荣誉特权判定 | 20m |
| 7.4 | `web/src/pages/MarketPage.tsx` | 交易市场页面（上架/购买/我的订单）| 40m |
| 7.5 | `web/src/components/DiamondBadge.tsx` | 星钻荣誉徽章/金框/入场特效 | 20m |

**验收标准**：
- 交易原子性（不会出现扣款成功但装备未转移）
- 5% 手续费正确沉淀
- 星钻荣誉仅影响外观，不影响战力

### 汇总

| Phase | 内容 | 工时 | 前置 | 产出 |
|-------|------|------|------|------|
| P0 | 美术资源 | 持续 | 无 | ~44 个 SVG |
| P1 | 成长系统 | 3-4h | 无 | 等级/路线/里程碑 |
| P2 | 日报+资产 | 3-4h | P1 | 日报/资产仪表盘 |
| P3 | 雷达+通知 | 2h | P1 | 雷达图/安装通知 |
| P4 | 竞技场核心 | 5-6h | P1,P0 | PK/装备/商店/排行 |
| P5 | 星尘经济 | 3-4h | P4 | 伴生星尘/紫装/强化 |
| P6 | 生存进化 | 3-4h | P5 | 赛季/打造/变异/衰减 |
| P7 | 交易+荣誉 | 2-3h | P5 | 交易市场/星钻荣誉 |
| **总计** | | **~25h** | | |

### 新增 DB 表汇总

| Phase | 表名 | 库 |
|-------|------|-----|
| P1 | `agent_growths`, `milestones` | Claw 本地 |
| P4 | `equipments`, `agent_equipments`, `battles` | Queen Arena |
| P5 | `stardust_accounts`, `stardust_transactions` | Queen Arena |
| P6 | `seasons`, `craft_materials`, `agent_mutations` | Queen Arena |
| P7 | `trade_orders` | Queen Arena |

### 新增 API 汇总

| Phase | 端点数 | 所属服务 |
|-------|--------|---------|
| P1 | 2 | Claw |
| P2 | 3 | Claw |
| P3 | 1 | Claw |
| P4 | 10 | Queen Arena |
| P5 | 7 | Queen Arena + Queen Credit |
| P6 | 7 | Queen Arena |
| P7 | 4 | Queen Arena |
| **总计** | **34** | |

### 部署顺序

```
1. Claw 更新（P1+P2+P3）→ git push nydus → 自动部署 starclaw.me
   - 新表 auto-migrate
   - 前端新页面路由
   - 验证：/growth + /assets 页面正常

2. Queen Arena 更新（P4+P5+P6+P7）→ docker-compose up -d
   - 新表 auto-migrate + seed_equipment.sql
   - 星尘伴生 hook 在 Queen credit handler 中
   - 赛季定时任务启动
   - 验证：/arena 页面 + PK + 商店 + 交易

3. 美术资源（P0）→ 随 Claw 前端一起部署
   - SVG 文件在 web/src/assets/pets/
   - 构建时打包进前端 bundle
```

---

## 8. 不做什么

明确列出本方案**不涉及**的内容，避免范围蔓延：

1. **不做虚拟货币/代币** — 星能是功能积分，星尘是游戏币，都不是加密货币
2. **不做区块链（暂时）** — 架构预留链桥，但当前中心化 DB
3. **不做社交网络** — 没有关注/粉丝/动态流
4. **不做付费墙** — 等级和里程碑纯展示，不解锁功能
5. **不改已有 model** — Agent/Memory/Task/Conversation 零改动
6. **不做推送基础设施** — 复用已有 SSE/WebSocket
7. **不做氪金** — 星尘不可充值，只能打出来，防止 pay-to-win
8. **不做永久死亡** — 衰减可恢复，Agent 永远不会死

---

## 9. 成功指标

| 指标 | 目标 | 衡量方式 |
|------|------|---------|
| 情感感知 | 用户能说出"我的 Agent 是 Lv.X" | 用户访谈 |
| 日活留存 | Growth 页面 DAU > 30% 总 DAU | 访问日志 |
| 日报打开率 | > 50% 的活跃用户查看日报 | API 调用统计 |
| 里程碑达成 | 活跃用户平均达成 > 5 个里程碑 | DB 统计 |
| 资产感知 | 用户主动查看资产页 > 1次/周 | API 调用统计 |
| 创作激励 | Agent 市场上架数量增长 > 20% | AgentListing COUNT |
| 竞技场参与 | > 20% 活跃用户发起过至少 1 场对战 | Battle COUNT |
| 竞技场观战 | 平均每场对战 > 3 人观战 | Battle.Spectators AVG |
| 竞技场留存 | 参加过对战的用户 7日留存 > 60% | 对战用户回访率 |
| 星尘活跃 | 活跃用户星尘余额 > 0 占比 > 40% | StardustAccount 统计 |
| 赛季参与 | 每赛季 > 30% 竞技场用户打满 5 场 | Season 统计 |
| 变异热度 | 变异 Agent 在排行榜占比 > 15% | AgentMutation COUNT |

---

## 10. 风险与缓解

| 风险 | 缓解 |
|------|------|
| Stats 聚合查询慢 | 对 COUNT 查询加 date 范围索引，前端加 loading 状态 |
| 日报 LLM 成本 | 用最便宜模型 (qwen-turbo)，且仅用户打开时触发 |
| 里程碑通胀（太容易达成） | 高级里程碑需大量累积，称号使用虫族进化路线保持稀缺感 |
| 等级系统显得"幼稚" | 用虫族军衔而非通用 RPG 等级，保持品牌调性 |
| 数据库新增 2 表 | 极轻量（agent_growths + milestones），无性能影响 |
| 装备氪金感太重 | 商店装备到蓝色为止，紫/橙只能对战掉落，不卖 |
| 高等级碾压新人 | ELO 匹配 ±200，等级差 >20 的不匹配 |
| ELO 通胀/通缩 | 新手保护期前 5 场 K=48，之后 K=32，1500+ 后 K=24 |
| 对战刷分（自己打自己） | 同一 user_id 下的 Agent 不能互相对战 |
| 战斗结果可预测（无聊） | 暴击+闪避+装备特效+克制+±10%伤害浮动，增加随机性 |
| 星能经济崩溃 | 装备有星能沉淀（消费），对战有星能产出（奖励），控制进出比 |
| 衰减逐用户 | 极轻惩罚（打1场就恢复），推送提醒“你的蛛要睡着了” | 
| 赛季环境不公平 | 每赛季只+10% buff，不是压倒性优势，装备/技能可弥补 |
| 打造太胝 | 打造有随机性（±15%浮动），出极品时很兴奋 |
| 变异太强/太弱 | 5%总概率保证稀缺，加成适中(+3%~+10%)，不破坏平衡 |
