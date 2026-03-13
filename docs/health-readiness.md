# Health and Readiness Contract

This document standardizes endpoint semantics across HTTP services.

## Endpoints

- `GET /healthz`
  - Liveness only. This probe should not check dependencies.
  - Returns `200` when the process's HTTP server is responsive.

- `GET /readyz`
  - Readiness gate for dependencies.
  - Returns `200` when the service is ready for traffic.
  - Returns `503` when required dependencies are unavailable. The response body SHOULD provide details on dependency status, for example:
    ```json
    {
      "status": "unhealthy",
      "checks": {
        "database": "up",
        "redis": "down"
      }
    }
    ```

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
