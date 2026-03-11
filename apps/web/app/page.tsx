import Link from "next/link";

import { formatMoney, sampleBuyerSummary, sampleOpsSummary, sampleProviderSummary } from "@1tok/contracts";

import { SiteHeader } from "../components/site-header";

const portalCards: Array<{ href: string; title: string; copy: string }> = [
  {
    href: "/buyer",
    title: "Buyer control room",
    copy: "Shape RFQs, accept provider templates, spend prepaid or credit capital, and intervene only when the budget wall is hit.",
  },
  {
    href: "/provider",
    title: "Provider command deck",
    copy: "Track Carrier callbacks, active milestones, usage proofs, payout unlocks, and the operational drag from dispute recovery.",
  },
  {
    href: "/ops",
    title: "Ops + risk floor",
    copy: "Review new providers, calibrate credit lines, watch Fiber exposure, and resolve post-payout disputes without losing traceability.",
  },
];

export const dynamic = "force-dynamic";

export default function HomePage() {
  return (
    <main className="page-frame">
      <SiteHeader />

      <section className="hero">
        <div className="hero__intro">
          <span className="eyebrow">Portal shell / real-time market</span>
          <h1 className="hero__title">The market is not a storefront. It is a control room.</h1>
          <p className="hero__copy">
            1-tok turns agent work into staged commitments: buyers publish intent, providers price milestones,
            Carrier proves execution, and the platform decides whether capital keeps moving.
          </p>

          <div className="hero__metrics">
            <div className="metric">
              <div className="metric__label">Buyer credit</div>
              <div className="metric__value">{formatMoney(sampleBuyerSummary.remainingCreditCents)}</div>
            </div>
            <div className="metric">
              <div className="metric__label">Provider payouts</div>
              <div className="metric__value">{formatMoney(sampleProviderSummary.availablePayoutCents)}</div>
            </div>
            <div className="metric">
              <div className="metric__label">Ops exposure</div>
              <div className="metric__value">{formatMoney(sampleOpsSummary.outstandingExposureCents)}</div>
            </div>
          </div>
        </div>

        <aside className="hero__panel">
          <div>
            <div className="label">What the shell covers</div>
            <div className="signal-stack">
              <div className="signal-row">
                <span>Hybrid market</span>
                <span className="signal-row__value">Listings + RFQ award</span>
              </div>
              <div className="signal-row">
                <span>Funding logic</span>
                <span className="signal-row__value signal-row__value--warning">Prepaid or platform credit</span>
              </div>
              <div className="signal-row">
                <span>Settlement</span>
                <span className="signal-row__value signal-row__value--accent">Immediate milestone release</span>
              </div>
            </div>
          </div>

          <div className="signal-pill signal-pill--mint">Graceful fallback demo data when API is offline</div>
        </aside>
      </section>

      <section className="section-block">
        <div>
          <span className="eyebrow">Three portals, one posture</span>
          <h2 className="section-title">Every role sees the same market, but through a different risk lens.</h2>
          <p className="section-copy">
            Buyers optimize outcome and budget. Providers optimize fulfillment and payout velocity. Operators
            optimize trust, credit discipline, and channel health.
          </p>
        </div>

        <div className="portal-grid">
          {portalCards.map((portal) => (
            <Link href={portal.href} key={portal.href} className="glass-card">
              <span className="tag">Portal</span>
              <h3>{portal.title}</h3>
              <p>{portal.copy}</p>
            </Link>
          ))}
        </div>
      </section>

      <section className="section-block">
        <div>
          <span className="eyebrow">Design stance</span>
          <h2 className="section-title">Editorial typography in front, telemetry discipline underneath.</h2>
          <p className="section-copy">
            The shell uses serif headlines to make each role feel deliberate, then switches to a mono body to keep
            operational data crisp and procedural.
          </p>
        </div>

        <div className="card-grid">
          <article className="glass-card">
            <span className="tag">Buyer</span>
            <h3>Budget walls stay visible.</h3>
            <p>When usage creeps, the portal should feel like an instrument panel, not a generic SaaS list.</p>
          </article>
          <article className="glass-card">
            <span className="tag">Provider</span>
            <h3>Payout readiness is a first-class signal.</h3>
            <p>Carrier hook health, capability coverage, and dispute drag sit beside pipeline metrics.</p>
          </article>
        </div>
      </section>

      <p className="footer-note">
        Set <code>NEXT_PUBLIC_API_BASE_URL</code> to point at the gateway. Without it, the shell uses embedded demo
        data so layout work stays reviewable before services are live.
      </p>
    </main>
  );
}
