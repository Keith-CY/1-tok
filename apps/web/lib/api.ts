import {
  type Bid,
  type BuyerDepositSummary,
  type DemoStatus,
  type Dispute,
  type FundingRecord,
  type Listing,
  type Order,
  type ProviderProfile,
  type RFQ,
} from "@1tok/contracts";
import { demoProviders, demoListings, demoOrders, demoFundingRecords, demoDisputes, demoRFQs, demoBids } from "./fixtures";

export interface CollectionRequestOptions {
  authToken?: string;
  requireLive?: boolean;
}

export interface BuyerDashboardData {
  summary: {
    activeOrders: number;
    availableListings: number;
    openRFQs: number;
    pausedOrders: number;
    buyerOrgId: string;
    prepaidBalanceCents: number;
    settledTopUps: number;
    pendingTopUps: number;
  };
  deposit: BuyerDepositSummary | null;
  recommendedListings: Listing[];
  activeOrders: Order[];
  rfqBook: Array<{
    id: string;
    title: string;
    status: string;
    budgetCents: number;
    bidCount: number;
    responseDeadlineAt: string;
    bids: Array<{ id: string; providerOrgId: string; quoteCents: number; status: string }>;
  }>;
  inbox: Array<{ id: string; title: string; detail: string }>;
}

export interface ProviderDashboardData {
  summary: {
    activeOrders: number;
    openRFQs: number;
    submittedBids: number;
    settledInvoices: number;
    inFlightWithdrawals: number;
    reputationTier: string;
    providerName: string;
  };
  pipeline: Array<{ id: string; label: string; detail: string }>;
  activeOrders: Order[];
  marketQueue: Array<{
    id: string;
    rfqId: string;
    orderId?: string;
    title: string;
    buyerOrgId: string;
    budgetCents: number;
    responseDeadlineAt: string;
    providerBidStatus: string;
    quoteCents: number;
  }>;
  marketOpportunities: Array<{
    id: string;
    title: string;
    buyerOrgId: string;
    budgetCents: number;
    responseDeadlineAt: string;
    proposalCount: number;
    lowestQuoteCents: number | null;
    hasProviderBid: boolean;
  }>;
  capabilities: string[];
}

export interface ProviderRFQDetail {
  rfq: RFQ;
  providerBid: Bid | null;
}

export interface ProviderOrderDetail {
  order: Order;
  rfq: RFQ | null;
  fundingRecords: FundingRecord[];
}
export interface OpsDashboardData {
  summary: {
    activeOrders: number;
    fundingRecords: number;
    settledInvoices: number;
    pendingWithdrawals: number;
    openDisputes: number;
  };
  pendingReviews: Array<{ id: string; title: string; detail: string }>;
  treasurySignals: Array<{ id: string; label: string; value: string; tone: "mint" | "warning" | "danger" }>;
  riskFeed: Array<{ id: string; title: string; detail: string }>;
  fundingRecords: FundingRecord[];
  disputes: Dispute[];
  demoStatus: DemoStatus | null;
}


export async function getProviders(options?: CollectionRequestOptions): Promise<ProviderProfile[]> {
  return readCollection("/api/v1/providers", "providers", demoProviders, options);
}

export async function getListings(options?: CollectionRequestOptions): Promise<Listing[]> {
  return readCollection("/api/v1/listings", "listings", demoListings, options);
}

export async function getOrders(options?: CollectionRequestOptions): Promise<Order[]> {
  return readCollection("/api/v1/orders", "orders", demoOrders, options);
}

export async function getRFQs(options?: CollectionRequestOptions): Promise<RFQ[]> {
  return readCollection("/api/v1/rfqs", "rfqs", demoRFQs, options);
}

export async function getRFQBids(rfqId: string, options?: CollectionRequestOptions): Promise<Bid[]> {
  return readCollection(`/api/v1/rfqs/${rfqId}/bids`, "bids", demoBids.filter((bid) => bid.rfqId === rfqId), options);
}

export async function getFundingRecords(options?: CollectionRequestOptions): Promise<FundingRecord[]> {
  const baseUrl = resolveBaseUrl("settlement");
  if (!baseUrl) {
    return options?.requireLive ? [] : demoFundingRecords;
  }

  return readCollectionFromBase(baseUrl, "/v1/funding-records", "records", demoFundingRecords, options);
}

export async function getDisputes(options?: CollectionRequestOptions): Promise<Dispute[]> {
  return readCollection("/api/v1/disputes", "disputes", demoDisputes, options);
}

