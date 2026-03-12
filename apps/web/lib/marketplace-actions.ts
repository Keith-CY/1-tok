import { NextResponse } from "next/server";

import { fetchIAMActor, findPortalMembership, type PortalKind } from "./viewer";
import { parseCookieValue, SESSION_COOKIE_NAME } from "./session";

export async function readRequestPortalViewer(request: Request, kind: PortalKind) {
  const token = parseCookieValue(request.headers.get("cookie"), SESSION_COOKIE_NAME);
  if (!token) {
    return null;
  }

  const actor = await fetchIAMActor(token);
  if (!actor) {
    return null;
  }

  const membership = findPortalMembership(actor, kind);
  if (!membership) {
    return null;
  }

  return {
    token,
    actor,
    membership,
  };
}

export function redirectToPortal(request: Request, path: string, error?: string) {
  const nextURL = new URL(path, request.url);
  if (error) {
    nextURL.searchParams.set("error", error);
  }
  return NextResponse.redirect(nextURL, 303);
}

export async function postGatewayJSON(path: string, token: string, payload: unknown) {
  const baseURL = resolveAPIBaseURL();
  if (!baseURL) {
    throw new Error("api base url is not configured");
  }

  const response = await fetch(`${baseURL}${path}`, {
    method: "POST",
    headers: {
      Accept: "application/json",
      Authorization: `Bearer ${token}`,
      "Content-Type": "application/json",
    },
    body: JSON.stringify(payload),
    cache: "no-store",
  });

  if (!response.ok) {
    throw new Error(`gateway status ${response.status}`);
  }

  return response;
}

export function resolveAPIBaseURL(): string | null {
  return process.env.NEXT_PUBLIC_API_BASE_URL?.replace(/\/$/, "") ?? null;
}

export function normalizeDateTimeInput(value: string): string {
  const trimmed = value.trim();
  if (!trimmed) {
    return "";
  }
  if (/[zZ]$|[+-]\d{2}:\d{2}$/.test(trimmed)) {
    return trimmed;
  }
  if (/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}$/.test(trimmed)) {
    return `${trimmed}:00Z`;
  }
  return trimmed;
}
