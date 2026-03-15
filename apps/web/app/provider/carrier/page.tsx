import Link from "next/link";
import { PortalShell } from "../../../components/portal-shell";
import { StatusBadge, EmptyState } from "../../../components/ui";
import { requirePortalViewer } from "../../../lib/viewer";

export const dynamic = "force-dynamic";

export default async function ProviderCarrierPage() {
  const viewer = await requirePortalViewer("provider", "/provider/carrier");

  return (
    <PortalShell
      eyebrow="Provider portal / carrier"
      title="Carrier integration status."
      copy="View your Carrier binding, execution profiles, and active jobs."
      signal="Carrier management"
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
                <p className="font-mono text-sm">—</p>
              </div>
              <div>
                <p className="text-sm text-gray-500">Agent</p>
                <p className="font-mono text-sm">—</p>
              </div>
              <div>
                <p className="text-sm text-gray-500">Backend</p>
                <p className="font-mono text-sm">—</p>
              </div>
            </div>
            <div className="mt-4">
              <Link href="/provider/carrier/register" className="bg-blue-600 text-white px-4 py-2 rounded text-sm">
                Register Carrier
              </Link>
            </div>
          </div>
        </section>

        <section>
          <h2 className="text-xl font-semibold mb-3">Active Jobs</h2>
          <EmptyState message="No active execution jobs." />
        </section>
      </div>
    </PortalShell>
  );
}
