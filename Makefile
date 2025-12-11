APP_NAME=eventhub
GOOSE_DIR := db/migrations
AIR := $(shell go env GOPATH)/bin/air

.PHONY: run dev tidy fmt vet test migrate-up migrate-down

-include .env
export

build:
	go build -o bin/$(APP_NAME) ./cmd/api


fmt:
	go fmt ./...

vet:
	go vet ./...

tidy: fmt vet
	go mod tidy

run:
	go run ./cmd/api

dev:
	$(AIR)

# Optional: fail fast if DB env vars are missing (remove if you prefer)
check-db-env:
	@test -n "$(DB_USER)" || (echo "DB_USER is required" && exit 1)
	@test -n "$(DB_PASSWORD)" || (echo "DB_PASSWORD is required" && exit 1)
	@test -n "$(DB_HOST)" || (echo "DB_HOST is required" && exit 1)
	@test -n "$(DB_PORT)" || (echo "DB_PORT is required" && exit 1)
	@test -n "$(DB_NAME)" || (echo "DB_NAME is required" && exit 1)
	@test -n "$(DB_SSLMODE)" || (echo "DB_SSLMODE is required" && exit 1)

migrate-up: check-db-env
	@GOOSE_DRIVER=postgres \
	GOOSE_DBSTRING="user=$(DB_USER) password='$(DB_PASSWORD)' host=$(DB_HOST) port=$(DB_PORT) dbname=$(DB_NAME) sslmode=$(DB_SSLMODE)" \
	goose -dir $(GOOSE_DIR) up

migrate-down: check-db-env
	@GOOSE_DRIVER=postgres \
	GOOSE_DBSTRING="user=$(DB_USER) password='$(DB_PASSWORD)' host=$(DB_HOST) port=$(DB_PORT) dbname=$(DB_NAME) sslmode=$(DB_SSLMODE)" \
	goose -dir $(GOOSE_DIR) down

test:
	go test ./... -v
