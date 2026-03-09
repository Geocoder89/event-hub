#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

OUT_DIR="${OUT_DIR:-tmp/day93}"
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

PGUSER="${POSTGRES_USER:-${DB_USER:-eventhub}}"
PGPASSWORD="${POSTGRES_PASSWORD:-${DB_PASSWORD:-eventhub}}"
PGHOST="${DB_HOST:-127.0.0.1}"
PGPORT="${DB_PORT:-5433}"
PGDATABASE="${POSTGRES_DB:-${DB_NAME:-eventhub}}"
DB_SSLMODE="${DB_SSLMODE:-disable}"

export TEST_DB_DSN="postgres://${PGUSER}:${PGPASSWORD}@${PGHOST}:${PGPORT}/${PGDATABASE}?sslmode=${DB_SSLMODE}"
if command -v goose >/dev/null 2>&1; then
  GOOSE_CMD=(goose)
else
  GOOSE_CMD=(go run github.com/pressly/goose/v3/cmd/goose@latest)
fi

start_epoch="$(date +%s)"

log "Ensuring integration dependencies are running (db/redis)"
docker compose up -d db redis

log "Applying migrations for idempotency integration checks"
"${GOOSE_CMD[@]}" -dir db/migrations postgres "${TEST_DB_DSN}" up > "${OUT_DIR}/goose_up.txt" 2>&1

log "Running idempotency integration tests"
go test ./internal/http/integration \
  -run 'TestPublishPipeline_IdempotentEnqueue|TestPipeline_RegistrationsCSVExport_IdempotentEnqueue|TestRegistrationCheckInIntegration_AlreadyCheckedIn|TestPipeline_Register_EnqueuesJob_Worker_SendsOnce' \
  -v > "${OUT_DIR}/idempotency_integration_tests.txt" 2>&1

log "Running targeted pipeline tests for regression safety"
go test ./internal/http/integration \
  -run 'TestPublishPipeline_EndToEnd|TestPipeline_RegistrationsCSVExport_EnqueueProcessDownload' \
  -v > "${OUT_DIR}/pipeline_regression_tests.txt" 2>&1

docker compose ps > "${OUT_DIR}/compose_ps.txt"
docker compose logs api --tail=120 > "${OUT_DIR}/api_logs_tail.txt"
docker compose logs worker --tail=120 > "${OUT_DIR}/worker_logs_tail.txt"

end_epoch="$(date +%s)"
duration="$((end_epoch - start_epoch))"

cat > "${OUT_DIR}/summary.txt" <<EOF
Day 93 idempotency checks summary
- Duplicate publish enqueue behavior: passed
- Duplicate registrations export enqueue behavior: passed
- Duplicate check-in behavior (already_checked_in): passed
- Pipeline regression tests: passed
- TEST_DB_DSN used: ${TEST_DB_DSN}
- Duration seconds: ${duration}
EOF

log "Day 93 idempotency checks completed successfully"
log "Evidence saved under ${OUT_DIR}"
