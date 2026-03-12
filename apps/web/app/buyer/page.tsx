import { formatMoney } from "@1tok/contracts";

import { PortalShell } from "../../components/portal-shell";
import { SummaryCard } from "../../components/summary-card";
import { getBuyerDashboardData } from "../../lib/api";
import { requirePortalViewer } from "../../lib/viewer";

export const dynamic = "force-dynamic";

export default async function BuyerPage() {
  const viewer = await requirePortalViewer("buyer", "/buyer");
  const data = await getBuyerDashboardData({
    authToken: viewer.token,
    buyerOrgId: viewer.membership.organization.id,
    requireLive: true,
  });

  return (
    <PortalShell
      eyebrow="Buyer portal / orchestration budget"
      title="Buy agent work like a floor trader, not a ticket submitter."
      copy="This view keeps discovery, funding mode, milestone exposure, and pause recovery in one frame. Buyers should see exactly when Carrier requests more budget and what will happen if they ignore it."
      signal="Credit and prepaid capital share the same order frame"
      asideTitle="Buyer signal deck"
      asideItems={[
        { label: "Buyer org", value: data.summary.buyerOrgId, tone: "mint" },
        { label: "Open RFQs", value: `${data.summary.openRFQs}` },
        { label: "Paused orders", value: `${data.summary.pausedOrders}`, tone: "warning" },
      ]}
    >
      <div className="stat-grid">
        <SummaryCard
          kicker="Active orders"
          value={`${data.summary.activeOrders}`}
          hint="Orders currently executing under platform-controlled channels."
        />
        <SummaryCard
          kicker="Open RFQs"
          value={`${data.summary.openRFQs}`}
          hint="Buyer-authored requests still collecting bids or awaiting award."
        />
        <SummaryCard
          kicker="Paused orders"
          value={`${data.summary.pausedOrders}`}
          hint="Orders currently waiting on more budget before Carrier can continue."
        />
        <SummaryCard
          kicker="Available listings"
          value={`${data.summary.availableListings}`}
          hint="Listings currently visible in the marketplace catalog for this buyer session."
        />
      </div>

      <div className="feed-grid">
        <article className="feed-card">
          <span className="tag">Recommended listings</span>
          <h3>Providers ranked for the current market temperature.</h3>
          <div className="feed-list">
            {data.recommendedListings.map((listing) => (
              <div key={listing.id} className="feed-item">
                <strong>{listing.title}</strong>
                <p>
                  {listing.category} · {formatMoney(listing.basePriceCents)} base price
                </p>
                <div className="chip-list">
                  {listing.tags.map((tag) => (
                    <div className="chip" key={tag}>
                      {tag}
                    </div>
                  ))}
                </div>
              </div>
            ))}
          </div>
        </article>

        <aside className="message-card">
          <span className="tag">RFQ book</span>
          <h3>Every open request should show bid pressure, not just status.</h3>
          <div className="message-list">
            {data.rfqBook.map((rfq) => (
              <div key={rfq.id} className="message-item">
                <strong>{rfq.title}</strong>
                <p>
                  {rfq.status} · {rfq.bidCount} bids · budget {formatMoney(rfq.budgetCents)}
                </p>
                <p>Response deadline {rfq.responseDeadlineAt.slice(0, 10)}</p>
              </div>
            ))}
          </div>
        </aside>
      </div>

      <article className="feed-card">
        <span className="tag">Inbox</span>
        <h3>Messages that change buyer decisions.</h3>
        <div className="feed-list">
          {data.inbox.map((message) => (
            <div key={message.id} className="feed-item">
              <strong>{message.title}</strong>
              <p>{message.detail}</p>
            </div>
          ))}
        </div>
      </article>

      <article className="timeline-card">
        <span className="tag">Active order frame</span>
        <h3>Milestone state is the thing that determines cash movement.</h3>
        <div className="timeline">
          {data.activeOrders[0]?.milestones.map((milestone) => (
            <div key={milestone.id} className="timeline-item">
              <strong>
                {milestone.title} · {milestone.state}
              </strong>
              <p>
                Budget {formatMoney(milestone.budgetCents)} · Settled {formatMoney(milestone.settledCents)}
              </p>
            </div>
          ))}
        </div>
      </article>
    </PortalShell>
  );
}
