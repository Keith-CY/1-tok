import { afterEach, describe, expect, it, mock } from "bun:test";

import { getBuyerDashboardData, getDemoStatus, getFundingRecords, getListings, getOpsDashboardData, getOrders, getProviderDashboardData } from "./api";

const originalFetch = globalThis.fetch;

afterEach(() => {
  globalThis.fetch = originalFetch;
  delete process.env.NEXT_PUBLIC_API_BASE_URL;
  delete process.env.NEXT_PUBLIC_SETTLEMENT_BASE_URL;
});

describe("api fallback", () => {
  it("uses demo listings when the API base url is missing", async () => {
    const listings = await getListings();

    expect(listings.length).toBeGreaterThan(0);
    expect(listings[0]?.id).toBe("listing_1");
  });

  it("falls back when remote order fetch fails", async () => {
    process.env.NEXT_PUBLIC_API_BASE_URL = "http://localhost:9999";
    globalThis.fetch = mock(async () => {
      throw new Error("network down");
    }) as unknown as typeof fetch;

    const orders = await getOrders();

    expect(orders.length).toBeGreaterThan(0);
    expect(orders[0]?.id).toBe("ord_14");
  });

  it("reads funding records from the settlement base url", async () => {
    process.env.NEXT_PUBLIC_SETTLEMENT_BASE_URL = "http://localhost:8083";
    globalThis.fetch = mock(async (input: RequestInfo | URL) => {
      expect(String(input)).toBe("http://localhost:8083/v1/funding-records");

      return new Response(
        JSON.stringify({
          records: [{ id: "fund_1", kind: "invoice", amount: "12.5", state: "SETTLED" }],
        }),
        {
          headers: { "Content-Type": "application/json" },
          status: 200,
        },
      );
    }) as unknown as typeof fetch;

    const records = await getFundingRecords();

    expect(records).toHaveLength(1);
    expect(records[0]?.id).toBe("fund_1");
    expect(records[0]?.state).toBe("SETTLED");
  });

  it("forwards bearer auth when reading funding records", async () => {
    process.env.NEXT_PUBLIC_SETTLEMENT_BASE_URL = "http://localhost:8083";
    globalThis.fetch = mock(async (_input: RequestInfo | URL, init?: RequestInit) => {
      expect((init?.headers as Record<string, string>)?.Authorization).toBe("Bearer tok_123");

      return new Response(
        JSON.stringify({
          records: [{ id: "fund_1", kind: "invoice", amount: "12.5", state: "SETTLED" }],
        }),
        {
          headers: { "Content-Type": "application/json" },
          status: 200,
        },
      );
    }) as unknown as typeof fetch;

    const records = await getFundingRecords({ authToken: "tok_123" });

    expect(records).toHaveLength(1);
  });

  it("falls back to demo funding records when settlement fetch fails", async () => {
    process.env.NEXT_PUBLIC_SETTLEMENT_BASE_URL = "http://localhost:8083";
    globalThis.fetch = mock(async () => {
      throw new Error("settlement unavailable");
    }) as unknown as typeof fetch;

    const records = await getFundingRecords();

    expect(records.length).toBeGreaterThan(0);
    expect(records[0]?.id).toBe("fund_topup_1");
  });

  it("returns an empty authenticated funding list when live settlement fetch fails", async () => {
    process.env.NEXT_PUBLIC_SETTLEMENT_BASE_URL = "http://localhost:8083";
    globalThis.fetch = mock(async () => {
      throw new Error("settlement unavailable");
    }) as unknown as typeof fetch;

    const records = await getFundingRecords({ authToken: "tok_123", requireLive: true });

    expect(records).toHaveLength(0);
  });

  it("builds buyer dashboard data from live listings and buyer-scoped orders", async () => {
    process.env.NEXT_PUBLIC_API_BASE_URL = "http://localhost:8080";
    globalThis.fetch = mock(async (input: RequestInfo | URL) => {
      const url = String(input);

      if (url.endsWith("/api/v1/listings")) {
        return new Response(
          JSON.stringify({
            listings: [{ id: "listing_live", providerOrgId: "provider_1", title: "Live listing", category: "agent-ops", basePriceCents: 1200, tags: [] }],
          }),
          {
            headers: { "Content-Type": "application/json" },
            status: 200,
          },
        );
      }

      if (url.endsWith("/api/v1/orders")) {
        return new Response(
          JSON.stringify({
            orders: [
              { id: "ord_live_1", buyerOrgId: "buyer_1", providerOrgId: "provider_1", fundingMode: "credit", platformWallet: "platform_main", status: "running", milestones: [] },
              { id: "ord_live_2", buyerOrgId: "buyer_2", providerOrgId: "provider_1", fundingMode: "credit", platformWallet: "platform_main", status: "running", milestones: [] },
            ],
          }),
          {
            headers: { "Content-Type": "application/json" },
            status: 200,
          },
        );
      }

      if (url.endsWith("/api/v1/rfqs")) {
        return new Response(
          JSON.stringify({
            rfqs: [
              { id: "rfq_live_1", buyerOrgId: "buyer_1", title: "Live RFQ", category: "agent-ops", scope: "Investigate", budgetCents: 1200, status: "open", responseDeadlineAt: "2026-03-15T12:00:00Z", createdAt: "2026-03-12T00:00:00Z", updatedAt: "2026-03-12T00:00:00Z" },
            ],
          }),
          {
            headers: { "Content-Type": "application/json" },
            status: 200,
          },
        );
      }

      if (url.endsWith("/api/v1/rfqs/rfq_live_1/bids")) {
        return new Response(
          JSON.stringify({
            bids: [
              { id: "bid_live_1", rfqId: "rfq_live_1", providerOrgId: "provider_1", message: "Bid 1", quoteCents: 900, status: "open", milestones: [], createdAt: "2026-03-12T00:00:00Z", updatedAt: "2026-03-12T00:00:00Z" },
              { id: "bid_live_2", rfqId: "rfq_live_1", providerOrgId: "provider_2", message: "Bid 2", quoteCents: 1100, status: "open", milestones: [], createdAt: "2026-03-12T00:00:00Z", updatedAt: "2026-03-12T00:00:00Z" },
            ],
          }),
          {
            headers: { "Content-Type": "application/json" },
            status: 200,
          },
        );
      }

      throw new Error(`unexpected url ${url}`);
    }) as unknown as typeof fetch;

    const data = await getBuyerDashboardData({
      authToken: "tok_123",
      buyerOrgId: "buyer_1",
      requireLive: true,
    });

    expect(data.summary.activeOrders).toBe(1);
    expect(data.summary.availableListings).toBe(1);
    expect(data.summary.openRFQs).toBe(1);
    expect(data.activeOrders).toHaveLength(1);
    expect(data.activeOrders[0]?.id).toBe("ord_live_1");
    expect(data.recommendedListings[0]?.id).toBe("listing_live");
    expect(data.rfqBook[0]?.bidCount).toBe(2);
    expect(data.rfqBook[0]?.id).toBe("rfq_live_1");
    expect(data.rfqBook[0]?.bids[0]?.id).toBe("bid_live_1");
  });

  it("builds provider dashboard data from live rfqs and provider bids", async () => {
    process.env.NEXT_PUBLIC_API_BASE_URL = "http://localhost:8080";
    process.env.NEXT_PUBLIC_SETTLEMENT_BASE_URL = "http://localhost:8083";
    globalThis.fetch = mock(async (input: RequestInfo | URL) => {
      const url = String(input);

      if (url.endsWith("/api/v1/providers")) {
        return new Response(
          JSON.stringify({
            providers: [{ id: "provider_1", name: "Atlas Ops", capabilities: ["carrier"], reputationTier: "gold" }],
          }),
          {
            headers: { "Content-Type": "application/json" },
            status: 200,
          },
        );
      }

      if (url.endsWith("/api/v1/orders")) {
        return new Response(
          JSON.stringify({
            orders: [
              { id: "ord_live_1", buyerOrgId: "buyer_1", providerOrgId: "provider_1", fundingMode: "credit", platformWallet: "platform_main", status: "running", milestones: [] },
            ],
          }),
          {
            headers: { "Content-Type": "application/json" },
            status: 200,
          },
        );
      }

      if (url.endsWith("/v1/funding-records")) {
        return new Response(
          JSON.stringify({
            records: [{ id: "fund_1", kind: "invoice", providerOrgId: "provider_1", amount: "12.5", state: "SETTLED" }],
          }),
          {
            headers: { "Content-Type": "application/json" },
            status: 200,
          },
        );
      }

      if (url.endsWith("/api/v1/rfqs")) {
        return new Response(
          JSON.stringify({
            rfqs: [
              { id: "rfq_live_1", buyerOrgId: "buyer_1", title: "Live RFQ", category: "agent-ops", scope: "Investigate", budgetCents: 2200, status: "open", responseDeadlineAt: "2026-03-15T12:00:00Z", createdAt: "2026-03-12T00:00:00Z", updatedAt: "2026-03-12T00:00:00Z" },
              { id: "rfq_live_2", buyerOrgId: "buyer_2", title: "Won RFQ", category: "agent-ops", scope: "Deliver", budgetCents: 3200, status: "awarded", responseDeadlineAt: "2026-03-16T12:00:00Z", createdAt: "2026-03-12T00:00:00Z", updatedAt: "2026-03-12T00:00:00Z", awardedBidId: "bid_live_2", awardedProviderOrgId: "provider_1" },
            ],
          }),
          {
            headers: { "Content-Type": "application/json" },
            status: 200,
          },
        );
      }

      if (url.endsWith("/api/v1/rfqs/rfq_live_1/bids")) {
        return new Response(
          JSON.stringify({
            bids: [{ id: "bid_live_1", rfqId: "rfq_live_1", providerOrgId: "provider_1", message: "Open bid", quoteCents: 1800, status: "open", milestones: [], createdAt: "2026-03-12T00:00:00Z", updatedAt: "2026-03-12T00:00:00Z" }],
          }),
          {
            headers: { "Content-Type": "application/json" },
            status: 200,
          },
        );
      }

      if (url.endsWith("/api/v1/rfqs/rfq_live_2/bids")) {
        return new Response(
          JSON.stringify({
            bids: [{ id: "bid_live_2", rfqId: "rfq_live_2", providerOrgId: "provider_1", message: "Awarded bid", quoteCents: 2800, status: "awarded", milestones: [], createdAt: "2026-03-12T00:00:00Z", updatedAt: "2026-03-12T00:00:00Z" }],
          }),
          {
            headers: { "Content-Type": "application/json" },
            status: 200,
          },
        );
      }

      throw new Error(`unexpected url ${url}`);
    }) as unknown as typeof fetch;

    const data = await getProviderDashboardData({
      authToken: "tok_123",
      providerOrgId: "provider_1",
      requireLive: true,
    });

    expect(data.summary.activeOrders).toBe(1);
    expect(data.summary.submittedBids).toBe(2);
    expect(data.summary.openRFQs).toBe(1);
    expect(data.marketQueue).toHaveLength(2);
    expect((data.marketQueue[0] as { rfqId?: string } | undefined)?.rfqId).toBe("rfq_live_1");
    expect(data.marketQueue[0]?.providerBidStatus).toBe("open");
    expect(data.marketOpportunities).toHaveLength(1);
    expect(data.marketOpportunities[0]?.id).toBe("rfq_live_1");
    expect(data.marketOpportunities[0]?.proposalCount).toBe(1);
    expect(data.marketOpportunities[0]?.lowestQuoteCents).toBe(1800);
  });

  it("reads provider task detail from the same live rfq dataset", async () => {
    process.env.NEXT_PUBLIC_API_BASE_URL = "http://localhost:8080";
    globalThis.fetch = mock(async (input: RequestInfo | URL) => {
      const url = String(input);

      if (url.endsWith("/api/v1/rfqs")) {
        return new Response(
          JSON.stringify({
            rfqs: [
              { id: "rfq_live_1", buyerOrgId: "buyer_1", title: "Live RFQ", category: "agent-ops", scope: "Investigate", budgetCents: 2200, status: "open", responseDeadlineAt: "2026-03-15T12:00:00Z", createdAt: "2026-03-12T00:00:00Z", updatedAt: "2026-03-12T00:00:00Z" },
            ],
          }),
          {
            headers: { "Content-Type": "application/json" },
            status: 200,
          },
        );
      }

      if (url.endsWith("/api/v1/rfqs/rfq_live_1/bids")) {
        return new Response(
          JSON.stringify({
            bids: [{ id: "bid_live_1", rfqId: "rfq_live_1", providerOrgId: "provider_1", message: "Open bid", quoteCents: 1800, status: "open", milestones: [], createdAt: "2026-03-12T00:00:00Z", updatedAt: "2026-03-12T00:00:00Z" }],
          }),
          {
            headers: { "Content-Type": "application/json" },
            status: 200,
          },
        );
      }

      throw new Error(`unexpected url ${url}`);
    }) as unknown as typeof fetch;

    const apiModule = await import("./api");
    const detail = await (apiModule as { getProviderRFQDetail?: (options: {
      authToken: string;
      providerOrgId: string;
      rfqId: string;
      requireLive?: boolean;
    }) => Promise<{
      rfq: { id: string; status: string };
      providerBid: { id: string; status: string } | null;
    } | null> }).getProviderRFQDetail?.({
      authToken: "tok_123",
      providerOrgId: "provider_1",
      rfqId: "rfq_live_1",
      requireLive: true,
    });

    expect(detail).toEqual({
      rfq: expect.objectContaining({ id: "rfq_live_1", status: "open" }),
      providerBid: expect.objectContaining({ id: "bid_live_1", status: "open" }),
    });
  });

  it("builds ops dashboard data from live disputes and funding records", async () => {
    process.env.NEXT_PUBLIC_API_BASE_URL = "http://localhost:8080";
    process.env.NEXT_PUBLIC_SETTLEMENT_BASE_URL = "http://localhost:8083";
    globalThis.fetch = mock(async (input: RequestInfo | URL) => {
      const url = String(input);

      if (url.endsWith("/api/v1/providers")) {
        return new Response(JSON.stringify({ providers: [{ id: "provider_1", name: "Atlas Ops", capabilities: [], reputationTier: "gold" }] }), {
          headers: { "Content-Type": "application/json" },
          status: 200,
        });
      }

      if (url.endsWith("/api/v1/orders")) {
        return new Response(JSON.stringify({ orders: [{ id: "ord_live_1", buyerOrgId: "buyer_1", providerOrgId: "provider_1", fundingMode: "credit", platformWallet: "platform_main", status: "running", milestones: [] }] }), {
          headers: { "Content-Type": "application/json" },
          status: 200,
        });
      }

      if (url.endsWith("/api/v1/disputes")) {
        return new Response(JSON.stringify({ disputes: [{ id: "disp_1", orderId: "ord_live_1", milestoneId: "ms_1", reason: "Output incomplete", refundCents: 900, status: "open", createdAt: "2026-03-12T00:00:00Z" }] }), {
          headers: { "Content-Type": "application/json" },
          status: 200,
        });
      }

      if (url.endsWith("/v1/funding-records")) {
        return new Response(JSON.stringify({ records: [{ id: "fund_1", kind: "invoice", providerOrgId: "provider_1", amount: "12.5", state: "SETTLED" }] }), {
          headers: { "Content-Type": "application/json" },
          status: 200,
        });
      }

      if (url.endsWith("/api/v1/ops/demo/status")) {
        return new Response(JSON.stringify({
          status: {
            checkedAt: "2026-03-23T00:00:00Z",
            verdict: "ready",
            blockerReasons: [],
            services: [{ id: "settlement", label: "Settlement", healthy: true }],
            actors: [{ role: "buyer", ready: true }],
            buyerBalance: { settledTopUpCents: 6000, settledTopUpCount: 1, pendingTopUpCount: 0, minimumRequiredCents: 5000, meetsMinimumThreshold: true },
            providerSettlement: { readyChannelCount: 1, availableToAllocateCents: 8000, reservedOutstandingCents: 0, minimumRequiredCents: 5500, meetsMinimumThreshold: true },
          },
        }), {
          headers: { "Content-Type": "application/json" },
          status: 200,
        });
      }

      throw new Error(`unexpected url ${url}`);
    }) as unknown as typeof fetch;

    const data = await getOpsDashboardData({
      authToken: "tok_123",
      requireLive: true,
    });

    expect(data.summary.openDisputes).toBe(1);
    expect(data.pendingReviews[0]?.title).toContain("Open disputes");
    expect(data.riskFeed[0]?.detail).toContain("1 disputes");
    expect(data.demoStatus?.verdict).toBe("ready");
  });

  it("reads the live demo status from the ops endpoint", async () => {
    process.env.NEXT_PUBLIC_API_BASE_URL = "http://localhost:8080";
    globalThis.fetch = mock(async (input: RequestInfo | URL, init?: RequestInit) => {
      expect(String(input)).toBe("http://localhost:8080/api/v1/ops/demo/status");
      expect((init?.headers as Record<string, string>)?.Authorization).toBe("Bearer ops-token");
      return new Response(JSON.stringify({
        status: {
          checkedAt: "2026-03-23T00:00:00Z",
          verdict: "blocked",
          blockerReasons: ["provider liquidity pool is below the demo threshold"],
          services: [{ id: "carrier", label: "Carrier", healthy: false }],
          actors: [{ role: "ops", ready: true }],
          buyerBalance: { settledTopUpCents: 6000, settledTopUpCount: 1, pendingTopUpCount: 0, minimumRequiredCents: 5000, meetsMinimumThreshold: true },
          providerSettlement: { readyChannelCount: 0, availableToAllocateCents: 0, reservedOutstandingCents: 0, minimumRequiredCents: 5500, meetsMinimumThreshold: false },
        },
      }), {
        headers: { "Content-Type": "application/json" },
        status: 200,
      });
    }) as unknown as typeof fetch;

    const status = await getDemoStatus({ authToken: "ops-token", requireLive: true });

    expect(status?.verdict).toBe("blocked");
    expect(status?.blockerReasons[0]).toContain("provider liquidity pool");
  });
});
