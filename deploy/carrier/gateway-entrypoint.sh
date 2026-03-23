#!/bin/sh
set -eu

KEY_PATH="${CARRIER_REMOTE_PRIVATE_KEY_PATH:-/keys/id_ed25519}"

if [ -n "${CARRIER_REMOTE_PRIVATE_KEY_BASE64:-}" ]; then
  umask 077
  mkdir -p "$(dirname "${KEY_PATH}")"
  printf '%s' "${CARRIER_REMOTE_PRIVATE_KEY_BASE64}" | base64 -d >"${KEY_PATH}"
elif [ -n "${CARRIER_REMOTE_PRIVATE_KEY:-}" ]; then
  umask 077
  mkdir -p "$(dirname "${KEY_PATH}")"
  printf '%s\n' "${CARRIER_REMOTE_PRIVATE_KEY}" >"${KEY_PATH}"
fi

if [ -f "${KEY_PATH}" ]; then
  chmod 600 "${KEY_PATH}"
fi

exec "$@"
