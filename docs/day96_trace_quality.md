# Day 96 - Trace Quality Pass

Goal: verify trace quality for a real admin publish flow, ensuring API and worker spans carry correlation attributes needed for production debugging.

## One-command run

```bash
make day96
```

Equivalent:

```bash
bash ./scripts/day96_trace_quality.sh
```

## What Day 96 verifies

1. Starts local stack (`db`, `redis`, `jaeger`, `api`, `worker`).
2. Applies migrations and ensures at least one event exists.
3. Authenticates an admin user (with bootstrap fallback if login is unavailable).
4. Triggers `POST /admin/events/:id/publish` to generate API + worker traces.
5. Waits for the queued job to finish with `status=done`.
6. Queries Jaeger APIs and verifies correlated span attributes.

## Correlation attributes checked

### API span checks (`eventhub-api`)

- `request.id`
- `event.id`
- `user.id`

### Worker span checks (`eventhub-worker`, operation `job.run`)

- `job.id`
- `request.id`
- `event.id`
- `user.id`

## Artifacts produced (`tmp/day96/`)

- `summary.txt`
- `trace_checks.txt`
- `goose_up.txt`
- `event_created_via_api.json`
- `admin_login_response.json`
- `publish_request_headers.txt`
- `publish_request_body.json`
- `job_status_response.json`
- `jaeger_services.json`
- `jaeger_api_traces.json`
- `jaeger_worker_traces.json`
- `compose_ps.txt`
- `api_logs_tail.txt`
- `worker_logs_tail.txt`

## Optional environment knobs

- `API_BASE_URL` (default `http://localhost:8080`)
- `JAEGER_API_URL` (default `http://localhost:16686/api`)
- `DAY96_ADMIN_EMAIL` (used only for bootstrap fallback)
- `DAY96_ADMIN_PASSWORD` (used only for bootstrap fallback)

## Done criteria

- Script exits successfully.
- Publish job reaches `done`.
- Jaeger contains `eventhub-api` and `eventhub-worker` services.
- Span attribute checks for correlation IDs pass in both API and worker traces.

## Evidence checklist

1. Screenshot `tmp/day96/summary.txt`
2. Screenshot `tmp/day96/trace_checks.txt`
3. Screenshot `tmp/day96/publish_request_headers.txt` showing `X-Request-Id`
4. Screenshot `tmp/day96/publish_request_body.json` showing `jobId`
5. Screenshot snippet from `tmp/day96/jaeger_api_traces.json` with `request.id`, `event.id`, `user.id`
6. Screenshot snippet from `tmp/day96/jaeger_worker_traces.json` with `job.id`, `request.id`, `event.id`, `user.id`
7. Jaeger UI screenshot of a `job.run` span for the same `job_id`
