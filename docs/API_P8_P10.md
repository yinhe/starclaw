# StarClaw 🦞 API 文档 — P8/P9/P10 扩展接口

> 本文档补充 [API.md](./API.md) 中未覆盖的 P8–P10 阶段新增接口。  
> Base URL: `http://localhost:8080/v1`  
> 认证: `Authorization: Bearer <token>`（除标注「公开」外均需认证）

---

## P8: Agent Economy — 市场

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/v1/marketplace/listings` | 浏览已发布列表（分页、筛选、排序） |
| GET | `/v1/marketplace/listings/:id` | 获取单个上架详情 |
| GET | `/v1/marketplace/trending` | 热门 Agent 排行 |
| GET | `/v1/marketplace/listings/:id/ratings` | 获取评价列表 |
| GET | `/v1/marketplace/listings/:id/access` | 检查当前用户是否有权使用 |
| POST | `/v1/marketplace/listings/:id/purchase` | 购买 Agent |
| GET | `/v1/marketplace/purchases` | 我的已购列表 |
| POST | `/v1/marketplace/listings/:id/rate` | 提交评价（1–5 星） |
| GET | `/v1/marketplace/creator/profile` | 获取创作者资料 |
| POST | `/v1/marketplace/creator/register` | 注册为创作者 |
| GET | `/v1/marketplace/creator/dashboard` | 创作者仪表盘（收入概览） |
| GET | `/v1/marketplace/creator/revenue` | 收入明细 |
| GET | `/v1/marketplace/creator/listings` | 我的上架列表 |
| POST | `/v1/marketplace/creator/listings` | 创建新上架 |
| PUT | `/v1/marketplace/creator/listings/:id` | 更新上架信息 |
| POST | `/v1/marketplace/creator/listings/:id/version` | 发布新版本 |

### Admin

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/v1/admin/marketplace/pending` | 待审核列表 |
| POST | `/v1/admin/marketplace/listings/:id/review` | 审核上架（通过/拒绝） |

---

## P8: Observability — 可观测性

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/v1/observe/stats` | 可观测性总览统计 |
| GET | `/v1/observe/traces/:trace_id` | 获取完整 Trace（含所有 Span） |
| GET | `/v1/observe/spans` | 查询 Span（按时间、Agent、Kind 筛选） |
| GET | `/v1/observe/logs` | 结构化日志查询（按级别、时间范围） |
| GET | `/v1/observe/alerts/rules` | 告警规则列表 |
| POST | `/v1/observe/alerts/rules` | 创建告警规则 |
| PUT | `/v1/observe/alerts/rules/:id` | 更新告警规则 |
| POST | `/v1/observe/alerts/rules/:id/toggle` | 启用/禁用告警规则 |
| DELETE | `/v1/observe/alerts/rules/:id` | 删除告警规则 |
| GET | `/v1/observe/alerts/history` | 告警触发历史 |
| POST | `/v1/observe/alerts/history/:id/resolve` | 解决告警 |

### 告警规则 JSON 结构

```json
{
  "name": "High Error Rate",
  "metric": "error_rate",
  "operator": "gt",
  "threshold": 0.1,
  "window_sec": 300,
  "severity": "critical",
  "cooldown_sec": 3600,
  "actions": [{"type": "webhook", "url": "https://..."}]
}
```

支持指标: `error_rate`, `p99_latency`, `p95_latency`, `agent_failures`, `error_count`, `avg_latency`  
支持算子: `gt`, `lt`, `gte`, `lte`, `eq`

---

## P8: Webhook Orchestration — 事件编排

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/v1/webhooks/rules` | 事件规则列表 |
| POST | `/v1/webhooks/rules` | 创建事件规则 |
| PUT | `/v1/webhooks/rules/:id` | 更新规则 |
| POST | `/v1/webhooks/rules/:id/toggle` | 启用/禁用规则 |
| DELETE | `/v1/webhooks/rules/:id` | 删除规则 |
| GET | `/v1/webhooks/logs` | 事件执行日志 |
| POST | `/v1/webhooks/logs/:id/retry` | 重试死信队列项 |
| GET | `/v1/webhooks/stats` | 统计概览 |
| GET | `/v1/webhooks/event-types` | 支持的事件类型列表 |
| POST | `/v1/webhooks/test` | 发送测试事件 |

