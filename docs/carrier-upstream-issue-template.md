# Carrier Integration Upstream Issue / PR Template

Use this template when opening an issue in the Carrier repo, or when drafting the upstream Carrier PR description for the `1-tok` integration.

The goal is to keep the upstream ask aligned with the **current bridge contract** we can validate now and the **target async contract** we still want Carrier to expose over time.

## Suggested Issue Title

`feat: add first-class 1-tok marketplace integration contract`

## Issue Body Template

```md
## Summary

`1-tok` still needs Carrier to expose a stable, provider-scoped integration contract so Carrier-backed providers can participate in the marketplace as first-class providers.

This issue is being updated to separate:

- the **current bridge contract** we can validate jointly today
- the **target first-class async contract** that should replace the bridge over time

That split matters because the current runnable `1-tok` integration already has:

- provider carrier bindings on the platform side
- signed replay-safe callback ingestion
- event ledgering by `eventId` and `sequence`
- bridge/probe support for existing `codeagent` endpoints

But it does **not** yet rely exclusively on Carrier-native async executions for end-to-end marketplace flow.

Reference context from the `1-tok` side:

- Design context: https://github.com/Keith-CY/1-tok/blob/main/docs/carrier-first-class-design.md
- Upstream checklist: https://github.com/Keith-CY/1-tok/blob/main/docs/carrier-pr-support.md

If there is any mismatch between those documents and the runnable bridge behavior called out below, treat **this issue** as the source of truth for upstream Carrier coordination until the docs are reconciled.

## Required Outcomes

This integration is complete only when all of the following are true:

1. A provider can delegate execution authority from Carrier to `1-tok` using a provider-scoped token model.
2. `1-tok` can validate one provider-owned Carrier resource tuple without manual DB edits or undocumented operator steps.
3. Carrier can push signed, replay-safe lifecycle callbacks and durable evidence for budget control, settlement evidence, and disputes.
4. The bridge path works now, and the target async namespace can replace it without redefining the provider-scoped trust model.

## Phase 1: Current Bridge Contract

### 1. Keep the existing compatibility probes

Carrier must continue to support:

- `GET /api/v1/remote/hosts/:hostId/instances/:agentId/codeagent/health`
- `GET /api/v1/remote/hosts/:hostId/instances/:agentId/codeagent/version`
- `POST /api/v1/remote/hosts/:hostId/instances/:agentId/codeagent/run`

These endpoints should continue to accept bearer-token auth.

### 2. Support provider-scoped auth for Carrier-owned resources

Carrier should support a provider-issued integration token that is:

- scoped to one provider account or tenant
- limited to that provider's own resources
- rotatable without hard downtime
- usable on the probe endpoints above
- reusable later for the async execution namespace in Phase 2

### 3. Support signed replay-safe callbacks to the current platform ingress

Current upstream callback target on the `1-tok` side:

- `POST /api/v1/carrier/callbacks/events`

Compatibility alias still accepted by the platform during migration:

- `POST /api/v1/carrier/callback`

Carrier upstream work should target the canonical `/api/v1/carrier/callbacks/events` path, not the internal bridge alias `/v1/carrier/events`.

### 4. Current callback envelope

Carrier should shape callbacks to the current platform envelope:

```json
{
  "type": "usage.reported",
  "jobId": "job_123",
  "bindingId": "bind_123",
  "timestamp": "2026-03-20T08:00:00Z",
  "signature": "hex-hmac",
  "payload": {
    "eventId": "evt_123",
    "sequence": 7,
    "attemptId": "attempt_1"
  }
}
```

Current platform expectations:

- `type`, `jobId`, `bindingId`, `timestamp`, and `signature` are top-level
- `eventId`, `sequence`, and optional `attemptId` are currently read from `payload`
- retries must reuse the same `eventId` and `sequence`
- out-of-order gaps can be rejected until missing earlier events are replayed
- callback response `recommendedAction.type=pause` or `cancel` should be treated as authoritative platform feedback

### 5. Canonical event names for current rollout

Carrier should prefer dot-separated event names now:

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

The platform may temporarily normalize snake_case aliases such as `usage_reported`, `budget_low`, and `milestone_ready`, but new Carrier work should not depend on that migration shim.

### 6. Phase 1 evidence requirements

Carrier needs to provide enough evidence for budget control and dispute handling:

- authoritative execution/job status
- artifact references for logs and outputs
- usage proofs for billable usage events
- typed failure category and failure code where failure occurs

Minimum failure categories:

- `provider_fault`
- `policy_denied`
- `infra_unavailable`
- `timeout`
- `invalid_input`
- `buyer_dependency_missing`
- `unknown`

## Phase 2: Target First-Class Async Contract

This remains the target-state Carrier surface that should replace bridge-only execution over time.

### Recommended namespace

- `/api/v1/integrations/one-tok/*`

### Required target endpoints

- `POST /api/v1/integrations/one-tok/bindings/verify`
- `POST /api/v1/integrations/one-tok/executions`
- `GET /api/v1/integrations/one-tok/executions/:id`
- `POST /api/v1/integrations/one-tok/executions/:id/actions`

### Required target-state properties

- stable `carrierExecutionId`
- stable `attemptId`
- idempotent execution create behavior
- idempotent pause / resume / cancel behavior
- authoritative execution status by execution id
- durable artifact and usage evidence by execution id
- provider-scoped bearer tokens on both probe and async execution endpoints

The target async namespace is still required for the full first-class Carrier integration, but it should not be treated as a prerequisite for the current bridge-based staging validation described in Phase 1.

## Required Coordination Artifacts

Carrier should provide:

- one stable staging base URL
- one documented token issuance or rotation flow
- one stable staging resource tuple: `hostId`, `agentId`, `backend`, `workspaceRoot`
- callback signing setup instructions, including key-id and secret rotation behavior
- sample callback payloads for success, budget pause, failure, and artifact publication
- sample usage-proof payloads
- one named Carrier owner for contract questions and rollout approval

## Acceptance Criteria

### Phase 1 acceptance

- existing probe endpoints still work
- provider-scoped auth story exists for provider-owned Carrier resources
- canonical callback path `/api/v1/carrier/callbacks/events` works
- callbacks are signed and replay-safe
- callback retries preserve `eventId` and `sequence`
- budget-aware platform recommendations are honored
- artifacts and usage proofs are available for joint validation

### Phase 2 acceptance

- stable `/api/v1/integrations/one-tok/*` namespace exists
- create and action endpoints are idempotent
- status, artifacts, and usage proofs are queryable by execution id
- pause / resume / cancel semantics are implemented on Carrier-native async executions
- Carrier-side execution identity replaces bridge-only assumptions

## Explicit Non-Goals

- interactive terminal streaming
- platform-managed host or agent creation
- multi-agent DAG orchestration
- buyer-facing UI work
- payment settlement logic inside Carrier
```

## Suggested PR Description Addendum

If Carrier maintainers prefer to open a PR directly, append this checklist to the PR body:

```md
## 1-tok Integration Checklist

### Phase 1

- [ ] existing probe endpoints still work
- [ ] provider-scoped auth story exists for provider-owned Carrier resources
- [ ] canonical callback path `/api/v1/carrier/callbacks/events` works
- [ ] signed callbacks use canonical dot-separated event names
- [ ] callback retries preserve `eventId` and `sequence`
- [ ] platform `pause` / `cancel` recommendations are honored
- [ ] artifact refs are exposed
- [ ] usage proofs are exposed

### Phase 2

- [ ] stable `/api/v1/integrations/one-tok/*` namespace added
- [ ] execution create is idempotent
- [ ] execution actions are idempotent
- [ ] status endpoint exposes authoritative state
- [ ] Carrier-side execution identity replaces bridge-only assumptions
```
