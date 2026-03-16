import { formatMoney } from "@1tok/contracts";

import { PortalShell } from "../../components/portal-shell";
import { SummaryCard } from "../../components/summary-card";
import { EmptyState } from "../../components/ui";
import { getProviderDashboardData } from "../../lib/api";
import { requirePortalViewer } from "../../lib/viewer";

export const dynamic = "force-dynamic";

export default async function ProviderPage() {
  const viewer = await requirePortalViewer("provider", "/provider");
  const data = await getProviderDashboardData({
    authToken: viewer.token,
    providerOrgId: viewer.membership.organization.id,
    requireLive: true,
  });

  return (
    <PortalShell
      eyebrow="Provider portal / delivery + payouts"
      title="Run Carrier like a performance engine with cash already watching."
      copy="Providers need a cockpit that treats milestones, usage proofs, payout hooks, and reputation as part of the same operational loop. This page stays close to that loop."
      signal="Immediate payout only when proof and policy agree"
      asideTitle="Provider signal deck"
      quickActions={[
        { label: "Review pipeline", href: "#pipeline", tone: "primary" },
        { label: "Open opportunities", href: "#opportunities", tone: "secondary" },
        { label: "Track payouts", href: "#payouts", tone: "secondary" },
      ]}
      asideItems={[
        { label: "Provider", value: data.summary.providerName, tone: "mint" },
        { label: "Submitted bids", value: `${data.summary.submittedBids}`, tone: "warning" },
        { label: "Tier", value: data.summary.reputationTier.toUpperCase() },
      ]}
    >
      <div className="stat-grid">
        <SummaryCard
          kicker="Active orders"
          value={`${data.summary.activeOrders}`}
          hint="Orders where Carrier callbacks are still responsible for keeping budget and settlement in sync."
        />
        <SummaryCard
          kicker="Open RFQs"
          value={`${data.summary.openRFQs}`}
          hint="Visible RFQs still open for provider responses across the marketplace."
        />
        <SummaryCard
          kicker="Submitted bids"
          value={`${data.summary.submittedBids}`}
          hint="Bids this provider currently has in play or already won."
        />
        <SummaryCard
          kicker="Withdrawals in flight"
          value={`${data.summary.inFlightWithdrawals}`}
          hint="Withdrawal records that still need dashboard sync or final settlement."
        />
        <SummaryCard
          kicker="Reputation"
          value={data.summary.reputationTier.toUpperCase()}
          hint="Tier influences ranking, RFQ visibility, and ops posture."
        />
      </div>

      <div className="feed-grid">
        <article className="feed-card" id="pipeline">
          <span className="tag">Pipeline</span>
          <h3>What will move revenue in the next hour.</h3>
          <div className="feed-list">
            {data.pipeline.length === 0 ? <EmptyState icon="⏱️" message="No active pipeline items at the moment." /> : null}
            {data.pipeline.map((item) => (
              <div key={item.id} className="feed-item">
                <strong>{item.label}</strong>
                <p>{item.detail}</p>
              </div>
            ))}
          </div>
        </article>

        <aside className="message-card" id="opportunities">
          <span className="tag">Open RFQs</span>
          <h3>Providers need a direct lane from opportunity to submitted bid.</h3>
          <div className="message-list">
            {data.marketOpportunities.length === 0 ? (
                  <EmptyState icon="📡" message="No marketplace opportunities right now. Refresh soon for fresh RFQs." />
                ) : null}
                  {data.marketOpportunities.map((item) => (
              <div key={item.id} className="message-item">
                <strong>{item.title}</strong>
                <p>
                  budget {formatMoney(item.budgetCents)} · buyer {item.buyerOrgId} · deadline {item.responseDeadlineAt.slice(0, 10)}
                </p>
                {item.hasProviderBid ? (
                  <p>Bid already submitted from this provider session.</p>
                ) : (
                  <form className="auth-form market-form" action={`/provider/rfqs/${item.id}/bids`} method="post">
                    <label className="auth-field">
                      <span>Message</span>
                      <textarea name="message" rows={2} placeholder="Carrier-ready response, availability, and outcome." required />
                    </label>
                    <div className="market-form__grid">
                      <label className="auth-field">
                        <span>Quote cents</span>
                        <input name="quoteCents" type="number" min="1" step="1" placeholder="3900" required />
                      </label>
                      <label className="auth-field">
                        <span>Milestone title</span>
                        <input name="milestoneTitle" type="text" defaultValue="Execution" />
                      </label>
                    </div>
                    <input type="hidden" name="milestoneBudgetCents" value={item.budgetCents} />
                    <button type="submit" className="action-button">
                      Submit bid
                    </button>
                  </form>
                )}
              </div>
            ))}
          </div>
        </aside>
      </div>

      <article className="feed-card" id="payouts">
        <span className="tag">Submitted bids</span>
        <h3>Bid posture should sit next to payout posture.</h3>
        <div className="feed-list">
          {data.marketQueue.length === 0 ? <EmptyState icon="🧾" message="No submitted bids to track yet; use the pipeline to submit." /> : null}
          {data.marketQueue.map((item) => (
            <div key={item.id} className="feed-item">
              <strong>{item.title}</strong>
              <p>
                {item.providerBidStatus} · quote {formatMoney(item.quoteCents)} · budget {formatMoney(item.budgetCents)}
              </p>
              <p>
                buyer {item.buyerOrgId} · deadline {item.responseDeadlineAt.slice(0, 10)}
              </p>
            </div>
          ))}
        </div>
      </article>

      <article className="feed-card">
        <span className="tag">Capabilities</span>
        <h3>What this provider can credibly sell.</h3>
        <div className="chip-list">
          {data.capabilities.map((capability) => (
            <div key={capability} className="chip">
              {capability}
            </div>
          ))}
        </div>
      </article>

      <article className="timeline-card">
        <span className="tag">Live order trace</span>
        <h3>Operators should never wonder why money did or did not move.</h3>
        <div className="timeline">
          {data.activeOrders.flatMap((order) =>
            order.milestones.map((milestone) => (
              <div key={`${order.id}-${milestone.id}`} className="timeline-item">
                <strong>
                  {order.id} · {milestone.title}
                </strong>
                <p>
                  {milestone.state} · {formatMoney(milestone.basePriceCents)} base ·{" "}
                  {milestone.usageCharges.length} usage proof events
                </p>
              </div>
            )),
          )}
        </div>
      </article>
    </PortalShell>
  );
}
