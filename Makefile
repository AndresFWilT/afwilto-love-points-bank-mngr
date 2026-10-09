# Banco de Puntitos — love-points manager (backend + migraciones).
# El frontend vive como MFE en afwilto-platform-web-ui; la plataforma completa
# se levanta con el docker-compose de afwilto-platform-iac.

SHELL := /bin/bash
COMPOSE_TEST := docker compose -f docker-compose.test.yml
MIGRATIONS := migrations

.DEFAULT_GOAL := help

.PHONY: help test test-int test-e2e migrate-new lint check fmt

help: ## Muestra esta ayuda
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

test: ## Unitarios de dominio y aplicación (rápidos, sin Docker)
	go test ./internal/domain/... ./internal/app/... && \
		go test -cover ./internal/domain/ ./internal/app/service/

test-int: ## Integración con testcontainers (Postgres real)
	go test -tags integration -count=1 ./internal/adapter/out/postgres/...

test-e2e: ## Levanta el compose de test y corre el humo
	$(COMPOSE_TEST) up --build -d
	@echo "esperando a la api..."; \
	for i in $$(seq 1 30); do \
		curl -sf http://localhost:18080/healthz >/dev/null && break || sleep 2; \
	done
	@status=0 ; \
		API_URL=http://localhost:18080 go test -tags e2e -count=1 ./test/e2e/... || status=$$? ; \
		$(COMPOSE_TEST) down -v ; \
		exit $$status

migrate-new: ## Crea el par up/down de migración (uso: make migrate-new name=crear_algo)
	@test -n "$(name)" || (echo "falta name=. Ej: make migrate-new name=crear_algo" && exit 1)
	@next=$$(printf "%06d" $$(( $$(ls $(MIGRATIONS) | grep -oE '^[0-9]+' | sort -n | tail -1 | sed 's/^0*//') + 1 ))); \
	touch $(MIGRATIONS)/$${next}_$(name).up.sql $(MIGRATIONS)/$${next}_$(name).down.sql; \
	echo "creadas $(MIGRATIONS)/$${next}_$(name).{up,down}.sql"

fmt: ## Formatea Go
	go fmt ./...

lint: ## go vet + golangci-lint (si está instalado)
	go vet ./... && (command -v golangci-lint >/dev/null && golangci-lint run || echo "golangci-lint no instalado, solo go vet")

check: ## Lo que corre CI: lint + test + test-int
	$(MAKE) lint
	$(MAKE) test
	$(MAKE) test-int
