# Spore（孢子）— StarClaw 超轻量部署系统

> 让 Claw 像孢子一样，在任何设备上落地即萌发

## 1. 定位

Spore 是 Docker 的轻量替代方案，专为 Claw 在异构设备上的部署而设计。
**用户可自由选择 Docker 或 Spore 部署方式**，两者并存。

| 场景 | 推荐方案 |
|------|---------|
| 云服务器 / 有 Docker 环境 | Docker（现有方案保留） |
| 树莓派 / ARM 开发板 | Spore |
| NAS（群晖/威联通） | Spore 或 Docker |
| 路由器（OpenWrt） | Spore |
| Android 设备 | Spore |
| 离线/内网环境 | Spore |
| Windows 桌面 | Spore（无需 WSL/Hyper-V） |
| macOS 桌面 | Spore（无需 Docker Desktop） |

---

## 2. 虫族命名体系

```
StarClaw 虫族部署体系
│
├── Spore (孢子)         — 超轻量容器运行时 + 包格式
│   ├── .spore 包        — 自含式部署单元
│   └── spore CLI        — 运行时管理工具
│
├── Hatchery (孵化场)    — 构建工具 + 本地镜像仓库
│   ├── hatchery build   — 构建 .spore 包
│   ├── hatchery serve   — 本地仓库服务
│   └── hatchery publish — 发布到 Nydus 网络
│
├── Creep (菌毯)         — 设备集群管理层
│   ├── Creep Agent      — 设备端轻量 Agent（< 1MB）
│   └── Creep Map        — Queen 上的设备拓扑视图
│
├── Nydus (虫洞)         — P2P 分发网络（已有）
│
├── Queen (女王)         — 中心控制台（已有，扩展 Creep 管理）
│
└── Docker (兼容)        — 保留现有 Docker 部署方式
    ├── Dockerfile       — 现有
    └── docker-compose   — 现有
```

---

## 3. Spore 包格式（.spore）

### 3.1 文件结构

```
claw-v1.2.0-linux-arm64.spore    (tar + zstd 压缩)
│
├── manifest.json                 # 元数据
├── bin/
│   └── claw                      # 静态链接二进制
├── config/
│   ├── default.yaml              # 默认配置
│   └── .env.example              # 环境变量模板
├── data/                          # 可选初始数据
│   ├── migrations/
│   └── prompts/
├── web/                           # 前端静态文件（可选）
│   └── dist/
└── hooks/                         # 生命周期钩子（可选）
    ├── pre-install.sh
    ├── post-install.sh
    ├── pre-start.sh
    └── post-stop.sh
```

### 3.2 manifest.json

```json
{
  "name": "claw",
  "version": "1.2.0",
  "description": "StarClaw AI Agent Node",
  "platform": {
    "os": "linux",
    "arch": "arm64",
    "min_kernel": "4.15"
  },
  "binary": "bin/claw",
  "args": ["serve", "--config", "config/default.yaml"],
  "resources": {
    "min_memory_mb": 256,
    "min_disk_mb": 100,
    "recommended_memory_mb": 1024
  },
  "network": {
    "ports": [
      {"port": 8080, "protocol": "tcp", "description": "API Server"},
      {"port": 8443, "protocol": "tcp", "description": "API Server (TLS)"}
    ]
  },
  "health": {
    "endpoint": "http://localhost:8080/health",
    "interval_seconds": 30,
    "timeout_seconds": 5
  },
  "update": {
    "channel": "stable",
    "auto_update": false,
    "delta_enabled": true
  },
  "dependencies": [],
  "checksum": "sha256:abc123...",
  "built_at": "2026-03-17T00:00:00Z",
  "built_by": "hatchery v0.1.0"
}
```

### 3.3 支持的平台矩阵

| OS | Arch | 状态 |
|----|------|------|
| linux | amd64 | ✅ 主力 |
| linux | arm64 | ✅ 树莓派4/5, Jetson |
| linux | arm/v7 | ✅ 树莓派3, 旧ARM |
| linux | mips64le | 🔄 路由器 |
| linux | riscv64 | 🔄 新兴平台 |
| darwin | amd64 | ✅ Intel Mac |
| darwin | arm64 | ✅ Apple Silicon |
| windows | amd64 | ✅ Windows 桌面 |
| android | arm64 | 🔄 Termux 环境 |

---

## 4. Spore Runtime

### 4.1 CLI 命令

```bash
# 安装
spore install ./claw-v1.2.0-linux-arm64.spore
spore install https://hatchery.starclaw.me/claw/latest
spore pull claw:latest                    # 从 Nydus P2P 网络拉取

# 运行管理
spore run claw                            # 前台运行
spore start claw                          # 后台运行（注册为系统服务）
spore stop claw                           # 停止
spore restart claw                        # 重启
spore status                              # 查看所有已安装 Spore 状态

# 更新
spore update claw                         # 拉取最新版 + delta patch
spore update claw --version 1.3.0         # 指定版本
spore rollback claw                       # 回滚到上一版本

# 信息
spore list                                # 列出已安装的 Spore
spore info claw                           # 查看详细信息
spore logs claw                           # 查看日志
spore logs claw --follow                  # 实时日志

# 分发
spore push ./claw.spore                   # 发布到 Nydus P2P 网络
spore export claw --output ./claw.spore   # 导出为 .spore 文件（离线传输）
```

### 4.2 进程管理（自适应）

```
检测平台 →
  ├─ Linux + systemd → 生成 systemd unit → systemctl 管理
  ├─ Linux + openrc  → 生成 openrc script → rc-service 管理
  ├─ Linux + procd   → 生成 procd config (OpenWrt)
  ├─ macOS           → 生成 launchd plist → launchctl 管理
  ├─ Windows         → 注册 Windows Service → sc 管理
  └─ 其他/Termux     → nohup + PID file + 自管理
```

### 4.3 数据目录布局

```
~/.spore/                          # Spore 根目录（可通过 SPORE_HOME 自定义）
├── bin/                           # Spore CLI 自身
├── cache/                         # 下载缓存 + delta patch 缓存
├── installed/
│   └── claw/
│       ├── current -> v1.2.0/     # 当前版本符号链接
│       ├── v1.2.0/                # 版本目录（包内容解压）
│       │   ├── bin/claw
│       │   ├── config/
│       │   └── manifest.json
│       ├── v1.1.0/                # 上一版本（用于回滚）
│       ├── data/                  # 持久数据（跨版本保留）
│       │   ├── db/
│       │   └── uploads/
│       └── logs/                  # 日志
├── registry/                      # 本地 Spore 索引
│   └── index.json
└── config.yaml                    # Spore 全局配置
```

---

## 5. Hatchery（孵化场）— 构建系统

### 5.1 Sporefile（构建描述文件）

```yaml
# Sporefile.yaml — 放在项目根目录
name: claw
version: "1.2.0"
description: "StarClaw AI Agent Node"

# 构建步骤
build:
  # Go 静态编译
  command: |
    CGO_ENABLED=0 GOOS=${TARGET_OS} GOARCH=${TARGET_ARCH} \
    go build -ldflags="-s -w -X main.version=${VERSION}" \
    -o bin/claw ./cmd/server
  
  # 前端构建（可选）
  pre_build:
    - cd web && npm ci && npm run build

# 打包内容
package:
  binary: bin/claw
  args: ["serve"]
  include:
    - config/default.yaml
    - web/dist/
  exclude:
    - "*.test"
    - "**/*_test.go"

# 运行时配置
runtime:
  ports: [8080, 8443]
  health: "http://localhost:8080/health"
  min_memory_mb: 256
  env:
    - CLAW_DATA_DIR=/data
    - CLAW_LOG_LEVEL=info

# 多平台构建目标
platforms:
  - linux/amd64
  - linux/arm64
  - linux/arm/v7
  - darwin/amd64
  - darwin/arm64
  - windows/amd64
```

### 5.2 构建命令

```bash
# 构建当前平台
hatchery build

# 构建所有目标平台
hatchery build --all

# 构建指定平台
hatchery build --platform linux/arm64,darwin/arm64

# 输出到指定目录
hatchery build --output dist/

# 发布到本地仓库
hatchery publish --local

# 发布到 Nydus P2P 网络
hatchery publish --nydus
```

### 5.3 本地仓库（Hatchery Serve）

```bash
# 启动本地仓库（用于内网/离线环境）
hatchery serve --port 7777 --dir ./repo

# 其他设备可以从这里拉取
spore pull claw:latest --registry http://192.168.1.100:7777
```

---

## 6. Delta 更新（差量更新）

### 6.1 原理

```
v1.2.0 (30MB) → v1.3.0 (31MB)

传统方式: 下载完整 31MB
Delta 方式: 下载 patch 文件 ~2MB

算法: bsdiff / zstd-seekable
  1. Hatchery 构建时同时生成与前 N 个版本的 delta patch
  2. spore update 时优先尝试 delta，失败回退到全量
```

### 6.2 更新流程

```
spore update claw
  │
  ├─ 1. 查询最新版本（Nydus P2P / Hatchery Registry / 直连）
  ├─ 2. 检查是否有 delta patch（当前版本 → 最新版本）
  │     ├─ 有 → 下载 delta (~2MB) → bspatch 应用 → 校验 checksum
  │     └─ 无 → 下载全量 .spore 包
  ├─ 3. 解压到新版本目录 installed/claw/v1.3.0/
  ├─ 4. 运行 pre-start hook（数据库迁移等）
  ├─ 5. 切换 current 符号链接 → v1.3.0
  ├─ 6. 重启服务
  ├─ 7. 健康检查
  │     ├─ 通过 → 更新成功，保留 v1.2.0 用于回滚
  │     └─ 失败 → 自动回滚到 v1.2.0
  └─ 8. 清理旧版本（保留最近 2 个）
```

---

## 7. Nydus P2P 分发集成

利用已有的 Nydus 虫洞网络进行去中心化分发：

```
节点 A (有 claw v1.3.0)
    │
    ├── Nydus P2P ──→ 节点 B (需要 claw)
    │                    ↓
    │              spore pull claw:latest
    │                    ↓
    │              从 A 的 P2P 网络直接下载（无需中心服务器）
    │
    ├── Nydus P2P ──→ 节点 C
    │
    └── 同时也可以从 Hatchery 中心仓库拉取（作为 fallback）
```

**分发优先级**：
1. 本地缓存（已有最新版）
2. Nydus P2P 邻居节点
3. Hatchery 中心仓库（hatchery.starclaw.me）
4. 直接 HTTP 下载

---

## 8. Creep（菌毯）— 设备集群管理

### 8.1 Creep Agent

超轻量 Agent（< 1MB），运行在每台部署了 Spore 的设备上：

```go
type CreepAgent struct {
    DeviceID    string   // 设备唯一标识
    Platform    Platform // os/arch/kernel
    Resources   Resource // CPU/内存/磁盘/网络
    Spores      []Spore  // 已安装的 Spore 列表
    Status      string   // online/offline/updating
    LastReport  time.Time
}
```

功能：
- 每 60s 向 Queen 上报设备状态
- 接收 Queen 下发的部署/更新/回滚指令
- 通过 Nydus 自动发现并加入 P2P 分发网络
- 设备离线时自主运行，上线后自动同步

### 8.2 Queen Creep Map（女王菌毯地图）

在 Queen 控制台新增设备拓扑视图：

```
┌─────────────────────────────────────────────┐
│  🗺️ Creep Map — 设备拓扑                    │
│                                              │
│  总设备: 12  在线: 10  更新中: 1  离线: 1   │
│                                              │
│  ┌─────────┐  ┌─────────┐  ┌─────────┐     │
│  │ 🖥️ SRV-1 │  │ 🍓 PI-4  │  │ 📱 AND-1│     │
│  │ x86_64   │  │ arm64   │  │ arm64   │     │
│  │ claw 1.3 │  │ claw 1.3│  │ claw 1.2│     │
│  │ ● 在线   │  │ ● 在线  │  │ ↻ 更新中│     │
│  └─────────┘  └─────────┘  └─────────┘     │
│                                              │
│  [一键全部更新]  [选择设备部署]              │
└─────────────────────────────────────────────┘
```

