# Carrier First-Class Agent Provider Design

Last updated: `2026-03-20`

This document is the authoritative design for making `Carrier-backed provider` a first-class provider type in `1-tok`.

It now defines both:

- the **current bridge contract** we can validate against existing Carrier probe endpoints and signed platform callbacks
- the **target first-class async contract** that should replace the bridge over time

The companion document [carrier-pr-support.md](./carrier-pr-support.md) is the upstream-facing checklist derived from this design.

## Decision Summary

The design fixes three product decisions:

- `Carrier` is a general execution substrate, not a single special seller account inside the marketplace.
- Each `provider org` owns and manages its own `Carrier` resources.
- The target end state uses an asynchronous `execution/job` protocol, but the current rollout keeps a bridge contract around existing `health`, `version`, and `run` probe endpoints plus signed replay-safe callbacks.

This split keeps platform ownership clear:

- `1-tok` owns marketplace truth, order state, budgets, settlement, disputes, credit, and provider governance.
- `Carrier` owns remote runtime truth, job orchestration, logs, artifacts, usage proof generation, and execution status.

## Goals

The design must allow:

- a provider to register `Carrier` resources once and reuse them across listings, bids, and orders
- the marketplace to treat `Carrier-backed` offers like any other provider offer
- order execution to survive retries, long runtimes, budget pauses, and disputes
- settlement to depend on explicit execution evidence instead of optimistic callbacks
- Carrier to implement a narrow, stable integration surface that can later be upstreamed as a PR

The design does not require:

- interactive terminal streaming in v1
- multi-agent DAG orchestration in v1
- platform-managed `Carrier` resource creation
- platform-side storage of raw logs or large artifacts

## Current State And Gap

Today the repo already has:

- platform-side provider carrier bindings
- platform-side execution jobs and callback event ledgering
- compatibility probe endpoints for `health`, `version`, and `run`
- signed callback ingress on the platform side
- an internal bridge path still used by smoke coverage

That is enough for bridge-based staging validation, but not yet enough to claim that Carrier-native async executions are the only marketplace execution path. The remaining gap is:

- provider-scoped auth wired end-to-end for live Carrier traffic
- a Carrier-native async execution namespace adopted as the formal execution path
- documentation and operator tooling fully switched off bridge-only assumptions

## Business Model

`Carrier-backed provider` becomes a first-class provider archetype.

The provider experience is:

1. The provider signs in to `1-tok`.
2. The provider connects a `Carrier` account or issues a provider-scoped integration token from `Carrier`.
3. The provider verifies one or more execution targets.
4. The provider creates one or more `ExecutionProfile`s from those targets.
5. Listings and bids reference an `ExecutionProfile`.
6. Awarded orders execute through the referenced `ExecutionProfile`.

The buyer experience does not change materially:

- buyers still discover listings, issue RFQs, compare bids, award orders, and view milestones
- the platform still decides whether money can move
- buyers see richer execution evidence, pause reasons, and dispute proof

## Platform Responsibilities

### 1. Provider Binding And Governance

The platform must add a new resource: `CarrierBinding`.

`CarrierBinding` is the provider-to-Carrier trust anchor. Required fields:

| Field | Meaning |
| --- | --- |
| `id` | Platform identifier |
| `providerOrgId` | Owning provider org |
| `carrierAccountId` | External account or tenant id in Carrier |
| `status` | `draft`, `verified`, `active`, `degraded`, `suspended`, `revoked` |
| `authMode` | v1 fixed to `bearer_token` |
| `credentialRef` | encrypted reference to provider-scoped Carrier token |
| `defaultHostId` | preferred Carrier host |
| `defaultAgentId` | preferred Carrier agent instance |
| `defaultBackend` | preferred backend name |
| `defaultWorkspaceRoot` | preferred workspace root |
| `supportedCapabilities` | capabilities the provider has approved for marketplace use |
| `maxConcurrency` | provider-declared concurrency limit |
| `healthStatus` | latest verification result |
| `verifiedAt` | latest successful verify timestamp |
| `callbackKeyId` | current outbound callback key id for Carrier to use |
| `callbackSecretRef` | encrypted callback secret reference |

Platform-side rules:

- only the owning provider org or ops can create or rotate a binding
- only `active` bindings may back listings or bids
- `degraded` or `suspended` bindings block new orders but do not auto-cancel running ones
- callback failures, heartbeat timeouts, and invalid signatures can move a binding to `degraded`
- callback signing secrets are established during binding creation or rotation and are not resent per execution request

