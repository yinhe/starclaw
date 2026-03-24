---
description: Parallel development workflow — branching strategy for multiple developers/AI sessions working concurrently
---

# Parallel Development Workflow

StarClaw monorepo 并行开发指南。适用于多人/多AI会话同时开发不同功能。

## 分支命名规范

```
master              ← 生产分支，只通过 PR 合并，push 后 nydus 自动部署
feat/{service}/{name}  ← 新功能       例: feat/claw/memory-v2
fix/{service}/{name}   ← 修复         例: fix/hive/hive-url
refactor/{name}        ← 重构         例: refactor/carapace-api
hotfix/{name}          ← 紧急修复     例: hotfix/auth-crash (可直接合 master)
```

## 开始新功能开发

// turbo
1. 确保 master 是最新的:
```
git checkout master && git pull nydus master
```

2. 创建功能分支:
```
git checkout -b feat/{service}/{feature-name}
```

3. 正常开发、commit。每个 commit 应只涉及一个服务目录。

4. 推送分支到 nydus（不会触发部署，hook 只响应 master）:
```
git push nydus feat/{service}/{feature-name}
```

## 合并到 master

5. 合并前先 rebase master:
```
git fetch nydus master
git rebase nydus/master
```

6. 解决冲突（如有），然后合并:
```
git checkout master
git merge --no-ff feat/{service}/{feature-name}
git push nydus master
```

7. nydus post-receive hook 自动检测变更目录并部署对应服务。

## 并行开发场景

### 场景 A: 两个 AI 会话同时开发

```
AI Session 1: feat/claw/memory-v2    (改 claw/api/internal/memory/)
AI Session 2: feat/queen/arena-rank  (改 queen/arena/)

互不影响 — 不同服务目录，无冲突。
谁先完成谁先合 master。
```

### 场景 B: 同一服务的不同功能

```
AI Session 1: feat/claw/memory-v2    (改 memory/, model/)
AI Session 2: feat/claw/mcp-tools    (改 mcp/, tool/)

可能有冲突 — 如果改了相同文件（如 router.go 加路由）。
解决: 第二个合并时 rebase + 手动解决冲突。
```

### 场景 C: 你开发 + AI 开发

```
你:    直接在 master 上改小东西（快速 fix）
AI:    feat/spore/sandbox-test（独立功能）

你先 push master → AI rebase → AI 合并
```

## 服务独立性参考

以下服务完全独立，可完全并行:

| 服务 | 目录 | go.mod | 独立部署 |
|------|------|--------|----------|
| Claw API | claw/api/ | ✅ | Server A |
| Claw Web | claw/web/ | N/A (npm) | Server A |
| Hive | hive/api/ | ✅ | Server A |
| Queen API | queen/api/ | ✅ | Server C |
| Queen Swarm | queen/swarm/ | ✅ | Server C |
| Queen Arena | queen/arena/ | ✅ | Server C |
| Synapse API | synapse/api/ | ✅ | Server B |
| Synapse Core | synapse/core/ | N/A (npm) | Server B |
| Overlord | overlord/api/ | ✅ | Server C |
| Nydus | nydus/api/ | ✅ | Server C |
| Spore | spore/ | ✅ | 客户端 |
| Carapace | carapace/ | ✅ | 库 |

## 冲突高危文件

这些文件跨功能常被修改，合并时注意:
- `claw/api/internal/router/router.go` — 路由注册
- `claw/web/src/App.tsx` — 前端路由
- `queen/api/internal/router/router.go` — Queen 路由
- `docker-compose.*.yml` — 容器编排
- `nydus/hooks/post-receive-starclaw` — 部署脚本

## CI 检查

PR 创建后 GitHub Actions 自动运行 `.github/workflows/ci.yml`:
- 只检查变更涉及的服务 (paths-filter)
- Go: vet + build
- Frontend: npm ci + vite build
- 全部通过后可合并

## 紧急 Hotfix

不需要走 PR 流程:
```
git checkout master
# 做最小修复
git commit -m "hotfix: ..."
git push nydus master
```
