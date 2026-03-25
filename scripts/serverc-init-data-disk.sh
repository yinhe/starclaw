#!/usr/bin/env bash
set -euo pipefail

# Server C data disk bootstrap for StarClaw
# Usage:
#   sudo bash scripts/serverc-init-data-disk.sh /dev/vdb /data
# Defaults:
#   DISK_DEVICE=/dev/vdb
#   MOUNT_POINT=/data

DISK_DEVICE="${1:-/dev/vdb}"
MOUNT_POINT="${2:-/data}"
FSTAB_FILE="/etc/fstab"

if [[ "$(id -u)" -ne 0 ]]; then
  echo "[ERROR] Please run as root (sudo)."
  exit 1
fi

if [[ ! -b "$DISK_DEVICE" ]]; then
  echo "[ERROR] Block device not found: $DISK_DEVICE"
  exit 1
fi

if findmnt -n "$MOUNT_POINT" >/dev/null 2>&1; then
  echo "[INFO] $MOUNT_POINT is already mounted. Skip mount setup."
else
  CURRENT_FS="$(blkid -s TYPE -o value "$DISK_DEVICE" || true)"

  if [[ -z "$CURRENT_FS" ]]; then
    echo "[INFO] No filesystem detected on $DISK_DEVICE. Formatting as ext4..."
    mkfs.ext4 "$DISK_DEVICE"
  elif [[ "$CURRENT_FS" != "ext4" ]]; then
    echo "[ERROR] Existing filesystem on $DISK_DEVICE is '$CURRENT_FS' (not ext4)."
    echo "        Refusing to reformat automatically. Migrate/check manually first."
    exit 1
  else
    echo "[INFO] Existing ext4 filesystem found on $DISK_DEVICE."
  fi

  mkdir -p "$MOUNT_POINT"
  mount "$DISK_DEVICE" "$MOUNT_POINT"
  echo "[INFO] Mounted $DISK_DEVICE -> $MOUNT_POINT"
fi

if grep -qE "^[^#]*[[:space:]]+$MOUNT_POINT[[:space:]]+ext4[[:space:]]" "$FSTAB_FILE"; then
  echo "[INFO] fstab already has an entry for $MOUNT_POINT"
else
  echo "$DISK_DEVICE $MOUNT_POINT ext4 defaults,nofail 0 2" >> "$FSTAB_FILE"
  echo "[INFO] Added fstab entry: $DISK_DEVICE $MOUNT_POINT ext4 defaults,nofail 0 2"
fi

# Create StarClaw data directories
mkdir -p \
  "$MOUNT_POINT/mysql" \
  "$MOUNT_POINT/redis" \
  "$MOUNT_POINT/nydus/repos" \
  "$MOUNT_POINT/nydus/releases" \
  "$MOUNT_POINT/pheromone/nats" \
  "$MOUNT_POINT/pheromone/logs" \
  "$MOUNT_POINT/backups" \
  "$MOUNT_POINT/logs"

echo "[OK] Data disk initialization completed."
findmnt "$MOUNT_POINT" || true
