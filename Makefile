APP_NAME=eventhub
GOOSE_DIR := db/migrations
AIR := $(shell go env GOPATH)/bin/air

.PHONY: run dev tidy test migrate-up migrate-down

-include .env
export

migrate-up:
	@GOOSE_DRIVER=postgres \
	GOOSE_DBSTRING="user=$(DB_USER) password='$(DB_PASSWORD)' host=$(DB_HOST) port=$(DB_PORT) dbname=$(DB_NAME) sslmode=$(DB_SSLMODE)" \
	goose -dir $(GOOSE_DIR) up

migrate-down:
	@GOOSE_DRIVER=postgres \
	GOOSE_DBSTRING="user=$(DB_USER) password='$(DB_PASSWORD)' host=$(DB_HOST) port=$(DB_PORT) dbname=$(DB_NAME) sslmode=$(DB_SSLMODE)" \
	goose -dir $(GOOSE_DIR) down

run:
	go run ./cmd/api

dev:
	$(AIR)

tidy:
	go mod tidy

test:
	go test ./...
