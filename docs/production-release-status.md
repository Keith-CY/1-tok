# Production Release Status

Last updated: `2026-03-12`

## Summary

The repository is release-engineered to the point where the main remaining blocker is external environment access, not a known missing implementation slice inside this repo.

Repo-local release work already in place:

- dedicated database bootstrap job via `cmd/bootstrap`
- bootstrapped-database enforcement for persisted services
- Postgres-backed IAM, marketplace, and settlement funding records
- real Fiber and Carrier client integrations
- RFQ, bidding, award, credit review, and dispute resolution flows
- dedicated `settlement-reconciler` worker
- rotating internal service-token support
- release smoke commands for local, persisted, compose, and external-dependency rehearsals
- CI-safe reference coverage for an optional Dockerized `fnn` runtime overlay
- a dedicated `release:compose-fnn-smoke` path for validating raw FNN container startup alongside the stack
- a Docker-only `release:compose-e2e` path that runs the full smoke suite from an `e2e-runner` container inside the compose network
- persisted release evidence artifacts plus aggregated `release-manifest.json`

## Remaining Blocker

The final blocker is a live rehearsal against real external dependencies. The current repo and shell environment do not contain usable values for:

- `DEPENDENCY_FIBER_RPC_URL`
- `DEPENDENCY_FIBER_APP_ID`
- `DEPENDENCY_FIBER_HMAC_SECRET`
- `DEPENDENCY_CARRIER_GATEWAY_URL`
- `DEPENDENCY_CARRIER_GATEWAY_API_TOKEN`

Without those values, the strongest available proof remains the local-mock external smoke and the compose/persisted local rehearsals.

There is also an important protocol boundary to keep in mind:

- this repo can now boot a raw `fnn` container in Docker for infra validation
- but the current `settlement` client still talks to higher-level `tip.*` / `withdrawal.*` RPC methods rather than raw FNN JSON-RPC

So the current honest release posture is: validate Dockerized `fnn` as runtime infrastructure, while business smoke still uses `mock-fiber` until a real adapter layer exists.

## Final Signoff Runbook

1. Export real external dependency settings.

```bash
export DEPENDENCY_FIBER_RPC_URL='https://fiber.example/rpc'
export DEPENDENCY_FIBER_APP_ID='app_live'
export DEPENDENCY_FIBER_HMAC_SECRET='replace-me'
export DEPENDENCY_CARRIER_GATEWAY_URL='https://carrier.example'
export DEPENDENCY_CARRIER_GATEWAY_API_TOKEN='replace-me'
```

2. Optionally export explicit healthcheck overrides if the real dependencies do not expose `/healthz`.

```bash
export DEPENDENCY_FIBER_HEALTHCHECK_URL='https://fiber.example/ready'
export DEPENDENCY_CARRIER_HEALTHCHECK_URL='https://carrier.example/ready'
```

3. Choose an artifact directory and run the live external rehearsal.

```bash
export RELEASE_ARTIFACT_DIR="$PWD/.release-artifacts/$(date +%Y%m%d-%H%M%S)"
bun run release:external-deps-smoke
```

4. Review the generated artifacts:

- `external-preflight.json`
- `release-smoke.json`
- `release-portal-smoke.json`
- `release-manifest.json`

5. Use `release-manifest.json` as the final release evidence package for signoff.

## What The Manifest Proves

`release-manifest.json` captures:

- the git SHA used for the rehearsal
- the artifact directory
- the external Fiber and Carrier endpoints used
- embedded copies of the preflight, backend smoke, and portal smoke results

That file is the shortest trustworthy answer to “what build was tested, against which dependencies, and what passed?”

## Non-Blocking Follow-Ups

These are still worth doing, but they are not the current hard blocker for a release claim:

- move internal service credentials from env secrets to a managed machine-identity or secret-manager flow
- add periodic artifact upload or archival to external storage
- add alerting around `settlement-reconciler` failures
- run the same external rehearsal from the intended deployment platform, not only from a developer workstation
- replace `mock-fiber` in business smoke with a real adapter-backed Fiber path once raw FNN integration is actually implemented
- upstream the Carrier contract described in [carrier-pr-support.md](/Users/ChenYu/Documents/Github/1-tok/docs/carrier-pr-support.md)
