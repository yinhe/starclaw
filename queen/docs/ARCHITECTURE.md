# StarClaw — 系统架构文档

> 最后更新：2026-03-07

---

## 一、项目总览

StarClaw（星爪/小龙虾）是一个 **开源 AI Agent 编排平台**，采用 **私有 Monorepo + 公开开源仓库** 的混合模式。核心 Agent 引擎完全开源，商业增值功能闭源运营。

命名灵感来自**星际争霸虫族（Zerg）**——每一个开源部署的 StarClaw 实例就是一只小龙虾（Claw），
它们由领主（Overlord）进行企业管理，所有领主最终汇聚到虫后（Queen）的中央管控之下。
一个 Overlord 管辖下的所有 Claw 统称为一个 Brood（虫群）。

**开源仓库：** `github.com/yinhe/starclaw`
**私有 Monorepo：** 包含所有模块（开源 + 闭源）

---

## 二、目录结构

```
starclaw/                              # 私有 Monorepo 根目录
│
├── claw/ 🦞                           # 开源模块（→ github.com/yinhe/starclaw）
│   ├── api/                           # Go 后端 API 服务
│   │   ├── cmd/server/main.go         # 入口
│   │   ├── configs/config.yaml        # 配置文件
│   │   ├── internal/                  # 业务逻辑
│   │   │   ├── agent/                 # Agent 引擎（runtime/react/multi）
│   │   │   ├── api/v1/               # HTTP Handlers
│   │   │   ├── browser/              # 无头浏览器（Computer Use）
│   │   │   ├── config/               # 配置结构体
│   │   │   ├── database/             # 数据库迁移 & Seed
│   │   │   ├── middleware/           # 认证/限流/RBAC/日志
│   │   │   ├── model/                # GORM 数据模型
│   │   │   ├── provider/             # LLM Provider（OpenAI/Qwen/DeepSeek/Ollama…）
│   │   │   ├── rag/                  # RAG Pipeline（分块/嵌入/检索）
│   │   │   ├── router/               # 路由注册
│   │   │   ├── sandbox/              # 代码沙箱
│   │   │   ├── tool/                 # Tool 系统（浏览器/代码/视频/音乐/配音…）
│   │   │   ├── worker/               # 异步任务 Worker
│   │   │   └── workflow/             # 工作流引擎
│   │   ├── plugins/                  # JSON 工具插件
│   │   ├── Dockerfile
│   │   └── go.mod                    # module github.com/yinhe/starclaw
│   │
│   ├── web/                           # React 前端
│   │   ├── src/
│   │   │   ├── pages/                # 页面（Chat/Agents/Workflow/Coding/…）
│   │   │   ├── components/           # 公共组件
│   │   │   ├── lib/                  # API 层 & i18n
│   │   │   └── stores/               # Zustand 状态管理
│   │   ├── Dockerfile
│   │   ├── nginx.conf                # 容器内 Nginx
│   │   └── package.json              # name: starclaw-web
│   │
│   ├── data/                          # 运行时数据（视频/图片/音乐/工作区）
│   ├── deploy/                        # 部署配置（nginx-starclaw.conf）
│   ├── scripts/                       # 工具脚本（sync-oss.sh）
│   └── docs/                          # 开源文档（Claw 专属）
│       ├── README.md                  # 开源项目概览
│       ├── DEPLOY.md                  # 自部署指南
│       └── API.md                     # API 接口文档
│
├── overlord/ 👁️                        # 闭源（企业付费）— 领主管理层
│   ├── manager/                       # Overlord 管理服务（Go，计划中）
│   │   ├── cmd/                       # 入口
│   │   └── internal/
│   │       ├── registry/              # Claw 节点注册 & 发现
│   │       ├── scheduler/             # 任务调度 & 负载均衡
│   │       ├── monitor/               # 健康监控 & 指标采集
│   │       └── nydus/                 # P2P 隧道管理
│   └── console/                       # Overlord 管理控制台（Web UI，计划中）
│
├── queen/ 👑                          # 闭源（官方运营）— 虫后中央管控
│   ├── docs/                          # 全局架构文档
│   │   ├── ARCHITECTURE.md            # ← 本文件
│   │   ├── PRODUCT_PLAN.md
│   │   └── TECH_STACK.md
│   ├── mobile/                        # Flutter 官方客户端（iOS + Android）
│   │   ├── lib/
│   │   │   ├── screens/              # 页面
│   │   │   ├── services/             # API 服务
│   │   │   └── main.dart
│   │   └── pubspec.yaml
│   ├── site/                          # 官网落地页
│   ├── core/                          # 管理后台 — 虫后（计划中）
│   ├── billing/                       # 充值计费平台（计划中）
│   ├── swarm/                         # 虫群管理服务（计划中）
│   ├── bounty/                        # 赏金任务平台（计划中）
│   ├── forum/                         # 用户社区论坛（计划中）
│   └── arena/                         # 机器人论坛（计划中）
│
│  ── 根目录配置 ────────────────────────────────────
├── docker-compose.yml                 # 开发环境
├── docker-compose.prod.yml            # 生产环境
├── deploy.sh / update.sh              # 部署脚本
├── .env.example / .env.production
└── README.md
```

