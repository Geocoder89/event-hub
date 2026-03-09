#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

OUT_DIR="${OUT_DIR:-tmp/day94}"
PERF_DIR="${PERF_DIR:-perf/day94}"
BASE_URL="${BASE_URL:-http://localhost:8080}"
DAY67_JSON="${DAY67_JSON:-perf/day67/k6_after.json}"
DB_CONTAINER="${DB_CONTAINER:-eventhub-db}"

mkdir -p "${OUT_DIR}" "${PERF_DIR}"

BOOTSTRAP_DEBUG_FILE="${OUT_DIR}/admin_bootstrap_debug.txt"
: > "${BOOTSTRAP_DEBUG_FILE}"

log() {
  printf '[%s] %s\n' "$(date +"%Y-%m-%dT%H:%M:%S%z")" "$*"
}

wait_http_200() {
  local url="$1"
  local label="$2"
  local tries="${3:-60}"
  local code=""
  for _ in $(seq 1 "${tries}"); do
    code="$(curl -sS -o /dev/null -w "%{http_code}" "${url}" || true)"
    if [[ "${code}" == "200" ]]; then
      log "PASS ${label}: 200"
      return 0
    fi
    sleep 1
  done
  log "FAIL ${label}: expected 200, got ${code:-unknown}"
  exit 1
}

extract_metric() {
  local file="$1"
  local jq_expr="$2"
  jq -r "${jq_expr} // empty" "${file}" 2>/dev/null || true
}

calc_delta() {
  local old="$1"
  local new="$2"
  awk -v o="${old}" -v n="${new}" 'BEGIN { printf "%.3f", (n-o) }'
}

sql_query_scalar() {
  local dsn="$1"
  local sql="$2"
  if command -v psql >/dev/null 2>&1; then
    psql "${dsn}" -tA -c "${sql}" 2>>"${BOOTSTRAP_DEBUG_FILE}" || return 1
    return 0
  fi

  docker exec -e PGPASSWORD="${PGPASSWORD}" "${DB_CONTAINER}" \
    psql -U "${PGUSER}" -d "${PGDATABASE}" -tA -c "${sql}" 2>>"${BOOTSTRAP_DEBUG_FILE}" || return 1
}

sql_exec() {
  local dsn="$1"
  local sql="$2"
  if command -v psql >/dev/null 2>&1; then
    psql "${dsn}" -v ON_ERROR_STOP=1 -q -c "${sql}" >>"${BOOTSTRAP_DEBUG_FILE}" 2>&1 || return 1
    return 0
  fi

  docker exec -e PGPASSWORD="${PGPASSWORD}" "${DB_CONTAINER}" \
    psql -U "${PGUSER}" -d "${PGDATABASE}" -v ON_ERROR_STOP=1 -q -c "${sql}" >>"${BOOTSTRAP_DEBUG_FILE}" 2>&1 || return 1
}

sql_escape() {
  printf "%s" "$1" | sed "s/'/''/g"
}

seed_events_if_empty() {
  local dsn="$1"
  local count="$2"

  local existing
  existing="$(sql_query_scalar "${dsn}" "SELECT COUNT(*) FROM events;" || echo "0")"
  existing="${existing//[[:space:]]/}"
  if [[ "${existing}" =~ ^[0-9]+$ ]] && (( existing > 0 )); then
    return 0
  fi

  log "Seeding ${count} events for k6 detail/read mix"
  for i in $(seq 1 "${count}"); do
    local id
    id="$(uuidgen | tr '[:upper:]' '[:lower:]')"
    sql_exec "${dsn}" "INSERT INTO events (id, title, description, city, start_at, capacity, created_at, updated_at) VALUES ('${id}', 'Day94 Seed Event ${i}', 'k6 baseline seed event', 'Toronto', NOW() + INTERVAL '${i} day', 200, NOW(), NOW());" || return 1
  done
}

