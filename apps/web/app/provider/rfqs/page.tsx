import { PortalShell } from "../../../components/portal-shell";
import { StatusBadge, EmptyState } from "../../../components/ui";
import { requirePortalViewer } from "../../../lib/viewer";
import { formatCents } from "../../../lib/currency";

export const dynamic = "force-dynamic";

export default async function ProviderRFQsPage() {
  const viewer = await requirePortalViewer("provider", "/provider/rfqs");

  // Demo data — will be replaced with API
  const openRFQs = [
    { id: "rfq_1", title: "Agent runtime triage", category: "agent-ops", budgetCents: 5400, deadline: "2026-03-15T12:00:00Z" },
  ];

  return (
    <PortalShell
      eyebrow="Provider portal / opportunities"
      title="Open RFQs."
      copy="Browse open requests for quotes and submit bids."
      signal="RFQ discovery"
      asideTitle="Quick info"
      asideItems={[]}
    >
      <div className="space-y-4">
        {openRFQs.length === 0 ? (
          <EmptyState message="No open RFQs available." />
        ) : (
          <div className="space-y-3">
            {openRFQs.map((rfq) => (
              <div key={rfq.id} className="border rounded-lg p-4 hover:shadow-md transition-shadow">
                <div className="flex justify-between items-start">
                  <div>
                    <h3 className="font-semibold text-lg">{rfq.title}</h3>
                    <p className="text-sm text-gray-500">{rfq.category}</p>
                  </div>
                  <div className="text-right">
                    <p className="font-bold text-lg">{formatCents(rfq.budgetCents)}</p>
                    <p className="text-xs text-gray-400">budget</p>
                  </div>
                </div>
                <div className="flex justify-between items-center mt-3">
                  <span className="text-xs text-gray-500">
                    Deadline: {new Date(rfq.deadline).toLocaleDateString()}
                  </span>
                  <a href={`/provider/rfqs/${rfq.id}/bid`} className="bg-blue-600 text-white px-3 py-1 rounded text-sm">
                    Submit Bid
                  </a>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </PortalShell>
  );
}
