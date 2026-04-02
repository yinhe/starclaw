# Extractor（萃取器）— 量化交易引擎

> 虫族气矿采集器 — 从金融市场中萃取财富，虫群经济命脉

## 一、定位

Extractor 是 StarClaw 虫族的经济引擎。通过 A股量化交易 + 海外预测市场为虫群提供持续资金来源。
交易利润自动转化为星能（Star Energy），注入 Queen 经济循环，实现虫群自我供血。

**独特竞争力：量化打分 × AI 二次确认（Claw Agent）**

传统量化：纯技术指标选股 → 执行
Extractor：技术指标选股 → **Claw AI 基本面/消息面/板块分析** → 确认后执行

## 二、核心架构

```
Server D: 139.224.10.5 (Windows, 量化专用)

  ┌─────────────┐  xtquant   ┌──────────────────────┐
  │ QMT Client   │◄──────────►│  Python Bridge :8098  │
  │ (迅投桌面端)  │   SDK      │  FastAPI              │
  │ 10个测试账号   │            │  ├─ 行情获取           │
  └─────────────┘            │  ├─ 下单/撤单          │
                              │  ├─ 账户查询           │
                              │  └─ 策略执行           │
                              └──────────┬───────────┘
                                          │ HTTP REST
                                          ▼
  ┌──────────────────────────────────────────────────────┐
  │  Extractor API (Go :8097)                             │
  │                                                        │
  │  ┌─────────────────────────────────────────────────┐  │
  │  │  策略管理 │ 交易记录 │ 风控引擎 │ 收益统计       │  │
  │  │  回测框架 │ 日终结算 │ 系统监控 │ Bridge 回调   │  │
  │  └─────────────────────────────────────────────────┘  │
  │                                                        │
  │  ┌─────────────────────────────────────────────────┐  │
  │  │  ⭐ Claw AI 二次确认模块                         │  │
  │  │  POST /v1/claw/confirm → Claw /v1/chat/completions│ │
  │  │  量化候选 → AI 风控分析 → confirm/reject/reduce   │  │
  │  └─────────────────────────────────────────────────┘  │
  └──────┬─────────────────┬─────────────────┬────────────┘
         │                 │                 │
  ┌──────┴──────┐  ┌──────┴──────┐  ┌──────┴──────┐
  │ PostgreSQL   │  │ Claw Agent  │  │ Queen       │
  │ extractor_db │  │ AI 二次确认  │  │ 星能结算     │
  └─────────────┘  └─────────────┘  └─────────────┘
```

## 三、Claw AI 二次确认流程

这是 Extractor 区别于所有传统量化系统的核心创新。

```
量化打分 (Python)                     Claw AI (LLM)
────────────────                     ──────────────
1. 获取全A股日线数据                   
2. score_main_rise_candidate()        
   四维打分 [0,1]:                    
   - 趋势+位置 (50%)                  
   - 近期涨速 (20%)                   
   - 当日量价 (30%)                   
   - ATR 波动率惩罚                   
3. scan_and_rank() → Top N           
         │                            
         ▼                            
4. POST /v1/claw/confirm              
   发送候选列表 + 打分详情    ────────→ 5. Claw AI 分析:
                                         ├─ 基本面快检 (财报/ST)
                                         ├─ 消息面扫描 (利空/减持)
                                         ├─ 技术面验证 (走势一致性)
                                         └─ 板块共振 (行业活跃度)
                                      
                              ←──────── 6. 返回 JSON:
                                         {action: confirm/reject/reduce,
                                          confidence, risk_flags}
         │                            
         ▼                            
7. 过滤 reject → 保留 confirm/reduce  
8. reduce → 减半仓位                  
9. 通过 Bridge → QMT 下单            
```

### Claw 确认 Prompt 设计

```
你是 StarClaw 量化交易系统的 AI 风控分析师。

当前市场环境: {牛市/震荡/熊市}

量化策略(主升浪趋势)筛选出以下候选股票，请你进行二次确认分析：

1. 600519.SH | 综合分=0.85 | 趋势=✅多头 | 涨幅=5.2% | 量比=2.1

请对每只股票分析: 基本面/消息面/技术面/板块共振
回复 JSON: [{code, action, confidence, risk_flags, suggestion}]
```

### 降级策略
- Claw 不可用 → 全部 confirm（降级放行），记录 risk_flag
- AI 回复解析失败 → 全部 confirm，confidence=0.5
- 超时 60s → 降级放行

## 四、主升浪打分算法

**来源：** 从生产 QMT 策略 main.py (41789行) 第 24475 行 `score_main_rise_candidate()` 完整提取。

**独立实现：** `strategies/trend_main_wave.py`，去除 QMT ContextInfo 依赖，纯数据输入。

### 四维评分 [0, 1]

