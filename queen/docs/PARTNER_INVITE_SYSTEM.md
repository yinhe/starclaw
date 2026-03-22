# 合伙人邀请码系统设计文档

> 最后更新：2026-03-23

---

## 一、背景与问题

StarClaw 的合伙人体系分三层：

| 层级 | 角色 | 说明 |
|------|------|------|
| L1 | **团队合伙人（TeamPartner）** | 核心团队成员，负责区域管理和大客户直签 |
| L2 | **城市合伙人（CityPartner）** | 城市级代理，负责本地客户拓展 |
| L3 | **终端客户（Client）** | 使用 StarClaw 的最终用户 |

### 改造前的痛点

1. **鸡蛋问题**：Admin 创建 TeamPartner 需要填 `claw_id`，但 `claw_id` 只有装好 Claw 才生成。流程是：候选人先装 Claw → 手动抄 `claw_id` → 告知 Admin → Admin 手动创建 → 候选人再登录才绑定。
2. **无裂变能力**：团队合伙人无法自主拓展城市合伙人网络，每个城市合伙人都需要 Admin 审核或团队合伙人手动填 `claw_id`。
3. **城市合伙人自申请路径缺陷**：用户通过 `POST /city/apply` 申请时不绑定 `claw_id`，导致 `autoLinkPartner()` 无法自动匹配。

---

## 二、整体架构

### 2.1 三层邀请码裂变

```
                     ┌─────────────┐
                     │    Admin    │
                     └──────┬──────┘
                            │ 创建 invite_code
                            │ type=team_partner
                            ▼
                  ┌───────────────────┐
                  │   团队合伙人候选人  │
                  │                   │
                  │  装 Claw → 注册时  │
                  │  填 invite_code   │
                  └────────┬──────────┘
                           │ 自动成为 TeamPartner
                           │ claw_id 自动绑定
                           ▼
                  ┌───────────────────┐
                  │   TeamPartner     │──── 创建 invite_code
                  │   (团队合伙人)     │     type=city_partner
                  └────────┬──────────┘
                           │
                           ▼
                  ┌───────────────────┐
                  │  城市合伙人候选人   │
                  │                   │
                  │  装 Claw → 注册时  │
                  │  填 invite_code   │
                  └────────┬──────────┘
                           │ 自动成为 CityPartner
                           │ claw_id + 上级自动绑定
                           ▼
                  ┌───────────────────┐
                  │   CityPartner     │──── 分享 ref_code
                  │   (城市合伙人)     │     (已有机制)
                  └────────┬──────────┘
                           │
                           ▼
                  ┌───────────────────┐
                  │   终端客户         │
                  │   装 Claw → 注册时 │
                  │   填 ref_code     │
                  └───────────────────┘
```

### 2.2 三种码的对比

| 属性 | invite_code (team) | invite_code (city) | ref_code |
|------|--------------------|--------------------|----------|
| 格式 | `SC-XXXX-XXXX` | `SC-XXXX-XXXX` | `city_xxxxxxxx` |
| 创建者 | Admin | TeamPartner | 系统自动生成 |
| 使用效果 | 成为 TeamPartner | 成为 CityPartner | 归因为该合伙人客户 |
| 可使用次数 | 可配置（默认 1） | 可配置（默认 1） | 无限 |
| 有效期 | 可配置 | 可配置 | 永久 |
| 自动绑定 claw_id | ✅ | ✅ | ✅（绑 User，非合伙人） |
| 自动绑定上级 | — | ✅（绑创建者 TeamPartner） | ✅（绑 CityPartner） |

---

## 三、核心实体关系

### 3.1 邀请码与 Claw 地址的关系

```
                                Ed25519 密钥对
                                    │
                              ┌─────┴─────┐
                              │  claw_id   │  ← SHA256(pubkey)[:40]
                              │ claw:xxxxx │
                              └─────┬─────┘
                                    │
                     ┌──────────────┼──────────────┐
                     │              │              │
                     ▼              ▼              ▼
              ┌────────────┐ ┌───────────┐ ┌────────────┐
              │ NodeBinding │ │TeamPartner│ │CityPartner │
              │             │ │           │ │            │
              │ node_id ────┤ │ claw_id ──┤ │ claw_id ──┤
              │ queen_user  │ │ user_id   │ │ user_id    │
              │ _id         │ │ level     │ │ city       │
              └──────┬──────┘ │ region    │ │ team_      │
                     │        └───────────┘ │ partner_id │
                     │              ▲       └────────────┘
                     │              │              ▲
                     │              │              │
                     ▼         invite_code    invite_code
              ┌────────────┐  type=team      type=city
              │    User    │
              │            │
              │ role:      │
              │  user      │  ← 普通用户
              │  city      │  ← 城市合伙人
              │  partner   │  ← 团队合伙人
              │  admin     │  ← 管理员
              └────────────┘
```

