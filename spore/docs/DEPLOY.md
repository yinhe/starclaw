# Spore 部署指南 — 开源用户

> 让 StarClaw Claw 像孢子一样，在任何设备上落地即萌发

## 概述

Spore 是 Docker 的轻量替代方案，专为 Claw 在异构设备上的部署而设计。开源用户可以选择 **Docker** 或 **Spore** 两种方式部署 Claw。

| 维度 | Docker | Spore |
|------|--------|-------|
| 运行时大小 | ~200MB | < 3MB |
| 网络要求 | 必须访问 Registry | P2P + 离线安装 |
| 支持平台 | Linux (native) | Linux / macOS / Windows / Android / OpenWrt |
| 启动速度 | 秒级 | 毫秒级 |
| 更新方式 | 拉全新 Image | Delta patch（差量更新，~2MB） |

---

## 方式一：Spore 部署（推荐）

### 1. 安装 Spore 运行时

**Linux / macOS（一行命令）：**

```bash
curl -fsSL https://spore.starclaw.me/install.sh | sh
```

**Windows（PowerShell）：**

```powershell
irm https://spore.starclaw.me/install.ps1 | iex
```

**从源码构建（需要 Go 1.22+）：**

```bash
go install github.com/yinhe/starclaw-spore/cmd/spore@latest
go install github.com/yinhe/starclaw-spore/cmd/hatchery@latest
```

安装完成后验证：

```bash
spore version
```

### 2. 安装 Claw

**从 P2P 网络拉取（推荐）：**

```bash
spore pull claw:latest
```

**从 .spore 包安装（离线环境）：**

```bash
# 下载对应平台的 .spore 包
wget https://github.com/yinhe/starclaw/releases/latest/download/claw-linux-amd64.spore

# 安装
spore install ./claw-linux-amd64.spore
```

**从 Hatchery 仓库安装：**

```bash
spore install https://hatchery.starclaw.me/claw/latest
```

### 3. 配置

```bash
# 查看默认配置
spore info claw

# 编辑配置文件
nano ~/.spore/installed/claw/current/config/default.yaml
```

**最小配置 (default.yaml)：**

```yaml
server:
  host: 0.0.0.0
  port: 8080

database:
  # SQLite（默认，无需额外安装）
  driver: sqlite
  path: ./data/claw.db
  # MySQL（可选）
  # driver: mysql
  # dsn: "user:pass@tcp(127.0.0.1:3306)/claw?charset=utf8mb4"

# AI 模型配置（至少配置一个）
models:
  - provider: openai
    api_key: "sk-xxx"           # 你的 OpenAI API Key
    # base_url: ""              # 自定义 API 端点（可选）
  # - provider: deepseek
  #   api_key: "sk-xxx"
  # - provider: qwen
  #   api_key: "sk-xxx"
  # - provider: ollama
  #   base_url: "http://localhost:11434"
```

### 4. 启动

```bash
# 后台启动（注册为系统服务）
spore start claw

# 前台运行（调试用）
spore run claw

# 查看状态
spore status

# 查看日志
spore logs claw --follow
```

启动后访问：
- **Web 界面**: http://localhost:8080
- **API 端点**: http://localhost:8080/v1

### 5. 更新

```bash
# 自动更新到最新版（差量更新，通常只需 ~2MB）
spore update claw

# 指定版本
spore update claw --version 1.3.0

# 回滚到上一版本
spore rollback claw
```

### 6. 管理

```bash
spore status          # 查看所有已安装服务状态
spore stop claw       # 停止
spore restart claw    # 重启
spore list            # 列出已安装的 Spore 包
spore logs claw       # 查看日志
```

---

## 方式二：Docker 部署

### 1. 前置条件

- Docker 20.10+
- Docker Compose v2+

### 2. 克隆仓库

```bash
git clone https://github.com/yinhe/starclaw.git
cd starclaw/claw
```

### 3. 配置环境变量

```bash
cp .env.example .env
nano .env
```

**必填项：**

```env
# 数据库
MYSQL_ROOT_PASSWORD=your_secure_password
MYSQL_DATABASE=starclaw

# 至少配置一个 AI Provider
OPENAI_API_KEY=sk-xxx
# DEEPSEEK_API_KEY=sk-xxx
# DASHSCOPE_API_KEY=sk-xxx
```

### 4. 启动

