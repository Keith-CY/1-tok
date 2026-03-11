import { formatMoney } from "@1tok/contracts";

import { PortalShell } from "../../components/portal-shell";
import { SummaryCard } from "../../components/summary-card";
import { getBuyerDashboardData } from "../../lib/api";

export const dynamic = "force-dynamic";

export default async function BuyerPage() {
  const data = await getBuyerDashboardData();

  return (
    <PortalShell
      eyebrow="Buyer portal / orchestration budget"
      title="Buy agent work like a floor trader, not a ticket submitter."
      copy="This view keeps discovery, funding mode, milestone exposure, and pause recovery in one frame. Buyers should see exactly when Carrier requests more budget and what will happen if they ignore it."
      signal="Credit and prepaid capital share the same order frame"
      asideTitle="Buyer signal deck"
      asideItems={[
        { label: "Remaining credit", value: formatMoney(data.summary.remainingCreditCents), tone: "mint" },
        { label: "Prepaid pool", value: formatMoney(data.summary.prepaidBalanceCents) },
        { label: "Open disputes", value: `${data.summary.openDisputes}`, tone: "warning" },
      ]}
    >
      <div className="stat-grid">
        <SummaryCard
          kicker="Active orders"
          value={`${data.summary.activeOrders}`}
          hint="Orders currently executing under platform-controlled channels."
        />
        <SummaryCard
          kicker="Remaining credit"
          value={formatMoney(data.summary.remainingCreditCents)}
          hint="Instant buying power assigned by the rules engine and ops overrides."
        />
        <SummaryCard
          kicker="Prepaid balance"
          value={formatMoney(data.summary.prepaidBalanceCents)}
          hint="Funding already parked and ready for orders without platform credit."
        />
        <SummaryCard
          kicker="Disputes"
          value={`${data.summary.openDisputes}`}
          hint="Post-payout claims still within the operator review window."
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
          <span className="tag">Inbox</span>
          <h3>Messages that change buyer decisions.</h3>
          <div className="message-list">
            {data.inbox.map((message) => (
              <div key={message.id} className="message-item">
                <strong>{message.title}</strong>
                <p>{message.detail}</p>
              </div>
            ))}
          </div>
        </aside>
      </div>

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
