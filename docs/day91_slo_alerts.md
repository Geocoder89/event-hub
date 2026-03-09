# Day 91 - SLO and Alerting Verification

Goal: make observability actionable by validating that alert rules are valid, loaded, and grounded in real API/worker metrics.

## One-command run

```bash
make day91
```

Equivalent:

```bash
bash ./scripts/day91_slo_alerts.sh
```

## What Day 91 verifies

1. Starts local stack with Prometheus/Grafana/API/worker.
2. Applies latest migrations for stable endpoint behavior.
3. Generates sample API traffic.
4. Captures API and worker `/metrics` snapshots.
5. Validates alert rules with `promtool`.
6. Confirms rules are loaded via Prometheus Rules API.

## Alert rules included

- `EventHubAPIHigh5xxRate`
- `EventHubAPIP95LatencyHigh`
- `EventHubWorkerDown`
- `EventHubWorkerDBErrorsBurst`
- `EventHubWorkerNoClaimActivity`

Rules file:

`/Users/oladelemoarukhe/Documents/codes/event-hub/eventhub/monitoring/alerts/eventhub-alerts.yml`

## Artifacts produced (`tmp/day91/`)

- `summary.txt`
- `goose_up.txt`
- `promtool_rules_check.txt`
- `prometheus_rules_api.json`
- `api_metrics_snapshot.txt`
- `worker_metrics_snapshot.txt`
- `worker_healthz.json`
- `worker_readyz.json`
- `events_list_1.json`
- `events_list_2.json`
- `events_list_3.json`
- `compose_ps.txt`
- `api_logs_tail.txt`
- `worker_logs_tail.txt`

## Done criteria

- Script exits successfully.
- `promtool_rules_check.txt` reports success.
- Metrics snapshots include key API and worker metric families.
- Prometheus rules API contains all Day 91 alerts.

## Evidence checklist

1. Screenshot `tmp/day91/summary.txt`
2. Screenshot `tmp/day91/promtool_rules_check.txt`
3. Screenshot snippet from `tmp/day91/api_metrics_snapshot.txt` showing `eventhub_http_requests_total`
4. Screenshot snippet from `tmp/day91/worker_metrics_snapshot.txt` showing `eventhub_db_query_duration_seconds` and `eventhub_jobs_in_flight`
5. Screenshot snippet from `tmp/day91/prometheus_rules_api.json` showing Day 91 alert names
6. Screenshot of committed:
   - `/Users/oladelemoarukhe/Documents/codes/event-hub/eventhub/scripts/day91_slo_alerts.sh`
   - `/Users/oladelemoarukhe/Documents/codes/event-hub/eventhub/docs/day91_slo_alerts.md`
