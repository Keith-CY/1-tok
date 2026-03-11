# 1-tok

`1-tok` is a monorepo for the Agent marketplace and settlement platform described in the approved plan. This first implementation pass establishes:

- a shared Go domain model for orders, milestones, usage charges, disputes, and credit decisions
- a JSON HTTP gateway that exposes the core marketplace and settlement flows
- service entrypoints for `iam`, `marketplace`, `settlement`, `risk`, `execution`, and `notification`
- shared TypeScript contracts for the web portal
- local container topology for Go services plus Postgres and NATS
- Postgres-backed repositories for orders, messages, and disputes when `DATABASE_URL` is set

## Layout

- `cmd/*`: service binaries
- `internal/core`: order, settlement, dispute, and credit domain logic
- `internal/gateway`: integrated HTTP API surface for marketplace flows
- `internal/services/*`: service-specific HTTP entrypoints
- `packages/contracts`: shared TypeScript types and dashboard helpers
- `apps/web`: Next.js portal shell

## Run

### Go tests

```bash
CGO_ENABLED=0 go test ./...
```

### Contracts tests

```bash
bun run test:contracts
```

### API gateway

```bash
CGO_ENABLED=0 go run ./cmd/api-gateway
```

To enable persistence locally:

```bash
export DATABASE_URL='postgres://onetok:onetok@127.0.0.1:5432/onetok?sslmode=disable'
CGO_ENABLED=0 go run ./cmd/api-gateway
```

### Compose

```bash
docker compose up --build
```

### Postgres repository integration test

```bash
export ONE_TOK_TEST_DATABASE_URL='postgres://onetok:onetok@127.0.0.1:5432/onetok?sslmode=disable'
CGO_ENABLED=0 go test ./internal/store/postgres
```

## Current scope

Provider and listing catalogs are still seeded in memory, and NATS/Fiber/Carrier are still adapter placeholders. Orders, messages, and disputes can now run against Postgres through `DATABASE_URL`, which moves the core task lifecycle off in-memory state.
