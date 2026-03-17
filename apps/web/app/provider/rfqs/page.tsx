import { PortalShell } from "../../../components/portal-shell";
import { StatusBadge, EmptyState } from "../../../components/ui";
import { requirePortalViewer } from "../../../lib/viewer";
import { formatCents } from "../../../lib/currency";

export const dynamic = "force-dynamic";

export default async function ProviderRFQsPage({
  searchParams,
}: {
  searchParams?: { q?: string; category?: string; status?: string; sort?: string };
}) {
  const viewer = await requirePortalViewer("provider", "/provider/rfqs");

  // Demo data — will be replaced with API
  const openRFQs = [
    { id: "rfq_1", title: "Agent runtime triage", category: "agent-ops", status: "open", budgetCents: 5400, deadline: "2026-03-15T12:00:00Z" },
    { id: "rfq_2", title: "Data pipeline cleanup", category: "data-pipeline", status: "awarded", budgetCents: 8200, deadline: "2026-03-18T12:00:00Z" },
  ];

  const q = (searchParams?.q ?? "").trim().toLowerCase();
  const categoryFilter = (searchParams?.category ?? "all").toLowerCase();
  const statusFilter = (searchParams?.status ?? "all").toLowerCase();
  const sort = (searchParams?.sort ?? "deadline").toLowerCase();

  const categoryValue = categoryFilter !== "all" ? categoryFilter : "";

  const chipClass = (active: boolean) => (active ? "action-button action-button--active" : "action-button");

  const sortValue = sort !== "deadline" ? sort : "";

  const buildCategoryHref = (nextCategory: string) => {
    const params = new URLSearchParams();

    if (q) {
      params.set("q", q);
    }

    if (statusFilter !== "all") {
      params.set("status", statusFilter);
    }

    if (sortValue) {
      params.set("sort", sortValue);
    }

    if (nextCategory !== "all") {
      params.set("category", nextCategory);
    }

    const queryString = params.toString();
    return queryString ? `/provider/rfqs?${queryString}` : "/provider/rfqs";
  };

  const buildStatusHref = (nextStatus: string) => {
    const params = new URLSearchParams();

    if (q) {
      params.set("q", q);
    }

    if (categoryValue) {
      params.set("category", categoryValue);
    }

    if (sortValue) {
      params.set("sort", sortValue);
    }

    if (nextStatus !== "all") {
      params.set("status", nextStatus);
    }

    const queryString = params.toString();
    return queryString ? `/provider/rfqs?${queryString}` : "/provider/rfqs";
  };

  const buildSortHref = (nextSort: string) => {
    const params = new URLSearchParams();

    if (q) {
      params.set("q", q);
    }

    if (categoryValue) {
      params.set("category", categoryValue);
    }

    if (statusFilter !== "all") {
      params.set("status", statusFilter);
    }

    if (nextSort !== "deadline") {
      params.set("sort", nextSort);
    }

    const queryString = params.toString();
    return queryString ? `/provider/rfqs?${queryString}` : "/provider/rfqs";
  };

  const filteredRFQs = openRFQs
    .filter(
      (rfq) =>
        (!q || rfq.title.toLowerCase().includes(q) || rfq.category.toLowerCase().includes(q) || rfq.status.toLowerCase().includes(q)) &&
        (categoryFilter === "all" || rfq.category === categoryFilter) &&
        (statusFilter === "all" || rfq.status === statusFilter),
    )
    .sort((a, b) => {
      if (sort === "budget") {
        return b.budgetCents - a.budgetCents;
      }
      if (sort === "title") {
        return a.title.localeCompare(b.title);
      }
      if (sort === "deadline") {
        return new Date(a.deadline).getTime() - new Date(b.deadline).getTime();
      }
      return 0;
    });

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

        <div className="flex gap-2 mb-2">
          <a href={buildCategoryHref("all")} className={chipClass(categoryFilter === 'all')} aria-current={categoryFilter === "all" ? "page" : undefined}>
            All categories
          </a>
          <a href={buildCategoryHref("agent-ops")} className={chipClass(categoryFilter === 'agent-ops')} aria-current={categoryFilter === "agent-ops" ? "page" : undefined}>
            Agent Ops
          </a>
          <a href={buildCategoryHref("agent-runtime")} className={chipClass(categoryFilter === 'agent-runtime')} aria-current={categoryFilter === "agent-runtime" ? "page" : undefined}>
            Agent Runtime
          </a>
          <a href={buildCategoryHref("data-pipeline")} className={chipClass(categoryFilter === 'data-pipeline')} aria-current={categoryFilter === "data-pipeline" ? "page" : undefined}>
            Data Pipeline
          </a>
          <a href={buildCategoryHref("compute")} className={chipClass(categoryFilter === 'compute')} aria-current={categoryFilter === "compute" ? "page" : undefined}>
            Compute
          </a>
        </div>
        <div className="flex gap-2 mb-2">
          <a href={buildStatusHref("all")} className={chipClass(statusFilter === 'all')} aria-current={statusFilter === "all" ? "page" : undefined}>
            All
          </a>
          <a href={buildStatusHref("open")} className={chipClass(statusFilter === 'open')} aria-current={statusFilter === "open" ? "page" : undefined}>
            Open
          </a>
          <a href={buildStatusHref("awarded")} className={chipClass(statusFilter === 'awarded')} aria-current={statusFilter === "awarded" ? "page" : undefined}>
            Awarded
          </a>
        </div>
        <div className="flex gap-2 mb-2">
          <a href={buildSortHref("deadline")} className={chipClass(sort === "deadline")} aria-current={sort === "deadline" ? "page" : undefined}>
            Sort by deadline
          </a>
          <a href={buildSortHref("budget")} className={chipClass(sort === "budget")} aria-current={sort === "budget" ? "page" : undefined}>
            Sort by budget
          </a>
          <a href={buildSortHref("title")} className={chipClass(sort === "title")} aria-current={sort === "title" ? "page" : undefined}>
            Sort by title
          </a>
        </div>
        <form method="GET" className="auth-form market-form">
          <div className="market-form__grid">
            <label className="auth-field">
              <span>Search opportunities</span>
              <input name="q" type="text" placeholder="Search by title or category" defaultValue={q} />
            </label>
            <label className="auth-field">
              <span>Category</span>
              <select name="category" defaultValue={categoryFilter}>
                <option value="all">All categories</option>
                <option value="agent-ops">Agent Ops</option>
                <option value="agent-runtime">Agent Runtime</option>
                <option value="data-pipeline">Data Pipeline</option>
                <option value="compute">Compute</option>
              </select>
            </label>
            <label className="auth-field">
              <span>Status</span>
              <select name="status" defaultValue={statusFilter}>
                <option value="all">All</option>
                <option value="open">Open</option>
                <option value="awarded">Awarded</option>
              </select>
            </label>
            <label className="auth-field">
              <span>Sort</span>
              <select name="sort" defaultValue={sort}>
                <option value="deadline">Deadline</option>
                <option value="budget">Budget</option>
                <option value="title">Title</option>
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
                      <StatusBadge status={rfq.status} />
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
