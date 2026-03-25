#!/bin/bash
# Nydus per-repo post-receive hook template
# Copy to /data/nydus/repos/<service>.git/hooks/post-receive
# and set: SERVICE, REPO, DEPLOY_DIR, DEPLOY_CMD
#
# Variables to customize per service:
#   SERVICE      — service name (e.g. pheromone, carapace, forge)
#   REPO         — path to this bare repo
#   DEPLOY_DIR   — where to extract and deploy (empty = library, no deploy)
#   DEPLOY_CMD   — command to run after extraction (empty = skip)
#   DEPLOY_TARGET — human-readable deploy target for logs

SERVICE="__SERVICE__"
REPO="__REPO__"
DEPLOY_DIR="__DEPLOY_DIR__"
DEPLOY_CMD="__DEPLOY_CMD__"
DEPLOY_TARGET="__DEPLOY_TARGET__"

LOG=/var/log/nydus-deploy.log
NYDUS_API="http://127.0.0.1:8095"
NYDUS_SECRET="nydus-sc2026-vKwRmT9xLpQjN3hB"
PHEROMONE_NATS_HOST="127.0.0.1"
PHEROMONE_NATS_PORT="4222"

publish_pheromone_deploy() {
  local svc="$1" branch="$2" commit="$3"
  local payload
  payload=$(printf '{"event":"deploy.triggered","repo":"%s","service":"%s","branch":"%s","commit":"%s","actor":"nydus"}' "$svc" "$svc" "$branch" "$commit")
  local subject="pheromone.events.deploy.$svc"
  local bytes=${#payload}
  (
    exec 3<>/dev/tcp/$PHEROMONE_NATS_HOST/$PHEROMONE_NATS_PORT || exit 0
    printf 'CONNECT {"verbose":false,"pedantic":false}\r\n' >&3
    printf 'PUB %s %s\r\n%s\r\n' "$subject" "$bytes" "$payload" >&3
    exec 3>&-
  ) >/dev/null 2>&1 &
}

while read oldrev newrev refname; do
  # Notify Nydus API (tags + branches)
  if echo "$refname" | grep -q '^refs/tags/'; then
    tag=$(echo "$refname" | sed 's|refs/tags/||')
    curl -sf -X POST "$NYDUS_API/hooks/push?secret=$NYDUS_SECRET" \
      -H "Content-Type: application/json" \
      -d "{\"repo\":\"$SERVICE\",\"branch\":\"\",\"newrev\":\"$newrev\",\"tag\":\"$tag\"}" || true
    echo "$(date '+%Y-%m-%d %H:%M:%S') [$SERVICE] Tag $tag pushed" | tee -a "$LOG"
    continue
  fi

  branch=$(echo "$refname" | sed 's|refs/heads/||')
  curl -sf -X POST "$NYDUS_API/hooks/push?secret=$NYDUS_SECRET" \
    -H "Content-Type: application/json" \
    -d "{\"repo\":\"$SERVICE\",\"branch\":\"$branch\",\"newrev\":\"$newrev\",\"tag\":\"\"}" || true

  if [ "$branch" != "master" ]; then
    continue
  fi

  echo "$(date '+%Y-%m-%d %H:%M:%S') [$SERVICE] Push to master: $oldrev -> $newrev" | tee -a "$LOG"
  publish_pheromone_deploy "$SERVICE" "$branch" "$newrev"

  # Deploy (skip if DEPLOY_DIR is empty — library-only repo)
  if [ -n "$DEPLOY_DIR" ] && [ -n "$DEPLOY_CMD" ]; then
    echo "$(date '+%Y-%m-%d %H:%M:%S') [$SERVICE] Deploying to $DEPLOY_TARGET..." | tee -a "$LOG"
    (
      mkdir -p "$DEPLOY_DIR"
      cd "$DEPLOY_DIR"
      git --git-dir="$REPO" archive HEAD | tar xf - 2>/dev/null || true
      eval "$DEPLOY_CMD"
      echo "  -> $SERVICE deployed to $DEPLOY_TARGET" | tee -a "$LOG"
    ) >> "$LOG" 2>&1 &
  else
    echo "$(date '+%Y-%m-%d %H:%M:%S') [$SERVICE] Library-only repo, no deployment" | tee -a "$LOG"
  fi
done
