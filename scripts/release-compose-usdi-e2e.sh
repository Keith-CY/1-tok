#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BASE_COMPOSE_FILE="${ROOT_DIR}/compose.yaml"
FNN_COMPOSE_FILE="${ROOT_DIR}/compose.fnn.yaml"
USDI_E2E_COMPOSE_FILE="${ROOT_DIR}/compose.usdi-e2e.yaml"
LOG_DIR="$(mktemp -d /tmp/1tok-compose-usdi-e2e.XXXXXX)"
ARTIFACT_DIR="${E2E_USDI_OUTPUT_DIR:-$(mktemp -d /tmp/1tok-usdi-e2e.XXXXXX)}"
ARTIFACT_DIR="$(cd "${ARTIFACT_DIR}" && pwd)"
CARRIER_REMOTE_KEY_DIR="$(mktemp -d /tmp/1tok-carrier-remote.XXXXXX)"
BUN_VERSION="${BUN_VERSION:-$(grep -o 'bun@[^\"]*' "${ROOT_DIR}/package.json" | cut -d'@' -f2)}"

COMPOSE_PROJECT_NAME="${COMPOSE_PROJECT_NAME:-1tok-usdi-e2e}"
KEEP_STACK_UP="${E2E_USDI_KEEP_STACK_UP:-0}"
ONE_TOK_EXECUTION_GATEWAY_TOKEN="${ONE_TOK_EXECUTION_GATEWAY_TOKEN:-local-execution-gateway-token}"
ONE_TOK_SETTLEMENT_SERVICE_TOKEN="${ONE_TOK_SETTLEMENT_SERVICE_TOKEN:-local-settlement-service-token}"
ONE_TOK_EXECUTION_EVENT_TOKEN="${ONE_TOK_EXECUTION_EVENT_TOKEN:-local-execution-event-token}"
CARRIER_GATEWAY_API_TOKEN="${CARRIER_GATEWAY_API_TOKEN:-test-gateway-token}"
CARRIER_SERVER_API_TOKEN="${CARRIER_SERVER_API_TOKEN:-test-daemon-token}"
OPENAI_API_KEY="${OPENAI_API_KEY:-}"
OPENAI_CODEX_TOKEN="${OPENAI_CODEX_TOKEN:-}"
OPENAI_BASE_URL="${OPENAI_BASE_URL:-}"
FIBER_APP_ID="${FIBER_APP_ID:-app_1}"
FIBER_HMAC_SECRET="${FIBER_HMAC_SECRET:-replace-me}"
FIBER_USDI_UDT_NAME="${FIBER_USDI_UDT_NAME:-}"
FIBER_USDI_UDT_TYPE_SCRIPT_JSON="${FIBER_USDI_UDT_TYPE_SCRIPT_JSON:-}"
POSTGRES_PUBLISHED_PORT="${POSTGRES_PUBLISHED_PORT:-45432}"
REDIS_PUBLISHED_PORT="${REDIS_PUBLISHED_PORT:-46379}"
NATS_PUBLISHED_PORT="${NATS_PUBLISHED_PORT:-44222}"
NATS_MONITOR_PUBLISHED_PORT="${NATS_MONITOR_PUBLISHED_PORT:-48222}"
IAM_PUBLISHED_PORT="${IAM_PUBLISHED_PORT:-48081}"
API_GATEWAY_PUBLISHED_PORT="${API_GATEWAY_PUBLISHED_PORT:-48080}"
MARKETPLACE_PUBLISHED_PORT="${MARKETPLACE_PUBLISHED_PORT:-48082}"
SETTLEMENT_PUBLISHED_PORT="${SETTLEMENT_PUBLISHED_PORT:-48083}"
EXECUTION_PUBLISHED_PORT="${EXECUTION_PUBLISHED_PORT:-48085}"
WEB_PUBLISHED_PORT="${WEB_PUBLISHED_PORT:-43000}"
FIBER_ADAPTER_PUBLISHED_PORT="${FIBER_ADAPTER_PUBLISHED_PORT:-48091}"
FNN_PUBLISHED_RPC_PORT="${FNN_PUBLISHED_RPC_PORT:-48227}"
FNN_PUBLISHED_P2P_PORT="${FNN_PUBLISHED_P2P_PORT:-48228}"
FNN2_PUBLISHED_RPC_PORT="${FNN2_PUBLISHED_RPC_PORT:-49227}"
FNN2_PUBLISHED_P2P_PORT="${FNN2_PUBLISHED_P2P_PORT:-49228}"
PROVIDER_FNN_PUBLISHED_RPC_PORT="${PROVIDER_FNN_PUBLISHED_RPC_PORT:-58227}"
PROVIDER_FNN_PUBLISHED_P2P_PORT="${PROVIDER_FNN_PUBLISHED_P2P_PORT:-58228}"
FNN_VERSION="${FNN_VERSION:-v0.6.1}"
FNN_ASSET="${FNN_ASSET:-fnn_v0.6.1-x86_64-linux-portable.tar.gz}"
FNN_ASSET_SHA256="${FNN_ASSET_SHA256:-8f9a69361f662438fa1fc29ddc668192810b13021536ebd1101c84dc0cfa330f}"
FIBER_SECRET_KEY_PASSWORD="${FIBER_SECRET_KEY_PASSWORD:-local-fnn-dev-password}"
FNN_CKB_RPC_URL="${FNN_CKB_RPC_URL:-https://testnet.ckbapp.dev/}"
FNN2_CKB_RPC_URL="${FNN2_CKB_RPC_URL:-${FNN_CKB_RPC_URL}}"
PROVIDER_FNN_CKB_RPC_URL="${PROVIDER_FNN_CKB_RPC_URL:-${FNN_CKB_RPC_URL}}"
SETTLEMENT_RECONCILER_INTERVAL="${SETTLEMENT_RECONCILER_INTERVAL:-5s}"
CARRIER_E2E_REMOTE_PRIVATE_KEY_PATH="${CARRIER_E2E_REMOTE_PRIVATE_KEY_PATH:-${CARRIER_REMOTE_KEY_DIR}/id_ed25519}"
CARRIER_E2E_REMOTE_AUTHORIZED_KEY="${CARRIER_E2E_REMOTE_AUTHORIZED_KEY:-}"

E2E_CKB_EXPLORER_BASE_URL="${E2E_CKB_EXPLORER_BASE_URL:-https://pudge.explorer.nervos.org}"
E2E_CKB_FAUCET_API_BASE="${E2E_CKB_FAUCET_API_BASE:-https://faucet-api.nervos.org}"
E2E_CKB_FAUCET_FALLBACK_API_BASE="${E2E_CKB_FAUCET_FALLBACK_API_BASE:-https://ckb-utilities.random-walk.co.jp/api}"
E2E_CKB_FAUCET_ENABLE_FALLBACK="${E2E_CKB_FAUCET_ENABLE_FALLBACK:-1}"
E2E_CKB_FAUCET_AMOUNT="${E2E_CKB_FAUCET_AMOUNT:-100000}"
E2E_CKB_FAUCET_WAIT_SECONDS="${E2E_CKB_FAUCET_WAIT_SECONDS:-20}"
E2E_CKB_BALANCE_WAIT_TIMEOUT_SECONDS="${E2E_CKB_BALANCE_WAIT_TIMEOUT_SECONDS:-180}"
E2E_CKB_BALANCE_POLL_INTERVAL_SECONDS="${E2E_CKB_BALANCE_POLL_INTERVAL_SECONDS:-5}"
E2E_CKB_BALANCE_CHECK_LIMIT_PAGES="${E2E_CKB_BALANCE_CHECK_LIMIT_PAGES:-20}"
E2E_USDI_FAUCET_COMMAND="${E2E_USDI_FAUCET_COMMAND:-}"
E2E_USDI_FAUCET_AMOUNT="${E2E_USDI_FAUCET_AMOUNT:-40}"
E2E_USDI_FAUCET_WAIT_SECONDS="${E2E_USDI_FAUCET_WAIT_SECONDS:-20}"
E2E_USDI_BALANCE_WAIT_TIMEOUT_SECONDS="${E2E_USDI_BALANCE_WAIT_TIMEOUT_SECONDS:-120}"
E2E_USDI_BALANCE_POLL_INTERVAL_SECONDS="${E2E_USDI_BALANCE_POLL_INTERVAL_SECONDS:-5}"
E2E_USDI_BALANCE_CHECK_LIMIT_PAGES="${E2E_USDI_BALANCE_CHECK_LIMIT_PAGES:-20}"
E2E_USDI_FAUCET_MAX_ATTEMPTS="${E2E_USDI_FAUCET_MAX_ATTEMPTS:-2}"
E2E_USDI_TOPUP_ADDRESS="${E2E_USDI_TOPUP_ADDRESS:-}"
E2E_USDI_PROVIDER_LIQUIDITY_BOOTSTRAP_AMOUNT="${E2E_USDI_PROVIDER_LIQUIDITY_BOOTSTRAP_AMOUNT:-25}"
E2E_CKB_TOPUP_ADDRESS="${E2E_CKB_TOPUP_ADDRESS:-}"
E2E_CKB_INVOICE_TOPUP_ADDRESS="${E2E_CKB_INVOICE_TOPUP_ADDRESS:-}"
E2E_CKB_PROVIDER_TOPUP_ADDRESS="${E2E_CKB_PROVIDER_TOPUP_ADDRESS:-}"
E2E_USDI_CHANNEL_FUNDING_AMOUNT="${E2E_USDI_CHANNEL_FUNDING_AMOUNT:-}"
E2E_CHANNEL_TLC_FEE_PROPORTIONAL_MILLIONTHS="${E2E_CHANNEL_TLC_FEE_PROPORTIONAL_MILLIONTHS:-0x0}"
OPEN_CHANNEL_INIT_RETRIES="${OPEN_CHANNEL_INIT_RETRIES:-10}"
OPEN_CHANNEL_INIT_RETRY_INTERVAL_SECONDS="${OPEN_CHANNEL_INIT_RETRY_INTERVAL_SECONDS:-2}"
ACCEPT_CHANNEL_RETRIES="${ACCEPT_CHANNEL_RETRIES:-20}"
ACCEPT_CHANNEL_RETRY_INTERVAL_SECONDS="${ACCEPT_CHANNEL_RETRY_INTERVAL_SECONDS:-2}"
CHANNEL_ACCEPT_RETRY_INTERVAL_ATTEMPTS="${E2E_CHANNEL_ACCEPT_RETRY_INTERVAL_ATTEMPTS:-6}"
CHANNEL_BOOTSTRAP_PROVISION_RETRIES="${E2E_CHANNEL_BOOTSTRAP_PROVISION_RETRIES:-3}"
CHANNEL_READY_ATTEMPTS_PER_ROUND="${E2E_CHANNEL_READY_ATTEMPTS_PER_ROUND:-60}"
CHANNEL_READY_STUCK_NEGOTIATING_ATTEMPTS="${E2E_CHANNEL_READY_STUCK_NEGOTIATING_ATTEMPTS:-20}"
FIBER_TESTNET_CONTRACTS_ISSUE_URL="${FIBER_TESTNET_CONTRACTS_ISSUE_URL:-https://github.com/nervosnetwork/fiber/issues/1226}"
DEFAULT_USDI_TYPE_SCRIPT_JSON='{"code_hash":"0xcc9dc33ef234e14bc788c43a4848556a5fb16401a04662fc55db9bb201987037","hash_type":"type","args":"0x71fd1985b2971a9903e4d8ed0d59e6710166985217ca0681437883837b86162f"}'
BUYER_DEPOSIT_ENABLE="${BUYER_DEPOSIT_ENABLE:-true}"
BUYER_DEPOSIT_WALLET_MASTER_SEED="${BUYER_DEPOSIT_WALLET_MASTER_SEED:-usdi-marketplace-e2e-buyer-deposit-wallet}"
BUYER_DEPOSIT_CKB_RPC_URL="${BUYER_DEPOSIT_CKB_RPC_URL:-${FNN2_CKB_RPC_URL}}"
BUYER_DEPOSIT_CKB_NETWORK="${BUYER_DEPOSIT_CKB_NETWORK:-testnet}"
BUYER_DEPOSIT_TREASURY_ADDRESS="${BUYER_DEPOSIT_TREASURY_ADDRESS:-}"
BUYER_DEPOSIT_TREASURY_RPC_URL="${BUYER_DEPOSIT_TREASURY_RPC_URL:-http://fnn2:8227}"
BUYER_DEPOSIT_UDT_TYPE_SCRIPT_JSON="${BUYER_DEPOSIT_UDT_TYPE_SCRIPT_JSON:-${DEFAULT_USDI_TYPE_SCRIPT_JSON}}"
BUYER_DEPOSIT_UDT_CELL_DEP_TX_HASH="${BUYER_DEPOSIT_UDT_CELL_DEP_TX_HASH:-0xaec423c2af7fe844b476333190096b10fc5726e6d9ac58a9b71f71ffac204fee}"
BUYER_DEPOSIT_UDT_CELL_DEP_INDEX="${BUYER_DEPOSIT_UDT_CELL_DEP_INDEX:-0}"
BUYER_DEPOSIT_MIN_USDI="${BUYER_DEPOSIT_MIN_USDI:-10}"
BUYER_DEPOSIT_CONFIRMATION_BLOCKS="${BUYER_DEPOSIT_CONFIRMATION_BLOCKS:-24}"

