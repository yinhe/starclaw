---
description: Deploy code to servers (Claw/Synapse/Queen). Use after git push nydus master.
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

## Deploy Synapse (Server B - star-ai.net)

Two steps: sync code from Nydus then rebuild on Server B.

// turbo
4. Sync code from Nydus (Server C) to Synapse (Server B):
```
ssh -i ~/.ssh/starai_deploy root@43.106.158.26 "git --git-dir=/data/nydus/repos/starclaw.git archive HEAD:synapse/ | ssh root@47.103.51.32 'cd /opt/starclaw/synapse && tar xf -'"
```

5. Build and restart Synapse API:
```
ssh -i ~/.ssh/starai_deploy root@47.103.51.32 "cd /opt/starclaw/synapse/api && docker compose build --no-cache api 2>&1 | tail -5 && docker compose up -d api 2>&1"
```
Expected: Container star-ai-api Started

## Deploy Queen/Gateway (Server C - starclaw.net)

Queen auto-deploys via Nydus hook on git push. Manual fallback:

6. Queen (if auto-deploy failed):
```
ssh -i ~/.ssh/starai_deploy root@43.106.158.26 "cd /opt/starclaw-queen && git --git-dir=/data/nydus/repos/starclaw.git archive HEAD:queen/ | tar xf - && docker compose -f docker-compose.prod.yml up -d --build 2>&1 | tail -10"
```

7. Gateway on Server B (queen/api code):
```
ssh -i ~/.ssh/starai_deploy root@43.106.158.26 "git --git-dir=/data/nydus/repos/starclaw.git archive HEAD:queen/api | ssh root@47.103.51.32 'cd /opt/starclaw/gateway && tar xf -'"
ssh -i ~/.ssh/starai_deploy root@47.103.51.32 "cd /opt/starclaw/gateway && docker compose -f docker-compose.gateway.yml up -d --build 2>&1 | tail -5"
```

## Quick Verification

// turbo
8. Verify Claw API health:
```
ssh -i ~/.ssh/claw_deploy root@starclaw.me "curl -s http://localhost:8080/health | head -1"
```

// turbo
9. Verify Synapse API health:
```
ssh -i ~/.ssh/starai_deploy root@47.103.51.32 "curl -s http://localhost:8096/health | head -1"
```

## SSH Keys Reference

- Server A (Claw): ssh -i ~/.ssh/claw_deploy root@starclaw.me
- Server B (Router): ssh -i ~/.ssh/starai_deploy root@47.103.51.32
- Server C (Queen/Nydus): ssh -i ~/.ssh/starai_deploy root@43.106.158.26

Note: Server C is accessed with starai_deploy key (not queen_deploy). The Nydus bare repo is at /data/nydus/repos/starclaw.git on Server C only.

## Common Issues

- Nydus auto-deploy SSH failure: Server C to Server A SSH key is broken. Use manual sync (step 2+3).
- git archive fails on wrong server: The bare repo only exists on Server C. Always run git archive FROM Server C via SSH.
- Synapse build uses cache: Add --no-cache flag to docker compose build if code changes are not reflected.