### 2. Execution Profiles

The platform must add `ExecutionProfile`.

This is the marketplace-visible execution target that listings and bids reference. Required fields:

| Field | Meaning |
| --- | --- |
| `id` | Platform identifier |
| `bindingId` | owning `CarrierBinding` |
| `providerOrgId` | owning provider org |
| `name` | provider-facing label |
| `capability` | marketplace-exposed capability, for example `diagnostics` or `repo_fix` |
| `backend` | resolved Carrier backend |
| `hostId` | resolved host |
| `agentId` | resolved agent |
| `workspaceTemplate` | workspace root or template identifier |
| `defaultTimeoutSec` | default runtime bound |
| `maxTimeoutSec` | hard limit for orders |
| `baseBudgetPolicy` | how the milestone base price maps to execution |
| `variableUsagePolicy` | which usage kinds can emit billable usage |
| `artifactPolicy` | which artifacts must be retained |
| `logPolicy` | whether logs are retained and exposed |
| `supportedUsageKinds` | subset of `step`, `token`, `external_api` |
| `status` | `draft`, `active`, `suspended` |

Rules:

- every listing and bid must carry one `executionProfileId`
- the profile must belong to the bidding provider
- the profile must resolve to an `active` binding

### 3. Order Execution Model

The platform must add `ExecutionJob` and `ExecutionAttempt`.

`ExecutionJob` is the formal execution truth used by the order, settlement, ops, and dispute surfaces. Required fields:

| Field | Meaning |
| --- | --- |
| `id` | platform execution id |
| `orderId` | linked order |
| `milestoneId` | linked milestone |
| `providerOrgId` | linked provider |
| `executionProfileId` | linked execution profile |
| `carrierExecutionId` | external Carrier job id |
| `state` | `draft`, `accepted`, `queued`, `running`, `paused`, `completed`, `failed`, `cancelled` |
| `queueState` | Carrier queue status if exposed |
| `currentAttemptId` | latest attempt |
| `lastSequence` | latest accepted Carrier event sequence |
| `lastHeartbeatAt` | latest heartbeat timestamp |
| `startedAt` | first start time |
| `completedAt` | terminal timestamp |
| `failureCategory` | normalized failure class |
| `failureCode` | machine-readable code |
| `failureMessage` | operator-readable summary |

`ExecutionAttempt` records retries or reschedules. Required fields:

| Field              | Meaning                                        |
| ------------------ | ---------------------------------------------- |
| `id`               | Platform identifier for the attempt            |
| `executionJobId`   | Owning execution job                           |
| `attemptNo`        | Monotonic attempt number (1-based)             |
| `carrierExecutionId` | External Carrier execution id for this attempt |
| `startedAt`        | Attempt start time                             |
| `endedAt`          | Attempt end time                               |
| `resultSummary`    | Summary of the attempt's outcome               |

The platform must also persist `ExecutionEvent` as the append-only callback ledger. Required fields:

| Field | Meaning |
| --- | --- |
| `id` | platform identifier |
| `executionJobId` | owning execution job |
| `eventId` | unique Carrier event id |
| `sequence` | monotonic sequence for the execution |
| `eventType` | canonical event name |
| `attemptId` | Carrier attempt id if present |
| `payloadJSON` | normalized payload snapshot |
| `receivedAt` | platform receive time |
| `decisionJSON` | callback response returned to Carrier |

Ledger rules:

- replaying the same `eventId` must return the previous `decisionJSON` without mutating job state
- a different `eventId` with `sequence <= lastSequence` is rejected as replay or reordering
- a gap where `sequence > lastSequence + 1` is rejected so Carrier redelivers the missing event first
- ledger persistence and derived `ExecutionJob` mutation happen in one transaction

Execution lifecycle rules are fixed:

1. Awarding or resuming an executable milestone creates or resumes one `ExecutionJob` and snapshots the resolved binding/profile target onto the attempt.
2. The platform calls Carrier create with an idempotency key derived from `executionJobId` and `attemptNo`; retries reuse that same key until Carrier returns the same `carrierExecutionId`.
3. The synchronous create response only confirms acceptance; callbacks and status fetch remain the authoritative runtime truth.
4. `execution.accepted`, `execution.started`, and `execution.heartbeat` advance derived runtime state and refresh `lastHeartbeatAt`.
5. `usage.reported` can change budget state and may cause the callback response to request `pause`.
6. `milestone.ready` hands evidence into settlement; `execution.completed` only closes execution bookkeeping.
7. Retries create a new `ExecutionAttempt` but stay inside the same `ExecutionJob`.

