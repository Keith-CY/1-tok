# Contributing To 1-tok

Thanks for contributing.

This repository mixes product-facing portal work, backend services, integration contracts, and release tooling, so the most useful contributions are the ones that stay scoped and come with verification.

## Recommended workflow

1. Start from `main` or `origin/main`.
2. Create an isolated branch or worktree for the change.
3. Make the smallest coherent update you can justify.
4. Run the checks that match the surface you touched.
5. Open a PR with enough reviewer context to evaluate the change quickly.

## What to run before opening a PR

### If you changed portal UI or shared web code

```bash
bun run lint:web
bun run test:web
bun run build:web
```

### If you changed contracts or shared logic

```bash
bun run test:contracts
bun run test:go
```

### If you changed CI-sensitive behavior or end-to-end flows

```bash
bun run test:coverage
bun run test:integration
```

### If you changed buyer/provider/ops portal behavior

```bash
bun run portal:check:fast
```

Use `bun run portal:check:strict` when you want CI-level confidence for portal work.

## PR expectations

Good PRs in this repo usually include:

- a clear summary of what changed
- the user or operator impact
- verification commands that were actually run
- screenshots for visible portal changes
- notes on scope boundaries if the work is intentionally partial

## Docs and design changes

If you are changing docs, product language, or integration contracts:

- keep user-facing and developer-facing documentation clearly separated
- update linked docs when you change the source of truth
- avoid leaving technical implementation details in the root `README.md`

## Useful references

- Repo orientation: [developer-guide.md](./developer-guide.md)
- Environment variables: [env.md](./env.md)
- Release posture: [production-release-status.md](./production-release-status.md)
- Launch checklist: [production-launch-checklist.md](./production-launch-checklist.md)
