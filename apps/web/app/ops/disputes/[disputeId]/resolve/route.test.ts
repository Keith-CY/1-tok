import { afterEach, describe, expect, it, mock } from "bun:test";

import { POST } from "./route";

const originalFetch = globalThis.fetch;

afterEach(() => {
  globalThis.fetch = originalFetch;
  delete process.env.IAM_BASE_URL;
  delete process.env.NEXT_PUBLIC_API_BASE_URL;
});

describe("ops dispute resolution route", () => {
  it("resolves a dispute using the authenticated ops membership", async () => {
    process.env.IAM_BASE_URL = "http://iam.internal";
    process.env.NEXT_PUBLIC_API_BASE_URL = "http://api.internal";

    globalThis.fetch = mock(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url === "http://iam.internal/v1/me") {
        return new Response(
          JSON.stringify({
            user: { id: "usr_1", email: "ops@example.com", name: "Ops User" },
            memberships: [{ role: "ops_reviewer", organization: { id: "ops_1", name: "Ops Org", kind: "ops" } }],
          }),
          { status: 200, headers: { "Content-Type": "application/json" } },
        );
      }

      expect(url).toBe("http://api.internal/api/v1/disputes/disp_1/resolve");
      expect(init?.method).toBe("POST");
      expect((init?.headers as Record<string, string>)?.Authorization).toBe("Bearer tok_123");
      expect(JSON.parse(String(init?.body))).toMatchObject({
        resolution: "Approved reimbursement after evidence review.",
        resolvedBy: "usr_1",
      });

      return new Response(
        JSON.stringify({
          dispute: {
            id: "disp_1",
            orderId: "ord_1",
            milestoneId: "ms_1",
            reason: "Output incomplete",
            refundCents: 900,
            status: "resolved",
            resolution: "Approved reimbursement after evidence review.",
            resolvedBy: "usr_1",
            resolvedAt: "2026-03-12T00:00:00Z",
            createdAt: "2026-03-12T00:00:00Z",
          },
        }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      );
    }) as unknown as typeof fetch;

    const form = new FormData();
    form.set("resolution", "Approved reimbursement after evidence review.");

    const response = await POST(
      new Request("http://web-7f9c6d4f8c-abcde:3000/ops/disputes/disp_1/resolve", {
        method: "POST",
        headers: {
          cookie: "one_tok_session=tok_123",
        },
        body: form,
      }),
      { params: Promise.resolve({ disputeId: "disp_1" }) },
    );

    expect(response.status).toBe(303);
    const locationHeader = response.headers.get("location") ?? "";
    expect(locationHeader.startsWith("/ops?")).toBe(true);
    const location = new URL(locationHeader, "http://localhost");
    expect(location.pathname).toBe("/ops");
    expect(location.searchParams.get("resolvedDisputeId")).toBe("disp_1");
    expect(location.searchParams.get("disputeStatus")).toBe("resolved");
  });
});
