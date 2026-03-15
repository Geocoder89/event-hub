#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

OUT_DIR="${OUT_DIR:-tmp/day99}"
REHEARSAL_DIR="${REHEARSAL_DIR:-/tmp/eventhub-day99-rehearsal}"
COMPOSE_PROJECT_NAME="${COMPOSE_PROJECT_NAME:-eventhub-day99}"
COMPOSE_FILES_VALUE="docker-compose.day99.yml"
BASE_URL="${BASE_URL:-http://localhost:18080}"
WORKER_CONTAINER="${WORKER_CONTAINER:-eventhub-day99-worker}"
DB_HOST_PORT="${DB_HOST_PORT:-15433}"
DB_NAME="${DB_NAME:-eventhub}"
DB_USER="${DB_USER:-eventhub}"
DB_PASSWORD="${DB_PASSWORD:-eventhub}"
DB_SSLMODE="${DB_SSLMODE:-disable}"

mkdir -p "${OUT_DIR}"

log() {
  printf '[%s] %s\n' "$(date +"%Y-%m-%dT%H:%M:%S%z")" "$*"
}

copy_workspace() {
  local src="$1"
  local dst="$2"

  case "${dst}" in
    /tmp/eventhub-day99-*) ;;
    *)
      log "FAIL rehearsal directory must stay under /tmp/eventhub-day99-*"
      exit 1
      ;;
  esac

  rm -rf "${dst}"
  mkdir -p "${dst}"

  if command -v rsync >/dev/null 2>&1; then
    rsync -a \
      --delete \
      --exclude '.git' \
      --exclude '.idea' \
      --exclude '.vscode' \
      --exclude 'tmp' \
      --exclude 'bin' \
      --exclude '.DS_Store' \
      "${src}/" "${dst}/"
    return 0
  fi

  tar \
    --exclude='.git' \
    --exclude='.idea' \
    --exclude='.vscode' \
    --exclude='tmp' \
    --exclude='bin' \
    --exclude='.DS_Store' \
    -cf - . | (
      cd "${dst}" && tar -xf -
    )
}

write_rehearsal_env() {
  local env_file="$1"

  {
    printf 'APP_ENV=dev\n'
    printf 'PORT=8080\n'
    printf 'API_HOST_PORT=18080\n'
    printf 'WORKER_HOST_PORT=18081\n'
    printf 'POSTGRES_USER=%s\n' "${DB_USER}"
    printf 'POSTGRES_PASSWORD=%s\n' "${DB_PASSWORD}"
    printf 'POSTGRES_DB=%s\n' "${DB_NAME}"
    printf 'DB_HOST=127.0.0.1\n'
    printf 'DB_PORT=%s\n' "${DB_HOST_PORT}"
    printf 'DB_USER=%s\n' "${DB_USER}"
    printf 'DB_PASSWORD=%s\n' "${DB_PASSWORD}"
    printf 'DB_NAME=%s\n' "${DB_NAME}"
    printf 'DB_SSLMODE=%s\n' "${DB_SSLMODE}"
    printf 'REDIS_ADDR=127.0.0.1:16379\n'
    printf 'REDIS_PASSWORD=\n'
    printf 'REDIS_DB=0\n'
    printf 'ADMIN_EMAIL=admin@example.com\n'
    printf 'ADMIN_PASSWORD=changeme\n'
    printf 'ADMIN_NAME=EventHub Admin\n'
    printf 'ADMIN_ROLE=admin\n'
    printf 'JWT_SECRET=change_me_in_real_env_min_32_chars\n'
    printf 'JWT_ACCESS_TTL_MINUTES=60\n'
    printf 'JWT_REFRESH_TTL_DAYS=14\n'
    printf 'OTEL_EXPORTER_OTLP_ENDPOINT=jaeger:4317\n'
  } > "${env_file}"
}

run_in_rehearsal() {
  local logfile="$1"
  shift

  (
    cd "${REHEARSAL_DIR}"
    COMPOSE_PROJECT_NAME="${COMPOSE_PROJECT_NAME}" \
    COMPOSE_FILE="${COMPOSE_FILES_VALUE}" \
    "$@"
  ) > "${logfile}" 2>&1
}

run_shell_in_rehearsal() {
  local logfile="$1"
  local script="$2"

  (
    cd "${REHEARSAL_DIR}"
    COMPOSE_PROJECT_NAME="${COMPOSE_PROJECT_NAME}" \
    COMPOSE_FILE="${COMPOSE_FILES_VALUE}" \
    sh -c "${script}"
  ) > "${logfile}" 2>&1
}

