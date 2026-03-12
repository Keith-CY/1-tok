#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LOG_DIR="$(mktemp -d /tmp/1tok-portal-smoke.XXXXXX)"

IAM_PORT="${IAM_PORT:-18081}"
API_GATEWAY_PORT="${API_GATEWAY_PORT:-18080}"
EXECUTION_PORT="${EXECUTION_PORT:-18085}"
WEB_PORT="${WEB_PORT:-13000}"
WEB_HOST="${WEB_HOST:-localhost}"

EXECUTION_GATEWAY_SHARED_TOKEN="${EXECUTION_GATEWAY_SHARED_TOKEN:-local-execution-gateway-token}"
EXECUTION_EVENT_TOKEN="${EXECUTION_EVENT_TOKEN:-local-execution-event-token}"

IAM_LOG="$LOG_DIR/iam.log"
API_LOG="$LOG_DIR/api-gateway.log"
EXECUTION_LOG="$LOG_DIR/execution.log"
WEB_LOG="$LOG_DIR/web.log"

cleanup() {
  local code=$?
  trap - EXIT
  for pid in "${WEB_PID:-}" "${EXECUTION_PID:-}" "${API_PID:-}" "${IAM_PID:-}"; do
    if [[ -n "${pid}" ]] && kill -0 "$pid" >/dev/null 2>&1; then
      kill "$pid" >/dev/null 2>&1 || true
      wait "$pid" >/dev/null 2>&1 || true
    fi
  done

  if [[ $code -ne 0 ]]; then
    echo "portal local smoke failed; logs are in $LOG_DIR" >&2
    for file in "$IAM_LOG" "$API_LOG" "$EXECUTION_LOG" "$WEB_LOG"; do
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

IAM_ADDR="127.0.0.1:${IAM_PORT}" \
CGO_ENABLED=0 go run ./cmd/iam >"$IAM_LOG" 2>&1 &
IAM_PID=$!

API_GATEWAY_ADDR="127.0.0.1:${API_GATEWAY_PORT}" \
IAM_UPSTREAM="http://127.0.0.1:${IAM_PORT}" \
API_GATEWAY_EXECUTION_TOKEN="${EXECUTION_GATEWAY_SHARED_TOKEN}" \
CGO_ENABLED=0 go run ./cmd/api-gateway >"$API_LOG" 2>&1 &
API_PID=$!

EXECUTION_ADDR="127.0.0.1:${EXECUTION_PORT}" \
API_GATEWAY_UPSTREAM="http://127.0.0.1:${API_GATEWAY_PORT}" \
EXECUTION_EVENT_TOKEN="${EXECUTION_EVENT_TOKEN}" \
EXECUTION_GATEWAY_TOKEN="${EXECUTION_GATEWAY_SHARED_TOKEN}" \
CGO_ENABLED=0 go run ./cmd/execution >"$EXECUTION_LOG" 2>&1 &
EXECUTION_PID=$!

bun run build:web >/dev/null

PORT="${WEB_PORT}" \
HOSTNAME="127.0.0.1" \
NEXT_PUBLIC_API_BASE_URL="http://127.0.0.1:${API_GATEWAY_PORT}" \
IAM_BASE_URL="http://127.0.0.1:${IAM_PORT}" \
ONE_TOK_ALLOW_INSECURE_SESSION_COOKIE="true" \
node ./apps/web/.next/standalone/apps/web/server.js >"$WEB_LOG" 2>&1 &
WEB_PID=$!

wait_for_url "http://127.0.0.1:${IAM_PORT}/healthz" "iam"
wait_for_url "http://127.0.0.1:${API_GATEWAY_PORT}/healthz" "api-gateway"
wait_for_url "http://127.0.0.1:${EXECUTION_PORT}/healthz" "execution"
wait_for_url "http://${WEB_HOST}:${WEB_PORT}/login" "web"

RELEASE_PORTAL_SMOKE_WEB_BASE_URL="http://${WEB_HOST}:${WEB_PORT}" \
RELEASE_PORTAL_SMOKE_API_BASE_URL="http://127.0.0.1:${API_GATEWAY_PORT}" \
RELEASE_PORTAL_SMOKE_IAM_BASE_URL="http://127.0.0.1:${IAM_PORT}" \
RELEASE_PORTAL_SMOKE_EXECUTION_BASE_URL="http://127.0.0.1:${EXECUTION_PORT}" \
RELEASE_PORTAL_SMOKE_EXECUTION_EVENT_TOKEN="${EXECUTION_EVENT_TOKEN}" \
bun run release:portal-smoke
