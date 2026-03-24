# Claw 记忆系统 v2 — 设计文档

> Cerebrate Memory System: 让 Claw 真正"记住"用户

## 1. 实现状态 (v2026.0323)

### 1.1 已实现

| 层 | 文件 | 状态 |
|---|---|---|
| 数据模型 | `model/memory.go` | ✅ Memory 6 类 (preference/fact/context/skill/instruct/summary) + importance + scope + tags |
| 引擎 | `memory/cerebrate.go` | ✅ Retrieve (关键词召回) + ExtractAndStore (LLM提取) + GenerateSummary + BuildPromptInjection |
| 生命周期 | `memory/lifecycle.go` | ✅ 每日衰减 + 过期清理 + 容量限制 (200条/agent) |
| API | `api/v1/memory.go` | ✅ 7 端点: list/create/update/delete/clear/stats/recall |
| 对话集成 | `api/v1/chat.go` | ✅ 对话前注入记忆，对话后异步提取 + 摘要 |
| 前端 API | `web/src/lib/api.ts` | ✅ memoryAPI 完整 |
| 前端 UI | `web/src/pages/MemoryPage.tsx` | ✅ 统计卡片 + 分类筛选 + Agent筛选 + 搜索 + CRUD + 新建记忆弹窗 |
| 全局记忆 | `cerebrate.go` scope | ✅ fact/preference/instruct 自动标记 global，跨 Agent 共享 |
| 来源追踪 | `model/memory.go` ConversationID | ✅ 每条记忆关联来源会话 |

### 1.2 已修复 Bug

| Bug | 修复 | 版本 |
|---|---|---|
| `getExtractionProvider` 选到空 Key 的 openrouter → 提取静默 401 | 优先 star-ai，跳过空 Key | v2026.0323.1809 |
| `SeedStarAIForAllUsers` 每次启动重建用户删除的 star-ai config | 移除启动时全量 seed | v2026.0323.1809 |

### 1.3 当前缺陷 (待优化)

1. **召回精度低** — 仅 LIKE 关键词匹配，无向量语义搜索
2. **提取模型未指定** — 用用户主聊天模型做提取，浪费大模型算力
3. **提取失败无重试** — LLM 调用失败只打 log，无重试机制
4. **缺少"记住这个"快捷操作** — 对话中说"记住这个"不会触发即时记忆
5. **跨节点记忆不同步** — Hive 模式下多节点记忆独立
6. **记忆冲突** — 同 key 覆盖可能丢失旧信息

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

### P1: 前端记忆页面 + 导航 ✅ 已完成

- `MemoryPage.tsx`: 统计卡片 + 6 类分类筛选 + Agent 筛选 + 搜索 + CRUD
- `Layout.tsx`: 左侧导航「记忆」入口
- `App.tsx`: /memories 路由
- `api.ts`: memoryAPI 完整 (list/create/update/delete/clear/stats)

### P2: 全局记忆 + 来源追踪 ✅ 已完成

- `model/memory.go`: Scope + ConversationID + Tags 字段
- `cerebrate.go`: fact/preference/instruct 自动 scope=global，跨 Agent 共享
- Retrieve 升级: instruct (最高优先) → global → agent keyword-matched

### P3: 记忆生命周期 ✅ 已完成

- `memory/lifecycle.go`: 每 24h 后台 Job
  - context 衰减 0.01/天, skill 衰减 0.005/天, summary 衰减 0.008/天
  - fact/preference/instruct 不衰减
  - importance < 0.1 + 30 天未访问 → 自动删除
  - 每 agent 上限 200 条，超限删除最低 importance

### P4: 向量语义召回 ❌ 未实现

当前仅 LIKE 关键词匹配，需升级为 embedding 向量检索。

**计划**：
- `model/memory.go`: 新增 Embedding 字段 (JSON float array)
- `memory/cerebrate.go`: 提取时调用 `rag.EmbeddingProvider` 生成 embedding
- Retrieve 时 cosine similarity 混合排序 (keyword + semantic)
- 利用现有 RAG embedding 基础设施 (`internal/rag/`)

### P5: 会话摘要记忆 ✅ 已完成

- `cerebrate.go` GenerateSummary(): 对话 ≥ 5 轮用户消息时生成
- LLM 生成 1-2 句摘要，存为 category=summary
- 每 agent 最多保留 20 条摘要，FIFO

---

## 8b. 下一步优化路线 (v3)

### O1: 提取模型优化 (低成本)

**问题**: 用用户主聊天模型做记忆提取，浪费大模型算力
**方案**: `getExtractionProvider` 优先选轻量模型 (qwen-turbo / deepseek-chat)
```go
// 新增提取专用模型选择逻辑
// 1. 优先 star-ai (免费)
// 2. 其次找 qwen-turbo / deepseek-chat 等低成本模型
// 3. 最后才用用户主模型
```

### O2: 提取失败重试

**问题**: LLM 调用失败只打 log，记忆丢失
**方案**: 失败后延迟 30s 重试一次
```go
func (c *Cerebrate) ExtractAndStore(...) {
    err := c.doExtract(...)
    if err != nil {
        time.AfterFunc(30*time.Second, func() {
            c.doExtract(...) // 重试一次
        })
    }
}
```

### O3: "记住这个" 即时记忆

**问题**: 用户说"记住这个"时，需等异步提取，且不一定提取到
**方案**: 在 chat handler 中检测关键词，即时创建记忆
```go
// 检测: "记住", "remember", "以后都", "别忘了"
// 提取当前消息上下文，立即创建 instruct 类记忆
```

### O4: 对话后 Toast 通知

**问题**: 用户不知道 Claw 学到了什么
**方案**: ExtractAndStore 返回数量 → SSE/WebSocket 推送 → 前端 toast
```
"🧠 已学习 3 条新记忆" [查看]
```

### O5: 跨节点记忆同步 (Hive)

**问题**: Hive 模式下多个 Claw 节点记忆独立
**方案**: 通过 Queen API 做中心化记忆存储，节点间定期同步

### O6: 记忆冲突与合并

**问题**: 同 key 直接覆盖，可能丢失有价值的旧信息
**方案**:
- 相似度 > 0.9 → 自动合并 (保留 importance 较高者，content 取较新者)
- 相似度 0.7-0.9 → 标记候选，前端提示用户确认
- 需要 P4 (向量召回) 作为前置依赖

### O7: Agent 记忆继承

**问题**: 新建 Agent 不知道用户基本信息
**方案**: 新建 Agent 首次对话时，自动注入用户的 global 记忆 (fact/preference/instruct)

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