bootstrap_admin_token_if_needed() {
  local dsn="$1"
  local email="$2"
  local password="$3"
  local name="${4:-Day94 Admin}"

  local body signup_status signup_resp login_body login_resp token safe_email
  local signup_resp_file="${OUT_DIR}/admin_bootstrap_signup_response.json"
  local login_resp_file="${OUT_DIR}/admin_bootstrap_login_response.json"
  body="$(jq -nc --arg e "${email}" --arg p "${password}" --arg n "${name}" '{email:$e,password:$p,name:$n}')"
  signup_status="$(curl -sS -o "${signup_resp_file}" -w "%{http_code}" -X POST "${BASE_URL}/signup" -H "Content-Type: application/json" -d "${body}" || true)"
  signup_resp="$(cat "${signup_resp_file}" 2>/dev/null || true)"
  echo "bootstrap email=${email} signup_status=${signup_status} signup_resp=${signup_resp}" >> "${BOOTSTRAP_DEBUG_FILE}"
  if [[ "${signup_status}" != "201" && "${signup_status}" != "409" ]]; then
    log "Admin bootstrap signup status=${signup_status}; continuing without bootstrap"
    return 1
  fi

  safe_email="$(sql_escape "${email}")"
  if ! sql_exec "${dsn}" "UPDATE users SET role='admin', updated_at=NOW() WHERE email='${safe_email}';"; then
    echo "bootstrap role update failed for email=${email}" >> "${BOOTSTRAP_DEBUG_FILE}"
    return 1
  fi

  login_body="$(jq -nc --arg e "${email}" --arg p "${password}" '{email:$e,password:$p}')"
  curl -sS -o "${login_resp_file}" -X POST "${BASE_URL}/login" -H "Content-Type: application/json" -d "${login_body}" >/dev/null || true
  login_resp="$(cat "${login_resp_file}" 2>/dev/null || true)"
  echo "bootstrap login email=${email} resp=${login_resp}" >> "${BOOTSTRAP_DEBUG_FILE}"
  token="$(echo "${login_resp}" | jq -r '.accessToken // empty' 2>/dev/null || true)"
  if [[ -n "${token}" ]]; then
    printf '%s' "${token}"
    return 0
  fi
  return 1
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

if ! command -v k6 >/dev/null 2>&1; then
  log "FAIL k6 not found in PATH"
  exit 1
fi

if ! command -v jq >/dev/null 2>&1; then
  log "FAIL jq not found in PATH"
  exit 1
fi

if ! command -v curl >/dev/null 2>&1; then
  log "FAIL curl not found in PATH"
  exit 1
fi

PGUSER="${POSTGRES_USER:-${DB_USER:-eventhub}}"
PGPASSWORD="${POSTGRES_PASSWORD:-${DB_PASSWORD:-eventhub}}"
PGHOST="${DB_HOST:-127.0.0.1}"
PGPORT="${DB_PORT:-5433}"
PGDATABASE="${POSTGRES_DB:-${DB_NAME:-eventhub}}"
DB_SSLMODE="${DB_SSLMODE:-disable}"
export PGPASSWORD
TEST_DB_DSN="postgres://${PGUSER}:${PGPASSWORD}@${PGHOST}:${PGPORT}/${PGDATABASE}?sslmode=${DB_SSLMODE}"

start_epoch="$(date +%s)"

log "Ensuring runtime stack is up for load tests"
docker compose up -d db redis jaeger api worker

wait_http_200 "${BASE_URL}/healthz" "api healthz"
wait_http_200 "${BASE_URL}/readyz" "api readyz"

log "Discovering event IDs for read-detail traffic"
seed_events_if_empty "${TEST_DB_DSN}" 5
events_json="$(curl -sS "${BASE_URL}/events?limit=20")"
echo "${events_json}" > "${OUT_DIR}/events_seed_snapshot.json"
EVENT_IDS="$(echo "${events_json}" | jq -r '.items[]?.id' | paste -sd "," -)"

log "Attempting admin login for jobs/registrations list traffic"
ADMIN_TOKEN="${K6_ADMIN_TOKEN:-}"
if [[ -z "${ADMIN_TOKEN}" ]]; then
  ADMIN_EMAIL="${ADMIN_EMAIL:-admin@example.com}"
  ADMIN_PASSWORD="${ADMIN_PASSWORD:-changeme}"
  login_body="$(jq -nc --arg e "${ADMIN_EMAIL}" --arg p "${ADMIN_PASSWORD}" '{email:$e,password:$p}')"
  login_resp="$(curl -sS -X POST "${BASE_URL}/login" -H "Content-Type: application/json" -d "${login_body}" || true)"
  echo "default admin login email=${ADMIN_EMAIL} resp=${login_resp}" >> "${BOOTSTRAP_DEBUG_FILE}"
  ADMIN_TOKEN="$(echo "${login_resp}" | jq -r '.accessToken // empty' 2>/dev/null || true)"
fi

if [[ -n "${ADMIN_TOKEN}" ]]; then
  REG_EVENT_IDS="${EVENT_IDS}"
  log "Admin token available; jobs/registrations k6 flows enabled"
else
  log "Admin login unavailable; attempting admin bootstrap for k6 auth flows"
  BOOTSTRAP_EMAIL="${K6_ADMIN_EMAIL:-day94-admin-$(date +%s)@example.com}"
  BOOTSTRAP_PASSWORD="${K6_ADMIN_PASSWORD:-StrongPassword123!}"
  if token="$(bootstrap_admin_token_if_needed "${TEST_DB_DSN}" "${BOOTSTRAP_EMAIL}" "${BOOTSTRAP_PASSWORD}")"; then
    ADMIN_TOKEN="${token}"
    REG_EVENT_IDS="${EVENT_IDS}"
    log "Admin bootstrap succeeded; jobs/registrations k6 flows enabled"
  else
    REG_EVENT_IDS=""
    log "Admin bootstrap failed; jobs/registrations k6 flows will be skipped (see ${BOOTSTRAP_DEBUG_FILE})"
  fi
fi

SMOKE_JSON="${PERF_DIR}/k6_smoke.json"
SMOKE_TXT="${PERF_DIR}/k6_smoke.txt"
BASELINE_JSON="${PERF_DIR}/k6_baseline.json"
BASELINE_TXT="${PERF_DIR}/k6_baseline.txt"

log "Running k6 smoke scenario"
k6 run load-test/k6-smoke.js \
  --env BASE_URL="${BASE_URL}" \
  --summary-export "${SMOKE_JSON}" \
  > "${SMOKE_TXT}" 2>&1

log "Running k6 baseline scenario"
k6 run load-test/k6-baseline.js \
  --env BASE_URL="${BASE_URL}" \
  --env EVENT_IDS="${EVENT_IDS}" \
  --env REG_EVENT_IDS="${REG_EVENT_IDS}" \
  --env ADMIN_TOKEN="${ADMIN_TOKEN}" \
  --env WARMUP="${K6_WARMUP:-30s}" \
  --env DURATION="${K6_DURATION:-2m}" \
  --env RATE="${K6_RATE:-50}" \
  --env PRE_VUS="${K6_PRE_VUS:-50}" \
  --env MAX_VUS="${K6_MAX_VUS:-200}" \
  --summary-export "${BASELINE_JSON}" \
  > "${BASELINE_TXT}" 2>&1

new_http_reqs_rate="$(extract_metric "${BASELINE_JSON}" '.metrics.http_reqs.rate')"
new_http_failed_rate="$(extract_metric "${BASELINE_JSON}" '.metrics.http_req_failed.value')"
new_p95="$(extract_metric "${BASELINE_JSON}" '.metrics["http_req_duration{expected_response:true}"]["p(95)"]')"
new_iter_rate="$(extract_metric "${BASELINE_JSON}" '.metrics.iterations.rate')"

old_http_reqs_rate=""
old_http_failed_rate=""
old_p95=""
old_iter_rate=""

if [[ -f "${DAY67_JSON}" ]]; then
  old_http_reqs_rate="$(extract_metric "${DAY67_JSON}" '.metrics.http_reqs.rate')"
  old_http_failed_rate="$(extract_metric "${DAY67_JSON}" '.metrics.http_req_failed.value')"
  old_p95="$(extract_metric "${DAY67_JSON}" '.metrics["http_req_duration{expected_response:true}"]["p(95)"]')"
  old_iter_rate="$(extract_metric "${DAY67_JSON}" '.metrics.iterations.rate')"
fi

{
  echo "Day 94 k6 re-baseline comparison (day67 -> day94)"
  if [[ -n "${old_http_reqs_rate}" && -n "${new_http_reqs_rate}" ]]; then
    echo "- http_reqs.rate: ${old_http_reqs_rate} -> ${new_http_reqs_rate} (delta $(calc_delta "${old_http_reqs_rate}" "${new_http_reqs_rate}"))"
  else
    echo "- http_reqs.rate: n/a"
  fi
  if [[ -n "${old_iter_rate}" && -n "${new_iter_rate}" ]]; then
    echo "- iterations.rate: ${old_iter_rate} -> ${new_iter_rate} (delta $(calc_delta "${old_iter_rate}" "${new_iter_rate}"))"
  else
    echo "- iterations.rate: n/a"
  fi
  if [[ -n "${old_p95}" && -n "${new_p95}" ]]; then
    echo "- p95 expected response duration (ms): ${old_p95} -> ${new_p95} (delta $(calc_delta "${old_p95}" "${new_p95}"))"
  else
    echo "- p95 expected response duration (ms): n/a"
  fi
  if [[ -n "${old_http_failed_rate}" && -n "${new_http_failed_rate}" ]]; then
    echo "- http_req_failed.value: ${old_http_failed_rate} -> ${new_http_failed_rate}"
  else
    echo "- http_req_failed.value: n/a"
  fi
} > "${OUT_DIR}/day67_vs_day94_comparison.txt"

end_epoch="$(date +%s)"
duration="$((end_epoch - start_epoch))"

cat > "${OUT_DIR}/summary.txt" <<EOF
Day 94 k6 re-baseline summary
- k6 smoke run: ${SMOKE_TXT}
- k6 baseline run: ${BASELINE_TXT}
- k6 smoke summary export: ${SMOKE_JSON}
- k6 baseline summary export: ${BASELINE_JSON}
- comparison report: ${OUT_DIR}/day67_vs_day94_comparison.txt
- event IDs discovered: ${EVENT_IDS}
- admin token enabled: $([[ -n "${ADMIN_TOKEN}" ]] && echo "yes" || echo "no")
- bootstrap debug file: ${BOOTSTRAP_DEBUG_FILE}
- Duration seconds: ${duration}
EOF

log "Day 94 k6 re-baseline completed successfully"
log "Evidence saved under ${OUT_DIR} and ${PERF_DIR}"
