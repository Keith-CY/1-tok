#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BASE_COMPOSE_FILE="${ROOT_DIR}/compose.yaml"
FNN_COMPOSE_FILE="${ROOT_DIR}/compose.fnn.yaml"
E2E_COMPOSE_FILE="${ROOT_DIR}/compose.e2e.yaml"
LOG_DIR="$(mktemp -d /tmp/1tok-compose-e2e-screenshots.XXXXXX)"
ARTIFACT_DIR="${E2E_SCREENSHOT_OUTPUT_DIR:-$(mktemp -d /tmp/1tok-e2e-screenshots.XXXXXX)}"

mkdir -p "${ARTIFACT_DIR}"
ARTIFACT_DIR="$(cd "${ARTIFACT_DIR}" && pwd)"
BUN_VERSION="${BUN_VERSION:-$(grep -o 'bun@[^"]*' "${ROOT_DIR}/package.json" | cut -d'@' -f2)}"

COMPOSE_PROJECT_NAME="${COMPOSE_PROJECT_NAME:-1tok-e2e-screenshots}"
ONE_TOK_EXECUTION_GATEWAY_TOKEN="${ONE_TOK_EXECUTION_GATEWAY_TOKEN:-local-execution-gateway-token}"
ONE_TOK_SETTLEMENT_SERVICE_TOKEN="${ONE_TOK_SETTLEMENT_SERVICE_TOKEN:-local-settlement-service-token}"
ONE_TOK_EXECUTION_EVENT_TOKEN="${ONE_TOK_EXECUTION_EVENT_TOKEN:-local-execution-event-token}"
CARRIER_GATEWAY_API_TOKEN="${CARRIER_GATEWAY_API_TOKEN:-test-gateway-token}"
POSTGRES_PUBLISHED_PORT="${POSTGRES_PUBLISHED_PORT:-35432}"
NATS_PUBLISHED_PORT="${NATS_PUBLISHED_PORT:-34222}"
NATS_MONITOR_PUBLISHED_PORT="${NATS_MONITOR_PUBLISHED_PORT:-38222}"
MOCK_FIBER_PUBLISHED_PORT="${MOCK_FIBER_PUBLISHED_PORT:-38090}"
MOCK_CARRIER_PUBLISHED_PORT="${MOCK_CARRIER_PUBLISHED_PORT:-38787}"
IAM_PUBLISHED_PORT="${IAM_PUBLISHED_PORT:-38081}"
API_GATEWAY_PUBLISHED_PORT="${API_GATEWAY_PUBLISHED_PORT:-38080}"
MARKETPLACE_PUBLISHED_PORT="${MARKETPLACE_PUBLISHED_PORT:-38082}"
SETTLEMENT_PUBLISHED_PORT="${SETTLEMENT_PUBLISHED_PORT:-38083}"
EXECUTION_PUBLISHED_PORT="${EXECUTION_PUBLISHED_PORT:-38085}"
WEB_PUBLISHED_PORT="${WEB_PUBLISHED_PORT:-33000}"
FNN_PUBLISHED_RPC_PORT="${FNN_PUBLISHED_RPC_PORT:-38227}"
FNN_PUBLISHED_P2P_PORT="${FNN_PUBLISHED_P2P_PORT:-38228}"
FNN_VERSION="${FNN_VERSION:-v0.6.1}"
FNN_ASSET="${FNN_ASSET:-fnn_v0.6.1-x86_64-linux-portable.tar.gz}"
FNN_ASSET_SHA256="${FNN_ASSET_SHA256:-8f9a69361f662438fa1fc29ddc668192810b13021536ebd1101c84dc0cfa330f}"
FIBER_SECRET_KEY_PASSWORD="${FIBER_SECRET_KEY_PASSWORD:-local-fnn-dev-password}"
FNN_CKB_RPC_URL="${FNN_CKB_RPC_URL:-https://testnet.ckbapp.dev/}"

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
    MARKETPLACE_PUBLISHED_PORT="${MARKETPLACE_PUBLISHED_PORT}" \
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
    BUN_VERSION="${BUN_VERSION}" \
    E2E_SCREENSHOT_HOST_OUTPUT_DIR="${ARTIFACT_DIR}" \
    docker compose -f "${BASE_COMPOSE_FILE}" -f "${FNN_COMPOSE_FILE}" -f "${E2E_COMPOSE_FILE}" "$@"
  )
}

cleanup() {
  local code=$?
  trap - EXIT

  compose logs >"${LOG_DIR}/compose.log" 2>&1 || true
  compose down -v --remove-orphans >/dev/null 2>&1 || true
  docker run --rm -v "${ARTIFACT_DIR}:/artifacts" alpine:3.20 chown -R "$(id -u):$(id -g)" /artifacts >/dev/null 2>&1 || true

  if [[ $code -ne 0 ]]; then
    echo "compose e2e screenshots failed; logs are in ${LOG_DIR}" >&2
    if [[ -f "${LOG_DIR}/compose.log" ]]; then
      tail -n 400 "${LOG_DIR}/compose.log" >&2 || true
    fi
  else
    rm -rf "${LOG_DIR}"
  fi

  echo "e2e screenshots are in ${ARTIFACT_DIR}"
  exit "$code"
}
trap cleanup EXIT

wait_for_container() {
  local service="$1"
  local attempts="${2:-180}"

  for ((i = 0; i < attempts; i++)); do
    local container_id
    container_id="$(compose ps -q "${service}" 2>/dev/null || true)"
    if [[ -n "${container_id}" ]]; then
      printf '%s\n' "${container_id}"
      return 0
    fi
    sleep 2
  done

  echo "timed out waiting for ${service} container" >&2
  return 1
}

wait_for_exit() {
  local container_id="$1"
  local label="$2"
  local attempts="${3:-300}"

  for ((i = 0; i < attempts; i++)); do
    local status
    status="$(docker inspect -f '{{.State.Status}}' "${container_id}" 2>/dev/null || true)"
    if [[ "${status}" == "exited" ]]; then
      docker inspect -f '{{.State.ExitCode}}' "${container_id}"
      return 0
    fi
    sleep 2
  done

  echo "timed out waiting for ${label} to exit" >&2
  return 1
}

cd "$ROOT_DIR"
./scripts/compose-fnn-reference.test.sh

compose up -d --build e2e-screenshot-runner
runner_id="$(wait_for_container e2e-screenshot-runner)"
exit_code="$(wait_for_exit "${runner_id}" "e2e-screenshot-runner")"

if [[ "${exit_code}" != "0" ]]; then
  echo "e2e-screenshot-runner failed with exit code ${exit_code}" >&2
  exit "${exit_code}"
fi
