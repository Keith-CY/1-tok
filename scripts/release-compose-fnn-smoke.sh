#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BASE_COMPOSE_FILE="${ROOT_DIR}/compose.yaml"
FNN_COMPOSE_FILE="${ROOT_DIR}/compose.fnn.yaml"
LOG_DIR="$(mktemp -d /tmp/1tok-compose-fnn-smoke.XXXXXX)"

COMPOSE_PROJECT_NAME="${COMPOSE_PROJECT_NAME:-1tok-fnn-smoke}"
ONE_TOK_EXECUTION_GATEWAY_TOKEN="${ONE_TOK_EXECUTION_GATEWAY_TOKEN:-local-execution-gateway-token}"
ONE_TOK_SETTLEMENT_SERVICE_TOKEN="${ONE_TOK_SETTLEMENT_SERVICE_TOKEN:-local-settlement-service-token}"
ONE_TOK_EXECUTION_EVENT_TOKEN="${ONE_TOK_EXECUTION_EVENT_TOKEN:-local-execution-event-token}"
CARRIER_GATEWAY_API_TOKEN="${CARRIER_GATEWAY_API_TOKEN:-test-gateway-token}"
POSTGRES_PUBLISHED_PORT="${POSTGRES_PUBLISHED_PORT:-25432}"
NATS_PUBLISHED_PORT="${NATS_PUBLISHED_PORT:-24222}"
NATS_MONITOR_PUBLISHED_PORT="${NATS_MONITOR_PUBLISHED_PORT:-28222}"
MOCK_FIBER_PUBLISHED_PORT="${MOCK_FIBER_PUBLISHED_PORT:-28090}"
MOCK_CARRIER_PUBLISHED_PORT="${MOCK_CARRIER_PUBLISHED_PORT:-28787}"
IAM_PUBLISHED_PORT="${IAM_PUBLISHED_PORT:-28081}"
API_GATEWAY_PUBLISHED_PORT="${API_GATEWAY_PUBLISHED_PORT:-28080}"
SETTLEMENT_PUBLISHED_PORT="${SETTLEMENT_PUBLISHED_PORT:-28083}"
EXECUTION_PUBLISHED_PORT="${EXECUTION_PUBLISHED_PORT:-28085}"
WEB_PUBLISHED_PORT="${WEB_PUBLISHED_PORT:-23000}"
FNN_PUBLISHED_RPC_PORT="${FNN_PUBLISHED_RPC_PORT:-28227}"
FNN_PUBLISHED_P2P_PORT="${FNN_PUBLISHED_P2P_PORT:-28228}"
FNN_VERSION="${FNN_VERSION:-v0.6.1}"
FNN_ASSET="${FNN_ASSET:-fnn_v0.6.1-x86_64-linux-portable.tar.gz}"
FNN_ASSET_SHA256="${FNN_ASSET_SHA256:-}"
FIBER_SECRET_KEY_PASSWORD="${FIBER_SECRET_KEY_PASSWORD:-}"
FNN_CKB_RPC_URL="${FNN_CKB_RPC_URL:-https://testnet.ckbapp.dev/}"

require_env() {
  local name="$1"
  local value="$2"
  if [[ -z "${value}" ]]; then
    echo "${name} is required" >&2
    exit 1
  fi
}

compose() {
  (
    cd "$ROOT_DIR"
    COMPOSE_PROJECT_NAME="${COMPOSE_PROJECT_NAME}" \
    ONE_TOK_EXECUTION_GATEWAY_TOKEN="${ONE_TOK_EXECUTION_GATEWAY_TOKEN}" \
    ONE_TOK_SETTLEMENT_SERVICE_TOKEN="${ONE_TOK_SETTLEMENT_SERVICE_TOKEN}" \
    ONE_TOK_EXECUTION_EVENT_TOKEN="${ONE_TOK_EXECUTION_EVENT_TOKEN}" \
    CARRIER_GATEWAY_API_TOKEN="${CARRIER_GATEWAY_API_TOKEN}" \
    POSTGRES_PUBLISHED_PORT="${POSTGRES_PUBLISHED_PORT}" \
    NATS_PUBLISHED_PORT="${NATS_PUBLISHED_PORT}" \
    NATS_MONITOR_PUBLISHED_PORT="${NATS_MONITOR_PUBLISHED_PORT}" \
    MOCK_FIBER_PUBLISHED_PORT="${MOCK_FIBER_PUBLISHED_PORT}" \
    MOCK_CARRIER_PUBLISHED_PORT="${MOCK_CARRIER_PUBLISHED_PORT}" \
    IAM_PUBLISHED_PORT="${IAM_PUBLISHED_PORT}" \
    API_GATEWAY_PUBLISHED_PORT="${API_GATEWAY_PUBLISHED_PORT}" \
    SETTLEMENT_PUBLISHED_PORT="${SETTLEMENT_PUBLISHED_PORT}" \
    EXECUTION_PUBLISHED_PORT="${EXECUTION_PUBLISHED_PORT}" \
    WEB_PUBLISHED_PORT="${WEB_PUBLISHED_PORT}" \
    FNN_PUBLISHED_RPC_PORT="${FNN_PUBLISHED_RPC_PORT}" \
    FNN_PUBLISHED_P2P_PORT="${FNN_PUBLISHED_P2P_PORT}" \
    FNN_VERSION="${FNN_VERSION}" \
    FNN_ASSET="${FNN_ASSET}" \
    FNN_ASSET_SHA256="${FNN_ASSET_SHA256}" \
    FIBER_SECRET_KEY_PASSWORD="${FIBER_SECRET_KEY_PASSWORD}" \
    FNN_CKB_RPC_URL="${FNN_CKB_RPC_URL}" \
    docker compose -f "${BASE_COMPOSE_FILE}" -f "${FNN_COMPOSE_FILE}" "$@"
  )
}

