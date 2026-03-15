# Day 95 - Grafana As Code

Goal: keep dashboards versioned and reproducible by provisioning Grafana datasource/dashboard from repo files, then verifying load through Grafana APIs.

## One-command run

```bash
make day95
```

Equivalent:

```bash
bash ./scripts/day95_grafana_as_code.sh
```

## What Day 95 verifies

1. Starts local stack including Prometheus and Grafana.
2. Applies latest DB migrations.
3. Generates sample API traffic so dashboard queries have metric activity.
4. Verifies provisioned Prometheus datasource via Grafana API.
5. Verifies provisioned dashboard via Grafana Search + Dashboard UID APIs.
6. Confirms expected panel titles exist in dashboard JSON served by Grafana.

## Provisioned files

- `/Users/oladelemoarukhe/Documents/codes/event-hub/eventhub/monitoring/grafana/provisioning/datasources/prometheus.yml`
- `/Users/oladelemoarukhe/Documents/codes/event-hub/eventhub/monitoring/grafana/provisioning/dashboards/eventhub.yml`
- `/Users/oladelemoarukhe/Documents/codes/event-hub/eventhub/monitoring/grafana/dashboards/eventhub-overview.json`

## Artifacts produced (`tmp/day95/`)

- `summary.txt`
- `goose_up.txt`
- `api_metrics_snapshot.txt`
- `worker_metrics_snapshot.txt`
- `grafana_datasources.json`
- `grafana_dashboard_search.json`
- `grafana_dashboard_definition.json`
- `compose_ps.txt`
- `grafana_logs_tail.txt`
- `events_list_1.json`
- `events_list_2.json`
- `events_list_3.json`

## Optional environment knobs

- `GRAFANA_URL` (default `http://localhost:3001`)
- `GRAFANA_USER` (default `admin`)
- `GRAFANA_PASSWORD` (default `admin`)
- `API_BASE_URL` (default `http://localhost:8080`)
- `PROM_API_URL` (default `http://localhost:9090`)

## Done criteria

- Script exits successfully.
- Grafana datasource UID `eventhub-prometheus` is present.
- Grafana dashboard UID `eventhub-overview` is present and queryable by UID API.
- Dashboard contains expected key panels.

## Evidence checklist

1. Screenshot `tmp/day95/summary.txt`
2. Screenshot `tmp/day95/grafana_datasources.json` showing `eventhub-prometheus`
3. Screenshot `tmp/day95/grafana_dashboard_search.json` showing `eventhub-overview`
4. Screenshot `tmp/day95/grafana_dashboard_definition.json` showing panel titles (e.g., `API Request Rate`)
5. Screenshot Grafana UI dashboard page (`EventHub Overview`) with live graphs
6. Screenshot of committed:
   - `/Users/oladelemoarukhe/Documents/codes/event-hub/eventhub/scripts/day95_grafana_as_code.sh`
   - `/Users/oladelemoarukhe/Documents/codes/event-hub/eventhub/docs/day95_grafana_as_code.md`
   - `/Users/oladelemoarukhe/Documents/codes/event-hub/eventhub/monitoring/grafana/provisioning/datasources/prometheus.yml`
   - `/Users/oladelemoarukhe/Documents/codes/event-hub/eventhub/monitoring/grafana/provisioning/dashboards/eventhub.yml`
   - `/Users/oladelemoarukhe/Documents/codes/event-hub/eventhub/monitoring/grafana/dashboards/eventhub-overview.json`
