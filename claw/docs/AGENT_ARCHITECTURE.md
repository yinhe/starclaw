# Agent 七元组架构 (Agent Heptad)

> 一个 Agent = 基因 + 技能 + 本能 + MCP 外接 + 工作流 + 记忆 + 腺体
>
> Gene + Skill + Instinct + MCP + Workflow + Memory + Gland

## 一、设计原则

1. **七元组完备** — 任何 Agent（内置/市场/自建）都由且仅由这 7 个组件构成
2. **MCP 唯一外接** — 所有外部工具通过 MCP 协议接入，废弃 plugin JSON 文件
3. **安装即完整** — 从市场安装一个 Agent = 一次性部署全部 7 个组件
4. **组件可热插拔** — 每个组件独立增删，不影响其他组件

### 命名约定

| 组件 | 中文 | English | 本质 |
|------|------|---------|------|
| ① | **基因** | Gene | 灵魂——定义 Agent 是谁 |
| ② | **技能** | Skill | 用户问→做（你让我做，我就做） |
| ③ | **本能** | Instinct | 自动→做（我自己会做，不用你管） |
| ④ | **外接** | MCP | 外部工具服务 |
| ⑤ | **工作流** | Workflow | 多步自动管道 |
| ⑥ | **记忆** | Memory | 经验积累 |
| ⑦ | **腺体** | Gland | 运行配置——凭证、参数、开关 |

**技能** vs **本能**：技能是“你让我做我就做”，本能是“我自己会做不用你管”。不说被动/主动，避免绕晕。

## 二、七元组定义

```
┌─────────────────────────────────────────────────────────┐
│                    Agent 七元组                           │
│                                                         │
│  ┌──────────┐  一个 Agent 的灵魂                          │
│  │ ① 基因   │  名字、人设、规则、模型偏好                    │
│  └──────────┘                                           │
│                                                         │
│  ┌──────────┐  ┌──────────┐                              │
│  │ ② 技能   │  │ ③ 本能   │  行为层                       │
│  │  Skill  │  │ Instinct │                              │
│  │ 用户问→做 │  │ 自动→做  │                              │
│  └──────────┘  └──────────┘                              │
│                                                         │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐ │
│  │ ④ MCP   │  │ ⑤ 工作流 │  │ ⑥ 记忆   │  │ ⑦ 腺体   │ │
│  │  外接    │  │          │  │          │  │  Gland  │ │
│  │ 外部工具  │  │ 多步管道  │  │ 经验积累  │  │ 运行配置  │ │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘ │
└─────────────────────────────────────────────────────────┘
```

**腺体 (Gland)** — 昆虫腺体存储并释放调控行为的关键物质。Agent 腺体存储运行配置：API Key、账号凭证、阈值参数、功能开关。凭证类自动 AES-256 加密存储。

---

### ① 基因 (Gene)

Agent 的灵魂。定义它是谁、怎么思考、遵循什么规则。

```go
type AgentGene struct {
    Name        string  // "Q8bot 量化分析师「麒博」"
    Description string  // 一句话描述
    Icon        string  // Lucide 图标名
    Prompt      string  // 系统提示词（人设 + 规则 + 输出规范）
    Model       string  // 首选模型 "qwen-max"
    Temperature float64 // 0.3
    MaxTokens   int     // 4096
    TopP        float64 // 0.9
}
```

**关键区分**：Prompt 只定义 Agent 的身份和规则，不包含具体工具使用方法（那是技能层的事）。

**命名理由**：如同生物基因决定物种特征，Agent 基因决定其本质。改基因 = 换了一个 Agent。

---

### ② 技能 (Skill)

用户提问时自动匹配触发的能力。像游戏里的主动技能，你按了才释放。

