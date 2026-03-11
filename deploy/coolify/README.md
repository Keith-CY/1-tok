# Coolify Deployment Notes

This repository is structured so Coolify can manage each Go service as an independent container build using `Dockerfile.go-service`.

## Suggested services

- `api-gateway`
- `iam`
- `marketplace`
- `settlement`
- `risk`
- `execution`
- `notification`
- `postgres`
- `nats`
- `web`

## Build settings for Go services

- Build context: repository root
- Dockerfile: `Dockerfile.go-service`
- Build arg `SERVICE`: one of `api-gateway`, `iam`, `marketplace`, `settlement`, `risk`, `execution`, `notification`

## Runtime settings

- Keep all Go services on the same internal network.
- Expose only `api-gateway`, `web`, and optionally `iam` externally.
- Mount persistent volume for `postgres`.
- Enable JetStream for `nats`.

## Minimum environment variables

- `API_GATEWAY_ADDR=:8080`
- `IAM_ADDR=:8081`
- `MARKETPLACE_ADDR=:8082`
- `SETTLEMENT_ADDR=:8083`
- `RISK_ADDR=:8084`
- `EXECUTION_ADDR=:8085`
- `NOTIFICATION_ADDR=:8086`

## Next steps

- Add per-service Postgres DSN and NATS URLs.
- Move in-memory repositories to schema-scoped Postgres adapters.
- Wire `execution` service hooks to Carrier-compatible lifecycle events.
- Wire `settlement` service to Fiber adapters.
