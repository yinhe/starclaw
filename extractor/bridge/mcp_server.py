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
    """Execute a trading tool and return result as text."""
    import math
    import httpx

    bridge_url = "http://localhost:8098"

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
        if name == "scan":
            resp = httpx.post(f"{bridge_url}/scan", json=args, timeout=300)
        elif name == "kline":
            code = args.get("code", "000001.SZ")
            period = args.get("period", "1d")
            count = args.get("count", 60)
            resp = httpx.get(f"{bridge_url}/market/kline?code={code}&period={period}&count={count}", timeout=30)
        elif name == "quote":
            codes = args.get("codes", "000001.SZ")
            resp = httpx.get(f"{bridge_url}/market/quote?codes={codes}", timeout=10)
        elif name == "positions":
            resp = httpx.get(f"{bridge_url}/positions", timeout=10)
        elif name == "check_exits":
            resp = httpx.post(f"{bridge_url}/positions/check_exits", timeout=30)
        elif name == "buy":
            body = {"account": args.get("account", "27800348"), "code": args["code"],
                    "direction": "buy", "price": args["price"], "volume": args["volume"], "order_type": "limit"}
            resp = httpx.post(f"{bridge_url}/order/submit", json=body, timeout=10)
        elif name == "sell":
            body = {"account": args.get("account", "27800348"), "code": args["code"],
                    "direction": "sell", "price": args["price"], "volume": args["volume"], "order_type": "limit"}
            resp = httpx.post(f"{bridge_url}/order/submit", json=body, timeout=10)
        elif name == "health":
            resp = httpx.get(f"{bridge_url}/health", timeout=5)
        elif name == "premarket":
            resp = httpx.get(f"{bridge_url}/premarket", timeout=60)
        elif name == "daily_report":
            resp = httpx.get(f"{bridge_url}/report/daily", timeout=10)
        elif name == "account_info":
            account = args.get("account", "27800348")
            resp = httpx.get(f"{bridge_url}/account/info?account={account}", timeout=10)
        else:
            return json.dumps({"error": f"Unknown tool: {name}"})

        data = resp.json()
        return json.dumps(json_safe(data), ensure_ascii=False, indent=2)

    except Exception as e:
        return json.dumps({"error": str(e)})
