# GitHub-Ready Carrier Upstream Issue Draft

## Suggested Title

`feat: add first-class 1-tok marketplace integration contract`

## Paste-Ready Issue Body

```md
## Summary

`1-tok` needs Carrier to expose a stable, provider-scoped integration contract so Carrier-backed providers can participate in the marketplace as first-class providers.

This work is not only about adding endpoints. The Carrier side also needs to support:

- provider-scoped execution authority
- stable execution identity
- signed, replay-safe lifecycle callbacks
- budget-aware control-plane feedback
- durable artifacts and usage proofs for settlement and disputes

Reference docs from the `1-tok` side:

- `docs/carrier-first-class-design.md`
- `docs/carrier-pr-support.md`

## Required Outcomes

This integration is complete only when all of the following are true:

1. A provider can delegate execution authority from Carrier to `1-tok` using a provider-scoped token.
2. `1-tok` can create, observe, pause, resume, and cancel Carrier executions through a stable API contract.
3. Carrier can push replay-safe lifecycle callbacks and expose durable artifacts and usage proofs.
4. Both teams can run staging validation without manual DB edits or undocumented internal steps.

## Required Carrier API Surface

### Provider-scoped token

Carrier needs a provider-issued integration token that is:

- scoped to one provider account or tenant
- limited to that provider's own resources
- rotatable without hard downtime
- usable on both probe endpoints and async execution endpoints

### Stable integration namespace

Recommended namespace:

- `/api/v1/integrations/one-tok/*`

Required endpoints:

- `POST /api/v1/integrations/one-tok/bindings/verify`
- `POST /api/v1/integrations/one-tok/executions`
- `GET /api/v1/integrations/one-tok/executions/:id`
- `POST /api/v1/integrations/one-tok/executions/:id/actions`

Existing probe endpoints should remain supported:

- `GET /api/v1/remote/hosts/:hostId/instances/:agentId/codeagent/health`
- `GET /api/v1/remote/hosts/:hostId/instances/:agentId/codeagent/version`
- `POST /api/v1/remote/hosts/:hostId/instances/:agentId/codeagent/run`

## Execution And Callback Requirements

Carrier needs to provide durable execution identity and replay-safe delivery:

- stable `carrierExecutionId`
- stable `attemptId`
- unique `eventId`
- monotonic `sequence` per execution
- idempotent execution create behavior
- idempotent pause / resume / cancel behavior

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

Required callback behavior:

- retries must reuse the same `eventId` and `sequence`
- `accepted=false` must be treated as delivery failure and retried
- out-of-order gaps should be recoverable by redelivering the missing earlier event
- callback response `recommendedAction.type=pause` or `cancel` should be treated as authoritative platform feedback

## Evidence Requirements

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

To unblock joint integration, Carrier also needs to provide:

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

## Minimum Staging Test Matrix

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
- pause / resume / cancel semantics are implemented
- status, artifacts, and usage proofs are queryable by execution id
- staging artifacts and ownership information are available for joint testing
- no new Carrier work depends on the legacy `/v1/carrier/events` alias or snake_case callback names

## Non-Goals

- interactive terminal streaming
- platform-managed host or agent creation
- multi-agent DAG orchestration
- buyer-facing UI work
- payment settlement logic inside Carrier
```

## Optional PR Checklist

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
- [ ] staging base URL and sample credentials shared with `1-tok`
- [ ] sample payloads shared for joint testing
```
