# Day 100 - Capstone Evidence Pack

This document is an evidence index for the Day 83-99 local hardening track on EventHub.

The goal is not to dump folders. The goal is to point to the exact files that prove each improvement.

## What EventHub Is

EventHub is a production-oriented backend platform built in Go with Gin, PostgreSQL, Redis, Docker, OpenTelemetry, Prometheus, Grafana, and Jaeger.

Core application capabilities already shipped before this local hardening track:

- JWT access tokens plus refresh-cookie auth flows with rotation and DB-backed refresh token safety.
- Async jobs and workers for publish/export flows with request and job correlation.
- Cursor pagination, ETag/If-None-Match support, in-memory caching, and Postgres full-text search.
- Admin workflows including audit logging, soft-delete and restore, registration exports, and check-in flows.

## How To Use This README

For each day:

1. Open the primary evidence file first.
2. Use the secondary evidence file only if you want more depth.
3. For LinkedIn or portfolio proof, screenshot the summary file plus one deeper artifact, not the entire folder.

## Evidence Matrix

| Day | Topic | Primary evidence | Secondary evidence | What it proves |
| --- | --- | --- | --- | --- |
| 83 | Local readiness baseline | [`tmp/day83/summary.txt`](../../tmp/day83/summary.txt) | [`tmp/day83/compose_ps.txt`](../../tmp/day83/compose_ps.txt) | API and worker boot locally with health, readiness, metrics, Swagger, and OpenAPI reachable. |
| 84 | Local release runbook | [`docs/local-release-checklist.md`](../../docs/local-release-checklist.md) | [`tmp/day83/summary.txt`](../../tmp/day83/summary.txt) | Startup, migration, smoke, and rollback steps are documented and reproducible. |
| 85 | Config and secret hygiene | [`docs/day85_env_hardening.md`](../../docs/day85_env_hardening.md) | [`scripts/day85_env_preflight.sh`](../../scripts/day85_env_preflight.sh) | Runtime config now fails fast on insecure defaults and invalid release settings. |
| 86 | Backup and restore drill | [`tmp/day86/summary.txt`](../../tmp/day86/summary.txt) | [`tmp/day86/restore_output.txt`](../../tmp/day86/restore_output.txt) | A real Postgres backup was created, restored, and validated against source row counts. |
| 87 | Migration safety drill | [`tmp/day87/summary.txt`](../../tmp/day87/summary.txt) | [`tmp/day87/schema_diff.txt`](../../tmp/day87/schema_diff.txt) | Goose up/reset/up leaves the schema stable with no drift. |
| 88 | OpenAPI contract guard | [`tmp/day88/summary.txt`](../../tmp/day88/summary.txt) | [`tmp/day88/openapi_snapshot.yaml`](../../tmp/day88/openapi_snapshot.yaml) | Handler, middleware, and integration contract checks align with the API spec. |
| 89 | ETag and cache regression | [`tmp/day89/summary.txt`](../../tmp/day89/summary.txt) | [`tmp/day89/etag_cache_tests.txt`](../../tmp/day89/etag_cache_tests.txt) | Read endpoints keep their conditional-response and cache behavior intact. |
| 90 | Worker resilience | [`tmp/day90/summary.txt`](../../tmp/day90/summary.txt) | [`tmp/day90/worker_reliability_integration_tests.txt`](../../tmp/day90/worker_reliability_integration_tests.txt) | Retry and dead-letter worker paths are exercised and passing. |
| 91 | SLO and alerting verification | [`tmp/day91/summary.txt`](../../tmp/day91/summary.txt) | [`tmp/day91/promtool_rules_check.txt`](../../tmp/day91/promtool_rules_check.txt) | Prometheus alert rules load cleanly and metrics needed for alerting are present. |
| 92 | Dependency failure drill | [`tmp/day92/summary.txt`](../../tmp/day92/summary.txt) | [`tmp/day92/db_down_api_readyz.headers.txt`](../../tmp/day92/db_down_api_readyz.headers.txt) | Readiness degrades correctly when Redis or Postgres is unavailable and recovers afterward. |
| 93 | Idempotency checks | [`tmp/day93/summary.txt`](../../tmp/day93/summary.txt) | [`tmp/day93/idempotency_integration_tests.txt`](../../tmp/day93/idempotency_integration_tests.txt) | Duplicate publish, export, and check-in flows behave safely instead of duplicating work. |
| 94 | k6 re-baseline | [`tmp/day94/summary.txt`](../../tmp/day94/summary.txt) | [`tmp/day94/day67_vs_day94_comparison.txt`](../../tmp/day94/day67_vs_day94_comparison.txt) | Load-test baselines are repeatable and comparable across performance checkpoints. |
| 95 | Grafana as code | [`tmp/day95/summary.txt`](../../tmp/day95/summary.txt) | [`tmp/day95/grafana_dashboard_definition.json`](../../tmp/day95/grafana_dashboard_definition.json) | Datasources and dashboards are provisioned from code rather than manual UI setup. |
| 96 | Trace quality | [`tmp/day96/summary.txt`](../../tmp/day96/summary.txt) | [`tmp/day96/trace_checks.txt`](../../tmp/day96/trace_checks.txt) | API and worker traces now carry correlation fields like request, event, user, and job identifiers. |
| 97 | Security abuse checks | [`tmp/day97/summary.txt`](../../tmp/day97/summary.txt) | [`tmp/day97/rate_limit_login_6.body.json`](../../tmp/day97/rate_limit_login_6.body.json) | Auth flows reject abuse cases such as brute-force login, refresh misuse, replay, and invalid tokens. |
| 98 | Local CI parity | [`tmp/day98/summary.txt`](../../tmp/day98/summary.txt) | [`tmp/day98/docker_images.txt`](../../tmp/day98/docker_images.txt) | A single local command now mirrors fmt, vet, build, test, lint, security, and Docker image checks. |
| 99 | Clean-machine rehearsal | [`tmp/day99/summary.txt`](../../tmp/day99/summary.txt) | [`tmp/day99/day83_rehearsal/summary.txt`](../../tmp/day99/day83_rehearsal/summary.txt) | A clean workspace copy can build, migrate, boot, and pass readiness smoke without hidden local state. |

