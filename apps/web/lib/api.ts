import {
  type FundingRecord,
  formatMoney,
  sampleBuyerSummary,
  sampleOpsSummary,
  sampleProviderSummary,
  type Listing,
  type Order,
  type ProviderProfile,
} from "@1tok/contracts";

export interface BuyerDashboardData {
  summary: typeof sampleBuyerSummary;
  recommendedListings: Listing[];
  activeOrders: Order[];
  inbox: Array<{ id: string; title: string; detail: string }>;
}

export interface ProviderDashboardData {
  summary: typeof sampleProviderSummary;
  pipeline: Array<{ id: string; label: string; detail: string }>;
  activeOrders: Order[];
  capabilities: string[];
}

export interface OpsDashboardData {
  summary: typeof sampleOpsSummary;
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

export async function getProviders(): Promise<ProviderProfile[]> {
  return readCollection("/api/v1/providers", "providers", demoProviders);
}

export async function getListings(): Promise<Listing[]> {
  return readCollection("/api/v1/listings", "listings", demoListings);
}

export async function getOrders(): Promise<Order[]> {
  return readCollection("/api/v1/orders", "orders", demoOrders);
}

export async function getFundingRecords(): Promise<FundingRecord[]> {
  const baseUrl = resolveBaseUrl("settlement");
  if (!baseUrl) {
    return demoFundingRecords;
  }

  return readCollectionFromBase(baseUrl, "/v1/funding-records", "records", demoFundingRecords);
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

export async function getProviderDashboardData(): Promise<ProviderDashboardData> {
  const [providers, activeOrders] = await Promise.all([getProviders(), getOrders()]);
  const provider = providers[0] ?? demoProviders[0];

  return {
    summary: sampleProviderSummary,
    pipeline: [
      { id: "pipe_1", label: "RFQs live", detail: "4 open requests, 2 waiting on buyer confirmation." },
      { id: "pipe_2", label: "Probation watch", detail: "1 new provider is inside the first 20-order window." },
      { id: "pipe_3", label: "Payout hook health", detail: "All Carrier hook callbacks are under 220ms p95." },
    ],
    activeOrders,
    capabilities: provider.capabilities,
  };
}

export async function getOpsDashboardData(): Promise<OpsDashboardData> {
  const fundingRecords = await getFundingRecords();

  return {
    summary: sampleOpsSummary,
    pendingReviews: [
      { id: "review_1", title: "Atlas Ops limit uplift", detail: "Rule engine suggested +$2,400 credit after 22 clean settlements." },
      { id: "review_2", title: "Kite Relay provider verification", detail: "Carrier heartbeat is stable, but payout signature window needs confirmation." },
    ],
    treasurySignals: [
      { id: "sig_1", label: "Outstanding exposure", value: formatMoney(sampleOpsSummary.outstandingExposureCents), tone: "warning" },
      { id: "sig_2", label: "Channel health", value: `${sampleOpsSummary.activeChannels} active`, tone: "mint" },
      { id: "sig_3", label: "Open disputes", value: `${sampleOpsSummary.openDisputes}`, tone: "danger" },
    ],
    riskFeed: [
      { id: "risk_1", title: "Delayed buyer settlement", detail: "buyer_7 is 38 minutes past the expected credit reconciliation checkpoint." },
      { id: "risk_2", title: "Budget wall hit", detail: "3 orders paused this morning because token + API usage exceeded milestone ceilings." },
    ],
    fundingRecords,
  };
}

async function readCollection<T>(path: string, key: string, fallback: T[]): Promise<T[]> {
  const baseUrl = resolveBaseUrl("api");
  if (!baseUrl) {
    return fallback;
  }

  return readCollectionFromBase(baseUrl, path, key, fallback);
}

async function readCollectionFromBase<T>(baseUrl: string, path: string, key: string, fallback: T[]): Promise<T[]> {
  try {
    const response = await fetch(`${baseUrl}${path}`, {
      headers: { Accept: "application/json" },
      cache: "no-store",
    });

    if (!response.ok) {
      return fallback;
    }

    const payload = (await response.json()) as Record<string, unknown>;
    const value = payload[key];

    return Array.isArray(value) ? (value as T[]) : fallback;
  } catch {
    return fallback;
  }
}

function resolveBaseUrl(kind: "api" | "settlement"): string | null {
  if (kind === "settlement") {
    return process.env.NEXT_PUBLIC_SETTLEMENT_BASE_URL?.replace(/\/$/, "") ?? null;
  }
  return process.env.NEXT_PUBLIC_API_BASE_URL?.replace(/\/$/, "") ?? null;
}
