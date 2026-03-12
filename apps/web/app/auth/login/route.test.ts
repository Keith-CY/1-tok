import { afterEach, describe, expect, it, mock } from "bun:test";

import { POST } from "./route";

const originalFetch = globalThis.fetch;

afterEach(() => {
  globalThis.fetch = originalFetch;
  delete process.env.IAM_BASE_URL;
});

describe("auth login route", () => {
  it("creates a session cookie and redirects to the requested path", async () => {
    process.env.IAM_BASE_URL = "http://iam.internal";
    globalThis.fetch = mock(async (input: RequestInfo | URL, init?: RequestInit) => {
      expect(String(input)).toBe("http://iam.internal/v1/sessions");
      expect(init?.method).toBe("POST");

      return new Response(
        JSON.stringify({
          session: {
            token: "tok_123",
            expiresAt: "2026-04-11T02:48:24Z",
          },
          user: {
            id: "usr_1",
            email: "provider@example.com",
            name: "Provider User",
          },
        }),
        {
          status: 201,
          headers: { "Content-Type": "application/json" },
        },
      );
    }) as unknown as typeof fetch;

    const form = new FormData();
    form.set("email", "provider@example.com");
    form.set("password", "correct horse battery staple");
    form.set("next", "/provider");

    const response = await POST(
      new Request("http://localhost/auth/login", {
        method: "POST",
        body: form,
      }),
    );

    expect(response.status).toBe(303);
    expect(new URL(response.headers.get("location") ?? "").pathname).toBe("/provider");

    const cookie = response.headers.get("set-cookie");
    expect(cookie).toContain("one_tok_session=tok_123");
    expect(cookie).toContain("HttpOnly");
    expect(cookie).toContain("Path=/");
  });
});