```go
type Skill struct {
    Name            string   // "个股诊断"
    Description     string   // "分析单只股票的趋势、支撑压力位"
    TriggerPatterns []string // ["帮我看看{code}", "分析一下{stock}", "{code}怎么样"]
    Tools           []string // ["trading_kline", "trading_quote", "web_search"]
    PromptTemplate  string   // 追加到基因 Prompt 后的技能说明
    Examples        []string // ["帮我看看600519", "000001.SZ怎么样"]
}
```

**工作机制**：
```
用户消息 → TriggerPatterns 模糊匹配
  → 命中 → 将 PromptTemplate 注入当前对话上下文
  → Agent 获得技能说明 + 工具列表 → 执行
  → 未命中 → Agent 用基因 Prompt 自由回答
```

**与基因的关系**：基因定义"我是谁"，技能定义"用户问X时，我该怎么做"。

---

### ③ 本能 (Instinct)

不需要用户触发，像生物本能一样自动运行。已有 UI：“本能系统”页面（关怀本能/时间本能/监控本能/事件本能）。

```go
type Instinct struct {
    Name        string   // "盘前分析"
    Description string   // "每个交易日08:30自动分析全球市场"
    Schedule    string   // cron: "0 30 8 * * 1-5"
    Tools       []string // ["trading_premarket", "web_search"]
    Prompt      string   // 执行时的指令 prompt
    AutoExecute bool     // true = 自动执行，false = 仅提醒
    Notify      bool     // 执行完推送通知给用户
    Enabled     bool     // 用户可开关
}
```

**工作机制**：
```
Cron 调度器 → 到时间
  → 创建临时对话
  → 注入 Instinct.Prompt + 工具列表
  → Agent 执行 → 结果写入对话历史
  → Notify=true → 推送通知给用户
```

---

### ④ MCP 外接服务 (MCP Service)

所有外部工具的唯一接入方式。

```go
type MCPService struct {
    Name        string // "Q8bot Trading Bridge"
    URL         string // "http://localhost:8098" (安装时可配置)
    Description string // "miniQMT 行情+交易接口"
    AutoConnect bool   // 启动时自动连接
}
```

**核心决策：废弃 plugin JSON，统一走 MCP**

| 旧方案 (plugin JSON) | 新方案 (MCP only) |
|---|---|
| 每个工具一个 JSON 文件 | MCP 服务器自动暴露所有工具 |
| 参数/端点硬编码 | MCP 协议动态发现 |
| 安装时写 11 个文件 | 安装时注册 1 个 MCP 服务 |
| URL 不可配 | URL 安装时配置 |

**MCP 服务器端（Bridge）负责**：
- 注册时返回工具列表（tools/list）
- 执行工具调用（tools/call）
- 健康检查

**Agent 工具解析顺序**：
```
Agent 要调用工具 "trading_scan"
  → 1. 查内置工具 registry → 未找到
  → 2. 查该 Agent 绑定的 MCP 服务 → 找到 Trading Bridge
  → 3. 调用 MCP tools/call → Bridge 执行 → 返回结果
```

---

### ⑤ 工作流 (Workflow)

多步骤自动化管道，绑定到具体 Agent。

```go
type AgentWorkflow struct {
    Name        string // "Q8bot 全日交易工作流"
    Description string
    Definition  string // JSON DAG: {nodes, edges}
    AgentID     string // 绑定的 Agent（当前：全局，改为：per-agent）
    Schedule    string // 可选：定时触发
    Enabled     bool
}
```

**与本能的区别**：
- **本能** = 单步任务（调几个工具，输出结果）
- **工作流** = 多步管道（节点串联/条件分支/并行执行）

```
主动技能:  盘前分析 → 直接出结果
工作流:    盘前分析 → 判断方向 → 扫描 → 确认 → 下单 → 监控 → 复盘
```

---

### ⑥ 记忆 (Memory)

Agent 的经验积累，跨会话持久化。

```go
// 已有实现 (model/memory.go)，增强为 Agent 六元组的正式成员
type AgentMemory struct {
    // 已有
    Category   string  // preference/fact/context/skill/instruct/summary
    Scope      string  // agent(私有) / global(共享)
    Importance float64 // 0.0-1.0，衰减+清理

    // 新增
    SeedMemory bool    // true = 安装时种入的初始记忆（不衰减）
}
```

