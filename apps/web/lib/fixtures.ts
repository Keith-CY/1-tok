// Demo/seed data fixtures for development and testing.
// Extracted from api.ts per #34.
import {
  type Bid,
  type Dispute,
  type FundingRecord,
  type Listing,
  type Order,
  type ProviderProfile,
  type RFQ,
} from "@1tok/contracts";

export const demoProviders: ProviderProfile[] = [
  {
    id: "provider_1",
    name: "Atlas Ops",
    capabilities: ["Carrier lifecycle", "Token metering", "Fast dispute traces"],
    reputationTier: "gold",
    rating: 4.8,
    ratingCount: 23,
  },
  {
    id: "provider_2",
    name: "Kite Relay",
    capabilities: ["Provider pooling", "Usage proofs", "Milestone tuning"],
    reputationTier: "silver",
    rating: 4.2,
    ratingCount: 11,
  },
];

export const demoListings: Listing[] = [
  {
    id: "listing_1",
    providerOrgId: "provider_1",
    title: "Managed agent operations",
    category: "agent-ops",
    basePriceCents: 1500,
    tags: ["carrier-compatible", "instant-settle"],
  },
  {
    id: "listing_2",
    providerOrgId: "provider_2",
    title: "Prompt + toolchain intervention",
    category: "agent-runtime",
    basePriceCents: 2200,
    tags: ["token-metered", "budget-pausing"],
  },
];

export const demoOrders: Order[] = [
  {
    id: "ord_14",
    buyerOrgId: "buyer_1",
    providerOrgId: "provider_1",
    fundingMode: "credit",
    creditLineId: "credit_1",
    platformWallet: "platform_main",
    status: "running",
    milestones: [
      {
        id: "ms_1",
        title: "Execution design",
        basePriceCents: 1200,
        budgetCents: 1800,
        settledCents: 1200,
        summary: "Carrier accepted and completed the control-plane install.",
        state: "settled",
        disputeStatus: "none",
        usageCharges: [{ kind: "token", amountCents: 180, proofRef: "evt_431" }],
      },
      {
        id: "ms_2",
        title: "Provider validation",
        basePriceCents: 600,
        budgetCents: 1200,
        settledCents: 0,
        state: "running",
        disputeStatus: "none",
        usageCharges: [],
      },
    ],
  },
  {
    id: "ord_18",
    buyerOrgId: "buyer_7",
    providerOrgId: "provider_2",
    fundingMode: "prepaid",
    platformWallet: "platform_main",
    status: "awaiting_budget",
    milestones: [
      {
        id: "ms_1",
        title: "Prompt rehearsal",
        basePriceCents: 900,
        budgetCents: 1000,
        settledCents: 0,
        state: "paused",
        disputeStatus: "none",
        usageCharges: [{ kind: "external_api", amountCents: 140, proofRef: "evt_887" }],
      },
    ],
  },
];

export const demoFundingRecords: FundingRecord[] = [
  {
    id: "fund_1",
    kind: "invoice",
    orderId: "ord_14",
    milestoneId: "ms_1",
    buyerOrgId: "buyer_1",
    providerOrgId: "provider_1",
    asset: "CKB",
    amount: "12.5",
    invoice: "inv_123",
    state: "SETTLED",
  },
  {
    id: "fund_2",
    kind: "withdrawal",
    providerOrgId: "provider_2",
    asset: "USDI",
    amount: "10",
    externalId: "wd_123",
    state: "PROCESSING",
  },
];

export const demoDisputes: Dispute[] = [
  {
    id: "disp_1",
    orderId: "ord_14",
    milestoneId: "ms_1",
    reason: "Carrier summary did not match the actual remediation performed.",
    refundCents: 900,
    status: "open",
    createdAt: "2026-03-12T00:00:00Z",
  },
  {
    id: "disp_2",
    orderId: "ord_18",
    milestoneId: "ms_1",
    reason: "Evidence pack was delivered after reimbursement.",
    refundCents: 400,
    status: "resolved",
    resolution: "Ops accepted the provider evidence and closed the case.",
    resolvedBy: "usr_ops_7",
    resolvedAt: "2026-03-12T04:00:00Z",
    createdAt: "2026-03-12T02:00:00Z",
  },
];

export const demoRFQs: RFQ[] = [
  {
    id: "rfq_1",
    buyerOrgId: "buyer_1",
    title: "Agent runtime triage",
    category: "agent-ops",
    scope: "Investigate runtime failures, stabilize the worker, and document fallout.",
    budgetCents: 5400,
    status: "open",
    responseDeadlineAt: "2026-03-15T12:00:00Z",
    createdAt: "2026-03-12T00:00:00Z",
    updatedAt: "2026-03-12T00:00:00Z",
  },
  {
    id: "rfq_2",
    buyerOrgId: "buyer_7",
    title: "Prompt intervention sprint",
    category: "agent-runtime",
    scope: "Tune prompts and guardrails before a customer launch.",
    budgetCents: 6200,
    status: "awarded",
    awardedBidId: "bid_2",
    awardedProviderOrgId: "provider_2",
    orderId: "ord_18",
    responseDeadlineAt: "2026-03-16T12:00:00Z",
    createdAt: "2026-03-12T00:00:00Z",
    updatedAt: "2026-03-12T00:00:00Z",
  },
];

export const demoBids: Bid[] = [
  {
    id: "bid_1",
    rfqId: "rfq_1",
    providerOrgId: "provider_1",
    message: "Atlas Ops can take live triage within the hour.",
    quoteCents: 4800,
    status: "open",
    milestones: [
      { id: "ms_1", title: "Triage", basePriceCents: 1800, budgetCents: 2200 },
      { id: "ms_2", title: "Stabilize", basePriceCents: 3000, budgetCents: 3600 },
    ],
    createdAt: "2026-03-12T00:00:00Z",
    updatedAt: "2026-03-12T00:00:00Z",
  },
  {
    id: "bid_2",
    rfqId: "rfq_2",
    providerOrgId: "provider_2",
    message: "Kite Relay can own the prompt rehearsal path.",
    quoteCents: 5900,
    status: "awarded",
    milestones: [{ id: "ms_1", title: "Prompt rehearsal", basePriceCents: 5900, budgetCents: 6200 }],
    createdAt: "2026-03-12T00:00:00Z",
    updatedAt: "2026-03-12T00:00:00Z",
  },
];
