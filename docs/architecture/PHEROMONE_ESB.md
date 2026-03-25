# Pheromone — StarClaw 企业服务总线

> 信息素（Pheromone）— 虫群的化学通信网络。每只虫子释放信号，所有同类感知并响应。蚂蚁用它标记路径，蜜蜂用它召集同伴，虫群用它协调百万个体的集体行为。

## 一、命名由来

在虫族体系中，各系统的命名遵循生物学隐喻：

| 系统 | 虫族名 | 生物学含义 | 职责 |
|------|--------|----------|------|
| **服务总线** | **Pheromone（信息素）** | 虫群化学通信网络 | 服务发现、消息路由、API 网关 |
| 执行节点 | Claw（螯足） | 小龙虾的钳子 | AI Agent 执行单元 |
| 企业管控 | Overlord（领主） | 虫群的统治者 | RBAC/SSO/审计 |
| 中央控制 | Queen（虫后） | 蚁后/蜂后 | 全局协调、注册、计费 |
| 代码分发 | Nydus（虫洞） | 星际虫洞通道 | Git 仓库、代码部署 |
| 加密库 | Carapace（甲壳） | 外骨骼保护层 | AES/KDF/Vault |
| AI 算力 | Synapse（突触） | 神经突触 | 模型路由、推理网关 |
| 开发平台 | Forge（熔炉） | 锻造兵器的熔炉 | Agent/Skill 开发 |
| 移动端 | Larva（幼虫） | 昆虫幼体 | Flutter App |
| 安装器 | Spore（孢子） | 真菌孢子，轻量扩散 | 一键安装 |
| P2P 通信 | Gossip（蜂语） | 蜜蜂的信息素通信 | 节点间消息传递 |
| **服务总线** | **Pheromone（信息素）** | 群体信号协调 | **连接一切** |

## 二、为什么需要 Pheromone

### 当前问题

```
现在的架构（点对点直连）：

Queen ──HTTP──> Synapse (StarAI)     ← 跨机房，网络不稳定
Queen ──HTTP──> Bounty               ← 同机器，容器间
Queen ──HTTP──> Forum                ← 同机器，容器间
Queen ──HTTP──> Arena                ← 同机器，容器间
Queen ──HTTP──> Overlord             ← 同机器
Claw  ──HTTP──> Queen (Swarm)        ← 跨公网
Claw  ──HTTP──> Synapse (StarAI)     ← 跨公网
Forge ──HTTP──> Queen                ← 同机器
```

问题：
1. **服务发现硬编码** — 每个服务的 URL 写死在配置里
2. **跨机房不稳定** — Server C ↔ Server B 网络不通（刚遇到的问题）
3. **无统一认证** — 每对服务有自己的 token 机制
4. **无消息队列** — 所有调用都是同步 HTTP，一个服务挂全链路阻塞
5. **无可观测性** — 服务间调用没有统一日志和监控

### 目标架构

```
Pheromone 架构（虫群通信网络）：

                    ┌─────────────────────┐
                    │  Pheromone (信息素)    │
                    │   NATS / Redis Pub   │
                    │   Service Registry   │
                    │   API Gateway        │
                    └──────────┬──────────┘
           ┌──────────┬────────┼────────┬──────────┐
           │          │        │        │          │
        Queen      Synapse  Overlord  Forge      Claw×N
        (虫后)     (突触)    (领主)    (熔炉)    (螯足)
           │          │        │        │          │
        Bounty     StarAI   Console  DevClaw    Agent
        Forum      Proxy    Web      Skills     Worker
        Arena                                   Gossip
        Swarm
```

## 三、Pheromone 技术方案

### 核心组件

| 组件 | 技术选型 | 说明 |
|------|---------|------|
| **消息总线** | NATS (轻量) 或 Redis Streams | 异步事件、发布订阅 |
| **服务注册** | 内置注册表 (Go map + 心跳) | 服务自动注册/发现 |
| **API 网关** | 反向代理 (Go net/http) | 统一入口、鉴权、限流 |
| **配置中心** | etcd 或 Viper 远程 | 统一配置下发 |

### 选择 NATS 的理由

- **极轻量**：单二进制，<20MB 内存
- **Go 原生**：与 StarClaw 技术栈完美匹配
- **发布/订阅 + 请求/响应**：两种模式都支持
- **JetStream**：持久化消息，支持消息重放
- **集群模式**：3 节点即可高可用

### 通信模式