| 维度 | 权重 | 指标 | 满分条件 |
|------|------|------|---------|
| **趋势+位置** | 35%+15% | MA20/MA30 多头排列 + 60日价格分位 | 多头+均线上升+分位0.4~0.85 |
| **近期涨速** | 20% | 3日涨幅 / 10日涨幅弹性比 | ≥2.0 |
| **当日量价** | 15%+15% | 涨幅分档 + 5日量比 | 涨5-9.5%突破 + 量比≥2.5 |
| **波动率** | ×乘数 | ATR14 归一化 | ≤0.08满分, ≥0.18打6折 |
| **预备形态** | +0.10 | 缩量整理+均线托底+筹码健康 | 小体量≥60%+缩量70% |

### 市场环境自适应

| 环境 | 最低分阈值 | 预判vs突破权重 | 特殊规则 |
|------|-----------|--------------|---------|
| 牛市 | 0.70 | 0.4/0.6 偏突破 | — |
| 震荡 | 0.70 | 0.4/0.6 | — |
| 熊市 | 0.50 | 0.6/0.4 偏预判 | 保底分0.55 |
| 极端熊 | 0.50 | 0.6/0.4 | 保底分0.55 |

## 五、技术决策

| 项目 | 决策 | 理由 |
|------|------|------|
| 数据库 | PostgreSQL 15 | 窗口函数/JSONB/高并发事务，金融时序最优 |
| 后端 | Go 1.24 (Gin + GORM) | 与虫族一致，单二进制，35 个 API 端点 |
| 桥接 | Python 3.11 (FastAPI) | xtquant SDK 仅 Python，Pandas/NumPy 生态 |
| Go↔Python | HTTP REST | 简单可调试，延迟 1-5ms |
| Go↔Claw | HTTP (/v1/chat/completions) | OpenAI 兼容接口，60s 超时 |
| 前端 | React + Vite + TailwindCSS | [E6] 与虫族一致 |
| 域名 | quant.starclaw.net | nginx 反代 139.224.10.5:8097 |

## 六、测试账户 (QMT)

| 组 | 账户 | 策略 | 资金配比 | 风控 |
|----|------|------|---------|------|
| **稳健A** | test1006~1008 | 网格/ETF轮动 | 各20% | 日回撤-2% |
| **趋势B** | test1009~1011 | 主升浪/动量 | 各12% | 日回撤-3% |
| **AI C** | test1012~1013 | LLM信号+情绪 | 各8% | 日回撤-5% |
| **套利D** | test1014 | 配对/跨品种 | 5% | 市场中性 |
| **实验E** | test1015 | 新策略沙箱 | 3% | 严格隔离 |

密码统一：888888
QMT 下载：https://marketing.ciccwm.com/ntg/file/XtItClient_x64_ciccwm_QMT_test.exe
监控面板：https://trade.q8bot.com/monitor.html

## 七、风控三级熔断

### L1 策略级
- 单笔止损: -2% 自动平仓
- 策略日亏: -3% 暂停该策略
- 单股仓位: ≤ 账户 25%

### L2 账户级
- 账户日回撤超阈值(按组) → 暂停全部策略
- 可用资金 < 20% → 禁止开新仓
- 连续亏损 3 天 → 减仓 50% + 告警

### L3 系统级
- 全账户日回撤 > -5% → **全部熔断** (撤回所有挂单, 禁止开仓)
- Python Bridge 断连 > 30s → 撤回所有未成交挂单
- QMT 连接异常 → 紧急告警 + 冻结策略

## 八、星能经济闭环

```
日终结算 (每交易日 15:30 后):

  净利润 = Σ(已实现盈亏) - 手续费 - 印花税

  ├─ 60% → 再投资 (留在账户, 复利增长)
  ├─ 20% → 星能池 (POST queen/internal/credits/inject)
  ├─ 10% → 投资人分红 (InvestorPool.Distribute)
  └─ 10% → 运营储备金

  换算: ¥1 = 100⚡ = 1,000,000 内部单位

  亏损日: 不分配, 从储备金覆盖
```

## 九、Go ↔ Python 通信协议

### Go → Python (命令)
```
POST /order/submit     { account, code, direction, price, volume }
POST /order/cancel     { account, order_id }
GET  /account/info     ?account=test1006
GET  /account/positions ?account=test1006
POST /strategy/start   { strategy_id, account, params }
POST /strategy/stop    { strategy_id }
GET  /market/quote     ?codes=600519,000858
GET  /market/kline     ?code=600519&period=1d&count=100
```

### Python → Go (回调)
```
POST /callback/order_filled    { order_id, fill_price, fill_volume, timestamp }
POST /callback/market_tick     { code, price, volume, timestamp }
POST /callback/risk_alert      { account, type, message }
POST /callback/strategy_signal { strategy_id, signal, data }
```

