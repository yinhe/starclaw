# XtQuant API 知识库

> 来源: https://dict.thinktrader.net/nativeApi/xtdata.html
> 整理日期: 2026-04-02

## 1. 行情订阅

### subscribe_whole_quote(code_list, callback=None)
- **用途**: 订阅全推行情数据，返回订阅号
- **参数**:
  - `code_list` - 代码列表，支持市场代码或合约代码
    - 市场代码: `['SH', 'SZ']` — 订阅全市场
    - 合约代码: `['600000.SH', '000001.SZ']`
  - `callback` - 数据推送回调，数据类型为分笔数据
    - 回调定义: `def on_data(datas):` datas格式 `{stock1:data1, stock2:data2, ...}`
- **返回**: 订阅号，>0 成功，-1 失败
- **备注**: 订阅后会**首先返回当前最新的全推数据**
- **⚠️ 重要**: 未订阅的股票调用 `get_full_tick` 会返回**旧数据**！

### subscribe_quote(stock_code, period='', start_time='', end_time='', count=0, callback=None)
- **用途**: 订阅单股行情
- **参数**:
  - `stock_code` - 合约代码 (e.g. '600000.SH')
  - `period` - 周期: tick/1m/5m/1d 等
  - `callback` - 回调函数
- **返回**: 订阅号

### unsubscribe_quote(seq)
- **用途**: 反订阅行情数据
- **参数**: seq - 订阅时返回的订阅号

## 2. 获取行情数据

### get_full_tick(code_list)
- **用途**: 获取全推数据（实时快照）
- **参数**: `code_list` - 代码列表，支持市场代码或合约代码
  - `['SH', 'SZ']` 或 `['600000.SH', '000001.SZ']`
- **返回**: `dict { stock1: data1, stock2: data2, ... }`
- **tick 数据字段**:
  - `lastPrice` - 最新价
  - `lastClose` - 昨收价
  - `open` - 开盘价
  - `high` - 最高价
  - `low` - 最低价
  - `amount` - 成交额
  - `volume` - 成交量（股）
  - `pvolume` - 前一笔成交量
  - `stockStatus` - 股票状态
  - `openInt` - 持仓量
  - `askPrice` - 卖价列表
  - `bidPrice` - 买价列表
  - `askVol` - 卖量列表
  - `bidVol` - 买量列表
  - `time` - 时间
  - `timetag` - 时间戳
- **⚠️ 注意**: 必须先 `subscribe_whole_quote` 才能拿到实时数据！

### get_market_data(field_list, stock_list, period, start_time, end_time, count, dividend_type, fill_data)
- **别名**: `get_market_data_ex` (推荐使用)
- **用途**: 从缓存获取行情数据，主动获取行情的主要接口
- **参数**:
  - `field_list` - 数据字段列表，传空则为全部字段
  - `stock_list` - 合约代码列表
  - `period` - 周期: `1m` / `5m` / `1d` / `tick` 等
  - `start_time` - 起始时间 (string)
  - `end_time` - 结束时间 (string)
  - `count` - 数据个数，>0 时以 end_time 为基准向前取 count 条
  - `dividend_type` - 除权方式
  - `fill_data` - 是否向后填充空缺数据
- **返回**:
  - K线周期 (1m/5m/1d): `dict { field1: DataFrame, ... }` index=stock_list, columns=time_list
  - tick 周期: `dict { stock1: np.ndarray, ... }` 按时间戳升序
- **备注**:
  - 获取 lv2 数据需要数据终端有 lv2 数据权限
  - 时间范围为闭区间
  - **需要先 download_history_data 下载数据到本地**

### get_local_data(field_list, stock_list, period, start_time, end_time, count, dividend_type, fill_data, data_dir)
- **用途**: 从本地数据文件获取行情数据，快速批量获取历史部分

## 3. 下载数据

### download_history_data(stock_code, period, start_time, end_time, incrementally=None)
- **用途**: 补充历史行情数据到本地缓存
- **参数**:
  - `stock_code` - 合约代码 (string)
  - `period` - 周期 (string)
  - `start_time` - 起始时间
  - `end_time` - 结束时间
  - `incrementally` - 是否增量下载 (bool)，True=只补增量，False=全量重下
