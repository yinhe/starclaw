# StarClaw Skill Development Guide

This guide explains how to create new skills (tools) for StarClaw. Skills extend what AI agents can do — from generating documents to calling external APIs.

---

## Architecture Overview

StarClaw skills are **Go structs** that implement the `Tool` interface. They are registered at startup and made available to all agents.

```
Agent receives user message
  → LLM decides which tool to call
  → Tool Registry dispatches to the correct skill
  → Skill executes and returns result
  → LLM incorporates result into response
```

### Three Ways to Add Skills

| Method | Difficulty | Use Case |
|--------|-----------|----------|
| **Built-in Tool** (Go) | Medium | Core features, DB access, file generation |
| **JSON Plugin** | Easy | Wrap any HTTP API as a skill |
| **MCP Server** | Easy | Connect external MCP-compatible tool servers |

---

## Method 1: Built-in Tool (Go)

### Step 1: Implement the `Tool` Interface

Create a new file in `api/internal/tool/`:

```go
// api/internal/tool/my_tool.go
package tool

import (
    "context"
    "encoding/json"
    "fmt"
)

// MyTool does something useful.
type MyTool struct {
    // Add dependencies: db *gorm.DB, config, etc.
}

func NewMyTool() *MyTool {
    return &MyTool{}
}

// Name returns the unique tool name (used by agents to call it).
func (t *MyTool) Name() string { return "my_tool" }

// Description tells the LLM what this tool does (in Chinese or English).
func (t *MyTool) Description() string {
    return "我的自定义技能：做一些有用的事情。"
}

// Parameters returns JSON Schema describing the tool's input.
func (t *MyTool) Parameters() interface{} {
    return &JSONSchema{
        Type: "object",
        Properties: map[string]Property{
            "action": {Type: "string", Description: "Action: do_something, do_other"},
            "input":  {Type: "string", Description: "The input text"},
        },
        Required: []string{"action"},
    }
}

// Execute runs the tool. args is a JSON string matching the Parameters schema.
func (t *MyTool) Execute(ctx context.Context, args string) (string, error) {
    var params struct {
        Action string `json:"action"`
        Input  string `json:"input"`
    }
    if err := json.Unmarshal([]byte(args), &params); err != nil {
        return "", fmt.Errorf("invalid arguments: %v", err)
    }

    switch params.Action {
    case "do_something":
        result := "did something with: " + params.Input
        // Return JSON so the LLM can parse it
        return toJSON(map[string]interface{}{
            "status":  "success",
            "result":  result,
        }), nil
    default:
        return "", fmt.Errorf("unknown action: %s", params.Action)
    }
}
```

### Step 2: Register in Router

Edit `api/internal/router/router.go`:

```go
// In the "Tool registry" section:
toolRegistry.Register(tool.NewMyTool())
```

### Step 3: Build and Test

```bash
cd api
go build ./...    # Verify it compiles
make up           # Start with Docker
```

### Key Patterns

**Accessing the database:**
```go
type MyTool struct {
    db *gorm.DB
}

func NewMyTool(db *gorm.DB) *MyTool {
    return &MyTool{db: db}
}
```

**Getting user/conversation context:**
```go
func (t *MyTool) Execute(ctx context.Context, args string) (string, error) {
    userID, _ := ctx.Value(CtxKeyUserID).(string)
    convID, _ := ctx.Value(CtxKeyConversationID).(string)
    // ...
}
```

**Generating files and returning download links:**
```go
// Save file
os.MkdirAll("/app/data/my_files", 0755)
filename := fmt.Sprintf("output_%s.pdf", uuid.New().String()[:8])
os.WriteFile("/app/data/my_files/"+filename, data, 0644)

// Return download URL (add matching route in router.go)
return toJSON(map[string]interface{}{
    "download_url": fmt.Sprintf("/v1/my_files/%s", filename),
}), nil
```

**Returning results to the LLM:**
Always return JSON. Use the `toJSON()` helper:
```go
return toJSON(map[string]interface{}{
    "status":  "success",
    "action":  "my_action",
    "data":    resultData,
    "message": "Human-readable description of what happened",
}), nil
```

### Existing Built-in Tools Reference

| Tool | File | Description |
|------|------|-------------|
| `system` | `system_tool.go` | Agent/workflow/task CRUD, delegation |
| `code` | `code_tool.go` | Code execution in sandbox (13 languages) |
| `web_search` | `web_search.go` | Web search via multiple engines |
| `http_request` | `http_request.go` | Generic HTTP requests |
| `browser` | `browser_tool.go` | Headless Chrome browser control |
| `video_generation` | `video_tool.go` | AI video generation |
| `image_generation` | `image_tool.go` | AI image generation (StarAI) |
| `music_generation` | `music_tool.go` | AI music generation |
| `document` | `document_tool.go` | Conversation summary + Word export |
| `dubbing` | `dubbing_tool.go` | TTS dubbing |
| `comic` | `comic_tool.go` | Comic/manga composition |
| `mv` | `mv_tool.go` | Music video composition |
| `deploy_web` | `lifecycle_deploy_tool.go` | Trigger deploy/status/rollback for web release |
| `bind_domain` | `lifecycle_bind_domain_tool.go` | Manage DNS binding records on Cloudflare |
| `verify_online` | `lifecycle_verify_tool.go` | Post-deploy URL health + keyword verification |

### Lifecycle MVP Example

Use these two tools together to complete the final release stage:

