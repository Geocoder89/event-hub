#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

OUT_DIR="${OUT_DIR:-tmp/day97}"
API_BASE_URL="${API_BASE_URL:-http://localhost:8080}"
DB_CONTAINER="${DB_CONTAINER:-eventhub-db}"

mkdir -p "${OUT_DIR}"

log() {
  printf '[%s] %s\n' "$(date +"%Y-%m-%dT%H:%M:%S%z")" "$*"
}

status_from_headers() {
  local headers_file="$1"
  awk 'NR==1 {print $2}' "${headers_file}"
}

assert_status() {
  local headers_file="$1"
  local expected="$2"
  local label="$3"
  local got
  got="$(status_from_headers "${headers_file}")"
  if [[ "${got}" != "${expected}" ]]; then
    log "FAIL ${label}: expected HTTP ${expected}, got ${got:-unknown}"
    exit 1
  fi
}

assert_error_code() {
  local body_file="$1"
  local expected="$2"
  local label="$3"
  local got
  got="$(jq -r '.error.code // empty' "${body_file}" 2>/dev/null || true)"
  if [[ "${got}" != "${expected}" ]]; then
    log "FAIL ${label}: expected error.code=${expected}, got ${got:-empty}"
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

extract_refresh_cookie() {
  local headers_file="$1"
  awk '
    BEGIN { IGNORECASE=1 }
    /^Set-Cookie:/ {
      if ($0 ~ /refresh_token=/) {
        line=$0
        sub(/\r$/, "", line)
        sub(/^Set-Cookie:[[:space:]]*/, "", line)
        split(line, parts, ";")
        print parts[1]
        exit
      }
    }
  ' "${headers_file}"
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

create_event_for_auth_checks() {
  local dsn="$1"
  local id
  id="$(uuidgen | tr '[:upper:]' '[:lower:]')"
  sql_exec "${dsn}" "INSERT INTO events (id, title, description, city, start_at, capacity, created_at, updated_at) VALUES ('${id}', 'Day97 Security Event ${id}', 'security abuse drill', 'Toronto', NOW() + INTERVAL '3 day', 100, NOW(), NOW());"
  printf '%s' "${id}"
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

log "Starting local stack for security abuse drill"
docker compose up -d db redis jaeger
docker compose up -d --build api worker

wait_http_200 "${API_BASE_URL}/healthz" "api healthz"
wait_http_200 "${API_BASE_URL}/readyz" "api readyz"

log "Applying migrations"
"${GOOSE_CMD[@]}" -dir db/migrations postgres "${TEST_DB_DSN}" up > "${OUT_DIR}/goose_up.txt" 2>&1

log "Scenario 1: login rate-limit abuse"
RATE_EMAIL="ratelimit-$(date +%s)@example.com"
RATE_PASS="WrongPassword123!"
for i in $(seq 1 6); do
  body="$(jq -nc --arg e "${RATE_EMAIL}" --arg p "${RATE_PASS}" '{email:$e,password:$p}')"
  curl -sS -D "${OUT_DIR}/rate_limit_login_${i}.headers.txt" \
    -o "${OUT_DIR}/rate_limit_login_${i}.body.json" \
    -X POST "${API_BASE_URL}/login" \
    -H "Content-Type: application/json" \
    -d "${body}" >/dev/null || true
done

for i in $(seq 1 5); do
  assert_status "${OUT_DIR}/rate_limit_login_${i}.headers.txt" "401" "rate-limit login attempt ${i}"
  assert_error_code "${OUT_DIR}/rate_limit_login_${i}.body.json" "invalid_credentials" "rate-limit login attempt ${i}"
done
assert_status "${OUT_DIR}/rate_limit_login_6.headers.txt" "429" "rate-limit login attempt 6"
assert_error_code "${OUT_DIR}/rate_limit_login_6.body.json" "rate_limited" "rate-limit login attempt 6"

log "Scenario 2: refresh misuse without cookie"
curl -sS -D "${OUT_DIR}/refresh_no_cookie.headers.txt" \
  -o "${OUT_DIR}/refresh_no_cookie.body.json" \
  -X POST "${API_BASE_URL}/auth/refresh" >/dev/null || true
assert_status "${OUT_DIR}/refresh_no_cookie.headers.txt" "401" "refresh without cookie"
assert_error_code "${OUT_DIR}/refresh_no_cookie.body.json" "no_refresh" "refresh without cookie"

log "Scenario 3: create user session for refresh rotation tests"
USER_EMAIL="day97-user-$(date +%s)@example.com"
USER_PASS="StrongPassword123!"
USER_NAME="Day97 User"
signup_body="$(jq -nc --arg e "${USER_EMAIL}" --arg p "${USER_PASS}" --arg n "${USER_NAME}" '{email:$e,password:$p,name:$n}')"
curl -sS -D "${OUT_DIR}/signup.headers.txt" \
  -o "${OUT_DIR}/signup.body.json" \
  -X POST "${API_BASE_URL}/signup" \
  -H "Content-Type: application/json" \
  -d "${signup_body}" >/dev/null || true

assert_status "${OUT_DIR}/signup.headers.txt" "201" "signup for day97 security user"
USER_ACCESS_TOKEN="$(jq -r '.accessToken // empty' "${OUT_DIR}/signup.body.json" 2>/dev/null || true)"
if [[ -z "${USER_ACCESS_TOKEN}" ]]; then
  log "FAIL signup access token is empty"
  exit 1
fi

REFRESH_COOKIE_1="$(extract_refresh_cookie "${OUT_DIR}/signup.headers.txt")"
if [[ -z "${REFRESH_COOKIE_1}" ]]; then
  log "FAIL could not extract refresh cookie from signup response"
  exit 1
fi

log "Scenario 4: refresh misuse with malformed cookie"
curl -sS -D "${OUT_DIR}/refresh_bad_cookie.headers.txt" \
  -o "${OUT_DIR}/refresh_bad_cookie.body.json" \
  -X POST "${API_BASE_URL}/auth/refresh" \
  -H "Cookie: refresh_token=malformed.invalid.token" >/dev/null || true
assert_status "${OUT_DIR}/refresh_bad_cookie.headers.txt" "401" "refresh malformed cookie"
assert_error_code "${OUT_DIR}/refresh_bad_cookie.body.json" "invalid_refresh" "refresh malformed cookie"

log "Scenario 5: refresh rotation misuse (replay old token)"
curl -sS -D "${OUT_DIR}/refresh_rotate_first.headers.txt" \
  -o "${OUT_DIR}/refresh_rotate_first.body.json" \
  -X POST "${API_BASE_URL}/auth/refresh" \
  -H "Cookie: ${REFRESH_COOKIE_1}" >/dev/null || true
assert_status "${OUT_DIR}/refresh_rotate_first.headers.txt" "200" "first refresh with valid cookie"

NEW_ACCESS_TOKEN="$(jq -r '.accessToken // empty' "${OUT_DIR}/refresh_rotate_first.body.json" 2>/dev/null || true)"
if [[ -z "${NEW_ACCESS_TOKEN}" ]]; then
  log "FAIL first refresh did not return access token"
  exit 1
fi

curl -sS -D "${OUT_DIR}/refresh_rotate_replay_old.headers.txt" \
  -o "${OUT_DIR}/refresh_rotate_replay_old.body.json" \
  -X POST "${API_BASE_URL}/auth/refresh" \
  -H "Cookie: ${REFRESH_COOKIE_1}" >/dev/null || true
assert_status "${OUT_DIR}/refresh_rotate_replay_old.headers.txt" "401" "refresh replay with old cookie"
assert_error_code "${OUT_DIR}/refresh_rotate_replay_old.body.json" "invalid_refresh" "refresh replay with old cookie"

log "Scenario 6: access-token misuse on authenticated route"
EVENT_ID="$(create_event_for_auth_checks "${TEST_DB_DSN}")"
curl -sS -D "${OUT_DIR}/invalid_access_token.headers.txt" \
  -o "${OUT_DIR}/invalid_access_token.body.json" \
  -X GET "${API_BASE_URL}/events/${EVENT_ID}/registrations" \
  -H "Authorization: Bearer not.a.valid.token" >/dev/null || true
assert_status "${OUT_DIR}/invalid_access_token.headers.txt" "401" "invalid access token on authed endpoint"
assert_error_code "${OUT_DIR}/invalid_access_token.body.json" "unauthorized" "invalid access token on authed endpoint"

docker compose ps > "${OUT_DIR}/compose_ps.txt"
docker compose logs api --tail=200 > "${OUT_DIR}/api_logs_tail.txt"

end_epoch="$(date +%s)"
duration="$((end_epoch - start_epoch))"

cat > "${OUT_DIR}/summary.txt" <<EOF
Day 97 security abuse summary
- login rate-limit abuse handled (attempt 6 -> 429 rate_limited): yes
- refresh without cookie rejected (401 no_refresh): yes
- refresh malformed cookie rejected (401 invalid_refresh): yes
- refresh replay old token rejected after rotation (401 invalid_refresh): yes
- invalid access token rejected on auth route (401 unauthorized): yes
- Duration seconds: ${duration}
EOF

log "Day 97 security abuse checks completed successfully"
log "Evidence saved under ${OUT_DIR}"

