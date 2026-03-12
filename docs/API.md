# StarClaw 🦞 API 接口文档

> Base URL: `http://localhost:8080/v1` 或通过 Nginx 代理 `http://your-domain/v1`

## 认证

除公开接口外，所有 API 需要在 Header 中携带 JWT Token：

```
Authorization: Bearer <token>
```

## 公开接口

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/v1/auth/register` | 用户注册 |
| POST | `/v1/auth/login` | 用户登录（返回 JWT） |
| GET | `/v1/health` | 健康检查 |
| GET | `/v1/version` | 版本信息 |
| GET | `/v1/config` | 部署模式配置 |

## 对话

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/v1/conversations` | 获取会话列表 |
| POST | `/v1/conversations` | 创建会话 |
| DELETE | `/v1/conversations/:id` | 删除会话 |
| GET | `/v1/conversations/:id/messages` | 获取消息列表 |
| POST | `/v1/chat` | 发送消息（SSE 流式响应） |
| POST | `/v1/chat/stop` | 停止生成 |

### 对话命令

在聊天输入框中输入以下命令可直接执行操作（不会发送给 AI）：

| 命令 | 说明 | 示例 |
|------|------|------|
| `/model` | 列出所有可用模型，标记当前对话使用的模型 | `/model` |
| `/model <name>` | 切换当前对话的模型 | `/model minimax` |

**模型匹配规则：** 按 provider 名称 → 显示名 → 模型名 依次模糊匹配，不区分大小写。

**作用范围：** 仅影响当前对话，不影响其他对话或 Agent 默认设置。

**模型优先级（从高到低）：**
1. 对话级覆盖（`/model` 命令设置）
2. Agent 设置中绑定的模型
3. 用户创建的第一个启用模型
4. 平台共享模型

## Agent

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/v1/agents` | 获取 Agent 列表 |
| POST | `/v1/agents` | 创建 Agent |
| PUT | `/v1/agents/:id` | 更新 Agent |
| DELETE | `/v1/agents/:id` | 删除 Agent |
| GET | `/v1/agents/builtin` | 获取内置 Agent 列表 |

## 工作流

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/v1/workflows` | 获取工作流列表 |
| POST | `/v1/workflows` | 创建工作流 |
| PUT | `/v1/workflows/:id` | 更新工作流 |
| DELETE | `/v1/workflows/:id` | 删除工作流 |
| POST | `/v1/workflows/:id/run` | 执行工作流（SSE） |

## RAG 知识库

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/v1/knowledge` | 获取知识库列表 |
| POST | `/v1/knowledge` | 创建知识库 |
| DELETE | `/v1/knowledge/:id` | 删除知识库 |
| POST | `/v1/knowledge/:id/documents` | 上传文档 |
| DELETE | `/v1/knowledge/:id/documents/:doc_id` | 删除文档 |
| POST | `/v1/knowledge/:id/query` | 检索知识库 |

## 模型管理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/v1/models` | 获取模型配置列表 |
| POST | `/v1/models` | 添加模型配置（API Key） |
| PUT | `/v1/models/:id` | 更新模型配置 |
| DELETE | `/v1/models/:id` | 删除模型配置 |

## 编程 Agent

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/v1/coding/run` | 启动编程 Agent（SSE） |
| GET | `/v1/coding/workspace/:id/files` | 获取工作区文件列表 |
| GET | `/v1/coding/workspace/:id/file` | 读取工作区文件 |

## 异步任务

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/v1/tasks` | 获取任务列表 |
| GET | `/v1/tasks/:id` | 获取任务详情 |
| POST | `/v1/tasks/:id/cancel` | 取消任务 |

## 用户

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/v1/user/profile` | 获取用户信息 |
| PUT | `/v1/user/profile` | 更新用户信息 |
| PUT | `/v1/user/password` | 修改密码 |

## 设备管理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/v1/devices` | 获取所有授权设备列表 |
| POST | `/v1/devices/:id/approve` | 审批通过待审设备 |
| POST | `/v1/devices/:id/reject` | 拒绝/删除设备 |
| POST | `/v1/devices/:id/revoke` | 撤销已授权设备 |

新设备通过 Token 登录时需要 Owner 审批（首台设备自动通过，密码登录自动通过）。

## 虫群（加入虫群后可用）

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/v1/swarm/register` | 节点注册 |
| POST | `/v1/swarm/heartbeat` | 心跳上报 |
| GET | `/v1/swarm/config` | 拉取虫群配置 |
| GET | `/v1/swarm/update/check` | 检查版本更新 |

## CLI 命令（starclaw / claw）

服务器上可直接使用 `starclaw` 或 `claw` 命令，也可通过 `make` 调用：

### 用户管理

| 命令 | Make | 说明 |
|------|------|------|
| `starclaw get-token` | `make get-token` | 查看当前 Owner Token |
| `starclaw reset-token` | `make reset-token` | 重新生成 Owner Token |
| `starclaw reset-password --password xxx` | `make reset-password PASS=xxx` | 重置 Owner 密码 |
| `starclaw devices` | `make devices` | 列出所有授权设备 |
| `starclaw approve <id>` | `make approve ID=xxx` | 审批待审设备 |
| `starclaw reject <id>` | `make reject ID=xxx` | 拒绝/撤销设备 |

### 钱包与身份

| 命令 | Make | 说明 |
|------|------|------|
| `starclaw export-key` | `make export-key` | 导出 24 词助记词（BIP-39 备份） |
| `starclaw import-key <words>` | `make import-key SEED='...'` | 从助记词或 hex 恢复身份 |
| `starclaw wallet-info` | `make wallet-info` | 查看 HD 钱包地址和派生路径 |
| `starclaw version` | `make api-version` | 查看版本号 |

## SSE 流式响应格式

对话和工作流接口使用 Server-Sent Events 流式返回：

```
data: {"type":"token","content":"你好"}
data: {"type":"token","content":"，我是"}
data: {"type":"tool_call","name":"web_search","arguments":{"query":"..."}}
data: {"type":"tool_result","name":"web_search","result":"..."}
data: {"type":"done","usage":{"prompt_tokens":100,"completion_tokens":50}}
```
