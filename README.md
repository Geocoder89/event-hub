# EventHub (Go)

EventHub is a production-oriented Go backend API project focused on clean structure (`cmd/` + `internal/`), database persistence, versioned migrations, and repeatable local development using Docker Compose.

This repo is part of my “build in public” Go learning series and is intentionally evolving.

---

## Tech Stack
- Go
- PostgreSQL
- Goose (database migrations)
- Docker Compose
- Air (hot reload)

---

## Repository Structure
- `cmd/api` — application entrypoint
- `internal/` — application/business packages
- `db/migrations` — Goose migration files
- `docker-compose.yml` — local infrastructure (Postgres, etc.)
- `Makefile` — developer workflows (run/test/migrations)

---

## Prerequisites
- Go (recommended: 1.25)
- Docker + Docker Compose
- Goose CLI
- Air (optional, for hot reload)

### Install Goose
```bash
go install github.com/pressly/goose/v3/cmd/goose@latest
