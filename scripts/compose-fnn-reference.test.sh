#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE_FNN_FILE="${ROOT_DIR}/compose.fnn.yaml"
COMPOSE_E2E_FILE="${ROOT_DIR}/compose.e2e.yaml"
FNN_DOCKERFILE="${ROOT_DIR}/deploy/fnn/Dockerfile"
FNN_ENTRYPOINT="${ROOT_DIR}/deploy/fnn/entrypoint.sh"
FNN_CONFIG="${ROOT_DIR}/deploy/fnn/config/testnet.yml"
FNN_SMOKE_SCRIPT="${ROOT_DIR}/scripts/release-compose-fnn-smoke.sh"
DOCKER_E2E_SCRIPT="${ROOT_DIR}/scripts/release-compose-e2e.sh"
DUAL_NODE_SCRIPT="${ROOT_DIR}/scripts/release-compose-fnn-dual-node-smoke.sh"
CARRIER_DOC="${ROOT_DIR}/docs/carrier-pr-support.md"
PACKAGE_JSON="${ROOT_DIR}/package.json"
README_FILE="${ROOT_DIR}/README.md"
COOLIFY_README="${ROOT_DIR}/deploy/coolify/README.md"
E2E_DOCKERFILE="${ROOT_DIR}/deploy/e2e/Dockerfile"

required_files=(
  "${COMPOSE_FNN_FILE}"
  "${COMPOSE_E2E_FILE}"
  "${FNN_DOCKERFILE}"
  "${FNN_ENTRYPOINT}"
  "${FNN_CONFIG}"
  "${FNN_SMOKE_SCRIPT}"
  "${DOCKER_E2E_SCRIPT}"
  "${DUAL_NODE_SCRIPT}"
  "${CARRIER_DOC}"
  "${E2E_DOCKERFILE}"
)

for file in "${required_files[@]}"; do
  [[ -f "${file}" ]] || {
    echo "missing required file: ${file}" >&2
    exit 1
  }
done

grep -q "^  fnn:$" "${COMPOSE_FNN_FILE}" || {
  echo "compose.fnn.yaml missing fnn service" >&2
  exit 1
}

grep -q "^  fnn2:$" "${COMPOSE_FNN_FILE}" || {
  echo "compose.fnn.yaml missing fnn2 service" >&2
  exit 1
}

grep -q "^  fiber-adapter:$" "${COMPOSE_FNN_FILE}" || {
  echo "compose.fnn.yaml missing fiber-adapter service" >&2
  exit 1
}

grep -q "^  e2e-runner:$" "${COMPOSE_E2E_FILE}" || {
  echo "compose.e2e.yaml missing e2e-runner service" >&2
  exit 1
}

grep -q "^  mock-sentry:$" "${COMPOSE_E2E_FILE}" || {
  echo "compose.e2e.yaml missing mock-sentry service" >&2
  exit 1
}

grep -q "dockerfile: deploy/fnn/Dockerfile" "${COMPOSE_FNN_FILE}" || {
  echo "compose.fnn.yaml missing deploy/fnn/Dockerfile reference" >&2
  exit 1
}

grep -q "dockerfile: deploy/e2e/Dockerfile" "${COMPOSE_E2E_FILE}" || {
  echo "compose.e2e.yaml missing deploy/e2e/Dockerfile reference" >&2
  exit 1
}

grep -Fq '${FNN_PUBLISHED_RPC_PORT:-28227}:8227' "${COMPOSE_FNN_FILE}" || {
  echo "compose.fnn.yaml missing consistent fnn rpc port mapping" >&2
  exit 1
}

grep -Fq '${FNN2_PUBLISHED_RPC_PORT:-29227}:8227' "${COMPOSE_FNN_FILE}" || {
  echo "compose.fnn.yaml missing consistent fnn2 rpc port mapping" >&2
  exit 1
}

grep -q "curl -sS --max-time 3" "${COMPOSE_FNN_FILE}" || {
  echo "compose.fnn.yaml missing fnn curl healthcheck" >&2
  exit 1
}

grep -q "FNN_INVOICE_RPC_URL: http://fnn:8227" "${COMPOSE_FNN_FILE}" || {
  echo "compose.fnn.yaml missing fiber-adapter invoice node wiring" >&2
  exit 1
}

