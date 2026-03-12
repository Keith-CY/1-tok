#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LOG_DIR="$(mktemp -d /tmp/1tok-external-deps-smoke.XXXXXX)"

POSTGRES_PORT="${POSTGRES_PORT:-15432}"
POSTGRES_CONTAINER="1tok-external-smoke-postgres-${POSTGRES_PORT}"
POSTGRES_DSN="postgres://onetok:onetok@127.0.0.1:${POSTGRES_PORT}/onetok?sslmode=disable"

IAM_PORT="${IAM_PORT:-18081}"
API_GATEWAY_PORT="${API_GATEWAY_PORT:-18080}"
SETTLEMENT_PORT="${SETTLEMENT_PORT:-18083}"
EXECUTION_PORT="${EXECUTION_PORT:-18085}"
WEB_PORT="${WEB_PORT:-13000}"
WEB_HOST="${WEB_HOST:-localhost}"

SERVICE_TOKEN="${SERVICE_TOKEN:-local-service-token}"
EXECUTION_EVENT_TOKEN="${EXECUTION_EVENT_TOKEN:-local-execution-event-token}"

USE_LOCAL_FIBER_MOCK="${USE_LOCAL_FIBER_MOCK:-false}"
USE_LOCAL_CARRIER_MOCK="${USE_LOCAL_CARRIER_MOCK:-false}"
MOCK_FIBER_PORT="${MOCK_FIBER_PORT:-18090}"
MOCK_CARRIER_PORT="${MOCK_CARRIER_PORT:-18787}"
MOCK_CARRIER_API_TOKEN="${MOCK_CARRIER_API_TOKEN:-test-gateway-token}"

DEPENDENCY_FIBER_RPC_URL="${DEPENDENCY_FIBER_RPC_URL:-}"
DEPENDENCY_FIBER_APP_ID="${DEPENDENCY_FIBER_APP_ID:-}"
DEPENDENCY_FIBER_HMAC_SECRET="${DEPENDENCY_FIBER_HMAC_SECRET:-}"
DEPENDENCY_CARRIER_GATEWAY_URL="${DEPENDENCY_CARRIER_GATEWAY_URL:-}"
DEPENDENCY_CARRIER_GATEWAY_API_TOKEN="${DEPENDENCY_CARRIER_GATEWAY_API_TOKEN:-}"

POSTGRES_LOG="$LOG_DIR/postgres.log"
MOCK_FIBER_LOG="$LOG_DIR/mock-fiber.log"
MOCK_CARRIER_LOG="$LOG_DIR/mock-carrier.log"
IAM_LOG="$LOG_DIR/iam.log"
API_LOG="$LOG_DIR/api-gateway.log"
SETTLEMENT_LOG="$LOG_DIR/settlement.log"
EXECUTION_LOG="$LOG_DIR/execution.log"
WEB_LOG="$LOG_DIR/web.log"

cleanup() {
  local code=$?
  trap - EXIT
  for pid in "${WEB_PID:-}" "${EXECUTION_PID:-}" "${SETTLEMENT_PID:-}" "${API_PID:-}" "${IAM_PID:-}" "${MOCK_CARRIER_PID:-}" "${MOCK_FIBER_PID:-}"; do
    if [[ -n "${pid}" ]] && kill -0 "$pid" >/dev/null 2>&1; then
      kill "$pid" >/dev/null 2>&1 || true
      wait "$pid" >/dev/null 2>&1 || true
    fi
  done

  if docker ps -a --format '{{.Names}}' | grep -Fxq "${POSTGRES_CONTAINER}"; then
    docker logs "${POSTGRES_CONTAINER}" >"${POSTGRES_LOG}" 2>&1 || true
    docker rm -f "${POSTGRES_CONTAINER}" >/dev/null 2>&1 || true
  fi

  if [[ $code -ne 0 ]]; then
    echo "external deps smoke failed; logs are in $LOG_DIR" >&2
    for file in "$POSTGRES_LOG" "$MOCK_FIBER_LOG" "$MOCK_CARRIER_LOG" "$IAM_LOG" "$API_LOG" "$SETTLEMENT_LOG" "$EXECUTION_LOG" "$WEB_LOG"; do
      if [[ -f "$file" ]]; then
        echo "===== $(basename "$file") =====" >&2
        tail -n 200 "$file" >&2 || true
      fi
    done
  else
    rm -rf "$LOG_DIR"
  fi

  exit "$code"
}
trap cleanup EXIT

