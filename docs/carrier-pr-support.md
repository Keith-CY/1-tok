# Carrier Upstream PR Checklist

This document is the upstream-facing checklist derived from the authoritative design in [carrier-first-class-design.md](./carrier-first-class-design.md).

Use this document to coordinate the actual Carrier implementation or PR scope. If there is any conflict, the design doc wins.

## Goal

`Carrier` should support `1-tok` as a first-class marketplace execution substrate for provider-owned resources.

That means:

- each provider can issue a provider-scoped Carrier integration token to `1-tok`
- each awarded marketplace milestone can become one asynchronous Carrier execution
- Carrier can push signed lifecycle events and expose durable execution evidence

## What Carrier Developers Need To Build

This checklist is written for the Carrier project, not for `1-tok`.

Carrier developers should read it as a product-integration contract with four concrete outcomes:

1. A provider can safely delegate execution authority from Carrier to `1-tok` without exposing unrelated tenants or resources.
2. `1-tok` can create, observe, pause, resume, and cancel Carrier executions through a stable API contract.
3. Carrier can push settlement-grade execution evidence back to `1-tok` in a replay-safe way.
4. Both teams can run joint staging tests without special-case scripts or manual database edits.

If a planned Carrier implementation does not satisfy one of those outcomes, it is incomplete even if the raw endpoint exists.

## Required Carrier Coordination Artifacts

Carrier needs to provide these non-code deliverables as part of the integration work:

- a stable staging base URL for the `1-tok` integration namespace
- a documented flow for minting or rotating provider-scoped integration tokens
- at least one stable staging resource tuple: `hostId`, `agentId`, `backend`, `workspaceRoot`
- callback signing setup instructions, including how `keyId` and secret rotation work
- sample callback payloads for success, budget pause, failure, and artifact publication cases
- sample usage-proof and artifact payloads that match real Carrier behavior
- a named Carrier owner for contract questions and rollout approvals

These are required because `1-tok` cannot infer Carrier tenancy, signing, or execution semantics by reading the Carrier codebase.

## Required Carrier Deliverables

### 1. Keep the existing compatibility probes

