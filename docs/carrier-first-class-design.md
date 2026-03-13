# Carrier First-Class Agent Provider Design

Last updated: `2026-03-13`

This document is the authoritative design for making `Carrier-backed provider` a first-class provider type in `1-tok`.

It replaces the older “three probe endpoints plus callback” framing as the main source of truth. The older document [carrier-pr-support.md](./carrier-pr-support.md) now serves as the upstream implementation checklist derived from this design.

## Decision Summary

The design fixes three product decisions:

- `Carrier` is a general execution substrate, not a single special seller account inside the marketplace.
- Each `provider org` owns and manages its own `Carrier` resources.
- Formal marketplace execution uses an asynchronous `execution/job` protocol. The existing synchronous `health`, `version`, and `run` endpoints remain as compatibility probes and diagnostics only.

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

Today the repo only has:

- synchronous probe endpoints for `health`, `version`, and `run`
- a coarse `POST /v1/carrier/events` callback into the execution service
- no persistent `Carrier` binding model
- no persistent execution job model
- no formal evidence package for disputes

That is enough for smoke testing, but not enough for a first-class provider integration. The missing pieces are:

- provider-scoped resource binding
- marketplace-visible execution profiles
- async job lifecycle
- signed, idempotent callback delivery
- durable artifacts and usage proofs
- operational health and credential rotation

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

`ExecutionAttempt` records retries or reschedules:

- `id`
- `executionJobId`
- `attemptNo`
- `carrierExecutionId`
- `startedAt`
- `endedAt`
- `resultSummary`

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

### 1. Provider-Scoped Integration Credential

Carrier must let a provider issue a provider-scoped integration token to `1-tok`.

Required properties:

- the token is restricted to that provider’s own `host`, `agent`, `profile`, `execution`, and artifact resources
- the token can be rotated without hard downtime
- Carrier can identify the provider account and resources associated with that token

### 2. Stable Integration Surface

Carrier must expose a stable `1-tok` integration namespace. Recommended base path:

- `/api/v1/integrations/one-tok/*`

Carrier must implement:

- resource verification
- execution creation
- execution status fetch
- execution action control
- artifact listing
- signed callback delivery

### 3. Execution Runtime Behavior

Carrier must guarantee:

- retry-safe execution creation by `Idempotency-Key`
- durable external ids for executions and attempts
- monotonic event sequences per execution
- explicit failure categories
- explicit timeout semantics
- durable artifact references for logs and outputs
- usage proofs attached to billable usage events

Carrier is the owner of:

- workspace preparation
- remote agent start and supervision
- retries or requeue within its own control plane
- log retention
- artifact retention

### 4. Callback Delivery

Carrier must push lifecycle events to the platform and retry on transient failures.

Callback behavior is fixed:

- every event has a globally unique `eventId`
- every execution has a monotonically increasing `sequence`
- retries reuse the same `eventId` and `sequence`
- callbacks are signed with the binding-specific HMAC secret
- Carrier treats non-2xx or `accepted=false` as delivery failure
- Carrier retries with exponential backoff for at least 15 minutes

## Interface Contract

### A. Platform To Carrier

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

### B. Carrier To Platform

#### `POST /v1/carrier/callbacks/events`

Headers:

- `X-One-Tok-Key-Id`
- `X-One-Tok-Timestamp`
- `X-One-Tok-Signature`

The body is signed using `HMAC-SHA256` over timestamp plus body.

The callback secret itself is provisioned during binding creation or rotation and looked up by `keyId`; it is never resent in per-execution requests.

Event envelope fields:

- `eventId`
- `sequence`
- `eventType`
- `occurredAt`
- `platformExecutionId`
- `carrierExecutionId`
- `orderId`
- `milestoneId`
- `providerOrgId`
- `executionProfileId`
- `attemptId`
- `payload`

Mandatory event types:

| Event | Required payload |
| --- | --- |
| `execution.accepted` | `queueState` |
| `execution.started` | `attemptNo` |
| `execution.heartbeat` | `progressPercent`, `summary` |
| `milestone.started` | `summary` |
| `usage.reported` | `usageKind`, `amountCents`, `proofRef`, `meterRef` |
| `artifact.ready` | `artifacts[]` |
| `budget.low` | `currentSpendCents`, `remainingBudgetCents`, `recommendedAction` |
| `milestone.ready` | `summary`, `artifacts[]`, `usageSnapshot` |
| `execution.pause_requested` | `reason` |
| `execution.paused` | `reason` |
| `execution.resumed` | `reason` |
| `execution.failed` | `failureCategory`, `failureCode`, `failureMessage`, `artifacts[]` |
| `execution.completed` | `summary`, `artifacts[]` |

Platform callback response fields:

- `accepted`
- `continueAllowed`
- `recommendedAction.type`
- `recommendedAction.reason`

Allowed `recommendedAction.type` values:

- `continue`
- `pause`
- `cancel`

### C. Compatibility Probe Endpoints

The existing endpoints remain valid:

- `GET /api/v1/remote/hosts/:hostId/instances/:agentId/codeagent/health`
- `GET /api/v1/remote/hosts/:hostId/instances/:agentId/codeagent/version`
- `POST /api/v1/remote/hosts/:hostId/instances/:agentId/codeagent/run`

Their role is restricted to:

- provider binding verification
- ops diagnostics
- compatibility checks
- emergency manual probes

They are not the authoritative marketplace execution path anymore.

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

- platform-to-Carrier requests use provider-scoped bearer tokens
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

1. Add `CarrierBinding` and `ExecutionProfile` to the platform.
2. Implement provider-scoped binding verification.
3. Add async execution endpoints in Carrier.
4. Add `ExecutionJob`, callback verification, and reconciliation in the platform.
5. Move listings and bids to reference `executionProfileId`.
6. Keep existing probe endpoints for diagnostics and fallback.
7. Rewrite smoke tests so formal marketplace execution uses the async path.

## Acceptance Criteria

The integration is ready when all of these are true:

- a provider can bind Carrier resources without ops editing database rows
- a listing or bid cannot be published without an active execution profile
- an awarded order creates an execution job and receives signed Carrier callbacks
- duplicate and out-of-order callbacks do not corrupt job state
- a budget overrun pauses execution and later resumes after budget extension
- a dispute case can show artifact refs and usage proofs from Carrier
- binding suspension blocks new awards but does not break already-running orders
- the upstream Carrier PR can be described entirely by the checklist in [carrier-pr-support.md](./carrier-pr-support.md)
