#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

OUT_DIR="${OUT_DIR:-tmp/day91}"
RULE_FILE="${RULE_FILE:-monitoring/alerts/eventhub-alerts.yml}"
PROM_API_URL="${PROM_API_URL:-http://localhost:9090}"
API_BASE_URL="${API_BASE_URL:-http://localhost:8080}"

mkdir -p "${OUT_DIR}"

log() {
  printf '[%s] %s\n' "$(date +"%Y-%m-%dT%H:%M:%S%z")" "$*"
}

assert_file_contains() {
  local file="$1"
  local pattern="$2"
  if command -v rg >/dev/null 2>&1; then
    if rg -n --fixed-strings "${pattern}" "${file}" >/dev/null; then
      return 0
    fi
  else
    if grep -nF "${pattern}" "${file}" >/dev/null; then
      return 0
    fi
  fi
  log "FAIL ${file} missing pattern: ${pattern}"
  exit 1
}

wait_http_200() {
  local url="$1"
  local name="$2"
  local tries="${3:-40}"
  for i in $(seq 1 "${tries}"); do
    code="$(curl -sS -o /dev/null -w "%{http_code}" "${url}" || true)"
    if [[ "${code}" == "200" ]]; then
      log "PASS ${name}: 200"
      return 0
    fi
    sleep 1
  done
  log "FAIL ${name}: did not return 200 in time (${url})"
  exit 1
}

wait_worker_metric() {
  local metric="$1"
  local tries="${2:-30}"

  for i in $(seq 1 "${tries}"); do
    docker exec eventhub-worker wget -qO- "http://127.0.0.1:8081/metrics" > "${OUT_DIR}/worker_metrics_snapshot.txt"
    if grep -q "${metric}" "${OUT_DIR}/worker_metrics_snapshot.txt"; then
      log "PASS worker metric present: ${metric}"
      return 0
    fi
    sleep 1
  done

  log "FAIL worker metric not found after waiting: ${metric}"
  exit 1
}

