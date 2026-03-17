# 1-tok

**Agent Runtime Marketplace** — a platform for discovering, procuring, and managing agent execution services.

## Features

### Core Marketplace
- **RFQ → Bid → Award → Order** lifecycle with milestone-based settlement
- **Listing search** with query, category, tag, and price range filters
- **Provider ratings** (1-5 stars) with duplicate prevention
- **RFQ-level messaging** between buyers and providers during bid phase
- **Credit decision engine** for funding mode selection

### Anti-Fraud
- **Layer 2: Usage Proof Signatures** — HMAC-SHA256 signed usage reports from Carriers
- **Layer 3: Summary Reconciliation** — deviation detection on milestone settlement

### Carrier Integration
- **Async Execution Protocol** — binding, job state machine (pending → running → completed/failed/cancelled), heartbeat liveness
- **8 HTTP endpoints** for carrier operations (bind, create job, start, complete, fail, progress, heartbeat)

### Notifications
- **8 event types** across the full lifecycle (order.created, milestone.settled, dispute.opened/resolved, rfq.awarded, order.completed, order.rated, budget_wall.hit)
- **Webhook delivery** with HMAC-SHA256 signature verification

### Discord Bot
- Slash commands: `/listings`, `/order-status`, `/rfq-status`, `/bids`
- Ed25519 signature verification
- Color-coded order status embeds

### Infrastructure
- IAM with session hashing, role-based access, rate limiting
- Postgres repositories with context propagation
- NATS event publishing with infinite reconnect
- `/livez` and `/readyz` health endpoints
- Request timeout middleware, CORS, access logging


## API Endpoints (64)

| Method | Path | Description |
|--------|------|-------------|
| GET | /api/v1/providers | List providers |
| GET | /api/v1/providers/:id | Get provider with rating |
| GET | /api/v1/listings/:id | Get listing |
| GET | /api/v1/listings | Search listings (q, category, tag, minPrice, maxPrice) |
| GET | /api/v1/rfqs | List RFQs |
| GET | /api/v1/rfqs/:id | Get RFQ |
| POST | /api/v1/rfqs | Create RFQ |
| GET | /api/v1/rfqs/:id/bids | List bids on RFQ |
| POST | /api/v1/rfqs/:id/bids | Create bid |
| POST | /api/v1/rfqs/:id/award | Award RFQ |
| GET | /api/v1/rfqs/:id/messages | List RFQ messages |
| POST | /api/v1/rfqs/:id/messages | Create RFQ message |
| GET | /api/v1/orders | List orders |
| GET | /api/v1/orders/:id | Get order |
| POST | /api/v1/orders | Create order |
| POST | /api/v1/orders/:id/milestones/:mid/settle | Settle milestone |
| POST | /api/v1/orders/:id/milestones/:mid/usage | Record usage charge |
| POST | /api/v1/orders/:id/milestones/:mid/disputes | Open dispute |
| POST | /api/v1/orders/:id/rating | Rate order |
| GET | /api/v1/orders/:id/messages | List order messages |
| POST | /api/v1/orders/:id/milestones/:mid/bind-carrier | Bind carrier |
| POST | /api/v1/orders/:id/milestones/:mid/jobs | Create job |
| GET | /api/v1/jobs/:id | Get job |
| PATCH | /api/v1/jobs/:id/start | Start job |
| PATCH | /api/v1/jobs/:id/complete | Complete job |
| PATCH | /api/v1/jobs/:id/fail | Fail job |
| POST | /api/v1/jobs/:id/progress | Update progress |
| POST | /api/v1/jobs/:id/heartbeat | Carrier heartbeat |
| GET | /api/v1/disputes | List disputes |
| GET | /api/v1/disputes/:id | Get dispute |
| POST | /api/v1/disputes/:id/resolve | Resolve dispute |
| POST | /api/v1/credits/decision | Credit decision |
| POST | /api/v1/messages | Create message |
## Layout

