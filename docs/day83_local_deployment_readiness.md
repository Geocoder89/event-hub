# Day 83 - Local Deployment-Readiness Checklist

Goal: validate the full local stack behaves like pre-production before paid hosting.

## Scope

- API process health and dependency readiness
- Worker process health and dependency readiness
- OpenAPI/Swagger availability
- Metrics endpoint exposure
- Service startup health in Docker Compose

## One-command run

```bash
make day83
```

Equivalent direct run:

```bash
bash ./scripts/day83_local_readiness.sh
```

## What the script validates

1. Starts required services:
   - `db`, `redis`, `jaeger`, `prometheus`, `grafana`, `api`, `worker`
2. Verifies API endpoints:
   - `GET /healthz` -> `200`
   - `GET /readyz` -> `200`
   - `GET /metrics` -> `200`
   - `GET /swagger` -> `200`
   - `GET /docs/openapi.yaml` -> `200` and contains `openapi:`
3. Verifies worker endpoints from inside container:
   - `GET /healthz` -> reachable
   - `GET /readyz` -> reachable
4. Captures recent API and worker logs for traceability.

## Evidence output

Script writes artifacts to `tmp/day83/`:

- `compose_ps.txt`
- `api_healthz.headers.txt`, `api_healthz.body.txt`
- `api_readyz.headers.txt`, `api_readyz.body.txt`
- `api_metrics.headers.txt`, `api_metrics.body.txt`
- `swagger.headers.txt`, `swagger.body.txt`
- `openapi.headers.txt`, `openapi.body.txt`
- `worker_healthz.body.txt`, `worker_readyz.body.txt`
- `api_logs_tail.txt`, `worker_logs_tail.txt`
- `summary.txt`

## Day 83 done criteria

- Script exits successfully
- `tmp/day83/summary.txt` exists
- `tmp/day83/compose_ps.txt` shows healthy/started core services
- `tmp/day83/openapi.body.txt` includes `openapi:`

## LinkedIn proof set (suggested)

1. `tmp/day83/summary.txt` screenshot
2. `docker compose ps` screenshot (from `compose_ps.txt`)
3. Swagger page screenshot and matching `openapi.body.txt`
4. API/worker log snippet showing both services healthy