**关键链路**：

1. **邀请码不直接绑定 claw_id** — 邀请码是一个「入场券」，在 Claw 节点注册时使用
2. **claw_id 在注册时才确定** — 候选人安装 Claw 后，节点自动生成 Ed25519 密钥对，`claw_id` 随之确定
3. **注册时一步完成所有绑定** — `POST /auth/claw-register` 同时完成：
   - 创建 User + NodeBinding（claw_id ↔ user）
   - 消费邀请码 → 创建 TeamPartner/CityPartner（claw_id ↔ partner）
   - 升级用户角色（user → partner/city）

### 3.2 数据模型

```sql
-- 邀请码
CREATE TABLE partner_invites (
    id           VARCHAR(36) PRIMARY KEY,
    code         VARCHAR(20) UNIQUE,           -- SC-XXXX-XXXX
    type         VARCHAR(20),                  -- team_partner / city_partner
    creator_id   VARCHAR(36),                  -- TeamPartner.ID 或 "admin"
    creator_type VARCHAR(20),                  -- admin / team_partner
    creator_name VARCHAR(100),                 -- 显示名
    label        VARCHAR(200),                 -- 内部备注
    max_uses     INT DEFAULT 1,                -- 最大使用次数，0=无限
    used_count   INT DEFAULT 0,                -- 已使用次数
    region       VARCHAR(100),                 -- 目标区域
    comm_rate    DOUBLE DEFAULT 0,             -- 预设佣金率
    level        VARCHAR(20),                  -- 预设级别（team用）
    base_salary  BIGINT DEFAULT 0,             -- 预设底薪（team用，分）
    expires_at   DATETIME,                     -- 过期时间，NULL=永不过期
    status       VARCHAR(20) DEFAULT 'active', -- active / expired / revoked
    created_at   DATETIME,
    updated_at   DATETIME
);

-- 邀请码使用记录
CREATE TABLE partner_invite_uses (
    id         VARCHAR(36) PRIMARY KEY,
    invite_id  VARCHAR(36),   -- PartnerInvite.ID
    code       VARCHAR(20),   -- 冗余，便于查询
    claw_id    VARCHAR(60),   -- 使用者的 claw_id
    user_id    VARCHAR(36),   -- 使用者的 Queen User.ID
    partner_id VARCHAR(36),   -- 创建的 TeamPartner.ID 或 CityPartner.ID
    type       VARCHAR(20),   -- team_partner / city_partner
    created_at DATETIME
);
```

---

## 四、API 清单

### 4.1 公开接口

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/v1/invite/verify?code=XXX` | 验证邀请码是否有效，返回类型/剩余次数 |

### 4.2 Admin 接口

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/v1/admin/invites` | 创建邀请码（team_partner 或 city_partner） |
| GET | `/v1/admin/invites?type=&status=` | 列出所有邀请码 |
| DELETE | `/v1/admin/invites/:id` | 撤销邀请码 |
| GET | `/v1/admin/invite-uses?invite_id=` | 查看使用记录 |

### 4.3 团队合伙人接口

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/v1/partner/invites` | 创建城市合伙人邀请码 |
| GET | `/v1/partner/invites` | 列出自己创建的邀请码 |
| DELETE | `/v1/partner/invites/:id` | 撤销自己的邀请码 |

### 4.4 Claw 节点注册接口（已改造）

```
POST /v1/auth/claw-register
```

**新增字段**：

```json
{
    "node_id": "claw:xxxxxxx",
    "public_key": "hex...",
    "signature": "hex...",
    "timestamp": 1711123456,
    "nickname": "张三的节点",
    "ref_code": "city_abc12345",     // 可选：推荐码（归因客户）
    "invite_code": "SC-A3F8-K9M2"   // 可选：邀请码（成为合伙人）
}
```

**处理优先级**：`invite_code` 和 `ref_code` 可同时存在。`invite_code` 用于合伙人身份创建，`ref_code` 用于客户归因，两者互不冲突。

---

## 五、完整注册流程

```
候选人收到邀请码 SC-A3F8-K9M2
        │
        ▼
