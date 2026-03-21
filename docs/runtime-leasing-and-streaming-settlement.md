# Runtime Leasing And Streaming Settlement

Last updated: `2026-03-21`

This document clarifies the recommended commercial model for 1-tok and the corresponding payment flow.

## Positioning Decision

There are two possible product narratives:

1. **Buy outcomes** (result-based procurement)
2. **Lease runtime** (usage-based execution)

For 1-tok, the recommended primary positioning is:

- **Lease runtime, pay by token usage, settle continuously**

Reason:

- Provider cost is incurred continuously from token consumption.
- Fiber/payment-channel rails are strongest for high-frequency small-value transfer.
- Continuous settlement reduces provider float risk and improves supply willingness.
- Budget control and dispute evidence become easier with sequence-based usage proofs.

Result-based packaging can still exist as an upper product layer, but settlement should stay usage-native.

## Commercial One-Liner

Use this external framing:

> 1-tok is an agent runtime leasing marketplace: customers pre-fund budget, consume runtime, and pay by verified token usage through streaming micro-settlement.

## Funding Principles

### Zero Provider Float (required)

Provider pre-financing is not allowed.

Required policy:

- Customer must pre-fund balance into 1-tok escrow.
- Runtime can start only after reserve lock succeeds.
- 1-tok releases micro-payouts to provider during execution.
- Runtime pauses automatically when available balance drops below safe threshold.
- Remaining balance is returned to customer after terminal reconciliation (less platform fee/dispute hold where applicable).

## Settlement Model

Use multi-trigger incremental settlement. Trigger payout when any condition is met:

- **Token trigger**: consumed tokens since last settle exceed `T`
- **Amount trigger**: accrued amount since last settle exceed `A`
- **Time trigger**: elapsed time since last settle exceed `N`
- **Event trigger**: step/milestone boundary, pause, fail, complete

This avoids both extremes:

- too coarse (3-5 large payments per request)
- too noisy (excessively frequent tiny settlements)

## Suggested Defaults

### Balanced profile (default)

- `T = 5,000 tokens`
- `A = $0.08`
- `N = 20s`
- `min_settle_interval = 3s`
- `max_settle_interval = 30s`

### Guardrails

- budget warning at 70% / 85% / 95%
- automatic pause near depletion
- idempotent settlement key: `(sessionId, sequence)`
- replay-safe event ledger: stable `eventId + sequence`

## Reference State Flow

1. `prefunded` — customer escrow funded
2. `reserved` — minimum runtime reserve locked
3. `running` — runtime consuming tokens
4. `streaming_settlement` — incremental provider payouts
5. `paused_budget` — auto-pause on low available balance
6. `resumed` — continue after top-up
7. `terminal_reconcile` — complete/fail/cancel final reconciliation
8. `refunded` — unused funds return to customer

## Implementation Notes

- Keep billing truth on usage events, not UI milestones.
- Keep payouts monotonic and idempotent.
- Keep proof payload compact but dispute-grade (amount, token class, timestamp, signature, source event).
- Keep bridge compatibility, but ensure canonical callback/event semantics for future Carrier-native async path.

## Metrics To Track

- median settlement lag (usage -> payout)
- settlement events per runtime-hour
- provider receivable at risk (should remain near zero)
- auto-pause frequency due to low balance
- dispute rate on usage proofs
- settlement replay/duplication rejection rate
