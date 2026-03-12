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

## 算力贡献（Compute Contribution）

StarClaw 允许有 GPU 的节点贡献算力给网络中的其他节点使用。Claw A（有 GPU）可以为 Claw B（无 GPU）提供推理服务，整个过程对用户完全透明。

### 工作原理

```
用户在 Claw B 提问
   │
   ├─① B 收到推理请求（如 "用 qwen2.5 回答xxx"）
   │
   ├─② B 本地没有该模型 → 查询 ContributorRegistry（已注册的算力节点列表）
   │
   ├─③ 信任加权选择最优贡献者 A（信任分*0.5 + 负载*0.3 + 延迟*0.2）
   │
   ├─④ B 向 A 发送 Ed25519 签名请求：POST /v1/inference/execute
   │
   ├─⑤ A 调用本地 Ollama 执行推理 → 返回结果给 B
   │
   └─⑥ B 将结果返回用户，异步上报用量结算（90% 给 A，10% 平台）
```

### 节点发现方式（3 种，逐级 fallback）

| 方式 | 原理 | 配置 |
|------|------|------|
| **Gossip P2P** | A 的 ContributorService 每 30s 向已知 peer 注册 | 互为 peer 节点 |
| **Swarm** | A 和 B 都注册到虫群，虫群广播算力节点列表 | `swarm.enabled: true` |
| **手动添加** | B 的管理界面手动添加 A 为 peer | Web UI → 节点管理 |

### 配置（算力贡献者）

在 `config.yaml` 中或通过环境变量 `STARCLAW_CONTRIBUTOR_*` 配置：

```yaml
contributor:
  enabled: true                          # 启用算力贡献（默认 false）
  ollama_url: "http://localhost:11434"   # 本地 Ollama 地址
  max_jobs: 2                            # 最大并发推理数
  external_addr: "https://a.example.com" # 其他节点访问本机的公网地址
```

> **注意：** 贡献者必须有公网可达地址（公网 IP 或端口映射）。NAT 穿透（Nydus）尚在规划中。

### 推理路由 API（公开）

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/v1/inference/status` | 查看路由器状态和已注册贡献者数量 |

### 推理路由 API（Ed25519 节点签名保护）

以下接口需要 `X-Node-ID` + `X-Node-PubKey` + `X-Node-Signature` + `X-Node-Timestamp` 签名头：

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/v1/inference/register` | 贡献者注册（上报模型列表、地址、GPU 信息） |
| POST | `/v1/inference/heartbeat` | 贡献者心跳（上报当前负载、延迟） |
| POST | `/v1/inference/unregister` | 贡献者注销 |
| POST | `/v1/inference/execute` | 执行推理（路由器转发请求到贡献者） |

#### POST /v1/inference/register

```json
{
  "address": "https://a.example.com",
  "models": ["qwen2.5:7b", "llama3:8b"],
  "max_jobs": 4,
  "gpu_memory_mb": 24000,
  "region": "cn-east"
}
```

#### POST /v1/inference/execute

```json
{
  "model": "qwen2.5:7b",
  "messages": [{"role": "user", "content": "你好"}],
  "temperature": 0.7,
  "max_tokens": 2048,
  "stream": true
}
```

响应：与标准 OpenAI Chat Completion 格式一致（流式为 SSE，非流式为 JSON）。

### 信任体系

每个贡献者有 0.0–1.0 的综合信任分，由 5 个维度加权计算：

| 维度 | 权重 | 说明 |
|------|------|------|
| 在线率 | 20% | 心跳一致性（收到 / 预期） |
| 成功率 | 30% | 成功任务 / 总任务 |
| 延迟稳定性 | 15% | 低延迟且稳定 = 高分 |
| 抽检通过率 | 25% | 1% 请求会发给第二个节点验证 |
| 服务时长 | 10% | 长期稳定服务的加分（对数增长，~90 天封顶） |

#### 信任等级

| 等级 | 综合分要求 | 用途 |
|------|-----------|------|
| **any** | 默认 | 任意贡献者，最便宜 |
| **brood** | ≥0.70 且 ≥10 任务 | 企业内部节点 |
| **verified** | ≥0.85 且 ≥100 任务 | 认证贡献者，适合隐私数据 |
| **self** | — | 仅路由到自己的节点 |

- **自动晋升**：达到综合分和任务数阈值后自动升级
- **自动降级**：抽检通过率 <50% 立即降为 `any`

### 结算

每次推理完成后，路由节点异步上报用量到虫群账本：

- 按 token 计费：输入 0.5⭐/千 token，输出 1⭐/千 token
- **90%** 归算力贡献者，**10%** 归平台
- 余额为 0 的节点进入休眠模式（本地功能正常，无法使用网络算力）

### 安全保障

| 层级 | 机制 |
|------|------|
| **身份验证** | Ed25519 签名 — 每个请求都携带签名，防伪造 |
| **信任过滤** | 请求时可指定最低信任等级，只路由到满足要求的贡献者 |
| **抽检验证** | 1% 请求随机发给第二个贡献者，Jaccard 相似度 ≥0.5 才通过 |
| **经济约束** | 作弊导致信任分下降 → 接单减少 → 收入降低 |

## SSE 流式响应格式

对话和工作流接口使用 Server-Sent Events 流式返回：

```
data: {"type":"token","content":"你好"}
data: {"type":"token","content":"，我是"}
data: {"type":"tool_call","name":"web_search","arguments":{"query":"..."}}
data: {"type":"tool_result","name":"web_search","result":"..."}
data: {"type":"done","usage":{"prompt_tokens":100,"completion_tokens":50}}
```
