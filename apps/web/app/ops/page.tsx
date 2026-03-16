import { PortalShell } from "../../components/portal-shell";
import { SummaryCard } from "../../components/summary-card";
import { EmptyState } from "../../components/ui";
import { getOpsDashboardData } from "../../lib/api";
import { requirePortalViewer } from "../../lib/viewer";

export const dynamic = "force-dynamic";

export default async function OpsPage({
  searchParams,
}: {
  searchParams?: Record<string, string | string[] | undefined>;
}) {
  const viewer = await requirePortalViewer("ops", "/ops");
  const data = await getOpsDashboardData({
    authToken: viewer.token,
    requireLive: true,
  });
  const creditApproved = readSearchParam(searchParams, "creditApproved");
  const recommendedLimitCents = readSearchParam(searchParams, "recommendedLimitCents");
  const creditReason = readSearchParam(searchParams, "creditReason");
  const resolvedDisputeId = readSearchParam(searchParams, "resolvedDisputeId");
  const disputeStatus = readSearchParam(searchParams, "disputeStatus");
  const error = readSearchParam(searchParams, "error");

  return (
    <PortalShell
      eyebrow="Ops portal / treasury + governance"
      title="The platform only looks graceful if the ugly parts are observable."
      copy="This view keeps provider review, credit discipline, dispute payouts, and channel stress in the same sightline. It should feel like the place where market trust is actively manufactured."
      signal="Platform-first reimbursement, provider recovery second"
      asideTitle="Ops signal deck"
      quickActions={[
        { label: "Run credit decision", href: "#credit-decision", tone: "primary" },
        { label: "Review disputes", href: "#disputes", tone: "secondary" },
        { label: "Treasury controls", href: "#treasury", tone: "secondary" },
      ]}
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
        <article className="feed-card" id="credit-decision">
          <span className="tag">Credit review</span>
          <h3>Ops should be able to re-price buyer trust without leaving the control plane.</h3>
          <form className="auth-form market-form" action="/ops/credits/decision" method="post">
            <div className="market-form__grid">
              <label className="auth-field">
                <span>Completed orders</span>
                <input name="completedOrders" type="number" min="0" step="1" defaultValue="12" required />
              </label>
              <label className="auth-field">
                <span>Successful payments</span>
                <input name="successfulPayments" type="number" min="0" step="1" defaultValue="11" required />
              </label>
              <label className="auth-field">
                <span>Failed payments</span>
                <input name="failedPayments" type="number" min="0" step="1" defaultValue="1" required />
              </label>
              <label className="auth-field">
                <span>Disputed orders</span>
                <input name="disputedOrders" type="number" min="0" step="1" defaultValue={data.summary.openDisputes} required />
              </label>
            </div>
            <label className="auth-field">
              <span>Lifetime spend cents</span>
              <input name="lifetimeSpendCents" type="number" min="0" step="1" defaultValue="480000" required />
            </label>
            <button type="submit" className="action-button">
              Run credit decision
            </button>
          </form>
        </article>

        <aside className="message-card" id="dispute-result">
          <span className="tag">Decision result</span>
          <h3>Show the last recommendation with the exact reason returned by policy.</h3>
          <div className="message-list">
            <div className="message-item">
              <strong>{creditApproved === "true" ? "Approved" : creditApproved === "false" ? "Rejected" : "No decision yet"}</strong>
              <p>Recommended limit {recommendedLimitCents || "0"} cents</p>
              <p>{creditReason || "Submit buyer history to generate a live recommendation."}</p>
            </div>
          </div>
        </aside>
      </div>

      <div className="feed-grid">
        <article className="feed-card">
          <span className="tag">Dispute action result</span>
          <h3>Ops actions should echo back into the queue immediately.</h3>
          <div className="feed-list">
            <div className="feed-item">
              <strong>{resolvedDisputeId ? `Resolved ${resolvedDisputeId}` : "No dispute action yet"}</strong>
              <p>
                {resolvedDisputeId
                  ? `Latest dispute action returned status ${disputeStatus || "resolved"}.`
                  : error === "dispute-resolution-failed"
                    ? "The dispute action failed. Retry from the queue below."
                    : "Resolve an open dispute from the queue to write a live ops action into the system."}
              </p>
            </div>
          </div>
        </article>

        <aside className="message-card" id="treasury">
          <span className="tag">Action posture</span>
          <h3>Open cases stay actionable, resolved cases stay legible.</h3>
          <div className="chip-list">
            <div className="chip">
              Open queue
              <span>{data.summary.openDisputes}</span>
            </div>
            <div className="chip">
              Resolved visible
              <span>{data.disputes.filter((dispute) => dispute.status === "resolved").length}</span>
            </div>
          </div>
        </aside>
      </div>

      <div className="feed-grid">
        <article className="feed-card">
          <span className="tag">Pending reviews</span>
          <h3>Items that require a human decision, not another dashboard filter.</h3>
          <div className="feed-list">
            {data.pendingReviews.length === 0 ? (
              <EmptyState icon="✅" message="No pending manual reviews right now." actionLabel="Check disputes" actionHref="#disputes" />
            ) : null}
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
            {data.treasurySignals.length === 0 ? (
              <EmptyState icon="🏦" message="No treasury signal changes in the last interval." actionLabel="Review disputes" actionHref="#disputes" />
            ) : null}
            {data.treasurySignals.map((signal) => (
              <div key={signal.id} className="chip">
                {signal.label}
                <span>{signal.value}</span>
              </div>
            ))}
          </div>
        </aside>
      </div>

      <article className="timeline-card" id="risk-feed">
        <span className="tag">Risk feed</span>
        <h3>Today’s market pressure points.</h3>
        <div className="timeline">
          {data.riskFeed.length === 0 ? (
            <EmptyState icon="📈" message="No risk alerts in the last period." actionLabel="Open treasury signals" actionHref="#treasury" />
          ) : null}
          {data.riskFeed.map((item) => (
            <div key={item.id} className="timeline-item">
              <strong>{item.title}</strong>
              <p>{item.detail}</p>
            </div>
          ))}
        </div>
      </article>

      <article className="timeline-card" id="disputes">
        <span className="tag">Dispute queue</span>
        <h3>Platform-first reimbursement only works if disputes stay visible.</h3>
        <div className="timeline">
          {data.disputes.length === 0 ? (
            <EmptyState icon="⚖️" message="No disputes in queue." actionLabel="View credit decision" actionHref="#credit-decision" />
          ) : null}
          {data.disputes.map((dispute) => (
            <div key={dispute.id} className="timeline-item">
              <strong>
                {dispute.orderId} · {dispute.milestoneId} · {dispute.status}
              </strong>
              <p>
                {dispute.reason} · refund {dispute.refundCents}
              </p>
              {dispute.status === "open" ? (
                <form className="auth-form market-form" action={`/ops/disputes/${dispute.id}/resolve`} method="post">
                  <label className="auth-field">
                    <span>Resolution note</span>
                    <textarea
                      name="resolution"
                      rows={2}
                      defaultValue="Approved reimbursement after ops evidence review."
                      required
                    />
                  </label>
                  <button type="submit" className="action-button">
                    Resolve dispute
                  </button>
                </form>
              ) : (
                <p>
                  {dispute.resolution || "Resolved by ops."}
                  {dispute.resolvedBy ? ` · ${dispute.resolvedBy}` : ""}
                </p>
              )}
            </div>
          ))}
        </div>
      </article>

      <article className="timeline-card" id="journal">
        <span className="tag">Funding journal</span>
        <h3>Live money movement, not demo theater.</h3>
        <div className="timeline">
          {data.fundingRecords.length === 0 ? (
            <EmptyState icon="📚" message="No funding records to display yet." actionLabel="Open treasury controls" actionHref="#treasury" />
          ) : null}
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

function readSearchParam(
  searchParams: Record<string, string | string[] | undefined> | undefined,
  key: string,
): string {
  const value = searchParams?.[key];
  return Array.isArray(value) ? value[0] ?? "" : value ?? "";
}
