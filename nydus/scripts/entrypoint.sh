#!/bin/bash
set -e

# Setup SSH authorized_keys from env
if [ -n "$NYDUS_SSH_PUBKEY" ]; then
    echo "$NYDUS_SSH_PUBKEY" > /home/git/.ssh/authorized_keys
    chown git:git /home/git/.ssh/authorized_keys
    chmod 600 /home/git/.ssh/authorized_keys
    echo "[nydus] SSH public key installed for git user"
fi

# If authorized_keys mounted as volume, fix permissions
if [ -f /home/git/.ssh/authorized_keys ]; then
    chown git:git /home/git/.ssh/authorized_keys
    chmod 600 /home/git/.ssh/authorized_keys
fi

# Ensure repos dir ownership
chown -R git:git /data/nydus

# Set git user's home to repos dir so SSH paths resolve correctly
# git clone git@host:queen.git → /data/nydus/repos/queen.git
sed -i "s|/home/git|/data/nydus/repos|g" /etc/passwd
mkdir -p /data/nydus/repos/.ssh
cp /home/git/.ssh/authorized_keys /data/nydus/repos/.ssh/authorized_keys 2>/dev/null || true
chown -R git:git /data/nydus/repos/.ssh
chmod 700 /data/nydus/repos/.ssh
echo "[nydus] git user home set to /data/nydus/repos"

# Start SSH daemon
/usr/sbin/sshd -D &
echo "[nydus] SSH daemon started on port 22"

# Start Nydus server
exec ./nydus-server
