# StarClaw Agent Developer Guide

> Build, test, and publish your own AI agent in minutes.

## Quick Start (3 Steps)

### Step 1: Create Directory

```bash
mkdir claw/agents/my_agent
```

### Step 2: Write manifest.yaml

```yaml
id: my_agent
type: solo
name:
  zh: "我的智能体"
  en: "My Agent"
description:
  zh: "一句话描述你的智能体"
  en: "One-line description of your agent"
icon: "🤖"
category: assistant
tags: ["demo"]
version: "0.1.0"
status: draft

author:
  id: your_name
  name: { zh: "你的名字", en: "Your Name" }

prompt_file: prompt.md

model:
  name: qwen-max
  temperature: 0.3
  max_tokens: 4096

tools:
  shared: [web_search]
```

### Step 3: Write Prompt

```bash
echo "你是一个AI助手。" > claw/agents/my_agent/prompt.md
```

Restart Claw → your agent appears in the agent list automatically.

---

## Manifest Reference

### Metadata (Required)

```yaml
id: my_agent                    # Unique ID (= directory name)
type: solo                      # solo | team

name:
  zh: "中文名"
  en: "English Name"
description:
  zh: "中文描述"
  en: "English description"

icon: "🤖"                     # Emoji or icon name
category: assistant             # See categories below
tags: ["tag1", "tag2"]
version: "1.0.0"               # Semver
status: stable                  # draft | beta | stable | deprecated
visibility: public              # public | private | unlisted

author:
  id: author_id
  name: { zh: "作者", en: "Author" }
  email: author@example.com     # Optional
  url: "https://..."            # Optional
  verified: false               # Set by Queen admin
```

**Categories**: `assistant`, `coding`, `creative`, `finance`, `data`, `research`, `support`, `marketing`, `content`, `ecommerce`, `ops`, `sales`, `development`

### Prompt (Required)

```yaml
prompt_file: prompt.md          # Default language (zh)
prompt_files:                   # Additional languages
  en: prompt.en.md
  ja: prompt.ja.md
```

Prompt files are Markdown. The entire file content becomes the agent's `system_prompt`.

### Model (Required)

```yaml
model:
  name: qwen-max                # Model name
  temperature: 0.3              # 0.0 - 1.0
  max_tokens: 4096              # Max output tokens
```

### Tools

```yaml
tools:
  # Tools provided by this agent's Bridge
  own:
    - scan                      # Registered as my_agent:scan
    - buy                       # Registered as my_agent:buy

  # Claw built-in tools (no namespace)
  shared:
    - web_search
    - code
    - browser

  # Tools from other agents (explicit cross-reference)
  foreign:
    - q8bot:macro               # Requires q8bot agent to be installed
```

If you only use shared tools (no Bridge), you can use the short form:

```yaml
tools:
  shared: [web_search, code]
```

### Skills

```yaml
skills:
  # Passive skill — triggered when user asks
  - name: { zh: "市场分析", en: "Market Analysis" }
    trigger: passive
    description:
      zh: "分析市场数据"
      en: "Analyze market data"
    tools: [my_agent:scan, web_search]
    example_triggers:
      - "分析市场"
      - "analyze market"

  # Proactive instinct — runs on schedule
  - name: { zh: "定时扫描", en: "Scheduled Scan" }
    trigger: proactive
    schedule: "0 */30 9-17 * * 1-5"   # cron6 format
    description:
      zh: "每30分钟自动扫描"
      en: "Auto-scan every 30 minutes"
    tools: [my_agent:scan]
    auto_execute: true
    notify: true                       # Send notification on completion
```

### Glands (Runtime Config)

```yaml
glands:
  - key: api_key
    label: { zh: "API 密钥", en: "API Key" }
    category: credential               # credential | threshold | toggle | endpoint | general
    encrypted: true                    # Encrypted at rest
    required: true
    help_text: { zh: "你的API密钥", en: "Your API key" }
    sort_order: 1

  - key: max_position
    label: { zh: "最大仓位", en: "Max Position" }
    category: threshold
    encrypted: false
    required: false
    sort_order: 2

  - key: auto_trade
    label: { zh: "自动交易", en: "Auto Trade" }
    category: toggle
    encrypted: false
    sort_order: 3
```

### Bridge (Optional)

```yaml
bridge:
  type: python                         # python | node | go | external
  entry: bridge/main.py               # Entry point relative to agent dir
  port: 8099                           # Preferred port
  port_range: [8099, 8199]             # Fallback range if preferred is occupied
  health_check: /health                # GET endpoint for health check
  dashboard: /                         # Dashboard URL path (optional)
  requirements: bridge/requirements.txt
  auto_start: true                     # Start with Claw
  env:                                 # Extra environment variables
    POLY_DRY_RUN: "true"
```

**Bridge directory structure:**

```
agents/my_agent/bridge/
├── main.py                # FastAPI/Flask app
├── requirements.txt       # Python dependencies
├── .env                   # Local config (gitignored)
└── ...                    # Any other files
```

The Bridge must:
1. Listen on the port assigned by Bridge Manager (passed as `PORT` env var)
2. Respond to `GET {health_check}` with 200
3. Expose tool endpoints that match `tools.own` names

### Workflow (Optional)

```yaml
workflow_file: workflow.json
```

If omitted, a default workflow is auto-generated from tools. The workflow JSON follows the existing Claw workflow format (nodes + edges DAG).

### Marketplace (Optional)

