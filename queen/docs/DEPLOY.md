# Queen 👑 部署指南

> 服务器 C — starclaw.net (中央控制)

## 一、架构

```
starclaw.net             → Queen Web Dashboard (:8086)
api.starclaw.net         → Queen API (:8085)
swarm.starclaw.net       → Swarm 节点管理 (:8090)
core.starclaw.net        → Core API (:8091)
bounty.starclaw.net      → 赏金市场 (:8092)
forum.starclaw.net       → 社区 (:8093)
arena.starclaw.net       → 竞技场 (:8094)
:9090 (内网)              → Prometheus
:3000 (内网)              → Grafana
```

## 二、服务器要求

| 项目 | 最低配置 | 推荐配置 |
|------|---------|---------|
| CPU | 4 核 | 8 核 |
| 内存 | 8 GB | 16 GB |
| 硬盘 | 80 GB SSD | 200 GB SSD |
| 系统 | Ubuntu 22.04 LTS | Ubuntu 22.04 LTS |

> Queen 运行 7 个 Go 服务 + MySQL + Prometheus + Grafana，建议 8G 以上。

## 三、安装依赖

```bash
# Docker
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER && newgrp docker

# Nginx + Certbot
sudo apt install -y nginx certbot python3-certbot-nginx git

# 国内 Docker 镜像加速
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

## 四、部署

### 1. 获取代码

```bash
cd /opt
git clone <仓库地址> starclaw
cd starclaw/queen
```

### 2. 配置环境变量

```bash
cp ../.env.production .env
nano .env
```

**必须修改：**
```
JWT_SECRET=<openssl rand -hex 32>
QUEEN_DB_PASSWORD=<强密码>
GRAFANA_PASSWORD=<强密码>
```

### 3. 构建 & 启动

```bash
# 生产环境（所有服务绑定 127.0.0.1，由 Nginx 代理）
docker compose -f docker-compose.prod.yml build
docker compose -f docker-compose.prod.yml up -d

# 查看状态（应有 9 个容器：mysql + 5 Go 服务 + web + prometheus + grafana）
docker compose -f docker-compose.prod.yml ps
```

### 4. 验证

```bash
# Queen API
curl http://127.0.0.1:8085/health

# Swarm
curl http://127.0.0.1:8090/health

# Grafana
curl http://127.0.0.1:3000/api/health
```

## 五、域名 + HTTPS

### 1. DNS 解析

在域名商处添加 A 记录，全部指向 Queen 服务器 IP：
```
starclaw.net          → <IP>
api.starclaw.net      → <IP>
swarm.starclaw.net    → <IP>
core.starclaw.net     → <IP>
bounty.starclaw.net   → <IP>
forum.starclaw.net    → <IP>
arena.starclaw.net    → <IP>
```

### 2. 申请 SSL 证书

```bash
# 通配符证书（需要 DNS 验证）
sudo certbot certonly --manual --preferred-challenges dns \
  -d starclaw.net -d '*.starclaw.net'

# 或逐个申请
sudo certbot certonly --nginx \
  -d starclaw.net \
  -d api.starclaw.net \
  -d swarm.starclaw.net \
  -d core.starclaw.net \
  -d bounty.starclaw.net \
  -d forum.starclaw.net \
  -d arena.starclaw.net
```

### 3. 配置 Nginx

```bash
sudo cp /opt/starclaw/queen/deploy/nginx-queen.conf /etc/nginx/sites-available/queen
sudo ln -sf /etc/nginx/sites-available/queen /etc/nginx/sites-enabled/
sudo rm -f /etc/nginx/sites-enabled/default
sudo nginx -t && sudo systemctl reload nginx
```

### 4. 自动续签

```bash
sudo crontab -e
# 添加：
0 3 * * * certbot renew --quiet && systemctl reload nginx
```

## 六、日常运维

### 更新

```bash
cd /opt/starclaw/queen
git pull origin main
docker compose -f docker-compose.prod.yml build
docker compose -f docker-compose.prod.yml up -d
```

### 日志

```bash
docker logs starclaw-queen-api --tail 100 -f      # Queen API
docker logs starclaw-queen-swarm --tail 100 -f     # Swarm
docker logs starclaw-queen-bounty --tail 100 -f    # Bounty
docker compose -f docker-compose.prod.yml logs -f  # 全部
```

### 备份

```bash
docker exec starclaw-queen-mysql mysqldump -uroot \
  -p"$QUEEN_DB_PASSWORD" starclaw_queen > queen_backup_$(date +%Y%m%d).sql
```

### 重启

```bash
docker compose -f docker-compose.prod.yml restart              # 全部
docker compose -f docker-compose.prod.yml restart queen-api    # 单个
```

### 监控

- **Prometheus**: http://127.0.0.1:9090 （仅内网）
- **Grafana**: http://127.0.0.1:3000 （仅内网）
  - 默认账号: `admin` / `$GRAFANA_PASSWORD`
  - 预配置 Dashboard: StarClaw Overview

如需外部访问 Grafana，在 nginx 中添加：
```nginx
server {
    listen 443 ssl http2;
    server_name grafana.starclaw.net;
    ssl_certificate     /etc/letsencrypt/live/starclaw.net/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/starclaw.net/privkey.pem;
    location / {
        proxy_pass http://127.0.0.1:3000;
        proxy_set_header Host $host;
    }
}
```

## 七、服务端口速查

| 服务 | 容器名 | 端口 |
|------|--------|------|
| MySQL | starclaw-queen-mysql | 3306 (内网) |
| Queen API | starclaw-queen-api | 8085 |
| Queen Web | starclaw-queen-web | 8086 |
| Swarm | starclaw-queen-swarm | 8090 |
| Core | starclaw-queen-core | 8091 |
| Bounty | starclaw-queen-bounty | 8092 |
| Forum | starclaw-queen-forum | 8093 |
| Arena | starclaw-queen-arena | 8094 |
| Prometheus | starclaw-queen-prometheus | 9090 |
| Grafana | starclaw-queen-grafana | 3000 |

## 八、常见问题

**Q: 构建时 Docker Hub 拉不下来？**
确认已配置 Docker 镜像加速。

**Q: 内存不足？**
```bash
sudo fallocate -l 4G /swapfile
sudo chmod 600 /swapfile && sudo mkswap /swapfile && sudo swapon /swapfile
echo '/swapfile swap swap defaults 0 0' | sudo tee -a /etc/fstab
```

**Q: 端口被占用？**
```bash
ss -tlnp | grep 8085
# 所有服务端口绑定 127.0.0.1，不对外暴露，由 Nginx 代理
```
