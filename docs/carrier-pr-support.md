# Carrier Upstream PR Checklist

This document is the upstream-facing checklist derived from the authoritative design in [carrier-first-class-design.md](./carrier-first-class-design.md).

Use this document to coordinate the actual Carrier implementation or PR scope. If there is any conflict, the design doc wins.

## Goal

`Carrier` should support `1-tok` as a first-class marketplace execution substrate for provider-owned resources.

That means:

- each provider can issue a provider-scoped Carrier integration token to `1-tok`
- each awarded marketplace milestone can become one asynchronous Carrier execution
- Carrier can push signed lifecycle events and expose durable execution evidence

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

### 5. Support durable execution evidence

Carrier must expose:

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

## Done Criteria For The Carrier PR

The Carrier work is complete when:

- a provider-scoped token can verify one host/agent/backend/workspace combination
- `1-tok` can create an async execution for one awarded milestone
- Carrier can push signed lifecycle events for that execution
- duplicate or retried callbacks are safe
- Carrier can expose logs, output artifacts, and usage proofs by execution id
- failure states are typed instead of only free-form strings

## Explicit Non-Goals For The First PR

The first Carrier PR does not need to include:

- interactive terminal streaming
- platform-managed host or agent creation
- multi-agent DAG orchestration
- buyer-facing UI work
- payment settlement logic inside Carrier