PAYER_LOCK_SCRIPT_JSON=""
INVOICE_LOCK_SCRIPT_JSON=""
PROVIDER_LOCK_SCRIPT_JSON=""
USDI_TYPE_SCRIPT_JSON=""
USDI_AUTO_ACCEPT_AMOUNT_HEX=""
PROVIDER_USDI_AUTO_ACCEPT_AMOUNT_HEX=""
ACCEPT_CHANNEL_FUNDING_AMOUNT_HEX=""
PROVIDER_ACCEPT_CHANNEL_FUNDING_AMOUNT_HEX=""
PAYER_NODE_ID=""
INVOICE_NODE_ID=""
PROVIDER_NODE_ID=""
PAYER_PEER_ID=""
INVOICE_PEER_ID=""
PROVIDER_PEER_ID=""
PAYER_NODE_CONTAINER=""
INVOICE_NODE_CONTAINER=""
PROVIDER_NODE_CONTAINER=""
USDI_FAUCET_TX_HASH=""
USDI_EXPLORER_PROOF_URLS=""
OPEN_CHANNEL_TEMPORARY_ID=""
NODE_INFO_RESULT=""
ACCEPT_CHANNEL_ATTEMPT_SEQ=0

"${ROOT_DIR}/scripts/prepare-carrier-dep.sh"

prepare_carrier_remote_fixture() {
  if [[ -z "${CARRIER_E2E_REMOTE_AUTHORIZED_KEY}" ]]; then
    ssh-keygen -q -t ed25519 -N '' -f "${CARRIER_E2E_REMOTE_PRIVATE_KEY_PATH}" >/dev/null
    CARRIER_E2E_REMOTE_AUTHORIZED_KEY="$(cat "${CARRIER_E2E_REMOTE_PRIVATE_KEY_PATH}.pub")"
  fi
  chmod 600 "${CARRIER_E2E_REMOTE_PRIVATE_KEY_PATH}" >/dev/null 2>&1 || true
}

prepare_carrier_remote_fixture

compose() {
  (
    cd "${ROOT_DIR}"
    COMPOSE_PROJECT_NAME="${COMPOSE_PROJECT_NAME}" \
    ONE_TOK_EXECUTION_GATEWAY_TOKEN="${ONE_TOK_EXECUTION_GATEWAY_TOKEN}" \
    ONE_TOK_SETTLEMENT_SERVICE_TOKEN="${ONE_TOK_SETTLEMENT_SERVICE_TOKEN}" \
    ONE_TOK_EXECUTION_EVENT_TOKEN="${ONE_TOK_EXECUTION_EVENT_TOKEN}" \
    CARRIER_GATEWAY_API_TOKEN="${CARRIER_GATEWAY_API_TOKEN}" \
    CARRIER_SERVER_API_TOKEN="${CARRIER_SERVER_API_TOKEN}" \
    OPENAI_API_KEY="${OPENAI_API_KEY}" \
    OPENAI_CODEX_TOKEN="${OPENAI_CODEX_TOKEN}" \
    OPENAI_BASE_URL="${OPENAI_BASE_URL}" \
    CARRIER_E2E_REMOTE_PRIVATE_KEY_PATH="${CARRIER_E2E_REMOTE_PRIVATE_KEY_PATH}" \
    CARRIER_E2E_REMOTE_AUTHORIZED_KEY="${CARRIER_E2E_REMOTE_AUTHORIZED_KEY}" \
    FIBER_APP_ID="${FIBER_APP_ID}" \
    FIBER_HMAC_SECRET="${FIBER_HMAC_SECRET}" \
    FIBER_USDI_UDT_NAME="${FIBER_USDI_UDT_NAME}" \
    FIBER_USDI_UDT_TYPE_SCRIPT_JSON="${FIBER_USDI_UDT_TYPE_SCRIPT_JSON}" \
    POSTGRES_PUBLISHED_PORT="${POSTGRES_PUBLISHED_PORT}" \
    REDIS_PUBLISHED_PORT="${REDIS_PUBLISHED_PORT}" \
    NATS_PUBLISHED_PORT="${NATS_PUBLISHED_PORT}" \
    NATS_MONITOR_PUBLISHED_PORT="${NATS_MONITOR_PUBLISHED_PORT}" \
    IAM_PUBLISHED_PORT="${IAM_PUBLISHED_PORT}" \
    API_GATEWAY_PUBLISHED_PORT="${API_GATEWAY_PUBLISHED_PORT}" \
    MARKETPLACE_PUBLISHED_PORT="${MARKETPLACE_PUBLISHED_PORT}" \
    SETTLEMENT_PUBLISHED_PORT="${SETTLEMENT_PUBLISHED_PORT}" \
    EXECUTION_PUBLISHED_PORT="${EXECUTION_PUBLISHED_PORT}" \
    WEB_PUBLISHED_PORT="${WEB_PUBLISHED_PORT}" \
    FIBER_ADAPTER_PUBLISHED_PORT="${FIBER_ADAPTER_PUBLISHED_PORT}" \
    FNN_PUBLISHED_RPC_PORT="${FNN_PUBLISHED_RPC_PORT}" \
    FNN_PUBLISHED_P2P_PORT="${FNN_PUBLISHED_P2P_PORT}" \
    FNN2_PUBLISHED_RPC_PORT="${FNN2_PUBLISHED_RPC_PORT}" \
    FNN2_PUBLISHED_P2P_PORT="${FNN2_PUBLISHED_P2P_PORT}" \
    PROVIDER_FNN_PUBLISHED_RPC_PORT="${PROVIDER_FNN_PUBLISHED_RPC_PORT}" \
    PROVIDER_FNN_PUBLISHED_P2P_PORT="${PROVIDER_FNN_PUBLISHED_P2P_PORT}" \
    FNN_VERSION="${FNN_VERSION}" \
    FNN_ASSET="${FNN_ASSET}" \
    FNN_ASSET_SHA256="${FNN_ASSET_SHA256}" \
    FIBER_SECRET_KEY_PASSWORD="${FIBER_SECRET_KEY_PASSWORD}" \
    FNN_CKB_RPC_URL="${FNN_CKB_RPC_URL}" \
    FNN2_CKB_RPC_URL="${FNN2_CKB_RPC_URL}" \
    PROVIDER_FNN_CKB_RPC_URL="${PROVIDER_FNN_CKB_RPC_URL}" \
    SETTLEMENT_RECONCILER_INTERVAL="${SETTLEMENT_RECONCILER_INTERVAL}" \
    BUYER_DEPOSIT_ENABLE="${BUYER_DEPOSIT_ENABLE}" \
    BUYER_DEPOSIT_WALLET_MASTER_SEED="${BUYER_DEPOSIT_WALLET_MASTER_SEED}" \
    BUYER_DEPOSIT_CKB_RPC_URL="${BUYER_DEPOSIT_CKB_RPC_URL}" \
    BUYER_DEPOSIT_CKB_NETWORK="${BUYER_DEPOSIT_CKB_NETWORK}" \
    BUYER_DEPOSIT_TREASURY_ADDRESS="${BUYER_DEPOSIT_TREASURY_ADDRESS}" \
    BUYER_DEPOSIT_TREASURY_RPC_URL="${BUYER_DEPOSIT_TREASURY_RPC_URL}" \
    BUYER_DEPOSIT_UDT_TYPE_SCRIPT_JSON="${BUYER_DEPOSIT_UDT_TYPE_SCRIPT_JSON}" \
    BUYER_DEPOSIT_UDT_CELL_DEP_TX_HASH="${BUYER_DEPOSIT_UDT_CELL_DEP_TX_HASH}" \
    BUYER_DEPOSIT_UDT_CELL_DEP_INDEX="${BUYER_DEPOSIT_UDT_CELL_DEP_INDEX}" \
    BUYER_DEPOSIT_MIN_USDI="${BUYER_DEPOSIT_MIN_USDI}" \
    BUYER_DEPOSIT_CONFIRMATION_BLOCKS="${BUYER_DEPOSIT_CONFIRMATION_BLOCKS}" \
    BUN_VERSION="${BUN_VERSION}" \
    E2E_USDI_HOST_OUTPUT_DIR="${ARTIFACT_DIR}" \
    RELEASE_USDI_E2E_GATEWAY_SERVICE_TOKEN="${ONE_TOK_EXECUTION_GATEWAY_TOKEN}" \
    RELEASE_USDI_E2E_FIBER_ADAPTER_BASE_URL="http://fiber-adapter:8091" \
    RELEASE_USDI_E2E_FIBER_ADAPTER_APP_ID="${FIBER_APP_ID}" \
    RELEASE_USDI_E2E_FIBER_ADAPTER_HMAC_SECRET="${FIBER_HMAC_SECRET}" \
    RELEASE_USDI_E2E_CARRIER_INTEGRATION_TOKEN="${CARRIER_GATEWAY_API_TOKEN}" \
    RELEASE_USDI_E2E_FAUCET_TX_HASH="${USDI_FAUCET_TX_HASH}" \
    RELEASE_USDI_E2E_EXPLORER_PROOF_URLS="${USDI_EXPLORER_PROOF_URLS}" \
    docker compose -f "${BASE_COMPOSE_FILE}" -f "${FNN_COMPOSE_FILE}" -f "${USDI_E2E_COMPOSE_FILE}" "$@"
  )
}

