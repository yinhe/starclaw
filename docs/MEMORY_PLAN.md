# Claw 记忆系统 v2 — 设计文档

> Cerebrate Memory System: 让 Claw 真正"记住"用户

## 1. 现状分析

### 1.1 已实现

| 层 | 文件 | 状态 |
|---|---|---|
| 数据模型 | `model/memory.go` | ✅ Memory 5 类 (preference/fact/context/skill/instruct) + importance + access_count |
| 引擎 | `memory/cerebrate.go` | ✅ Retrieve (关键词召回) + ExtractAndStore (LLM提取) + BuildPromptInjection (注入) |
| API | `api/v1/memory.go` | ✅ 7 端点: list/create/update/delete/clear/stats/recall |
| 对话集成 | `api/v1/chat.go` | ✅ 每次对话前注入记忆，对话后异步提取 |
| 前端 API | `web/src/lib/api.ts` | ✅ memoryAPI 5 方法 |
| 前端 UI | — | ❌ **完全没有** |

### 1.2 核心缺陷

1. **用户不可见** — 后端静默工作，用户不知道 Claw 记住了什么
2. **无法管理** — 不能查看、编辑、删除、搜索记忆
3. **无来源追踪** — 不知道记忆来自哪次对话
4. **无全局记忆** — 记忆绑定 agent_id，换 Agent 就丢失
5. **无生命周期** — 不会衰减、不会合并、无上限控制
6. **召回精度低** — 仅 LIKE 关键词匹配，无向量语义搜索

---

## 2. 架构设计

### 2.1 数据流全景

```
┌─────────────────────────────────────────────────────────────────┐
│                        用户对话开始                               │
│                             │                                    │
│                             ▼                                    │
│              Cerebrate.Retrieve(query)                           │
│                    ┌────────┴────────┐                           │
│                    ▼                 ▼                           │
│            Agent 专属记忆      Global 全局记忆                    │
│                    └────────┬────────┘                           │
│                             ▼                                    │
│              BuildPromptInjection()                              │
│                    注入 system prompt                            │
│                             │                                    │
│                             ▼                                    │
│                    LLM 参考记忆回答                               │
│                             │                                    │
│                             ▼                                    │
│                      对话结束                                    │
│                             │                                    │
│              ┌──────────────┼──────────────┐                    │
│              ▼              ▼              ▼                    │
│    ExtractAndStore()  GenerateSummary()  前端 toast              │
│    (提取事实记忆)     (生成会话摘要)     "新记忆 +N"             │
│              │              │                                    │
│              ▼              ▼                                    │
│         Upsert Memory   Summary Memory                          │
│              │              │                                    │
│              ▼              ▼                                    │
│         MemoryPage 实时展示 + 管理                               │
└─────────────────────────────────────────────────────────────────┘
```

### 2.2 记忆分层

```
┌─────────────────────────────────────────┐
│  L0: 指令记忆 (instruct)                │  "以后都用中文回答"
│  — 用户显式指令，最高优先级，永不衰减     │
├─────────────────────────────────────────┤
│  L1: 事实记忆 (fact)                    │  "用户是全栈工程师"
│  — 用户信息和项目事实                    │
├─────────────────────────────────────────┤
│  L2: 偏好记忆 (preference)              │  "偏好 TypeScript + React"
│  — 用户偏好和习惯                        │
├─────────────────────────────────────────┤
│  L3: 技能记忆 (skill)                   │  "FFmpeg xfade 实现视频转场"
│  — 成功解决问题的经验方法                 │
├─────────────────────────────────────────┤
│  L4: 上下文记忆 (context)               │  "正在开发 StarClaw 项目"
│  — 近期上下文，会衰减                    │
├─────────────────────────────────────────┤
│  L5: 摘要记忆 (summary)                 │  "上次讨论了 Squad 系统架构"
│  — 会话摘要，自动生成，最多保留 N 条     │
└─────────────────────────────────────────┘
```

---

## 3. 数据模型扩展

### 3.1 Memory 模型新增字段

```go
type Memory struct {
    // ... 现有字段保留 ...

    // 新增字段
    Scope          string `json:"scope" gorm:"type:varchar(20);default:agent;index"` // "agent" | "global"
    ConversationID string `json:"conversation_id,omitempty" gorm:"type:varchar(36);index"` // 来源会话
    Tags           string `json:"tags,omitempty" gorm:"type:json"`  // JSON array: ["golang","react"]
}
```