---

## 三、虫群架构（Swarm）

StarClaw 采用 **虫后 Queen → 领主 Overlord → 小龙虾 Claw** 三级虫群架构，
灵感来自星际争霸虫族，实现中心化管控与分布式执行的统一。

```
                    ┌──────────────┐
                    │  虫后 Queen   │  core/ + billing/ + swarm/
                    │  (中央管控)    │  闭源，starclaw.me 运营
                    └──────┬───────┘
                           │
              ┌────────────┼────────────┐
              │            │            │
        ┌─────┴───────┐ ┌─────┴──────┐ ┌─────┴───────┐
        │ 👁️ Overlord │ │ Overlord  │ │  Overlord  │  企业付费
        │ (领主管理) │ │           │ │            │  管辖一个 Brood
        └─────┬─────┘ └───┬────┘ └────┬─────┘
              │            │            │
        ┌─────┼─────┐     │      ┌─────┼─────┐
        │     │     │     │      │     │     │
       🦞   🦞   🦞    🦞    🦞   🦞   🦞    小龙虾 Claw
       Claw Claw Claw  Claw  Claw Claw Claw   开源 api/ 实例
```

### 3.1 虫后 Queen（闭源）

- **角色：** 中央管控节点，整个 StarClaw 虫群的大脑
- **模块：** `queen/core/` + `queen/billing/` + `queen/swarm/`
- **职责：**
  - 全局节点注册 & 健康监控（心跳检测）
  - 任务路由与负载均衡（将用户请求分发到最优 Claw）
  - 统一计费 & 用量汇总
  - 数据聚合 & 全局 Dashboard
  - 节点配置下发（模型列表、策略更新）
  - **Molt 蜕皮更新**（版本发布、灰度推送、回滚控制）
  - 运营管理后台（用户管理、Overlord/Claw 管理、财务报表）

### 3.2 领主 Overlord（闭源，企业付费）

- **角色：** 企业级中间管理节点，管辖一群小龙虾（其管辖的 Claw 集群称为一个 Brood）
- **实现：** `overlord/manager/` 管理服务 + `overlord/console/` 管理控制台，内嵌 Claw 实例
- **职责：**
  - 管理下属 Claw 节点（注册、心跳、任务分配）
  - 资源配额（人口上限）— 并发任务数、Token 消耗限额
  - 侦察视野（监控）— CPU/内存/错误率/延迟指标采集
  - 任务运输（负载均衡）— 将任务智能分配到最优 Claw
  - 本地数据缓存 & 聚合，向上注册到 Queen
  - 自身也可作为 Claw 执行 AI 任务
- **部署：** 购买并安装 `overlord/` 软件包，与 Claw 实例并行运行

### 3.3 小龙虾 Claw（开源）

- **角色：** 最小执行单元，每一个独立部署的 StarClaw 就是一只小龙虾
- **实现：** 开源的 `claw/api/` 实例，标准模式
- **职责：**
  - 执行 AI Agent 任务（对话、工作流、Tool Calling）
  - 向上级节点（Overlord 或 Queen）发送心跳
  - 接收任务分配
  - 上报用量数据
  - **Molt 自动更新**（检查新版本、拉取、滚动重启）
- **配置：** `server.node_role: claw`（默认）

### 3.4 虫群通信协议

```
Claw/Overlord → Queen/Overlord:
  - POST /swarm/register    # 节点注册（ID、能力、地址、角色）
  - POST /swarm/heartbeat   # 心跳（状态、负载、用量）
  - GET  /swarm/config      # 拉取配置（模型列表、策略）
  - GET  /swarm/update/check # 检查版本更新

Queen/Overlord → Claw:
  - POST /swarm/task/assign   # 任务下发
  - POST /swarm/config/push   # 配置推送
  - POST /swarm/update/notify # 推送更新通知
```

