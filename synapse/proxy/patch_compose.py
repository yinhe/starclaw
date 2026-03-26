#!/usr/bin/env python3
"""Add proxy + redis services to Queen docker-compose.prod.yml"""

path = '/opt/queen/docker-compose.prod.yml'
with open(path) as f:
    content = f.read()

if 'redis-proxy' in content:
    print('SKIP: proxy services already present')
    exit(0)

proxy_services = """
  redis-proxy:
    image: redis:7-alpine
    container_name: starclaw-queen-redis
    restart: unless-stopped
    volumes:
      - redis_proxy_data:/data
    networks:
      - starqueen

  proxy:
    build:
      context: ./proxy
    container_name: starclaw-queen-proxy
    restart: unless-stopped
    env_file:
      - ./proxy/.env
    ports:
      - "127.0.0.1:8000:8000"
    volumes:
      - ./proxy/data:/app/data
    depends_on:
      - redis-proxy
    networks:
      - starqueen
"""

# Insert before 'volumes:' section and add redis_proxy_data volume
content = content.replace(
    '\nvolumes:\n  prometheus_data:\n  grafana_data:',
    proxy_services + '\nvolumes:\n  prometheus_data:\n  grafana_data:\n  redis_proxy_data:'
)

with open(path, 'w') as f:
    f.write(content)
print('OK: added proxy + redis services')
