import { PortalShell } from "../../../components/portal-shell";
import { StatusBadge, EmptyState } from "../../../components/ui";
import { requirePortalViewer } from "../../../lib/viewer";
import { formatCents } from "../../../lib/currency";

export const dynamic = "force-dynamic";

export default async function OpsDisputesPage() {
  const viewer = await requirePortalViewer("ops", "/ops/disputes");

  // Demo data
  const disputes = [
    {
      id: "disp_1", orderId: "ord_14", milestoneId: "ms_1",
      reason: "Carrier summary did not match actual remediation.",
      refundCents: 900, status: "open", createdAt: "2026-03-12T00:00:00Z",
    },
  ];

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
        <div className="flex gap-2 mb-4">
          <a href="?status=open" className="action-button">Open</a>
          <a href="?status=resolved" className="action-button">Resolved</a>
          <a href="?status=all" className="action-button">All</a>
        </div>

        {disputes.length === 0 ? (
          <EmptyState message="No disputes to review." actionLabel="Open dispute controls" actionHref="/ops" />
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
                    <p className="text-xs text-gray-400 mt-1">
                      Order: {d.orderId} / Milestone: {d.milestoneId}
                    </p>
                  </div>
                  <div className="text-right">
                    <p className="font-bold text-red-600">{formatCents(d.refundCents)}</p>
                    <p className="text-xs text-gray-400">refund requested</p>
                  </div>
                </div>
                {d.status === "open" && (
                  <div className="flex gap-2 mt-3">
                    <button className="action-button">Approve Refund</button>
                    <button className="action-button">Reject</button>
                    <a href={`/ops/disputes/${d.id}/evidence`} className="action-button">View Evidence</a>
                  </div>
                )}
              </div>
            ))}
          </div>
        )}
      </div>
    </PortalShell>
  );
}
