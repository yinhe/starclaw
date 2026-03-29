# Chrysalis 化蛹 — 宠物进化 & PK 对战系统

> "从幼虫到蛟龙的蜕变之旅" — StarClaw 情感宠物系统

## 概述

Chrysalis 是 StarClaw 生态中的宠物进化与竞技系统。每个 Claw 节点养育一只数字宠物，通过日常使用积累成长，装备武器，在竞技场与其他节点对战。

```
Larva(幼虫) ──→ Chrysalis(化蛹) ──→ Claw(成虫)
  日常使用        成长·进化·变异      装备·PK·竞技
```

## 架构

```
chrysalis/
├── cmd/server/       # 独立微服务入口
├── internal/
│   ├── engine/       # 战斗引擎 + Queen 计费客户端
│   ├── handler/      # HTTP API handlers
│   └── model/        # 数据模型 (GORM)
├── sdk/              # 外部集成 SDK (Claw 导入)
├── docs/             # 设计文档
├── Dockerfile
└── go.mod            # module starclaw.net/chrysalis
```

## 核心系统

| 系统 | 描述 | 开源 |
|------|------|------|
| **PK 对战** | 回合制战斗，ELO 匹配，路线克制 | ✅ |
| **装备系统** | 5 品质 × 6 部位，属性加成 | ✅ |
| **赛季系统** | 环境 buff，赛季排名，不活跃衰减 | ✅ |
| **打造系统** | 8 种材料，8 个配方，属性浮动 | ✅ |
| **变异系统** | 13 种变异特质，概率触发 | ✅ |
| **星尘经济** | 伴生货币，星能 10:1 转化 | ❌ 闭源 |
| **计费对接** | QueenClient 星能扣费 | ❌ 闭源 |

## 运行

```bash
# 环境变量
export CHRYSALIS_DSN="root:pass@tcp(127.0.0.1:3306)/starclaw_queen?charset=utf8mb4&parseTime=True&loc=Local"
export CHRYSALIS_PORT=8094

# 直接运行
go run ./cmd/server

# Docker
docker build -t chrysalis .
docker run -p 8094:8094 chrysalis
```

## API

所有端点前缀: `/chrysalis/pk/` (兼容 `/arena/pk/`)

| 方法 | 路径 | 描述 |
|------|------|------|
| POST | `/register` | 注册战斗员 |
| GET | `/fighter/:claw_id` | 查询战斗员 |
| POST | `/challenge` | 发起 PK |
| GET | `/leaderboard` | 排行榜 |
| GET | `/shop` | 装备商店 |
| POST | `/shop/buy` | 购买装备 |
| GET | `/season` | 当前赛季 |
| GET | `/stardust/:claw_id` | 星尘账户 |
| GET | `/craft/recipes` | 打造配方 |
| POST | `/craft` | 打造装备 |
| GET | `/mutations/:claw_id` | 变异列表 |
| POST | `/mutations/trigger` | 触发变异 |

## 虫族命名谱系

```
Larva(幼虫)     → 移动端
Chrysalis(化蛹)  → 宠物进化系统 ← 本项目
Claw(爪)        → AI Agent 节点
Cerebrate(脑虫)  → 记忆系统
Carapace(甲壳)   → 加密保险库
Hive(蜂巢)      → 多节点协作
Queen(虫后)      → 中枢协调
Overlord(领主)   → 市场/DevClaw
Nydus(虫洞)     → Git 部署
Pheromone(信息素) → 事件总线
Synapse(突触)    → 路由/计费
Spore(孢子)     → 桌面端
Drone(工蜂)     → 数据采集
Forge(锻造)     → 开发工具
```
