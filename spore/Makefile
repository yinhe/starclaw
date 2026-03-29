# Spore — StarClaw Ultra-Lightweight Deployment Runtime
# Usage: make <command>

# Version: prefer git tag, fallback to YYYY.MMDD.HHmm (UTC)
VERSION ?= $(shell git describe --tags --abbrev=0 2>/dev/null | sed 's/^v//' || date -u +"%Y.%m%d.%H%M")
LDFLAGS_SPORE = -s -w -X main.version=$(VERSION)
LDFLAGS_CLAW  = -s -w -X github.com/yinhe/starclaw/internal/molt.Version=$(VERSION)

# ======================== Build ========================

.PHONY: spore
spore: ## Build spore CLI
	cd . && CGO_ENABLED=0 go build -ldflags '$(LDFLAGS_SPORE)' -o spore ./cmd/spore/
	@echo "✓ Built spore ($(VERSION))"

.PHONY: desktop
desktop: ## Build spore-desktop GUI
	cd . && CGO_ENABLED=0 go build -ldflags '$(LDFLAGS_SPORE)' -o spore-desktop ./cmd/desktop/
	@echo "✓ Built spore-desktop ($(VERSION))"

.PHONY: hatchery
hatchery: ## Build hatchery (spore packaging tool)
	cd . && CGO_ENABLED=0 go build -ldflags '-s -w -X main.version=$(VERSION)' -o hatchery ./cmd/hatchery/
	@echo "✓ Built hatchery ($(VERSION))"

.PHONY: all
all: spore desktop hatchery ## Build all binaries
	@echo "✓ All binaries built ($(VERSION))"

# ======================== Cross-compile ========================

.PHONY: release
release: ## Cross-compile spore for all platforms
	@mkdir -p dist
	@for pair in linux:amd64 linux:arm64 darwin:amd64 darwin:arm64 windows:amd64; do \
		os_name=$${pair%%:*}; \
		arch_name=$${pair##*:}; \
		ext=""; [ "$$os_name" = "windows" ] && ext=".exe"; \
		outname="spore-$$os_name-$$arch_name$$ext"; \
		echo "Building $$outname (v$(VERSION))..."; \
		GOOS=$$os_name GOARCH=$$arch_name CGO_ENABLED=0 \
			go build -ldflags '$(LDFLAGS_SPORE)' -o dist/$$outname ./cmd/spore/ || true; \
	done
	@echo "✓ Release binaries in dist/"

# ======================== Version ========================

.PHONY: version
version: ## Show current build version
	@echo $(VERSION)

# ======================== Help ========================

.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-16s\033[0m %s\n", $$1, $$2}'

.DEFAULT_GOAL := help
