# StarClaw 并行开发体系

> **三层 AI 军团：DevClaw 集群（产品层）+ 多编辑器（代码层）+ 主编辑器（指挥层）**
> 支持 Windsurf / Cursor / VS Code / JetBrains / Claude Desktop 等所有 MCP 兼容编辑器

---

## 一、全景架构

```
                        ┌─────────────────────┐
                        │    👑 你（指挥官）     │
                        │   主 Windsurf 会话    │
                        │   审查 / 合并 / 部署   │
                        └──────────┬──────────┘
                                   │
                 ┌─────────────────┼─────────────────┐
                 │                 │                 │
      ┌──────────▼──────────┐     │     ┌───────────▼──────────┐
      │   代码层 (任意编辑器)  │     │     │   产品层 (DevClaw)    │
      │                      │     │     │                      │
      │  Windsurf: memory-v2 │     │     │  DC-1: 药理虫 Agent   │
      │  Cursor:   arena     │     │     │  DC-2: 股票技能       │
      │  VS Code:  hive-fix  │     │     │  DC-3: 法务团队模板   │
      │                      │     │     │  DC-4: 竞品分析工作流  │
      └──────────────────────┘     │     └──────────────────────┘
                                   │
                        ┌──────────▼──────────┐
                        │    基础设施层         │
                        │  CI (.github/ci.yml) │
                        │  CD (nydus hook)     │
                        │  Branch protection   │
                        └─────────────────────┘
```

### 三层职责

| 层 | 工具 | 产出 | 冲突风险 |
|----|------|------|----------|
| **指挥层** | 主编辑器 (任意 MCP 编辑器) | 架构决策、代码审查、merge、部署、协调 | — |
| **代码层** | N 个编辑器会话 (Windsurf/Cursor/VS Code/...) | Go/React 代码、基础设施、配置 | 同文件修改 |
| **产品层** | N 个 DevClaw 实例 | Agent、Skill、Workflow、Team Template | 零（各实例独立） |

---

## 二、代码层并行 — 多 Windsurf 模式

### 2.1 分支策略

```
master              ← 保护分支，只通过合并进入
  │
  ├── feat/claw/memory-v2        ← Windsurf-1
  ├── feat/queen/arena-rank      ← Windsurf-2
  ├── feat/spore/sandbox-test    ← Windsurf-3
  ├── fix/hive/docker-internal   ← Windsurf-4
  └── hotfix/auth-crash          ← 紧急修复（可直接合 master）
```

命名规范: `{type}/{service}/{name}`
- `feat/` — 新功能
- `fix/` — 修复
- `refactor/` — 重构
- `hotfix/` — 紧急修复

### 2.2 Windsurf 会话规则

每个 Windsurf 会话**只在自己的分支上工作**，遵守：

1. **开始前**: `git checkout -b feat/{service}/{name}` 从最新 master 创建分支
2. **开发中**: 只改自己负责的服务目录，commit 信息带服务前缀
3. **完成后**: 告知主 Windsurf，等待审查合并
4. **禁止**: 直接 push master、修改其他分支的文件

### 2.3 服务独立性矩阵

以下服务**完全独立**，可同时由不同 Windsurf 开发，零冲突：

| 服务 | 目录 | go.mod | 部署目标 |
|------|------|--------|----------|
| Claw API | `claw/api/` | 独立 | Server A |
| Claw Web | `claw/web/` | npm | Server A |
| Hive | `hive/api/` | 独立 | Server A |
| Queen API | `queen/api/` | 独立 | Server C |
| Queen Swarm | `queen/swarm/` | 独立 | Server C |
| Queen Arena | `queen/arena/` | 独立 | Server C |
| Synapse API | `synapse/api/` | 独立 | Server B |
| Synapse Core | `synapse/core/` | npm | Server B |
| Overlord | `overlord/api/` | 独立 | Server C |
| Nydus | `nydus/api/` | 独立 | Server C |
| Spore | `spore/` | 独立 | 客户端 |
| Carapace | `carapace/` | 独立 | 库 |

### 2.4 冲突高危文件

多个 Windsurf 同时改这些文件时需要协调：

