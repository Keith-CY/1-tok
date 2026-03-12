import Link from "next/link";

import { SiteHeader } from "../../components/site-header";

export const dynamic = "force-dynamic";

const errorMessages: Record<string, string> = {
  "invalid-credentials": "Credentials were rejected by IAM. Check the email and password, then try again.",
  "missing-fields": "Email and password are both required.",
};

export default function LoginPage({
  searchParams,
}: {
  searchParams?: { error?: string; next?: string };
}) {
  const error = searchParams?.error ? errorMessages[searchParams.error] ?? "Authentication failed." : null;
  const next = searchParams?.next?.startsWith("/") ? searchParams.next : "/provider";

  return (
    <main className="page-frame">
      <SiteHeader />

      <section className="auth-shell">
        <div className="auth-panel">
          <span className="eyebrow">Access / session bootstrap</span>
          <h1 className="portal-title">Enter the market with a server-held session.</h1>
          <p className="section-copy">
            The web shell does not keep the bearer token in client-side state. Login submits to a Next route,
            the route talks to IAM, and the resulting session cookie stays httpOnly.
          </p>

          {error ? <p className="auth-error">{error}</p> : null}

          <form className="auth-form" action="/auth/login" method="post">
            <input type="hidden" name="next" value={next} />

            <label className="auth-field">
              <span>Email</span>
              <input name="email" type="email" autoComplete="email" placeholder="owner@example.com" required />
            </label>

            <label className="auth-field">
              <span>Password</span>
              <input
                name="password"
                type="password"
                autoComplete="current-password"
                placeholder="correct horse battery staple"
                required
              />
            </label>

            <button type="submit" className="auth-submit">
              Create session
            </button>
          </form>

          <p className="footer-note">
            Default destination is the provider portal. Use direct links like{" "}
            <Link href="/login?next=/ops">`/login?next=/ops`</Link> when you need a different landing page.
          </p>
        </div>
      </section>
    </main>
  );
}

