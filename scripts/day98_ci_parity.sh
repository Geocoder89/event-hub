#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

OUT_DIR="${OUT_DIR:-tmp/day98}"
DB_CONTAINER="${DB_CONTAINER:-eventhub-db}"
TEMP_DB_NAME="${TEMP_DB_NAME:-eventhub_day98_$(date +%s)}"

mkdir -p "${OUT_DIR}"

log() {
  printf '[%s] %s\n' "$(date +"%Y-%m-%dT%H:%M:%S%z")" "$*"
}

run_step() {
  local label="$1"
  local logfile="$2"
  shift 2

  log "Running ${label}"
  if "$@" > "${logfile}" 2>&1; then
    log "PASS ${label}"
    return 0
  fi

  log "FAIL ${label} (see ${logfile})"
  return 1
}

sql_admin_exec() {
  local sql="$1"

  if command -v psql >/dev/null 2>&1; then
    PGPASSWORD="${PGPASSWORD}" psql \
      "postgres://${PGUSER}:${PGPASSWORD}@${PGHOST}:${PGPORT}/postgres?sslmode=${DB_SSLMODE}" \
      -v ON_ERROR_STOP=1 \
      -q \
      -c "${sql}"
    return 0
  fi

  docker exec -e PGPASSWORD="${PGPASSWORD}" "${DB_CONTAINER}" \
    psql -U "${PGUSER}" -d postgres -v ON_ERROR_STOP=1 -q -c "${sql}"
}

wait_db_ready() {
  local tries="${1:-60}"

  for _ in $(seq 1 "${tries}"); do
    if docker exec "${DB_CONTAINER}" pg_isready -U "${PGUSER}" -d "${PGDATABASE}" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done

  return 1
}

docker_build_target() {
  local target="$1"

  docker build \
    --target "${target}" \
    --platform linux/amd64 \
    -t "eventhub-${target}:day98-local" \
    -f Dockerfile \
    .
}

ensure_golangci_lint() {
  if command -v golangci-lint >/dev/null 2>&1; then
    return 0
  fi

  log "golangci-lint not found; installing via go install"
  go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest > "${OUT_DIR}/install_golangci_lint.txt" 2>&1
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

if ! command -v go >/dev/null 2>&1; then
  log "FAIL go CLI not found in PATH"
  exit 1
fi

if ! command -v docker >/dev/null 2>&1; then
  log "FAIL docker CLI not found in PATH"
  exit 1
fi

if ! docker info >/dev/null 2>&1; then
  log "FAIL docker daemon is not running. Start Docker Desktop and rerun."
  exit 1
fi

PGUSER="${POSTGRES_USER:-${DB_USER:-eventhub}}"
PGPASSWORD="${POSTGRES_PASSWORD:-${DB_PASSWORD:-eventhub}}"
PGHOST="${DB_HOST:-127.0.0.1}"
PGPORT="${DB_PORT:-5433}"
PGDATABASE="${POSTGRES_DB:-${DB_NAME:-eventhub}}"
DB_SSLMODE="${DB_SSLMODE:-disable}"
export PGPASSWORD

TEST_DB_DSN="postgres://${PGUSER}:${PGPASSWORD}@${PGHOST}:${PGPORT}/${TEMP_DB_NAME}?sslmode=${DB_SSLMODE}"
GOOSE_CMD=(go run github.com/pressly/goose/v3/cmd/goose@latest)

start_epoch="$(date +%s)"

log "Starting local DB/Redis stack for CI parity"
docker compose up -d db redis

if ! wait_db_ready; then
  log "FAIL postgres did not become ready in time"
  exit 1
fi

ensure_golangci_lint

run_step "create temp CI database" "${OUT_DIR}/create_temp_db.txt" \
  sql_admin_exec "CREATE DATABASE ${TEMP_DB_NAME};"

run_step "goose migrate temp CI database" "${OUT_DIR}/goose_up.txt" \
  "${GOOSE_CMD[@]}" -dir db/migrations postgres "${TEST_DB_DSN}" up

run_step "go fmt" "${OUT_DIR}/fmt.txt" go fmt ./...
run_step "go vet" "${OUT_DIR}/vet.txt" go vet ./...
run_step "go build api+worker" "${OUT_DIR}/build_binaries.txt" sh -c 'go build -v ./cmd/api && go build -v ./cmd/worker'
run_step "go test" "${OUT_DIR}/test.txt" env TEST_DB_DSN="${TEST_DB_DSN}" go test ./... -v
run_step "golangci-lint" "${OUT_DIR}/lint.txt" golangci-lint run ./...
run_step "gosec" "${OUT_DIR}/gosec.txt" golangci-lint run --no-config --enable-only gosec ./...
run_step "govulncheck" "${OUT_DIR}/govuln.txt" go run golang.org/x/vuln/cmd/govulncheck@latest ./...
run_step "docker build api target" "${OUT_DIR}/docker_build_api.txt" docker_build_target api
run_step "docker build worker target" "${OUT_DIR}/docker_build_worker.txt" docker_build_target worker

docker compose ps > "${OUT_DIR}/compose_ps.txt"
if command -v rg >/dev/null 2>&1; then
  docker images --format '{{.Repository}}:{{.Tag}} {{.ID}}' | rg '^eventhub-(api|worker):day98-local' > "${OUT_DIR}/docker_images.txt"
else
  docker images --format '{{.Repository}}:{{.Tag}} {{.ID}}' | grep -E '^eventhub-(api|worker):day98-local' > "${OUT_DIR}/docker_images.txt"
fi

end_epoch="$(date +%s)"
duration="$((end_epoch - start_epoch))"

cat > "${OUT_DIR}/summary.txt" <<EOF
Day 98 CI parity summary
- temp test database: ${TEMP_DB_NAME}
- go fmt: passed
- go vet: passed
- go build (api, worker): passed
- go test ./...: passed
- golangci-lint: passed
- gosec: passed
- govulncheck: passed
- docker build target api: passed
- docker build target worker: passed
- Duration seconds: ${duration}
EOF

log "Day 98 CI parity checks completed successfully"
log "Evidence saved under ${OUT_DIR}"
