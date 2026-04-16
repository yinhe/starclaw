# StarClaw Agent Plugin Architecture

> Version 1.0 | 2026-04-04

## Overview

StarClaw adopts a **plugin-based agent architecture** where every agent — solo or team — is a self-contained directory under `claw/agents/`. The system auto-discovers agents on startup by scanning for `manifest.yaml` files, extracting the **Octad (八元) properties**, and registering them into the database.

**Zero Go code changes are needed to add a new agent.**

```
claw/agents/
├── super/              # Solo — Super Agent (总管)
├── mv_director/        # Solo — MV Director
├── polymarket/         # Solo — with Bridge subprocess
├── q8bot/              # Solo — with Bridge subprocess
├── devclaw/            # Team — 7 roles
├── quantclaw/          # Team — 5 roles + Bridge
└── _template/          # Scaffold template for developers
```

---

## 1. Octad Properties (八元属性)

Every agent declares exactly **8 categories** of properties in its `manifest.yaml`:

| # | Property | Description | Required |
|---|----------|-------------|----------|
| 1 | **Metadata** | id, name, description, icon, category, tags, version, author, status | Yes |
| 2 | **Prompt** | System prompt files (i18n: `prompt.md`, `prompt.en.md`) | Yes |
| 3 | **Model** | Default LLM model, temperature, max_tokens | Yes |
| 4 | **Tools** | Own (namespaced), shared (Claw built-in), foreign (cross-agent) | Yes |
| 5 | **Skills** | Passive skills (user-triggered) + proactive instincts (scheduled) | No |
| 6 | **Glands** | Runtime config: credentials, thresholds, toggles, endpoints | No |
| 7 | **Bridge** | Optional subprocess: Python/Node/Go API server | No |
| 8 | **Workflow** | DAG workflow definition (auto-generated from tools if absent) | No |

---

## 2. Agent Types

### 2.1 Solo Agent (`type: solo`)

A single agent instance with one prompt, one set of tools.

```
agents/polymarket/
├── manifest.yaml       # type: solo
├── prompt.md           # Chinese prompt
├── prompt.en.md        # English prompt
└── bridge/             # Optional Bridge subprocess
    ├── main.py
    └── requirements.txt
```

### 2.2 Team Agent (`type: team`)

A multi-role agent team with hierarchical/flat/pipeline topology.

```
agents/devclaw/
├── manifest.yaml       # type: team
└── roles/
    ├── architect.md    # Lead role prompt
    ├── drone.md        # Worker role prompt (×3 instances)
    ├── tester.md
    ├── reviewer.md
    └── docbot.md
```

Each role has its own prompt, tools, model config, and instance count. The `is_lead` role receives team-level skills/glands and dispatches tasks to other roles.

---

## 3. Namespace Isolation

Every agent's exported symbols are automatically namespaced to prevent conflicts.

### 3.1 Tools

| Category | Syntax | Example |
|----------|--------|---------|
| **Own** (from Bridge) | `{agent_id}:{tool}` | `polymarket:scan`, `q8bot:buy` |
| **Shared** (Claw built-in) | `{tool}` (no prefix) | `web_search`, `code` |
| **Foreign** (cross-agent) | `{other_id}:{tool}` | `q8bot:macro` |

Developers write **short names** in manifest (`buy`, `sell`). The discovery engine auto-prefixes with `{agent_id}:` at registration.

### 3.2 Skills & Glands

Isolated by `agent_id` foreign key in the database — no namespace prefix needed. Two agents can both have a skill named "持仓查看" without conflict.

### 3.3 Bridge Ports

Each Bridge declares a preferred port and fallback range. The Bridge Manager allocates ports with conflict detection:

```
preferred port → if occupied → scan port_range → if all occupied → random free port
```

### 3.4 Team Internal Tools

| Scope | Format | Visibility |
|-------|--------|------------|
| External | `devclaw:deploy` | Other agents can call |
| Internal | `devclaw.architect:plan` | Team members only |

---

## 4. Discovery Engine

### 4.1 Startup Flow

```
Claw API starts
       │
       ▼
ScanAgentsDir("agents/")
       │
       ├── Read manifest.yaml from each subdirectory
       ├── Parse Octad properties
       ├── Validate (id uniqueness, required fields, schema)
       └── Return []AgentManifest
       │
       ▼
SeedFromManifests(db, manifests, ownerID)
       │
       ├── For each manifest:
       │   ├── Upsert Agent record (solo) or TeamInstance + role Agents (team)
       │   ├── Upsert AgentSkills
       │   ├── Upsert AgentGlands (schema only, values from user)
       │   ├── Register MCP binding for Bridge URL
       │   └── Create/update Workflow
       │
       ▼
StartBridges(manifests)
       │
       ├── For each manifest with bridge.auto_start=true:
       │   ├── Allocate port
       │   ├── Start subprocess (python bridge/main.py)
       │   ├── Wait for health check
       │   └── Update Agent.BridgeStatus = "running"
       │
       ▼
Ready — all agents registered and bridges running
```

### 4.2 Hot Reload (Future)

Watch `agents/` directory for changes. On file modification:
- Re-parse affected manifest
- Diff against DB state
- Apply incremental updates (no full restart needed)

---

## 5. i18n Strategy

| Layer | Approach |
|-------|----------|
| **manifest.yaml** | All string fields use `{zh: "", en: ""}` map |
| **Prompts** | Separate files: `prompt.md` (default/zh), `prompt.en.md` |
| **Dashboard HTML** | JS i18n: load `/web/i18n/{lang}.json` |
| **API responses** | `Accept-Language` header selects locale |
| **Team roles** | `roles/{code}.md` (zh), `roles/{code}.en.md` |

The system reads user's locale preference and serves the matching content. Falls back to `zh` if the requested locale is unavailable.

