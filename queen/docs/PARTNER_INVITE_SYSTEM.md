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
                  │   CityPartner     │──── 创建 invite_code
                  │   (城市合伙人)     │     type=referral
                  └────────┬──────────┘     (升级版 ref_code，带次数/过期/别名控制)
                           │
                           ▼
                  ┌───────────────────┐
                  │   终端客户候选人    │
                  │   装 Claw → 注册时 │
                  │   填 invite_code  │
                  │   或 ref_code     │
                  └───────────────────┘
```

### 2.2 三种码的对比

| 属性 | invite (team) | invite (city) | invite (referral) | ref_code（旧） |
|------|---------------|---------------|-------------------|---------------|
| 格式 | `SC-XXXX-XXXX` 或别名 | `SC-XXXX-XXXX` 或别名 | `SC-XXXX-XXXX` 或别名 | `city_xxxxxxxx` |
| 创建者 | Admin | TeamPartner | CityPartner | 系统自动 |
| 使用效果 | 成为 TeamPartner | 成为 CityPartner | 归因为客户 | 归因为客户 |
| 可使用次数 | 可配置（默认 1） | 可配置（默认 1） | 可配置（默认无限） | 无限 |
| 有效期 | 可配置 | 可配置 | 可配置 | 永久 |
| 自动绑定 claw_id | ✅ | ✅ | ✅ | ✅ |
| 自动绑定上级 | — | ✅ 绑 TeamPartner | ✅ 绑 CityPartner | ✅ 绑 CityPartner |
| 别名码 | ✅ | ✅ | ✅ | ❌ |
| 预设信息 | ✅ | ✅ | ✅ | ❌ |
| 落地页链接 | ✅ | ✅ | ✅ | ❌ |
| 统计看板 | ✅ | ✅ | ✅ | ❌ |

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

### 8.1 优化实施清单（全部已完成 ✅）

#### ✅ ① 邀请码可读性提升 — 别名码
- `PartnerInvite` 新增 `Alias` 字段（`uniqueIndex`），支持自定义可读别名
- 例如：`SC-BEIJING-001`、`SC-HUADONG-ZHANGSAN`
- `ValidateInviteCode()` 同时匹配 `code` 和 `alias`
- `DisplayCode()` 优先返回 alias

#### ✅ ② 邀请码 → 落地页引导
- 所有 API 响应自动附带 `join_url` 字段
- 格式：`https://starclaw.net/join?code=SC-BEIJING-001`
- 基础 URL 通过环境变量 `SITE_BASE_URL` 配置
- `JoinURL()` 方法优先使用 alias 生成链接

#### ✅ ③ 邀请码预绑定候选人信息
- `PartnerInvite` 新增 `PresetName`、`PresetPhone`、`PresetEmail`
- `ConsumeInviteCode()` 自动将 preset 信息写入 User 和 Partner 档案
- 仅在用户字段为空或为 `@claw.local` 占位符时覆盖

#### ✅ ④ 城市合伙人自申请补充 claw_id
- `POST /city/apply` 新增 `resolveClawID()` 自动从 NodeBinding/OAuth 解析 `claw_id`
- 新增 `invite_code` 可选字段，自动绑定上级 TeamPartner
- 彻底修复 `autoLinkPartner()` 匹配不上的 bug

#### ✅ ⑤ 邀请码统计看板
- Admin: `GET /admin/invite-stats` — 总量/活跃/使用/转化率/按类型分布/Top 创建者/7 天趋势
- TeamPartner: `GET /partner/invite-stats` — 自己的邀请码和使用统计

#### ✅ ⑥ 多级邀请码 — 城市合伙人也能发码
- CityPartner 新增 `referral` 类型邀请码，替代原始 `ref_code` 的无控制分享
- 支持使用次数限制、过期时间、别名、预设信息
- 路由：`POST/GET/DELETE /city/invites`
- `consumeReferral()` 自动创建 CityClient 归因记录

#### ✅ ⑦ invite_code 与 ref_code 统一
- `PartnerInvite.Type` 现支持三种：`team_partner` / `city_partner` / `referral`
- 统一验证逻辑 `ValidateInviteCode()` 处理所有类型
- 统一消费逻辑 `ConsumeInviteCode()` 按 type 分发
- 统一管理界面和统计

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
| `queen/api/internal/model/partner.go` | 修改 | +PartnerInvite（含 Alias/Preset*/JoinURL/DisplayCode）, +PartnerInviteUse |
| `queen/api/internal/handler/invite.go` | 新建 | 全功能：Admin/TeamPartner/CityPartner CRUD + Stats + 统一 Validate/Consume + referral 类型 |
| `queen/api/internal/handler/claw_auth.go` | 修改 | ClawRegister 增加 invite_code 字段和消费逻辑 |
| `queen/api/internal/handler/city.go` | 修改 | Apply 自动解析 claw_id（resolveClawID）+ 支持 invite_code 绑定上级 |
| `queen/api/internal/router/router.go` | 修改 | +20 条路由（public + admin + partner + city） |
| `queen/api/cmd/server/main.go` | 修改 | AutoMigrate 新增 2 张表 |
| `claw/api/internal/api/v1/queen.go` | 修改 | AutoRegister 传递 invite_code |

