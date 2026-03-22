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
  proofSignature?: string;
  proofTimestamp?: string;
}

export interface Milestone {
  id: string;
  title: string;
  basePriceCents: number;
  budgetCents: number;
  settledCents: number;
  summary?: string;
  state: MilestoneState;
  disputeStatus: "none" | "open" | "resolved";
  usageCharges: UsageCharge[];
  anomalyFlags?: string[];
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

export interface Dispute {
  id: string;
  orderId: string;
  milestoneId: string;
  reason: string;
  refundCents: number;
  status: "open" | "resolved";
  resolution?: string;
  resolvedBy?: string;
  resolvedAt?: string;
  createdAt: string;
}

export interface ProviderProfile {
  id: string;
  name: string;
  capabilities: string[];
  reputationTier: string;
  rating?: number;
  ratingCount?: number;
}

export interface Listing {
  id: string;
  providerOrgId: string;
  title: string;
  category: string;
  basePriceCents: number;
  tags: string[];
}

export const rfqStatuses = ["open", "awarded", "closed"] as const;
export type RFQStatus = (typeof rfqStatuses)[number];

export interface RFQ {
  id: string;
  buyerOrgId: string;
  title: string;
  category: string;
  scope: string;
  budgetCents: number;
  status: RFQStatus;
  awardedBidId?: string;
  awardedProviderOrgId?: string;
  orderId?: string;
  responseDeadlineAt: string;
  createdAt: string;
  updatedAt: string;
}

export const bidStatuses = ["open", "awarded", "rejected"] as const;
export type BidStatus = (typeof bidStatuses)[number];

export interface BidMilestone {
  id: string;
  title: string;
  basePriceCents: number;
  budgetCents: number;
}

export interface Bid {
  id: string;
  rfqId: string;
  providerOrgId: string;
  message: string;
  quoteCents: number;
  status: BidStatus;
  milestones: BidMilestone[];
  createdAt: string;
  updatedAt: string;
}

export const fundingRecordKinds = ["invoice", "withdrawal", "buyer_topup", "provider_payout"] as const;
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
  destination?: Record<string, string>;
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

// Runtime validation guards

export function isFundingMode(value: unknown): value is FundingMode {
  return typeof value === "string" && (fundingModes as readonly string[]).includes(value);
}

export function isOrderStatus(value: unknown): value is OrderStatus {
  return typeof value === "string" && (orderStatuses as readonly string[]).includes(value);
}

export function isMilestoneState(value: unknown): value is MilestoneState {
  return typeof value === "string" && (milestoneStates as readonly string[]).includes(value);
}

export function isUsageChargeKind(value: unknown): value is UsageChargeKind {
  return typeof value === "string" && (usageChargeKinds as readonly string[]).includes(value);
}

export function assertFundingMode(value: unknown): asserts value is FundingMode {
  if (!isFundingMode(value)) throw new Error(`Invalid funding mode: ${value}`);
}

export function assertOrderStatus(value: unknown): asserts value is OrderStatus {
  if (!isOrderStatus(value)) throw new Error(`Invalid order status: ${value}`);
}

export function assertMilestoneState(value: unknown): asserts value is MilestoneState {
  if (!isMilestoneState(value)) throw new Error(`Invalid milestone state: ${value}`);
}

export function assertUsageChargeKind(value: unknown): asserts value is UsageChargeKind {
  if (!isUsageChargeKind(value)) throw new Error(`Invalid usage charge kind: ${value}`);
}

// --- Rating ---

export interface OrderRating {
  orderId: string;
  providerOrgId: string;
  buyerOrgId: string;
  score: number; // 1-5
  comment?: string;
  createdAt: string;
}

// --- Carrier ---

export const jobStates = ["pending", "running", "completed", "failed", "cancelled"] as const;
export type JobState = (typeof jobStates)[number];

export interface CarrierBinding {
  id: string;
  carrierId: string;
  orderId: string;
  milestoneId: string;
  capabilities: string[];
  boundAt: string;
  lastHeartbeat: string;
}

export interface ExecutionJob {
  id: string;
  bindingId: string;
  milestoneId: string;
  state: JobState;
  input?: string;
  output?: string;
  errorMessage?: string;
  progress?: { step: number; total: number; message: string };
  createdAt: string;
  startedAt?: string;
  completedAt?: string;
}

export function isJobState(value: unknown): value is JobState {
  return typeof value === "string" && (jobStates as readonly string[]).includes(value);
}

// --- Notifications ---

export const notificationEvents = [
  "order.created",
  "milestone.settled",
  "dispute.opened",
  "dispute.resolved",
  "rfq.awarded",
  "order.completed",
  "order.rated",
  "budget_wall.hit",
] as const;
export type NotificationEvent = (typeof notificationEvents)[number];

export interface Notification {
  id: string;
  event: NotificationEvent;
  target: string;
  payload: Record<string, unknown>;
  createdAt: string;
  delivered: boolean;
}

// --- Carrier Callback Ledger ---

export interface ExecutionEvent {
  id: string;
  executionId: string;
  eventId: string;
  sequence: number;
  eventType: string;
  attemptId?: string;
  payloadJson?: string;
  decisionJson?: string;
  receivedAt: string;
}

// --- Provider Vetting ---

export const vettingStatuses = ["pending", "approved", "rejected"] as const;
export type VettingStatus = (typeof vettingStatuses)[number];

export interface ProviderApplication {
  id: string;
  orgId: string;
  name: string;
  capabilities: string[];
  status: VettingStatus;
  reviewedBy?: string;
  reviewNote?: string;
  submittedAt: string;
  reviewedAt?: string;
}

// --- Provider Carrier Binding ---

export interface ProviderCarrierBinding {
  id: string;
  providerOrgId: string;
  carrierBaseUrl: string;
  hostId: string;
  agentId: string;
  backend: string;
  workspaceRoot: string;
  status: "pending_verification" | "active" | "suspended";
  createdAt: string;
  verifiedAt?: string;
}

// --- Order Budget ---

export interface MilestoneBudget {
  id: string;
  title: string;
  budgetCents: number;
  spentCents: number;
  settledCents: number;
  usagePercent: number;
  state: string;
}

export interface OrderBudgetSummary {
  orderId: string;
  totalBudgetCents: number;
  totalSpentCents: number;
  totalSettledCents: number;
  overallPercent: number;
  milestones: MilestoneBudget[];
}

// --- Marketplace Stats ---

export interface MarketplaceStats {
  totalProviders: number;
  totalListings: number;
  totalRfqs: number;
  openRfqs: number;
  totalOrders: number;
  activeOrders: number;
  totalDisputes: number;
  openDisputes: number;
  totalRatings: number;
  averageRating: number;
}

// --- Dispute With Evidence ---

export interface DisputeWithEvidence {
  dispute: {
    id: string;
    orderId: string;
    milestoneId: string;
    reason: string;
    refundCents: number;
    status: string;
    evidenceIds?: string[];
  };
  evidence?: unknown[];
}

// --- Timeline ---

export interface TimelineEvent {
  type: string;
  timestamp: string;
  details?: Record<string, unknown>;
}

// --- Leaderboard ---

export interface LeaderboardEntry {
  providerId: string;
  name: string;
  rating: number;
  ratingCount: number;
  totalOrders: number;
  reputationTier: string;
}
