# Claw 云船队（Cloud Fleet）— 大规模 Claw 部署方案

> 让每个用户都能一键拥有自己的 AI 智能体节点

## 1. 全局架构

```
用户在 app.starclaw.me 点击「一键创建 Claw」
         │
         ▼
    ┌─────────────┐
    │  Hive 控制器  │  ← 调度中心（运行在 starclaw.me 服务器）
    │  (Go API)    │
    └──────┬──────┘
           │ 根据资源/策略选择部署模式
           │
    ┌──────┼───────────────────────────┐
    │      │                           │
    ▼      ▼                           ▼
┌────────┐ ┌────────────┐      ┌──────────────┐
│ Hive   │ │ Aliyun ECS │      │ Spore 本地   │
│ 蜂巢模式│ │ 云服务器模式 │      │ 孢子模式     │
│        │ │            │      │              │
│同一台   │ │独立 VM     │      │用户自己的    │
│服务器   │ │每 Claw 一台│      │电脑/树莓派   │
│多容器   │ │完全隔离    │      │              │
└────────┘ └────────────┘      └──────────────┘
    │            │                     │
    └────────────┼─────────────────────┘
                 │
                 ▼  全部注册到
           ┌──────────┐
           │ Overlord │  ← 虫巢网络中心
           │ 领主控制台 │
           └──────────┘
```

## 2. 三种部署模式对比

| 维度 | Hive 蜂巢 | Aliyun ECS 云 | Spore 孢子 |
|------|----------|--------------|-----------|
| **场景** | 免费/试用/轻量 | 企业/生产/独占 | 个人/本地/边缘 |
| **成本** | 共享资源，低 | 按 VM 计费，中高 | 用户自有设备，0 |
| **隔离** | 容器级（共享内核） | VM级（完全隔离） | 物理隔离 |
| **创建速度** | < 10秒 | 2-5分钟 | 手动安装 |
| **域名** | `{slug}.starclaw.me` | `{slug}.starclaw.me` | 用户自定义 |
| **数据持久化** | 共享MySQL+独立卷 | 独立MySQL+独立磁盘 | SQLite本地 |
| **可控性** | 平台全权管理 | 平台管理 | 用户自管 |
| **适合规模** | 50-200个/服务器 | 无限（预算为限） | 无限（去中心化） |

---

## 3. Hive 蜂巢模式（核心，优先实现）

### 3.1 架构

```
starclaw.me 服务器
│
├── Nginx（通配符反向代理）
│   ├── *.starclaw.me → 路由到对应 Claw 容器
│   ├── app.starclaw.me → 主站 Web (现有)
│   └── api.starclaw.me → Hive API
│
├── Hive Controller（Go API :9090）
│   ├── POST /hive/claws          创建 Claw 实例
│   ├── DELETE /hive/claws/:slug  销毁 Claw 实例
│   ├── GET /hive/claws           列出所有实例
│   ├── GET /hive/claws/:slug     实例详情
│   ├── POST /hive/claws/:slug/start|stop|restart
│   └── GET /hive/stats           资源统计
│
├── 共享基础设施
│   ├── MySQL 8.0（共享实例，每 Claw 独立数据库）
│   │   ├── claw_alice
│   │   ├── claw_bob
│   │   └── claw_charlie
│   ├── Redis 7（共享实例，key 前缀隔离）
│   │   ├── alice:*
│   │   ├── bob:*
│   │   └── charlie:*
│   └── 共享 Web 前端容器（所有 Claw 共用一套 UI）
│
└── Claw 容器池
    ├── claw-alice-api   (:9001) → alice.starclaw.me
    ├── claw-bob-api     (:9002) → bob.starclaw.me
    └── claw-charlie-api (:9003) → charlie.starclaw.me
```

### 3.2 数据持久化策略

```
/opt/starclaw-hive/
├── shared/
│   ├── mysql/          # 共享 MySQL 数据
│   └── redis/          # 共享 Redis 数据
│
├── instances/
│   ├── alice/
│   │   ├── identity/   # Ed25519 密钥（每 Claw 唯一身份）
│   │   ├── uploads/    # 用户上传文件
│   │   ├── workspaces/ # 工作空间
│   │   └── images/     # 生成的图片
│   ├── bob/
│   │   └── ...
│   └── charlie/
│       └── ...
│
├── nginx/
│   ├── conf.d/
│   │   ├── alice.conf  # 自动生成的 nginx 配置
│   │   ├── bob.conf
│   │   └── charlie.conf
│   └── ssl/
│       └── starclaw.me/  # 通配符 SSL 证书
│
├── docker-compose.hive.yml   # Hive 基础设施
├── .env                       # 全局配置
└── hive-controller            # Hive 控制器二进制
```

