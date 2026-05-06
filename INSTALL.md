# StarClaw 🦞 Docker 安装指南

适用于已经 `git clone` 下来的服务器。

---

## 1. 系统要求

| 项目 | 最低 | 推荐 |
|------|------|------|
| CPU | 2 核 | 4 核 |
| 内存 | 4 GB | 8 GB |
| 硬盘 | 40 GB SSD | 100 GB SSD |
| 系统 | Ubuntu 22.04 / CentOS 8+ | Ubuntu 22.04 LTS |

> 后端镜像包含 Chromium / FFmpeg / Python / Node.js，构建约 1.5 GB，首次构建 5–10 分钟。

---

## 2. 安装 Docker

```bash
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER
newgrp docker
```

### ⚠️ 国内服务器必须配置镜像加速

```bash
sudo mkdir -p /etc/docker
sudo tee /etc/docker/daemon.json <<-'EOF'
{
  "registry-mirrors": [
    "https://docker.1ms.run",
    "https://docker.xuanyuan.me"
  ]
}
EOF
sudo systemctl daemon-reload && sudo systemctl restart docker
```

验证加速生效：

```bash
docker info | grep -A5 "Registry Mirrors"
```

---

## 3. 启动服务

进入 `git clone` 下来的目录：

```bash
cd starclaw

make up        # 海外服务器
# 或
make up-cn     # 国内服务器（推荐）
```

`make up` 会自动：

- 从 `.env.example` 生成 `.env`
- 创建 `data/` 子目录
- 构建并启动 `mysql` + `redis` + `api` (`:8080`) + `web` (`:80`)

首次构建约 5–10 分钟。

---

## 4. 验证

```bash
docker compose ps                       # 所有服务应为 Up
curl http://localhost:8080/v1/health    # 应返回 OK
```

浏览器访问 `http://<服务器IP>` 即可。

---

## 5. 公网部署（必改 `.env`）

```bash
nano .env
```

```bash
DB_PASSWORD=<强密码>
JWT_SECRET=<随机字符串，可用 openssl rand -hex 32 生成>
WEB_PORT=80
```

改完重启：

```bash
make down && make up
```

---

## 6. 备份身份（强烈建议）

```bash
make export-key
```

会输出 24 个 BIP-39 助记词，**抄到纸上保管**。  
服务器毁了也能凭这 24 词完整恢复 Claw 节点身份和钱包。

---

## 7. 配置 AI Key

浏览器进 Web → **设置 → 模型管理** → 填 API Key：

| Provider | 获取地址 |
|----------|---------|
| 通义千问 Qwen | https://dashscope.console.aliyun.com |
| OpenAI | https://platform.openai.com/api-keys |
| DeepSeek | https://platform.deepseek.com |
| Ollama（本地） | 无需 Key，填 Ollama 地址即可 |

---

## 8. 常用运维命令

### 生命周期

```bash
make up         # 构建并启动
make start      # 启动已有容器
make stop       # 停止（保留数据）
make restart    # 重启
make down       # 停止并移除容器（保留数据）
make destroy    # ⚠️ 删除 MySQL/Redis 数据
```

### 更新

```bash
make update      # git pull + 仅重建 api/web，不动数据库
make update-cn   # 国内镜像加速更新
make rebuild-api # 仅重建 API
make rebuild-web # 仅重建 Web
```

### 日志 / 状态

```bash
make logs        # 全部日志
make la          # API 日志
make lw          # Web 日志
make ps          # 容器状态
make health      # API 健康检查
make version     # 版本信息
```

### 备份

```bash
make backup                              # 备份数据库 + data/
make restore-db backups/db_20260307.sql  # 恢复数据库
```

### Shell 访问

```bash
make shell       # 进入 API 容器
make mysql       # MySQL CLI
make redis       # Redis CLI
```

---

## 9. 安装 CLI 别名（可选，推荐）

```bash
sudo ln -sf $(pwd)/scripts/claw /usr/local/bin/claw
```

之后任意目录都可用：

```bash
claw up
claw logs
claw update
claw health
```

---

## 10. 常见问题

**Q: 构建太慢？**  
国内服务器一定要配 Docker 镜像加速，后续构建用缓存会很快。

**Q: 内存不足？**

```bash
sudo fallocate -l 4G /swapfile
sudo chmod 600 /swapfile && sudo mkswap /swapfile && sudo swapon /swapfile
```

**Q: 端口冲突（3306 / 6379 被占用）？**  
不会冲突 —— MySQL 和 Redis 仅在 Docker 内部网络通信，不暴露到宿主机。

**Q: 数据存在哪？**  
全部在 `./data/` 下：`mysql/`、`redis/`、`identity/`、`images/`、`videos/`、`uploads/` 等。`make update` 不会触碰数据库容器。

---

## 备注

- 域名 + HTTPS 配置参考 `docs/DEPLOY.md` 第九节
- 加入虫群 / 关联 Queen 账号参考 `docs/DEPLOY.md` 第七节
- 企业 Overlord 模式参考 `docs/DEPLOY.md` 第八节