- **备注**: 单只股票下载

### download_history_data2(stock_list, period, start_time='', end_time='', callback=None)
- **用途**: 批量下载历史数据
- **参数**:
  - `stock_list` - 合约代码**列表** (list)
  - `period` - 周期
  - `start_time` / `end_time` - 时间范围
  - `callback` - 下载完成回调
- **⚠️ 注意**: 没有 `count` 参数！不接受 `count` 关键字参数

### download_sector_data(sector, period)
- **用途**: 按板块批量下载数据
- **参数**:
  - `sector` - 板块名称 (e.g. "沪深A股")
  - `period` - 周期

## 4. 基础行情信息 ⭐

### get_instrument_detail(stock_code, iscomplete=False)
- **用途**: 获取合约基础信息
- **参数**:
  - `stock_code` - 合约代码
  - `iscomplete` - 是否获取全部字段，默认 False
- **返回**: `dict { field1: value1, ... }`，找不到返回 None
- **核心字段** (iscomplete=False):
  - `InstrumentName` - 合约名称
  - `PreClose` - **前收盘价格** ⭐
  - `UpStopPrice` - **当日涨停价** ⭐
  - `DownStopPrice` - **当日跌停价** ⭐
  - `FloatVolume` - 流通股本
  - `TotalVolume` - 总股本
  - `InstrumentStatus` - 合约停牌状态
  - `IsTrading` - 是否可交易
  - `OpenDate` - IPO日期
  - `ExpireDate` - 退市日/到期日
  - `PriceTick` - 最小变价单位
  - `VolumeMultiple` - 合约乘数
- **扩展字段** (iscomplete=True):
  - `ChargeType` - 手续费方式 (0未知, 1按元/手, 2按费率)
  - `secuCategory` - 证券分类
  - `secuAttri` - 证券属性
  - `QualifiedType` - 投资者适当性分类

### get_stock_list_in_sector(sector_name)
- **用途**: 获取板块成分股列表
- **参数**: `sector_name` - 板块名称
  - `"沪深A股"` — 全A股
  - `"上证A股"` / `"深证A股"`
  - `"沪深300"` / `"创业板"`
- **返回**: `list [stock_code1, stock_code2, ...]`

## 5. 财务数据

### get_financial_data(stock_list, table_list, start_time, end_time)
- **用途**: 获取财务数据
- **参数**:
  - `stock_list` - 合约代码列表
  - `table_list` - 财务数据表名列表
  - `start_time` / `end_time` - 时间范围

### download_financial_data(stock_list, table_list, start_time, end_time)
- **用途**: 下载财务数据

## 6. 常用类型说明

### 合约代码格式
- 股票: `000001.SZ` (深圳), `600000.SH` (上海)
- 指数: `000001.SH` (上证指数), `399001.SZ` (深成指)
- ETF: `510300.SH`, `159919.SZ`

### 周期 (period)
- `tick` - 分笔
- `1m` / `5m` / `15m` / `30m` / `60m` - 分钟线
- `1d` - 日线
- `1w` - 周线
- `1mon` - 月线

### 时间格式
- `'20261231'` 或 `'20261231235959'`
- 空字符串 `''` 表示默认

## 7. 运行逻辑

### 数据获取流程
1. **下载**: `download_history_data` / `download_history_data2` — 从服务器拉数据到本地
2. **获取**: `get_market_data` / `get_market_data_ex` — 从本地缓存读取
3. **实时**: `subscribe_whole_quote` → `get_full_tick` — 实时推送

### 请求限制
- 数据接口有频率限制，避免高频循环调用
- 建议使用回调方式而非轮询

---

## 8. 最佳实践（从实际使用中总结）

