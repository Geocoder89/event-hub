#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

OUT_DIR="${OUT_DIR:-tmp/day96}"
API_BASE_URL="${API_BASE_URL:-http://localhost:8080}"
JAEGER_API_URL="${JAEGER_API_URL:-http://localhost:16686/api}"
DB_CONTAINER="${DB_CONTAINER:-eventhub-db}"

mkdir -p "${OUT_DIR}"

log() {
  printf '[%s] %s\n' "$(date +"%Y-%m-%dT%H:%M:%S%z")" "$*"
}

assert_non_empty() {
  local value="$1"
  local label="$2"
  if [[ -z "${value}" ]]; then
    log "FAIL ${label} is empty"
    exit 1
  fi
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

sql_query_scalar() {
  local dsn="$1"
  local sql="$2"
  if command -v psql >/dev/null 2>&1; then
    psql "${dsn}" -tA -c "${sql}" || return 1
    return 0
  fi

  docker exec -e PGPASSWORD="${PGPASSWORD}" "${DB_CONTAINER}" \
    psql -U "${PGUSER}" -d "${PGDATABASE}" -tA -c "${sql}" || return 1
}

sql_exec() {
  local dsn="$1"
  local sql="$2"
  if command -v psql >/dev/null 2>&1; then
    psql "${dsn}" -v ON_ERROR_STOP=1 -q -c "${sql}" || return 1
    return 0
  fi

  docker exec -e PGPASSWORD="${PGPASSWORD}" "${DB_CONTAINER}" \
    psql -U "${PGUSER}" -d "${PGDATABASE}" -v ON_ERROR_STOP=1 -q -c "${sql}" || return 1
}

sql_escape() {
  printf "%s" "$1" | sed "s/'/''/g"
}

create_trace_event() {
  local dsn="$1"
  local id
  id="$(uuidgen | tr '[:upper:]' '[:lower:]')"
  sql_exec "${dsn}" "INSERT INTO events (id, title, description, city, start_at, capacity, created_at, updated_at) VALUES ('${id}', 'Day96 Trace Event ${id}', 'trace quality drill', 'Toronto', NOW() + INTERVAL '7 day', 200, NOW(), NOW());"
  printf '%s' "${id}"
}

bootstrap_admin_token_if_needed() {
  local dsn="$1"
  local email="$2"
  local password="$3"
  local name="${4:-Day96 Admin}"

  local body signup_status login_body token safe_email
  local signup_resp_file="${OUT_DIR}/admin_bootstrap_signup_response.json"
  local login_resp_file="${OUT_DIR}/admin_bootstrap_login_response.json"

  body="$(jq -nc --arg e "${email}" --arg p "${password}" --arg n "${name}" '{email:$e,password:$p,name:$n}')"
  signup_status="$(curl -sS -o "${signup_resp_file}" -w "%{http_code}" -X POST "${API_BASE_URL}/signup" -H "Content-Type: application/json" -d "${body}" || true)"
  if [[ "${signup_status}" != "201" && "${signup_status}" != "409" ]]; then
    return 1
  fi

  safe_email="$(sql_escape "${email}")"
  sql_exec "${dsn}" "UPDATE users SET role='admin', updated_at=NOW() WHERE email='${safe_email}';" || return 1

  login_body="$(jq -nc --arg e "${email}" --arg p "${password}" '{email:$e,password:$p}')"
  curl -sS -o "${login_resp_file}" -X POST "${API_BASE_URL}/login" -H "Content-Type: application/json" -d "${login_body}" >/dev/null || true
  token="$(jq -r '.accessToken // empty' "${login_resp_file}" 2>/dev/null || true)"
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

if ! command -v curl >/dev/null 2>&1; then
  log "FAIL curl not found in PATH"
  exit 1
fi

if ! command -v jq >/dev/null 2>&1; then
  log "FAIL jq not found in PATH"
  exit 1
fi

if ! command -v go >/dev/null 2>&1; then
  log "FAIL go CLI not found in PATH"
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
GOOSE_CMD=(go run github.com/pressly/goose/v3/cmd/goose@latest)

start_epoch="$(date +%s)"

log "Starting local stack for trace quality drill"
docker compose up -d db redis jaeger
docker compose up -d --build api worker

wait_http_200 "${API_BASE_URL}/healthz" "api healthz"
wait_http_200 "${API_BASE_URL}/readyz" "api readyz"
wait_http_200 "${JAEGER_API_URL}/services" "jaeger services API"

log "Applying migrations and preparing event data"
"${GOOSE_CMD[@]}" -dir db/migrations postgres "${TEST_DB_DSN}" up > "${OUT_DIR}/goose_up.txt" 2>&1
EVENT_ID="$(create_trace_event "${TEST_DB_DSN}")"
assert_non_empty "${EVENT_ID}" "event_id"
curl -sS "${API_BASE_URL}/events/${EVENT_ID}" > "${OUT_DIR}/event_created_via_api.json"

log "Authenticating admin for publish flow"
ADMIN_TOKEN=""
ADMIN_EMAIL="${ADMIN_EMAIL:-admin@example.com}"
ADMIN_PASSWORD="${ADMIN_PASSWORD:-changeme}"
login_body="$(jq -nc --arg e "${ADMIN_EMAIL}" --arg p "${ADMIN_PASSWORD}" '{email:$e,password:$p}')"
curl -sS -o "${OUT_DIR}/admin_login_response.json" -X POST "${API_BASE_URL}/login" -H "Content-Type: application/json" -d "${login_body}" >/dev/null || true
ADMIN_TOKEN="$(jq -r '.accessToken // empty' "${OUT_DIR}/admin_login_response.json" 2>/dev/null || true)"

if [[ -z "${ADMIN_TOKEN}" ]]; then
  log "Admin login unavailable; attempting admin bootstrap for trace drill"
  BOOTSTRAP_EMAIL="${DAY96_ADMIN_EMAIL:-day96-admin-$(date +%s)@example.com}"
  BOOTSTRAP_PASSWORD="${DAY96_ADMIN_PASSWORD:-StrongPassword123!}"
  if token="$(bootstrap_admin_token_if_needed "${TEST_DB_DSN}" "${BOOTSTRAP_EMAIL}" "${BOOTSTRAP_PASSWORD}")"; then
    ADMIN_TOKEN="${token}"
  fi
fi
assert_non_empty "${ADMIN_TOKEN}" "admin token"

log "Triggering publish endpoint to generate API+worker correlated spans"
curl -sS -D "${OUT_DIR}/publish_request_headers.txt" \
  -o "${OUT_DIR}/publish_request_body.json" \
  -X POST "${API_BASE_URL}/admin/events/${EVENT_ID}/publish" \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{}' >/dev/null

PUBLISH_STATUS="$(awk 'NR==1 {print $2}' "${OUT_DIR}/publish_request_headers.txt")"
if [[ "${PUBLISH_STATUS}" != "202" ]]; then
  log "FAIL publish status expected 202, got ${PUBLISH_STATUS:-unknown}"
  exit 1
fi

REQUEST_ID="$(awk 'tolower($1)=="x-request-id:" {print $2}' "${OUT_DIR}/publish_request_headers.txt" | tr -d '\r' | tail -1)"
JOB_ID="$(jq -r '.jobId // empty' "${OUT_DIR}/publish_request_body.json" 2>/dev/null || true)"
assert_non_empty "${REQUEST_ID}" "request_id"
assert_non_empty "${JOB_ID}" "job_id"

log "Waiting for job completion"
JOB_STATUS=""
for _ in $(seq 1 60); do
  curl -sS "${API_BASE_URL}/admin/jobs/${JOB_ID}" -H "Authorization: Bearer ${ADMIN_TOKEN}" > "${OUT_DIR}/job_status_response.json" || true
  JOB_STATUS="$(jq -r '.status // empty' "${OUT_DIR}/job_status_response.json" 2>/dev/null || true)"
  if [[ "${JOB_STATUS}" == "done" ]]; then
    break
  fi
  if [[ "${JOB_STATUS}" == "failed" || "${JOB_STATUS}" == "dead" ]]; then
    log "FAIL job ended in terminal non-success status: ${JOB_STATUS}"
    exit 1
  fi
  sleep 1
done
if [[ "${JOB_STATUS}" != "done" ]]; then
  log "FAIL job did not reach done status in time"
  exit 1
fi

log "Querying Jaeger services and traces"
curl -sS "${JAEGER_API_URL}/services" > "${OUT_DIR}/jaeger_services.json"

API_MATCHED=0
for _ in $(seq 1 60); do
  curl -sS "${JAEGER_API_URL}/traces?service=eventhub-api&limit=50&lookback=1h" > "${OUT_DIR}/jaeger_api_traces.json"
  API_MATCHED="$(jq -r --arg req "${REQUEST_ID}" --arg event "${EVENT_ID}" '
    [
      .data[]?.spans[]?
      | select(any(.tags[]?; .key=="request.id" and ((.value|tostring)==$req)))
      | select(any(.tags[]?; .key=="event.id" and ((.value|tostring)==$event)))
      | select(any(.tags[]?; .key=="user.id" and ((.value|tostring)|length > 0)))
    ] | length
  ' "${OUT_DIR}/jaeger_api_traces.json")"
  if [[ "${API_MATCHED}" =~ ^[0-9]+$ ]] && (( API_MATCHED > 0 )); then
    break
  fi
  sleep 1
done
if ! [[ "${API_MATCHED}" =~ ^[0-9]+$ ]] || (( API_MATCHED == 0 )); then
  log "FAIL no API spans found with request.id + event.id + user.id correlation attributes"
  exit 1
fi

WORKER_MATCHED=0
for _ in $(seq 1 60); do
  curl -sS "${JAEGER_API_URL}/traces?service=eventhub-worker&operation=job.run&limit=50&lookback=1h" > "${OUT_DIR}/jaeger_worker_traces.json"
  WORKER_MATCHED="$(jq -r --arg req "${REQUEST_ID}" --arg event "${EVENT_ID}" --arg job "${JOB_ID}" '
    [
      .data[]?.spans[]?
      | select(.operationName=="job.run")
      | select(any(.tags[]?; .key=="job.id" and ((.value|tostring)==$job)))
      | select(any(.tags[]?; .key=="request.id" and ((.value|tostring)==$req)))
      | select(any(.tags[]?; .key=="event.id" and ((.value|tostring)==$event)))
      | select(any(.tags[]?; .key=="user.id" and ((.value|tostring)|length > 0)))
    ] | length
  ' "${OUT_DIR}/jaeger_worker_traces.json")"
  if [[ "${WORKER_MATCHED}" =~ ^[0-9]+$ ]] && (( WORKER_MATCHED > 0 )); then
    break
  fi
  sleep 1
done
if ! [[ "${WORKER_MATCHED}" =~ ^[0-9]+$ ]] || (( WORKER_MATCHED == 0 )); then
  log "FAIL no worker spans found with job.id + request.id + event.id + user.id correlation attributes"
  exit 1
fi

cat > "${OUT_DIR}/trace_checks.txt" <<EOF
request_id=${REQUEST_ID}
job_id=${JOB_ID}
event_id=${EVENT_ID}
api_span_matches=${API_MATCHED}
worker_span_matches=${WORKER_MATCHED}
EOF

docker compose ps > "${OUT_DIR}/compose_ps.txt"
docker compose logs api --tail=200 > "${OUT_DIR}/api_logs_tail.txt"
docker compose logs worker --tail=200 > "${OUT_DIR}/worker_logs_tail.txt"

end_epoch="$(date +%s)"
duration="$((end_epoch - start_epoch))"

cat > "${OUT_DIR}/summary.txt" <<EOF
Day 96 trace quality summary
- publish flow executed and completed: yes
- Jaeger services discovered: eventhub-api, eventhub-worker
- API spans include request.id + event.id + user.id: yes (${API_MATCHED} matches)
- Worker job.run spans include job.id + request.id + event.id + user.id: yes (${WORKER_MATCHED} matches)
- request_id: ${REQUEST_ID}
- job_id: ${JOB_ID}
- event_id: ${EVENT_ID}
- Duration seconds: ${duration}
EOF

log "Day 96 trace quality checks completed successfully"
log "Evidence saved under ${OUT_DIR}"
