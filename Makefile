APP_NAME=eventhub
AIR := $(shell go env GOPATH)/bin/air

.PHONY: run dev tidy test

run:
	go run ./cmd/api

dev:
	$(AIR)

tidy:
	go mod tidy

test:
	go test ./...
