# Overlord Manager — 领主管理服务

> Go 服务，负责管理下属 Claw 节点（资源配额 + 侦察视野 + 任务运输）

## 计划功能

- Claw 节点注册 & 发现（`internal/registry/`）
- 任务调度 & 负载均衡（`internal/scheduler/`）
- 健康监控 & Overlord 指标采集（`internal/monitor/`）
- Nydus P2P 隧道管理（`internal/nydus/`）
- 管理 API（`internal/api/`）
- 配置/模板本地缓存（`internal/cache/`）

## 目录结构（计划）

```
manager/
├── cmd/server/main.go
├── internal/
│   ├── registry/
│   ├── scheduler/
│   ├── monitor/
│   ├── nydus/
│   ├── cache/
│   └── api/
├── Dockerfile
└── go.mod
```
