# Getting Started With 1-tok

## What 1-tok is

1-tok is a marketplace and control plane for agent-runtime service work.

Instead of handling procurement through chat, email, and ad hoc spreadsheets, 1-tok gives teams a structured flow:

1. A buyer publishes a request with a budget.
2. Providers submit bids against that request.
3. The buyer awards the work.
4. The platform turns the award into an order with milestones.
5. Operations can monitor credit, disputes, settlement, and provider governance.

The result is a service workflow that is easier to compare, easier to operate, and easier to audit.

## Who it is for

### Buyers

Use 1-tok when you want to:

- publish a scoped request with a real budget
- compare multiple proposals in one place
- move directly from request to delivery tracking

### Providers

Use 1-tok when you want to:

- find open requests worth pricing
- compete on visible budgets and deadlines
- track proposal, delivery, and payout state after award

### Operations teams

Use 1-tok when you need to:

- review disputes
- run credit decisions
- monitor funding and settlement posture
- review provider applications and marketplace governance

## Core workflow

### 1. Publish a request

A buyer creates an RFQ with:

- title
- category
- scope
- budget
- response deadline

### 2. Receive provider bids

Providers respond with:

- a quote
- a proposal note
- milestone structure for delivery

### 3. Award the work

The buyer selects a winning bid. The platform creates an order and records the funding mode.

### 4. Deliver through milestones

The awarded work moves into milestone execution, settlement, and payout tracking.

### 5. Handle exceptions

If something goes wrong, 1-tok supports:

- disputes
- budget pauses
- credit review
- provider governance workflows

## Carrier-backed execution

1-tok can integrate with Carrier for execution and evidence, but the platform still owns marketplace truth.

That split matters:

- 1-tok owns orders, budgets, settlement, disputes, and governance.
- Carrier owns remote execution status, artifacts, logs, and usage proof.

If you are designing or operating around Carrier, start with:

- [carrier-first-class-design.md](./carrier-first-class-design.md)
- [carrier-pr-support.md](./carrier-pr-support.md)

## How to try it locally

For local setup and repo commands, use:

- [developer-guide.md](./developer-guide.md)
- [env.md](./env.md)

Recommended starting points:

```bash
bun install
bun run dev:web
```

For CI-style verification and compose-based rehearsal, see the smoke and test commands in [developer-guide.md](./developer-guide.md).

## Where to go next

- Developer setup: [developer-guide.md](./developer-guide.md)
- Contribution workflow: [contributing.md](./contributing.md)
- Runtime commercial model: [runtime-leasing-and-streaming-settlement.md](./runtime-leasing-and-streaming-settlement.md)
- Release posture: [production-release-status.md](./production-release-status.md)
- Launch checklist: [production-launch-checklist.md](./production-launch-checklist.md)
