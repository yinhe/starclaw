#!/bin/bash
echo "=== SSL Certificate ==="
openssl x509 -in /etc/letsencrypt/live/starclaw.net/fullchain.pem -noout -dates -subject 2>&1

echo ""
echo "=== All Subdomains (HTTPS from outside via curl) ==="
for d in starclaw.net api.starclaw.net swarm.starclaw.net core.starclaw.net bounty.starclaw.net forum.starclaw.net arena.starclaw.net overseer.starclaw.net partner.starclaw.net city.starclaw.net nydus.starclaw.net proxy.starclaw.net grafana.starclaw.net; do
    code=$(curl -sk -o /dev/null -w '%{http_code}' "https://$d/" 2>/dev/null)
    echo "  $code  $d"
done

echo ""
echo "=== Docker containers ==="
docker ps --format '{{.Names}}\t{{.Status}}' 2>/dev/null | sort
