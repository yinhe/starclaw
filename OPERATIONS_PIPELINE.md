# StarClaw 运营全链路方案

> 版本: v1.0 | 日期: 2026-03-19

---

## 一、系统架构总览

### 虫群层级

```
                    👑 Queen（虫后）
               starclaw.net 中央管控
          充值/Credit/合伙人/结算/监控(Overseer)
                    │
        ┌───────────┼───────────┐
        ▼                       ▼
  👁️ Overlord（领主）       🦞 Claw（直连 Queen）
  企业 AI 管控平台          个人/小团队用户
  节点编排/RBAC/SSO
  计费/预算/审计
        │
   🦞🦞🦞 Claw（企业员工）
   通过 Overlord 管辖
```

### 数据流

```
┌─────────────┐     ┌──────────────┐     ┌───────────────┐
│   Claw 节点  │────▶│ Synapse/StarAI│────▶│  上游 LLM API  │
│  (用户端)    │◀────│  (star-ai.net)│◀────│  (OpenAI 等)   │
└──────┬──────┘     └──────┬───────┘     └───────────────┘
       │                   │
       │  Swarm 注册       │  Credit 消费
       ▼                   ▼
┌──────────────────────────────────────────┐
│              Queen (管理平台)              │
│  ┌────────┐ ┌────────┐ ┌──────────────┐ │
│  │ 充值/支付│ │ Credit │ │ 合伙人/结算  │ │
│  │billing  │ │credit  │ │partner/city  │ │
│  └────────┘ └────────┘ └──────────────┘ │
│  ┌────────┐ ┌────────┐ ┌──────────────┐ │
│  │用户管理 │ │节点绑定 │ │  Overseer    │ │
│  │auth/user│ │binding │ │  监控中心     │ │
│  └────────┘ └────────┘ └──────────────┘ │
└──────────────────┬───────────────────────┘
                   │
┌──────────────────▼───────────────────────┐
│          Overlord（领主/企业管控）         │
│  ┌────────┐ ┌────────┐ ┌──────────────┐ │
│  │节点编排 │ │多租户   │ │ 订阅计费     │ │
│  │registry │ │RBAC    │ │ billing      │ │
│  └────────┘ └────────┘ └──────────────┘ │
│  ┌────────┐ ┌────────┐ ┌──────────────┐ │
│  │SSO集成  │ │Molt更新 │ │ Webhook     │ │
│  │OAuth/LDAP│ │审批/滚动│ │ 事件通知    │ │
│  └────────┘ └────────┘ └──────────────┘ │
│  ┌──────────────┐  ┌──────────────────┐ │
│  │ 管理控制台    │  │ 员工工作台        │ │
│  │ console:3095 │  │ web:3096          │ │
│  └──────────────┘  └──────────────────┘ │
└──────────────────────────────────────────┘
```

---

## 二、全链路流程图

```
用户安装 Claw ──→ 注册/绑定 Queen ──→ 充值 ──→ 对话消费 ──→ 合伙人分润 ──→ 结算 ──→ 监控
     │                  │               │          │             │            │         │
     ▼                  ▼               ▼          ▼             ▼            ▼         ▼
  邀请码归属        节点绑定         支付宝/微信   Star Energy   城市/核心     月度账单   Overseer
  (ref_code)     (NodeBinding)     充值到余额    按token扣费    佣金自动计算  审批/打款   仪表盘
```

---

## 三、现状审计

### 3.1 已完成模块（可直接使用）

