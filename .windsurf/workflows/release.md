---
description: Full release workflow for StarClaw. Build, tag, deploy, sync OSS, publish GitHub Release.
---

# Release Workflow

Full end-to-end release process based on `RELEASE_GUIDE.md`.

## ⚠ CRITICAL: Open-Source vs Closed-Source Classification

Before ANY release or OSS sync, review this list. Only `claw/` goes to GitHub. Everything else stays private.

### ✅ OPEN-SOURCE (synced to GitHub via `E:\starclaw-oss`)

| Directory | Description | Notes |
|-----------|-------------|-------|
| `claw/` | Claw node (API + Web + MCP Bridge) | **The ONLY dir pushed to GitHub.** OSS repo root = contents of `claw/` |
| `spore/` | Lightweight deployment system (installer + runtime) | Build artifacts uploaded to Nydus/StarAI, but source is NOT on GitHub. Spore binaries are GitHub Release assets via CI. |

### 🔒 CLOSED-SOURCE (NEVER sync to GitHub or mention in release notes)

| Directory | Description | Why closed |
|-----------|-------------|------------|
| `queen/` | Central control (API, Swarm, Bounty, Forum, Arena, Core, Web, Site, Overseer) | Core commercial platform |
| `overlord/` | Enterprise management (API, Console, Web) | Enterprise/white-label product |
| `synapse/` | StarAI Router (API, Web, Core, Proxy) — star-ai.net | Commercial AI gateway + payment |
| `cerebrate/` | Partner portals (City Partner + Partner Web) | Commercial partner ecosystem |
| `hive/` | Hive hosting controller (API, Web) | Commercial hosting service |
| `nydus/` | Code distribution & deployment system (API, Web, hooks) | Internal DevOps infra |
| `larva/` | Mobile app (Flutter — iOS + Android) | Commercial mobile client |
| `carapace/` | Encryption library (AES, KDF, Vault, Envelope) | Internal security module |

### 🚫 NEVER goes to GitHub (root-level files)

| File | Reason |
|------|--------|
| `RELEASE_GUIDE.md` | Contains internal architecture & server info |
| `MONOREPO.md` | Full monorepo structure docs |
| `SERVERS.md` | Server IPs, credentials layout |
| `DEPLOY.md` | Internal deployment procedures |
| `OPERATIONS_PIPELINE.md` | Internal ops pipeline |
| `.env` / `.env.production` | Secrets |
| `docker-compose*.yml` (root) | Internal infra compose |
| `starclaw.bundle` | Git bundle (full repo) |

### Audit Checklist (run before every OSS sync)
- [ ] `git diff` in OSS repo: no queen/overlord/synapse/cerebrate/hive/nydus/larva/carapace paths
- [ ] No Chinese in release notes or CHANGELOG.md
- [ ] No internal service names (Queen, Overlord, Synapse, Cerebrate, Hive, Nydus, Larva, Carapace) in commit messages
- [ ] No server IPs or tokens in committed code
- [ ] Release title is exactly `StarClaw vYYYY.MMDD.HHmm` (no subtitle)

---

## Step 1: Build & Test

// turbo
1. Build and vet Claw API:
```
cd e:\starclaw\claw\api && go build ./... && go vet ./...
```

// turbo
2. Build Claw Web:
```
cd e:\starclaw\claw\web && npm run build
```

## Step 2: Generate Version Tag

// turbo
3. Generate version string (UTC timestamp format YYYY.MMDD.HHmm):
```
powershell -Command "Write-Host ('v' + (Get-Date).ToUniversalTime().ToString('yyyy.MMdd.HHmm'))"
```

## Step 3: Commit & Tag (MUST be before Spore build — Spore reads version from git tag)

4. Commit all changes and tag (replace VERSION with output from step 3):
```
cd e:\starclaw && git add -A && git commit -m "release: VERSION"
git tag VERSION
```

## Step 4: Build Spore Packages (AFTER tag — Spore reads version from git tag)

