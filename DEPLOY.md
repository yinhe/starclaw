# StarClaw 部署手册

> **给 AI 助手的指引：当用户说「部署」，执行本文档的「日常部署」流程即可。**

## 一、基础设施

```
┌─────────────────────────────────────────────────────────────┐
│  开发者本机  git push nydus master                          │
│       │                                                     │
│       ▼                                                     │
│  Server C: Nydus CI/CD (43.106.158.26)                     │
│       │ post-receive hook → Nydus API → 按 target 自动分发  │
│       ├──────────────────┬──────────────────┬───────────────┤
│       ▼                  ▼                  ▼               │
│  queen-server-c    gateway-server-b   claw-starclaw-me     │
│  (本地 Worm)       (SSH+Worm)         (SSH direct)          │
│  queen/ subdir     queen/api/ subdir  claw/ subdir          │
│  starclaw.net      star-ai.net        starclaw.me           │
│                                             │               │
│                                        claw-dgp (客户)      │
│                                        43.106.114.174       │
└─────────────────────────────────────────────────────────────┘
```

### 服务器清单

| 代号 | 域名 | IP | 用途 | SSH |
|------|------|----|------|-----|
| A | starclaw.me | — | Claw 官方实例 | `ssh -i ~/.ssh/claw_deploy root@starclaw.me` |
| B | star-ai.net | 47.103.51.32 | Synapse (AI 算力平台) | `ssh -i ~/.ssh/starai_deploy root@47.103.51.32` |
| C | starclaw.net | 43.106.158.26 | Queen + Nydus CI/CD | `ssh -i ~/.ssh/queen_deploy root@43.106.158.26` |
| D | proxy.starclaw.net | 47.237.11.193 | 海外中转 Proxy | `ssh -i ~/.ssh/starai_proxy_deploy root@47.237.11.193` |

> SSH key 配置详见根目录 `.env` 文件。

### 代码目录映射

| 本地 monorepo 子目录 | 部署到 | 服务器路径 |
|---------------------|--------|-----------|
| `queen/` | Server C | `/opt/queen` |
| `queen/api/` | Server B | `/opt/starclaw/gateway` |
| `claw/` | Server A | `/opt/starclaw` |
| `claw/` | 客户 Claw | `/opt/starclaw` |

---

## 二、日常部署（Deploy）

### 什么是 Deploy

**Deploy = 把当前代码推送到所有生产服务器，Docker 重新构建并重启。**

所有在线服务器同步更新，无版本号，适用于日常开发迭代。

### 步骤（3 条命令）

```bash
# 1. 提交代码
git add -A
git commit -m "feat: 简要描述改动"

# 2. 推送到 Nydus（自动触发全量部署）
git push nydus master
```

**结束。** Nydus 会自动：
1. `post-receive` hook 通知 Nydus API
2. 按 `nydus.yaml` 配置，串行部署到 4 个 target
3. 每个 target: 提取对应 subdir → SSH 传输 → Docker compose build + up

### 验证

```bash
# 查看 Nydus 部署日志
ssh -i ~/.ssh/queen_deploy root@43.106.158.26 "docker logs nydus-api --tail 20 2>&1 | grep -E 'deploy|success|fail'"

# 检查各服务健康
curl -sf https://api.starclaw.me/v1/health       # Claw
curl -sf https://swarm.starclaw.net/health        # Queen Swarm
curl -sf https://star-ai.net/health               # Router
```

### 注意事项

- **不要手动 SCP 文件到服务器** — 一切通过 `git push nydus master`
- tag 推送会触发 `"no targets for this branch"` — 正常，tag 不匹配 branch 过滤
- 部署是串行的，全部完成约 3-5 分钟
- 如果某个 target 失败，不影响其他 target

---

## 三、发布版本（Release）

### 什么是 Release

**Release = 给当前代码打版本号 tag，用于 Molt 蜕皮自动更新分发给所有 Claw 节点。**

Release 面向的是**外部用户的 Claw 节点**（通过 swarm 加入的客户），不是我们自己的服务器。

### Deploy vs Release 对比

| | Deploy | Release |
|--|--------|---------|
| **对象** | 我们自己的 4 台服务器 | 所有外部 Claw 节点 |
| **触发** | `git push nydus master` | `git tag` + Nydus Release API |
| **机制** | SSH + Docker rebuild | Molt 蜕皮心跳检查 + OTA 下载 |
| **速度** | 即时（3-5 分钟） | 渐进（节点下次心跳时更新） |
| **回滚** | 重新 push 旧代码 | 发布旧版本 tag |
| **频率** | 每次提交都可以 | 稳定后才发 |

### 发布步骤

```bash
# 1. 先完成日常部署（确保我们自己的服务器运行正常）
git push nydus master

# 2. 确认线上无问题后，打版本 tag
git tag v2026.MMDD.HHMM

# 3. 推送 tag（仅推送 tag，不会触发重新部署）
git push nydus --tags

# 4. 创建 Release（通知 Nydus 打包供外部节点下载）
# 外部 Claw 节点通过 Molt 心跳自动检测并更新
```