### 事件类型

| 事件 | 说明 |
|------|------|
| `agent.error` | Agent 执行出错 |
| `agent.complete` | Agent 执行完成 |
| `chat.message` | 新聊天消息 |
| `workflow.fail` | 工作流失败 |
| `workflow.complete` | 工作流完成 |
| `alert.fired` | 告警触发 |
| `system.health` | 系统健康变更 |
| `marketplace.purchase` | 市场购买 |
| `user.login` | 用户登录 |
| `node.offline` | 节点离线 |

### 条件算子

数值: `gt`, `gte`, `lt`, `lte`, `eq`, `neq`  
字符串: `contains`, `not_contains`, `starts_with`, `ends_with`

---

## P9: Developer Platform — 开发者平台

### 公开接口（无需认证）

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/v1/developer/openapi.json` | OpenAPI 3.0 规范 JSON |
| GET | `/v1/developer/docs` | Swagger UI 文档页 |
| GET | `/v1/developer/plugins/categories` | 插件分类列表 |

### 认证接口

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/v1/developer/plugins` | 浏览插件市场 |
| GET | `/v1/developer/plugins/:id` | 插件详情 |
| POST | `/v1/developer/plugins` | 发布插件 |
| GET | `/v1/developer/plugins/mine` | 我发布的插件 |
| POST | `/v1/developer/plugins/:id/install` | 安装插件 |
| DELETE | `/v1/developer/plugins/:id/install` | 卸载插件 |
| GET | `/v1/developer/plugins/installed` | 已安装插件列表 |
| POST | `/v1/developer/plugins/:id/rate` | 评价插件 |
| POST | `/v1/developer/playground/execute` | API Playground 执行 |
| GET | `/v1/developer/playground/history` | Playground 历史记录 |
| GET | `/v1/developer/stats` | 开发者统计 |

### Admin

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/v1/admin/plugins/pending` | 待审核插件 |
| POST | `/v1/admin/plugins/:id/review` | 审核插件 |

---

## P9: Security — 安全中心

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/v1/security/encryption` | AES-256-GCM 加密状态 |
| GET | `/v1/security/overview` | 安全总览 |
| GET | `/v1/security/audit` | 审计链查询（分页 + 按操作/执行者筛选） |
| GET | `/v1/security/audit/verify` | 验证审计链完整性（Merkle 链校验） |
| GET | `/v1/security/audit/export` | 导出审计日志（JSON 格式，外部审计用） |
| GET | `/v1/security/audit/stats` | 审计统计 |
| GET | `/v1/security/gdpr/export` | GDPR Article 20 — 数据导出 |
| POST | `/v1/security/gdpr/delete` | GDPR Article 17 — 数据删除（被遗忘权） |
| GET | `/v1/security/gdpr/consent` | 用户同意状态 |
| GET | `/v1/security/compliance` | 合规检查清单（等保三级 / GDPR / SOC2） |

---

## P10: Multimodal Agent — 多模态

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/v1/multimodal/chat` | 多模态对话（text/image/audio/video） |
| GET | `/v1/multimodal/modalities` | 支持的模态列表 |

---

## P10: Proactive Goals — 自主目标

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/v1/goals` | 创建目标 |
| GET | `/v1/goals` | 目标列表 |
| GET | `/v1/goals/:id` | 目标详情（含步骤） |
| POST | `/v1/goals/:id/activate` | 激活目标（开始执行） |
| POST | `/v1/goals/:id/cancel` | 取消目标 |
| GET | `/v1/goals/stats` | 目标统计 |
| GET | `/v1/goals/decomposition-prompt` | 获取目标分解 Prompt 模板 |