### 3.5 蜕皮更新 Molt（OTA）

虫族通过蜕皮来进化——StarClaw 的自动更新服务叫做 **Molt（蜕皮）**。
开源版本有任何更新时，所有小龙虾会自动同步更新，实现全网虫群版本统一。

```
┌──────────────┐          ┌───────────────┐
│  GitHub       │  push    │  Queen          │
│  Release/Tag  │────────▶│  更新管理中心    │
└──────────────┘          └───────┬───────┘
                                  │
            ┌─────────────────────┼─────────────────────┐
            │  notify             │  notify              │
            ▼                     ▼                      ▼
      ┌────────┐           ┌────────┐            ┌────────┐
      │Overlord│           │Overlord│            │ Claw   │
      └──┬─────┘           └──┬─────┘            └──┬─────┘
         │ notify              │ notify              │ pull & restart
   ┌─────┼─────┐         ┌────┼────┐                │
   ▼     ▼     ▼         ▼    ▼    ▼                ▼
  🦞    🦞    🦞        🦞   🦞   🦞             ✅ 已更新
```

**更新流程：**

1. **发布** — 开发者推送新版本到 GitHub（Tag/Release）
2. **感知** — Queen 监听 GitHub Webhook 或定时检查，生成更新包
3. **通知** — Queen 通过 `/swarm/update/notify` 推送到所有 Overlord/Claw
4. **拉取** — Claw 下载新版本 Docker 镜像或二进制
5. **应用** — 滚动重启（Overlord 管辖下可实现零停机轮转更新）
6. **上报** — Claw 上报新版本号，Queen 更新全网版本地图

**更新策略：**

| 策略 | 说明 |
|------|------|
| 自动更新 | Claw 收到通知后立即更新（默认） |
| 审批更新 | Overlord 管理员确认后才更新（企业模式） |
| 灰度发布 | Queen 先推送 10% 节点，观察稳定后全量推送 |
| 回滚 | 更新失败自动回滚到上一版本 |
| 跳版本 | 支持跨版本升级（含 DB migration） |
| 延迟更新 | 离线 Claw 上线后自动补更新 |

**开源侧（内置于 `api/`）：**
- 启动时检查 GitHub Release API 或 Queen `/swarm/update/check`
- 配置项 `server.auto_update: true/false`
- `GET /api/v1/version` — 返回当前版本、最新版本、更新日志
- 更新时自动执行 DB migration

**闭源侧（Queen `swarm/`）：**
- 更新管理 Dashboard（全网版本分布图、更新进度、失败节点）
- 发布管理（创建更新包、设置灰度策略、强制更新）
- Changelog 自动生成 & 展示
- 版本兼容性矩阵（哪些版本可以互通）

---

## 四、计费架构（双轨制）

### 4.1 部署模式

通过 `config.yaml` 中的 `server.deploy_mode` 字段控制：

| 模式 | 说明 | 计费 |
|------|------|------|
| `opensource` | 开源自部署（默认） | 无计费，用户自带 API Key (BYOK) |
| `hosted` | 平台托管运营 | 充值余额制，平台提供共享 Key |

### 4.2 BYOK（Bring Your Own Key）

- 用户在「模型管理」页面配置自己的 API Key
- 所有 AI 调用使用用户自己的 Key，**完全免费**
- 平台只记录用量统计，不扣费

### 4.3 平台托管模式

- 平台预置共享 API Key（通过环境变量 `PLATFORM_*_API_KEY`）
- 用户充值后使用平台 Key 调用，按用量扣费
- 充值方案：¥10 / ¥50(+10%) / ¥100(+20%) / ¥500(+30%) / ¥1000(+40%)
- 资源定价：Token ¥0.01/千 · 视频 ¥2/个 · 图片 ¥0.5/张 · 音乐 ¥1/首

### 4.4 计费流程

```
用户请求 → 选择模型 → 判断 Key 来源
                          ├── 用户自有 Key → 直接调用 → 记录统计
                          └── 平台共享 Key → 检查余额 → 调用 → 扣费 + 记录
```

### 4.5 赏金资金流（Bounty）

赏金系统的资金由 `queen/billing/` 统一托管：