1. Trigger deployment:
```json
{
  "action": "deploy",
  "provider": "vercel",
  "deploy_hook_url": "https://api.vercel.com/v1/integrations/deploy/xxxx",
  "target_env": "production",
  "note": "Release from chat orchestrator"
}
```

2. Verify website is online:
```json
{
  "url": "https://your-domain.com",
  "expected_keywords": "StarClaw,AI Agent",
  "retry": "3",
  "interval_sec": "5"
}
```

3. Bind custom domain DNS (Cloudflare):
```json
{
  "action": "upsert",
  "provider": "cloudflare",
  "api_token": "<CLOUDFLARE_API_TOKEN>",
  "zone_id": "<ZONE_ID>",
  "record_type": "CNAME",
  "record_name": "app.your-domain.com",
  "record_value": "cname.vercel-dns.com",
  "proxied": "false",
  "ttl": "120"
}
```

---

## Method 2: JSON Plugin (No Code)

Drop a `.json` file in the `plugins/` directory. StarClaw auto-loads it on startup.

### Plugin Spec Format

```json
{
  "name": "weather",
  "description": "查询城市天气",
  "version": "1.0.0",
  "author": "Your Name",
  "parameters": {
    "type": "object",
    "properties": {
      "city": {
        "type": "string",
        "description": "City name, e.g. Beijing"
      }
    },
    "required": ["city"]
  },
  "endpoint": {
    "url": "https://api.weatherapi.com/v1/current.json?key=YOUR_KEY&q={{city}}",
    "method": "GET"
  },
  "headers": {
    "Accept": "application/json"
  }
}
```

### Features

- **URL placeholders**: `{{param_name}}` in URL are replaced with argument values
- **POST body**: For POST methods, arguments are sent as JSON body (or use `body_template`)
- **Custom headers**: Add auth tokens, API keys, etc.
- **100KB response limit**: Responses are truncated to prevent memory issues

### Example: Translation Plugin

```json
{
  "name": "translate",
  "description": "翻译文本到指定语言",
  "version": "1.0.0",
  "author": "StarClaw",
  "parameters": {
    "type": "object",
    "properties": {
      "text": { "type": "string", "description": "Text to translate" },
      "target_lang": { "type": "string", "description": "Target language code: en, zh, ja, ko" }
    },
    "required": ["text", "target_lang"]
  },
  "endpoint": {
    "url": "https://api.example.com/translate",
    "method": "POST"
  },
  "headers": {
    "Authorization": "Bearer YOUR_API_KEY"
  }
}
```

### Plugin Directory

```
plugins/
  weather.json
  translate.json
  stock_price.json
```

Plugins appear as type `plugin` on the Skills page.

---

## Method 3: MCP Server (External)

Connect any [Model Context Protocol](https://modelcontextprotocol.io) compatible server.

### Via UI

1. Go to **MCP** page in StarClaw
2. Click **Add Server**
3. Enter name and server URL (e.g. `http://localhost:3001/sse`)
4. StarClaw auto-discovers all tools from the server

### Via MCP Bridge

For tools that need host system access (file system, shell, etc.):

1. Download and run MCP Bridge (see Settings page)
2. Bridge auto-registers as `mcp_bridge` tool
3. Agents can execute host commands through the bridge

### Building an MCP Server

Any MCP-compatible server works. Example with Node.js:

```javascript
import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";

const server = new McpServer({ name: "my-tools", version: "1.0.0" });

server.tool("greet", { name: { type: "string" } }, async ({ name }) => ({
  content: [{ type: "text", text: `Hello, ${name}!` }],
}));

const transport = new StdioServerTransport();
await server.connect(transport);
```

MCP tools appear as type `mcp` on the Skills page.

---

## Best Practices

### Tool Design

1. **Clear `Description()`** — The LLM reads this to decide when to use your tool. Be specific about capabilities and usage.
2. **Action-based pattern** — Use an `action` parameter for tools with multiple operations (see `system_tool.go`, `image_tool.go`).
3. **JSON output** — Always return structured JSON so the LLM can extract and present results.
4. **Error messages** — Return user-friendly errors in Chinese when possible.
5. **Context injection** — Use `CtxKeyUserID` / `CtxKeyConversationID` for user-scoped operations.

### File Generation

1. Save files to `/app/data/<your_type>/` with UUID-based filenames
2. Add a public GET endpoint in `router.go` to serve the files
3. Return the download URL in the tool result
4. Set proper `Content-Disposition` and `Content-Type` headers

### Security

1. **UUID filenames** — Prevents directory traversal; files are "public but unlisted"
2. **Input validation** — Always validate file extensions, sizes, and paths
3. **Rate limiting** — Heavy operations should respect existing rate limits
4. **API keys** — Never hardcode; read from user's model config or environment

### Testing

```bash
# Build check
cd api && go build ./...

# Run locally
make up

# Check skills endpoint
curl http://localhost:8080/v1/skills -H "Authorization: Bearer YOUR_TOKEN"

# Test tool via chat
# Just ask the agent: "帮我总结这个对话并导出为Word文档"
```

---

## Contributing

1. Fork the repo
2. Create your tool in `api/internal/tool/`
3. Register it in `api/internal/router/router.go`
4. Add icon in `web/src/pages/SkillsPage.tsx` (optional)
5. Test with `go build ./...`
6. Submit a Pull Request

For JSON plugins, just submit the `.json` file to be placed in `plugins/`.
