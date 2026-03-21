# Runtime Leasing, Outcome Packaging, And Settlement Spec

Last updated: `2026-03-21`

This document is the top-level commercial and settlement spec for `1-tok`.

It defines how `1-tok` should package work for buyers, how funds move during execution, and which settlement rules are fixed before implementation.

Lower-level callback, replay, and API details already tracked elsewhere remain authoritative:

- [carrier-first-class-design.md](./carrier-first-class-design.md)
- [carrier-pr-support.md](./carrier-pr-support.md)
- [api-spec.json](./api-spec.json)

## Decision Summary

`1-tok` uses a two-layer model:

1. **Buyer-facing package layer**: customers can buy fixed-price, capped-price, or SLA-backed outcome packages.
2. **Settlement core**: the platform still funds and settles execution as runtime leasing with verified streaming usage settlement.

The core rule is:

- sell outcomes on the surface
- settle runtime underneath

This keeps the buyer experience simple without forcing providers to pre-finance runtime.

## External Framing

Use this framing externally:

> 1-tok sells outcome-oriented agent packages, but funds and settles them through a pre-funded runtime leasing model with verified streaming usage settlement.

## Funding Principles

### Zero Provider Float (required)

Provider pre-financing is not allowed.

Required policy:

- customer must pre-fund balance into `1-tok` escrow before runtime starts
- runtime can start only after reserve lock succeeds
- `1-tok` releases provider payouts incrementally during execution
- runtime pauses automatically when available balance is no longer safe
- terminal reconciliation returns unused customer balance after fees, dispute holdback, and approved adjustments

### Reserve Lock

Default reserve formula:

`reserve = estimated_usage * unit_cost_snapshot * safety_factor + fixed_buffer`

Definitions:

- `estimated_usage` is the expected usage envelope for the awarded runtime window or milestone
- `unit_cost_snapshot` is the provider/model/usage-class price basis captured when the reserve is locked
- `safety_factor` protects against estimation error and bursty usage
- `fixed_buffer` protects against short runtime spikes, callback lag, and settlement rounding

Policy rules:

- reserve calculation must use a versioned price snapshot from the awarded provider/model configuration
- price refreshes may update future reserve extensions, but must not retroactively reprice already accepted usage
- runtime cannot start or resume unless the reserve lock covers the next safe execution window

## Commercial Layers

### Runtime Settlement Layer

Runtime usage is the billing truth for execution.

Accepted billable usage remains usage-native:

- step
- token
- external_api

Streaming settlement continuously converts verified usage into provider payout events.

### Outcome Package Layer

Buyer-facing offers may still be fixed-price, capped-price, or SLA-backed.

That package layer does not replace runtime settlement. Instead, it constrains or adjusts the final commercial outcome for a milestone.

### Milestone Delta Settlement

When a milestone reaches `milestone.ready`, the platform performs a package-layer delta settlement.

This delta settlement:

- aligns cumulative runtime settlement with the awarded milestone commercial terms
- can add a fixed package remainder, capped-price adjustment, or SLA-linked premium/penalty
- does not reopen or replace previously accepted runtime usage proofs

`execution.completed` closes execution bookkeeping, but `milestone.ready` remains the only event that can enter milestone settlement.

## Settlement Model

Use multi-trigger incremental settlement for runtime usage. A settlement attempt becomes eligible when any condition is met:

- **Token trigger**: consumed tokens since last settlement exceeds `T`
- **Amount trigger**: accrued billable amount since last settlement exceeds `A`
- **Time trigger**: elapsed time since last settlement exceeds `N`
- **Event trigger**: pause, fail, cancel, `milestone.ready`, or terminal reconcile

### Trigger Precedence

The trigger rule is "any condition makes the session eligible", but settlement still follows guardrails:

