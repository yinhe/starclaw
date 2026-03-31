package handler

// q8botAgents returns Q8bot paid agents for the marketplace.
// Extracted from extractor/scripts/setup_trading_agent_full.py (tested locally).
func q8botAgents() []officialAgent {
	return []officialAgent{
		{
			Name: "Q8bot 量化分析师「麒博」",
			Description: "A股AI量化交易分析师 — 5000+全市场扫描、四维打分、AI风险排查、自动止损止盈。覆盖主板、创业板、科创板。\n\n" +
				"包含：\n" +
				"- 5个被动技能（个股诊断、持仓复盘、风险排查、全市场扫描、市场解读）\n" +
				"- 4个主动技能（盘前分析、自动扫描、持仓监控、日终复盘）\n" +
				"- MCP Trading Bridge（miniQMT行情+交易接口）\n" +
				"- 全日交易工作流（8节点完整链路）\n\n" +
				"适用场景：A股量化投资、个人理财、投资顾问辅助\n" +
				"要求：需要连接 miniQMT 客户端（中金财富/国金/中信建投等券商QMT）",
			Icon:     "TrendingUp",
			Tags:     "量化交易,A股,AI分析,自动交易,风控,主升浪,Q8bot",
			Category: "finance",
			Prompt:   q8botSystemPrompt,
			Tools:    `["trading_scan","trading_kline","trading_quote","trading_positions_list","trading_check_exits","trading_buy","trading_sell","trading_health","trading_premarket","trading_daily_report","web_search","code","mcp_trading_bridge"]`,
			// ¥299 一次性买断
			Pricing:    "one_time",
			PriceCents: 29900,
			DemoURL:    "https://q8bot.com",
			Featured:   true,
			ModelName:  "qwen-max",

			// ── Full installation bundle ──
			Skills:     q8botSkills(),
			MCPServers: q8botMCPServers(),
			Workflows:  q8botWorkflows(),
			Plugins:    q8botPlugins(),
		},
	}
}

// ── 被动技能 ×5 + 主动技能 ×4 ──

func q8botSkills() []AgentSkillSpec {
	return []AgentSkillSpec{
		// 被动技能 (passive)
		{
			Name:    "个股诊断",
			Version: "1.0.0",
			Spec: `{"trigger":"passive","description":"分析单只股票的趋势、支撑压力位、量价关系，给出买卖建议",` +
				`"tools":["trading_kline","trading_quote","web_search"],` +
				`"example_triggers":["帮我看看600519","分析一下平安银行","000001.SZ怎么样"]}`,
		},
		{
			Name:    "持仓复盘",
			Version: "1.0.0",
			Spec: `{"trigger":"passive","description":"查看所有持仓，分析每只股票的盈亏和操作建议",` +
				`"tools":["trading_positions_list","trading_kline"],` +
				`"example_triggers":["看看我的持仓","今天持仓怎么样","哪些票该卖了"]}`,
		},
		{
			Name:    "风险排查",
			Version: "1.0.0",
			Spec: `{"trigger":"passive","description":"检查所有持仓的止损/止盈/时间止损条件，发现风险立即卖出",` +
				`"tools":["trading_check_exits","trading_sell"],` +
				`"example_triggers":["检查风险","有没有该止损的","帮我排查一下"]}`,
		},
		{
			Name:    "全市场扫描",
			Version: "1.0.0",
			Spec: `{"trigger":"passive","description":"扫描5000+只A股，用四维打分模型筛选主升浪候选，AI二次确认后下单",` +
				`"tools":["trading_scan","trading_buy"],` +
				`"example_triggers":["帮我选股","扫描一下","有什么好票"]}`,
		},
		{
			Name:    "市场解读",
			Version: "1.0.0",
			Spec: `{"trigger":"passive","description":"分析当前市场环境（牛市/震荡/熊市），给出仓位水位建议",` +
				`"tools":["trading_premarket","trading_kline","web_search"],` +
				`"example_triggers":["今天大盘怎么样","市场方向如何","该满仓还是减仓"]}`,
		},
		// 主动技能 (proactive)
		{
			Name:    "盘前分析",
			Version: "1.0.0",
			Spec: `{"trigger":"proactive","schedule":"0 30 8 * * 1-5",` +
				`"description":"每个交易日08:30自动分析全球市场、外盘走势，输出今日交易方向和仓位建议",` +
				`"tools":["trading_premarket","web_search"],"auto_execute":true,"notify":true}`,
		},
		{
			Name:    "自动扫描选股",
			Version: "1.0.0",
			Spec: `{"trigger":"proactive","schedule":"0 */30 9-11,13-14 * * 1-5",` +
				`"description":"交易时段每30分钟自动扫描全A股，筛选主升浪候选，AI确认后自动下单",` +
				`"tools":["trading_scan","trading_buy"],"auto_execute":true,"notify":true}`,
		},
		{
			Name:    "持仓实时监控",
			Version: "1.0.0",
			Spec: `{"trigger":"proactive","schedule":"0 * 9-11,13-14 * * 1-5",` +
				`"description":"交易时段每分钟检查持仓止损/止盈条件，触发条件自动卖出",` +
				`"tools":["trading_check_exits","trading_sell"],"auto_execute":true,"notify":true}`,
		},
		{
			Name:    "日终复盘",
			Version: "1.0.0",
			Spec: `{"trigger":"proactive","schedule":"0 30 15 * * 1-5",` +
				`"description":"每个交易日15:30自动生成日报：今日买卖记录、盈亏统计、持仓分析、明日建议",` +
				`"tools":["trading_daily_report","trading_positions_list"],"auto_execute":true,"notify":true}`,
		},
	}
}

