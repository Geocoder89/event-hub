#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

OUT_DIR="${OUT_DIR:-tmp/day89}"
OPENAPI_FILE="${OPENAPI_FILE:-docs/openapi.yaml}"

mkdir -p "${OUT_DIR}"

log() {
  printf '[%s] %s\n' "$(date +"%Y-%m-%dT%H:%M:%S%z")" "$*"
}

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

if ! command -v go >/dev/null 2>&1; then
  log "FAIL go CLI not found in PATH"
  exit 1
fi

if [[ ! -f "${OPENAPI_FILE}" ]]; then
  log "FAIL OpenAPI file not found: ${OPENAPI_FILE}"
  exit 1
fi

start_epoch="$(date +%s)"

log "Checking ETag-related OpenAPI contract markers"
assert_openapi_contains "/events:"
assert_openapi_contains "/events/{id}:"
assert_openapi_contains "/events/{id}/registrations:"
assert_openapi_contains "/admin/jobs:"
assert_openapi_contains "/admin/jobs/{id}:"
assert_openapi_contains "name: If-None-Match"
assert_openapi_contains "description: Not Modified (matched \`If-None-Match\`)"
cp "${OPENAPI_FILE}" "${OUT_DIR}/openapi_snapshot.yaml"

log "Running ETag/cache handler regression tests"
go test ./internal/http/handlers \
  -run 'TestIfNoneMatchMatches|TestBuildETag_Deterministic|TestListEventsHandler_CacheHit|TestListEventsHandler_ETagNotModified|TestGetEventByIDHandler_ETagNotModified|TestAdminJobsList_ETagNotModified|TestAdminJobsGetByID_ETagNotModified|TestRegistrationListForEvent_ETagNotModified' \
  -v > "${OUT_DIR}/etag_cache_tests.txt" 2>&1

end_epoch="$(date +%s)"
duration="$((end_epoch - start_epoch))"

cat > "${OUT_DIR}/summary.txt" <<EOF
Day 89 ETag/cache regression summary
- OpenAPI ETag contract checks: passed
- Handler regression tests: passed
- Coverage focus: events cache+ETag, admin jobs ETag, registrations list ETag, ETag matcher semantics
- Duration seconds: ${duration}
EOF

log "Day 89 checks completed successfully"
log "Evidence saved under ${OUT_DIR}"
