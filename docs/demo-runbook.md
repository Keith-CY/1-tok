# Demo Runbook

## Before the session

10 minutes before start:

1. Open `/ops`.
2. If the environment is cold or has drifted, press `Prepare demo`.
3. Refresh `/ops` and confirm `Demo readiness` reads `ready`.
3. Log in the three browsers or profiles you will use:
   - buyer
   - provider
   - ops
4. Keep `/ops` open during the walkthrough.

If readiness stays `blocked`, stop and fix the blockers first. Do not improvise a live recovery path during the demo.

## Live sequence

1. Buyer opens `/buyer/topups`.
   Expected: existing USDI prefund is visible.

2. Buyer creates an RFQ from `/buyer/rfqs/create`.
   Expected: the new request appears in the buyer RFQ board.

3. Provider opens `/provider/rfqs` and submits a bid.
   Expected: the proposal appears on both buyer and provider views.

4. Buyer awards the bid.
   Expected: the order is created and the provider liquidity pool is already warm enough to proceed without rail setup drama.

5. Show streaming delivery progress.
   Expected: provider payout records move as usage is recorded.

6. Complete the milestone.
   Expected: order reaches `completed`, ops can still inspect funding records and disputes from the control plane.

## What to say

- Buyer-facing pricing is outcome-shaped, but settlement stays usage-aware underneath.
- Carrier owns execution. 1-tok owns the marketplace, budget control, and dispute/governance layer.
- Provider settlement runs over a real USDI rail with pre-warmed liquidity.
- Ops can answer “are we safe to demo?” from a single readiness strip.
