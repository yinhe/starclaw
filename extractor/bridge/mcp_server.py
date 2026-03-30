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
        "description": "Full A-share market scan: quantitative 4-factor scoring across 5000+ stocks, returns top candidates with scores and reasons. Takes 2-3 minutes.",
        "inputSchema": {
            "type": "object",
            "properties": {
                "min_score": {"type": "number", "description": "Minimum score threshold (0.0-1.0, default 0.6)"},
                "top_n": {"type": "integer", "description": "Number of top candidates (default 10)"},
            },
        },
    },
    {
        "name": "kline",
        "description": "Get K-line (candlestick) data for a stock. Returns OHLCV bars.",
        "inputSchema": {
            "type": "object",
            "properties": {
                "code": {"type": "string", "description": "Stock code e.g. 600519.SH"},
                "period": {"type": "string", "description": "Period: 1d, 1w, 1m"},
                "count": {"type": "integer", "description": "Number of bars (default 60)"},
            },
            "required": ["code"],
        },
    },
    {
        "name": "quote",
        "description": "Get real-time quote for stocks. Only works during trading hours (9:30-15:00).",
        "inputSchema": {
            "type": "object",
            "properties": {
                "codes": {"type": "string", "description": "Comma-separated stock codes"},
            },
            "required": ["codes"],
        },
    },
    {
        "name": "positions",
        "description": "List all currently held stock positions with entry price, volume, score, and highest price.",
        "inputSchema": {"type": "object", "properties": {}},
    },
    {
        "name": "check_exits",
        "description": "Check all positions for exit conditions (stop-loss, trailing stop, take-profit, time-stop) and auto-execute sells.",
        "inputSchema": {"type": "object", "properties": {}},
    },
    {
        "name": "buy",
        "description": "Submit a buy order. Use with caution - places real orders.",
        "inputSchema": {
            "type": "object",
            "properties": {
                "account": {"type": "string", "description": "QMT account ID"},
                "code": {"type": "string", "description": "Stock code"},
                "price": {"type": "number", "description": "Limit price"},
                "volume": {"type": "integer", "description": "Shares (multiple of 100)"},
            },
            "required": ["account", "code", "price", "volume"],
        },
    },
    {
        "name": "sell",
        "description": "Submit a sell order. Use with caution - places real orders.",
        "inputSchema": {
            "type": "object",
            "properties": {
                "account": {"type": "string", "description": "QMT account ID"},
                "code": {"type": "string", "description": "Stock code"},
                "price": {"type": "number", "description": "Limit price"},
                "volume": {"type": "integer", "description": "Shares (multiple of 100)"},
            },
            "required": ["account", "code", "price", "volume"],
        },
    },
    {
        "name": "health",
        "description": "Check Bridge health and QMT connection status.",
        "inputSchema": {"type": "object", "properties": {}},
    },
    {
        "name": "premarket",
        "description": "Run premarket analysis: market direction, position sizing suggestion, position review.",
        "inputSchema": {"type": "object", "properties": {}},
    },
    {
        "name": "daily_report",
        "description": "Generate daily trading report: market env, positions, P&L summary.",
        "inputSchema": {"type": "object", "properties": {}},
    },
    {
        "name": "account_info",
        "description": "Get account balance: total assets, available cash, market value.",
        "inputSchema": {
            "type": "object",
            "properties": {
                "account": {"type": "string", "description": "QMT account ID (default: 27800348)"},
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
