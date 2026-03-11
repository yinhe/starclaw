# Star-AI ⛽ 部署指南

> 服务器 B — star-ai.net (Router/Extractor AI 算力平台)

## 一、架构

```
用户 → star-ai.net (:443)
         ├── Web Dashboard (:3096, TODO)
         └── api.star-ai.net (:8096)
                ├── 国内直连 → 通义千问 / DeepSeek
                └── 海外中转 → Proxy (:8000) → OpenAI / Anthropic / Google / Grok / fal.ai
```

| 域名 | 用途 | 内部端口 |
|------|------|----------|
| star-ai.net | Web 前端（暂代理到 API） | :3096 |
| api.star-ai.net | Go API 后端 | :8096 |
| — | Node.js Proxy（仅内网） | :8000 |

## 二、服务器要求

| 项目 | 最低配置 | 推荐配置 |
|------|---------|---------|
| CPU | 2 核 | 4 核 |
| 内存 | 4 GB | 8 GB |
| 硬盘 | 40 GB SSD | 80 GB SSD |
| 系统 | Ubuntu 22.04 | Ubuntu 22.04 LTS |
| 网络 | 需能访问国内 AI API | 海外 API 由 Proxy 转发 |

## 三、安装依赖

```bash
# Docker
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER && newgrp docker

# Nginx + Certbot
sudo apt install -y nginx certbot python3-certbot-nginx

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
git clone https://github.com/yinhe/starclaw.git
cd starclaw/router
```

### 2. 配置环境变量

```bash
cp ../.env.production .env
nano .env
```

**必须修改：**
```
MYSQL_ROOT_PASSWORD=<强密码>
PROXY_INTERNAL_SECRET=<随机字符串>
```

**填入 API Keys（有哪些填哪些）：**
```
DASHSCOPE_API_KEY=sk-xxx          # 通义千问（国内直连）
DEEPSEEK_API_KEY=sk-xxx           # DeepSeek（国内直连）
OPENAI_API_KEY=sk-xxx             # OpenAI（经 Proxy）
ANTHROPIC_API_KEY=sk-xxx          # Anthropic（经 Proxy）
GOOGLE_API_KEY=xxx                # Google Gemini（经 Proxy）
FAL_API_KEY=xxx                   # fal.ai（经 Proxy）
GROK_API_KEY_1=xai-xxx            # Grok（经 Proxy，支持多 Key 轮转）
```

### 3. 构建 & 启动

```bash
# 构建（首次约 3-5 分钟）
docker compose build

# 启动
docker compose up -d

# 查看状态
docker compose ps
```

### 4. 验证

```bash
# 健康检查
curl http://localhost:8096/health
# 应返回: {"service":"star-ai","status":"ok"}

# 创建测试用户 + API Key（可选）
# 需要暴露 MySQL 端口或 docker exec 进去
```

## 五、域名 + HTTPS

### 1. DNS 解析

在域名商处添加 A 记录：
```
star-ai.net       → <服务器IP>
api.star-ai.net   → <服务器IP>
```

### 2. 申请 SSL 证书

```bash
sudo certbot certonly --nginx -d star-ai.net -d api.star-ai.net
```

### 3. 配置 Nginx

```bash
sudo cp /opt/starclaw/router/deploy/nginx-starai.conf /etc/nginx/sites-available/star-ai
sudo ln -sf /etc/nginx/sites-available/star-ai /etc/nginx/sites-enabled/
sudo nginx -t && sudo systemctl reload nginx
```

配置文件 `router/deploy/nginx-starai.conf` 已包含：
- HTTP → HTTPS 自动跳转
- star-ai.net → Web 前端（暂代理到 API）
- api.star-ai.net → Go API（含 SSE/WebSocket 支持）

### 4. 自动续签

```bash
sudo crontab -e
# 添加：
0 3 * * * certbot renew --quiet && systemctl reload nginx
```

## 六、支付证书

Router 的计费系统支持支付宝和微信支付充值，需要手动部署支付证书（不跟踪在 Git 中）。

### 证书文件

```
router/api/certs/
├── alipayCertPublicKey_RSA2.crt              # 支付宝公钥证书
├── alipayRootCert.crt                        # 支付宝根证书
├── appCertPublicKey_2021004192620828.crt      # 应用公钥证书
├── private_key.pem                           # 支付宝应用私钥
├── apiclient_cert.pem                        # 微信支付商户证书
├── apiclient_key.pem                         # 微信支付商户私钥
└── wechatpay_public.pem                      # 微信支付平台公钥
```

### 部署证书

证书已被 `.gitignore` 排除，需手动上传到服务器：

```bash
scp -r router/api/certs/ root@47.103.51.32:/opt/starclaw/router/api/certs/
```

> Dockerfile 已配置 `COPY certs/ ./certs/`，Docker 构建时会自动打包。
> 更换证书后需重新构建：`docker compose build api && docker compose up -d api`

## 七、日常运维

### 更新

```bash
cd /opt/starclaw/router
git pull origin main
docker compose build api
docker compose up -d api
```

### 日志

```bash
docker logs star-ai-api --tail 100 -f     # API 日志
docker logs star-ai-proxy --tail 100 -f   # Proxy 日志
docker compose logs -f                      # 全部
```

### 备份

```bash
# MySQL
docker exec star-ai-mysql mysqldump -uroot -p"$MYSQL_ROOT_PASSWORD" star_ai > backup_$(date +%Y%m%d).sql

# 恢复
docker exec -i star-ai-mysql mysql -uroot -p"$MYSQL_ROOT_PASSWORD" star_ai < backup_xxx.sql
```

### 重启

```bash
docker compose restart          # 全部
docker compose restart api      # 仅 API
docker compose restart proxy    # 仅 Proxy
```

## 八、API 端点速查

所有请求需携带 `Authorization: Bearer sk-star-xxx` 头。

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /health | 健康检查（无需认证） |
| GET | /v1/models | 可用模型列表（41 个） |
| POST | /v1/chat/completions | 对话补全 |
| POST | /v1/embeddings | 向量嵌入 |
| POST | /v1/images/generations | 图片生成 |
| POST | /v1/audio/speech | 语音合成 |
| POST | /v1/audio/transcriptions | 语音识别 |
| GET | /v1/keys | API Key 列表 |
| POST | /v1/keys | 创建 API Key |
| DELETE | /v1/keys/:id | 删除 API Key |
| GET | /v1/usage | 用量查询 |
| GET | /v1/balance | 余额查询 |

## 九、常见问题

**Q: Proxy 构建失败（Docker Hub 不可达）？**
在 `.env` 中设置镜像源：
```
DOCKER_REGISTRY=dockerpull.org
```

**Q: 国内 Provider 返回 401？**
检查 `DASHSCOPE_API_KEY` / `DEEPSEEK_API_KEY` 是否正确设置。

**Q: 海外请求超时？**
确认 Proxy 容器正常运行 `docker logs star-ai-proxy`，Proxy 需能访问海外网络。