### Go → Claw (AI 确认)
```
POST /v1/claw/confirm    { candidates: [...], market_env, model }
GET  /v1/claw/status     → { connected: bool, url }
```

## 十、API 端点总览 (35+)

| 分组 | 端点 | 说明 |
|------|------|------|
| **策略** | GET/POST/PUT/DELETE /v1/strategies, POST start/stop | 策略 CRUD + 生命周期 |
| **交易** | GET/POST /v1/orders, POST cancel, GET trades/positions | 委托/成交/持仓 |
| **风控** | GET/POST/PUT/DELETE /v1/risk/rules, GET alerts | 规则+告警 |
| **收益** | GET /v1/pnl/daily, summary, curve | P&L 统计 |
| **回测** | POST/GET /v1/backtest | 回测任务管理 |
| **监控** | GET /v1/accounts, monitor/overview | 账户+系统健康 |
| **结算** | POST /v1/settlement/run, GET history | 日终结算 |
| **Claw** | POST /v1/claw/confirm, GET status | AI 二次确认 |
| **回调** | POST /callback/order_filled, tick, alert, signal | Bridge→Go |

## 十一、Pheromone 事件

| 事件 | 触发时机 |
|------|---------|
| `extractor.trade.executed` | 每笔成交 |
| `extractor.pnl.daily` | 日终结算 |
| `extractor.risk.alert` | 风控触发 |
| `extractor.risk.circuit_break` | 熔断触发 |
| `extractor.settlement.done` | 星能分配完成 |
| `extractor.claw.confirmed` | Claw AI 确认通过 |
| `extractor.claw.rejected` | Claw AI 拒绝买入 |

## 十二、目录结构

```
extractor/                          # 🏦 萃取器
├── main.py                         # 生产 QMT 策略参考 (41789行, 保留不动)
├── api/                            # Go 后端 :8097
│   ├── cmd/server/main.go
│   ├── internal/
│   │   ├── handler/                # 9 个 handler 文件
│   │   │   ├── router.go           # 35+ 路由
│   │   │   ├── strategy.go         # 策略 CRUD
│   │   │   ├── trade.go            # 交易
│   │   │   ├── risk.go             # 风控
│   │   │   ├── pnl.go              # 收益
│   │   │   ├── backtest.go         # 回测
│   │   │   ├── monitor.go          # 监控
│   │   │   ├── settlement.go       # 结算
│   │   │   ├── callback.go         # Bridge 回调
│   │   │   └── claw_confirm.go     # Claw AI 确认
│   │   ├── model/                  # 13 张 PostgreSQL 表
│   │   ├── engine/
│   │   │   ├── scheduler.go        # 策略调度器
│   │   │   ├── risk_ctrl.go        # 三级风控引擎
│   │   │   ├── settlement.go       # 日终结算
│   │   │   └── claw_client.go      # Claw HTTP 客户端
│   │   └── bridge/client.go        # Python Bridge 客户端
│   ├── Dockerfile
│   └── go.mod
├── bridge/                         # Python FastAPI :8098
│   ├── main.py                     # FastAPI 服务入口
│   ├── qmt_client.py               # xtquant SDK 封装
│   ├── account_manager.py          # 多账户管理
│   ├── config.yaml                 # 10 个测试账户
│   └── requirements.txt
├── strategies/                     # 策略库 (Python)
│   ├── base.py                     # 策略基类 + Signal
│   └── trend_main_wave.py          # ⭐ 主升浪策略 (585行)
│       ├── score_main_rise_candidate()  # 四维打分
│       ├── scan_and_rank()              # 批量扫描排序
│       ├── build_claw_confirmation_prompt()  # Claw prompt
│       └── parse_claw_confirmation()    # AI 回复解析
├── predict/                        # [E9] 海外预测市场
├── web/                            # [E6] React 仪表盘
├── docker-compose.yml              # PostgreSQL + Go API
├── .env.example
├── docs/DESIGN.md                  # 本文档
└── README.md
```

## 十三、实施计划

| 阶段 | 内容 | 状态 |
|------|------|:----:|
| **E0** | 项目骨架 + PG + Go/Python 初始化 | ✅ |
| **E1** | 策略提取 + Claw AI 集成模块 | ✅ |
| **E2** | miniQMT xtquant 真实对接 + 调度循环 | ✅ 2026-03-31 |
| **E3** | 端到端测试: 扫描→打分→Claw确认→下单 | ✅ 2026-03-31 |
| **E4** | 风控系统完整实现 | ✅ 2026-03-31 (portfolio_risk.py 5层风控) |
| **E5** | 回测框架 | ✅ 2026-03-31 (backtest.py) |
| **E6** | React 量化仪表盘 | 🟡 Q8bot对话替代，20个trading工具 |
| **E7** | 星能闭环: 客户月度账单→Synapse支付/线下→Queen星能注入 | ✅ 2026-04-01 (client.go+billing.go, 15个API端点) |
| **E8** | AI Agent 5层分析体系 | ✅ 2026-04-01 (L1宏观+L2板块+L3情绪+L4个股研报+L5 Qwen LLM Master) |
| **E9** | Polymarket 海外预测市场 | ⏳ |
| **E10** | Nydus hook + nginx + 监控 + 上线 | ⏳ |