### 获取实时涨跌停数据的正确方式
```python
# 方法1（推荐）: 用 get_instrument_detail 直接获取涨停价/跌停价/昨收
detail = xtdata.get_instrument_detail("000001.SZ")
pre_close = detail['PreClose']       # 昨收
up_limit = detail['UpStopPrice']     # 涨停价
dn_limit = detail['DownStopPrice']   # 跌停价

# 方法2: subscribe_whole_quote 后用 get_full_tick
xtdata.subscribe_whole_quote(["SH", "SZ"])
ticks = xtdata.get_full_tick(["SH", "SZ"])
# tick['lastPrice'] = 最新价, tick['lastClose'] = 昨收

# ⚠️ 错误方式: 不订阅直接调 get_full_tick → 返回旧数据！
```

### 获取历史K线数据
```python
# 1. 先下载
xtdata.download_history_data("000001.SZ", period="1d", start_time="20260101")
# 2. 再获取
data = xtdata.get_market_data_ex([], ["000001.SZ"], period="1d", count=20)
```

### 批量获取板块数据
```python
codes = xtdata.get_stock_list_in_sector("沪深A股")  # ~5000只
xtdata.download_sector_data("沪深A股", period="1d")  # 批量下载
data = xtdata.get_market_data_ex([], codes, period="1d", count=1)
```

---

# XtQuant.XtTrade 交易模块

> 来源: https://dict.thinktrader.net/nativeApi/xttrader.html
> 整理日期: 2026-04-02

## 9. 系统设置接口

### XtQuantTrader(path, session_id)
- **用途**: 创建 XtQuant API 交易实例
- **参数**:
  - `path` - str MiniQMT 客户端 `userdata_mini` 的完整路径
  - `session_id` - int 会话编号，不同策略需要不同的会话编号
- **返回**: XtQuant API 实例对象

### register_callback(callback)
- **用途**: 注册回调类实例，接收交易主推

### start()
- **用途**: 启动交易线程，准备交易环境

### connect()
- **用途**: 连接 MiniQMT
- **返回**: 0=成功，非0=失败
- **⚠️ 注意**: 一次性连接，断开后不会重连

### stop()
- **用途**: 停止 API 接口

### run_forever()
- **用途**: 阻塞当前线程进入等待状态

### set_relaxed_response_order_enabled(enabled)
- **用途**: 控制主动请求接口是否从专用线程返回（宽松时序）
- **参数**: `enabled` - bool，默认 False
- **备注**: 开启后在 `on_stock_order` 等推送回调中调用同步请求不会卡住

## 10. 操作接口

### subscribe(account)
- **用途**: 订阅账号信息（资金、委托、成交、持仓的变动推送）
- **参数**: `account` - StockAccount
- **返回**: 0=成功，-1=失败

### unsubscribe(account)
- **用途**: 反订阅账号信息

### order_stock(account, stock_code, order_type, order_volume, price_type, price, strategy_name, order_remark)
- **用途**: 同步下单
- **参数**:
  - `account` - StockAccount 资金账号
  - `stock_code` - str 证券代码 (e.g. '600000.SH')
  - `order_type` - int 委托类型 (xtconstant.STOCK_BUY / STOCK_SELL)
  - `order_volume` - int 委托数量（股，必须100的整数倍）
  - `price_type` - int 报价类型 (xtconstant.FIX_PRICE / LATEST_PRICE)
  - `price` - float 委托价格
  - `strategy_name` - str 策略名称
  - `order_remark` - str 委托备注（最大24英文字符）
- **返回**: 订单编号（>0成功，-1失败）

### order_stock_async(account, stock_code, order_type, order_volume, price_type, price, strategy_name, order_remark)
- **用途**: 异步下单
- **返回**: 请求序号 seq，可与 `on_order_stock_async_response` 对应

### cancel_order_stock(account, order_id)
- **用途**: 同步撤单
- **返回**: 0=成功，非0=失败

### cancel_order_stock_async(account, order_id)
- **用途**: 异步撤单
- **返回**: 撤单请求序号（>0成功，-1失败）

### fund_transfer(account, transfer_direction, price)
- **用途**: 资金划拨
- **返回**: (success: bool, msg: str)

### sync_transaction_from_external(operation, data_type, account, deal_list)
- **用途**: 外部交易数据录入
- **参数**:
  - `operation` - str: "UPDATE" / "REPLACE" / "ADD" / "DELETE"
  - `data_type` - str: "DEAL"
  - `deal_list` - list of dict，键名参考官网数据字典

