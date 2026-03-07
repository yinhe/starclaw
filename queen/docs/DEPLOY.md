# StarClaw 服务器部署指南

## 一、服务器要求

| 项目 | 最低配置 | 推荐配置 |
|------|---------|---------|
| CPU | 2 核 | 4 核 |
| 内存 | 4 GB | 8 GB |
| 硬盘 | 40 GB SSD | 100 GB SSD |
| 系统 | Ubuntu 22.04 / CentOS 8+ | Ubuntu 22.04 LTS |
| 带宽 | 3 Mbps | 5 Mbps+ |

> **说明**: 后端 Dockerfile 包含 Chromium、FFmpeg、Python、Node.js，构建镜像约 1.5GB，请确保硬盘充足。

## 二、购买云服务器

### 阿里云 ECS（推荐国内）
1. 访问 https://ecs.console.aliyun.com
2. 选择 **按量付费** 或 **包年包月**
3. 推荐配置: **ecs.c6.xlarge** (4核8G) 或 **ecs.c6.large** (2核4G)
4. 系统镜像选择 **Ubuntu 22.04 LTS 64位**
5. 安全组放行端口: **22 (SSH)、80 (HTTP)、443 (HTTPS)**

### 腾讯云 CVM
1. 访问 https://console.cloud.tencent.com/cvm
2. 推荐配置: **S5.LARGE8** (4核8G)
3. 系统镜像选择 **Ubuntu 22.04 LTS**
4. 安全组放行: 22、80、443

### AWS EC2
1. 推荐实例: **t3.large** (2核8G)
2. AMI: Ubuntu 22.04 LTS
3. Security Group: 22、80、443

## 三、服务器初始化

SSH 连接到服务器后执行以下步骤：

### 1. 安装 Docker

```bash
# Ubuntu 22.04 一键安装
curl -fsSL https://get.docker.com | sh

# 将当前用户加入 docker 组（免 sudo）
sudo usermod -aG docker $USER
newgrp docker

# 验证
docker --version
docker compose version
```

如果在国内服务器，建议配置 Docker 镜像加速：

```bash
sudo mkdir -p /etc/docker
sudo tee /etc/docker/daemon.json <<-'EOF'
{
  "registry-mirrors": [
    "https://mirror.ccs.tencentyun.com",
    "https://docker.mirrors.ustc.edu.cn"
  ]
}
EOF
sudo systemctl daemon-reload
sudo systemctl restart docker
```

### 2. 安装 Git

```bash
sudo apt update && sudo apt install -y git
```

## 四、部署 StarClaw

### 1. 上传代码到服务器

**方式 A: Git 克隆（推荐）**

如果代码在 Git 仓库中：
```bash
cd ~
git clone https://your-git-repo-url/starclaw.git
cd starclaw
```

**方式 B: 本地打包上传**

在本地 Windows 电脑上（PowerShell）：
```powershell
# 在项目根目录打包（排除 node_modules 和其他不需要的文件）
cd E:\starclaw
tar --exclude='node_modules' --exclude='.git' --exclude='data' -czf starclaw.tar.gz .

# 上传到服务器（替换为你的服务器 IP）
scp starclaw.tar.gz root@YOUR_SERVER_IP:~/
```

在服务器上解压：
```bash
mkdir -p ~/starclaw && cd ~/starclaw
tar -xzf ~/starclaw.tar.gz
```

### 2. 配置环境变量

```bash
cd ~/starclaw

# 复制生产环境配置
cp .env.production .env

# 编辑配置（必须修改！）
nano .env
```

**必须修改的配置：**
```
JWT_SECRET=随机字符串至少32位     # 例: openssl rand -hex 32
DB_ROOT_PASSWORD=强密码           # MySQL root 密码
REDIS_PASSWORD=强密码             # Redis 密码
```

**可选配置：**
```
OPENAI_API_KEY=sk-xxx             # 如果需要 RAG 功能
HTTP_PORT=80                       # 前端端口
```

> 快速生成随机密码: `openssl rand -hex 16`

### 3. 一键部署

```bash
# 创建数据目录 + 构建 + 启动
bash deploy.sh
```

