import {
  type Bid,
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
  };
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
    hasProviderBid: boolean;
  }>;
  capabilities: string[];
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
  const [recommendedListings, orders, rfqs] = await Promise.all([
    getListings({ authToken: options.authToken, requireLive: options.requireLive }),
    getOrders({ authToken: options.authToken, requireLive: options.requireLive }),
    getRFQs({ authToken: options.authToken, requireLive: options.requireLive }),
  ]);
  const activeOrders = orders.filter((order) => order.buyerOrgId === options.buyerOrgId);
  const buyerRFQs = rfqs.filter((rfq) => rfq.buyerOrgId === options.buyerOrgId);
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
    },
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
    ],
  };
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
    (record) => record.kind === "withdrawal" && record.state !== "SETTLED",
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
      { id: "pipe_3", label: "Withdrawal queue", detail: `${inFlightWithdrawals} provider withdrawals are still in flight.` },
    ],
    activeOrders,
    marketQueue,
    marketOpportunities,
    capabilities: provider.capabilities,
  };
}

export async function getOpsDashboardData(options: { authToken: string; requireLive?: boolean }): Promise<OpsDashboardData> {
  const [providers, orders, fundingRecords, disputes] = await Promise.all([
    getProviders({ authToken: options.authToken, requireLive: options.requireLive }),
    getOrders({ authToken: options.authToken, requireLive: options.requireLive }),
    getFundingRecords({ authToken: options.authToken, requireLive: options.requireLive }),
    getDisputes({ authToken: options.authToken, requireLive: options.requireLive }),
  ]);
  const settledInvoices = fundingRecords.filter((record) => record.kind === "invoice" && record.state === "SETTLED").length;
  const pendingWithdrawals = fundingRecords.filter(
    (record) => record.kind === "withdrawal" && record.state !== "SETTLED",
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
      { id: "review_2", title: "Pending withdrawals", detail: `${pendingWithdrawals} settlement withdrawals still need completion or review.` },
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