## 11. 查询接口 ⭐

### query_stock_asset(account)
- **用途**: 查询资产
- **返回**: XtAsset 或 None

### query_stock_orders(account, cancelable_only=False)
- **用途**: 查询**当日**所有委托
- **返回**: list[XtOrder] 或 None
- **⚠️ 重要**: 仅返回当日委托，不含历史！

### query_stock_order(account, order_id)
- **用途**: 根据订单编号查询单个委托
- **返回**: XtOrder 或 None

### query_stock_trades(account)
- **用途**: 查询**当日**所有成交
- **返回**: list[XtTrade] 或 None
- **⚠️ 重要**: 仅返回当日成交，不含历史！

### query_stock_positions(account)
- **用途**: 查询当前持仓
- **返回**: list[XtPosition] 或 None

### query_stock_position(account, stock_code)
- **用途**: 查询单只股票持仓
- **返回**: XtPosition 或 None

### query_new_purchase_limit(account)
- **用途**: 查询新股申购额度
- **返回**: dict { "KCB": int, "SH": int, "SZ": int }

### query_ipo_data()
- **用途**: 查询当日新股新债信息
- **返回**: dict { stock_code: { name, type, minPurchaseNum, maxPurchaseNum, purchaseDate, issuePrice } }

### query_account_infos()
- **用途**: 查询所有资金账号
- **返回**: list[XtAccountInfo]

### query_account_status()
- **用途**: 查询所有账号状态
- **返回**: list[XtAccountStatus]

## 12. 通用数据导出/查询 ⭐⭐（获取历史成交的关键接口）

### export_data(account, result_path, data_type, start_time=None, end_time=None, user_param={})
- **用途**: 通用数据导出（**支持历史数据**）
- **参数**:
  - `account` - StockAccount 资金账号
  - `result_path` - str 导出路径（含文件名及.csv后缀）
  - `data_type` - str 数据类型，如 `'deal'`（成交记录）
  - `start_time` - str 开始时间（可缺省）
  - `end_time` - str 结束时间（可缺省）
- **返回**: dict 结果反馈 `{'msg': 'export success'}` 或 `{'error': {...}}`
- **⚠️ 关键**: 这是获取历史成交的唯一方式！query_stock_trades 只返回当日

### query_data(account, result_path, data_type, start_time=None, end_time=None, user_param={})
- **用途**: 通用数据查询（底层调用 export_data 后读取内容，读完删除文件）
- **参数**: 同 export_data
- **返回**: DataFrame 数据（成功）或 dict 错误信息
- **示例**:
```python
acc = StockAccount('27800348')
# 查询历史成交
data = xt_trader.query_data(acc, 'C:\\temp\\deal.csv', 'deal', '20260101', '20260402')
# 返回 DataFrame: account_id, account_Type, stock_code, order_type, traded_price, traded_volume, ...
```

## 13. 数据结构

### XtAsset (资产)
| 属性 | 类型 | 注释 |
|------|------|------|
| account_id | str | 资金账号 |
| cash | float | 可用金额 |
| frozen_cash | float | 冻结金额 |
| market_value | float | 持仓市值 |
| total_asset | float | 总资产 |

### XtOrder (委托)
| 属性 | 类型 | 注释 |
|------|------|------|
| account_id | str | 资金账号 |
| stock_code | str | 证券代码 |
| order_id | int | 订单编号 |
| order_sysid | str | 柜台合同编号 |
| order_time | int | 报单时间 |
| order_type | int | 委托类型 (23=买入, 24=卖出) |
| order_volume | int | 委托数量 |
| price | float | 委托价格 |
| traded_volume | int | 成交数量 |
| traded_price | float | 成交均价 |
| order_status | int | 委托状态 |
| status_msg | str | 状态描述 |
| strategy_name | str | 策略名称 |
| order_remark | str | 委托备注 |
| direction | int | 多空方向（股票不适用） |
| offset_flag | int | 交易操作（48=买入/开仓, 49=卖出/平仓） |

