# Coolify Deployment Notes

This repository is structured so Coolify can manage each Go service as an independent container build using `Dockerfile.go-service`.

## Suggested services

- `bootstrap`
- `api-gateway`
- `iam`
- `marketplace`
- `settlement`
- `settlement-reconciler`
- `risk`
- `execution`
- `notification`
- `fnn` (optional infra service; current app smoke still uses `mock-fiber`)
- `fiber-adapter` (optional bridge service if you want local `tip.*` / `withdrawal.*` translation onto raw `fnn`)
- `fnn2` (optional second raw FNN node if you want to rehearse channel bootstrap and real payment routing)
- `postgres`
- `redis`
- `nats`
- `web`

`e2e-runner` is not a long-lived production service. It exists only for Dockerized end-to-end validation and CI.

## Build settings for Go services

- Build context: repository root
- Dockerfile: `Dockerfile.go-service`
- Build arg `SERVICE`: one of `bootstrap`, `api-gateway`, `iam`, `marketplace`, `settlement`, `settlement-reconciler`, `risk`, `execution`, `notification`
- Build arg `SERVICE`: one of `bootstrap`, `api-gateway`, `iam`, `marketplace`, `settlement`, `settlement-reconciler`, `risk`, `execution`, `notification`, `fiber-adapter`

## Build settings for optional `fnn`

- Build context: repository root
- Dockerfile: `deploy/fnn/Dockerfile`
- Build args:
  - `FNN_VERSION`
  - `FNN_ASSET`
  - `FNN_ASSET_SHA256`

## Runtime settings

- Keep all Go services on the same internal network.
- Expose only `api-gateway`, `web`, and optionally `iam` externally.
- Mount persistent volume for `postgres`.
- Enable JetStream for `nats`.
- Run `bootstrap` as a one-shot job before `iam`, `api-gateway`, `settlement`, and `settlement-reconciler`.
- Run `settlement-reconciler` as a long-lived worker on the same internal network as `postgres` and `settlement`.
- Keep `redis` on the same internal network as `iam` and `api-gateway`; the current production rate limiting depends on it.
- If you also want raw Fiber node infra under Coolify, add the optional `fnn` service using [compose.fnn.yaml](../../compose.fnn.yaml) as the reference shape.
- If you want to rehearse real raw-FNN payment routing, add both `fnn` and `fnn2` from [compose.fnn.yaml](../../compose.fnn.yaml), then run the dual-node smoke once those nodes are funded.
- If you want a bridge between existing `tip.*` / `withdrawal.*` calls and raw `fnn`, add the optional `fiber-adapter` service from the same [compose.fnn.yaml](../../compose.fnn.yaml) overlay.

## Minimum environment variables

- `DATABASE_URL=postgres://...`
- `API_GATEWAY_ADDR=:8080`
- `IAM_ADDR=:8081`
- `MARKETPLACE_ADDR=:8082`
- `SETTLEMENT_ADDR=:8083`
- `RISK_ADDR=:8084`
- `EXECUTION_ADDR=:8085`
- `NOTIFICATION_ADDR=:8086`
- `REDIS_URL=redis://...`
- `RATE_LIMIT_ENFORCE=true`
- `RATE_LIMIT_TRUST_PROXY=true`
- `RATE_LIMIT_TRUSTED_HOPS=1`
- `ONE_TOK_REQUIRE_PERSISTENCE=true`
- `ONE_TOK_REQUIRE_BOOTSTRAP=true`
- `ONE_TOK_REQUIRE_EXTERNALS=true`
- `SENTRY_DSN`
- `NEXT_PUBLIC_SENTRY_DSN`
- `SENTRY_ENVIRONMENT`
- `SENTRY_RELEASE`
- `SENTRY_TRACES_SAMPLE_RATE`
- `API_GATEWAY_EXECUTION_TOKEN` or `API_GATEWAY_EXECUTION_TOKENS`
- `EXECUTION_EVENT_TOKEN` or `EXECUTION_EVENT_TOKENS`
- `EXECUTION_GATEWAY_TOKEN` or `EXECUTION_GATEWAY_TOKENS`
- `SETTLEMENT_SERVICE_TOKEN` or `SETTLEMENT_SERVICE_TOKENS`
- `FIBER_RPC_URL`
- `FIBER_APP_ID`
- `FIBER_HMAC_SECRET`
- `CARRIER_GATEWAY_URL`
- `CARRIER_GATEWAY_API_TOKEN`
- `SETTLEMENT_RECONCILER_INTERVAL=30s`

Optional `fnn` service env:

- `FNN_VERSION`
- `FNN_ASSET`
- `FNN_ASSET_SHA256`
- `FIBER_SECRET_KEY_PASSWORD`
- `FNN_CKB_RPC_URL`
- `FNN_PUBLISHED_RPC_PORT`
- `FNN_PUBLISHED_P2P_PORT`

Optional `fiber-adapter` service env:

- `FIBER_ADAPTER_ADDR`
- `FNN_INVOICE_RPC_URL`
- `FNN_PAYER_RPC_URL`

## Next steps

- Provide real preproduction or production `Fiber` and `Carrier` endpoints and credentials.
- Configure real Sentry alert rules and notification routing for the production project.
- Run `bun run release:external-deps-smoke` from the target deployment environment.
- Archive the resulting `release-manifest.json` as the deployment evidence package.
- Complete the remaining `P0` items in [production-launch-checklist.md](../../docs/production-launch-checklist.md).
- Implement and upstream the Carrier work described in [carrier-first-class-design.md](../../docs/carrier-first-class-design.md) and [carrier-pr-support.md](../../docs/carrier-pr-support.md).