| 环节 | 模块文件 | 说明 |
|------|----------|------|
| Queen 充值 | `billing.go` | 支付宝 + 微信支付 → 用户余额 → Star Energy 自动到账 |
| 充值套餐 | `billing.go` SeedDefaultPackages | 体验包 ¥10 ~ 专业包 ¥1000，含赠送比例 |
| Star Energy 发放 | `billing.go` grantStarEnergy() | 充值完成后自动将星能发放到绑定的 Claw 节点 |
| StarAI Gateway 按次扣费 | `gateway.go` calculateAndBill() | 按 token 数量 × 模型单价计费，从 Queen 用户余额扣除 |
| Synapse 消费 Star Energy | `queen_credit.go` Consume() | Synapse 每次 API 调用后通知 Queen 扣除星能 |
| Queen Credit 系统 | `credit.go` | 完整的余额/消费/冻结/结算/转账功能 |
| 节点绑定 | `node_binding.go` | Claw 节点 ↔ Queen 用户双向绑定，支持内部 API |
| 城市合伙人 | `city.go` | 申请/审核/客户管理/佣金计算/Dashboard |
| 核心合伙人 | `partner.go` | Dashboard/股权分润/城市合伙人管理 |
| 充值时佣金生成 | `billing.go` generateCityCommission() | 充值完成后自动计算城市合伙人佣金 |
| 注册邀请码 | `auth.go` ref_code | 注册时传入 ref_code → 自动归属城市合伙人 |
| 结算引擎 | `settlement.go` | 月度账单生成（核心+城市合伙人），审批/打款流程 |
| Overseer 监控 | `overseer/` | 节点在线、服务健康、星能消费趋势、告警 |
| 运营分析 | `admin_analytics.go` | GMV/MRR/客户总览/合伙人绩效 |

### 3.2 缺失的关键链路

| 编号 | 缺失环节 | 影响 | 优先级 |
|------|----------|------|--------|
| G1 | Claw 安装/启动时没有传递邀请码 | 安装用户无法归属到合伙人 | P0 |
| G2 | Claw 没有自动注册 + 绑定 Queen 的一键流程 | 需手动到网站注册再绑定，断裂 | P0 |
| G3 | Claw 对话界面不显示每次消费金额 | 用户不知道花了多少钱 | P1 |
| G4 | 合伙人看不到下线用户的消费活跃度 | 无法评估客户价值 | P2 |
| G5 | 核心合伙人分润计算逻辑需确认 | 分润基数和比例待明确 | P2 |

---

## 四、链路详细设计

### 4.1 链路 1：用户归属（邀请码机制）

#### 流程

```
合伙人分享下载链接                      下载页                    Claw 安装器
starclaw.me/download?ref=ABC123 ──→ 页面记住 ref param ──→ Setup 页面显示邀请码框
                                                                    │
                                                          写入 config.yaml
                                                          ref_code: ABC123
                                                                    │
                                                                    ▼
                                                          Claw 首次启动
                                                          自动注册 Queen 账号
                                                          传入 ref_code
                                                                    │
                                                                    ▼
                                                          Queen auth.go Register
                                                          ref_code → CityClient
                                                          自动归属合伙人 ABC123
```

#### 邀请码格式

- 城市合伙人：`city_` + 6位随机码，如 `city_A3B7X9`
- 核心合伙人：`core_` + 6位随机码，如 `core_K2M8P1`
- 通用推广码：6位随机码，如 `A3B7X9`（默认归属城市合伙人）

#### 涉及改动

| 文件 | 改动内容 |
|------|----------|
| `spore/cmd/setup/embed/wizard.html` | 添加"邀请码"输入框（可选） |
| `spore/cmd/setup/gui.go` | 将邀请码写入 claw config.yaml |
| `claw/api/internal/config/` | config 增加 `ref_code` 字段 |
| `claw/api` 启动逻辑 | 首次启动检测，引导注册 Queen |
| `queen/site/src/pages/DownloadPage.tsx` | URL `?ref=xxx` 传递到安装包下载 |

### 4.2 链路 2：Claw 自动注册 + 绑定 Queen

#### 方案：Claw 签名认证（推荐）

复用 Claw 的 Ed25519 身份体系，无需用户记密码：

