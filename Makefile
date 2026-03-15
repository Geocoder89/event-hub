leAPP_NAME   := eventhub
GOOSE_DIR  := db/migrations
AIR        := $(shell go env GOPATH)/bin/air

.PHONY: up down build run dev fmt vet tidy migrate migrate-up migrate-down test lint check-db-env gosec govuln security day83 day85-preflight day86 day87 day88 day89 day90 day91 day92 day93 day94 day95 day96 day97 day98 day99

-include .env
export

# ---- Infra (Docker) ----

up:
	docker compose up -d

down:
	docker compose down -v

# ---- Build / Run ----

build:
	go build -o bin/$(APP_NAME) ./cmd/api

run:
	go run ./cmd/api

dev:
	$(AIR)

# ---- Code Quality ----

fmt:
	go fmt ./...

vet:
	go vet ./...

tidy: fmt vet
	go mod tidy

lint: fmt vet
	golangci-lint run ./...

test:
	go test ./... -v

gosec:
	golangci-lint run --no-config --enable-only gosec ./...

govuln:
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...

security: gosec govuln

# ---- DB / Migrations ----

check-db-env:
	@test -n "$(DB_USER)"     || (echo "DB_USER is required" && exit 1)
	@test -n "$(DB_PASSWORD)" || (echo "DB_PASSWORD is required" && exit 1)
	@test -n "$(DB_HOST)"     || (echo "DB_HOST is required" && exit 1)
	@test -n "$(DB_PORT)"     || (echo "DB_PORT is required" && exit 1)
	@test -n "$(DB_NAME)"     || (echo "DB_NAME is required" && exit 1)
	@test -n "$(DB_SSLMODE)"  || (echo "DB_SSLMODE is required" && exit 1)

migrate: migrate-up

migrate-up: check-db-env
	@GOOSE_DRIVER=postgres \
	GOOSE_DBSTRING="user=$(DB_USER) password='$(DB_PASSWORD)' host=$(DB_HOST) port=$(DB_PORT) dbname=$(DB_NAME) sslmode=$(DB_SSLMODE)" \
	goose -dir $(GOOSE_DIR) up

migrate-down: check-db-env
	@GOOSE_DRIVER=postgres \
	GOOSE_DBSTRING="user=$(DB_USER) password='$(DB_PASSWORD)' host=$(DB_HOST) port=$(DB_PORT) dbname=$(DB_NAME) sslmode=$(DB_SSLMODE)" \
	goose -dir $(GOOSE_DIR) down

worker:
		go run ./cmd/worker

day83:
	bash ./scripts/day83_local_readiness.sh

day85-preflight:
	bash ./scripts/day85_env_preflight.sh

day86:
	bash ./scripts/day86_backup_restore.sh

day87:
	bash ./scripts/day87_migration_safety.sh

day88:
	bash ./scripts/day88_contract_check.sh

day89:
	bash ./scripts/day89_etag_cache_regression.sh

day90:
	bash ./scripts/day90_worker_reliability.sh

day91:
	bash ./scripts/day91_slo_alerts.sh

day92:
	bash ./scripts/day92_dependency_failure_drill.sh

day93:
	bash ./scripts/day93_idempotency_checks.sh

day94:
	bash ./scripts/day94_k6_rebaseline.sh

day95:
	bash ./scripts/day95_grafana_as_code.sh

day96:
	bash ./scripts/day96_trace_quality.sh

day97:
	bash ./scripts/day97_security_abuse.sh

day98:
	bash ./scripts/day98_ci_parity.sh

day99:
	bash ./scripts/day99_clean_machine_rehearsal.sh