```
claw/api/internal/router/router.go    ← 路由注册（所有 claw 功能都改这里）
claw/web/src/App.tsx                  ← 前端路由
queen/api/internal/router/router.go   ← Queen 路由
docker-compose.*.yml                  ← 容器编排
nydus/hooks/post-receive-starclaw     ← 部署脚本
```

### 2.5 主 Windsurf 合并流程

```
Windsurf-1 完成 feat/claw/memory-v2:
  │
  ├── 1. Windsurf-1: git push nydus feat/claw/memory-v2
  │
  ├── 2. 主 Windsurf: 审查代码
  │      git fetch nydus
  │      git log nydus/feat/claw/memory-v2 --oneline -10
  │      git diff master..nydus/feat/claw/memory-v2 --stat
  │
  ├── 3. 主 Windsurf: 合并
  │      git checkout master
  │      git merge --no-ff nydus/feat/claw/memory-v2
  │
  ├── 4. 主 Windsurf: 推送部署
  │      git push nydus master
  │      (nydus hook 自动检测变更目录并部署对应服务)
  │
  └── 5. 主 Windsurf: 通知其他 Windsurf rebase
         "master 已更新，请 git rebase nydus/master"
```

---

## 三、产品层并行 — DevClaw 集群模式

### 3.1 DevClaw 实例隔离

每个 DevClaw 实例是一个**独立的 Overlord Team Agent 实例**，自带：
- 5 个 AI 角色（设计虫/编码虫/测试虫/审查虫/文档虫）
- 独立的对话上下文和任务列表
- 独立的沙箱测试环境
- 独立的发布管道

```
Overlord
  ├── DevClaw-1 "Agent 工厂"     → 专做 Agent 开发
  │     ├── 任务: 开发药理虫
  │     ├── 任务: 开发客服虫
  │     └── 任务: 开发翻译虫
  │
  ├── DevClaw-2 "技能工坊"       → 专做 Skill/Plugin 开发
  │     ├── 任务: 股票行情技能
  │     └── 任务: 天气查询技能
  │
  └── DevClaw-3 "团队设计室"     → 专做 Team Template 设计
        ├── 任务: 法务团队模板
        └── 任务: 营销团队模板
```

### 3.2 DevClaw vs Windsurf 职责边界

| 任务类型 | 谁做 | 原因 |
|----------|------|------|
| 开发新 Agent (Prompt + 配置) | **DevClaw** | 对话驱动，无需 IDE |
| 开发 JSON Plugin (包装 API) | **DevClaw** | 纯 JSON 配置 |
| 开发 Workflow (流程编排) | **DevClaw** | 可视化 DAG |
| 开发 Team Template | **DevClaw** | 角色 + 拓扑设计 |
| 开发 Go Built-in Tool | **Windsurf** | 需要编码 + 编译 |
| 修改 Claw API/Web | **Windsurf** | Go/React 代码 |
| 修改基础设施 (Docker/Nginx/CI) | **主 Windsurf** | 影响全局 |
| 数据库 migration | **主 Windsurf** | 需要协调 |
| 架构决策 | **你 + 主 Windsurf** | 人类拍板 |

### 3.3 DevClaw 产出 → 代码层衔接

DevClaw 有时会产出需要代码层实现的需求：

```
DevClaw: "这个 Agent 需要一个自定义技能：查询医保目录"
    │
    ▼ 产出 PluginSpec JSON 或需求文档
    │
    ▼ 转发给 Windsurf
    │
Windsurf-N: feat/claw/medical-tool
    │ 实现 Go Tool → 注册到 Registry → 提交
    │
    ▼
主 Windsurf: 审查 + 合并
    │
    ▼
DevClaw: 技能可用 → 继续 Agent 测试 → 上架
```

---

## 四、典型并行开发场景

### 场景 A: 产品 + 代码同时推进

```
时间线:
  09:00  你: "今天计划：优化记忆系统 + 上架 3 个新 Agent + Arena 排名算法"
  
  09:05  分配任务:
         DevClaw-1 → 开发药理虫 Agent
         DevClaw-2 → 开发翻译虫 Agent  
         DevClaw-3 → 开发客服虫 Agent
         Windsurf-1 → feat/claw/memory-v2 (记忆系统向量化)
         Windsurf-2 → feat/queen/arena-rank (排名算法)

  09:05~11:00  各自并行工作，互不干扰

  11:00  DevClaw-1 完成: 药理虫上架到市场 ✅
         DevClaw-2 完成: 翻译虫上架到市场 ✅
         Windsurf-1 完成: memory-v2 等待审查

  11:10  主 Windsurf: 审查 memory-v2 → 合并 master → 部署 ✅
         主 Windsurf: 通知 Windsurf-2 rebase

  12:00  DevClaw-3 完成: 客服虫上架 ✅
         Windsurf-2 完成: arena-rank 等待审查

  12:10  主 Windsurf: 审查 arena-rank → 合并 → 部署 ✅

  结果: 半天内完成 3 个 Agent + 2 个代码功能
```

