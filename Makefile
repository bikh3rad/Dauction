# Dauction — root convenience targets.
# Per-service builds/tests live in each services/<name>/Makefile (go-template).

COMPOSE := docker compose -f deploy/docker-compose.yml

.PHONY: help up down logs ps check check-i18n check-proto vet test

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

## ---- local infra ----
up: ## Bring up pg-per-service + NATS(JetStream) + Jaeger
	$(COMPOSE) up -d

down: ## Tear down local infra (keeps volumes)
	$(COMPOSE) down

logs: ## Tail infra logs
	$(COMPOSE) logs -f

ps: ## Show infra status
	$(COMPOSE) ps

## ---- checks (CI) ----
check: check-i18n check-proto vet ## Run all repo-level checks

check-i18n: ## Assert all four i18n catalogs have an identical key set
	@node i18n/check_keys.mjs

check-proto: ## Lint the frozen proto contract (skipped if `buf` is absent)
	@if command -v buf >/dev/null 2>&1; then \
		cd proto && buf lint; \
	else \
		echo "… buf not installed — skipping proto lint (install: https://buf.build)"; \
	fi

vet: ## go vet across every service (no-op until services exist)
	@found=0; \
	for d in services/*/; do \
		if [ -f "$$d/go.mod" ]; then found=1; echo "vet $$d"; (cd "$$d" && go vet ./...) || exit 1; fi; \
	done; \
	[ $$found -eq 0 ] && echo "… no services yet — skipping go vet" || true

test: ## go test across every service (no-op until services exist)
	@for d in services/*/; do \
		if [ -f "$$d/go.mod" ]; then echo "test $$d"; (cd "$$d" && go test ./...) || exit 1; fi; \
	done
