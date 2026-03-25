import Link from "next/link";
import { RiArrowLeftLine, RiArrowRightUpLine } from "react-icons/ri";

import { formatMoney } from "@1tok/contracts";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { SectionCard, WorkspaceShell } from "@/components/workspace-shell";
import { getOrders } from "@/lib/api";
import { requirePortalViewer } from "@/lib/viewer";

export const metadata = {
  title: "Buyer Delivery",
};

export const dynamic = "force-dynamic";

export default async function DeliveryPage() {
  const viewer = await requirePortalViewer("buyer", "/buyer/delivery");
  const orders = await getOrders({ authToken: viewer.token, requireLive: true });
  const buyerOrders = orders
    .filter((order) => order.buyerOrgId === viewer.membership.organization.id)
    .sort((left, right) => {
      // Sort by order ID descending as a proxy for creation time
      return right.id.localeCompare(left.id);
    });

  return (
    <WorkspaceShell
      role="buyer"
      title="Delivery"
      description="Requests that cleared pricing and moved into execution. Track milestones and spend here."
      status={`${buyerOrders.length} awarded`}
      actions={[{ href: "/buyer", label: "Back to requests", icon: RiArrowLeftLine, variant: "outline" }]}
    >
      <SectionCard
        eyebrow="Awarded requests"
        title="Delivery pipeline"
        description="Each row is a request that already cleared the board. Click through to see milestones and spend."
      >
        <div className="space-y-3">
          {buyerOrders.length === 0 ? (
            <Card className="bg-card p-6 text-sm text-muted-foreground">
              No awarded requests yet. Once a request is awarded, it will appear here.
            </Card>
          ) : (
            buyerOrders.map((order) => {
              const totalBudget = order.milestones.reduce((sum, ms) => sum + ms.budgetCents, 0);
              const totalSpent = order.milestones.reduce(
                (sum, ms) =>
                  sum +
                  ms.settledCents +
                  (Array.isArray(ms.usageCharges) ? ms.usageCharges.reduce((u, c) => u + c.amountCents, 0) : 0),
                0,
              );
              const runningMilestone = order.milestones.find((ms) => ms.state === "running");

              return (
                <Link key={order.id} href={`/buyer/orders/${order.id}`} className="block">
                  <div className="market-row cursor-pointer transition-colors hover:bg-accent/5">
                    <div className="grid gap-5 xl:grid-cols-[1.5fr_0.7fr_0.7fr_0.7fr_auto] xl:items-end">
                      <div className="min-w-0 space-y-3">
                        <div className="text-[11px] font-semibold uppercase tracking-[0.24em] text-accent">
                          {order.status === "running" ? "In delivery" : order.status}
                        </div>
                        <h3 className="font-display text-2xl font-medium leading-[1.05] tracking-[-0.02em] break-words text-balance">
                          {order.id}
                        </h3>
                      </div>
                      <DeliveryMetric label="Budget" value={formatMoney(totalBudget)} />
                      <DeliveryMetric label="Spent" value={formatMoney(totalSpent)} />
                      <DeliveryMetric
                        label="Current milestone"
                        value={runningMilestone ? runningMilestone.title : "None"}
                      />
                      <Button variant="outline" size="sm" className="w-full xl:w-auto" tabIndex={-1}>
                        View
                        <RiArrowRightUpLine className="size-4" />
                      </Button>
                    </div>
                  </div>
                </Link>
              );
            })
          )}
        </div>
      </SectionCard>
    </WorkspaceShell>
  );
}

function DeliveryMetric({ label, value }: { label: string; value: string }) {
  return (
    <div className="min-w-0">
      <div className="text-[11px] font-semibold uppercase tracking-[0.22em] text-muted-foreground">{label}</div>
      <div className="mt-3 font-mono text-xl font-semibold leading-tight tracking-tight tabular-nums break-words text-foreground">
        {value}
      </div>
    </div>
  );
}
