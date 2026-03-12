# 1-tok

`1-tok` is a monorepo for the Agent marketplace and settlement platform described in the approved plan. This first implementation pass establishes:

- a shared Go domain model for orders, milestones, usage charges, disputes, and credit decisions
- a JSON HTTP gateway that exposes the core marketplace and settlement flows
- service entrypoints for `iam`, `marketplace`, `settlement`, `risk`, `execution`, and `notification`
- Fiber JSON-RPC integration for invoice creation, invoice status, withdrawal quote, and withdrawal request
- Carrier gateway integration for remote codeagent health, version, and run control-plane calls
- Local `mock-carrier` support for release smoke and compose-based rehearsal
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
export RELEASE_SMOKE_SETTLEMENT_SERVICE_TOKEN='replace-me'
export RELEASE_SMOKE_EXECUTION_BASE_URL='http://127.0.0.1:8085'
export RELEASE_SMOKE_EXECUTION_EVENT_TOKEN='replace-me'
bun run release:smoke
```

The smoke command defaults to the smallest cross-service path: create an order, drive a milestone-ready execution event, create an invoice, sync settlement state, and assert invoice funding records. Optional probes can be enabled with:

```bash
export RELEASE_SMOKE_INCLUDE_WITHDRAWAL=true
export RELEASE_SMOKE_INCLUDE_CARRIER_PROBE=true
```

### Portal release smoke

```bash
export RELEASE_PORTAL_SMOKE_WEB_BASE_URL='http://127.0.0.1:3000'
export RELEASE_PORTAL_SMOKE_API_BASE_URL='http://127.0.0.1:8080'
export RELEASE_PORTAL_SMOKE_IAM_BASE_URL='http://127.0.0.1:8081'
export RELEASE_PORTAL_SMOKE_EXECUTION_BASE_URL='http://127.0.0.1:8085'
export RELEASE_PORTAL_SMOKE_EXECUTION_EVENT_TOKEN='replace-me'
bun run release:portal-smoke
```

This smoke exercises the web login shell plus buyer/provider/ops form flows over real HTTP cookies, then settles the awarded order through the execution service's carrier event path:

- buyer login and RFQ publish
- provider login and bid submit
- buyer award of the submitted bid
- ops credit review
- ops dispute resolution after an API-seeded dispute

### Local portal release smoke

```bash
bun run release:portal-local-smoke
```

This script builds the web app, starts local `iam`, `api-gateway`, `execution`, and the built Next standalone server on dedicated localhost ports, wires the execution service token pair, waits for readiness, and then runs the HTTP-level portal smoke against those real services.

### Local services release smoke

```bash
bun run release:services-local-smoke
```

This script starts local `mock-fiber`, `mock-carrier`, `api-gateway`, `settlement`, and `execution` processes on dedicated localhost ports, wires the required service tokens, and then runs the cross-service `release:smoke` flow with invoice settlement, provider withdrawal, and carrier codeagent probe against those real HTTP services.

### Full local release smoke

```bash
bun run release:full-local-smoke
```

This script starts local `mock-fiber`, `mock-carrier`, `iam`, `api-gateway`, `settlement`, `execution`, and the built Next standalone server, wires IAM plus service-token auth, and then runs both `release:smoke` with withdrawal and carrier-probe coverage and `release:portal-smoke` against the same stack.

### Full persisted local release smoke

```bash
bun run release:full-persisted-local-smoke
```

This script boots an isolated `postgres:16-alpine` container on `127.0.0.1:${POSTGRES_PORT:-15432}`, wires `iam`, `api-gateway`, and `settlement` to that database, and then runs the same full-stack smoke sequence, including withdrawal and carrier probe coverage, against persisted repositories instead of the in-memory defaults.

### External dependency release smoke

```bash
export DEPENDENCY_FIBER_RPC_URL='https://fiber.example/rpc'
export DEPENDENCY_FIBER_APP_ID='app_live'
export DEPENDENCY_FIBER_HMAC_SECRET='replace-me'
export DEPENDENCY_CARRIER_GATEWAY_URL='https://carrier.example'
export DEPENDENCY_CARRIER_GATEWAY_API_TOKEN='replace-me'
bun run release:external-deps-smoke
```

This script boots an isolated Postgres container plus local `iam`, `api-gateway`, `settlement`, `execution`, and standalone `web`, but points settlement and execution at externally supplied Fiber and Carrier endpoints. For local verification of the script shape you can set `USE_LOCAL_FIBER_MOCK=true` and `USE_LOCAL_CARRIER_MOCK=true`.

### Compose release smoke

```bash
bun run release:compose-smoke
```

This script builds and boots the compose stack with `postgres`, `mock-fiber`, `mock-carrier`, `iam`, `api-gateway`, `settlement`, `execution`, and `web`, then runs both release smoke commands against the published localhost ports before tearing the stack down.

### Contracts tests

```bash
bun run test:contracts
```

### API gateway

```bash
export API_GATEWAY_EXECUTION_TOKEN='replace-me'
CGO_ENABLED=0 go run ./cmd/api-gateway
```

To enable persistence locally:

```bash
export DATABASE_URL='postgres://onetok:onetok@127.0.0.1:5432/onetok?sslmode=disable'
export NATS_URL='nats://127.0.0.1:4222'
export API_GATEWAY_EXECUTION_TOKEN='replace-me'
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
export SETTLEMENT_SERVICE_TOKEN='replace-me'
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

