# Carrier PR Support Contract

This document captures the `1-tok -> carrier` contract we want to upstream into `carrier`. The current repo still uses [mock-carrier](/Users/ChenYu/Documents/Github/1-tok/internal/mockcarrier/server.go), but the goal is to converge the mock onto a real Carrier surface and later submit that support upstream as a PR.

## Current must-have endpoints

The execution service currently depends on three synchronous control-plane endpoints:

| Method | Path | Purpose | Current caller |
| --- | --- | --- | --- |
| `GET` | `/api/v1/remote/hosts/:hostId/instances/:agentId/codeagent/health` | Validate the remote codeagent is reachable and healthy for the selected backend/workspace | [client.go](/Users/ChenYu/Documents/Github/1-tok/internal/integrations/carrier/client.go) |
| `GET` | `/api/v1/remote/hosts/:hostId/instances/:agentId/codeagent/version` | Expose the actual backend/version string for compatibility checks and ops visibility | [client.go](/Users/ChenYu/Documents/Github/1-tok/internal/integrations/carrier/client.go) |
| `POST` | `/api/v1/remote/hosts/:hostId/instances/:agentId/codeagent/run` | Run a bounded capability on the remote codeagent and return a policy decision/result envelope | [client.go](/Users/ChenYu/Documents/Github/1-tok/internal/integrations/carrier/client.go) |

All three paths should accept `Authorization: Bearer <token>`.

## Run payload we need Carrier to support

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

And the response envelope should continue to include:

- `backend`
- `result.ok`
- `result.policy_decision`
- `result.cost_estimate_usd`

## Event delivery we want next

Beyond the synchronous control-plane calls, `1-tok` needs Carrier to be able to emit execution lifecycle events toward the execution service route [server.go](/Users/ChenYu/Documents/Github/1-tok/internal/services/execution/server.go):

- `POST /v1/carrier/events`
- Auth: `X-One-Tok-Service-Token`

Current event payload fields:

- `orderId`
- `milestoneId`
- `eventType`
- `usageKind`
- `amountCents`
- `proofRef`
- `summary`

Current event types consumed by `1-tok`:

- `milestone_ready`
- `usage_reported`
- `budget_low`

Event types we should standardize next for upstream Carrier support:

- `order_started`
- `milestone_started`
- `heartbeat`
- `pause_required`
- `order_failed`
- `order_completed`

## Behavioral expectations

Carrier support should eventually guarantee:

- idempotent retry-safe event delivery
- explicit timeout semantics for `run`
- stable backend naming
- policy decision values that are machine-readable
- a way to correlate async events back to the original `run` or task execution
- artifact references for stdout/stderr/log bundles rather than only inline summaries

## What is still mocked today

The current mock implementation always returns success and does not yet model:

- remote workspace creation/fetch
- streamed logs or progressive run state
- resumable long-running sessions
- artifact upload/download
- typed failure categories
- async event push from Carrier to `1-tok`

Those are the gaps we should close before or during the upstream Carrier PR.