5. Build all Spore installers + runtimes:
```
cd e:\starclaw\spore && powershell -ExecutionPolicy Bypass -File scripts/build-release.ps1
```
Output goes to `spore/dist/`. Expected: 8 files with correct VERSION in filenames.

## Step 5: Upload Spore to Servers

6. Upload to Nydus (Server C) + set permissions:
```
scp -i ~/.ssh/starai_deploy spore/dist/StarClaw-Setup* spore/dist/spore-* root@43.106.158.26:/opt/spore/releases/
ssh -i ~/.ssh/starai_deploy root@43.106.158.26 "chmod +x /opt/spore/releases/StarClaw-Setup-* /opt/spore/releases/spore-*"
```

7. Upload to StarAI mirror (Server B):
```
scp -i ~/.ssh/starai_deploy spore/dist/StarClaw-Setup* root@47.103.51.32:/dnmp/www/downloads/
```

## Step 6: Package macOS DMGs on Nydus

8. Run package-releases.sh on Nydus to create DMGs from uploaded raw binaries:
```
ssh -i ~/.ssh/starai_deploy root@43.106.158.26 "bash /opt/spore/scripts/package-releases.sh VERSION 2>&1"
```

9. Sync DMGs from Nydus to StarAI mirror:
```
ssh -i ~/.ssh/starai_deploy root@43.106.158.26 "scp -o StrictHostKeyChecking=no /opt/spore/releases/StarClaw-Setup-VERSION-darwin-arm64.dmg /opt/spore/releases/StarClaw-Setup-VERSION-darwin-amd64.dmg root@47.103.51.32:/dnmp/www/downloads/"
```

## Step 7: Push to Nydus (triggers auto-deploy + version update)

10. Push code + tags to Nydus:
```
cd e:\starclaw && git push nydus master --tags
```
This triggers the Nydus post-receive hook which:
- Auto-deploys changed services (queen/api, queen/site, synapse/api, synapse/web)
- **Auto-syncs the version tag to `claw.git`** (so `/releases/latest` returns the new version)
- **Auto-regenerates `spore-latest.json`** (so Spore update checks find the new version)
- **Download pages auto-update** (they read version from Nydus API at runtime, no manual edit needed)

## Step 8: Deploy Download Pages (only if Nydus hook misses them)

NOTE: Download pages read version dynamically from `https://nydus.starclaw.net/releases/spore/latest`.
They should auto-update after Step 7. Only manually deploy if they don't reflect the new version.

11. (If needed) Deploy queen/site on Server C:
```
ssh -i ~/.ssh/starai_deploy root@43.106.158.26 "cd /opt/queen && docker compose -f docker-compose.prod.yml build --no-cache queen-site && docker compose -f docker-compose.prod.yml up -d queen-site"
```

12. (If needed) Deploy synapse/web to Server B:
```
ssh -i ~/.ssh/starai_deploy root@43.106.158.26 "git --git-dir=/data/nydus/repos/starclaw.git archive HEAD:synapse/ | ssh root@47.103.51.32 'cd /opt/starclaw/synapse && tar xf -'"
ssh -i ~/.ssh/starai_deploy root@47.103.51.32 "cd /opt/starclaw/synapse && docker compose build --no-cache web 2>&1 | tail -3 && docker compose up -d web 2>&1"
```

## Step 9: Manual Deploy (if Nydus hook misses anything)

Use `/deploy` workflow for any services that need manual deployment (Claw, Hive, Synapse, Gateway).

## Step 9: Sync to OSS Repo

// turbo
10. Sync claw/ to OSS repo (robocopy on Windows).
    NOTE: Excludes trading plugin dir and trading_*.json to protect proprietary Q8bot quant strategy.
```
robocopy "E:\starclaw\claw" "E:\starclaw-oss" /MIR /XD node_modules .git data build trading /XF sync-oss.sh *.tar.gz trading_*.json builtin_agents_q8bot.go
```

