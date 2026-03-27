#!/usr/bin/env bash
set -euo pipefail

remote_home="${REMOTE_HOME:-/home/carrier}"
remote_workspace_root="${REMOTE_WORKSPACE_ROOT:-/workspace}"
profile_path="${remote_home}/.bash_profile"
codex_dir="${remote_home}/.codex"
codex_config_path="${codex_dir}/config.toml"

append_export_from_env() {
  local key="$1"
  local value="${!key:-}"
  if [[ -n "${value}" ]]; then
    printf 'export %s=%q\n' "${key}" "${value}" >>"${profile_path}"
  fi
}

write_default_codex_config() {
  cat >"${codex_config_path}" <<EOF
model_provider = "openai-custom"
model = "gpt-5.4"
review_model = "gpt-5.4"
model_reasoning_effort = "xhigh"
disable_response_storage = true
network_access = "enabled"
approval_policy = "never"
sandbox_mode = "danger-full-access"
windows_wsl_setup_acknowledged = true
model_context_window = 1000000
model_auto_compact_token_limit = 900000

[model_providers.openai-custom]
name = "OpenAI custom"
base_url = "${OPENAI_BASE_URL}"
env_key = "OPENAI_API_KEY"
wire_api = "responses"
request_max_retries = 4
stream_max_retries = 10
stream_idle_timeout_ms = 300000
websocket_connect_timeout_ms = 15000
EOF
}

mkdir -p "${remote_home}" "${codex_dir}" "${remote_workspace_root}"

cat >"${profile_path}" <<'EOF'
export NPM_CONFIG_PREFIX="$HOME/.npm-global"
export PATH="$HOME/.npm-global/bin:$HOME/.local/bin:$PATH"
EOF

for key in \
  OPENAI_API_KEY \
  OPENAI_CODEX_TOKEN; do
  append_export_from_env "${key}"
done

if [[ -n "${OPENAI_API_KEY:-}" && -z "${OPENAI_CODEX_TOKEN:-}" ]]; then
  printf 'export OPENAI_CODEX_TOKEN=%q\n' "${OPENAI_API_KEY}" >>"${profile_path}"
fi

if [[ -n "${OPENAI_CODEX_TOKEN:-}" && -z "${OPENAI_API_KEY:-}" ]]; then
  printf 'export OPENAI_API_KEY=%q\n' "${OPENAI_CODEX_TOKEN}" >>"${profile_path}"
fi

if [[ -n "${OPENAI_BASE_URL:-}" ]]; then
  write_default_codex_config
else
  rm -f "${codex_config_path}"
fi

chmod 600 "${profile_path}"
if [[ -f "${codex_config_path}" ]]; then
  chmod 600 "${codex_config_path}"
fi
