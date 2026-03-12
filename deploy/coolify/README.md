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
- `postgres`
- `nats`
- `web`

## Build settings for Go services

- Build context: repository root
- Dockerfile: `Dockerfile.go-service`
- Build arg `SERVICE`: one of `bootstrap`, `api-gateway`, `iam`, `marketplace`, `settlement`, `settlement-reconciler`, `risk`, `execution`, `notification`

## Runtime settings

- Keep all Go services on the same internal network.
- Expose only `api-gateway`, `web`, and optionally `iam` externally.
- Mount persistent volume for `postgres`.
- Enable JetStream for `nats`.
- Run `bootstrap` as a one-shot job before `iam`, `api-gateway`, `settlement`, and `settlement-reconciler`.
- Run `settlement-reconciler` as a long-lived worker on the same internal network as `postgres` and `settlement`.

## Minimum environment variables

- `DATABASE_URL=postgres://...`
- `API_GATEWAY_ADDR=:8080`
- `IAM_ADDR=:8081`
- `MARKETPLACE_ADDR=:8082`
- `SETTLEMENT_ADDR=:8083`
- `RISK_ADDR=:8084`
- `EXECUTION_ADDR=:8085`
- `NOTIFICATION_ADDR=:8086`
- `ONE_TOK_REQUIRE_PERSISTENCE=true`
- `ONE_TOK_REQUIRE_BOOTSTRAP=true`
- `ONE_TOK_REQUIRE_EXTERNALS=true`
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

## Next steps

- Provide real preproduction or production `Fiber` and `Carrier` endpoints and credentials.
- Run `bun run release:external-deps-smoke` from the target deployment environment.
- Archive the resulting `release-manifest.json` as the deployment evidence package.
