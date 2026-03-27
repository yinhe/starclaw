# StarClaw Monorepo — 虫族架构全景

> 灵感来自星际争霸虫族（Zerg）— 每个模块都有明确的虫族角色，协同构成完整的 AI 虫群生态。

---

## 一、模块总览

```
starclaw/                          # 私有 monorepo
│
├── claw/       🦞 小龙虾           # 开源 AI Agent 平台（核心产品）
├── synapse/    ⛽ 突触              # AI 算力网关（star-ai.net）
├── larva/      🐛 幼虫              # 跨平台客户端（Flutter App）
├── queen/      👑 虫后              # 中央管控平台（starclaw.net）
├── overlord/   👁️ 领主              # 企业 AI 管控（overlord.starclaw.net）
├── cerebrate/  🧠 脑虫              # 合伙人生态（partner + city 门户）
├── forge/      🔥 熔炉              # 研发管控 + 可视化大屏（forge.starclaw.net）
├── nydus/      🕳️ 虫道              # 部署管道 + CI/CD
├── spore/      🍄 孢子              # 桌面一键安装器
├── drone/      🐝 工蜂              # 数据采集 + 虫茧同化（Agent/Skill 市场填充）
│
├── docker-compose.yml             # Claw 本地开发
├── docker-compose.prod.yml        # Claw 生产部署
└── .env / .env.example            # 环境变量
```

---

## 二、虫族层级

```
                         👑 Queen（虫后）
                    starclaw.net — 中央大脑
              用户/计费/星能/投资人/监控/社区
                         │
      ┌──────────┬───────┼───────┬──────────┐
      │          │       │       │          │
 👁️ Overlord 🧠 Cerebrate ⛽ Synapse  🐛 Larva
  企业管控层   合伙人生态    算力网关    用户客户端
  节点编排     战略合伙人    LLM 路由   iOS/Android
  RBAC/SSO    城市合伙人    计费/代理   桌面/Web
  订阅计费     拓客/部署     媒体算力
      │          │           │          │
      └────┬─────┴───────────┘          │
           │                            │
      🦞🦞🦞 Claw（小龙虾）               │
       AI Agent 执行单元 ◄──────────────┘
       开源核心产品
           │
      ┌────┼────┐
      │         │
  🕳️ Nydus   🍄 Spore
   部署管道    桌面安装器
   代码分发    一键部署
   自动构建    跨平台打包
```

---

## 三、模块职责 & 定位

