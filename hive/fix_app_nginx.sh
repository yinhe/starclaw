#!/bin/bash
# Fix app.starclaw.me nginx config: add /v1/ API routing + CORS + /health

CONF="/etc/nginx/sites-enabled/starclaw"

# Find the line number of "client_max_body_size 100M;" in the app.starclaw.me block
# We need to insert the /v1/ and /health blocks BEFORE "location / {" in that server block

# First, find the app.starclaw.me server block start
APP_START=$(grep -n '# ========== app.starclaw.me' "$CONF" | head -1 | cut -d: -f1)
if [ -z "$APP_START" ]; then
    echo "ERROR: Could not find app.starclaw.me block"
    exit 1
fi

# Find "location / {" after APP_START (the catch-all)
LOCATION_LINE=$(tail -n +$APP_START "$CONF" | grep -n 'location / {' | head -1 | cut -d: -f1)
INSERT_LINE=$((APP_START + LOCATION_LINE - 2))

echo "Inserting API routes at line $INSERT_LINE (app block starts at $APP_START)"

# Create the API routing block
cat > /tmp/api_block.txt << 'BLOCK'
    # API endpoints — proxy to Claw API (with CORS for cross-origin auth)
    location /v1/ {
        if ($request_method = OPTIONS) {
            add_header 'Access-Control-Allow-Origin' $http_origin always;
            add_header 'Access-Control-Allow-Methods' 'GET, POST, PUT, DELETE, OPTIONS' always;
            add_header 'Access-Control-Allow-Headers' 'Authorization, Content-Type' always;
            add_header 'Access-Control-Allow-Credentials' 'true' always;
            add_header 'Access-Control-Max-Age' 86400;
            return 204;
        }
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        add_header 'Access-Control-Allow-Origin' $http_origin always;
        add_header 'Access-Control-Allow-Credentials' 'true' always;
    }

    location = /health {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        add_header 'Access-Control-Allow-Origin' $http_origin always;
    }

BLOCK

# Insert the block before the location / line
head -n $INSERT_LINE "$CONF" > /tmp/starclaw_fixed.conf
cat /tmp/api_block.txt >> /tmp/starclaw_fixed.conf
tail -n +$((INSERT_LINE + 1)) "$CONF" >> /tmp/starclaw_fixed.conf

cp /tmp/starclaw_fixed.conf "$CONF"
echo "Config updated. Testing..."
nginx -t 2>&1