```
Claw 首次启动
    │
    ├─ 检测是否已绑定 Queen（~/.spore/queen_binding.json）
    │
    ├─ 未绑定 → Claw Web UI 显示引导面板：
    │     "绑定 Queen 账号以使用 StarAI 算力"
    │     [一键绑定] ← 用 Claw Ed25519 签名自动注册
    │     [输入邀请码] ← 可选
    │
    └─ 绑定流程：
          Claw → POST queen/api/v1/auth/claw-register
          body: { node_id, public_key, signature, ref_code?, username? }
          Queen 验证签名 → 创建用户 → 创建 NodeBinding → 返回 JWT
          Claw 保存 queen_token 到本地
```

#### 涉及改动

| 文件 | 改动内容 |
|------|----------|
| `queen/api/internal/handler/claw_auth.go` | 新增 `/auth/claw-register` 端点 |
| `claw/api/internal/overlord/client.go` | 启动时检测并自动绑定 Queen |
| `claw/web/src/pages/` | 新增 Queen 绑定引导 UI |

### 4.3 链路 3：充值 → Star Energy → 对话消费

#### 充值流程（已完成）

```
用户在 starclaw.net 充值
    │
    ├──→ 支付宝/微信 支付回调
    │
    ├──→ billing.go completeOrder()
    │       ├─ UserBalance += 充值金额 + 赠送金额
    │       ├─ BalanceTransaction (type: recharge)
    │       ├─ grantStarEnergy() → CreditAccount.balance +=
    │       └─ generateCityCommission() → Commission 记录
    │
    └──→ 完成
```

#### 对话消费流程

```
用户在 Claw 发送消息
    │
    ├─ Claw inference 选择模型
    │
    ├─ 如果走 StarAI (star-ai.net)：
    │     Claw → POST star-ai.net/v1/chat/completions
    │     │       (带 Ed25519 签名 或 API Key)
    │     │
    │     ├─ Synapse 鉴权 → 识别 claw_id / user_id
    │     ├─ Synapse → upstream LLM API (OpenAI/Qwen/Claude...)
    │     ├─ Synapse 收到响应 + usage{prompt_tokens, completion_tokens}
    │     │
    │     ├─ [方式 A] Claw 签名用户：
    │     │     Synapse → Queen /internal/credits/consume
    │     │     扣除 Star Energy，返回 {deducted, balance}
    │     │
    │     ├─ [方式 B] API Key 用户 (star-ai.net 网页)：
    │     │     Queen gateway.go calculateAndBill()
    │     │     直接扣除 Queen UserBalance
    │     │
    │     └─ 返回响应给 Claw（含 usage 信息）
    │
    ├─ ★ 新增：Claw 解析 usage，显示消费：
    │     "本次消费 0.3⚡ (约 ¥0.003) | 剩余 986.7⚡"
    │
    └─ 如果用本地模型 / 自有 API Key：
          不走 StarAI，不扣费
```

#### 计费公式

```
消费（分）= prompt_tokens × 输入单价/1M + completion_tokens × 输出单价/1M

Star Energy 换算：
  1 分（¥0.01）= 1 Star = 10,000 内部单位

示例（qwen-turbo, 1000 tokens 对话）：
  输入 500 tokens × 10分/1M = 0.005 分
  输出 500 tokens × 20分/1M = 0.01 分
  合计 ≈ 0.015 分 → 最低扣 1 分 = 0.1⚡
```

### 4.4 链路 4：合伙人分润

#### 4.4.1 城市合伙人

```
佣金来源：下线用户充值金额 × 佣金比例（默认 20%）

用户充值 ¥100
    │
    └─ billing.go completeOrder()
         └─ generateCityCommission()
              ├─ 查找用户 → CityClient → 所属合伙人
              ├─ 佣金 = ¥100 × 20% = ¥20
              └─ 写入 Commission 表 (status: pending)

月底结算：
    Admin → POST /settlement/generate { month: "2026-03" }
    → 汇总该月所有 Commission
    → 生成 SettlementBill
    → Admin 审批 → 打款
```

#### 4.4.2 核心合伙人