**记忆类型**：

| 类型 | 来源 | 示例 | 衰减 |
|------|------|------|------|
| **种子记忆** | 安装包预置 | "A股交易时间9:30-15:00" | 不衰减 |
| **用户指令** | 用户说"记住" | "以后分析时带上换手率" | 不衰减 |
| **偏好** | 自动提取 | "用户偏好看日K线" | 不衰减 |
| **事实** | 自动提取 | "用户持有600519" | 不衰减 |
| **上下文** | 自动提取 | "正在关注半导体板块" | 缓慢衰减 |
| **技能经验** | 执行总结 | "600519 止盈策略成功率高" | 缓慢衰减 |
| **会话摘要** | 自动生成 | "讨论了银行股估值" | 正常衰减 |

---

## 三、统一安装包格式 (Agent Bundle)

从市场安装一个 Agent 时，下载的就是这个 Bundle：

```json
{
  "version": "1.0.0",
  "gene": {
    "name": "Q8bot 量化分析师「麒博」",
    "description": "A股AI量化交易分析师...",
    "icon": "TrendingUp",
    "prompt": "你是Q8bot...",
    "model": "qwen-max",
    "temperature": 0.3,
    "max_tokens": 4096
  },
  "skills": [
    {
      "name": "个股诊断",
      "description": "分析单只股票...",
      "trigger_patterns": ["帮我看看{}", "分析一下{}", "{}怎么样"],
      "tools": ["trading_kline", "trading_quote", "web_search"],
      "prompt_template": "用户想分析个股...",
      "examples": ["帮我看看600519"]
    }
  ],
  "instincts": [
    {
      "name": "盘前分析",
      "category": "time",
      "schedule": "0 30 8 * * 1-5",
      "tools": ["trading_premarket", "web_search"],
      "prompt": "分析今日市场...",
      "auto_execute": true,
      "notify": true
    }
  ],
  "mcp_services": [
    {
      "name": "Q8bot Trading Bridge",
      "url": "http://localhost:8098",
      "description": "miniQMT 行情+交易接口",
      "auto_connect": true
    }
  ],
  "workflows": [
    {
      "name": "Q8bot 全日交易工作流",
      "definition": {"nodes": [...], "edges": [...]}
    }
  ],
  "memory_seeds": [
    {"key": "trading_hours", "content": "A股交易时间：9:30-11:30, 13:00-15:00，周一到周五", "category": "fact"},
    {"key": "color_convention", "content": "中国A股：红色=涨/盈利，绿色=跌/亏损", "category": "fact"},
    {"key": "risk_rules", "content": "单票不超10%，止损-5%，跟踪止盈回落8%", "category": "instruct"}
  ],
  "pricing": {
    "model": "one_time",
    "price_cents": 29900,
    "currency": "CNY"
  }
}
```

---

## 四、DB Schema 变化

### 现状 → 目标

```
现状 agents 表:
  system_prompt  (longtext)    ← 基因+技能说明混在一起
  tools          (json)        ← 平铺工具名，不分来源
  config         (json)        ← temperature/max_tokens
  model_name     (varchar)

目标 agents 表:
  gene           (json)        ← 干净的基因定义
  source_id      (varchar)     ← 市场安装来源
  source_version (varchar)     ← 安装版本，用于更新检测
```

### 新增/改造表

