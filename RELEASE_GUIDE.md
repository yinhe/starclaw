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

## 6. Spore Package Build & Upload

Spore packages (the one-click installer served at `starclaw.me/download`) must be rebuilt whenever:
- Claw API or Web code changes significantly
- Spore runtime (`cmd/spore`) or setup installer (`cmd/setup`) changes
- A new release tag is created

### 6.1 Build All Platforms

```powershell
# From monorepo root — builds 4 setup installers + 4 spore runtimes
cd spore
powershell -ExecutionPolicy Bypass -File scripts/build-release.ps1
```

Output in `spore/dist/`:
| File | Description |
|------|-------------|
| `StarClaw-Setup.exe` | Windows GUI/CLI installer (embeds spore + claw.spore + wizard) |
| `StarClaw-Setup-linux-amd64` | Linux installer |
| `StarClaw-Setup-darwin-arm64` | macOS Apple Silicon installer |
| `StarClaw-Setup-darwin-amd64` | macOS Intel installer |
| `spore-windows-amd64.exe` | Spore runtime (Windows) |
| `spore-linux-amd64` | Spore runtime (Linux) |
| `spore-darwin-arm64` | Spore runtime (macOS ARM) |
| `spore-darwin-amd64` | Spore runtime (macOS Intel) |

The build script automatically:
1. Cross-compiles `spore` runtime for each platform
2. Embeds the platform-matched runtime into `cmd/setup/embed/spore_bin`
3. Cross-compiles the setup installer (includes GUI wizard HTML)

### 6.2 Upload to Nydus

```powershell
# Upload all artifacts to Nydus server's /opt/spore/releases/
scp -i $env:USERPROFILE\.ssh\queen_deploy spore/dist/StarClaw-Setup* spore/dist/spore-* spore/dist/install.* root@43.106.158.26:/opt/spore/releases/

# Set execute permissions
ssh -i $env:USERPROFILE\.ssh\queen_deploy root@43.106.158.26 "chmod +x /opt/spore/releases/StarClaw-Setup-* /opt/spore/releases/spore-*"
```

Nginx on Nydus serves `/opt/spore/` at `https://nydus.starclaw.net/spore/`.
The download page at `starclaw.me/download` links to these files.

### 6.3 Updating Embedded Claw Package

The `claw.spore` package in `cmd/setup/embed/` contains the compiled Claw binary.
To update it, use `hatchery`:

```bash
# Build latest Claw for all platforms
hatchery build --all

# Copy the target platform .spore to embed dir
cp claw-v*.spore spore/cmd/setup/embed/claw.spore
```

### 6.4 Version Display on Download Page

Both download pages (`queen/site/` and `synapse/web/`) fetch the latest version from
`https://nydus.starclaw.net/releases/spore/latest` (spore-latest.json, tracks actual built installers),
with fallback to `https://nydus.starclaw.net/releases/latest` (git tags from claw.git).
CORS is enabled via nginx `location /releases/` block.

---

## 7. Nydus Deployment Flow

When pushing to `nydus master`, the post-receive hook auto-detects changed directories and deploys:

| 变更目录 | 部署目标 | 说明 |
|----------|----------|------|
| `queen/api/` | Server C (本地) | `docker compose build queen-api` |
| `queen/swarm/` | Server C (本地) | `docker compose build swarm` |
| `queen/site/` | **Server A** (starclaw.me) | SSH → Docker 构建 → 静态文件到 `/var/www/starclaw/website/` |
| `claw/` | **Server A** (starclaw.me) | SSH → `server-deploy-update.sh` → 重建 `starclaw-api:latest` → **自动调用 Hive 升级 API** |
| `hive/` | **Server A** (starclaw.me) | SSH → `docker compose build controller` → 重建 Hive 控制器 |
| `synapse/api/` | Server B (star-ai.net) | SSH → `docker compose build api` |
| `synapse/web/` | Server B (star-ai.net) | SSH → `docker compose build web` |

Hook 位置: `/data/nydus/repos/starclaw.git/hooks/post-receive`

> **重要：** Claw 部署后会自动触发 `POST /hive/admin/upgrade-instances`，滚动升级所有 Hive 托管的 Claw 实例到最新镜像。
> Hive 控制器本身在 `hive/` 目录变更时自动重建（需要 `carapace/` 依赖）。
> `queen/site/` 部署到 Server A（官网），不是 Server C 的 `queen-web` 容器。

---

## 8. macOS DMG 构建 (GitHub Actions)

Spore 安装包的 macOS DMG 通过 GitHub Actions 在 `macos-latest` runner 上用 `hdiutil` 构建（真正的 macOS DMG 格式）。

