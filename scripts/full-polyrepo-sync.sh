#!/bin/bash
# One-time full resync: monorepo master → all polyrepos
REPO=/data/nydus/repos/starclaw.git
REV=$(git --git-dir="$REPO" rev-parse master)
SHORT=$(echo "$REV" | head -c 7)
MSG="sync: full resync ($SHORT)"

for svc in queen hive synapse overlord pheromone forge nydus carapace cerebrate larva spore; do
  POLYREPO="/data/nydus/repos/${svc}.git"
  [ ! -d "$POLYREPO" ] && echo "SKIP $svc (no repo)" && continue

  TMPDIR=$(mktemp -d)
  git --git-dir="$REPO" archive "${REV}:${svc}" 2>/dev/null | tar xf - -C "$TMPDIR" 2>/dev/null
  if [ -z "$(ls -A "$TMPDIR" 2>/dev/null)" ]; then
    rm -rf "$TMPDIR"
    echo "SKIP $svc (empty subdir)"
    continue
  fi

  IDX=$(mktemp)
  rm -f "$IDX"

  export GIT_DIR="$POLYREPO"
  export GIT_WORK_TREE="$TMPDIR"
  export GIT_INDEX_FILE="$IDX"
  export GIT_AUTHOR_NAME="Nydus Sync"
  export GIT_AUTHOR_EMAIL="nydus@starclaw.net"
  export GIT_COMMITTER_NAME="Nydus Sync"
  export GIT_COMMITTER_EMAIL="nydus@starclaw.net"

  git add -A 2>/dev/null

  if git diff --cached --quiet HEAD 2>/dev/null; then
    echo "SKIP $svc (no diff)"
    rm -f "$IDX"; rm -rf "$TMPDIR"
    unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_AUTHOR_NAME GIT_AUTHOR_EMAIL GIT_COMMITTER_NAME GIT_COMMITTER_EMAIL
    continue
  fi

  TREE=$(git write-tree)
  PARENT=$(git rev-parse refs/heads/master 2>/dev/null) || true
  if [ -n "$PARENT" ]; then
    NC=$(echo "$MSG" | git commit-tree "$TREE" -p "$PARENT")
  else
    NC=$(echo "$MSG" | git commit-tree "$TREE")
  fi
  git update-ref refs/heads/master "$NC"

  echo "OK $svc ($(echo "$NC" | head -c 7))"
  rm -f "$IDX"; rm -rf "$TMPDIR"
  unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_AUTHOR_NAME GIT_AUTHOR_EMAIL GIT_COMMITTER_NAME GIT_COMMITTER_EMAIL
done