**为什么选共享 MySQL 而不是每 Claw 一个 MySQL？**
- 每个 MySQL 容器占 200-400MB 内存
- 100 个 Claw = 20-40GB 只给 MySQL → 不现实
- 共享 MySQL + 独立数据库：1 个 MySQL 容器，所有 Claw 共用，数据通过 DB 名隔离
- 删除 Claw 时：DROP DATABASE claw_{slug} → 干净清理

### 3.3 创建流程

```
用户点击「一键创建」
    │
    ├─ 1. 校验 slug（字母数字，3-20字符，不在白名单中）
    │
    ├─ 2. 分配端口号（从 9001 起递增，查找空闲端口）
    │
    ├─ 3. 创建 MySQL 数据库
    │     CREATE DATABASE claw_{slug}
    │     CREATE USER 'claw_{slug}'@'%' IDENTIFIED BY '{random}'
    │     GRANT ALL ON claw_{slug}.* TO 'claw_{slug}'@'%'
    │
    ├─ 4. 创建数据目录
    │     mkdir -p /opt/starclaw-hive/instances/{slug}/{identity,uploads,...}
    │
    ├─ 5. 生成 Ed25519 密钥对（节点身份）
    │     → 得到 claw:{hex_pubkey} 地址
    │
    ├─ 6. 启动 Docker 容器
    │     docker run -d --name claw-{slug}-api \
    │       --network hive-net \
    │       -e STARCLAW_DATABASE_HOST=hive-mysql \
    │       -e STARCLAW_DATABASE_DBNAME=claw_{slug} \
    │       -e STARCLAW_NODE_ADDRESS=https://{slug}.starclaw.me \
    │       -e STARCLAW_OVERLORD_ENABLED=true \
    │       -e STARCLAW_OVERLORD_OVERLORD_URL=https://overlord.starclaw.net \
    │       -v /opt/starclaw-hive/instances/{slug}/identity:/app/data/identity \
    │       -v /opt/starclaw-hive/instances/{slug}/uploads:/app/uploads \
    │       -p 127.0.0.1:{port}:8080 \
    │       starclaw-api:latest
    │
    ├─ 7. 生成 Nginx 配置
    │     server {
    │       server_name {slug}.starclaw.me;
    │       location / { proxy_pass http://127.0.0.1:{port}; }
    │       # SSL 使用通配符证书
    │     }
    │     nginx -s reload
    │
    ├─ 8. 等待健康检查通过
    │     curl http://127.0.0.1:{port}/health
    │
    └─ 9. 注册到 Overlord（Claw 自动注册）
          → 节点出现在 Overlord Console
```

### 3.4 二级域名白名单（保留域名）

以下子域名不允许用户注册：

```
# 系统保留
app, api, www, admin, console, root, system, null

# 基础设施
overlord, queen, nydus, swarm, hive, spore, creep
mail, mx, smtp, imap, pop, ftp, ssh, dns, ns1, ns2

# 静态资源
cdn, static, assets, img, images, media, files, uploads

# 产品/服务
docs, help, support, status, blog, forum, wiki, community
store, shop, pay, billing, auth, sso, login, register
download, downloads, release, releases, update, updates
git, repo, registry, mirror, proxy

# 开发/运维
dev, staging, test, demo, sandbox, preview, beta, alpha
monitor, grafana, prometheus, kibana, elastic
ci, cd, jenkins, travis, drone

# 品牌保护
starclaw, star-claw, yinhe, yinheai
```

### 3.5 资源限制（per Claw）

```yaml
# Docker resource limits
deploy:
  resources:
    limits:
      cpus: '0.5'        # 最多 0.5 核
      memory: 512M        # 最多 512MB
    reservations:
      cpus: '0.1'        # 保证 0.1 核
      memory: 128M        # 保证 128MB

# 存储限制
storage:
  max_upload_size: 50MB   # 单文件上传限制
  max_total_storage: 2GB  # 总存储限制
  max_db_size: 500MB      # 数据库大小限制
```