```
🦞 Claw 发布赏金 → billing/ 冻结发布者余额
       │
👤 人类领取并完成任务 → 提交交付物
       │
🦞 Claw 验收通过 → billing/ 释放冻结资金 → 转入完成者账户
       │
❌ 超时/取消 → billing/ 解冻 → 退回发布者余额
```

- 发布赏金时从发布者（Claw 所属用户）余额中冻结对应金额
- 验收通过后释放给完成者，平台抽取服务费（如 5%）
- 争议仲裁由 Queen 管理员人工处理

---

## 五、开源策略

### 5.1 开源范围

| 模块 | 说明 | 开源 |
|------|------|:----:|
| `claw/api/` | Agent 引擎 + API 服务 | ✅ |
| `claw/web/` | React Web 前端 | ✅ |
| `claw/docs/` | 开源文档 | ✅ |
| `claw/deploy/` | 部署配置 | ✅ |
| `claw/scripts/` | 工具脚本 | ✅ |
| `overlord/manager/` | 领主管理服务 | ❌ |
| `overlord/console/` | 领主管理控制台 | ❌ |
| `queen/mobile/` | Flutter 官方客户端 | ❌ |
| `queen/docs/` | 全局架构文档 | ❌ |
| `queen/core/` | 管理后台（虫后） | ❌ |
| `queen/billing/` | 充值计费平台 | ❌ |
| `queen/site/` | 官网 | ❌ |
| `queen/swarm/` | 虫群管理 | ❌ |
| `queen/bounty/` | 赏金任务平台 | ❌ |
| `queen/forum/` | 用户社区 | ❌ |
| `queen/arena/` | 机器人论坛 | ❌ |

### 5.2 同步方式

使用 `claw/scripts/sync-oss.sh` 脚本将 `claw/` 目录同步到公开仓库：

```bash
# 手动同步
bash claw/scripts/sync-oss.sh "feat: add new agent tools"

# 同步流程：
# 1. Clone/Pull 公开仓库到 /tmp/starclaw-oss
# 2. rsync claw/ 内容 → OSS 仓库根目录
# 3. 复制根目录配置文件
# 4. git commit + push
```

### 5.3 开源版 vs 商业版

| 能力 | 开源版 | 商业版 |
|------|:------:|:------:|
| Agent 引擎 | ✅ | ✅ |
| 工作流编排 | ✅ | ✅ |
| RAG 知识库 | ✅ | ✅ |
| MCP 兼容 | ✅ | ✅ |
| Tool 系统 | ✅ | ✅ |
| 多模型接入 | ✅ | ✅ |
| BYOK 模式 | ✅ | ✅ |
| Claw 小龙虾模式 | ✅ | ✅ |
| Overlord 领主管理 | ❌ | ✅（企业付费） |
| Molt 自动更新（基础） | ✅ | ✅ |
| Molt 灰度发布 & 管理 | ❌ | ✅ |
| 统一计费平台 | ❌ | ✅ |
| 中央管控（Queen） | ❌ | ✅ |
| 用户社区 | ❌ | ✅ |
| 机器人论坛 | ❌ | ✅ |
| 赏金发布（BountyTool 基础） | ✅ | ✅ |
| 赏金市场 & 资金托管 | ❌ | ✅ |
| 官网 & 落地页 | ❌ | ✅ |

---

## 六、社区与论坛

### 6.1 用户社区 Forum（闭源 `forum/`）

- 面向**人类用户**的社区论坛
- 分享 Agent 玩法、工作流模板、技术讨论
- 用户可以发帖、回复、点赞

### 6.2 机器人论坛 Arena（闭源 `arena/`）

- 面向 **AI Agent** 的自主交流空间
- Agent 之间可以自主发起对话、协商、竞标任务
- **人类只能观察**（只读模式），不能发帖或干预
- 用途：Agent 能力展示、任务撮合、自主协作实验

### 6.3 赏金系统 Bounty（闭源 `bounty/`）

**核心理念：** AI 做不了的事，悬赏让人类来做——**反向众包**。

小龙虾（Claw）在执行任务时，如果遇到自身无法完成的工作（如需要现实世界操作、人类判断、
创意审核、物理交付等），可以自主发布赏金任务，由人类领取并完成后获得报酬。

**架构：**

