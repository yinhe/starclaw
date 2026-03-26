---
description: Deploy code to servers (Claw/Hive/Synapse/Queen). Use after git push nydus master.
auto_execution_mode: 3
---

# Deploy Workflow

All deploys start with `git push nydus master` to sync code to the Nydus bare repo on Server C.

## Prerequisites
// turbo
1. Push code to Nydus:
```
git push nydus master
```

## Deploy Claw (Server A - starclaw.me)

Two steps: sync code from Nydus then rebuild on Server A.

// turbo
2. Sync code from Nydus (Server C) to Claw (Server A):
```
ssh -i ~/.ssh/starai_deploy root@43.106.158.26 "git --git-dir=/data/nydus/repos/starclaw.git archive HEAD:claw/ | ssh root@starclaw.me 'cd /opt/starclaw && tar xf -'"
```

3. Build and restart Claw:
```
ssh -i ~/.ssh/claw_deploy root@starclaw.me "cd /opt/starclaw; bash scripts/server-deploy-update.sh all --no-pull 2>&1 | tail -15"
```
This takes about 2 minutes. Expected output ends with Deploy complete.

## Deploy Hive (Server A - starclaw.me)

Hive Controller runs on the same server as Claw. Needs both `hive/` and `carapace/` synced.

// turbo
4. Sync hive + carapace from Nydus (Server C) to Server A:
```
ssh -i ~/.ssh/starai_deploy root@43.106.158.26 "git --git-dir=/data/nydus/repos/starclaw.git archive HEAD:hive/ | ssh root@starclaw.me 'cd /opt/starclaw/hive && tar xf -'"
ssh -i ~/.ssh/starai_deploy root@43.106.158.26 "git --git-dir=/data/nydus/repos/starclaw.git archive HEAD:carapace/ | ssh root@starclaw.me 'cd /opt/starclaw/carapace && tar xf -'"
```

5. Build and restart Hive (controller build context is repo root for carapace dep):
```
ssh -i ~/.ssh/claw_deploy root@starclaw.me "cd /opt/starclaw && docker compose -f hive/docker-compose.hive.yml --env-file hive/.env up -d --build 2>&1 | tail -10"
```
Expected: 4 containers (hive-mysql, hive-redis, hive-controller, hive-web) all started.

## Deploy Synapse (Server B - star-ai.net)

Two steps: sync code from Nydus then rebuild on Server B.

// turbo
6. Sync code from Nydus (Server C) to Synapse (Server B):
```
ssh -i ~/.ssh/starai_deploy root@43.106.158.26 "git --git-dir=/data/nydus/repos/starclaw.git archive HEAD:synapse/ | ssh root@47.103.51.32 'cd /opt/starclaw/synapse && tar xf -'"
```

7. Build and restart Synapse API:
```
ssh -i ~/.ssh/starai_deploy root@47.103.51.32 "cd /opt/starclaw/synapse/api && docker compose build --no-cache api 2>&1 | tail -5 && docker compose up -d api 2>&1"
```
Expected: Container star-ai-api Started

## Deploy Queen/Gateway (Server C - starclaw.net)

Queen auto-deploys via Nydus hook on git push. Manual fallback:

8. Queen (if auto-deploy failed) — **must clean api/ first** to remove stale files:
```
ssh -i ~/.ssh/starai_deploy root@43.106.158.26 "cd /opt/queen && rm -rf api/ && git --git-dir=/data/nydus/repos/starclaw.git archive HEAD:queen/ | tar xf - && mkdir -p api/certs && docker compose -f docker-compose.prod.yml build --no-cache queen-api 2>&1 | tail -5 && docker compose -f docker-compose.prod.yml up -d queen-api 2>&1"
```

9. Gateway on Server B (queen/api code):
```
ssh -i ~/.ssh/starai_deploy root@43.106.158.26 "git --git-dir=/data/nydus/repos/starclaw.git archive HEAD:queen/api | ssh root@47.103.51.32 'cd /opt/starclaw/gateway && tar xf -'"
ssh -i ~/.ssh/starai_deploy root@47.103.51.32 "cd /opt/starclaw/gateway && docker compose -f docker-compose.gateway.yml up -d --build 2>&1 | tail -5"
```

## Quick Verification

// turbo
10. Verify Claw API health:
```
ssh -i ~/.ssh/claw_deploy root@starclaw.me "curl -s http://localhost:8080/health | head -1"
```

// turbo
11. Verify Hive Controller health:
```
ssh -i ~/.ssh/claw_deploy root@starclaw.me "curl -s http://localhost:9090/hive/health"
```

// turbo
12. Verify Synapse API health:
```
ssh -i ~/.ssh/starai_deploy root@47.103.51.32 "curl -s http://localhost:8096/health | head -1"
```

## SSH Keys Reference

- Server A (Claw + Hive): ssh -i ~/.ssh/claw_deploy root@starclaw.me
- Server B (Router): ssh -i ~/.ssh/starai_deploy root@47.103.51.32
- Server C (Queen/Nydus): ssh -i ~/.ssh/starai_deploy root@43.106.158.26

Note: Server C is accessed with starai_deploy key (not queen_deploy). The Nydus bare repo is at /data/nydus/repos/starclaw.git on Server C only.

## Server Layout

| Server | IP | Services | Port Mapping |
|--------|----|----------|--------------|
| A (starclaw.me) | 43.106.138.214 | Claw API :8080, Claw Web :8081, Hive Controller :9090, Hive Web :8082, Hive MySQL :3307, Hive Redis :6380 | nginx :443 |
| B (star-ai.net) | 47.103.51.32 | Synapse API :8096, Gateway :8085, Synapse Web :3096 | dnmp nginx :443 |
| C (starclaw.net) | 43.106.158.26 | Queen API, Nydus, bare git repo | auto-deploy hook |

## Common Issues

- Nydus auto-deploy SSH failure: Server C to Server A SSH key is broken. Use manual sync (step 2+3).
- git archive fails on wrong server: The bare repo only exists on Server C. Always run git archive FROM Server C via SSH.
- Synapse build uses cache: Add --no-cache flag to docker compose build if code changes are not reflected.
- Hive controller build needs carapace: Always sync both `hive/` AND `carapace/` dirs (step 4). The Dockerfile uses `context: ..` to access `../../carapace`.
- Hive .env not in git: The `.env` file on Server A at `/opt/starclaw/hive/.env` is NOT tracked in git. Edit it directly on server for secrets.
- Queen stale files: `tar xf` only adds/overwrites, never deletes removed files. Always `rm -rf api/` before extracting queen archive. Stale `.go` files with old imports will break Docker build.
- Queen api/certs: Dockerfile expects `certs/` dir. Run `mkdir -p api/certs` after extraction if it doesn't exist.

## Go Module Vanity Import (starclaw.net/*)

All private Go modules use `starclaw.net/*` vanity import paths (commit `07c96da`).

- **Meta tag handler**: nginx on `starclaw.net` serves `?go-get=1` → points to `nydus.starclaw.net/git/starclaw.git`
- **Git HTTP backend**: `nydus.starclaw.net/git/` serves bare repo read-only via `git-http-backend`
- **Claw (open-source)**: stays at `github.com/yinhe/starclaw`, NOT migrated
- **Dockerfiles**: Queen + Synapse have `GOPRIVATE=starclaw.net` for sum DB bypass
- **Config doc**: `nydus/deploy/nginx-go-module.conf`
- **nginx backup**: `/etc/nginx/sites-enabled/queen.bak` on Server C
