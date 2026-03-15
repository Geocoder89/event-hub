#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

OUT_DIR="${OUT_DIR:-tmp/day95}"
API_BASE_URL="${API_BASE_URL:-http://localhost:8080}"
PROM_API_URL="${PROM_API_URL:-http://localhost:9090}"
GRAFANA_URL="${GRAFANA_URL:-http://localhost:3001}"
GRAFANA_USER="${GRAFANA_USER:-admin}"
GRAFANA_PASSWORD="${GRAFANA_PASSWORD:-admin}"
DASHBOARD_UID="${DASHBOARD_UID:-eventhub-overview}"
DATASOURCE_UID="${DATASOURCE_UID:-eventhub-prometheus}"

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
  local tries="${3:-50}"
  local code=""
  for _ in $(seq 1 "${tries}"); do
    code="$(curl -sS -o /dev/null -w "%{http_code}" "${url}" || true)"
    if [[ "${code}" == "200" ]]; then
      log "PASS ${name}: 200"
      return 0
    fi
    sleep 1
  done
  log "FAIL ${name}: expected 200, got ${code:-unknown}"
  exit 1
}

wait_dashboard_uid() {
  local tries="${1:-50}"
  local dashboard_api="${GRAFANA_URL}/api/dashboards/uid/${DASHBOARD_UID}"
  local code=""

  for _ in $(seq 1 "${tries}"); do
    code="$(curl -sS -u "${GRAFANA_USER}:${GRAFANA_PASSWORD}" -o "${OUT_DIR}/grafana_dashboard_definition.json" -w "%{http_code}" "${dashboard_api}" || true)"
    if [[ "${code}" == "200" ]]; then
      log "PASS grafana dashboard uid resolved: ${DASHBOARD_UID}"
      return 0
    fi
    sleep 1
  done

  log "FAIL grafana dashboard uid not found: ${DASHBOARD_UID} (last code ${code:-unknown})"
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

if ! command -v jq >/dev/null 2>&1; then
  log "FAIL jq not found in PATH"
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

log "Starting local observability stack with Grafana provisioning"
docker compose up -d db redis jaeger prometheus grafana
docker compose up -d --build api worker
docker compose up -d --force-recreate prometheus grafana

wait_http_200 "${API_BASE_URL}/healthz" "api healthz"
wait_http_200 "${API_BASE_URL}/metrics" "api metrics"
wait_http_200 "${PROM_API_URL}/-/ready" "prometheus ready"
wait_http_200 "${GRAFANA_URL}/api/health" "grafana api health"

log "Applying migrations"
"${GOOSE_CMD[@]}" -dir db/migrations postgres "${TEST_DB_DSN}" up > "${OUT_DIR}/goose_up.txt" 2>&1

log "Generating sample API traffic for dashboard queries"
curl -sS "${API_BASE_URL}/events?limit=5" > "${OUT_DIR}/events_list_1.json"
curl -sS "${API_BASE_URL}/events?limit=5&includeTotal=true" > "${OUT_DIR}/events_list_2.json"
curl -sS "${API_BASE_URL}/events?limit=5&q=backend" > "${OUT_DIR}/events_list_3.json"
curl -sS "${API_BASE_URL}/metrics" > "${OUT_DIR}/api_metrics_snapshot.txt"
docker exec eventhub-worker wget -qO- "http://127.0.0.1:8081/metrics" > "${OUT_DIR}/worker_metrics_snapshot.txt"

assert_file_contains "${OUT_DIR}/api_metrics_snapshot.txt" "eventhub_http_requests_total"
assert_file_contains "${OUT_DIR}/worker_metrics_snapshot.txt" "eventhub_jobs_in_flight"

log "Checking provisioned Grafana datasource"
curl -sS -u "${GRAFANA_USER}:${GRAFANA_PASSWORD}" "${GRAFANA_URL}/api/datasources" > "${OUT_DIR}/grafana_datasources.json"
assert_file_contains "${OUT_DIR}/grafana_datasources.json" "${DATASOURCE_UID}"

log "Checking provisioned Grafana dashboard"
curl -sS -u "${GRAFANA_USER}:${GRAFANA_PASSWORD}" "${GRAFANA_URL}/api/search?query=EventHub%20Overview" > "${OUT_DIR}/grafana_dashboard_search.json"
wait_dashboard_uid
assert_file_contains "${OUT_DIR}/grafana_dashboard_search.json" "${DASHBOARD_UID}"
assert_file_contains "${OUT_DIR}/grafana_dashboard_definition.json" "API Request Rate"
assert_file_contains "${OUT_DIR}/grafana_dashboard_definition.json" "API P95 Latency"
assert_file_contains "${OUT_DIR}/grafana_dashboard_definition.json" "Worker Jobs In Flight"

docker compose ps > "${OUT_DIR}/compose_ps.txt"
docker compose logs grafana --tail=200 > "${OUT_DIR}/grafana_logs_tail.txt"

end_epoch="$(date +%s)"
duration="$((end_epoch - start_epoch))"

cat > "${OUT_DIR}/summary.txt" <<EOF
Day 95 Grafana-as-code summary
- datasource provisioned: ${DATASOURCE_UID}
- dashboard provisioned: ${DASHBOARD_UID}
- dashboard API checks: passed
- metrics snapshot checks: passed
- Duration seconds: ${duration}
EOF

log "Day 95 checks completed successfully"
log "Evidence saved under ${OUT_DIR}"