### 4. Settlement, Risk, And Dispute Integration

Platform rules are fixed:

- `usage.reported` is the only accepted variable-charge event
- accepted `usageKind` values remain `step`, `token`, and `external_api`
- usage is recorded against the linked milestone and evaluated against platform budget
- if spend exceeds budget, the platform moves the order to `awaiting_budget` and asks Carrier to pause
- only `milestone.ready` can trigger the milestone settlement path
- dispute cases must be able to reference `ExecutionJob`, `ArtifactRef`, and `UsageProof`

The platform must add:

- `ArtifactRef` with `kind`, `name`, `downloadURL`, `contentType`, `sizeBytes`, `sha256`, `expiresAt`
- `UsageProof` with `kind`, `amountCents`, `proofRef`, `meterRef`, `occurredAt`

### 5. Operational Controls

The platform must provide:

- a verify action that checks current health and resolved target identity
- a credential rotation action
- suspend and resume actions on the binding and the profile
- a reconciliation worker that polls stale Carrier executions when callbacks are missing
- ops visibility for:
  - last callback time
  - verification status
  - active jobs
  - paused jobs awaiting budget
  - failed jobs by failure category

## Carrier Responsibilities

Carrier responsibilities are split into two phases.

### Phase 1: Current Bridge Contract

This is the contract Carrier should satisfy for near-term staging validation and bridge-based rollout.

Carrier must provide:

- the existing compatibility probes:
  - `GET /api/v1/remote/hosts/:hostId/instances/:agentId/codeagent/health`
  - `GET /api/v1/remote/hosts/:hostId/instances/:agentId/codeagent/version`
  - `POST /api/v1/remote/hosts/:hostId/instances/:agentId/codeagent/run`
- a provider-issued integration token model that is scoped to one provider's resources and can later be reused by the async namespace
- signed replay-safe callback delivery to `POST /api/v1/carrier/callbacks/events`
- durable execution/job status, artifacts, and usage-proof evidence
- callback retry behavior that preserves `eventId` and `sequence`
- budget-aware behavior that honors platform `pause` or `cancel` recommendations

Current callback envelope is fixed to the platform's live ingress shape:

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
- retries reuse the same `eventId` and `sequence`
- out-of-order gaps can be rejected until missing earlier events are replayed
- callback response `recommendedAction.type=pause` or `cancel` is authoritative platform feedback

Carrier should prefer canonical dot-separated event names now:

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

Snake_case aliases may still be normalized by the platform during migration, but new Carrier work should not depend on that shim.

### Phase 2: Target First-Class Async Contract

This remains the target-state Carrier surface that should replace bridge-only execution over time.

Carrier must expose a stable `1-tok` integration namespace. Recommended base path:

- `/api/v1/integrations/one-tok/*`

Target-state endpoints:

- `POST /api/v1/integrations/one-tok/bindings/verify`
- `POST /api/v1/integrations/one-tok/executions`
- `GET /api/v1/integrations/one-tok/executions/:carrierExecutionId`
- `POST /api/v1/integrations/one-tok/executions/:carrierExecutionId/actions`

Target-state guarantees:

- retry-safe execution creation by `Idempotency-Key`
- durable `carrierExecutionId` and `attemptId`
- monotonic event sequences per execution
- durable artifact references for logs and outputs
- usage proofs attached to billable usage events
- explicit failure categories and timeout semantics
- idempotent pause, resume, and cancel semantics

The target async namespace is still required for the full first-class Carrier integration, but it should not be treated as a prerequisite for the current bridge-based staging validation described in Phase 1.

### Joint Integration Requirements

Carrier and `1-tok` have separate ownership, so successful rollout also requires explicit coordination artifacts from the Carrier side.

Carrier must provide:

- one named maintainer or team owning the `1-tok` integration contract
- one staging tenant or account that `1-tok` can use for end-to-end testing
- one sample provider-scoped token for staging, or a documented self-serve token issuance flow
- one verified `(hostId, agentId, backend, workspaceRoot)` tuple that remains stable during integration
- one callback signing key flow that supports initial setup and later rotation
- example success, pause, failure, artifact, and usage-proof payloads for local and CI fixtures
- a changelog policy for contract changes affecting `1-tok`

The integration should not be considered ready for rollout until both teams can run the same happy-path and failure-path scenarios against Carrier staging without relying on undocumented internal knowledge.

## Interface Contract

### Phase 1: Current Bridge Contract

