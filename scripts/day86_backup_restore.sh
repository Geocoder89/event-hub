#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

DB_CONTAINER="${DB_CONTAINER:-eventhub-db}"
OUT_DIR="${OUT_DIR:-tmp/day86}"
RESTORE_DB="${RESTORE_DB:-eventhub_restore_test}"

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

PGUSER="${POSTGRES_USER:-eventhub}"
PGPASSWORD="${POSTGRES_PASSWORD:-eventhub}"
PGDATABASE="${POSTGRES_DB:-eventhub}"

if ! command -v docker >/dev/null 2>&1; then
  log "FAIL docker CLI not found in PATH"
  exit 1
fi

if ! docker info >/dev/null 2>&1; then
  log "FAIL docker daemon is not running. Start Docker Desktop and rerun."
  exit 1
fi

start_epoch="$(date +%s)"
ts="$(date +"%Y%m%d_%H%M%S")"
backup_file="${OUT_DIR}/${PGDATABASE}_backup_${ts}.sql"

COUNT_SQL="SELECT 'users' AS table_name, COUNT(*)::bigint AS row_count FROM users
UNION ALL SELECT 'events', COUNT(*) FROM events
UNION ALL SELECT 'registrations', COUNT(*) FROM registrations
UNION ALL SELECT 'jobs', COUNT(*) FROM jobs
ORDER BY table_name;"

log "Ensuring db service is running"
docker compose up -d db

log "Waiting for database readiness"
for i in {1..30}; do
  if docker exec -e PGPASSWORD="${PGPASSWORD}" "${DB_CONTAINER}" pg_isready -U "${PGUSER}" -d "${PGDATABASE}" >/dev/null 2>&1; then
    break
  fi
  if [[ "${i}" == "30" ]]; then
    log "FAIL database did not become ready in time"
    exit 1
  fi
  sleep 1
done

log "Capturing source row counts"
docker exec -e PGPASSWORD="${PGPASSWORD}" "${DB_CONTAINER}" \
  psql -X -A -F $'\t' -U "${PGUSER}" -d "${PGDATABASE}" -c "${COUNT_SQL}" \
  > "${OUT_DIR}/source_counts.tsv"

log "Creating backup: ${backup_file}"
docker exec -e PGPASSWORD="${PGPASSWORD}" "${DB_CONTAINER}" \
  pg_dump -U "${PGUSER}" -d "${PGDATABASE}" --no-owner --no-privileges \
  > "${backup_file}"

backup_size="$(wc -c < "${backup_file}" | tr -d ' ')"
backup_sha="$(shasum -a 256 "${backup_file}" | awk '{print $1}')"

log "Recreating restore database: ${RESTORE_DB}"
docker exec -e PGPASSWORD="${PGPASSWORD}" "${DB_CONTAINER}" dropdb --if-exists -U "${PGUSER}" "${RESTORE_DB}"
docker exec -e PGPASSWORD="${PGPASSWORD}" "${DB_CONTAINER}" createdb -U "${PGUSER}" "${RESTORE_DB}"

log "Restoring backup into ${RESTORE_DB}"
docker exec -i -e PGPASSWORD="${PGPASSWORD}" "${DB_CONTAINER}" \
  psql -v ON_ERROR_STOP=1 -X -U "${PGUSER}" -d "${RESTORE_DB}" \
  < "${backup_file}" \
  > "${OUT_DIR}/restore_output.txt"

log "Capturing restored row counts"
docker exec -e PGPASSWORD="${PGPASSWORD}" "${DB_CONTAINER}" \
  psql -X -A -F $'\t' -U "${PGUSER}" -d "${RESTORE_DB}" -c "${COUNT_SQL}" \
  > "${OUT_DIR}/restored_counts.tsv"

if ! diff -u "${OUT_DIR}/source_counts.tsv" "${OUT_DIR}/restored_counts.tsv" > "${OUT_DIR}/counts_diff.txt"; then
  log "FAIL source/restored row counts differ (see ${OUT_DIR}/counts_diff.txt)"
  exit 1
fi

docker exec -e PGPASSWORD="${PGPASSWORD}" "${DB_CONTAINER}" \
  psql -X -A -F ',' -U "${PGUSER}" -d "${RESTORE_DB}" \
  -c "SELECT id, title, city, start_at FROM events ORDER BY created_at DESC LIMIT 5;" \
  > "${OUT_DIR}/restored_events_sample.csv" || true

docker compose ps > "${OUT_DIR}/compose_ps.txt"

end_epoch="$(date +%s)"
duration="$((end_epoch - start_epoch))"

cat > "${OUT_DIR}/summary.txt" <<EOF
Day 86 backup/restore drill summary
- Source DB: ${PGDATABASE}
- Restore DB: ${RESTORE_DB}
- Backup file: ${backup_file}
- Backup size bytes: ${backup_size}
- Backup sha256: ${backup_sha}
- Row counts compare: matched
- Duration seconds: ${duration}
EOF

log "Day 86 drill completed successfully"
log "Evidence saved under ${OUT_DIR}"
