# Day 94 - k6 Re-Baseline and Comparison

Goal: rerun smoke/baseline load tests and compare current performance against Day 67 baseline outputs.

## One-command run

```bash
make day94
```

Equivalent:

```bash
bash ./scripts/day94_k6_rebaseline.sh
```

## What Day 94 validates

1. Confirms API health/readiness before load.
2. Ensures there are seed events for read-detail traffic (if none exist).
3. Attempts admin login and auto-bootstraps an admin test user when needed.
4. Runs smoke load test (`load-test/k6-smoke.js`).
5. Runs baseline load test (`load-test/k6-baseline.js`).
6. Exports fresh k6 summaries to `perf/day94`.
7. Produces a comparison report versus `perf/day67/k6_after.json`.

## Artifacts produced

### `perf/day94/`

- `k6_smoke.txt`
- `k6_smoke.json`
- `k6_baseline.txt`
- `k6_baseline.json`

### `tmp/day94/`

- `summary.txt`
- `events_seed_snapshot.json`
- `day67_vs_day94_comparison.txt`
- `admin_bootstrap_debug.txt`

## Optional tuning knobs

- `K6_WARMUP` (default `30s`)
- `K6_DURATION` (default `2m`)
- `K6_RATE` (default `50`)
- `K6_PRE_VUS` (default `50`)
- `K6_MAX_VUS` (default `200`)
- `K6_ADMIN_EMAIL` (optional bootstrap admin email)
- `K6_ADMIN_PASSWORD` (optional bootstrap admin password)
- `K6_ADMIN_TOKEN` (optional direct admin access token; skips login/bootstrap path)

Example shorter run:

```bash
K6_WARMUP=10s K6_DURATION=45s K6_RATE=30 make day94
```

Example forcing admin-auth traffic with an existing token:

```bash
K6_ADMIN_TOKEN="<access-token>" make day94
```

## Done criteria

- Script exits successfully.
- Fresh Day 94 smoke/baseline outputs are generated.
- Comparison report contains day67->day94 metric deltas.

## Evidence checklist

1. Screenshot `tmp/day94/summary.txt`
2. Screenshot `tmp/day94/day67_vs_day94_comparison.txt`
3. Screenshot from `perf/day94/k6_baseline.txt` showing run completion and key metrics
4. Screenshot from `perf/day94/k6_smoke.txt` showing smoke pass
5. Screenshot `tmp/day94/admin_bootstrap_debug.txt` if bootstrap/login path is used
6. Screenshot of committed:
   - `/Users/oladelemoarukhe/Documents/codes/event-hub/eventhub/scripts/day94_k6_rebaseline.sh`
   - `/Users/oladelemoarukhe/Documents/codes/event-hub/eventhub/docs/day94_k6_rebaseline.md`