---

## 9. Docker 兼容并存

### 9.1 保留现有 Docker 方案

```
claw/
├── Dockerfile           # 保留
├── docker-compose.yml   # 保留
├── .dockerignore        # 保留
└── Sporefile.yaml       # 新增 Spore 构建描述
```

### 9.2 统一安装入口

用户安装时提供选择：

```bash
# 方式 1: Docker（现有）
docker pull starclaw/claw:latest
docker-compose up -d

# 方式 2: Spore（新增）
curl -fsSL https://spore.starclaw.me/install.sh | sh
spore pull claw:latest
spore start claw

# 方式 3: 离线 Spore
# 从 U盘/局域网 获取 .spore 文件
spore install ./claw-v1.3.0-linux-arm64.spore
spore start claw
```

### 9.3 CI/CD 同时构建

```yaml
# GitHub Actions / Nydus Pipeline
build:
  steps:
    - name: Build Docker Image
      run: docker build -t starclaw/claw:$VERSION .
    
    - name: Build Spore Packages
      run: hatchery build --all --output dist/
    
    - name: Publish Docker
      run: docker push starclaw/claw:$VERSION
    
    - name: Publish Spore
      run: hatchery publish --nydus dist/*.spore
```

---

## 10. 安全

- Spore 包使用 Ed25519 签名（复用 Claw 节点身份体系）
- manifest.json 包含 SHA256 checksum，安装时校验
- Delta patch 同样签名 + checksum 校验
- Nydus P2P 传输加密
- Creep Agent ↔ Queen 通信使用 TLS + 节点认证

---

## 11. 开发阶段

| 阶段 | 内容 | 产出 |
|------|------|------|
| **S1** | Spore 包格式 + manifest + 构建工具 | `spore/` 项目骨架, Sporefile 解析, `hatchery build` |
| **S2** | Spore Runtime | `spore install/run/stop/status/list/logs`, 多平台服务注册 |
| **S3** | P2P 分发 + Delta 更新 | `spore pull/push/update/rollback`, Nydus 集成, bsdiff |
| **S4** | Creep Agent + Queen UI | Creep Agent, Queen Creep Map 页面 |
| **S5** | 多平台构建 + 测试 | 交叉编译, ARM/MIPS 测试, 安装脚本 |

---

## 12. 零依赖架构（Standalone Mode）

### 12.1 问题

Docker 方案下 Claw 依赖三个外部服务：

| 依赖 | Docker 方案 | 问题 |
|------|------------|------|
| MySQL | 容器 | 200MB+，需要外网拉镜像 |
| Redis | 容器 | 额外进程 |
| Web 前端 | nginx 容器 | 额外容器 |

Spore 的目标是**单文件零依赖**，因此需要替换这三个外部依赖。

### 12.2 替代方案

| 依赖 | Spore 替代 | 优势 |
|------|-----------|------|
| **MySQL** | **SQLite**（嵌入式） | 零进程、单文件 `.db`、GORM 无缝切换、树莓派可跑 |
| **Redis** | **内存缓存**（Go sync.Map + TTL） | 零依赖、单节点足够、重启后自动恢复 |
| **Web 前端** | **embed.FS** 嵌入 Go 二进制 | 单文件部署、无需 nginx |

### 12.3 数据库双模式

```yaml
# config.yaml
database:
  driver: sqlite    # sqlite（Spore 默认）或 mysql（Docker/生产环境）
  # SQLite 模式
  sqlite_path: "./data/claw.db"
  # MySQL 模式
  host: localhost
  port: 3306
  user: root
  password: starclaw
  dbname: starclaw
```

- **SQLite 模式**：零配置，数据库文件保存在 `~/.spore/installed/claw/data/claw.db`
- **MySQL 模式**：连接外部 MySQL（Docker 或独立安装），适合大规模生产
- GORM 业务代码**零改动**，只换 driver

### 12.4 缓存双模式

```yaml
redis:
  enabled: false    # false = 使用内存缓存（Spore 默认）
  host: localhost
  port: 6379
```

- **内存模式**：Go 内置 map + RWMutex + 过期清理协程，零依赖
- **Redis 模式**：连接外部 Redis，适合多节点共享缓存

### 12.5 Web 前端嵌入

```go
//go:embed web/dist/*
var webAssets embed.FS

// 在 router 中注册静态文件服务
r.StaticFS("/", http.FS(webAssets))
```

构建时 `npm run build` → `web/dist/` → `go:embed` → 单二进制包含前后端。

---

## 13. 多实例支持（同一设备跑多个 Claw）

### 13.1 场景

- 一台服务器跑 3 个 Claw：研发用、测试用、生产用
- 一台 NAS 跑 2 个 Claw：家庭助手 + 工作助手
- 开发时同时跑多个版本对比测试

### 13.2 实例命名

```bash
# 安装多个实例（不同名称）
spore install ./claw.spore --name claw-dev
spore install ./claw.spore --name claw-prod
spore install ./claw.spore --name claw-test

# 分别管理
spore start claw-dev
spore start claw-prod
spore status
```

### 13.3 实例隔离

每个实例完全独立：

```
~/.spore/installed/
├── claw-dev/
│   ├── current -> v0.1.0/
│   ├── v0.1.0/bin/claw.exe
│   ├── data/
│   │   └── claw.db          # 独立 SQLite 数据库
│   ├── logs/
│   └── claw-dev.pid
├── claw-prod/
│   ├── current -> v0.1.0/
│   ├── data/
│   │   └── claw.db          # 独立数据库
│   ├── logs/
│   └── claw-prod.pid
└── claw-test/
    └── ...
```

### 13.4 端口自动分配

```json
// manifest.json — 每个实例可自定义 env
{
  "env": {
    "STARCLAW_SERVER_PORT": "8080"   // claw-dev
  }
}
```

```bash
# 或通过 CLI 参数覆盖
spore start claw-dev  --env STARCLAW_SERVER_PORT=8080
spore start claw-prod --env STARCLAW_SERVER_PORT=8090
spore start claw-test --env STARCLAW_SERVER_PORT=8100
```

### 13.5 多实例状态一览

```
$ spore status

NAME           VERSION   STATUS      PORT    LOCATION
─────────────────────────────────────────────────────────────
claw-dev       0.1.0     🟢 running  8080    ~/.spore/installed/claw-dev/v0.1.0
claw-prod      0.1.0     🟢 running  8090    ~/.spore/installed/claw-prod/v0.1.0
claw-test      0.2.0-rc  ⏹ stopped  8100    ~/.spore/installed/claw-test/v0.2.0-rc
```

### 13.6 多实例资源总览

Creep Agent 会上报所有实例状态到 Queen：

```json
{
  "device_id": "my-server",
  "spores": [
    {"name": "claw-dev",  "version": "0.1.0", "status": "running", "port": 8080},
    {"name": "claw-prod", "version": "0.1.0", "status": "running", "port": 8090},
    {"name": "claw-test", "version": "0.2.0-rc", "status": "stopped"}
  ]
}
```

---

## 14. 零依赖开发阶段

| 阶段 | 内容 | 产出 |
|------|------|------|
| **D1** | SQLite 支持 | database 层双 driver、config 切换、AutoMigrate 兼容 |
| **D2** | 内存缓存 | Redis fallback、cache 接口抽象、TTL 清理 |
| **D3** | Web 嵌入 | embed.FS、前端 build → Go 二进制、SPA 路由 |
| **D4** | 多实例 | `--name` 参数、端口隔离、独立数据目录 |
| **D5** | 关 Docker 全流程测试 | 打包 → 安装 → 启动 → 健康检查 → 多实例 |

---

## 15. 零依赖验证结果（2026-03-17）

### 15.1 已验证环境

```
OS:       Windows 11 (amd64)
Go:       1.24
CGO:      DISABLED (纯 Go 编译)
SQLite:   glebarez/sqlite (pure Go, no gcc)
Redis:    DISABLED (in-memory fallback)
Web:      embed.FS (前端嵌入二进制)
```

### 15.2 日志确认

```
[database] Using SQLite driver (path: ./data/claw.db)   ✅
[info] Redis disabled, using in-memory cache             ✅
StarClaw server starting on :8090                        ✅
Health: ✅ healthy                                        ✅
```

### 15.3 Claw 改动清单

| 文件 | 改动 |
|------|------|
| `config/config.go` | +Database.Driver, +SQLitePath, +Redis.Enabled |
| `database/sqlite.go` | NEW: 纯 Go SQLite (glebarez/sqlite) |
| `database/init.go` | NEW: 统一 InitDB() 按 driver 分发 |
| `cmd/server/main.go` | InitDB() 替换 InitMySQL(), Redis 可选 |
| `internal/web/embed.go` | NEW: embed.FS + SPA fallback |
| `router/router.go` | +web.RegisterRoutes(r) |

### 15.4 环境变量格式

Claw 使用 Viper `SetEnvPrefix("STARCLAW")` + `SetEnvKeyReplacer(".", "_")`：

```
server.port          → STARCLAW_SERVER_PORT
database.driver      → STARCLAW_DATABASE_DRIVER
database.sqlite_path → STARCLAW_DATABASE_SQLITE_PATH
redis.enabled        → STARCLAW_REDIS_ENABLED
```

---

## 16. Spore Desktop（图形管理工具）

> 类似 Docker Desktop，提供可视化管理界面

### 16.1 定位

| 对比 | Docker Desktop | Spore Desktop |
|------|---------------|---------------|
| 体积 | ~1GB | < 15MB |
| 资源占用 | 高（Hyper-V/WSL2） | 极低（纯本地 HTTP） |
| 功能 | 通用容器管理 | 专为 Claw 实例管理 |
| 技术 | Electron | Go 后端 + 嵌入式 Web UI |

### 16.2 架构

```
Spore Desktop
├── Go 后端 (localhost:7890)
│   ├── REST API（调用 spore runtime 包）
│   ├── WebSocket（实时日志推送）
│   └── 系统托盘（最小化到托盘）
│
├── React 前端（embed.FS 嵌入）
│   ├── 仪表盘（总览所有实例）
│   ├── 实例管理（安装/启动/停止/删除）
│   ├── 日志查看器（实时 + 历史）
│   ├── 配置编辑（端口/环境变量）
│   └── 仓库浏览（可用 .spore 包）
│
└── 系统托盘
    ├── 快捷启停实例
    ├── 打开 Web UI
    └── 退出
```

### 16.3 页面设计

**仪表盘 Dashboard**
```
┌─────────────────────────────────────────────────────┐
│  🏠 Spore Desktop                          ─ □ ✕   │
├─────────────────────────────────────────────────────┤
│  ┌──────────┐ ┌──────────┐ ┌──────────┐            │
│  │ 实例总数  │ │ 运行中    │ │ 总磁盘    │            │
│  │    3     │ │  🟢 2    │ │  45 MB   │            │
│  └──────────┘ └──────────┘ └──────────┘            │
│                                                     │
│  实例列表                          [+ 安装新实例]    │
│  ┌─────────────────────────────────────────────┐   │
│  │ 🟢 claw-dev    v0.2.0  :8080  ✅ healthy   │   │
│  │    [停止] [重启] [日志] [配置] [删除]        │   │
│  ├─────────────────────────────────────────────┤   │
│  │ 🟢 claw-prod   v0.2.0  :8090  ✅ healthy   │   │
│  │    [停止] [重启] [日志] [配置] [删除]        │   │
│  ├─────────────────────────────────────────────┤   │
│  │ ⏹ claw-test   v0.1.0  :8100  ─ stopped    │   │
│  │    [启动] [日志] [配置] [删除]              │   │
│  └─────────────────────────────────────────────┘   │
│                                                     │
│  平台: windows/amd64 │ Spore v0.1.0 │ 3 实例       │
└─────────────────────────────────────────────────────┘
```

### 16.4 技术栈

