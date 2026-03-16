import { PortalShell } from "../../../components/portal-shell";
import { StatusBadge, EmptyState } from "../../../components/ui";
import { requirePortalViewer } from "../../../lib/viewer";
import { formatCents } from "../../../lib/currency";

export const dynamic = "force-dynamic";

export default async function ProviderRFQsPage({
  searchParams,
}: {
  searchParams?: { q?: string; category?: string };
}) {
  const viewer = await requirePortalViewer("provider", "/provider/rfqs");

  // Demo data — will be replaced with API
  const openRFQs = [
    { id: "rfq_1", title: "Agent runtime triage", category: "agent-ops", budgetCents: 5400, deadline: "2026-03-15T12:00:00Z" },
    { id: "rfq_2", title: "Data pipeline cleanup", category: "data-pipeline", budgetCents: 8200, deadline: "2026-03-18T12:00:00Z" },
  ];

  const q = (searchParams?.q ?? "").trim().toLowerCase();
  const categoryFilter = (searchParams?.category ?? "all").toLowerCase();

  const filteredRFQs = openRFQs
    .filter(
      (rfq) =>
        !q ||
        rfq.title.toLowerCase().includes(q) ||
        rfq.category.toLowerCase().includes(q),
    )
    .filter((rfq) => categoryFilter === "all" || rfq.category === categoryFilter);

  return (
    <PortalShell
      eyebrow="Provider portal / opportunities"
      title="Open RFQs."
      copy="Browse open requests for quotes and submit bids."
      signal="RFQ discovery"
      asideTitle="Quick info"
      quickActions={[
        { label: "Back to provider dashboard", href: "/provider", tone: "secondary" },
        { label: "Carrier operations", href: "/provider/carrier", tone: "secondary" },
      ]}
      asideItems={[]}
    >
      <div className="space-y-4">
        <form method="GET" className="auth-form market-form">
          <div className="market-form__grid">
            <label className="auth-field">
              <span>Search opportunities</span>
              <input name="q" type="text" placeholder="Search by title or category" defaultValue={searchParams?.q ?? ""} />
            </label>
            <label className="auth-field">
              <span>Category</span>
              <select name="category" defaultValue={searchParams?.category ?? "all"}>
                <option value="all">All categories</option>
                <option value="agent-ops">Agent Ops</option>
                <option value="agent-runtime">Agent Runtime</option>
                <option value="data-pipeline">Data Pipeline</option>
                <option value="compute">Compute</option>
              </select>
            </label>
          </div>
          <button type="submit" className="auth-submit">
            Find opportunities
          </button>
        </form>

        {filteredRFQs.length === 0 ? (
          <EmptyState
            message="No RFQs match your filter."
            actionLabel="Open all opportunities"
            actionHref="/provider/rfqs"
          />
        ) : (
          <div className="space-y-3">
            {filteredRFQs.map((rfq) => (
              <div key={rfq.id} className="border rounded-lg p-4 hover:shadow-md transition-shadow">
                <div className="flex justify-between items-start">
                  <div>
                    <h3 className="font-semibold text-lg">{rfq.title}</h3>
                    <p className="text-sm text-gray-500">{rfq.category}</p>
                    <div className="mt-1 text-xs">
                      <StatusBadge status="open" />
                    </div>
                  </div>
                  <div className="text-right">
                    <p className="font-bold text-lg">{formatCents(rfq.budgetCents)}</p>
                    <p className="text-xs text-gray-400">budget</p>
                  </div>
                </div>
                <div className="flex justify-between items-center mt-3">
                  <span className="text-xs text-gray-500">Deadline: {new Date(rfq.deadline).toLocaleDateString()}</span>
                  <a href="/provider#opportunities" className="action-button">
                    Review in pipeline
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
