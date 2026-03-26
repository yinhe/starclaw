---
description: Deploy code to servers (Claw/Hive/Synapse/Queen). Use after git push nydus master.
---

# Deploy Workflow

## How It Works (Diff-Based Auto-Deploy)

`git push nydus master` triggers the `post-receive-starclaw` bash hook on Server C which:
1. Runs `git diff --name-only oldrev newrev` to detect changed directories
2. Only deploys services whose code actually changed
3. Deploys in **parallel** (background `&`) — independent services build simultaneously
4. Syncs changed subdirectories to polyrepos (claw.git → GitHub, queen.git, etc.)
5. Reports results to Nydus API dashboard via `POST /hooks/deploy-report`

**Hook location**: `/data/nydus/repos/starclaw.git/hooks/post-receive`
**Source**: `nydus/hooks/post-receive-starclaw`
**Log**: `/var/log/nydus-deploy.log` on Server C

### Auto-Deploy Trigger Map

| Changed Directory | Target | Server | What Rebuilds |
|-------------------|--------|--------|---------------|
| `queen/api/` | queen-server-c | C | queen-api container |
| `queen/web/` | queen-server-c | C | queen-web container |
| `queen/swarm/` | queen-server-c | C | swarm container |
| `queen/core/` | queen-server-c | C | queen-core container |
| `queen/site/` | queen-server-c → A | C+A | static site on starclaw.me |
| `claw/` | claw-starclaw-me | A | full Claw rebuild + Hive upgrade |
| `hive/` | hive-server-a | A | hive-controller container |
| `synapse/api/` | synapse-server-b | B | synapse-api container |
| `synapse/web/` | synapse-server-b | B | synapse-web container |
| `synapse/core/` | synapse-server-b | B | synapse-core container |
| `synapse/proxy/` | synapse-server-b | B | synapse-proxy container |
| `overlord/` | overlord-server-c | C | overlord api+console+web |
| `pheromone/` | pheromone-server-c | C | pheromone api+web |
| `pheromone/sdk/` | (dependency) | — | triggers rebuild of dependents |
| `nydus/` | nydus-server-c | C | nydus api container |
| `forge/` | forge-server-c | C | forge (if enabled) |

### Polyrepo Sync (automatic on push)

Changed subdirectories are auto-synced to individual bare repos on Nydus:
- `claw/` → `claw.git` (OSS, main branch, pushes to GitHub)
- `queen/`, `hive/`, `synapse/`, `overlord/`, `pheromone/`, `forge/`, `nydus/`, `carapace/`, `cerebrate/`, `larva/`, `spore/` → `{name}.git`

## Standard Deploy

// turbo
1. Push code to Nydus (auto-deploys only changed services):
```
git push nydus master
```

The push returns **immediately** — deploys run in background on the server.
Monitor progress: `ssh -i ~/.ssh/queen_deploy root@43.106.158.26 "tail -f /var/log/nydus-deploy.log"`

## Manual Deploy (Fallback)

Use these only when auto-deploy fails for a specific service.

2. Queen API (Server C):
```
ssh -i ~/.ssh/queen_deploy root@43.106.158.26 "cd /opt/queen && rm -rf api/ && git --git-dir=/data/nydus/repos/starclaw.git archive HEAD:queen/ | tar xf - && mkdir -p api/certs api/pheromone-sdk && git --git-dir=/data/nydus/repos/starclaw.git archive HEAD:pheromone/sdk | tar xf - -C api/pheromone-sdk && docker compose -f docker-compose.prod.yml build --no-cache queen-api && docker compose -f docker-compose.prod.yml up -d queen-api"
```

3. Claw (Server A):
```
ssh -i ~/.ssh/queen_deploy root@43.106.158.26 "git --git-dir=/data/nydus/repos/starclaw.git archive HEAD:claw/ | ssh root@starclaw.me 'cd /opt/starclaw && tar xf -'"
ssh -i ~/.ssh/queen_deploy root@starclaw.me "cd /opt/starclaw && bash scripts/server-deploy-update.sh all --no-pull"
```