export async function getBuyerDashboardData(options: {
  authToken: string;
  buyerOrgId: string;
  requireLive?: boolean;
}): Promise<BuyerDashboardData> {
  const [recommendedListings, orders, rfqs, fundingRecords, deposit] = await Promise.all([
    getListings({ authToken: options.authToken, requireLive: options.requireLive }),
    getOrders({ authToken: options.authToken, requireLive: options.requireLive }),
    getRFQs({ authToken: options.authToken, requireLive: options.requireLive }),
    getFundingRecords({ authToken: options.authToken, requireLive: options.requireLive }),
    getBuyerDepositSummary({
      authToken: options.authToken,
      buyerOrgId: options.buyerOrgId,
      requireLive: options.requireLive,
    }),
  ]);
  const activeOrders = orders.filter((order) => order.buyerOrgId === options.buyerOrgId);
  const buyerRFQs = rfqs.filter((rfq) => rfq.buyerOrgId === options.buyerOrgId);
  const buyerFunding = fundingRecords.filter((record) => record.buyerOrgId === options.buyerOrgId);
  const settledTopUps = buyerFunding.filter((record) => record.kind === "buyer_topup" && record.state === "SETTLED");
  const pendingTopUps = buyerFunding.filter((record) => record.kind === "buyer_topup" && record.state !== "SETTLED");
  const committedPrepaidCents = activeOrders.reduce((sum, order) => {
    if (order.fundingMode !== "prepaid") return sum;
    if (
      order.status !== "running" &&
      order.status !== "awaiting_budget" &&
      order.status !== "awaiting_payment_rail"
    ) {
      return sum;
    }
    return sum + order.milestones.reduce((milestoneSum, milestone) => {
      if (milestone.state === "settled") return milestoneSum;
      const unsettled = Number(milestone.budgetCents ?? 0) - Number(milestone.settledCents ?? 0);
      return unsettled > 0 ? milestoneSum + unsettled : milestoneSum;
    }, 0);
  }, 0);
  const creditedTopUpCents = settledTopUps.reduce((sum, record) => sum + parseAmountToCents(record.amount), 0);
  const pausedOrders = activeOrders.filter(
    (order) =>
      order.status === "awaiting_budget" || order.milestones.some((milestone) => milestone.state === "paused"),
  ).length;
  const rfqBook = await Promise.all(
    buyerRFQs.map(async (rfq) => {
      const bids = await getRFQBids(rfq.id, {
        authToken: options.authToken,
        requireLive: options.requireLive,
      });
      return {
        id: rfq.id,
        title: rfq.title,
        status: rfq.status,
        budgetCents: rfq.budgetCents,
        bidCount: bids.length,
        responseDeadlineAt: rfq.responseDeadlineAt,
        bids: bids.map((bid) => ({
          id: bid.id,
          providerOrgId: bid.providerOrgId,
          quoteCents: bid.quoteCents,
          status: bid.status,
        })),
      };
    }),
  );

  return {
    summary: {
      activeOrders: activeOrders.length,
      availableListings: recommendedListings.length,
      openRFQs: buyerRFQs.filter((rfq) => rfq.status === "open").length,
      pausedOrders,
      buyerOrgId: options.buyerOrgId,
      prepaidBalanceCents: Math.max(0, creditedTopUpCents - committedPrepaidCents),
      settledTopUps: settledTopUps.length,
      pendingTopUps: pendingTopUps.length,
    },
    deposit,
    recommendedListings,
    activeOrders,
    rfqBook,
    inbox: [
      {
        id: "msg_1",
        title: "Open order pressure",
        detail: `${activeOrders.length} orders currently belong to ${options.buyerOrgId}.`,
      },
      {
        id: "msg_2",
        title: "RFQ watch",
        detail: `${rfqBook.length} RFQs are active for this buyer session, with ${rfqBook.reduce((sum, rfq) => sum + rfq.bidCount, 0)} bids in play.`,
      },
      {
        id: "msg_3",
        title: "USDI prefund",
        detail: `${settledTopUps.length} settled top-ups and ${pendingTopUps.length} pending top-ups are visible for this buyer.`,
      },
    ],
  };
}

export async function getBuyerDepositSummary(options: {
  authToken: string;
  buyerOrgId: string;
  requireLive?: boolean;
}): Promise<BuyerDepositSummary | null> {
  const baseUrl = resolveBaseUrl("settlement");
  if (!baseUrl) {
    return null;
  }

  return readJSONFromBase<BuyerDepositSummary>(
    baseUrl,
    `/v1/buyer/deposit-address?buyerOrgId=${encodeURIComponent(options.buyerOrgId)}`,
    options,
  );
}

