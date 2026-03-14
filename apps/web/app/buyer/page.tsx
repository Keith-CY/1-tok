import { formatMoney } from "@1tok/contracts";

import { PortalShell } from "../../components/portal-shell";
import { SummaryCard } from "../../components/summary-card";
import { StatusBadge, ProgressBar, EmptyState } from "../../components/ui";
import { getBuyerDashboardData } from "../../lib/api";
import { requirePortalViewer } from "../../lib/viewer";

export const dynamic = "force-dynamic";

const PROGRESS_WARNING_THRESHOLD = 0.9;

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

      <article className="feed-card">
        <span className="tag">Open an RFQ</span>
        <h3>Buyers should be able to turn intent into a priced market request immediately.</h3>
        <form className="auth-form market-form" action="/buyer/rfqs" method="post">
          <div className="market-form__grid">
            <label className="auth-field">
              <span>Title</span>
              <input name="title" type="text" placeholder="Need live carrier triage" required />
            </label>
            <label className="auth-field">
              <span>Category</span>
              <input name="category" type="text" defaultValue="agent-ops" required />
            </label>
            <label className="auth-field">
              <span>Budget cents</span>
              <input name="budgetCents" type="number" min="1" step="1" placeholder="4200" required />
            </label>
            <label className="auth-field">
              <span>Response deadline</span>
              <input name="responseDeadlineAt" type="datetime-local" required />
            </label>
          </div>
          <label className="auth-field">
            <span>Scope</span>
            <textarea name="scope" rows={3} placeholder="Investigate the failure, stabilize the runtime, and summarize next steps." required />
          </label>
          <button type="submit" className="auth-submit">
            Publish RFQ
          </button>
        </form>
      </article>

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
                  <StatusBadge status={rfq.status} /> · {rfq.bidCount} bids · budget {formatMoney(rfq.budgetCents)}
                </p>
                <p>Response deadline {rfq.responseDeadlineAt.slice(0, 10)}</p>
                <div className="message-list">
                  {rfq.bids.map((bid) => (
                    <form key={bid.id} className="inline-form" action={`/buyer/rfqs/${rfq.id}/award`} method="post">
                      <input type="hidden" name="bidId" value={bid.id} />
                      <input type="hidden" name="fundingMode" value="credit" />
                      <input type="hidden" name="creditLineId" value="credit_1" />
                      <div>
                        <strong>{bid.providerOrgId}</strong>
                        <p>
                          {bid.status} · quote {formatMoney(bid.quoteCents)}
                        </p>
                      </div>
                      <button type="submit" className="action-button">
                        Award
                      </button>
                    </form>
                  ))}
                </div>
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
                {milestone.title} · <StatusBadge status={milestone.state} />
              </strong>
              <p>
                Budget {formatMoney(milestone.budgetCents)} · Settled {formatMoney(milestone.settledCents)}
              </p>
              <ProgressBar
                current={milestone.settledCents}
                total={milestone.budgetCents}
                tone={milestone.settledCents > milestone.budgetCents * PROGRESS_WARNING_THRESHOLD ? "warning" : "default"}
              />
            </div>
          ))}
          {!data.activeOrders[0]?.milestones.length && (
            <EmptyState icon="📋" message="No active milestones yet." />
          )}
        </div>
      </article>
    </PortalShell>
  );
}
