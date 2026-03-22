# Claw 节点分级体系 — Spark / Pulse / Surge / Storm

> 免费极致体验 → 自然升级 → 企业级

## 设计理念

- **免费永久可用**，不是试用期。限制来自技术架构的自然瓶颈，不是人为设卡
- **Spore 极简技术**：单 binary + SQLite，3 秒部署，30MB 镜像
- **平滑升级**：数据保留、节点 ID 不变、团队智能体不中断

---

## 四级规格

### 🔥 Spark（火花）— 永久免费

| 项目 | 规格 |
|------|------|
| 部署方式 | **claw-lite 容器**（Spore + SQLite，单容器） |
| 镜像大小 | ~30MB |
| 启动时间 | **3 秒** |
| CPU | 0.25 核 |
| 内存 | 256MB |
| 存储 | 1GB |
| 数据库 | SQLite（嵌入式，零配置） |
| 缓存 | 内存缓存（无 Redis） |
| Web UI | ❌（仅 API + 员工 App 入口） |
| 团队智能体上限 | 2 个 |
| 并发 | SQLite 单写（1-3 人适用） |
| 有效期 | **永久** |
| 适合 | 个人体验、小团队试用、Demo 演示 |

**技术栈**：
```
Docker 镜像 (~30MB):
├── spore-linux-amd64  (进程管理 + 健康检查 + 自动重启)
├── claw-api-linux-amd64 (Claw 核心，SQLite 模式)
└── manifest.json (Spore 包描述)

运行时:
└── data/claw.db (SQLite，自动创建)
```

**环境变量**：
```env
STARCLAW_DATABASE_DRIVER=sqlite
STARCLAW_DATABASE_SQLITE_PATH=/app/data/claw.db
STARCLAW_REDIS_ENABLED=false
STARCLAW_SERVER_MODE=release
STARCLAW_OVERLORD_ENABLED=true
STARCLAW_OVERLORD_OVERLORD_URL=https://overlord.starclaw.net
STARCLAW_NODE_ADDRESS=https://{slug}.starclaw.me
```

---

### ⚡ Pulse（脉冲）— ¥50/月

| 项目 | 规格 |
|------|------|
| 部署方式 | Hive 容器组（Claw + 共享 MySQL + 共享 Redis） |
| 启动时间 | ~30 秒 |
| CPU | 1 核 |
| 内存 | 1GB |
| 存储 | 10GB |
| 数据库 | MySQL（共享实例，独立数据库） |
| 缓存 | Redis（共享实例，key 前缀隔离） |
| Web UI | ✅ |
| 团队智能体上限 | 10 个 |
| 并发 | MySQL 并发写入（5-20 人适用） |
| 自动备份 | 每日 |
| 有效期 | 按月续费 |
| 适合 | 正式团队、日常办公 |

---

### 🚀 Surge（激流）— ¥200/月

| 项目 | 规格 |
|------|------|
| 部署方式 | 独立 ECS（2C4G） |
| 启动时间 | ~2 分钟 |
| CPU | 2 核 |
| 内存 | 4GB |
| 存储 | 40GB |
| 带宽 | 5Mbps |
| 数据库 | MySQL（独立实例） |
| 缓存 | Redis（独立实例） |
| Web UI | ✅ |
| 团队智能体 | 无限 |
| 自定义子域名 | ✅ |
| 数据导出 | ✅ |
| 高级监控 | ✅ |
| 有效期 | 按月续费 |
| 适合 | 中型企业、多部门协作 |

---

### 👑 Storm（风暴）— ¥800/月

| 项目 | 规格 |
|------|------|
| 部署方式 | 独立 ECS 高配（4C8G） |
| CPU | 4 核 |
| 内存 | 8GB |
| 存储 | 100GB |
| 带宽 | 10Mbps |
| 团队智能体 | 无限 |
| 自定义域名 | ✅（绑定企业域名） |
| SSO | ✅（LDAP / OAuth2） |
| SLA | 99.9% |
| 合规审计 | ✅ |
| 私有部署选项 | ✅ |
| 专属技术支持 | ✅ |
| 适合 | 大型企业、100+ 人 |

---

## 升级路径

```
🔥 Spark ──→ ⚡ Pulse ──→ 🚀 Surge ──→ 👑 Storm
  (免费)      (¥50/月)     (¥200/月)    (¥800/月)

升级时发生什么：
1. 创建新规格的实例
2. 自动迁移数据（SQLite → MySQL 或 MySQL → 独立 MySQL）
3. 更新 Overlord 节点地址指向新实例
4. 销毁旧实例
5. 节点 ID 不变 → 团队智能体、任务历史全部保留
```