### 场景 B: DevClaw 触发代码需求

```
  DevClaw: "药理虫需要查询医保目录的能力，现有技能不够"
    │
    ├── DevClaw 产出需求: { name: "medical_catalog", parameters: {...} }
    │
    ├── 你: "Windsurf-3，接一下这个需求"
    │
    ├── Windsurf-3: feat/claw/medical-tool → 实现 Go Tool
    │
    ├── 主 Windsurf: 审查 → 合并 → 部署
    │
    └── DevClaw: 新技能可用 → 药理虫加上 medical_catalog → 重新测试 → 上架
```

---

## 五、实操指南

### 5.1 启动并行开发

#### 代码层 (Windsurf)

```bash
# 每个 Windsurf 会话开始时:
git checkout master
git pull nydus master
git checkout -b feat/{service}/{feature}

# 工作中:
# ... 正常开发 + commit ...

# 完成时:
git push nydus feat/{service}/{feature}
# 通知主 Windsurf 审查
```

#### 产品层 (DevClaw)

```
1. 登录 Overlord (overlord.starclaw.me)
2. 团队智能体 → 新建实例 → 选 DevClaw 模板
3. 对话: "开发一个 [描述] Agent"
4. DevClaw 五虫协作 → 设计 → 实现 → 测试 → 审查 → 上架
5. 如需自定义技能 → 转发给 Windsurf 会话实现
```

#### 主 Windsurf（你的主会话）

```
职责:
  1. 分配任务给各 Windsurf 和 DevClaw
  2. 审查代码变更 (git diff)
  3. 合并分支 + 推送部署
  4. 协调冲突
  5. 做架构决策和全局基础设施改动
```

### 5.2 CI/CD 支持

`.github/workflows/ci.yml` 已配置:
- PR 打开 → `dorny/paths-filter` 检测变更目录
- 只构建受影响的服务 (Go vet+build / npm ci+build)
- 全部通过才允许合并

`nydus/hooks/post-receive-starclaw` 已配置:
- push master → 检测变更目录 → 只部署变更的服务
- claw/ → Server A, synapse/ → Server B, queen/ → Server C

---

## 六、Dev Bridge MCP 工具

Dev Bridge 是 DevClaw ↔ 编辑器桥接的核心。标准 MCP JSON-RPC 2.0 over HTTP，**任何 MCP 兼容编辑器都能直接接入**。

### 6.1 架构

```
  Dev Bridge (:9102)              MCP Bridge (:9101)
  开发流程控制                      宿主机控制
  git/task/build/agent             shell/file/GUI
       │                                │
       └────── Claw 自动发现两个 Bridge ──┘
                      │
     ┌────────────────┼────────────────┐
     │                │                │
  Windsurf       Cursor          VS Code
  (主编辑器)     (开发者 B)      (开发者 C)
```

### 6.2 15 个工具

| 分类 | 工具 | 用途 |
|------|------|------|
| **Git** | `git_status` | 当前分支 + 工作区状态 |
| | `git_branches` | 列出所有功能分支 |
| | `git_create_branch` | 创建新功能分支 (强制命名规范) |
| | `git_diff` | 分支与 master 的差异 |
| | `git_merge` | 合并分支到 master (auto fetch+rebase) |
| | `git_log` | 查看提交记录 |
| **Task** | `task_create` | DevClaw→编辑器 或反向创建任务 |
| | `task_list` | 查看任务列表 (按状态/目标筛选) |
| | `task_update` | 更新任务状态/添加备注 |
| **Service** | `service_list` | 列出 13 个服务 + 最后修改时间 |
| | `service_build` | 构建指定服务验证编译 |
| | `deploy_push` | git push nydus master 触发部署 |
| **Agent** | `agent_test` | 沙箱测试 Agent 配置 (via Claw API) |
| | `agent_publish` | 发布 Agent 到市场 |