1. `min_settle_interval` throttles normal non-terminal settlement attempts so bursts do not create noisy micro-settlements.
2. `max_settle_interval` forces a flush when there is unsettled billable usage, even if token or amount thresholds are not reached.
3. Forced events such as pause, fail, cancel, `milestone.ready`, and terminal reconcile bypass `min_settle_interval`.
4. `milestone.ready` may trigger both a final runtime flush for the milestone and the package-layer delta settlement.

This removes the ambiguity between "any trigger fires" and interval guardrails:

- triggers decide **eligibility**
- guardrails decide **when the eligible settlement is allowed or forced**

## Suggested Defaults

### Balanced Runtime Profile

- `T = 5,000 tokens`
- `A = $0.08`
- `N = 20s`
- `min_settle_interval = 3s`
- `max_settle_interval = 30s`

### Runtime Guardrails

- budget warning at 70% / 85% / 95%
- automatic pause near depletion
- idempotent runtime settlement key: `(sessionId, sequence)`
- replay-safe event ledger identity: stable `eventId + sequence`

## Fee, Pause, And Dispute Defaults

### Platform Fee Timing

Platform fees are deducted proportionally on both settlement layers:

- each streaming runtime settlement deducts its proportional platform fee
- each milestone delta settlement deducts its proportional platform fee

Refund behavior is symmetric:

- any approved refund reverses the underlying commercial amount and platform fee in the same proportion

### Low-Balance Pause And Resume

Low-balance behavior is fixed:

- runtime pauses when available balance falls below the safe execution threshold
- top-up must restore the reserve threshold for the next execution window
- if the balance is restored and there is no new risk signal, the runtime resumes automatically
- if new risk or credit signals appear while paused, the runtime stays paused pending review

### Terminal Reconcile And Dispute Window

Terminal settlement behavior is fixed:

- terminal reconcile runs on complete, fail, or cancel
- after terminal reconcile, the platform opens a default `72h` dispute window
- default dispute holdback is `10%` of recognized milestone gross, capped by the remaining milestone release amount
- if the dispute window expires without an open case, the holdback is released automatically

## Reference State Flow

1. `prefunded` - customer escrow funded
2. `reserved` - runtime reserve locked
3. `running` - runtime consuming billable usage
4. `streaming_settlement` - incremental runtime payouts
5. `paused_budget` - auto-pause on low balance
6. `resumed` - automatic or approved restart after top-up
7. `milestone_ready` - milestone evidence accepted for package-layer settlement
8. `terminal_reconcile` - final usage, fee, holdback, and refund calculation
9. `dispute_window` - timed holdback period after reconcile
10. `refunded_or_released` - unused funds refunded and holdback released or adjusted

## Protocol Boundary

This document sets the commercial and settlement rules. The lower-level protocol documents remain authoritative for wire details:

- callback ingress path: `POST /api/v1/carrier/callbacks/events`
- canonical event names such as `usage.reported`, `milestone.ready`, `execution.paused`, and `execution.resumed`
- replay semantics and sequence handling
- callback response mapping such as `200 accepted`, `200 replay`, and `409` for gaps or out-of-order events
- execution and webhook API shapes

Use these documents when implementing the wire contract:

- [carrier-first-class-design.md](./carrier-first-class-design.md)
- [carrier-pr-support.md](./carrier-pr-support.md)
- [api-spec.json](./api-spec.json)

## Implementation Notes

- keep billing truth on verified usage events, not UI milestones
- keep payouts monotonic, idempotent, and replay-safe
- keep outcome packages as a commercial layer on top of runtime truth, not a replacement for it
- keep pause, resume, and milestone settlement semantics aligned with the Carrier callback contract
- keep evidence compact but dispute-grade: `sessionId`, `sequence`, `eventId`, `timestamp`, `signature`, usage kind, and amount

## Metrics To Track

- median settlement lag from usage to payout
- settlement events per runtime-hour
- provider receivable at risk
- auto-pause frequency due to low balance
- milestone delta settlement frequency and average delta size
- dispute rate on usage proofs
- replay and duplication rejection rate