wait_for_url() {
  local url="$1"
  local label="$2"
  local attempts="${3:-60}"

  for ((i = 0; i < attempts; i++)); do
    if curl -fsS "$url" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done

  echo "timed out waiting for $label at $url" >&2
  return 1
}

wait_for_postgres() {
  local attempts="${1:-60}"

  for ((i = 0; i < attempts; i++)); do
    if docker exec "${POSTGRES_CONTAINER}" pg_isready -U onetok -d onetok >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done

  echo "timed out waiting for postgres container ${POSTGRES_CONTAINER}" >&2
  return 1
}

require_env() {
  local name="$1"
  local value="$2"
  if [[ -z "${value}" ]]; then
    echo "missing required env ${name}" >&2
    exit 1
  fi
}

cd "$ROOT_DIR"

if [[ "${USE_LOCAL_FIBER_MOCK}" == "true" ]]; then
  MOCK_FIBER_ADDR="127.0.0.1:${MOCK_FIBER_PORT}" \
  CGO_ENABLED=0 go run ./cmd/mock-fiber >"$MOCK_FIBER_LOG" 2>&1 &
  MOCK_FIBER_PID=$!

  wait_for_url "http://127.0.0.1:${MOCK_FIBER_PORT}/healthz" "mock-fiber"
  DEPENDENCY_FIBER_RPC_URL="${DEPENDENCY_FIBER_RPC_URL:-http://127.0.0.1:${MOCK_FIBER_PORT}}"
  DEPENDENCY_FIBER_APP_ID="${DEPENDENCY_FIBER_APP_ID:-app_local}"
  DEPENDENCY_FIBER_HMAC_SECRET="${DEPENDENCY_FIBER_HMAC_SECRET:-secret_local}"
fi

if [[ "${USE_LOCAL_CARRIER_MOCK}" == "true" ]]; then
  MOCK_CARRIER_ADDR="127.0.0.1:${MOCK_CARRIER_PORT}" \
  MOCK_CARRIER_API_TOKEN="${MOCK_CARRIER_API_TOKEN}" \
  CGO_ENABLED=0 go run ./cmd/mock-carrier >"$MOCK_CARRIER_LOG" 2>&1 &
  MOCK_CARRIER_PID=$!

  wait_for_url "http://127.0.0.1:${MOCK_CARRIER_PORT}/healthz" "mock-carrier"
  DEPENDENCY_CARRIER_GATEWAY_URL="${DEPENDENCY_CARRIER_GATEWAY_URL:-http://127.0.0.1:${MOCK_CARRIER_PORT}}"
  DEPENDENCY_CARRIER_GATEWAY_API_TOKEN="${DEPENDENCY_CARRIER_GATEWAY_API_TOKEN:-${MOCK_CARRIER_API_TOKEN}}"
fi

require_env "DEPENDENCY_FIBER_RPC_URL" "${DEPENDENCY_FIBER_RPC_URL}"
require_env "DEPENDENCY_FIBER_APP_ID" "${DEPENDENCY_FIBER_APP_ID}"
require_env "DEPENDENCY_FIBER_HMAC_SECRET" "${DEPENDENCY_FIBER_HMAC_SECRET}"

if docker ps -a --format '{{.Names}}' | grep -Fxq "${POSTGRES_CONTAINER}"; then
  docker rm -f "${POSTGRES_CONTAINER}" >/dev/null 2>&1 || true
fi

docker run --rm -d \
  --name "${POSTGRES_CONTAINER}" \
  -e POSTGRES_DB=onetok \
  -e POSTGRES_USER=onetok \
  -e POSTGRES_PASSWORD=onetok \
  -p "${POSTGRES_PORT}:5432" \
  postgres:16-alpine >/dev/null

wait_for_postgres

IAM_ADDR="127.0.0.1:${IAM_PORT}" \
IAM_DATABASE_URL="${POSTGRES_DSN}" \
CGO_ENABLED=0 go run ./cmd/iam >"$IAM_LOG" 2>&1 &
IAM_PID=$!

wait_for_url "http://127.0.0.1:${IAM_PORT}/healthz" "iam"

API_GATEWAY_ADDR="127.0.0.1:${API_GATEWAY_PORT}" \
DATABASE_URL="${POSTGRES_DSN}" \
IAM_UPSTREAM="http://127.0.0.1:${IAM_PORT}" \
API_GATEWAY_EXECUTION_TOKEN="${SERVICE_TOKEN}" \
CGO_ENABLED=0 go run ./cmd/api-gateway >"$API_LOG" 2>&1 &
API_PID=$!

wait_for_url "http://127.0.0.1:${API_GATEWAY_PORT}/healthz" "api-gateway"

