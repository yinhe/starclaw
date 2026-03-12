# StarClaw 🦞 Docker Lifecycle Commands
# Usage: make <command>

COMPOSE = docker compose
COMPOSE_CN = docker compose -f docker-compose.yml -f docker-compose.cn.yml

# Version: prefer git tag, fallback to YYYY.MMDD.HHmm (UTC)
VERSION ?= $(shell git describe --tags --abbrev=0 2>/dev/null | sed 's/^v//' || date -u +"%Y.%m%d.%H%M")
LDFLAGS_API = -X github.com/yinhe/starclaw/internal/molt.Version=$(VERSION)
LDFLAGS_BRIDGE = -X main.version=$(VERSION)

# ======================== Build & Start ========================

.PHONY: up
up: ## Build and start all services (version stamped)
	BUILD_VERSION=$(VERSION) $(COMPOSE) up -d --build

.PHONY: up-cn
up-cn: ## Build and start (China mirror acceleration)
	BUILD_VERSION=$(VERSION) $(COMPOSE_CN) up -d --build

.PHONY: start
start: ## Start existing containers (no rebuild)
	$(COMPOSE) start

.PHONY: stop
stop: ## Stop all containers (keep data)
	$(COMPOSE) stop

.PHONY: restart
restart: ## Restart all containers
	$(COMPOSE) restart

.PHONY: down
down: ## Stop and remove containers + network (keep data volumes)
	$(COMPOSE) down

.PHONY: destroy
destroy: ## Stop, remove containers, AND delete all data ⚠️
	$(COMPOSE) down
	rm -rf data/mysql data/redis data/sandbox

# ======================== Logs ========================

.PHONY: logs
logs: ## Follow all service logs
	$(COMPOSE) logs -f --tail 100

.PHONY: logs-api
logs-api: ## Follow API service logs
	$(COMPOSE) logs -f --tail 200 api

.PHONY: logs-web
logs-web: ## Follow Web service logs
	$(COMPOSE) logs -f --tail 100 web

.PHONY: logs-mysql
logs-mysql: ## Follow MySQL logs
	$(COMPOSE) logs -f --tail 100 mysql

.PHONY: logs-redis
logs-redis: ## Follow Redis logs
	$(COMPOSE) logs -f --tail 100 redis

# ======================== Status ========================

.PHONY: ps
ps: ## Show running containers
	$(COMPOSE) ps

.PHONY: stats
stats: ## Show container resource usage (CPU/MEM)
	docker stats --no-stream starclaw-api starclaw-web starclaw-mysql starclaw-redis

.PHONY: health
health: ## Check API health endpoint
	@curl -sf http://localhost:8080/health | python3 -m json.tool 2>/dev/null || echo "API not reachable"

# ======================== Update ========================

.PHONY: write-version
write-version: ## Write .version file for Docker build
	@echo $(VERSION) > api/.version
	@echo "✓ api/.version = $(VERSION)"

.PHONY: update
update: ## Pull latest code, rebuild API+Web only (data safe)
	git pull origin main
	@echo "Building API..."
	BUILD_VERSION=$(VERSION) $(COMPOSE) build api
	@echo "Building Web..."
	BUILD_VERSION=$(VERSION) $(COMPOSE) build web
	@echo "Restarting API+Web..."
	$(COMPOSE) up -d --no-deps api web
	@sleep 3
	@$(MAKE) verify

.PHONY: update-cn
update-cn: ## Pull latest code, rebuild API+Web only (China mirror)
	git pull origin main
	@echo "Building API..."
	BUILD_VERSION=$(VERSION) $(COMPOSE_CN) build api
	@echo "Building Web..."
	BUILD_VERSION=$(VERSION) $(COMPOSE_CN) build web
	@echo "Restarting API+Web..."
	$(COMPOSE_CN) up -d --no-deps api web
	@sleep 3
	@$(MAKE) verify

.PHONY: verify
verify: ## Verify API and Web are healthy after update
	@echo "Checking API..."
	@curl -sf http://localhost:8080/v1/setup/status > /dev/null && echo "  API: OK" || echo "  API: FAILED"
	@echo "Checking Web..."
	@curl -sf http://localhost:$${WEB_PORT:-3080}/ > /dev/null && echo "  Web: OK" || echo "  Web: FAILED"
	@$(COMPOSE) ps --format 'table {{.Name}}\t{{.Status}}'

# ======================== Rebuild Single Service ========================

.PHONY: rebuild-api
rebuild-api: ## Rebuild and restart only the API service
	BUILD_VERSION=$(VERSION) $(COMPOSE) up -d --build --no-deps api

.PHONY: rebuild-web
rebuild-web: ## Rebuild and restart only the Web service
	BUILD_VERSION=$(VERSION) $(COMPOSE) up -d --build --no-deps web

# ======================== Backup & Restore ========================

.PHONY: backup
backup: ## Backup MySQL + data directory
	@mkdir -p backups
	docker exec starclaw-mysql mysqldump -uroot -p"$${DB_PASSWORD:-starclaw}" starclaw > backups/db_$(shell date +%Y%m%d_%H%M%S).sql
	tar -czf backups/data_$(shell date +%Y%m%d_%H%M%S).tar.gz data/
	@echo "✓ Backup saved to backups/"

.PHONY: restore-db
restore-db: ## Restore MySQL from latest backup (usage: make restore-db FILE=backups/db_xxx.sql)
	@test -n "$(FILE)" || (echo "Usage: make restore-db FILE=backups/db_xxx.sql" && exit 1)
	docker exec -i starclaw-mysql mysql -uroot -p"$${DB_PASSWORD:-starclaw}" starclaw < $(FILE)
	@echo "✓ Database restored from $(FILE)"