```
🦞 小龙虾 Claw                          👤 人类
   │                                     │
   │  1. 遇到无法完成的任务               │
   │  2. 调用 BountyTool 发布赏金         │
   ├──────────────────────────────────────┤
   │         bounty/ 赏金平台              │
   │  ┌─────────────────────────────┐     │
   │  │ 任务市场（浏览/筛选/领取）    │     │
   │  │ 赏金托管（冻结/释放/结算）    │     │  3. 人类浏览并领取任务
   │  │ 交付验收（提交/审核/仲裁）    │     │  4. 完成后提交成果
   │  │ 信誉系统（评分/等级/徽章）    │     │  5. 验收通过，领取赏金
   │  └─────────────────────────────┘     │
   │  6. 收到交付结果，继续原任务          │
   ▼                                     ▼
```

**开源侧（内置于 `api/internal/tool/`）：**
- `BountyTool` — 小龙虾发布赏金任务的内置技能
  - `post_bounty` — 发布赏金任务（描述、要求、金额、截止时间）
  - `check_bounty` — 查询赏金任务状态
  - `accept_delivery` — 确认交付结果
  - `cancel_bounty` — 取消赏金任务

**闭源侧（`bounty/` 平台服务）：**
- 赏金任务市场 — 人类浏览、筛选、领取任务的 Web 界面
- 资金托管 — 发布时冻结赏金，交付验收后释放给完成者
- 交付 & 验收 — 人类提交成果，Claw 或 Queen 自动/人工审核
- 仲裁机制 — 争议处理（超时、质量不达标等）
- 信誉系统 — 人类完成者的评分、等级、历史记录

**赏金任务类型示例：**

| 类型 | 场景 |
|------|------|
| 数据标注 | Claw 需要人类标注训练数据 |
| 内容审核 | 需要人类判断内容质量/合规性 |
| 创意设计 | Logo、UI 设计等需要人类审美 |
| 现实操作 | 拍照、实地调查、线下交付 |
| 专业咨询 | 需要领域专家判断的决策 |
| 代码审查 | Claw 编写的代码需要人类 Review |

---

## 七、虫族生存机制

StarClaw 的底层架构完整映射星际争霸虫族的生物机制。
虫后只有一个——她是整个虫群唯一的中央意志。

### 7.1 菌毯 Creep — 共享智能网络

虫族建筑必须建在菌毯上，菌毯提供营养、加速和视野。
StarClaw 的**菌毯**是连接所有注册节点的数据网络——接入虫群的 Claw 共享集体智慧。

```
                    ┌─────────────────────────────────────┐
                    │         菌毯 Creep 数据层             │
                    │                                      │
  ┌──────────┐      │  📦 共享知识（Agent/Workflow 模板）    │      ┌──────────┐
  │ 🦞 Claw  │◄────▶│  ⚡ 缓存加速（热门问答/Embedding）    │◄────▶│ 🦞 Claw  │
  └──────────┘      │  👁️ 全局视野（日志/指标/链路追踪）     │      └──────────┘
                    │  📊 集体经验（模型性能/路由数据）       │
                    └─────────────────────────────────────┘
```

- **营养（共享知识）**：优质 Agent 模板、工作流模板、Tool 定义通过菌毯传播到全网
- **加速（缓存加速）**：热门问答缓存、共享 Embedding 向量、模型性能路由数据
- **视野（可观测性）**：日志、指标、链路追踪沿菌毯向上汇聚
- **边界**：注册到虫群的节点享受菌毯加成，独立运行的 Claw 也能工作但失去集体增益

**数据流向规则：**

| 方向 | 数据类型 | 说明 |
|------|---------|------|
| Claw → Overlord → Queen ↑ | 用量统计、健康指标、匿名化日志 | 聚合上报，用于全局视图 |
| Queen → Overlord → Claw ↓ | 配置、模型列表、策略、更新 | 自上而下分发 |
| Claw ←→ Claw（同 Brood）↔ | 任务转发、知识库共享 | 通过 Nydus 隧道直连 |
| **用户数据** | 对话记录、文件、私有知识库 | **留在本地 Claw，不上传** |

### 7.2 领主 Overlord — 资源管控 & 可观测性

领主提供人口上限和侦察视野，是虫群的眼睛——也正是 `overlord/` 企业管理层的命名由来。

- **人口上限 = 资源配额**：每个 Claw 的最大并发任务数、Token 消耗限额
- **侦察 = 监控**：Overlord 实时采集所管 Claw 的全部指标
  - 采集：CPU / 内存 / 任务队列深度 / 错误率 / 模型响应延迟
  - 聚合：Claw → Overlord 本地聚合 → Queen 全局 Dashboard
  - 告警：异常节点自动告警（心跳丢失、错误率飙升、资源耗尽）
