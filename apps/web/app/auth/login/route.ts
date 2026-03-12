import { NextResponse } from "next/server";

import { createIAMSession, SESSION_COOKIE_NAME, shouldUseSecureSessionCookie } from "../../../lib/session";

export async function POST(request: Request) {
  const form = await request.formData();
  const email = String(form.get("email") ?? "").trim();
  const password = String(form.get("password") ?? "");
  const next = sanitizeNextPath(String(form.get("next") ?? "/"));

  if (!email || !password) {
    return NextResponse.redirect(new URL(`/login?error=missing-fields&next=${encodeURIComponent(next)}`, request.url), 303);
  }

  try {
    const result = await createIAMSession(email, password);
    const response = NextResponse.redirect(new URL(next, request.url), 303);

    response.cookies.set({
      name: SESSION_COOKIE_NAME,
      value: result.session.token,
      httpOnly: true,
      sameSite: "lax",
      secure: shouldUseSecureSessionCookie(),
      path: "/",
    });

    return response;
  } catch {
    return NextResponse.redirect(new URL(`/login?error=invalid-credentials&next=${encodeURIComponent(next)}`, request.url), 303);
  }
}

function sanitizeNextPath(value: string): string {
  if (!value.startsWith("/") || value.startsWith("//")) {
    return "/";
  }
  return value;
}
