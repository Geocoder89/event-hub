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
- **Auth**: JWT (access + refresh), refresh-token rotation (DB-backed)
- **Password Hashing**: bcrypt (via internal/security)
- **Cookies**: HttpOnly refresh cookie (refresh_token)

---

## Features (so far)

### Health & readiness

- `GET /healthz` – basic liveness check.
- `GET /readyz` – readiness check that pings Postgres with a timeout.

Authentication & Sessions (JWT + refresh rotation)

EventHub uses short-lived access tokens and long-lived refresh tokens with rotation.

Endpoints

- POST /signup
  Creates a user (default role: user) and returns:

  - accessToken in JSON response
  - refresh_token as an HttpOnly cookie

  Request Body:

  ```json
  {
    "email": "sam@example.com",
    "password": "strong-password",
    "name": "Sam Example"
  }
  ```

````

POST /login
Verifies email/password and returns:
  * accessToken in JSON
  * refresh_token as an HttpOnly cookie

  ```json
  {
  "email": "sam@example.com",
  "password": "strong-password"
}
````

- POST /auth/refresh
  Uses the refresh_token cookie to:

- Validate the refresh token
- Rotate it (revoke old token, issue new token)
- Return a new accessToken in JSON
- Set a new refresh_token cookie

- POST /auth/logout
  Revokes the current refresh token (best-effort) and clears the cookie.

Response body

```json
{ "accessToken": "<new_access_token>" }
```

How to call protected endpoints

Protected routes require:

```makefile
Authorization: Bearer <accessToken>
```

Access token claims are verified by middleware; userID, email, and role are attached to the request context.

**Refresh token storage**

Refresh tokens are stored hashed in Postgres and tracked for rotation:

- refresh_tokens table stores:

- token id (jti), user_id, token_hash, expires_at
- revoked_at, replaced_by (rotation metadata)

Implementation highlights:

- Refresh rotation uses a transaction + row lock (SELECT ... FOR UPDATE) to prevent reuse / race conditions.
- Raw refresh tokens are never stored in the database (only hashes).

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
  Requires Authorization: Bearer <accessToken>  
   Request body:

  ```json
  {
    "name": "Sam Example",
    "email": "sam@example.com"
  }
  ```

Behavior:

- 201 Created – registration created.

- 409 Conflict with code: "already_registered" – this email is already registered for that event.

- 409 Conflict with code: "event_full" – the event has reached its capacity.
- 401 Unauthorized – missing/invalid access token


Cancel registration (ownership enforced)

* DELETE /events/:id/registrations/:registrationId (protected)
Users can only cancel their own registration. Admin can override.

Responses:
* 204 No Content – canceled
* 403 Forbidden – attempting to cancel someone else’s registration
* 404 Not Found – registration not found

Implementation details:

- registrations table:

  - id UUID PRIMARY KEY

  - event_id (FK → events(id) with ON DELETE CASCADE)

  - name, email

  - created_at, updated_at

  - UNIQUE (event_id, email) to prevent duplicate registrations per event/email.

  - registrations includes user_id (FK → users)

  - Ownership check enforced in handler (role == admin override)

- Domain model + DTO in internal/domain/registration.

- Postgres repository in internal/repo/postgres with:

  - Transactional logic:

    1. Check if (event_id, email) already exists (SELECT EXISTS).

    2. Lock the event row (FOR UPDATE), compute current registrations, enforce capacity.

    3. Insert registration.

  - Domain errors:

    - ErrAlreadyRegistered

    - ErrEventFull

- Handler in internal/http/handlers:

  - Uses the route param :id as the source of truth for eventId.

Maps domain errors to 409 responses with structured JSON errors.




Project Structure

- cmd/api – application entrypoint (wires config, DB, router, server).

- internal/config – configuration loading (env, port, DB URL, timeouts).

- internal/http – router setup, middleware, and HTTP handlers.

- internal/domain – domain models and DTOs (event, registration, etc.).

- internal/repo/postgres – Postgres-backed repositories (events, registrations).

- internal/observability – logging, request IDs, etc.

- db/migrations – Goose migration files.

- docker-compose.yml – local infrastructure (Postgres).

- Makefile – developer workflows (run, dev, test, migrate-up, etc.).

Prerequisites

- Go (recommended: 1.25)

- Docker + Docker Compose

- Goose CLI

- Air (optional, for hot reload)

<h3>Install Goose<h3>

```bash
go install github.com/pressly/goose/v3/cmd/goose@latest
```

Install Air (optional)

```bash
go install github.com/air-verse/air@latest
```

<h3>Running the app locally<h3>

1. Start Postgres with Docker Compose:

```bash
docker compose up -d
```

2. Apply database migrations:

```bash
make migrate-up
# or
goose -dir db/migrations postgres "$GOOSE_DBSTRING" up
```

3. Run the API

```bash
# Simple run
make run

# Or with hot reload (Air)
make dev
```

The server will start on the port configured in your environment (default 5000).

**Validation & Input Hardening**

* Request payload validation via Gin binding tags (required, email, min/max, etc.)

* Path param validation for UUIDs (e.g., /events/:id)

* Consistent JSON error shape across handlers

**Testing**

Unit + integration tests use the same Postgres instance defined in docker-compose.yml.

1. Ensure Postgres is running and migrations are applied:

```bash
docker compose up -d
make migrate-up
```

2. Run Tests

```bash
go test ./... -v
```

Tests currently cover:

- Handlers (unit tests) for events CRUD (table-driven).

- Registration flow (integration tests) hitting:

  - POST /events/:id/register

  - Real Postgres via pgx

  - Scenarios:

    - Happy path (201 Created).

    - Duplicate email (409 already_registered).

    - Event at capacity (409 event_full).

Auth integration tests (in progress / Day 29):

  - signup/login sets refresh cookie

  - refresh rotation returns new access token and rotates cookie

  - logout clears cookie and revokes refresh token

100 Days of Go (build in public)

This repo is part of my ongoing 100 Days of Go series.You can follow my Progress on [LinkedIn](https://www.linkedin.com/in/oladele-omoarukhe/) where I’m continuously posting short daily recaps and screenshots as the project evolves.

## Postman collection

For manual API exploration there is a Postman collection and local environment:

- `postman/eventhub-api.postman_collection.json`
- `postman/eventhub-local.postman_environment.json`

Usage:

1. Import both files into Postman.
2. Select the **event-hub-local** environment.
3. Use the collection’s “Create event” request to create an event; scripts will capture the `eventId` into the environment.
4. Use the other requests (list, get by id, register, etc.) to exercise the API.

Auth workflow in Postman:

1) Call Signup or Login to receive accessToken and a refresh_token cookie.

2) Use accessToken as a Bearer token in protected requests.

3) When access expires, call /auth/refresh (cookie-based) to get a new access token.

4) Call /auth/logout to invalidate the refresh session.
