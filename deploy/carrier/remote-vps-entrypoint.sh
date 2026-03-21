#!/usr/bin/env bash
set -euo pipefail

: "${AUTHORIZED_KEY:?AUTHORIZED_KEY is required}"

mkdir -p /home/carrier/.ssh
printf '%s\n' "${AUTHORIZED_KEY}" >/home/carrier/.ssh/authorized_keys
chown -R carrier:carrier /home/carrier/.ssh
chmod 700 /home/carrier/.ssh
chmod 600 /home/carrier/.ssh/authorized_keys

cat >/home/carrier/.bash_profile <<'EOF'
export NPM_CONFIG_PREFIX="$HOME/.npm-global"
export PATH="$HOME/.npm-global/bin:$HOME/.local/bin:$PATH"
EOF

if [[ -n "${OPENAI_API_KEY:-}" ]]; then
  printf 'export OPENAI_API_KEY=%q\n' "${OPENAI_API_KEY}" >>/home/carrier/.bash_profile
fi
if [[ -n "${OPENAI_CODEX_TOKEN:-}" ]]; then
  printf 'export OPENAI_CODEX_TOKEN=%q\n' "${OPENAI_CODEX_TOKEN}" >>/home/carrier/.bash_profile
fi
if [[ -n "${OPENAI_BASE_URL:-}" ]]; then
  printf 'export OPENAI_BASE_URL=%q\n' "${OPENAI_BASE_URL}" >>/home/carrier/.bash_profile
fi
chown carrier:carrier /home/carrier/.bash_profile
chmod 600 /home/carrier/.bash_profile

export LANG=C.UTF-8
export LC_ALL=C.UTF-8

exec /usr/sbin/sshd -D -e -p "${SSH_PORT:-22}"
