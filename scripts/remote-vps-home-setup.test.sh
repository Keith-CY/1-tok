#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SETUP_SCRIPT="${ROOT_DIR}/deploy/carrier/setup-remote-vps-home.sh"

[[ -x "${SETUP_SCRIPT}" ]] || {
  echo "missing setup helper: ${SETUP_SCRIPT}" >&2
  exit 1
}

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT

export REMOTE_HOME="${TMP_DIR}"
export OPENAI_API_KEY="sk-openai-demo"
export ANTHROPIC_BASE_URL="https://share-ai.ckbdev.com"
export ANTHROPIC_AUTH_TOKEN="sk-demo-anthropic-token"
export CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC="1"

"${SETUP_SCRIPT}"

PROFILE="${TMP_DIR}/.bash_profile"
CONFIG="${TMP_DIR}/.config/opencode/opencode.json"

[[ -f "${PROFILE}" ]] || {
  echo "missing profile: ${PROFILE}" >&2
  exit 1
}

[[ -f "${CONFIG}" ]] || {
  echo "missing opencode config: ${CONFIG}" >&2
  exit 1
}

grep -q 'export OPENAI_API_KEY=' "${PROFILE}" || {
  echo "profile missing OPENAI_API_KEY export" >&2
  exit 1
}

grep -q 'export ANTHROPIC_BASE_URL=' "${PROFILE}" || {
  echo "profile missing ANTHROPIC_BASE_URL export" >&2
  exit 1
}

grep -q 'export ANTHROPIC_AUTH_TOKEN=' "${PROFILE}" || {
  echo "profile missing ANTHROPIC_AUTH_TOKEN export" >&2
  exit 1
}

grep -q 'export ANTHROPIC_API_KEY=' "${PROFILE}" || {
  echo "profile missing ANTHROPIC_API_KEY alias export" >&2
  exit 1
}

grep -q 'export CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=' "${PROFILE}" || {
  echo "profile missing CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC export" >&2
  exit 1
}

grep -q '"npm": "@ai-sdk/anthropic"' "${CONFIG}" || {
  echo "opencode config missing anthropic provider package" >&2
  exit 1
}

grep -q '"baseURL": "https://share-ai.ckbdev.com"' "${CONFIG}" || {
  echo "opencode config missing anthropic base url" >&2
  exit 1
}

grep -q '"Authorization": "Bearer {env:ANTHROPIC_AUTH_TOKEN}"' "${CONFIG}" || {
  echo "opencode config missing authorization header template" >&2
  exit 1
}

grep -q '"model": "demo-anthropic/claude-sonnet-4-20250514"' "${CONFIG}" || {
  echo "opencode config missing default model" >&2
  exit 1
}

echo "remote vps home setup checks passed"