```yaml
# 1. 事件广播（发布/订阅）
pheromone.events.user.registered    # 新用户注册 → 所有服务收到
pheromone.events.order.paid         # 订单支付 → 投资/计费/通知
pheromone.events.claw.heartbeat     # Claw 心跳 → 监控/统计

# 2. 服务间请求（请求/响应）
pheromone.rpc.synapse.create-order  # Queen → Synapse 创建支付订单
pheromone.rpc.queen.verify-token    # 任何服务 → Queen 验证令牌
pheromone.rpc.overlord.check-rbac   # Queen → Overlord 权限检查

# 3. 任务队列（工作队列）
pheromone.queue.bounty.notify       # 赏金通知队列
pheromone.queue.email.send          # 邮件发送队列
pheromone.queue.deploy.trigger      # 部署触发队列
```

## 四、数据盘使用方案

Server C 新增 200GB 数据盘建议用法：

```bash
# 挂载数据盘到 /data
sudo mkfs.ext4 /dev/vdb                    # 格式化（首次）
sudo mkdir -p /data
sudo mount /dev/vdb /data
echo '/dev/vdb /data ext4 defaults 0 2' >> /etc/fstab  # 开机自动挂载

# 目录规划
/data/
├── mysql/          # Queen MySQL 数据 (从 docker volume 迁移)
├── redis/          # Redis 持久化数据
├── nydus/
│   ├── repos/      # Git 裸仓库 (starclaw.git, claw.git)
│   └── releases/   # 编译产物和发布包
├── pheromone/
│   ├── nats/       # NATS JetStream 持久化
│   └── logs/       # 服务总线日志
├── backups/        # 每日备份
└── logs/           # 统一日志归档
```

## 五、微服务化架构 — 独立仓库模式

### 从 Monorepo 到 Polyrepo

每个子项目拆分为**完全独立的 Git 仓库**，每个用独立的 Windsurf 实例开发：

```
当前 Monorepo (starclaw.git):        目标 Polyrepo:
starclaw/                             
├── claw/        ──────────→  nydus.starclaw.net:claw.git         (已有，开源)
├── queen/       ──────────→  nydus.starclaw.net:queen.git        (新建)
├── synapse/     ──────────→  nydus.starclaw.net:synapse.git      (新建)
├── overlord/    ──────────→  nydus.starclaw.net:overlord.git     (新建)
├── forge/       ──────────→  nydus.starclaw.net:forge.git        (新建)
├── hive/        ──────────→  nydus.starclaw.net:hive.git         (新建)
├── cerebrate/   ──────────→  nydus.starclaw.net:cerebrate.git    (新建)
├── larva/       ──────────→  nydus.starclaw.net:larva.git        (新建)
├── pheromone/   ──────────→  nydus.starclaw.net:pheromone.git    (新建)
├── spore/       ──────────→  nydus.starclaw.net:spore.git        (新建)
├── carapace/    ──────────→  nydus.starclaw.net:carapace.git     (新建)
├── nydus/       ──────────→  nydus.starclaw.net:nydus.git        (新建)
└── starclaw.git ──────────→  保留为归档/协调仓库 (不再日常开发)
```

### Windsurf 多实例开发

每个子项目 = 1 个独立 Windsurf 实例 = 1 个独立 Git 仓库：

```
┌─ Windsurf #1 ─┐  ┌─ Windsurf #2 ─┐  ┌─ Windsurf #3 ─┐
│ E:\claw\       │  │ E:\queen\      │  │ E:\synapse\    │
│ claw.git       │  │ queen.git      │  │ synapse.git    │
│ AI: Claw Dev   │  │ AI: Queen Dev  │  │ AI: StarAI Dev │
└────────────────┘  └────────────────┘  └────────────────┘

┌─ Windsurf #4 ─┐  ┌─ Windsurf #5 ─┐  ┌─ Windsurf #6 ─┐
│ E:\overlord\   │  │ E:\forge\      │  │ E:\pheromone\  │
│ overlord.git   │  │ forge.git      │  │ pheromone.git  │
│ AI: Overlord   │  │ AI: Forge Dev  │  │ AI: ESB Dev    │
└────────────────┘  └────────────────┘  └────────────────┘

┌─ Windsurf #0 ─────────────────────────────────────────┐
│ E:\starclaw\  (你 — 总控/协调/架构决策/发布管理)        │
│ starclaw.git  (monorepo 归档，只读参考)                 │
└───────────────────────────────────────────────────────┘
```

### Nydus 独立仓库管理

每个子项目在 Nydus 上有自己的 bare repo + post-receive hook：

