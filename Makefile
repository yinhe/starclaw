# StarClaw 🦞 Docker Lifecycle Commands
# Usage: make <command>

COMPOSE = docker compose
COMPOSE_CN = docker compose -f docker-compose.yml -f docker-compose.cn.yml

# ======================== Build & Start ========================

.PHONY: up
up: ## Build and start all services
	$(COMPOSE) up -d --build

.PHONY: up-cn
up-cn: ## Build and start (China mirror acceleration)
	$(COMPOSE_CN) up -d --build

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

.PHONY: update
update: ## Pull latest code and rebuild
	git pull
	$(COMPOSE) up -d --build

.PHONY: update-cn
update-cn: ## Pull latest code and rebuild (China mirror)
	git pull
	$(COMPOSE_CN) up -d --build

# ======================== Rebuild Single Service ========================

.PHONY: rebuild-api
rebuild-api: ## Rebuild and restart only the API service
	$(COMPOSE) up -d --build --no-deps api

.PHONY: rebuild-web
rebuild-web: ## Rebuild and restart only the Web service
	$(COMPOSE) up -d --build --no-deps web

# ======================== Backup & Restore ========================

.PHONY: backup
backup: ## Backup MySQL + data directory
	@mkdir -p backups
	docker exec starclaw-mysql mysqldump -uroot -pstarclaw starclaw > backups/db_$(shell date +%Y%m%d_%H%M%S).sql
	tar -czf backups/data_$(shell date +%Y%m%d_%H%M%S).tar.gz data/
	@echo "✓ Backup saved to backups/"

.PHONY: restore-db
restore-db: ## Restore MySQL from latest backup (usage: make restore-db FILE=backups/db_xxx.sql)
	@test -n "$(FILE)" || (echo "Usage: make restore-db FILE=backups/db_xxx.sql" && exit 1)
	docker exec -i starclaw-mysql mysql -uroot -pstarclaw starclaw < $(FILE)
	@echo "✓ Database restored from $(FILE)"

# ======================== Shell Access ========================

.PHONY: shell-api
shell-api: ## Open shell in API container
	docker exec -it starclaw-api sh

.PHONY: shell-mysql
shell-mysql: ## Open MySQL CLI
	docker exec -it starclaw-mysql mysql -uroot -pstarclaw starclaw

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

# ======================== Help ========================

.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-16s\033[0m %s\n", $$1, $$2}'

.DEFAULT_GOAL := help