### 2026-03-31 补充实现 (Bridge直连模式)

> 注意: 当前实现跳过了Go API层(:8097)，由Bridge(:8098)直接承担策略+执行+风控。
> 这是架构v1.5（Bridge直连），后续再升级到DESIGN.md设计的Go API分层架构。

| 新增文件 | 功能 |
|----------|------|
| `bridge/alpha_engine.py` | 10个核心算法（从main.py 41K行提炼：层叠信号/6维打分/Kelly/预突破/生命周期/筹码/板块/市场环境/分批建仓/动态止盈） |
| `bridge/portfolio_risk.py` | 组合级风控（日亏损-2%暂停/仓位水位/最大10只/板块集中度≤3/每轮≤3笔/回踩确认） |
| `bridge/trade_tracker.py` | 策略归因（胜率/盈亏比/利润因子/期望值/按退出原因归因/按信号强度归因） |
| `bridge/backtest.py` | 回测框架（夏普比率/最大回撤/参数验证） |
| `claw/api/plugins/trading_*.json` | 18个Claw交易工具插件 |

| 重大改动 | 说明 |
|----------|------|
| 打分升级 4维→6维 | +MACD确认+RSI超买保护+相对强度RS |
| 退出引擎重写 | MA(M5/M8)趋势退出+ATR硬止损+分批止盈(+8%/+15%)+lifecycle出货退出 |
| 买入链路增强 | 预突破扫描+回踩确认(拒绝追涨>7%)+Kelly仓位 |
| 挂单管理 | 5分钟未成交自动撤单追单 |

### 2026-04-01 E8 AI Agent 5层分析体系

| 层级 | 功能 | API端点 | 文件 |
|------|------|---------|------|
| **L1 宏观** | 大盘方向+仓位建议 | `GET /trading/macro` | `alpha_engine.py::analyze_macro()` |
| **L2 板块** | 板块轮动+热度排名 | `GET /trading/sectors` | `alpha_engine.py::analyze_sectors()` |
| **L3 情绪** | 恐贪指标+涨跌停+量能 | `GET /trading/sentiment` | `alpha_engine.py::analyze_sentiment()` |
| **L4 个股** | 技术面+相对强度+风险 | `GET /trading/research?code=&cost_price=` | `alpha_engine.py::research_stock()` |
| **L5 Master** | Qwen LLM综合研判 | `GET /trading/master` | `alpha_engine.py::master_analysis()` |

辅助端点:
- `GET /trading/research/portfolio` — 批量研报全部持仓
- `GET /trading/premarket_v2` — 盘前综合报告 (L1+L2)

## 十四、环境变量

```bash
# PostgreSQL
EXTRACTOR_DATABASE_DSN=host=localhost user=extractor password=ExtractorDb!2026 dbname=extractor_db port=5432 sslmode=disable

# Claw AI (二次确认)
EXTRACTOR_CLAW_URL=http://localhost:8080      # Claw API 地址
EXTRACTOR_CLAW_API_KEY=                        # Claw 认证 key
EXTRACTOR_CLAW_MODEL=qwen-max                 # 默认 LLM 模型

# Queen (星能结算)
QUEEN_URL=https://api.starclaw.net
QUEEN_TOKEN=sc2026-xK9mWqL3vNpR7tYhBjF5sDcEaGiUoZ4

# Qwen LLM (Master Analysis)
QWEN_API_KEY=                                  # DashScope API Key
QWEN_MODEL=qwen-plus                          # 模型 (qwen-plus/qwen-max)
QWEN_BASE_URL=https://dashscope.aliyuncs.com/compatible-mode/v1

# Python Bridge
EXTRACTOR_BRIDGE_URL=http://localhost:8098
BRIDGE_PORT=8098

# QMT
QMT_PATH=C:\国金QMT交易端模拟\userdata_mini
QMT_SESSION_ID=123456
```

## 十五、服务器拓扑

```
Server A: starclaw.me      (43.106.138.214) — 🦞 Claw + 🐝 Hive
Server B: star-ai.net      (47.103.51.32)   — ⛽ Synapse
Server C: starclaw.net     (43.106.158.26)  — 👑 Queen + 👁️ Overlord + 🕳️ Nydus
Server D: quant.starclaw.net (139.224.10.5)  — 🏦 Extractor [NEW]
```