**单台 8C32G 服务器容量估算：**
- MySQL 共享: ~2GB
- Redis 共享: ~1GB
- 每 Claw: 128-512MB
- 安全容量: 约 50 个 Claw 实例
- 极限容量: 约 150 个（轻负载场景）

---

## 4. Aliyun ECS 云服务器模式

### 4.1 何时用 ECS 模式

- Hive 服务器资源不足时自动扩展
- 用户需要独占资源（企业客户）
- 需要特定地域部署（合规要求）
- 用户愿意付费获得更高性能

### 4.2 阿里云 API 集成

```go
// 使用阿里云 OpenAPI SDK
import (
    ecs "github.com/alibabacloud-go/ecs-20140526/v4/client"
    dns "github.com/alibabacloud-go/alidns-20150109/v4/client"
)

// 创建 ECS 实例
func CreateClawECS(slug string, spec ECSSpec) (*ECSInstance, error) {
    // 1. 调用 RunInstances API 创建 ECS
    request := &ecs.RunInstancesRequest{
        RegionId:        tea.String("cn-shanghai"),
        ImageId:         tea.String("ubuntu_22_04"),     // Ubuntu 22.04
        InstanceType:    tea.String(spec.InstanceType),   // ecs.t6-c1m2.large
        SecurityGroupId: tea.String(securityGroupId),
        VSwitchId:       tea.String(vswitchId),
        Amount:          tea.Int32(1),
        
        // Cloud-Init 自动安装 Claw
        UserData: tea.String(base64Encode(cloudInitScript(slug))),
    }
    
    // 2. 绑定弹性公网 IP
    // 3. 添加 DNS 记录
    // 4. 等待初始化完成
    // 5. 注册到 Overlord
}

// 添加 DNS 记录
func AddSubdomain(slug, ip string) error {
    request := &dns.AddDomainRecordRequest{
        DomainName: tea.String("starclaw.me"),
        RR:         tea.String(slug),      // 子域名前缀
        Type:       tea.String("A"),       // A 记录
        Value:      tea.String(ip),        // ECS 公网 IP
    }
    _, err := dnsClient.AddDomainRecord(request)
    return err
}
```

### 4.3 Cloud-Init 脚本

```bash
#!/bin/bash
# ECS 实例初始化脚本（通过 UserData 传入）

# 安装 Docker
curl -fsSL https://get.docker.com | sh

# 拉取 Claw 代码
git clone --depth 1 https://github.com/yinhe/starclaw.git /opt/starclaw

# 配置环境变量
cat > /opt/starclaw/.env << 'EOF'
JWT_SECRET={{JWT_SECRET}}
DB_ROOT_PASSWORD={{DB_PASSWORD}}
OVERLORD_ENABLED=true
OVERLORD_URL=https://overlord.starclaw.net
OVERLORD_NODE_NAME=claw-{{SLUG}}
OVERLORD_CLAW_TOKEN={{CLAW_TOKEN}}
EOF

# 启动
cd /opt/starclaw
docker compose -f docker-compose.prod.yml up -d

# 配置 SSL (Let's Encrypt)
apt install -y certbot
certbot certonly --standalone -d {{SLUG}}.starclaw.me --non-interactive --agree-tos -m admin@starclaw.me

# 配置 Nginx
apt install -y nginx
# ... nginx config ...

# 回调通知 Hive Controller
curl -X POST https://api.starclaw.me/hive/callback \
  -H "X-Hive-Token: {{CALLBACK_TOKEN}}" \
  -d '{"slug":"{{SLUG}}","status":"ready","ip":"$(curl -s ifconfig.me)"}'
```

### 4.4 ECS 规格推荐

| 套餐 | 阿里云规格 | CPU | 内存 | 磁盘 | 月费(约) | 适合 |
|------|-----------|-----|------|------|---------|------|
| 入门 | ecs.t6-c1m2.large | 2C | 4GB | 40GB | ¥50 | 个人/试用 |
| 标准 | ecs.c7.xlarge | 4C | 8GB | 80GB | ¥200 | 小团队 |
| 专业 | ecs.c7.2xlarge | 8C | 16GB | 200GB | ¥400 | 企业 |
| 旗舰 | ecs.c7.4xlarge | 16C | 32GB | 500GB | ¥800 | 大企业 |

---

## 5. Spore + Hive + ECS 融合

### 5.1 统一注册流程