### Spark → Pulse 升级（SQLite → MySQL）

```
1. 创建新 Pulse 容器组 + MySQL 数据库
2. 导出 SQLite 数据: sqlite3 claw.db .dump > dump.sql
3. 转换 SQL 语法（SQLite → MySQL 兼容）
4. 导入 MySQL: mysql claw_{slug} < dump.sql
5. 切换 DNS/Nginx 指向新实例
6. 销毁旧 Spark 容器
```

### Pulse → Surge 升级（Hive → ECS）

```
1. 创建 ECS 实例 + 初始化 Docker 环境
2. mysqldump 导出 Hive 共享 MySQL 中的 claw_{slug} 数据库
3. 导入到 ECS 独立 MySQL
4. 更新 DNS A 记录指向 ECS 公网 IP
5. 销毁 Hive 容器
```

---

## 为什么 Spark 能永久免费

| 成本项 | 单节点开销 |
|--------|-----------|
| 容器运行 | 0.25 核 + 256MB ≈ ¥3/月 |
| SQLite 存储 | 几 MB，可忽略 |
| 网络 | 心跳 + 少量 API ≈ ¥0.5/月 |
| **总计** | **~¥3.5/月** |

单台 8C32G 服务器可承载 ~100 个 Spark 节点，月成本 ¥800，单节点 ¥8。
Spark 用户中有 5% 升级到 Pulse (¥50/月)，即 5 个付费用户 = ¥250/月，覆盖全部 Spark 成本。

---

## 自然升级触发点

| 场景 | 用户感知 | 推荐升级 |
|------|---------|---------|
| 创建第 3 个团队智能体 | "已达上限" | Spark → Pulse |
| 多人同时操作卡顿 | SQLite 写锁 | Spark → Pulse |
| 需要 Web UI 管理 | 只有 API/App | Spark → Pulse |
| 超过 10 个团队 | "已达上限" | Pulse → Surge |
| 需要独占资源 | 共享容器偶尔慢 | Pulse → Surge |
| 需要 SSO/合规 | 功能不可用 | Surge → Storm |

---

## 技术实现：claw-lite 镜像

### Dockerfile

```dockerfile
FROM alpine:3.19 AS base
RUN apk add --no-cache ca-certificates tzdata

FROM scratch
COPY --from=base /etc/ssl/certs /etc/ssl/certs
COPY --from=base /usr/share/zoneinfo /usr/share/zoneinfo
COPY spore-linux-amd64 /usr/bin/spore
COPY claw-api-linux-amd64 /opt/claw/claw-api
COPY manifest.json /opt/claw/manifest.json
EXPOSE 8080
ENTRYPOINT ["/usr/bin/spore", "run-inline", "/opt/claw"]
```

### Spore manifest.json

```json
{
  "name": "claw-lite",
  "version": "2026.0322",
  "description": "Claw Lite — SQLite mode for Spark tier",
  "platform": { "os": "linux", "arch": "amd64" },
  "binary": "claw-api",
  "args": [],
  "resources": {
    "min_memory_mb": 128,
    "min_disk_mb": 50,
    "recommended_memory_mb": 256
  },
  "network": {
    "ports": [{ "port": 8080, "protocol": "tcp", "description": "Claw API" }]
  },
  "health": {
    "endpoint": "http://localhost:8080/health",
    "interval_seconds": 15,
    "timeout_seconds": 5
  },
  "env": {
    "STARCLAW_DATABASE_DRIVER": "sqlite",
    "STARCLAW_DATABASE_SQLITE_PATH": "/app/data/claw.db",
    "STARCLAW_REDIS_ENABLED": "false",
    "STARCLAW_SERVER_MODE": "release",
    "GIN_MODE": "release"
  }
}
```

---

## 套餐 ID 映射

| 旧 ID | 新 ID | 新名称 | 价格 |
|--------|--------|--------|------|
| free | spark | 🔥 Spark 火花 | 永久免费 |
| basic | pulse | ⚡ Pulse 脉冲 | ¥50/月 |
| pro | surge | 🚀 Surge 激流 | ¥200/月 |
| enterprise | storm | 👑 Storm 风暴 | ¥800/月 |
