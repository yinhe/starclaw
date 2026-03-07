# 本地打包上传（排除运行时数据和构建产物）
tar --exclude="node_modules" --exclude=".git" --exclude="claw/data" --exclude="*.tar.gz" --exclude="build" --exclude=".gradle" --exclude=".dart_tool" -czf starclaw.tar.gz -C e:\starclaw .
scp -P 22 starclaw.tar.gz root@starclaw.me:~/

# 服务器上更新
ssh root@starclaw.me "cd /opt/starclaw && tar -xzf ~/starclaw.tar.gz && docker compose -f docker-compose.prod.yml build && docker compose -f docker-compose.prod.yml up -d"

# 更新 nginx 配置 (首次或配置变更时)
# ssh root@starclaw.me "cp /opt/starclaw/claw/deploy/nginx-starclaw.conf /etc/nginx/sites-available/starclaw && nginx -t && systemctl reload nginx"