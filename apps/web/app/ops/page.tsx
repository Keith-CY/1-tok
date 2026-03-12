import { formatMoney } from "@1tok/contracts";

import { PortalShell } from "../../components/portal-shell";
import { SummaryCard } from "../../components/summary-card";
import { getOpsDashboardData } from "../../lib/api";

export const dynamic = "force-dynamic";

export default async function OpsPage() {
  const data = await getOpsDashboardData();

  return (
    <PortalShell
      eyebrow="Ops portal / treasury + governance"
      title="The platform only looks graceful if the ugly parts are observable."
      copy="This view keeps provider review, credit discipline, dispute payouts, and channel stress in the same sightline. It should feel like the place where market trust is actively manufactured."
      signal="Platform-first reimbursement, provider recovery second"
      asideTitle="Ops signal deck"
      asideItems={[
        { label: "Exposure", value: formatMoney(data.summary.outstandingExposureCents), tone: "warning" },
        { label: "Open disputes", value: `${data.summary.openDisputes}`, tone: "danger" },
        { label: "Provider reviews", value: `${data.summary.pendingProviderReviews}` },
      ]}
    >
      <div className="stat-grid">
        <SummaryCard
          kicker="Exposure"
          value={formatMoney(data.summary.outstandingExposureCents)}
          hint="Capital already advanced by the platform across active credit-funded orders."
        />
        <SummaryCard
          kicker="Open disputes"
          value={`${data.summary.openDisputes}`}
          hint="Cases where buyer reimbursement may convert into provider recovery or write-down."
        />
        <SummaryCard
          kicker="Pending reviews"
          value={`${data.summary.pendingProviderReviews}`}
          hint="Provider onboarding, capability checks, or credit line overrides waiting on operators."
        />
        <SummaryCard
          kicker="Fiber channels"
          value={`${data.summary.activeChannels}`}
          hint="Live platform-controlled channels currently carrying active market volume."
        />
      </div>

      <div className="feed-grid">
        <article className="feed-card">
          <span className="tag">Pending reviews</span>
          <h3>Items that require a human decision, not another dashboard filter.</h3>
          <div className="feed-list">
            {data.pendingReviews.map((review) => (
              <div key={review.id} className="feed-item">
                <strong>{review.title}</strong>
                <p>{review.detail}</p>
              </div>
            ))}
          </div>
        </article>

        <aside className="message-card">
          <span className="tag">Treasury signals</span>
          <h3>Read the funding posture at a glance.</h3>
          <div className="chip-list">
            {data.treasurySignals.map((signal) => (
              <div key={signal.id} className="chip">
                {signal.label}
                <span>{signal.value}</span>
              </div>
            ))}
          </div>
        </aside>
      </div>

      <article className="timeline-card">
        <span className="tag">Risk feed</span>
        <h3>Today’s market pressure points.</h3>
        <div className="timeline">
          {data.riskFeed.map((item) => (
            <div key={item.id} className="timeline-item">
              <strong>{item.title}</strong>
              <p>{item.detail}</p>
            </div>
          ))}
        </div>
      </article>

      <article className="timeline-card">
        <span className="tag">Funding journal</span>
        <h3>Live money movement, not demo theater.</h3>
        <div className="timeline">
          {data.fundingRecords.map((record) => (
            <div key={record.id} className="timeline-item">
              <strong>
                {record.kind.toUpperCase()} · {record.state}
              </strong>
              <p>
                {record.amount}
                {record.asset ? ` ${record.asset}` : ""} · {record.providerOrgId ?? record.buyerOrgId ?? "platform"}
                {record.invoice ? ` · ${record.invoice}` : record.externalId ? ` · ${record.externalId}` : ""}
              </p>
            </div>
          ))}
        </div>
      </article>
    </PortalShell>
  );
}
