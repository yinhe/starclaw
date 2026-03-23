# Overlord 全链路设计：管理员创建 → 员工使用

> 文档版本: v1.0 | 2026-03-23

## 1. 当前架构

```
管理员(Console :3095)                    员工(Web :3096)                     Claw节点
    │                                      │                                  │
    ├─ 登录 /brood/auth/login              ├─ 同一登录接口(role=viewer)         │
    ├─ 查看模板(9个官方+自定义)             ├─ 看到实例列表(HomePage)             │
    ├─ 创建实例→桥接Claw创建Squad           ├─ 点击实例→对话(ChatPage)            ├─ 注册到Overlord
    ├─ 创建任务→桥接Claw创建Mission         ├─ 通用对话(DirectChatPage)           ├─ 30s心跳
    ├─ 查看用量/员工统计                    ├─ 个人资料页(ProfilePage)            ├─ 接收Squad/Mission
    └─ WebSocket实时推送                    └─                                   └─ 状态同步(30s)
```

### 角色权限

| 角色 | 权限 |
|------|------|
| superadmin | 全部 (*) |
| admin | claws/teams/nydus/molt/webhook/billing/brand/license/compliance/features + team_agent RW |
| operator | claws RW + nydus RW + molt RA + billing R + team_agent RW |
| viewer (员工) | claws R + 基础 R + team_agent.read (**只读**) |

### 认证体系

- **Overlord**: AdminUser + SHA256(password) → token_hash, 请求头 `X-Admin-Token`
- **Claw 节点**: 独立 JWT 体系, Ed25519 身份 (claw:xxx)
- **Overlord→Claw 桥接**: `X-Overlord-Token` 共享密钥 (`OVERLORD_CLAW_TOKEN`)
- **两套体系完全独立, 无互通**

---

## 2. 缺口分析

### P0 — 流程断裂（必须修复）

| # | 缺口 | 现状 | 影响 |
|---|------|------|------|
| 1 | 员工无法提交任务 | `CreateMission` 需 `team_agent.write`, viewer 只有 `read` | 员工只能聊天不能派任务 |
| 2 | 聊天无流式输出 | `ChatCompletion` 固定 `Stream:false`, 前端同步等待 | 长回复等 10-30s |
| 3 | 无实例可见性控制 | `ListInstances` 返回团队所有实例 | 客服不应看到 QuantClaw |

### P1 — 节点登录

| # | 缺口 | 现状 | 影响 |
|---|------|------|------|
| 4 | Overlord↔Claw 无统一认证 | 两套独立账号体系 | 无法互通 |
| 5 | 员工无法通过 Claw 节点登录 | Claw API 只认 Overlord 管理令牌 | 员工不能直接访问节点 |
| 6 | Claw 用户无法反向登录 Overlord | 无 token exchange | 已有 Claw 账号不能成为员工 |

### P2 — 体验优化

| # | 缺口 | 现状 | 影响 |
|---|------|------|------|
| 7 | 无多会话管理 | 每实例每用户一条线程 | 无法新建/切换对话 |
| 8 | 无 Markdown 渲染 | `whitespace-pre-wrap` 纯文本 | 代码/表格格式丢失 |
| 9 | 模型选择硬编码 | Direct Chat 固定 deepseek-chat | 无法按需切换 |
| 10 | 无实例欢迎语 | 空页面只有"发送消息" | 员工不知道问什么 |

### P3 — 规模化

| # | 缺口 | 现状 | 影响 |
|---|------|------|------|
| 11 | 无员工自助注册 | 管理员手动创建 | 规模化困难 |
| 12 | 无每员工用量限制 | 实例有 budget 无员工维度 | 一人耗完预算 |
| 13 | 无实例分配控制 | 同团队全可见 | 无法精细化管理 |

---

## 3. 实施计划

### Phase 1: 断裂修复

#### 1a. 员工提交权限

**后端**: `model/team.go` — viewer 增加 `team_agent.submit` 权限

```
viewer 权限新增: "team_agent.submit"
```