### 6.3 启动 Dev Bridge

```bash
# 在 starclaw 仓库根目录
go run claw/api/cmd/dev-bridge/main.go -port 9102 -repo . -claw http://localhost:8080

# 或编译后运行
cd claw/api && go build -o dev-bridge ./cmd/dev-bridge/
./dev-bridge -port 9102 -repo /path/to/starclaw
```

Claw 启动时自动探测 `localhost:9102`，发现即注册所有工具。

---

## 七、多编辑器支持

Dev Bridge 是标准 MCP 服务器，不绑定任何 IDE。以下编辑器均可直接接入：

| 编辑器 | MCP 支持 | 配置方式 |
|--------|---------|----------|
| **Windsurf** | ✅ 原生 | Claw Settings → MCP 工具 → 添加 URL |
| **Cursor** | ✅ 原生 | `~/.cursor/mcp.json` |
| **VS Code** | ✅ Copilot Chat | `.vscode/settings.json` |
| **Claude Desktop** | ✅ 原生 | `claude_desktop_config.json` |
| **JetBrains** | ✅ AI Assistant 插件 | MCP Servers 设置 |
| **Cline (VS Code)** | ✅ 原生 | MCP 配置面板 |

### Cursor 配置

```json
// ~/.cursor/mcp.json
{
  "mcpServers": {
    "starclaw-dev": {
      "url": "http://localhost:9102"
    },
    "starclaw-host": {
      "url": "http://localhost:9101"
    }
  }
}
```

### VS Code (Copilot) 配置

```json
// .vscode/settings.json
{
  "github.copilot.chat.mcpServers": {
    "starclaw-dev": {
      "url": "http://localhost:9102"
    }
  }
}
```

### Claude Desktop 配置

```json
// claude_desktop_config.json
{
  "mcpServers": {
    "starclaw-dev": {
      "command": "path/to/dev-bridge",
      "args": ["-port", "9102", "-repo", "/path/to/starclaw"]
    }
  }
}
```

### 混合编辑器并行开发

```
开发者 A (Windsurf):  feat/claw/memory-v2   ─┐
开发者 B (Cursor):    feat/queen/arena-rank  ├─ 共享 Dev Bridge (:9102)
开发者 C (VS Code):   feat/spore/sandbox     ─┘  共享 Git 仓库
                                                  共享任务列表
DevClaw-1 (Claw Agent): 开发药理虫 Agent     ─── 通过 task_create 请求代码支持
DevClaw-2 (Claw Agent): 开发翻译虫 Agent     ─── 独立工作，自动上架
```

---

## 八、扩展能力

### 8.1 当前规模

| 并行通道 | 数量 | 限制因素 |
|---------|------|----------|
| 编辑器会话 | 不限 | Windsurf/Cursor/VS Code 任意组合 |
| DevClaw 实例 | 不限 | Overlord 节点算力 |
| Git 分支 | 不限 | 12 个独立服务 = 12 个并行通道 |
| MCP Bridge | 2 个 | host (:9101) + dev (:9102) |

### 8.2 未来增强

| 增强 | 说明 | 优先级 |
|------|------|--------|
| 自动 PR 审查 | DevClaw 审查虫复审代码 | 高 |
| 分支部署预览 | 非 master 分支 → 临时预览环境 | 中 |
| DevClaw → 编辑器自动桥接 | DevClaw 需要代码时自动创建 Task | ✅ 已实现 |
| 任务看板 Web UI | Dev Bridge 内置任务看板页面 | 低 |
| SSE 实时通知 | Dev Bridge 推送任务变更到编辑器 | 低 |

---

## 九、规则总结

```
Rule 1: master 是生产分支，不直接 push
Rule 2: 每个编辑器会话一个分支，只改自己的服务目录
Rule 3: DevClaw 做产品层（Agent/Skill/Workflow），编辑器做代码层
Rule 4: 主编辑器负责审查、合并、部署、协调
Rule 5: 合并后通知其他编辑器 rebase
Rule 6: 冲突高危文件（router.go 等）由主编辑器统一改
Rule 7: 紧急 hotfix 可以跳过分支直接合 master
```
