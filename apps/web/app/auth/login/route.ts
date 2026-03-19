import { redirectToPath } from "../../../lib/redirect";
import { createIAMSession, SESSION_COOKIE_NAME, SESSION_MAX_AGE_SECONDS, shouldUseSecureSessionCookie } from "../../../lib/session";

export async function POST(request: Request) {
  const form = await request.formData();
  const email = String(form.get("email") ?? "").trim();
  const password = String(form.get("password") ?? "");
  const next = sanitizeNextPath(String(form.get("next") ?? "/"));
  const loginPath = getLoginPath(next);

  if (!email || !password) {
    return redirectToPath(`${loginPath}?error=missing-fields&next=${encodeURIComponent(next)}`);
  }

  try {
    const result = await createIAMSession(email, password);
    const response = redirectToPath(next);

    response.cookies.set({
      name: SESSION_COOKIE_NAME,
      value: result.session.token,
      httpOnly: true,
      sameSite: "lax",
      secure: shouldUseSecureSessionCookie(),
      path: "/",
      maxAge: SESSION_MAX_AGE_SECONDS,
    });

    return response;
  } catch {
    return redirectToPath(`${loginPath}?error=invalid-credentials&next=${encodeURIComponent(next)}`);
  }
}

function sanitizeNextPath(value: string): string {
  if (!value.startsWith("/") || value.startsWith("//")) {
    return "/";
  }
  return value;
}

function getLoginPath(next: string) {
  return next.startsWith("/ops") ? "/internal/login" : "/login";
}
