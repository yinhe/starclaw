# Q8bot 量化交易节点 — 配置手册

> 对应架构文档: `extractor/docs/AI_TRADING_ARCHITECTURE.md` 第四节"Claw+Worker融合节点"

## 唯一配置文件: `config.yaml`

```yaml
qmt:
  path: "D:\\中金财富QMT个人版交易端\\userdata_mini"   # QMT miniQMT数据目录
  session_id: 123456

callback_url: "http://localhost:8097/callback"

accounts:
  - id: "8218033845"       # 你的QMT资金账号
    password: ""
    group: "trend"
    label: "primary-real"
```

**换账号**: 改 `accounts.id` → 重启 Bridge
**换券商**: 改 `qmt.path` 指向新券商的 `userdata_mini` + 改 `accounts.id`
**换电脑**: 改 `qmt.path` + `accounts.id`（QMT路径每台电脑可能不同）

## 启动（3步）

```
1. 打开 QMT 客户端 → 登录
2. python main.py           ← Bridge (:8098)
3. Claw 自动连接 Bridge     ← Claw (:8081)
```

验证: `http://localhost:8098/health` → `qmt_connected: True`

## 自动运行的后台任务

| 任务 | 频率 | 对应架构层 |
|------|------|----------|
| 全A扫描→6维打分→AI确认→风控→下单 | 每30分钟 | Layer 3 个股层 |
| 持仓监控→ATR止损/MA退出/分批止盈 | 每60秒 | Layer 4 风控层 |
| 挂单管理→5分钟未成交自动撤单追单 | 每60秒 | 执行层 |

## 风控参数

| 参数 | 默认值 | 在哪改 |
|------|--------|--------|
| 日亏损暂停 | -2% | `portfolio_risk.py` `daily_loss_limit_pct` |
| 仓位水位(牛/震荡/熊) | 80%/50%/20% | `portfolio_risk.py` `max_exposure_pct()` |
| 最大持仓只数 | 10 | `portfolio_risk.py` `max_holdings` |
| 同板块上限 | 3 | `portfolio_risk.py` `max_per_sector` |
| 每轮买入上限 | 3 | `portfolio_risk.py` `max_buys_per_scan` |
| 追涨保护 | 当日>7%不追 | `strategy_executor.py` `_is_pullback_confirmed()` |

## 策略参数

| 参数 | 默认值 | 在哪改 |
|------|--------|--------|
| 最低打分阈值 | 0.60 | 环境变量 `MIN_SCORE` 或 `strategy_executor.py` |
| 每轮选股上限 | 10 | 环境变量 `TOP_N` |
| 扫描间隔 | 1800秒 | 环境变量 `SCAN_INTERVAL` |
| AI二次确认 | 开启 | 环境变量 `USE_CLAW_CONFIRM` |

## 退出优先级

```
1. ATR硬止损 (2×ATR, 上限5%)
2. 保本止损 (卖出1/3后, 止损=成本价)
3. 分批止盈 (+8%卖1/3, +15%卖1/3)
4. MA趋势退出 (M5/M8小时线死叉)
5. 生命周期退出 (出货阶段全卖)
6. 时间止损 (5-7天无盈利+MA空头)
```

## Q8bot 工具 (20个)

| 类别 | 工具 |
|------|------|
| 面板 | `trading_dashboard` `trading_risk_status` `trading_logs` |
| 分析 | `trading_alpha_analysis` `trading_stats` `trading_history` `trading_backtest` |
| 行情 | `trading_scan` `trading_quote` `trading_kline` `trading_premarket` |
| 持仓 | `trading_positions_list` `trading_check_exits` `trading_daily_report` |
| 交易 | `trading_buy` `trading_sell` `trading_health` |
| 通用 | `web_search` `code` `mcp_trading_bridge` |

## 文件说明

```
extractor/bridge/
├── config.yaml           ← 唯一需要改的配置
├── main.py               # API服务 + 3个后台任务
├── qmt_client.py         # QMT连接 (xtquant SDK)
├── strategy_executor.py  # 扫描→打分→确认→下单
├── position_manager.py   # 持仓监控→退出引擎
├── alpha_engine.py       # 10个核心算法 (从main.py 41K行提炼)
├── portfolio_risk.py     # 组合级风控 (5层保护)
├── trade_tracker.py      # 策略归因 (胜率/盈亏比)
├── backtest.py           # 回测框架
├── positions.json        # 持仓跟踪 (启动时自动与QMT同步)
└── trades.json           # 历史交易记录
```

## 故障排查

| 现象 | 查 |
|------|---|
| QMT未连接 | QMT客户端是否运行？`config.yaml` 的 `qmt.path` 对不对？ |
| 连到模拟端 | `qmt.path` 必须指向**实盘**的 `userdata_mini`，不是模拟端 |
| 没有自动买入 | 正常。查Bridge日志看是否有扫描记录，可能是打分未达标或风控拦截 |
| 端口冲突 | `Get-Process python \| Stop-Process -Force` |
