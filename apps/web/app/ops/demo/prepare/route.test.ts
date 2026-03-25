import { afterEach, describe, expect, it, mock } from "bun:test";

import { POST } from "./route";

const originalFetch = globalThis.fetch;

afterEach(() => {
  globalThis.fetch = originalFetch;
  delete process.env.IAM_BASE_URL;
  delete process.env.NEXT_PUBLIC_API_BASE_URL;
});

describe("ops demo prepare route", () => {
  it("triggers demo prepare using the authenticated ops membership", async () => {
    process.env.IAM_BASE_URL = "http://iam.internal";
    process.env.NEXT_PUBLIC_API_BASE_URL = "http://api.internal";

    globalThis.fetch = mock(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url === "http://iam.internal/v1/me") {
        return new Response(
          JSON.stringify({
            user: { id: "usr_ops", email: "ops@example.com", name: "Ops User" },
            memberships: [{ role: "ops_reviewer", organization: { id: "ops_auth_1", name: "Ops Org", kind: "ops" } }],
          }),
          { status: 200, headers: { "Content-Type": "application/json" } },
        );
      }

      expect(url).toBe("http://api.internal/api/v1/ops/demo/prepare");
      expect(init?.method).toBe("POST");
      expect((init?.headers as Record<string, string>)?.Authorization).toBe("Bearer tok_ops");
      expect(JSON.parse(String(init?.body))).toEqual({});

      return new Response(JSON.stringify({ summary: { status: { verdict: "ready" } } }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      });
    }) as unknown as typeof fetch;

    const response = await POST(
      new Request("http://web:3000/ops/demo/prepare", {
        method: "POST",
        headers: {
          cookie: "one_tok_session=tok_ops",
        },
      }),
    );

    expect(response.status).toBe(303);
    expect(response.headers.get("location")).toBe("/ops?demoPrepared=success");
  });

  it("forwards demo prepare blocker reason when gateway request fails", async () => {
    process.env.IAM_BASE_URL = "http://iam.internal";
    process.env.NEXT_PUBLIC_API_BASE_URL = "http://api.internal";

    globalThis.fetch = mock(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url === "http://iam.internal/v1/me") {
        return new Response(
          JSON.stringify({
            user: { id: "usr_ops", email: "ops@example.com", name: "Ops User" },
            memberships: [{ role: "ops_reviewer", organization: { id: "ops_auth_1", name: "Ops Org", kind: "ops" } }],
          }),
          { status: 200, headers: { "Content-Type": "application/json" } },
        );
      }

      expect(url).toBe("http://api.internal/api/v1/ops/demo/prepare");
      expect(init?.method).toBe("POST");

      return new Response(
        JSON.stringify({ error: "provider settlement pool unavailable", summary: { status: { verdict: "blocked" } } }),
        { status: 409, headers: { "Content-Type": "application/json" } },
      );
    }) as unknown as typeof fetch;

    const response = await POST(
      new Request("http://web:3000/ops/demo/prepare", {
        method: "POST",
        headers: {
          cookie: "one_tok_session=tok_ops",
        },
      }),
    );

    expect(response.status).toBe(303);
    const location = new URL(response.headers.get("location") ?? "", "http://example.com");
    expect(location.pathname).toBe("/ops");
    expect(location.searchParams.get("error")).toBe("demo-prepare-failed");
    expect(location.searchParams.get("demoPrepareError")).toBe("provider settlement pool unavailable");
  });
});
