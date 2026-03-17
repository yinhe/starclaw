#!/bin/bash
set -e

NGINX_CONF="/etc/nginx/sites-enabled/queen"

# 1. Add missing subdomains to HTTP→HTTPS redirect block
# Current: server_name starclaw.net www.starclaw.net api.starclaw.net swarm.starclaw.net core.starclaw.net bounty.starclaw.net forum.starclaw.net arena.starclaw.net grafana.starclaw.net overseer.starclaw.net;
# Need to add: partner.starclaw.net city.starclaw.net nydus.starclaw.net proxy.starclaw.net
sed -i 's/overseer\.starclaw\.net;/overseer.starclaw.net\n                partner.starclaw.net city.starclaw.net nydus.starclaw.net proxy.starclaw.net;/' "$NGINX_CONF"

# 2. Append missing HTTPS server blocks
cat /tmp/nginx-missing-subdomains.conf >> "$NGINX_CONF"

# 3. Test nginx config
nginx -t
if [ $? -ne 0 ]; then
    echo "ERROR: nginx config test failed!"
    exit 1
fi

# 4. Reload nginx
nginx -s reload
echo "nginx reloaded with new subdomains"

# 5. Verify
sleep 1
for d in partner.starclaw.net city.starclaw.net nydus.starclaw.net proxy.starclaw.net; do
    code=$(curl -sk -o /dev/null -w '%{http_code}' "https://$d/" 2>/dev/null)
    echo "$code $d"
done