- `cmd/*`: service binaries
- `internal/core`: order, settlement, dispute, and credit domain logic
- `internal/gateway`: integrated HTTP API surface for marketplace flows
- `internal/services/*`: service-specific HTTP entrypoints
- `packages/contracts`: shared TypeScript types and dashboard helpers
- `apps/web`: Next.js portal shell

## Run

Go service builds now target Go `1.25`.

## CI

GitHub Actions now runs two lanes on every `push` and `pull_request`:

- `Unit And Coverage`: Go unit tests plus Bun tests for `apps/web` and `packages/contracts`, with a merged coverage summary artifact and job summary
- `Integration Smoke`: a Docker-only end-to-end path that boots `postgres`, `redis`, `nats`, `fnn`, `fiber-adapter`, `mock-fiber`, `mock-carrier`, `mock-sentry`, `iam`, `api-gateway`, `marketplace`, `settlement`, `settlement-reconciler`, `execution`, `web`, and a dedicated `e2e-runner`
- `Docker FNN reference test`: static contract checks for the `fnn` overlay, `e2e-runner`, and release scripts that support that path

For pull requests opened from the same repository, the `Report` job upserts a sticky PR comment from `github-actions[bot]` with the latest unit/integration status and coverage table. Each new push to the PR updates that same comment in place.

Local entrypoints match the workflow:

```bash
bun run test:coverage
bun run test:integration
```

## Alpha portal UX audit (governance)

The portal-facing consistency checks for quick-filters and empty-state actions can be run locally with:

```bash
bun run alpha:ux-audit
bun run alpha:ux-audit:strict
bun run portal:check
```

- `alpha:ux-audit` runs a baseline consistency scan and writes `alpha-portal-ux-audit.json`.
- `alpha:ux-audit:strict` treats non-canonical EmptyState action targets as hard failures (for CI or gated pre-merge checks).

This command scans `apps/web/app/{buyer,provider,ops}` and validates:
- quick chip accessibility marker (`aria-current` with `chipClass`)
- `EmptyState` action label/href presence
- hash-only action links
- optionally surfaced non-canonical action targets

It outputs a baseline JSON report in `alpha-portal-ux-audit.json`.

The Docker-only end-to-end command can also be run directly:

```bash
bun run release:compose-e2e
```

By default it uses:

- `FNN_VERSION=v0.6.1`
- `FNN_ASSET=fnn_v0.6.1-x86_64-linux-portable.tar.gz`
- `FNN_ASSET_SHA256=8f9a69361f662438fa1fc29ddc668192810b13021536ebd1101c84dc0cfa330f`
- `FIBER_SECRET_KEY_PASSWORD=local-fnn-dev-password`

You can override those with env vars if you need a different FNN build or password.

This path is fully Dockerized: the services boot inside Docker, and the smoke itself runs from the `e2e-runner` container over the Docker network. The runner now executes four checks:

- `release-fnn-adapter-smoke`: verifies that the local `fiber-adapter` can translate `tip.create`, `tip.status`, and `withdrawal.quote` to raw `fnn`
- the existing marketplace/settlement/portal smoke flow, which still uses `mock-fiber` for commit-safe full business coverage
- `release-abuse-smoke`: verifies IAM login throttling through Redis-backed limits and checks that a rate-limit event reaches `mock-sentry`
- the stack itself now boots with production-style Sentry env wiring and rate-limit enforcement on `iam` and `api-gateway`

That split is deliberate. The repo now has a real raw-`fnn` adapter path under CI, but the full paid-settlement business flow still stays on `mock-fiber` until a dual-node, funded FNN environment is available.

There is now also a separate dual-node raw-FNN smoke command:

- `release-fnn-dual-node-smoke`: bootstraps `node_info -> connect_peer -> open_channel -> accept_channel -> list_channels`, then runs adapter-backed payment smoke against `fnn` and `fnn2`

This command is not part of the default every-commit CI lane yet. It assumes the two FNN nodes are funded enough to open a channel and route a payment.

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

### Abuse smoke

