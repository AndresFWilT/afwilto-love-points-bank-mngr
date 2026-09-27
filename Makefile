# Banco de Puntitos — comandos de trabajo.
# Todo corre en contenedores; los tests rápidos corren en la máquina.

SHELL := /bin/bash
COMPOSE := docker compose
COMPOSE_TEST := docker compose -f docker-compose.test.yml
MIGRATIONS := data-base/migrations

# Carga variables de .env si existe (para migrate-new, backup, etc.).
-include .env
export

.DEFAULT_GOAL := help

.PHONY: help up down logs test test-int test-e2e migrate-new lint check backup fmt

help: ## Muestra esta ayuda
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

up: ## Levanta db + migrate + api + web
	$(COMPOSE) up --build

down: ## Apaga y borra volúmenes
	$(COMPOSE) down -v

logs: ## Sigue los logs de la api
	$(COMPOSE) logs -f api

test: ## Unitarios de dominio y aplicación (rápidos, sin Docker)
	cd backend && go test ./internal/domain/... ./internal/app/... && \
		go test -cover ./internal/domain/ ./internal/app/service/
	cd frontend && npm run test

test-int: ## Integración con testcontainers (Postgres real)
	cd backend && go test -tags integration -count=1 ./internal/adapter/out/postgres/...

test-e2e: ## Levanta el compose de test y corre el humo
	$(COMPOSE_TEST) up --build -d
	@echo "esperando a la api..."; \
	for i in $$(seq 1 30); do \
		curl -sf http://localhost:18080/healthz >/dev/null && break || sleep 2; \
	done
	@status=0 ; \
		( cd backend && API_URL=http://localhost:18080 go test -tags e2e -count=1 ./test/e2e/... ) || status=$$? ; \
		$(COMPOSE_TEST) down -v ; \
		exit $$status

migrate-new: ## Crea el par up/down de migración (uso: make migrate-new name=crear_algo)
	@test -n "$(name)" || (echo "falta name=. Ej: make migrate-new name=crear_algo" && exit 1)
	@next=$$(printf "%06d" $$(( $$(ls $(MIGRATIONS) | grep -oE '^[0-9]+' | sort -n | tail -1 | sed 's/^0*//') + 1 ))); \
	touch $(MIGRATIONS)/$${next}_$(name).up.sql $(MIGRATIONS)/$${next}_$(name).down.sql; \
	echo "creadas $(MIGRATIONS)/$${next}_$(name).{up,down}.sql"

fmt: ## Formatea Go y front
	cd backend && go fmt ./...
	cd frontend && npx prettier --write "src/**/*.{ts,tsx,css}" 2>/dev/null || true

lint: ## golangci-lint + eslint (si están instalados)
	cd backend && go vet ./... && (command -v golangci-lint >/dev/null && golangci-lint run || echo "golangci-lint no instalado, solo go vet")
	cd frontend && npm run lint 2>/dev/null || echo "eslint no configurado"

check: ## Lo que corre CI: lint + test + test-int
	$(MAKE) lint
	$(MAKE) test
	$(MAKE) test-int

backup: ## Respaldo de la base (perfil por defecto levantado)
	@mkdir -p backup
	$(COMPOSE) exec -T db pg_dump -U "$(DB_USER)" -Fc "$(DB_NAME)" > backup/$$(date +%F).dump
	@echo "respaldo en backup/$$(date +%F).dump"
