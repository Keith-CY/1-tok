# 1-tok

`1-tok` is a monorepo for the Agent marketplace and settlement platform described in the approved plan. This first implementation pass establishes:

- a shared Go domain model for orders, milestones, usage charges, disputes, and credit decisions
- a JSON HTTP gateway that exposes the core marketplace and settlement flows
- service entrypoints for `iam`, `marketplace`, `settlement`, `risk`, `execution`, and `notification`
- shared TypeScript contracts for the web portal
- local container topology for Go services plus Postgres and NATS

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

### Compose

```bash
docker compose up --build
```

## Current scope

The Go services currently use in-memory state for orders, disputes, and messages. Postgres and NATS are wired in the deployment topology but are not yet used as the persistence/event backbone. This keeps the first pass executable while preserving the service and deployment shape from the plan.
