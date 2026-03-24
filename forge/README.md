# Forge 🔥 熔炉

> 虫群的战略指挥中心 — 全局研发管控 + 可视化大屏

## 定位

Forge 是 StarClaw monorepo 的**项目管理中心**，类似 Jira + Linear + Grafana Dashboard。

- **项目管理**: Project → Issue → Sprint → Milestone
- **看板视图**: Backlog / Todo / In Progress / Review / Done
- **可视化大屏**: 12 服务健康状态、Sprint 燃尽图、Git 活跃度热力图、部署时间线
- **数据聚合**: 整合 Nydus (Git/PR/Deploy)、Dev Bridge (MCP Tasks)、Overlord (DevClaw)、GitHub Actions (CI)

## 架构

```
forge/
├── api/     Go 后端 (:8099)
├── web/     React 可视化大屏 (:3099)
└── docs/    设计文档
```

## 启动

```bash
# API
cd api && go run ./cmd/server

# Web
cd web && npm run dev
```

## 文档

- [设计文档](docs/DESIGN.md) — 完整架构、数据模型、API 设计、可视化大屏、实施分期
