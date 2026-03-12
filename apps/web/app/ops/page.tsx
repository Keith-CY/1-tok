import { PortalShell } from "../../components/portal-shell";
import { SummaryCard } from "../../components/summary-card";
import { getOpsDashboardData } from "../../lib/api";
import { requirePortalViewer } from "../../lib/viewer";

export const dynamic = "force-dynamic";

export default async function OpsPage() {
  const viewer = await requirePortalViewer("ops", "/ops");
  const data = await getOpsDashboardData({
    authToken: viewer.token,
    requireLive: true,
  });

  return (
    <PortalShell
      eyebrow="Ops portal / treasury + governance"
      title="The platform only looks graceful if the ugly parts are observable."
      copy="This view keeps provider review, credit discipline, dispute payouts, and channel stress in the same sightline. It should feel like the place where market trust is actively manufactured."
      signal="Platform-first reimbursement, provider recovery second"
      asideTitle="Ops signal deck"
      asideItems={[
        { label: "Active orders", value: `${data.summary.activeOrders}`, tone: "warning" },
        { label: "Open disputes", value: `${data.summary.openDisputes}`, tone: "danger" },
        { label: "Settled invoices", value: `${data.summary.settledInvoices}` },
      ]}
    >
      <div className="stat-grid">
        <SummaryCard
          kicker="Active orders"
          value={`${data.summary.activeOrders}`}
          hint="Orders currently visible to the ops control plane."
        />
        <SummaryCard
          kicker="Open disputes"
          value={`${data.summary.openDisputes}`}
          hint="Disputes currently waiting on reimbursement, recovery, or manual ops review."
        />
        <SummaryCard
          kicker="Settled invoices"
          value={`${data.summary.settledInvoices}`}
          hint="Funding records already marked settled by the settlement service."
        />
        <SummaryCard
          kicker="Pending withdrawals"
          value={`${data.summary.pendingWithdrawals}`}
          hint="Withdrawal records still moving through the settlement queue."
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
        <span className="tag">Dispute queue</span>
        <h3>Platform-first reimbursement only works if disputes stay visible.</h3>
        <div className="timeline">
          {data.disputes.map((dispute) => (
            <div key={dispute.id} className="timeline-item">
              <strong>
                {dispute.orderId} · {dispute.milestoneId}
              </strong>
              <p>
                {dispute.reason} · refund {dispute.refundCents}
              </p>
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
