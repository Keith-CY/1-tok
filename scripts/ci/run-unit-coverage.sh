#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
OUT_DIR="${COVERAGE_OUTPUT_DIR:-$ROOT_DIR/.artifacts/coverage}"
GO_PACKAGE_DIR="$OUT_DIR/go-packages"

mkdir -p "$GO_PACKAGE_DIR"

GO_LOG="$OUT_DIR/go.log"
GO_SUMMARY="$OUT_DIR/go-summary.txt"
GO_PROFILE="$OUT_DIR/go.coverprofile"
WEB_LOG="$OUT_DIR/web.log"
CONTRACTS_LOG="$OUT_DIR/contracts.log"
SUMMARY_JSON="$OUT_DIR/summary.json"
SUMMARY_MD="$OUT_DIR/summary.md"

failures=0

run_with_log() {
  local log_file="$1"
  shift

  set +e
  "$@" 2>&1 | tee "$log_file"
  local status=${PIPESTATUS[0]}
  set -e

  return "$status"
}

run_go_coverage() {
  : >"$GO_LOG"
  echo "mode: atomic" >"$GO_PROFILE"

  local packages=()
  while IFS= read -r line; do
    if [[ -n "$line" ]]; then
      packages+=("$line")
    fi
  done < <(go list -f '{{if or (gt (len .TestGoFiles) 0) (gt (len .XTestGoFiles) 0)}}{{.ImportPath}}{{end}}' ./... | grep -v '/cmd/')

  if [[ ${#packages[@]} -eq 0 ]]; then
    printf 'total:\t\t\t\t\t\t\t\t(statements)\t0.0%%\n' >"$GO_SUMMARY"
    return 0
  fi

  local package_failed=0
  local index=0
  for pkg in "${packages[@]}"; do
    local profile="$GO_PACKAGE_DIR/${index}.coverprofile"
    index=$((index + 1))

    set +e
    {
      echo "==> $pkg"
      CGO_ENABLED=0 go test -tags=integration -covermode=atomic -coverprofile="$profile" "$pkg"
    } 2>&1 | tee -a "$GO_LOG"
    local status=${PIPESTATUS[0]}
    set -e

    if [[ $status -ne 0 ]]; then
      package_failed=1
      continue
    fi

    if [[ -f "$profile" ]]; then
      tail -n +2 "$profile" >>"$GO_PROFILE"
    fi
  done

  go tool cover -func="$GO_PROFILE" | tee "$GO_SUMMARY"

  return "$package_failed"
}

cd "$ROOT_DIR"

if ! run_go_coverage; then
  failures=1
fi

if ! run_with_log "$WEB_LOG" bun --cwd apps/web test --coverage; then
  failures=1
fi

if ! run_with_log "$CONTRACTS_LOG" bun --cwd packages/contracts test --coverage; then
  failures=1
fi

bun "$ROOT_DIR/scripts/ci/coverage-summary.mjs" \
  --go-summary "$GO_SUMMARY" \
  --web-log "$WEB_LOG" \
  --contracts-log "$CONTRACTS_LOG" \
  --output-json "$SUMMARY_JSON" \
  --output-md "$SUMMARY_MD"

if [[ $failures -ne 0 ]]; then
  echo "unit coverage run failed" >&2
  exit 1
fi
