# EventHub (Go)

EventHub is a production-oriented Go backend API focused on clean structure, database persistence, and repeatable local development. I’m building it in public as part of a **100 Days of Go** learning series.

The goal is to practice “real” backend work in Go: routing, domain modeling, persistence, migrations, transactions, validation, and testing.

---

## Tech Stack

- **Language:** Go
- **Web framework:** Gin
- **Database:** PostgreSQL
- **Migrations:** Goose
- **Dev tooling:** Docker Compose, Air (hot reload)
- **Logging:** `log/slog`
- **Testing:** `go test` (unit + integration), `httptest`, pgx

---

## Features (so far)

### Health & readiness

- `GET /healthz` – basic liveness check.
- `GET /readyz` – readiness check that pings Postgres with a timeout.

### Events API

CRUD + filtering for events:

- `POST /events`  
  - Create an event (title, description, city, startAt, capacity, etc.).
- `GET /events`  
  - List events with:
    - Pagination: `page`, `limit`
    - Optional filters: `city`, `from`, `to` (RFC3339)
- `GET /events/:id`  
  - Fetch a single event by ID.
- `PUT /events/:id`  
  - Update an existing event.
- `DELETE /events/:id`  
  - Delete an event.

Implementation details:

- Domain model + DTOs in `internal/domain/event`.
- Postgres repository in `internal/repo/postgres`.
- Versioned migrations for the `events` table via Goose.
- Standardized JSON error responses across handlers.

### Registrations API

Users can register for events by email.

- `POST /events/:id/register`  
  Request body:

  ```json
  {
    "name": "Sam Example",
    "email": "sam@example.com"
  }

Behavior:

* 201 Created – registration created.

* 409 Conflict with code: "already_registered" – this email is already registered for that event.

* 409 Conflict with code: "event_full" – the event has reached its capacity.

* 404 Not Found – if the event does not exist (depending on how you map event.ErrNotFound).


Implementation details:

* registrations table:

    * id UUID PRIMARY KEY

    * event_id (FK → events(id) with ON DELETE CASCADE)

    * name, email

    * created_at, updated_at

    * UNIQUE (event_id, email) to prevent duplicate registrations per event/email.

* Domain model + DTO in internal/domain/registration.

* Postgres repository in internal/repo/postgres with:

  * Transactional logic:

    1. Check if (event_id, email) already exists (SELECT EXISTS).

    2. Lock the event row (FOR UPDATE), compute current registrations, enforce capacity.

    3. Insert registration.

  *  Domain errors:

      * ErrAlreadyRegistered

      * ErrEventFull

* Handler in internal/http/handlers:

  * Uses the route param :id as the source of truth for eventId.

Maps domain errors to 409 responses with structured JSON errors.



Project Structure

* cmd/api – application entrypoint (wires config, DB, router, server).

* internal/config – configuration loading (env, port, DB URL, timeouts).

* internal/http – router setup, middleware, and HTTP handlers.

* internal/domain – domain models and DTOs (event, registration, etc.).

* internal/repo/postgres – Postgres-backed repositories (events, registrations).

* internal/observability – logging, request IDs, etc.

* db/migrations – Goose migration files.

* docker-compose.yml – local infrastructure (Postgres).

* Makefile – developer workflows (run, dev, test, migrate-up, etc.).


Prerequisites

* Go (recommended: 1.25)

* Docker + Docker Compose

* Goose CLI

* Air (optional, for hot reload)

<h3>Install Goose<h3>

```bash
go install github.com/pressly/goose/v3/cmd/goose@latest
```
Install Air (optional)
```bash
go install github.com/air-verse/air@latest
```

<h3>Running the app locally<h3>

1) Start Postgres with Docker Compose:
```bash
docker compose up -d
```

2) Apply database migrations:
```bash
make migrate-up
# or
goose -dir db/migrations postgres "$GOOSE_DBSTRING" up
```
3) Run the API

```bash
# Simple run
make run

# Or with hot reload (Air)
make dev
```

The server will start on the port configured in your environment (default 5000).


Testing

Unit + integration tests use the same Postgres instance defined in docker-compose.yml.

1) Ensure Postgres is running and migrations are applied:

```bash
docker compose up -d
make migrate-up
```
2) Run Tests

```bash
go test ./... -v
```

Tests currently cover:

* Handlers (unit tests) for events CRUD (table-driven).

* Registration flow (integration tests) hitting:

    * POST /events/:id/register

    * Real Postgres via pgx

    * Scenarios:

        * Happy path (201 Created).

        * Duplicate email (409 already_registered).

        * Event at capacity (409 event_full).

100 Days of Go (build in public)

This repo is part of my ongoing 100 Days of Go series. Highlights so far:

* Day 1–5: basic Gin setup, health endpoints, and Event CRUD with in-memory + Postgres.

* Day 6–10: standardized error responses, config/env handling, migrations, and table-driven handler tests.

* Day 11–13: registrations domain + schema, POST /events/:id/register, uniqueness + capacity enforcement with transactions.

* Day 14: integration tests for the registration flow and documentation updates.

You can follow my Progress on [LinkedIn](https://www.linkedin.com/in/oladele-omoarukhe/) where I’m continuously posting short daily recaps and screenshots as the project evolves.

Next Steps (planned)

* Add endpoints to list/cancel registrations.

* Add more observability (structured logging, request IDs already in place, expand metrics).

* Explore authentication and multi-tenant support.

* Experiment with Terraform around the stack once the core API is stable.