if [[ -f ".env" ]]; then
  while IFS= read -r line || [[ -n "${line}" ]]; do
    line="${line%$'\r'}"
    [[ -z "${line}" || "${line}" == \#* ]] && continue
    [[ "${line}" != *"="* ]] && continue

    key="${line%%=*}"
    value="${line#*=}"
    key="${key#"${key%%[![:space:]]*}"}"
    key="${key%"${key##*[![:space:]]}"}"
    value="${value#"${value%%[![:space:]]*}"}"
    value="${value%"${value##*[![:space:]]}"}"
    export "${key}=${value}"
  done < ".env"
fi

if ! command -v docker >/dev/null 2>&1; then
  log "FAIL docker CLI not found in PATH"
  exit 1
fi

if ! docker info >/dev/null 2>&1; then
  log "FAIL docker daemon is not running. Start Docker Desktop and rerun."
  exit 1
fi

if ! command -v go >/dev/null 2>&1; then
  log "FAIL go CLI not found in PATH"
  exit 1
fi

if ! command -v curl >/dev/null 2>&1; then
  log "FAIL curl not found in PATH"
  exit 1
fi

if [[ ! -f "${RULE_FILE}" ]]; then
  log "FAIL alert rule file not found: ${RULE_FILE}"
  exit 1
fi

PGUSER="${POSTGRES_USER:-${DB_USER:-eventhub}}"
PGPASSWORD="${POSTGRES_PASSWORD:-${DB_PASSWORD:-eventhub}}"
PGHOST="${DB_HOST:-127.0.0.1}"
PGPORT="${DB_PORT:-5433}"
PGDATABASE="${POSTGRES_DB:-${DB_NAME:-eventhub}}"
DB_SSLMODE="${DB_SSLMODE:-disable}"

export TEST_DB_DSN="postgres://${PGUSER}:${PGPASSWORD}@${PGHOST}:${PGPORT}/${PGDATABASE}?sslmode=${DB_SSLMODE}"
GOOSE_CMD=(go run github.com/pressly/goose/v3/cmd/goose@latest)

start_epoch="$(date +%s)"

log "Starting local observability stack"
docker compose up -d db redis jaeger prometheus grafana
docker compose up -d --build api worker
docker compose up -d --force-recreate prometheus

log "Waiting for API/Prometheus readiness"
wait_http_200 "${API_BASE_URL}/healthz" "api healthz"
wait_http_200 "${API_BASE_URL}/metrics" "api metrics"
wait_http_200 "${PROM_API_URL}/-/ready" "prometheus ready"

log "Checking worker health endpoints from inside container"
docker exec eventhub-worker wget -qO- "http://127.0.0.1:8081/healthz" > "${OUT_DIR}/worker_healthz.json"
docker exec eventhub-worker wget -qO- "http://127.0.0.1:8081/readyz" > "${OUT_DIR}/worker_readyz.json"

log "Applying migrations for metric-producing API requests"
"${GOOSE_CMD[@]}" -dir db/migrations postgres "${TEST_DB_DSN}" up > "${OUT_DIR}/goose_up.txt" 2>&1

log "Generating a small amount of API traffic"
curl -sS "${API_BASE_URL}/events?limit=5" > "${OUT_DIR}/events_list_1.json"
curl -sS "${API_BASE_URL}/events?limit=5&includeTotal=true" > "${OUT_DIR}/events_list_2.json"
curl -sS "${API_BASE_URL}/events?limit=3&q=go" > "${OUT_DIR}/events_list_3.json"

log "Capturing metrics snapshots"
curl -sS "${API_BASE_URL}/metrics" > "${OUT_DIR}/api_metrics_snapshot.txt"
wait_worker_metric "eventhub_jobs_in_flight"
wait_worker_metric "eventhub_db_query_duration_seconds"

assert_file_contains "${OUT_DIR}/api_metrics_snapshot.txt" "eventhub_http_requests_total"
assert_file_contains "${OUT_DIR}/api_metrics_snapshot.txt" "eventhub_http_request_duration_seconds"
assert_file_contains "${OUT_DIR}/api_metrics_snapshot.txt" "eventhub_db_query_duration_seconds"
assert_file_contains "${OUT_DIR}/worker_metrics_snapshot.txt" "eventhub_db_query_duration_seconds"
assert_file_contains "${OUT_DIR}/worker_metrics_snapshot.txt" "eventhub_jobs_in_flight"

log "Validating alert rules with promtool inside Prometheus container"
docker compose exec -T prometheus promtool check rules "/etc/prometheus/alerts/$(basename "${RULE_FILE}")" > "${OUT_DIR}/promtool_rules_check.txt"
assert_file_contains "${OUT_DIR}/promtool_rules_check.txt" "SUCCESS"

log "Checking loaded rules through Prometheus API"
curl -sS "${PROM_API_URL}/api/v1/rules" > "${OUT_DIR}/prometheus_rules_api.json"
assert_file_contains "${OUT_DIR}/prometheus_rules_api.json" "EventHubAPIHigh5xxRate"
assert_file_contains "${OUT_DIR}/prometheus_rules_api.json" "EventHubAPIP95LatencyHigh"
assert_file_contains "${OUT_DIR}/prometheus_rules_api.json" "EventHubWorkerDown"
assert_file_contains "${OUT_DIR}/prometheus_rules_api.json" "EventHubWorkerDBErrorsBurst"
assert_file_contains "${OUT_DIR}/prometheus_rules_api.json" "EventHubWorkerNoClaimActivity"

docker compose ps > "${OUT_DIR}/compose_ps.txt"
docker compose logs api --tail=120 > "${OUT_DIR}/api_logs_tail.txt"
docker compose logs worker --tail=120 > "${OUT_DIR}/worker_logs_tail.txt"

end_epoch="$(date +%s)"
duration="$((end_epoch - start_epoch))"

cat > "${OUT_DIR}/summary.txt" <<EOF
Day 91 SLO/alerting verification summary
- Prometheus rule lint (promtool): passed
- API metrics snapshot checks: passed
- Worker metrics snapshot checks: passed
- Prometheus loaded-rule API checks: passed
- Duration seconds: ${duration}
EOF

log "Day 91 checks completed successfully"
log "Evidence saved under ${OUT_DIR}"
