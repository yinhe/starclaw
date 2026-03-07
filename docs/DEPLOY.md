# StarClaw 🦞 部署指南

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
```
JWT_SECRET=<openssl rand -hex 32>
DB_ROOT_PASSWORD=<强密码>
REDIS_PASSWORD=<强密码>
```

### 3. 启动

```bash
# 创建数据目录
mkdir -p data/{merged_videos,thumbnails,music,images,workspaces}

# 构建 & 启动（首次约 5-10 分钟）
docker compose up -d --build

# ⚠️ 国内服务器请使用加速配置：
# docker compose -f docker-compose.yml -f docker-compose.cn.yml up -d --build
```

### 4. 验证

```bash
docker compose ps                        # 所有服务应显示 Up
curl http://localhost/v1/health           # 应返回 OK
```

浏览器访问 `http://你的服务器IP` 即可使用。

## 四、配置 AI 模型

进入 Web 界面 → 设置 → 模型管理 → 添加你的 API Key：

| Provider | 获取 Key |
|----------|---------|
| 通义千问 Qwen | https://dashscope.console.aliyun.com |
| OpenAI | https://platform.openai.com/api-keys |
| DeepSeek | https://platform.deepseek.com |
| Ollama（本地） | 无需 Key，填写 Ollama 地址即可 |

## 五、加入虫群（可选）

默认情况下你的小龙虾独立运行。如果想加入虫群网络：

编辑 `api/configs/config.yaml`：
```yaml
server:
  node_role: claw
  queen_url: "https://api.starclaw.me"   # 连接到 Queen
  auto_update: true                       # 自动接收蜕皮更新
```

重启后你的 Claw 会自动注册到虫群，获得：
- 🔄 自动版本更新（Molt 蜕皮）
- 📦 共享 Agent/Workflow 模板（Creep 菌毯）
- 💰 赏金任务发布能力（Bounty）

## 六、升级为领主 Overlord（可选）

如果你需要管理多个 Claw 节点：

```yaml
server:
  node_role: overlord    # 启用领主管理模式（需购买 overlord/ 软件包）
```

Overlord 节点可以：
- 管理下属 Claw 节点
- 企业内部负载均衡
- 通过 Nydus 隧道实现 Claw 间直连

## 七、域名 + HTTPS

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

## 八、日常运维

```bash
# 更新
git pull && docker compose up -d --build

# 查看日志
docker logs -f starclaw-api --tail 100

# 备份数据库
docker exec starclaw-mysql mysqldump -uroot -p$DB_ROOT_PASSWORD starclaw > backup.sql

# 备份数据
tar -czf data-backup.tar.gz data/

# 重启
docker compose restart
```

## 九、常见问题

**Q: 构建太慢？**
国内服务器配置 Docker 镜像加速，后续构建使用缓存会很快。

**Q: 内存不足？**
```bash
sudo fallocate -l 4G /swapfile
sudo chmod 600 /swapfile && sudo mkswap /swapfile && sudo swapon /swapfile
```

**Q: 断开虫群后还能用吗？**
可以。进入 Feral（失控）模式后所有 AI 功能正常，只是失去自动更新和共享知识。重连后自动恢复。