| 模块 | 虫族角色 | 一句话定位 | 技术栈 | 开源 |
|------|---------|-----------|--------|:----:|
| **claw/** | 🦞 小龙虾 | AI Agent 执行引擎 — 对话/工具/RAG/工作流/MCP | Go + React | ✅ |
| **synapse/** | ⛽ 突触 | AI 算力网关 — 统一代理 50+ 模型 + 媒体算力 + 算力市场 | Go + Node.js + React | ❌ |
| **larva/** | 🐛 幼虫 | 跨平台用户入口 — 移动端/桌面/Web 一体化客户端 | Flutter (Dart) | ❌ |
| **queen/** | 👑 虫后 | 中央管控 — 用户/计费/星能/投资人/社区/赏金/竞技场 | Go + React × 7 服务 | ❌ |
| **overlord/** | 👁️ 领主 | 企业管控 — 节点编排/RBAC/SSO/订阅/Molt 更新 | Go + React × 2 前端 | ❌ |
| **cerebrate/** | 🧠 脑虫 | 合伙人生态 — 战略合伙人门户 + 城市合伙人门户 | React × 2 前端 | ❌ |
| **forge/** | 🔥 熔炉 | 研发管控 — 项目/Issue/看板/Sprint + 可视化大屏 | Go + React | ❌ |
| **nydus/** | 🕳️ 虫道 | 部署管道 — git push → 自动分发到 3 台服务器 | Go + React | ❌ |
| **spore/** | 🍄 孢子 | 桌面安装器 — 一键部署 Claw（免 Docker） | Go | ❌ |

---

## 四、模块间调用关系

### 4.1 核心数据流

```
用户
 │
 ├─ 📱 Larva App ──────────┐
 │    Flutter 客户端         │
 │    调用 Queen API         │
 │                           ▼
 ├─ 🖥️ Claw Web UI ──→ 🦞 Claw API ──→ ⛽ Synapse (star-ai.net)
 │    React 前端        Go 后端           │ 国内模型: 直连 (Qwen/DeepSeek)
 │                      │                 │ 海外模型: → Proxy → OpenAI/Claude/Grok
 │                      │                 │
 │                      │                 ▼
 │                      │           💰 按 token 计费
 │                      │           Synapse → Queen /internal/credits/consume
 │                      │
 │                      ├──→ 👑 Queen (Swarm 注册/心跳/星能)
 │                      ├──→ 👁️ Overlord (企业管辖时)
 │                      └──→ 🦞 其他 Claw (P2P Gossip/Nydus 直连)
 │
 └─ 🏢 企业管理员 ──→ 👁️ Overlord Console ──→ Overlord API ──→ Queen
```

### 4.2 模块间 API 调用矩阵

| 调用方 → 被调方 | Queen | Synapse | Claw | Overlord | Nydus |
|:----------------|:-----:|:-------:|:----:|:--------:|:-----:|
| **Claw** | Swarm 注册/心跳, 星能查询 | /v1/chat (AI 推理) | P2P 直连 | 注册/心跳 (企业) | — |
| **Synapse** | /internal/credits (扣费) | — | — | — | — |
| **Queen** | — | — | — | — | — |
| **Overlord** | 上报节点/消费 | — | 管辖下发 | — | — |
| **Larva** | 用户 API (登录/充值/社区) | — | — | — | — |
| **Nydus** | — | — | — | — | Worm (部署执行) |

### 4.3 认证体系

```
三种认证方式，覆盖所有场景：

1. Ed25519 签名认证（Claw ↔ Synapse / Queen）
   X-Claw-ID + X-Claw-Timestamp + X-Claw-Signature
   → 无需密码，私钥签名即身份

2. API Key 认证（外部开发者 → Synapse）
   Authorization: Bearer sk-star-xxx
   → star-ai.net 网页用户/第三方开发者

3. JWT 认证（Larva / Web → Queen / Overlord）
   Authorization: Bearer <jwt>
   → 传统登录流，邮箱+密码 或 Claw 地址登录
```

---

## 五、星能经济流转

星能（Star Energy ⚡）是虫群的血液，贯穿所有模块：

```
充值入口                              消费出口
────────                              ────────
 Queen 网页充值 ─┐                ┌─ Synapse AI 推理 (按 token)
 Larva App 充值 ─┤                ├─ 赏金发布 (冻结)
 企业订阅充值 ───┘                ├─ 市场购买 (模板/插件)
        │                         └─ 节点间转账
        ▼
  ┌─────────────┐     grant      ┌─────────────┐
  │ UserBalance  │ ──────────→   │CreditAccount │
  │ (¥ 余额)    │   星能兑换     │ (⚡ 星能)    │
  │ Queen 记账   │               │ Queen 记账    │
  └─────────────┘               └──────┬───────┘
                                       │ consume
                                       ▼
                                  Claw / Synapse
                                  每次 API 调用扣费

换算：1 分 (¥0.01) = 1 ⚡ = 10,000 内部单位
```

---

## 六、部署拓扑

### 6.1 三服务器架构

```
┌─────────────────────────────────────────────────────────────────┐
│                        开发者本地                                │
│  git push nydus master ──→ 自动触发下方所有部署                  │
└────────────────────────────┬────────────────────────────────────┘
                             │ SSH
                             ▼
┌─ Server C: starclaw.net (新加坡 43.106.158.26) ────────────────┐
│                                                                  │
│  👑 Queen (10 容器)         🕳️ Nydus Server + Worm              │
│  ├─ queen-api    :8085      ├─ nydus-api   :8095                │
│  ├─ queen-web    :8086      ├─ nydus-web   :80                  │
│  ├─ swarm        :8090      └─ nydus-worm  :8096                │
│  ├─ core         :8091                                           │
│  ├─ bounty       :8092      👁️ Overlord (3 容器)                │
│  ├─ forum        :8093      ├─ overlord-api     :8098           │
│  ├─ arena        :8094      ├─ overlord-console :3095           │
│  ├─ overseer     :8087      └─ overlord-web     :3096           │
│  └─ proxy        :8000                                           │
│                             🧠 Cerebrate (2 容器)               │
│  🌏 Proxy (海外 AI 中转)     ├─ partner      :8088               │
│  └─ queen-proxy :8000      └─ city         :8089               │
│                                                                  │
│  ──→ SSH 分发代码到 Server A + Server B                          │
└──────────────┬──────────────────────────┬───────────────────────┘
               │                          │
               ▼                          ▼
┌─ Server A: starclaw.me ──┐  ┌─ Server B: star-ai.net ──────┐
│                           │  │                               │
│  🦞 Claw (官方实例)       │  │  ⛽ Synapse                   │
│  ├─ claw-api    :8080    │  │  ├─ synapse-api  :8096        │
│  ├─ claw-web    :8081    │  │  ├─ synapse-web  :3096        │
│  └─ 官网静态文件          │  │  ├─ synapse-core :3097        │
│     /var/www/starclaw/    │  │  └─ gateway      :8085        │
│                           │  │     (queen-api 网关副本)       │
│  代码来源: claw/ + site/  │  │                               │
└───────────────────────────┘  │  代码来源: synapse/ + queen/api│
                               └───────────────────────────────┘
```

### 6.2 代码分发映射

| monorepo 子目录 | 部署目标 | 服务器路径 | 方式 |
|:---------------|:---------|:----------|:-----|
| `queen/` | Server C (本地) | `/opt/queen/` | Nydus Worm |
| `cerebrate/` | Server C (本地) | `/opt/queen/` | Nydus Worm (随 queen 一起构建) |
| `overlord/` | Server C (本地) | `/opt/overlord/` | Nydus Worm |
| `queen/api/` | Server B (Gateway) | `/opt/starclaw/gateway/` | SSH + Worm |
| `synapse/` | Server B | `/opt/starclaw/synapse/` | SSH + Worm |
| `claw/` | Server A | `/opt/starclaw/` | SSH direct |
| `queen/site/` | Server A (官网) | `/var/www/starclaw/website/` | SSH + Docker build |

### 6.3 Nydus 一键部署流

```
git push nydus master
    │
    ▼
starclaw.git (bare repo, Server C)
    │ post-receive hook
    ▼
Nydus Server (:8095) — 检测变更目录，分发到对应 target
    │
    ├─ queen/ 变更 ──→ 本地 Worm → docker compose build (Queen 10 容器)
    ├─ cerebrate/ 变更 → 本地 Worm → docker compose build (Cerebrate 2 容器)
    ├─ overlord/ 变更 → 本地 Worm → docker compose build (Overlord 3 容器)
    ├─ queen/api/ 变更 → SSH → Server B → Worm → Gateway 重建
    ├─ synapse/ 变更 ──→ SSH → Server B → Worm → Synapse 重建
    ├─ claw/ 变更 ────→ SSH → Server A → 直接构建
    └─ queen/site/ 变更 → SSH → Server A → Docker 多阶段构建 → 静态文件
```

---

## 七、模块详细衔接

### 7.1 Claw ↔ Queen（虫群注册）

```
Claw 首次启动:
  → POST queen/swarm/register { node_id, public_key, version, region }
  ← { config, queen_url, heartbeat_interval }

Claw 运行中 (每 30s):
  → POST queen/swarm/heartbeat { node_id, status, metrics }
  ← { config_updates, molt_release? }

作用: Queen 掌握全网所有 Claw 节点状态，可下发配置/更新/任务。
```

### 7.2 Claw ↔ Synapse（AI 推理）

```
Claw 需要 AI 推理时:
  → POST star-ai.net/v1/chat/completions
    Header: X-Claw-ID + X-Claw-Signature (Ed25519)
    Body: { model: "qwen/qwen-max", messages: [...] }

Synapse 处理:
  1. 验证签名 → 识别 claw_id
  2. 查 Queen 星能余额 → 充足则放行
  3. 路由到 Provider (国内直连 / 海外走 Proxy)
  4. 返回结果 + usage { prompt_tokens, completion_tokens }
  5. POST queen/internal/credits/consume → 扣除星能

也可 BYOK (自带 API Key) → 不走 Synapse，不消耗星能。
```

### 7.3 Claw ↔ Overlord（企业管辖）

```
企业场景:
  Claw 配置 overlord_url → 注册到 Overlord（而非直连 Queen）
  Overlord 管辖: 节点编排 / RBAC / 预算 / 审计 / Molt 审批
  Overlord 上报 Queen: 节点数量 + 消费统计

个人场景:
  Claw 直连 Queen (Swarm)，不经过 Overlord

层级: Queen > Overlord > Claw
```

### 7.4 Larva ↔ Queen（用户客户端）

```
Larva 是 Queen 的移动/桌面前端:
  → Queen API: 登录/注册/充值/个人中心/赏金/社区/Agent 市场
  → 内嵌 Zergling Claw 引擎 (未来): Go → .so/.framework 移动端运行

Larva 不直接调用 Synapse — 通过内嵌 Claw 间接使用算力。

当前 6 屏: 首页 / AI 对话 / Agent / 工作流 / 钱包 / 我的
```

### 7.5 Spore — 安装分发

```
Spore 是 Claw 的安装载体:

用户下载 StarClaw-Setup.exe / .dmg
  → Spore 安装器解包 Claw 二进制
  → 配置数据目录 / 端口 / 邀请码
  → 启动 Claw 进程 (免 Docker)
  → Molt 自动更新: 定期从 Nydus 拉取新版本

Spore Desktop (localhost:7890):
  管理多个 Claw 实例 — 启动/停止/日志/配置
```

### 7.6 Nydus — 部署中枢

```
Nydus 是虫群的物流系统:

  Server (调度中心):
    接收 git push → 按配置分发到各 target
    管理 Git 仓库 → 提供公开 API (claw.git 可外部访问)
    Release API → Spore/Molt 拉取更新

  Worm (执行 Agent):
    每台服务器一个 Worm → 接收代码 → 执行 docker compose build

  Dashboard (Web UI):
    查看仓库/提交/部署记录/服务器状态
```

---

## 八、关键接口一览

| 接口 | 调用路径 | 用途 |
|------|---------|------|
| `queen/swarm/register` | Claw → Queen | 节点注册 |
| `queen/swarm/heartbeat` | Claw → Queen | 心跳 + 状态上报 |
| `queen/internal/credits/consume` | Synapse → Queen | AI 推理扣费 |
| `queen/internal/credits/grant` | Synapse → Queen | 充值到账 |
| `star-ai.net/v1/chat/completions` | Claw → Synapse | AI 对话 |
| `star-ai.net/v1/models` | Claw/Larva → Synapse | 模型列表 |
| `queen/api/v1/auth/*` | Larva → Queen | 用户认证 |
| `queen/api/v1/billing/*` | Larva → Queen | 充值/账单 |
| `overlord/api/v1/registry/*` | Claw → Overlord | 企业节点管理 |
| `nydus/hooks/post-receive` | git push → Nydus | 触发部署 |
| `nydus/releases/latest` | Spore/Molt → Nydus | 获取最新版本 |

---

## 九、虫族命名体系

| 虫族名 | 英文 | 模块 | 类比 |
|--------|------|------|------|
| 小龙虾 | Claw | `claw/` | 工蜂 — 干活的基本单位 |
| 突触 | Synapse | `synapse/` | 神经网络 — 连接算力资源 |
| 幼虫 | Larva | `larva/` | 虫卵 → 幼虫 — 用户接触虫群的入口 |
| 虫后 | Queen | `queen/` | 虫群大脑 — 指挥一切 |
| 领主 | Overlord | `overlord/` | 中层管理 — 管辖一片区域的 Claw |
| 熔炉 | Forge | `forge/` | 进化室 — 研发指挥中心，熔炼项目与进度 |
| 虫道 | Nydus | `nydus/` | 传送网络 — 代码瞬间到达目的地 |
| 孢子 | Spore | `spore/` | 繁殖体 — 在新设备上萌发 |
| 蜕皮 | Molt | Claw 内置 | 自动更新 — 脱壳进化 |
| 虫群 | Swarm | Queen 内置 | 全网节点注册网络 |
| 虫巢 | Brood | Overlord 内置 | 企业级节点注册 |
| 星能 | Star Energy | 跨模块 | 虫群血液 — 内部货币 |
| 菌毯 | Creep | 规划中 | 数据同步层 — 覆盖即掌控 |

---

## 十、开发指南

### 本地开发各模块

```bash
# Claw（开源核心）
cd claw/api && go run ./cmd/server
cd claw/web && npm run dev

# Synapse（算力网关）
cd synapse/api && go run ./cmd/server
cd synapse/web && npm run dev

# Queen（中央管控 — 需要完整 docker compose）
cd queen && docker compose up -d

# Overlord（企业管控）
cd overlord && docker compose up -d

# Larva（Flutter 客户端）
cd larva && flutter run

# Nydus
cd nydus && docker compose up -d

# Spore
cd spore && go run ./cmd/spore
```

### 部署到生产

```bash
# 一键部署所有变更的模块
git push nydus master

# 手动部署单个模块（见 .windsurf/workflows/deploy.md）
```

---

## 十一、开发者生态

StarClaw 的核心竞争力 = **Agent + 技能 + 工作流** 三位一体。
开发者可以创建智能体、开发技能、编排工作流，并通过市场发布盈利。

### 11.1 架构总览

```
开发者
 │
 ├── 创建 Agent（智能体） ──→ 配置人格 + 模型 + 技能组合 + 知识库
 │
 ├── 开发技能（主动/被动）
 │    ├── 主动技能: Go Tool / JSON Plugin / MCP Server
 │    └── 被动技能: Webhook 事件规则 / 工作流触发器 / 计费钩子
 │
 ├── 编排工作流 ──→ 可视化 DAG（Start → LLM → Tool → Condition → End）
 │
 └── 发布到市场 ──→ Agent / 模板 / 插件 → 定价 + 审核 + 上架 → 分润
```

---

### 11.2 智能体（Agent）开发

Agent 是 StarClaw 的核心交互单元 — 一个拥有人格、技能和记忆的 AI 实体。

```
Agent = System Prompt（人格）
      + Model（LLM 模型）
      + Tools[]（技能列表）
      + KnowledgeBase?（RAG 知识库）
      + Config（温度/max_tokens/...）
```

**创建方式：**

| 方式 | 说明 | 适合 |
|------|------|------|
| **Web UI** | 页面上填写 prompt + 选择模型 + 勾选技能 | 普通用户 |
| **API** | `POST /v1/agents` 传 JSON | 开发者/自动化 |
| **市场安装** | 一键安装社区共享的 Agent | 所有人 |
| **模板导入** | 从模板创建，可自定义修改 | 快速上手 |

**Agent 数据模型（`claw/api/internal/model/agent.go`）：**

| 字段 | 作用 |
|------|------|
| `SystemPrompt` | LLM 系统提示词 — 定义 Agent 的角色和行为 |
| `ModelName` | 使用的 LLM 模型（如 `qwen/qwen-max`） |
| `Tools` | JSON 数组，Agent 可调用的技能名列表 |
| `KnowledgeBaseID` | 绑定的 RAG 知识库 ID |
| `Config` | JSON 配置（temperature, max_tokens, top_p 等） |
| `IsPublic` | 是否公开（可被其他用户发现） |
| `SourceID` | 从市场安装时关联的 MarketplaceItem ID |

**Agent 能力边界：**

```
Agent 能做什么取决于给了它哪些技能：

🤖 纯对话 Agent     → Tools: []                  → 只能文本对话
🔍 研究 Agent       → Tools: [web_search, browser] → 能搜索和浏览网页
🎨 创作 Agent       → Tools: [image_generation, video_generation, music_generation]
💻 编程 Agent       → Tools: [code, git]          → 能写代码、执行、版本管理
📊 数据 Agent       → Tools: [code, http_request]  → 能跑 Python 分析 + 调 API
🏢 企业 Agent       → Tools: [system, wecom, dingtalk, feishu] → 能管理任务+通知
🎬 影视 Agent       → Tools: [video_generation, dubbing, subtitle, comic_production, mv_production]
🌐 全能 Agent       → Tools: [*]                  → 所有技能全开
```

---

### 11.3 技能（Skill/Tool）开发

技能是 Agent 的手和脚 — 让 AI 能做对话以外的事情。

#### 主动技能 vs 被动技能

| 类型 | 触发方式 | 例子 | 开发位置 |
|------|---------|------|---------|
| **主动技能** | LLM 判断用户意图后主动调用 | 搜索、生图、写代码、发消息 | `claw/api/internal/tool/` |
| **被动技能** | 事件/条件自动触发，无需 LLM 决策 | Webhook 推送、定时工作流、计费钩子、记忆提取 | `webhook/` `workflow/` `billing/` `memory/` |

#### 主动技能开发 — 三种方式

**① Go Built-in Tool（核心技能）**

```go
// claw/api/internal/tool/my_tool.go
type MyTool struct{ db *gorm.DB }

func (t *MyTool) Name() string        { return "my_tool" }
func (t *MyTool) Description() string  { return "技能描述，LLM 据此决定何时调用" }
func (t *MyTool) Parameters() interface{} { return &JSONSchema{...} }
func (t *MyTool) Execute(ctx context.Context, args string) (string, error) {
    // 执行逻辑，返回 JSON 结果
}
```

注册到 `router.go`: `toolRegistry.Register(tool.NewMyTool(db))`

**② JSON Plugin（零代码，包装 HTTP API）**

```json
// claw/plugins/weather.json
{
  "name": "weather",
  "description": "查询城市天气",
  "parameters": { "type": "object", "properties": { "city": { "type": "string" } } },
  "endpoint": { "url": "https://api.weather.com/v1/current?q={{city}}", "method": "GET" }
}
```

放入 `plugins/` 目录即自动加载，无需编译。

**③ MCP Server（外部工具服务器）**

```
任何实现 Model Context Protocol 的服务器都可以接入：
  → Claw UI: MCP 页面 → 添加服务器 URL → 自动发现所有工具
  → MCP Bridge: 本地桥接器，让 Agent 调用宿主系统命令
  → 第三方 MCP: 社区生态中任何 MCP 兼容的工具服务
```

#### 当前已有的 24+ 内置主动技能

| 分类 | 技能 | 说明 |
|------|------|------|
| **系统** | `system` | Agent/工作流/任务 CRUD + 委派 |
| **编程** | `code` | 沙箱代码执行（13 种语言） |
| **版本控制** | `git` | Git 操作（clone/commit/push/branch/diff） |
| **搜索** | `web_search` | 多引擎网页搜索 |
| **浏览** | `browser` | Headless Chrome 浏览器控制 |
| **HTTP** | `http_request` | 通用 HTTP 请求 |
| **图片** | `image_generation` | AI 图片生成（Flux/fal.ai） |
| **视频** | `video_generation` | AI 视频生成（多模型+场景衔接） |
| **音乐** | `music_generation` | AI 音乐/歌曲生成 |
| **配音** | `dubbing` | TTS 配音（CosyVoice 多音色） |
| **字幕** | `subtitle` | 视频 SRT 字幕添加 |
| **漫剧** | `comic_production` | 图片→漫剧视频（Ken Burns + AI 动画） |
| **MV** | `mv_production` | 音乐+视频合成 MV |
| **音频** | `audio_analysis` | 音频分析（BPM/能量/节拍） |
| **文档** | `document` | 对话总结 + Word 导出 |
| **赏金** | `bounty` | 悬赏人类完成 AI 做不了的任务 |
| **部署** | `deploy_web` | 网站部署（Vercel Deploy Hook） |
| **域名** | `bind_domain` | Cloudflare DNS 管理 |
| **验证** | `verify_online` | 上线健康检查 + 关键词验证 |
| **通讯** | `wecom` `dingtalk` `feishu` `slack` `discord` `telegram` | 6 大平台消息推送 |

#### 被动技能 — 事件驱动自动化

**Webhook 事件规则（`claw/api/internal/webhook/`）：**

```
事件类型:
  agent.complete    — Agent 完成任务
  agent.error       — Agent 执行出错
  workflow.complete  — 工作流执行完成
  chat.message      — 收到新消息
  ...

规则引擎:
  IF event_type == "agent.error"
  AND data.agent_name == "客服机器人"
  THEN POST https://hooks.dingtalk.com/xxx { ... }
```

**计费钩子（`claw/api/internal/tool/tool.go` → `ExecuteHook`）：**

```
每个技能执行时自动经过 Billing Gateway:
  Before: 检查余额 → 查定价表
  Execute: 执行技能
  After: 计算成本 → 扣费 → 分润 → 同步 Synapse 对账
```

**记忆提取（`claw/api/internal/memory/`）：**

```
每次对话后自动触发:
  → LLM 分析对话内容
  → 提取事实/偏好/指令/技能/上下文
  → 存入记忆数据库 (带重要性评分)
  → 下次对话自动召回相关记忆
```

---

### 11.4 工作流（Workflow）

工作流是可视化的 AI 流水线 — 把多个 LLM 和技能串联成自动化流程。

**工作流引擎（`claw/api/internal/workflow/engine.go`）：**

```
工作流 = 有向无环图 (DAG)

节点类型：
  ┌─────────┐    ┌─────────┐    ┌──────────┐    ┌───────────┐    ┌─────────┐
  │  Start  │───→│   LLM   │───→│   Tool   │───→│ Condition │───→│   End   │
  │ (入口)  │    │ (AI推理) │    │ (技能调用)│    │ (条件分支) │    │ (出口)  │
  └─────────┘    └─────────┘    └──────────┘    └───────────┘    └─────────┘
                      │                              │ true
                      │                              │ false
                      ▼                              ▼
                 可配置模型           可配置表达式
                 可配置 Prompt        input.contains("X")
                 可配置温度           input.length > N
```

**触发方式：**

| 触发 | 说明 | API |
|------|------|-----|
| **手动执行** | UI 点击 "运行" | `POST /v1/workflows/:id/run` |
| **Webhook 触发** | 外部系统 POST 到 Webhook URL | `POST /v1/webhooks/workflow/:token` |
| **Agent 委派** | Agent 通过 system tool 调起工作流 | `system` tool → `run_workflow` action |
| **定时触发** | Webhook 事件规则 + Cron 表达式 | 事件规则引擎 |

**示例：自动客服工作流**

```json
{
  "nodes": [
    { "type": "start" },
    { "type": "llm", "data": { "model": "qwen/qwen-max", "prompt": "分类用户问题: {{input}}" } },
    { "type": "condition", "data": { "expression": "input.contains(\"退款\")" } },
    { "type": "tool", "data": { "toolName": "http_request", "argsTemplate": "{\"url\":\"https://crm.example.com/api/refund\"}" } },
    { "type": "llm", "data": { "model": "qwen/qwen-max", "prompt": "用友好的语气回复用户: {{input}}" } },
    { "type": "end" }
  ],
  "edges": [...]
}
```

---

### 11.5 市场 & 分润（Marketplace）

开发者生态的经济循环：创作 → 上架 → 销售 → 分润。

```
┌─ 创作者 ────────────────────────────────────────────────────┐
│                                                              │
│  注册 CreatorProfile → 提交 Agent/模板/插件 → 审核 → 发布    │
│                                                              │
│  可上架物：                                                   │
│  ├── Agent 模板 (AgentTemplate)    — 预设好的智能体            │
│  ├── 工作流模板 (WorkflowTemplate) — 自动化流程               │
│  └── 技能插件 (PluginListing)      — JSON/MCP 技能包          │
│       ├── 7 大分类: api/data/productivity/dev/media/finance/social │
│       ├── 定价: free / paid (自定义价格)                       │
│       └── 审核: draft → pending_review → published            │
│                                                              │
└──────────────────────────────────────────────────────────────┘
                             │
                             ▼
┌─ 用户 ──────────────────────────────────────────────────────┐
│                                                              │
│  浏览市场 → 安装/购买 → 评分(1-5⭐) → 使用                    │
│                                                              │
└──────────────────────────────────────────────────────────────┘
                             │
                             ▼
┌─ 分润引擎 ──────────────────────────────────────────────────┐
│                                                              │
│  用户支付 ¥100                                                │
│  ├── 上游成本: ¥90.91 (成本 ÷ 1.10)                          │
│  ├── 毛利: ¥9.09                                             │
│  │   ├── 创作者:   70%  = ¥6.36                              │
│  │   ├── 平台:     25%  = ¥2.27                              │
│  │   └── 推荐人:    5%  = ¥0.45                              │
│  └── 投资人始终拿总利润的 10%（从上方各方等比扣除）             │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

**市场 API（`claw/api/internal/api/v1/marketplace.go`）：**

| 端点 | 说明 |
|------|------|
| `POST /v1/marketplace/creator` | 注册创作者 |
| `GET /v1/marketplace/items` | 浏览市场 |
| `POST /v1/marketplace/items/:id/install` | 安装/购买 |
| `POST /v1/marketplace/items/:id/rate` | 评分 |
| `GET /v1/marketplace/stats` | 创作者收益统计 |

---

### 11.6 开发者门户（Developer Portal）

**OpenAPI 文档（`/developer/docs`）：**
- 自动生成 OpenAPI 3.0 规范
- 内置 Swagger UI 可交互
- 覆盖 11 个 Tag: Auth / Agents / Chat / Workflows / Knowledge / Tools / Marketplace / Observe / Webhooks / P2P / System

**API Playground（`/developer/playground`）：**
- 在线调试任意 API 端点
- 自动记录请求/响应历史
- 显示耗时、状态码

---

### 11.7 开发者入门路径

```
新手路径:
  1. 创建 Agent（Web UI）→ 选模型 + 写 Prompt + 勾选技能 → 对话测试
  2. 添加 JSON Plugin → 包装外部 API → Agent 获得新能力
  3. 编排 Workflow → 串联多步骤 → Webhook 触发自动化

进阶路径:
  4. 开发 Go Tool → 实现 Tool 接口 → 注册到 Registry → 深度集成
  5. 连接 MCP Server → 扩展宿主系统能力（文件/Shell/数据库）
  6. 配置 Webhook 规则 → 事件驱动 → 企业系统联动（钉钉/飞书/企微）

创作者路径:
  7. 注册 CreatorProfile → 上架 Agent/模板/插件 → 审核发布
  8. 设置定价 → 用户安装 → 自动分润 → 提现

详细文档: claw/docs/SKILL_DEVELOPMENT.md
```
