#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SETUP_SCRIPT="${ROOT_DIR}/deploy/carrier/setup-remote-vps-home.sh"

[[ -x "${SETUP_SCRIPT}" ]] || {
  echo "missing setup helper: ${SETUP_SCRIPT}" >&2
  exit 1
}

TMP_DIR="$(mktemp -d)"
TMP_DIR_NO_BASE="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}" "${TMP_DIR_NO_BASE}"' EXIT

export REMOTE_HOME="${TMP_DIR}"
export REMOTE_WORKSPACE_ROOT="${TMP_DIR}/workspace"
export OPENAI_API_KEY="sk-openai-demo"
export OPENAI_BASE_URL="https://openai.example.invalid"

"${SETUP_SCRIPT}"

PROFILE="${TMP_DIR}/.bash_profile"
CONFIG="${TMP_DIR}/.codex/config.toml"

[[ -f "${PROFILE}" ]] || {
  echo "missing profile: ${PROFILE}" >&2
  exit 1
}

[[ -f "${CONFIG}" ]] || {
  echo "missing codex config: ${CONFIG}" >&2
  exit 1
}

grep -q 'export OPENAI_API_KEY=' "${PROFILE}" || {
  echo "profile missing OPENAI_API_KEY export" >&2
  exit 1
}

grep -q 'export OPENAI_CODEX_TOKEN=' "${PROFILE}" || {
  echo "profile missing OPENAI_CODEX_TOKEN alias export" >&2
  exit 1
}

grep -q 'model_provider = "openai-custom"' "${CONFIG}" || {
  echo "codex config missing custom model provider id" >&2
  exit 1
}

grep -q 'model = "gpt-5.4"' "${CONFIG}" || {
  echo "codex config missing default model" >&2
  exit 1
}

grep -q 'review_model = "gpt-5.4"' "${CONFIG}" || {
  echo "codex config missing review model" >&2
  exit 1
}

grep -q 'model_reasoning_effort = "xhigh"' "${CONFIG}" || {
  echo "codex config missing reasoning effort" >&2
  exit 1
}

grep -q 'approval_policy = "never"' "${CONFIG}" || {
  echo "codex config missing approval policy" >&2
  exit 1
}

grep -q 'sandbox_mode = "danger-full-access"' "${CONFIG}" || {
  echo "codex config missing sandbox mode" >&2
  exit 1
}

grep -q '\[model_providers.openai-custom\]' "${CONFIG}" || {
  echo "codex config missing custom provider block" >&2
  exit 1
}

grep -q 'base_url = "https://openai.example.invalid"' "${CONFIG}" || {
  echo "codex config missing custom OpenAI base url" >&2
  exit 1
}

grep -q 'env_key = "OPENAI_API_KEY"' "${CONFIG}" || {
  echo "codex config missing API key env binding" >&2
  exit 1
}

grep -q 'wire_api = "responses"' "${CONFIG}" || {
  echo "codex config missing responses wire api" >&2
  exit 1
}

echo "remote vps home setup checks passed"

export REMOTE_HOME="${TMP_DIR_NO_BASE}"
export REMOTE_WORKSPACE_ROOT="${TMP_DIR_NO_BASE}/workspace"
unset OPENAI_BASE_URL

"${SETUP_SCRIPT}"

PROFILE_NO_BASE="${TMP_DIR_NO_BASE}/.bash_profile"
CONFIG_NO_BASE="${TMP_DIR_NO_BASE}/.codex/config.toml"

[[ -f "${PROFILE_NO_BASE}" ]] || {
  echo "missing profile without custom base url: ${PROFILE_NO_BASE}" >&2
  exit 1
}

[[ -f "${CONFIG_NO_BASE}" ]] || {
  echo "missing codex config without custom base url: ${CONFIG_NO_BASE}" >&2
  exit 1
}

grep -q 'export OPENAI_API_KEY=' "${PROFILE_NO_BASE}" || {
  echo "profile without custom base url missing OPENAI_API_KEY export" >&2
  exit 1
}

grep -q 'approval_policy = "never"' "${CONFIG_NO_BASE}" || {
  echo "codex config without custom base url missing approval policy" >&2
  exit 1
}

grep -q 'sandbox_mode = "danger-full-access"' "${CONFIG_NO_BASE}" || {
  echo "codex config without custom base url missing sandbox mode" >&2
  exit 1
}

if grep -q 'model_provider = "openai-custom"' "${CONFIG_NO_BASE}"; then
  echo "codex config without custom base url should not force openai-custom provider" >&2
  exit 1
fi

if grep -q '\[model_providers.openai-custom\]' "${CONFIG_NO_BASE}"; then
  echo "codex config without custom base url should not include custom provider block" >&2
  exit 1
fi

echo "remote vps home setup checks without custom base url passed"
