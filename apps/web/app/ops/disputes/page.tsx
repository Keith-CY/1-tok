import { PortalShell } from "../../../components/portal-shell";
import { StatusBadge, EmptyState } from "../../../components/ui";
import { requirePortalViewer } from "../../../lib/viewer";
import { formatCents } from "../../../lib/currency";

export const dynamic = "force-dynamic";

const DISPUTES = [
  {
    id: "disp_1",
    orderId: "ord_14",
    milestoneId: "ms_1",
    reason: "Carrier summary did not match actual remediation.",
    refundCents: 900,
    status: "open",
    buyerOrgId: "Buyer Ops",
    createdAt: "2026-03-12T00:00:00Z",
    resolvedAt: "",
  },
  {
    id: "disp_2",
    orderId: "ord_20",
    milestoneId: "ms_2",
    reason: "Service quality did not meet stated SLO.",
    refundCents: 1700,
    status: "resolved",
    buyerOrgId: "Acme Retail",
    createdAt: "2026-03-08T00:00:00Z",
    resolvedAt: "2026-03-09T11:12:00Z",
  },
];

export default async function OpsDisputesPage({
  searchParams,
}: {
  searchParams?: { q?: string; status?: string };
}) {
  const viewer = await requirePortalViewer("ops", "/ops/disputes");

  const query = (searchParams?.q ?? "").trim().toLowerCase();
  const status = (searchParams?.status ?? "open").toLowerCase();

  const disputes = DISPUTES.filter(
    (d) =>
      (status === "all" || d.status === status) &&
      (!query ||
        d.orderId.toLowerCase().includes(query) ||
        d.milestoneId.toLowerCase().includes(query) ||
        d.buyerOrgId.toLowerCase().includes(query) ||
        d.reason.toLowerCase().includes(query)),
  );

  return (
    <PortalShell
      eyebrow="Ops portal / disputes"
      title="Dispute arbitration."
      copy="Review open disputes, examine evidence, and decide on refund/recovery."
      signal="Dispute review"
      asideTitle="Quick info"
      quickActions={[
        { label: "Review applications", href: "/ops/applications", tone: "secondary" },
        { label: "Credit decision", href: "/ops#credit-decision", tone: "secondary" },
        { label: "Dispute evidence", href: "/ops/disputes", tone: "primary" },
      ]}
      asideItems={[]}
    >
      <div className="space-y-4">
        <form method="GET" className="auth-form market-form">
          <div className="market-form__grid">
            <label className="auth-field">
              <span>Search disputes</span>
              <input
                name="q"
                type="text"
                placeholder="Search by order, milestone, buyer, reason"
                defaultValue={searchParams?.q ?? ""}
              />
            </label>
            <label className="auth-field">
              <span>Status</span>
              <select name="status" defaultValue={searchParams?.status ?? "open"}>
                <option value="open">Open</option>
                <option value="resolved">Resolved</option>
                <option value="all">All</option>
              </select>
            </label>
          </div>
          <button type="submit" className="auth-submit">
            Filter disputes
          </button>
        </form>

        <div className="flex gap-2 mb-2">
          <a href="?status=open" className="action-button">Open</a>
          <a href="?status=resolved" className="action-button">Resolved</a>
          <a href="?status=all" className="action-button">All</a>
        </div>

        {disputes.length === 0 ? (
          <EmptyState message="No disputes to review." actionLabel="Clear filters" actionHref="/ops/disputes" />
        ) : (
          <div className="space-y-3">
            {disputes.map((d) => (
              <div key={d.id} className="border rounded-lg p-4">
                <div className="flex justify-between items-start">
                  <div>
                    <div className="flex items-center gap-2">
                      <h3 className="font-semibold">{d.id}</h3>
                      <StatusBadge status={d.status} />
                    </div>
                    <p className="text-sm text-gray-600 mt-1">{d.reason}</p>
                    <p className="text-xs text-gray-400 mt-1">Order {d.orderId} · Milestone {d.milestoneId} · Buyer {d.buyerOrgId}</p>
                    <p className="text-xs text-gray-400">
                      {d.status === "resolved" ? `Resolved ${d.resolvedAt}` : `Opened ${d.createdAt.slice(0, 10)}`}
                    </p>
                  </div>
                  <div className="text-right">
                    <p className="font-bold text-red-600">{formatCents(d.refundCents)}</p>
                    <p className="text-xs text-gray-400">refund requested</p>
                  </div>
                </div>

                {d.status === "open" ? (
                  <div className="flex gap-2 mt-3">
                    <button className="action-button">Approve Refund</button>
                    <button className="action-button">Reject</button>
                    <a href={`/ops/disputes/${d.id}/evidence`} className="action-button">View Evidence</a>
                  </div>
                ) : null}
              </div>
            ))}
          </div>
        )}
      </div>
    </PortalShell>
  );
}