When `SETTLEMENT_SERVICE_TOKEN` is set, invoice creation, invoice status refresh, and settled-feed sync require the `X-One-Tok-Service-Token` header.

### Execution service with Carrier

```bash
export API_GATEWAY_UPSTREAM='http://127.0.0.1:8080'
export EXECUTION_EVENT_TOKEN='replace-me'
export EXECUTION_GATEWAY_TOKEN='replace-me'
export CARRIER_GATEWAY_URL='http://127.0.0.1:8787'
export CARRIER_GATEWAY_API_TOKEN='test-gateway-token'
CGO_ENABLED=0 go run ./cmd/execution
```

HTTP routes added by `execution`:

- `POST /v1/carrier/events`
- `GET /v1/carrier/codeagent/health`
- `GET /v1/carrier/codeagent/version`
- `POST /v1/carrier/codeagent/run`

When `EXECUTION_EVENT_TOKEN` and `API_GATEWAY_EXECUTION_TOKEN` are set to the same value, carrier event ingestion and gateway settlement/usage mutations become service-token protected instead of public.

### IAM service with persisted sessions

```bash
export IAM_DATABASE_URL='postgres://onetok:onetok@127.0.0.1:5432/onetok?sslmode=disable'
CGO_ENABLED=0 go run ./cmd/iam
```

HTTP routes added by `iam`:

- `POST /v1/signup`
- `POST /v1/sessions`
- `POST /v1/logout`
- `GET /v1/me`
- `GET /v1/roles`

### Web login shell

```bash
export NEXT_PUBLIC_API_BASE_URL='http://127.0.0.1:8080'
export IAM_BASE_URL='http://127.0.0.1:8081'
bun --cwd apps/web dev
```

The web app now exposes `/login`, `POST /auth/login`, and `POST /auth/logout`. The bearer token returned by IAM is stored in an `HttpOnly` cookie owned by the Next server.

Buyer, provider, and ops portal pages now require a matching IAM membership and redirect unauthenticated requests to `/login?next=...`. Their server-side fetches also forward the bearer token, and the authenticated portal pages stop falling back to demo marketplace or settlement lists.

### Membership-aware gateway and settlement

When `IAM_UPSTREAM` is configured for `api-gateway` and `settlement`, the platform starts binding selected routes to authenticated memberships instead of trusting caller-supplied org IDs:

- `POST /api/v1/orders` derives `buyerOrgId` from the authenticated buyer membership
- `GET /v1/funding-records` scopes provider visibility to the authenticated provider membership, with ops memberships retaining global access
- `POST /v1/withdrawals`, `POST /v1/withdrawals/quote`, and `GET /v1/withdrawals/status` can derive `providerOrgId` from the authenticated provider membership

## Current scope

- Provider and listing catalogs are durably backed by Postgres and seeded on bootstrap.
- Settlement and execution now speak to real Fiber and Carrier interfaces, and settlement keeps local funding records when `DATABASE_URL` or `SETTLEMENT_DATABASE_URL` is configured.
- IAM now supports persisted `signup`, `session`, and `me` flows when `DATABASE_URL` or `IAM_DATABASE_URL` is configured, but full gateway/web enforcement is still not wired.
- Gateway order creation and settlement funding-record reads can now honor authenticated memberships when `IAM_UPSTREAM` is configured, but the rest of the platform still has unauthenticated paths.
- Execution event ingestion and gateway settlement/usage mutations can now be bound to a shared service token, and the local full-stack smoke covers that protected path end to end. Real external Carrier/Fiber release rehearsals still need more work.
- RFQ publishing, bidding, award, credit review, and dispute resolution now have live web entry points, and ops-only dispute/credit routes are membership-gated when `IAM_UPSTREAM` is configured.
- Ledger-grade reconciliation, broad read-path authorization, and an end-to-end release rehearsal against real external Carrier/Fiber dependencies still need more work before a production release claim would be accurate.
