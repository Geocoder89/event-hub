#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

OUT_DIR="${OUT_DIR:-tmp/day92}"
API_BASE_URL="${API_BASE_URL:-http://localhost:8080}"
WORKER_BASE_URL="${WORKER_BASE_URL:-http://localhost:8081}"

mkdir -p "${OUT_DIR}"

log() {
  printf '[%s] %s\n' "$(date +"%Y-%m-%dT%H:%M:%S%z")" "$*"
}

http_status() {
  local url="$1"
  curl -sS -o /dev/null -w "%{http_code}" "${url}" || true
}

wait_http_status() {
  local url="$1"
  local expected="$2"
  local label="$3"
  local tries="${4:-60}"

  for i in $(seq 1 "${tries}"); do
    code="$(http_status "${url}")"
    if [[ "${code}" == "${expected}" ]]; then
      log "PASS ${label}: ${code}"
      return 0
    fi
    sleep 1
  done

  log "FAIL ${label}: expected ${expected}, got ${code:-unknown}"
  exit 1
}

dump_http() {
  local prefix="$1"
  local url="$2"
  curl -sS -D "${prefix}.headers.txt" "${url}" > "${prefix}.body.txt" || true
}

restore_dependencies() {
  docker compose up -d db redis >/dev/null 2>&1 || true
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

trap restore_dependencies EXIT

start_epoch="$(date +%s)"

log "Starting API/worker dependency stack"
docker compose up -d db redis jaeger api worker

log "Waiting for baseline readiness"
wait_http_status "${API_BASE_URL}/healthz" "200" "api healthz baseline"
wait_http_status "${API_BASE_URL}/readyz" "200" "api readyz baseline"
wait_http_status "${WORKER_BASE_URL}/healthz" "200" "worker healthz baseline"
wait_http_status "${WORKER_BASE_URL}/readyz" "200" "worker readyz baseline"
dump_http "${OUT_DIR}/baseline_api_readyz" "${API_BASE_URL}/readyz"
dump_http "${OUT_DIR}/baseline_worker_readyz" "${WORKER_BASE_URL}/readyz"

log "Scenario 1: stop Redis and verify readiness degradation"
docker compose stop redis
wait_http_status "${API_BASE_URL}/readyz" "503" "api readyz with redis down"
wait_http_status "${WORKER_BASE_URL}/readyz" "200" "worker readyz with redis down"
dump_http "${OUT_DIR}/redis_down_api_readyz" "${API_BASE_URL}/readyz"
dump_http "${OUT_DIR}/redis_down_worker_readyz" "${WORKER_BASE_URL}/readyz"

log "Scenario 1 recovery: restart Redis"
docker compose up -d redis
wait_http_status "${API_BASE_URL}/readyz" "200" "api readyz after redis recovery"
wait_http_status "${WORKER_BASE_URL}/readyz" "200" "worker readyz after redis recovery"
dump_http "${OUT_DIR}/redis_recovered_api_readyz" "${API_BASE_URL}/readyz"
dump_http "${OUT_DIR}/redis_recovered_worker_readyz" "${WORKER_BASE_URL}/readyz"

log "Scenario 2: stop DB and verify readiness degradation"
docker compose stop db
wait_http_status "${API_BASE_URL}/readyz" "503" "api readyz with db down"
wait_http_status "${WORKER_BASE_URL}/readyz" "503" "worker readyz with db down"
dump_http "${OUT_DIR}/db_down_api_readyz" "${API_BASE_URL}/readyz"
dump_http "${OUT_DIR}/db_down_worker_readyz" "${WORKER_BASE_URL}/readyz"

log "Scenario 2 recovery: restart DB"
docker compose up -d db
wait_http_status "${API_BASE_URL}/readyz" "200" "api readyz after db recovery"
wait_http_status "${WORKER_BASE_URL}/readyz" "200" "worker readyz after db recovery"
dump_http "${OUT_DIR}/db_recovered_api_readyz" "${API_BASE_URL}/readyz"
dump_http "${OUT_DIR}/db_recovered_worker_readyz" "${WORKER_BASE_URL}/readyz"

docker compose ps > "${OUT_DIR}/compose_ps.txt"
docker compose logs api --tail=200 > "${OUT_DIR}/api_logs_tail.txt"
docker compose logs worker --tail=200 > "${OUT_DIR}/worker_logs_tail.txt"

end_epoch="$(date +%s)"
duration="$((end_epoch - start_epoch))"

cat > "${OUT_DIR}/summary.txt" <<EOF
Day 92 dependency failure drill summary
- Baseline: API readyz 200, Worker readyz 200
- Redis down: API readyz 503, Worker readyz 200
- Redis recovered: API readyz 200, Worker readyz 200
- DB down: API readyz 503, Worker readyz 503
- DB recovered: API readyz 200, Worker readyz 200
- Duration seconds: ${duration}
EOF

log "Day 92 dependency failure drill completed successfully"
log "Evidence saved under ${OUT_DIR}"
