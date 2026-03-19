#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

cd "$ROOT_DIR"
SCREENSHOT_CAPTURE_MODE="${SCREENSHOT_CAPTURE_MODE:-full-pages}" \
./scripts/release-compose-e2e-screenshots.sh
