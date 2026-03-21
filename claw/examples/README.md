# StarClaw Examples

Runnable code examples for integrating with StarClaw.

## Prerequisites

Set environment variables:

```bash
export STARCLAW_ENDPOINT=http://localhost:8080   # your Claw instance
export STARCLAW_API_KEY=your-jwt-token           # from /v1/auth/login
```

## Examples

### 1. Basic Chat (Go)

Simple non-streaming chat completion:

```bash
cd chat-basic
go run main.go "What is StarClaw?"
```

### 2. Streaming Chat (TypeScript)

Real-time streaming with SSE:

```bash
cd chat-stream
npx tsx index.ts "Explain quantum computing"
```

### 3. Chat Widget (HTML)

Drop-in `<starclaw-chat>` web component — open `chat-widget/index.html` in a browser.

Edit the `endpoint` and `api-key` attributes to connect to your instance.

## SDK

For full SDK documentation, see:

- **JavaScript/TypeScript**: [`@starclaw/sdk`](https://www.npmjs.com/package/@starclaw/sdk) — `StarClawClient` + `<starclaw-chat>` Web Component
- **Go**: `go get github.com/yinhe/starclaw-sdk-go` — typed client with streaming support

## API Reference

StarClaw exposes an OpenAI-compatible API. See [API Docs](../docs/API_EN.md) for the full specification.
