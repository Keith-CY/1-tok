# Production Release Status

Last updated: `2026-03-23`

## Summary

The repo now has two distinct readiness postures:

- `demo-ready`: implemented and verifiable from a fixed remote environment
- `production-ready`: still blocked on the broader operational and launch checklist

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
- a real `USDI marketplace e2e` path with real Carrier, `fnn`, `fnn2`, `provider-fnn`, and screenshot/comment evidence
- dedicated `release:demo:prepare` and `release:demo:verify` commands for a fixed remote demo environment
- an ops-side demo-readiness surface that reports service health, buyer prefund, provider liquidity, and blockers
- persisted release evidence artifacts plus aggregated `release-manifest.json`

## Demo-Ready Posture

The repo is now strong enough for a live remote demo when all of these are true:

- the fixed Coolify environment is up
- `bun run release:demo:prepare` has already been run for that environment
- `bun run release:demo:verify` returns `ready`
- `/ops` shows `Demo readiness` as `ready`

The shortest operator path is now:

```bash
bun run release:demo:prepare
bun run release:demo:verify
```

The demo-specific runbooks now live in:

- [demo-environment.md](./demo-environment.md)
- [demo-runbook.md](./demo-runbook.md)

## Production Blockers

Production readiness still depends on the broader launch checklist. The main remaining blockers are operational rather than missing product slices:

- backup and restore rehearsal
- Sentry alert routing and escalation
- legal/policy surfaces
- external artifact archival
- the remaining production launch items in [production-launch-checklist.md](./production-launch-checklist.md)
