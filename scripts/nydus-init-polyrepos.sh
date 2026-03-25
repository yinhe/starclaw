#!/usr/bin/env bash
set -euo pipefail

# Initialize StarClaw polyrepo bare repositories on Nydus host.
# Usage:
#   sudo -u git bash scripts/nydus-init-polyrepos.sh /data/nydus/repos

REPO_ROOT="${1:-/data/nydus/repos}"
OWNER_USER="${OWNER_USER:-git}"
OWNER_GROUP="${OWNER_GROUP:-git}"

REPOS=(
  starclaw
  claw
  queen
  synapse
  overlord
  forge
  hive
  pheromone
  cerebrate
  larva
  spore
  carapace
  nydus
)

mkdir -p "$REPO_ROOT"

for repo in "${REPOS[@]}"; do
  target="$REPO_ROOT/$repo.git"

  if [[ -d "$target" ]]; then
    echo "[SKIP] $repo.git already exists"
  else
    git init --bare "$target"
    echo "[NEW ] created $repo.git"
  fi

done

# Only chown/chmod the *.git repo dirs — never touch .ssh, .docker, .cache etc.
for repo in "${REPOS[@]}"; do
  target="$REPO_ROOT/$repo.git"
  if [[ -d "$target" ]]; then
    chown -R "$OWNER_USER:$OWNER_GROUP" "$target"
    chmod -R ug+rwX "$target"
  fi
done
# Ensure REPO_ROOT itself is owned by git but world-readable (sshd requires <=755 for home)
chown "$OWNER_USER:$OWNER_GROUP" "$REPO_ROOT"
chmod 755 "$REPO_ROOT"

printf "\n[OK] Repo bootstrap done at %s\n" "$REPO_ROOT"
ls -1 "$REPO_ROOT" | sed 's/^/ - /'
