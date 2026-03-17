import { PortalShell } from "../../../../components/portal-shell";
import { requirePortalViewer } from "../../../../lib/viewer";

export default async function OpsApplicationDetailPage({ params }: { params: { applicationId: string } }) {
  const viewer = await requirePortalViewer("ops", "/ops/applications");

  return (
    <PortalShell
      eyebrow="Ops portal / applications"
      title={`Application ${params.applicationId}`}
      copy="Review application details and continue triage from this dedicated view."
      signal="Application review"
      asideTitle="Quick info"
      quickActions={[
        { label: "Back to applications", href: "/ops/applications", tone: "secondary" },
        { label: "Open disputes", href: "/ops/disputes", tone: "secondary" },
      ]}
      asideItems={[]}
    >
      <p className="text-sm text-gray-500">This detail page is available to keep the Open application action navigable.</p>
    </PortalShell>
  );
}