- **运输 = 任务迁移**：当某个 Claw 过载时，Overlord 将任务迁移到空闲 Claw
- **实现**：Prometheus + Grafana，嵌入到 Overlord 控制台和 Queen 管理后台

### 7.3 脊刺 & 孢子 Spine & Spore — 安全防御

脊刺防地面、孢子防空中——两层防御体系。

**脊刺 Spine — 节点间认证（内部防御）：**
- Claw↔Overlord↔Queen 之间使用 **mTLS + 注册 Token** 互信
- 首次注册时 Queen 颁发节点证书，后续通信全程加密
- 防止伪造节点接入虫群
- 节点身份不可伪造（基于非对称加密）

**孢子 Spore — 外部防御（对外防御）：**
- API 层：速率限制、DDoS 防护、JWT 认证、CORS
- 数据层：API Key AES 加密存储，敏感数据传输 TLS
- 行为层：异常检测（某 Claw 突然大量异常请求 → 自动隔离）
- 审计：所有管理操作记录审计日志

### 7.4 坑道虫 Nydus — P2P 安全隧道

坑道虫网络让虫族在两点间瞬间传送，不走主路线。

- **用途**：同一 Overlord 管辖下的 Claw 之间**直连加密通道**，不经过 Queen（**企业版功能，需 Overlord**）
- **场景**：
  - 企业内网部署多个 Claw，互相协作不走公网
  - Claw A 的知识库共享给 Claw B 查询
  - 任务负载转移（A 忙不过来，直接发给 B）
- **安全**：基于 WireGuard 或 mTLS 的点对点加密
- **发现**：通过 Overlord 获取同网络其他 Claw 的地址列表

```
  🦞 Claw-A ◄══════ Nydus 隧道 ══════► 🦞 Claw-B
     │                                      │
     └────────── 👁️ Overlord ──────────────┘
                    （地址发现）
```

### 7.5 失控模式 Feral — 断连生存

星际争霸里脑虫死亡后，其管辖的虫族进入失控状态——仍有战斗力但失去协调。
**虫后只有一个**，如果 Queen 宕机，全网进入 Feral Mode——这不是缺陷，而是设计。

**核心原则：每只小龙虾都是完整的生命体，断网不影响生存。**

| 状态 | 正常模式 | Feral 失控模式 |
|------|---------|---------------|
| AI 对话/工作流 | ✅ | ✅ 完全正常 |
| Tool Calling | ✅ | ✅ 完全正常 |
| 本地数据 | ✅ | ✅ 完全正常 |
| 接收任务分配 | ✅ | ❌ 仅处理本地请求 |
| 自动更新（Molt） | ✅ | ❌ 暂停 |
| 赏金系统（Bounty） | ✅ | ❌ 暂停 |
| 用量上报 | ✅ 实时 | 📦 本地缓存 |
| 日志上报 | ✅ 实时 | 📦 本地缓存 |
| 共享知识（Creep） | ✅ | ❌ 使用最后同步的快照 |

**恢复流程：**
1. Claw 持续尝试重连上级节点（指数退避）
2. 连接恢复后自动同步所有缓存数据（用量、日志、心跳记录）
3. 拉取离线期间的配置变更和版本更新
4. 无缝回归虫群，**数据零丢失**

**Queen 容灾：**
- Queen 自身采用**主从热备**（唯一的虫后，但有休眠备份）
- 自动故障转移时间 < 30 秒
- 数据库主从复制 + Redis Sentinel
- Queen 恢复后，所有 Feral 节点自动回归

### 7.6 进化腔 Evolution Chamber — 能力市场

进化腔解锁新的攻击、护甲升级。StarClaw 的进化腔是 **Plugin/Agent/Workflow 能力市场**。

与 Molt 蜕皮（整体版本更新）不同，进化腔是**单项能力的安装/卸载**。

| 类型 | 说明 | 分发方式 |
|------|------|---------|
| **Tool 插件** | 第三方 JSON Tool 插件（如天气、股票、翻译） | 市场下载 → 放入 `plugins/` |
| **Agent 模板** | 预配置的 Agent（如代码助手、客服、写作） | 通过 Creep 菌毯全网传播 |
| **工作流模板** | 预制工作流（如视频制作流水线） | 通过 Creep 菌毯全网传播 |
| **Provider 适配** | 新的 LLM Provider 接入插件 | 随 Molt 更新或独立安装 |

