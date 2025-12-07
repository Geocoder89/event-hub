APP_NAME=eventhub

.PHONY: run tidy test

run:
	go run ./cmd/api

tidy:
		go mod tidy

test:
	go test ./...