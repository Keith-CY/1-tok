# 1-tok

`1-tok` is a monorepo for the Agent marketplace and settlement platform described in the approved plan. This first implementation pass establishes:

- a shared Go domain model for orders, milestones, usage charges, disputes, and credit decisions
- a JSON HTTP gateway that exposes the core marketplace and settlement flows
- service entrypoints for `iam`, `marketplace`, `settlement`, `risk`, `execution`, and `notification`
- Fiber JSON-RPC integration for invoice creation, invoice status, withdrawal quote, and withdrawal request
- Carrier gateway integration for remote codeagent health, version, and run control-plane calls
- shared TypeScript contracts for the web portal
- local container topology for Go services plus Postgres and NATS
- Postgres-backed repositories for orders, providers, listings, messages, and disputes when `DATABASE_URL` is set

## Layout

- `cmd/*`: service binaries
- `internal/core`: order, settlement, dispute, and credit domain logic
- `internal/gateway`: integrated HTTP API surface for marketplace flows
- `internal/services/*`: service-specific HTTP entrypoints
- `packages/contracts`: shared TypeScript types and dashboard helpers
- `apps/web`: Next.js portal shell

## Run

Go service builds now target Go `1.25`.

### Go tests

```bash
CGO_ENABLED=0 go test ./...
```

### Release smoke

```bash
export RELEASE_SMOKE_API_BASE_URL='http://127.0.0.1:8080'
export RELEASE_SMOKE_SETTLEMENT_BASE_URL='http://127.0.0.1:8083'
export RELEASE_SMOKE_EXECUTION_BASE_URL='http://127.0.0.1:8085'
bun run release:smoke
```

The smoke command defaults to the smallest cross-service path: create an order, drive a milestone-ready execution event, create an invoice, sync settlement state, and assert invoice funding records. Optional probes can be enabled with:

```bash
export RELEASE_SMOKE_INCLUDE_WITHDRAWAL=true
export RELEASE_SMOKE_INCLUDE_CARRIER_PROBE=true
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
export NATS_URL='nats://127.0.0.1:4222'
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

### Settlement service with Fiber

```bash
export FIBER_RPC_URL='http://127.0.0.1:3000/rpc'
export FIBER_APP_ID='app_1'
export FIBER_HMAC_SECRET='replace-me'
CGO_ENABLED=0 go run ./cmd/settlement
```

HTTP routes added by `settlement`:

- `POST /v1/invoices`
- `GET /v1/invoices/:invoice`
- `POST /v1/withdrawals/quote`
- `POST /v1/withdrawals`
- `GET /v1/funding-records`

### Execution service with Carrier

```bash
export API_GATEWAY_UPSTREAM='http://127.0.0.1:8080'
export CARRIER_GATEWAY_URL='http://127.0.0.1:8787'
export CARRIER_GATEWAY_API_TOKEN='test-gateway-token'
CGO_ENABLED=0 go run ./cmd/execution
```

HTTP routes added by `execution`:

- `POST /v1/carrier/events`
- `GET /v1/carrier/codeagent/health`
- `GET /v1/carrier/codeagent/version`
- `POST /v1/carrier/codeagent/run`

## Current scope

- Provider and listing catalogs are durably backed by Postgres and seeded on bootstrap.
- Settlement and execution now speak to real Fiber and Carrier interfaces, and settlement keeps local funding records when `DATABASE_URL` or `SETTLEMENT_DATABASE_URL` is configured.
- IAM, RFQ/bidding flows, dispute backoffice, and ledger-grade reconciliation are still skeletal and are not yet release-complete.
