# StarClaw 🦞 用户手册

> StarClaw 是一个自托管的 AI Agent 平台，支持多模型、多工具、工作流自动化、P2P 算力共享和丰富的运维能力。

---

## 目录

1. [快速开始](#快速开始)
2. [核心功能](#核心功能)
3. [Agent 管理](#agent-管理)
4. [对话系统](#对话系统)
5. [模型配置](#模型配置)
6. [知识库 (RAG)](#知识库-rag)
7. [工作流自动化](#工作流自动化)
8. [自主任务](#自主任务)
9. [技能 & MCP](#技能--mcp)
10. [可观测性 (P8)](#可观测性)
11. [Webhook 编排 (P8)](#webhook-编排)
12. [开发者平台 (P9)](#开发者平台)
13. [安全中心 (P9)](#安全中心)
14. [自主目标 (P10)](#自主目标)
15. [微调 & 蒸馏 (P10)](#微调--蒸馏)
16. [星能经济](#星能经济)
17. [国际化 (i18n)](#国际化)
18. [CLI 命令](#cli-命令)
19. [部署指南](#部署指南)

---

## 快速开始

### 1. 首次初始化

访问 `http://your-server:8081`，首次打开会进入初始化向导：

- **用户名**（可选）：留空自动生成 `Claw#XXXX`
- **密码**（可选）：对外暴露时建议设置

点击「开始使用」后会生成 **Owner Token**，自动保存到浏览器。

### 2. 添加模型

进入 **设置 → 模型** 页面，添加 AI 模型配置：

| Provider | 配置项 |
|----------|--------|
| OpenAI | API Key + 模型名 (gpt-4o, gpt-4o-mini) |
| Anthropic | API Key + 模型名 (claude-sonnet-4-20250514) |
| DeepSeek | API Key + 模型名 (deepseek-chat) |
| Ollama | Base URL (http://localhost:11434) + 模型名 |
| star-ai | 无需配置，自动注册（使用节点身份签名认证） |

### 3. 开始对话

进入 **对话** 页面，选择一个 Agent，开始输入消息即可。

---

## 核心功能

| 功能 | 说明 |
|------|------|
| **多模型支持** | OpenAI / Anthropic / DeepSeek / Qwen / Ollama / star-ai |
| **Agent 系统** | 自定义 Agent + 内置 Agent + 市场安装 |
| **RAG 知识库** | 文档上传 → 分块 → 向量化 → 检索增强生成 |
| **工作流** | 可视化编排，支持 Webhook 触发和定时执行 |
| **自主任务** | 后台异步执行，7×24 自主运行 |
| **MCP 工具** | 标准化工具协议，支持外部 MCP 服务 |
| **P2P 算力共享** | 有 GPU 的节点为其他节点提供推理服务 |
| **星能经济** | 节点间价值交换，Ed25519 签名转账 |

---

## Agent 管理

### 创建 Agent

1. 进入 **智能体** 页面
2. 点击「创建 Agent」
3. 填写名称、描述、系统提示词
4. 选择模型和工具
5. （可选）关联知识库

### Agent 配置

- **系统提示词**: 定义 Agent 角色和行为
- **模型**: 绑定特定模型或使用默认
- **工具**: 选择启用的技能（代码执行、网页搜索、浏览器等）
- **知识库**: 关联 RAG 知识库进行检索增强
- **公开**: 设为公开后其他用户可见

### 市场安装

从 **市场** 页面浏览和安装社区分享的 Agent。

---

## 对话系统

### 对话命令

在聊天输入框中输入以下命令：

| 命令 | 说明 |
|------|------|
| `/model` | 列出所有可用模型 |
| `/model <name>` | 切换当前对话模型 |

### 消息操作

- **反馈**: 点击消息旁的 👍/👎 标记质量
- **置顶**: 右键对话 → 置顶
- **导出**: 导出整个对话为 JSON
- **截断**: 从指定消息处截断上下文

---

## 模型配置

### 模型优先级（从高到低）

1. 对话级覆盖（`/model` 命令）
2. Agent 设置中绑定的模型
3. 用户创建的第一个启用模型
4. 平台共享模型

### star-ai 网关

StarClaw 内置 star-ai 网关，用一个入口调用所有主流模型：

```
Base URL: https://star-ai.net/v1
认证: Ed25519 节点签名（自动，无需 API Key）
```

---

## 知识库 (RAG)

1. **创建知识库**: 知识库 → 新建
2. **上传文档**: 支持 PDF, TXT, MD, DOCX 等格式
3. **自动处理**: 分块 → 向量化 (text-embedding-3-small) → 存储
4. **关联 Agent**: 在 Agent 设置中绑定知识库
5. **检索增强**: 对话时自动检索相关内容注入上下文

---

## 工作流自动化

### 创建工作流

使用可视化编辑器拖拽节点创建工作流：

- **触发器**: 手动 / Webhook / 定时
- **节点类型**: LLM / 条件 / 循环 / 工具调用 / 代码执行
- **输出**: 变量传递、最终结果

### Webhook 触发

启用 Webhook 后会生成唯一 URL：
```
POST /v1/webhooks/workflow/<token>
```

### 定时执行

支持 cron 表达式定时触发工作流。

---

## 自主任务

后台异步执行的长时间任务，支持：

- **创建**: 描述目标，系统自动分解执行
- **暂停/恢复**: 随时控制任务状态
- **取消**: 终止执行中的任务
- **可视化**: 查看任务执行图谱

---

## 技能 & MCP

### 内置技能

| 技能 | 说明 |
|------|------|
| `web_search` | 网页搜索 |
| `code` | 沙箱代码执行 |
| `browser` | 无头浏览器操作 |
| `http_request` | HTTP 请求 |
| `video_generation` | AI 视频生成 |
| `system` | Agent 间委托 |

### MCP 外部服务

在 **技能 & MCP** 页面添加外部 MCP 服务器，扩展 Agent 能力。

---

## 可观测性

> P8 新增 — 运维 → 可观测性

### 功能

- **总览**: Trace 总数、Span 数、活跃告警、日志统计
- **链路追踪**: 查看完整调用链（Agent → LLM → Tool → RAG）
- **结构化日志**: 按级别 (debug/info/warn/error) 查询
- **告警规则**: 基于指标自动触发（error_rate、p99 延迟等）
- **告警历史**: 查看触发记录，标记已解决

### 告警配置

支持 6 种指标 × 5 种算子 × 3 种严重度（info/warning/critical）。

---

## Webhook 编排

> P8 新增 — 运维 → Webhook

### 功能

- **规则管理**: IF 事件匹配条件 THEN 执行动作
- **事件日志**: 查看所有事件处理记录
- **死信队列**: 失败事件自动进入死信，支持手动重试
- **测试事件**: 发送测试事件验证规则

### 10 种事件类型

`agent.error` / `agent.complete` / `chat.message` / `workflow.fail` / `workflow.complete` / `alert.fired` / `system.health` / `marketplace.purchase` / `user.login` / `node.offline`

---

## 开发者平台

> P9 新增 — 运维 → 开发者

### 功能

- **API 文档**: 自动生成 OpenAPI 3.0 规范 + Swagger UI
- **插件市场**: 浏览、安装、发布插件
- **API Playground**: 在线测试 API，自动记录历史

### 插件分类

`api` / `data` / `productivity` / `dev` / `media` / `finance` / `social`

---

## 安全中心

> P9 新增 — 运维 → 安全中心

### 功能

| Tab | 说明 |
|-----|------|
| **总览** | 加密状态、审计统计、安全评分 |
| **审计链** | Merkle 链式审计日志，支持完整性校验和导出 |
| **GDPR** | 数据导出（Article 20）、数据删除（Article 17）、同意管理 |
| **合规** | 等保三级 / GDPR / SOC2 检查清单 |

### 加密

- **算法**: AES-256-GCM
- **密钥管理**: STARCLAW_MASTER_KEY 环境变量或自动生成
- **密钥派生**: SHA-256 HKDF 按用途派生

---

## 自主目标

> P10 新增 — 智能 → 自主目标

### 目标驱动执行

1. 创建目标（标题、描述、优先级、截止日期）
2. 系统自动分解为步骤（think / tool_call / sub_goal / decide / report）
3. 后台自主执行，支持定时触发和手动触发
4. 实时查看进度和步骤详情

### 协作模式

- **共识决策**: 多 Agent 投票，>50% 通过
- **委托执行**: Leader 分配任务给 Worker
- **竞标分配**: Agent 竞标任务
- **投票表决**: 成员投票决定方案

---

## 微调 & 蒸馏

> P10 新增 — 智能 → 微调 & 蒸馏

### LoRA 适配器

1. **创建**: 选择基座模型、设置 Rank/Alpha/Epochs/LR
2. **添加样本**: 手动添加或批量导入 input/output 对
3. **训练**: 一键启动微调训练
4. **导出**: 导出为 JSONL 格式

### 知识蒸馏

将大模型（Teacher）的能力蒸馏到小模型（Student）：

1. 设置 Teacher/Student 模型
2. 提供种子 Prompt
3. 设置目标样本数
4. 系统自动生成训练数据

---

## 星能经济

### 血量系统 (HP)

| HP 状态 | 余额 | 说明 |
|---------|------|------|
| `full` | >1000⚡ | 满血 |
| `healthy` | 100–1000⚡ | 健康 |
| `low` | 10–100⚡ | 低血量 |
| `critical` | 1–10⚡ | 危急 |
| `hibernated` | 0⚡ | 休眠 |

### 操作

- **查余额**: 侧栏底部 HP 条，或 CLI `starclaw balance`
- **转账**: 设置 → 星能 → 转账
- **交易记录**: 设置 → 星能 → 历史

---

## 国际化

StarClaw 支持中文和英文两种语言，在侧栏底部点击 **EN/中** 按钮切换。

语言设置会持久化到浏览器 localStorage，刷新后保持。

覆盖范围：侧栏导航、通知、授权弹窗、更新提示、通用按钮文本、所有 P8-P10 页面标题和标签。

---

## CLI 命令

| 命令 | 说明 |
|------|------|
| `starclaw get-token` | 查看 Owner Token |
| `starclaw reset-token` | 重新生成 Token |
| `starclaw reset-password --password xxx` | 重置密码 |
| `starclaw devices` | 列出授权设备 |
| `starclaw approve <id>` | 审批设备 |
| `starclaw reject <id>` | 拒绝/撤销设备 |
| `starclaw export-key` | 导出 24 词助记词 |
| `starclaw import-key <words>` | 恢复身份 |
| `starclaw wallet-info` | 查看钱包信息 |
| `starclaw balance` | 查看星能余额 |
| `starclaw transfer <addr> <amount>` | 转账 |
| `starclaw transactions` | 交易记录 |
| `starclaw version` | 版本号 |

---

## 部署指南

### Docker Compose（推荐）

```bash
cd /opt/starclaw
docker compose -f docker-compose.prod.yml up -d
```

### 服务端口

| 服务 | 端口 | 说明 |
|------|------|------|
| API | 8080 | 后端接口 |
| Web | 8081 | 前端页面 |
| MySQL | 3306 | 数据库 |
| Redis | 6379 | 缓存 |

### 一键部署

```bash
# Windows
scripts\deploy.bat "commit message" all

# Linux/macOS
bash scripts/deploy.sh "commit message" all
```

### Nydus CI/CD

项目支持 Nydus CI/CD 系统自动部署，配置文件: `nydus.pipeline.yml`

流程: `git push → test → build → deploy → health check → notify`

---

## 系统要求

| 项目 | 最低要求 | 推荐 |
|------|---------|------|
| CPU | 2 核 | 4 核 |
| 内存 | 2 GB | 4 GB |
| 磁盘 | 10 GB | 50 GB |
| Docker | 20.x | 24.x |
| Node.js | 18.x | 20.x |

### 可选依赖

| 依赖 | 用途 |
|------|------|
| Ollama | 本地模型推理 |
| Chromium | PDF 转换、浏览器工具 |
| FFmpeg | 视频处理 |

---

*StarClaw — 你的私人 AI 助手 🦞*
