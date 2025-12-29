# Changelog

All notable changes to this project will be documented in this file.

## [0.1.0] - 2025-12-25

### Added
- Core EventHub API in Go using Gin, with structured logging via `slog`.
- Health endpoints:
  - `GET /healthz` – liveness check.
  - `GET /readyz` – readiness check (includes DB ping).
- Events domain and HTTP handlers:
  - `POST /events` – create an event.
  - `GET /events` – list events with pagination.
  - `GET /events/:id` – fetch single event by ID.
  - `PUT /events/:id` – update an event.
  - `DELETE /events/:id` – delete an event.
- PostgreSQL persistence for events and registrations using `pgxpool`.
- Goose migrations:
  - `events` table.
  - `registrations` table with `UNIQUE (event_id, email)` constraint and FK to `events(id)`.
- Registrations domain and API:
  - `POST /events/:id/register` – register an attendee for an event.
  - `GET /events/:id/registrations` – list registrations for a given event.
  - `GET /events/:id/registrations/:registrationId` – fetch a single registration by ID.
  - `DELETE /events/:id/registrations/:registrationId` – cancel a registration.
  - Enforce uniqueness per `(event_id, email)` with a clear `already_registered` error.
  - Enforce capacity with a transaction and row locking:
    - Returns `409 event_full` when an event is at max capacity.
- Request validation:
  - DTOs for create/update/list operations with Gin binding + validation tags.
  - Consistent JSON error envelope with `code`, `message`, `requestId`, and optional `details.fields`.

### Testing
- Table-driven unit tests for event handlers:
  - `CreateEvent`, `ListEvents`, `GetEventById`, `UpdateEvent`, `DeleteEvent`.
- Integration tests for registration flow against a real Postgres instance:
  - Happy path registration.
  - Duplicate email (`already_registered`).
  - Capacity enforcement (`event_full`).
  - Non-existent event (404 mapping).
  - Listing registrations by event and verifying counts.

### Tooling & DX
- Makefile targets:
  - `build` – compile `cmd/api` into `bin/eventhub`.
  - `run` – run the API locally.
  - `dev` – run with `air` for live reload (when installed).
  - `fmt`, `vet`, `tidy` – formatting, static analysis, and module cleanup.
  - `test` – `go test ./... -v`.
  - `migrate-up` / `migrate-down` – apply/rollback Goose migrations using environment variables.
  - `lint` – run `golangci-lint`.
- Docker Compose setup for local Postgres with health checks.
- GitHub Actions CI workflow:
  - Spins up Postgres service.
  - Runs Goose migrations.
  - Executes `go vet`, `go test ./... -v`, and `golangci-lint run ./...` on `main` pushes and PRs.

### API Client Support
- Postman collection for EventHub API (events + registrations).
- Local Postman environment:
  - `protocol`, `host`, `port`, and `baseUrl` (`{{protocol}}://{{host}}:{{port}}`).
  - Dynamic `eventId` and `registrationId` variables populated from HTTP responses via scripts.

### Operations / Resilience
- HTTP server configuration:
  - Sensible `ReadTimeout`, `WriteTimeout`, and `IdleTimeout`.
- Graceful shutdown:
  - `SIGINT`/`SIGTERM` handling via `signal.NotifyContext`.
  - Graceful `srv.Shutdown` with timeout and fallback to `Close()`.

---

Future work will build on this foundation: richer filters, RBAC, more observability, and a UI.
