---
description: Parallel development workflow — branching strategy for multiple developers/AI sessions working concurrently
---

# Parallel Development Workflow

StarClaw monorepo 并行开发指南。三层 AI 军团：DevClaw 集群（产品层）+ 多 Windsurf（代码层）+ 主 Windsurf（指挥层）。

完整文档: `PARALLEL_DEV.md`

## 快速参考

```
代码层 (Windsurf):  feat/{service}/{name} 分支 → 开发 → push → 主 Windsurf 审查合并
产品层 (DevClaw):   Overlord 新建 DevClaw 实例 → 对话开发 Agent/Skill → 自动上架
指挥层 (主 Windsurf): 分配任务 → 审查代码 → 合并分支 → 部署 → 协调冲突
```

## 分支命名规范

```
master                     ← 生产分支，只通过合并进入
feat/{service}/{name}      ← 新功能       例: feat/claw/memory-v2
fix/{service}/{name}       ← 修复         例: fix/hive/hive-url
refactor/{name}            ← 重构         例: refactor/carapace-api
hotfix/{name}              ← 紧急修复     例: hotfix/auth-crash (可直接合 master)
```

## 代码层: 开始新功能 (Windsurf)

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

## 代码层: 合并到 master (主 Windsurf)

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

8. 通知其他 Windsurf 会话 rebase:
```
"master 已更新 (合并了 feat/xxx)，请 git fetch nydus && git rebase nydus/master"
```

## 产品层: DevClaw 并行开发

DevClaw 负责 Agent / Skill / Workflow / Team Template 开发，不需要 IDE：

9. 登录 Overlord (overlord.starclaw.me) → 团队智能体 → 新建 DevClaw 实例

10. 对话描述需求，DevClaw 五虫协作：设计 → 编码 → 测试 → 审查 → 上架

11. 如果 DevClaw 产出需要新的 Go Tool，转发给 Windsurf 会话:
```
DevClaw: "这个 Agent 需要 medical_catalog 技能"
  → 主 Windsurf 分配: "Windsurf-N，接 feat/claw/medical-tool"
  → Windsurf-N 实现 → 主 Windsurf 合并 → DevClaw 继续
```

## 职责边界

| 任务 | 谁做 |
|------|------|
| 新 Agent (Prompt + 配置) | DevClaw |
| JSON Plugin (包装 API) | DevClaw |
| Workflow 编排 | DevClaw |
| Team Template | DevClaw |
| Go Built-in Tool | Windsurf |
| API / Web 功能 | Windsurf |
| 基础设施 / CI / 部署 | 主 Windsurf |
| 架构决策 | 你 + 主 Windsurf |

## 服务独立性 (12 个并行通道)

| 服务 | 目录 | 独立部署 |
|------|------|----------|
| Claw API | claw/api/ | Server A |
| Claw Web | claw/web/ | Server A |
| Hive | hive/api/ | Server A |
| Queen API | queen/api/ | Server C |
| Queen Swarm | queen/swarm/ | Server C |
| Queen Arena | queen/arena/ | Server C |
| Synapse API | synapse/api/ | Server B |
| Synapse Core | synapse/core/ | Server B |
| Overlord | overlord/api/ | Server C |
| Nydus | nydus/api/ | Server C |
| Spore | spore/ | 客户端 |
| Carapace | carapace/ | 库 |

## 冲突高危文件 (主 Windsurf 统一改)

- `claw/api/internal/router/router.go` — 路由注册
- `claw/web/src/App.tsx` — 前端路由
- `queen/api/internal/router/router.go` — Queen 路由
- `docker-compose.*.yml` — 容器编排
- `nydus/hooks/post-receive-starclaw` — 部署脚本

## CI 检查

PR/push 后 GitHub Actions 自动运行 `.github/workflows/ci.yml`:
- `dorny/paths-filter` 检测变更目录
- 只构建受影响的服务
- Go: vet + build / Frontend: npm ci + vite build

## 紧急 Hotfix

不需要走分支流程:
```
git checkout master
# 做最小修复
git commit -m "hotfix: ..."
git push nydus master
```
