# Claw 🦞 部署指南

> 服务器 A — starclaw.me (开源官网 + 应用)
>
> 域名: starclaw.me / app.starclaw.me / api.starclaw.me

## 一、服务器要求

| 项目 | 最低配置 | 推荐配置 |
|------|---------|---------|
| CPU | 2 核 | 4 核 |
| 内存 | 4 GB | 8 GB |
| 硬盘 | 40 GB SSD | 100 GB SSD |
| 系统 | Ubuntu 22.04 / CentOS 8+ | Ubuntu 22.04 LTS |

> 后端镜像包含 Chromium、FFmpeg、Python、Node.js，构建约 1.5GB。

## 二、安装 Docker

```bash
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER
newgrp docker
```

**⚠️ 国内服务器必须配置 Docker 镜像加速**（否则无法拉取基础镜像）：
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

> 验证加速是否生效：`docker info | grep -A5 "Registry Mirrors"`

## 三、部署

### 1. 获取代码

```bash
git clone https://github.com/yinhe/starclaw.git
cd starclaw
```

### 2. 配置环境变量

```bash
cp .env.example .env
nano .env
```

**必须修改：**
```bash
DB_PASSWORD=<强密码>          # MySQL root 密码
JWT_SECRET=<随机字符串>       # openssl rand -hex 32
WEB_PORT=3080                 # 前端端口（nginx 代理到此端口）
```

### 3. 启动

```bash
# 创建数据目录
mkdir -p data/{merged_videos,thumbnails,music,images,workspaces,identity}

# 构建 & 启动（首次约 5-10 分钟）
make up

# ⚠️ 国内服务器请使用加速配置：
make up-cn
```

### 4. 验证

```bash
docker compose ps                        # 所有服务应显示 Up
curl http://localhost/v1/health           # 应返回 OK
```

浏览器访问 `http://你的服务器IP` 即可使用。

## 四、更新到最新版本

### 方式一：开发模式（git pull）

服务器直接从 GitHub 拉取最新代码并重建，**只重建 API 和 Web，不碰数据库**：

```bash
cd /opt/starclaw
make update         # git pull → build api → build web → restart → verify

# 国内服务器：
make update-cn
```

`make update` 会自动执行：
1. `git pull origin main`
2. 构建 API 镜像（增量构建，有缓存很快）
3. 构建 Web 镜像
4. 仅重启 API 和 Web 容器（`--no-deps`，不碰 MySQL/Redis）
5. 等 3 秒后自动验证 API 和 Web 是否正常

也可以只重建单个服务：
```bash
make rebuild-api    # 仅重建 API
make rebuild-web    # 仅重建 Web
make verify         # 检查服务状态
```

### 方式二：Release 更新（一键更新）

在 Web 界面 **设置 → 系统 → 检查更新**，或通过 Molt 蜕皮自动更新。

> **数据安全保证：**
> - MySQL 数据持久化在 `data/mysql/`
> - Redis 数据持久化在 `data/redis/`
> - Node 身份持久化在 `data/identity/`
> - 所有用户文件在 `data/` 子目录中
> - `make update` **绝不触碰** MySQL 和 Redis 容器

## 五、配置 AI 模型

进入 Web 界面 → 设置 → 模型管理 → 添加你的 API Key：

| Provider | 获取 Key |
|----------|---------|
| 通义千问 Qwen | https://dashscope.console.aliyun.com |
| OpenAI | https://platform.openai.com/api-keys |
| DeepSeek | https://platform.deepseek.com |
| Ollama（本地） | 无需 Key，填写 Ollama 地址即可 |

## 六、加入虫群（可选）

默认情况下你的小龙虾独立运行。如果想加入虫群网络：

编辑 `api/configs/config.yaml`：
```yaml
server:
  node_role: claw
  queen_url: "https://swarm.starclaw.net"  # 连接到 Queen
  auto_update: true                       # 自动接收蜕皮更新
```

重启后你的 Claw 会自动注册到虫群，获得：
- 🔄 自动版本更新（Molt 蜕皮）
- 📦 共享 Agent/Workflow 模板（Creep 菌毯）
- 💰 赏金任务发布能力（Bounty）

### 关联 Queen 账号

加入虫群后，可在 **设置 → Queen 账号关联** 中使用 Queen 平台账号登录，
将本 Claw 节点绑定到你的 Queen 用户。关联后可使用：
- 赏金结算（Bounty 冻结/释放/结算）
- 社区互动（论坛、竞技场）
- Agent 市场
- 跨 Claw 身份统一

### Feral 模式（失控模式）

当 Claw 连续 3 次心跳失败时，自动进入 **Feral 模式**：
- 所有本地 AI 功能照常运行
- 设置页面显示琥珀色失控警告
- 恢复连接后自动退出 Feral 模式并记录日志

## 七、升级为领主 Overlord（可选）

如果你需要管理多个 Claw 节点：

```yaml
server:
  node_role: overlord    # 启用领主管理模式（需购买 overlord/ 软件包）
```

Overlord 节点可以：
- 管理下属 Claw 节点
- 企业内部负载均衡
- 通过 Nydus 隧道实现 Claw 间直连

## 八、域名 + HTTPS

```bash
# 安装 certbot
sudo apt install -y certbot nginx

# 申请证书
sudo certbot certonly --standalone -d your-domain.com

# 配置 Nginx 反向代理
sudo nano /etc/nginx/sites-available/starclaw
```