- **后端**: Go + `pkg/runtime` (复用 Spore 核心包)
- **前端**: React + TailwindCSS + Lucide Icons
- **打包**: `go:embed` 嵌入前端，单二进制分发
- **托盘**: `getlantern/systray` 跨平台系统托盘

---

## 17. 项目结构

```
starclaw/spore/
├── cmd/
│   ├── spore/           # Spore CLI (运行时)
│   ├── hatchery/        # Hatchery CLI (构建工具)
│   └── desktop/         # Spore Desktop (图形管理)
│       └── main.go
├── pkg/
│   ├── manifest/        # manifest.json 解析
│   ├── archive/         # .spore 包打包/解包
│   ├── runtime/         # 进程管理 + 服务注册
│   ├── registry/        # 本地仓库 + 远程索引
│   ├── platform/        # 平台检测 + 自适应
│   ├── creep/           # Creep Agent
│   └── service/         # 系统服务注册
├── desktop/
│   ├── api/             # Desktop REST API
│   ├── web/             # Desktop React 前端
│   └── tray/            # 系统托盘
├── Sporefile.yaml
├── install.sh
├── install.ps1
├── docs/
│   └── SPORE_PLAN.md
├── go.mod
└── README.md
```

---

## 18. Spore Desktop 验证结果（2026-03-17）

### 18.1 产品数据

```
二进制大小:  ~15MB (单文件, go:embed 前端)
启动端口:   127.0.0.1:7890
技术栈:     Go 后端 + TailwindCSS CDN + 原生 JS
前端体积:   ~15KB (单 HTML 文件)
依赖:       无 (无 npm/node/Electron)
```

### 18.2 已验证 API

| 端点 | 方法 | 功能 | 状态 |
|------|------|------|------|
| `/api/instances` | GET | 实例列表 + 状态/健康/端口/环境变量 | ✅ |
| `/api/instances/{name}` | GET | 单实例详情 | ✅ |
| `/api/instances/{name}/start` | POST | 启动实例 | ✅ |
| `/api/instances/{name}/stop` | POST | 停止实例 | ✅ |
| `/api/instances/{name}/restart` | POST | 重启实例 | ✅ |
| `/api/instances/{name}/env` | PUT | 编辑环境变量 | ✅ |
| `/api/instances/{name}` | DELETE | 卸载实例 | ✅ |
| `/api/install` | POST | 安装新实例 (支持 --name) | ✅ |
| `/api/platform` | GET | 平台信息 | ✅ |
| `/api/logs/{name}` | GET | SSE 实时日志流 | ✅ |

### 18.3 新增文件

| 文件 | 说明 |
|------|------|
| `desktop/api/server.go` | Go 后端: REST API + SSE 日志 |
| `desktop/web/embed.go` | 前端嵌入 |
| `desktop/web/dist/index.html` | 单文件前端 UI |
| `cmd/desktop/main.go` | 入口: 启动服务 + 自动打开浏览器 |

### 18.4 全局命令

```bash
spore      # CLI 管理
hatchery   # 构建打包
desktop    # 图形管理界面 (http://127.0.0.1:7890)
```

---

## 19. 研发团队 Claw 集群部署

> 使用 Spore 多实例能力，在同一设备上部署研发团队的多个 Claw

### 19.1 完整团队（7 角色）

| 实例名 | 角色 | 端口 | 状态 | 健康 |
|--------|------|------|------|------|
| `claw-backend` | 后端开发 | 8081 | 🟢 running | ✅ healthy |
| `claw-frontend` | 前端开发 | 8082 | 🟢 running | ✅ healthy |
| `claw-qa` | QA 测试 | 8083 | 🟢 running | ✅ healthy |
| `claw-pm` | 产品经理 | 8084 | 🟢 running | ✅ healthy |
| `claw-design` | UI 设计 | 8085 | 🟢 running | ✅ healthy |
| `claw-devops` | DevOps | 8086 | 🟢 running | ✅ healthy |
| `claw-lead` | 架构师/TechLead | 8087 | 🟢 running | ✅ healthy |

> 已于 2026-03-17 在单台 Windows 机器上验证通过，7 实例并行运行

### 19.2 部署命令

```bash
# 用同一个 .spore 包安装 7 个独立实例
spore install ./claw.spore --name claw-backend
spore install ./claw.spore --name claw-frontend
spore install ./claw.spore --name claw-qa
spore install ./claw.spore --name claw-pm
spore install ./claw.spore --name claw-design
spore install ./claw.spore --name claw-devops
spore install ./claw.spore --name claw-lead

# 启动（端口已写入各自 manifest.json）
spore start claw-backend
spore start claw-frontend
spore start claw-qa
spore start claw-pm
spore start claw-design
spore start claw-devops
spore start claw-lead

# 统一查看
spore status
# 或打开 Desktop 图形管理: desktop
```

### 19.3 数据隔离

```
~/.spore/installed/
├── claw-backend/    data/claw.db  logs/  :8081
├── claw-frontend/   data/claw.db  logs/  :8082
├── claw-qa/         data/claw.db  logs/  :8083
├── claw-pm/         data/claw.db  logs/  :8084
├── claw-design/     data/claw.db  logs/  :8085
├── claw-devops/     data/claw.db  logs/  :8086
└── claw-lead/       data/claw.db  logs/  :8087
```

每个实例有独立的 SQLite 数据库、日志、配置，互不干扰。

### 19.4 各角色访问入口

| 角色 | URL | 用途 |
|------|-----|------|
| 后端开发 | http://localhost:8081 | 编码、API 设计、数据库建模 |
| 前端开发 | http://localhost:8082 | UI 开发、组件编写、联调 |
| QA 测试 | http://localhost:8083 | 测试用例、自动化测试、Bug 追踪 |
| 产品经理 | http://localhost:8084 | PRD 撰写、需求分析、用户故事 |
| UI 设计 | http://localhost:8085 | 设计评审、原型、设计规范 |
| DevOps | http://localhost:8086 | CI/CD、部署脚本、监控配置 |
| 架构师 | http://localhost:8087 | Code Review、架构决策、技术选型 |
| **Desktop 管理** | http://localhost:7890 | 统一管理全部 7 个实例 |

### 19.5 团队组网验证（2026-03-17）

#### 初始化结果

```
[Step 1] 7 实例 owner 账号创建成功 ✅
[Step 2] 7 个 Node ID 采集完成 ✅ (claw:xxxx 格式)
[Step 3] 6 个 peer 注册到 claw-lead (虫洞链路) ✅
[Step 4] Squad "StarClaw Dev Team" 创建成功 ✅
[Step 5] 6 个成员邀请加入 Squad ✅
```

#### 指挥架构

```
                    ┌─────────────────────┐
                    │   claw-lead :8087   │
                    │   Tech Lead/架构师   │
                    │   (Captain/Overlord) │
                    └──────────┬──────────┘
                               │ Squad 指挥链
          ┌────────────────────┼────────────────────┐
          │                    │                    │
   ┌──────┴──────┐     ┌──────┴──────┐     ┌──────┴──────┐
   │ claw-pm     │     │ claw-design │     │ claw-devops │
   │ :8084 PM    │     │ :8085 设计   │     │ :8086 运维   │
   └─────────────┘     └─────────────┘     └─────────────┘
          │
   ┌──────┴──────────────────────────────┐
   │        开发 & 测试                   │
   ├──────────────┬──────────┬───────────┤
   │ claw-backend │ claw-fe  │ claw-qa   │
   │ :8081 后端    │ :8082 前端│ :8083 测试 │
   └──────────────┴──────────┴───────────┘
```

#### 命令下发方式

**不需要额外的 Overlord 服务。claw-lead 就是 Overlord。**

| 方式 | 入口 | 说明 |
|------|------|------|
| Web UI | http://localhost:8087 | 在 claw-lead 的 Squad 页面创建 Mission |
| API | POST /v1/squads/{id}/missions | 通过 REST API 下发任务 |
| Desktop | http://localhost:7890 | 查看全局状态，管理实例 |

#### Mission 工作流

```
1. 用户在 claw-lead 创建 Mission (例: "实现用户登录功能")
2. claw-lead 的 Squad Engine 通过 LLM 分解为 Steps:
   → Step 1: claw-pm 编写需求文档
   → Step 2: claw-design 设计登录页原型
   → Step 3: claw-backend 实现 API
   → Step 4: claw-frontend 实现页面
   → Step 5: claw-qa 编写测试用例
   → Step 6: claw-devops 配置部署
3. 各 Step 通过虫洞链路分发到对应实例执行
4. 执行结果通过回调汇报给 Captain
5. claw-lead 汇总结果，Mission 完成
```

#### 初始化脚本

```bash
# 一键初始化团队
powershell -File spore/scripts/init-team.ps1
```

---

## 20. Squad 代码协作系统（Nydus Git P2P）

> 让 Squad Mission 产出**真正可运行的代码**，而非仅仅 LLM 文本规划。
> 多节点通过 Nydus Git P2P 协议进行分支协作，最终合并产出完整项目。

### 20.1 问题分析

当前 Squad Mission 流程的产出仅为 LLM 文本（需求文档、架构描述等），原因：

| 缺失 | 说明 |
|------|------|
| **Agent 无 CodeTool** | Squad 创建的 Task 未启用 `code` 工具，Agent 无法写文件 |
| **无共享 workspace** | 各 Step 无法读写同一个项目目录 |
| **无代码同步** | 跨节点执行时没有代码传输机制 |
| **无构建预览** | Mission 完成后没有 build + serve 流程 |

### 20.2 两阶段方案概览

```
Phase 1: 本地 Git 协作                      Phase 2: Nydus Git P2P 跨网络
───────────────────                         ─────────────────────────────
同机多实例（file:// 协议）                    跨机器/跨网络（HTTP Git 协议）
CodeTool + GitTool 写码                      Git HTTP Smart Protocol 嵌入 Claw
本地 bare repo + 分支并行                     Nydus Relay NAT 穿透
git merge → build → preview                 与现有 Nydus 部署管道对接
```

### 20.3 Phase 1 — Squad + CodeTool + 本地 Git 协作

#### 20.3.1 架构

同机 7 个实例共享文件系统，Captain 创建本地 Git bare repo，
各 Member 通过 `file://` 协议 clone/push，实现分支并行开发：

```
User: "建一个新闻门户网站"
  │
  ▼
┌──────────────────────────────────────────────────────────────┐
│              claw-lead (Captain, :8087)                        │
│                                                               │
│  Squad Engine                                                 │
│    ├── 1. git init --bare ~/.spore/repos/mission-{id}.git     │
│    ├── 2. LLM 规划 → 6 Steps (每 Step 分配 branch)            │
│    ├── 3. Dispatch: repo_path + branch + enabled_tools        │
│    └── 4. 全部完成后: merge → build → preview                  │
│                                                               │
│  Git Bare Repo (本地)                                         │
│    ~/.spore/repos/mission-{id}.git                            │
│      ├── refs/heads/main                                      │
│      ├── refs/heads/feat/requirements   (Step 0)              │
│      ├── refs/heads/feat/ui-design      (Step 1)              │
│      ├── refs/heads/feat/frontend       (Step 2)              │
│      ├── refs/heads/feat/backend        (Step 3)              │
│      ├── refs/heads/feat/testing        (Step 4)              │
│      └── refs/heads/feat/deploy         (Step 5)              │
│                                                               │
│  合并后 Workspace                                              │
│    ~/.spore/workspaces/mission-{id}/                           │
│      ├── README.md         (PM 产出)                          │
│      ├── design/mockup.md  (设计产出)                          │
│      ├── src/                                                  │
│      │   ├── index.html    (前端产出)                          │
│      │   ├── style.css                                         │
│      │   └── app.js                                            │
│      ├── api/                                                  │
│      │   └── server.py     (后端产出)                          │
│      └── tests/            (QA 产出)                           │
│                                                               │
│  Preview: http://localhost:{auto-port}/                        │
└──────────────────────────────────────────────────────────────┘
          │ file:// clone/push
    ┌─────┼─────────────┬──────────────┐
    │     │             │              │
┌───▼────┐ ┌──────▼────┐ ┌─────▼─────┐ ┌──────▼──────┐
│claw-pm │ │claw-design│ │claw-fe    │ │claw-backend │ ...
│:8084   │ │:8085      │ │:8082      │ │:8081        │
│branch: │ │branch:    │ │branch:    │ │branch:      │
│feat/req│ │feat/ui    │ │feat/front │ │feat/back    │
│写 README│ │写 mockup  │ │写 html/css│ │写 server.py │
│commit  │ │commit     │ │commit     │ │commit       │
│push    │ │push       │ │push       │ │push         │
└────────┘ └───────────┘ └───────────┘ └─────────────┘
```

