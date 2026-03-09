# syntax=docker/dockerfile:1.7

FROM golang:1.25.8-alpine AS builder

WORKDIR /src

RUN apk add --no-cache ca-certificates tzdata git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG TARGETOS=linux
ARG TARGETARCH=amd64

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
  go build -trimpath -ldflags="-s -w" -o /out/eventhub-api ./cmd/api

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
  go build -trimpath -ldflags="-s -w" -o /out/eventhub-worker ./cmd/worker

FROM alpine:3.21 AS runtime

RUN apk add --no-cache ca-certificates tzdata \
  && addgroup -S app \
  && adduser -S -G app app

WORKDIR /app
USER app

FROM runtime AS api

COPY --from=builder /out/eventhub-api /app/eventhub-api
COPY --from=builder /src/docs /app/docs

EXPOSE 8080
ENTRYPOINT ["/app/eventhub-api"]

FROM runtime AS worker

COPY --from=builder /out/eventhub-worker /app/eventhub-worker

EXPOSE 8081
ENTRYPOINT ["/app/eventhub-worker"]