wait_api_ready() {
  local tries="${1:-80}"
  local code=""

  for _ in $(seq 1 "${tries}"); do
    code="$(curl -s -o /dev/null -w "%{http_code}" "${BASE_URL}/readyz" 2>/dev/null || true)"
    if [[ "${code}" == "200" ]]; then
      return 0
    fi
    sleep 1
  done

  return 1
}

wait_db_ready() {
  local tries="${1:-40}"

  for _ in $(seq 1 "${tries}"); do
    if run_shell_in_rehearsal "${OUT_DIR}/db_ready_check.txt" \
      "docker compose exec -T db pg_isready -U '${DB_USER}' -d '${DB_NAME}'"; then
      return 0
    fi
    sleep 1
  done

  return 1
}

run_goose_up_with_retry() {
  local logfile="$1"
  local tries="${2:-6}"
  local attempt=1

  : > "${logfile}"

  while [[ "${attempt}" -le "${tries}" ]]; do
    {
      printf 'attempt=%s\n' "${attempt}"
      (
        cd "${REHEARSAL_DIR}"
        COMPOSE_PROJECT_NAME="${COMPOSE_PROJECT_NAME}" \
        COMPOSE_FILE="${COMPOSE_FILES_VALUE}" \
        go run github.com/pressly/goose/v3/cmd/goose@latest -dir db/migrations postgres \
          "postgres://${DB_USER}:${DB_PASSWORD}@127.0.0.1:${DB_HOST_PORT}/${DB_NAME}?sslmode=${DB_SSLMODE}" up
      )
    } >> "${logfile}" 2>&1 && return 0

    attempt="$((attempt + 1))"
    sleep 2
  done

  return 1
}

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

if ! command -v go >/dev/null 2>&1; then
  log "FAIL go CLI not found in PATH"
  exit 1
fi

start_epoch="$(date +%s)"

log "Copying current workspace into clean rehearsal directory"
copy_workspace "${ROOT_DIR}" "${REHEARSAL_DIR}"

log "Writing clean-machine .env for rehearsal copy"
write_rehearsal_env "${REHEARSAL_DIR}/.env"

log "Preparing exact command list"
cat > "${OUT_DIR}/rehearsal_commands.txt" <<EOF
cd ${REHEARSAL_DIR}
cp .env.example .env
# replace blank dev credentials with runnable values
cat > .env <<'ENV'
APP_ENV=dev
PORT=8080
API_HOST_PORT=18080
WORKER_HOST_PORT=18081
POSTGRES_USER=${DB_USER}
POSTGRES_PASSWORD=${DB_PASSWORD}
POSTGRES_DB=${DB_NAME}
DB_HOST=127.0.0.1
DB_PORT=${DB_HOST_PORT}
DB_USER=${DB_USER}
DB_PASSWORD=${DB_PASSWORD}
DB_NAME=${DB_NAME}
DB_SSLMODE=${DB_SSLMODE}
REDIS_ADDR=127.0.0.1:16379
REDIS_PASSWORD=
REDIS_DB=0
ADMIN_EMAIL=admin@example.com
ADMIN_PASSWORD=changeme
ADMIN_NAME=EventHub Admin
ADMIN_ROLE=admin
JWT_SECRET=change_me_in_real_env_min_32_chars
JWT_ACCESS_TTL_MINUTES=60
JWT_REFRESH_TTL_DAYS=14
OTEL_EXPORTER_OTLP_ENDPOINT=jaeger:4317
ENV
export COMPOSE_PROJECT_NAME=${COMPOSE_PROJECT_NAME}
export COMPOSE_FILE=${COMPOSE_FILES_VALUE}
docker compose down -v
docker build --target api -t eventhub-day99-api:local -f Dockerfile .
docker build --target worker -t eventhub-day99-worker:local -f Dockerfile .
docker compose up -d --no-build db redis jaeger prometheus grafana
go run github.com/pressly/goose/v3/cmd/goose@latest -dir db/migrations postgres "postgres://${DB_USER}:${DB_PASSWORD}@127.0.0.1:${DB_HOST_PORT}/${DB_NAME}?sslmode=${DB_SSLMODE}" up
docker compose up -d --no-build api worker
BASE_URL=${BASE_URL} WORKER_CONTAINER=${WORKER_CONTAINER} OUT_DIR=${OUT_DIR}/day83_rehearsal bash ./scripts/day83_local_readiness.sh
docker compose ps
docker compose logs api --tail=120
docker compose logs worker --tail=120
# cleanup when finished inspecting
docker compose down -v
EOF

