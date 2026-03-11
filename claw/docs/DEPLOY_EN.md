# StarClaw 🦞 Deploy Guide

## 1. Server Requirements

| Item | Minimum | Recommended |
|------|---------|-------------|
| CPU | 2 cores | 4 cores |
| RAM | 4 GB | 8 GB |
| Disk | 40 GB SSD | 100 GB SSD |
| OS | Ubuntu 22.04 / CentOS 8+ | Ubuntu 22.04 LTS |

> The backend image includes Chromium, FFmpeg, Python, and Node.js — build size ~1.5GB.

## 2. Install Docker

```bash
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER
newgrp docker
```

## 3. Deploy

### 3.1 Get the Code

```bash
git clone https://github.com/yinhe/starclaw.git
cd starclaw
```

### 3.2 Configure Environment

```bash
cp .env.example .env
nano .env
```

**Required changes:**
```
JWT_SECRET=<openssl rand -hex 32>
DB_ROOT_PASSWORD=<strong-password>
REDIS_PASSWORD=<strong-password>
```

### 3.3 Launch

```bash
# Create data directories
mkdir -p data/{merged_videos,thumbnails,music,images,workspaces}

# Build & start (first time ~5-10 min)
docker compose up -d --build
```

### 3.4 Verify

```bash
docker compose ps                        # All services should show Up
curl http://localhost/v1/health           # Should return OK
```

Visit `http://your-server-ip` in your browser.

## 4. Configure AI Models

Go to Web UI → Settings → Models → Add your API keys:

| Provider | Get Key |
|----------|---------|
| Qwen | https://dashscope.console.aliyun.com |
| OpenAI | https://platform.openai.com/api-keys |
| DeepSeek | https://platform.deepseek.com |
| Anthropic | https://console.anthropic.com |
| Ollama (local) | No key needed, just provide Ollama URL |
| OpenRouter | https://openrouter.ai/keys |

## 5. Join the Swarm (Optional)

By default your Claw runs standalone. To join the swarm network:

Edit `api/configs/config.yaml`:
```yaml
server:
  node_role: claw
  queen_url: "https://api.starclaw.me"   # Connect to Queen
  auto_update: true                       # Auto-receive Molt updates
```

After restarting, your Claw will auto-register to the swarm and get:
- 🔄 Auto version updates (Molt)
- 📦 Shared Agent/Workflow templates (Creep)
- 💰 Bounty task publishing capability

## 6. Upgrade to Overlord (Optional)

If you need to manage multiple Claw nodes (enterprise):

```yaml
server:
  node_role: overlord    # Enable Overlord management (requires overlord/ package)
```

Overlord nodes can:
- Manage subordinate Claw nodes
- Internal load balancing
- Nydus P2P tunnels for direct Claw-to-Claw connections

## 7. Domain + HTTPS

```bash
# Install certbot
sudo apt install -y certbot nginx

# Get certificate
sudo certbot certonly --standalone -d your-domain.com

# Configure Nginx reverse proxy
sudo nano /etc/nginx/sites-available/starclaw
```

```nginx
server {
    listen 80;
    server_name your-domain.com;
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    server_name your-domain.com;

    ssl_certificate /etc/letsencrypt/live/your-domain.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/your-domain.com/privkey.pem;
    client_max_body_size 100M;

    location / {
        proxy_pass http://127.0.0.1:8081;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    location /api/ {
        proxy_pass http://127.0.0.1:8081;
        proxy_buffering off;
        proxy_cache off;
        proxy_http_version 1.1;
        proxy_set_header Connection '';
        proxy_read_timeout 600s;
    }
}
```

```bash
sudo ln -s /etc/nginx/sites-available/starclaw /etc/nginx/sites-enabled/
sudo nginx -t && sudo systemctl reload nginx
```

## 8. Operations

```bash
# Update
git pull && docker compose up -d --build

# View logs
docker logs -f starclaw-api --tail 100

# Backup database
docker exec starclaw-mysql mysqldump -uroot -p$DB_ROOT_PASSWORD starclaw > backup.sql

# Backup data
tar -czf data-backup.tar.gz data/

# Restart
docker compose restart
```

## 9. FAQ

**Q: Build is too slow?**
Subsequent builds use Docker cache and will be much faster.

**Q: Out of memory?**
```bash
sudo fallocate -l 4G /swapfile
sudo chmod 600 /swapfile && sudo mkswap /swapfile && sudo swapon /swapfile
```

**Q: Can I still use it after disconnecting from the swarm?**
Yes. In Feral mode all AI features work normally — you only lose auto-updates and shared knowledge. Reconnecting restores everything automatically.

## 10. Team Agent Release SOP (R&D + DevOps)

When using the Team Agent in the **Agents** page (e.g. `研发DevOps团队`), use this release sequence:

1. `deploy_web`: trigger preview/production deployment
2. `bind_domain`: create or update DNS records (Cloudflare)
3. `verify_online`: check online availability + keyword acceptance

### Approval Gates (Recommended)

- Confirm once before production deployment
- Confirm again before DNS changes

### Team Agent Prompt Template (Copy/Paste)

```text
Please run the release workflow with the R&D + DevOps team:
1) deploy_web for production
2) bind_domain for app.example.com
3) verify_online for https://app.example.com

Requirements: ask for approval before production release and before DNS changes; if verification fails, provide rollback suggestions.
```

### bind_domain Example (Cloudflare)

```json
{
  "action": "upsert",
  "provider": "cloudflare",
  "api_token": "<CLOUDFLARE_API_TOKEN>",
  "zone_id": "<ZONE_ID>",
  "record_type": "CNAME",
  "record_name": "app.example.com",
  "record_value": "cname.vercel-dns.com",
  "proxied": "false",
  "ttl": "120"
}
```

> Security note: pass `api_token` at runtime only. Do not commit it into repository files.
