#!/usr/bin/env bash
set -euo pipefail

# Run integration tests with postgres for coverage
# Requires docker to be available

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
OUT_DIR="${COVERAGE_OUTPUT_DIR:-$ROOT_DIR/.artifacts/coverage}"
PROFILE="$OUT_DIR/go-integration.coverprofile"

cd "$ROOT_DIR"

# Start postgres
PG_CONTAINER="1tok-ci-integration-pg"
docker rm -f "$PG_CONTAINER" 2>/dev/null || true
docker run -d --name "$PG_CONTAINER" \
  -e POSTGRES_USER=onetok \
  -e POSTGRES_PASSWORD=testpass \
  -e POSTGRES_DB=onetok_test \
  -p 25433:5432 \
  postgres:16-alpine

# Wait for ready
for i in $(seq 1 30); do
  if docker exec "$PG_CONTAINER" pg_isready -U onetok >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

export ONE_TOK_TEST_DATABASE_URL="postgres://onetok:testpass@127.0.0.1:25433/onetok_test?sslmode=disable"

# Run integration tests
echo "mode: atomic" > "$PROFILE"
CGO_ENABLED=0 go test -tags=integration -covermode=atomic -coverprofile="$PROFILE.tmp" ./internal/ 2>&1 || true
if [[ -f "$PROFILE.tmp" ]]; then
  tail -n +2 "$PROFILE.tmp" >> "$PROFILE"
  rm "$PROFILE.tmp"
fi

# Run postgres store tests
CGO_ENABLED=0 go test -covermode=atomic -coverprofile="$PROFILE.pg.tmp" ./internal/store/postgres/ 2>&1 || true
if [[ -f "$PROFILE.pg.tmp" ]]; then
  tail -n +2 "$PROFILE.pg.tmp" >> "$PROFILE"
  rm "$PROFILE.pg.tmp"
fi

# Run settlement postgres tests
CGO_ENABLED=0 go test -covermode=atomic -coverprofile="$PROFILE.settle.tmp" ./internal/services/settlement/ 2>&1 || true
if [[ -f "$PROFILE.settle.tmp" ]]; then
  tail -n +2 "$PROFILE.settle.tmp" >> "$PROFILE"
  rm "$PROFILE.settle.tmp"
fi

# Run bootstrap with postgres
CGO_ENABLED=0 go test -covermode=atomic -coverprofile="$PROFILE.boot.tmp" ./internal/bootstrap/ 2>&1 || true
if [[ -f "$PROFILE.boot.tmp" ]]; then
  tail -n +2 "$PROFILE.boot.tmp" >> "$PROFILE"
  rm "$PROFILE.boot.tmp"
fi

# Cleanup
docker stop "$PG_CONTAINER" && docker rm "$PG_CONTAINER"

echo "Integration coverage profile: $PROFILE"
go tool cover -func="$PROFILE" | tail -1
