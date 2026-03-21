#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DEPS_DIR="${ROOT_DIR}/.deps"
CARRIER_DIR="${DEPS_DIR}/carrier"
CARRIER_REPO_URL="${CARRIER_REPO_URL:-https://github.com/Keith-CY/carrier.git}"
CARRIER_REF="${CARRIER_REF:-706b071b5c04be95a3cfbf3dea13768b3d43f214}"

mkdir -p "${DEPS_DIR}"

if [[ ! -d "${CARRIER_DIR}/.git" ]]; then
  git clone "${CARRIER_REPO_URL}" "${CARRIER_DIR}"
fi

git -C "${CARRIER_DIR}" fetch --tags --prune origin
git -C "${CARRIER_DIR}" checkout "${CARRIER_REF}"
