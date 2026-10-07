SHELL := /bin/bash
API_DIR := apps/api

-include .env
# Worktrees override it in their own .env to run next to the main checkout.
WEB_PORT ?= 3000
export

.PHONY: up down jaeger logs psql api-run api-build api-test api-lint api-generate api-openapi web-install web-dev web-test web-lint web-e2e migrate-up migrate-down migrate-status migrate-new tunnel tunnel-down

## Infrastructure
up: ## Start Postgres and Mailpit
	docker compose up -d --wait

down: ## Stop local services
	docker compose --profile observability down

jaeger: ## Start the local trace viewer (http://localhost:16686)
	docker compose --profile observability up -d jaeger

## Tunnel (Tailscale Funnel): exposes the API so Claude.ai/ChatGPT can reach
## the MCP server. Set PUBLIC_URL in .env to the printed https URL.
tunnel: ## Expose the API port publicly over HTTPS
	tailscale funnel --bg $(PORT)

tunnel-down: ## Stop exposing the API
	tailscale funnel reset

logs:
	docker compose logs -f

psql:
	docker compose exec postgres psql -U finance -d finance

## API
api-run:
	cd $(API_DIR) && go run ./cmd/api serve

api-build:
	cd $(API_DIR) && CGO_ENABLED=0 go build -o bin/api ./cmd/api

api-test:
	cd $(API_DIR) && go test -race -count=1 ./...

api-generate: ## Regenerate sqlc code from internal/db/queries
	cd $(API_DIR) && go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate

api-lint:
	cd $(API_DIR) && go vet ./... && test -z "$$(gofmt -l .)"

## Migrations
migrate-up:
	cd $(API_DIR) && go run ./cmd/api migrate up

migrate-down:
	cd $(API_DIR) && go run ./cmd/api migrate down

migrate-status:
	cd $(API_DIR) && go run ./cmd/api migrate status

migrate-new: ## Usage: make migrate-new name=add_budgets
	cd $(API_DIR) && go run github.com/pressly/goose/v3/cmd/goose@v3.28.0 -dir internal/db/migrations create $(name) sql

## Web
api-openapi: ## Export the OpenAPI document and regenerate the web API types
	cd $(API_DIR) && go run ./cmd/api openapi > ../web/openapi.json.tmp
	mv apps/web/openapi.json.tmp apps/web/openapi.json
	cd apps/web && pnpm gen:api

web-install:
	cd apps/web && pnpm install

# The root .env (exported above) sets PORT for the API, so pin the web port.
web-dev:
	cd apps/web && pnpm dev --port $(WEB_PORT)

web-test:
	cd apps/web && pnpm test

web-lint:
	cd apps/web && pnpm lint && pnpm typecheck

web-e2e: ## Requires `make up` and the API running (`make api-run`)
	cd apps/web && pnpm e2e
