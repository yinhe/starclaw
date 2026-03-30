"""
MCP Server endpoint for Python Bridge.
Implements JSON-RPC 2.0 protocol compatible with Claw's MCP client.

Exposes all trading tools as MCP tools so they show as "MCP 外接" in Claw UI.
"""

import json
import logging
from typing import Any, Dict, List

logger = logging.getLogger("mcp_server")

TRADING_TOOLS: List[Dict[str, Any]] = [
    {
        "name": "scan",
        "description": "全A股扫描选股：对5000+只股票进行四维打分（趋势/动量/量价/波动率），返回得分最高的候选股及理由。耗时约2-3分钟。",
        "inputSchema": {
            "type": "object",
            "properties": {
                "min_score": {"type": "number", "description": "最低分数阈值（0.0-1.0，默认0.6）"},
                "top_n": {"type": "integer", "description": "返回前N只（默认10）"},
            },
        },
    },
    {
        "name": "kline",
        "description": "获取个股K线数据：返回日期、开盘、最高、最低、收盘、成交量。",
        "inputSchema": {
            "type": "object",
            "properties": {
                "code": {"type": "string", "description": "股票代码，如 600519.SH 或 000001.SZ"},
                "period": {"type": "string", "description": "周期：1d(日线) 1w(周线) 1m(1分钟)"},
                "count": {"type": "integer", "description": "K线根数（默认60）"},
            },
            "required": ["code"],
        },
    },
    {
        "name": "quote",
        "description": "获取实时行情：当前价、涨跌幅、成交量。仅交易时段（9:30-15:00）可用。",
        "inputSchema": {
            "type": "object",
            "properties": {
                "codes": {"type": "string", "description": "股票代码，多只用逗号分隔"},
            },
            "required": ["codes"],
        },
    },
    {
        "name": "positions",
        "description": "查看所有持仓：代码、买入价、数量、得分、最高价、买入时间。",
        "inputSchema": {"type": "object", "properties": {}},
    },
    {
        "name": "check_exits",
        "description": "检查持仓退出条件（止损-5%/跟踪止盈8%/止盈15%/时间止损5天），触发自动卖出。",
        "inputSchema": {"type": "object", "properties": {}},
    },
    {
        "name": "buy",
        "description": "买入下单：提交限价买入委托到QMT。注意：会产生真实委托。",
        "inputSchema": {
            "type": "object",
            "properties": {
                "account": {"type": "string", "description": "QMT账号（默认27800348）"},
                "code": {"type": "string", "description": "股票代码"},
                "price": {"type": "number", "description": "委托价格"},
                "volume": {"type": "integer", "description": "数量（100的整数倍）"},
            },
            "required": ["account", "code", "price", "volume"],
        },
    },
    {
        "name": "sell",
        "description": "卖出下单：提交限价卖出委托到QMT。注意：会产生真实委托。",
        "inputSchema": {
            "type": "object",
            "properties": {
                "account": {"type": "string", "description": "QMT账号（默认27800348）"},
                "code": {"type": "string", "description": "股票代码"},
                "price": {"type": "number", "description": "委托价格"},
                "volume": {"type": "integer", "description": "数量（100的整数倍）"},
            },
            "required": ["account", "code", "price", "volume"],
        },
    },
    {
        "name": "health",
        "description": "健康检查：Bridge服务状态 + QMT连接状态。",
        "inputSchema": {"type": "object", "properties": {}},
    },
    {
        "name": "premarket",
        "description": "盘前分析：市场方向判断、仓位建议、持仓复盘。",
        "inputSchema": {"type": "object", "properties": {}},
    },
    {
        "name": "daily_report",
        "description": "日报生成：市场环境、持仓数量、总成本、最近扫描结果。",
        "inputSchema": {"type": "object", "properties": {}},
    },
    {
        "name": "account_info",
        "description": "账户信息：总资产、可用资金、冻结资金、持仓市值。",
        "inputSchema": {
            "type": "object",
            "properties": {
                "account": {"type": "string", "description": "QMT账号（默认27800348）"},
            },
        },
    },
]


def handle_jsonrpc(request_body: dict, app_state: dict) -> dict:
    """
    Handle a JSON-RPC 2.0 request for MCP protocol.

    Supported methods:
    - tools/list: return all available trading tools
    - tools/call: execute a trading tool
    """
    jsonrpc = request_body.get("jsonrpc", "2.0")
    req_id = request_body.get("id", 1)
    method = request_body.get("method", "")
    params = request_body.get("params", {})

    try:
        if method == "tools/list":
            return {
                "jsonrpc": jsonrpc,
                "id": req_id,
                "result": {"tools": TRADING_TOOLS},
            }

        elif method == "tools/call":
            tool_name = params.get("name", "")
            arguments = params.get("arguments", {})
            result_text = execute_tool(tool_name, arguments, app_state)
            return {
                "jsonrpc": jsonrpc,
                "id": req_id,
                "result": {
                    "content": [{"type": "text", "text": result_text}],
                    "isError": False,
                },
            }

        else:
            return {
                "jsonrpc": jsonrpc,
                "id": req_id,
                "error": {"code": -32601, "message": f"Method not found: {method}"},
            }

    except Exception as e:
        logger.error(f"MCP error: {e}")
        return {
            "jsonrpc": jsonrpc,
            "id": req_id,
            "result": {
                "content": [{"type": "text", "text": str(e)}],
                "isError": True,
            },
        }