**后端**: `cmd/server/main.go` — 新增 submit 路由组

```go
taSubmit := brood.Group("/team-agent")
taSubmit.Use(middleware.RequirePermission("team_agent.submit"))
{
    taSubmit.POST("/instances/:id/missions", teamAgentH.CreateMission)
    taSubmit.POST("/instances/:id/chat", teamAgentH.SendChat)
    taSubmit.POST("", teamAgentH.SendDirectChat)  // direct chat
}
```

#### 1b. 实例可见性

**模型**: `TeamInstance` 增加字段

```go
Published    bool   `json:"published" gorm:"default:false"`         // 员工可见
VisibleTo    string `json:"visible_to" gorm:"type:text"`            // JSON: ["user_id1","user_id2"] 空=全员可见
WelcomeMsg   string `json:"welcome_msg" gorm:"type:text"`           // 首次对话欢迎语
DefaultModel string `json:"default_model" gorm:"type:varchar(100)"` // 覆盖模板默认模型
```

**逻辑**: `ListInstances` 对 viewer 角色过滤 `published=true` + `visible_to` 包含当前用户

**Console**: `TeamAgentPage` 增加"发布"开关 + 可见范围设置

#### 1c. 流式聊天 (SSE)

**Claw Client**: `claw/client.go` 新增 `ChatCompletionStream()` 返回 `io.ReadCloser`

**API**: `SendChat` / `SendDirectChat` 检测 `Accept: text/event-stream` 头:
- 有 → SSE 流式响应, 每个 chunk 写 `data: {...}\n\n`
- 无 → 保持现有同步行为 (向后兼容)

**Web 前端**: `ChatPage.tsx` / `DirectChatPage.tsx`:
- `fetch` + `ReadableStream` 读取 SSE
- 逐 token 追加到 assistant message
- 打字机效果

### Phase 2: 节点登录

#### 2a. Claw 侧令牌交换

**文件**: `claw/api/internal/api/v1/overlord_internal.go`

```
POST /v1/internal/auth/exchange
请求: X-Overlord-Token + { overlord_user_id, username, role }
响应: { token (Claw JWT), user_id, expires_at }
```

逻辑:
1. 验证 X-Overlord-Token
2. 查找/创建 Claw 本地 User (username = `overlord:{overlord_user_id}`)
3. 签发 Claw JWT (有效期 24h)
4. 返回 token

#### 2b. Overlord 侧节点登录

**文件**: `overlord/api/internal/handler/team.go`

```
POST /brood/auth/node-login
请求: { claw_id, node_address, claw_token }
响应: { overlord_token, user }
```

逻辑:
1. 根据 claw_id 找到 ClawNode 记录, 获取 node address
2. 调用 Claw 节点 `GET /api/auth/verify` (带 claw_token) 验证身份
3. 提取 username / user_id
4. 查找/创建 AdminUser (role=viewer, 关联 claw_id)
5. 签发 overlord token
6. 返回

#### 2c. 前端登录页

**Web**: `LoginPage.tsx` 增加"节点登录"标签页
- 输入: 节点地址 + Claw JWT (或 Claw Web 自动携带)
- 调用 `/brood/auth/node-login`
- 成功 → setAuth + 进入主页

**Console**: `LoginPage.tsx` 同步增加节点登录入口

### Phase 3: 体验优化

#### 3a. 多会话管理

**模型**: 新增 `Conversation`

```go
type Conversation struct {
    ID         string    // UUID
    InstanceID string    // TeamInstance ID, "direct" 表示通用对话
    UserID     string    // AdminUser ID
    Title      string    // 自动从首条消息截取
    Model      string    // 本会话使用的模型
    CreatedAt  time.Time
    UpdatedAt  time.Time
}
```

`ChatMessage` 增加 `ConversationID` 字段

**API**:
- `GET /brood/team-agent/instances/:id/conversations` — 会话列表
- `POST /brood/team-agent/instances/:id/conversations` — 新建会话
- `DELETE /brood/team-agent/instances/:id/conversations/:cid` — 删除会话

