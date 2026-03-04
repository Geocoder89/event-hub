#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

OUT_DIR="${OUT_DIR:-tmp/day88}"
OPENAPI_FILE="${OPENAPI_FILE:-docs/openapi.yaml}"

mkdir -p "${OUT_DIR}"

log() {
  printf '[%s] %s\n' "$(date +"%Y-%m-%dT%H:%M:%S%z")" "$*"
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

if [[ ! -f "${OPENAPI_FILE}" ]]; then
  log "FAIL OpenAPI file not found: ${OPENAPI_FILE}"
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
GOOSE_DSN="postgres://${PGUSER}:${PGPASSWORD}@${PGHOST}:${PGPORT}/${PGDATABASE}?sslmode=${DB_SSLMODE}"

assert_openapi_contains() {
  local pattern="$1"
  if command -v rg >/dev/null 2>&1; then
    if rg -n --fixed-strings "${pattern}" "${OPENAPI_FILE}" >/dev/null; then
      return 0
    fi
  else
    if grep -nF "${pattern}" "${OPENAPI_FILE}" >/dev/null; then
      return 0
    fi
  fi
  if [[ $? -ne 0 ]]; then
    log "FAIL OpenAPI missing pattern: ${pattern}"
    exit 1
  fi
}

start_epoch="$(date +%s)"

log "Ensuring local dependencies are running (db/redis/jaeger)"
docker compose up -d db redis jaeger

log "Applying migrations to contract test DB"
"${GOOSE_CMD[@]}" -dir db/migrations postgres "${GOOSE_DSN}" up \
  > "${OUT_DIR}/goose_up.txt" 2>&1

log "Checking critical OpenAPI contract coverage"
assert_openapi_contains "/events:"
assert_openapi_contains "/events/{id}:"
assert_openapi_contains "/admin/jobs:"
assert_openapi_contains "/admin/events/{id}/registrations/check-in:"
assert_openapi_contains "/admin/events/{id}/registrations/export:"
assert_openapi_contains "name: If-None-Match"
assert_openapi_contains "\"415\":"
assert_openapi_contains "nextCursor"
cp "${OPENAPI_FILE}" "${OUT_DIR}/openapi_snapshot.yaml"

log "Running handler contract tests (ETag + pagination + validation)"
go test ./internal/http/handlers \
  -run 'TestBindJSON_ValidationErrorsUseJSONFieldNames|TestBindJSON_TypeMismatchUsesJSONFieldNames|TestBuildCursorPageResponse|TestListEventsHandler_ETagNotModified|TestGetEventByIDHandler_ETagNotModified' \
  -v > "${OUT_DIR}/handlers_contract_tests.txt" 2>&1

log "Running middleware contract tests (content-type gate)"
go test ./internal/http/middlewares \
  -run 'TestRequireJSON' \
  -v > "${OUT_DIR}/middlewares_contract_tests.txt" 2>&1

log "Running integration contract tests (check-in + export CSV)"
go test ./internal/http/integration \
  -run 'TestRegistrationCheckInIntegration_Success|TestRegistrationCheckInIntegration_AlreadyCheckedIn|TestRegistrationCheckInIntegration_InvalidToken|TestPipeline_RegistrationsCSVExport_EnqueueProcessDownload' \
  -v > "${OUT_DIR}/integration_contract_tests.txt" 2>&1

docker compose ps > "${OUT_DIR}/compose_ps.txt"

end_epoch="$(date +%s)"
duration="$((end_epoch - start_epoch))"

cat > "${OUT_DIR}/summary.txt" <<EOF
Day 88 API contract check summary
- OpenAPI coverage checks: passed
- Handler contract tests: passed
- Middleware contract tests: passed
- Integration contract tests: passed
- TEST_DB_DSN used: ${TEST_DB_DSN}
- Duration seconds: ${duration}
EOF

log "Day 88 contract checks completed successfully"
log "Evidence saved under ${OUT_DIR}"