```yaml
marketplace:
  pricing:
    type: free                         # free | one_time | subscription
    # For paid agents:
    # type: subscription
    # price: 99900                     # In cents
    # period: quarter                  # month | quarter | year
    # currency: USD
    # display: "$999/quarter"
    # trial_days: 7

  screenshots:
    - url: screenshots/dashboard.png
      caption: { zh: "仪表盘", en: "Dashboard" }

  demo_url: "https://demo.example.com"
  video_url: "https://youtube.com/watch?v=xxx"

  docs:
    readme: README.md
    changelog: CHANGELOG.md

  keywords:
    zh: ["关键词1", "关键词2"]
    en: ["keyword1", "keyword2"]

  compatibility:
    claw_version: ">=2026.0401"
    os: [windows, linux, macos]
```

---

## Team Agent

### manifest.yaml for Teams

```yaml
id: dev_team
type: team

name:
  zh: "Dev 研发智能体"
  en: "Dev Agent Team"

team:
  topology: hierarchical           # hierarchical | flat | pipeline

  roles:
    - code: architect
      name: { zh: "架构师", en: "Architect" }
      prompt_file: roles/architect.md
      tools: [code, web_search]
      model: { name: qwen-max, temperature: 0.3 }
      count: 1
      is_lead: true

    - code: drone
      name: { zh: "执行者", en: "Drone" }
      prompt_file: roles/drone.md
      tools: [code]
      model: { name: qwen-plus, temperature: 0.2 }
      count: 3

  quality_gate:
    review_required: true
    max_retries: 3
    auto_test: true

  escalation:
    on_failure: notify_user
    on_conflict: lead_decides
```

### Directory Structure

```
agents/dev_team/
├── manifest.yaml
├── roles/
│   ├── architect.md
│   ├── architect.en.md
│   ├── drone.md
│   ├── drone.en.md
│   ├── tester.md
│   └── reviewer.md
└── workflow.json           # Optional team workflow
```

### Team Namespace

- External tools: `dev_team:deploy` (callable by other agents)
- Internal tools: `dev_team.architect:plan` (team members only)
- The lead role holds team-level skills and glands.

---

## Bridge Development

### Python Bridge Template

```python
"""Minimal Bridge template for StarClaw agents."""
import os
from fastapi import FastAPI
import uvicorn

app = FastAPI(title="My Agent Bridge")
PORT = int(os.getenv("PORT", "8099"))

@app.get("/health")
def health():
    return {"ok": True, "agent": "my_agent"}

@app.get("/scan")
def scan():
    """This becomes the my_agent:scan tool."""
    return {"ok": True, "results": []}

@app.post("/buy")
def buy(slug: str, amount: float = 1.0):
    """This becomes the my_agent:buy tool."""
    return {"ok": True, "slug": slug, "amount": amount}

if __name__ == "__main__":
    uvicorn.run(app, host="0.0.0.0", port=PORT)
```

### Bridge Contract

| Requirement | Detail |
|------------|--------|
| Port | Read from `PORT` env var (fallback to manifest default) |
| Health | `GET /health` returns 200 with `{"ok": true}` |
| Tools | Each `tools.own` entry maps to a route: `GET/POST /{tool_name}` |
| Startup | Must be healthy within 30 seconds |
| Shutdown | Handle SIGTERM gracefully |

### Dashboard (Optional)

If your Bridge serves a web dashboard, declare it:

```yaml
bridge:
  dashboard: /                     # Accessible at http://localhost:{port}/
```

The Claw frontend will show a "Open Dashboard" button for your agent.

---

## Testing

### Validate Manifest

```bash
claw agent validate my_agent
```

Checks:
- YAML syntax
- Required fields present
- ID matches directory name
- Referenced files exist (prompt, workflow, bridge entry)
- No ID conflicts with other agents

### Run Bridge Locally

```bash
cd claw/agents/my_agent/bridge
pip install -r requirements.txt
python main.py
```

### Test Tools

```bash
curl http://localhost:8099/health
curl http://localhost:8099/scan
```

---

## Publishing to Marketplace

### 1. Prepare

- Set `status: stable` in manifest
- Add screenshots, README, CHANGELOG
- Fill in `marketplace` section (pricing, keywords)

### 2. Register as Creator

On Queen web → Creator Portal → Register with display name, bio, payout info.

### 3. Publish

```bash
claw agent publish my_agent
```

This packages your agent directory and uploads to Queen marketplace for review.

### 4. Updates

Bump `version` in manifest, then:

```bash
claw agent publish my_agent
```

Existing users receive update notifications.

---

## FAQ

**Q: Can I use any programming language for the Bridge?**
A: Yes. Set `bridge.type` to `python`, `node`, `go`, or `external`. The contract is HTTP — any language works.

**Q: What if my agent doesn't need a Bridge?**
A: Omit the `bridge` section entirely. Many agents (MV director, coding agent) only use Claw's built-in tools.

**Q: Can a team agent have a Bridge?**
A: Yes. The Bridge is shared infrastructure — all team roles can call its tools via `{team_id}:{tool}`.

**Q: How do I reference another agent's tools?**
A: List them under `tools.foreign`: `- q8bot:macro`. The referenced agent must be installed.

**Q: How does i18n work at runtime?**
A: The system reads the user's locale preference. If a matching prompt file exists (`prompt.en.md`), it uses that. Otherwise falls back to the default (`prompt.md`).

**Q: Can I have private agents?**
A: Yes. Set `visibility: private`. It won't appear in marketplace but works locally.