```
分润来源：平台月净收入 × 股权比例（equity_ratio）

月净收入 = 总充值额 - 上游 API 成本 - 运营成本
核心合伙人 A (equity 30%) → 分润 = 净收入 × 30%
核心合伙人 B (equity 20%) → 分润 = 净收入 × 20%

月底结算：
    settlement.go generateCorePartnerBill()
    → 按 equity_ratio 生成账单
    → Admin 审批 → 打款
```

#### 4.4.3 分润层级关系

```
平台月收入 ¥100,000
    │
    ├─ 上游 API 成本 ¥30,000 (30%)
    │
    ├─ 城市合伙人佣金 ¥14,000 (充值额 × 20%)
    │
    ├─ 平台运营成本 ¥10,000
    │
    └─ 净利润 ¥46,000
         ├─ 核心合伙人 A (30%) → ¥13,800
         ├─ 核心合伙人 B (20%) → ¥9,200
         └─ 平台留存 (50%) → ¥23,000
```

### 4.5 链路 5：监控闭环（Overseer）

```
Overseer 仪表盘
    │
    ├── 实时监控
    │     ├─ 节点在线状态（Swarm heartbeat，30s 间隔）
    │     ├─ 服务健康（Queen/Synapse/Forum/Bounty/Arena ping）
    │     └─ 告警（节点离线 / 服务异常 / 余额异常）
    │
    ├── 星能监控
    │     ├─ 总发放量 / 总消耗量 / 留存量
    │     ├─ 日消耗趋势
    │     └─ 低余额节点预警
    │
    └── 运营数据（Admin Analytics）
          ├─ GMV（总充值额）
          ├─ 净收入 = GMV - 上游成本
          ├─ MRR / ARR
          ├─ 活跃用户数 / 新增用户数
          ├─ ARPU（每用户平均收入）
          ├─ 用户留存率
          └─ 合伙人绩效排名
```

### 4.6 链路 6：Overlord（领主）— 企业 AI 管控平台

#### 4.6.1 定位

Overlord 是虫群架构的**中间管理层**，面向**企业客户**。个人用户的 Claw 直连 Queen，企业用户的 Claw 通过 Overlord 管辖。

```
个人用户:  Claw ──→ Queen（直连，通过 Swarm）
企业用户:  Claw ──→ Overlord ──→ Queen（领主代管）
```

#### 4.6.2 模块清单（已完成）

| 模块 | 端点数 | 功能 |
|------|:------:|------|
| 节点管理 (registry) | 8 | Claw 注册/心跳/配额/调度/解析/审计 |
| 多租户 RBAC (team) | 8 | 团队 CRUD + 管理员 CRUD + 4 级角色 |
| Nydus 隧道 (nydus) | 6 | TCP/UDP 正反向隧道管理 |
| Molt 更新 (molt) | 6 | 版本提交 → 审批 → 滚动更新 → 自动熔断 |
| Webhook (webhook) | 6 | HMAC 签名投递 + 事件驱动 |
| 订阅计费 (billing) | 18 | 套餐/订阅/用量统计/预算告警/概览 |
| SSO 集成 (sso) | 10 | OAuth2/OIDC + LDAP + 自动用户配置 |

**总计 60+ API 端点，17 张数据表。**

#### 4.6.3 前端

| 界面 | 端口 | 用户 | 页面 |
|------|:----:|------|------|
| **管理控制台** (console) | :3095 | 企业 IT 管理员 | 总览/节点/团队/隧道/Molt/Webhook/计费/分析/审计/解析（12 页） |
| **员工工作台** (web) | :3096 | 企业员工 | AI 对话/Agent 市场/工具集/个人中心（5 页） |

#### 4.6.4 订阅套餐（Overlord 商业模式）

| 版本 | 月付 | 节点上限 | 团队 | 核心特性 |
|------|:----:|:-------:|:----:|---------|
| **Community** | 免费 | ≤10 | 1 | 基础用量统计 |
| **Starter** | ¥499 | ≤20 | 3 | + 预算告警 |
| **Pro** | ¥1,999 | ≤100 | 不限 | + SSO + 审计日志 + 高级分析 |
| **Enterprise** | ¥4,999 | ≤500 | 不限 | + 合规面板 + SLA 99.9% |
| **White-Label** | ¥9,999+ | 不限 | 不限 | + 品牌定制 + 自定义域名 |

