#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
WORKER_CONTAINER="${WORKER_CONTAINER:-eventhub-worker}"
OUT_DIR="${OUT_DIR:-tmp/day83}"

mkdir -p "${OUT_DIR}"

ts() {
  date +"%Y-%m-%dT%H:%M:%S%z"
}

log() {
  printf '[%s] %s\n' "$(ts)" "$*"
}

http_status() {
  local url="$1"
  curl -sS -o /dev/null -w "%{http_code}" "${url}"
}

assert_status() {
  local name="$1"
  local expected="$2"
  local actual="$3"
  if [[ "${actual}" != "${expected}" ]]; then
    log "FAIL ${name}: expected ${expected}, got ${actual}"
    exit 1
  fi
  log "PASS ${name}: ${actual}"
}

write_http_dump() {
  local file="$1"
  local url="$2"
  curl -sS -D "${file}.headers.txt" "${url}" > "${file}.body.txt"
}

log "Day 83 local deployment-readiness check starting"

if ! command -v docker >/dev/null 2>&1; then
  log "FAIL docker CLI not found in PATH"
  exit 1
fi

if ! docker info >/dev/null 2>&1; then
  log "FAIL docker daemon is not running. Start Docker Desktop and rerun."
  exit 1
fi

log "Bringing stack up"
docker compose up -d db redis jaeger prometheus grafana api worker

log "Capturing docker compose status"
docker compose ps > "${OUT_DIR}/compose_ps.txt"

api_health="$(http_status "${BASE_URL}/healthz")"
assert_status "api healthz" "200" "${api_health}"
write_http_dump "${OUT_DIR}/api_healthz" "${BASE_URL}/healthz"

api_ready="$(http_status "${BASE_URL}/readyz")"
assert_status "api readyz" "200" "${api_ready}"
write_http_dump "${OUT_DIR}/api_readyz" "${BASE_URL}/readyz"

api_metrics="$(http_status "${BASE_URL}/metrics")"
assert_status "api metrics" "200" "${api_metrics}"
write_http_dump "${OUT_DIR}/api_metrics" "${BASE_URL}/metrics"

swagger_ui="$(http_status "${BASE_URL}/swagger")"
assert_status "swagger ui" "200" "${swagger_ui}"
write_http_dump "${OUT_DIR}/swagger" "${BASE_URL}/swagger"

openapi_status="$(http_status "${BASE_URL}/docs/openapi.yaml")"
assert_status "openapi spec" "200" "${openapi_status}"
write_http_dump "${OUT_DIR}/openapi" "${BASE_URL}/docs/openapi.yaml"

if ! grep -q "^openapi:" "${OUT_DIR}/openapi.body.txt"; then
  log "FAIL openapi body does not include top-level 'openapi:' key"
  exit 1
fi
log "PASS openapi body includes top-level key"

log "Checking worker health endpoints from inside container"
docker exec "${WORKER_CONTAINER}" wget -qO- "http://127.0.0.1:8081/healthz" > "${OUT_DIR}/worker_healthz.body.txt"
docker exec "${WORKER_CONTAINER}" wget -qO- "http://127.0.0.1:8081/readyz" > "${OUT_DIR}/worker_readyz.body.txt"
log "PASS worker healthz/readyz reachable"

log "Capturing recent API and worker logs"
docker compose logs api --tail=120 > "${OUT_DIR}/api_logs_tail.txt"
docker compose logs worker --tail=120 > "${OUT_DIR}/worker_logs_tail.txt"

cat > "${OUT_DIR}/summary.txt" <<EOF
Day 83 local deployment-readiness summary
- API /healthz: 200
- API /readyz: 200
- API /metrics: 200
- API /swagger: 200
- API /docs/openapi.yaml: 200
- Worker /healthz: reachable from container
- Worker /readyz: reachable from container
- docker compose status: ${OUT_DIR}/compose_ps.txt
EOF

log "Completed successfully. Evidence saved under ${OUT_DIR}"
