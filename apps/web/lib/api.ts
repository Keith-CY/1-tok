import {
  type FundingRecord,
  formatMoney,
  sampleBuyerSummary,
  type Listing,
  type Order,
  type ProviderProfile,
} from "@1tok/contracts";

export interface CollectionRequestOptions {
  authToken?: string;
  requireLive?: boolean;
}

export interface BuyerDashboardData {
  summary: typeof sampleBuyerSummary;
  recommendedListings: Listing[];
  activeOrders: Order[];
  inbox: Array<{ id: string; title: string; detail: string }>;
}

export interface ProviderDashboardData {
  summary: {
    activeOrders: number;
    settledInvoices: number;
    inFlightWithdrawals: number;
    reputationTier: string;
    providerName: string;
  };
  pipeline: Array<{ id: string; label: string; detail: string }>;
  activeOrders: Order[];
  capabilities: string[];
}

export interface OpsDashboardData {
  summary: {
    activeOrders: number;
    fundingRecords: number;
    settledInvoices: number;
    pendingWithdrawals: number;
  };
  pendingReviews: Array<{ id: string; title: string; detail: string }>;
  treasurySignals: Array<{ id: string; label: string; value: string; tone: "mint" | "warning" | "danger" }>;
  riskFeed: Array<{ id: string; title: string; detail: string }>;
  fundingRecords: FundingRecord[];
}

const demoProviders: ProviderProfile[] = [
  {
    id: "provider_1",
    name: "Atlas Ops",
    capabilities: ["Carrier lifecycle", "Token metering", "Fast dispute traces"],
    reputationTier: "gold",
  },
  {
    id: "provider_2",
    name: "Kite Relay",
    capabilities: ["Provider pooling", "Usage proofs", "Milestone tuning"],
    reputationTier: "silver",
  },
];