#### 4.6.5 RBAC 角色

| 角色 | 权限范围 |
|------|----------|
| `superadmin` | 全部（`*`） |
| `admin` | 节点/团队/隧道/Molt/Webhook/审计/统计/计费读写 |
| `operator` | 节点读写/隧道/Molt 审批/审计只读/计费只读 |
| `viewer` | 全部只读 |

#### 4.6.6 Overlord 在运营链路中的位置

```
                     ┌─── 个人用户 ───┐
                     │                │
                   Claw ──→ Queen ──→ 充值/消费/合伙人分润
                     │
                     └─── 企业用户 ───┐
                                      │
              企业管理员部署 Overlord ──→ 管理控制台
                     │                    ├─ 节点编排：管理 N 个 Claw
                     │                    ├─ 团队隔离：部门/项目组
                     │                    ├─ 预算告警：月用量 > 阈值时通知
                     │                    ├─ SSO：企业微信/LDAP 一键登录
                     │                    └─ 审计日志：谁在何时做了什么
                     │
              企业员工使用 Claw ──→ 员工工作台
                     │                ├─ AI 对话（Agent 选择 + 多轮对话）
                     │                ├─ Agent 市场（模板浏览/搜索）
                     │                └─ 工具集（MCP 工具目录）
                     │
              Overlord ──→ Queen（上报节点 + 消费星能）
                     │
              Queen ──→ 收取企业订阅费 + 星能消费费
```

#### 4.6.7 Overlord 收入模型

```
Overlord 为平台带来两层收入：

  1. 订阅费（SaaS）
     企业按节点数量/功能等级 → 月付 ¥499 ~ ¥9,999+
     → 收入归平台（核心合伙人可分润）

  2. 星能消费费（按量）
     企业员工通过 Claw 调用 AI → 消耗星能
     → 与个人用户走同一条 Queen Credit 扣费链路
     → 城市合伙人可从企业客户充值中获得佣金

  3. 增值服务
     私有部署实施 / 定制开发 / 培训 / 技术支持
     → 一次性收费或年度服务合同
```

#### 4.6.8 数据表（17 表）

| 分类 | 表名 | 说明 |
|------|------|------|
| 节点 | claw_nodes | Claw 节点注册信息 + 指标 |
| 节点 | task_assignments | 任务分配记录 |
| 节点 | audit_logs | 操作审计日志 |
| 团队 | teams | 多租户团队 |
| 团队 | admin_users | 管理员/用户账号 |
| 隧道 | nydus_tunnels | Nydus 隧道实例 |
| 更新 | molt_releases | 版本发布 |
| 更新 | molt_node_statuses | 节点更新状态 |
| 通知 | webhooks | Webhook 配置 |
| 通知 | webhook_logs | 投递日志 |
| 计费 | plans | 订阅套餐定义 |
| 计费 | subscriptions | 团队订阅关系 |
| 计费 | usage_records | 逐条用量记录 |
| 计费 | usage_daily_summaries | 每日汇总 |
| 计费 | budget_alerts | 预算告警规则 |
| SSO | sso_providers | 身份提供商配置 |
| SSO | sso_sessions | SSO 登录会话 |

#### 4.6.9 部署信息

