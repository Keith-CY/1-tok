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
    >
      <div className="space-y-4">
        <div className="flex gap-2 mb-4">
          <a href="?status=pending" className="px-3 py-1 bg-yellow-100 text-yellow-800 rounded text-sm">Pending</a>
          <a href="?status=approved" className="px-3 py-1 bg-green-100 text-green-800 rounded text-sm">Approved</a>
          <a href="?status=rejected" className="px-3 py-1 bg-red-100 text-red-800 rounded text-sm">Rejected</a>
          <a href="?" className="px-3 py-1 bg-gray-100 text-gray-800 rounded text-sm">All</a>
        </div>

        <EmptyState message="No applications to review." />
      </div>
    </PortalShell>
  );
}