```bash
# 生产模式
docker compose -f docker-compose.prod.yml up -d

# 开发模式
docker compose up -d
```

### 5. 验证

```bash
# 检查容器状态
docker compose ps

# API 健康检查
curl http://localhost:8080/health

# 查看日志
docker compose logs -f api
```

启动后访问：
- **Web 界面**: http://localhost:8081
- **API 端点**: http://localhost:8080/v1

### 6. 更新

```bash
git pull
docker compose -f docker-compose.prod.yml up -d --build
```

---

## 方式三：源码编译

### 1. 前置条件

- Go 1.22+
- Node.js 20+
- MySQL 8.0+ 或 SQLite

### 2. 编译后端

```bash
cd claw/api
go build -o claw-api ./cmd/server
```

### 3. 编译前端

```bash
cd claw/web
npm install --legacy-peer-deps
npm run build
```

### 4. 运行

```bash
# 复制配置
cp configs/config.example.yaml configs/config.yaml
# 编辑配置
nano configs/config.yaml

# 启动
./claw-api serve --config configs/config.yaml
```

---

## 方式四：使用 Hatchery 自构建 Spore 包

适用于需要定制化构建或内网分发的场景。

### 1. 安装 Hatchery

```bash
go install github.com/yinhe/starclaw-spore/cmd/hatchery@latest
```

### 2. 构建 Spore 包

```bash
cd starclaw/claw

# 构建当前平台
hatchery build

# 构建指定平台
hatchery build --platform linux/arm64

# 构建所有支持平台
hatchery build --all --output dist/
```

### 3. 内网分发

```bash
# 启动本地 Hatchery 仓库
hatchery serve --port 7777 --dir ./dist

# 其他设备从内网仓库安装
spore install http://192.168.1.100:7777/claw/latest
```

---

## 支持平台

| OS | 架构 | 状态 | 适用场景 |
|----|------|------|---------|
| Linux | amd64 | ✅ | 云服务器、PC |
| Linux | arm64 | ✅ | 树莓派 4/5、Jetson |
| Linux | arm/v7 | ✅ | 树莓派 3、旧 ARM |
| Linux | mips64le | 🔄 | 路由器 |
| Linux | riscv64 | 🔄 | 新兴平台 |
| macOS | amd64 | ✅ | Intel Mac |
| macOS | arm64 | ✅ | Apple Silicon |
| Windows | amd64 | ✅ | Windows 桌面 |
| Android | arm64 | 🔄 | Termux 环境 |

✅ = 已支持 &nbsp; 🔄 = 开发中

---

## 数据目录

Spore 安装后的数据目录结构：

```
~/.spore/
├── cache/                    # 下载缓存 + delta patch 缓存
├── installed/
│   └── claw/
│       ├── current -> v1.x/  # 当前版本（符号链接）
│       ├── v1.x/             # 版本目录
│       │   ├── bin/claw
│       │   ├── config/
│       │   └── manifest.json
│       ├── data/             # 持久数据（跨版本保留）
│       │   ├── db/
│       │   └── uploads/
│       └── logs/
├── registry/                 # 本地索引
└── config.yaml               # Spore 全局配置
```

---

## 常见问题

### Q: Spore 和 Docker 能同时使用吗？

可以。两者完全独立，互不干扰。Docker 部署使用容器隔离，Spore 直接运行原生二进制。

### Q: 如何从 Docker 迁移到 Spore？

1. 导出 Docker 中的数据库和上传文件
2. 安装 Spore 版本的 Claw
3. 将数据文件复制到 `~/.spore/installed/claw/current/data/`
4. 启动 Spore 版本

### Q: 如何在没有网络的环境部署？

1. 在有网络的机器上下载 `.spore` 包
2. 通过 USB / 局域网传输到目标设备
3. `spore install ./claw-xxx.spore`

### Q: 如何配置自动更新？

```bash
# 启用自动更新（每天检查一次）
spore config set auto_update true
spore config set update_channel stable
```

### Q: 如何加入 P2P 分发网络？

安装 Spore 后自动加入 Nydus P2P 网络。你的节点会自动为附近的节点提供 Spore 包分发服务，无需额外配置。

---

## 获取帮助

- GitHub Issues: https://github.com/yinhe/starclaw/issues
- 文档: https://starclaw.me/docs
- 设计文档: [SPORE_PLAN.md](./SPORE_PLAN.md)
