# StarClaw Overlord 👁️ — 领主企业管理层（闭源）

> 领主（Overlord）是 StarClaw 虫群架构的企业级中间管理节点

## 定位

Overlord 是**付费的企业管理层**，部署在 Claw 之上、Queen 之下，
为企业客户提供多节点管理、任务编排、负载均衡等高级能力。

星际争霸中，领主提供人口上限、侦察视野和运输能力——
StarClaw 的领主同样掌控**资源配额、监控视野和任务分发**。
一个 Overlord 管辖的所有 Claw 集群统称为一个 **Brood（虫群）**。

```
👑 Queen（虫后）         闭源，starclaw.me 中央管控
     │
👁️ Overlord（领主）      闭源，企业付费管理层  ← 本模块
     │  管辖一个 Brood（虫群）
🦞🦞🦞 Claw（小龙虾）    开源，最小执行单元
```

## 核心功能（计划中）

| 功能 | 说明 |
|------|------|
| **Claw 注册管理** | 下属 Claw 节点注册、心跳监控、健康检查 |
| **资源配额（人口上限）** | 管控并发任务数、Token 消耗限额 |
| **侦察视野（监控）** | 实时采集全网指标（CPU/内存/错误率/延迟） |
| **任务编排（运输）** | 将任务智能分配到最优 Claw（负载均衡） |
| **企业管理控制台** | Web UI 管理下属 Claw、查看用量、配置策略 |
| **Nydus 隧道管理** | 管理 Brood 内部 Claw 间的 P2P 直连 |
| **数据聚合** | 汇聚下属 Claw 的用量/日志，向上报告给 Queen |
| **审批更新** | 企业模式下审批 Molt 蜕皮更新 |
| **本地缓存** | 缓存 Queen 下发的配置/模板，降低延迟 |
| **多租户隔离** | 企业内部团队/部门级数据隔离 |
| **审计日志** | 所有管理操作记录审计日志 |

## 目录结构（计划）

```
overlord/
├── manager/               # Overlord 管理服务（Go）
│   ├── cmd/               # 入口
│   ├── internal/
│   │   ├── registry/      # Claw 节点注册 & 发现
│   │   ├── scheduler/     # 任务调度 & 负载均衡
│   │   ├── monitor/       # 健康监控 & 指标采集
│   │   ├── nydus/         # P2P 隧道管理
│   │   ├── cache/         # 配置/模板本地缓存
│   │   └── api/           # 管理 API
│   ├── Dockerfile
│   └── go.mod
├── console/               # Overlord 管理控制台（Web UI）
│   ├── src/
│   └── package.json
└── README.md              # ← 本文件
```

## 与 Claw 的关系

- 每个 Overlord 部署时**内嵌一个完整的 Claw 实例**（Overlord 自身也能执行 AI 任务）
- Overlord 管理服务作为独立进程运行在 Claw 旁边
- 企业客户部署：1 个 Overlord + N 个 Claw = 1 个 Brood（虫群）

## 商业模式

- 按管理的 Claw 节点数量收费
- 企业年度订阅制
- 包含技术支持和 SLA 保障