**Workflow:** `.github/workflows/build-spore-dmg.yml` (手动触发)

**流程：**
```bash
# 1. 触发 (从 OSS repo)
cd E:\starclaw-oss
gh workflow run build-spore-dmg.yml -f version=vXXXX.XXXX.XXXX

# 2. 等待构建完成
gh run watch <run-id> --exit-status

# 3. Nydus 服务器下载 artifact并分发
TOKEN=$(gh auth token)
ssh root@43.106.158.26 "curl -sL -H 'Authorization: Bearer $TOKEN' \
  'https://api.github.com/repos/yinhe/starclaw/actions/artifacts/<id>/zip' \
  -o /tmp/dmg.zip && unzip -o /tmp/dmg.zip -d /tmp/"

# 4. 复制到 Nydus releases + StarAI 镜像
ssh root@43.106.158.26 "cp /tmp/StarClaw-Setup-*.dmg /opt/spore/releases/"
ssh root@43.106.158.26 "scp /tmp/StarClaw-Setup-*.dmg root@47.103.51.32:/dnmp/www/downloads/"
```

> **注意：** `package-releases.sh` 也支持 `hdiutil`，在 macOS 上运行时自动使用真正 DMG 格式，Linux 上回退到 `genisoimage`。

---

## 9. 完整发布流程 (End-to-End)

### Step 1: 构建 & 测试
```bash
# Claw API
cd claw/api && go build ./... && go vet ./...
# Claw Web
cd claw/web && npm run build
# Spore 安装包 (跨平台编译)
cd spore/scripts && powershell ./build-release.ps1
```

### Step 2: Spore 包上传
```bash
# 上传到 Nydus (Server C)
scp spore/dist/StarClaw-Setup-* root@43.106.158.26:/opt/spore/releases/
# 上传到 StarAI 镜像 (Server B)
scp spore/dist/StarClaw-Setup-* root@47.103.51.32:/dnmp/www/downloads/
```

### Step 3: macOS DMG (可选，但推荐)
```bash
# GitHub Actions 构建真正 DMG
cd E:\starclaw-oss
gh workflow run build-spore-dmg.yml -f version=vXXXX.XXXX.XXXX
# 等待完成 → 下载 artifact → 上传到 Nydus + StarAI（见 Section 8）
```

### Step 4: Monorepo 推送
```bash
cd E:\starclaw
git add -A && git commit -m "release: vXXXX.XXXX.XXXX"
git tag vXXXX.XXXX.XXXX
git push nydus master --tags   # 触发 Nydus 自动部署
```

### Step 5: OSS 同步 + GitHub Release
```powershell
# 同步到 OSS repo
robocopy "E:\starclaw\claw" "E:\starclaw-oss" /MIR /XD node_modules .git data build /XF sync-oss.sh *.tar.gz

# 推送 + 打 tag
cd E:\starclaw-oss
git add -A && git commit -m "sync: vXXXX.XXXX.XXXX"
git tag vXXXX.XXXX.XXXX
git push origin main --tags   # 触发 release.yml 自动创建 GitHub Release
```

### Step 6: 验证
- [ ] `https://nydus.starclaw.net/releases/latest` 返回新 tag
- [ ] `starclaw.me/download` 显示新版本号
- [ ] Nydus 下载链接全部 200: .exe / .tar.gz / .dmg (arm64 + amd64)
- [ ] StarAI 镜像下载链接全部 200
- [ ] GitHub Release 页面正常，包含所有 assets
- [ ] macOS DMG 双击测试 (Install StarClaw.command)
- [ ] **Hive 实例已升级**: `curl http://starclaw.me:9090/hive/admin/stats` 确认 running 数量正常
- [ ] 抽检 1-2 个 Hive 实例: `curl https://<slug>.starclaw.me/health` 确认版本号为最新

---

## 10. Checklist (快速参考)

- [ ] `go build` + `npm run build` pass
- [ ] CHANGELOG.md updated (English, Claw-scope only)
- [ ] Spore packages rebuilt & uploaded (Nydus + StarAI)
- [ ] macOS DMG rebuilt via GitHub Actions & uploaded
- [ ] `git push nydus master --tags`
- [ ] OSS sync: `robocopy` → `git push origin main --tags`
- [ ] Verify: Nydus `/releases/latest`, starclaw.me/download, all download links
- [ ] **Hive**: 确认所有托管实例已自动升级到最新 `starclaw-api:latest`
- [ ] **Audit**: no Queen/Overlord mentions in release notes or assets
