import Link from "next/link";

import { PortalShell } from "../../../components/portal-shell";
import { StatusBadge, EmptyState } from "../../../components/ui";
import { requirePortalViewer } from "../../../lib/viewer";

export const dynamic = "force-dynamic";

const CARRIER_JOBS = [
  {
    id: "job_19",
    orderId: "ord_19",
    title: "Log ingestion remediation",
    status: "running",
    progress: "7/10",
    host: "carrier-runner-01",
    lastSeen: "2m ago",
  },
  {
    id: "job_20",
    orderId: "ord_20",
    title: "Pipeline burst scaling",
    status: "paused",
    progress: "3/6",
    host: "carrier-runner-07",
    lastSeen: "11m ago",
  },
  {
    id: "job_21",
    orderId: "ord_21",
    title: "Agent runtime audit",
    status: "pending",
    progress: "0/4",
    host: "carrier-runner-09",
    lastSeen: "28m ago",
  },
];

export default async function ProviderCarrierPage({
  searchParams,
}: {
  searchParams?: { q?: string; status?: string };
}) {
  const viewer = await requirePortalViewer("provider", "/provider/carrier");

  const q = (searchParams?.q ?? "").trim().toLowerCase();
  const status = (searchParams?.status ?? "all").toLowerCase();
  const encodedQuery = encodeURIComponent((searchParams?.q ?? ""));
  const chipClass = (active: boolean) =>
    active ? "action-button action-button--active" : "action-button";

  const jobs = CARRIER_JOBS.filter(
    (job) =>
      (!q || job.title.toLowerCase().includes(q) || job.orderId.toLowerCase().includes(q) || job.host.toLowerCase().includes(q)) &&
      (status === "all" || job.status === status),
  );

  return (
    <PortalShell
      eyebrow="Provider portal / carrier"
      title="Carrier integration status."
      copy="View your Carrier binding, execution profiles, and active jobs."
      signal="Carrier management"
      asideTitle="Quick info"
      quickActions={[
        { label: "Open your listings", href: "/provider/listings", tone: "secondary" },
        { label: "Review RFQ opportunities", href: "/provider/rfqs", tone: "primary" },
      ]}
      asideItems={[]}
    >
      <div className="space-y-6">
        <section>
          <h2 className="text-xl font-semibold mb-3">Carrier Binding</h2>
          <div className="border rounded-lg p-4">
            <div className="grid grid-cols-2 gap-4">
              <div>
                <p className="text-sm text-gray-500">Status</p>
                <StatusBadge status="pending_verification" />
              </div>
              <div>
                <p className="text-sm text-gray-500">Host</p>
                <p className="font-mono text-sm">carrier-prod.internal</p>
              </div>
              <div>
                <p className="text-sm text-gray-500">Agent</p>
                <p className="font-mono text-sm">{viewer.membership.organization.name}</p>
              </div>
              <div>
                <p className="text-sm text-gray-500">Runtime</p>
                <p className="font-mono text-sm">v0.9.2</p>
              </div>
            </div>
          </div>
        </section>

        <section>
          <div className="flex justify-between items-center">
            <h2 className="text-xl font-semibold mb-3">Active Jobs</h2>
            <Link href="/provider/rfqs" className="action-button">
              Browse opportunities
            </Link>
          </div>

          <div className="flex gap-2 mb-2">
            <a
              href={`/provider/carrier?status=all${q ? `&q=${encodedQuery}` : ""}`}
              className={chipClass(status === "all")}
              aria-current={status === "all" ? "page" : undefined}
            >
              All
            </a>
            <a
              href={`/provider/carrier?status=running${q ? `&q=${encodedQuery}` : ""}`}
              className={chipClass(status === "running")}
              aria-current={status === "running" ? "page" : undefined}
            >
              Running
            </a>
            <a
              href={`/provider/carrier?status=pending${q ? `&q=${encodedQuery}` : ""}`}
              className={chipClass(status === "pending")}
              aria-current={status === "pending" ? "page" : undefined}
            >
              Pending
            </a>
            <a
              href={`/provider/carrier?status=paused${q ? `&q=${encodedQuery}` : ""}`}
              className={chipClass(status === "paused")}
              aria-current={status === "paused" ? "page" : undefined}
            >
              Paused
            </a>
            <a
              href={`/provider/carrier?status=completed${q ? `&q=${encodedQuery}` : ""}`}
              className={chipClass(status === "completed")}
              aria-current={status === "completed" ? "page" : undefined}
            >
              Completed
            </a>
          </div>
          <form method="GET" className="auth-form market-form">
            <div className="market-form__grid">
              <label className="auth-field">
                <span>Search jobs</span>
                <input
                  name="q"
                  type="text"
                  defaultValue={q}
                  placeholder="Search by order id, host, or title"
                />
              </label>
              <label className="auth-field">
                <span>Status</span>
                <select name="status" defaultValue={status}>
                  <option value="all">All states</option>
                  <option value="running">Running</option>
                  <option value="pending">Pending</option>
                  <option value="paused">Paused</option>
                  <option value="completed">Completed</option>
                </select>
              </label>
            </div>
            <button type="submit" className="auth-submit">
              Filter jobs
            </button>
          </form>

          {jobs.length === 0 ? (
            <EmptyState
              message="No active jobs match your filters."
              actionLabel="Clear filters"
              actionHref="/provider/carrier"
            />
          ) : (
            <div className="feed-list mt-4">
              {jobs.map((job) => (
                <div key={job.id} className="feed-item">
                  <div className="flex justify-between items-start gap-4">
                    <div>
                      <strong>
                        {job.title} · {job.orderId}
                      </strong>
                      <p>
                        Host {job.host} · Progress {job.progress} · Last seen {job.lastSeen}
                      </p>
                    </div>
                    <StatusBadge status={job.status} />
                  </div>
                </div>
              ))}
            </div>
          )}
        </section>
      </div>
    </PortalShell>
  );
}