```sql
-- agent_skills → 重命名为 agent_abilities（统一技能+本能）
-- ability_type: 'skill' (你让我做) | 'instinct' (我自己做)
ALTER TABLE agent_skills ADD COLUMN ability_type VARCHAR(20) DEFAULT 'skill';
ALTER TABLE agent_skills ADD COLUMN instinct_category VARCHAR(20);
-- instinct_category: care/time/monitor/event (对应 UI 本能分类)

-- agent_mcp_bindings: Agent ↔ MCP 多对多绑定 (新表)
CREATE TABLE agent_mcp_bindings (
  id         VARCHAR(36) PRIMARY KEY,
  agent_id   VARCHAR(36) NOT NULL,
  mcp_id     VARCHAR(36) NOT NULL,
  created_at DATETIME
);

-- workflows: 加 agent_id 字段 (从全局改为 per-agent)
ALTER TABLE workflows ADD COLUMN agent_id VARCHAR(36) DEFAULT '';

-- memories: 加 seed 标记
ALTER TABLE memories ADD COLUMN is_seed BOOLEAN DEFAULT FALSE;
```

---

## 五、交互流程

### 对话流程（技能触发）

```
用户: "帮我看看600519"
  │
  ▼
Claw Chat Handler
  │
  ├─ 1. 加载 Agent 基因 → system_prompt
  ├─ 2. 加载技能列表 → 匹配 TriggerPatterns
  │     命中「个股诊断」
  ├─ 3. 注入技能 PromptTemplate 到上下文
  ├─ 4. 加载记忆 → 注入相关记忆
  ├─ 5. 解析工具列表 → 内置 + MCP 服务暴露的工具
  ├─ 6. 调用 LLM
  │     LLM 决定调用 trading_kline(code="600519.SH")
  ├─ 7. 工具路由:
  │     trading_kline 不在内置 → 查 MCP 绑定
  │     → Trading Bridge → MCP tools/call → 返回K线数据
  ├─ 8. LLM 继续分析 → 返回诊断结果
  └─ 9. 记忆提取: "用户关注600519" → 写入记忆
```

### 定时流程（本能触发）

```
Cron 08:30 触发
  │
  ▼
Instinct Scheduler
  │
  ├─ 1. 查找到时的 Instincts
  │     → 「盘前分析」(agent: Q8bot)
  ├─ 2. 创建临时会话
  ├─ 3. 加载 Agent 基因 + 本能 Prompt
  ├─ 4. 执行 LLM → 调用 trading_premarket (MCP)
  ├─ 5. 结果写入会话历史
  ├─ 6. 推送通知给用户
  └─ 7. 记忆提取: "今日看多，建议七成仓" → 写入记忆
```

---

## 六、Q8bot 在新架构下的完整定义

| 组件 | 数量 | 内容 |
|------|------|------|
| **基因** | 1 | 麒博人设 + 风控规则 + 输出规范 |
| **技能** | 5 | 个股诊断、持仓复盘、风险排查、全市场扫描、市场解读 |
| **本能** | 4 | 盘前分析、自动扫描、持仓监控、日终复盘 |
| **MCP 服务** | 1 | Trading Bridge (暴露 11 个交易工具) |
| **工作流** | 1 | 全日交易工作流 (10 节点) |
| **种子记忆** | 3+ | 交易时间、颜色惯例、风控规则 |

**安装一个 Q8bot = 写入 6 张表，注册 1 个 MCP 服务，零文件写入磁盘。**

---

## 七、迁移路径

### Phase 1: Schema 迁移（向后兼容）
- agents 表保留旧字段，新增 `gene` JSON 字段
- agent_skills 新增 `trigger_type` 字段
- 新建 `agent_mcp_bindings` 表
- workflows 新增 `agent_id` 字段
- memories 新增 `is_seed` 字段

### Phase 2: 写入兼容
- 新建 Agent 时写 gene 字段 + 旧字段（双写）
- InstallRemote 按新 Bundle 格式安装
- 旧 Agent 读取时从旧字段 fallback

### Phase 3: 读取迁移
- Chat Handler 优先读 gene，fallback 旧字段
- 被动技能匹配引擎上线
- 主动技能调度器上线
- MCP 工具路由上线

### Phase 4: 清理
- 废弃 plugins/ 目录
- 废弃 agents.system_prompt / tools / config 旧字段
- 所有内置 Agent 迁移到 Bundle 格式
