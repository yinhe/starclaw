# StarClaw Queen 🦞 — Production Operations
# Usage: make <command>

COMPOSE = docker compose -f docker-compose.prod.yml

.PHONY: deploy
deploy: ## Full deployment (SSL + Nginx + build + start)
	bash deploy/deploy.sh

.PHONY: up
up: ## Build and start all Queen services
	$(COMPOSE) up -d --build

.PHONY: down
down: ## Stop and remove containers
	$(COMPOSE) down

.PHONY: restart
restart: ## Restart all services
	$(COMPOSE) restart

.PHONY: logs
logs: ## Follow all logs
	$(COMPOSE) logs -f --tail 100

.PHONY: logs-swarm
logs-swarm: ## Follow swarm logs
	$(COMPOSE) logs -f --tail 200 swarm

.PHONY: ps
ps: ## Show running containers
	$(COMPOSE) ps

.PHONY: health
health: ## Check all service health endpoints
	@echo "Swarm:  $$(curl -sf http://127.0.0.1:8090/health || echo 'DOWN')"
	@echo "Core:   $$(curl -sf http://127.0.0.1:8091/health || echo 'DOWN')"
	@echo "Bounty: $$(curl -sf http://127.0.0.1:8092/health || echo 'DOWN')"
	@echo "Forum:  $$(curl -sf http://127.0.0.1:8093/health || echo 'DOWN')"
	@echo "Arena:  $$(curl -sf http://127.0.0.1:8094/health || echo 'DOWN')"

.PHONY: rebuild
rebuild: ## Rebuild all services
	$(COMPOSE) up -d --build --force-recreate

.PHONY: update
update: ## Pull latest code and rebuild
	git pull
	$(COMPOSE) up -d --build

.PHONY: backup
backup: ## Backup MySQL
	@mkdir -p backups
	docker exec starclaw-queen-mysql mysqldump -uroot -p$(QUEEN_DB_PASSWORD) starclaw_queen > backups/queen_$$(date +%Y%m%d_%H%M%S).sql
	@echo "✓ Backup saved"

.PHONY: shell-mysql
shell-mysql: ## Open MySQL CLI
	docker exec -it starclaw-queen-mysql mysql -uroot -p$(QUEEN_DB_PASSWORD) starclaw_queen

.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-16s\033[0m %s\n", $$1, $$2}'

.DEFAULT_GOAL := help
