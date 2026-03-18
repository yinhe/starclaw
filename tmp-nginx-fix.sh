#!/bin/bash
# Fix: partner and city /api/ should proxy to Queen /api/v1/

# Partner: change proxy_pass from /api/ to /api/v1/
sed -i '/server_name partner.starclaw.net;/,/^}/ {
  s|proxy_pass http://127.0.0.1:8085/api/;|proxy_pass http://127.0.0.1:8085/api/v1/;|
}' /etc/nginx/sites-enabled/queen

# City: change proxy_pass from /api/ to /api/v1/
sed -i '/server_name city.starclaw.net;/,/^}/ {
  s|proxy_pass http://127.0.0.1:8085/api/;|proxy_pass http://127.0.0.1:8085/api/v1/;|
}' /etc/nginx/sites-enabled/queen

nginx -t 2>&1 | tail -2 && nginx -s reload 2>&1 | tail -1
echo "done"