const demoListings: Listing[] = [
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

const demoOrders: Order[] = [
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

const demoFundingRecords: FundingRecord[] = [
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

export async function getProviders(options?: CollectionRequestOptions): Promise<ProviderProfile[]> {
  return readCollection("/api/v1/providers", "providers", demoProviders, options);
}

export async function getListings(options?: CollectionRequestOptions): Promise<Listing[]> {
  return readCollection("/api/v1/listings", "listings", demoListings, options);
}

export async function getOrders(options?: CollectionRequestOptions): Promise<Order[]> {
  return readCollection("/api/v1/orders", "orders", demoOrders, options);
}

export async function getFundingRecords(options?: CollectionRequestOptions): Promise<FundingRecord[]> {
  const baseUrl = resolveBaseUrl("settlement");
  if (!baseUrl) {
    return options?.requireLive ? [] : demoFundingRecords;
  }

  return readCollectionFromBase(baseUrl, "/v1/funding-records", "records", demoFundingRecords, options);
}

export async function getBuyerDashboardData(): Promise<BuyerDashboardData> {
  const [recommendedListings, activeOrders] = await Promise.all([getListings(), getOrders()]);

  return {
    summary: sampleBuyerSummary,
    recommendedListings,
    activeOrders,
    inbox: [
      {
        id: "msg_1",
        title: "Carrier paused for top-up",
        detail: "Order ord_18 crossed the milestone budget ceiling and is waiting for authorization.",
      },
      {
        id: "msg_2",
        title: "Credit refreshed overnight",
        detail: `Your available credit is now ${formatMoney(sampleBuyerSummary.remainingCreditCents)}.`,
      },
    ],
  };
}

export async function getProviderDashboardData(options: {
  authToken: string;
  providerOrgId: string;
  requireLive?: boolean;
}): Promise<ProviderDashboardData> {
  const [providers, orders, fundingRecords] = await Promise.all([
    getProviders({ authToken: options.authToken, requireLive: options.requireLive }),
    getOrders({ authToken: options.authToken, requireLive: options.requireLive }),
    getFundingRecords({ authToken: options.authToken, requireLive: options.requireLive }),
  ]);
  const provider = providers.find((candidate) => candidate.id === options.providerOrgId) ?? {
    id: options.providerOrgId,
    name: options.providerOrgId,
    capabilities: [],
    reputationTier: "unknown",
  };
  const activeOrders = orders.filter((order) => order.providerOrgId === options.providerOrgId);
  const providerFunding = fundingRecords.filter((record) => record.providerOrgId === options.providerOrgId);
  const settledInvoices = providerFunding.filter((record) => record.kind === "invoice" && record.state === "SETTLED").length;
  const inFlightWithdrawals = providerFunding.filter(
    (record) => record.kind === "withdrawal" && record.state !== "SETTLED",
  ).length;

  return {
    summary: {
      activeOrders: activeOrders.length,
      settledInvoices,
      inFlightWithdrawals,
      reputationTier: provider.reputationTier,
      providerName: provider.name,
    },
    pipeline: [
      { id: "pipe_1", label: "Active orders", detail: `${activeOrders.length} orders currently tied to ${provider.name}.` },
      { id: "pipe_2", label: "Settled invoices", detail: `${settledInvoices} invoice funding records have already settled.` },
      { id: "pipe_3", label: "Withdrawal queue", detail: `${inFlightWithdrawals} provider withdrawals are still in flight.` },
    ],
    activeOrders,
    capabilities: provider.capabilities,
  };
}

export async function getOpsDashboardData(options: { authToken: string; requireLive?: boolean }): Promise<OpsDashboardData> {
  const [providers, orders, fundingRecords] = await Promise.all([
    getProviders({ authToken: options.authToken, requireLive: options.requireLive }),
    getOrders({ authToken: options.authToken, requireLive: options.requireLive }),
    getFundingRecords({ authToken: options.authToken, requireLive: options.requireLive }),
  ]);
  const settledInvoices = fundingRecords.filter((record) => record.kind === "invoice" && record.state === "SETTLED").length;
  const pendingWithdrawals = fundingRecords.filter(
    (record) => record.kind === "withdrawal" && record.state !== "SETTLED",
  ).length;

  return {
    summary: {
      activeOrders: orders.length,
      fundingRecords: fundingRecords.length,
      settledInvoices,
      pendingWithdrawals,
    },
    pendingReviews: [
      { id: "review_1", title: "Provider coverage", detail: `${providers.length} provider profiles are currently published in the catalog.` },
      { id: "review_2", title: "Pending withdrawals", detail: `${pendingWithdrawals} settlement withdrawals still need completion or review.` },
    ],
    treasurySignals: [
      { id: "sig_1", label: "Funding records", value: `${fundingRecords.length}`, tone: "warning" },
      { id: "sig_2", label: "Settled invoices", value: `${settledInvoices}`, tone: "mint" },
      { id: "sig_3", label: "Pending withdrawals", value: `${pendingWithdrawals}`, tone: "danger" },
    ],
    riskFeed: [
      { id: "risk_1", title: "Order volume", detail: `${orders.length} orders are visible to ops in the current control plane.` },
      { id: "risk_2", title: "Catalog posture", detail: `${providers.length} providers remain available in the marketplace catalog.` },
    ],
    fundingRecords,
  };
}

async function readCollection<T>(path: string, key: string, fallback: T[], options?: CollectionRequestOptions): Promise<T[]> {
  const baseUrl = resolveBaseUrl("api");
  if (!baseUrl) {
    return options?.requireLive ? [] : fallback;
  }

  return readCollectionFromBase(baseUrl, path, key, fallback, options);
}

async function readCollectionFromBase<T>(
  baseUrl: string,
  path: string,
  key: string,
  fallback: T[],
  options?: CollectionRequestOptions,
): Promise<T[]> {
  const empty: T[] = [];
  try {
    const response = await fetch(`${baseUrl}${path}`, {
      headers: {
        Accept: "application/json",
        ...(options?.authToken ? { Authorization: `Bearer ${options.authToken}` } : {}),
      },
      cache: "no-store",
    });

    if (!response.ok) {
      return options?.requireLive ? empty : fallback;
    }

    const payload = (await response.json()) as Record<string, unknown>;
    const value = payload[key];

    return Array.isArray(value) ? (value as T[]) : options?.requireLive ? empty : fallback;
  } catch {
    return options?.requireLive ? empty : fallback;
  }
}

function resolveBaseUrl(kind: "api" | "settlement"): string | null {
  if (kind === "settlement") {
    return process.env.NEXT_PUBLIC_SETTLEMENT_BASE_URL?.replace(/\/$/, "") ?? null;
  }
  return process.env.NEXT_PUBLIC_API_BASE_URL?.replace(/\/$/, "") ?? null;
}
