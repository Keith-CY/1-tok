# Production Launch Checklist

Last updated: `2026-03-13`

This checklist is the current internal source of truth for what still separates this repo from a formal production launch. It is intentionally split by `P0 / P1 / P2` and by functional owner so that each item can be assigned and signed off without interpretation drift.

Status values:

- `done`: implemented and verified in this repo
- `blocked`: technically ready to execute but blocked by external environment or credentials
- `todo`: not yet implemented or not yet signed off

## P0

| Owner | Item | Status | Acceptance | Evidence / Notes |
| --- | --- | --- | --- | --- |
| Platform / Backend | Production Sentry on all Go services | `done` | `api-gateway`, `iam`, `marketplace`, `settlement`, `settlement-reconciler`, `execution`, `risk`, `notification`, `fiber-adapter`, and `bootstrap` initialize Sentry and recover panics | See [README.md](/Users/ChenYu/Documents/Github/1-tok/README.md) and Go entrypoints under `cmd/*` |
| Frontend | Production Sentry on Next web | `done` | Next browser and server runtimes initialize Sentry with shared `release` / `environment`; global error capture exists for App Router | See [README.md](/Users/ChenYu/Documents/Github/1-tok/README.md) |
| Platform / Backend | Redis-backed abuse protection on critical write routes | `done` | `signup`, `login`, `logout`, RFQ, bid, award, order, message, dispute, and credit decision routes enforce 429 with standard headers | Verified by unit tests and Docker E2E |
| Infra / DevOps | Docker E2E includes abuse smoke and mock Sentry | `done` | CI `Integration Smoke` runs inside Docker with `redis` and `mock-sentry`, and verifies both rate limiting and Sentry event delivery | See [README.md](/Users/ChenYu/Documents/Github/1-tok/README.md) |
| Infra / DevOps | Real external `Fiber` / `Carrier` live rehearsal | `blocked` | `bun run release:external-deps-smoke` passes with real credentials and archived `release-manifest.json` | Blocked on external credentials |
| Infra / DevOps | Backup and restore drill | `todo` | A documented backup command, a restore rehearsal, and a timestamped restore proof exist for Postgres and persistent FNN data | Not yet in repo |
| Infra / DevOps | Sentry project alerts and notification routing | `todo` | Sentry project has production alert rules and an agreed notification destination; runbook documents ownership and escalation | Repo now supports SDKs; project-side routing still needs to be configured |
| Product / Legal | Terms, privacy, dispute policy, support boundary | `todo` | Buyer/provider-facing policies exist and are linked from launch surfaces | Not yet in repo |

## P1

| Owner | Item | Status | Acceptance | Evidence / Notes |
| --- | --- | --- | --- | --- |
| Platform / Backend | Replace `mock-fiber` in business smoke with deterministic funded raw-FNN path | `todo` | Full business marketplace smoke no longer depends on `mock-fiber` | Current CI still uses `mock-fiber` for commit-safe coverage |
| Infra / DevOps | Artifact archival outside local disk | `todo` | Release artifacts are automatically uploaded to durable storage | Currently generated locally |
| Infra / DevOps | Managed secret / machine identity flow | `todo` | Service-to-service auth and Sentry/Carrier/Fiber secrets are not long-lived env secrets | Current model supports rotation but not secret manager integration |
| Ops / Risk | Abuse threshold tuning and escalation SOP | `todo` | Thresholds have owner-approved values, false-positive review flow, and unblock path | First defaults are now implemented in code |
| Product / Support | Incident and customer support runbook | `todo` | On-call, escalation, and customer-facing response path are documented | Not yet in repo |
| Provider Platform | Carrier upstream contract PR | `todo` | Minimum carrier hooks are upstreamed or tracked as accepted PRs/issues | Contract lives in [carrier-pr-support.md](/Users/ChenYu/Documents/Github/1-tok/docs/carrier-pr-support.md) |

## P2

| Owner | Item | Status | Acceptance | Evidence / Notes |
| --- | --- | --- | --- | --- |
| Finance / Ops | Customer billing and external reconciliation SOP | `todo` | Monthly reconciliation, exception review, and payout escalation are documented | Internal ledger exists; operating process still missing |
| Product / Growth | Buyer/provider onboarding docs | `todo` | New user can complete onboarding without ad-hoc engineer guidance | Not yet in repo |
| Security / Infra | Extended anti-abuse controls | `todo` | CAPTCHA, WAF integration, or risk scoring is added if traffic profile requires it | Current scope is rate limiting plus observability |
| Platform / Backend | Deployment-platform live rehearsal | `todo` | Final external rehearsal is run from the intended Coolify environment, not only a dev machine | Recommended after credentials are available |

## Signoff Path

1. Finish all `P0` items that are not `done`.
2. Archive `release-manifest.json` from the real external rehearsal.
3. Record owner signoff for `Platform / Backend`, `Infra / DevOps`, and `Product / Legal`.
4. Move any unfinished `P1` / `P2` items into the post-launch backlog with an explicit owner.
