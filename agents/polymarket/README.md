# 🔮 Polymarket 预测市场分析师

全球最大预测市场的**生产级**实战交易智能体。直连 Gamma API + Polymarket CLOB (Polygon 链)，内置信号引擎、风控管理器和自动交易周期。

## 架构

```
┌─────────────┐     ┌───────────────────────────────────────┐
│  Claw Agent  │────▶│  polymarket/bridge/ (:8099)            │
│  (LLM+Tools) │     │  ├── SignalEngine (60s 信号采集+评分)  │
└─────────────┘     │  ├── RiskManager (30s 止盈止损监控)    │
                     │  ├── TradeCycle (15min 自动交易)       │
                     │  └── PolyClient (Gamma API + CLOB)    │
                     └──────────┬──────────────┬─────────────┘
                                │              │
                       ┌────────▼──────┐  ┌────▼──────┐
                       │ Gamma API     │  │ CLOB API  │
                       │ (市场数据)     │  │ (链上下单) │
                       └───────────────┘  └───────────┘
```

## 八元属性概览

| # | 属性 | 内容 |
|---|------|------|
| ① | **Prompt** | 中英双语，含9工具 API 路径、信号评分公式、7条风控规则、6种操作流程 |
| ② | **Model** | qwen-max, temperature=0.2, max_tokens=4096 |
| ③ | **Tools** | 9个专属 (markets, signals, signals_top, account, account_positions, positions, risk, order, sell) + 2共享 |
| ④ | **Skills** | 3被动 (市场扫描/持仓盈亏/风控) + 3主动 (60s信号采集/30s止盈止损/15min交易周期) |
| ⑤ | **Glands** | 10个 .env 参数 (私钥, Proxy, API Key, DRY_RUN, 单笔/总敞口, TP/SL%, 本金, Admin Key) |
| ⑥ | **Bridge** | polymarket/bridge/main.py (:8099) — 内置引擎，非代理 |
| ⑦ | **Workflow** | 4阶段: 信号采集 → 风控检查 → 交易执行 → 持仓监控 |
| ⑧ | **Marketplace** | $999/quarter, 7天试用 |

## 快速开始

### 1. 配置 .env

编辑 `polymarket/bridge/.env`：

| 变量 | 说明 | 当前值 |
|------|------|--------|
| `POLYMARKET_PRIVATE_KEY` | Polygon 钱包私钥 | `0x...` |
| `POLY_FUNDER` | Proxy 钱包地址 | `0x...` |
| `DRY_RUN` | 模拟模式 | `true` ← **首次必须 true** |
| `MAX_PER_TRADE_USDC` | 单笔上限 | `3` |
| `MAX_TOTAL_EXPOSURE_USDC` | 总敞口上限 | `4` |
| `TAKE_PROFIT_PCT` | 止盈 | `5` |
| `STOP_LOSS_PCT` | 止损 | `3` |

### 2. 启动 Bridge

```bash
cd polymarket
pip install -r bridge/requirements.txt
python bridge/main.py
# → 🏦 Polymarket Bridge starting on :8099
# → Gamma API: connected
# → DRY_RUN=True | Positions=0
# → signal_collector started (interval=60s)
# → position_monitor started (checks every 30s)
# → trade_cycle started (every 15 min)
```

Claw 的 BridgeManager 也会自动启动（`auto_start: true`）。

### 3. 验证

```bash
curl http://127.0.0.1:8099/health      # 健康检查
curl http://127.0.0.1:8099/signals     # 信号排行
curl http://127.0.0.1:8099/markets     # 活跃市场
curl http://127.0.0.1:8099/account     # 链上权益
curl http://127.0.0.1:8099/risk        # 风控状态
```

### 4. 对话使用

```
用户: 扫描市场
Agent: [GET /signals/top] → Top 10 信号表格

用户: 看看持仓
Agent: [GET /account] + [GET /positions] → 权益 + 持仓盈亏

用户: 买 us-forces-enter-iran YES $2
Agent: [GET /risk 检查] → [POST /order {slug, side, usdc_amount}]

用户: 卖掉 xxx
Agent: [POST /sell {slug}] → 展示平仓盈亏
```

## 风控参数

| 参数 | 当前值 | 说明 |
|------|--------|------|
| MAX_PER_TRADE_USDC | $3 | 单笔上限 |
| MAX_TOTAL_EXPOSURE_USDC | $4 | 总敞口上限 |
| TAKE_PROFIT_PCT | 5% | 盈利自动平仓 |
| STOP_LOSS_PCT | 3% | 亏损自动平仓 |
| URGENT_STOP_LOSS_PCT | 25% | 紧急止损 |
| BANKROLL_USD | $100 | 本金 |
| DRY_RUN | true | 模拟模式 |

## 信号评分公式

```
score = (mom1 × 100) + (mom5 × 160) − (spread × 80) + liq_boost
```

- 过滤: 0.08 < YES价格 < 0.92, 价差 ≤ 6%, 流动性 ≥ $50k
- 可交易信号: score > 0.003 🔥

## 文件结构

```
claw/agents/polymarket/          # Claw Agent 定义
├── manifest.yaml                # 八元属性
├── prompt.md / prompt.en.md     # 系统提示
└── README.md

polymarket/bridge/               # 生产 Bridge（被 manifest 引用）
├── main.py                      # FastAPI 主服务 (9 endpoints + 3 background tasks)
├── poly_client.py               # Gamma API + CLOB 客户端
├── signal_engine.py             # 信号评分引擎
├── risk_manager.py              # 风控管理器
├── dashboard.html               # Web 仪表盘
├── .env                         # 配置（私钥/参数）
└── requirements.txt
```
