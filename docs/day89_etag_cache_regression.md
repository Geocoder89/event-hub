# Day 89 - ETag and Cache Regression Guardrail

Goal: prevent regressions in conditional GET behavior (`ETag`/`If-None-Match`) and cache-aware list responses.

## One-command run

```bash
make day89
```

Equivalent:

```bash
bash ./scripts/day89_etag_cache_regression.sh
```

## What Day 89 validates

1. Confirms key ETag contract markers remain in `docs/openapi.yaml`:
   - `/events`, `/events/{id}`, `/events/{id}/registrations`
   - `/admin/jobs`, `/admin/jobs/{id}`
   - global `If-None-Match` parameter and `304 Not Modified` descriptions
2. Runs handler-level regression tests for:
   - events list cache hit behavior
   - events list/detail `304` behavior with `If-None-Match`
   - admin jobs list/detail `304` behavior
   - registrations list `304` behavior
3. Runs unit tests for ETag matcher semantics:
   - wildcard (`*`)
   - weak validators (`W/"..."`)
   - comma-separated `If-None-Match` values

## Artifacts produced (`tmp/day89/`)

- `summary.txt`
- `openapi_snapshot.yaml`
- `etag_cache_tests.txt`

## Done criteria

- Script exits successfully.
- `etag_cache_tests.txt` shows pass for all targeted ETag/cache tests.
- `summary.txt` reports OpenAPI contract and regression tests passed.

## Evidence checklist

1. Screenshot `tmp/day89/summary.txt`
2. Screenshot from `tmp/day89/etag_cache_tests.txt` showing:
   - `TestListEventsHandler_ETagNotModified`
   - `TestAdminJobsList_ETagNotModified`
   - `TestRegistrationListForEvent_ETagNotModified`
3. Screenshot from `tmp/day89/openapi_snapshot.yaml` showing ETag-related endpoint sections
4. Screenshot of committed `scripts/day89_etag_cache_regression.sh` and this runbook
