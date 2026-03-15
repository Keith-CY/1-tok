# Carrier Integration Upstream Issue / PR Template

Use this template when opening an issue in the Carrier repo, or when drafting the upstream Carrier PR description for the `1-tok` integration.

The goal is to give Carrier maintainers a concrete, implementation-ready request instead of a loose compatibility ask.

## Suggested Issue Title

`feat: add first-class 1-tok marketplace integration contract`

## Issue Body Template

```md
## Summary

`1-tok` needs Carrier to expose a stable, provider-scoped integration contract so Carrier-backed providers can participate in the marketplace as first-class providers.

This is not only an endpoint request. The Carrier side also needs to support stable execution identity, callback signing, budget-aware control-plane feedback, and dispute-grade execution evidence.

Authoritative design references:

- 1-tok design: `docs/carrier-first-class-design.md`
- 1-tok upstream checklist: `docs/carrier-pr-support.md`

## Desired Outcomes

This work is complete only when all of these outcomes are true:

1. A provider can safely delegate execution authority from Carrier to `1-tok` using a provider-scoped token.
2. `1-tok` can create, observe, pause, resume, and cancel Carrier executions through a stable API namespace.
3. Carrier can push replay-safe lifecycle callbacks and expose durable artifacts and usage proofs.
4. Both teams can run joint staging tests without manual DB edits or undocumented internal steps.

## Required Carrier Deliverables

### 1. Provider-scoped integration credential

Carrier needs a provider-issued integration token with these properties:

- scoped to one provider account or tenant
- limited to that provider's own resources
- rotatable without hard downtime
- usable on both probe and async execution endpoints

### 2. Stable integration namespace

Recommended namespace:

- `/api/v1/integrations/one-tok/*`

Required endpoints:

- `POST /api/v1/integrations/one-tok/bindings/verify`
- `POST /api/v1/integrations/one-tok/executions`
- `GET /api/v1/integrations/one-tok/executions/:id`
- `POST /api/v1/integrations/one-tok/executions/:id/actions`

Existing compatibility probes should remain supported:

- `GET /api/v1/remote/hosts/:hostId/instances/:agentId/codeagent/health`
- `GET /api/v1/remote/hosts/:hostId/instances/:agentId/codeagent/version`
- `POST /api/v1/remote/hosts/:hostId/instances/:agentId/codeagent/run`

### 3. Stable execution identity and sequencing

Carrier needs to provide durable identifiers and replay-safe behavior:

- stable `carrierExecutionId`
- stable `attemptId`
- unique `eventId`
- monotonic `sequence` per execution
- idempotent handling for execution create and action requests

### 4. Signed callback delivery

Canonical callback target on the `1-tok` side:

- `POST /v1/carrier/callbacks/events`

Canonical event names:

- `execution.accepted`
- `execution.started`
- `execution.heartbeat`
- `milestone.started`
- `usage.reported`
- `artifact.ready`
- `budget.low`
- `milestone.ready`
- `execution.pause_requested`
- `execution.paused`
- `execution.resumed`
- `execution.failed`
- `execution.completed`

Required behavior:

- retries reuse the same `eventId` and `sequence`
- `accepted=false` means delivery failed and must be retried
- out-of-order gaps should be recoverable by redelivering the missing earlier event
- callback response `recommendedAction.type=pause` or `cancel` should be treated as authoritative platform feedback

### 5. Durable evidence

Carrier needs to expose:

- authoritative execution status
- typed failure category and failure code
- artifact references for logs and outputs
- usage proofs for billable usage events

Minimum failure categories:

- `provider_fault`
- `policy_denied`
- `infra_unavailable`
- `timeout`
- `invalid_input`
- `buyer_dependency_missing`
- `unknown`

## Required Coordination Artifacts

Carrier also needs to provide the following to unblock joint integration:

- one stable staging base URL
- one documented token issuance or rotation flow
- one stable staging resource tuple:
  - `hostId`
  - `agentId`
  - `backend`
  - `workspaceRoot`
- callback signing setup instructions
- sample callback payloads for success, budget pause, failure, and artifact publication
- sample usage-proof payloads
- one named Carrier owner for contract questions and rollout approval

## Minimum Joint Test Matrix

We should be able to demonstrate all of the following in staging:

1. Verify one provider resource tuple successfully.
2. Create one execution and receive a durable `carrierExecutionId`.
3. Emit `execution.accepted`, `execution.started`, `usage.reported`, `milestone.ready`, and `execution.completed`.
4. Honor a platform pause recommendation after budget or usage pressure.
5. Retry one callback safely without duplicate execution history.
6. Recover from one out-of-order callback by redelivering the missing earlier event.
7. Expose at least one artifact reference and one usage proof for a completed execution.

## Acceptance Criteria

- provider-scoped token flow exists and is documented
- integration namespace exists and is stable
- create and action endpoints are idempotent
- callbacks are signed and replay-safe
- pause/resume/cancel semantics are implemented
- status, artifacts, and usage proofs are queryable by execution id
- staging artifacts and ownership information are available for joint testing
- no new Carrier work depends on the legacy `/v1/carrier/events` alias or snake_case callback names

## Explicit Non-Goals

- interactive terminal streaming
- platform-managed host or agent creation
- multi-agent DAG orchestration
- buyer-facing UI work
- payment settlement logic inside Carrier

## Open Questions

- Does Carrier already have a public token issuance flow that matches the provider-scoped model, or does this need new surface area?
- Which Carrier internal state machine should map to the canonical execution states in the `1-tok` design?
- What is the intended retention window for artifacts and usage proofs in staging vs production?
```

## Suggested PR Description Addendum

If Carrier maintainers prefer to open a PR directly, append this checklist to the PR body:

```md
## 1-tok Integration Checklist

- [ ] provider-scoped token flow exists
- [ ] stable `/api/v1/integrations/one-tok/*` namespace added
- [ ] existing probe endpoints still work
- [ ] execution create is idempotent
- [ ] execution actions are idempotent
- [ ] signed callbacks use canonical dot-separated event names
- [ ] callback retries preserve `eventId` and `sequence`
- [ ] platform `pause` / `cancel` recommendations are honored
- [ ] status endpoint exposes authoritative state
- [ ] artifact refs are exposed
- [ ] usage proofs are exposed
- [ ] staging base URL and sample credentials shared with 1-tok
- [ ] sample payloads shared for joint testing
```

## Notes For The 1-tok Side

When sending this upstream:

- link the two authoritative docs instead of pasting large excerpts
- keep the issue focused on contract and rollout expectations, not on `1-tok` internal implementation details
- ask Carrier maintainers to call out any mismatch in sequencing, retention, or control-plane semantics before implementation starts
