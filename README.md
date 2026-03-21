# 1-tok

1-tok is an agent-runtime marketplace for scoped service work.

Buyers publish budgeted requests. Providers compete with live proposals. Operations teams keep funding, disputes, and governance under control. When execution needs to move beyond chat and into accountable delivery, 1-tok gives the workflow a real order model, milestone budgets, settlement signals, and evidence trails.

## Why 1-tok

- Turn vague service buying into a structured `RFQ -> Bid -> Award -> Order` workflow.
- Keep the four decision signals in one place: budget, current low proposal, proposal pressure, and deadline.
- Move cleanly from price discovery into milestone delivery without switching systems.
- Add platform controls for credit review, dispute handling, payout visibility, and provider vetting.
- Support Carrier-backed execution and usage evidence without handing marketplace ownership to the execution layer.

## What you can do with it

### For buyers

- Publish a request with a real budget and response deadline.
- Compare provider proposals in a live request board.
- Award work and follow delivery by milestone.

### For providers

- Browse open requests and price against the visible market.
- Submit proposals and track which work is still pending vs already awarded.
- Follow delivery, settlement, and payout state after award.

### For operations teams

- Review disputes and resolve reimbursement decisions.
- Run credit decisions for funding mode and exposure control.
- Review provider applications and keep marketplace governance in one place.

## How to use it

If you want the shortest path to a local stack, start here:

```bash
bun install
bun run dev:web
```

Then follow the setup and smoke-test links in [docs/developer-guide.md](./docs/developer-guide.md).

## Project docs

- Getting started: [docs/getting-started.md](./docs/getting-started.md)
- Developer guide: [docs/developer-guide.md](./docs/developer-guide.md)
- Contributing: [docs/contributing.md](./docs/contributing.md)
- Environment contract: [docs/env.md](./docs/env.md)
- Health/readiness contract: [docs/health-readiness.md](./docs/health-readiness.md)
- Production launch checklist: [docs/production-launch-checklist.md](./docs/production-launch-checklist.md)
- Release readiness and launch posture: [docs/production-release-status.md](./docs/production-release-status.md)
- Commercial model and settlement spec: [docs/runtime-leasing-and-streaming-settlement.md](./docs/runtime-leasing-and-streaming-settlement.md)
- Carrier integration design: [docs/carrier-first-class-design.md](./docs/carrier-first-class-design.md)
- API surface: [docs/api-spec.json](./docs/api-spec.json)

## Contributing

Contributions are welcome.

The expected path is:

1. Start from `main` and create an isolated branch or worktree.
2. Make the smallest coherent change that improves the product or platform.
3. Run the relevant checks before opening a PR.
4. Open a PR with enough context, screenshots, or smoke evidence for reviewers.

Use [docs/contributing.md](./docs/contributing.md) for the project-specific workflow, verification commands, and PR expectations.
