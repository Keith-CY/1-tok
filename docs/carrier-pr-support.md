# Carrier Upstream PR Checklist

This checklist mirrors the current upstream contract tracked in `carrier#1564`.

Use it when coordinating Carrier implementation work or when reviewing an upstream Carrier PR. The goal is to keep the upstream ask aligned with the **current bridge contract** we can validate now and the **target async contract** we still want Carrier to expose over time.

For product rationale and platform-side context, see [carrier-first-class-design.md](./carrier-first-class-design.md).

## Goal

`Carrier` should support `1-tok` as a first-class marketplace execution substrate for provider-owned resources.

That contract is now split into two phases:

- **Phase 1**: bridge-compatible rollout using the existing `codeagent` probe endpoints plus signed callbacks into the platform
- **Phase 2**: Carrier-native async executions under a stable `/api/v1/integrations/one-tok/*` namespace

The async namespace is still required, but it is no longer treated as a prerequisite for the first bridge-based staging success.

## Required Outcomes

This integration is complete only when all of the following are true:

1. A provider can delegate execution authority from Carrier to `1-tok` using a provider-scoped token model.
2. `1-tok` can validate one provider-owned Carrier resource tuple without manual DB edits or undocumented operator steps.
3. Carrier can push signed, replay-safe lifecycle callbacks and durable evidence for budget control, settlement evidence, and disputes.
4. The bridge path works now, and the target async namespace can replace it without redefining the provider-scoped trust model.

## Phase 1: Current Bridge Contract

### 1. Keep the existing compatibility probes

Carrier must continue to support:

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/v1/remote/hosts/:hostId/instances/:agentId/codeagent/health` | resource verification and diagnostics |
| `GET` | `/api/v1/remote/hosts/:hostId/instances/:agentId/codeagent/version` | backend/version visibility |
| `POST` | `/api/v1/remote/hosts/:hostId/instances/:agentId/codeagent/run` | bridge-compatible execution/probe path |

These endpoints should continue to accept bearer-token auth.

### 2. Support provider-scoped auth for Carrier-owned resources

Carrier should support a provider-issued integration token that is:

- scoped to one provider account or tenant
- limited to that provider's own resources
- rotatable without hard downtime
- usable on the probe endpoints above
- reusable later for the async execution namespace in Phase 2

### 3. Support signed replay-safe callbacks to the current platform ingress

Carrier upstream work should target:

- `POST /api/v1/carrier/callbacks/events`

Compatibility alias still accepted by the platform during migration:

- `POST /api/v1/carrier/callback`

Carrier upstream work should not depend on the internal bridge alias `/v1/carrier/events`.

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
    "attemptId": "attempt_1",
    "kind": "step",
    "amountCents": 4200,
    "proofRef": "usage://proof-1",
    "proofSignature": "proof-hmac",
    "proofTimestamp": "2026-03-20T08:00:00Z"
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

### 7. Phase 1 callback response mapping

Carrier should expect these platform callback responses from `POST /api/v1/carrier/callbacks/events`:

- `200` + `accepted=true`: callback accepted
- `200` + `replay=true`: callback was a replay; previous decision is echoed back
- `400`: payload validation failed
- `401`: authentication mismatch
- `409`: sequence gap or out-of-order event; replay the missing earlier event
- `5xx`: transport/runtime failure; retry safely with the same event identity

### 8. Phase 1 staging test matrix

We should be able to demonstrate all of the following without manual DB edits:

1. Verify one provider resource tuple successfully using the existing probe endpoints.
2. Run one bridge-compatible execution/probe against a provider-owned resource.
3. Emit signed callbacks to `/api/v1/carrier/callbacks/events`.
4. Deliver `usage.reported`, `milestone.ready`, and `execution.completed` without corrupting event history.
5. Retry one callback safely with the same `eventId` and `sequence`.
6. Recover from one out-of-order callback by replaying the missing earlier event.
7. Honor one platform `pause` recommendation after budget or usage pressure.
8. Expose at least one artifact reference and one usage proof for a completed run.

## Phase 2: Target First-Class Async Contract

This remains the target-state Carrier surface that should replace bridge-only execution over time.

### Recommended namespace

- `/api/v1/integrations/one-tok/*`

### Required target endpoints

| Method | Path | Purpose |
| --- | --- | --- |
| `POST` | `/api/v1/integrations/one-tok/bindings/verify` | verify provider host/agent/backend/workspace |
| `POST` | `/api/v1/integrations/one-tok/executions` | create one async execution |
| `GET` | `/api/v1/integrations/one-tok/executions/:id` | fetch authoritative execution state |
| `POST` | `/api/v1/integrations/one-tok/executions/:id/actions` | pause, resume, or cancel execution |

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
