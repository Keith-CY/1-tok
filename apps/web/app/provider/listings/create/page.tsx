import { PortalShell } from "../../../../components/portal-shell";
import { requirePortalViewer } from "../../../../lib/viewer";

export default async function ProviderListingsCreatePage() {
  const viewer = await requirePortalViewer("provider", "/provider/listings");

  return (
    <PortalShell
      eyebrow="Provider portal / listings"
      title="Create a listing."
      copy="Create your first listing here to showcase capacity and service profile."
      signal="Provider listings"
      asideTitle="Quick info"
      quickActions={[
        { label: "Back to listings", href: "/provider/listings", tone: "secondary" },
        { label: "Provider dashboard", href: "/provider", tone: "secondary" },
      ]}
      asideItems={[]}
    >
      <p className="text-sm text-gray-500">Create flow UI is intentionally minimal in this stage.</p>
    </PortalShell>
  );
}