#### Platform callback ingress

Canonical platform callback path:

- `POST /api/v1/carrier/callbacks/events`

Compatibility alias still accepted during migration:

- `POST /api/v1/carrier/callback`

Carrier upstream work should target the canonical `/api/v1/carrier/callbacks/events` path, not the internal bridge alias `/v1/carrier/events`.

Headers:

- `X-One-Tok-Callback-Key-Id` *(preferred)*
- `X-One-Tok-Key-Id` *(compat alias, accepted)*
- `X-One-Tok-Timestamp` *(override for request body timestamp)*
- `X-One-Tok-Signature` *(override for request body signature)*
- `X-One-Tok-Callback-Secret` *(manual override in special integrations)*

Notes:

- header values `X-One-Tok-Timestamp` and `X-One-Tok-Signature` are applied before signature verification and take precedence over `timestamp`/`signature` in payload body
- preferred key lookup key is `X-One-Tok-Callback-Key-Id`; `X-One-Tok-Key-Id` remains supported for compatibility
- a callback without a key id can still pass if the legacy secret binding path is explicitly configured; key-id plus wrong secret becomes unauthorized immediately
- the callback secret itself is provisioned during binding creation or rotation and looked up by `keyId`; it is never resent in per-execution requests

`usage.reported` proof validation behavior (when secret is available):

- `proofRef`, `proofSignature`, and `proofTimestamp` are required together
- signature is verified with HMAC over `(executionId, milestoneId, kind, amountCents, proofTimestamp)`
- replay and stale-window checks on proof timestamp are enforced using the same max-age policy as callback authenticity

Platform callback response fields:

- `accepted`
- `continueAllowed`
- `recommendedAction.type`
- `recommendedAction.reason`

Allowed `recommendedAction.type` values:

- `continue`
- `pause`
- `cancel`

#### Compatibility probe endpoints

The existing endpoints remain valid:

- `GET /api/v1/remote/hosts/:hostId/instances/:agentId/codeagent/health`
- `GET /api/v1/remote/hosts/:hostId/instances/:agentId/codeagent/version`
- `POST /api/v1/remote/hosts/:hostId/instances/:agentId/codeagent/run`

Their role in Phase 1 is:

- provider binding verification
- ops diagnostics
- compatibility checks
- bridge-compatible execution/probe behavior

### Phase 2: Target First-Class Async Contract

#### `POST /api/v1/integrations/one-tok/bindings/verify`

Purpose:

- validate that the bound host, agent, backend, and workspace are real and usable

Request fields:

- `bindingId`
- `providerOrgId`
- `hostId`
- `agentId`
- `backend`
- `workspaceRoot`
- `supportedCapabilities`

Response fields:

- `verified`
- `health.healthy`
- `health.workspaceRoot`
- `version.value`
- `resolvedHostId`
- `resolvedAgentId`
- `resolvedBackend`
- `capabilities`

#### `POST /api/v1/integrations/one-tok/executions`

Purpose:

- create one async execution for one platform milestone

Headers:

- `Authorization: Bearer <provider-scoped-token>`
- `Idempotency-Key`

Request fields:

- `platformExecutionId`
- `orderId`
- `milestoneId`
- `providerOrgId`
- `executionProfileId`
- `target.hostId`
- `target.agentId`
- `target.backend`
- `target.workspaceRoot`
- `task.capability`
- `task.title`
- `task.instructions`
- `task.timeoutSec`
- `budget.basePriceCents`
- `budget.maxVariableSpendCents`
- `budget.pauseThresholdCents`
- `artifacts.retentionHours`
- `artifacts.requiredKinds`
- `callbacks.eventsUrl`
- `callbacks.auth.type`
- `callbacks.auth.keyId`

Response fields:

- `carrierExecutionId`
- `accepted`
- `queueState`
- `estimatedStartAt`

#### `GET /api/v1/integrations/one-tok/executions/:carrierExecutionId`

Response fields:

- `carrierExecutionId`
- `platformExecutionId`
- `state`
- `queueState`
- `lastSequence`
- `currentAttempt`
- `startedAt`
- `completedAt`
- `failureCategory`
- `failureCode`
- `failureMessage`
- `artifacts`
- `usage`

#### `POST /api/v1/integrations/one-tok/executions/:carrierExecutionId/actions`

Headers:

- `Authorization: Bearer <provider-scoped-token>`
- `Idempotency-Key`

Request fields:

- `action`
- `reason`
- `requestedBy`

Allowed `action` values:

- `pause`
- `resume`
- `cancel`