或手动执行：
```bash
# 创建数据目录
mkdir -p data/{merged_videos,thumbnails,music,images,workspaces}

# 构建镜像（首次约 5-10 分钟）
docker compose -f docker-compose.prod.yml build

# 启动所有服务
docker compose -f docker-compose.prod.yml up -d
```

### 4. 验证部署

```bash
# 查看容器状态
docker compose -f docker-compose.prod.yml ps

# 查看后端日志
docker logs starclaw-api --tail 20

# 测试 API
curl http://localhost:80/v1/health
```

浏览器访问 `http://你的服务器IP` 即可看到 StarClaw 界面。

## 五、域名 + HTTPS（可选但推荐）

### 1. 域名解析

在域名服务商处添加 A 记录：
- 主机记录: `@` 或 `starclaw`
- 记录值: 你的服务器 IP

### 2. 安装 Certbot + 自动 SSL

```bash
# 安装 certbot
sudo apt install -y certbot

# 先停止前端（释放 80 端口给 certbot）
docker compose -f docker-compose.prod.yml stop web

# 申请证书（替换为你的域名和邮箱）
sudo certbot certonly --standalone -d your-domain.com -m your@email.com --agree-tos

# 重启前端
docker compose -f docker-compose.prod.yml start web
```

### 3. 使用 Nginx 反向代理（HTTPS 方式）

如果需要 HTTPS，建议在服务器上安装独立的 Nginx 作为反向代理：

```bash
sudo apt install -y nginx

# 修改 docker-compose.prod.yml 中 frontend 端口为 8081:80（避免冲突）
# 然后配置 Nginx:
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
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_buffering off;
        proxy_cache off;
        proxy_set_header Connection '';
        proxy_http_version 1.1;
        chunked_transfer_encoding off;
        proxy_read_timeout 600s;
        proxy_send_timeout 600s;
    }
}
```

```bash
sudo ln -s /etc/nginx/sites-available/starclaw /etc/nginx/sites-enabled/
sudo nginx -t && sudo systemctl reload nginx
```

## 六、日常运维

### 更新部署

```bash
cd ~/starclaw

# 拉取最新代码（Git 方式）
git pull

# 重新构建并启动
docker compose -f docker-compose.prod.yml build
docker compose -f docker-compose.prod.yml up -d
```

### 查看日志

```bash
# 后端日志
docker logs -f starclaw-api --tail 100

# 全部日志
docker compose -f docker-compose.prod.yml logs -f

# MySQL 日志
docker logs starclaw-mysql --tail 50
```

### 数据备份

```bash
# 备份 MySQL
docker exec starclaw-mysql mysqldump -uroot -p${DB_ROOT_PASSWORD} starclaw > backup_$(date +%Y%m%d).sql

# 备份整个 data 目录
tar -czf starclaw-data-$(date +%Y%m%d).tar.gz data/

# 恢复 MySQL
docker exec -i starclaw-mysql mysql -uroot -p${DB_ROOT_PASSWORD} starclaw < backup_20260306.sql
```

### 重启服务

```bash
# 重启所有
docker compose -f docker-compose.prod.yml restart

# 重启单个
docker compose -f docker-compose.prod.yml restart api
```

### 清理磁盘

```bash
# 清理无用的 Docker 镜像
docker system prune -f

# 查看磁盘使用
docker system df
```

## 七、常见问题

### Q: 构建时间很长？
A: 首次构建需要下载基础镜像和依赖，国内服务器建议配置 Docker 镜像加速。后续构建会使用缓存，几十秒即可完成。

### Q: 访问不了？
A: 检查安全组/防火墙是否放行了 80 端口：
```bash
# 查看端口监听
ss -tlnp | grep 80

# Ubuntu UFW 防火墙
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
```

### Q: 内存不足？
A: 构建 backend 镜像时需要较多内存。如果服务器内存不足，可以添加 swap：
```bash
sudo fallocate -l 4G /swapfile
sudo chmod 600 /swapfile
sudo mkswap /swapfile
sudo swapon /swapfile
echo '/swapfile swap swap defaults 0 0' | sudo tee -a /etc/fstab
```

### Q: 如何查看服务是否正常？
```bash
docker compose -f docker-compose.prod.yml ps
# 所有服务应该显示 Up (healthy)
```
