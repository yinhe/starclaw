//go:build private

package v1

// This file contains the Q8bot 量化分析师 marketplace template.
// It is EXCLUDED from OSS sync (proprietary trading strategy).
// Build tag `private` is OFF by default — public OSS builds (default `go build`)
// pick the stub in builtin_agents_q8bot_stub.go and skip this file entirely.
// Internal production builds opt-in via `-tags private` (see claw/api/Dockerfile).

import (
	"encoding/json"
	"log"

	"github.com/yinhe/starclaw/internal/model"
	"gorm.io/gorm"
)

// seedQ8botMarketplaceTemplate creates/updates the Q8bot 麒博 marketplace template
// with the FULL rich data: complete system prompt, 11 trading tools, 9 skills,
// MCP Trading Bridge, and full trading workflow — matching Queen's seed_q8bot.go.
func seedQ8botMarketplaceTemplate(db *gorm.DB, ownerID string) {
	db.Where("name = ?", "Q8bot Quant Analyst").Delete(&model.AgentTemplate{})

	q8botName := `Q8bot 量化分析师「麒博」`
	q8botDesc := "A股AI量化交易分析师 — 5000+全市场扫描、四维打分、AI风险排查、自动止损止盈。覆盖主板、创业板、科创板。\n\n" +
		"包含：\n" +
		"- 5个被动技能（个股诊断、持仓复盘、风险排查、全市场扫描、市场解读）\n" +
		"- 4个主动技能（盘前分析、自动扫描、持仓监控、日终复盘）\n" +
		"- MCP Trading Bridge（miniQMT行情+交易接口）\n" +
		"- 全日交易工作流（8节点完整链路）\n\n" +
		"适用场景：A股量化投资、个人理财、投资顾问辅助\n" +
		"要求：需要连接 miniQMT 客户端\n" +
		"官网：q8bot.com"
	q8botTags := `["量化","A股","AI交易","风控","自动选股","QMT"]`

	// Build rich config with full bundle data
	cfg := map[string]interface{}{
		"system_prompt": q8botFullSystemPrompt,
		"tools":         `["trading_dashboard","trading_alpha_analysis","trading_risk_status","trading_stats","trading_history","trading_backtest","trading_macro_analysis","trading_sector_rotation","trading_scan","trading_kline","trading_quote","trading_positions_list","trading_check_exits","trading_buy","trading_sell","trading_health","trading_premarket","trading_daily_report","trading_logs","web_search","code","mcp_trading_bridge"]`,
		"model_name":    "qwen-max",
		"temperature":   0.3,
		"max_tokens":    4096,
		"pricing": map[string]interface{}{
			"type": "subscription", "price": 299900, "period": "quarter",
			"currency": "CNY", "display": "¥2,999/季度",
		},
		"bundle": map[string]interface{}{
			"skills": []map[string]string{
				{"name": "个股诊断", "spec": `{"trigger":"passive","description":"分析单只股票的趋势、支撑压力位、量价关系，给出买卖建议","tools":["trading_kline","trading_quote","web_search"],"example_triggers":["帮我看看600519","分析一下平安银行","000001.SZ怎么样"]}`},
				{"name": "持仓复盘", "spec": `{"trigger":"passive","description":"查看所有持仓，分析每只股票的盈亏和操作建议","tools":["trading_positions_list","trading_kline"],"example_triggers":["看看我的持仓","今天持仓怎么样","哪些票该卖了"]}`},
				{"name": "风险排查", "spec": `{"trigger":"passive","description":"检查所有持仓的止损/止盈/时间止损条件，发现风险立即卖出","tools":["trading_check_exits","trading_sell"],"example_triggers":["检查风险","有没有该止损的","帮我排查一下"]}`},
				{"name": "全市场扫描", "spec": `{"trigger":"passive","description":"扫描5000+只A股，用六维打分模型筛选主升浪候选，AI二次确认后下单","tools":["trading_scan","trading_buy"],"example_triggers":["帮我选股","扫描一下","有什么好票"]}`},
				{"name": "市场解读", "spec": `{"trigger":"passive","description":"分析当前市场环境（牛市/震荡/熊市），给出仓位水位建议","tools":["trading_macro_analysis","trading_sector_rotation","web_search"],"example_triggers":["今天大盘怎么样","市场方向如何","该满仓还是减仓"]}`},
				{"name": "盘前分析", "spec": `{"trigger":"proactive","schedule":"0 30 8 * * 1-5","description":"每个交易日08:30自动分析全球市场、外盘走势，输出今日方向判断","tools":["trading_premarket","trading_macro_analysis","web_search"],"auto_execute":true,"notify":true}`},
				{"name": "自动扫描选股", "spec": `{"trigger":"proactive","schedule":"0 */30 9-11,13-14 * * 1-5","description":"交易时段每30分钟自动扫描全A股，筛选主升浪候选，AI确认后自动下单","tools":["trading_scan","trading_buy"],"auto_execute":true,"notify":true}`},
				{"name": "持仓实时监控", "spec": `{"trigger":"proactive","schedule":"0 * 9-11,13-14 * * 1-5","description":"交易时段每分钟检查持仓止损/止盈条件，触发条件自动卖出","tools":["trading_check_exits","trading_sell"],"auto_execute":true,"notify":true}`},
				{"name": "日终复盘", "spec": `{"trigger":"proactive","schedule":"0 30 15 * * 1-5","description":"每个交易日15:30自动生成日报：今日买卖记录、盈亏统计、持仓分析、明日建议","tools":["trading_daily_report","trading_positions_list","trading_stats"],"auto_execute":true,"notify":true}`},
			},
			"mcp_servers": []map[string]string{
				{"name": "Q8bot Trading Bridge", "base_url": "http://localhost:8098", "description": "Q8bot 量化交易桥接层 — 连接 miniQMT，提供行情获取、下单执行、持仓管理等能力"},
			},
			"workflows": []map[string]string{
				{"name": "Q8bot 全日交易工作流", "description": "完整的A股交易日工作流：盘前分析 → 方向判断 → 扫描选股 → AI确认 → 执行下单 → 持仓监控 → 日终复盘"},
			},
			"glands": []map[string]interface{}{
				{"key": "qmt_account", "label": "QMT 交易账号", "category": "credential", "encrypted": true, "required": true, "help_text": "miniQMT 资金账号（如 27800348）", "sort_order": 1},
				{"key": "qmt_password", "label": "QMT 密码", "category": "credential", "encrypted": true, "required": true, "help_text": "miniQMT 登录密码", "sort_order": 2},
				{"key": "qmt_path", "label": "QMT 安装路径", "category": "endpoint", "encrypted": false, "required": true, "help_text": "QMT 客户端路径（如 D:\\中金财富QMT个人版模拟交易端）", "sort_order": 3},
				{"key": "bridge_url", "label": "Trading Bridge 地址", "category": "endpoint", "encrypted": false, "required": false, "help_text": "默认 http://localhost:8098", "sort_order": 4},
				{"key": "risk_threshold", "label": "AI 置信度阈值", "category": "threshold", "encrypted": false, "required": false, "help_text": "低于此值不执行交易（默认 0.6）", "sort_order": 5},
				{"key": "max_position_pct", "label": "单只最大仓位 %", "category": "threshold", "encrypted": false, "required": false, "help_text": "单只股票占总资金比例上限（默认 10）", "sort_order": 6},
				{"key": "stop_loss_pct", "label": "固定止损 %", "category": "threshold", "encrypted": false, "required": false, "help_text": "亏损达到此比例自动卖出（默认 -5）", "sort_order": 7},
				{"key": "trailing_stop_pct", "label": "跟踪止盈回撤 %", "category": "threshold", "encrypted": false, "required": false, "help_text": "从最高点回撤此比例自动卖出（默认 8）", "sort_order": 8},
				{"key": "auto_trade", "label": "自动交易开关", "category": "toggle", "encrypted": false, "required": false, "help_text": "关闭后仅推荐不执行（true/false）", "sort_order": 9},
			},
		},
	}
	cfgBytes, _ := json.Marshal(cfg)
	q8botConfig := string(cfgBytes)

	var existing model.AgentTemplate
	if err := db.Where("name = ?", q8botName).First(&existing).Error; err != nil {
		tpl := model.AgentTemplate{
			AuthorID:    ownerID,
			Name:        q8botName,
			Description: q8botDesc,
			Category:    "finance",
			Tags:        q8botTags,
			Config:      q8botConfig,
			Icon:        "📈",
			Featured:    true,
			IsBuiltin:   true,
		}
		db.Create(&tpl)
		log.Printf("[Seed] Created Q8bot marketplace template: %s", q8botName)
	} else {
		db.Model(&existing).Updates(map[string]interface{}{
			"description": q8botDesc,
			"tags":        q8botTags,
			"config":      q8botConfig,
			"icon":        "📈",
			"featured":    true,
			"is_builtin":  true,
		})
	}
}

// q8botFullSystemPrompt is the complete Q8bot system prompt.
const q8botFullSystemPrompt = `你是 Q8bot AI量化智能体的核心交易分析师「麒博」。

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