// ── MCP Trading Bridge ──

func q8botMCPServers() []AgentMCPSpec {
	return []AgentMCPSpec{
		{
			Name:        "Q8bot Trading Bridge",
			BaseURL:     "http://localhost:8098",
			Description: "Q8bot 量化交易桥接层 — 连接 miniQMT，提供行情获取、下单执行、持仓管理等能力",
			Tools: `[{"name":"mcp_trading_scan","description":"全A股扫描选股"},` +
				`{"name":"mcp_trading_kline","description":"获取K线数据"},` +
				`{"name":"mcp_trading_quote","description":"获取实时行情"},` +
				`{"name":"mcp_trading_buy","description":"买入下单"},` +
				`{"name":"mcp_trading_sell","description":"卖出下单"},` +
				`{"name":"mcp_trading_positions","description":"查询持仓"},` +
				`{"name":"mcp_trading_health","description":"健康检查"},` +
				`{"name":"mcp_trading_premarket","description":"盘前分析"},` +
				`{"name":"mcp_trading_report","description":"日报生成"}]`,
		},
	}
}

// ── 全日交易工作流 ──

func q8botWorkflows() []AgentWorkflowSpec {
	return []AgentWorkflowSpec{
		{
			Name:        "Q8bot 全日交易工作流",
			Description: "完整的A股交易日工作流：盘前分析 → 方向判断 → 扫描选股 → AI确认 → 执行下单 → 持仓监控 → 日终复盘",
			Definition: `{"nodes":[` +
				`{"id":"start","type":"start","data":{"label":"交易日开始"}},` +
				`{"id":"premarket","type":"llm","data":{"label":"盘前分析","model":"qwen-max","prompt":"分析今日A股市场方向，给出仓位建议。使用 trading_premarket 工具获取数据。","tools":["trading_premarket","web_search"]}},` +
				`{"id":"condition_trade","type":"condition","data":{"label":"是否交易？","condition":"output.includes('满仓') || output.includes('七成') || output.includes('半仓')"}},` +
				`{"id":"scan","type":"llm","data":{"label":"扫描选股","model":"qwen-max","prompt":"执行全A股扫描，筛选主升浪候选。使用 trading_scan 工具。","tools":["trading_scan"]}},` +
				`{"id":"confirm","type":"llm","data":{"label":"AI确认","model":"qwen-max","prompt":"对扫描出的候选股逐一分析基本面/消息面/技术面/板块，给出confirm/reject/reduce决策。","tools":["trading_kline","web_search"]}},` +
				`{"id":"execute","type":"llm","data":{"label":"执行下单","model":"qwen-max","prompt":"对确认通过的股票执行买入。使用 trading_buy 工具。","tools":["trading_buy"]}},` +
				`{"id":"monitor","type":"llm","data":{"label":"持仓监控","model":"qwen-max","prompt":"检查所有持仓的止损/止盈条件。使用 trading_check_exits 工具。","tools":["trading_check_exits","trading_sell"]}},` +
				`{"id":"report","type":"llm","data":{"label":"日终复盘","model":"qwen-max","prompt":"生成今日交易日报。使用 trading_daily_report 工具。","tools":["trading_daily_report","trading_positions_list"]}},` +
				`{"id":"skip","type":"llm","data":{"label":"观望","model":"qwen-max","prompt":"今日市场不适合交易，进入观望模式。只监控现有持仓的止损条件。","tools":["trading_check_exits"]}},` +
				`{"id":"end","type":"end","data":{"label":"交易日结束"}}` +
				`],"edges":[` +
				`{"source":"start","target":"premarket"},` +
				`{"source":"premarket","target":"condition_trade"},` +
				`{"source":"condition_trade","target":"scan","label":"是"},` +
				`{"source":"condition_trade","target":"skip","label":"否"},` +
				`{"source":"scan","target":"confirm"},` +
				`{"source":"confirm","target":"execute"},` +
				`{"source":"execute","target":"monitor"},` +
				`{"source":"monitor","target":"report"},` +
				`{"source":"skip","target":"report"},` +
				`{"source":"report","target":"end"}` +
				`]}`,
		},
	}
}