无论哪种部署模式，Claw 都通过相同方式注册到 Overlord：

```
Claw 启动
    │
    ├─ 生成 Ed25519 密钥对 → claw:{hex_pubkey} 地址
    │
    ├─ 连接 Overlord
    │   POST /brood/register
    │   {
    │     "name": "claw-alice",
    │     "address": "https://alice.starclaw.me",
    │     "claw_id": "claw:6aff1154...",
    │     "role": "claw",
    │     "deploy_mode": "hive|ecs|spore",  ← 标识部署模式
    │     "region": "cn-east"
    │   }
    │
    └─ Overlord 记录节点，Console 可见

三种模式的 Claw 在 Overlord 看来完全相同
→ 可以混合组队：Hive 节点 + ECS 节点 + Spore 节点 = 一个团队
```

### 5.2 Spore 作为「免费套餐」

```
starclaw.me 官网
│
├── 🆓 免费体验（Hive 蜂巢）
│   ├── 一键创建，10秒可用
│   ├── 限制：0.5核 / 512MB / 2GB 存储
│   ├── 子域名：{slug}.starclaw.me
│   └── 适合：体验产品、个人轻量使用
│
├── 💰 云端专属（Aliyun ECS）
│   ├── 独占服务器，3分钟可用
│   ├── 多种规格可选
│   ├── 子域名：{slug}.starclaw.me 或自定义域名
│   └── 适合：企业生产、高性能需求
│
└── 🏠 本地部署（Spore 孢子）
    ├── 下载安装包，本地运行
    ├── 无限制，完全私有
    ├── 自定义域名 / 内网访问
    └── 适合：隐私敏感、离线环境、已有设备
```

---

## 6. Hive Controller 数据模型

```go
// ClawInstance — Hive 管理的 Claw 实例
type ClawInstance struct {
    ID          string    `gorm:"primaryKey" json:"id"`
    Slug        string    `gorm:"uniqueIndex;size:30" json:"slug"`        // 子域名, e.g. "alice"
    DisplayName string    `json:"display_name"`                           // 显示名称
    OwnerID     string    `json:"owner_id"`                               // 创建者 ID
    OwnerEmail  string    `json:"owner_email"`                            // 创建者邮箱
    
    // 部署信息
    DeployMode  string    `json:"deploy_mode"`  // hive, ecs, spore
    Port        int       `json:"port"`         // Hive 模式: 内部端口
    ContainerID string    `json:"container_id"` // Docker 容器 ID
    ECSID       string    `json:"ecs_id"`       // ECS 模式: 实例 ID
    PublicIP    string    `json:"public_ip"`    // ECS 模式: 公网 IP
    
    // 身份
    ClawID      string    `json:"claw_id"`      // claw:{hex} 加密地址
    NodeID      string    `json:"node_id"`      // Overlord 分配的节点 ID
    
    // 状态
    Status      string    `json:"status"`       // creating, running, stopped, error, destroying
    DBName      string    `json:"db_name"`      // claw_{slug}
    StorageUsed int64     `json:"storage_used"` // bytes
    
    // 资源配置
    CPULimit    float64   `json:"cpu_limit"`    // CPU 核数限制
    MemoryLimit int64     `json:"memory_limit"` // 内存限制 bytes
    StorageMax  int64     `json:"storage_max"`  // 存储上限 bytes
    
    // 时间
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
    ExpiresAt   *time.Time `json:"expires_at"`  // 免费实例过期时间
    LastActiveAt time.Time `json:"last_active_at"`
}

// SubdomainBlacklist — 保留域名
type SubdomainBlacklist struct {
    Subdomain string `gorm:"primaryKey;size:50"`
    Reason    string // system, infrastructure, brand, service
}
```

---

## 7. Hive Controller API

```
# 用户 API（需要认证）
POST   /hive/claws                 创建 Claw 实例
GET    /hive/claws                 列出我的实例
GET    /hive/claws/:slug           实例详情
DELETE /hive/claws/:slug           销毁实例
POST   /hive/claws/:slug/stop     停止
POST   /hive/claws/:slug/start    启动
POST   /hive/claws/:slug/restart  重启

# 管理 API（需要管理员权限）
GET    /hive/admin/stats           全局资源统计
GET    /hive/admin/claws           所有实例列表
POST   /hive/admin/cleanup         清理过期实例
GET    /hive/admin/blacklist       查看保留域名
POST   /hive/admin/blacklist       添加保留域名

# 回调 API（内部）
POST   /hive/callback              ECS 初始化完成回调

# 健康检查
GET    /hive/health                Hive Controller 健康
```