```bash
export RELEASE_ABUSE_IAM_BASE_URL='http://127.0.0.1:8081'
export RELEASE_ABUSE_SENTRY_BASE_URL='http://127.0.0.1:8092'
bun run release:abuse-smoke
```

This smoke signs up a buyer, floods the IAM login path until the Redis-backed limiter returns `429`, and then confirms that at least one rate-limit event has reached the configured Sentry-compatible endpoint.

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

This script now preflights the external Fiber and Carrier endpoints before starting the local stack, then boots an isolated Postgres container plus local `iam`, `api-gateway`, `settlement`, `execution`, and standalone `web`. If the external dependencies expose a nonstandard health route, set `DEPENDENCY_FIBER_HEALTHCHECK_URL` and `DEPENDENCY_CARRIER_HEALTHCHECK_URL`. Set `RELEASE_ARTIFACT_DIR` if you want to persist the generated `external-preflight.json`, `release-smoke.json`, `release-portal-smoke.json`, and aggregated `release-manifest.json` artifacts in a specific location. For local verification of the script shape you can set `USE_LOCAL_FIBER_MOCK=true` and `USE_LOCAL_CARRIER_MOCK=true`.

### Compose release smoke

```bash
bun run release:compose-smoke
```

This script builds and boots the compose stack with `postgres`, `redis`, `mock-fiber`, `mock-carrier`, `bootstrap`, `iam`, `api-gateway`, `settlement`, `settlement-reconciler`, `execution`, and `web`, then runs both release smoke commands against the published localhost ports before tearing the stack down.

### Compose Docker-only end-to-end

```bash
bun run release:compose-e2e
```

This is the main Docker-only end-to-end path. It layers [compose.fnn.yaml](./compose.fnn.yaml) and [compose.e2e.yaml](./compose.e2e.yaml) on top of [compose.yaml](./compose.yaml), boots the full stack including `marketplace`, `redis`, `mock-sentry`, `fnn`, `fiber-adapter`, `mock-carrier`, and `mock-fiber`, and then runs four checks from the `e2e-runner` container inside the Docker network:

- raw `fnn` adapter smoke via `release-fnn-adapter-smoke`
- backend settlement smoke via `release-smoke`
- portal workflow smoke via `release-portal-smoke`
- abuse and Sentry smoke via `release-abuse-smoke`

### Compose + FNN release smoke

```bash
export FNN_ASSET_SHA256='replace-me-with-official-sha256'
export FIBER_SECRET_KEY_PASSWORD='replace-me'
bun run release:compose-fnn-smoke
```

This script layers [compose.fnn.yaml](./compose.fnn.yaml) on top of the base compose stack, boots a real Dockerized `fnn` node using the same general image/entrypoint pattern as `fiber-link`, and makes the local `fiber-adapter` service available for raw `fnn` translation checks. The commit-safe full business path still points to `mock-fiber`; only the adapter smoke talks to raw `fnn` today.

### Compose + dual-node FNN payment smoke

```bash
export FNN_ASSET_SHA256='replace-me-with-official-sha256'
export FIBER_SECRET_KEY_PASSWORD='replace-me'
bun run release:compose-fnn-dual-node-smoke
```

This script boots `fnn`, `fnn2`, `fiber-adapter`, and an `e2e-runner` in the same Docker network, then runs `release-fnn-dual-node-smoke` from inside Docker. It is the first repo-local path that is shaped for real raw-FNN channel bootstrap plus adapter-backed payment. It is still not wired into default CI because fresh nodes need enough funds to open the channel and complete the payment.

The wrapper now includes a first-cut CKB funding preflight for fresh nodes:

- derive payer and invoice top-up addresses from `node_info` when you do not provide them explicitly
- query CKB balances from `FNN_CKB_RPC_URL` via `get_cells`
- request faucet top-ups when balances are below the required threshold
- wait for the balance to reach the channel bootstrap threshold before invoking `release-fnn-dual-node-smoke`

Useful overrides:

