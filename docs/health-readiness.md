# Health and Readiness Contract

This document standardizes endpoint semantics across HTTP services.

## Endpoints

- `GET /healthz`
  - Liveness only.
  - Returns `200` when process can serve HTTP.

- `GET /readyz`
  - Readiness gate for dependencies.
  - Returns `200` when the service is ready for traffic.
  - Returns `503` when required dependencies are unavailable.

## Adoption plan

1. Keep existing `/healthz` behavior.
2. Add `/readyz` route to each service handler.
3. Update smoke/compose scripts to wait on `/readyz` instead of fixed delays when possible.

## Initial checklist

- [ ] api-gateway
- [ ] iam
- [ ] marketplace
- [ ] settlement
- [ ] execution
- [ ] notification
- [ ] fiber-adapter