Carrier must continue to support:

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/v1/remote/hosts/:hostId/instances/:agentId/codeagent/health` | resource verification and diagnostics |
| `GET` | `/api/v1/remote/hosts/:hostId/instances/:agentId/codeagent/version` | backend/version visibility |
| `POST` | `/api/v1/remote/hosts/:hostId/instances/:agentId/codeagent/run` | emergency probe and compatibility fallback |

All three paths should continue to accept `Authorization: Bearer <token>`.

### 2. Add a provider-scoped integration namespace

Recommended namespace:

- `/api/v1/integrations/one-tok/*`

Required endpoints:

| Method | Path | Purpose |
| --- | --- | --- |
| `POST` | `/api/v1/integrations/one-tok/bindings/verify` | verify provider host/agent/backend/workspace |
| `POST` | `/api/v1/integrations/one-tok/executions` | create one async execution |
| `GET` | `/api/v1/integrations/one-tok/executions/:id` | fetch authoritative execution state |
| `POST` | `/api/v1/integrations/one-tok/executions/:id/actions` | pause, resume, or cancel execution |

Carrier should treat this namespace as a versioned public contract for `1-tok`. Avoid coupling `1-tok` to internal-only routes, internal state names, or one-off staging shims.

### 3. Support provider-scoped bearer tokens

Carrier must support a provider-issued integration token with these properties:

- scoped to one provider account or tenant
- limited to that provider’s resources
- rotatable without hard downtime
- usable on both probe endpoints and async execution endpoints

### 4. Support signed async callbacks

Carrier must be able to push lifecycle events to the platform callback:

- `POST /v1/carrier/callbacks/events`

Required delivery guarantees:

- unique `eventId`
- monotonic `sequence` per execution
- retry-safe delivery with the same `eventId`
- HMAC signing using a binding-specific key and secret
- exponential retry on transient failure

Callback contract details that should shape the Carrier implementation:

- canonical callback path is `POST /v1/carrier/callbacks/events`
- canonical event names use dot-separated form such as `usage.reported` and `milestone.ready`
- the platform may temporarily keep a legacy `/v1/carrier/events` alias with snake_case names for internal bridge traffic, but Carrier upstream work should target the canonical path and names
- if the platform returns `accepted=false`, Carrier must retry the same `eventId` and `sequence` rather than minting a new event
- Carrier should expect the platform to reject a sequence gap until the missing earlier event is redelivered
- when the platform returns `recommendedAction.type=pause` or `cancel`, Carrier should treat it as authoritative control-plane feedback for that execution

Required event types:

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

### Callback callback auth and response mapping

Carrier should expect these platform callback responses from `POST /v1/carrier/callbacks/events`:

- `200` + `accepted=true`: callback accepted (new event or replay).
- `200` + `accepted=false`: transient platform conflict; same `eventId` should be retried with backoff.
- `400`: payload validation failed (missing required fields / malformed event / bad proof payload); Carrier should fix payload and retry only if the root cause is transient.
- `401`: authentication mismatch (wrong/missing secret or keyId); treat as an integration misconfiguration and alert instead of blind retries.
- `409`: sequence gap or out-of-order event; Carrier should replay missing earlier events to restore sequence continuity, preserving `eventId`/`sequence`.
- `5xx`: transport/runtime side failures; safe to retry with exponential backoff while preserving idempotency keys.

### 5. Support durable execution evidence


- execution status
- machine-readable failure category and code
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

### 6. Honor platform control-plane decisions

Carrier must treat the platform callback response as part of the control plane.

Required behavior:

- if callback response says `recommendedAction.type=continue`, Carrier may keep running
- if callback response says `recommendedAction.type=pause`, Carrier should pause as soon as safely possible and emit `execution.paused`
- if callback response says `recommendedAction.type=cancel`, Carrier should cancel as soon as safely possible and emit the matching terminal event
- if callback response says `accepted=false`, Carrier must retry the same callback delivery rather than assuming the platform observed the event

This prevents budget enforcement and dispute safety from depending on out-of-band operator action.

## Payload Expectations

### Existing `run` payload

`POST .../run` should continue to support the current request body:

- `hostId`
- `agentId`
- `backend`
- `workspaceRoot`
- `capability`
- `path`
- `content`
- `writeMode`
- `command`
- `cwd`
- `timeoutSec`
- `stdoutPath`
- `stderrPath`
- `appendOutput`
- `resumeSessionId`

The response envelope should continue to include:

- `backend`
- `result.ok`
- `result.policy_decision`
- `result.cost_estimate_usd`

### Async execution payload

`POST /api/v1/integrations/one-tok/executions` must support these fields:

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

The response must include:

- `carrierExecutionId`
- `accepted`
- `queueState`
- `estimatedStartAt`

The callback secret should be established during binding creation or credential rotation, then referenced by `callbacks.auth.keyId`. It should not be resent on every execution create request.

### Idempotency And Sequencing Expectations

- reusing the same `Idempotency-Key` on `POST /api/v1/integrations/one-tok/executions` must return the same `carrierExecutionId`
- retries of the same action request must be safe and must not double-apply `pause`, `resume`, or `cancel`
- replaying one callback delivery must reuse the same `eventId` and `sequence`; a logical retry must not mint a new identity
- status fetch should converge with the last accepted callback so the platform can reconcile stale jobs deterministically

### Minimum Joint Test Matrix

Carrier should be able to demonstrate all of the following in staging:

- verify one provider resource tuple successfully
- create one execution and return a durable `carrierExecutionId`
- emit `execution.accepted`, `execution.started`, `usage.reported`, `milestone.ready`, and `execution.completed`
- emit `budget.low` or honor a platform `pause` recommendation after usage is reported
- recover from one callback retry without creating duplicate execution history
- recover from one out-of-order callback by redelivering the missing earlier event
- expose at least one artifact reference and one usage proof for a completed execution

If Carrier cannot run this matrix, the integration is not ready for `1-tok` production rollout.

## Done Criteria For The Carrier PR

The Carrier work is complete when:

- a provider-scoped token can verify one host/agent/backend/workspace combination
- `1-tok` can create an async execution for one awarded milestone
- Carrier can push signed lifecycle events for that execution
- duplicate or retried callbacks are safe
- out-of-order callback gaps can be recovered by redelivering the missing event without inventing a new `eventId`
- Carrier honors platform `pause` or `cancel` recommendations returned from callback responses
- Carrier can expose logs, output artifacts, and usage proofs by execution id
- failure states are typed instead of only free-form strings
- no new Carrier work depends on the platform's legacy `/v1/carrier/events` alias or snake_case event names
- Carrier has provided the staging artifacts and ownership information listed above so both teams can complete joint integration

## Explicit Non-Goals For The First PR

The first Carrier PR does not need to include:

- interactive terminal streaming
- platform-managed host or agent creation
- multi-agent DAG orchestration
- buyer-facing UI work
- payment settlement logic inside Carrier