---

## 十、使用示例

### Admin 创建团队合伙人邀请码（含别名 + 预设信息）

```bash
curl -X POST https://queen.starclaw.net/v1/admin/invites \
  -H "Authorization: Bearer <admin_jwt>" \
  -d '{
    "type": "team_partner",
    "alias": "SC-HUADONG-ZHANGSAN",
    "label": "华东区负责人-张三",
    "max_uses": 1,
    "region": "华东",
    "level": "overlord",
    "base_salary": 1000000,
    "comm_rate": 0.30,
    "preset_name": "张三",
    "preset_phone": "13800138000",
    "preset_email": "zhangsan@example.com"
  }'
# 返回:
# {
#   "invite": {
#     "code": "SC-A3F8-K9M2",
#     "alias": "SC-HUADONG-ZHANGSAN",
#     "display_code": "SC-HUADONG-ZHANGSAN",
#     "join_url": "https://starclaw.net/join?code=SC-HUADONG-ZHANGSAN",
#     ...
#   }
# }
```

### 团队合伙人创建城市合伙人邀请码

```bash
curl -X POST https://queen.starclaw.net/v1/partner/invites \
  -H "Authorization: Bearer <partner_jwt>" \
  -d '{
    "alias": "SC-HANGZHOU-001",
    "label": "杭州代理-李四",
    "max_uses": 1,
    "region": "杭州",
    "comm_rate": 0.20,
    "preset_name": "李四"
  }'
# 返回: {"invite": {"display_code": "SC-HANGZHOU-001", "join_url": "https://starclaw.net/join?code=SC-HANGZHOU-001", ...}}
```

### 城市合伙人创建推荐邀请码（多级裂变）

```bash
curl -X POST https://queen.starclaw.net/v1/city/invites \
  -H "Authorization: Bearer <city_jwt>" \
  -d '{
    "alias": "SC-HANGZHOU-VIP",
    "label": "杭州 VIP 客户通道",
    "max_uses": 0
  }'
# 返回: {"invite": {"type": "referral", "display_code": "SC-HANGZHOU-VIP", "join_url": "...", ...}}
# max_uses=0 表示无限次使用
```

### 候选人通过 Claw 注册（支持别名码）

```bash
# 可使用 code 或 alias
curl -X POST http://localhost:8080/v1/queen/auto-register \
  -d '{
    "invite_code": "SC-HUADONG-ZHANGSAN",
    "nickname": "张三的AI节点"
  }'
# 返回: {token, user: {role: "partner", nickname: "张三"}, partner: {partner_type: "team_partner"}}
# preset_name "张三" 自动填入 user.nickname 和 partner.name
```

### 验证邀请码（无需登录）

```bash
curl https://queen.starclaw.net/v1/invite/verify?code=SC-HUADONG-ZHANGSAN
# 返回:
# {
#   "valid": true,
#   "type": "team_partner",
#   "creator_name": "admin",
#   "remaining": 1,
#   "unlimited": false,
#   "join_url": "https://starclaw.net/join?code=SC-HUADONG-ZHANGSAN"
# }
```

### 查看邀请码统计（Admin）

```bash
curl https://queen.starclaw.net/v1/admin/invite-stats \
  -H "Authorization: Bearer <admin_jwt>"
# 返回:
# {
#   "total_invites": 42,
#   "active_invites": 15,
#   "total_uses": 28,
#   "conversion_rate": "66.7%",
#   "by_type": [{"type": "team_partner", "count": 10}, ...],
#   "top_creators": [{"creator_name": "张三", "total_used": 12}, ...],
#   "trend_7d": [{"day": "2026-03-22", "count": 3}, ...]
# }
```

### 城市合伙人申请（自动绑 claw_id + 邀请码上级）

```bash
curl -X POST https://queen.starclaw.net/v1/city/apply \
  -H "Authorization: Bearer <user_jwt>" \
  -d '{
    "name": "王五",
    "city": "杭州",
    "phone": "13900139000",
    "email": "wangwu@example.com",
    "invite_code": "SC-HANGZHOU-001"
  }'
# claw_id 从 NodeBinding 自动解析
# invite_code 自动绑定上级 TeamPartner
```