grep -q "FNN_PAYER_RPC_URL: http://fnn2:8227" "${COMPOSE_FNN_FILE}" || {
  echo "compose.fnn.yaml missing fiber-adapter payer node wiring" >&2
  exit 1
}

grep -q "sha256sum --check" "${FNN_DOCKERFILE}" || {
  echo "deploy/fnn/Dockerfile missing sha256 verification" >&2
  exit 1
}

grep -Eq 'exec ./fnn -c .* -d .* --rpc-listening-addr .*' "${FNN_ENTRYPOINT}" || {
  echo "deploy/fnn/entrypoint.sh missing canonical fnn startup command" >&2
  exit 1
}

grep -q "go build -o /out/release-smoke ./cmd/release-smoke" "${E2E_DOCKERFILE}" || {
  echo "deploy/e2e/Dockerfile missing release-smoke build" >&2
  exit 1
}

grep -q "go build -o /out/release-portal-smoke ./cmd/release-portal-smoke" "${E2E_DOCKERFILE}" || {
  echo "deploy/e2e/Dockerfile missing release-portal-smoke build" >&2
  exit 1
}

grep -q "go build -o /out/release-abuse-smoke ./cmd/release-abuse-smoke" "${E2E_DOCKERFILE}" || {
  echo "deploy/e2e/Dockerfile missing release-abuse-smoke build" >&2
  exit 1
}

grep -q "go build -o /out/release-fnn-adapter-smoke ./cmd/release-fnn-adapter-smoke" "${E2E_DOCKERFILE}" || {
  echo "deploy/e2e/Dockerfile missing release-fnn-adapter-smoke build" >&2
  exit 1
}

grep -q "go build -o /out/release-fnn-dual-node-smoke ./cmd/release-fnn-dual-node-smoke" "${E2E_DOCKERFILE}" || {
  echo "deploy/e2e/Dockerfile missing release-fnn-dual-node-smoke build" >&2
  exit 1
}

grep -q '"release:compose-fnn-smoke"' "${PACKAGE_JSON}" || {
  echo "package.json missing release:compose-fnn-smoke script" >&2
  exit 1
}

grep -q '"release:compose-fnn-dual-node-smoke"' "${PACKAGE_JSON}" || {
  echo "package.json missing release:compose-fnn-dual-node-smoke script" >&2
  exit 1
}

grep -q "derive_ckb_topup_address_with_retry" "${DUAL_NODE_SCRIPT}" || {
  echo "dual-node fnn smoke script missing top-up address derivation" >&2
  exit 1
}

grep -q "ensure_ckb_balance_or_request_faucet" "${DUAL_NODE_SCRIPT}" || {
  echo "dual-node fnn smoke script missing CKB balance bootstrap" >&2
  exit 1
}

grep -q "request_ckb_faucet" "${DUAL_NODE_SCRIPT}" || {
  echo "dual-node fnn smoke script missing CKB faucet helper" >&2
  exit 1
}

grep -q "get_cells" "${DUAL_NODE_SCRIPT}" || {
  echo "dual-node fnn smoke script missing CKB balance query" >&2
  exit 1
}

grep -q '"release:compose-e2e"' "${PACKAGE_JSON}" || {
  echo "package.json missing release:compose-e2e script" >&2
  exit 1
}

grep -q '"release:abuse-smoke"' "${PACKAGE_JSON}" || {
  echo "package.json missing release:abuse-smoke script" >&2
  exit 1
}

grep -q "release:compose-fnn-smoke" "${README_FILE}" || {
  echo "README missing compose fnn smoke documentation" >&2
  exit 1
}

grep -q "release:compose-e2e" "${README_FILE}" || {
  echo "README missing docker-only compose e2e documentation" >&2
  exit 1
}

grep -q "\`fnn\`" "${COOLIFY_README}" || {
  echo "deploy/coolify/README.md missing fnn service guidance" >&2
  exit 1
}

grep -q "Carrier" "${CARRIER_DOC}" || {
  echo "carrier support doc missing carrier contract content" >&2
  exit 1
}

echo "compose fnn reference checks passed"