#### 20.3.2 协作流程（本地 file:// 模式）

```
Step  Captain (claw-lead)               Member (同机其他实例)
────  ───────────────────               ────────────────────────

1     git init --bare                   
      ~/.spore/repos/mission-{id}.git   
      git commit --allow-empty "init"   
      ↓                                 
2     LLM 规划 → Steps                  
      每个 Step 分配:                    
        branch: "feat/{role}"           
        tools: ["code", "git", "system"]
      ↓                                 
3     Dispatch:                          ← POST /peer/squad/execute
      {                                    {
        repo_path: "~/.spore/repos/         repo_path, branch,
          mission-{id}.git",                task, context
        branch: "feat/frontend",          }
        task: "写前端页面..."              
      }                                 
                                         ▼
4                                        git clone file://{repo_path} workspace/
                                         git checkout -b {branch}

5                                        Agent 用 CodeTool + GitTool:
                                           code.write_file("index.html", ...)
                                           code.write_file("style.css", ...)
                                           code.run_command("npm init -y")

6                                        git.add(".")
                                         git.commit("feat: implement homepage")
                                         git.push("origin", branch)

7                                        ← callback(branch, commit_hash)

8     收齐所有 callback 后:              
      git checkout main                 
      git merge feat/requirements       
      git merge feat/ui-design          
      git merge feat/frontend           
      git merge feat/backend            
      git merge feat/testing            
      (冲突时: 用 LLM 辅助解决)          

9     git checkout → workspace/         
      code.run_command("npm install")   
      code.run_command("npm run build") 

10    code.start_app("npx serve dist")  
      → http://localhost:{port}/        

11    Mission status: "completed"        
      preview_url: http://localhost:9001/
```

#### 20.3.3 新增组件

**GitTool（Agent 工具）**

```go
type GitTool struct {
    sandbox *sandbox.Manager
}

func (t *GitTool) Name() string { return "git" }
func (t *GitTool) Description() string {
    return `Git 版本控制工具：
    - clone: 克隆仓库到 workspace
    - status: 查看工作区状态
    - add: 暂存文件
    - commit: 提交更改
    - push: 推送到远程
    - pull: 拉取更新
    - branch: 创建/切换分支
    - merge: 合并分支
    - log: 查看提交历史
    - diff: 查看变更`
}

// Execute 内部调用系统 git 命令
func (t *GitTool) Execute(ctx context.Context, args string) (string, error) {
    // git clone file:///path/to/repo workspace/
    // git checkout -b feat/frontend
    // git add . && git commit -m "..."
    // git push origin feat/frontend
}
```

**Squad Engine 改造**

| 函数 | 改动 |
|------|------|
| `planAndDispatch` | 创建 bare repo，LLM 规划时分配 branch 名称 |
| `executeLocal` | Task 注入 repo_path + branch + 启用 code/git/system 工具 |
| `dispatchStep` | Dispatch 请求包含 repo_path + branch |
| `checkMissionComplete` | 合并所有分支 → build → start_app → 设置 preview_url |
| `HandleExecute` (peer) | 远程 Task 也注入 repo_path + branch |

**数据模型扩展**

```sql
ALTER TABLE missions ADD COLUMN repo_path TEXT;       -- 本地 Git bare repo 路径
ALTER TABLE missions ADD COLUMN preview_url TEXT;      -- 预览地址
ALTER TABLE missions ADD COLUMN workspace_path TEXT;   -- 合并后的工作目录

ALTER TABLE mission_steps ADD COLUMN branch TEXT;      -- Git 分支名
ALTER TABLE mission_steps ADD COLUMN commit_hash TEXT;  -- 最终提交 hash
```

#### 20.3.4 Task Goal 注入示例

```
## 任务
根据需求设计网站前端页面，包括首页、新闻列表、文章详情页。

## 工作要求
你必须使用 code 和 git 工具写出真实的代码文件并提交到 Git 仓库。

1. 先用 git.clone 克隆仓库:
   git clone file:///C:/Users/Yinhe/.spore/repos/mission-48a1b56c.git

2. 切换到你的工作分支:
   git branch feat/frontend

3. 用 code.write_file 创建代码文件:
   - src/index.html (首页)
   - src/news-list.html (新闻列表)
   - src/article.html (文章详情)
   - src/style.css (TailwindCSS 样式)

4. 用 code.list_files 查看前序步骤的产出（README.md 等）

5. 完成后提交并推送:
   git add .
   git commit -m "feat: implement frontend pages"
   git push origin feat/frontend

## 上下文（前序步骤已合并到 main）
README.md: 新闻门户需求文档...
design/mockup.md: UI 设计稿...
```

#### 20.3.5 预览系统

Mission 完成后自动检测项目类型并启动预览：

```
┌──────────────────────────────────────┐
│ Mission Preview                       │
│                                       │
│  自动检测项目类型:                      │
│    package.json → npm install && start│
│    requirements.txt → pip && python   │
│    go.mod → go run                    │
│    index.html (无其他) → 静态文件服务   │
│                                       │
│  启动方式:                              │
│    Sandbox ProcessManager.StartApp()  │
│    自动分配端口，反向代理到 /v1/app/    │
│                                       │
│  Preview URL:                         │
│    http://localhost:8087/v1/app/       │
│        mission-{id}/                  │
│                                       │
│  用户在 Claw Web UI 的 Mission 页面    │
│  点击 "Preview" 按钮即可查看            │
└──────────────────────────────────────┘
```

### 20.4 Phase 2 — Nydus Git P2P 跨网络协作

#### 20.4.1 核心理念

Phase 1 的 `file://` 协议仅限同机。跨物理网络需要 **Git HTTP Smart Protocol**。

现有 Nydus 是**中心化部署系统**（Server → Worm 单向推送）。
Squad 场景需要**去中心化双向协作**（Captain ⇄ Member 双向 Git 同步）。

新增 **Nydus Git P2P** 协议层，复用现有 Nydus 的 Git 能力，但改为 P2P 模式：

```
┌─────────────────────────────────────────────────────────────┐
│                  现有 Nydus                                   │
│  Server (bare repo) ──push──► Worm (deploy)                  │
│  单向：开发者 → 服务器                                        │
└─────────────────────────────────────────────────────────────┘
                        ↓ 演进
┌─────────────────────────────────────────────────────────────┐
│                  Nydus Git P2P                                │
│  Captain (bare repo) ◄──clone/push──► Member (workspace)     │
│  双向：Captain ⇄ Members                                     │
│  每个 Claw 节点既是 Git server 也是 Git client                │
└─────────────────────────────────────────────────────────────┘
```

#### 20.4.2 Git HTTP Server（嵌入 Claw）

Phase 2 为每个 Claw 节点内嵌一个轻量级 Git HTTP Smart Protocol 服务：

```go
// 路由注册
apiV1.GET("/git/:repo/info/refs", gitHandler.InfoRefs)
apiV1.POST("/git/:repo/git-upload-pack", gitHandler.UploadPack)
apiV1.POST("/git/:repo/git-receive-pack", gitHandler.ReceivePack)
```

实现方式选择：

| 方式 | 优点 | 缺点 |
|------|------|------|
| **Shell git http-backend** | 标准协议，零实现量 | 需要系统装 git |
| **go-git 纯 Go** | 零依赖 | 实现复杂 |
| **git CGI 代理** | 简单，调用系统 git | 需要 git 可执行文件 |

推荐方案：**Shell git http-backend**（Windows/Linux/macOS 都自带 git），通过 CGI 代理实现：

```go
type GitHandler struct {
    reposDir string // ~/.spore/git-repos/
}

func (h *GitHandler) InfoRefs(c *gin.Context) {
    repo := c.Param("repo")
    service := c.Query("service")
    cmd := exec.Command("git", "http-backend")
    cmd.Env = append(os.Environ(),
        "GIT_PROJECT_ROOT="+h.reposDir,
        "GIT_HTTP_EXPORT_ALL=1",
        "PATH_INFO=/"+repo+"/info/refs",
        "QUERY_STRING=service="+service,
        "REQUEST_METHOD=GET",
    )
    // pipe stdout → response
}
```

#### 20.4.3 协作流程变化（Phase 1 → Phase 2）

```
Phase 1 (同机):
  repo_path: "file:///C:/Users/Yinhe/.spore/repos/mission-xxx.git"
  dispatch 时传: repo_path (本地路径)
  ↓
Phase 2 (跨网络):
  repo_url: "http://captain:8087/v1/git/mission-xxx"
  dispatch 时传: repo_url (HTTP URL)
  跨 NAT 时: 通过 Nydus Relay 中继
```

GitTool 自动适配两种协议，代码改动极小：仅 `git clone {url}` 的 URL 不同。

#### 20.4.4 Nydus P2P vs 中心化对比

```
传统 Git 协作:
  Developer → GitHub/GitLab → CI/CD → Server
  (需要中心化 Git 服务，互联网连接)

Nydus Git P2P:
  Agent → Captain Node (bare repo) ← HTTP → Member Node (workspace)
  (零外部依赖，纯内网/本地运行)
  
  特殊场景支持：
  ├── 同机多实例：file:// 协议（Phase 1）
  ├── 局域网：HTTP 直连（Phase 2）
  └── 跨网络：Nydus Relay 中继（Phase 2+）
```

### 20.5 实现优先级

| 优先级 | 任务 | 预估 | 阶段 | 效果 |
|--------|------|------|------|------|
| P0 | GitTool 工具实现（clone/commit/push/merge） | 3h | Ph1 | Agent 能做 Git 操作 |
| P0 | Squad Engine 创建 bare repo + 分配分支 | 2h | Ph1 | 每个 Step 有独立分支 |
| P0 | Squad Task 启用 CodeTool + GitTool | 1h | Ph1 | Agent 能写真实代码 |
| P0 | 优化 LLM 规划 prompt（要求写代码 + Git 提交） | 1h | Ph1 | 产出质量提升 |
| P1 | Squad Engine merge + build + preview | 3h | Ph1 | 浏览器可访问结果 |
| P1 | 数据模型扩展（repo_path, branch, preview_url） | 1h | Ph1 | 状态可追踪 |
| P2 | Git HTTP Server 嵌入 Claw | 4h | Ph2 | 跨网络代码同步 |
| P2 | dispatch 改为传 repo_url (HTTP) | 1h | Ph2 | 跨机器协作 |
| P3 | 自动合并冲突处理（LLM 辅助） | 3h | Ph2 | 健壮性 |
| P3 | Nydus Relay 跨网络支持 | 4h | Ph2 | 跨物理网络协作 |

### 20.6 Bug 修复记录（2026-03-17）

Squad Mission 调试过程中修复了 5 个 bug：

| # | Bug | 文件 | 修复方案 |
|---|-----|------|---------|
| 1 | Gossip 心跳覆盖 peer 地址为空 | `peer.go:711` | 只在新值非空时更新字段 |
| 2 | planAndDispatch race condition | `engine.go:116` | 规划前立即设 total_steps=-1，改同步调用 |
| 3 | SQLite 不支持 FIELD() | `task_worker.go:155` | 改用 CASE WHEN 语法 |
| 4 | 远程节点无 callback 发送机制 | `squad_peer.go:284` | 新增 StartCallbackWatcher 后台轮询 |
| 5 | Captain callback URL 为空 | `engine.go:412` | Engine 加 selfAddress 字段 |

验证结果：Mission 6/6 Steps 完成，status=reviewing。

### 20.7 敏捷 Sprint 模型