## Platform Compatibility Bridge

`origin/main` still ships a platform-side execution bridge. The target Carrier contract in this document must coexist with that bridge until migration completes.

| Current platform route | Current behavior | Target contract |
| --- | --- | --- |
| `GET /v1/carrier/codeagent/health` | platform proxy for health verification | Carrier `GET /api/v1/remote/hosts/:hostId/instances/:agentId/codeagent/health` |
| `GET /v1/carrier/codeagent/version` | platform proxy for backend/version inspection | Carrier `GET /api/v1/remote/hosts/:hostId/instances/:agentId/codeagent/version` |
| `POST /v1/carrier/codeagent/run` | platform proxy for bridge-compatible probe execution | Carrier `POST /api/v1/remote/hosts/:hostId/instances/:agentId/codeagent/run` |
| `POST /v1/carrier/events` | internal bridge ingress still used by smoke coverage | Platform `POST /api/v1/carrier/callbacks/events` using canonical dot-separated event names |

Bridge rules:

- Carrier upstream work targets the canonical `/api/v1/carrier/callbacks/events` path, not the internal bridge route
- the platform bridge may temporarily accept snake_case aliases such as `usage_reported`, `budget_low`, and `milestone_ready`, but it normalizes them to canonical names before persistence
- new smoke tests, docs, and operator runbooks should prefer `/api/v1/carrier/callbacks/events` and canonical dot-separated event names
- the internal `/v1/carrier/events` bridge is removable only after ops tooling and smoke coverage no longer depend on it

## State Mapping

Carrier-to-platform state mapping is fixed:

| Carrier state | Platform execution job | Order/milestone implication |
| --- | --- | --- |
| `accepted` | `accepted` | order remains `running` |
| `queued` | `queued` | order remains `running` |
| `running` | `running` | milestone stays `running` |
| `paused` | `paused` | order may become `awaiting_budget` or remain `running` depending on reason |
| `completed` | `completed` | not sufficient for payout by itself |
| `failed` | `failed` | order failure handling or dispute evidence path |
| `cancelled` | `cancelled` | platform marks job terminal |

Platform settlement mapping is also fixed:

- `usage.reported` updates usage and may trigger `pause`
- `budget.low` is advisory and does not settle money
- `milestone.ready` is the only event that can enter milestone settlement
- `execution.completed` closes execution bookkeeping but does not replace `milestone.ready`

## Security And Reliability Requirements

These are mandatory requirements for implementation:

- platform-to-Carrier requests move toward provider-scoped bearer tokens as the bridge is retired
- Carrier-to-platform callbacks use per-binding HMAC signing
- every create and action request is idempotent
- callback processing is idempotent by `eventId`
- event ordering is enforced by `sequence`
- platform keeps a stale-job reconciliation loop for missing callbacks
- invalid signature, wrong provider scope, or out-of-order replay must be rejected

Retention defaults:

- Carrier should retain artifacts and logs for at least 30 days
- signed artifact URLs may expire earlier, but Carrier must be able to mint fresh URLs during that retention window

## Rollout

Implementation order is fixed:

1. Keep the bridge contract healthy enough for joint staging validation.
2. Keep the internal execution bridge in place, but prefer the canonical `/api/v1/carrier/callbacks/events` path and dot-separated event normalization in new docs and tooling.
3. Implement provider-scoped binding verification and provider-scoped auth end to end.
4. Add async execution endpoints in Carrier.
5. Add callback verification, ledger persistence, and stale-job reconciliation in the platform.
6. Move listings and bids to reference `executionProfileId`.
7. Rewrite smoke tests and operator tooling so formal marketplace execution prefers the target async path and callback names.
8. Remove the internal bridge alias only after the migration is complete.

## Acceptance Criteria

The integration is ready when all of these are true:

- a provider can bind Carrier resources without ops editing database rows
- a listing or bid cannot be published without an active execution profile
- an awarded order creates an execution job and receives signed Carrier callbacks
- execution callbacks are durably ledgered and gap or replay handling is deterministic
- duplicate and out-of-order callbacks do not corrupt job state
- a budget overrun pauses execution and later resumes after budget extension
- a dispute case can show artifact refs and usage proofs from Carrier
- binding suspension blocks new awards but does not break already-running orders
- smoke tests and ops tooling no longer depend on `/v1/carrier/events` or snake_case callback names before that alias is removed
- the upstream Carrier PR can be described entirely by the checklist in [carrier-pr-support.md](./carrier-pr-support.md)
