import { afterEach, describe, expect, it, mock } from "bun:test";

import { POST } from "./route";

const originalFetch = globalThis.fetch;

afterEach(() => {
  globalThis.fetch = originalFetch;
  delete process.env.IAM_BASE_URL;
});

describe("auth logout route", () => {
  it("revokes the upstream session and clears the local cookie", async () => {
    process.env.IAM_BASE_URL = "http://iam.internal";
    globalThis.fetch = mock(async (input: RequestInfo | URL, init?: RequestInit) => {
      expect(String(input)).toBe("http://iam.internal/v1/logout");
      expect(init?.method).toBe("POST");
      expect((init?.headers as HeadersInit | undefined) as Record<string, string>).toMatchObject({
        Authorization: "Bearer tok_123",
      });

      return new Response(JSON.stringify({ revoked: true }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      });
    }) as unknown as typeof fetch;

    const response = await POST(
      new Request("http://localhost/auth/logout", {
        method: "POST",
        headers: {
          cookie: "one_tok_session=tok_123",
        },
      }),
    );

    expect(response.status).toBe(303);
    expect(new URL(response.headers.get("location") ?? "").pathname).toBe("/login");

    const cookie = response.headers.get("set-cookie");
    expect(cookie).toContain("one_tok_session=");
    expect(cookie).toContain("Max-Age=0");
  });
});
