import Link from "next/link";
import { cookies } from "next/headers";

import { SESSION_COOKIE_NAME } from "../lib/session";

export async function SiteHeader() {
  const hasSession = cookies().has(SESSION_COOKIE_NAME);

  return (
    <header className="masthead">
      <div className="brand">
        <span className="brand__eyebrow">1-tok / market command</span>
        <span className="brand__name">Settlement as stagecraft.</span>
        <span className="brand__meta">
          Hybrid listings, RFQ motion, immediate milestone payouts, and controlled Fiber exposure.
        </span>
      </div>

      <nav className="nav-links" aria-label="Primary">
        <Link href="/">Overview</Link>
        <Link href="/buyer">Buyer</Link>
        <Link href="/provider">Provider</Link>
        <Link href="/ops">Ops</Link>
        <Link href="/login">{hasSession ? "Switch session" : "Login"}</Link>
        {hasSession ? (
          <form action="/auth/logout" method="post">
            <button type="submit" className="nav-button">
              Logout
            </button>
          </form>
        ) : null}
      </nav>
    </header>
  );
}
