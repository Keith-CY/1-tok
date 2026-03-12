#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BASE_COMPOSE_FILE="${ROOT_DIR}/compose.yaml"
FNN_COMPOSE_FILE="${ROOT_DIR}/compose.fnn.yaml"
E2E_COMPOSE_FILE="${ROOT_DIR}/compose.e2e.yaml"
LOG_DIR="$(mktemp -d /tmp/1tok-compose-fnn-dual.XXXXXX)"

COMPOSE_PROJECT_NAME="${COMPOSE_PROJECT_NAME:-1tok-fnn-dual}"
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
MARKETPLACE_PUBLISHED_PORT="${MARKETPLACE_PUBLISHED_PORT:-28082}"
SETTLEMENT_PUBLISHED_PORT="${SETTLEMENT_PUBLISHED_PORT:-28083}"
EXECUTION_PUBLISHED_PORT="${EXECUTION_PUBLISHED_PORT:-28085}"
WEB_PUBLISHED_PORT="${WEB_PUBLISHED_PORT:-23000}"
FNN_PUBLISHED_RPC_PORT="${FNN_PUBLISHED_RPC_PORT:-28227}"
FNN_PUBLISHED_P2P_PORT="${FNN_PUBLISHED_P2P_PORT:-28228}"
FNN2_PUBLISHED_RPC_PORT="${FNN2_PUBLISHED_RPC_PORT:-29227}"
FNN2_PUBLISHED_P2P_PORT="${FNN2_PUBLISHED_P2P_PORT:-29228}"
FNN_VERSION="${FNN_VERSION:-v0.6.1}"
FNN_ASSET="${FNN_ASSET:-fnn_v0.6.1-x86_64-linux-portable.tar.gz}"
FNN_ASSET_SHA256="${FNN_ASSET_SHA256:-}"
FIBER_SECRET_KEY_PASSWORD="${FIBER_SECRET_KEY_PASSWORD:-}"
FNN_CKB_RPC_URL="${FNN_CKB_RPC_URL:-https://testnet.ckbapp.dev/}"
FNN2_CKB_RPC_URL="${FNN2_CKB_RPC_URL:-${FNN_CKB_RPC_URL}}"
FNN_DUAL_CKB_TOPUP_ADDRESS="${FNN_DUAL_CKB_TOPUP_ADDRESS:-}"
FNN_DUAL_INVOICE_CKB_TOPUP_ADDRESS="${FNN_DUAL_INVOICE_CKB_TOPUP_ADDRESS:-}"
FNN_DUAL_TOPUP_INVOICE_NODE_CKB="${FNN_DUAL_TOPUP_INVOICE_NODE_CKB:-1}"
FNN_DUAL_CHANNEL_FUNDING_AMOUNT="${FNN_DUAL_CHANNEL_FUNDING_AMOUNT:-10000000000}"
FNN_DUAL_NODE_INFO_DERIVE_RETRIES="${FNN_DUAL_NODE_INFO_DERIVE_RETRIES:-30}"
FNN_DUAL_NODE_INFO_DERIVE_RETRY_INTERVAL_SECONDS="${FNN_DUAL_NODE_INFO_DERIVE_RETRY_INTERVAL_SECONDS:-2}"
FNN_DUAL_CKB_FAUCET_API_BASE="${FNN_DUAL_CKB_FAUCET_API_BASE:-https://faucet-api.nervos.org}"
FNN_DUAL_CKB_FAUCET_FALLBACK_API_BASE="${FNN_DUAL_CKB_FAUCET_FALLBACK_API_BASE:-https://ckb-utilities.random-walk.co.jp/api}"
FNN_DUAL_CKB_FAUCET_ENABLE_FALLBACK="${FNN_DUAL_CKB_FAUCET_ENABLE_FALLBACK:-1}"
FNN_DUAL_CKB_FAUCET_AMOUNT="${FNN_DUAL_CKB_FAUCET_AMOUNT:-100000}"
FNN_DUAL_CKB_FAUCET_WAIT_SECONDS="${FNN_DUAL_CKB_FAUCET_WAIT_SECONDS:-20}"
FNN_DUAL_CKB_BALANCE_WAIT_TIMEOUT_SECONDS="${FNN_DUAL_CKB_BALANCE_WAIT_TIMEOUT_SECONDS:-180}"
FNN_DUAL_CKB_BALANCE_POLL_INTERVAL_SECONDS="${FNN_DUAL_CKB_BALANCE_POLL_INTERVAL_SECONDS:-5}"
FNN_DUAL_CKB_BALANCE_CHECK_LIMIT_PAGES="${FNN_DUAL_CKB_BALANCE_CHECK_LIMIT_PAGES:-20}"

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
    MARKETPLACE_PUBLISHED_PORT="${MARKETPLACE_PUBLISHED_PORT}" \
    SETTLEMENT_PUBLISHED_PORT="${SETTLEMENT_PUBLISHED_PORT}" \
    EXECUTION_PUBLISHED_PORT="${EXECUTION_PUBLISHED_PORT}" \
    WEB_PUBLISHED_PORT="${WEB_PUBLISHED_PORT}" \
    FNN_PUBLISHED_RPC_PORT="${FNN_PUBLISHED_RPC_PORT}" \
    FNN_PUBLISHED_P2P_PORT="${FNN_PUBLISHED_P2P_PORT}" \
    FNN2_PUBLISHED_RPC_PORT="${FNN2_PUBLISHED_RPC_PORT}" \
    FNN2_PUBLISHED_P2P_PORT="${FNN2_PUBLISHED_P2P_PORT}" \
    FNN_VERSION="${FNN_VERSION}" \
    FNN_ASSET="${FNN_ASSET}" \
    FNN_ASSET_SHA256="${FNN_ASSET_SHA256}" \
    FIBER_SECRET_KEY_PASSWORD="${FIBER_SECRET_KEY_PASSWORD}" \
    FNN_CKB_RPC_URL="${FNN_CKB_RPC_URL}" \
    FNN2_CKB_RPC_URL="${FNN2_CKB_RPC_URL}" \
    docker compose -f "${BASE_COMPOSE_FILE}" -f "${FNN_COMPOSE_FILE}" -f "${E2E_COMPOSE_FILE}" "$@"
  )
}

