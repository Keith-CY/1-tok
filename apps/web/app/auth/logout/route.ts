import { NextResponse } from "next/server";

import { parseCookieValue, revokeIAMSession, SESSION_COOKIE_NAME, shouldUseSecureSessionCookie } from "../../../lib/session";

export async function POST(request: Request) {
  const token = parseCookieValue(request.headers.get("cookie"), SESSION_COOKIE_NAME);
  if (token) {
    await revokeIAMSession(token);
  }

  const response = NextResponse.redirect(new URL("/login", request.url), 303);
  response.cookies.set({
    name: SESSION_COOKIE_NAME,
    value: "",
    httpOnly: true,
    sameSite: "lax",
    secure: shouldUseSecureSessionCookie(),
    path: "/",
    maxAge: 0,
  });
  return response;
}
