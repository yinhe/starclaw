# StarClaw 🦞 API Reference

> Base URL: `http://localhost:8080/v1` or via Nginx proxy `http://your-domain/v1`

## Authentication

All endpoints (except public ones) require a JWT token in the header:

```
Authorization: Bearer <token>
```

## Public Endpoints

| Method | Path | Description |
|--------|------|-------------|
| POST | `/v1/auth/register` | Register a new user |
| POST | `/v1/auth/login` | Login (returns JWT) |
| GET | `/v1/health` | Health check |
| GET | `/v1/version` | Version info |
| GET | `/v1/config` | Deployment mode config |

## Chat

| Method | Path | Description |
|--------|------|-------------|
| GET | `/v1/conversations` | List conversations |
| POST | `/v1/conversations` | Create conversation |
| DELETE | `/v1/conversations/:id` | Delete conversation |
| GET | `/v1/conversations/:id/messages` | List messages |
| POST | `/v1/chat` | Send message (SSE streaming) |
| POST | `/v1/chat/stop` | Stop generation |

## Agents

| Method | Path | Description |
|--------|------|-------------|
| GET | `/v1/agents` | List agents |
| POST | `/v1/agents` | Create agent |
| PUT | `/v1/agents/:id` | Update agent |
| DELETE | `/v1/agents/:id` | Delete agent |
| GET | `/v1/agents/builtin` | List built-in agents |

## Workflows

| Method | Path | Description |
|--------|------|-------------|
| GET | `/v1/workflows` | List workflows |
| POST | `/v1/workflows` | Create workflow |
| PUT | `/v1/workflows/:id` | Update workflow |
| DELETE | `/v1/workflows/:id` | Delete workflow |
| POST | `/v1/workflows/:id/run` | Execute workflow (SSE) |

## RAG Knowledge Base

| Method | Path | Description |
|--------|------|-------------|
| GET | `/v1/knowledge` | List knowledge bases |
| POST | `/v1/knowledge` | Create knowledge base |
| DELETE | `/v1/knowledge/:id` | Delete knowledge base |
| POST | `/v1/knowledge/:id/documents` | Upload document |
| DELETE | `/v1/knowledge/:id/documents/:doc_id` | Delete document |
| POST | `/v1/knowledge/:id/query` | Query knowledge base |

## Model Management

| Method | Path | Description |
|--------|------|-------------|
| GET | `/v1/models` | List model configs |
| POST | `/v1/models` | Add model config (API key) |
| PUT | `/v1/models/:id` | Update model config |
| DELETE | `/v1/models/:id` | Delete model config |

## Coding Agent

| Method | Path | Description |
|--------|------|-------------|
| POST | `/v1/coding/run` | Start coding agent (SSE) |
| GET | `/v1/coding/workspace/:id/files` | List workspace files |
| GET | `/v1/coding/workspace/:id/file` | Read workspace file |

## Async Tasks

| Method | Path | Description |
|--------|------|-------------|
| GET | `/v1/tasks` | List tasks |
| GET | `/v1/tasks/:id` | Get task details |
| POST | `/v1/tasks/:id/cancel` | Cancel task |

## User

| Method | Path | Description |
|--------|------|-------------|
| GET | `/v1/user/profile` | Get user profile |
| PUT | `/v1/user/profile` | Update user profile |
| PUT | `/v1/user/password` | Change password |

## Swarm (available after joining swarm)

| Method | Path | Description |
|--------|------|-------------|
| POST | `/v1/swarm/register` | Node registration |
| POST | `/v1/swarm/heartbeat` | Heartbeat report |
| GET | `/v1/swarm/config` | Pull swarm config |
| GET | `/v1/swarm/update/check` | Check for version updates |

## SSE Streaming Response Format

Chat and workflow endpoints use Server-Sent Events for streaming:

```
data: {"type":"token","content":"Hello"}
data: {"type":"token","content":", I am"}
data: {"type":"tool_call","name":"web_search","arguments":{"query":"..."}}
data: {"type":"tool_result","name":"web_search","result":"..."}
data: {"type":"done","usage":{"prompt_tokens":100,"completion_tokens":50}}
```