> AI Agent 团队的敏捷开发：不是解决沟通问题，而是解决**质量和方向**问题。
> 核心价值：每个 Sprint 产出可运行增量，用户及时纠偏。

#### 20.7.1 当前问题：瀑布式 Mission

```
现状:
  Mission: "建新闻门户网站"
    → LLM 一次性规划 6 个 Steps
    → 全部执行完 → 结束
    → 没有迭代、没有反馈、没有增量交付

问题:
  1. LLM 一次规划可能不准确，但无法中途修正
  2. 后面的 Step 无法基于前面的实际产出调整方向
  3. 没有验收环节，质量无法保证
  4. 无法响应用户需求变更
```

#### 20.7.2 敏捷实践适用性分析（AI vs 人类）

| 敏捷实践 | 人类团队 | AI 团队 | 价值 |
|----------|---------|---------|------|
| **Sprint 迭代** | 2-4 周 | 10-30 分钟 | ⭐⭐⭐ 极高：增量交付，每轮可验证 |
| **CI/CD** | Jenkins/Actions | 自动 merge + test + preview | ⭐⭐⭐ 极高：每个 Step 完成后自动验证 |
| **Code Review** | 人工审查 | Agent 交叉审查 | ⭐⭐⭐ 极高：claw-qa 审查 claw-backend 代码 |
| **Backlog** | 用户故事列表 | Mission 优先级队列 | ⭐⭐ 高 |
| **Definition of Done** | 验收标准 | 自动化测试 + 构建通过 | ⭐⭐⭐ 极高：硬性质量门禁 |
| **Daily Standup** | 15分钟晨会 | Heartbeat 自动同步 | ⭐ 低：Agent 不会忘事 |
| **Planning Poker** | 估算工时 | LLM 直接估算 | ⭐ 低：不需要协商 |
| **Retrospective** | 回顾改进 | LLM 分析失败模式 | ⭐⭐ 高：自动优化 prompt |

核心差异：
```
人类敏捷: 解决沟通和协调问题
  → Standup 防遗忘，Review 防偷懒，Retro 防重蹈覆辙

AI 敏捷: 解决质量和方向问题
  → 迭代防一次规划偏差
  → CI Gate 防代码质量问题
  → Cross-review 防逻辑漏洞
  → Sprint 增量交付让用户及早发现方向偏差
```

#### 20.7.3 Sprint 生命周期

```
┌─────────────────────────────────────────────────────────────────┐
│                    Mission Sprint 生命周期                        │
│                                                                  │
│  Sprint 0 (骨架): ~10 min                                        │
│    PM      → 需求文档 (README.md)                                │
│    架构师   → 技术方案 + 目录结构                                  │
│    前端    → 项目脚手架 (create-react-app / vite)                 │
│    后端    → API 框架骨架 (FastAPI / Express)                     │
│    DevOps  → CI 配置 + Dockerfile                                │
│    → merge → build → ✅ 空壳能跑                                 │
│                                                                  │
│  Sprint 1 (MVP): ~15 min                                         │
│    前端    → 首页 + 导航 + 布局                                   │
│    后端    → 核心 API + 数据库模型                                 │
│    QA      → 基础单元测试                                         │
│    → merge → test → build → ✅ MVP 可演示                        │
│    → 👤 用户可查看，提出反馈                                      │
│                                                                  │
│  Sprint 2 (功能): ~15 min                                        │
│    前端    → 新闻列表 + 详情页 + 搜索                              │
│    后端    → 新闻 CRUD API + 分页                                 │
│    QA      → 集成测试 + API 测试                                  │
│    → merge → test → build → ✅ 功能完整                          │
│    → 👤 用户验收核心功能                                          │
│                                                                  │
│  Sprint 3 (打磨): ~10 min                                        │
│    设计    → UI/UX 优化                                          │
│    前端    → 响应式 + 动画 + SEO                                  │
│    QA      → E2E 测试 + 性能测试                                  │
│    DevOps  → 部署脚本 + 监控                                      │
│    → merge → test → build → ✅ 生产就绪                          │
│                                                                  │
│  每个 Sprint 结束自动执行:                                        │
│    1. git merge 所有分支到 main                                   │
│    2. 运行测试套件 (test gate)                                    │
│    3. npm build / go build (build gate)                          │
│    4. start_app → preview URL                                    │
│    5. Captain LLM 审查产出，规划下一个 Sprint                      │
│    6. 用户可查看 Preview，提出反馈纳入下一轮                        │
└─────────────────────────────────────────────────────────────────┘
```

#### 20.7.4 数据模型

```
Mission 1:N Sprint 1:N Step

Mission: "建新闻门户网站"
  ├── Sprint 0: "项目骨架"     → 5 Steps → done ✅
  │     preview: http://localhost:9001/  (空壳)
  ├── Sprint 1: "MVP 功能"    → 4 Steps → done ✅
  │     preview: http://localhost:9001/  (MVP)
  ├── Sprint 2: "完整功能"    → 4 Steps → executing 🔄
  │     preview: (building...)
  └── Sprint 3: "打磨上线"    → pending
```

```sql
-- 新增 Sprint 表
CREATE TABLE sprints (
    id          TEXT PRIMARY KEY,
    mission_id  TEXT NOT NULL REFERENCES missions(id),
    number      INTEGER NOT NULL,           -- Sprint 序号: 0, 1, 2, 3...
    goal        TEXT NOT NULL,              -- Sprint 目标
    status      TEXT DEFAULT 'pending',     -- pending | planning | executing | reviewing | done | failed
    total_steps INTEGER DEFAULT 0,
    done_steps  INTEGER DEFAULT 0,
    preview_url TEXT,                       -- 本轮 Sprint 的预览地址
    review_notes TEXT,                      -- Captain LLM 的审查笔记
    user_feedback TEXT,                     -- 用户反馈（纳入下一轮规划）
    started_at  DATETIME,
    completed_at DATETIME,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- MissionStep 增加 sprint 关联
ALTER TABLE mission_steps ADD COLUMN sprint_id TEXT REFERENCES sprints(id);

-- Mission 增加 Sprint 控制字段
ALTER TABLE missions ADD COLUMN current_sprint INTEGER DEFAULT 0;
ALTER TABLE missions ADD COLUMN max_sprints INTEGER DEFAULT 4;
```

#### 20.7.5 Sprint 引擎流程

```go
// Squad Engine Sprint 循环（伪代码）
func (e *Engine) runMissionSprints(mission Mission) {
    // 1. 创建 Git bare repo
    repoPath := initMissionRepo(mission.ID)

    for sprintNum := 0; sprintNum < mission.MaxSprints; sprintNum++ {
        // 2. Sprint Planning: LLM 规划本轮目标和 Steps
        sprint := e.planSprint(mission, sprintNum, previousResults, userFeedback)

        // 3. 分配分支，创建 Steps
        for _, step := range sprint.Steps {
            step.Branch = fmt.Sprintf("sprint-%d/%s", sprintNum, step.Role)
            e.dispatchStep(step)
        }

        // 4. 等待所有 Steps 完成
        e.waitForSprintCompletion(sprint)

        // 5. CI Gate: merge + test + build
        mergeResult := e.mergeSprintBranches(sprint, repoPath)
        if mergeResult.HasConflicts {
            e.resolveConflictsWithLLM(mergeResult)
        }

        testResult := e.runTests(repoPath)
        buildResult := e.runBuild(repoPath)

        // 6. Quality Gate
        if !testResult.Passed || !buildResult.Passed {
            // 自动重试失败的 Steps
            e.retryFailedSteps(sprint)
            continue
        }

        // 7. Preview
        sprint.PreviewURL = e.startPreview(repoPath)

        // 8. Sprint Review: Captain LLM 审查
        sprint.ReviewNotes = e.reviewSprint(sprint, mergeResult)

        // 9. 检查是否需要继续
        if sprint.ReviewNotes.IsComplete {
            mission.Status = "completed"
            break
        }

        // 10. 等待用户反馈（可选，超时自动继续）
        userFeedback = e.waitForUserFeedback(sprint, timeout: 5*time.Minute)
    }
}
```

#### 20.7.6 自动 Code Review（Agent 交叉审查）

每个 Step 完成后，由另一个角色的 Agent 自动审查代码：

```
审查矩阵:
┌──────────────┬───────────────────┬──────────────────────┐
│ 产出者        │ 审查者             │ 审查重点              │
├──────────────┼───────────────────┼──────────────────────┤
│ claw-backend │ claw-qa           │ 安全、错误处理、测试性  │
│ claw-frontend│ claw-design       │ UI 一致性、可访问性    │
│ claw-frontend│ claw-backend      │ API 接口匹配          │
│ claw-pm      │ claw-lead(架构师)  │ 需求完整性、可行性     │
│ claw-devops  │ claw-backend      │ 配置正确性、安全性     │
└──────────────┴───────────────────┴──────────────────────┘

审查流程:
  1. claw-backend 推送 feat/backend 分支
  2. Captain 自动创建 Review Task → 分发给 claw-qa
  3. claw-qa 用 git.diff + code.read_file 审查代码
  4. 审查结果:
     → approved: Step 标记 done，进入 merge
     → changes_requested: 打回给 claw-backend，附修改建议
     → 最多重试 3 次
```

```sql
-- 新增 Review 记录
CREATE TABLE step_reviews (
    id          TEXT PRIMARY KEY,
    step_id     TEXT NOT NULL REFERENCES mission_steps(id),
    reviewer_node TEXT NOT NULL,             -- 审查者节点 ID
    status      TEXT DEFAULT 'pending',     -- pending | approved | changes_requested
    comments    TEXT,                       -- 审查意见
    diff_summary TEXT,                      -- 变更摘要
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

#### 20.7.7 CI Gate（质量门禁）

```
每个 Sprint 完成后自动执行质量检查:

┌─────────────────────────────────────────────────┐
│                 CI Pipeline                       │
│                                                   │
│  Stage 1: Merge                                  │
│    git merge --no-ff feat/frontend → main         │
│    git merge --no-ff feat/backend → main          │
│    → 冲突? → LLM 辅助解决 or 标记失败              │
│                                                   │
│  Stage 2: Lint (可选)                             │
│    eslint src/ (前端)                              │
│    pylint api/ (后端)                              │
│    → 警告记录，不阻塞                               │
│                                                   │
│  Stage 3: Test                                    │
│    npm test (前端测试)                              │
│    pytest tests/ (后端测试)                         │
│    → 失败 → Sprint 标记 needs_fix → 重试           │
│                                                   │
│  Stage 4: Build                                   │
│    npm run build (前端构建)                         │
│    go build / pip install (后端)                    │
│    → 失败 → Sprint 标记 build_failed → 重试        │
│                                                   │
│  Stage 5: Preview                                 │
│    start_app → 分配端口 → preview URL              │
│    → 成功 → Sprint 标记 done ✅                    │
│                                                   │
│  所有 Stage 通过 = Definition of Done              │
└─────────────────────────────────────────────────┘
```

#### 20.7.8 Sprint Retrospective（LLM 自优化）

```
每个 Sprint 结束后，Captain LLM 自动分析:

输入:
  - 本轮 Sprint 的所有 Step 执行记录
  - 失败/重试的 Step 及其错误信息
  - Code Review 的审查意见
  - CI Pipeline 的结果
  - 用户反馈（如果有）

分析:
  1. 成功率: 6/8 Steps 一次通过 (75%)
  2. 失败原因:
     - claw-backend: API 返回格式与前端不一致 → 下轮需要先定义接口契约
     - claw-frontend: build 失败，缺少依赖 → 下轮 Step 需包含 npm install
  3. 质量问题:
     - 后端缺少输入验证 → 下轮 prompt 强调安全性
     - 前端无错误边界 → 下轮 prompt 要求 error handling

输出 (纳入下一轮 Sprint Planning):
  - 调整后的 Step 描述（更精确的 prompt）
  - 新增的约束条件
  - 优化的任务分配
