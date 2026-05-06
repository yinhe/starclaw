#!/bin/sh
# Setup nginx static file serving for cdn.starclaw.net
cat > /etc/nginx/conf.d/cdn.starclaw.net.conf << 'EOF'
server {
    listen 80;
    server_name cdn.starclaw.net;

    root /opt/cdn;
    autoindex off;

    location / {
        try_files $uri $uri/ =404;
        add_header Access-Control-Allow-Origin *;
        add_header Cache-Control "public, max-age=31536000, immutable";
        expires max;
    }
}
EOF

nginx -t && nginx -s reload && echo "NGINX_OK" || echo "NGINX_FAIL"
