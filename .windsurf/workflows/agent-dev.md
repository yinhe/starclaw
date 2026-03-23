---
description: Develop a new Agent, Skill, or Team Template using DevClaw and publish to the StarClaw marketplace
---

# Agent Development Workflow

Use this workflow to develop new Agents, Skills (Plugins), or Team Agent Templates for the StarClaw marketplace.

## Prerequisites
- Access to a StarClaw Claw node (app.starclaw.me or self-hosted)
- Overlord management console access (for Team Templates)

---

## Step 1: Choose what to build

Ask the developer what they want to create:
- **Agent**: An AI assistant with specific system_prompt + model + tools (publishes to Agent Marketplace)
- **Skill/Plugin**: A tool that agents can use, defined by a JSON spec (publishes to Skill Marketplace)  
- **Team Template**: A multi-agent team with roles, topology, and quality gates (publishes to Overlord)

## Step 2: Gather requirements

Ask the developer to describe:
- **Name**: What should this agent/skill be called?
- **Purpose**: What problem does it solve? Who is the target user?
- **Capabilities**: What should it be able to do? What should it NOT do?
- **Domain**: What knowledge domain? (medical, legal, finance, coding, etc.)
- **Tools needed**: Does it need web_search, code execution, document reading, custom APIs?

## Step 3: Design the Agent

Based on the requirements, design:

### For an Agent:
1. **System Prompt** — Write a comprehensive system prompt that includes:
   - Role definition and expertise
   - Specific responsibilities and capabilities  
   - Output format constraints
   - Safety boundaries and disclaimers
   - Tone and style guidelines

2. **Model Selection** — Choose the best model:
   - `deepseek-chat` — Good balance of cost and capability
   - `gpt-4o` — Best for complex reasoning, safety-critical domains
   - `claude-sonnet-4-20250514` — Best for creative writing, nuanced responses

3. **Tools** — Select from available tools:
   - `web_search` — Internet search
   - `code` — Code execution
   - `document_read` — Read documents/files
   - `document_write` — Write documents
   - `http_request` — Make HTTP API calls
   - `image_generation` — Generate images
   - `browser` — Browse web pages
   - Or define a custom plugin (see Skill development below)

4. **Config** — Set parameters:
   - `temperature`: 0.1-0.3 for factual, 0.5-0.8 for creative
   - `max_tokens`: 2048-8192 depending on use case

### For a Skill/Plugin:
1. Design the Plugin Spec JSON following the format in `claw/api/plugins/`
2. Define endpoints, parameters, and response format
3. Implement the backend if it requires custom logic

### For a Team Template:
1. Define roles (code, name, system_prompt, model, tools, max_instances)
2. Define topology (pipeline, fan_out, fan_in, review_loop)
3. Set quality gates (review_threshold, test_required, max_retries)
4. Set escalation policy

## Step 4: Implement

### Agent implementation:
Create the agent configuration as JSON:
```json
{
  "name": "Agent Name",
  "description": "What this agent does",
  "system_prompt": "You are...",
  "model": "deepseek-chat",
  "tools": ["web_search", "document_read"],
  "config": { "temperature": 0.3, "max_tokens": 4096 },
  "category": "assistant",
  "tags": ["tag1", "tag2"],
  "icon": "🤖"
}
```

### Skill/Plugin implementation:
Create the plugin spec JSON in `claw/api/plugins/`:
```json
{
  "name": "my_plugin",
  "display_name": "My Plugin",
  "description": "What this plugin does",
  "version": "1.0.0",
  "endpoints": [
    {
      "name": "action_name",
      "method": "POST",
      "url": "https://api.example.com/action",
      "parameters": { ... }
    }
  ]
}
```

### Team Template implementation:
Define in Go following the pattern in `overlord/api/internal/handler/team_agent.go`:
- Use `buildDevClaw()` as reference
- Define roles, topology, quality gate, escalation

## Step 5: Test

### Quick test via Claw API:
```bash
# Create a temporary agent for testing
curl -X POST https://app.starclaw.me/v1/agents \
  -H "Authorization: Bearer $TOKEN" \
  -d '{ "name": "Test Agent", "system_prompt": "...", "tools": [...] }'

# Test with a chat message
curl -X POST https://app.starclaw.me/v1/chat/completions \
  -H "Authorization: Bearer $TOKEN" \
  -d '{ "agent_id": "...", "message": "test question" }'
```

### Test cases to verify:
1. **Happy path**: Does it answer correctly for typical questions?
2. **Boundaries**: Does it refuse out-of-scope requests?
3. **Safety**: Does it include appropriate disclaimers?
4. **Format**: Does it output in the expected format?
5. **Prompt injection**: Does it resist attempts to override its instructions?

## Step 6: Publish

### Publish Agent to marketplace:
```bash
# 1. Create agent template
curl -X POST https://app.starclaw.me/v1/templates \
  -H "Authorization: Bearer $TOKEN" \
  -d '{ "agent_id": "...", "category": "medical", "tags": "[\"医疗\"]" }'

# 2. Create marketplace listing (optional, for paid agents)
curl -X POST https://app.starclaw.me/v1/marketplace/creator/listings \
  -d '{ "template_id": "...", "pricing": "free" }'
```

### Publish Skill to marketplace:
```bash
curl -X POST https://app.starclaw.me/v1/developer/plugins \
  -H "Authorization: Bearer $TOKEN" \
  -d '{ "name": "my_plugin", "display_name": "...", "spec_json": "..." }'
```

### Publish Team Template to Overlord:
Contact an Overlord admin or use the admin API to register the template.

## Step 7: Verify in Overlord

After publishing, verify the agent appears in:
1. Claw Web → Agent Marketplace → search for your agent
2. Overlord Console → Team Agent → Edit Role → "从市场导入" button

---

## Tips
- Start with a narrow, well-defined scope — a focused agent beats a generic one
- Always include safety boundaries in system_prompt for sensitive domains
- Test with adversarial inputs before publishing
- Use `temperature: 0.3` or lower for factual/safety-critical agents
- Include example outputs in your system_prompt for consistent formatting
