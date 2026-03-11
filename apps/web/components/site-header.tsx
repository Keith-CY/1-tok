import Link from "next/link";

export function SiteHeader() {
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
      </nav>
    </header>
  );
}