### 版本号规范

```
v2026.0315.2316
  │     │    │
  │     │    └── 时间 HHMM（24 小时制）
  │     └─────── 日期 MMDD
  └───────────── 年份
```

---

## 四、Spore 桌面客户端发布

### 什么是 Spore 发布

**Spore = 桌面安装包（Windows/Linux/macOS），内嵌 Claw API + Web 前端。**

Spore 不走 Nydus 自动部署（构建链复杂：Go 交叉编译 + embed 嵌入），采用**本地构建 + 上传**模式。

### 发布步骤（1 条命令）

```powershell
# 在 monorepo 根目录执行（需要 Go 工具链 + SSH key）
.\spore\scripts\release-spore.ps1
```

**自动完成：**
1. 调用 `build-release.ps1` 交叉编译 4 个平台（Windows/Linux/macOS×2）
2. SCP 上传安装包到 Nydus `/data/nydus/repos/releases/`
3. 更新 `spore-latest.json` 版本清单

### 输出产物

| 平台 | 文件名 | 下载地址 |
|------|--------|----------|
| Windows | `StarClaw-Setup-v2026.MMDD.HHMM.exe` | `https://nydus.starclaw.net/releases/download/...` |
| Linux | `StarClaw-Setup-v2026.MMDD.HHMM-linux-amd64.tar.gz` | 同上 |
| macOS ARM | `StarClaw-Setup-v2026.MMDD.HHMM-darwin-arm64` | 同上 |
| macOS Intel | `StarClaw-Setup-v2026.MMDD.HHMM-darwin-amd64` | 同上 |

### 版本检查 API

```bash
# Spore 最新版本信息
curl -sf https://nydus.starclaw.net/releases/spore/latest
```

### 可选参数

```powershell
.\spore\scripts\release-spore.ps1 -SkipBuild    # 仅上传（已有 dist/）
.\spore\scripts\release-spore.ps1 -SkipUpload   # 仅构建（不上传）
```

---

## 五、手动部署（备用）

仅在 Nydus 故障时使用。

### Claw (Server A)

```bash
ssh -i ~/.ssh/claw_deploy root@starclaw.me
cd /opt/starclaw
make update    # git pull + docker rebuild
```

### Queen (Server C)

```bash
ssh -i ~/.ssh/queen_deploy root@43.106.158.26
cd /opt/queen
git pull
docker compose -f docker-compose.prod.yml up -d --build
```

### Synapse/Gateway (Server B)

```bash
ssh -i ~/.ssh/starai_deploy root@47.103.51.32
cd /opt/starclaw/gateway
# 代码由 Nydus Worm 管理，手动时需要从 monorepo 提取 queen/api/ 子目录
```

---

## 六、Nydus 管理

### 配置文件

- 服务器上: `/opt/nydus/api/configs/nydus.yaml`
- 本地: `nydus/api/configs/nydus.yaml`

### 手动触发部署

```bash
# 在 Server C 上
curl -X POST 'http://127.0.0.1:8095/api/repos/starclaw/deploy?branch=master' \
  -H 'X-Nydus-Secret: nydus-sc2026-vKwRmT9xLpQjN3hB'
```

### 查看部署记录

```bash
# Nydus Dashboard
https://43.106.158.26  (Nydus Web)

# 或 API
curl -s 'http://127.0.0.1:8095/api/deploys' \
  -H 'X-Nydus-Secret: nydus-sc2026-vKwRmT9xLpQjN3hB'
```

### 添加新的 Claw 客户节点

编辑 `nydus/api/configs/nydus.yaml`，在 `starclaw.targets` 下添加：

```yaml
- name: "claw-<客户名>"
  ssh_host: "root@<客户IP>"
  ssh_key: "/root/.ssh/id_ed25519"
  deploy_path: "/opt/starclaw"
  deploy_cmd: "docker compose up -d --build api web"
  subdir: "claw"          # 必须是 claw，绝不允许 queen
  branch: "master"
```

然后 `git push nydus master` 更新 Nydus 配置并生效。

---

## 七、故障排查

```bash
# Nydus 容器状态
ssh -i ~/.ssh/queen_deploy root@43.106.158.26 "docker ps | grep nydus"

# Nydus API 日志
ssh -i ~/.ssh/queen_deploy root@43.106.158.26 "docker logs nydus-api --tail 50"

# 某个服务的 Docker 日志
ssh -i ~/.ssh/claw_deploy root@starclaw.me "docker logs starclaw-api --tail 50"
ssh -i ~/.ssh/queen_deploy root@43.106.158.26 "docker logs starclaw-queen-api --tail 50"
ssh -i ~/.ssh/starai_deploy root@47.103.51.32 "docker logs star-ai-api --tail 50"
```
