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
- `carrier-daemon`
- `carrier-gateway`
- `remote-vps`
- `fnn`
- `fnn2`
- `provider-fnn`
- `fiber-adapter`
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

## Demo environment shape

The current demo-ready topology assumes one fixed remote environment under Coolify with:

- one buyer account
- one provider account
- one ops account
- one active provider carrier binding
- one active provider settlement binding
- one platform treasury payer path over `fnn2`
- one provider-owned settlement node over `provider-fnn`

For the current live marketplace demo, treat `fnn`, `fnn2`, `provider-fnn`, `fiber-adapter`, `carrier-daemon`, `carrier-gateway`, and `remote-vps` as part of the standard stack rather than optional extras.

Use the repo’s demo control-plane commands from that environment:

```bash
bun run release:demo:prepare
bun run release:demo:verify
```

The ops home page then reflects the same verdict through `GET /api/v1/ops/demo/status`.

## Runtime settings

- Keep all Go services on the same internal network.
- Expose only `api-gateway`, `web`, and optionally `iam` externally.
- Mount persistent volume for `postgres`.
- Enable JetStream for `nats`.
- Run `bootstrap` as a one-shot job before `iam`, `api-gateway`, `settlement`, and `settlement-reconciler`.
- Run `settlement-reconciler` as a long-lived worker on the same internal network as `postgres` and `settlement`.
- Keep `redis` on the same internal network as `iam` and `api-gateway`; the current production rate limiting depends on it.
- Use [compose.fnn.yaml](../../compose.fnn.yaml) as the reference shape for `fnn`, `fnn2`, `provider-fnn`, and `fiber-adapter`.
- Use [compose.usdi-e2e.yaml](../../compose.usdi-e2e.yaml) as the reference shape for `carrier-daemon`, `carrier-gateway`, and `remote-vps`.

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

- Set the fixed demo actor IDs and credentials from [env.md](../../docs/env.md).
- Run `bun run release:demo:prepare` once from the deployed environment to ensure bindings, prefund, and provider liquidity.
- Run `bun run release:demo:verify` before each live session.
- Follow [demo-environment.md](../../docs/demo-environment.md) and [demo-runbook.md](../../docs/demo-runbook.md) for operator-facing steps.