- `Scope = "global"`: 全局记忆，所有 Agent 对话时都注入
- `Scope = "agent"`: 绑定特定 Agent（现有行为）
- `ConversationID`: 追踪来源，前端可点击跳转原会话
- `Tags`: 自由标签，用于搜索和分组

---

## 4. API 扩展

### 4.1 现有端点升级

```
GET  /memories          — 新增 ?scope=global|agent&search=keyword&tags=tag1,tag2 参数
POST /memories          — 新增 scope, conversation_id, tags 字段
GET  /memories/stats    — 新增 scope 维度统计
GET  /memories/recall/:agent_id — 同时返回 global 记忆
```

### 4.2 新增端点

```
GET  /memories/search   — 语义搜索（P4 阶段）
GET  /memories/timeline — 按时间线返回记忆（含来源会话标题）
POST /memories/merge    — 手动合并两条记忆
```

---

## 5. Cerebrate 引擎升级

### 5.1 Retrieve 升级

```go
func (c *Cerebrate) Retrieve(userID, agentID, query string, maxResults int) ([]Memory, error) {
    // 1. 始终加载 instruct 类型（agent + global）
    // 2. 加载 global scope 记忆（所有分类）
    // 3. 加载 agent 专属记忆（关键词匹配）
    // 4. P4 阶段: embedding cosine similarity 混合排序
    // 5. 去重 + importance 排序 + 截断
}
```

### 5.2 ExtractAndStore 升级

```go
func (c *Cerebrate) ExtractAndStore(...) (int, error) {  // 返回提取数量
    // 1. 现有 LLM 提取逻辑
    // 2. 新增: 写入 ConversationID
    // 3. 新增: 自动判断 scope (跨 agent 通用信息 → global)
    // 4. 新增: 合并检测 (同 key 不同 content → 更新而非创建)
    // 5. 返回 stored 数量，供前端 toast 使用
}
```

### 5.3 会话摘要生成（P5）

```go
func (c *Cerebrate) GenerateSummary(userID, agentID, conversationID string, messages []ChatMessage) error {
    // 1. 对话 ≥ 5 轮时触发
    // 2. LLM 生成 1-2 句摘要
    // 3. 存为 category="summary", scope="agent", conversation_id=xxx
    // 4. 每 agent 最多保留最近 20 条摘要
}
```

---

## 6. 记忆生命周期管理（P3）

### 6.1 自动衰减

```
每 24 小时后台 Job:
  - context 类记忆: importance -= 0.01 × daysSinceLastAccess
  - skill 类记忆:   importance -= 0.005 × daysSinceLastAccess
  - fact/preference: 不衰减
  - instruct:        不衰减
  - importance < 0.1 → 标记为 stale，30 天后自动删除
```

### 6.2 上限控制

```
每 agent 记忆上限: 200 条
超限策略:
  1. 删除 stale 记忆
  2. 删除 importance 最低的 context 记忆
  3. 合并相似 fact 记忆
```

### 6.3 合并检测

```
新提取记忆时:
  1. 查找同 key 的现有记忆 → 更新 content（已有）
  2. 查找 content 高度相似的记忆（P4 阶段用 embedding 判断）
  3. 相似度 > 0.9 → 自动合并（保留 importance 较高者，content 取较新者）
  4. 相似度 0.7-0.9 → 标记为候选合并，前端提示用户确认
```

---

## 7. 前端设计

### 7.1 MemoryPage（独立页面）

```
┌──────────────────────────────────────────────────────────┐
│  🧠 记忆中心                                              │
│                                                          │
│  ┌──────┐ ┌──────┐ ┌──────┐ ┌──────┐ ┌──────┐          │
│  │  42  │ │  12  │ │   8  │ │   6  │ │  16  │          │
│  │ 总计  │ │ 事实  │ │ 偏好  │ │ 指令  │ │ 技能  │          │
│  └──────┘ └──────┘ └──────┘ └──────┘ └──────┘          │
│                                                          │
│  🔍 搜索记忆...  [全部▾] [全部Agent▾] [+ 新建记忆]       │
│                                                          │
│  ┌────────────────────────────────────────────────────┐  │
│  │ 📌 preferred_language                               │  │
│  │ 用户偏好使用中文进行对话                              │  │
│  │ #preference  ⭐ 0.9  🤖 自动提取  📅 3天前           │  │
│  │                                    [编辑] [删除]    │  │
│  ├────────────────────────────────────────────────────┤  │
│  │ 📋 project_name                                     │  │
│  │ 用户正在开发 StarClaw 智能体平台                      │  │
│  │ #fact  ⭐ 0.85  🌐 全局  💬 来自「Squad架构讨论」      │  │
│  │                                    [编辑] [删除]    │  │
│  ├────────────────────────────────────────────────────┤  │
│  │ 🛠️ video_transition_method                          │  │
│  │ 使用 FFmpeg xfade filter 实现视频片段间的转场效果     │  │
│  │ #skill  ⭐ 0.7  🤖 自动提取  📅 1周前                │  │
│  │                                    [编辑] [删除]    │  │
│  └────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────┘
```