```bash
export FNN_DUAL_CKB_TOPUP_ADDRESS='ckt1...'
export FNN_DUAL_INVOICE_CKB_TOPUP_ADDRESS='ckt1...'
export FNN_DUAL_TOPUP_INVOICE_NODE_CKB=1
export FNN_DUAL_CHANNEL_FUNDING_AMOUNT=10000000000
export RELEASE_FNN_DUAL_ACCEPT_FUNDING_AMOUNT=0x2540be400
```

This path is still not part of default CI, and I have not claimed a fresh-node live pass in this repo yet. The new wrapper is the first funded-bootstrap slice, not final proof.

As of `2026-03-13`, this command has been live-verified once from this repo against fresh Dockerized `fnn` / `fnn2` nodes plus testnet faucet/RPC. The successful run produced:

- `channelTemporaryId=0x022e4074deb8efa1ab9d04fae59bcc99a65641a078e4e6ca5c1418113c206c1e`
- a valid raw-FNN invoice under `adapter.invoice`
- `quoteValid=true`
- `withdrawalId=0xd77a0b1baa247e3028844180c0ebee4adc0a9e8e8bdd9ad997efe4f998529165`

### Carrier support contract

The authoritative first-class Carrier design is captured in [carrier-first-class-design.md](./docs/carrier-first-class-design.md).

The upstream-facing implementation checklist derived from that design is captured in [carrier-pr-support.md](./docs/carrier-pr-support.md).

Persisted release paths now force `ONE_TOK_REQUIRE_PERSISTENCE=true`, so `iam`, `api-gateway`, and `settlement` fail fast instead of silently falling back to in-memory stores when database configuration is missing or broken.

Full-stack release paths also force `ONE_TOK_REQUIRE_EXTERNALS=true`, so `api-gateway`, `execution`, and `settlement` refuse to start if `IAM`, `Carrier`, `Fiber`, or internal service-token wiring is missing.

Persisted release paths now also run a dedicated `cmd/bootstrap` job first and force `ONE_TOK_REQUIRE_BOOTSTRAP=true`, so the database schema plus default catalog must be initialized before `iam`, `api-gateway`, or `settlement` start.

The release harness now scopes internal secrets by edge: one token for `execution -> api-gateway`, and a separate token for settlement internal routes.

Receivers can now accept comma-separated rotated service tokens through `API_GATEWAY_EXECUTION_TOKENS`, `EXECUTION_EVENT_TOKENS`, `EXECUTION_GATEWAY_TOKENS`, and `SETTLEMENT_SERVICE_TOKENS`, while senders continue to use the first token as the current outbound value.

Compose now also includes a dedicated `settlement-reconciler` worker, which polls unsettled invoices and nonterminal withdrawals directly from Postgres/Fiber instead of relying on manual sync requests.

## Production Release Status

As of `2026-03-12`, the repo-local productionization work is largely complete. The main remaining release blocker is not missing application code inside this repository; it is the absence of real external `Fiber` and `Carrier` preproduction or production credentials in the current environment.

The fastest path to final signoff is:

```bash
export DEPENDENCY_FIBER_RPC_URL='https://fiber.example/rpc'
export DEPENDENCY_FIBER_APP_ID='app_live'
export DEPENDENCY_FIBER_HMAC_SECRET='replace-me'
export DEPENDENCY_CARRIER_GATEWAY_URL='https://carrier.example'
export DEPENDENCY_CARRIER_GATEWAY_API_TOKEN='replace-me'
export RELEASE_ARTIFACT_DIR="$PWD/.release-artifacts/$(date +%Y%m%d-%H%M%S)"
bun run release:external-deps-smoke
```

That command now produces:

- `external-preflight.json`
- `release-smoke.json`
- `release-portal-smoke.json`
- `release-manifest.json`

The aggregated manifest is the handoff artifact to review before claiming a real production release.

