export const SESSION_COOKIE_NAME = "one_tok_session";

export interface IAMSessionResponse {
  session: {
    token: string;
    expiresAt?: string;
  };
  user?: {
    id: string;
    email: string;
    name: string;
  };
}

export function resolveIAMBaseURL(): string | null {
  return process.env.IAM_BASE_URL?.replace(/\/$/, "") ?? null;
}

export function shouldUseSecureSessionCookie(): boolean {
  if (process.env.NODE_ENV !== "production") {
    return false;
  }
  return process.env.ONE_TOK_ALLOW_INSECURE_SESSION_COOKIE !== "true";
}

export async function createIAMSession(email: string, password: string): Promise<IAMSessionResponse> {
  const baseUrl = resolveIAMBaseURL();
  if (!baseUrl) {
    throw new Error("iam base url is not configured");
  }

  const response = await fetch(`${baseUrl}/v1/sessions`, {
    method: "POST",
    headers: {
      Accept: "application/json",
      "Content-Type": "application/json",
    },
    body: JSON.stringify({
      email,
      password,
    }),
    cache: "no-store",
  });

  if (!response.ok) {
    throw new Error(`iam session status ${response.status}`);
  }

  return (await response.json()) as IAMSessionResponse;
}

export async function revokeIAMSession(token: string): Promise<void> {
  const baseUrl = resolveIAMBaseURL();
  if (!baseUrl || !token) {
    return;
  }

  await fetch(`${baseUrl}/v1/logout`, {
    method: "POST",
    headers: {
      Accept: "application/json",
      Authorization: `Bearer ${token}`,
    },
    cache: "no-store",
  });
}

export function parseCookieValue(cookieHeader: string | null, name: string): string | null {
  if (!cookieHeader) {
    return null;
  }

  const prefix = `${name}=`;
  for (const part of cookieHeader.split(";")) {
    const value = part.trim();
    if (value.startsWith(prefix)) {
      return value.slice(prefix.length);
    }
  }

  return null;
}