def execute_tool(name: str, args: dict, app_state: dict) -> str:
    """Execute a trading tool directly (no HTTP self-call to avoid deadlock)."""
    import math

    def json_safe(obj):
        if obj is None or isinstance(obj, (str, bool, int)):
            return obj
        if isinstance(obj, float):
            return obj if math.isfinite(obj) else None
        if isinstance(obj, dict):
            return {str(k): json_safe(v) for k, v in obj.items()}
        if isinstance(obj, (list, tuple)):
            return [json_safe(v) for v in obj]
        if hasattr(obj, "item"):
            try:
                return json_safe(obj.item())
            except Exception:
                return str(obj)
        return str(obj)

    try:
        # Access the live global state from __main__ module (the running process)
        import __main__ as bridge_main

        qmt = getattr(bridge_main, "qmt", None)
        accounts = getattr(bridge_main, "accounts", None)

        if name == "health":
            connected = qmt.is_connected() if qmt else False
            return json.dumps({"status": "ok", "qmt_connected": connected})

        # For tools needing executor, try to get it; for simple reads, use position_manager directly
        executor = bridge_main._executor
        if executor is None and qmt and bridge_main.accounts:
            try:
                executor = bridge_main._get_executor()
            except Exception as e:
                logger.warning(f"MCP: executor init failed: {e}")

        # positions can be read directly from file even without executor
        if name == "positions":
            from position_manager import PositionManager, POSITIONS_FILE
            import os
            if os.path.exists(POSITIONS_FILE):
                with open(POSITIONS_FILE, "r", encoding="utf-8") as f:
                    data = json.load(f)
                return json.dumps(json_safe(data), ensure_ascii=False, indent=2)
            return json.dumps([])

        elif name == "check_exits":
            if executor is None:
                return json.dumps({"error": "Executor not ready"})
            exits = executor.positions.check_exits()
            if not exits:
                return json.dumps({"exits": 0, "message": "no exit conditions triggered"})
            results = executor.positions.execute_exits(exits)
            return json.dumps(json_safe({"exits": len(results), "details": results}), ensure_ascii=False, indent=2)

        elif name == "health":
            connected = qmt.is_connected() if qmt else False
            return json.dumps({"status": "ok", "qmt_connected": connected})

        # Tools below require QMT connection
        elif name in ("kline", "quote", "account_info", "buy", "sell"):
            if not qmt or not qmt.is_connected():
                return json.dumps({"error": "QMT未连接。请确认miniQMT客户端已启动并登录。当前为非交易时段QMT可能已断开。"})

            if name == "kline":
                code = args.get("code", "000001.SZ")
                period = args.get("period", "1d")
                count = args.get("count", 60)
                data = qmt.get_kline(code, period, count)
                return json.dumps(json_safe(data), ensure_ascii=False, indent=2)

            elif name == "quote":
                codes = [c.strip() for c in args.get("codes", "000001.SZ").split(",")]
                data = qmt.get_quotes(codes)
                return json.dumps(json_safe(data), ensure_ascii=False, indent=2)

            elif name == "account_info":
                account = args.get("account", "27800348")
                data = qmt.get_account_info(account)
                return json.dumps(json_safe(data), ensure_ascii=False, indent=2)

            elif name == "buy":
                order_id = qmt.submit_order(
                    account=args.get("account", "27800348"),
                    code=args["code"], direction="buy",
                    price=args["price"], volume=args["volume"], order_type="limit")
                return json.dumps({"order_id": str(order_id), "status": "submitted"})

            elif name == "sell":
                order_id = qmt.submit_order(
                    account=args.get("account", "27800348"),
                    code=args["code"], direction="sell",
                    price=args["price"], volume=args["volume"], order_type="limit")
                return json.dumps({"order_id": str(order_id), "status": "submitted"})

        elif name == "daily_report":
            positions = executor.positions.get_positions_list()
            summary = executor.positions.summary()
            scan = bridge_main._last_scan_result or {}
            from datetime import datetime
            report = {
                "date": datetime.now().strftime("%Y-%m-%d"),
                "market_env": executor._market_env,
                "positions_held": len(positions),
                "positions_cost": summary.get("total_cost", 0),
                "last_scan": {"scanned": scan.get("scanned", 0), "candidates": scan.get("candidates", 0), "orders": scan.get("orders", 0)},
                "positions": positions,
            }
            return json.dumps(json_safe(report), ensure_ascii=False, indent=2)

        elif name == "premarket":
            env = executor.detect_market_env()
            held = executor.positions.get_positions_list()
            from datetime import datetime
            return json.dumps(json_safe({
                "time": datetime.now().strftime("%Y-%m-%d %H:%M"),
                "market_env": env,
                "positions_count": len(held),
                "positions": held,
            }), ensure_ascii=False, indent=2)

        elif name == "scan":
            result = executor.scan_once()
            return json.dumps(json_safe(result), ensure_ascii=False, indent=2)

        else:
            return json.dumps({"error": f"Unknown tool: {name}"})

    except Exception as e:
        logger.error(f"MCP execute_tool error: {e}")
        return json.dumps({"error": str(e)})