### 目标生命周期

```
pending → active → completed
                 → failed
         → cancelled
```

### 目标 JSON 结构

```json
{
  "title": "优化数据库性能",
  "description": "分析慢查询并添加索引",
  "priority": "high",
  "deadline": "2025-06-01T00:00:00Z",
  "trigger_type": "manual",
  "max_steps": 20
}
```

---

## P10: Multi-Agent Collaboration — 协作

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/v1/collaborations` | 创建协作会话 |
| GET | `/v1/collaborations` | 协作列表 |
| POST | `/v1/collaborations/:id/join` | 加入协作 |
| GET | `/v1/collaborations/:id/members` | 成员列表 |
| GET | `/v1/collaborations/:id/messages` | 协作消息流 |
| POST | `/v1/collaborations/:id/messages` | 发送协作消息 |
| POST | `/v1/collaborations/:id/vote` | 提交投票 |

### 协作协议

| 协议 | 说明 |
|------|------|
| `consensus` | 共识决策（多数通过 >50%） |
| `delegation` | 委托执行 |
| `auction` | 竞标分配 |
| `voting` | 投票表决 |

### 角色

`leader`, `worker`, `reviewer`, `observer`

---

## P10: Fine-Tune & Distillation — 微调 & 蒸馏

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/v1/finetune/adapters` | LoRA 适配器列表 |
| POST | `/v1/finetune/adapters` | 创建适配器 |
| GET | `/v1/finetune/adapters/:id` | 适配器详情 |
| DELETE | `/v1/finetune/adapters/:id` | 删除适配器 |
| POST | `/v1/finetune/adapters/:id/train` | 开始训练 |
| GET | `/v1/finetune/adapters/:id/export` | 导出训练样本（JSONL） |
| GET | `/v1/finetune/adapters/:id/samples` | 训练样本列表 |
| POST | `/v1/finetune/adapters/:id/samples` | 添加训练样本 |
| POST | `/v1/finetune/adapters/:id/samples/batch` | 批量添加训练样本 |
| DELETE | `/v1/finetune/samples/:sample_id` | 删除训练样本 |
| GET | `/v1/finetune/distillation` | 蒸馏任务列表 |
| POST | `/v1/finetune/distillation` | 创建蒸馏任务 |
| GET | `/v1/finetune/distillation/:id` | 蒸馏任务详情 |
| POST | `/v1/finetune/distillation/:id/cancel` | 取消蒸馏任务 |
| GET | `/v1/finetune/distillation/prompt` | 获取蒸馏 Prompt 模板 |
| GET | `/v1/finetune/stats` | 微调统计 |

### LoRA 适配器 JSON 结构

```json
{
  "name": "customer-service-v1",
  "base_model": "qwen2.5:7b",
  "rank": 16,
  "alpha": 32,
  "target_modules": "q_proj,v_proj",
  "epochs": 3,
  "learning_rate": 0.0002,
  "batch_size": 4
}
```

### 训练样本 JSON 结构

```json
{
  "input": "用户问题",
  "output": "期望回答",
  "system": "系统提示（可选）",
  "source": "manual",
  "quality": 0.9
}
```

---

## 接口统计

| 阶段 | 模块 | 接口数 |
|------|------|--------|
| P8 | 市场 (Marketplace) | 18 |
| P8 | 可观测性 (Observe) | 11 |
| P8 | Webhook 编排 | 10 |
| P9 | 开发者平台 (Developer) | 16 |
| P9 | 安全中心 (Security) | 10 |
| P10 | 多模态 (Multimodal) | 2 |
| P10 | 自主目标 (Goals) | 7 |
| P10 | 协作 (Collaboration) | 7 |
| P10 | 微调 & 蒸馏 (FineTune) | 16 |
| **合计** | | **97** |