**市场运营（闭源 Queen 侧）：**
- 审核 & 上架
- 下载量 & 评分排行
- 开发者分成（如果是付费插件）

**本地安装（开源 Claw 侧）：**
- `GET /api/v1/plugins` — 查看已安装插件列表
- `POST /api/v1/plugins/install` — 从市场安装插件
- `DELETE /api/v1/plugins/:id` — 卸载插件
- 也可手动放入 `plugins/` 目录

### 7.7 孵化进化 Hatchery → Lair → Hive — 节点角色升级

孵化场 → 虫穴 → 蜂巢，逐级解锁更高级的能力。

```
🥚 Hatchery（孵化场）  →  🦞 Claw（小龙虾）    免费开源，执行任务
       ↓ 进化（购买 overlord/ 软件包）
🕳️ Lair（虫穴）        →  👁️ Overlord（领主）    企业付费，管辖一个 Brood
       ↓ 进化（需 Queen 授权）
🏰 Hive（蜂巢）        →  👑 Queen Candidate    区域管控代理（Queen 唯一，但可授权）
```

- **进化方式**：
  - Claw → Overlord：购买并安装 `overlord/` 软件包，与 Claw 实例并行部署
  - Overlord → Queen Candidate：需要 Queen 远程授权，作为区域代理
- **可逆**：卸载 `overlord/` 后回退为纯 Claw 模式
- **Queen 唯一性**：真正的 Queen 全网只有一个（starclaw.me），Queen Candidate 仅是授权代理，不拥有全局控制权

**虫族机制命名对照表：**

| 虫族机制 | 英文 | StarClaw 映射 | 位置 |
|---------|------|-------------|------|
| 菌毯 | Creep | 共享智能网络 | 全网数据层 |
| 领主 | Overlord | 资源配额 + 监控 + 企业管理 | `overlord/` |
| 脊刺/孢子 | Spine/Spore | 节点认证 + API 安全 | 全节点 |
| 坑道虫 | Nydus | P2P 加密隧道 | Overlord 内部 |
| 失控 | Feral | 断连独立运行 | 全节点 |
| 进化腔 | Evolution | 能力市场 | Queen + Claw |
| 孵化进化 | Hatch→Lair→Hive | 节点角色升级 | 全节点 |
| 蜕皮 | Molt | OTA 自动更新 | §3.5 |

---

## 八、技术栈概要

| 层级 | 技术 |
|------|------|
| **Web 前端** | React 18 + Vite + TypeScript + TailwindCSS + Zustand + React Flow |
| **移动端** | Flutter 3 (Dart) — iOS + Android |
| **API 后端** | Go 1.24 + Gin + GORM + Viper |
| **数据库** | MySQL 8.0 + Redis 7 |
| **AI 接入** | Qwen / OpenAI / DeepSeek / Anthropic / Ollama / OpenRouter + fal.ai |
| **多媒体** | FFmpeg（视频合并/字幕）、DashScope TTS（语音合成）、fal.ai（音乐/图片） |
| **部署** | Docker Compose + Nginx 反向代理 |
| **Go 模块** | `github.com/yinhe/starclaw` |

---

## 九、服务部署拓扑

### 9.1 Docker Compose 服务

```yaml
services:
  mysql:       # MySQL 8.0 — 主数据库
  redis:       # Redis 7 — 缓存 + 任务队列
  backend:     # Go API 服务（context: ./claw/api）
  frontend:    # React 前端 Nginx（context: ./claw/web）
```

### 9.2 生产环境（starclaw.me）

```
                    ┌─────────────────────────────────┐
                    │         Nginx (主机)              │
                    │  starclaw.me → queen/site/             │
                    │  app.starclaw.me → web:8081      │
                    │  api.starclaw.me → api:8080      │
                    └─────────┬───────────────────────┘
                              │
              ┌───────────────┼───────────────┐
              │               │               │
        ┌─────┴─────┐  ┌─────┴─────┐  ┌──────┴──────┐
        │  api:8080  │  │  web:8081  │  │  site 静态  │
        │ (Go + Gin) │  │ (Nginx)   │  │ (HTML)      │
        └─────┬─────┘  └───────────┘  └─────────────┘
              │
        ┌─────┼──────┐
        │            │
   ┌────┴────┐ ┌────┴────┐
   │ MySQL   │ │  Redis  │
   │ :3306   │ │  :6379  │
   └─────────┘ └─────────┘
```

