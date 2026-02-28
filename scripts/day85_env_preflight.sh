#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

if [[ -f ".env" ]]; then
  while IFS= read -r line || [[ -n "${line}" ]]; do
    line="${line%$'\r'}"
    [[ -z "${line}" || "${line}" == \#* ]] && continue
    [[ "${line}" != *"="* ]] && continue

    key="${line%%=*}"
    value="${line#*=}"

    # Trim surrounding whitespace on key and value.
    key="${key#"${key%%[![:space:]]*}"}"
    key="${key%"${key##*[![:space:]]}"}"
    value="${value#"${value%%[![:space:]]*}"}"
    value="${value%"${value##*[![:space:]]}"}"

    export "${key}=${value}"
  done < ".env"
fi

env_name="${APP_ENV:-dev}"
force_release="${FORCE_RELEASE_CHECK:-0}"

is_release=0
if [[ "${force_release}" == "1" ]]; then
  is_release=1
elif [[ "${env_name}" != "dev" && "${env_name}" != "test" ]]; then
  is_release=1
fi

issues=()

require_non_empty() {
  local key="$1"
  local val="${!key:-}"
  if [[ -z "${val}" ]]; then
    issues+=("${key} is required")
  fi
}

# Baseline requirements (all envs)
require_non_empty "DB_HOST"
require_non_empty "DB_PORT"
require_non_empty "DB_USER"
require_non_empty "DB_NAME"
require_non_empty "DB_SSLMODE"

if [[ "${is_release}" == "1" ]]; then
  jwt_secret="${JWT_SECRET:-}"
  admin_password="${ADMIN_PASSWORD:-}"
  db_password="${DB_PASSWORD:-}"
  db_sslmode="${DB_SSLMODE:-}"

  require_non_empty "DB_PASSWORD"
  require_non_empty "REDIS_ADDR"
  require_non_empty "JWT_SECRET"
  require_non_empty "ADMIN_EMAIL"
  require_non_empty "ADMIN_PASSWORD"

  if [[ "${jwt_secret}" == "dev-secret-change-me" ]]; then
    issues+=("JWT_SECRET must not use development default in release checks")
  fi

  if [[ "${#jwt_secret}" -lt 32 ]]; then
    issues+=("JWT_SECRET must be at least 32 characters in release checks")
  fi

  if [[ "${admin_password}" == "changeme" ]]; then
    issues+=("ADMIN_PASSWORD must not use default value in release checks")
  fi

  if [[ "${db_password}" == "eventhub" ]]; then
    issues+=("DB_PASSWORD must not use default value in release checks")
  fi

  if [[ "${db_sslmode}" == "disable" ]]; then
    issues+=("DB_SSLMODE=disable is not allowed in release checks")
  fi
fi

if [[ "${#issues[@]}" -gt 0 ]]; then
  printf 'Day 85 preflight failed for APP_ENV=%s\n' "${env_name}"
  for issue in "${issues[@]}"; do
    printf ' - %s\n' "${issue}"
  done
  exit 1
fi

if [[ "${is_release}" == "1" ]]; then
  printf 'Day 85 preflight passed (release checks enabled). APP_ENV=%s\n' "${env_name}"
else
  printf 'Day 85 preflight passed (non-release mode). APP_ENV=%s\n' "${env_name}"
  printf 'Tip: FORCE_RELEASE_CHECK=1 make day85-preflight\n'
fi
