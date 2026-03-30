# Q8bot AI量化智能体 (Extractor)

**AI 增强量化交易系统** — A股全市场扫描 + AI二次确认 + 自动交易 + 可视化大屏

> 官网：https://q8bot.com | 基于 StarClaw Claw+Worker 融合架构

## 架构

```
extractor/
├── api/            Go 后端 API (:8097) — 策略管理/风控/结算/Claw AI 中转
├── bridge/         Python Bridge (:8098) — miniQMT 桥接/策略执行/持仓管理
│   ├── qmt_client.py         QMT 连接 (自动检测 C/D/E/F 盘)
│   ├── strategy_executor.py  扫描→打分→AI确认→下单→记录
│   ├── position_manager.py   持仓跟踪/止损/止盈/时间止损
│   └── main.py               API + 自动调度器 + 后台监控
├── strategies/     量化策略库 (主升浪四维打分)
├── web/            React 量化大屏 (部署在 q8bot.com)
└── docs/           设计文档 + 架构设计
```

## 技术栈

| 层级 | 技术 |
|------|------|
| 后端 | Go 1.24 + Gin + GORM + SQLite（默认）/ PostgreSQL |
| 桥接 | Python 3.11 + FastAPI + xtquant (miniQMT SDK) |
| 前端 | React 18 + Vite + TailwindCSS |
| 部署 | Docker Compose / Windows 原生 |

## 核心能力

| 能力 | 说明 |
|------|------|
| **全A股扫描** | 5000+ 只股票四维打分（趋势/动量/量价/波动率） |
| **AI 二次确认** | Claw AI 对候选股做基本面/消息面/板块分析，reject 有风险的票 |
| **自动调度** | 交易时段每 30 分钟自动扫描，无需手动触发 |
| **动态仓位** | 每只票最多用 10% 可用资金，自动计算手数 |
| **持仓管理** | 自动记录买入，防止重复买入，持久化到 JSON |
| **止损/止盈** | 固定止损(-5%)、跟踪止损(高点回落8%)、止盈(+15%)、时间止损(5天) |
| **后台监控** | 每 60 秒检查持仓退出条件，触发自动卖出 |
| **盘前分析** | AI 分析市场方向 + 仓位建议 + 持仓复盘 |
| **日报生成** | 收盘后自动统计今日交易和持仓 |
| **可视化大屏** | https://q8bot.com — 实时账户/持仓/AI决策日志 |

## 快速开始

```bash
# 1. 确保 miniQMT 已登录
# 2. 启动 Python Bridge（自动检测 QMT 路径）
cd bridge && python main.py

# 3. 启动 Go API（配置 Claw AI）
cd api
set EXTRACTOR_CLAW_URL=http://localhost:8081
.\extractor-api.exe

# Bridge 启动后自动运行:
#   - 每 30 分钟自动扫描
#   - 每 60 秒检查持仓止损/止盈
#   - AI 二次确认（通过 Claw）
```

## API 端点

### Bridge (:8098)

| 端点 | 说明 |
|------|------|
| `GET /health` | 健康检查 + QMT 连接状态 |
| `POST /scan` | 手动触发一次扫描 |
| `GET /scan/status` | 最近扫描结果摘要 |
| `GET /positions` | 所有持仓列表 |
| `GET /positions/summary` | 持仓汇总 |
| `POST /positions/check_exits` | 手动检查退出条件 |
| `GET /premarket` | 盘前分析（AI） |
| `GET /report/daily` | 日报 |
| `GET /market/kline` | K 线数据 |
| `GET /market/quote` | 实时行情 |
| `GET /account/info` | 账户资金 |

### Go API (:8097)

| 端点 | 说明 |
|------|------|
| `POST /v1/scan` | 触发扫描（代理到 Bridge） |
| `POST /v1/claw/confirm` | Claw AI 二次确认 |
| `GET /v1/claw/status` | Claw 连接状态 |
| `GET /health` | API 健康检查 |

## 部署

- **本地开发**: 直接 `python main.py` + `.\extractor-api.exe`
- **Server D**: `scripts/persist-extractor.ps1`（注册 Windows 计划任务）
- **Q8bot 官网**: 部署在 121.41.68.79，nginx + Let's Encrypt

## 服务器

| 服务器 | IP | 用途 |
|--------|-----|------|
| **Server D** | 139.224.10.5 | QMT 模拟交易 (test1006) |
| **本地** | localhost | QMT 模拟交易 (27800348) |
| **Q8bot** | 121.41.68.79 | 官网 + 大屏 (q8bot.com) |

## 文档

- [AI 交易架构设计](docs/AI_TRADING_ARCHITECTURE.md)
- [设计文档](docs/DESIGN.md)
- [Server D 部署文档](docs/DEPLOY_SERVER_D.md)