# ======================== Shell Access ========================

.PHONY: shell-api
shell-api: ## Open shell in API container
	docker exec -it starclaw-api sh

.PHONY: shell-mysql
shell-mysql: ## Open MySQL CLI
	docker exec -it starclaw-mysql mysql -uroot -p"$${DB_PASSWORD:-starclaw}" starclaw

.PHONY: shell-redis
shell-redis: ## Open Redis CLI
	docker exec -it starclaw-redis redis-cli

# ======================== Cleanup ========================

.PHONY: prune
prune: ## Remove unused Docker images and build cache
	docker image prune -f
	docker builder prune -f
	@echo "✓ Cleaned up unused images and build cache"

# ======================== Init ========================

.PHONY: init
init: ## First-time setup: create data dirs + copy .env
	mkdir -p data/{mysql,redis,sandbox,merged_videos,thumbnails,music,images,workspaces}
	@test -f .env || cp .env.example .env && echo "✓ .env created from .env.example — please edit it"
	@echo "✓ Data directories ready. Run 'make up' to start."

# ======================== Versioned Build ========================

.PHONY: version
version: ## Show current build version
	@echo $(VERSION)

.PHONY: build-api
build-api: ## Build API binary with version stamp
	cd api && CGO_ENABLED=0 go build -ldflags '$(LDFLAGS_API)' -o ../starclaw-api ./cmd/server/
	@echo "✓ Built starclaw-api ($(VERSION))"

.PHONY: tag
tag: ## Create git tag with timestamp version (usage: make tag)
	@echo $(VERSION) > api/.version
	@git add api/.version && git commit -m "chore: bump version to $(VERSION)" --allow-empty || true
	@echo "Tagging v$(VERSION)..."
	git tag -a "v$(VERSION)" -m "Release $(VERSION)"
	@echo "✓ Tagged v$(VERSION). Push with: git push origin v$(VERSION)"

# ======================== MCP Bridge (Host Control) ========================

.PHONY: bridge
bridge: ## Build MCP Bridge binary for current OS
	cd api && go build -ldflags '$(LDFLAGS_BRIDGE)' -o ../mcp-bridge ./cmd/mcp-bridge/
	@echo "✓ Built mcp-bridge $(VERSION). Run: ./mcp-bridge -port 9100"

.PHONY: bridge-linux
bridge-linux: ## Cross-compile MCP Bridge for Linux (server deployment)
	cd api && GOOS=linux GOARCH=amd64 go build -ldflags '$(LDFLAGS_BRIDGE)' -o ../mcp-bridge-linux ./cmd/mcp-bridge/
	@echo "✓ Built mcp-bridge-linux $(VERSION)"

.PHONY: bridge-start
bridge-start: bridge ## Build and start MCP Bridge
	./mcp-bridge -port 9100

.PHONY: bridge-install
bridge-install: bridge-linux ## Deploy MCP Bridge to server as systemd service
	scp mcp-bridge-linux root@starclaw.me:/usr/local/bin/mcp-bridge
	scp deploy/mcp-bridge.service root@starclaw.me:/etc/systemd/system/
	ssh root@starclaw.me "chmod +x /usr/local/bin/mcp-bridge && systemctl daemon-reload && systemctl enable --now mcp-bridge"
	@echo "✓ MCP Bridge installed and running on server"

# ======================== CLI Commands (run inside API container) ========================

.PHONY: get-token
get-token: ## Show current Owner Token (read-only)
	docker exec starclaw-api ./starclaw get-token

.PHONY: reset-token
reset-token: ## Reset Owner Token (prints new token)
	docker exec starclaw-api ./starclaw reset-token

.PHONY: reset-password
reset-password: ## Reset Owner password (usage: make reset-password PASS=newpass123)
	@test -n "$(PASS)" || (echo "Usage: make reset-password PASS=<new-password>" && exit 1)
	docker exec starclaw-api ./starclaw reset-password --password $(PASS)

.PHONY: devices
devices: ## List all authorized devices
	docker exec starclaw-api ./starclaw devices

.PHONY: approve
approve: ## Approve a pending device (usage: make approve ID=a1b2c3d4)
	@test -n "$(ID)" || (echo "Usage: make approve ID=<device-id-prefix>" && exit 1)
	docker exec starclaw-api ./starclaw approve $(ID)

.PHONY: reject
reject: ## Reject/revoke a device (usage: make reject ID=a1b2c3d4)
	@test -n "$(ID)" || (echo "Usage: make reject ID=<device-id-prefix>" && exit 1)
	docker exec starclaw-api ./starclaw reject $(ID)

.PHONY: export-key
export-key: ## Export node identity key (for backup)
	docker exec starclaw-api ./starclaw export-key

.PHONY: import-key
import-key: ## Import node identity key (usage: make import-key SEED=<hex>)
	@test -n "$(SEED)" || (echo "Usage: make import-key SEED=<64-char-hex>" && exit 1)
	docker exec -it starclaw-api ./starclaw import-key $(SEED)

.PHONY: api-version
api-version: ## Show API binary version inside container
	docker exec starclaw-api ./starclaw version

.PHONY: install-cli
install-cli: ## Install starclaw & claw commands to /usr/local/bin
	cp scripts/starclaw-cli.sh /usr/local/bin/starclaw
	chmod +x /usr/local/bin/starclaw
	ln -sf /usr/local/bin/starclaw /usr/local/bin/claw
	@echo "✓ Installed: starclaw & claw → /usr/local/bin/"

# ======================== Help ========================

.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-16s\033[0m %s\n", $$1, $$2}'

.DEFAULT_GOAL := help