cleanup() {
  local code=$?
  trap - EXIT

  compose logs >"${LOG_DIR}/compose.log" 2>&1 || true
  compose down -v --remove-orphans >/dev/null 2>&1 || true

  if [[ $code -ne 0 ]]; then
    echo "compose fnn smoke failed; logs are in ${LOG_DIR}" >&2
    if [[ -f "${LOG_DIR}/compose.log" ]]; then
      tail -n 300 "${LOG_DIR}/compose.log" >&2 || true
    fi
  else
    rm -rf "${LOG_DIR}"
  fi

  exit "$code"
}
trap cleanup EXIT

wait_for_url() {
  local url="$1"
  local label="$2"
  local attempts="${3:-90}"

  for ((i = 0; i < attempts; i++)); do
    if curl -fsS "$url" >/dev/null 2>&1; then
      return 0
    fi
    sleep 2
  done

  echo "timed out waiting for ${label} at ${url}" >&2
  return 1
}

wait_for_http_reachable() {
  local url="$1"
  local label="$2"
  local attempts="${3:-90}"

  for ((i = 0; i < attempts; i++)); do
    if curl -sS --max-time 3 "$url" >/dev/null 2>&1; then
      return 0
    fi
    sleep 2
  done

  echo "timed out waiting for ${label} at ${url}" >&2
  return 1
}

require_env "FNN_ASSET_SHA256" "${FNN_ASSET_SHA256}"
require_env "FIBER_SECRET_KEY_PASSWORD" "${FIBER_SECRET_KEY_PASSWORD}"

cd "$ROOT_DIR"
./scripts/compose-fnn-reference.test.sh

compose up -d --build postgres nats fnn mock-fiber mock-carrier bootstrap iam api-gateway settlement settlement-reconciler execution web

wait_for_http_reachable "http://127.0.0.1:${FNN_PUBLISHED_RPC_PORT}" "fnn"
wait_for_url "http://127.0.0.1:${IAM_PUBLISHED_PORT}/healthz" "iam"
wait_for_url "http://127.0.0.1:${API_GATEWAY_PUBLISHED_PORT}/healthz" "api-gateway"
wait_for_url "http://127.0.0.1:${SETTLEMENT_PUBLISHED_PORT}/healthz" "settlement"
wait_for_url "http://127.0.0.1:${EXECUTION_PUBLISHED_PORT}/healthz" "execution"
wait_for_url "http://127.0.0.1:${WEB_PUBLISHED_PORT}/login" "web"

RELEASE_SMOKE_API_BASE_URL="http://127.0.0.1:${API_GATEWAY_PUBLISHED_PORT}" \
RELEASE_SMOKE_IAM_BASE_URL="http://127.0.0.1:${IAM_PUBLISHED_PORT}" \
RELEASE_SMOKE_SETTLEMENT_BASE_URL="http://127.0.0.1:${SETTLEMENT_PUBLISHED_PORT}" \
RELEASE_SMOKE_SETTLEMENT_SERVICE_TOKEN="${ONE_TOK_SETTLEMENT_SERVICE_TOKEN}" \
RELEASE_SMOKE_EXECUTION_BASE_URL="http://127.0.0.1:${EXECUTION_PUBLISHED_PORT}" \
RELEASE_SMOKE_EXECUTION_EVENT_TOKEN="${ONE_TOK_EXECUTION_EVENT_TOKEN}" \
RELEASE_SMOKE_INCLUDE_WITHDRAWAL="true" \
RELEASE_SMOKE_INCLUDE_CARRIER_PROBE="true" \
bun run release:smoke

RELEASE_PORTAL_SMOKE_WEB_BASE_URL="http://127.0.0.1:${WEB_PUBLISHED_PORT}" \
RELEASE_PORTAL_SMOKE_API_BASE_URL="http://127.0.0.1:${API_GATEWAY_PUBLISHED_PORT}" \
RELEASE_PORTAL_SMOKE_IAM_BASE_URL="http://127.0.0.1:${IAM_PUBLISHED_PORT}" \
RELEASE_PORTAL_SMOKE_EXECUTION_BASE_URL="http://127.0.0.1:${EXECUTION_PUBLISHED_PORT}" \
RELEASE_PORTAL_SMOKE_EXECUTION_EVENT_TOKEN="${ONE_TOK_EXECUTION_EVENT_TOKEN}" \
bun run release:portal-smoke
