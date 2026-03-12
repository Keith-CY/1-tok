import { afterEach, describe, expect, it, mock } from "bun:test";

import { fetchIAMActor, findPortalMembership } from "./viewer";

const originalFetch = globalThis.fetch;

afterEach(() => {
  globalThis.fetch = originalFetch;
  delete process.env.IAM_BASE_URL;
});

describe("viewer helpers", () => {
  it("fetches the actor profile from IAM using the bearer token", async () => {
    process.env.IAM_BASE_URL = "http://iam.internal";
    globalThis.fetch = mock(async (input: RequestInfo | URL, init?: RequestInit) => {
      expect(String(input)).toBe("http://iam.internal/v1/me");
      expect((init?.headers as Record<string, string>)?.Authorization).toBe("Bearer tok_123");

      return new Response(
        JSON.stringify({
          user: {
            id: "usr_1",
            email: "ops@example.com",
            name: "Ops User",
          },
          memberships: [
            {
              role: "finance_admin",
              organization: {
                id: "ops_org_1",
                name: "1-tok Ops",
                kind: "ops",
              },
            },
          ],
        }),
        {
          status: 200,
          headers: { "Content-Type": "application/json" },
        },
      );
    }) as unknown as typeof fetch;

    const actor = await fetchIAMActor("tok_123");

    expect(actor?.user.id).toBe("usr_1");
    expect(actor?.memberships[0]?.organization.kind).toBe("ops");
  });

  it("selects the matching membership for a portal kind", () => {
    const membership = findPortalMembership(
      {
        user: { id: "usr_1", email: "provider@example.com", name: "Provider User" },
        memberships: [
          {
            role: "finance_viewer",
            organization: {
              id: "provider_1",
              name: "Atlas Ops",
              kind: "provider",
            },
          },
        ],
      },
      "provider",
    );

    expect(membership?.organization.id).toBe("provider_1");
  });
});
