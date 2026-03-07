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

## 虫群（加入虫群后可用）

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/v1/swarm/register` | 节点注册 |
| POST | `/v1/swarm/heartbeat` | 心跳上报 |
| GET | `/v1/swarm/config` | 拉取虫群配置 |
| GET | `/v1/swarm/update/check` | 检查版本更新 |

## SSE 流式响应格式

对话和工作流接口使用 Server-Sent Events 流式返回：

```
data: {"type":"token","content":"你好"}
data: {"type":"token","content":"，我是"}
data: {"type":"tool_call","name":"web_search","arguments":{"query":"..."}}
data: {"type":"tool_result","name":"web_search","result":"..."}
data: {"type":"done","usage":{"prompt_tokens":100,"completion_tokens":50}}
```
