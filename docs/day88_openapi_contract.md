# Day 88 - OpenAPI Contract Conformance

Goal: keep implementation and OpenAPI contract aligned for high-impact API behavior.

## One-command run

```bash
make day88
```

Equivalent:

```bash
bash ./scripts/day88_contract_check.sh
```

## What Day 88 checks

1. Confirms critical paths and contract markers exist in `docs/openapi.yaml`:
   - `/events`, `/events/{id}`, `/admin/jobs`
   - check-in/export endpoints
   - `If-None-Match`, `nextCursor`, and `unsupported_media_type` markers
2. Runs contract-focused handler tests:
   - ETag 304 behavior
   - pagination response metadata shape
   - validation error field mapping
3. Runs middleware test for `Content-Type` enforcement.
4. Runs DB-backed integration tests for:
   - registration check-in flow
   - registrations CSV export flow

## Artifacts produced (`tmp/day88/`)

- `summary.txt`
- `openapi_snapshot.yaml`
- `goose_up.txt`
- `handlers_contract_tests.txt`
- `middlewares_contract_tests.txt`
- `integration_contract_tests.txt`
- `compose_ps.txt`

## Done criteria

- Script exits successfully.
- All contract test logs report pass.
- `summary.txt` reports all checks passed.

## Evidence checklist

1. Screenshot `tmp/day88/summary.txt`
2. Screenshot from `handlers_contract_tests.txt` (ETag + pagination tests passing)
3. Screenshot from `integration_contract_tests.txt` (check-in/export tests passing)
4. Screenshot of `openapi_snapshot.yaml` with key endpoint sections
5. Screenshot of committed script and runbook
