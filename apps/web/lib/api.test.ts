import { afterEach, describe, expect, it, mock } from "bun:test";

import { getBuyerDashboardData, getFundingRecords, getListings, getOrders } from "./api";

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
    expect(records[0]?.id).toBe("fund_1");
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

      throw new Error(`unexpected url ${url}`);
    }) as unknown as typeof fetch;

    const data = await getBuyerDashboardData({
      authToken: "tok_123",
      buyerOrgId: "buyer_1",
      requireLive: true,
    });

    expect(data.summary.activeOrders).toBe(1);
    expect(data.summary.availableListings).toBe(1);
    expect(data.activeOrders).toHaveLength(1);
    expect(data.activeOrders[0]?.id).toBe("ord_live_1");
    expect(data.recommendedListings[0]?.id).toBe("listing_live");
  });
});