### XtTrade (成交)
| 属性 | 类型 | 注释 |
|------|------|------|
| account_id | str | 资金账号 |
| stock_code | str | 证券代码 |
| order_type | int | 委托类型 |
| traded_id | str | 成交编号 |
| traded_time | int | 成交时间 |
| traded_price | float | 成交均价 |
| traded_volume | int | 成交数量 |
| traded_amount | float | 成交金额 |
| order_id | int | 订单编号 |
| order_sysid | str | 柜台合同编号 |
| strategy_name | str | 策略名称 |
| order_remark | str | 委托备注 |
| offset_flag | int | 交易操作 |

### XtPosition (持仓)
| 属性 | 类型 | 注释 |
|------|------|------|
| account_id | str | 资金账号 |
| stock_code | str | 证券代码 |
| volume | int | 持仓数量 |
| can_use_volume | int | 可用数量 |
| open_price | float | 开仓价 |
| avg_price | float | 成本价 |
| market_value | float | 市值 |
| frozen_volume | int | 冻结数量 |
| on_road_volume | int | 在途股份 |
| yesterday_volume | int | 昨夜拥股 |

## 14. 委托状态枚举 (order_status)
| 枚举 | 值 | 含义 |
|------|------|------|
| ORDER_UNREPORTED | 48 | 未报 |
| ORDER_WAIT_REPORTING | 49 | 待报 |
| ORDER_REPORTED | 50 | 已报 |
| ORDER_REPORTED_CANCEL | 51 | 已报待撤 |
| ORDER_PARTSUCC_CANCEL | 52 | 部成待撤 |
| ORDER_PART_CANCEL | 53 | 部撤 |
| ORDER_CANCELED | 54 | 已撤 |
| ORDER_PART_SUCC | 55 | 部成 |
| ORDER_SUCCEEDED | 56 | 已成 |
| ORDER_JUNK | 57 | 废单 |

## 15. 委托类型 (order_type)
- 股票买入 - `xtconstant.STOCK_BUY` (23)
- 股票卖出 - `xtconstant.STOCK_SELL` (24)

## 16. 回调类

```python
class MyXtQuantTraderCallback(XtQuantTraderCallback):
    def on_disconnected(self):
        """连接断开"""
    def on_account_status(self, status):
        """账号状态推送 - XtAccountStatus"""
    def on_stock_order(self, order):
        """委托信息推送 - XtOrder"""
    def on_stock_trade(self, trade):
        """成交信息推送 - XtTrade"""
    def on_order_error(self, order_error):
        """下单失败推送 - XtOrderError"""
    def on_cancel_error(self, cancel_error):
        """撤单失败推送 - XtCancelError"""
    def on_order_stock_async_response(self, response):
        """异步下单回报 - XtOrderResponse"""
```

## 17. 最佳实践（交易模块）

### 获取历史成交记录（非当日）
```python
# ⚠️ 错误方式: query_stock_trades 只返回当日成交！
trades = xt_trader.query_stock_trades(acc)  # 只有今天的

# ✅ 正确方式: 用 export_data / query_data 导出历史成交
import tempfile, os
tmp = os.path.join(tempfile.gettempdir(), 'qmt_deals.csv')
data = xt_trader.query_data(acc, tmp, 'deal', '20260101', '20260402')
# data 是 DataFrame，包含所有历史成交记录
```

### 完整交易流程
```python
from xtquant.xttrader import XtQuantTrader, XtQuantTraderCallback
from xtquant.xttype import StockAccount
from xtquant import xtconstant

path = 'D:\\迅投极速交易终端\\userdata_mini'
xt_trader = XtQuantTrader(path, 123456)
acc = StockAccount('27800348')

callback = MyXtQuantTraderCallback()
xt_trader.register_callback(callback)
xt_trader.start()
xt_trader.connect()
xt_trader.subscribe(acc)

# 下单
order_id = xt_trader.order_stock(acc, '600000.SH',
    xtconstant.STOCK_BUY, 200, xtconstant.FIX_PRICE, 10.5,
    'strategy1', 'remark')

# 查询当日成交
trades = xt_trader.query_stock_trades(acc)

# 查询历史成交
data = xt_trader.query_data(acc, 'C:\\temp\\deal.csv', 'deal')
```