安装 Claw 节点（自动生成 Ed25519 密钥对）
        │
        ▼
Claw 启动 → POST /v1/queen/auto-register
{invite_code: "SC-A3F8-K9M2", nickname: "张三"}
        │
        ▼
Claw 代理签名 → POST /v1/auth/claw-register
        │
        ├── ① 验证 Ed25519 签名 ✅
        ├── ② 创建 User + NodeBinding（claw_id ↔ user）
        ├── ③ autoLinkPartner()（检查白名单）
        ├── ④ ValidateInviteCode("SC-A3F8-K9M2") ✅
        ├── ⑤ ConsumeInviteCode() → 创建 TeamPartner/CityPartner
        │       ├── 绑定 claw_id
        │       ├── 绑定 user_id
        │       ├── 绑定上级 TeamPartner（如果是 city 类型）
        │       └── 升级用户角色
        ├── ⑥ 发放 100⚡ 欢迎奖励
        └── ⑦ 返回 JWT + partner 信息
```

---

## 六、与现有机制的兼容性

| 现有机制 | 是否保留 | 说明 |
|----------|----------|------|
| Admin 白名单创建 TeamPartner | ✅ 保留 | `POST /admin/partners` 仍可直接填 claw_id 创建 |
| autoLinkPartner() 自动匹配 | ✅ 保留 | 已在白名单中的 claw_id 登录时仍自动绑定 |
| 城市合伙人自主申请 | ✅ 保留 | `POST /city/apply` 仍可用（需 Admin 审核） |
| TeamPartner 手动添加城市合伙人 | ✅ 保留 | `POST /partner/city-partners/claw` 仍可用 |
| ref_code 客户推荐 | ✅ 保留 | 城市合伙人分享推荐码不受影响 |
| 收入分润链 | ✅ 保留 | `resolve-partners` 链路完全兼容 |

邀请码是在现有机制之上的**增量功能**，不影响任何已有流程。

---

## 七、安全设计

1. **邀请码格式**：`SC-XXXX-XXXX`（8 位随机 hex，大写），碰撞概率极低（4 billion 可能值）
2. **使用次数限制**：每个邀请码有 `max_uses` 上限，默认 1 次（一码一人）
3. **过期时间**：可选设置 `expires_at`，过期自动失效
4. **撤销机制**：Admin 和创建者均可随时撤销邀请码
5. **防重复**：同一 `claw_id` 不会重复创建合伙人记录（幂等）
6. **权限隔离**：TeamPartner 只能创建 `city_partner` 类型邀请码，不能创建 `team_partner`
7. **审计追踪**：每次使用均记录在 `partner_invite_uses` 表，包含完整链路信息

---

## 八、优化建议与待办

### 8.1 已识别的优化点

#### ① 邀请码可读性提升
**现状**：`SC-A3F8-K9M2` 是随机 hex，不好记忆和口头传播。
**建议**：支持自定义别名码，例如 `SC-BEIJING-001` 或 `SC-张三专属`。实现方式：在 `PartnerInvite` 上增加 `alias` 字段，验证时同时匹配 `code` 和 `alias`。

#### ② 邀请码 → 落地页引导
**现状**：候选人拿到邀请码后，需要自己知道去哪里下载 Claw、怎么填码。
**建议**：生成带邀请码的落地页链接，如 `https://starclaw.net/join?code=SC-A3F8-K9M2`，页面自动引导下载安装并预填邀请码。

#### ③ 邀请码预绑定信息
**现状**：候选人注册后，合伙人的 `name`/`email` 默认是 Claw 节点的 nickname。
**建议**：邀请码创建时可预填候选人姓名/手机/邮箱，注册时自动填入合伙人档案。增加字段：

```go
type PartnerInvite struct {
    // ...existing fields...
    PresetName  string `json:"preset_name" gorm:"type:varchar(100)"`
    PresetPhone string `json:"preset_phone" gorm:"type:varchar(20)"`
    PresetEmail string `json:"preset_email" gorm:"type:varchar(200)"`
}
```

