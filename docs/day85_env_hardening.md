# Day 85 - Config and Secret Hygiene

Goal: fail fast on invalid/insecure runtime configuration before startup.

## What was hardened

1. API startup now validates config before opening DB connections.
2. Worker startup now validates config before boot.
3. Release environments (`APP_ENV` not `dev`/`test`) reject insecure defaults:
   - default `JWT_SECRET`
   - short `JWT_SECRET` (< 32 chars)
   - default `ADMIN_PASSWORD`
   - default `DB_PASSWORD`
   - `DB_SSLMODE=disable`
4. Added local preflight script for env checks.

## Commands

Run baseline preflight from `.env`:

```bash
make day85-preflight
```

Force release checks locally (even when `APP_ENV=dev`):

```bash
FORCE_RELEASE_CHECK=1 make day85-preflight
```

Run config tests:

```bash
go test ./internal/config -v
```

## Startup failure examples

API/worker now exits early with explicit configuration errors, e.g.:

- `invalid configuration: JWT_SECRET must be changed from development default...`
- `invalid configuration: DB_SSLMODE=disable is not allowed in release environments`

## Evidence checklist

1. Screenshot: failing preflight (`FORCE_RELEASE_CHECK=1 make day85-preflight`) with invalid defaults.
2. Screenshot: passing preflight after secret/env fixes.
3. Screenshot: `go test ./internal/config -v` passing.
4. Diff/commit containing:
   - `internal/config/config.go`
   - `cmd/api/main.go`
   - `cmd/worker/main.go`
   - `scripts/day85_env_preflight.sh`
   - this doc.
