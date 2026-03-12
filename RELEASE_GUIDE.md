# StarClaw Release Guide (Internal)

> **This file lives in the PRIVATE monorepo root (`E:\starclaw\`). It is NEVER synced to GitHub.**
> The public-facing version is at `claw/docs/RELEASE_GUIDE.md`.

This document defines the versioning rules and release process for the StarClaw project.

---

## 0. Repository Structure

StarClaw uses a **dual-repo** setup:

| Repo | Path | Remote | Visibility |
|------|------|--------|------------|
| Private monorepo | `E:\starclaw` | none (local only) | Private — contains ALL code |
| OSS repo | `E:\starclaw-oss` | `github.com:yinhe/starclaw` | Public — Claw only |

**Private monorepo** (`E:\starclaw`):
```
E:\starclaw/
  claw/          ← open-source Claw (API, Web, MCP Bridge)
  queen/         ← closed-source central control
  overlord/      ← closed-source enterprise management
```

**OSS repo** (`E:\starclaw-oss`):
```
E:\starclaw-oss/         ← git root, pushed to GitHub
  .github/workflows/    ← CI/CD (ci.yml, release.yml)
  api/                  ← Claw API (Go)
  web/                  ← Claw Web (React)
  docs/                 ← Documentation
  deploy/               ← Deployment configs
  scripts/              ← Helper scripts
  Makefile, README.md, LICENSE, etc.
```

The OSS repo contains the **contents** of `claw/` at its root level (not `claw/` as a subdirectory).

### 0.1 Sync Workflow

To publish changes from monorepo to GitHub:

```powershell
# Windows (robocopy)
robocopy "E:\starclaw\claw" "E:\starclaw-oss" /MIR /XD node_modules .git data build /XF sync-oss.sh *.tar.gz

# Linux/macOS (rsync)
bash claw/scripts/sync-oss.sh "commit message"
```

Then commit and push from the OSS repo:
```bash
cd E:\starclaw-oss
git add -A
git commit -m "description of changes"
git push origin main
```

### 0.2 What NEVER goes to GitHub

- `queen/` — closed-source central control (API, Swarm, Bounty, Forum, Arena, Core, Web, Mobile)
- `overlord/` — closed-source enterprise management (Manager, Console)
- Any mention of Queen/Overlord in code, docs, release notes, or commit messages
- This file (`RELEASE_GUIDE.md` at monorepo root)

---

## 1. Version Format

**Format:** `YYYY.MMDD.HHmm` (UTC)

```
Example: 2026.0310.1214 = March 10, 2026 at 12:14 UTC
Git Tag: v2026.0310.1214
```

- Naturally sortable — string comparison determines newer/older
- No manual version management needed
- Supports multiple releases per day (minute precision)
- Unified across Git Tag, Docker Tag, API response, and heartbeat

**Version is injected at build time** via `-ldflags`, never hardcoded in source:
```go
// molt.go
var Version = "dev"  // overridden by: -X .../molt.Version=2026.0310.1214
```

---

## 2. Release Commands

```bash
# View current version (auto-generated from UTC time)
make version

# Build API binary with version stamp
make build-api

# Docker build (version auto-injected)
make up          # local
make up-cn       # China mirrors

# Create git tag + push to trigger CI/CD
make tag
git push origin v2026.0310.1214

# Override version manually (rare)
make build-api VERSION=2026.0310.0000
```

---

## 3. Release Pipeline

```
make tag → git push
  → GitHub Actions release.yml triggers
  → Extract version from tag
  → Cross-compile 10 binaries (5 platforms × 2 programs)
  → Build & push Docker images to ghcr.io
  → Create GitHub Release with all assets
```

---

## 4. GitHub Release Rules

### 4.1 Title Format

```
StarClaw v2026.MMDD.HHmm
```

- **Always** prefix with `StarClaw`
- **No subtitle** after the version (no `- Feature Name`)
- Example: `StarClaw v2026.0310.1214`

### 4.2 Release Notes Language

- **All English** — no Chinese in release notes
- Keep descriptions concise and factual

### 4.3 Content Scope — Claw Only

This is the **open-source Claw** repository. Release notes must **NEVER** mention:

- Queen (central control, admin dashboard, billing, swarm management)
- Overlord (enterprise management, console, teams, RBAC)
- Any closed-source service names or internal architecture

Only include changes related to:

- Claw API (agents, tools, providers, chat, workflows)
- Claw Web (frontend UI)
- MCP Bridge
- P2P / Gossip protocol
- Molt self-update (client-side only)
- Docker / deployment improvements

### 4.4 Two Distribution Channels

StarClaw has two distinct install channels. The release assets serve both:

| Channel | Target | Assets | Platforms |
|---------|--------|--------|-----------|
| **Docker auto-update** | Server operators, Molt self-update | Docker images (`ghcr.io`) + raw binaries (for Molt in-place swap) | Linux (primary) |
| **Manual download** | Local dev, MCP Bridge users | tar.gz archives (macOS/Linux) + DMG installers (macOS) + raw binaries | macOS, Windows, Linux |

**Why macOS needs special packaging:**
- Raw darwin binaries are blocked by macOS Gatekeeper (quarantine attribute)
- tar.gz archives include an `install.sh` that runs `xattr -d com.apple.quarantine`
- DMG disk images provide a familiar macOS install experience (double-click `install.command`)

### 4.5 Release Assets

Every release must include **18 assets**:

| # | File | Description |
|---|------|-------------|
| | **StarClaw API Server** | |
| 1 | `starclaw-linux-amd64` | Raw binary — Linux x64 (Docker/Molt) |
| 2 | `starclaw-linux-arm64` | Raw binary — Linux ARM64 (Docker/Molt) |
| 3 | `starclaw-darwin-amd64` | Raw binary — macOS Intel |
| 4 | `starclaw-darwin-arm64` | Raw binary — macOS Apple Silicon |
| 5 | `starclaw-darwin-amd64.tar.gz` | macOS Intel archive (binary + README) |
| 6 | `starclaw-darwin-arm64.tar.gz` | macOS Apple Silicon archive (binary + README) |
| 7 | `starclaw-windows-amd64.exe` | Windows x64 |
| | **MCP Bridge** | |
| 8 | `mcp-bridge-linux-amd64` | Raw binary — Linux x64 |
| 9 | `mcp-bridge-linux-arm64` | Raw binary — Linux ARM64 |
| 10 | `mcp-bridge-darwin-amd64` | Raw binary — macOS Intel |
| 11 | `mcp-bridge-darwin-arm64` | Raw binary — macOS Apple Silicon |
| 12 | `mcp-bridge-darwin-amd64.tar.gz` | macOS Intel archive (binary + install.sh + README) |
| 13 | `mcp-bridge-darwin-arm64.tar.gz` | macOS Apple Silicon archive (binary + install.sh + README) |
| 14 | `mcp-bridge-darwin-amd64.dmg` | macOS Intel DMG installer |
| 15 | `mcp-bridge-darwin-arm64.dmg` | macOS Apple Silicon DMG installer |
| 16 | `mcp-bridge-windows-amd64.exe` | Windows x64 |
| | **Source** | |
| 17 | `Source code (zip)` | Auto-generated by GitHub |
| 18 | `Source code (tar.gz)` | Auto-generated by GitHub |

### 4.6 Release Body Template

```markdown
## Install

### 🐳 Docker (server deployment — recommended)
\```bash
docker pull ghcr.io/yinhe/starclaw-api:YYYY.MMDD.HHmm
docker pull ghcr.io/yinhe/starclaw-web:YYYY.MMDD.HHmm
\```
Existing servers with Molt auto-update will receive this version automatically.

### 💻 Manual Install (local development / MCP Bridge)

**macOS (recommended: tar.gz or DMG):**
| Chip | StarClaw API | MCP Bridge | MCP Bridge DMG |
|------|-------------|------------|----------------|
| Apple Silicon | starclaw-darwin-arm64.tar.gz | mcp-bridge-darwin-arm64.tar.gz | mcp-bridge-darwin-arm64.dmg |
| Intel | starclaw-darwin-amd64.tar.gz | mcp-bridge-darwin-amd64.tar.gz | mcp-bridge-darwin-amd64.dmg |

> DMG: open and double-click install.command to install to /usr/local/bin.
> tar.gz: extract and run sudo ./install.sh or copy binary manually.

**Linux / Windows (raw binary):**
| Platform | StarClaw API | MCP Bridge |
|----------|-------------|------------|
| Linux x64 | starclaw-linux-amd64 | mcp-bridge-linux-amd64 |
| Linux ARM64 | starclaw-linux-arm64 | mcp-bridge-linux-arm64 |
| Windows x64 | starclaw-windows-amd64.exe | mcp-bridge-windows-amd64.exe |

## Changes
- Change 1 (English only, Claw-scope only)
- Change 2
```

---

## 5. CHANGELOG.md Rules

- Written in **English**
- Only **Claw-related** changes
- Group by: `Added`, `Changed`, `Fixed`, `Removed`
- Reference the timestamp version, not SemVer

---

## 6. Checklist Before Release

- [ ] All changes committed and pushed
- [ ] `go build ./...` passes in `claw/api`
- [ ] `npm run build` passes in `claw/web`
- [ ] CHANGELOG.md updated (English, Claw-scope only)
- [ ] Sync monorepo → OSS repo (`robocopy` or `sync-oss.sh`)
- [ ] `make tag` → `git push origin v...`
- [ ] Verify GitHub Release: title, notes, 18 assets
- [ ] Verify macOS assets: 2 tar.gz (API) + 2 tar.gz (MCP) + 2 DMG (MCP)
- [ ] Test DMG install on macOS (double-click install.command)
- [ ] **Audit**: no Queen/Overlord mentions in release notes or assets
