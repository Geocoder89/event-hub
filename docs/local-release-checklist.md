# Day 84 - Local Release Checklist

Goal: run a repeatable local release process with clear startup, migration, smoke, and rollback steps.

## 1) Preflight

- Ensure Docker Desktop is running.
- Ensure `.env` exists and contains DB/Auth/Redis values.
- Ensure no pending local edits you do not want in this release.

```bash
git status --short
docker compose ps
```

## 2) Startup

Start the full local stack:

```bash
docker compose up -d db redis jaeger prometheus grafana api worker
```

Wait for core dependencies to become healthy:

```bash
docker compose ps
```

Expected:
- `db` -> healthy
- `redis` -> healthy
- `api` -> healthy
- `worker` -> healthy

## 3) Migration

Run migrations against local DB (choose one path).

Path A (recommended if Goose is installed):

```bash
goose -dir db/migrations postgres "postgres://eventhub:eventhub@127.0.0.1:5433/eventhub?sslmode=disable" up
```

Path B (via Makefile and `.env` DB values):

```bash
make migrate-up
```

Confirm migration status:

```bash
goose -dir db/migrations postgres "postgres://eventhub:eventhub@127.0.0.1:5433/eventhub?sslmode=disable" status
```

## 4) Smoke

Run Day 83 readiness smoke:

```bash
make day83
```

Manual spot checks:

```bash
curl -si http://localhost:8080/healthz
curl -si http://localhost:8080/readyz
curl -si http://localhost:8080/metrics | head -n 20
curl -si http://localhost:8080/docs/openapi.yaml | head -n 20
```

Check logs for startup/readiness:

```bash
docker compose logs api --tail=120
docker compose logs worker --tail=120
```

## 5) Rollback

Use this if new code/migrations break smoke checks.

### 5.1 Code rollback

```bash
git checkout <last-known-good-commit>
docker compose build api worker
docker compose up -d api worker
```

### 5.2 Migration rollback (single step)

Only run if the latest migration caused breakage and is safe to revert.

```bash
goose -dir db/migrations postgres "postgres://eventhub:eventhub@127.0.0.1:5433/eventhub?sslmode=disable" down
```

Then restart app services:

```bash
docker compose up -d api worker
```

Re-run smoke:

```bash
make day83
```

## 6) Evidence for Day 84

- `tmp/day83/summary.txt`
- `tmp/day83/compose_ps.txt`
- API and worker logs from smoke (`tmp/day83/api_logs_tail.txt`, `tmp/day83/worker_logs_tail.txt`)
- Migration status output screenshot
- Commit containing this runbook

## 7) Done Criteria

- Startup succeeds with healthy core services.
- Migrations apply cleanly.
- `make day83` passes.
- Rollback steps are documented and executable.