4. Hive (Server A):
```
ssh -i ~/.ssh/queen_deploy root@43.106.158.26 "git --git-dir=/data/nydus/repos/starclaw.git archive HEAD:hive/ | ssh root@starclaw.me 'cd /opt/hive && tar xf -'"
ssh -i ~/.ssh/queen_deploy root@starclaw.me "cd /opt/hive && docker compose -f docker-compose.hive.yml build --no-cache controller && docker compose -f docker-compose.hive.yml up -d controller"
```

5. Synapse API (Server B):
```
ssh -i ~/.ssh/queen_deploy root@43.106.158.26 "git --git-dir=/data/nydus/repos/starclaw.git archive HEAD:synapse/ | ssh root@47.103.51.32 'cd /opt/starclaw/synapse && tar xf -'"
ssh -i ~/.ssh/queen_deploy root@47.103.51.32 "cd /opt/starclaw/synapse && docker compose build --no-cache api && docker compose up -d api"
```

## Quick Verification

// turbo
6. Verify all services:
```
ssh -i ~/.ssh/queen_deploy root@43.106.158.26 "curl -s http://127.0.0.1:8085/health; echo; curl -s http://127.0.0.1:8098/health"
```

// turbo
7. Verify Server A:
```
ssh -i ~/.ssh/queen_deploy root@starclaw.me "curl -s http://localhost:8080/health; echo; curl -s http://localhost:9090/hive/health"
```

// turbo
8. Verify Server B:
```
ssh -i ~/.ssh/queen_deploy root@47.103.51.32 "curl -s http://localhost:8096/health"
```

## SSH Keys Reference

- **Server C** (Queen/Nydus): `ssh -i ~/.ssh/queen_deploy root@43.106.158.26`
- **Server A** (Claw/Hive): `ssh -i ~/.ssh/queen_deploy root@starclaw.me`
- **Server B** (Synapse): `ssh -i ~/.ssh/queen_deploy root@47.103.51.32`

Nydus bare repo: `/data/nydus/repos/starclaw.git` on Server C only.

## Server Layout

| Server | IP | Services | Key Ports |
|--------|----|----------|-----------|
| A (starclaw.me) | 43.106.138.214 | Claw :8080/:8081, Hive :9090/:8082 | nginx :443 |
| B (star-ai.net) | 47.103.51.32 | Synapse :8096/:3096, Gateway :8085 | dnmp nginx :443 |
| C (starclaw.net) | 43.106.158.26 | Queen :8085, Nydus :8098, Overlord :8095, Pheromone :8100/:4222 | nginx :443 |

## Common Issues

- **Push returns instantly, deploy still running**: Deploys run in background. Check `/var/log/nydus-deploy.log` on Server C.
- **Service not rebuilt after push**: Check if the changed directory matches the trigger map above. Only changed dirs trigger deploys.
- **Queen stale files**: `tar xf` only adds/overwrites. For queen, always `rm -rf api/` before extracting.
- **Hive needs carapace + pheromone/sdk**: The hook auto-syncs both dependencies. For manual deploy, sync them separately.
- **pheromone/sdk change**: Triggers rebuild of queen-api, synapse-api, and other dependents that import it.
- **Polyrepo not synced**: Check `/var/log/nydus-deploy.log` for `[sync]` entries. Sync runs in background after deploys.

## Go Module Vanity Import (starclaw.net/*)

All private Go modules use `starclaw.net/*` vanity import paths.

- **Meta tag handler**: nginx on `starclaw.net` serves `?go-get=1` → `nydus.starclaw.net/git/starclaw.git`
- **Claw (open-source)**: stays at `github.com/yinhe/starclaw`, NOT migrated
- **Dockerfiles**: Queen + Synapse have `GOPRIVATE=starclaw.net` for sum DB bypass
