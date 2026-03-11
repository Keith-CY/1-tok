import { afterEach, describe, expect, it, mock } from "bun:test";

import { getListings, getOrders } from "./api";

const originalFetch = globalThis.fetch;

afterEach(() => {
  globalThis.fetch = originalFetch;
  delete process.env.NEXT_PUBLIC_API_BASE_URL;
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
});