| 项目 | 值 |
|------|------|
| 代码位置 | `e:\starclaw\overlord\` |
| 后端 API | Go 1.24 + Gin + GORM + MySQL 8.0 (:8095) |
| 管理控制台 | React 18 + Vite 6 + TailwindCSS (:3095) |
| 员工工作台 | React 18 + Vite 6 + TailwindCSS (:3096) |
| 生产 Compose | `docker-compose.prod.yml` |
| 默认管理员 | admin / admin123 |

---

## 五、Claw 绑定机制设计

### 5.1 双轨制：邀请码 + 自动绑定

| 场景 | 机制 | 说明 |
|------|------|------|
| 合伙人推广 | 分享链接带 `?ref=ABC123` | 下载页/安装器自动填充邀请码 |
| 直接安装 | 无邀请码 | Claw 设置页可以后补邀请码 |
| Claw ↔ Queen 绑定 | Ed25519 签名认证 | 无需密码，Claw 签名即身份 |
| 合伙人归属 | 注册时 ref_code | 自动创建 CityClient，归属合伙人 |
| 多节点 | 一个 Queen 用户可绑定多个 Claw | NodeBinding 一对多 |

### 5.2 绑定状态机

```
未绑定 ──[一键注册/签名登录]──→ 已绑定 (active)
                                    │
                              [用户主动解绑]
                                    │
                                    ▼
                              已解绑 (revoked)
                                    │
                              [重新绑定]
                                    │
                                    ▼
                              已绑定 (active)
```

---

## 六、数据模型关系图

```
User (Queen)
  │
  ├─── UserBalance          # ¥ 余额（充值/消费）
  │       └── BalanceTransaction   # 余额变动记录
  │
  ├─── NodeBinding[]        # 绑定的 Claw 节点（1:N）
  │       └── node_id → CreditAccount
  │                   └── CreditTransaction  # 星能变动记录
  │
  ├─── APIKey[]             # StarAI API Key（1:N）
  │       └── GatewayUsageLog[]   # 每次 API 调用记录
  │
  ├─── RechargeOrder[]      # 充值订单
  │
  ├─── CityClient?          # 被归属的城市合伙人客户（0:1）
  │       └── partner_id → CityPartner
  │                   └── Commission[]    # 佣金记录
  │
  └─── CorePartner?         # 如果是核心合伙人（0:1）
          └── PartnerCommission[]  # 分润记录
          └── CityPartner[]        # 管辖的城市合伙人（1:N）

SettlementBill              # 月度结算账单
  └── partner_type: core / city
  └── status: pending → approved → paid
```

---

## 七、开发优先级和排期

| 优先级 | 任务 | 预估工作量 | 依赖 |
|--------|------|-----------|------|
| **P0** | Claw 自动注册 + 绑定 Queen（签名认证） | 2-3 天 | 无 |
| **P0** | 安装器/Claw UI 支持邀请码输入 | 1 天 | 无 |
| **P1** | Claw 对话界面显示每次消费金额 | 1-2 天 | P0 |
| **P2** | 城市合伙人 Dashboard 增加下线消费统计 | 1 天 | 无 |
| **P2** | 核心合伙人分润逻辑确认和完善 | 1 天 | 无 |
| **P3** | AdminAnalytics 运营报表完善 | 1-2 天 | 无 |
| **P3** | Overseer 增加运营指标 panel | 1 天 | P3 上条 |

**总计约 8-11 天工作量**，建议按 P0 → P1 → P2 → P3 顺序推进。

---

## 八、关键配置和账号

| 系统 | 地址 | 用途 |
|------|------|------|
| Queen Core | starclaw.net (后台管理) | 运营管理、合伙人管理、结算 |
| StarAI | star-ai.net | AI API 网关，用户充值/消费 |
| Overseer | starclaw.net/overseer (或 :8087) | 监控仪表盘 |
| Nydus | 43.106.158.26 | 部署中心、Release 存储 |
| starclaw.me | 下载页 | 公网下载入口 |

---

## 九、风险和注意事项

1. **支付安全**：支付宝/微信回调必须验签，防止伪造充值
2. **星能超扣**：Synapse 消费是异步的，需要允许小额透支但设上限
3. **佣金防刷**：城市合伙人自己充值不应产生佣金（需过滤 self-referral）
4. **数据一致性**：充值 → 星能 → 佣金必须在同一事务中完成（已实现）
5. **退款处理**：支付退款需要同步扣回星能和佣金（待实现）
