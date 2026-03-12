export const fundingModes = ["prepaid", "credit"] as const;
export type FundingMode = (typeof fundingModes)[number];

export const orderStatuses = [
  "draft",
  "running",
  "awaiting_budget",
  "completed",
  "failed",
] as const;
export type OrderStatus = (typeof orderStatuses)[number];

export const milestoneStates = [
  "pending",
  "running",
  "paused",
  "settled",
] as const;
export type MilestoneState = (typeof milestoneStates)[number];

export const usageChargeKinds = [
  "step",
  "token",
  "external_api",
] as const;
export type UsageChargeKind = (typeof usageChargeKinds)[number];

export interface UsageCharge {
  kind: UsageChargeKind;
  amountCents: number;
  proofRef?: string;
}

export interface Milestone {
  id: string;
  title: string;
  basePriceCents: number;
  budgetCents: number;
  settledCents: number;
  summary?: string;
  state: MilestoneState;
  disputeStatus: "none" | "open";
  usageCharges: UsageCharge[];
}

export interface Order {
  id: string;
  buyerOrgId: string;
  providerOrgId: string;
  fundingMode: FundingMode;
  creditLineId?: string;
  platformWallet?: string;
  status: OrderStatus;
  milestones: Milestone[];
}

export interface CreditHistory {
  completedOrders: number;
  successfulPayments: number;
  failedPayments: number;
  disputedOrders: number;
  lifetimeSpendCents: number;
}

export interface CreditDecision {
  approved: boolean;
  recommendedLimitCents: number;
  reason: string;
}

export interface ProviderProfile {
  id: string;
  name: string;
  capabilities: string[];
  reputationTier: string;
}

export interface Listing {
  id: string;
  providerOrgId: string;
  title: string;
  category: string;
  basePriceCents: number;
  tags: string[];
}

export const fundingRecordKinds = ["invoice", "withdrawal"] as const;
export type FundingRecordKind = (typeof fundingRecordKinds)[number];

export interface FundingRecord {
  id: string;
  kind: FundingRecordKind;
  orderId?: string;
  milestoneId?: string;
  buyerOrgId?: string;
  providerOrgId?: string;
  asset?: string;
  amount: string;
  invoice?: string;
  externalId?: string;
  state: string;
  createdAt?: string;
  updatedAt?: string;
}

export const sampleBuyerSummary = {
  activeOrders: 8,
  remainingCreditCents: 138_000,
  openDisputes: 1,
  prepaidBalanceCents: 42_500,
};

export const sampleProviderSummary = {
  activeOrders: 11,
  reputationTier: "gold",
  heldPayoutCents: 12_000,
  availablePayoutCents: 91_300,
};

export const sampleOpsSummary = {
  pendingProviderReviews: 7,
  openDisputes: 3,
  outstandingExposureCents: 223_000,
  activeChannels: 14,
};

export function formatMoney(cents: number): string {
  return new Intl.NumberFormat("en-US", {
    style: "currency",
    currency: "USD",
    minimumFractionDigits: 2,
  }).format(cents / 100);
}
