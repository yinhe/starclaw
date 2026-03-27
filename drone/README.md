# Drone 🐝 工蜂 — 数据采集 + 同化微服务

> 从外部平台采集 Agent/Skill，经虫茧同化为 StarClaw 原生格式，批量注入市场。

## 架构

```
collectors/ → 采集原始数据 → cocoon/ → 同化转化 → api/ → 导入 Queen 市场
```

### 三层同化（虫茧 Cocoon）

| Level | 名称 | 速度 | 适用 | 比例 |
|-------|------|------|------|------|
| L1 | 自动变态 (Auto-Morph) | <1秒/个 | 格式清晰的来源 | 95% |
| L2 | LLM 进化 (LLM-Evolve) | ~5秒/个 | 热门/高价值 Agent | 4% |
| L3 | DevClaw 深度同化 | ~2分钟/个 | 顶级复杂 Agent | 1% |

### 数据源

| 来源 | 类型 | 预计数量 | 同化难度 |
|------|------|---------|---------|
| ClawHub.ai | API (MIT开源) | 500+ | ⭐ 直通 |
| SkillHub.club | API | 7000+ | ⭐⭐ |
| Coze | API + 爬虫 | 50000+ | ⭐⭐⭐ |
| Dify | API | 5000+ | ⭐⭐ |
| GPTs Store | Scrapling 反爬 | 100000+ | ⭐⭐⭐⭐ |
| GitHub awesome-gpts | HTTP | 2000+ | ⭐⭐⭐ |
| FlowGPT | Scrapling | 100000+ | ⭐⭐⭐ |
| LangChain Hub | API | 3000+ | ⭐⭐ |

## 快速开始

```bash
# 1. 启动服务
docker compose up -d

# 2. 手动触发采集
curl -X POST http://localhost:8110/harvest/clawhub
curl -X POST http://localhost:8110/harvest/skillhub

# 3. 查看采集状态
curl http://localhost:8110/stats
```

## 目录结构

```
drone/
├── api/                Go API（调度+导入）
├── collectors/         Python 采集器（每源一个）
├── cocoon/             🧬 虫茧同化引擎
├── configs/            数据源+调度配置
├── docs/               设计文档
└── docker-compose.yml
```
