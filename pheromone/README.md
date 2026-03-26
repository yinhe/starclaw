# Pheromone

Pheromone is the StarClaw Enterprise Service Bus (ESB), built on **NATS JetStream**.

## Architecture

```
┌─────────────┐    NATS JetStream    ┌─────────────────┐
│  Service A   │◄──────────────────►│  Pheromone API   │
│  (SDK client)│   pub/sub + RPC     │  (registry +     │
└─────────────┘                      │   event hub)     │
┌─────────────┐                      └─────────────────┘
│  Service B   │◄────────┘                  │
│  (SDK client)│                     ┌──────┴──────┐
└─────────────┘                      │ Pheromone Web│
                                     │ (dashboard)  │
                                     └─────────────┘
```

## Components

| Component | Path | Port | Description |
|-----------|------|------|-------------|
| **API** | `api/` | 8100 | Event hub, service registry, SSE stream |
| **Web** | `web/` | 3110 | Real-time dashboard |
| **SDK** | `sdk/` | — | Go client library for services |
| **NATS** | (Docker) | 4222 | JetStream message broker |

## SDK (`sdk/`)

Go module: `starclaw.net/pheromone/sdk`

### Features

- **Service Registration** — announce service on connect (`pheromone.registry.announce`)
- **Heartbeat** — periodic liveness signal (`pheromone.heartbeat.{name}`)
- **Event Pub/Sub** — publish and subscribe to events (`pheromone.events.{topic}`)
- **RPC** — request/reply pattern (`pheromone.rpc.{service}.{method}`)

### Quick Start

```go
import pheromone "starclaw.net/pheromone/sdk"

ph, err := pheromone.New("nats://pheromone-nats:4222", pheromone.ServiceInfo{
    Name:    "my-service",
    Version: "1.0.0",
    Port:    8080,
    Tags:    []string{"api", "core"},
})
if err != nil {
    log.Fatal(err)
}
defer ph.Close()

ph.StartHeartbeat(30 * time.Second)
```

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/services` | List registered services |
| GET | `/api/health` | Health check |
| GET | `/api/events` | Recent events |
| POST | `/api/publish` | Publish event |
| GET | `/api/events/stream` | SSE event stream |

## Integrating the SDK into a Service

1. **go.mod** — add dependency with local replace:
   ```
   require starclaw.net/pheromone/sdk v0.0.0
   replace starclaw.net/pheromone/sdk => ./pheromone-sdk
   ```

2. **Local dev** — create symlink: `pheromone-sdk` → `../../pheromone/sdk`

3. **.gitignore** — add `pheromone-sdk`

4. **Dockerfile** — copy SDK *before* `go mod download`:
   ```dockerfile
   COPY go.mod go.sum ./
   COPY pheromone-sdk/ ./pheromone-sdk/
   RUN go mod download
   ```

5. **docker-compose.yml** — add env + network:
   ```yaml
   environment:
     PHEROMONE_NATS_URL: nats://pheromone-nats:4222
   networks:
     - starclaw-pheromone
   ```

6. **Deploy hook** — extract SDK into build context:
   ```bash
   mkdir -p /opt/{service}/api/pheromone-sdk
   git archive HEAD:pheromone/sdk | tar xf - -C /opt/{service}/api/pheromone-sdk
   ```

## Docker Networking

- Pheromone services run on `starclaw-pheromone` external network
- Any service connecting to NATS must join this network
- NATS URL inside containers: `nats://pheromone-nats:4222`

## Deployment

Deployed at `/opt/starclaw-pheromone/` via `docker compose`.

```bash
docker compose up -d
```

## Registered Services

Currently integrated:
- **nydus** (v1.0.0) — Git hosting, deploy, registry

See also:
- `docs/architecture/PHEROMONE_ESB.md`
- `docs/architecture/PHEROMONE_MVP_RUNBOOK.md`
