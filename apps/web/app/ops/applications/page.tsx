import Link from "next/link";
import { PortalShell } from "../../../components/portal-shell";
import { StatusBadge, EmptyState } from "../../../components/ui";
import { requirePortalViewer } from "../../../lib/viewer";

export const dynamic = "force-dynamic";

export default async function OpsApplicationsPage() {
  const viewer = await requirePortalViewer("ops", "/ops/applications");

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
        <div className="flex gap-2 mb-4">
          <Link href="?status=pending" className="action-button">Pending</Link>
          <Link href="?status=approved" className="action-button">Approved</Link>
          <Link href="?status=rejected" className="action-button">Rejected</Link>
          <Link href="?status=all" className="action-button">All</Link>
        </div>

        <EmptyState message="No applications to review." actionLabel="Check new applications" actionHref="?status=pending" />
      </div>
    </PortalShell>
  );
}
