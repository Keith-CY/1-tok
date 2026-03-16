import { PortalShell } from "../../../components/portal-shell";
import { EmptyState } from "../../../components/ui";
import { requirePortalViewer } from "../../../lib/viewer";

export const dynamic = "force-dynamic";

export default async function ProviderListingsPage() {
  const viewer = await requirePortalViewer("provider", "/provider/listings");

  return (
    <PortalShell
      eyebrow="Provider portal / listings"
      title="Manage your listings."
      copy="Create and edit listings to showcase your agent runtime capabilities."
      signal="Provider listings"
      asideTitle="Quick info"
      quickActions={[
        { label: "Create first listing", href: "/provider/listings/create", tone: "primary" },
        { label: "Return to provider dashboard", href: "/provider", tone: "secondary" },
      ]}
      asideItems={[]}
    >
      <div className="space-y-4">
        <div className="flex justify-between items-center">
          <h2 className="text-lg font-semibold">Your Listings</h2>
          <a href="/provider/listings/create" className="action-button">
            + New Listing
          </a>
        </div>

        <EmptyState message="No listings yet. Create your first listing to start receiving RFQs." actionLabel="Create your first listing" actionHref="/provider/listings/create" />
      </div>
    </PortalShell>
  );
}