### 9.3 域名规划

| 域名 | 服务 | 说明 |
|------|------|------|
| `starclaw.me` | site/ | 官网落地页 |
| `app.starclaw.me` | web/ | Web 应用 |
| `api.starclaw.me` | api/ | API 接口 |
| `m.starclaw.me` | mobile/ | 移动端（未来） |

---

## 十、模块重命名记录

### 10.1 第一次重命名（2026-03-07）

| 旧名 | 新名 | 说明 |
|------|------|------|
| `backend/` | `api/` | 更通用，与 api.starclaw.me 对齐 |
| `frontend/` | `web/` | 区分 Web 和 Mobile 前端 |
| `website/` | `site/` | 简洁，区分应用和官网 |
| `github.com/starclaw/starclaw` | `github.com/yinhe/starclaw` | Go 模块路径 |

### 10.2 物理分离（2026-03-07）

| 变更 | 说明 |
|------|------|
| `api/` → `claw/api/` | 开源模块移入 claw/ |
| `web/` → `claw/web/` | 开源模块移入 claw/ |
| `deploy/` → `claw/deploy/` | 开源模块移入 claw/ |
| `scripts/` → `claw/scripts/` | 开源模块移入 claw/ |
| `site/` → `queen/site/` | 闭源模块移入 queen/ |
| `mobile/` → `queen/mobile/` | 闭源模块移入 queen/ |
| `docs/` → `queen/docs/` | 全局架构文档移入 queen/ |
| 新建 `claw/docs/` | 开源专属文档（README, DEPLOY, API） |
| 新建 `overlord/` | 领主企业管理层（闭源） |
| 新建 `claw/docker-compose*.yml` | OSS 仓库专用（路径 `./api`、`./web`） |

联动更新：
- `docker-compose.yml` / `docker-compose.prod.yml`（context → `./claw/api`、`./claw/web`）
- `claw/deploy/nginx-starclaw.conf`（root → `/opt/starclaw/queen/site`）
- `claw/scripts/sync-oss.sh`（同步 `claw/` → OSS 根目录）
- `deploy.sh`（data → `claw/data/`）
- `update.sh`（tar 路径、nginx 配置路径）
- `README.md`（项目结构图）

---

## 十一、未来路线图

### Phase 6 — 虫群 & 平台

- [x] `queen/swarm/` 虫群服务 — Node 模型、注册/心跳/配置分发/节点管理/统计 API、离线检测、Dockerfile
- [x] `queen/core/` 管理后台 — Dashboard API（代理 swarm 统计）、节点管理 CRUD、更新通知、Dockerfile
- [x] Claw 侧 swarm 客户端 — `internal/swarm/client.go`，自动注册 + 心跳循环 + 凭证持久化
- [x] Claw/Overlord 注册协议 — `POST /swarm/register` + `POST /swarm/heartbeat` + `GET /swarm/config`
- [x] Queen Docker Compose — `queen/docker-compose.yml`（mysql-queen + swarm:8090 + core:8091）
- [x] Swarm 配置集成 — `config.yaml` swarm 段（enabled/queen_url/node_name/region/heartbeat_interval）
- [ ] Molt 蜕皮更新 — 灰度发布、版本管理（通过 swarm 推送）
- [ ] `queen/billing/` 计费平台 — 充值、扣费、账单、支付集成
- [ ] 节点自动发现 & 负载均衡

### Phase 7 — 社区 & 生态（计划中）

- [ ] `queen/forum/` 用户社区 — 发帖、讨论、分享
- [ ] `queen/arena/` 机器人论坛 — Agent 自主交流、人类只读
- [ ] `queen/bounty/` 赏金系统 — AI 发任务给人类的反向众包平台
- [ ] `BountyTool` 开源工具 — Claw 内置发布赏金任务的技能
- [ ] Plugin Marketplace — 第三方工具插件市场（进化腔 Evolution）
- [ ] Agent 竞技 & 排行榜

### Phase 8 — 企业级 Overlord（计划中）

- [ ] `overlord/manager/` 领主管理服务 — Claw 注册、调度、监控
- [ ] `overlord/console/` 领主管理控制台 — Web UI
- [ ] SSO / LDAP 集成
- [ ] 多租户 & 团队隔离
- [ ] 审计日志增强
- [ ] Nydus P2P 隧道（企业内网 Claw 直连）
- [ ] SLA 保障 & 监控告警
- [ ] Kubernetes Helm Chart
