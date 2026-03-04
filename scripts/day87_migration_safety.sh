#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

DB_CONTAINER="${DB_CONTAINER:-eventhub-db}"
OUT_DIR="${OUT_DIR:-tmp/day87}"
SCRATCH_DB="${SCRATCH_DB:-eventhub_migration_safety}"

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
SSLMODE="${DB_SSLMODE:-disable}"

export PGPASSWORD

SCRATCH_DSN="postgres://${PGUSER}:${PGPASSWORD}@${PGHOST}:${PGPORT}/${SCRATCH_DB}?sslmode=${SSLMODE}"
GOOSE_CMD=(go run github.com/pressly/goose/v3/cmd/goose@latest)

start_epoch="$(date +%s)"
ts="$(date +"%Y%m%d_%H%M%S")"

log "Ensuring db service is running"
docker compose up -d db

log "Waiting for database readiness"
for i in {1..30}; do
  if docker exec -e PGPASSWORD="${PGPASSWORD}" "${DB_CONTAINER}" pg_isready -U "${PGUSER}" -d "${POSTGRES_DB:-eventhub}" >/dev/null 2>&1; then
    break
  fi
  if [[ "${i}" == "30" ]]; then
    log "FAIL database did not become ready in time"
    exit 1
  fi
  sleep 1
done

log "Recreating scratch database: ${SCRATCH_DB}"
docker exec -e PGPASSWORD="${PGPASSWORD}" "${DB_CONTAINER}" dropdb --if-exists -U "${PGUSER}" "${SCRATCH_DB}"
docker exec -e PGPASSWORD="${PGPASSWORD}" "${DB_CONTAINER}" createdb -U "${PGUSER}" "${SCRATCH_DB}"

log "Step 1/3: goose up on scratch DB"
"${GOOSE_CMD[@]}" -dir db/migrations postgres "${SCRATCH_DSN}" up 2>&1 | tee "${OUT_DIR}/goose_up_1.txt"
"${GOOSE_CMD[@]}" -dir db/migrations postgres "${SCRATCH_DSN}" status 2>&1 | tee "${OUT_DIR}/goose_status_after_up_1.txt"

log "Capturing schema snapshot #1"
docker exec -e PGPASSWORD="${PGPASSWORD}" "${DB_CONTAINER}" \
  pg_dump -s --no-owner --no-privileges -U "${PGUSER}" -d "${SCRATCH_DB}" \
  > "${OUT_DIR}/schema_up_1.sql"

grep -Ev '^\\(un)?restrict ' "${OUT_DIR}/schema_up_1.sql" > "${OUT_DIR}/schema_up_1.normalized.sql"

log "Step 2/3: goose reset (down all)"
"${GOOSE_CMD[@]}" -dir db/migrations postgres "${SCRATCH_DSN}" reset 2>&1 | tee "${OUT_DIR}/goose_reset.txt"
"${GOOSE_CMD[@]}" -dir db/migrations postgres "${SCRATCH_DSN}" status 2>&1 | tee "${OUT_DIR}/goose_status_after_reset.txt"

log "Step 3/3: goose up again"
"${GOOSE_CMD[@]}" -dir db/migrations postgres "${SCRATCH_DSN}" up 2>&1 | tee "${OUT_DIR}/goose_up_2.txt"
"${GOOSE_CMD[@]}" -dir db/migrations postgres "${SCRATCH_DSN}" status 2>&1 | tee "${OUT_DIR}/goose_status_after_up_2.txt"

log "Capturing schema snapshot #2"
docker exec -e PGPASSWORD="${PGPASSWORD}" "${DB_CONTAINER}" \
  pg_dump -s --no-owner --no-privileges -U "${PGUSER}" -d "${SCRATCH_DB}" \
  > "${OUT_DIR}/schema_up_2.sql"

grep -Ev '^\\(un)?restrict ' "${OUT_DIR}/schema_up_2.sql" > "${OUT_DIR}/schema_up_2.normalized.sql"

if ! diff -u "${OUT_DIR}/schema_up_1.normalized.sql" "${OUT_DIR}/schema_up_2.normalized.sql" > "${OUT_DIR}/schema_diff.txt"; then
  log "FAIL schema drift detected after up/down/up (see ${OUT_DIR}/schema_diff.txt)"
  exit 1
fi

if grep -Eiq '\bPending\b' "${OUT_DIR}/goose_status_after_up_2.txt"; then
  log "FAIL goose status after second up still has pending migrations"
  exit 1
fi

end_epoch="$(date +%s)"
duration="$((end_epoch - start_epoch))"
sha1="$(shasum -a 256 "${OUT_DIR}/schema_up_1.normalized.sql" | awk '{print $1}')"
sha2="$(shasum -a 256 "${OUT_DIR}/schema_up_2.normalized.sql" | awk '{print $1}')"

cat > "${OUT_DIR}/summary.txt" <<EOF
Day 87 migration safety drill summary
- Scratch DB: ${SCRATCH_DB}
- Goose cycle: up -> reset -> up
- Schema drift check: passed
- Schema snapshot #1 sha256: ${sha1}
- Schema snapshot #2 sha256: ${sha2}
- Duration seconds: ${duration}
EOF

docker compose ps > "${OUT_DIR}/compose_ps.txt"

log "Day 87 drill completed successfully"
log "Evidence saved under ${OUT_DIR}"
log "Run tag: ${ts}"
