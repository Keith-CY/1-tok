import { redirectToPath } from "../../../lib/redirect";
import { parseCookieValue, revokeIAMSession, SESSION_COOKIE_NAME, shouldUseSecureSessionCookie } from "../../../lib/session";

export async function POST(request: Request) {
  const token = parseCookieValue(request.headers.get("cookie"), SESSION_COOKIE_NAME);
  if (token) {
    await revokeIAMSession(token);
  }

  const next = await readNextPath(request);
  const response = redirectToPath(next);
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

function sanitizeNextPath(value: string): string {
  if (!value.startsWith("/") || value.startsWith("//")) {
    return "/login";
  }
  return value;
}

async function readNextPath(request: Request) {
  const contentType = request.headers.get("content-type") ?? "";
  if (!contentType.includes("application/x-www-form-urlencoded") && !contentType.includes("multipart/form-data")) {
    return "/login";
  }

  try {
    const form = await request.formData();
    return sanitizeNextPath(String(form.get("next") ?? "/login"));
  } catch {
    return "/login";
  }
}