**前端**: ChatPage 左侧增加会话列表栏 (移动端可折叠)

#### 3b. Markdown + 欢迎语 + 模型选择

**Markdown**: 安装 `react-markdown` + `remark-gfm` + `rehype-highlight`, 替换 `whitespace-pre-wrap`

**欢迎语**: ChatPage 空消息时显示 `instance.welcome_msg` (管理员在 Console 配置)

**模型选择**: ChatPage 头部增加模型下拉, 使用 `instance.default_model` 作为默认值

### Phase 4: 规模化

#### 4a. 员工邀请链接

**模型**: `EmployeeInvite` (code, team_id, role, max_uses, expires_at)

**API**: 
- `POST /brood/admins/invite` — 生成邀请链接
- `POST /brood/auth/register` — 员工通过邀请码自助注册

**前端**: Web LoginPage 增加"注册"标签, Console 管理员可生成邀请链接

#### 4b. 用量限额 + 实例分配

**模型**: `AdminUser` 增加 `daily_token_limit`, `monthly_token_limit`

**模型**: `InstanceAccess` (instance_id, user_id) — 精确控制哪个员工可访问哪个实例

**逻辑**: 
- `SendChat` 检查员工当日 token 用量, 超限拒绝
- `ListInstances` 对 viewer 额外过滤 `InstanceAccess`

---

## 4. 数据流总览（实施后）

```
                    ┌─────────────────────────────────────────┐
                    │              Overlord API                │
                    │                                         │
  Console ────→     │  /brood/auth/login     (用户名密码)      │
  (管理员)          │  /brood/auth/node-login (Claw令牌)       │    ←──── Claw Node
                    │                                         │          (注册/心跳)
  Web ────────→     │  /brood/team-agent/*    (实例/对话/任务)  │
  (员工)            │  /brood/chat           (通用对话 SSE)    │
                    │                                         │
                    │         ↕ X-Overlord-Token               │
                    │                                         │
                    │  Claw Bridge: ChatCompletion(Stream)     │
                    │  Claw Bridge: Squad/Mission CRUD         │
                    │  Claw Bridge: Auth Exchange              │
                    └─────────────────────────────────────────┘
```

---

## 5. 文件变更清单

| Phase | 文件 | 变更 |
|-------|------|------|
| 1a | `model/team.go` | viewer 权限增加 team_agent.submit |
| 1a | `cmd/server/main.go` | submit 路由组 |
| 1b | `model/team_agent.go` | TeamInstance +4 字段 |
| 1b | `handler/team_agent.go` | ListInstances 过滤 + Publish/SetVisibility |
| 1b | `console/.../TeamAgentPage.tsx` | 发布开关 + 可见范围 |
| 1c | `claw/client.go` | ChatCompletionStream() |
| 1c | `handler/team_agent.go` | SSE 流式 SendChat/SendDirectChat |
| 1c | `web/.../ChatPage.tsx` | SSE 读取 + 逐token渲染 |
| 1c | `web/.../DirectChatPage.tsx` | 同上 |
| 2a | `claw/.../overlord_internal.go` | POST /v1/internal/auth/exchange |
| 2b | `handler/team.go` | POST /brood/auth/node-login |
| 2b | `claw/client.go` | VerifyToken() 方法 |
| 2c | `web/.../LoginPage.tsx` | 节点登录标签页 |
| 2c | `console/.../LoginPage.tsx` | 同上 |
| 3a | `model/team_agent.go` | Conversation 模型 |
| 3a | `handler/team_agent.go` | 会话 CRUD |
| 3a | `web/.../ChatPage.tsx` | 会话列表 |
| 3b | `web/package.json` | react-markdown + remark-gfm |
| 3b | `web/.../ChatPage.tsx` | Markdown 渲染 + 欢迎语 |
| 4a | `model/team.go` | EmployeeInvite 模型 |
| 4a | `handler/team.go` | 邀请 + 自助注册 |
| 4b | `model/team.go` | AdminUser +2 字段 |
| 4b | `model/team_agent.go` | InstanceAccess 模型 |