---

## 6. Marketplace Integration

### 6.1 Static Properties (in manifest.yaml)

- **Author**: id, name, avatar, url, verified
- **Pricing**: free / one_time / subscription + trial_days
- **Screenshots**: image URLs with captions
- **Docs**: README, CHANGELOG, FAQ
- **Compatibility**: claw_version, os, arch
- **Keywords**: searchable tags per locale

### 6.2 Runtime Properties (in database)

- **install_count**: Incremented on each install
- **rating / rating_count**: Aggregated from AgentReview
- **AgentReview**: User reviews with rating, content, helpful votes, author replies
- **AgentInstallLog**: Install/uninstall/update audit trail
- **CreatorStats**: Total installs, revenue, avg rating

### 6.3 Classification Badges (auto-derived)

| Badge | Source | Values |
|-------|--------|--------|
| Source | `is_builtin` + `marketplace` | builtin / marketplace / local |
| Pricing | `marketplace.pricing.type` | free / paid / trial |
| Type | `type` | solo / team |
| Status | `status` | draft / beta / stable / deprecated |
| Languages | prompt file scan | zh, en, ja, ... |
| Bridge | `bridge` presence | yes / no |

### 6.4 Ranking Algorithm

```
score = wilson_lower_bound(rating, rating_count)
      × log10(install_count + 1)
      × freshness(updated_at)
      × official_boost(is_builtin, featured)
```

---

## 7. Data Model Changes

### 7.1 Agent Model (add 3 fields)

```go
type Agent struct {
    // ... existing fields unchanged ...
    ManifestID   string `json:"manifest_id" gorm:"type:varchar(50);index"`
    BridgePort   int    `json:"bridge_port" gorm:"default:0"`
    BridgeStatus string `json:"bridge_status" gorm:"type:varchar(20)"`
}
```

### 7.2 New Models

```go
type AgentReview struct {
    ID, AgentID, UserID, Rating, Title, Content, Locale,
    HelpfulUp, HelpfulDown, AuthorReply, CreatedAt, UpdatedAt
}

type AgentInstallLog struct {
    ID, AgentID, UserID, Action, Version, CreatedAt
}
```

---

## 8. API Endpoints

### 8.1 Marketplace

```
GET  /v1/marketplace/agents                    # List (filter/sort/paginate)
GET  /v1/marketplace/agents/:id                # Detail
GET  /v1/marketplace/agents/:id/reviews        # Reviews
GET  /v1/marketplace/agents/:id/changelog      # Changelog
GET  /v1/marketplace/featured                  # Staff picks
POST /v1/marketplace/agents/:id/install        # Install
POST /v1/marketplace/agents/:id/uninstall      # Uninstall
POST /v1/marketplace/agents/:id/review         # Write review
POST /v1/marketplace/reviews/:rid/helpful      # Vote helpful
```

### 8.2 Creator

```
GET  /v1/marketplace/creator/stats             # Creator dashboard
POST /v1/marketplace/creator/publish           # Publish agent
POST /v1/marketplace/reviews/:rid/reply        # Reply to review
GET  /v1/marketplace/creator/revenue           # Revenue report
```

---

## 9. Implementation Phases

| Phase | Scope | Status |
|-------|-------|--------|
| **P1** | `agents/` dir + manifest spec + `discovery.go` scanner + template | ✅ Done |
| **P2** | Migrate 11 builtin agents to `agents/` (super, mv, video, music, coding, research, comic, business, short_drama, douyin, devclaw) | ✅ Done (2026-04-04) |
| **P2.1** | Discovery-first startup: `server/main.go` runs Discovery before `SeedBuiltinAgents`; hardcoded agents skipped when Discovery covers them; DB dedup cleans old duplicates | ✅ Done (2026-04-04) |
| **P3** | Migrate Polymarket/Q8bot bridges into `agents/` | ✅ Polymarket done; Q8bot pending |
| **P4** | Bridge Manager + i18n frontend + `claw agent` CLI + reviews | Pending |

---

## 10. Architecture Diagram

```
┌────────────────────────────────────────────────────────────┐
│                       Claw API (Go)                         │
│                                                             │
│  ┌──────────────┐  ┌──────────┐  ┌──────────────────────┐  │
│  │ Agent Handler │  │ Chat/LLM │  │  Discovery Engine    │  │
│  │  (CRUD)       │  │ (ReAct)  │  │  (scan agents/ dir)  │  │
│  └──────┬────────┘  └────┬─────┘  └──────────┬───────────┘  │
│         │                │                    │              │
│  ┌──────┴────────────────┴────────────────────┴───────────┐  │
│  │                    Agent Model (DB)                      │  │
│  │  Agent → Skills[] → Glands[] → MCP[] → Workflow          │  │
│  │  AgentReview → AgentInstallLog → CreatorStats            │  │
│  └──────────────────────┬──────────────────────────────────┘  │
│                         │                                     │
│  ┌──────────────────────┴──────────────────────────────────┐  │
│  │                Bridge Manager (子进程管理)                │  │
│  │  ┌──────────┐ ┌────────────┐ ┌────────────────────────┐ │  │
│  │  │ Q8bot    │ │ Polymarket │ │ Future Agent Bridges    │ │  │
│  │  │ :8098    │ │ :8099      │ │ :80xx                  │ │  │
│  │  └──────────┘ └────────────┘ └────────────────────────┘ │  │
│  └─────────────────────────────────────────────────────────┘  │
└────────────────────────────────────────────────────────────┘
         │                    │                   │
    ┌────┴───┐          ┌────┴────┐         ┌────┴────┐
    │External│          │External │         │ Queen   │
    │  APIs  │          │  APIs   │         │Marketpl.│
    └────────┘          └─────────┘         └─────────┘
```
