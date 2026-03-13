import { afterEach, describe, expect, it, mock } from "bun:test";

import { POST } from "./route";

const originalFetch = globalThis.fetch;

afterEach(() => {
  globalThis.fetch = originalFetch;
  delete process.env.IAM_BASE_URL;
  delete process.env.NEXT_PUBLIC_API_BASE_URL;
});

describe("provider bid route", () => {
  it("submits a bid using the authenticated provider membership", async () => {
    process.env.IAM_BASE_URL = "http://iam.internal";
    process.env.NEXT_PUBLIC_API_BASE_URL = "http://api.internal";

    globalThis.fetch = mock(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url === "http://iam.internal/v1/me") {
        return new Response(
          JSON.stringify({
            user: { id: "usr_1", email: "provider@example.com", name: "Provider User" },
            memberships: [{ role: "sales", organization: { id: "provider_auth_1", name: "Provider Org", kind: "provider" } }],
          }),
          { status: 200, headers: { "Content-Type": "application/json" } },
        );
      }

      expect(url).toBe("http://api.internal/api/v1/rfqs/rfq_1/bids");
      expect(init?.method).toBe("POST");
      expect((init?.headers as Record<string, string>)?.Authorization).toBe("Bearer tok_123");
      expect(JSON.parse(String(init?.body))).toMatchObject({
        providerOrgId: "provider_auth_1",
        message: "We can take this today",
        quoteCents: 3900,
        milestones: [{ id: "ms_1", title: "Triage", basePriceCents: 3900, budgetCents: 4200 }],
      });

      return new Response(JSON.stringify({ bid: { id: "bid_1" } }), {
        status: 201,
        headers: { "Content-Type": "application/json" },
      });
    }) as unknown as typeof fetch;

    const form = new FormData();
    form.set("message", "We can take this today");
    form.set("quoteCents", "3900");
    form.set("milestoneTitle", "Triage");
    form.set("milestoneBasePriceCents", "3900");
    form.set("milestoneBudgetCents", "4200");

    const response = await POST(
      new Request("http://web-7f9c6d4f8c-abcde:3000/provider/rfqs/rfq_1/bids", {
        method: "POST",
        headers: {
          cookie: "one_tok_session=tok_123",
        },
        body: form,
      }),
      { params: Promise.resolve({ rfqId: "rfq_1" }) },
    );

    expect(response.status).toBe(303);
    expect(response.headers.get("location")).toBe("/provider");
  });
});
