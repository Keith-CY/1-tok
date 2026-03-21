# Developer Guide

This document holds the technical orientation that used to live in the repository root `README.md`.

Use it if you are trying to run, test, debug, or extend the codebase.

## Technical overview

1-tok is an agent runtime marketplace with a web portal, shared contracts, Go services, and compose-based smoke paths.

Core capabilities in the repo today include:

- `RFQ -> Bid -> Award -> Order` marketplace lifecycle
- milestone-based order tracking and settlement signals
- listing and provider search
- credit decisioning
- dispute handling
- provider ratings
- RFQ and order messaging
- Carrier binding, job lifecycle, and evidence paths
- notification and webhook support
- IAM, rate limiting, and health/readiness contracts

## Repository layout

- `apps/web`: Next.js portal for buyer, provider, and ops workspaces
- `packages/contracts`: shared TypeScript contracts and formatting helpers
- `cmd/*`: Go service binaries and release smoke entrypoints
- `internal/core`: domain logic for orders, settlement, disputes, and credit
- `internal/gateway`: unified marketplace HTTP API surface
- `internal/services/*`: service-specific HTTP entrypoints and workers
- `scripts/*`: local workflows, CI helpers, and compose smoke wrappers
- `docs/*`: design, release, and operational documentation

## Local development

The repository uses Bun for JavaScript package management and Go for backend services.

### Prerequisites

- Bun
- Go `1.25`
- Docker and Docker Compose for smoke and integration paths

### Install dependencies

```bash
bun install
```

### Common local commands

```bash
bun run dev:web
bun run dev:api-gateway
bun run dev:bootstrap
```

### Web verification

```bash
bun run lint:web
bun run test:web
bun run build:web
```

### Repository-wide verification

```bash
bun run test:go
bun run test:contracts
bun run test:coverage
bun run test:integration
```

## Portal UX governance

Portal consistency checks are available through:

```bash
bun run alpha:ux-audit
bun run alpha:ux-audit:strict
bun run alpha:ux-audit:summary
bun run alpha:ux-audit:summary:strict
bun run portal:check
bun run portal:check:quick
bun run portal:check:fast
bun run portal:check:strict
```

These commands validate empty-state actions, quick-filter consistency, and related portal UX rules.

## Smoke and release workflows

### Compose-based end-to-end stack

```bash
bun run release:compose-e2e
```

### Portal smoke

```bash
bun run release:portal-local-smoke
```

### Backend release smoke

```bash
bun run release:smoke
bun run release:abuse-smoke
```

### FNN / adapter / dual-node paths

```bash
bun run release:compose-fnn-smoke
bun run release:compose-fnn-dual-node-smoke
bun run release:fnn-dual-node-smoke
```

For the current honest launch posture and remaining blockers, read [production-release-status.md](./production-release-status.md).

## CI lanes

The repository currently runs these main CI lanes on `push` and `pull_request`:

- `Unit And Coverage`
- `Integration Smoke`
- `Portal UX Governance`
- `Report`

The compose-based screenshot and smoke paths are also used to validate portal flows and PR evidence where applicable.

## Environment and contracts

Use these documents as the source of truth:

- Environment variables: [env.md](./env.md)
- Health/readiness semantics: [health-readiness.md](./health-readiness.md)
- Launch checklist: [production-launch-checklist.md](./production-launch-checklist.md)
- Production release posture: [production-release-status.md](./production-release-status.md)
- Carrier design: [carrier-first-class-design.md](./carrier-first-class-design.md)
- Carrier upstream checklist: [carrier-pr-support.md](./carrier-pr-support.md)
- API surface: [api-spec.json](./api-spec.json)

## Notes on the API surface

The historical root README contained a long endpoint list. The repo now treats [api-spec.json](./api-spec.json) as the better source for the HTTP surface because it can stay machine-readable and easier to diff.

If you need exact endpoint behavior, combine:

- [api-spec.json](./api-spec.json)
- the gateway handlers in `internal/gateway`
- the integration and release smoke tests
