# EventHub

A production-oriented backend platform built in **Go** and **PostgreSQL**, focused on authentication, authorization, transactional workflows, background processing, observability, testing, and operational reliability.

EventHub exposes APIs for managing events and registrations, protects user-owned resources with JWT-based authentication and authorization, and runs asynchronous jobs through a PostgreSQL-backed worker model.

## Engineering Highlights

- **Authentication and sessions:** short-lived JWT access tokens, DB-backed refresh-token rotation, HttpOnly refresh cookies, hashed refresh-token storage, logout revocation, and protected routes.
- **Authorization:** ownership checks on protected operations with administrative override where appropriate.
- **Concurrency and consistency:** PostgreSQL transactions and row locking for capacity-sensitive registration flows and refresh-token rotation.
- **Background processing:** persisted jobs, `FOR UPDATE SKIP LOCKED` claiming, retry scheduling, failure recording, and idempotent publish workflows.
- **Observability:** structured logging, request IDs, OpenTelemetry instrumentation, Prometheus metrics, and Grafana monitoring assets.
- **Quality gates:** GitHub Actions CI runs database migrations, `go vet`, builds, tests, `golangci-lint`, `gosec`, `govulncheck`, and Docker image builds.
- **Performance work:** k6 smoke/baseline load tests and repository-backed performance investigation notes.

## Tech Stack

| Area | Technology |
|---|---|
| Language | Go |
| HTTP | Gin |
| Database | PostgreSQL + pgx |
| Migrations | Goose |
| Authentication | JWT access + refresh tokens |
| Password hashing | bcrypt |
| Background jobs | PostgreSQL-backed worker |
| Observability | OpenTelemetry, Prometheus, Grafana, structured `slog` |
| Testing | `go test`, `httptest`, PostgreSQL integration tests |
| Delivery | Docker, Docker Compose, GitHub Actions |

## Core Capabilities

### Health and readiness

- `GET /healthz` — liveness check.
- `GET /readyz` — readiness check with a bounded PostgreSQL ping.

### Authentication and session management

EventHub uses short-lived access tokens with long-lived refresh tokens that are rotated on refresh.

- `POST /signup` — creates a user and starts an authenticated session.
- `POST /login` — validates credentials and returns an access token.
- `POST /auth/refresh` — rotates the refresh token and returns a new access token.
- `POST /auth/logout` — revokes the current refresh session and clears the refresh cookie.

Protected routes require:

```http
Authorization: Bearer <accessToken>
```

Refresh tokens are stored **hashed** in PostgreSQL. Rotation runs inside a transaction and locks the relevant row with `SELECT ... FOR UPDATE` to prevent concurrent reuse.

### Events API

- `POST /events` — create an event.
- `GET /events` — list events with pagination and optional city, text, and date filters.
- `GET /events/:id` — retrieve one event.
- `PUT /events/:id` — update an event.
- `DELETE /events/:id` — delete an event.

The implementation separates HTTP handling, domain models, and PostgreSQL repositories. Full-text-search and query-plan investigation artifacts are available under [`perf/day68`](./perf/day68).

### Registrations and ownership

Authenticated users can register for events while the service enforces duplicate-registration and capacity constraints.

- `POST /events/:id/register`
- `DELETE /events/:id/registrations/:registrationId`

Registration creation uses transactional database logic and row locking to prevent capacity races. Cancellation enforces ownership, with administrator override supported by the authorization flow.

### Background jobs and worker

Asynchronous publish jobs are persisted with lifecycle states such as `pending`, `processing`, `done`, and `failed`.

The worker provides:

- concurrent-safe claiming with `FOR UPDATE SKIP LOCKED`;
- scheduled retries using `run_at`;
- failure recording through `last_error`;
- producer-side idempotency keys;
- consumer-side guards using event publication state.

## Architecture

```text
Client
  │
  ▼
Gin HTTP API
  │
  ├── Authentication / authorization
  ├── Event and registration handlers
  │
  ▼
Domain + repository layers
  │
  ▼
PostgreSQL
  │
  ├── Application data
  ├── Refresh sessions
  └── Background jobs

Worker ───────────────► PostgreSQL job queue

API / Worker
  ├── structured logs
  ├── OpenTelemetry
  └── Prometheus metrics ──► Grafana
```

## Project Structure

```text
cmd/
├── api/                    API entrypoint
└── worker/                 Worker entrypoint

internal/
├── config/                 Environment and runtime configuration
├── domain/                 Domain models and DTOs
├── http/                   Router, middleware, and handlers
├── observability/          Logging, metrics, and tracing
└── repo/postgres/          PostgreSQL repositories

db/migrations/              Goose migrations
monitoring/                 Grafana and alerting configuration
load-test/                  k6 smoke and baseline tests
perf/                       Performance investigation artifacts
postman/                    Postman collection and environment
docs/                       Reliability and operational notes
.github/workflows/          CI pipeline
```

## Local Development

### Prerequisites

- Go
- Docker + Docker Compose
- Goose CLI

Install Goose:

```bash
go install github.com/pressly/goose/v3/cmd/goose@latest
```

Start dependencies:

```bash
docker compose up -d db redis
```

Apply migrations:

```bash
make migrate-up
```

Run the API:

```bash
make run
```

Or use hot reload:

```bash
make dev
```

Run the worker in a second terminal:

```bash
make worker
```

The API uses port `8080` by default.

## Testing

Run the full Go test suite:

```bash
go test ./... -v
```

The CI pipeline additionally validates:

- PostgreSQL migrations;
- `go vet`;
- API and worker builds;
- unit and integration tests;
- `golangci-lint`;
- `gosec`;
- `govulncheck`;
- Docker builds for the API and worker targets.

## Manual API Exploration

The repository includes:

- `postman/eventhub-api.postman_collection.json`
- `postman/eventhub-local.postman_environment.json`

Import both into Postman to exercise signup/login, protected event operations, registration, token refresh, logout, and publish workflows.

## Performance and Reliability Work

The repository contains engineering notes and artifacts covering topics such as:

- query-plan and indexing investigation;
- environment hardening;
- backup/restore;
- migration safety;
- worker reliability;
- SLO/alert design;
- dependency-failure drills;
- load testing.

See [`docs/`](./docs), [`perf/`](./perf), and [`load-test/`](./load-test).

## Project Background

EventHub started as part of a **100 Days of Go** build-in-public series and evolved into a broader backend-engineering project focused on production concerns rather than isolated language exercises.

The project is intentionally used to explore how authentication, persistence, concurrency, asynchronous work, observability, CI, and operational practices fit together in a service that can be reasoned about end to end.