### 7.2 ChatPage 右侧面板集成

在现有「关联面板」中新增「记忆」折叠区：

```
┌─ 关联面板 ──────────────┐
│                          │
│  🧠 记忆  (8条)     ▼   │
│  ├ 📌 用户偏好中文      │
│  ├ 📋 StarClaw 项目     │
│  ├ 🛠️ FFmpeg 转场       │
│  └ + 手动添加记忆        │
│                          │
│  📋 任务  (2运行中)  ▼  │
│  ...                     │
│  🔗 工作流  (1)      ▼  │
│  ...                     │
└──────────────────────────┘
```

### 7.3 对话后 Toast 通知

```
对话结束 → ExtractAndStore 返回提取数量 → WebSocket 推送 → 前端 toast:
  "🧠 已学习 3 条新记忆"  [查看]
```

### 7.4 左侧导航

```
能力 (group)
  ├ 模型
  ├ 知识库
  ├ 技能 & MCP
  └ 记忆        ← 新增
```

---

## 8. 开发阶段

### P1: 前端记忆页面 + 导航（优先）

**后端改动**：
- `memory.go` API: 增加 search 查询参数
- Chat SSE: 对话结束后推送 `memory_extracted` 事件

**前端新增**：
- `MemoryPage.tsx`: 统计卡片 + 搜索筛选 + 记忆列表 + CRUD 弹窗
- `Layout.tsx`: 导航新增「记忆」入口
- `App.tsx`: /memories 路由
- `api.ts`: memoryAPI 扩展 search/stats
- `i18n.ts`: 翻译 key
- `ChatPage.tsx`: 右侧面板增加记忆折叠区

### P2: 全局记忆 + 来源追踪

**后端改动**：
- `model/memory.go`: 新增 Scope, ConversationID, Tags 字段
- `memory/cerebrate.go`: Retrieve 升级 (global + agent 混合召回), ExtractAndStore 写入 ConversationID
- `api/v1/memory.go`: 端点升级支持新字段
- AutoMigrate 新字段

**前端改动**：
- MemoryPage: 新增 scope 筛选 (全局/Agent专属)
- 记忆卡片: 显示来源会话链接
- 创建记忆弹窗: 新增 scope 和 tags

### P3: 记忆生命周期

**后端新增**：
- `memory/lifecycle.go`: 衰减 Job + 上限淘汰 + 合并检测
- 在 router.go 启动后台 lifecycle goroutine

### P4: 向量语义召回

**后端改动**：
- `model/memory.go`: 新增 Embedding 字段 (JSON float array)
- `memory/cerebrate.go`: 提取时生成 embedding, 召回时 cosine similarity
- 利用现有 RAG embedding 基础设施

### P5: 会话摘要记忆

**后端改动**：
- `memory/cerebrate.go`: 新增 GenerateSummary()
- `api/v1/chat.go`: 对话结束时调用 GenerateSummary
- 新增 MemCatSummary 分类常量

---

## 9. 安全考虑

- 记忆数据属于用户隐私，仅 user_id 所有者可访问
- 全局记忆 (scope=global) 仍然绑定 user_id，不跨用户
- 记忆导出/删除符合 GDPR（已有 security 模块支持）
- LLM 提取的记忆不包含敏感信息（提取 prompt 中强调）

---

## 10. 成功指标

- 用户可在 MemoryPage 看到所有记忆并管理
- 对话中引用记忆的命中率 > 80%
- 每次有效对话平均提取 1-3 条记忆
- 记忆总量稳定在合理范围（衰减 + 上限控制）
- 跨会话连续性：用户感知 Claw "记得"之前的对话内容