// ── 11 个 Trading 工具插件 (从 claw/api/plugins/trading_*.json 提取) ──

func q8botPlugins() []AgentPluginSpec {
	return []AgentPluginSpec{
		{Name: "trading_scan", Spec: `{"name":"trading_scan","description":"Trigger a full A-share stock scan: quantitative scoring across ~3000+ stocks, returning top candidates with scores, reasons, and order details. Takes 2-3 minutes.","version":"1.0.0","author":"Extractor","parameters":{"type":"object","properties":{"min_score":{"type":"number","description":"Minimum score threshold (0.0-1.0, default 0.6)"},"top_n":{"type":"integer","description":"Number of top candidates to return (default 10)"}}},"endpoint":{"url":"http://localhost:8098/scan","method":"POST"},"metadata":{"category":"trading","timeout":300}}`},
		{Name: "trading_kline", Spec: `{"name":"trading_kline","description":"Get K-line (candlestick) data for a stock. Returns OHLCV bars: date, open, high, low, close, volume, amount.","version":"1.0.0","author":"Extractor","parameters":{"type":"object","properties":{"code":{"type":"string","description":"Stock code with exchange suffix, e.g. 600519.SH or 000001.SZ"},"period":{"type":"string","description":"K-line period: 1d (daily), 1w (weekly), 1m (1-minute)"},"count":{"type":"integer","description":"Number of bars to return (default 60)"}},"required":["code"]},"endpoint":{"url":"http://localhost:8098/market/kline?code={{code}}&period={{period}}&count={{count}}","method":"GET"},"metadata":{"category":"trading"}}`},
		{Name: "trading_quote", Spec: `{"name":"trading_quote","description":"Get real-time quote for one or more stocks. Returns price, open, high, low, volume. Only works during trading hours (9:30-15:00 weekdays).","version":"1.0.0","author":"Extractor","parameters":{"type":"object","properties":{"codes":{"type":"string","description":"Comma-separated stock codes, e.g. 600519.SH,000001.SZ"}},"required":["codes"]},"endpoint":{"url":"http://localhost:8098/market/quote?codes={{codes}}","method":"GET"},"metadata":{"category":"trading"}}`},
		{Name: "trading_positions_list", Spec: `{"name":"trading_positions_list","description":"List all currently held stock positions. Returns code, account, entry price, volume, score, entry time, and highest price since entry for each position.","version":"1.0.0","author":"Extractor","parameters":{"type":"object","properties":{}},"endpoint":{"url":"http://localhost:8098/positions","method":"GET"},"metadata":{"category":"trading"}}`},
		{Name: "trading_positions", Spec: `{"name":"trading_positions","description":"Query current stock positions for a QMT account. Returns code, name, volume, available volume, cost price, market price, and floating P&L for each holding.","version":"1.0.0","author":"Extractor","parameters":{"type":"object","properties":{"account":{"type":"string","description":"QMT account ID, e.g. test1006"}},"required":["account"]},"endpoint":{"url":"http://localhost:8098/account/positions?account={{account}}","method":"GET"},"metadata":{"category":"trading"}}`},
		{Name: "trading_check_exits", Spec: `{"name":"trading_check_exits","description":"Check all held positions for exit conditions (stop-loss, trailing stop, take-profit, time-stop) and automatically execute sell orders for any triggered exits. Returns the number of exits executed and their details.","version":"1.0.0","author":"Extractor","parameters":{"type":"object","properties":{}},"endpoint":{"url":"http://localhost:8098/positions/check_exits","method":"POST"},"metadata":{"category":"trading"}}`},
		{Name: "trading_buy", Spec: `{"name":"trading_buy","description":"Submit a buy order for a stock via QMT. Returns the QMT order ID. Only works during trading hours. Use with caution — this places real orders on the connected account.","version":"1.0.0","author":"Extractor","parameters":{"type":"object","properties":{"account":{"type":"string","description":"QMT account ID, e.g. test1006"},"code":{"type":"string","description":"Stock code with exchange suffix, e.g. 600519.SH"},"price":{"type":"number","description":"Limit price for the order"},"volume":{"type":"integer","description":"Number of shares to buy (must be multiple of 100)"}},"required":["account","code","price","volume"]},"endpoint":{"url":"http://localhost:8098/order/submit","method":"POST"},"body_template":"{\"account\":\"{{account}}\",\"code\":\"{{code}}\",\"direction\":\"buy\",\"price\":{{price}},\"volume\":{{volume}},\"order_type\":\"limit\"}","metadata":{"category":"trading","dangerous":true}}`},
		{Name: "trading_sell", Spec: `{"name":"trading_sell","description":"Submit a sell order for a stock via QMT. Returns the QMT order ID. Only works during trading hours. Use with caution — this places real orders on the connected account.","version":"1.0.0","author":"Extractor","parameters":{"type":"object","properties":{"account":{"type":"string","description":"QMT account ID, e.g. test1006"},"code":{"type":"string","description":"Stock code with exchange suffix, e.g. 600519.SH"},"price":{"type":"number","description":"Limit price for the order"},"volume":{"type":"integer","description":"Number of shares to sell (must be multiple of 100)"}},"required":["account","code","price","volume"]},"endpoint":{"url":"http://localhost:8098/order/submit","method":"POST"},"body_template":"{\"account\":\"{{account}}\",\"code\":\"{{code}}\",\"direction\":\"sell\",\"price\":{{price}},\"volume\":{{volume}},\"order_type\":\"limit\"}","metadata":{"category":"trading","dangerous":true}}`},
		{Name: "trading_health", Spec: `{"name":"trading_health","description":"Check the health status of the Python Bridge and QMT connection. Returns whether QMT is connected and the bridge is responsive.","version":"1.0.0","author":"Extractor","parameters":{"type":"object","properties":{}},"endpoint":{"url":"http://localhost:8098/health","method":"GET"},"metadata":{"category":"trading"}}`},
		{Name: "trading_premarket", Spec: `{"name":"trading_premarket","description":"Run premarket analysis before market open. Uses AI to analyze market direction, suggest position sizing (full/70%/half/light/empty), highlight key sectors, and review existing holdings. Best called between 8:00-9:25.","version":"1.0.0","author":"Extractor","parameters":{"type":"object","properties":{}},"endpoint":{"url":"http://localhost:8098/premarket","method":"GET"},"metadata":{"category":"trading"}}`},
		{Name: "trading_daily_report", Spec: `{"name":"trading_daily_report","description":"Generate daily trading report: market environment, positions held, total cost, last scan results. Best called after market close (15:00+).","version":"1.0.0","author":"Extractor","parameters":{"type":"object","properties":{}},"endpoint":{"url":"http://localhost:8098/report/daily","method":"GET"},"metadata":{"category":"trading"}}`},
	}
}