```nginx
server {
    listen 80;
    server_name your-domain.com;
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    server_name your-domain.com;

    ssl_certificate /etc/letsencrypt/live/your-domain.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/your-domain.com/privkey.pem;
    client_max_body_size 100M;

    location / {
        proxy_pass http://127.0.0.1:8081;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    location /api/ {
        proxy_pass http://127.0.0.1:8081;
        proxy_buffering off;
        proxy_cache off;
        proxy_http_version 1.1;
        proxy_set_header Connection '';
        proxy_read_timeout 600s;
    }
}
```

```bash
sudo ln -s /etc/nginx/sites-available/starclaw /etc/nginx/sites-enabled/
sudo nginx -t && sudo systemctl reload nginx
```

## 九、日常运维

### 安装 CLI 别名（推荐）

项目提供 `claw` CLI 工具，是 `make` 的简写别名，支持所有命令 + 额外快捷方式。

**Linux / macOS:**

```bash
# 创建符号链接（二选一或都建）
sudo ln -sf $(pwd)/scripts/claw /usr/local/bin/claw
sudo ln -sf $(pwd)/scripts/claw /usr/local/bin/starclaw

# 之后可在任意目录使用
claw up
claw logs
starclaw update
```

**Windows:**

将 `scripts\` 目录加入系统 PATH，或复制 `scripts\claw.bat` 到 PATH 中的目录：

```powershell
# 复制到用户目录
copy scripts\claw.bat %USERPROFILE%\claw.bat

# 然后使用
claw up
claw logs
```

### 命令速查

以下命令通过 `claw`、`starclaw` 或 `make` 均可调用：

```bash
claw help              # 查看所有可用命令
```

### 生命周期

```bash
claw up                # 构建并启动所有服务
claw up-cn             # 国内镜像加速构建
claw start             # 启动已有容器（不重新构建）
claw stop              # 停止所有容器（保留数据）
claw restart           # 重启所有容器
claw down              # 停止并移除容器和网络（保留数据）
claw destroy           # ⚠️ 停止并删除 MySQL/Redis/Sandbox 数据
```

### 更新

```bash
claw update            # git pull + 重新构建（别名: claw pull）
claw update-cn         # 国内镜像加速更新
claw rebuild-api       # 仅重建 API 服务（别名: claw ra）
claw rebuild-web       # 仅重建 Web 前端（别名: claw rw）
```

### 日志 & 状态

```bash
claw logs              # 查看所有服务日志
claw la                # 查看 API 日志（别名: claw logs-api）
claw lw                # 查看 Web 日志（别名: claw logs-web）
claw ps                # 查看容器状态（别名: claw status）
claw stats             # 查看 CPU/内存占用
claw health            # 检查 API 健康状态（别名: claw ping）
claw version           # 查看版本信息（别名: claw v）
```

### 备份 & 恢复

```bash
claw backup            # 备份数据库 + data 目录到 backups/
claw restore-db backups/db_20260307.sql  # 恢复数据库
```

### Shell 访问

```bash
claw shell             # 进入 API 容器（别名: claw sh）
claw mysql             # 打开 MySQL CLI
claw redis             # 打开 Redis CLI
```

### 清理

```bash
claw prune             # 清理未使用的 Docker 镜像（别名: claw clean）
```

> **提示:** 所有命令仍然兼容 `make` 方式调用，如 `make up`、`make logs` 等。

## 十、常见问题

**Q: 构建太慢？**
国内服务器配置 Docker 镜像加速，后续构建使用缓存会很快。

**Q: 内存不足？**
```bash
sudo fallocate -l 4G /swapfile
sudo chmod 600 /swapfile && sudo mkswap /swapfile && sudo swapon /swapfile
```

**Q: 断开虫群后还能用吗？**
可以。进入 Feral（失控）模式后所有 AI 功能正常，只是失去自动更新和共享知识。重连后自动恢复。

## 十一、团队代理上线流程（研发 + DevOps）

当你在「Agents」页面使用“团队代理（研发DevOps团队）”时，推荐按以下 SOP 执行上线：

1. `deploy_web`：触发部署（preview/production）
2. `bind_domain`：绑定/更新域名 DNS（Cloudflare）
3. `verify_online`：检查线上可达性 + 关键词验收

### 审批闸门（强烈建议）

- 生产部署前先人工确认
- DNS 变更前再人工确认

### 团队代理测试提示词（可直接复制）

```text
请研发DevOps团队执行上线流程：
1) deploy_web 生产部署
2) bind_domain 绑定 app.example.com
3) verify_online 验证 https://app.example.com

要求：生产发布前审批一次，DNS 修改前再审批一次；失败时给回滚建议。
```

### bind_domain 示例参数（Cloudflare）

```json
{
  "action": "upsert",
  "provider": "cloudflare",
  "api_token": "<CLOUDFLARE_API_TOKEN>",
  "zone_id": "<ZONE_ID>",
  "record_type": "CNAME",
  "record_name": "app.example.com",
  "record_value": "cname.vercel-dns.com",
  "proxied": "false",
  "ttl": "120"
}
```

> 安全提示：`api_token` 只应通过运行时参数传入，不要写入仓库文件或提交到 Git。
