import { PortalShell } from "../../../../components/portal-shell";
import { StatusBadge, ProgressBar } from "../../../../components/ui";
import { formatCents, formatBudgetUsage, budgetUsageColor } from "../../../../lib/currency";
import { formatStars, ratingColor } from "../../../../lib/rating";

export const dynamic = "force-dynamic";

// Demo data — will be replaced with API calls
const demoOrder = {
  id: "ord_14",
  status: "running",
  buyerOrgId: "buyer_1",
  providerOrgId: "provider_1",
  fundingMode: "credit",
  milestones: [
    { id: "ms_1", title: "Execution design", state: "settled", budgetCents: 1800, settledCents: 1200, usageCharges: [] },
    { id: "ms_2", title: "Provider validation", state: "running", budgetCents: 1200, settledCents: 0, usageCharges: [{ kind: "token", amountCents: 340 }] },
  ],
};

const orderStatusTone: Record<string, "default" | "mint" | "warning" | "danger"> = {
  draft: "default",
  running: "mint",
  awaiting_budget: "warning",
  completed: "mint",
  failed: "danger",
};

export default async function OrderDetailPage({ params }: { params: { orderId: string } }) {
  const { orderId } = params;
  const order = demoOrder; // TODO: fetch from API

  return (
    <PortalShell
      eyebrow="Buyer portal / order"
      title={`Order ${orderId}`}
      copy="Track milestone progress, budget consumption, and carrier execution status."
      signal={order.status}
      asideTitle="Order signal deck"
      asideItems={[
        { label: "Status", value: order.status, tone: orderStatusTone[order.status] ?? "default" },
        { label: "Funding", value: order.fundingMode },
        { label: "Provider", value: order.providerOrgId },
        { label: "Milestones", value: `${order.milestones.length}` },
      ]}
    >
      <div className="space-y-6">
        {/* Order Header */}
        <div className="flex items-center gap-4">
          <StatusBadge status={order.status} />
          <span className="text-sm text-gray-500">Funding: {order.fundingMode}</span>
          <span className="text-sm text-gray-500">Provider: {order.providerOrgId}</span>
        </div>

        {/* Milestones */}
        <section>
          <h2 className="text-xl font-semibold mb-3">Milestones</h2>
          <div className="space-y-3">
            {order.milestones.map((ms) => {
              const spent = ms.usageCharges.reduce((sum: number, c: any) => sum + c.amountCents, 0) + ms.settledCents;
              const usage = ms.budgetCents > 0 ? spent / ms.budgetCents : 0;
              return (
                <div key={ms.id} className="border rounded-lg p-4">
                  <div className="flex items-center justify-between mb-2">
                    <div className="flex items-center gap-2">
                      <h3 className="font-medium">{ms.title}</h3>
                      <StatusBadge status={ms.state} />
                    </div>
                    <div className="text-right">
                      <span className={`text-sm font-medium ${budgetUsageColor(spent, ms.budgetCents)}`}>
                        {formatCents(spent)} / {formatCents(ms.budgetCents)}
                      </span>
                      <span className="text-xs text-gray-400 ml-1">
                        ({formatBudgetUsage(spent, ms.budgetCents)})
                      </span>
                    </div>
                  </div>
                  <ProgressBar value={usage} />
                </div>
              );
            })}
          </div>
        </section>

        {/* Actions */}
        <section className="flex gap-3">
          <a href={`/buyer/orders/${orderId}/messages`} className="px-4 py-2 border rounded text-sm hover:bg-gray-50">
            💬 Messages
          </a>
          <a href={`/buyer/orders/${orderId}/timeline`} className="px-4 py-2 border rounded text-sm hover:bg-gray-50">
            📋 Timeline
          </a>
          {order.status === "awaiting_budget" && (
            <button className="bg-green-600 text-white px-4 py-2 rounded text-sm">
              💰 Top Up Budget
            </button>
          )}
        </section>
      </div>
    </PortalShell>
  );
}