export async function getProviderDashboardData(options: {
  authToken: string;
  providerOrgId: string;
  requireLive?: boolean;
}): Promise<ProviderDashboardData> {
  const [providers, orders, fundingRecords, rfqs] = await Promise.all([
    getProviders({ authToken: options.authToken, requireLive: options.requireLive }),
    getOrders({ authToken: options.authToken, requireLive: options.requireLive }),
    getFundingRecords({ authToken: options.authToken, requireLive: options.requireLive }),
    getRFQs({ authToken: options.authToken, requireLive: options.requireLive }),
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
    (record) => (record.kind === "withdrawal" || record.kind === "provider_payout") && record.state !== "SETTLED",
  ).length;
  const rfqBidGroups = await Promise.all(
    rfqs.map(async (rfq) => ({
      rfq,
      bids: await getRFQBids(rfq.id, {
        authToken: options.authToken,
        requireLive: options.requireLive,
      }),
    })),
  );
  const marketQueue = rfqBidGroups.flatMap(({ rfq, bids }) =>
    bids
      .filter((bid) => bid.providerOrgId === options.providerOrgId)
      .map((bid) => ({
        id: `${rfq.id}:${bid.id}`,
        rfqId: rfq.id,
        orderId: rfq.orderId,
        title: rfq.title,
        buyerOrgId: rfq.buyerOrgId,
        budgetCents: rfq.budgetCents,
        responseDeadlineAt: rfq.responseDeadlineAt,
        providerBidStatus: bid.status,
        quoteCents: bid.quoteCents,
      })),
  );
  const marketOpportunities = rfqBidGroups
    .filter(({ rfq }) => rfq.status === "open")
    .map(({ rfq, bids }) => ({
      id: rfq.id,
      title: rfq.title,
      buyerOrgId: rfq.buyerOrgId,
      budgetCents: rfq.budgetCents,
      responseDeadlineAt: rfq.responseDeadlineAt,
      proposalCount: bids.length,
      lowestQuoteCents: bids.length ? Math.min(...bids.map((bid) => bid.quoteCents)) : null,
      hasProviderBid: bids.some((bid) => bid.providerOrgId === options.providerOrgId),
    }));

  return {
    summary: {
      activeOrders: activeOrders.length,
      openRFQs: rfqs.filter((rfq) => rfq.status === "open").length,
      submittedBids: marketQueue.length,
      settledInvoices,
      inFlightWithdrawals,
      reputationTier: provider.reputationTier,
      providerName: provider.name,
    },
    pipeline: [
      { id: "pipe_1", label: "Active orders", detail: `${activeOrders.length} orders currently tied to ${provider.name}.` },
      { id: "pipe_2", label: "Submitted bids", detail: `${marketQueue.length} RFQ responses currently belong to ${provider.name}.` },
      { id: "pipe_3", label: "Payout queue", detail: `${inFlightWithdrawals} provider payouts are still in flight.` },
    ],
    activeOrders,
    marketQueue,
    marketOpportunities,
    capabilities: provider.capabilities,
  };
}

export async function getProviderRFQDetail(options: {
  authToken: string;
  providerOrgId: string;
  rfqId: string;
  requireLive?: boolean;
}): Promise<ProviderRFQDetail | null> {
  const [rfqs, bids] = await Promise.all([
    getRFQs({ authToken: options.authToken, requireLive: options.requireLive }),
    getRFQBids(options.rfqId, { authToken: options.authToken, requireLive: options.requireLive }),
  ]);

  const rfq = rfqs.find((candidate) => candidate.id === options.rfqId);
  if (!rfq) {
    return null;
  }

  const providerBid = bids.find((bid) => bid.providerOrgId === options.providerOrgId) ?? null;
  if (rfq.status !== "open" && !providerBid) {
    return null;
  }

  return {
    rfq,
    providerBid,
  };
}

export async function getProviderOrderDetail(options: {
  authToken: string;
  providerOrgId: string;
  orderId: string;
  requireLive?: boolean;
}): Promise<ProviderOrderDetail | null> {
  const [orders, fundingRecords, rfqs] = await Promise.all([
    getOrders({ authToken: options.authToken, requireLive: options.requireLive }),
    getFundingRecords({ authToken: options.authToken, requireLive: options.requireLive }),
    getRFQs({ authToken: options.authToken, requireLive: options.requireLive }),
  ]);

  const order = orders.find((candidate) => candidate.id === options.orderId && candidate.providerOrgId === options.providerOrgId);
  if (!order) {
    return null;
  }

  return {
    order,
    rfq: rfqs.find((candidate) => candidate.orderId === order.id) ?? null,
    fundingRecords: fundingRecords.filter(
      (record) =>
        record.orderId === order.id &&
        (!record.providerOrgId || record.providerOrgId === options.providerOrgId),
    ),
  };
}
export async function getOpsDashboardData(options: { authToken: string; requireLive?: boolean }): Promise<OpsDashboardData> {
  const [providers, orders, fundingRecords, disputes, demoStatus] = await Promise.all([
    getProviders({ authToken: options.authToken, requireLive: options.requireLive }),
    getOrders({ authToken: options.authToken, requireLive: options.requireLive }),
    getFundingRecords({ authToken: options.authToken, requireLive: options.requireLive }),
    getDisputes({ authToken: options.authToken, requireLive: options.requireLive }),
    getDemoStatus({ authToken: options.authToken, requireLive: options.requireLive }),
  ]);
  const settledInvoices = fundingRecords.filter((record) => record.kind === "invoice" && record.state === "SETTLED").length;
  const pendingWithdrawals = fundingRecords.filter(
    (record) => (record.kind === "withdrawal" || record.kind === "provider_payout") && record.state !== "SETTLED",
  ).length;
  const openDisputes = disputes.filter((dispute) => dispute.status !== "resolved");

  return {
    summary: {
      activeOrders: orders.length,
      fundingRecords: fundingRecords.length,
      settledInvoices,
      pendingWithdrawals,
      openDisputes: openDisputes.length,
    },
    pendingReviews: [
      { id: "review_1", title: "Open disputes", detail: `${openDisputes.length} disputes currently need reimbursement or recovery review.` },
      { id: "review_2", title: "Pending payouts", detail: `${pendingWithdrawals} settlement payouts still need completion or review.` },
    ],
    treasurySignals: [
      { id: "sig_1", label: "Funding records", value: `${fundingRecords.length}`, tone: "warning" },
      { id: "sig_2", label: "Settled invoices", value: `${settledInvoices}`, tone: "mint" },
      { id: "sig_3", label: "Open disputes", value: `${openDisputes.length}`, tone: "danger" },
    ],
    riskFeed: [
      { id: "risk_1", title: "Dispute pressure", detail: `${openDisputes.length} disputes are visible to ops in the current control plane.` },
      { id: "risk_2", title: "Catalog posture", detail: `${providers.length} providers remain available in the marketplace catalog.` },
    ],
    fundingRecords,
    disputes,
    demoStatus,
  };
}

export async function getDemoStatus(options: { authToken: string; requireLive?: boolean }): Promise<DemoStatus | null> {
  const baseUrl = resolveBaseUrl("api");
  if (!baseUrl) {
    return null;
  }

  try {
    const response = await fetch(`${baseUrl}/api/v1/ops/demo/status`, {
      headers: {
        Accept: "application/json",
        Authorization: `Bearer ${options.authToken}`,
      },
      cache: "no-store",
    });
    if (!response.ok) {
      return null;
    }
    const payload = (await response.json()) as { status?: DemoStatus };
    return payload.status ?? null;
  } catch {
    return options.requireLive ? null : null;
  }
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

async function readJSONFromBase<T>(
  baseUrl: string,
  path: string,
  options?: CollectionRequestOptions,
): Promise<T | null> {
  try {
    const response = await fetch(`${baseUrl}${path}`, {
      headers: {
        Accept: "application/json",
        ...(options?.authToken ? { Authorization: `Bearer ${options.authToken}` } : {}),
      },
      cache: "no-store",
    });
    if (!response.ok) {
      return null;
    }
    return (await response.json()) as T;
  } catch {
    return null;
  }
}

function resolveBaseUrl(kind: "api" | "settlement"): string | null {
  if (kind === "settlement") {
    return process.env.NEXT_PUBLIC_SETTLEMENT_BASE_URL?.replace(/\/$/, "") ?? null;
  }
  return process.env.NEXT_PUBLIC_API_BASE_URL?.replace(/\/$/, "") ?? null;
}

function parseAmountToCents(amount: string): number {
  const parsed = Number.parseFloat(amount);
  if (Number.isNaN(parsed)) {
    return 0;
  }
  return Math.round(parsed * 100);
}

// --- New v1 API functions ---

export async function rateOrder(
  orderId: string,
  score: number,
  comment: string,
  options?: { authToken?: string }
): Promise<{ rating: { orderId: string; score: number; comment: string } }> {
  const baseUrl = process.env.NEXT_PUBLIC_API_BASE_URL ?? "";
  const headers: Record<string, string> = { "Content-Type": "application/json" };
  if (options?.authToken) headers["Authorization"] = `Bearer ${options.authToken}`;

  const res = await fetch(`${baseUrl}/api/v1/orders/${orderId}/rating`, {
    method: "POST",
    headers,
    body: JSON.stringify({ score, comment }),
  });
  if (!res.ok) throw new Error(`rate order: ${res.status}`);
  return res.json();
}

export async function searchListings(params: {
  q?: string;
  category?: string;
  tag?: string;
  minPrice?: number;
  maxPrice?: number;
}): Promise<{ listings: Listing[]; pagination: { total: number } }> {
  const baseUrl = process.env.NEXT_PUBLIC_API_BASE_URL ?? "";
  const query = new URLSearchParams();
  if (params.q) query.set("q", params.q);
  if (params.category) query.set("category", params.category);
  if (params.tag) query.set("tag", params.tag);
  if (params.minPrice) query.set("minPrice", String(params.minPrice));
  if (params.maxPrice) query.set("maxPrice", String(params.maxPrice));

  const res = await fetch(`${baseUrl}/api/v1/listings?${query}`);
  if (!res.ok) throw new Error(`search listings: ${res.status}`);
  return res.json();
}

export async function getRFQMessages(
  rfqId: string,
  options?: { authToken?: string }
): Promise<{ messages: Array<{ id: string; rfqId: string; author: string; body: string; createdAt: string }> }> {
  const baseUrl = process.env.NEXT_PUBLIC_API_BASE_URL ?? "";
  const headers: Record<string, string> = {};
  if (options?.authToken) headers["Authorization"] = `Bearer ${options.authToken}`;

  const res = await fetch(`${baseUrl}/api/v1/rfqs/${rfqId}/messages`, { headers });
  if (!res.ok) throw new Error(`rfq messages: ${res.status}`);
  return res.json();
}

export async function createRFQMessage(
  rfqId: string,
  author: string,
  body: string,
  options?: { authToken?: string }
): Promise<{ message: { id: string; rfqId: string; author: string; body: string } }> {
  const baseUrl = process.env.NEXT_PUBLIC_API_BASE_URL ?? "";
  const headers: Record<string, string> = { "Content-Type": "application/json" };
  if (options?.authToken) headers["Authorization"] = `Bearer ${options.authToken}`;

  const res = await fetch(`${baseUrl}/api/v1/rfqs/${rfqId}/messages`, {
    method: "POST",
    headers,
    body: JSON.stringify({ author, body }),
  });
  if (!res.ok) throw new Error(`create rfq message: ${res.status}`);
  return res.json();
}

export async function bindCarrier(
  orderId: string,
  milestoneId: string,
  carrierId: string,
  capabilities: string[],
  options?: { authToken?: string }
): Promise<{ binding: { id: string; carrierId: string } }> {
  const baseUrl = process.env.NEXT_PUBLIC_API_BASE_URL ?? "";
  const headers: Record<string, string> = { "Content-Type": "application/json" };
  if (options?.authToken) headers["Authorization"] = `Bearer ${options.authToken}`;

  const res = await fetch(`${baseUrl}/api/v1/orders/${orderId}/milestones/${milestoneId}/bind-carrier`, {
    method: "POST",
    headers,
    body: JSON.stringify({ carrierId, capabilities }),
  });
  if (!res.ok) throw new Error(`bind carrier: ${res.status}`);
  return res.json();
}

export async function getJob(
  jobId: string,
  options?: { authToken?: string }
): Promise<{ job: import("@1tok/contracts").ExecutionJob }> {
  const baseUrl = process.env.NEXT_PUBLIC_API_BASE_URL ?? "";
  const headers: Record<string, string> = {};
  if (options?.authToken) headers["Authorization"] = `Bearer ${options.authToken}`;

  const res = await fetch(`${baseUrl}/api/v1/jobs/${jobId}`, { headers });
  if (!res.ok) throw new Error(`get job: ${res.status}`);
  return res.json();
}