#### ④ 城市合伙人自申请补充 claw_id
**现状**：`POST /city/apply` 不绑定 `claw_id`，导致 `autoLinkPartner()` 无法匹配。
**建议**：在申请接口增加可选的 `claw_id` 字段。如果用户已通过 Claw 登录（JWT 中有 claw_id），自动填入。

#### ⑤ 邀请码统计看板
**现状**：只有基础的 CRUD。
**建议**：在 Cerebrate Partner 面板和 Queen Core 后台增加：
- 邀请码转化率（创建 vs 使用）
- 邀请来源分布（哪个团队合伙人拉的人最多）
- 时间趋势图（每日/每周新增合伙人）

#### ⑥ 多级邀请码（城市合伙人也能发码）
**现状**：只有 Admin 和 TeamPartner 能创建邀请码。
**建议**：未来可让 CityPartner 也能创建 `ref_code` 升级版邀请码，被邀请的客户自动归因到该城市合伙人。当前 `ref_code` 已实现此功能，但没有使用次数和过期时间控制。

#### ⑦ invite_code 与 ref_code 统一
**现状**：两套码、两套逻辑、两张表。
**建议**：长期可考虑统一为一套「推广码」系统，通过 `type` 字段区分用途（partner_team / partner_city / referral），共享验证、统计、管理界面。

### 8.2 收入分润链完整性

当前分润链路：
```
Claw 消费 API → profit-split
    │
    ├── resolve-partners(claw_id)
    │   └── claw_id → NodeBinding → user_id → CityClient → CityPartner → TeamPartner
    │
    ├── CityPartner 佣金 (20%)
    ├── TeamPartner 管理费 (5%)
    └── 投资人池
```

**邀请码创建的合伙人完全兼容此链路**，因为：
- 通过邀请码注册的 TeamPartner/CityPartner 有完整的 `claw_id` + `user_id` 绑定
- `CityPartner.TeamPartnerID` 在邀请码消费时自动设置为创建者
- 后续客户通过 `ref_code` 归因时，整条链路自动打通

---

## 九、文件清单

| 文件 | 变更类型 | 说明 |
|------|----------|------|
| `queen/api/internal/model/partner.go` | 修改 | +PartnerInvite, +PartnerInviteUse |
| `queen/api/internal/handler/invite.go` | 新建 | 完整 CRUD + ValidateInviteCode + ConsumeInviteCode |
| `queen/api/internal/handler/claw_auth.go` | 修改 | ClawRegister 增加 invite_code 字段和处理逻辑 |
| `queen/api/internal/router/router.go` | 修改 | +13 条路由（public + admin + partner） |
| `queen/api/cmd/server/main.go` | 修改 | AutoMigrate 新增 2 张表 |
| `claw/api/internal/api/v1/queen.go` | 修改 | AutoRegister 传递 invite_code |

---

## 十、使用示例

### Admin 创建团队合伙人邀请码

```bash
curl -X POST https://queen.starclaw.net/v1/admin/invites \
  -H "Authorization: Bearer <admin_jwt>" \
  -d '{
    "type": "team_partner",
    "label": "华东区负责人-张三",
    "max_uses": 1,
    "region": "华东",
    "level": "overlord",
    "base_salary": 1000000,
    "direct_comm_rate": 0.30
  }'
# 返回: {"invite": {"code": "SC-A3F8-K9M2", ...}}
```

### 团队合伙人创建城市合伙人邀请码

```bash
curl -X POST https://queen.starclaw.net/v1/partner/invites \
  -H "Authorization: Bearer <partner_jwt>" \
  -d '{
    "label": "杭州代理-李四",
    "max_uses": 1,
    "region": "杭州",
    "comm_rate": 0.20
  }'
# 返回: {"invite": {"code": "SC-B7E2-N4P1", ...}}
```

### 候选人通过 Claw 注册

```bash
# Claw 节点上调用
curl -X POST http://localhost:8080/v1/queen/auto-register \
  -d '{
    "invite_code": "SC-A3F8-K9M2",
    "nickname": "张三的AI节点"
  }'
# 返回: {token, user: {role: "partner"}, partner: {partner_id, partner_type: "team_partner"}}
```

### 验证邀请码（无需登录）

```bash
curl https://queen.starclaw.net/v1/invite/verify?code=SC-A3F8-K9M2
# 返回: {"valid": true, "type": "team_partner", "creator_name": "admin", "remaining": 1}
```
