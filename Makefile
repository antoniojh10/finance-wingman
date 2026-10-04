SHELL := /bin/bash
API_DIR := apps/api

-include .env
export

.PHONY: up down logs psql api-run api-build api-test api-lint migrate-up migrate-down migrate-status migrate-new

## Infrastructure
up: ## Start Postgres and Mailpit
	docker compose up -d --wait

down: ## Stop local services
	docker compose down

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
