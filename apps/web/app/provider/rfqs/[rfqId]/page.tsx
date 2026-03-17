import { PortalShell } from "../../../../components/portal-shell";
import { EmptyState } from "../../../../components/ui";
import { getRFQs } from "../../../../lib/api";
import { formatCents } from "../../../../lib/currency";
import { requirePortalViewer } from "../../../../lib/viewer";

export const dynamic = "force-dynamic";

export default async function ProviderRFQDetailsPage({ params }: { params: { rfqId: string } }) {
  await requirePortalViewer("provider", "/provider/rfqs");
  const rfqs = await getRFQs({ requireLive: false });
  const rfq = rfqs.find((candidate) => candidate.id === params.rfqId);

  if (!rfq) {
    return (
      <PortalShell
        eyebrow="Provider portal / RFQ"
        title="RFQ not found"
        copy="This RFQ may have been removed or is not available in this environment."
        signal="RFQ detail"
        asideTitle="Quick info"
        quickActions={[{ label: "Back to opportunities", href: "/provider/rfqs", tone: "secondary" }]}
        asideItems={[]}
      >
        <EmptyState
          message="RFQ not found"
          actionLabel="Clear filters"
          actionHref="/provider/rfqs"
        />
      </PortalShell>
    );
  }

  const deadline = new Date(rfq.responseDeadlineAt).toLocaleString();

  return (
    <PortalShell
      eyebrow="Provider portal / RFQ"
      title={rfq.title}
      copy="Review details and take action on this request for quotes."
      signal="RFQ detail"
      asideTitle="Quick info"
      quickActions={[
        { label: "Back to opportunities", href: "/provider/rfqs", tone: "secondary" },
      ]}
      asideItems={[
        { label: "Buyer", value: rfq.buyerOrgId },
        { label: "Category", value: rfq.category },
        { label: "Status", value: rfq.status },
        { label: "Budget", value: formatCents(rfq.budgetCents) },
      ]}
    >
      <div className="space-y-5">
        <section className="rounded-lg border p-4">
          <h2 className="text-sm font-semibold text-gray-400">RFQ detail</h2>
          <p className="mt-2 text-sm text-gray-500">Scope: {rfq.scope}</p>
          <p className="mt-1 text-sm text-gray-500">Response deadline: {deadline}</p>
          <p className="mt-1 text-sm text-gray-500">Created: {new Date(rfq.createdAt).toLocaleString()}</p>
        </section>

        <section className="rounded-lg border p-4">
          <h2 className="text-sm font-semibold text-gray-400">Submit a bid</h2>
          <form className="space-y-3 mt-3" method="post" action={`/provider/rfqs/${rfq.id}/bids`}>
            <label className="block">
              <span className="text-sm text-gray-700">Message</span>
              <textarea
                name="message"
                required
                minLength={12}
                placeholder="Describe your delivery plan, milestones, and timeline."
                className="mt-1 w-full border rounded px-3 py-2"
              />
            </label>
            <label className="block">
              <span className="text-sm text-gray-700">Quote (cents)</span>
              <input
                name="quoteCents"
                required
                type="number"
                min="1"
                defaultValue={String(Math.max(Math.floor(rfq.budgetCents * 0.6), 1000))}
                className="mt-1 w-full border rounded px-3 py-2"
              />
            </label>

            {rfq.status === "open" ? (
              <button type="submit" className="action-button">
                Submit bid for this RFQ
              </button>
            ) : (
              <p className="text-sm text-gray-500">This RFQ is no longer open for new bids.</p>
            )}
          </form>
        </section>
      </div>
    </PortalShell>
  );
}
