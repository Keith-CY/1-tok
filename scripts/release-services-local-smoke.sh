#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LOG_DIR="$(mktemp -d /tmp/1tok-services-smoke.XXXXXX)"

MOCK_FIBER_PORT="${MOCK_FIBER_PORT:-18090}"
API_GATEWAY_PORT="${API_GATEWAY_PORT:-18080}"
SETTLEMENT_PORT="${SETTLEMENT_PORT:-18083}"
EXECUTION_PORT="${EXECUTION_PORT:-18085}"

SERVICE_TOKEN="${SERVICE_TOKEN:-local-service-token}"
EXECUTION_EVENT_TOKEN="${EXECUTION_EVENT_TOKEN:-local-execution-event-token}"

MOCK_FIBER_LOG="$LOG_DIR/mock-fiber.log"
API_LOG="$LOG_DIR/api-gateway.log"
SETTLEMENT_LOG="$LOG_DIR/settlement.log"
EXECUTION_LOG="$LOG_DIR/execution.log"

cleanup() {
  local code=$?
  trap - EXIT
  for pid in "${EXECUTION_PID:-}" "${SETTLEMENT_PID:-}" "${API_PID:-}" "${MOCK_FIBER_PID:-}"; do
    if [[ -n "${pid}" ]] && kill -0 "$pid" >/dev/null 2>&1; then
      kill "$pid" >/dev/null 2>&1 || true
      wait "$pid" >/dev/null 2>&1 || true
    fi
  done

  if [[ $code -ne 0 ]]; then
    echo "services local smoke failed; logs are in $LOG_DIR" >&2
    for file in "$MOCK_FIBER_LOG" "$API_LOG" "$SETTLEMENT_LOG" "$EXECUTION_LOG"; do
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

cd "$ROOT_DIR"

MOCK_FIBER_ADDR="127.0.0.1:${MOCK_FIBER_PORT}" \
CGO_ENABLED=0 go run ./cmd/mock-fiber >"$MOCK_FIBER_LOG" 2>&1 &
MOCK_FIBER_PID=$!

API_GATEWAY_ADDR="127.0.0.1:${API_GATEWAY_PORT}" \
API_GATEWAY_EXECUTION_TOKEN="${SERVICE_TOKEN}" \
CGO_ENABLED=0 go run ./cmd/api-gateway >"$API_LOG" 2>&1 &
API_PID=$!

SETTLEMENT_ADDR="127.0.0.1:${SETTLEMENT_PORT}" \
FIBER_RPC_URL="http://127.0.0.1:${MOCK_FIBER_PORT}" \
FIBER_APP_ID="app_local" \
FIBER_HMAC_SECRET="secret_local" \
SETTLEMENT_SERVICE_TOKEN="${SERVICE_TOKEN}" \
CGO_ENABLED=0 go run ./cmd/settlement >"$SETTLEMENT_LOG" 2>&1 &
SETTLEMENT_PID=$!

EXECUTION_ADDR="127.0.0.1:${EXECUTION_PORT}" \
API_GATEWAY_UPSTREAM="http://127.0.0.1:${API_GATEWAY_PORT}" \
EXECUTION_EVENT_TOKEN="${EXECUTION_EVENT_TOKEN}" \
EXECUTION_GATEWAY_TOKEN="${SERVICE_TOKEN}" \
CGO_ENABLED=0 go run ./cmd/execution >"$EXECUTION_LOG" 2>&1 &
EXECUTION_PID=$!

wait_for_url "http://127.0.0.1:${MOCK_FIBER_PORT}/healthz" "mock-fiber"
wait_for_url "http://127.0.0.1:${API_GATEWAY_PORT}/healthz" "api-gateway"
wait_for_url "http://127.0.0.1:${SETTLEMENT_PORT}/healthz" "settlement"
wait_for_url "http://127.0.0.1:${EXECUTION_PORT}/healthz" "execution"

RELEASE_SMOKE_API_BASE_URL="http://127.0.0.1:${API_GATEWAY_PORT}" \
RELEASE_SMOKE_SETTLEMENT_BASE_URL="http://127.0.0.1:${SETTLEMENT_PORT}" \
RELEASE_SMOKE_SETTLEMENT_SERVICE_TOKEN="${SERVICE_TOKEN}" \
RELEASE_SMOKE_EXECUTION_BASE_URL="http://127.0.0.1:${EXECUTION_PORT}" \
RELEASE_SMOKE_EXECUTION_EVENT_TOKEN="${EXECUTION_EVENT_TOKEN}" \
bun run release:smoke
