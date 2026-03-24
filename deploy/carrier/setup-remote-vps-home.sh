#!/usr/bin/env bash
set -euo pipefail

remote_home="${REMOTE_HOME:-/home/carrier}"
profile_path="${remote_home}/.bash_profile"
opencode_dir="${remote_home}/.config/opencode"
opencode_config_path="${opencode_dir}/opencode.json"

append_export_from_env() {
  local key="$1"
  local value="${!key:-}"
  if [[ -n "${value}" ]]; then
    printf 'export %s=%q\n' "${key}" "${value}" >>"${profile_path}"
  fi
}

mkdir -p "${remote_home}" "${opencode_dir}"

cat >"${profile_path}" <<'EOF'
export NPM_CONFIG_PREFIX="$HOME/.npm-global"
export PATH="$HOME/.npm-global/bin:$HOME/.local/bin:$PATH"
EOF

for key in \
  OPENAI_API_KEY \
  OPENAI_CODEX_TOKEN \
  OPENAI_BASE_URL \
  OPENAI_COMPATIBLE_API_KEY \
  OPENAI_COMPATIBLE_BASE_URL \
  ANTHROPIC_BASE_URL \
  ANTHROPIC_AUTH_TOKEN \
  ANTHROPIC_API_KEY \
  CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC; do
  append_export_from_env "${key}"
done

if [[ -n "${ANTHROPIC_AUTH_TOKEN:-}" && -z "${ANTHROPIC_API_KEY:-}" ]]; then
  printf 'export ANTHROPIC_API_KEY=%q\n' "${ANTHROPIC_AUTH_TOKEN}" >>"${profile_path}"
fi

if [[ -n "${ANTHROPIC_BASE_URL:-}" && ( -n "${ANTHROPIC_AUTH_TOKEN:-}" || -n "${ANTHROPIC_API_KEY:-}" ) ]]; then
  provider_id="${OPENCODE_CUSTOM_PROVIDER_ID:-demo-anthropic}"
  provider_name="${OPENCODE_CUSTOM_PROVIDER_NAME:-Demo Anthropic Gateway}"
  provider_npm="${OPENCODE_CUSTOM_PROVIDER_NPM:-@ai-sdk/anthropic}"
  model_id="${OPENCODE_CUSTOM_MODEL_ID:-claude-sonnet-4-20250514}"
  model_name="${OPENCODE_CUSTOM_MODEL_NAME:-Claude Sonnet 4}"
  api_key_ref="${OPENCODE_CUSTOM_API_KEY_REF:-{env:ANTHROPIC_API_KEY}}"
  auth_header="${OPENCODE_CUSTOM_AUTH_HEADER:-}"
  if [[ -z "${auth_header}" && -n "${ANTHROPIC_AUTH_TOKEN:-}" ]]; then
    auth_header="Bearer {env:ANTHROPIC_AUTH_TOKEN}"
  fi

  jq -n \
    --arg schema "https://opencode.ai/config.json" \
    --arg provider_id "${provider_id}" \
    --arg provider_name "${provider_name}" \
    --arg provider_npm "${provider_npm}" \
    --arg base_url "${ANTHROPIC_BASE_URL}" \
    --arg model_id "${model_id}" \
    --arg model_name "${model_name}" \
    --arg api_key_ref "${api_key_ref}" \
    --arg auth_header "${auth_header}" \
    '{
      "$schema": $schema,
      "provider": {
        ($provider_id): ({
          "npm": $provider_npm,
          "name": $provider_name,
          "options": ({
            "baseURL": $base_url,
            "apiKey": $api_key_ref
          } + (if $auth_header != "" then {
            "headers": {
              "Authorization": $auth_header
            }
          } else {} end)),
          "models": {
            ($model_id): {
              "name": $model_name
            }
          }
        })
      },
      "model": ($provider_id + "/" + $model_id)
    }' >"${opencode_config_path}"
else
  rm -f "${opencode_config_path}"
fi

chmod 600 "${profile_path}"
if [[ -f "${opencode_config_path}" ]]; then
  chmod 600 "${opencode_config_path}"
fi