```

#### 20.7.9 用户反馈循环

```
Sprint 完成 → Preview 就绪 → 通知用户
  │
  ├── 用户查看 Preview URL
  │
  ├── 用户反馈方式:
  │   ├── 1. Web UI: Mission 页面的反馈输入框
  │   ├── 2. API: POST /v1/squads/{id}/missions/{mid}/feedback
  │   └── 3. Chat: 在 claw-lead 聊天中直接说
  │
  ├── 反馈示例:
  │   "首页看起来不错，但新闻列表需要加分页，
  │    另外颜色太暗了，换成浅色主题"
  │
  └── 下一轮 Sprint Planning 时:
      Captain LLM 将用户反馈纳入规划:
        Sprint 3 新增:
          - Step: claw-frontend 修改为浅色主题
          - Step: claw-backend 实现分页 API
          - Step: claw-frontend 实现分页组件
```

#### 20.7.10 完整 Sprint 架构图

```
┌──────────────────────────────────────────────────────────────────┐
│                    Agile Squad Architecture                        │
│                                                                   │
│  User ──feedback──► Captain (claw-lead :8087)                     │
│    │                  │                                           │
│    │ view preview     ├── Sprint Planner (LLM)                    │
│    │                  ├── CI Pipeline (merge/test/build)          │
│    │                  ├── Review Coordinator (交叉审查调度)         │
│    │                  ├── Retro Analyzer (LLM 自优化)             │
│    │                  └── Preview Server (ProcessManager)         │
│    │                                                              │
│    │   ┌──────────── Sprint Loop ────────────┐                    │
│    │   │                                      │                   │
│    │   │  Plan → Dispatch → Execute → Review  │                   │
│    │   │    ↑                          │      │                   │
│    │   │    └── Retro + Feedback ◄─────┘      │                   │
│    │   │                                      │                   │
│    │   └──────────────────────────────────────┘                   │
│    │                  │                                           │
│    │          ┌───────┼───────┐───────┐───────┐                   │
│    │          │       │       │       │       │                   │
│    │       ┌──▼──┐ ┌──▼──┐ ┌──▼──┐ ┌──▼──┐ ┌──▼──┐              │
│    │       │ PM  │ │ FE  │ │ BE  │ │ QA  │ │ Ops │              │
│    │       │:8084│ │:8082│ │:8081│ │:8083│ │:8086│              │
│    │       └─────┘ └─────┘ └─────┘ └─────┘ └─────┘              │
│    │                                                              │
│    └──► Preview: http://localhost:{port}/ ◄── CI Pipeline 产出    │
│                                                                   │
│  Git Repo: ~/.spore/repos/mission-{id}.git                       │
│    main ← merge ← sprint-0/* ← sprint-1/* ← sprint-2/*          │
└──────────────────────────────────────────────────────────────────┘
```

#### 20.7.11 实现优先级（敏捷扩展）

| 优先级 | 任务 | 预估 | 阶段 | 效果 |
|--------|------|------|------|------|
| P0 | Sprint 数据模型 + 基础生命周期 | 2h | Ph1 | Mission 支持多轮迭代 |
| P0 | Sprint Planning (LLM 分轮规划) | 3h | Ph1 | 增量式任务分解 |
| P1 | CI Gate (merge + test + build) | 3h | Ph1 | 质量自动保障 |
| P1 | Sprint 完成后自动 Preview | 1h | Ph1 | 每轮可查看结果 |
| P1 | 用户反馈 API + UI 入口 | 2h | Ph1 | 用户参与纠偏 |
| P2 | Agent 交叉 Code Review | 3h | Ph1+ | 代码质量提升 |
| P2 | Sprint Retro (LLM 自优化) | 2h | Ph1+ | prompt 自动迭代 |
| P3 | Kanban Board UI (Mission 看板) | 4h | Ph2 | 可视化管理 |

### 20.8 3D 可视化指挥中心（Hive Mind Dashboard）

> 虫族 Overlord 视角：俯瞰整个基地，每个 Claw 是一个活体单位，
> 实时看到谁在写代码、谁在测试、代码在虫道中流动。

#### 20.8.1 设计理念

整个项目的命名体系源自星际争霸虫族（Spore/Nydus/Creep/Hatchery/Queen）。
3D 可视化将这套隐喻具象化为**可交互的虫族基地**：

```
你是虫族 Overlord，悬浮在基地上空，俯瞰一切：

  中央是 Hatchery (主巢) — claw-lead / Captain
  脉动发光，处理所有指挥调度

  周围 6 个建筑 — 各 Claw 节点
  每个建筑根据角色有不同形态
  工作时发光脉动，空闲时暗淡

  建筑之间有 Nydus 虫道
  代码/数据以发光粒子在虫道中流动
  粒子颜色 = 数据类型（代码/测试/部署）

  地面覆盖 Creep 菌毯
  菌毯面积 = Sprint 整体进度
  从 Hatchery 向外扩张

  完成的 Preview 部署 = Spore Colony（孢子殖民地）
  立在基地边缘，点击可访问预览 URL
```

#### 20.8.2 虫族建筑映射

| Claw 实例 | 端口 | 虫族建筑 | 3D 形态 | 工作动画 |
|-----------|------|---------|---------|---------|
| **claw-lead** | :8087 | Hatchery (主巢) | 巨型有机球体，顶部冠状触角 | 脉动 + 发射指令光波 |
| **claw-backend** | :8081 | Spawning Pool (产卵池) | 地面液态池，气泡翻涌 | 液面沸腾 + 代码符号浮起 |
| **claw-frontend** | :8082 | Evolution Chamber (进化腔) | 半透明卵形舱，内部可见 | 内部光线旋转 + UI 碎片闪烁 |
| **claw-qa** | :8083 | Spire (塔刺) | 高耸尖塔，顶部扫描光束 | 360° 扫描射线 + 红/绿脉冲 |
| **claw-pm** | :8084 | Hydralisk Den (巢穴) | 洞穴状结构，内壁发光 | 文档符号从洞口飘出 |
| **claw-design** | :8085 | Roach Warren (虫穴) | 圆形半地下结构，表面纹路 | 表面纹路流动变色 |
| **claw-devops** | :8086 | Nydus Network (虫道入口) | 巨型虫洞门框，旋转漩涡 | 漩涡加速 + 部署粒子射出 |

#### 20.8.3 视觉状态系统

每个节点通过颜色、动画、粒子三维度表达状态：

```
状态颜色:
  🟢 绿色脉动  = 正在执行任务 (running)
  🔵 蓝色稳定  = 空闲等待 (idle)
  🟡 黄色闪烁  = 等待依赖 (waiting)
  🔴 红色警报  = 执行失败 (failed)
  🟣 紫色流转  = Code Review 中 (reviewing)
  ⚪ 白色辉光  = 任务完成 (done)

动画强度:
  高频脉动 = CPU 密集（LLM 推理中）
  低频呼吸 = I/O 等待（等待响应）
  静止暗淡 = 完全空闲
  震动 + 红光 = 错误/崩溃

粒子效果:
  💚 绿色粒子 = 代码文件传输
  💛 黄色粒子 = 测试数据
  🔵 蓝色粒子 = 配置/部署指令
  ❤️ 红色粒子 = 错误报告
  ⭐ 金色粒子 = Git commit 推送
```

#### 20.8.4 四大视图模式

**View 1: Hive View（鸟瞰全局）— 默认视图**

```
┌──────────────────────────────────────────────────────────────────┐
│  ┌─ 顶部状态栏 ──────────────────────────────────────────────┐  │
│  │ Mission: 新闻门户网站  │  Sprint 2/4  │  67% ████████░░░  │  │
│  │ Active: 5/7 nodes     │  Steps: 8/12  │  ⏱ 12:34 elapsed  │  │
│  └────────────────────────────────────────────────────────────┘  │
│                                                                   │
│                     ╭── Overlord Info ──╮                         │
│                     │ 📋 当前 Sprint 2   │                         │
│                     │ 目标: 完整功能     │                         │
│                     ╰──────────────────╯                         │
│                                                                   │
│         🟢 Spire                      🟡 Hydra Den               │
│         (QA: 运行测试)                (PM: 等待中)                │
│          ╲  ·····✨粒子流✨·····  ╱                              │
│           ╲                      ╱                                │
│            ╲                    ╱                                  │
│    🟢═══════ 🔵 HATCHERY 🔵═══════🟢                             │
│    Evo Chamber  (Captain:调度中)  Spawning Pool                   │
│    (FE:写React)      ║          (BE:写API)                       │
│            ╱         ║              ╲                             │
│           ╱          ║               ╲                            │
│          ╱    ·✨粒子✨·    ╲                                     │
│    🟠 Roach Warren    ║     🔵 Nydus Network                     │
│    (Design:设计中)     ║     (DevOps:空闲)                        │
│                       ║                                           │
│  ──── Creep 菌毯 (67%) ════════════════════                      │
│  ░░░░░░░██████████████████████████░░░░░░░░░                      │
│                                                                   │
│  ┌─ 底部工具栏 ─────────────────────────────────────────────┐   │
│  │ [🏠 Hive] [📋 Kanban] [⏱ Timeline] [🔍 Node] [💻 Code] │   │
│  └──────────────────────────────────────────────────────────┘   │
└──────────────────────────────────────────────────────────────────┘
```

**View 2: Kanban View（敏捷看板 — 2D 覆盖层 + 3D 背景）**

```
┌──────────────────────────────────────────────────────────────────┐
│  3D 场景模糊为背景，前景为半透明看板                                │
│                                                                   │
│  ┌─Backlog─┐ ┌─Sprint 2─┐ ┌─Running──┐ ┌─Review──┐ ┌──Done──┐  │
│  │         │ │          │ │          │ │         │ │        │  │
│  │ ┌─────┐ │ │          │ │ ┌──────┐ │ │         │ │┌──────┐│  │
│  │ │S3-01│ │ │          │ │ │S2-03 │ │ │ ┌─────┐ │ ││S2-01 ││  │
│  │ │响应式│ │ │          │ │ │🟢    │ │ │ │S2-02│ │ ││✅    ││  │
│  │ │claw- │ │ │          │ │ │后端API│ │ │ │🟣   │ │ ││需求  ││  │
│  │ │fe    │ │ │          │ │ │claw- │ │ │ │前端  │ │ ││claw- ││  │
│  │ │      │ │ │          │ │ │back  │ │ │ │claw- │ │ ││pm    ││  │
│  │ └─────┘ │ │          │ │ │:8081 │ │ │ │fe    │ │ │└──────┘│  │
│  │ ┌─────┐ │ │          │ │ │78%   │ │ │ │:8082 │ │ │┌──────┐│  │
│  │ │S3-02│ │ │          │ │ └──────┘ │ │ └─────┘ │ ││S1-*  ││  │
│  │ │SEO  │ │ │          │ │ ┌──────┐ │ │         │ ││✅    ││  │
│  │ │claw- │ │ │          │ │ │S2-04 │ │ │         │ ││Sprint1││  │
│  │ │devops│ │ │          │ │ │🟢    │ │ │         │ ││全部   ││  │
│  │ └─────┘ │ │          │ │ │测试  │ │ │         │ │└──────┘│  │
│  │         │ │          │ │ │claw- │ │ │         │ │        │  │
│  │         │ │          │ │ │qa    │ │ │         │ │        │  │
│  │         │ │          │ │ └──────┘ │ │         │ │        │  │
│  └─────────┘ └──────────┘ └──────────┘ └─────────┘ └────────┘  │
│                                                                   │
│  每张卡片显示: 任务名 | 节点 | 分支 | 进度条 | 状态色             │
│  点击卡片 → 跳转到 Node Detail View                               │
│  拖拽卡片 → 手动调整优先级（未来）                                  │
└──────────────────────────────────────────────────────────────────┘
```

**View 3: Node Detail View（点击建筑 → 沉浸式放大）**

```
┌──────────────────────────────────────────────────────────────────┐
│  镜头推进 → Spawning Pool (claw-backend) 放大旋转                  │
│                                                                   │
│  ┌─ 左侧: 3D 建筑特写 ──┐  ┌─ 右侧: 详情面板 ────────────────┐  │
│  │                       │  │                                  │  │
│  │   🟢 Spawning Pool    │  │  claw-backend (:8081)           │  │
│  │                       │  │  Zerg Unit: Spawning Pool       │  │
│  │   [3D 模型旋转展示]    │  │  Status: 🟢 执行中              │  │
│  │   液面沸腾动画         │  │  Uptime: 2h 15m                │  │
│  │   代码符号浮起         │  │  CPU: 34%  RAM: 128MB          │  │
│  │                       │  │                                  │  │
│  │   周围浮动:           │  │  ── 当前任务 ──                  │  │
│  │   📄 news.py (刚写入)  │  │  Sprint 2 / Step S2-03         │  │
│  │   📄 models.py        │  │  Branch: feat/backend           │  │
│  │   📄 test_news.py     │  │  "实现新闻 CRUD API + 分页"      │  │
│  │                       │  │  Progress: ████████░░ 78%       │  │
│  │                       │  │  Commits: 3                     │  │
│  └───────────────────────┘  │                                  │  │
│                              │  ── 实时输出 (终端) ──           │  │
│  ┌─ 底部: Git 历史 ───────┐ │  ┌────────────────────────────┐ │  │
│  │ ● abc123 feat: news    │ │  │ > code.write_file(         │ │  │
│  │ │  API CRUD             │ │  │ >   "api/news.py",        │ │  │
│  │ ● def456 feat: add     │ │  │ >   content=...)           │ │  │
│  │ │  pagination           │ │  │ > ✅ 文件已写入 (2.3KB)    │ │  │
│  │ ● 789abc feat: news    │ │  │ > code.run_command(        │ │  │
│  │    model + migration    │ │  │ >   "pytest tests/")       │ │  │
│  └─────────────────────────┘ │  │ > 🟢 3 passed, 0 failed   │ │  │
│                              │  └────────────────────────────┘ │  │
│                              └──────────────────────────────────┘  │
│  [← Back to Hive]  [📋 View Branch Diff]  [🔄 Force Retry]       │
└──────────────────────────────────────────────────────────────────┘
```

**View 4: Timeline View（Sprint 时间线 — 水平滚动）**

```
┌──────────────────────────────────────────────────────────────────┐
│  3D 背景: 虫道从左到右延伸，时间粒子在上面流动                      │
│                                                                   │
│  Sprint 0          Sprint 1          Sprint 2         Sprint 3   │
│  ┌─骨架─┐         ┌──MVP──┐         ┌─功能──┐        ┌─打磨─┐   │
│  │ ✅    │─────────│ ✅    │─────────│ 🔄    │· · · · │ ⏳   │   │
│  │ 5/5   │         │ 4/4   │         │ 8/12  │        │ ?/?  │   │
│  │ 2min  │         │ 8min  │         │ 12min │        │      │   │
│  └───────┘         └───────┘         └───────┘        └──────┘   │
│       │                 │                 │                       │
│  Preview v0.1      Preview v0.2      Preview v0.3                │
│  (空壳能跑)        (MVP 可用)        (building...)               │
│       │                 │                                         │
│  ┌─ Git 提交流 ─────────────────────────────────────────────┐    │
│  │ ●──●──●──●──●──●──●──●──●──●──●──●──●──●──●──●──●──●    │    │
│  │ │  merge  │  merge  │      │  │     ↑ 当前               │    │
│  │ S0       S1        S2-01  S2-03  S2-04                   │    │
│  └──────────────────────────────────────────────────────────┘    │
│                                                                   │
│  点击任意 Sprint → 展开该轮的 Step 详情                            │
│  点击任意 Commit → 查看 diff                                      │
└──────────────────────────────────────────────────────────────────┘
```

#### 20.8.5 交互设计

```
鼠标/触摸操作:
  🖱️ 左键拖拽     → 旋转视角（OrbitControls）
  🖱️ 滚轮缩放     → 缩放场景
  🖱️ 右键拖拽     → 平移视角
  🖱️ 单击建筑     → 选中，显示简要信息气泡
  🖱️ 双击建筑     → 进入 Node Detail View（镜头推进动画）
  🖱️ 点击虫道     → 高亮数据流路径
  🖱️ 悬浮 Overlord → 展开全局状态面板

键盘快捷键:
  1 → Hive View（鸟瞰全局）
  2 → Kanban View（敏捷看板）
  3 → Timeline View（时间线）
  4 → Code Flow View（代码流）
  H → 回到 Home（重置视角）
  Space → 暂停/恢复 Sprint
  F → 聚焦当前活跃节点
```

#### 20.8.6 实时数据流架构

```
┌────────────────────────────────────────────────────────────┐
│                    WebSocket 实时架构                        │
│                                                             │
│  claw-backend ─┐                                           │
│  claw-frontend ┤                                           │
│  claw-qa ──────┤  Heartbeat (5s)                           │
│  claw-pm ──────┼──────────────► Captain (claw-lead)        │
│  claw-design ──┤                    │                      │
│  claw-devops ──┘                    │ 聚合                  │
│                                     ▼                      │
│                              ┌─────────────┐               │
│                              │ WebSocket   │               │
│                              │ /ws/hive    │               │
│                              └──────┬──────┘               │
│                                     │ push                 │
│                                     ▼                      │
│                         ┌──────────────────────┐           │
│                         │  3D Dashboard (浏览器) │           │
│                         │  React Three Fiber    │           │
│                         └──────────────────────┘           │
│                                                             │
│  推送消息类型:                                               │
│  {                                                          │
│    "type": "node_status",                                   │
│    "node_id": "claw:xxxx",                                  │
│    "role": "backend",                                       │
│    "status": "running",                                     │
│    "current_task": "实现新闻 CRUD API",                      │
│    "progress": 78,                                          │
│    "branch": "feat/backend",                                │
│    "last_action": "write_file: api/news.py",                │
│    "cpu": 34, "ram_mb": 128                                 │
│  }                                                          │
│                                                             │
│  {                                                          │
│    "type": "data_flow",                                     │
│    "from": "claw-backend",                                  │
│    "to": "claw-lead",                                       │
│    "data_type": "git_push",                                 │
│    "size_kb": 12                                            │
│  }                                                          │
│                                                             │
│  {                                                          │
│    "type": "sprint_progress",                               │
│    "sprint_number": 2,                                      │
│    "done_steps": 8,                                         │
│    "total_steps": 12,                                       │
│    "creep_coverage": 0.67                                   │
│  }                                                          │
└────────────────────────────────────────────────────────────┘
```

#### 20.8.7 技术栈

```
3D 渲染层:
  ├── React Three Fiber (R3F)    — React 集成 Three.js
  ├── @react-three/drei           — 辅助组件（OrbitControls, Text, HTML）
  ├── @react-three/postprocessing — 后处理（Bloom 辉光, SSR）
  └── Three.js                    — 底层 WebGL

3D 资源:
  ├── GLTF/GLB 模型               — 虫族建筑 3D 模型（可用 Blender 制作或生成式 AI）
  ├── Instanced Mesh              — 粒子系统（数千个发光粒子）
  ├── Custom Shaders (GLSL)       — 菌毯扩散、虫道发光、脉动效果
  └── Environment Map              — 暗色太空/洞穴环境

UI 层:
  ├── React + TailwindCSS          — 2D 覆盖层（面板、看板、状态栏）
  ├── @react-three/drei HTML       — 3D 场景内嵌 HTML（节点标签）
  ├── Framer Motion                — 2D 面板动画
  └── Zustand                      — 全局状态管理

数据层:
  ├── WebSocket (native)           — 实时数据推送
  ├── REST API                     — 初始数据加载、历史查询
  └── LocalStorage                 — 视角偏好、布局记忆
```

#### 20.8.8 3D 场景分层

```
Layer 0: 环境 (Environment)
  ├── 太空/深渊背景 (Skybox)
  ├── 环境光 + 点光源（建筑自发光）
  └── 雾效 (距离感)

Layer 1: 地形 (Terrain)
  ├── 基础地面（暗色岩石纹理）
  └── Creep 菌毯（自定义 Shader，从中心向外扩散）
      └── 面积 = Sprint 进度百分比

Layer 2: 建筑 (Structures)
  ├── 7 个虫族建筑 GLTF 模型
  ├── 每个建筑独立的脉动/旋转动画
  ├── 状态颜色 emissive 材质
  └── 悬浮标签 (HTML in 3D)

Layer 3: 连接 (Connections)
  ├── Nydus 虫道（管道几何体 + 发光材质）
  └── 数据粒子（Instanced Mesh + 路径动画）
      └── 颜色/速度 = 数据类型/大小

Layer 4: 特效 (Effects)
  ├── Bloom 辉光（活跃节点发光溢出）
  ├── 粒子爆发（commit 完成瞬间）
  ├── 扫描光束（QA 节点专属）
  └── 部署光波（从 DevOps 向外扩散）

Layer 5: UI 覆盖 (Overlay)
  ├── 顶部状态栏
  ├── 底部工具栏/视图切换
  ├── 侧边详情面板
  └── Kanban 看板（全屏覆盖模式）
```

#### 20.8.9 特色动画效果

**1. Mission 启动 — 孵化动画**
```
用户点击 "Start Mission"
  → Hatchery 剧烈脉动
  → 光波从中心向外扩散
  → 6 个建筑依次亮起（像虫卵孵化）
  → Nydus 虫道连线建立
  → 第一批指令粒子从 Hatchery 射出
  → Sprint 0 开始
```

**2. 代码提交 — 金色粒子爆发**
```
某个 Claw 完成 git push
  → 该建筑释放一簇金色粒子
  → 粒子沿虫道飞向 Hatchery
  → Hatchery 吸收粒子，闪烁一下
  → 进度条 +1
  → Creep 菌毯微微扩展
```

**3. Sprint 完成 — 菌毯扩张 + 殖民地生成**
```
Sprint CI Gate 全部通过
  → 所有虫道同时脉冲白光
  → Creep 菌毯快速向外扩展一圈
  → 基地边缘生长出 Spore Colony（预览部署）
  → Colony 上方浮现 Preview URL
  → 全局短暂的金色辉光（庆祝）
```

**4. 步骤失败 — 红色警报**
```
某个 Step 执行失败
  → 该建筑剧烈震动
  → 颜色变红，释放红色粒子
  → 虫道中传回红色粒子到 Hatchery
  → Hatchery 闪烁红光一次
  → 自动重试: 建筑颜色变黄 → 重新尝试 → 成功变绿
```

**5. Code Review — 紫色审查光束**
```
Step 完成，进入 Review
  → 审查者建筑(如 Spire/QA) 发射紫色光束
  → 光束扫描被审查者建筑
  → 审查通过: 光束变绿 ✅
  → 审查不通过: 光束变红 ❌ → 打回
```

#### 20.8.10 响应式设计

```
桌面 (>1280px):    完整 3D 场景 + 侧边面板 + 底部工具栏
平板 (768-1280px): 3D 场景全屏 + 浮动面板（点击展开）
手机 (<768px):     简化 3D（低多边形）+ 底部抽屉式面板
                   或退化为 2D Kanban + 节点列表

性能优化:
  ├── LOD (Level of Detail): 远距离低模，近距离高模
  ├── Instanced Rendering: 粒子系统用 GPU 实例化
  ├── Frustum Culling: 视锥外物体不渲染
  ├── WebSocket 节流: 状态更新 5s 间隔，数据流事件实时
  └── 按需加载: GLTF 模型懒加载 + 纹理压缩 (KTX2)
```

#### 20.8.11 集成位置

```
3D Dashboard 的入口:

1. Spore Desktop (http://localhost:7890)
   └── 新增 "Hive Mind" 菜单项
       └── 内嵌 iframe 或新窗口打开 3D Dashboard

2. Claw Web UI (http://localhost:8087)
   └── Squad → Mission 详情页
       └── "3D View" 按钮 → 打开 Dashboard

3. 独立部署
   └── npx serve dist/ → 任意端口
   └── 通过 API 连接 Captain 获取数据

数据来源:
  GET  /v1/squads/{id}/dashboard     → 初始全景数据
  GET  /v1/squads/{id}/missions/{id} → Mission + Sprint + Steps
  WS   /ws/hive?squad_id={id}        → 实时推送
```

#### 20.8.12 实现优先级

| 优先级 | 任务 | 预估 | 效果 |
|--------|------|------|------|
| P0 | R3F 项目搭建 + 基础 3D 场景 | 4h | 能看到 3D 空间 |
| P0 | 7 个节点几何体 + 状态颜色 + 标签 | 3h | 看到每个 Claw |
| P1 | Nydus 虫道连线 + 粒子系统 | 4h | 看到数据流动 |
| P1 | WebSocket 实时数据 + 状态同步 | 3h | 实时更新 |
| P1 | Kanban 2D 覆盖层 | 4h | 敏捷看板可用 |
| P2 | GLTF 虫族建筑模型 | 8h | 视觉质量飞跃 |
| P2 | Creep 菌毯 Shader + 扩散动画 | 4h | Sprint 进度可视化 |
| P2 | Node Detail View（镜头推进 + 终端） | 4h | 节点深入查看 |
| P2 | Timeline View | 3h | Sprint 历史 |
| P3 | Bloom/后处理 + 特效动画 | 4h | 视觉震撼 |
| P3 | 响应式 + 性能优化 | 3h | 多设备支持 |
| P3 | 独立可部署前端包 | 2h | 灵活部署 |

---

## 20.9 Phase 1 & 2 实现记录

### 20.9.1 Phase 1: 本地 Git 协作 ✅

**实现时间**: 2026-03-17

**新建文件**:
- `claw/api/internal/tool/git_tool.go` — Agent Git 工具，12 个操作 (init/clone/add/commit/push/pull/branch/checkout/merge/status/log/diff)
- `claw/api/internal/squad/git.go` — Squad 级 Git 仓库管理器 (InitMissionRepo/CloneForStep/MergeBranches)

**数据模型扩展**:
- Mission: +RepoPath, WorkspacePath, CurrentSprint, MaxSprints, PreviewURL
- MissionStep: +SprintID, Branch, CommitHash
- 新增 Sprint 表 (mission_id/number/goal/status/total_steps/done_steps/review_notes/user_feedback)
- 新增 StepReview 表 (step_id/reviewer_node/status/comments/diff_summary)

**Engine 改造**:
- planAndDispatch: 创建 bare repo → Sprint 记录 → 分支分配 (sprint-{n}/step-{i}-{specialty})
- dispatchStep: 传递 repo_path + branch 给远程节点
- executeLocal: clone workspace → checkout branch → 注入 Git 工作流指令
- checkMissionComplete: 自动 merge 所有分支到 master → 更新 Sprint 状态
- LLM Prompt: 要求产出实际代码文件 + git add/commit/push

### 20.9.2 Phase 2: Nydus Git P2P 跨网络协作 ✅

**实现时间**: 2026-03-17

**新建文件**:
- `claw/api/internal/squad/git_http.go` — Git Smart HTTP Protocol 服务端
- `claw/api/internal/squad/git_relay.go` — Nydus Relay Git 代理 (NAT 穿透)

**Git HTTP Smart Protocol**:
```
GET  /v1/git/{repo}/info/refs?service=git-upload-pack    → 克隆/拉取广告
GET  /v1/git/{repo}/info/refs?service=git-receive-pack   → 推送广告
POST /v1/git/{repo}/git-upload-pack                       → 克隆/拉取数据
POST /v1/git/{repo}/git-receive-pack                      → 推送数据
GET  /v1/git/{repo}/HEAD                                  → HEAD 引用
```

**跨网络协作流程**:
```
Captain Node (有公网 IP 或 Relay)
    │
    ├─ /app/repos/mission-{id}.git (bare repo)
    ├─ Git HTTP Server @ /v1/git/mission-{id}/*
    │
    ├── 本地节点: file:// 直接访问
    └── 远程节点: http://{captain}/v1/git/mission-{id} (Smart HTTP)
         │
         ├── 直连: 直接 HTTP clone/push
         └── NAT: GitRelayProxy → Nydus Relay → Captain GitHTTPHandler
```

**GitManager 升级**:
- SetSelfAddress(): 设置节点 HTTP 地址
- RepoHTTPURL(): 生成 `http://{addr}/v1/git/mission-{id}` URL
- InitMissionRepo(): 自动 EnableReceivePack + UpdateServerInfo

**GitRelayProxy 三层策略**:
1. **直连**: 节点有公网地址 → 直接 HTTP 请求
2. **Relay**: 节点在 NAT 后 → 通过 Nydus Relay 转发 Git HTTP 请求/响应
3. **HandleRelayedGitRequest**: 目标节点接收 Relay 转发的 Git 请求，交给 GitHTTPHandler 处理

**编译状态**: go build ✅ | go vet ✅

### 20.9.3 3D Hive Mind Dashboard 实现 ✅

**实现时间**: 2026-03-17

**新建文件**:
- `claw/web/src/pages/HiveMindPage.tsx` — React Three Fiber 3D 虫族基地可视化
- `claw/api/internal/squad/hive_broadcast.go` — WebSocket 实时数据推送

**依赖**:
```
@react-three/fiber@8.17.10
@react-three/drei@9.114.3
@react-three/postprocessing (Bloom)
three@0.169.0
```

**3D 场景组件**:
- **ZergNode** (7 个) — 每个 Claw 实例对应一个虫族建筑，独立几何体 + 状态颜色 + 脉动动画
- **NydusCanal** — Captain 到各节点的发光连线
- **FlowParticle** — 3 个粒子沿虫道流动，模拟数据传输
- **CreepGround** — 菌毯面积 = Sprint 进度，外圈脉冲光环
- **FloatingParticles** — 150 个漂浮粒子营造氛围
- **Bloom** — 后处理辉光效果 (luminanceThreshold=0.2, intensity=1.5)

**状态映射**:
| 状态 | 颜色 | 动画 |
|------|------|------|
| running | #00ff88 绿 | 高频脉动 |
| idle | #4488ff 蓝 | 低频呼吸 |
| waiting | #ffaa00 黄 | 中频闪烁 |
| failed | #ff3344 红 | 线框 + 震动 |
| reviewing | #aa44ff 紫 | 流转 |
| done | #ffffff 白 | 辉光 |

**3 种视图模式** (键盘 1/2/3 切换):
1. **Hive View** — 3D 鸟瞰，点击建筑弹出 Node Detail Panel (终端输出 + Git 历史 + 任务信息)
2. **Kanban View** — 半透明看板覆盖 (Pending/Running/Review/Done/Failed)
3. **Timeline View** — Sprint 时间线，进度条 + 状态标记

**实时数据**:
- HiveBroadcaster: 每 5s 聚合活跃 Mission 状态 → WebSocket 推送
- 事件类型: hive_step_update, hive_sprint, hive_status, hive_data_flow
- 前端: starclawWS.on() 实时更新 3D 场景 + 补充 10s 轮询兜底

**后端新增 API**:
- `GET /missions` — 所有 Mission 列表
- `GET /missions/:id/steps` — Mission 步骤列表
- `GET /missions/:id/sprints` — Sprint 列表

**路由 + 导航**:
- `/hivemind` 路由, Radar 图标, 蜂巢/Hive Mind i18n

**编译状态**: go build ✅ | go vet ✅ | tsc --noEmit ✅

### 20.9.4 敏捷 Sprint 生命周期实现 ✅

**实现时间**: 2026-03-17

**新建文件**:
- `claw/api/internal/squad/sprint_lifecycle.go` — Sprint 全生命周期管理

**20.7.6 自动 Code Review**:
- ReviewMatrix: 6 个角色映射 (Backend→QA, Frontend→Design, QA→Backend, PM→Captain, Design→Frontend, DevOps→Backend)
- 每个 Step 完成后自动创建 StepReview → 分派给对应 Reviewer 角色
- LLM 审查: 分析步骤产出 → JSON 结果 (approved / changes_requested)
- 最多 3 次 Review 重试，超限自动批准
- guessRoleFromStep: 从 task/agent/node 关键词推断角色

**20.7.7 CI Gate (质量门禁)**:
- checkSprintComplete: 所有 Step 完成后触发 CI 管道
- Stage 1 — Git Merge: 合并所有分支到 master
- Stage 2 — LLM Quality Check: LLM 评分 1-10，≥5 分通过
- CIGateResult 结构: Passed/Stage/Message/Details
- Sprint 状态流: executing → reviewing → done/failed

**20.7.8 Sprint Retrospective**:
- runRetrospective: Sprint 完成后 LLM 自动分析
- 输入: Step 执行记录 + Review 意见 + CI 结果 + 用户反馈
- 输出: RetroResult (成功率 / 失败分析 / 质量问题 / 改进建议 / 下轮提示)
- Retro 结果存入 Sprint.ReviewNotes

**20.7.9 用户反馈循环**:
- `POST /missions/:id/feedback` — 用户提交反馈
- `GET /missions/:id/reviews` — 查看所有 Code Review 记录
- 反馈存入 Sprint.UserFeedback，纳入下轮 Planning 和 Retrospective

**引擎流程变更**:
```
Step 执行完成
  → triggerAutoReview (ReviewMatrix 匹配 Reviewer)
    → LLM 审查代码
      → approved → finalizeStepDone
      → changes_requested → 记录反馈 → 自动批准 (重试支持待扩展)
  → checkSprintComplete (所有 Step 完成?)
    → runCIGate (Git Merge + LLM Quality Check)
      → CI PASS → runRetrospective → completeMission
      → CI FAIL → failSprint
```

**编译状态**: go build ✅ | go vet ✅ | tsc --noEmit ✅

### 20.9.5 多 Sprint 迭代 + 自动 Preview ✅

**实现时间**: 2026-03-17

**多 Sprint 迭代**:
- `shouldStartNextSprint()`: 判断是否需要下一轮 Sprint
  - 条件 1: Sprint.UserFeedback 不为空 → 必须迭代
  - 条件 2: RetroResult 有 QualityIssues + NextSprintHints → 自动迭代
  - 限制: sprint.Number+1 < mission.MaxSprints (默认 4)
- `startNextSprint()`: 创建新 Sprint
  - 构建 Sprint 目标: 原始任务 + 用户反馈 + Retro 改进建议 + 下轮提示
  - `generateSprintPlan()`: LLM 生成改进计划 (聚焦修复，不重复上轮工作)
  - 分配步骤 → 创建 Sprint/Step 记录 → WebSocket 推送 → advanceMission 调度
- 完整循环: Sprint N 完成 → CI Gate → Retro → 检查是否需要 Sprint N+1 → 自动开始

**自动 Preview (CI Gate Stage 5)**:
- `launchPreview()`: Sprint CI Gate 通过后自动启动预览服务器
- 项目类型检测: package.json/main.go/requirements.txt/index.html
- 端口分配: 9000-9999 随机, Preview URL 存入 Sprint + Mission

**编译状态**: go build ✅ | go vet ✅ | tsc --noEmit ✅

### 20.9.6 LLM 辅助合并冲突解决 ✅

**实现时间**: 2026-03-17

**改动文件**:
- `squad/git.go`: ConflictResolver 回调类型 + resolveConflicts 方法 + fileExists/fileContains
- `squad/engine.go`: llmResolveConflict 方法 + NewEngine 注册 resolver

**机制**:
- `ConflictResolver` callback: `func(filePath, ours, theirs, base string) (string, error)`
- `resolveConflicts()`: 列出冲突文件 → 提取 HEAD/MERGE_HEAD/base 版本 → 逐文件调用 LLM → 写入解决方案 → git add → commit
- `llmResolveConflict()`: LLM prompt 包含文件名 + BASE/OURS/THEIRS 三方内容，要求智能合并
- 合并策略: 保留两边有用改动 > 选择更完整版本 > fallback: checkout --theirs
- 文件截断: 每方最多 2000 字符防止 token 溢出

**编译状态**: go build ✅ | go vet ✅