cleanup() {
  local code=$?
  trap - EXIT

  compose logs >"${LOG_DIR}/compose.log" 2>&1 || true
  compose down -v --remove-orphans >/dev/null 2>&1 || true

  if [[ $code -ne 0 ]]; then
    echo "compose fnn dual-node smoke failed; logs are in ${LOG_DIR}" >&2
    if [[ -f "${LOG_DIR}/compose.log" ]]; then
      tail -n 400 "${LOG_DIR}/compose.log" >&2 || true
    fi
  else
    rm -rf "${LOG_DIR}"
  fi

  exit "$code"
}
trap cleanup EXIT

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

log() {
  printf '[fnn-dual] %s\n' "$*"
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

bigint_gte() {
  node -e 'process.exit(BigInt(process.argv[1]) >= BigInt(process.argv[2]) ? 0 : 1)' "$1" "$2"
}

hex_quantity_to_decimal() {
  node -e 'const value = String(process.argv[1] ?? "").trim();
if (!/^0x[0-9a-fA-F]+$/.test(value)) process.exit(1);
console.log(BigInt(value).toString(10));' "$1"
}

derive_ckb_testnet_address_from_lock_args() {
  local lock_args_hex="$1"
  node -e 'const lockArgsHex = String(process.argv[1] ?? "").trim();
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
const checksum = createChecksum("ckt", data);
console.log("ckt1" + data.concat(checksum).map((idx) => charset[idx]).join(""));' "${lock_args_hex}"
}

derive_ckb_topup_address_from_node_info() {
  local rpc_port="$1"
  local payload='{"jsonrpc":"2.0","id":"derive-topup-address","method":"node_info","params":[]}'
  local response
  response="$(fnn_rpc_call_on_port "${rpc_port}" "${payload}")" || return 1

  local lock_args
  lock_args="$(printf '%s' "${response}" | jq -r '.result.default_funding_lock_script.args // empty')"
  [[ -n "${lock_args}" ]] || return 1

  derive_ckb_testnet_address_from_lock_args "${lock_args}"
}

derive_ckb_topup_address_with_retry() {
  local rpc_port="$1"
  local label="$2"
  local attempt=1
  local derived=""

  while [[ "${attempt}" -le "${FNN_DUAL_NODE_INFO_DERIVE_RETRIES}" ]]; do
    set +e
    derived="$(derive_ckb_topup_address_from_node_info "${rpc_port}")"
    local rc=$?
    set -e
    if [[ "${rc}" -eq 0 && -n "${derived}" ]]; then
      printf '%s' "${derived}"
      return 0
    fi
    if [[ "${attempt}" -lt "${FNN_DUAL_NODE_INFO_DERIVE_RETRIES}" ]]; then
      log "derive ${label} top-up address attempt=${attempt} failed; retrying in ${FNN_DUAL_NODE_INFO_DERIVE_RETRY_INTERVAL_SECONDS}s"
      sleep "${FNN_DUAL_NODE_INFO_DERIVE_RETRY_INTERVAL_SECONDS}"
    fi
    attempt=$((attempt + 1))
  done

  return 1
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
const objs = Array.isArray(resp?.result?.objects) ? resp.result.objects : [];
let total = 0n;
for (const obj of objs) {
  const cap = obj?.output?.capacity ?? "";
  if (typeof cap !== "string" || cap.length === 0) continue;
  try {
    total += BigInt(cap);
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
    if [[ "${page}" -gt "${FNN_DUAL_CKB_BALANCE_CHECK_LIMIT_PAGES}" ]]; then
      break
    fi

    local payload
    payload="$(jq -cn \
      --arg id "ckb-balance-${label}-${page}" \
      --argjson lock_script "${lock_script_json}" \
      --arg cursor "${cursor}" \
      '
      if $cursor == "0x" then
        {
          jsonrpc:"2.0",
          id:$id,
          method:"get_cells",
          params:[
            {script:$lock_script,script_type:"lock"},
            "asc",
            "0x64"
          ]
        }
      else
        {
          jsonrpc:"2.0",
          id:$id,
          method:"get_cells",
          params:[
            {script:$lock_script,script_type:"lock"},
            "asc",
            "0x64",
            $cursor
          ]
        }
      end
      ')" 

    local response
    response="$(ckb_rpc_call "${payload}")" || return 1
    if printf '%s' "${response}" | jq -e '.error != null' >/dev/null 2>&1; then
      return 1
    fi

    local page_sum
    page_sum="$(sum_ckb_capacity_from_get_cells_response "${response}")"
    total="$(node -e 'console.log((BigInt(process.argv[1]) + BigInt(process.argv[2])).toString())' "${total}" "${page_sum}")"

    local count next_cursor
    count="$(printf '%s' "${response}" | jq -r '.result.objects | length')"
    next_cursor="$(printf '%s' "${response}" | jq -r '.result.last_cursor // "0x"')"
    if [[ "${count}" -eq 0 || -z "${next_cursor}" || "${next_cursor}" == "${cursor}" ]]; then
      break
    fi
    cursor="${next_cursor}"
  done

  printf '%s' "${total}"
}

request_ckb_faucet() {
  local address="$1"
  local label="$2"

  local payload response_file http_code
  payload="$(jq -cn --arg address "${address}" --arg amount "${FNN_DUAL_CKB_FAUCET_AMOUNT}" '{claim_event:{address_hash:$address,amount:$amount}}')"
  response_file="${LOG_DIR}/ckb-faucet-${label}.json"
  set +e
  http_code="$(curl -sS -o "${response_file}" -w "%{http_code}" \
    -H "content-type: application/json" \
    -d "${payload}" \
    "${FNN_DUAL_CKB_FAUCET_API_BASE%/}/claim_events")"
  local rc=$?
  set -e
  if [[ "${rc}" -ne 0 || "${http_code}" -lt 200 || "${http_code}" -ge 300 ]]; then
    if [[ "${FNN_DUAL_CKB_FAUCET_ENABLE_FALLBACK}" == "1" ]]; then
      local fallback_payload fallback_file fallback_code
      fallback_payload="$(jq -cn --arg address "${address}" '{address:$address,token:"ckb"}')"
      fallback_file="${LOG_DIR}/ckb-faucet-fallback-${label}.json"
      set +e
      fallback_code="$(curl -sS -o "${fallback_file}" -w "%{http_code}" \
        -H "content-type: application/json" \
        -d "${fallback_payload}" \
        "${FNN_DUAL_CKB_FAUCET_FALLBACK_API_BASE%/}/faucet")"
      rc=$?
      set -e
      if [[ "${rc}" -eq 0 && "${fallback_code}" -ge 200 && "${fallback_code}" -lt 300 ]]; then
        log "CKB faucet fallback accepted for ${label}; waiting ${FNN_DUAL_CKB_FAUCET_WAIT_SECONDS}s"
        sleep "${FNN_DUAL_CKB_FAUCET_WAIT_SECONDS}"
        return 0
      fi
    fi
    return 1
  fi

  log "CKB faucet accepted for ${label}; waiting ${FNN_DUAL_CKB_FAUCET_WAIT_SECONDS}s"
  sleep "${FNN_DUAL_CKB_FAUCET_WAIT_SECONDS}"
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
    log "CKB balance precheck passed for ${label} (balance=${balance}, required=${required_amount})"
    return 0
  fi

  log "CKB faucet required for ${label}"
  request_ckb_faucet "${address}" "${label}" || {
    echo "unable to top up ${label} with faucet" >&2
    exit 1
  }

  local deadline
  deadline=$(( $(date +%s) + FNN_DUAL_CKB_BALANCE_WAIT_TIMEOUT_SECONDS ))
  while [[ "$(date +%s)" -lt "${deadline}" ]]; do
    set +e
    balance="$(query_ckb_balance_for_lock_script "${lock_script_json}" "${label}")"
    rc=$?
    set -e
    if [[ "${rc}" -eq 0 && -n "${balance}" ]] && bigint_gte "${balance}" "${required_amount}"; then
      log "CKB balance reached required threshold for ${label} (balance=${balance}, required=${required_amount})"
      return 0
    fi
    sleep "${FNN_DUAL_CKB_BALANCE_POLL_INTERVAL_SECONDS}"
  done

  echo "CKB balance still below required threshold for ${label}" >&2
  exit 1
}

require_env "FNN_ASSET_SHA256" "${FNN_ASSET_SHA256}"
require_env "FIBER_SECRET_KEY_PASSWORD" "${FIBER_SECRET_KEY_PASSWORD}"

cd "$ROOT_DIR"
./scripts/compose-fnn-reference.test.sh

for binary in docker curl jq node; do
  command -v "${binary}" >/dev/null 2>&1 || {
    echo "missing required binary: ${binary}" >&2
    exit 1
  }
done

compose up -d --build fnn fnn2 fiber-adapter
compose build e2e-runner >/dev/null

wait_for_http_reachable "http://127.0.0.1:${FNN_PUBLISHED_RPC_PORT}" "fnn"
wait_for_http_reachable "http://127.0.0.1:${FNN2_PUBLISHED_RPC_PORT}" "fnn2"
wait_for_http_reachable "http://127.0.0.1:${FIBER_ADAPTER_PUBLISHED_PORT:-28091}/healthz" "fiber-adapter"

if [[ -z "${FNN_DUAL_CKB_TOPUP_ADDRESS}" ]]; then
  FNN_DUAL_CKB_TOPUP_ADDRESS="$(derive_ckb_topup_address_with_retry "${FNN2_PUBLISHED_RPC_PORT}" "fnn2")" || {
    echo "unable to derive payer top-up address from fnn2 node_info" >&2
    exit 1
  }
  log "derived payer top-up address=${FNN_DUAL_CKB_TOPUP_ADDRESS}"
fi

if [[ -z "${FNN_DUAL_INVOICE_CKB_TOPUP_ADDRESS}" ]]; then
  FNN_DUAL_INVOICE_CKB_TOPUP_ADDRESS="$(derive_ckb_topup_address_with_retry "${FNN_PUBLISHED_RPC_PORT}" "fnn")" || {
    echo "unable to derive invoice top-up address from fnn node_info" >&2
    exit 1
  }
  log "derived invoice top-up address=${FNN_DUAL_INVOICE_CKB_TOPUP_ADDRESS}"
fi

payer_lock_script_json="$(fetch_lock_script_json "${FNN2_PUBLISHED_RPC_PORT}")"
invoice_lock_script_json="$(fetch_lock_script_json "${FNN_PUBLISHED_RPC_PORT}")"
[[ -n "${payer_lock_script_json}" ]] || {
  echo "payer node_info missing default_funding_lock_script" >&2
  exit 1
}
[[ -n "${invoice_lock_script_json}" ]] || {
  echo "invoice node_info missing default_funding_lock_script" >&2
  exit 1
}

invoice_required_amount="1"
if [[ -n "${RELEASE_FNN_DUAL_ACCEPT_FUNDING_AMOUNT:-}" ]]; then
  if [[ "${RELEASE_FNN_DUAL_ACCEPT_FUNDING_AMOUNT}" =~ ^0x[0-9a-fA-F]+$ ]]; then
    invoice_required_amount="$(hex_quantity_to_decimal "${RELEASE_FNN_DUAL_ACCEPT_FUNDING_AMOUNT}")"
  else
    invoice_required_amount="${RELEASE_FNN_DUAL_ACCEPT_FUNDING_AMOUNT}"
  fi
fi

ensure_ckb_balance_or_request_faucet "${FNN_DUAL_CKB_TOPUP_ADDRESS}" "payer-bootstrap" "${FNN_DUAL_CHANNEL_FUNDING_AMOUNT}" "${payer_lock_script_json}"
if [[ "${FNN_DUAL_TOPUP_INVOICE_NODE_CKB}" == "1" ]]; then
  ensure_ckb_balance_or_request_faucet "${FNN_DUAL_INVOICE_CKB_TOPUP_ADDRESS}" "invoice-bootstrap" "${invoice_required_amount}" "${invoice_lock_script_json}"
else
  log "invoice bootstrap top-up skipped"
fi

runner_args=(
  run --rm --no-deps
  -e "RELEASE_FNN_DUAL_FUNDING_AMOUNT=${FNN_DUAL_CHANNEL_FUNDING_AMOUNT}"
)
if [[ -n "${RELEASE_FNN_DUAL_ACCEPT_FUNDING_AMOUNT:-}" ]]; then
  runner_args+=(-e "RELEASE_FNN_DUAL_ACCEPT_FUNDING_AMOUNT=${RELEASE_FNN_DUAL_ACCEPT_FUNDING_AMOUNT}")
fi
runner_args+=(e2e-runner /usr/local/bin/release-fnn-dual-node-smoke)

compose "${runner_args[@]}"