log "Cleaning up any prior rehearsal stack"
run_in_rehearsal "${OUT_DIR}/compose_down.txt" docker compose down -v || true

log "Building API and worker images inside rehearsal copy"
if ! run_shell_in_rehearsal "${OUT_DIR}/docker_build_api.txt" \
  "docker build --target api -t eventhub-day99-api:local -f Dockerfile ."; then
  log "FAIL clean-machine api docker build (see ${OUT_DIR}/docker_build_api.txt)"
  exit 1
fi
if ! run_shell_in_rehearsal "${OUT_DIR}/docker_build_worker.txt" \
  "docker build --target worker -t eventhub-day99-worker:local -f Dockerfile ."; then
  log "FAIL clean-machine worker docker build (see ${OUT_DIR}/docker_build_worker.txt)"
  exit 1
fi

log "Starting isolated clean-machine infrastructure"
if ! run_in_rehearsal "${OUT_DIR}/compose_up_infra.txt" docker compose up -d --no-build db redis jaeger prometheus grafana; then
  log "FAIL clean-machine infra compose up (see ${OUT_DIR}/compose_up_infra.txt)"
  exit 1
fi

log "Waiting for rehearsal Postgres to stabilize"
if ! wait_db_ready; then
  log "FAIL clean-machine postgres did not become ready in time (see ${OUT_DIR}/db_ready_check.txt)"
  exit 1
fi

log "Applying migrations inside rehearsal copy"
if ! run_goose_up_with_retry "${OUT_DIR}/goose_up.txt"; then
  log "FAIL clean-machine migrations (see ${OUT_DIR}/goose_up.txt)"
  exit 1
fi

log "Starting API and worker from migrated rehearsal copy"
if ! run_in_rehearsal "${OUT_DIR}/compose_up_app.txt" docker compose up -d --no-build api worker; then
  log "FAIL clean-machine app compose up (see ${OUT_DIR}/compose_up_app.txt)"
  exit 1
fi

if ! wait_api_ready; then
  log "FAIL clean-machine API did not become ready in time"
  exit 1
fi

log "Running Day 83 smoke from rehearsal copy"
if ! run_in_rehearsal "${OUT_DIR}/day83_rehearsal_run.txt" \
  env BASE_URL="${BASE_URL}" WORKER_CONTAINER="${WORKER_CONTAINER}" OUT_DIR="${OUT_DIR}/day83_rehearsal" \
  bash ./scripts/day83_local_readiness.sh; then
  log "FAIL clean-machine smoke rehearsal (see ${OUT_DIR}/day83_rehearsal_run.txt)"
  exit 1
fi

mkdir -p "${OUT_DIR}/day83_rehearsal"
if [ -d "${REHEARSAL_DIR}/${OUT_DIR}/day83_rehearsal" ]; then
  cp -R "${REHEARSAL_DIR}/${OUT_DIR}/day83_rehearsal/." "${OUT_DIR}/day83_rehearsal/"
fi

run_in_rehearsal "${OUT_DIR}/compose_ps.txt" docker compose ps
run_in_rehearsal "${OUT_DIR}/api_logs_tail.txt" docker compose logs api --tail=120
run_in_rehearsal "${OUT_DIR}/worker_logs_tail.txt" docker compose logs worker --tail=120

printf '%s\n' "${REHEARSAL_DIR}" > "${OUT_DIR}/rehearsal_workspace.txt"
printf '%s\n' "${COMPOSE_PROJECT_NAME}" > "${OUT_DIR}/compose_project_name.txt"

end_epoch="$(date +%s)"
duration="$((end_epoch - start_epoch))"

cat > "${OUT_DIR}/summary.txt" <<EOF
Day 99 clean-machine rehearsal summary
- rehearsal workspace: ${REHEARSAL_DIR}
- compose project: ${COMPOSE_PROJECT_NAME}
- .env generated from clean-machine defaults: yes
- api image built from rehearsal copy: yes
- worker image built from rehearsal copy: yes
- isolated docker stack booted from rehearsal copy: yes
- migrations applied from rehearsal copy: yes
- Day 83 smoke from rehearsal copy: passed
- exact command list: ${OUT_DIR}/rehearsal_commands.txt
- cleanup command: cd ${REHEARSAL_DIR} && COMPOSE_PROJECT_NAME=${COMPOSE_PROJECT_NAME} COMPOSE_FILE=${COMPOSE_FILES_VALUE} docker compose down -v
- Duration seconds: ${duration}
EOF

log "Day 99 clean-machine rehearsal completed successfully"
log "Evidence saved under ${OUT_DIR}"
