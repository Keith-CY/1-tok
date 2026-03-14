# Production Release Status

Last updated: `2026-03-13`

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
- an authoritative first-class Carrier design doc plus an upstream implementation checklist
- release smoke commands for local, persisted, compose, and external-dependency rehearsals
- CI-safe reference coverage for a Dockerized `fnn` runtime overlay plus local `fiber-adapter`
- a dedicated `release:compose-fnn-smoke` path for validating raw FNN container startup alongside the stack
- a Docker-only `release:compose-e2e` path that runs both raw-`fnn` adapter smoke and the existing full business smoke suite from an `e2e-runner` container inside the compose network
- production Sentry initialization for all long-lived Go services plus the Next web runtime
- Redis-backed rate limiting on IAM and API gateway critical write routes
- a Docker-only abuse smoke that proves 429 behavior and mock-Sentry event delivery inside the compose network
- a `release:compose-fnn-dual-node-smoke` path that now includes first-cut CKB faucet/top-up preflight before running the dual-node adapter-backed payment smoke
- one successful local live verification of `release:compose-fnn-dual-node-smoke` against fresh Dockerized `fnn` / `fnn2` nodes plus testnet faucet/RPC on `2026-03-13`
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
- this repo now also contains a local `fiber-adapter` service that translates `tip.*` / `withdrawal.*` calls into raw FNN JSON-RPC for invoice creation, invoice status, and payment-request validation
- this repo now also contains a dual-node raw-FNN smoke runner plus a first-cut CKB funding wrapper that derives top-up addresses, checks balances, requests faucet top-ups, and then exercises adapter-backed payment
- but the full paid-settlement marketplace smoke still talks to `mock-fiber` for commit-safe business coverage

So the current honest release posture is: validate Dockerized `fnn` plus adapter translation under CI, and keep full paid-settlement marketplace smoke on `mock-fiber` until the dual-node funded FNN path is deterministic enough to replace it in routine release rehearsal.

The repo is now also materially closer to "production-ready" from an operational perspective:

- failures in long-lived services can now be shipped to Sentry with shared `service`, `release`, and `environment` tags
- the critical public write paths now have Redis-backed throttling instead of being fully unprotected
- Docker CI now has a built-in abuse proof path, not just happy-path business smoke

## Latest Live Dual-Node Result

`bun run release:compose-fnn-dual-node-smoke` completed successfully on `2026-03-13` using:

- `FNN_ASSET_SHA256=8f9a69361f662438fa1fc29ddc668192810b13021536ebd1101c84dc0cfa330f`
- `FIBER_SECRET_KEY_PASSWORD=local-fnn-dev-password`
- the default testnet CKB RPC and faucet fallback settings from the repo scripts

Key outputs from that successful run:

- `channelTemporaryId=0x022e4074deb8efa1ab9d04fae59bcc99a65641a078e4e6ca5c1418113c206c1e`
- `invoicePeerId=QmeDrSbsRmwXeW1omJv4WvGcfhF3u5wj9DaeUpApq4phAP`
- `payerPeerId=QmfHqijxz8QSVuMcE2pM8x5vTfyqn6squ24LjmwtyGU6m2`
- `adapter.quoteValid=true`
- `adapter.withdrawalId=0xd77a0b1baa247e3028844180c0ebee4adc0a9e8e8bdd9ad997efe4f998529165`

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
- configure real Sentry project alerts and notification routing for production
- add backup and restore rehearsal for Postgres and persistent FNN data
- run the same external rehearsal from the intended deployment platform, not only from a developer workstation
- persist and archive artifacts from the successful dual-node live smoke so it can serve as reusable release evidence
- replace `mock-fiber` in business smoke with a dual-node, funded adapter-backed FNN path once paid settlement is deterministic enough for CI or release rehearsal
- implement and upstream the Carrier work described in [carrier-first-class-design.md](./carrier-first-class-design.md) and [carrier-pr-support.md](./carrier-pr-support.md)
- track the remaining launch items in [production-launch-checklist.md](./production-launch-checklist.md)