SETTLEMENT_ADDR="127.0.0.1:${SETTLEMENT_PORT}" \
SETTLEMENT_DATABASE_URL="${POSTGRES_DSN}" \
IAM_UPSTREAM="http://127.0.0.1:${IAM_PORT}" \
FIBER_RPC_URL="${DEPENDENCY_FIBER_RPC_URL}" \
FIBER_APP_ID="${DEPENDENCY_FIBER_APP_ID}" \
FIBER_HMAC_SECRET="${DEPENDENCY_FIBER_HMAC_SECRET}" \
SETTLEMENT_SERVICE_TOKEN="${SERVICE_TOKEN}" \
CGO_ENABLED=0 go run ./cmd/settlement >"$SETTLEMENT_LOG" 2>&1 &
SETTLEMENT_PID=$!

wait_for_url "http://127.0.0.1:${SETTLEMENT_PORT}/healthz" "settlement"

EXECUTION_ADDR="127.0.0.1:${EXECUTION_PORT}" \
API_GATEWAY_UPSTREAM="http://127.0.0.1:${API_GATEWAY_PORT}" \
EXECUTION_EVENT_TOKEN="${EXECUTION_EVENT_TOKEN}" \
EXECUTION_GATEWAY_TOKEN="${SERVICE_TOKEN}" \
CARRIER_GATEWAY_URL="${DEPENDENCY_CARRIER_GATEWAY_URL}" \
CARRIER_GATEWAY_API_TOKEN="${DEPENDENCY_CARRIER_GATEWAY_API_TOKEN}" \
CGO_ENABLED=0 go run ./cmd/execution >"$EXECUTION_LOG" 2>&1 &
EXECUTION_PID=$!

wait_for_url "http://127.0.0.1:${EXECUTION_PORT}/healthz" "execution"

bun run build:web >/dev/null

PORT="${WEB_PORT}" \
HOSTNAME="127.0.0.1" \
NEXT_PUBLIC_API_BASE_URL="http://127.0.0.1:${API_GATEWAY_PORT}" \
NEXT_PUBLIC_SETTLEMENT_BASE_URL="http://127.0.0.1:${SETTLEMENT_PORT}" \
IAM_BASE_URL="http://127.0.0.1:${IAM_PORT}" \
ONE_TOK_ALLOW_INSECURE_SESSION_COOKIE="true" \
node ./apps/web/.next/standalone/apps/web/server.js >"$WEB_LOG" 2>&1 &
WEB_PID=$!

wait_for_url "http://${WEB_HOST}:${WEB_PORT}/login" "web"

SMOKE_INCLUDE_CARRIER_PROBE="false"
if [[ -n "${DEPENDENCY_CARRIER_GATEWAY_URL}" ]]; then
  SMOKE_INCLUDE_CARRIER_PROBE="true"
fi

RELEASE_SMOKE_API_BASE_URL="http://127.0.0.1:${API_GATEWAY_PORT}" \
RELEASE_SMOKE_IAM_BASE_URL="http://127.0.0.1:${IAM_PORT}" \
RELEASE_SMOKE_SETTLEMENT_BASE_URL="http://127.0.0.1:${SETTLEMENT_PORT}" \
RELEASE_SMOKE_SETTLEMENT_SERVICE_TOKEN="${SERVICE_TOKEN}" \
RELEASE_SMOKE_EXECUTION_BASE_URL="http://127.0.0.1:${EXECUTION_PORT}" \
RELEASE_SMOKE_EXECUTION_EVENT_TOKEN="${EXECUTION_EVENT_TOKEN}" \
RELEASE_SMOKE_INCLUDE_WITHDRAWAL="true" \
RELEASE_SMOKE_INCLUDE_CARRIER_PROBE="${SMOKE_INCLUDE_CARRIER_PROBE}" \
bun run release:smoke

RELEASE_PORTAL_SMOKE_WEB_BASE_URL="http://${WEB_HOST}:${WEB_PORT}" \
RELEASE_PORTAL_SMOKE_API_BASE_URL="http://127.0.0.1:${API_GATEWAY_PORT}" \
RELEASE_PORTAL_SMOKE_IAM_BASE_URL="http://127.0.0.1:${IAM_PORT}" \
RELEASE_PORTAL_SMOKE_EXECUTION_BASE_URL="http://127.0.0.1:${EXECUTION_PORT}" \
RELEASE_PORTAL_SMOKE_EXECUTION_EVENT_TOKEN="${EXECUTION_EVENT_TOKEN}" \
bun run release:portal-smoke