---

## 8. SSL 通配符证书

### 8.1 Let's Encrypt 通配符证书

```bash
# 使用 DNS-01 挑战获取通配符证书
# 需要阿里云 DNS API 配合自动验证

# 安装 acme.sh + 阿里云 DNS 插件
curl https://get.acme.sh | sh
export Ali_Key="LTAI5t..."
export Ali_Secret="..."

# 申请通配符证书
acme.sh --issue \
  -d starclaw.me \
  -d '*.starclaw.me' \
  --dns dns_ali \
  --keylength ec-256

# 自动续期（acme.sh 自带 cron）
# 证书路径: ~/.acme.sh/starclaw.me_ecc/
```

### 8.2 Nginx 通配符配置模板

```nginx
# /etc/nginx/conf.d/hive-wildcard.conf
# 所有 *.starclaw.me 的默认处理

server {
    listen 443 ssl http2;
    server_name ~^(?<slug>[a-z0-9][a-z0-9-]{1,28}[a-z0-9])\.starclaw\.me$;

    ssl_certificate     /etc/letsencrypt/live/starclaw.me/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/starclaw.me/privkey.pem;

    # 动态解析 upstream 端口
    # 方案 A: 包含独立配置文件（Hive Controller 动态生成）
    include /opt/starclaw-hive/nginx/conf.d/$slug.conf;
}
```

**方案 B（更灵活）: 通过 Lua 动态路由**

```nginx
server {
    listen 443 ssl http2;
    server_name *.starclaw.me;

    ssl_certificate     /etc/letsencrypt/live/starclaw.me/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/starclaw.me/privkey.pem;

    location / {
        # 从 Redis 查询 slug → port 映射
        set_by_lua_block $backend_port {
            local slug = ngx.var.host:match("^(.-)%.starclaw%.me$")
            if not slug then return "" end
            local redis = require "resty.redis"
            local red = redis:new()
            red:connect("127.0.0.1", 6379)
            local port = red:get("hive:route:" .. slug)
            red:set_keepalive(10000, 100)
            return port ~= ngx.null and port or ""
        }

        if ($backend_port = "") {
            return 404;
        }

        proxy_pass http://127.0.0.1:$backend_port;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }
}
```

---

## 9. 实现优先级

### Phase 1: Hive 蜂巢（本周）
- [x] Hive Controller Go API
- [x] 共享 MySQL + per-slug 数据库管理
- [x] Docker API 容器生命周期管理
- [x] Nginx 配置自动生成 + reload
- [x] 子域名白名单
- [x] 通配符 SSL 证书
- [x] 官网「一键创建」前端

### Phase 2: 集成 Overlord
- [ ] Hive 实例自动注册到 Overlord
- [ ] Overlord Console 显示 Hive 实例
- [ ] 从 Console 创建/管理 Claw 实例

### Phase 3: Aliyun ECS
- [ ] 阿里云 ECS SDK 集成
- [ ] Cloud-Init 自动化脚本
- [ ] DNS API 子域名管理
- [ ] ECS 实例生命周期管理

### Phase 4: Spore 融合
- [ ] Spore Desktop 增加 Overlord 注册功能
- [ ] 本地 Claw 可选择加入虫巢网络
- [ ] Creep Agent 上报到 Hive Controller

---

## 10. 成本估算

### Hive 模式（单台 8C32G 服务器）

| 项目 | 成本 |
|------|------|
| 阿里云 ECS 8C32G | ~¥800/月 |
| 通配符 SSL | 免费 (Let's Encrypt) |
| 域名 | ~¥50/年 |
| **可支持 Claw 数** | **约 50-150 个** |
| **单 Claw 成本** | **¥5-16/月** |

### ECS 模式（按需创建）

| 规格 | 成本 | 定价建议 |
|------|------|---------|
| 2C4G | ~¥50/月 | ¥99/月 |
| 4C8G | ~¥200/月 | ¥399/月 |
| 8C16G | ~¥400/月 | ¥799/月 |

### Spore 模式

| 项目 | 成本 |
|------|------|
| 用户自有设备 | ¥0 |
| 仅消耗 AI API 调用 | 按使用量 |
