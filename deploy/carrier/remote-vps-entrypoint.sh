#!/usr/bin/env bash
set -euo pipefail

: "${AUTHORIZED_KEY:?AUTHORIZED_KEY is required}"

mkdir -p /home/carrier/.ssh
printf '%s\n' "${AUTHORIZED_KEY}" >/home/carrier/.ssh/authorized_keys
chown -R carrier:carrier /home/carrier/.ssh
chmod 700 /home/carrier/.ssh
chmod 600 /home/carrier/.ssh/authorized_keys

/usr/local/bin/setup-remote-vps-home.sh
chown carrier:carrier /home/carrier/.bash_profile
chmod 600 /home/carrier/.bash_profile
if [[ -d /home/carrier/.codex ]]; then
  chown -R carrier:carrier /home/carrier/.codex
fi
mkdir -p /workspace
chown carrier:carrier /workspace

export LANG=C.UTF-8
export LC_ALL=C.UTF-8

exec /usr/sbin/sshd -D -e -p "${SSH_PORT:-22}" -o "MaxStartups=100:30:200"
