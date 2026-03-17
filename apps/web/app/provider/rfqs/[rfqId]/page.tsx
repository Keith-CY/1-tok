import { PortalShell } from "../../../../components/portal-shell";
import { requirePortalViewer } from "../../../../lib/viewer";

export default async function ProviderRFQDetailsPage({ params }: { params: { rfqId: string } }) {
  const viewer = await requirePortalViewer("provider", "/provider/rfqs");

  return (
    <PortalShell
      eyebrow="Provider portal / RFQ"
      title={`RFQ ${params.rfqId}`}
      copy="Open this RFQ detail page to continue your bid workflow from the full request view."
      signal="RFQ detail"
      asideTitle="Quick info"
      quickActions={[
        { label: "Back to opportunities", href: "/provider/rfqs", tone: "secondary" },
        { label: "Search RFQs", href: "/provider/rfqs", tone: "secondary" },
      ]}
      asideItems={[]}
    >
      <p className="text-sm text-gray-500">
        This RFQ detail page is available to preserve a stable navigation target for review workflows.
      </p>
      <div className="mt-3 text-sm">You can submit bids from a dedicated action card in this view.</div>
    </PortalShell>
  );
}
