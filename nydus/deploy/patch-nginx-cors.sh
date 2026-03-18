#!/bin/bash
# Patch nydus.starclaw.net nginx config to add CORS for /releases/
CONF=/etc/nginx/sites-enabled/queen

# Insert /releases/ location block BEFORE the catch-all location / in the nydus server block
python3 << 'PYEOF'
with open("/etc/nginx/sites-enabled/queen", "r") as f:
    content = f.read()

patch = """    # CORS-enabled releases endpoint (for browser version check)
    location /releases/ {
        proxy_pass http://127.0.0.1:8095;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        add_header Access-Control-Allow-Origin * always;
        add_header Access-Control-Allow-Methods "GET, OPTIONS" always;
        if ($request_method = OPTIONS) {
            return 204;
        }
    }
"""

# Find the nydus server block's catch-all location /
marker = "server_name nydus.starclaw.net;"
idx = content.find(marker)
if idx < 0:
    print("ERROR: nydus server block not found")
    exit(1)

# Find "    location / {" after the nydus server_name
loc_search = "    location / {"
loc_idx = content.find(loc_search, idx)
if loc_idx < 0:
    print("ERROR: location / not found in nydus block")
    exit(1)

# Check if already patched
if "location /releases/" in content[idx:loc_idx+200]:
    print("ALREADY PATCHED")
    exit(0)

new_content = content[:loc_idx] + patch + "\n" + content[loc_idx:]
with open("/etc/nginx/sites-enabled/queen", "w") as f:
    f.write(new_content)
print("PATCHED OK")
PYEOF

nginx -t && systemctl reload nginx && echo "NGINX RELOADED OK"