```bash
# Nydus Server C: /data/nydus/repos/
/data/nydus/repos/
├── starclaw.git     # 归档 monorepo（保留历史）
├── claw.git         # ✅ 已存在（开源同步 GitHub）
├── queen.git        # 新建
├── synapse.git      # 新建
├── overlord.git     # 新建
├── forge.git        # 新建
├── hive.git         # 新建
├── pheromone.git    # 新建
├── cerebrate.git    # 新建
├── larva.git        # 新建
├── spore.git        # 新建
├── carapace.git     # 新建
└── nydus.git        # 新建

# 每个 repo 独立的 post-receive hook:
claw.git/hooks/post-receive      → 部署到 Server A (starclaw.me)
queen.git/hooks/post-receive     → 部署到 Server C (starclaw.net) 本地
synapse.git/hooks/post-receive   → 部署到 Server B (star-ai.net)
overlord.git/hooks/post-receive  → 部署到 Server C 本地
forge.git/hooks/post-receive     → 部署到 Server C 本地
pheromone.git/hooks/post-receive → 部署到 Server C 本地（总线核心）
hive.git/hooks/post-receive      → 部署到 Server A (starclaw.me)
```

### 共享依赖：Pheromone SDK

各服务通过 Go module 引用 Pheromone SDK：

```go
// queen/api/go.mod
module starclaw.net/queen/api

require (
    starclaw.net/pheromone v0.1.0  // Pheromone SDK（服务注册+消息）
    starclaw.net/carapace  v0.1.0  // 加密库
)
```

Go vanity import 已经配好（starclaw.net/* → nydus.starclaw.net/git/*.git），
各独立仓库可直接 `go get starclaw.net/pheromone` 引用。

### Forge 对接流程

```
1. Windsurf #2 (Claw Dev) 开发新功能
   → git push nydus master (推到 claw.git)
   → Nydus post-receive hook 触发：
     a. 编译 + 测试
     b. 部署到 Server A
     c. 通过 Ganglion 广播: pheromone.events.deploy.claw.completed

2. Forge 监听 Ganglion 事件
   → 记录构建历史、版本变更
   → 自动通知相关 Windsurf 实例（如果有 API 依赖变更）

3. 跨服务依赖更新
   → pheromone.git 更新了 SDK
   → Forge 自动检测哪些服务依赖了 pheromone
   → 通知相关 Windsurf 实例更新 go.mod
```

### 迁移步骤

```bash
# 1. 在 Nydus 上创建独立 bare repos
for repo in queen synapse overlord forge hive pheromone cerebrate larva spore carapace nydus; do
  git init --bare /data/nydus/repos/$repo.git
  chown -R git:git /data/nydus/repos/$repo.git
done

# 2. 从 monorepo 提取子项目历史到独立 repo
# （使用 git-filter-repo 保留子目录历史）
for dir in queen synapse overlord forge hive pheromone cerebrate larva spore carapace nydus; do
  git clone starclaw.git $dir-split
  cd $dir-split
  git filter-repo --subdirectory-filter $dir/
  git remote add nydus git@nydus.starclaw.net:$dir.git
  git push nydus master
  cd ..
done

# 3. 本地 clone 每个独立 repo
for repo in queen synapse overlord forge hive ganglion cerebrate larva spore carapace; do
  git clone git@nydus.starclaw.net:$repo.git E:\$repo
done

# 4. 每个 repo 创建独立 post-receive hook
# 5. 逐步将 starclaw.git 的 monorepo hook 废弃
```

## 六、实施路线图

### Phase 1：数据盘 + 基础设施（本周）
- [ ] 挂载数据盘到 /data
- [ ] 迁移 MySQL 数据到 /data/mysql
- [ ] 迁移 Nydus repos 到 /data/nydus
- [ ] 迁移 Redis 数据到 /data/redis

### Phase 2：Ganglion MVP（2 周）
- [ ] 搭建 NATS Server（Docker，/data/ganglion/nats）
- [ ] Ganglion SDK（Go 库，服务注册 + 消息发布/订阅）
- [ ] Queen → Synapse 支付链路改走 Ganglion
- [ ] 统一服务健康检查

### Phase 3：微服务分支（1 个月）
- [ ] Nydus 支持子项目独立分支
- [ ] Forge CI/CD pipeline 对接 Ganglion 事件
- [ ] 每个子项目独立 Windsurf workspace

### Phase 4：全面微服务化（2 个月）
- [ ] 所有服务间通信迁移到 Ganglion
- [ ] 统一认证中心（Ganglion Auth）
- [ ] 分布式日志（Ganglion 收集 → 统一存储）
- [ ] 服务网格可观测性（Prometheus + Grafana）

---

> **Pheromone（信息素）** — 虫群的化学通信网络，让每个个体都能感知群体信号，协同进化。
