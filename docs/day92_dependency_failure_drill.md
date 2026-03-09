# Day 92 - Dependency Failure Drill (DB and Redis)

Goal: verify readiness behavior under dependency outages and recoveries for API and worker.

## One-command run

```bash
make day92
```

Equivalent:

```bash
bash ./scripts/day92_dependency_failure_drill.sh
```

## What Day 92 validates

1. Baseline readiness:
   - API `/readyz` returns `200`
   - Worker `/readyz` returns `200`
2. Redis outage:
   - stop `redis`
   - API `/readyz` degrades to `503`
   - Worker `/readyz` remains `200`
3. Redis recovery:
   - start `redis`
   - API and worker `/readyz` return to `200`
4. DB outage:
   - stop `db`
   - API `/readyz` degrades to `503`
   - Worker `/readyz` degrades to `503`
5. DB recovery:
   - start `db`
   - API and worker `/readyz` return to `200`

## Artifacts produced (`tmp/day92/`)

- `summary.txt`
- `baseline_api_readyz.headers.txt`
- `baseline_api_readyz.body.txt`
- `baseline_worker_readyz.headers.txt`
- `baseline_worker_readyz.body.txt`
- `redis_down_api_readyz.headers.txt`
- `redis_down_api_readyz.body.txt`
- `redis_down_worker_readyz.headers.txt`
- `redis_down_worker_readyz.body.txt`
- `redis_recovered_api_readyz.headers.txt`
- `redis_recovered_api_readyz.body.txt`
- `redis_recovered_worker_readyz.headers.txt`
- `redis_recovered_worker_readyz.body.txt`
- `db_down_api_readyz.headers.txt`
- `db_down_api_readyz.body.txt`
- `db_down_worker_readyz.headers.txt`
- `db_down_worker_readyz.body.txt`
- `db_recovered_api_readyz.headers.txt`
- `db_recovered_api_readyz.body.txt`
- `db_recovered_worker_readyz.headers.txt`
- `db_recovered_worker_readyz.body.txt`
- `compose_ps.txt`
- `api_logs_tail.txt`
- `worker_logs_tail.txt`

## Done criteria

- Script exits successfully.
- Summary confirms readiness degradation and recovery exactly as expected.
- Logs show dependency errors during outage windows and normal checks after recovery.

## Evidence checklist

1. Screenshot `tmp/day92/summary.txt`
2. Screenshot `tmp/day92/redis_down_api_readyz.headers.txt` (show `503`)
3. Screenshot `tmp/day92/db_down_worker_readyz.headers.txt` (show `503`)
4. Screenshot `tmp/day92/db_recovered_api_readyz.headers.txt` (show `200`)
5. Screenshot of `tmp/day92/api_logs_tail.txt` and `tmp/day92/worker_logs_tail.txt` around failure/recovery
6. Screenshot of committed:
   - `/Users/oladelemoarukhe/Documents/codes/event-hub/eventhub/scripts/day92_dependency_failure_drill.sh`
   - `/Users/oladelemoarukhe/Documents/codes/event-hub/eventhub/docs/day92_dependency_failure_drill.md`
