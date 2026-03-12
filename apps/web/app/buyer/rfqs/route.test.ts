import { afterEach, describe, expect, it, mock } from "bun:test";

import { POST } from "./route";

const originalFetch = globalThis.fetch;

afterEach(() => {
  globalThis.fetch = originalFetch;
  delete process.env.IAM_BASE_URL;
  delete process.env.NEXT_PUBLIC_API_BASE_URL;
});

describe("buyer rfq route", () => {
  it("creates an rfq using the authenticated buyer membership", async () => {
    process.env.IAM_BASE_URL = "http://iam.internal";
    process.env.NEXT_PUBLIC_API_BASE_URL = "http://api.internal";

    globalThis.fetch = mock(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url === "http://iam.internal/v1/me") {
        expect((init?.headers as Record<string, string>)?.Authorization).toBe("Bearer tok_123");

        return new Response(
          JSON.stringify({
            user: { id: "usr_1", email: "buyer@example.com", name: "Buyer User" },
            memberships: [{ role: "procurement", organization: { id: "buyer_auth_1", name: "Buyer Org", kind: "buyer" } }],
          }),
          { status: 200, headers: { "Content-Type": "application/json" } },
        );
      }

      expect(url).toBe("http://api.internal/api/v1/rfqs");
      expect(init?.method).toBe("POST");
      expect((init?.headers as Record<string, string>)?.Authorization).toBe("Bearer tok_123");
      expect(JSON.parse(String(init?.body))).toMatchObject({
        buyerOrgId: "buyer_auth_1",
        title: "Need live triage",
        category: "agent-ops",
        scope: "Investigate and stabilize",
        budgetCents: 4200,
      });

      return new Response(JSON.stringify({ rfq: { id: "rfq_1" } }), {
        status: 201,
        headers: { "Content-Type": "application/json" },
      });
    }) as unknown as typeof fetch;

    const form = new FormData();
    form.set("title", "Need live triage");
    form.set("category", "agent-ops");
    form.set("scope", "Investigate and stabilize");
    form.set("budgetCents", "4200");
    form.set("responseDeadlineAt", "2026-03-15T12:00");

    const response = await POST(
      new Request("http://web-7f9c6d4f8c-abcde:3000/buyer/rfqs", {
        method: "POST",
        headers: {
          cookie: "one_tok_session=tok_123",
        },
        body: form,
      }),
    );

    expect(response.status).toBe(303);
    expect(response.headers.get("location")).toBe("/buyer");
  });
});
