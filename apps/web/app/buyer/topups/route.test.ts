import { afterEach, describe, expect, it, mock } from "bun:test";

import { POST } from "./route";

const originalFetch = globalThis.fetch;

afterEach(() => {
  globalThis.fetch = originalFetch;
  delete process.env.IAM_BASE_URL;
  delete process.env.NEXT_PUBLIC_SETTLEMENT_BASE_URL;
});

describe("buyer topup route", () => {
  it("ensures a buyer deposit address using the authenticated buyer membership", async () => {
    process.env.IAM_BASE_URL = "http://iam.internal";
    process.env.NEXT_PUBLIC_SETTLEMENT_BASE_URL = "http://settlement.internal";

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

      expect(url).toBe("http://settlement.internal/v1/topups");
      expect(init?.method).toBe("POST");
      expect((init?.headers as Record<string, string>)?.Authorization).toBe("Bearer tok_123");
      expect(JSON.parse(String(init?.body))).toMatchObject({
        asset: "USDI",
      });

      return new Response(JSON.stringify({ address: "ckt1qyqbuyer0address", asset: "USDI", confirmationBlocks: 24 }), {
        status: 201,
        headers: { "Content-Type": "application/json" },
      });
    }) as unknown as typeof fetch;

    const form = new FormData();
    form.set("amount", "25.00");
    form.set("asset", "USDI");

    const response = await POST(
      new Request("http://web:3000/buyer/topups", {
        method: "POST",
        headers: {
          cookie: "one_tok_session=tok_123",
        },
        body: form,
      }),
    );

    expect(response.status).toBe(303);
    expect(response.headers.get("location")).toBe("/buyer?topup=success");
  });
});
