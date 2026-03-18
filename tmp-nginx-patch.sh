#!/bin/bash
# Patch partner.starclaw.net and city.starclaw.net nginx to proxy /api/ to Queen API

# Partner: add /api/ proxy before the catch-all location /
sed -i '/server_name partner.starclaw.net;/,/^}/ {
  /location \/ {/i\
    location /api/ {\
        proxy_pass http://127.0.0.1:8085/api/;\
        proxy_set_header Host $host;\
        proxy_set_header X-Real-IP $remote_addr;\
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;\
        proxy_set_header X-Forwarded-Proto $scheme;\
    }\

}' /etc/nginx/sites-enabled/queen

# City: add /api/ proxy before the catch-all location /
sed -i '/server_name city.starclaw.net;/,/^}/ {
  /location \/ {/i\
    location /api/ {\
        proxy_pass http://127.0.0.1:8085/api/;\
        proxy_set_header Host $host;\
        proxy_set_header X-Real-IP $remote_addr;\
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;\
        proxy_set_header X-Forwarded-Proto $scheme;\
    }\

}' /etc/nginx/sites-enabled/queen

nginx -t && nginx -s reload