cleanup() {
  local code=$?
  trap - EXIT

  compose logs >"${LOG_DIR}/compose.log" 2>&1 || true
  cp "${LOG_DIR}/compose.log" "${ARTIFACT_DIR}/compose.log" >/dev/null 2>&1 || true
  write_known_failure_hints "${LOG_DIR}/compose.log"
  if [[ "${code}" -ne 0 || "${KEEP_STACK_UP}" != "1" ]]; then
    compose down -v --remove-orphans >/dev/null 2>&1 || true
  fi
  docker run --rm -v "${ARTIFACT_DIR}:/artifacts" alpine:3.20 chown -R "$(id -u):$(id -g)" /artifacts >/dev/null 2>&1 || true

  if [[ "${code}" -ne 0 ]]; then
    echo "usdi e2e failed; logs are in ${LOG_DIR}" >&2
    tail -n 400 "${LOG_DIR}/compose.log" >&2 || true
  elif [[ "${KEEP_STACK_UP}" == "1" ]]; then
    echo "usdi e2e stack kept running under compose project ${COMPOSE_PROJECT_NAME}"
  else
    rm -rf "${LOG_DIR}"
  fi
  rm -rf "${CARRIER_REMOTE_KEY_DIR}" >/dev/null 2>&1 || true

  echo "usdi e2e artifacts are in ${ARTIFACT_DIR}"
  exit "${code}"
}
trap cleanup EXIT

log() {
  printf '[usdi-e2e] %s\n' "$*"
}

write_known_failure_hints() {
  local log_file="$1"
  local hint_file="${ARTIFACT_DIR}/failure-hints.txt"
  if [[ ! -f "${log_file}" ]]; then
    return 0
  fi
  if grep -Fq "failed to init contracts context: Cannot resolve cell dep for type id Script" "${log_file}"; then
    cat >"${hint_file}" <<EOF
Detected upstream Fiber testnet startup failure while initializing contract deps.
Known issue: ${FIBER_TESTNET_CONTRACTS_ISSUE_URL}
Current impact: true-chain USDI e2e cannot reach faucet/top-up/order flow because fnn/fnn2 exits during startup.
EOF
    cat "${hint_file}" >&2
  elif grep -iqE "feature not found.*waiting for peer to send init message|waiting for peer to send init message" "${log_file}"; then
    cat >"${hint_file}" <<EOF
Detected FNN peer connectivity failure: peers failed to exchange Init messages.
Known issue: ${FIBER_TESTNET_CONTRACTS_ISSUE_URL}
Current impact: true-chain USDI e2e cannot open payment channels because FNN nodes cannot connect to each other.
EOF
    cat "${hint_file}" >&2
  fi
}

wait_for_http_reachable() {
  local url="$1"
  local label="$2"
  local attempts="${3:-180}"
  for ((i = 0; i < attempts; i++)); do
    if curl -sS --max-time 3 "${url}" >/dev/null 2>&1; then
      return 0
    fi
    sleep 2
  done
  echo "timed out waiting for ${label} at ${url}" >&2
  return 1
}

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
  local attempts="${3:-600}"
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

fnn_rpc_call_on_port() {
  local port="$1"
  local payload="$2"
  curl -sS --max-time 8 -H "content-type: application/json" -d "${payload}" "http://127.0.0.1:${port}"
}

ckb_rpc_call() {
  local payload="$1"
  curl -sS --max-time 15 -H "content-type: application/json" -d "${payload}" "${FNN_CKB_RPC_URL}"
}

contains_jsonrpc_error() {
  local payload="$1"
  printf '%s' "${payload}" | jq -e '.error != null' >/dev/null 2>&1
}

jsonrpc_error_message() {
  local payload="$1"
  printf '%s' "${payload}" | jq -r '.error.message // "unknown jsonrpc error"'
}

contains_open_channel_peer_not_ready_error() {
  local message="${1,,}"
  [[ "${message}" == *"peer not found"* ]] \
    || [[ "${message}" == *"peer not ready"* ]] \
    || [[ "${message}" == *"temporarily unavailable"* ]] \
    || [[ "${message}" == *"waiting for peer to send init message"* ]] \
    || ([[ "${message}" == *"feature not found"* ]] && [[ "${message}" == *"peer"* ]])
}

contains_accept_channel_ignorable_error() {
  local message="${1,,}"
  [[ "${message}" == *"already"* ]] \
    || [[ "${message}" == *"not found"* ]] \
    || [[ "${message}" == *"no channel with temp id"* ]] \
    || [[ "${message}" == *"no channel"* ]] \
    || [[ "${message}" == *"unknown channel"* ]]
}

bigint_gte() {
  node -e 'process.exit(BigInt(process.argv[1]) >= BigInt(process.argv[2]) ? 0 : 1)' "$1" "$2"
}

hex_quantity_to_decimal() {
  node -e 'const value = String(process.argv.length > 1 ? process.argv[1] : "").trim();
if (!/^0x[0-9a-fA-F]+$/.test(value)) process.exit(1);
console.log(BigInt(value).toString(10));' "$1"
}

to_hex_quantity() {
  node -e 'const raw = String(process.argv.length > 1 ? process.argv[1] : "").trim();
if (!/^[0-9]+$/.test(raw)) process.exit(1);
console.log("0x" + BigInt(raw).toString(16));' "$1"
}

derive_ckb_testnet_address_from_lock_args() {
  local lock_args_hex="$1"
  node -e 'const lockArgsHex = String(process.argv.length > 1 ? process.argv[1] : "").trim();
if (!lockArgsHex.startsWith("0x")) process.exit(1);
let args;
try {
  args = Buffer.from(lockArgsHex.slice(2), "hex");
} catch (_error) {
  process.exit(1);
}
if (args.length !== 20) process.exit(1);
const payload = Buffer.concat([Buffer.from([0x01, 0x00]), args]);
const charset = "qpzry9x8gf2tvdw0s3jn54khce6mua7l";
const generators = [0x3B6A57B2, 0x26508E6D, 0x1EA119FA, 0x3D4233DD, 0x2A1462B3];
const hrpExpand = (hrp) => {
  const expanded = [];
  for (const char of hrp) expanded.push(char.charCodeAt(0) >> 5);
  expanded.push(0);
  for (const char of hrp) expanded.push(char.charCodeAt(0) & 31);
  return expanded;
};
const polymod = (values) => {
  let chk = 1;
  for (const value of values) {
    const top = chk >>> 25;
    chk = ((chk & 0x1ffffff) << 5) ^ value;
    for (let i = 0; i < 5; i += 1) {
      if ((top >>> i) & 1) chk ^= generators[i];
    }
  }
  return chk >>> 0;
};
const createChecksum = (hrp, data) => {
  const values = hrpExpand(hrp).concat(data);
  const pm = polymod(values.concat([0, 0, 0, 0, 0, 0])) ^ 1;
  const checksum = [];
  for (let i = 0; i < 6; i += 1) checksum.push((pm >>> (5 * (5 - i))) & 31);
  return checksum;
};
const convertBits = (data, fromBits, toBits, pad) => {
  let acc = 0;
  let bits = 0;
  const ret = [];
  const maxv = (1 << toBits) - 1;
  for (const value of data) {
    if (value < 0 || (value >>> fromBits) !== 0) return null;
    acc = (acc << fromBits) | value;
    bits += fromBits;
    while (bits >= toBits) {
      bits -= toBits;
      ret.push((acc >>> bits) & maxv);
    }
  }
  if (pad) {
    if (bits > 0) ret.push((acc << (toBits - bits)) & maxv);
  } else if (bits >= fromBits || ((acc << (toBits - bits)) & maxv) !== 0) {
    return null;
  }
  return ret;
};
const data = convertBits([...payload], 8, 5, true);
if (!data) process.exit(1);
console.log("ckt1" + data.concat(createChecksum("ckt", data)).map((idx) => charset[idx]).join(""));' "${lock_args_hex}"
}

