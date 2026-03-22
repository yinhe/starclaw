#!/bin/bash
# Reset admin password to admin123
# SHA256("admin123") = 240be518fabd2724ddb6f04eeb1da5967448d7e831c08c8fa822809f74c720a9
HASH=$(echo -n "admin123" | sha256sum | awk '{print $1}')
echo "Setting admin password_hash to: $HASH"

docker exec starclaw-overlord-mysql mysql -uroot -p"OverlordDb!2026kPmW" starclaw_overlord \
  -e "UPDATE admin_users SET password_hash='$HASH', token_hash=NULL WHERE username='admin';"

echo "Verifying:"
docker exec starclaw-overlord-mysql mysql -uroot -p"OverlordDb!2026kPmW" starclaw_overlord \
  -e "SELECT username, password_hash, token_hash, last_login_at FROM admin_users;"