const q8botSystemPrompt = `你是 Q8bot AI量化智能体的核心交易分析师「麒博」。

## 身份定位
你是一位拥有10年A股实战经验的资深量化分析师，精通技术分析、基本面研究和消息面解读。你为投资人提供专业、冷静、数据驱动的投资建议。

## 核心能力

### 被动技能（用户提问时触发）
1. **个股诊断** — 用户问"帮我看看600519"，你会调用 trading_kline 获取K线数据，分析趋势、支撑位、压力位
2. **持仓复盘** — 用户问"看看我的持仓"，你会调用 trading_positions_list 获取所有持仓，逐只分析盈亏和操作建议
3. **风险排查** — 用户问"有没有该卖的"，你会调用 trading_check_exits 检查所有止损/止盈条件
4. **市场解读** — 用户问"今天大盘怎么样"，你会分析市场环境并给出仓位建议
5. **选股推荐** — 用户问"帮我选几只票"，你会调用 trading_scan 执行全A股扫描

### 主动技能（定时自动执行）
1. **盘前分析** — 每日08:30自动分析全球市场、隔夜外盘、A50期指，输出今日方向判断
2. **自动扫描** — 交易时段每30分钟自动扫描5000+只A股，筛选主升浪候选
3. **持仓监控** — 交易时段每分钟检查持仓止损/止盈条件，触发自动卖出
4. **日终复盘** — 每日15:30自动生成交易日报，统计盈亏，优化参数

## 决策框架
对于每只候选股票，你会从四个维度分析：
1. **基本面快检** — 近期财报、ST风险、重大公告
2. **消息面扫描** — 政策影响、行业利空、高管减持
3. **技术面验证** — 均线多头排列、放量突破、支撑位
4. **板块共振** — 所属行业/概念板块是否活跃

## 输出规范
- 分析候选股时，用JSON格式输出：
  [{"code":"600519.SH","action":"confirm/reject/reduce","confidence":0.85,"risk_flags":["风险1"],"suggestion":"建议"}]
- action说明：confirm=确认买入, reject=拒绝(有风险), reduce=减半仓位
- 中国A股颜色惯例：红色=涨/盈利, 绿色=跌/亏损

## 风控红线
- 单只股票不超过总资金10%
- 固定止损-5%，跟踪止盈从高点回落8%
- 持仓超5天不盈利自动清仓
- AI置信度低于60%不执行
- 大盘极端行情（跌幅>3%）暂停买入

## 性格
- 冷静理性，不受情绪影响
- 言简意赅，结论先行
- 风险意识第一，盈利其次
- 每个建议都附带理由`
