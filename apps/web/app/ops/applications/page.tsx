import { PortalShell } from "../../../components/portal-shell";
import { EmptyState } from "../../../components/ui";
import { requirePortalViewer } from "../../../lib/viewer";

export const dynamic = "force-dynamic";

const APPLICATIONS = [
  {
    id: "app_1",
    providerOrg: "Atlas Ops",
    category: "agent-ops",
    contact: "ops-support@atlas.io",
    status: "pending",
    submittedAt: "2026-03-15",
    notes: "Carrier-first remediation experience with SLA commitments.",
  },
  {
    id: "app_2",
    providerOrg: "Kite Relay",
    category: "agent-runtime",
    contact: "ops@kiterelay.io",
    status: "approved",
    submittedAt: "2026-03-12",
    notes: "Distributed execution and custom orchestration support.",
  },
  {
    id: "app_3",
    providerOrg: "Cloudline Runtime",
    category: "compute",
    contact: "infra@cloudline.run",
    status: "rejected",
    submittedAt: "2026-03-10",
    notes: "Duplicate identity; follow-up requested.",
  },
];

export default async function OpsApplicationsPage({
  searchParams,
}: {
  searchParams?: { q?: string; status?: string };
}) {
  const viewer = await requirePortalViewer("ops", "/ops/applications");

  const query = (searchParams?.q ?? "").trim();
  const queryLower = query.toLowerCase();
  const status = (searchParams?.status ?? "pending").toLowerCase();

  const encodedQuery = encodeURIComponent(query);

  const applications = APPLICATIONS.filter(
    (application) =>
      (status === "all" || application.status === status) &&
      (!queryLower ||
        application.providerOrg.toLowerCase().includes(queryLower) ||
        application.category.toLowerCase().includes(queryLower) ||
        application.contact.toLowerCase().includes(queryLower) ||
        application.notes.toLowerCase().includes(queryLower)),
  );

  return (
    <PortalShell
      eyebrow="Ops portal / vetting"
      title="Provider application review."
      copy="Review pending provider applications. Approve or reject with notes."
      signal="Provider vetting"
      asideTitle="Quick info"
      quickActions={[
        { label: "All applications", href: "/ops/applications", tone: "secondary" },
        { label: "Resolved disputes", href: "/ops/disputes?status=resolved", tone: "secondary" },
        { label: "Open disputes", href: "/ops/disputes", tone: "primary" },
      ]}
      asideItems={[]}
    >
      <div className="space-y-4">
        <form method="GET" className="auth-form market-form">
          <div className="market-form__grid">
            <label className="auth-field">
              <span>Search applications</span>
              <input
                name="q"
                type="text"
                placeholder="Search provider, category, contact, notes"
                defaultValue={searchParams?.q ?? ""}
              />
            </label>
            <label className="auth-field">
              <span>Status</span>
              <select name="status" defaultValue={searchParams?.status ?? "pending"}>
                <option value="pending">Pending</option>
                <option value="approved">Approved</option>
                <option value="rejected">Rejected</option>
                <option value="all">All</option>
              </select>
            </label>
          </div>
          <button type="submit" className="auth-submit">
            Filter applications
          </button>
        </form>

        <div className="flex gap-2 mb-2">
          <a href={`/ops/applications?status=pending${query ? `&q=${encodedQuery}` : ""}`} className="action-button">Pending</a>
          <a href={`/ops/applications?status=approved${query ? `&q=${encodedQuery}` : ""}`} className="action-button">Approved</a>
          <a href={`/ops/applications?status=rejected${query ? `&q=${encodedQuery}` : ""}`} className="action-button">Rejected</a>
          <a href={`/ops/applications?status=all${query ? `&q=${encodedQuery}` : ""}`} className="action-button">All</a>
        </div>

        {applications.length === 0 ? (
          <EmptyState message="No applications match this filter." actionLabel="Clear filters" actionHref="/ops/applications" />
        ) : (
          <div className="space-y-3">
            {applications.map((application) => (
              <div key={application.id} className="border rounded-lg p-4">
                <div className="flex justify-between items-start">
                  <div>
                    <p className="text-xs text-gray-500">{application.status.toUpperCase()}</p>
                    <h3 className="font-semibold text-lg">{application.providerOrg}</h3>
                    <p className="text-sm text-gray-500">
                      {application.category} · {application.contact}
                    </p>
                    <p className="text-sm text-gray-500">Submitted {application.submittedAt}</p>
                  </div>
                  <a href={`/ops/applications/${application.id}`} className="action-button">
                    Open application
                  </a>
                </div>
                <p className="mt-2 text-sm">{application.notes}</p>
              </div>
            ))}
          </div>
        )}
      </div>
    </PortalShell>
  );
}