derive_peer_id_from_node_id() {
  local node_id_hex="$1"
  node -e 'const crypto = require("node:crypto");
let nodeIdHex = String(process.argv.length > 1 ? process.argv[1] : "").trim();
if (nodeIdHex.startsWith("0x")) nodeIdHex = nodeIdHex.slice(2);
let pubkey;
try {
  pubkey = Buffer.from(nodeIdHex, "hex");
} catch (_error) {
  process.exit(1);
}
if (pubkey.length !== 33) process.exit(1);
const digest = crypto.createHash("sha256").update(pubkey).digest();
const raw = Buffer.concat([Buffer.from([0x12, 0x20]), digest]);
const alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz";
let num = BigInt("0x" + raw.toString("hex"));
let out = "";
while (num > 0n) {
  const rem = Number(num % 58n);
  out = alphabet[rem] + out;
  num /= 58n;
}
let leadingZero = 0;
for (const b of raw) {
  if (b === 0) leadingZero += 1;
  else break;
}
console.log("1".repeat(leadingZero) + out);' "${node_id_hex}"
}

extract_usdi_type_script_from_node_info() {
  local node_info_payload="$1"
  if [[ -n "${FIBER_USDI_UDT_TYPE_SCRIPT_JSON}" ]]; then
    printf '%s' "${FIBER_USDI_UDT_TYPE_SCRIPT_JSON}" | jq -ce '
      select(
        (type == "object")
        and ((.code_hash // "") | type == "string")
        and ((.hash_type // "") | type == "string")
        and ((.args // "") | type == "string")
        and (.code_hash | length > 0)
        and (.hash_type | length > 0)
        and (.args | length > 0)
      )
    '
    return
  fi
  extract_usdi_type_script_from_node_info_without_override "${node_info_payload}"
}

extract_usdi_type_script_from_node_info_without_override() {
  local node_info_payload="$1"
  printf '%s' "${node_info_payload}" | jq -c '
    ([
      .result.udt_cfg_infos[]?
      | select((((.name // "") | ascii_downcase) == "usdi") or (((.name // "") | ascii_downcase) == "rusd"))
      | .script
    ] | .[0]) // (.result.udt_cfg_infos[0].script // empty)
  '
}

extract_usdi_auto_accept_amount_from_node_info() {
  local node_info_payload="$1"
  printf '%s' "${node_info_payload}" | jq -r '
    (
      [
        .result.udt_cfg_infos[]?
        | select((((.name // "") | ascii_downcase) == "usdi") or (((.name // "") | ascii_downcase) == "rusd"))
        | .auto_accept_amount
      ] | .[0]
    ) // (.result.udt_cfg_infos[0].auto_accept_amount // empty)
  '
}

fetch_node_info() {
  local rpc_port="$1"
  local target_file="$2"
  local payload='{"jsonrpc":"2.0","id":"node-info","method":"node_info","params":[]}'
  local response
  response="$(fnn_rpc_call_on_port "${rpc_port}" "${payload}")"
  printf '%s\n' "${response}" > "${target_file}"
  if contains_jsonrpc_error "${response}"; then
    echo "node_info failed on port ${rpc_port}: $(jsonrpc_error_message "${response}")" >&2
    exit 1
  fi
  NODE_INFO_RESULT="${response}"
}

fetch_lock_script_json() {
  local rpc_port="$1"
  local payload='{"jsonrpc":"2.0","id":"lock-script","method":"node_info","params":[]}'
  local response
  response="$(fnn_rpc_call_on_port "${rpc_port}" "${payload}")"
  printf '%s' "${response}" | jq -c '.result.default_funding_lock_script // empty'
}

sum_ckb_capacity_from_get_cells_response() {
  local response_payload="$1"
  node -e 'let resp;
try {
  resp = JSON.parse(process.argv[1]);
} catch (_error) {
  process.exit(1);
}
const result = resp && typeof resp === "object" ? resp.result : null;
const objs = result && Array.isArray(result.objects) ? result.objects : [];
let total = 0n;
for (const obj of objs) {
  const output = obj && typeof obj === "object" ? obj.output : null;
  const cap = output && typeof output === "object" && typeof output.capacity === "string" ? output.capacity : "";
  if (typeof cap !== "string" || cap.length === 0) continue;
  try {
    total += BigInt(cap);
  } catch (_error) {}
}
console.log(total.toString());' "${response_payload}"
}

sum_xudt_amount_from_get_cells_response() {
  local response_payload="$1"
  node -e 'let resp;
try {
  resp = JSON.parse(process.argv[1]);
} catch (_error) {
  process.exit(1);
}
const result = resp && typeof resp === "object" ? resp.result : null;
const objs = result && Array.isArray(result.objects) ? result.objects : [];
let total = 0n;
for (const obj of objs) {
  const data = obj && typeof obj === "object" && typeof obj.output_data === "string" ? obj.output_data : "";
  if (typeof data !== "string" || !data.startsWith("0x") || data.length < 34) continue;
  const leHex = data.slice(2, 34);
  const bytes = [];
  for (let i = 0; i < leHex.length; i += 2) bytes.push(leHex.slice(i, i + 2));
  const beHex = bytes.reverse().join("");
  try {
    total += BigInt("0x" + beHex);
  } catch (_error) {}
}
console.log(total.toString());' "${response_payload}"
}

query_ckb_balance_for_lock_script() {
  local lock_script_json="$1"
  local label="$2"
  local cursor="0x"
  local page=0
  local total=0
  while true; do
    page=$((page + 1))
    if [[ "${page}" -gt "${E2E_CKB_BALANCE_CHECK_LIMIT_PAGES}" ]]; then
      break
    fi
    local payload
    payload="$(jq -cn \
      --arg id "ckb-balance-${label}-${page}" \
      --argjson lock_script "${lock_script_json}" \
      --arg cursor "${cursor}" '
      if $cursor == "0x" then
        {jsonrpc:"2.0",id:$id,method:"get_cells",params:[{script:$lock_script,script_type:"lock"},"asc","0x64"]}
      else
        {jsonrpc:"2.0",id:$id,method:"get_cells",params:[{script:$lock_script,script_type:"lock"},"asc","0x64",$cursor]}
      end
    ')"
    local response
    response="$(ckb_rpc_call "${payload}")" || return 1
    if printf '%s' "${response}" | jq -e '.error != null' >/dev/null 2>&1; then
      return 1
    fi
    local page_sum count next_cursor
    page_sum="$(sum_ckb_capacity_from_get_cells_response "${response}")"
    total="$(node -e 'console.log((BigInt(process.argv[1]) + BigInt(process.argv[2])).toString())' "${total}" "${page_sum}")"
    count="$(printf '%s' "${response}" | jq -r '.result.objects | length')"
    next_cursor="$(printf '%s' "${response}" | jq -r '.result.last_cursor // "0x"')"
    if [[ "${count}" -eq 0 || -z "${next_cursor}" || "${next_cursor}" == "${cursor}" ]]; then
      break
    fi
    cursor="${next_cursor}"
  done
  printf '%s' "${total}"
}

query_usdi_balance_for_lock_script() {
  local lock_script_json="$1"
  local label="$2"
  local cursor="0x"
  local page=0
  local total=0
  while true; do
    page=$((page + 1))
    if [[ "${page}" -gt "${E2E_USDI_BALANCE_CHECK_LIMIT_PAGES}" ]]; then
      break
    fi
    local payload
    payload="$(jq -cn \
      --arg id "usdi-balance-${label}-${page}" \
      --argjson lock_script "${lock_script_json}" \
      --argjson type_script "${USDI_TYPE_SCRIPT_JSON}" \
      --arg cursor "${cursor}" '
      if $cursor == "0x" then
        {jsonrpc:"2.0",id:$id,method:"get_cells",params:[{script:$lock_script,script_type:"lock",filter:{script:$type_script}},"asc","0x64"]}
      else
        {jsonrpc:"2.0",id:$id,method:"get_cells",params:[{script:$lock_script,script_type:"lock",filter:{script:$type_script}},"asc","0x64",$cursor]}
      end
    ')"
    local response
    response="$(ckb_rpc_call "${payload}")" || return 1
    if printf '%s' "${response}" | jq -e '.error != null' >/dev/null 2>&1; then
      return 1
    fi
    local page_sum count next_cursor
    page_sum="$(sum_xudt_amount_from_get_cells_response "${response}")"
    total="$(node -e 'console.log((BigInt(process.argv[1]) + BigInt(process.argv[2])).toString())' "${total}" "${page_sum}")"
    count="$(printf '%s' "${response}" | jq -r '.result.objects | length')"
    next_cursor="$(printf '%s' "${response}" | jq -r '.result.last_cursor // "0x"')"
    if [[ "${count}" -eq 0 || -z "${next_cursor}" || "${next_cursor}" == "${cursor}" ]]; then
      break
    fi
    cursor="${next_cursor}"
  done
  printf '%s' "${total}"
}

query_usdi_balance_for_payer() {
  query_usdi_balance_for_lock_script "${PAYER_LOCK_SCRIPT_JSON}" "payer"
}

request_ckb_faucet() {
  local address="$1"
  local label="$2"
  local payload response_file http_code
  payload="$(jq -cn --arg address "${address}" --arg amount "${E2E_CKB_FAUCET_AMOUNT}" '{claim_event:{address_hash:$address,amount:$amount}}')"
  response_file="${ARTIFACT_DIR}/ckb-faucet-${label}.json"
  set +e
  http_code="$(curl -sS -o "${response_file}" -w "%{http_code}" -H "content-type: application/json" -d "${payload}" "${E2E_CKB_FAUCET_API_BASE%/}/claim_events")"
  local rc=$?
  set -e
  if [[ "${rc}" -ne 0 || "${http_code}" -lt 200 || "${http_code}" -ge 300 ]]; then
    if [[ "${E2E_CKB_FAUCET_ENABLE_FALLBACK}" == "1" ]]; then
      local fallback_payload fallback_file fallback_code
      fallback_payload="$(jq -cn --arg address "${address}" '{address:$address,token:"ckb"}')"
      fallback_file="${ARTIFACT_DIR}/ckb-faucet-fallback-${label}.json"
      set +e
      fallback_code="$(curl -sS -o "${fallback_file}" -w "%{http_code}" -H "content-type: application/json" -d "${fallback_payload}" "${E2E_CKB_FAUCET_FALLBACK_API_BASE%/}/faucet")"
      rc=$?
      set -e
      if [[ "${rc}" -eq 0 && "${fallback_code}" -ge 200 && "${fallback_code}" -lt 300 ]]; then
        log "ckb faucet fallback accepted for ${label}; waiting ${E2E_CKB_FAUCET_WAIT_SECONDS}s"
        sleep "${E2E_CKB_FAUCET_WAIT_SECONDS}"
        return 0
      fi
    fi
    return 1
  fi
  log "ckb faucet accepted for ${label}; waiting ${E2E_CKB_FAUCET_WAIT_SECONDS}s"
  sleep "${E2E_CKB_FAUCET_WAIT_SECONDS}"
}

ensure_ckb_balance_or_request_faucet() {
  local address="$1"
  local label="$2"
  local required_amount="$3"
  local lock_script_json="$4"
  local balance
  set +e
  balance="$(query_ckb_balance_for_lock_script "${lock_script_json}" "${label}")"
  local rc=$?
  set -e
  if [[ "${rc}" -eq 0 && -n "${balance}" ]] && bigint_gte "${balance}" "${required_amount}"; then
    log "ckb balance precheck passed for ${label} (balance=${balance}, required=${required_amount})"
    return 0
  fi
  log "ckb faucet required for ${label}"
  request_ckb_faucet "${address}" "${label}" || {
    echo "unable to top up ${label} with ckb faucet" >&2
    exit 1
  }
  local deadline
  deadline=$(( $(date +%s) + E2E_CKB_BALANCE_WAIT_TIMEOUT_SECONDS ))
  while [[ "$(date +%s)" -lt "${deadline}" ]]; do
    set +e
    balance="$(query_ckb_balance_for_lock_script "${lock_script_json}" "${label}")"
    rc=$?
    set -e
    if [[ "${rc}" -eq 0 && -n "${balance}" ]] && bigint_gte "${balance}" "${required_amount}"; then
      log "ckb balance reached required threshold for ${label} (balance=${balance}, required=${required_amount})"
      return 0
    fi
    sleep "${E2E_CKB_BALANCE_POLL_INTERVAL_SECONDS}"
  done
  echo "ckb balance still below required threshold for ${label}" >&2
  exit 1
}

extract_tx_hash_from_file() {
  local file_path="$1"
  [[ -f "${file_path}" ]] || return 1
  local tx_hash
  tx_hash="$(jq -r '
    .. | .txHash? // .tx_hash? // .transactionHash? // .transaction_hash? // .hash? // empty
  ' "${file_path}" 2>/dev/null | awk "NF {print; exit}")"
  if [[ -z "${tx_hash}" ]]; then
    tx_hash="$(grep -Eo '0x[0-9a-fA-F]{64}' "${file_path}" | head -n1 || true)"
  fi
  [[ -n "${tx_hash}" ]] || return 1
  printf '%s' "${tx_hash}"
}

build_explorer_url() {
  local tx_hash="$1"
  if [[ -z "${tx_hash}" ]]; then
    return 0
  fi
  printf '%s/transaction/%s' "${E2E_CKB_EXPLORER_BASE_URL%/}" "${tx_hash}"
}

canonicalize_json_object() {
  local value="${1:-}"
  [[ -n "${value}" ]] || return 1
  printf '%s' "${value}" | jq -cS '.'
}

fetch_matching_type_script_from_tx_for_lock() {
  local tx_hash="$1"
  local lock_script_json="$2"
  [[ -n "${tx_hash}" && -n "${lock_script_json}" ]] || return 1
  local payload response
  payload="$(jq -cn --arg id "tx-${tx_hash}" --arg tx_hash "${tx_hash}" '{jsonrpc:"2.0",id:$id,method:"get_transaction",params:[$tx_hash,"0x2"]}')"
  response="$(ckb_rpc_call "${payload}")" || return 1
  if printf '%s' "${response}" | jq -e '.error != null or .result == null' >/dev/null 2>&1; then
    return 1
  fi
  printf '%s' "${response}" | jq -c --argjson lock_script "${lock_script_json}" '
    ([
      .result.transaction.outputs[]?
      | select(.lock == $lock_script and .type != null)
      | .type
    ] | .[0]) // empty
  '
}

request_usdi_faucet_for_lock_script() {
  local address="$1"
  local label="$2"
  local required_amount="${3:-0}"
  local lock_script_json="$4"
  local stdout_file="${ARTIFACT_DIR}/usdi-faucet-${label}.stdout.log"
  local stderr_file="${ARTIFACT_DIR}/usdi-faucet-${label}.stderr.log"
  local balance_rc before_balance faucet_tx_hash=""
  if [[ -n "${lock_script_json}" && -n "${USDI_TYPE_SCRIPT_JSON}" && "${required_amount}" =~ ^[0-9]+$ ]]; then
    set +e
    before_balance="$(query_usdi_balance_for_lock_script "${lock_script_json}" "${label}")"
    balance_rc=$?
    set -e
    if [[ "${balance_rc}" -eq 0 && -n "${before_balance}" ]] && bigint_gte "${before_balance}" "${required_amount}"; then
      log "usdi balance precheck passed for ${label} (balance=${before_balance}, required=${required_amount})"
      return 0
    fi
  fi
  if [[ -z "${E2E_USDI_FAUCET_COMMAND}" ]]; then
    E2E_USDI_FAUCET_COMMAND='curl -fsS -X POST https://ckb-utilities.random-walk.co.jp/api/faucet -H "content-type: application/json" -d "{\"address\":\"${E2E_FAUCET_ADDRESS}\",\"token\":\"usdi\"}"'
    log "E2E_USDI_FAUCET_COMMAND is unset; using built-in default faucet command"
  fi
  printf '%s\n' "${E2E_USDI_FAUCET_COMMAND}" > "${ARTIFACT_DIR}/usdi-faucet-${label}.command.txt"
  local attempt=0
  while true; do
    attempt=$((attempt + 1))
    set +e
    E2E_FAUCET_ASSET="USDI" \
    E2E_FAUCET_ADDRESS="${address}" \
    E2E_FAUCET_AMOUNT="${E2E_USDI_FAUCET_AMOUNT}" \
      bash -lc "${E2E_USDI_FAUCET_COMMAND}" >"${stdout_file}" 2>"${stderr_file}"
    local rc=$?
    set -e
    if [[ "${rc}" -ne 0 ]]; then
      echo "usdi faucet command failed" >&2
      cat >"${ARTIFACT_DIR}/failure-hints.txt" <<HINT
Detected CKB/USDI testnet faucet failure (label=${label}, attempt=${attempt}).
Current impact: true-chain USDI e2e cannot fund wallets because the faucet is unreachable or rejected the request.
HINT
      exit 1
    fi
    faucet_tx_hash="$(extract_tx_hash_from_file "${stdout_file}" || true)"
    if [[ -z "${faucet_tx_hash}" ]]; then
      faucet_tx_hash="$(extract_tx_hash_from_file "${stderr_file}" || true)"
    fi
    if [[ "${label}" == "payer-bootstrap" ]]; then
      USDI_FAUCET_TX_HASH="${faucet_tx_hash}"
      if [[ -n "${USDI_FAUCET_TX_HASH}" ]]; then
        USDI_EXPLORER_PROOF_URLS="$(build_explorer_url "${USDI_FAUCET_TX_HASH}")"
      fi
    fi
    log "usdi faucet succeeded for ${label}; waiting ${E2E_USDI_FAUCET_WAIT_SECONDS}s"
    sleep "${E2E_USDI_FAUCET_WAIT_SECONDS}"

    if [[ -n "${faucet_tx_hash}" && -n "${lock_script_json}" && -n "${USDI_TYPE_SCRIPT_JSON}" ]]; then
      local actual_type_script expected_canonical actual_canonical
      actual_type_script="$(fetch_matching_type_script_from_tx_for_lock "${faucet_tx_hash}" "${lock_script_json}" || true)"
      if [[ -n "${actual_type_script}" ]]; then
        printf '%s\n' "${actual_type_script}" > "${ARTIFACT_DIR}/usdi-faucet-${label}.actual-type-script.json"
        printf '%s\n' "${USDI_TYPE_SCRIPT_JSON}" > "${ARTIFACT_DIR}/usdi-faucet-${label}.expected-type-script.json"
        expected_canonical="$(canonicalize_json_object "${USDI_TYPE_SCRIPT_JSON}" || true)"
        actual_canonical="$(canonicalize_json_object "${actual_type_script}" || true)"
        if [[ -n "${expected_canonical}" && -n "${actual_canonical}" && "${expected_canonical}" != "${actual_canonical}" ]]; then
          {
            printf 'faucet tx %s minted a different UDT than fnn node_info expects\n' "${faucet_tx_hash}"
            printf 'expected=%s\n' "${expected_canonical}"
            printf 'actual=%s\n' "${actual_canonical}"
            printf 'impact=current fnn open_channel rejects the faucet asset as invalid UDT type script\n'
          } > "${ARTIFACT_DIR}/usdi-faucet-${label}-mismatch.txt"
          echo "usdi faucet minted a UDT that does not match fnn node_info; current fnn open_channel rejects that asset as invalid UDT type script" >&2
          exit 1
        fi
      fi
    fi

    if [[ -n "${lock_script_json}" && -n "${USDI_TYPE_SCRIPT_JSON}" && "${required_amount}" =~ ^[0-9]+$ ]]; then
      local deadline after_balance
      deadline=$(( $(date +%s) + E2E_USDI_BALANCE_WAIT_TIMEOUT_SECONDS ))
      while [[ "$(date +%s)" -lt "${deadline}" ]]; do
        set +e
        after_balance="$(query_usdi_balance_for_lock_script "${lock_script_json}" "${label}")"
        balance_rc=$?
        set -e
        if [[ "${balance_rc}" -eq 0 && -n "${after_balance}" ]]; then
          log "usdi balance after faucet ${label} attempt=${attempt}: ${after_balance}"
          if bigint_gte "${after_balance}" "${required_amount}"; then
            return 0
          fi
        fi
        sleep "${E2E_USDI_BALANCE_POLL_INTERVAL_SECONDS}"
      done
      if [[ "${attempt}" -ge "${E2E_USDI_FAUCET_MAX_ATTEMPTS}" ]]; then
        echo "usdi balance still below required threshold for ${label} after faucet (required=${required_amount})" >&2
        exit 1
      fi
      log "usdi balance still below required threshold for ${label} after faucet attempt=${attempt}; retrying faucet"
      continue
    fi
    return 0
  done
}

request_usdi_faucet() {
  request_usdi_faucet_for_lock_script "${E2E_USDI_TOPUP_ADDRESS}" "payer-bootstrap" "${1:-0}" "${PAYER_LOCK_SCRIPT_JSON}"
}

request_provider_liquidity_bootstrap_usdi_faucet() {
  local amount="${1:-${E2E_USDI_PROVIDER_LIQUIDITY_BOOTSTRAP_AMOUNT}}"
  request_usdi_faucet_for_lock_script "${E2E_USDI_TOPUP_ADDRESS}" "payer-provider-liquidity" "${amount}" "${PAYER_LOCK_SCRIPT_JSON}"
}

get_container_ip() {
  local container="$1"
  docker inspect --format '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' "${container}"
}

connect_peer_on_port() {
  local from_port="$1"
  local remote_addr="$2"
  local label="$3"
  local payload response
  payload="$(jq -cn --arg id "connect-${label}-$(date +%s)-$RANDOM" --arg addr "${remote_addr}" '{jsonrpc:"2.0",id:$id,method:"connect_peer",params:[{address:$addr}]}')"
  response="$(fnn_rpc_call_on_port "${from_port}" "${payload}")"
  printf '%s\n' "${payload}" > "${ARTIFACT_DIR}/connect-peer-${label}.request.json"
  printf '%s\n' "${response}" > "${ARTIFACT_DIR}/connect-peer-${label}.response.json"
  if contains_jsonrpc_error "${response}"; then
    local message
    message="$(jsonrpc_error_message "${response}")"
    if [[ "${message,,}" == *"already connected"* || "${message,,}" == *"already exists"* ]]; then
      return 0
    fi
    echo "connect_peer failed (${label}): ${message}" >&2
    exit 1
  fi
}

open_channel_from_payer() {
  local peer_id="$1"
  local funding_amount_hex="$2"
  local funding_udt_type_script_json="${3:-}"
  local invoice_addr="${4:-}"
  local payer_addr="${5:-}"
  local attempt=0
  while true; do
    attempt=$((attempt + 1))
    local payload response
    if [[ -n "${funding_udt_type_script_json}" ]]; then
      payload="$(jq -cn \
        --arg id "open-channel-${attempt}-$(date +%s)-$RANDOM" \
        --arg peer_id "${peer_id}" \
        --arg funding_amount "${funding_amount_hex}" \
        --arg tlc_fee_proportional_millionths "${E2E_CHANNEL_TLC_FEE_PROPORTIONAL_MILLIONTHS}" \
        --argjson funding_udt_type_script "${funding_udt_type_script_json}" \
        '{jsonrpc:"2.0",id:$id,method:"open_channel",params:[{peer_id:$peer_id,funding_amount:$funding_amount,funding_udt_type_script:$funding_udt_type_script,tlc_fee_proportional_millionths:$tlc_fee_proportional_millionths}]}'
      )"
    else
      payload="$(jq -cn \
        --arg id "open-channel-${attempt}-$(date +%s)-$RANDOM" \
        --arg peer_id "${peer_id}" \
        --arg funding_amount "${funding_amount_hex}" \
        --arg tlc_fee_proportional_millionths "${E2E_CHANNEL_TLC_FEE_PROPORTIONAL_MILLIONTHS}" \
        '{jsonrpc:"2.0",id:$id,method:"open_channel",params:[{peer_id:$peer_id,funding_amount:$funding_amount,tlc_fee_proportional_millionths:$tlc_fee_proportional_millionths}]}'
      )"
    fi
    printf '%s\n' "${payload}" > "${ARTIFACT_DIR}/open-channel.request.json"
    printf '%s\n' "${payload}" > "${ARTIFACT_DIR}/open-channel.request.${attempt}.json"

    set +e
    response="$(fnn_rpc_call_on_port "${FNN2_PUBLISHED_RPC_PORT}" "${payload}")"
    local rc=$?
    set -e
    if [[ "${rc}" -ne 0 ]]; then
      if [[ "${attempt}" -lt "${OPEN_CHANNEL_INIT_RETRIES}" ]]; then
        sleep "${OPEN_CHANNEL_INIT_RETRY_INTERVAL_SECONDS}"
        continue
      fi
      echo "open_channel transport failed" >&2
      exit 1
    fi
    printf '%s\n' "${response}" > "${ARTIFACT_DIR}/open-channel.response.json"
    printf '%s\n' "${response}" > "${ARTIFACT_DIR}/open-channel.response.${attempt}.json"
    if contains_jsonrpc_error "${response}"; then
      local message
      message="$(jsonrpc_error_message "${response}")"
      if contains_open_channel_peer_not_ready_error "${message}" && [[ "${attempt}" -lt "${OPEN_CHANNEL_INIT_RETRIES}" ]]; then
        if [[ -n "${invoice_addr}" && -n "${payer_addr}" ]]; then
          connect_peer_on_port "${FNN2_PUBLISHED_RPC_PORT}" "${invoice_addr}" "payer-to-invoice"
          connect_peer_on_port "${FNN_PUBLISHED_RPC_PORT}" "${payer_addr}" "invoice-to-payer"
        fi
        sleep "${OPEN_CHANNEL_INIT_RETRY_INTERVAL_SECONDS}"
        continue
      fi
      echo "open_channel failed: ${message}" >&2
      exit 1
    fi
    OPEN_CHANNEL_TEMPORARY_ID="$(printf '%s' "${response}" | jq -r '.result.temporary_channel_id // empty')"
    return 0
  done
}

accept_channel_on_invoice_node() {
  local temporary_channel_id="$1"
  [[ -n "${temporary_channel_id}" ]] || return 0
  local funding_amount_hex="${2:-${ACCEPT_CHANNEL_FUNDING_AMOUNT_HEX}}"
  ACCEPT_CHANNEL_ATTEMPT_SEQ=$((ACCEPT_CHANNEL_ATTEMPT_SEQ + 1))
  local attempt="${ACCEPT_CHANNEL_ATTEMPT_SEQ}"
  local payload response
  payload="$(jq -cn \
    --arg id "accept-channel-${attempt}-$(date +%s)-$RANDOM" \
    --arg temporary_channel_id "${temporary_channel_id}" \
    --arg funding_amount "${funding_amount_hex}" \
    '{jsonrpc:"2.0",id:$id,method:"accept_channel",params:[{temporary_channel_id:$temporary_channel_id,funding_amount:$funding_amount}]}' \
  )"
  printf '%s\n' "${payload}" > "${ARTIFACT_DIR}/accept-channel.request.json"
  printf '%s\n' "${payload}" > "${ARTIFACT_DIR}/accept-channel.request.${attempt}.json"

  set +e
  response="$(fnn_rpc_call_on_port "${FNN_PUBLISHED_RPC_PORT}" "${payload}")"
  local rc=$?
  set -e
  if [[ "${rc}" -ne 0 ]]; then
    echo "accept_channel transport failed" >&2
    exit 1
  fi

  printf '%s\n' "${response}" > "${ARTIFACT_DIR}/accept-channel.response.json"
  printf '%s\n' "${response}" > "${ARTIFACT_DIR}/accept-channel.response.${attempt}.json"
  if ! contains_jsonrpc_error "${response}"; then
    return 0
  fi

  local message
  message="$(jsonrpc_error_message "${response}")"
  if contains_accept_channel_ignorable_error "${message}"; then
    return 0
  fi
  echo "accept_channel failed: ${message}" >&2
  exit 1
}

list_channels_by_peer() {
  local rpc_port="$1"
  local peer_id="$2"
  local payload
  payload="$(jq -cn --arg id "list-channels-$(date +%s)-$RANDOM" --arg peer_id "${peer_id}" '{jsonrpc:"2.0",id:$id,method:"list_channels",params:[{peer_id:$peer_id}]}')"
  fnn_rpc_call_on_port "${rpc_port}" "${payload}"
}

extract_first_channel_state() {
  local response="$1"
  printf '%s' "${response}" | jq -r '
    .result.channels[0].state as $s
    | if $s == null then ""
      elif ($s | type) == "string" then $s
      elif ($s | type) == "object" then ($s.state_name // "")
      else ""
      end
  '
}

normalize_channel_state() {
  local state="$1"
  case "${state}" in
    ChannelReady|CHANNEL_READY)
      printf 'CHANNEL_READY'
      ;;
    Closed|CLOSED)
      printf 'CLOSED'
      ;;
    *)
      printf '%s' "${state}"
      ;;
  esac
}

wait_until_channel_ready() {
  local attempts="${1:-180}"
  local temporary_channel_id="${2:-}"
  local accept_funding_amount_hex="${3:-}"
  local seen_channel=0
  local asymmetric_negotiating_attempts=0
  for ((i = 1; i <= attempts; i++)); do
    local payer_resp invoice_resp payer_state invoice_state payer_count invoice_count
    payer_resp="$(list_channels_by_peer "${FNN2_PUBLISHED_RPC_PORT}" "${INVOICE_PEER_ID}")"
    invoice_resp="$(list_channels_by_peer "${FNN_PUBLISHED_RPC_PORT}" "${PAYER_PEER_ID}")"
    printf '%s\n' "${payer_resp}" > "${ARTIFACT_DIR}/list-channels-payer.response.${i}.json"
    printf '%s\n' "${invoice_resp}" > "${ARTIFACT_DIR}/list-channels-invoice.response.${i}.json"
    if contains_jsonrpc_error "${payer_resp}" || contains_jsonrpc_error "${invoice_resp}"; then
      echo "list_channels failed while waiting for channel readiness" >&2
      exit 1
    fi
    payer_count="$(printf '%s' "${payer_resp}" | jq -r '.result.channels | length')"
    invoice_count="$(printf '%s' "${invoice_resp}" | jq -r '.result.channels | length')"
    payer_state="$(normalize_channel_state "$(extract_first_channel_state "${payer_resp}")")"
    invoice_state="$(normalize_channel_state "$(extract_first_channel_state "${invoice_resp}")")"
    if [[ "${payer_count}" -gt 0 || "${invoice_count}" -gt 0 ]]; then
      seen_channel=1
    fi
    if [[ "${seen_channel}" -eq 1 && "${payer_count}" -eq 0 && "${invoice_count}" -eq 0 ]]; then
      return 2
    fi
    if [[ "${payer_count}" -gt 0 && "${invoice_count}" -eq 0 && "${payer_state}" == "NEGOTIATING_FUNDING" ]]; then
      asymmetric_negotiating_attempts=$((asymmetric_negotiating_attempts + 1))
    elif [[ "${invoice_count}" -gt 0 && "${payer_count}" -eq 0 && "${invoice_state}" == "NEGOTIATING_FUNDING" ]]; then
      asymmetric_negotiating_attempts=$((asymmetric_negotiating_attempts + 1))
    else
      asymmetric_negotiating_attempts=0
    fi
    if [[ "${CHANNEL_READY_STUCK_NEGOTIATING_ATTEMPTS}" =~ ^[0-9]+$ \
      && "${CHANNEL_READY_STUCK_NEGOTIATING_ATTEMPTS}" -gt 0 \
      && "${asymmetric_negotiating_attempts}" -ge "${CHANNEL_READY_STUCK_NEGOTIATING_ATTEMPTS}" ]]; then
      return 2
    fi
    if [[ -n "${temporary_channel_id}" \
      && "${invoice_state}" == "AWAITING_CHANNEL_READY" \
      && "${CHANNEL_ACCEPT_RETRY_INTERVAL_ATTEMPTS}" =~ ^[0-9]+$ \
      && "${CHANNEL_ACCEPT_RETRY_INTERVAL_ATTEMPTS}" -gt 0 \
      && $((i % CHANNEL_ACCEPT_RETRY_INTERVAL_ATTEMPTS)) -eq 0 ]]; then
      accept_channel_on_invoice_node "${temporary_channel_id}" "${accept_funding_amount_hex}"
    fi
    if [[ "${payer_count}" -gt 0 && "${invoice_count}" -gt 0 && "${payer_state}" == "CHANNEL_READY" && "${invoice_state}" == "CHANNEL_READY" ]]; then
      return 0
    fi
    if [[ "${payer_state}" == "CLOSED" || "${invoice_state}" == "CLOSED" ]]; then
      return 2
    fi
    sleep 2
  done
  return 1
}

has_ready_usdi_channel_on_port() {
  local rpc_port="$1"
  local peer_id="$2"
  local response
  response="$(list_channels_by_peer "${rpc_port}" "${peer_id}")" || return 1
  if contains_jsonrpc_error "${response}"; then
    return 1
  fi
  printf '%s' "${response}" | jq -e --argjson script "${USDI_TYPE_SCRIPT_JSON}" '
    .result.channels // []
    | any(
        ((.state.state_name // .state) | tostring) as $state
        | ($state == "CHANNEL_READY" or $state == "ChannelReady")
        and (.funding_udt_type_script != null)
        and ((.funding_udt_type_script.code_hash // "") == ($script.code_hash // ""))
        and ((.funding_udt_type_script.hash_type // "") == ($script.hash_type // ""))
        and ((.funding_udt_type_script.args // "") == ($script.args // ""))
      )
  ' >/dev/null 2>&1
}

wait_until_usdi_channel_ready() {
  local attempts="${1:-180}"
  for ((i = 0; i < attempts; i++)); do
    if has_ready_usdi_channel_on_port "${FNN2_PUBLISHED_RPC_PORT}" "${INVOICE_PEER_ID}" \
      && has_ready_usdi_channel_on_port "${FNN_PUBLISHED_RPC_PORT}" "${PAYER_PEER_ID}"; then
      return 0
    fi
    sleep 2
  done
  return 1
}

resolve_usdi_channel_funding_amount() {
  local amount="${E2E_USDI_CHANNEL_FUNDING_AMOUNT}"
  if [[ -z "${amount}" ]]; then
    amount="50"
  fi
  printf '%s' "${amount}"
}

bootstrap_usdi_channel() {
  [[ -n "${USDI_TYPE_SCRIPT_JSON}" && "${USDI_TYPE_SCRIPT_JSON}" != "null" ]] || {
    echo "node_info does not expose a USDI UDT type script" >&2
    exit 1
  }
  if has_ready_usdi_channel_on_port "${FNN2_PUBLISHED_RPC_PORT}" "${INVOICE_PEER_ID}" \
    && has_ready_usdi_channel_on_port "${FNN_PUBLISHED_RPC_PORT}" "${PAYER_PEER_ID}"; then
    log "usdi channel already ready"
    return 0
  fi
  local invoice_ip payer_ip
  invoice_ip="$(get_container_ip "${INVOICE_NODE_CONTAINER}")"
  payer_ip="$(get_container_ip "${PAYER_NODE_CONTAINER}")"
  [[ -n "${invoice_ip}" && -n "${payer_ip}" ]] || {
    echo "unable to resolve fnn container IPs" >&2
    exit 1
  }
  local invoice_addr payer_addr funding_amount funding_amount_hex accept_funding_hex
  invoice_addr="/ip4/${invoice_ip}/tcp/8228/p2p/${INVOICE_PEER_ID}"
  payer_addr="/ip4/${payer_ip}/tcp/8228/p2p/${PAYER_PEER_ID}"
  printf '%s\n' "${invoice_addr}" > "${ARTIFACT_DIR}/invoice-node.addr"
  printf '%s\n' "${payer_addr}" > "${ARTIFACT_DIR}/payer-node.addr"
  connect_peer_on_port "${FNN2_PUBLISHED_RPC_PORT}" "${invoice_addr}" "payer-to-invoice"
  connect_peer_on_port "${FNN_PUBLISHED_RPC_PORT}" "${payer_addr}" "invoice-to-payer"
  funding_amount="$(resolve_usdi_channel_funding_amount)"
  funding_amount_hex="$(to_hex_quantity "${funding_amount}")"
  accept_funding_hex="${USDI_AUTO_ACCEPT_AMOUNT_HEX:-${funding_amount_hex}}"
  local provision_attempt wait_rc
  for ((provision_attempt = 1; provision_attempt <= CHANNEL_BOOTSTRAP_PROVISION_RETRIES; provision_attempt++)); do
    OPEN_CHANNEL_TEMPORARY_ID=""
    connect_peer_on_port "${FNN2_PUBLISHED_RPC_PORT}" "${invoice_addr}" "payer-to-invoice"
    connect_peer_on_port "${FNN_PUBLISHED_RPC_PORT}" "${payer_addr}" "invoice-to-payer"
    open_channel_from_payer "${INVOICE_PEER_ID}" "${funding_amount_hex}" "${USDI_TYPE_SCRIPT_JSON}" "${invoice_addr}" "${payer_addr}"
    accept_channel_on_invoice_node "${OPEN_CHANNEL_TEMPORARY_ID}" "${accept_funding_hex}"
    wait_rc=0
    set +e
    wait_until_channel_ready "${CHANNEL_READY_ATTEMPTS_PER_ROUND}" "${OPEN_CHANNEL_TEMPORARY_ID}" "${accept_funding_hex}"
    wait_rc=$?
    set -e
    if [[ "${wait_rc}" -ne 0 ]]; then
      accept_channel_on_invoice_node "${OPEN_CHANNEL_TEMPORARY_ID}" "${accept_funding_hex}"
      set +e
      wait_until_channel_ready "${CHANNEL_READY_ATTEMPTS_PER_ROUND}" "${OPEN_CHANNEL_TEMPORARY_ID}" "${accept_funding_hex}"
      wait_rc=$?
      set -e
    fi
    if [[ "${wait_rc}" -eq 0 ]]; then
      break
    fi
    if [[ "${provision_attempt}" -lt "${CHANNEL_BOOTSTRAP_PROVISION_RETRIES}" ]]; then
      log "ckb channel still not ready after provision attempt=${provision_attempt}; reconnecting and reopening"
      sleep "${OPEN_CHANNEL_INIT_RETRY_INTERVAL_SECONDS}"
      continue
    fi
    echo "timed out waiting for ckb channel readiness" >&2
    exit 1
  done
  wait_until_usdi_channel_ready 180 || {
    echo "timed out waiting for usdi channel readiness" >&2
    exit 1
  }
  log "usdi channel is ready (fnn2 <-> fnn)"
}

cd "${ROOT_DIR}"
./scripts/compose-fnn-reference.test.sh

for binary in docker curl jq node bash; do
  command -v "${binary}" >/dev/null 2>&1 || {
    echo "missing required binary: ${binary}" >&2
    exit 1
  }
done

compose up -d --build \
  fnn \
  fnn2 \
  provider-fnn \
  fiber-adapter \
  carrier-daemon \
  carrier-gateway \
  iam \
  api-gateway \
  marketplace \
  settlement \
  settlement-reconciler \
  execution \
  web

PAYER_NODE_CONTAINER="$(wait_for_container fnn2)"
INVOICE_NODE_CONTAINER="$(wait_for_container fnn)"
PROVIDER_NODE_CONTAINER="$(wait_for_container provider-fnn)"

wait_for_http_reachable "http://127.0.0.1:${FNN_PUBLISHED_RPC_PORT}" "fnn rpc"
wait_for_http_reachable "http://127.0.0.1:${FNN2_PUBLISHED_RPC_PORT}" "fnn2 rpc"
wait_for_http_reachable "http://127.0.0.1:${PROVIDER_FNN_PUBLISHED_RPC_PORT}" "provider-fnn rpc"
wait_for_http_reachable "http://127.0.0.1:${FIBER_ADAPTER_PUBLISHED_PORT}/healthz" "fiber-adapter"
wait_for_http_reachable "http://127.0.0.1:${API_GATEWAY_PUBLISHED_PORT}/healthz" "api-gateway"
wait_for_http_reachable "http://127.0.0.1:${SETTLEMENT_PUBLISHED_PORT}/healthz" "settlement"
wait_for_http_reachable "http://127.0.0.1:${EXECUTION_PUBLISHED_PORT}/healthz" "execution"
wait_for_http_reachable "http://127.0.0.1:${WEB_PUBLISHED_PORT}" "web"

fetch_node_info "${FNN2_PUBLISHED_RPC_PORT}" "${ARTIFACT_DIR}/node-info-payer.response.json"
payer_info="${NODE_INFO_RESULT}"
fetch_node_info "${FNN_PUBLISHED_RPC_PORT}" "${ARTIFACT_DIR}/node-info-invoice.response.json"
invoice_info="${NODE_INFO_RESULT}"
fetch_node_info "${PROVIDER_FNN_PUBLISHED_RPC_PORT}" "${ARTIFACT_DIR}/node-info-provider.response.json"
provider_info="${NODE_INFO_RESULT}"

PAYER_LOCK_SCRIPT_JSON="$(printf '%s' "${payer_info}" | jq -c '.result.default_funding_lock_script // empty')"
INVOICE_LOCK_SCRIPT_JSON="$(printf '%s' "${invoice_info}" | jq -c '.result.default_funding_lock_script // empty')"
PROVIDER_LOCK_SCRIPT_JSON="$(printf '%s' "${provider_info}" | jq -c '.result.default_funding_lock_script // empty')"
NODE_INFO_USDI_TYPE_SCRIPT_JSON="$(extract_usdi_type_script_from_node_info_without_override "${payer_info}")"
USDI_TYPE_SCRIPT_JSON="$(extract_usdi_type_script_from_node_info "${payer_info}")"
USDI_AUTO_ACCEPT_AMOUNT_HEX="$(extract_usdi_auto_accept_amount_from_node_info "${payer_info}")"
PROVIDER_USDI_AUTO_ACCEPT_AMOUNT_HEX="$(extract_usdi_auto_accept_amount_from_node_info "${provider_info}")"
if [[ -z "${FIBER_USDI_UDT_TYPE_SCRIPT_JSON}" && -n "${USDI_TYPE_SCRIPT_JSON}" ]]; then
  FIBER_USDI_UDT_TYPE_SCRIPT_JSON="${USDI_TYPE_SCRIPT_JSON}"
fi
if [[ -n "${FIBER_USDI_UDT_TYPE_SCRIPT_JSON}" ]]; then
  expected_canonical="$(canonicalize_json_object "${NODE_INFO_USDI_TYPE_SCRIPT_JSON}" || true)"
  override_canonical="$(canonicalize_json_object "${USDI_TYPE_SCRIPT_JSON}" || true)"
  if [[ -n "${expected_canonical}" && -n "${override_canonical}" && "${expected_canonical}" != "${override_canonical}" ]]; then
    printf '%s\n' "${NODE_INFO_USDI_TYPE_SCRIPT_JSON}" > "${ARTIFACT_DIR}/usdi-node-info-type-script.json"
    printf '%s\n' "${USDI_TYPE_SCRIPT_JSON}" > "${ARTIFACT_DIR}/usdi-override-type-script.json"
    echo "FIBER_USDI_UDT_TYPE_SCRIPT_JSON differs from fnn node_info, but current fnn open_channel rejects non-whitelisted UDT type scripts; update fnn whitelist/testnet config instead of overriding in e2e" >&2
    exit 1
  fi
  log "using USDI udt_type_script override from FIBER_USDI_UDT_TYPE_SCRIPT_JSON"
fi
PAYER_NODE_ID="$(printf '%s' "${payer_info}" | jq -r '.result.node_id // empty')"
INVOICE_NODE_ID="$(printf '%s' "${invoice_info}" | jq -r '.result.node_id // empty')"
PROVIDER_NODE_ID="$(printf '%s' "${provider_info}" | jq -r '.result.node_id // empty')"
ACCEPT_CHANNEL_FUNDING_AMOUNT_HEX="$(printf '%s' "${invoice_info}" | jq -r '.result.auto_accept_channel_ckb_funding_amount // empty')"
[[ -n "${ACCEPT_CHANNEL_FUNDING_AMOUNT_HEX}" ]] || ACCEPT_CHANNEL_FUNDING_AMOUNT_HEX="$(to_hex_quantity 1)"
PROVIDER_ACCEPT_CHANNEL_FUNDING_AMOUNT_HEX="$(printf '%s' "${provider_info}" | jq -r '.result.auto_accept_channel_ckb_funding_amount // empty')"
[[ -n "${PROVIDER_ACCEPT_CHANNEL_FUNDING_AMOUNT_HEX}" ]] || PROVIDER_ACCEPT_CHANNEL_FUNDING_AMOUNT_HEX="$(to_hex_quantity 1)"

if [[ -z "${E2E_CKB_TOPUP_ADDRESS}" ]]; then
  payer_lock_args="$(printf '%s' "${PAYER_LOCK_SCRIPT_JSON}" | jq -r '.args // empty')"
  E2E_CKB_TOPUP_ADDRESS="$(derive_ckb_testnet_address_from_lock_args "${payer_lock_args}")"
fi
if [[ -z "${E2E_CKB_INVOICE_TOPUP_ADDRESS}" ]]; then
  invoice_lock_args="$(printf '%s' "${INVOICE_LOCK_SCRIPT_JSON}" | jq -r '.args // empty')"
  E2E_CKB_INVOICE_TOPUP_ADDRESS="$(derive_ckb_testnet_address_from_lock_args "${invoice_lock_args}")"
fi
if [[ -z "${E2E_CKB_PROVIDER_TOPUP_ADDRESS}" ]]; then
  provider_lock_args="$(printf '%s' "${PROVIDER_LOCK_SCRIPT_JSON}" | jq -r '.args // empty')"
  E2E_CKB_PROVIDER_TOPUP_ADDRESS="$(derive_ckb_testnet_address_from_lock_args "${provider_lock_args}")"
fi
if [[ -z "${E2E_USDI_TOPUP_ADDRESS}" ]]; then
  E2E_USDI_TOPUP_ADDRESS="${E2E_CKB_TOPUP_ADDRESS}"
fi

PAYER_PEER_ID="$(derive_peer_id_from_node_id "${PAYER_NODE_ID}")"
INVOICE_PEER_ID="$(derive_peer_id_from_node_id "${INVOICE_NODE_ID}")"
PROVIDER_PEER_ID="$(derive_peer_id_from_node_id "${PROVIDER_NODE_ID}")"

log "payer node=fnn2 peer_id=${PAYER_PEER_ID} ckb_address=${E2E_CKB_TOPUP_ADDRESS} usdi_address=${E2E_USDI_TOPUP_ADDRESS}"
log "invoice node=fnn peer_id=${INVOICE_PEER_ID} ckb_address=${E2E_CKB_INVOICE_TOPUP_ADDRESS}"
log "provider settlement node=provider-fnn peer_id=${PROVIDER_PEER_ID} ckb_address=${E2E_CKB_PROVIDER_TOPUP_ADDRESS}"

ensure_ckb_balance_or_request_faucet "${E2E_CKB_TOPUP_ADDRESS}" "payer-bootstrap" "10000000000" "${PAYER_LOCK_SCRIPT_JSON}"
invoice_required_amount="1"
if [[ -n "${ACCEPT_CHANNEL_FUNDING_AMOUNT_HEX}" && "${ACCEPT_CHANNEL_FUNDING_AMOUNT_HEX}" =~ ^0x[0-9a-fA-F]+$ ]]; then
  invoice_required_amount="$(hex_quantity_to_decimal "${ACCEPT_CHANNEL_FUNDING_AMOUNT_HEX}")"
fi
ensure_ckb_balance_or_request_faucet "${E2E_CKB_INVOICE_TOPUP_ADDRESS}" "invoice-bootstrap" "${invoice_required_amount}" "${INVOICE_LOCK_SCRIPT_JSON}"
provider_required_amount="1"
if [[ -n "${PROVIDER_ACCEPT_CHANNEL_FUNDING_AMOUNT_HEX}" && "${PROVIDER_ACCEPT_CHANNEL_FUNDING_AMOUNT_HEX}" =~ ^0x[0-9a-fA-F]+$ ]]; then
  provider_required_amount="$(hex_quantity_to_decimal "${PROVIDER_ACCEPT_CHANNEL_FUNDING_AMOUNT_HEX}")"
fi
ensure_ckb_balance_or_request_faucet "${E2E_CKB_PROVIDER_TOPUP_ADDRESS}" "provider-bootstrap" "${provider_required_amount}" "${PROVIDER_LOCK_SCRIPT_JSON}"
required_usdi_amount="$(resolve_usdi_channel_funding_amount)"
request_usdi_faucet "${required_usdi_amount}"
invoice_required_usdi_amount="0"
if [[ -n "${USDI_AUTO_ACCEPT_AMOUNT_HEX}" && "${USDI_AUTO_ACCEPT_AMOUNT_HEX}" =~ ^0x[0-9a-fA-F]+$ ]]; then
  invoice_required_usdi_amount="$(hex_quantity_to_decimal "${USDI_AUTO_ACCEPT_AMOUNT_HEX}")"
fi
if [[ "${invoice_required_usdi_amount}" =~ ^[0-9]+$ ]] && [[ "${invoice_required_usdi_amount}" -gt 0 ]]; then
  request_usdi_faucet_for_lock_script "${E2E_CKB_INVOICE_TOPUP_ADDRESS}" "invoice-bootstrap" "${invoice_required_usdi_amount}" "${INVOICE_LOCK_SCRIPT_JSON}"
fi
provider_required_usdi_amount="0"
if [[ -n "${PROVIDER_USDI_AUTO_ACCEPT_AMOUNT_HEX}" && "${PROVIDER_USDI_AUTO_ACCEPT_AMOUNT_HEX}" =~ ^0x[0-9a-fA-F]+$ ]]; then
  provider_required_usdi_amount="$(hex_quantity_to_decimal "${PROVIDER_USDI_AUTO_ACCEPT_AMOUNT_HEX}")"
fi
if [[ "${provider_required_usdi_amount}" =~ ^[0-9]+$ ]] && [[ "${provider_required_usdi_amount}" -gt 0 ]]; then
  request_usdi_faucet_for_lock_script "${E2E_CKB_PROVIDER_TOPUP_ADDRESS}" "provider-bootstrap" "${provider_required_usdi_amount}" "${PROVIDER_LOCK_SCRIPT_JSON}"
fi
bootstrap_usdi_channel
request_provider_liquidity_bootstrap_usdi_faucet "${E2E_USDI_PROVIDER_LIQUIDITY_BOOTSTRAP_AMOUNT}"

compose up -d --build usdi-e2e-runner
runner_id="$(wait_for_container usdi-e2e-runner)"
exit_code="$(wait_for_exit "${runner_id}" "usdi-e2e-runner")"
if [[ "${exit_code}" != "0" ]]; then
  echo "usdi-e2e-runner failed with exit code ${exit_code}" >&2
  exit "${exit_code}"
fi