11. Commit, tag, and push OSS (triggers GitHub Actions release.yml → cross-compile + GitHub Release):
```
cd e:\starclaw-oss && git add -A && git commit -m "sync: VERSION" && git tag VERSION && git push origin main --tags
```
This automatically creates a GitHub Release with 18 assets (10 binaries, 6 archives/DMGs, 2 source).

## Step 10: Verify

// turbo
15. Check Nydus latest version endpoint (Docker update path):
```
curl -s https://nydus.starclaw.net/releases/latest
```

// turbo
16. Check Nydus Spore latest endpoint (Spore update path):
```
curl -s https://nydus.starclaw.net/releases/spore/latest
```

// turbo
13. Verify Claw API health:
```
ssh -i ~/.ssh/claw_deploy root@starclaw.me "curl -s http://localhost:8080/health | head -1"
```

// turbo
14. Verify Synapse API health:
```
ssh -i ~/.ssh/starai_deploy root@47.103.51.32 "curl -s http://localhost:8096/health | head -1"
```

// turbo
14b. Verify Hive instances upgraded (all running instances should use latest starclaw-api image):
```
ssh -i ~/.ssh/claw_deploy root@starclaw.me "curl -s http://localhost:9090/hive/admin/stats -H 'Authorization: Bearer \$HIVE_ADMIN_TOKEN'"
```

// turbo
15. Verify download file links return 200 for all 4 platforms:
```
ssh -i ~/.ssh/starai_deploy root@43.106.158.26 "for f in StarClaw-Setup-VERSION.exe StarClaw-Setup-VERSION-linux-amd64.tar.gz StarClaw-Setup-VERSION-darwin-arm64.dmg StarClaw-Setup-VERSION-darwin-amd64.dmg; do echo -n \"$f: \"; curl -sI https://nydus.starclaw.net/spore/releases/$f | head -1; done"
```

// turbo
16. Verify StarAI mirror download links:
```
ssh -i ~/.ssh/starai_deploy root@47.103.51.32 "for f in StarClaw-Setup-VERSION.exe StarClaw-Setup-VERSION-linux-amd64.tar.gz StarClaw-Setup-VERSION-darwin-arm64.dmg StarClaw-Setup-VERSION-darwin-amd64.dmg; do echo -n \"$f: \"; curl -sI https://star-ai.net/downloads/$f | head -1; done"
```

17. Manual verification checklist:
- [ ] `starclaw.net/download` shows new version number (queen-site :8097)
- [ ] `star-ai.net/download` shows new version number (synapse-web)
- [ ] Nydus download links all return 200 (.exe / .tar.gz / .dmg × 2)
- [ ] StarAI mirror download links all return 200 (.exe / .tar.gz / .dmg × 2)
- [ ] `/releases/latest` returns correct `tag_name`
- [ ] `/releases/spore/latest` returns correct `tag_name`
- [ ] DownloadPage `const V` matches new tag in BOTH files (queen/site + synapse/web)
- [ ] GitHub Release page has all 18 assets
- [ ] Release notes are English-only, Claw-scope only (NO Queen/Overlord mentions)
- [ ] CHANGELOG.md updated (English, grouped by Added/Changed/Fixed/Removed)

## Key Rules

- **Version format:** `YYYY.MMDD.HHmm` (UTC), e.g. `v2026.0310.1214`
- **Release title:** `StarClaw vYYYY.MMDD.HHmm` (no subtitle)
- **Release notes:** English only, Claw-scope only — NEVER mention Queen/Overlord
- **CHANGELOG.md:** English, Claw-related changes only
- **OSS repo** (`E:\starclaw-oss`) = contents of `claw/` at root level

## SSH Keys Reference

- Server A (Claw + Hive): `ssh -i ~/.ssh/claw_deploy root@starclaw.me`
- Server B (Synapse): `ssh -i ~/.ssh/starai_deploy root@47.103.51.32`
- Server C (Queen/Nydus): `ssh -i ~/.ssh/starai_deploy root@43.106.158.26`
