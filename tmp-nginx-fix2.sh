#!/bin/bash
# Fix: partner and city /api/ should proxy to Queen /v1/ (not /api/v1/)

sed -i 's|proxy_pass http://127.0.0.1:8085/api/v1/;|proxy_pass http://127.0.0.1:8085/v1/;|g' /etc/nginx/sites-enabled/queen

nginx -t 2>&1 | tail -2 && nginx -s reload 2>&1 | tail -1
echo "done"