For a more detailed release handoff and blocker summary, see [docs/production-release-status.md](./docs/production-release-status.md).

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
CGO_ENABLED=0 go run ./cmd/bootstrap
export ONE_TOK_REQUIRE_BOOTSTRAP='true'
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
- `GET /v1/settled-feed`
- `GET /v1/withdrawals/status`

When `SETTLEMENT_SERVICE_TOKEN` is set, invoice creation, invoice status refresh, and settled-feed sync require the `X-One-Tok-Service-Token` header.

### Settlement reconciler

```bash
export DATABASE_URL='postgres://onetok:onetok@127.0.0.1:5432/onetok?sslmode=disable'
export FIBER_RPC_URL='http://127.0.0.1:3000/rpc'
export FIBER_APP_ID='app_1'
export FIBER_HMAC_SECRET='replace-me'
export ONE_TOK_REQUIRE_PERSISTENCE='true'
export ONE_TOK_REQUIRE_BOOTSTRAP='true'
CGO_ENABLED=0 go run ./cmd/settlement-reconciler
```

Set `SETTLEMENT_RECONCILER_ONCE=true` for a one-shot reconciliation run, or `SETTLEMENT_RECONCILER_INTERVAL=30s` to control the polling interval for the long-running worker.

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

`execution` uses `EXECUTION_EVENT_TOKEN` or `EXECUTION_EVENT_TOKENS` for inbound carrier event authorization, and forwards the primary token from `EXECUTION_GATEWAY_TOKEN` or `EXECUTION_GATEWAY_TOKENS` to `api-gateway`. Receivers can accept comma-separated rotated token sets; senders use the first token as the active outbound credential.

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
- A dedicated `bootstrap` job initializes schema and seed data, and persisted services can now refuse startup unless the database was bootstrapped first.
- Settlement and execution speak to real Fiber and Carrier interfaces, and settlement keeps durable funding records plus a dedicated reconciliation worker when `DATABASE_URL` or `SETTLEMENT_DATABASE_URL` is configured.
- IAM supports persisted `signup`, `session`, `logout`, and `me` flows, and the web portals now use real session-backed buyer, provider, and ops entry points.
- Marketplace RFQ publishing, bidding, award, credit review, dispute resolution, and portal submission flows are wired end to end in the local release harnesses.
- Internal service-token boundaries are enforced and can accept rotated comma-separated token sets for `api-gateway`, `execution`, and `settlement`.
- Local release verification now covers service-only, portal-only, full local, full persisted local, compose, and external-dependency rehearsals, with persisted evidence artifacts and an aggregated `release-manifest.json`.
- The primary remaining blocker to a real production release claim is a successful run of `bun run release:external-deps-smoke` against real external `Fiber` and `Carrier` environments using live credentials and endpoints.

## Webhook Signature Verification

Webhooks include an HMAC-SHA256 signature in the `X-1Tok-Signature` header.

Verify with:

```python
import hmac, hashlib

def verify(secret: str, body: bytes, signature: str) -> bool:
    expected = hmac.new(secret.encode(), body, hashlib.sha256).hexdigest()
    return hmac.compare_digest(expected, signature)
```

```go
mac := hmac.New(sha256.New, []byte(secret))
mac.Write(body)
expected := hex.EncodeToString(mac.Sum(nil))
valid := hmac.Equal([]byte(expected), []byte(signature))
```

## Quick Start

```bash
# Start all services
docker compose up -d

# Create an RFQ
curl -X POST http://localhost:8080/api/v1/rfqs \
  -H "Content-Type: application/json" \
  -d '{"title":"Agent triage","category":"agent-ops","scope":"Investigate failures","budgetCents":5000,"responseDeadlineAt":"2026-04-01T00:00:00Z"}'

# Search listings
curl "http://localhost:8080/api/v1/listings?q=agent&category=agent-ops"

# Check marketplace stats
curl http://localhost:8080/api/v1/stats

# Register a webhook
curl -X POST http://localhost:8080/api/v1/webhooks \
  -H "Content-Type: application/json" \
  -d '{"target":"org_buyer","url":"https://example.com/webhook"}'
```