## Supporting Project Artifacts

Use these when you want to show broader system scope beyond the Day 83-99 reliability track:

- API contract: [`docs/openapi.yaml`](../../docs/openapi.yaml)
- Alert rules: [`monitoring/alerts/eventhub-alerts.yml`](../../monitoring/alerts/eventhub-alerts.yml)
- Grafana dashboard JSON: [`monitoring/grafana/dashboards/eventhub-overview.json`](../../monitoring/grafana/dashboards/eventhub-overview.json)
- CI workflow: [`.github/workflows/ci.yml`](../../.github/workflows/ci.yml)

## Recommended Screenshot Set

If you want one strong capstone post instead of a folder dump, use these:

1. [`tmp/day99/summary.txt`](../../tmp/day99/summary.txt)
2. [`tmp/day98/summary.txt`](../../tmp/day98/summary.txt)
3. [`tmp/day97/summary.txt`](../../tmp/day97/summary.txt)
4. [`tmp/day96/summary.txt`](../../tmp/day96/summary.txt)
5. [`tmp/day95/summary.txt`](../../tmp/day95/summary.txt) plus a Grafana UI screenshot
6. [`tmp/day94/day67_vs_day94_comparison.txt`](../../tmp/day94/day67_vs_day94_comparison.txt)
7. [`tmp/day92/summary.txt`](../../tmp/day92/summary.txt)
8. [`tmp/day83/summary.txt`](../../tmp/day83/summary.txt)

## Final Takeaway

The Day 100 capstone is not about claiming deployment before it exists.

It is about showing that EventHub now has evidence for:

- startup and readiness
- rollback and release discipline
- config hardening
- backup and migration safety
- contract and regression checks
- load and observability baselines
- alerting and dependency degradation behavior
- auth abuse protection
- CI parity
- reproducible clean-machine startup

That is a credible backend engineering story even before hosted deployment is added.
