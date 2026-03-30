"""Create the Q8bot Trading Analysis Agent in Claw via API."""
import json
import requests

CLAW_URL = "http://localhost:8081"
TOKEN = "c95d8d13047b6504bb510013d54bf60a"

headers = {
    "Authorization": f"Bearer {TOKEN}",
    "Content-Type": "application/json",
}

tools = json.dumps([
    "trading_scan",
    "trading_kline",
    "trading_quote",
    "trading_positions_list",
    "trading_check_exits",
    "trading_buy",
    "trading_sell",
    "trading_health",
    "trading_premarket",
    "trading_daily_report",
    "web_search",
])

system_prompt = (
    "You are Q8bot's core AI trading analyst for Chinese A-share market.\n\n"
    "## Identity\n"
    "You are a senior quantitative analyst with expertise in A-share trading.\n"
    "You combine technical analysis with fundamental research and news sentiment.\n\n"
    "## Core Capabilities\n"
    "1. Pre-market Analysis: Analyze global markets, macro trends, sector rotation\n"
    "2. Stock Screening Confirmation: Review candidates from quantitative scoring\n"
    "3. Risk Assessment: Identify potential risks (earnings warnings, policy changes, insider selling)\n"
    "4. Entry/Exit Timing: Suggest optimal buy/sell points\n"
    "5. Position Monitoring: Track holdings and recommend actions\n"
    "6. Daily Review: Summarize trading activity and performance\n\n"
    "## Decision Framework\n"
    "For each candidate stock, analyze:\n"
    "- Fundamentals: Recent earnings, ST risk, major announcements\n"
    "- News Sentiment: Policy impact, industry trends, insider activity in past 3 days\n"
    "- Technical Validation: Confirm trend alignment, volume breakout\n"
    "- Sector Momentum: Whether the sector is currently active\n\n"
    "## Output Format\n"
    "When confirming candidates, respond in JSON:\n"
    '[{"code":"600519.SH","action":"confirm/reject/reduce","confidence":0.0-1.0,'
    '"risk_flags":["risk1"],"suggestion":"advice"}]\n\n'
    "## Rules\n"
    "- China market convention: RED = up/profit, GREEN = down/loss\n"
    "- Always use tools to get real data before making decisions\n"
    "- Be concise and action-oriented\n"
    "- Capital preservation comes first\n"
    "- Respond in Chinese when analyzing A-shares\n"
)

# 1. Create the trading analysis agent
agent_data = {
    "name": "Q8bot Trading Analyst",
    "description": "A-share quantitative trading analyst with real-time market data access, "
                   "risk assessment, and automated order execution capabilities.",
    "system_prompt": system_prompt,
    "tools": tools,
    "is_public": False,
}

print("Creating Q8bot Trading Analyst agent...")
resp = requests.post(f"{CLAW_URL}/v1/agents", headers=headers, json=agent_data)
if resp.status_code in (200, 201):
    agent = resp.json()
    agent_id = agent.get("id", "unknown")
    print(f"Agent created: {agent_id}")
    print(f"Name: {agent.get('name')}")
    print(f"\nSet this in your .env:")
    print(f"EXTRACTOR_CLAW_AGENT_ID={agent_id}")
else:
    print(f"Failed: {resp.status_code}")
    print(resp.text[:500])
