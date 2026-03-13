import { afterEach, describe, expect, it, mock } from "bun:test";

import { POST } from "./route";

const originalFetch = globalThis.fetch;

afterEach(() => {
  globalThis.fetch = originalFetch;
  delete process.env.IAM_BASE_URL;
  delete process.env.NEXT_PUBLIC_API_BASE_URL;
});

describe("buyer award route", () => {
  it("awards an rfq using the authenticated buyer membership", async () => {
    process.env.IAM_BASE_URL = "http://iam.internal";
    process.env.NEXT_PUBLIC_API_BASE_URL = "http://api.internal";

    globalThis.fetch = mock(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url === "http://iam.internal/v1/me") {
        return new Response(
          JSON.stringify({
            user: { id: "usr_1", email: "buyer@example.com", name: "Buyer User" },
            memberships: [{ role: "procurement", organization: { id: "buyer_auth_1", name: "Buyer Org", kind: "buyer" } }],
          }),
          { status: 200, headers: { "Content-Type": "application/json" } },
        );
      }

      expect(url).toBe("http://api.internal/api/v1/rfqs/rfq_1/award");
      expect(init?.method).toBe("POST");
      expect((init?.headers as Record<string, string>)?.Authorization).toBe("Bearer tok_123");
      expect(JSON.parse(String(init?.body))).toMatchObject({
        bidId: "bid_1",
        fundingMode: "credit",
        creditLineId: "credit_1",
      });

      return new Response(JSON.stringify({ rfq: { id: "rfq_1", status: "awarded" }, order: { id: "ord_1" } }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      });
    }) as unknown as typeof fetch;

    const form = new FormData();
    form.set("bidId", "bid_1");
    form.set("fundingMode", "credit");
    form.set("creditLineId", "credit_1");

    const response = await POST(
      new Request("http://web-7f9c6d4f8c-abcde:3000/buyer/rfqs/rfq_1/award", {
        method: "POST",
        headers: {
          cookie: "one_tok_session=tok_123",
        },
        body: form,
      }),
      { params: Promise.resolve({ rfqId: "rfq_1" }) },
    );

    expect(response.status).toBe(303);
    expect(response.headers.get("location")).toBe("/buyer");
  });
});
